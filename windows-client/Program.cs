using CefSharp;
using CefSharp.WinForms;

namespace ZupuClient;

internal static class Program
{
    private static string DataDir => AppPaths.DataDir;

    [STAThread]
    private static void Main(string[] args)
    {
        // 开机自启用 Run 注册表拉起时，当前目录是 C:\Windows\System32。
        // Chromium 会往当前目录写运行期文件（crashpad、临时文件等），写不进去就在 libcef 里
        // 直接断言崩溃（异常码 0x80000003），表现就是"开机自启了但窗口一闪没有"。
        // 所以第一件事：把工作目录钉到程序目录。
        try { Directory.SetCurrentDirectory(AppContext.BaseDirectory); } catch { /* 目录被占用也不阻塞启动 */ }

        // 只打开设置窗口（不启动网页内核，便于排查界面问题）
        if (args.Length > 0 && args[0] == "--settings")
        {
            ApplicationConfiguration.Initialize();
            Application.Run(new SettingsDialog());
            return;
        }

        // 静默开关开机自启：--autostart <client|server> <on|off>
        if (args.Length >= 3 && args[0] == "--autostart")
        {
            try
            {
                var which = args[1].ToLowerInvariant();
                var on = args[2].Equals("on", StringComparison.OrdinalIgnoreCase);
                if (which == "client") Autostart.SetClient(on);
                else Autostart.SetServer(on);
                Environment.Exit(0);
            }
            catch (Exception ex)
            {
                try
                {
                    File.AppendAllText(Path.Combine(DataDir, "install.log"),
                        DateTime.Now.ToString("HH:mm:ss") + "  autostart 失败：" + ex.Message + Environment.NewLine);
                }
                catch { }
                Environment.Exit(1);
            }
            return;
        }

        // 静默安装服务端（给脚本/运维用）：
        //   ZupuClient.exe --install-server [auto|<包地址或本地文件>] [目标目录]
        if (args.Length > 0 && args[0] == "--install-server")
        {
            Environment.Exit(RunInstallServer(args));
            return;
        }

        AppDomain.CurrentDomain.UnhandledException += (_, e) =>
            Log("UNHANDLED: " + e.ExceptionObject);
        Application.ThreadException += (_, e) => Log("THREAD: " + e.Exception);

        try
        {
            Log("step 1: ApplicationConfiguration.Initialize  cwd=" + Environment.CurrentDirectory);
            ApplicationConfiguration.Initialize();

            Log("step 2: Cef.Initialize");
            if (!InitCef())
            {
                Log("step 2 FAILED: Cef.Initialize returned false");
                MessageBox.Show(
                    "无法初始化内置浏览器内核（CefSharp）。\n\n" +
                    "请确认程序目录完整（libcef.dll、CefSharp.BrowserSubprocess.exe、locales 等），" +
                    "并已安装 Microsoft Visual C++ 2015-2022 运行库。",
                    "族谱客户端", MessageBoxButtons.OK, MessageBoxIcon.Error);
                return;
            }
            Log("step 2 OK");

            // 上次更新留下的暂存目录，启动时清掉
            try { ClientUpdater.CleanupStages(); } catch { /* 忽略 */ }

            // 开了服务端开机自启时，先把自启项刷新成当前命令行（补上 --autostart-launch）
            try { Autostart.Refresh(); } catch { /* 写不了注册表不影响使用 */ }

            // 开机启动且服务端也自启：等服务端能响应再进界面
            try { BootWait.RunIfNeeded(args, Log); } catch (Exception ex) { Log("boot wait 异常：" + ex.Message); }

            var hasSession = !string.IsNullOrWhiteSpace(Prefs.Current.Token)
                          && !string.IsNullOrWhiteSpace(Prefs.Current.Address);
            Log("step 3: session=" + hasSession);

            // 已登录过就直接进主窗，否则先登录
            if (hasSession)
                Application.Run(new MainForm());
            else
                Application.Run(new LoginForm());

            Log("step 4: run loop ended");
        }
        catch (Exception ex)
        {
            Log("FATAL: " + ex);
            throw;
        }
        finally
        {
            try { Cef.Shutdown(); } catch { /* 关闭阶段忽略 */ }
        }
    }

