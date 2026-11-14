using System.Diagnostics;
using System.Drawing.Drawing2D;
using System.IO.Compression;
using System.Runtime.InteropServices;
using System.Text;
using Microsoft.Win32;

namespace ZupuSetup;

internal static class Program
{
    public const string AppExeName = "ZupuClient.exe";
    private const string Magic = "ZUPSPKG1";

    [STAThread]
    private static void Main(string[] args)
    {
        ApplicationConfiguration.Initialize();

        if (args.Any(a => a.Equals("--uninstall", StringComparison.OrdinalIgnoreCase)))
        {
            Uninstaller.Run(args);
            return;
        }

        var silent = args.Any(a => a.Equals("--silent", StringComparison.OrdinalIgnoreCase));
        var dir = ArgValue(args, "--dir");
        var desktop = args.Any(a => a.Equals("--desktop", StringComparison.OrdinalIgnoreCase));
        var autostart = args.Any(a => a.Equals("--autostart", StringComparison.OrdinalIgnoreCase));

        if (silent)
        {
            var target = string.IsNullOrWhiteSpace(dir) ? Installer.DefaultDir : dir!;
            try
            {
                Installer.Install(target, desktop, autostart, null);
                Environment.Exit(0);
            }
            catch (Exception ex)
            {
                File.AppendAllText(Path.Combine(Path.GetTempPath(), "zupu-setup-error.log"),
                    DateTime.Now + " " + ex + Environment.NewLine, Encoding.UTF8);
                Environment.Exit(1);
            }
            return;
        }

        Application.Run(new SetupForm(string.IsNullOrWhiteSpace(dir) ? Installer.DefaultDir : dir!));
    }

    private static string? ArgValue(string[] args, string key)
    {
        for (var i = 0; i < args.Length - 1; i++)
            if (args[i].Equals(key, StringComparison.OrdinalIgnoreCase)) return args[i + 1];
        return null;
    }

    /// <summary>
    /// 找到载荷（客户端 app 的 zip）：① 程序集里的嵌入资源 ② 安装包同目录的 zip ③ 从更新源下载。
    /// 返回值可能是临时文件或就地文件，调用方不要删别人的文件。
    /// </summary>
    public static string ResolvePayload(Action<string, int>? report)
    {
        // ① 嵌入资源
        using (var res = typeof(Program).Assembly.GetManifestResourceStream("payload.zip"))
        {
            if (res is not null)
            {
                report?.Invoke("正在解包安装文件…", 2);
                var temp = Path.Combine(Path.GetTempPath(), "zupu-setup-" + Guid.NewGuid().ToString("N") + ".zip");
                using var outFs = new FileStream(temp, FileMode.Create, FileAccess.Write, FileShare.None);
                res.CopyTo(outFs);
                return temp;
            }
        }

        // ② 同目录的 zip（离线版：把 update.zip 和安装包放在一起）
        var dir = Path.GetDirectoryName(Environment.ProcessPath!)!;
        foreach (var f in Directory.GetFiles(dir, "*.zip"))
        {
            if (LooksLikeAppPayload(f)) return f;
        }

        // ③ 在线下载
        report?.Invoke("正在从更新源下载…", 2);
        return DownloadPayload(report);
    }

    private static bool LooksLikeAppPayload(string zip)
    {
        try
        {
            using var z = ZipFile.OpenRead(zip);
            return z.Entries.Any(e => e.FullName.Equals(Program.AppExeName, StringComparison.OrdinalIgnoreCase));
        }
        catch { return false; }
    }

    public const string ClientFeed = "https://zupu-update.lijiacore.com:4440/Client/latest.json";

