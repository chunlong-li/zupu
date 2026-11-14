using System.Diagnostics;
using System.IO.Compression;
using System.Net.Http;
using System.Reflection;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using Microsoft.Win32;

namespace ZupuInstall;

/// <summary>
/// 族谱客户端「在线安装包」。
///
/// 控制台程序：双击后显示下载/解压进度，装完自动关窗。
/// 流程和自动更新一致：读清单 → 下载 → 校验 sha256 → 解压 → 快捷方式 → 卸载项。
///
/// 命令行（给运维/自动化用，全部装完即退出）：
///   ZupuInstall.exe                          在线安装（默认目录）
///   ZupuInstall.exe --dir "D:\族谱"           指定安装目录
///   ZupuInstall.exe --package app.zip --quiet 离线安装，不输出交互
/// </summary>
internal static class Program
{
    public const string DefaultFeed = "https://zupu-update.lijiacore.com:4440/Client/latest.json";
    public const string AppExeName = "ZupuClient.exe";

    private static int Main(string[] args)
    {
        var opt = Options.Parse(args);
        try { Console.OutputEncoding = Encoding.UTF8; } catch { }
        try { Console.Title = "族谱客户端 安装程序"; } catch { }

        if (opt.Help)
        {
            Usage();
            return 0;
        }

        try
        {
            Installer.Run(opt, new ConsoleProgress());
            Console.WriteLine();
            Console.WriteLine("安装完成。");
            return 0;      // 控制台窗口随之关闭，不留「按任意键」
        }
        catch (Exception ex)
        {
            Console.WriteLine();
            Console.WriteLine("安装失败：" + ex.Message);
            WriteErrorLog(ex);

            if (!opt.Quiet)
            {
                // 出错时留一下，别让窗口一闪而过看不见原因
                Console.WriteLine();
                Console.WriteLine("按任意键关闭…");
                try { Console.ReadKey(true); } catch { }
            }
            return 1;
        }
    }

    private static void WriteErrorLog(Exception ex)
    {
        try
        {
            var dir = Path.GetDirectoryName(Environment.ProcessPath!)!;
            File.AppendAllText(Path.Combine(dir, "zupu-install-error.log"),
                DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss ") + ex + Environment.NewLine, Encoding.UTF8);
        }
        catch { }
    }

    private static void Usage()
    {
        Console.WriteLine("""
            族谱客户端 安装程序

              ZupuInstall.exe                        在线安装（默认目录，装完自动关闭）
              ZupuInstall.exe --dir "D:\族谱"         安装到指定目录
              ZupuInstall.exe --package app.zip      离线安装（用本地包）
              ZupuInstall.exe --quiet                不等待、不提示（给脚本用）
              ZupuInstall.exe --autostart            安装后设置开机自启

            默认目录：%LOCALAPPDATA%\Programs\族谱客户端
            （不可写时自动依次尝试 用户目录、公共目录、安装器所在目录）
            """);
    }
}

internal sealed class ConsoleProgress : IProgressSink
{
    private int _last = -1;

    public void Report(string message, int percent)
    {
        // 进度变化才刷新，避免刷屏
        if (percent == _last && !message.StartsWith("正在解压", StringComparison.Ordinal)) return;
        _last = percent;
        Console.WriteLine(percent >= 0 ? $"{message}  {percent}%" : message);
    }
}

/// <summary>安装步骤（控制台与静默模式共用）</summary>
internal static class Installer
{
    public static string DefaultDir => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Programs", "族谱客户端");

    public static string DesktopLnk
    {
        get
        {
            var custom = Environment.GetEnvironmentVariable("ZUPU_SETUP_DESKTOP");
            var dir = string.IsNullOrWhiteSpace(custom)
                ? Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory)
                : custom!;
            return Path.Combine(dir, "族谱客户端.lnk");
        }
    }