    /// <summary>静默安装/更新服务端；返回 0 成功，1 失败（详情写 install.log）</summary>
    private static int RunInstallServer(string[] args)
    {
        var logFile = Path.Combine(DataDir, "install.log");
        void Log(string msg)
        {
            try
            {
                Directory.CreateDirectory(DataDir);
                File.AppendAllText(logFile, DateTime.Now.ToString("HH:mm:ss") + "  " + msg + Environment.NewLine);
            }
            catch { }
        }

        try
        {
            // 路径里可能带空格且调用方没加引号，这里把剩余参数拼回目标目录
            var target = args.Length > 3
                ? string.Join(" ", args, 2, args.Length - 2)
                : (args.Length > 2 ? args[2] : ServerInstaller.InstallDir);
            var src = args.Length > 1 ? args[1] : "auto";
            string pkg;
            var version = "";

            if (!src.Equals("auto", StringComparison.OrdinalIgnoreCase))
            {
                if (src.StartsWith("http", StringComparison.OrdinalIgnoreCase))
                {
                    Log("下载 " + src);
                    pkg = ServerInstaller
                        .DownloadAsync(new UpdateManifest("", src, 0, "", "", ""), null)
                        .GetAwaiter().GetResult();
                }
                else
                {
                    if (!File.Exists(src)) throw new FileNotFoundException("找不到本地包", src);
                    pkg = src;
                }
            }
            else
            {
                Log("读取包清单：" + ServerInstaller.FeedUrl);
                var m = ServerInstaller.CheckLatestAsync().GetAwaiter().GetResult();
                Log($"最新 Windows 包：v{m.Version}  {m.SizeText}  {m.ReleaseAt}");
                pkg = ServerInstaller
                    .DownloadAsync(m, new Progress<int>(p =>
                    {
                        if (p > 0 && p % 25 == 0) Log($"下载 {p}%");
                    }))
                    .GetAwaiter().GetResult();
                version = m.Version;
            }

            Log("安装到 " + target);
            ServerInstaller.InstallFromPackage(pkg, target, Log, version);
            Log("安装完成：" + target);
            return 0;
        }
        catch (Exception ex)
        {
            Log("失败：" + ex);
            return 1;
        }
    }

    private static bool InitCef()
    {
        // 上次"清除缓存"改成 .old 的目录，启动时真正删掉
        try
        {
            var old = Path.Combine(DataDir, "cef-cache.old");
            if (Directory.Exists(old)) Directory.Delete(old, true);
        }
        catch { /* 删不掉不影响启动 */ }

        var settings = new CefSettings
        {
            CachePath = Path.Combine(DataDir, "cef-cache"),
            LogFile = Path.Combine(DataDir, "cef.log"),
            LogSeverity = LogSeverity.Warning,
            Locale = "zh-CN",
            // 自包含发布时子进程 exe 就在程序目录里
            BrowserSubprocessPath = Path.Combine(AppContext.BaseDirectory, "CefSharp.BrowserSubprocess.exe"),
        };

        // 精简版系统/虚拟机里 GPU 合成经常出问题，走软件渲染更稳
        settings.DisableGpuAcceleration();
        settings.CefCommandLineArgs.Add("disable-features", "HardwareMediaKeyHandling");

        Log("  subprocess = " + settings.BrowserSubprocessPath
            + " exists=" + File.Exists(settings.BrowserSubprocessPath));
        return Cef.Initialize(settings, performDependencyCheck: false);
    }

    private static void Log(string message)
    {
        try
        {
            Directory.CreateDirectory(DataDir);
            File.AppendAllText(
                Path.Combine(DataDir, "startup.log"),
                DateTime.Now.ToString("HH:mm:ss.fff") + "  " + message + Environment.NewLine);
        }
        catch { /* 日志失败不影响主流程 */ }
    }
}