    /// <summary>从更新源下载客户端更新包（自签证书放行），并校验 sha256</summary>
    private static string DownloadPayload(Action<string, int>? report)
    {
        var handler = new System.Net.Http.HttpClientHandler
        {
            ServerCertificateCustomValidationCallback = (_, _, _, _) => true,
        };
        using var http = new System.Net.Http.HttpClient(handler) { Timeout = TimeSpan.FromMinutes(10) };

        var manifestJson = http.GetStringAsync(ClientFeed).GetAwaiter().GetResult();
        using var doc = System.Text.Json.JsonDocument.Parse(manifestJson);
        var root = doc.RootElement;
        var url = root.GetProperty("url").GetString() ?? throw new InvalidDataException("清单里没有下载地址");
        var sha = root.TryGetProperty("sha256", out var s) ? s.GetString() ?? "" : "";

        // 相对地址按站点根解析（/Client/... 或 Client/...）
        var baseUri = new Uri(ClientFeed);
        if (!url.StartsWith("http", StringComparison.OrdinalIgnoreCase))
            url = new Uri(baseUri, url.StartsWith("/") ? url : "/" + url).ToString();

        var temp = Path.Combine(Path.GetTempPath(), "zupu-setup-" + Guid.NewGuid().ToString("N") + ".zip");
        using (var resp = http.GetAsync(url, System.Net.Http.HttpCompletionOption.ResponseHeadersRead)
                               .GetAwaiter().GetResult())
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
                    report?.Invoke($"正在下载… {read / 1048576} / {total / 1048576} MB", 2 + (int)(45.0 * read / total));
            }
        }

        if (!string.IsNullOrWhiteSpace(sha))
        {
            using var fs = File.OpenRead(temp);
            var actual = Convert.ToHexString(System.Security.Cryptography.SHA256.HashData(fs)).ToLowerInvariant();
            if (!actual.Equals(sha.Trim().ToLowerInvariant(), StringComparison.Ordinal))
            {
                try { File.Delete(temp); } catch { }
                throw new InvalidDataException("下载的安装文件校验失败（sha256 不一致）");
            }
        }
        return temp;
    }

    private static byte[] ReadExactly(Stream s, int n)
    {
        var b = new byte[n];
        var read = 0;
        while (read < n)
        {
            var k = s.Read(b, read, n - read);
            if (k <= 0) throw new EndOfStreamException();
            read += k;
        }
        return b;
    }
}

internal static class Installer
{
    public static string DefaultDir => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Programs", "族谱客户端");

    public static string ExeIn(string dir) => Path.Combine(dir, Program.AppExeName);

    public static bool IsInstalled(string dir) => File.Exists(ExeIn(dir));

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

    /// <summary>默认目录不可写时的备选位置</summary>
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
        try
        {
            var exe = Environment.ProcessPath;
            if (!string.IsNullOrWhiteSpace(exe))
                list.Add(Path.Combine(Path.GetDirectoryName(exe!)!, "族谱客户端"));
        }
        catch { }