    public static string StartMenuLnk => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.Programs), "族谱客户端.lnk");

    public static void Run(Options opt, IProgressSink ui)
    {
        var dir = string.IsNullOrWhiteSpace(opt.Dir) ? DefaultDir : opt.Dir!;
        Console.WriteLine("安装目录：" + dir);

        if (!TryEnsureWritable(dir, out var why))
        {
            if (!string.IsNullOrWhiteSpace(opt.Dir))
                throw new IOException("安装目录不可写：" + dir + "（" + why + "）");

            var tried = new List<string> { dir + "（" + why + "）" };
            string? found = null;
            foreach (var alt in FallbackDirs())
            {
                if (TryEnsureWritable(alt, out var w2)) { found = alt; break; }
                tried.Add(alt + "（" + w2 + "）");
            }
            if (found is null)
                throw new IOException("没有可写的安装目录，试过：\n" + string.Join("\n", tried));

            Console.WriteLine("默认位置不可写，改装到：" + found);
            dir = found;
        }

        KillRunning();

        var package = ResolvePackage(opt, dir, ui);

        Console.WriteLine("正在解压文件…");
        Directory.CreateDirectory(dir);
        using (var zip = ZipFile.OpenRead(package))
        {
            var total = Math.Max(1, zip.Entries.Count);
            var done = 0;
            foreach (var e in zip.Entries)
            {
                var rel = e.FullName.Replace('/', Path.DirectorySeparatorChar);
                var dst = Path.GetFullPath(Path.Combine(dir, rel));
                if (!dst.StartsWith(Path.GetFullPath(dir), StringComparison.OrdinalIgnoreCase)) continue;

                if (rel.EndsWith(Path.DirectorySeparatorChar)) Directory.CreateDirectory(dst);
                else
                {
                    Directory.CreateDirectory(Path.GetDirectoryName(dst)!);
                    e.ExtractToFile(dst, overwrite: true);
                }
                done++;
                if (done % 50 == 0 || done == total)
                {
                    Console.Write($"\r  解压 {done}/{total}   ");
                    if (done == total) Console.WriteLine();
                }
            }
        }

        if (Path.GetFileName(package).StartsWith("zupu-install-", StringComparison.OrdinalIgnoreCase))
        {
            try { File.Delete(package); } catch { }
        }

        var exe = Path.Combine(dir, Program.AppExeName);
        if (!File.Exists(exe)) throw new InvalidDataException("安装包里没有 ZupuClient.exe");

        Console.WriteLine("正在创建快捷方式…");
        var made = Shortcuts.TryCreate(DesktopLnk, exe, dir);
        Shortcuts.TryCreate(StartMenuLnk, exe, dir);
        if (!made)
            Console.WriteLine("  提示：桌面快捷方式没建成，可右键 ZupuClient.exe → 发送到 → 桌面快捷方式");

        WriteUninstallEntry(dir);

        if (opt.Autostart)
        {
            try
            {
                using var key = Registry.CurrentUser.CreateSubKey(
                    @"Software\Microsoft\Windows\CurrentVersion\Run", writable: true);
                key.SetValue("ZupuClient", "\"" + exe + "\" --autostart-launch", RegistryValueKind.String);
                Console.WriteLine("已设置开机自启");
            }
            catch { Console.WriteLine("开机自启设置失败（可稍后在客户端设置里开）"); }
        }

        ui.Report("安装完成", 100);
        Console.WriteLine("程序位置：" + exe);

        if (opt.RunAfter)
        {
            try
            {
                Process.Start(new ProcessStartInfo { FileName = exe, WorkingDirectory = dir, UseShellExecute = true });
            }
            catch { }
        }
    }

    /// <summary>目录能不能建、能不能写文件</summary>
    public static bool TryEnsureWritable(string dir, out string why)
    {
        why = "";
        try
        {
            Directory.CreateDirectory(dir);
            var probe = Path.Combine(dir, "zupu-write-test.tmp");
            using (var fs = new FileStream(probe, FileMode.Create, FileAccess.Write, FileShare.None, 4096))
                fs.WriteByte(0);
            File.Delete(probe);
            return true;
        }
        catch (Exception ex)
        {
            why = ex.GetType().Name + " " + ex.Message;
            return false;
        }
    }

    /// <summary>默认目录不可写时的备选安装位置（逐个试）</summary>
    public static IEnumerable<string> FallbackDirs()
    {
        var list = new List<string>();
        var local = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
        var roaming = Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData);
        var profile = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);

        list.Add(Path.Combine(local, "族谱客户端"));
        list.Add(Path.Combine(roaming, "族谱客户端"));
        list.Add(Path.Combine(profile, "族谱客户端"));

        try
        {
            var pub = Path.GetDirectoryName(Environment.GetFolderPath(Environment.SpecialFolder.CommonDocuments));
            if (!string.IsNullOrWhiteSpace(pub)) list.Add(Path.Combine(pub!, "族谱客户端"));
        }
        catch { }
        try
        {
            var pd = Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData);
            if (!string.IsNullOrWhiteSpace(pd)) list.Add(Path.Combine(pd, "族谱客户端"));
        }
        catch { }

        // 最后兜底：装到安装器自己旁边（受限环境里通常只有这个位置可写）
        try
        {
            var exe = Environment.ProcessPath;
            if (!string.IsNullOrWhiteSpace(exe))
                list.Add(Path.Combine(Path.GetDirectoryName(exe!)!, "族谱客户端"));
        }
        catch { }

        return list;
    }

    /// <summary>选一个真能写的下载位置</summary>
    public static string PickDownloadPath(string installDir, string ext = ".zip")
    {
        var name = "zupu-install-" + Guid.NewGuid().ToString("N") + ext;
        var dir = Path.GetFullPath(installDir);
        var candidates = new[]
        {
            dir,
            Path.GetTempPath(),
            Path.GetDirectoryName(dir) ?? dir,
        };

        var tried = new List<string>();
        foreach (var c in candidates)
        {
            if (string.IsNullOrWhiteSpace(c)) continue;
            try
            {
                Directory.CreateDirectory(c);
                var probe = Path.Combine(c, name);
                using (var fs = new FileStream(probe, FileMode.Create, FileAccess.Write, FileShare.None, 4096))
                    fs.WriteByte(0);
                File.Delete(probe);
                return probe;
            }
            catch (Exception ex)
            {
                tried.Add(c + "（" + ex.GetType().Name + "）");
            }
        }
        throw new IOException("没有可写的下载位置，试过：" + string.Join("、", tried));
    }

    public static string ResolvePackage(Options opt, string installDir, IProgressSink ui)
    {
        if (!string.IsNullOrWhiteSpace(opt.Package))
        {
            if (!File.Exists(opt.Package)) throw new FileNotFoundException("找不到本地安装包", opt.Package);
            ui.Report("使用本地安装包", 5);
            return opt.Package!;
        }

        var beside = Path.Combine(Path.GetDirectoryName(Environment.ProcessPath!)!, "app.zip");
        if (File.Exists(beside))
        {
            ui.Report("使用同目录的 app.zip", 5);
            return beside;
        }

        var feed = string.IsNullOrWhiteSpace(opt.Feed) ? Program.DefaultFeed : opt.Feed!;
        ui.Report("正在连接更新源…", 2);

        using var handler = new HttpClientHandler
        {
            ServerCertificateCustomValidationCallback = (_, _, _, _) => true,
        };
        using var http = new HttpClient(handler) { Timeout = TimeSpan.FromMinutes(30) };

        var manifest = http.GetStringAsync(feed).GetAwaiter().GetResult();
        using var doc = JsonDocument.Parse(manifest);
        var root = doc.RootElement;
        var version = root.TryGetProperty("version", out var v) ? v.GetString() ?? "" : "";
        var url = root.GetProperty("url").GetString() ?? throw new InvalidDataException("清单里没有下载地址");
        var sha = root.TryGetProperty("sha256", out var s) ? s.GetString() ?? "" : "";

        ui.Report("最新版本 " + version, 3);

        if (!url.StartsWith("http", StringComparison.OrdinalIgnoreCase))
            url = new Uri(new Uri(feed), url.StartsWith("/") ? url : "/" + url).ToString();

        var temp = PickDownloadPath(installDir);
        using (var resp = http.GetAsync(url, HttpCompletionOption.ResponseHeadersRead).GetAwaiter().GetResult())
        {
            resp.EnsureSuccessStatusCode();
            var total = resp.Content.Headers.ContentLength ?? 0;
            using var src = resp.Content.ReadAsStream();
            using var dst = new FileStream(temp, FileMode.Create, FileAccess.Write, FileShare.None);
            var buf = new byte[81920];
            long read = 0;
            int n;
            while ((n = src.Read(buf, 0, buf.Length)) > 0)
            {
                dst.Write(buf, 0, n);
                read += n;
                if (total > 0)
                    ui.Report($"正在下载… {read / 1048576} / {total / 1048576} MB",
                        3 + (int)(43.0 * read / total));
            }
        }

        if (!string.IsNullOrWhiteSpace(sha))
        {
            ui.Report("正在校验文件…", 46);
            using var fs = File.OpenRead(temp);
            var actual = Convert.ToHexString(SHA256.HashData(fs)).ToLowerInvariant();
            if (!actual.Equals(sha.Trim().ToLowerInvariant(), StringComparison.Ordinal))
            {
                try { File.Delete(temp); } catch { }
                throw new InvalidDataException("安装包校验失败（sha256 不一致），请重试");
            }
        }
        return temp;
    }

    public static void KillRunning()
    {
        foreach (var name in new[] { "ZupuClient", "ZupuUpdater" })
            foreach (var p in Process.GetProcessesByName(name))
            {
                try { p.Kill(); p.WaitForExit(5000); } catch { }
            }
    }

    private static void WriteUninstallEntry(string dir)
    {
        try
        {
            var exe = Path.Combine(dir, Program.AppExeName);
            using var key = Registry.CurrentUser.CreateSubKey(
                @"Software\Microsoft\Windows\CurrentVersion\Uninstall\ZupuClient", writable: true);
            key.SetValue("DisplayName", "族谱客户端", RegistryValueKind.String);
            key.SetValue("DisplayVersion", "1.2.0.1113.6", RegistryValueKind.String);
            key.SetValue("DisplayIcon", exe, RegistryValueKind.String);
            key.SetValue("InstallLocation", dir, RegistryValueKind.String);
            key.SetValue("Publisher", "族谱系统", RegistryValueKind.String);
            key.SetValue("NoModify", 1, RegistryValueKind.DWord);
            key.SetValue("NoRepair", 1, RegistryValueKind.DWord);
            key.SetValue("UninstallString", "\"" + exe + "\"", RegistryValueKind.String);
        }
        catch { }
    }
}

