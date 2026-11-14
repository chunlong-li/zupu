using System.Formats.Tar;
using System.IO.Compression;

namespace ZupuClient;

/// <summary>
/// 服务端一键安装 / 全量更新。
/// 服务端发包是全量包（二进制 + web 资源），所以"下载包 → 解压到位"就等于安装/更新；
/// 用户数据目录 data\ 与 knowledge\ 永远不覆盖。
/// </summary>
internal static class ServerInstaller
{
    public static string DefaultDir => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "ZupuServer");

    public static string InstallDir
    {
        get => string.IsNullOrWhiteSpace(Prefs.Current.ServerDir) ? DefaultDir : Prefs.Current.ServerDir;
        set { Prefs.Current.ServerDir = value?.Trim() ?? ""; Prefs.Save(); }
    }

    public static string ExePath => Path.Combine(InstallDir, "familytree.exe");
    public static string StartBatPath => Path.Combine(InstallDir, "start.bat");
    public static bool IsInstalled => File.Exists(ExePath);

    public static string InstalledVersion
    {
        get
        {
            try
            {
                var f = Path.Combine(InstallDir, ".installed-version");
                return File.Exists(f) ? File.ReadAllText(f).Trim() : (IsInstalled ? "未知" : "未安装");
            }
            catch { return "未知"; }
        }
    }

    public static string FeedUrl
    {
        get => string.IsNullOrWhiteSpace(Prefs.Current.ServerFeed)
            ? UpdateFeed.DefaultServerFeed
            : Prefs.Current.ServerFeed;
        set { Prefs.Current.ServerFeed = value?.Trim() ?? ""; Prefs.Save(); }
    }

    public static Task<UpdateManifest> CheckAsync(CancellationToken ct = default)
        => UpdateFeed.FetchManifestAsync(FeedUrl, ct);

    /// <summary>从包清单里取最新的 Windows 全量包</summary>
    public static async Task<UpdateManifest> CheckLatestAsync(CancellationToken ct = default)
    {
        var list = await UpdateFeed.FetchPackagesAsync(FeedUrl, ct);
        var p = UpdateFeed.PickLatestWindows(list);
        return new UpdateManifest(p.Version, p.Url, p.Size, p.Sha256, p.Notes, p.ReleaseAt);
    }

    /// <summary>下载全量包到数据目录（带进度）</summary>
    public static async Task<string> DownloadAsync(
        UpdateManifest m, IProgress<int>? progress, CancellationToken ct = default)
    {
        if (string.IsNullOrWhiteSpace(m.Url)) throw new InvalidDataException("清单里没有下载地址");

        // 下载文件名一律用 ASCII：源站包名是中文，落到磁盘上容易在别的工具里显示成乱码
        var urlPath = new Uri(m.Url).LocalPath.ToLowerInvariant();
        var ext = urlPath.EndsWith(".tar.gz") ? ".tar.gz"
                : urlPath.EndsWith(".tgz") ? ".tgz"
                : Path.GetExtension(urlPath);
        if (string.IsNullOrEmpty(ext)) ext = ".bin";
        var safeVer = new string((string.IsNullOrWhiteSpace(m.Version) ? "pkg" : m.Version)
            .Where(ch => char.IsLetterOrDigit(ch) || ch == '.' || ch == '-' || ch == '_').ToArray());
        if (safeVer.Length == 0) safeVer = "pkg";
        var dest = Path.Combine(AppPaths.DataDir, "update", "server-" + safeVer + ext);

        if (File.Exists(dest) && m.Sha256.Length > 0
            && !string.Equals(UpdateFeed.Sha256OfFile(dest), m.Sha256, StringComparison.OrdinalIgnoreCase))
            File.Delete(dest);

        if (!File.Exists(dest))
            await UpdateFeed.DownloadAsync(m.Url, dest, progress, ct);

        if (m.Sha256.Length > 0)
        {
            var actual = UpdateFeed.Sha256OfFile(dest);
            if (!string.Equals(actual, m.Sha256, StringComparison.OrdinalIgnoreCase))
            {
                try { File.Delete(dest); } catch { }
                throw new InvalidDataException($"校验失败：期望 {m.Sha256[..12]}…，实际 {actual[..12]}…");
            }
        }
        return dest;
    }

    /// <summary>把包解压安装到目标目录（支持 .zip / .tar.gz / .tgz）</summary>
    public static void InstallFromPackage(string packagePath, string targetDir,
        Action<string> log, string version = "")
    {
        var tmp = Path.Combine(AppPaths.DataDir, "update", "unpack-" + Guid.NewGuid().ToString("N")[..8]);
        if (Directory.Exists(tmp)) Directory.Delete(tmp, true);
        Directory.CreateDirectory(tmp);

        try
        {
            log("正在解包…");
            Extract(packagePath, tmp);

            var root = FindServerRoot(tmp)
                ?? throw new InvalidDataException(
                    "包里找不到 familytree.exe。\n" +
                    "如果这是 Linux 版全量包（只有无扩展名的 familytree），Windows 上不能直接安装，\n" +
                    "需要发布一个 Windows 版全量包。");

            Directory.CreateDirectory(targetDir);
            log("正在复制到 " + targetDir + " …");
            CopyTree(root, targetDir, log);

            if (!File.Exists(Path.Combine(targetDir, "familytree.exe")))
                throw new InvalidDataException("复制完成但目标目录里没有 familytree.exe");

            WriteStartBat(targetDir);
            if (!string.IsNullOrWhiteSpace(version))
                File.WriteAllText(Path.Combine(targetDir, ".installed-version"), version);
            log("安装完成");
        }
        finally
        {
            try { Directory.Delete(tmp, true); } catch { }
        }
    }

    private static void Extract(string packagePath, string destDir)
    {
        var lower = packagePath.ToLowerInvariant();
        if (lower.EndsWith(".zip"))
        {
            ZipFile.ExtractToDirectory(packagePath, destDir, overwriteFiles: true);
            return;
        }
        if (lower.EndsWith(".tar.gz") || lower.EndsWith(".tgz") || lower.EndsWith(".tar"))
        {
            using var fs = File.OpenRead(packagePath);
            using var gz = lower.EndsWith(".tar") ? (Stream)fs : new GZipStream(fs, CompressionMode.Decompress);
            using var reader = new TarReader(gz);
            while (reader.GetNextEntry() is { } entry)
            {
                var target = Path.GetFullPath(Path.Combine(destDir, entry.Name));
                if (!target.StartsWith(Path.GetFullPath(destDir), StringComparison.OrdinalIgnoreCase)) continue; // 防目录穿越
                if (entry.EntryType == TarEntryType.Directory)
                {
                    Directory.CreateDirectory(target);
                    continue;
                }
                if (entry.EntryType is TarEntryType.SymbolicLink or TarEntryType.HardLink) continue;
                Directory.CreateDirectory(Path.GetDirectoryName(target)!);
                entry.ExtractToFile(target, overwrite: true);
            }
            return;
        }
        throw new InvalidDataException("不支持的包格式（只支持 .zip / .tar.gz）");
    }

    /// <summary>找到含 familytree.exe 的目录（包可能多套一层）</summary>
    private static string? FindServerRoot(string dir)
    {
        if (File.Exists(Path.Combine(dir, "familytree.exe"))) return dir;
        foreach (var sub in Directory.GetDirectories(dir))
        {
            if (File.Exists(Path.Combine(sub, "familytree.exe"))) return sub;
        }
        return null;
    }

    /// <summary>复制文件，跳过用户数据目录</summary>
    private static void CopyTree(string src, string dst, Action<string> log)
    {
        var skip = new[] { "data", "knowledge" };
        var copied = 0;

        foreach (var file in Directory.EnumerateFiles(src, "*", SearchOption.AllDirectories))
        {
            var rel = Path.GetRelativePath(src, file);
            var first = rel.Split(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar)[0];
            if (skip.Any(s => s.Equals(first, StringComparison.OrdinalIgnoreCase))) continue;

            var target = Path.Combine(dst, rel);
            Directory.CreateDirectory(Path.GetDirectoryName(target)!);
            File.Copy(file, target, overwrite: true);
            copied++;
        }
        log($"已写入 {copied} 个文件");
    }

    public static void WriteStartBat(string dir)
    {
        var bat = string.Join("\r\n", new[]
        {
            "@echo off",
            "chcp 65001 >nul",
            "cd /d \"%~dp0\"",
            $"set FAMILY_TREE_UPDATE_URL={FeedUrl}",
            "set FAMILY_TREE_UPDATE_INSECURE=true",
            "echo ==========================================",
            "echo   Family Tree System starting...",
            "echo   Local access:  http://127.0.0.1:8080",
            "echo   Close this window to stop the service",
            "echo ==========================================",
            "familytree.exe",
            "pause",
        }) + "\r\n";
        File.WriteAllText(Path.Combine(dir, "start.bat"), bat, System.Text.Encoding.Default);
    }

    public static void Start()
    {
        if (!File.Exists(StartBatPath)) throw new FileNotFoundException("还没安装服务端", StartBatPath);
        System.Diagnostics.Process.Start(new System.Diagnostics.ProcessStartInfo
        {
            FileName = StartBatPath,
            WorkingDirectory = InstallDir,
            UseShellExecute = true,
            WindowStyle = System.Diagnostics.ProcessWindowStyle.Minimized,
        });
    }

    public static bool IsRunning()
        => System.Diagnostics.Process.GetProcessesByName("familytree").Length > 0;
}