        return list;
    }

    /// <summary>安装：解压 + 快捷方式 + 卸载项 + 可选自启</summary>
    public static string Install(string dir, bool desktopShortcut, bool autostart, Action<string, int>? report)
    {
        dir = Path.GetFullPath(dir);
        report?.Invoke("正在准备…", 0);

        // 目录不可写就换一个（受限环境里可能只有安装包所在目录可写）
        if (!TryEnsureWritable(dir, out var why))
        {
            var tried = new List<string> { dir + "（" + why + "）" };
            string? found = null;
            foreach (var alt in FallbackDirs())
            {
                if (TryEnsureWritable(alt, out var w2)) { found = alt; break; }
                tried.Add(alt + "（" + w2 + "）");
            }
            if (found is null)
                throw new IOException("没有可写的安装目录，试过：\n" + string.Join("\n", tried));
            report?.Invoke("该位置不可写，改装到 " + found, 0);
            dir = found;
        }

        // 正在运行的旧版本要先关掉，否则文件占用
        KillRunning();

        var payload = Program.ResolvePayload(report);
        try
        {
            Directory.CreateDirectory(dir);
            report?.Invoke("正在解压文件…", 48);

            using (var zip = ZipFile.OpenRead(payload))
            {
                var total = zip.Entries.Count;
                var done = 0;
                foreach (var e in zip.Entries)
                {
                    var rel = e.FullName.Replace('/', Path.DirectorySeparatorChar);
                    var dst = Path.GetFullPath(Path.Combine(dir, rel));
                    if (!dst.StartsWith(dir, StringComparison.OrdinalIgnoreCase)) continue;

                    if (rel.EndsWith(Path.DirectorySeparatorChar)) Directory.CreateDirectory(dst);
                    else
                    {
                        Directory.CreateDirectory(Path.GetDirectoryName(dst)!);
                        e.ExtractToFile(dst, overwrite: true);
                    }
                    done++;
                    if (done % 20 == 0 || done == total)
                        report?.Invoke($"正在解压文件… {done}/{total}", 48 + (int)(44.0 * done / Math.Max(1, total)));
                }
            }

            report?.Invoke("正在创建快捷方式…", 70);
            var exe = ExeIn(dir);
            if (!File.Exists(exe)) throw new FileNotFoundException("安装包里没有 ZupuClient.exe", exe);

            report?.Invoke("正在写入卸载信息…", 78);
            WriteUninstallEntry(dir);

            // 卸载器 = 自己的一份拷贝
            try
            {
                var self = Environment.ProcessPath!;
                var uninst = Path.Combine(dir, "uninst.exe");
                if (!string.Equals(self, uninst, StringComparison.OrdinalIgnoreCase))
                    File.Copy(self, uninst, overwrite: true);
            }
            catch { /* 卸载器拷不进去不阻塞安装 */ }

            // 快捷方式：失败只记警告，不能让整个安装失败
            report?.Invoke("正在创建快捷方式…", 84);
            var warnings = new List<string>();
            if (desktopShortcut && !TryShortcut(DesktopLnk, exe, dir, "族谱客户端"))
                warnings.Add("桌面快捷方式没建成（" + DesktopLnk + "）");
            if (!TryShortcut(StartMenuLnk, exe, dir, "族谱客户端"))
                warnings.Add("开始菜单快捷方式没建成（" + StartMenuLnk + "）");
            foreach (var w in warnings)
            {
                try
                {
                    File.AppendAllText(Path.Combine(Path.GetTempPath(), "zupu-setup-warnings.log"),
                        DateTime.Now + " " + w + Environment.NewLine, Encoding.UTF8);
                }
                catch { }
            }

            if (autostart)
            {
                report?.Invoke("正在设置开机自启…", 88);
                try
                {
                    RunClient(exe, dir, "--autostart client on");
                    RegisterRunValue(dir);
                }
                catch { /* 自启失败不影响安装 */ }
            }

            report?.Invoke("安装完成", 100);
        }
        finally
        {
            // 只删我们自己下到临时目录的那份，别动用户放在旁边的 zip
            try
            {
                if (payload.StartsWith(Path.GetTempPath(), StringComparison.OrdinalIgnoreCase)) File.Delete(payload);
            }
            catch { }
        }

        return dir;   // 可能是兜底换过的目录
    }

    public static string DesktopLnk
    {
        get
        {
            // 允许覆盖（企业部署/自动化测试把快捷方式放到别处）
            var custom = Environment.GetEnvironmentVariable("ZUPU_SETUP_DESKTOP");
            var dir = string.IsNullOrWhiteSpace(custom)
                ? Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory)
                : custom!;
            return Path.Combine(dir, "族谱客户端.lnk");
        }
    }

    public static string StartMenuLnk => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.Programs), "族谱客户端.lnk");

    public static void KillRunning()
    {
        foreach (var name in new[] { "ZupuClient", "ZupuUpdater" })
        {
            foreach (var p in Process.GetProcessesByName(name))
            {
                try { p.Kill(); p.WaitForExit(5000); } catch { }
            }
        }
    }

    public static void RunClient(string exe, string dir, string arguments)
    {
        var psi = new ProcessStartInfo
        {
            FileName = exe,
            Arguments = arguments,
            WorkingDirectory = dir,
            UseShellExecute = false,
            CreateNoWindow = true,
        };
        using var p = Process.Start(psi);
        p?.WaitForExit(20000);
    }

    public static void RegisterRunValue(string dir)
    {
        try
        {
            using var key = Registry.CurrentUser.CreateSubKey(
                @"Software\Microsoft\Windows\CurrentVersion\Run", writable: true);
            key.SetValue("ZupuClient", "\"" + ExeIn(dir) + "\" --autostart-launch", RegistryValueKind.String);
        }
        catch { }
    }

    private static void WriteUninstallEntry(string dir)
    {
        try
        {
            using var key = Registry.CurrentUser.CreateSubKey(
                @"Software\Microsoft\Windows\CurrentVersion\Uninstall\ZupuClient", writable: true);
            key.SetValue("DisplayName", "族谱客户端", RegistryValueKind.String);
            key.SetValue("DisplayVersion", Version, RegistryValueKind.String);
            key.SetValue("DisplayIcon", ExeIn(dir), RegistryValueKind.String);
            key.SetValue("InstallLocation", dir, RegistryValueKind.String);
            key.SetValue("Publisher", "族谱系统", RegistryValueKind.String);
            key.SetValue("NoModify", 1, RegistryValueKind.DWord);
            key.SetValue("NoRepair", 1, RegistryValueKind.DWord);
            key.SetValue("UninstallString", "\"" + Path.Combine(dir, "uninst.exe") + "\" --uninstall",
                RegistryValueKind.String);
            key.SetValue("EstimatedSize", 520 * 1024, RegistryValueKind.DWord);
        }
        catch { }
    }

    public static string Version
    {
        get
        {
            try
            {
                var v = FileVersionInfo.GetVersionInfo(Environment.ProcessPath!).ProductVersion;
                return string.IsNullOrWhiteSpace(v) ? "1.0.0" : v;
            }
            catch { return "1.0.0"; }
        }
    }

    /// <summary>建快捷方式；失败只返回 false，不抛</summary>
    public static bool TryShortcut(string lnk, string target, string workDir, string desc)
    {
        try
        {
            CreateShortcut(lnk, target, workDir, desc);
            return true;
        }
        catch { return false; }
    }

    public static void CreateShortcut(string lnk, string target, string workDir, string desc)
    {
        Directory.CreateDirectory(Path.GetDirectoryName(lnk)!);
        var shellType = Type.GetTypeFromProgID("WScript.Shell")
            ?? throw new InvalidOperationException("系统缺少 WScript.Shell，无法创建快捷方式");
        dynamic shell = Activator.CreateInstance(shellType)!;
        try
        {
            dynamic sc = shell.CreateShortcut(lnk);
            sc.TargetPath = target;
            sc.WorkingDirectory = workDir;
            sc.Description = desc;
            sc.IconLocation = target + ",0";
            sc.Save();
            Marshal.FinalReleaseComObject(sc);
        }
        finally
        {
            Marshal.FinalReleaseComObject(shell);
        }
    }
}

