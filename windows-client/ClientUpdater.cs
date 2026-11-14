using System.Diagnostics;

namespace ZupuClient;

/// <summary>
/// 客户端自更新：查清单 → 下载 → 校验 → 解压 → 交给辅助脚本替换并重启。
/// 运行中的 exe 无法自我覆盖，所以用批处理等进程退出后再 robocopy。
/// </summary>
internal static class ClientUpdater
{
    public static string AppDir => AppContext.BaseDirectory.TrimEnd('\\');

    /// <summary>
    /// 完整版本号（可能是 4 段或 5 段的测试版号，如 1.2.0.1112.1）。
    /// 取 InformationalVersion；没有则退回程序集版本。
    /// </summary>
    public static string CurrentVersion
    {
        get
        {
            var asm = typeof(ClientUpdater).Assembly;
            var info = asm
                .GetCustomAttributes(typeof(System.Reflection.AssemblyInformationalVersionAttribute), false)
                .OfType<System.Reflection.AssemblyInformationalVersionAttribute>()
                .FirstOrDefault()?.InformationalVersion;
            if (!string.IsNullOrWhiteSpace(info))
            {
                var v = info.Split('+')[0].Trim();     // 去掉 +commit 元数据
                if (v.Length > 0) return v;
            }
            return asm.GetName().Version?.ToString(3) ?? "0.0.0";
        }
    }

    /// <summary>三段基础版本（正式版号）</summary>
    public static string BaseVersion
    {
        get
        {
            var parts = CurrentVersion.Split('.');
            return parts.Length >= 3 ? string.Join(".", parts.Take(3)) : CurrentVersion;
        }
    }

    private static string UpdateRoot => Path.Combine(AppPaths.DataDir, "update");

    public static string FeedUrl
    {
        get => string.IsNullOrWhiteSpace(Prefs.Current.ClientFeed)
            ? UpdateFeed.DefaultClientFeed
            : Prefs.Current.ClientFeed;
        set { Prefs.Current.ClientFeed = value?.Trim() ?? ""; Prefs.Save(); }
    }

    public static Task<UpdateManifest> CheckAsync(CancellationToken ct = default)
        => UpdateFeed.FetchManifestAsync(FeedUrl, ct);

    /// <summary>下载并把包解压到 update\&lt;版本&gt;\，返回该目录</summary>
    public static async Task<string> DownloadAndStageAsync(
        UpdateManifest m, IProgress<int>? progress, CancellationToken ct = default)
    {
        if (string.IsNullOrWhiteSpace(m.Url)) throw new InvalidDataException("清单里没有下载地址");

        var zip = Path.Combine(UpdateRoot, "client-" + m.Version + ".zip");
        if (File.Exists(zip) && m.Sha256.Length > 0
            && !string.Equals(UpdateFeed.Sha256OfFile(zip), m.Sha256, StringComparison.OrdinalIgnoreCase))
            File.Delete(zip);

        if (!File.Exists(zip))
            await UpdateFeed.DownloadAsync(m.Url, zip, progress, ct);

        if (m.Sha256.Length > 0)
        {
            var actual = UpdateFeed.Sha256OfFile(zip);
            if (!string.Equals(actual, m.Sha256, StringComparison.OrdinalIgnoreCase))
            {
                try { File.Delete(zip); } catch { }
                throw new InvalidDataException($"校验失败：期望 {m.Sha256[..12]}…，实际 {actual[..12]}…");
            }
        }

        var stage = Path.Combine(UpdateRoot, "stage-" + m.Version);
        if (Directory.Exists(stage)) Directory.Delete(stage, true);
        Directory.CreateDirectory(stage);
        System.IO.Compression.ZipFile.ExtractToDirectory(zip, stage, overwriteFiles: true);

        // 有的包会多套一层目录
        var probe = Directory.GetDirectories(stage);
        if (probe.Length == 1 && !File.Exists(Path.Combine(stage, "ZupuClient.exe")))
        {
            var inner = probe[0];
            if (File.Exists(Path.Combine(inner, "ZupuClient.exe"))) return inner;
        }
        return stage;
    }

    /// <summary>
    /// 交给独立更新器 ZupuUpdater.exe：它等本进程退出后覆盖文件并重新拉起客户端。
    /// 找不到更新器时退回旧的批处理方式（仅作兜底）。
    /// </summary>
    public static void ApplyAndRestart(string stagedDir, string version)
    {
        if (!File.Exists(Path.Combine(stagedDir, "ZupuClient.exe")))
            throw new InvalidDataException("更新包里没有 ZupuClient.exe，无法安装");

        Directory.CreateDirectory(UpdateRoot);
        var zip = Path.Combine(UpdateRoot, "client-" + version + ".zip");
        var log = Path.Combine(UpdateRoot, "updater.log");

        // 优先用更新包里自带的那份（与本次版本配套，也能随包升级）；
        // 包内没有时用程序目录里已安装的那份。
        var stagedUpdater = Path.Combine(stagedDir, "ZupuUpdater.exe");
        var installedUpdater = Path.Combine(AppDir, "ZupuUpdater.exe");
        var updater = File.Exists(stagedUpdater) ? stagedUpdater
                    : File.Exists(installedUpdater) ? installedUpdater
                    : null;

        if (updater is not null)
        {
            // ArgumentList 会按 Windows 规则正确转义，中文路径不会乱码
            var psi = new ProcessStartInfo
            {
                FileName = updater,
                WorkingDirectory = AppDir,
                UseShellExecute = false,
                CreateNoWindow = true,
            };
            psi.ArgumentList.Add("--wait-pid");
            psi.ArgumentList.Add(Environment.ProcessId.ToString());
            psi.ArgumentList.Add("--stage");
            psi.ArgumentList.Add(stagedDir);
            psi.ArgumentList.Add("--app");
            psi.ArgumentList.Add(AppDir);
            psi.ArgumentList.Add("--log");
            psi.ArgumentList.Add(log);
            if (File.Exists(zip))
            {
                psi.ArgumentList.Add("--clean-zip");
                psi.ArgumentList.Add(zip);
            }
            Process.Start(psi);
            return;
        }

        // —— 兜底：没有更新器时用批处理（老机器/包不完整）——
        var script = Path.Combine(UpdateRoot, "apply-update.cmd");
        var lines = new[]
        {
            "@echo off",
            "chcp 65001 >nul",
            "setlocal",
            "ping -n 3 127.0.0.1 >nul",
            $"robocopy \"{stagedDir}\" \"{AppDir}\" /E /NFL /NDL /NJH /NJS /NC /NS /R:3 /W:1 >nul",
            $"start \"\" \"{Path.Combine(AppDir, "ZupuClient.exe")}\"",
            "del \"%~f0\"",
        };
        File.WriteAllText(script, string.Join("\r\n", lines), new System.Text.UTF8Encoding(true));

        Process.Start(new ProcessStartInfo
        {
            FileName = "cmd.exe",
            Arguments = $"/c \"{script}\"",
            CreateNoWindow = true,
            UseShellExecute = false,
            WorkingDirectory = UpdateRoot,
        });
    }

    /// <summary>清理旧的暂存目录（保留最近一次）</summary>
    public static void CleanupStages()
    {
        try
        {
            if (!Directory.Exists(UpdateRoot)) return;
            foreach (var d in Directory.GetDirectories(UpdateRoot, "stage-*"))
            {
                try { Directory.Delete(d, true); } catch { }
            }
        }
        catch { }
    }
}