internal static class Shortcuts
{
    /// <summary>先试 COM，失败回落系统 PowerShell</summary>
    public static bool TryCreate(string lnk, string target, string workDir)
    {
        try
        {
            Directory.CreateDirectory(Path.GetDirectoryName(lnk)!);
            var shellType = Type.GetTypeFromProgID("WScript.Shell")
                ?? throw new InvalidOperationException("缺少 WScript.Shell");
            var shell = Activator.CreateInstance(shellType)!;
            var sc = shellType.InvokeMember("CreateShortcut", BindingFlags.InvokeMethod, null, shell, new object[] { lnk })!;
            var t = sc.GetType();
            t.InvokeMember("TargetPath", BindingFlags.SetProperty, null, sc, new object[] { target });
            t.InvokeMember("WorkingDirectory", BindingFlags.SetProperty, null, sc, new object[] { workDir });
            t.InvokeMember("Description", BindingFlags.SetProperty, null, sc, new object[] { "族谱客户端" });
            t.InvokeMember("IconLocation", BindingFlags.SetProperty, null, sc, new object[] { target + ",0" });
            t.InvokeMember("Save", BindingFlags.InvokeMethod, null, sc, null);
            if (File.Exists(lnk)) return true;
        }
        catch { }

        try
        {
            var script =
                "$s=(New-Object -ComObject WScript.Shell).CreateShortcut('" + lnk.Replace("'", "''") + "');" +
                "$s.TargetPath='" + target.Replace("'", "''") + "';" +
                "$s.WorkingDirectory='" + workDir.Replace("'", "''") + "';" +
                "$s.Description='族谱客户端';" +
                "$s.IconLocation='" + target.Replace("'", "''") + ",0';" +
                "$s.Save()";
            var psi = new ProcessStartInfo
            {
                FileName = "powershell.exe",
                Arguments = "-NoProfile -NonInteractive -ExecutionPolicy Bypass -Command \"" + script.Replace("\"", "\\\"") + "\"",
                UseShellExecute = false,
                CreateNoWindow = true,
            };
            using var p = Process.Start(psi);
            p?.WaitForExit(15000);
            return File.Exists(lnk);
        }
        catch { return false; }
    }
}

internal interface IProgressSink
{
    void Report(string message, int percent);
}

internal sealed class Options
{
    public string? Dir;
    public string? Package;
    public string? Feed;
    public bool Autostart;
    public bool Quiet;
    public bool Help;
    public bool RunAfter;

    public static Options Parse(string[] args)
    {
        var o = new Options();
        for (var i = 0; i < args.Length; i++)
        {
            switch (args[i].ToLowerInvariant())
            {
                case "--dir" when i + 1 < args.Length: o.Dir = args[++i]; break;
                case "--package" when i + 1 < args.Length: o.Package = args[++i]; break;
                case "--feed" when i + 1 < args.Length: o.Feed = args[++i]; break;
                case "--autostart": o.Autostart = true; break;
                case "--quiet": o.Quiet = true; break;
                case "--run": o.RunAfter = true; break;
                case "-h": case "--help": o.Help = true; break;
            }
        }
        return o;
    }
}