/// <summary>卸载：删快捷方式、卸载项、程序目录；数据目录可选保留</summary>
internal static class Uninstaller
{
    public static void Run(string[] args)
    {
        var dir = Path.GetDirectoryName(Environment.ProcessPath!)!;
        var dataDir = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "ZupuClient");

        var answer = MessageBox.Show(
            "确定要卸载族谱客户端吗？\n\n" +
            "程序目录：" + dir + "\n" +
            "登录信息与缓存（" + dataDir + "）会保留，除非你手动删除。",
            "卸载族谱客户端", MessageBoxButtons.YesNo, MessageBoxIcon.Question,
            MessageBoxDefaultButton.Button2);
        if (answer != DialogResult.Yes) return;

        Installer.KillRunning();

        TryDelete(Installer.DesktopLnk);
        TryDelete(Installer.StartMenuLnk);
        try
        {
            using var key = Registry.CurrentUser.OpenSubKey(
                @"Software\Microsoft\Windows\CurrentVersion\Run", writable: true);
            key?.DeleteValue("ZupuClient", throwOnMissingValue: false);
            key?.DeleteValue("ZupuServer", throwOnMissingValue: false);
        }
        catch { }
        try
        {
            Registry.CurrentUser.DeleteSubKeyTree(
                @"Software\Microsoft\Windows\CurrentVersion\Uninstall\ZupuClient", throwOnMissingSubKey: false);
        }
        catch { }

