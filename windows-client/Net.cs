using System.Net.Http.Json;
using System.Net.Security;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text.Json;

namespace ZupuClient;

public static class Net
{
    /// <summary>
    /// 证书校验：正常证书直接通过；只有**本机/内网地址**的自签证书才放宽，
    /// 且采用首次信任 + 指纹固定（TOFU）—— 首次见到的证书指纹会被记录，
    /// 之后同一主机证书一旦变化即拒绝，可发现内网中间人。公网地址保持严格校验。
    /// </summary>
    public static readonly HttpClient Http = new(new HttpClientHandler
    {
        ServerCertificateCustomValidationCallback = ValidateServerCertificate
    })
    { Timeout = TimeSpan.FromSeconds(20) };

    private static bool ValidateServerCertificate(
        HttpRequestMessage req, X509Certificate2? cert, X509Chain? chain, SslPolicyErrors errors)
    {
        if (errors == SslPolicyErrors.None) return true;
        if (cert is null) return false;

        var host = req.RequestUri?.Host;
        if (string.IsNullOrEmpty(host) || !IsLocalOrPrivateHost(host)) return false;

        var fp = Convert.ToHexString(SHA256.HashData(cert.GetRawCertData()));
        var pinned = Prefs.GetPin(host);
        if (string.IsNullOrEmpty(pinned))
        {
            Prefs.SetPin(host, fp);   // 首次信任
            return true;
        }
        return string.Equals(pinned, fp, StringComparison.OrdinalIgnoreCase);
    }

    /// <summary>本机、私有网段或 .local 主机名</summary>
    public static bool IsLocalOrPrivateHost(string host)
    {
        if (string.IsNullOrEmpty(host)) return false;
        if (host.Equals("localhost", StringComparison.OrdinalIgnoreCase)) return true;
        if (host.EndsWith(".local", StringComparison.OrdinalIgnoreCase)) return true;
        if (!System.Net.IPAddress.TryParse(host, out var ip)) return false;

        var b = ip.GetAddressBytes();
        if (ip.AddressFamily == System.Net.Sockets.AddressFamily.InterNetwork)
        {
            return b[0] == 10
                || (b[0] == 172 && b[1] >= 16 && b[1] <= 31)
                || (b[0] == 192 && b[1] == 168)
                || (b[0] == 169 && b[1] == 254)
                || b[0] == 127;
        }
        if (ip.AddressFamily == System.Net.Sockets.AddressFamily.InterNetworkV6)
        {
            // ::1 / fe80::/10（链路本地）/ fc00::/7（唯一本地）
            bool loopback = b.Length == 16 && b.Take(15).All(x => x == 0) && b[15] == 1;
            bool linkLocal = (b[0] == 0xFE) && ((b[1] & 0xC0) == 0x80);
            bool uniqueLocal = (b[0] & 0xFE) == 0xFC;
            return loopback || linkLocal || uniqueLocal;
        }

        return false;
    }

    public static string Normalize(string input)
    {
        var s = (input ?? "").Trim();
        if (s.Length == 0) return "";
        if (!s.StartsWith("http://") && !s.StartsWith("https://")) s = "http://" + s;
        return s.TrimEnd('/');
    }

    public record LoginResult(string Token, string UserJson, string Role, string UserName);

    public static async Task<LoginResult> LoginAsync(string baseUrl, string user, string pass)
    {
        var resp = await Http.PostAsJsonAsync($"{baseUrl}/api/auth/login", new { username = user, password = pass });
        var text = await resp.Content.ReadAsStringAsync();
        if (!resp.IsSuccessStatusCode)
        {
            string msg = "登录失败";
            try { msg = JsonDocument.Parse(text).RootElement.GetProperty("error").GetString() ?? msg; } catch { }
            throw new Exception(msg);
        }
        using var doc = JsonDocument.Parse(text);
        var root = doc.RootElement;
        var token = root.GetProperty("token").GetString() ?? "";
        var userJson = root.TryGetProperty("user", out var u) ? u.GetRawText() : "{}";
        var role = u.ValueKind == JsonValueKind.Object && u.TryGetProperty("role", out var r) ? r.GetString() ?? "" : "";
        var name = u.ValueKind == JsonValueKind.Object && u.TryGetProperty("username", out var n) ? n.GetString() ?? user : user;
        return new LoginResult(token, userJson, role, name);
    }

    /// <summary>扫本机所在 /24 网段的 8080/8888/8090，命中 /health 的即族谱服务</summary>
    public static async Task<List<string>> DiscoverAsync(IProgress<string>? progress = null, CancellationToken ct = default)
    {
        var found = new List<string>();
        var local = System.Net.NetworkInformation.NetworkInterface.GetAllNetworkInterfaces()
            .Where(i => i.OperationalStatus == System.Net.NetworkInformation.OperationalStatus.Up)
            .SelectMany(i => i.GetIPProperties().UnicastAddresses)
            .Select(a => a.Address)
            .FirstOrDefault(a => a.AddressFamily == System.Net.Sockets.AddressFamily.InterNetwork && !System.Net.IPAddress.IsLoopback(a));
        if (local == null) return found;

        var prefix = string.Join('.', local.ToString().Split('.').Take(3));
        int[] ports = { 8080, 8888, 8090 };
        var tasks = new List<Task>();
        var sem = new SemaphoreSlim(64);
        foreach (var ip in Enumerable.Range(1, 254))
        {
            foreach (var port in ports)
            {
                var addr = $"{prefix}.{ip}:{port}";
                await sem.WaitAsync(ct);
                tasks.Add(Task.Run(async () =>
                {
                    try
                    {
                        using var cts = CancellationTokenSource.CreateLinkedTokenSource(ct);
                        cts.CancelAfter(800);
                        var r = await Http.GetAsync($"http://{addr}/health", cts.Token);
                        if (r.IsSuccessStatusCode)
                        {
                            var body = await r.Content.ReadAsStringAsync(cts.Token);
                            if (body.Contains("ok", StringComparison.OrdinalIgnoreCase))
                            {
                                lock (found) { found.Add(addr); }
                                progress?.Report(addr);
                            }
                        }
                    }
                    catch { /* 忽略单个探测失败 */ }
                    finally { sem.Release(); }
                }, ct));
            }
        }
        await Task.WhenAll(tasks);
        return found.OrderBy(a => a).ToList();
    }
}
