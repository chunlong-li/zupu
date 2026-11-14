using System.Net;
using System.Security.Cryptography;
using System.Text.Json;

namespace ZupuClient;

/// <summary>更新清单（与源站 latest.json 同结构）</summary>
internal sealed record UpdateManifest(
    string Version,
    string Url,
    long Size,
    string Sha256,
    string Notes,
    string ReleaseAt)
{
    public string SizeText => Size > 0 ? $"{Size / 1048576.0:0.0} MB" : "未知大小";
}

/// <summary>
/// 更新源访问：拉清单、下载包、校验 SHA256。
/// 源站用自签证书，这里只对白名单主机放宽校验，其它域名仍然严格。
/// </summary>
internal static class UpdateFeed
{
    public const string DefaultClientFeed = "https://zupu-update.lijiacore.com:4440/Client/latest.json";
    /// <summary>服务端包清单（按平台区分，Windows 用 windows-amd64）</summary>
    public const string DefaultServerFeed = "https://zupu-update.lijiacore.com:4440/packages.json";

    /// <summary>包清单里的一条（服务端全量包）</summary>
    public sealed record ServerPackage(
        string Version, string Url, string Sha256, long Size,
        string Platform, string Channel, string Notes, string ReleaseAt);

    /// <summary>拉包清单并挑出最新的 Windows 稳定包</summary>
    public static async Task<List<ServerPackage>> FetchPackagesAsync(string registryUrl, CancellationToken ct = default)
    {
        using var http = CreateClient(allowInsecure: true);
        var text = await http.GetStringAsync(registryUrl, ct);
        using var doc = JsonDocument.Parse(text);
        if (!doc.RootElement.TryGetProperty("packages", out var arr) || arr.ValueKind != JsonValueKind.Array)
            throw new InvalidDataException("包清单里没有 packages 数组");

        var baseUri = new Uri(registryUrl);
        var list = new List<ServerPackage>();
        foreach (var p in arr.EnumerateArray())
        {
            string Get(string name) =>
                p.TryGetProperty(name, out var v) && v.ValueKind == JsonValueKind.String ? v.GetString() ?? "" : "";

            var url = Get("url");
            if (url.Length == 0) continue;
            if (!url.StartsWith("http", StringComparison.OrdinalIgnoreCase))
                url = new Uri(baseUri, url).ToString();

            var size = p.TryGetProperty("size", out var s) && s.TryGetInt64(out var sv) ? sv : 0;
            list.Add(new ServerPackage(
                Get("version"), url, Get("sha256").ToLowerInvariant(), size,
                Get("platform"), Get("channel"), Get("notes"), Get("releaseAt")));
        }
        return list;
    }

    /// <summary>从清单里挑最新的 Windows 稳定包</summary>
    public static ServerPackage PickLatestWindows(IEnumerable<ServerPackage> packages)
    {
        var win = packages
            .Where(p => p.Platform.StartsWith("windows", StringComparison.OrdinalIgnoreCase))
            .Where(p => p.Channel.Length == 0 || p.Channel.Equals("stable", StringComparison.OrdinalIgnoreCase))
            .ToList();
        if (win.Count == 0) throw new InvalidDataException("清单里没有 Windows 稳定包");

        return win.OrderByDescending(p => p.Version, Comparer<string>.Create(CompareVersion)).First();
    }

    private static readonly string[] InsecureHosts = { "zupu-update.lijiacore.com" };

    private static HttpClient CreateClient(bool allowInsecure)
    {
        var handler = new HttpClientHandler();
        if (allowInsecure)
        {
            handler.ServerCertificateCustomValidationCallback = (req, cert, chain, errors) =>
            {
                if (errors == System.Net.Security.SslPolicyErrors.None) return true;
                var host = req.RequestUri?.Host ?? "";
                return InsecureHosts.Any(h => host.Equals(h, StringComparison.OrdinalIgnoreCase));
            };
        }
        return new HttpClient(handler) { Timeout = TimeSpan.FromMinutes(10) };
    }

    /// <summary>拉取并解析清单；相对 url 会按清单地址补全</summary>
    public static async Task<UpdateManifest> FetchManifestAsync(string feedUrl, CancellationToken ct = default)
    {
        using var http = CreateClient(allowInsecure: true);
        var text = await http.GetStringAsync(feedUrl, ct);
        using var doc = JsonDocument.Parse(text);
        var r = doc.RootElement;

        string Get(string name) =>
            r.TryGetProperty(name, out var v) && v.ValueKind == JsonValueKind.String ? v.GetString() ?? "" : "";

        var version = Get("version");
        if (version.Length == 0) throw new InvalidDataException("清单里没有 version 字段");

        var url = Get("url");
        if (url.Length > 0 && !url.StartsWith("http", StringComparison.OrdinalIgnoreCase))
        {
            var baseUri = new Uri(feedUrl);
            var firstSeg = baseUri.AbsolutePath.Trim('/').Split('/')[0];

            // 清单里的相对地址两种写法都要能认：
            //   "1.2/xxx.zip"        → 相对清单所在目录
            //   "Client/1.2/xxx.zip" → 已含目录名，按站点根解析（否则会拼成 /Client/Client/...）
            var alreadyHasDir = firstSeg.Length > 0
                && url.StartsWith(firstSeg + "/", StringComparison.OrdinalIgnoreCase);
            url = new Uri(baseUri, alreadyHasDir ? "/" + url : url).ToString();
        }

        var size = r.TryGetProperty("size", out var s) && s.TryGetInt64(out var sv) ? sv : 0;
        return new UpdateManifest(version, url, size, Get("sha256").ToLowerInvariant(), Get("notes"), Get("releaseAt"));
    }

    /// <summary>下载到指定路径，回报 0-100</summary>
    public static async Task DownloadAsync(
        string url, string destPath, IProgress<int>? progress, CancellationToken ct = default)
    {
        Directory.CreateDirectory(Path.GetDirectoryName(destPath)!);
        using var http = CreateClient(allowInsecure: true);
        using var resp = await http.GetAsync(url, HttpCompletionOption.ResponseHeadersRead, ct);
        resp.EnsureSuccessStatusCode();

        var total = resp.Content.Headers.ContentLength ?? -1;
        await using var src = await resp.Content.ReadAsStreamAsync(ct);
        await using var dst = File.Create(destPath);

        var buffer = new byte[81920];
        long read = 0;
        int n;
        var lastPct = -1;
        while ((n = await src.ReadAsync(buffer, ct)) > 0)
        {
            await dst.WriteAsync(buffer.AsMemory(0, n), ct);
            read += n;
            if (total > 0)
            {
                var pct = (int)(read * 100 / total);
                if (pct != lastPct) { lastPct = pct; progress?.Report(pct); }
            }
            else progress?.Report(-1);
        }
    }

    public static string Sha256OfFile(string path)
    {
        using var fs = File.OpenRead(path);
        return Convert.ToHexString(SHA256.HashData(fs)).ToLowerInvariant();
    }

    /// <summary>版本号比较：a 比 b 新返回正数</summary>
    public static int CompareVersion(string a, string b)
    {
        static int[] Parts(string v) => v.Split('-', '+')[0].Split('.')
            .Select(p => int.TryParse(p, out var n) ? n : 0).ToArray();
        var pa = Parts(a);
        var pb = Parts(b);
        for (var i = 0; i < Math.Max(pa.Length, pb.Length); i++)
        {
            var x = i < pa.Length ? pa[i] : 0;
            var y = i < pb.Length ? pb[i] : 0;
            if (x != y) return x - y;
        }
        return 0;
    }
}