        // 卸载器自己还在目录里跑，先把自己挪到临时目录再删
        var self = Environment.ProcessPath!;
        var tmp = Path.Combine(Path.GetTempPath(), "zupu-uninst-" + Guid.NewGuid().ToString("N") + ".exe");
        try { File.Copy(self, tmp, overwrite: true); } catch { tmp = null; }

        if (tmp is not null)
        {
            Process.Start(new ProcessStartInfo
            {
                FileName = "cmd.exe",
                Arguments = $"/c ping -n 2 127.0.0.1 >nul & rmdir /s /q \"{dir}\" & del /f /q \"{tmp}\"",
                CreateNoWindow = true,
                UseShellExecute = false,
            });
        }

        MessageBox.Show("已卸载。", "族谱客户端", MessageBoxButtons.OK, MessageBoxIcon.Information);
    }

    private static void TryDelete(string path)
    {
        try { if (File.Exists(path)) File.Delete(path); } catch { }
    }
}

// ==================== 安装界面 ====================

internal sealed class SetupForm : Form
{
    private static readonly Color Surface = Color.White;
    private static readonly Color TextColor = ColorTranslator.FromHtml("#17181A");
    private static readonly Color Muted = ColorTranslator.FromHtml("#6B7280");
    private static readonly Color Border = ColorTranslator.FromHtml("#E7E8EC");
    private static readonly Color Accent = ColorTranslator.FromHtml("#3B6FE0");

    private readonly TextBox _dir;
    private readonly CheckBox _desktop;
    private readonly CheckBox _autostart;
    private readonly CheckBox _run;
    private readonly Button _install;
    private readonly Button _cancel;
    private readonly ProgressBar _bar;
    private readonly Label _status;

    public SetupForm(string defaultDir)
    {
        Text = "族谱客户端 安装";
        FormBorderStyle = FormBorderStyle.FixedSingle;
        MaximizeBox = false;
        MinimizeBox = false;
        StartPosition = FormStartPosition.CenterScreen;
        BackColor = Surface;
        ClientSize = new Size(560, 372);
        Font = Ui(9f);

        var title = new Label
        {
            Text = "族谱客户端",
            Font = Ui(15f, FontStyle.Bold),
            ForeColor = TextColor,
            AutoSize = false,
        };
        var sub = new Label
        {
            Text = "安装到你这台电脑，装完可从桌面或开始菜单启动。",
            ForeColor = Muted,
            AutoSize = false,
        };

        var dirLabel = new Label { Text = "安装位置", ForeColor = TextColor, AutoSize = false };
        _dir = new TextBox
        {
            Text = defaultDir,
            BorderStyle = BorderStyle.FixedSingle,
            BackColor = ColorTranslator.FromHtml("#FBFBFC"),
        };
        var browse = new Button
        {
            Text = "浏览…",
            FlatStyle = FlatStyle.Flat,
            BackColor = Color.White,
            ForeColor = TextColor,
            Cursor = Cursors.Hand,
        };
        browse.FlatAppearance.BorderColor = Border;
        browse.Click += (_, _) =>
        {
            using var dlg = new FolderBrowserDialog { Description = "选择安装位置", ShowNewFolderButton = true };
            if (Directory.Exists(_dir.Text)) dlg.SelectedPath = _dir.Text;
            if (dlg.ShowDialog(this) == DialogResult.OK) _dir.Text = Path.Combine(dlg.SelectedPath, "族谱客户端");
        };

        _desktop = new CheckBox { Text = "创建桌面快捷方式", Checked = true, ForeColor = TextColor, AutoSize = true };
        _autostart = new CheckBox { Text = "开机时自动启动客户端", Checked = false, ForeColor = TextColor, AutoSize = true };
        _run = new CheckBox { Text = "安装完成后立即运行", Checked = true, ForeColor = TextColor, AutoSize = true };

        _status = new Label { Text = "", ForeColor = Muted, AutoSize = false };
        _bar = new ProgressBar { Minimum = 0, Maximum = 100, Style = ProgressBarStyle.Continuous, Visible = false };

        _install = new Button
        {
            Text = "安装",
            FlatStyle = FlatStyle.Flat,
            BackColor = Accent,
            ForeColor = Color.White,
            Cursor = Cursors.Hand,
        };
        _install.FlatAppearance.BorderSize = 0;
        _install.Click += async (_, _) => await DoInstall();

        _cancel = new Button
        {
            Text = "取消",
            FlatStyle = FlatStyle.Flat,
            BackColor = Color.White,
            ForeColor = TextColor,
            Cursor = Cursors.Hand,
        };
        _cancel.FlatAppearance.BorderColor = Border;
        _cancel.Click += (_, _) => Close();

        Controls.Add(title); Controls.Add(sub);
        Controls.Add(dirLabel); Controls.Add(_dir); Controls.Add(browse);
        Controls.Add(_desktop); Controls.Add(_autostart); Controls.Add(_run);
        Controls.Add(_bar); Controls.Add(_status);
        Controls.Add(_install); Controls.Add(_cancel);

        void Layout()
        {
            const int pad = 28;
            var w = ClientSize.Width - pad * 2;
            title.SetBounds(pad, 24, w, 32);
            sub.SetBounds(pad, 56, w, 22);
            dirLabel.SetBounds(pad, 96, w, 20);
            _dir.SetBounds(pad, 118, w - 92, 27);
            browse.SetBounds(pad + w - 84, 117, 84, 29);
            _desktop.SetBounds(pad, 164, w, 22);
            _autostart.SetBounds(pad, 190, w, 22);
            _run.SetBounds(pad, 216, w, 22);
            _status.SetBounds(pad, 254, w, 20);
            _bar.SetBounds(pad, 276, w, 6);
            _install.SetBounds(ClientSize.Width - pad - 104, ClientSize.Height - 26 - 34, 104, 34);
            _cancel.SetBounds(_install.Left - 12 - 88, _install.Top + 2, 88, 30);
        }
        Layout();
        Resize += (_, _) => Layout();
    }

    private async Task DoInstall()
    {
        var dir = _dir.Text.Trim();
        if (dir.Length == 0) { MessageBox.Show("请填写安装位置", "族谱客户端", MessageBoxButtons.OK, MessageBoxIcon.Warning); return; }

        SetBusy(true);
        var ok = false;
        var installedTo = dir;
        try
        {
            installedTo = await Task.Run(() => Installer.Install(dir, _desktop.Checked, _autostart.Checked,
                (msg, pct) => BeginInvoke(new Action(() =>
                {
                    _status.Text = msg;
                    if (pct >= 0) { _bar.Visible = true; _bar.Value = Math.Min(100, pct); }
                }))));
            ok = true;
        }
        catch (Exception ex)
        {
            _status.Text = "安装失败";
            MessageBox.Show("安装失败：\n\n" + ex.Message, "族谱客户端", MessageBoxButtons.OK, MessageBoxIcon.Error);
        }
        finally
        {
            SetBusy(false);
        }

        if (!ok) return;

        _status.Text = "安装完成";
        if (_run.Checked)
        {
            try
            {
                Process.Start(new ProcessStartInfo
                {
                    FileName = Installer.ExeIn(installedTo),
                    WorkingDirectory = installedTo,
                    UseShellExecute = true,
                });
            }
            catch { }
        }
        Close();
    }

    private void SetBusy(bool busy)
    {
        _install.Enabled = !busy;
        _cancel.Enabled = !busy;
        _dir.Enabled = !busy;
        _desktop.Enabled = !busy;
        _autostart.Enabled = !busy;
        _run.Enabled = !busy;
        if (!busy) return;
        _bar.Visible = true;
        _bar.Value = 0;
        _status.Text = "正在安装…";
    }

    private static Font Ui(float size, FontStyle style = FontStyle.Regular)
        => new("Microsoft YaHei UI", size, style, GraphicsUnit.Point);

    protected override void OnPaint(PaintEventArgs e)
    {
        base.OnPaint(e);
        e.Graphics.SmoothingMode = SmoothingMode.AntiAlias;
        using var pen = new Pen(Border, 1f);
        e.Graphics.DrawRectangle(pen, 0, 0, Width - 1, Height - 1);
    }
}
