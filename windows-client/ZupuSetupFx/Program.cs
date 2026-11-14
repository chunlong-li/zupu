// 族谱客户端安装程序（.NET Framework 4.x 版）
//
// 和 .NET 8 版界面一致，但依赖 Windows 自带的 .NET Framework 4.8，
// 因此不用打包运行时，exe 只有几十 KB。
//
// 编译（系统自带 csc，无需 SDK）：
//   csc /nologo /codepage:65001 /target:winexe /out:族谱客户端安装包.exe /win32icon:app.ico
//       /r:System.Windows.Forms.dll /r:System.Drawing.dll
//       /r:System.IO.Compression.dll /r:System.IO.Compression.FileSystem.dll
//       Program.cs

using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Diagnostics;
using System.Drawing;
using System.IO;
using System.IO.Compression;
using System.Net;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Text;
using System.Text.RegularExpressions;
using System.Threading;
using System.Windows.Forms;
using Microsoft.Win32;

namespace ZupuSetupFx
{
    internal static class Program
    {
        public const string Feed = "https://zupu-update.lijiacore.com:4440/Client/latest.json";
        public const string AppExeName = "ZupuClient.exe";

        public static string DefaultDir
        {
            get
            {
                return Path.Combine(
                    Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                    "Programs", "族谱客户端");
            }
        }

        public static string DesktopLnk
        {
            get
            {
                string custom = Environment.GetEnvironmentVariable("ZUPU_SETUP_DESKTOP");
                string dir = string.IsNullOrEmpty(custom)
                    ? Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory)
                    : custom;
                return Path.Combine(dir, "族谱客户端.lnk");
            }
        }

        public static string StartMenuLnk
        {
            get
            {
                return Path.Combine(
                    Environment.GetFolderPath(Environment.SpecialFolder.Programs), "族谱客户端.lnk");
            }
        }

        [STAThread]
        private static void Main(string[] args)
        {
            // .NET Framework 默认可能走 TLS1.0，更新源要 TLS1.2（3072）
            try
            {
                ServicePointManager.SecurityProtocol =
                    (SecurityProtocolType)3072 | (SecurityProtocolType)768;   // Tls12 | Tls11
            }
            catch { }
            ServicePointManager.ServerCertificateValidationCallback =
                new System.Net.Security.RemoteCertificateValidationCallback(delegate { return true; });

            bool silent = false;
            string dir = null;
            bool uninstall = false;
            for (int i = 0; i < args.Length; i++)
            {
                if (args[i] == "--quiet") silent = true;
                else if (args[i] == "--uninstall") uninstall = true;
                else if (args[i] == "--dir" && i + 1 < args.Length) dir = args[++i];
            }

            if (uninstall)
            {
                Uninstaller.Run(silent);
                return;
            }

            if (silent)
            {
                try
                {
                    Installer.Install(dir ?? DefaultDir, true, false, null);
                    Environment.Exit(0);
                }
                catch (Exception ex)
                {
                    Log("静默安装失败：" + ex);
                    Environment.Exit(1);
                }
                return;
            }

            Application.EnableVisualStyles();
            Application.SetCompatibleTextRenderingDefault(false);
            Application.Run(new SetupForm(dir ?? DefaultDir));
        }

        public static void Log(string message)
        {
            try
            {
                string exe = Assembly.GetExecutingAssembly().Location;
                File.AppendAllText(
                    Path.Combine(Path.GetDirectoryName(exe), "install-error.log"),
                    DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss ") + message + Environment.NewLine,
                    Encoding.UTF8);
            }
            catch { }
        }
    }

    internal static class Installer
    {
        public static string ExeIn(string dir)
        {
            return Path.Combine(dir, Program.AppExeName);
        }

        public static bool TryEnsureWritable(string dir, out string why)
        {
            why = "";
            try
            {
                Directory.CreateDirectory(dir);
                string probe = Path.Combine(dir, "zupu-write-test.tmp");
                File.WriteAllText(probe, "x");
                File.Delete(probe);
                return true;
            }
            catch (Exception ex)
            {
                why = ex.GetType().Name + " " + ex.Message;
                return false;
            }
        }

        public static List<string> FallbackDirs()
        {
            List<string> list = new List<string>();
            string local = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
            string roaming = Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData);
            string profile = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
            list.Add(Path.Combine(local, "族谱客户端"));
            list.Add(Path.Combine(roaming, "族谱客户端"));
            list.Add(Path.Combine(profile, "族谱客户端"));
            try
            {
                string pub = Path.GetDirectoryName(
                    Environment.GetFolderPath(Environment.SpecialFolder.CommonDocuments));
                if (!string.IsNullOrEmpty(pub)) list.Add(Path.Combine(pub, "族谱客户端"));
            }
            catch { }
            try
            {
                string self = Path.GetDirectoryName(Assembly.GetExecutingAssembly().Location);
                if (!string.IsNullOrEmpty(self)) list.Add(Path.Combine(self, "族谱客户端"));
            }
            catch { }
            return list;
        }

        /// <summary>安装：下载 → 校验 → 解压 → 快捷方式 → 卸载项</summary>
        public static string Install(string dir, bool desktopShortcut, bool autostart,
            Action<string, int> report)
        {
            dir = Path.GetFullPath(dir);
            if (report != null) report("正在准备…", 0);

            string why;
            if (!TryEnsureWritable(dir, out why))
            {
                string found = null;
                List<string> tried = new List<string>();
                tried.Add(dir + "（" + why + "）");
                foreach (string alt in FallbackDirs())
                {
                    string w2;
                    if (TryEnsureWritable(alt, out w2)) { found = alt; break; }
                    tried.Add(alt + "（" + w2 + "）");
                }
                if (found == null)
                    throw new IOException("没有可写的安装目录，试过：\r\n" + string.Join("\r\n", tried.ToArray()));
                if (report != null) report("该位置不可写，改装到 " + found, 0);
                dir = found;
            }

            KillRunning();

            string package = Download(dir, report);

            if (report != null) report("正在解压文件…", 60);
            Directory.CreateDirectory(dir);
            using (ZipArchive zip = ZipFile.OpenRead(package))
            {
                int total = zip.Entries.Count;
                int done = 0;
                foreach (ZipArchiveEntry e in zip.Entries)
                {
                    string rel = e.FullName.Replace('/', Path.DirectorySeparatorChar);
                    string dst = Path.GetFullPath(Path.Combine(dir, rel));
                    if (!dst.StartsWith(dir, StringComparison.OrdinalIgnoreCase)) continue;

                    if (rel.EndsWith(Path.DirectorySeparatorChar.ToString()))
                    {
                        Directory.CreateDirectory(dst);
                    }
                    else
                    {
                        Directory.CreateDirectory(Path.GetDirectoryName(dst));
                        if (File.Exists(dst))
                        {
                            try { File.SetAttributes(dst, FileAttributes.Normal); } catch { }
                        }
                        e.ExtractToFile(dst, true);
                    }
                    done++;
                    if (report != null && (done % 20 == 0 || done == total))
                        report(string.Format("正在解压… {0}/{1}", done, total),
                            60 + (int)(30.0 * done / Math.Max(1, total)));
                }
            }

            try
            {
                if (Path.GetFileName(package).StartsWith("zupu-install-", StringComparison.OrdinalIgnoreCase))
                    File.Delete(package);
            }
            catch { }

            string exe = ExeIn(dir);
            if (!File.Exists(exe)) throw new InvalidDataException("安装包里没有 ZupuClient.exe");

            if (report != null) report("正在创建快捷方式…", 92);
            // 卸载器 = 安装包自己的一份拷贝（控制面板点「卸载」时运行它）
            try
            {
                string self = Assembly.GetExecutingAssembly().Location;
                string uninst = Path.Combine(dir, "uninst.exe");
                if (!string.Equals(self, uninst, StringComparison.OrdinalIgnoreCase))
                {
                    try { File.SetAttributes(uninst, FileAttributes.Normal); } catch { }
                    File.Copy(self, uninst, true);
                }
            }
            catch { }
            if (desktopShortcut) Shortcuts.TryCreate(Program.DesktopLnk, exe, dir);
            Shortcuts.TryCreate(Program.StartMenuLnk, exe, dir);
            WriteUninstallEntry(dir);

            if (autostart)
            {
                try
                {
                    RegistryKey key = Registry.CurrentUser.CreateSubKey(
                        @"Software\Microsoft\Windows\CurrentVersion\Run");
                    key.SetValue("ZupuClient", "\"" + exe + "\" --autostart-launch", RegistryValueKind.String);
                    key.Close();
                }
                catch { }
            }

            if (report != null) report("安装完成", 100);
            return dir;
        }

        private static string Download(string dir, Action<string, int> report)
        {
            string manifest = HttpGet(Program.Feed);
            string version = Grab(manifest, "version");
            string url = Grab(manifest, "url");
            string sha = Grab(manifest, "sha256");
            if (string.IsNullOrEmpty(url)) throw new InvalidDataException("清单里没有下载地址");
            if (url.StartsWith("/") == false && url.StartsWith("http") == false) url = "/" + url;
            if (url.StartsWith("/")) url = new Uri(new Uri(Program.Feed), url).ToString();

            string temp = Path.Combine(dir, "zupu-install-" + Guid.NewGuid().ToString("N") + ".zip");

            if (report != null) report("正在下载 " + version + " …", 2);
            using (WebClient wc = new WebClient())
            {
                wc.Headers.Add("User-Agent", "ZupuSetup/1.0");
                wc.DownloadProgressChanged += delegate (object s, DownloadProgressChangedEventArgs e)
                {
                    if (report != null)
                        report(string.Format("正在下载… {0} / {1} MB",
                            e.BytesReceived / 1048576, e.TotalBytesToReceive / 1048576),
                            2 + (int)(55.0 * e.BytesReceived / Math.Max(1, e.TotalBytesToReceive)));
                };
                wc.DownloadFile(url, temp);
            }

            if (!string.IsNullOrEmpty(sha))
            {
                if (report != null) report("正在校验…", 58);
                string actual = "";
                using (SHA256 sha2 = SHA256.Create())
                using (FileStream fs = File.OpenRead(temp))
                {
                    byte[] hash = sha2.ComputeHash(fs);
                    StringBuilder sb = new StringBuilder();
                    foreach (byte b in hash) sb.Append(b.ToString("x2"));
                    actual = sb.ToString();
                }
                if (actual != sha.Trim().ToLowerInvariant())
                {
                    try { File.Delete(temp); } catch { }
                    throw new InvalidDataException("安装包校验失败（sha256 不一致），请重试");
                }
            }
            return temp;
        }

        private static string HttpGet(string url)
        {
            using (WebClient wc = new WebClient())
            {
                wc.Headers.Add("User-Agent", "ZupuSetup/1.0");
                wc.Encoding = Encoding.UTF8;
                return wc.DownloadString(url);
            }
        }

        /// <summary>从 JSON 里取一个字符串字段（够用，不引入序列化库）</summary>
        private static string Grab(string json, string key)
        {
            Match m = Regex.Match(json, "\"" + key + "\"\\s*:\\s*\"([^\"]*)\"");
            return m.Success ? m.Groups[1].Value : "";
        }

        public static void KillRunning()
        {
            foreach (string name in new string[] { "ZupuClient", "ZupuUpdater" })
            {
                foreach (Process p in Process.GetProcessesByName(name))
                {
                    try { p.Kill(); p.WaitForExit(5000); } catch { }
                }
            }
        }

        private static void WriteUninstallEntry(string dir)
        {
            try
            {
                string exe = ExeIn(dir);
                string uninst = Path.Combine(dir, "uninst.exe");
                RegistryKey key = Registry.CurrentUser.CreateSubKey(
                    @"Software\Microsoft\Windows\CurrentVersion\Uninstall\ZupuClient");
                key.SetValue("DisplayName", "族谱客户端", RegistryValueKind.String);
                key.SetValue("DisplayVersion", "1.2.0.1113.6", RegistryValueKind.String);
                key.SetValue("DisplayIcon", exe, RegistryValueKind.String);
                key.SetValue("InstallLocation", dir, RegistryValueKind.String);
                key.SetValue("Publisher", "族谱系统", RegistryValueKind.String);
                key.SetValue("NoModify", 1, RegistryValueKind.DWord);
                key.SetValue("NoRepair", 1, RegistryValueKind.DWord);
                // 指向卸载器（不是客户端本体！）
                key.SetValue("UninstallString", "\"" + uninst + "\" --uninstall", RegistryValueKind.String);
                key.SetValue("QuietUninstallString", "\"" + uninst + "\" --uninstall --quiet", RegistryValueKind.String);
                key.Close();
            }
            catch { }
        }
    }

    /// <summary>卸载：删快捷方式、开机自启、控制面板条目，再删程序目录（保留数据目录）</summary>
    internal static class Uninstaller
    {
        public static void Run(bool silent)
        {
            string exePath = Assembly.GetExecutingAssembly().Location;
            string dir = Path.GetDirectoryName(exePath);
            string dataDir = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "ZupuClient");

            if (!silent)
            {
                DialogResult r = MessageBox.Show(
                    "确定要卸载族谱客户端吗？\r\n\r\n" +
                    "程序目录：" + dir + "\r\n\r\n" +
                    "登录信息与缓存（" + dataDir + "）会保留，如需清除请手动删除。",
                    "卸载族谱客户端", MessageBoxButtons.YesNo, MessageBoxIcon.Question,
                    MessageBoxDefaultButton.Button2);
                if (r != DialogResult.Yes) return;
            }

            Installer.KillRunning();

            TryDelete(Program.DesktopLnk);
            TryDelete(Program.StartMenuLnk);

            try
            {
                RegistryKey k = Registry.CurrentUser.OpenSubKey(
                    @"Software\Microsoft\Windows\CurrentVersion\Run", true);
                if (k != null)
                {
                    k.DeleteValue("ZupuClient", false);
                    k.DeleteValue("ZupuServer", false);
                    k.Close();
                }
            }
            catch { }
            try
            {
                Registry.CurrentUser.DeleteSubKeyTree(
                    @"Software\Microsoft\Windows\CurrentVersion\Uninstall\ZupuClient", false);
            }
            catch { }

            // 自己还在目录里跑，交给 cmd 等本进程退出后删
            try
            {
                Process.Start(new ProcessStartInfo
                {
                    FileName = "cmd.exe",
                    Arguments = "/c ping -n 3 127.0.0.1 >nul & rmdir /s /q \"" + dir + "\"",
                    UseShellExecute = false,
                    CreateNoWindow = true
                });
            }
            catch { }

            if (!silent)
                MessageBox.Show("已卸载。\r\n\r\n程序目录会被删除（数据目录保留）。",
                    "族谱客户端", MessageBoxButtons.OK, MessageBoxIcon.Information);
        }

        private static void TryDelete(string path)
        {
            try { if (File.Exists(path)) File.Delete(path); } catch { }
        }
    }

    internal static class Shortcuts
    {
        public static bool TryCreate(string lnk, string target, string workDir)
        {
            try
            {
                Directory.CreateDirectory(Path.GetDirectoryName(lnk));
                Type shellType = Type.GetTypeFromProgID("WScript.Shell");
                if (shellType == null) return false;
                object shell = Activator.CreateInstance(shellType);
                object sc = shellType.InvokeMember("CreateShortcut", BindingFlags.InvokeMethod, null,
                    shell, new object[] { lnk });
                Type scType = sc.GetType();
                scType.InvokeMember("TargetPath", BindingFlags.SetProperty, null, sc, new object[] { target });
                scType.InvokeMember("WorkingDirectory", BindingFlags.SetProperty, null, sc, new object[] { workDir });
                scType.InvokeMember("Description", BindingFlags.SetProperty, null, sc, new object[] { "族谱客户端" });
                scType.InvokeMember("IconLocation", BindingFlags.SetProperty, null, sc, new object[] { target + ",0" });
                scType.InvokeMember("Save", BindingFlags.InvokeMethod, null, sc, null);
                return File.Exists(lnk);
            }
            catch { return false; }
        }
    }

    /// <summary>安装窗口</summary>
    internal sealed class SetupForm : Form
    {
        private readonly TextBox _dir;
        private readonly CheckBox _desktop;
        private readonly CheckBox _autostart;
        private readonly CheckBox _run;
        private readonly Button _install;
        private readonly Button _cancel;
        private readonly ProgressBar _bar;
        private readonly Label _status;
        private volatile bool _busy;

        public SetupForm(string defaultDir)
        {
            Text = "族谱客户端 安装";
            FormBorderStyle = FormBorderStyle.FixedSingle;
            MaximizeBox = false;
            MinimizeBox = false;
            StartPosition = FormStartPosition.CenterScreen;
            BackColor = Color.White;
            ClientSize = new Size(560, 372);
            Font = new Font("Microsoft YaHei UI", 9f);

            Label title = new Label();
            title.Text = "族谱客户端";
            title.Font = new Font("Microsoft YaHei UI", 15f, FontStyle.Bold);
            title.ForeColor = Color.FromArgb(23, 24, 26);
            title.AutoSize = false;

            Label sub = new Label();
            sub.Text = "安装到你这台电脑，装完可从桌面或开始菜单启动。";
            sub.ForeColor = Color.FromArgb(107, 114, 128);
            sub.AutoSize = false;

            Label dirLabel = new Label();
            dirLabel.Text = "安装位置";
            dirLabel.AutoSize = false;

            _dir = new TextBox();
            _dir.Text = defaultDir;
            _dir.BorderStyle = BorderStyle.FixedSingle;

            Button browse = new Button();
            browse.Text = "浏览…";
            browse.FlatStyle = FlatStyle.Flat;
            browse.BackColor = Color.White;
            browse.Click += delegate
            {
                FolderBrowserDialog dlg = new FolderBrowserDialog();
                dlg.Description = "选择安装位置";
                if (Directory.Exists(_dir.Text)) dlg.SelectedPath = _dir.Text;
                if (dlg.ShowDialog(this) == DialogResult.OK)
                    _dir.Text = Path.Combine(dlg.SelectedPath, "族谱客户端");
            };

            _desktop = new CheckBox();
            _desktop.Text = "创建桌面快捷方式";
            _desktop.Checked = true;
            _desktop.AutoSize = true;

            _autostart = new CheckBox();
            _autostart.Text = "开机时自动启动客户端";
            _autostart.AutoSize = true;

            _run = new CheckBox();
            _run.Text = "安装完成后立即运行";
            _run.Checked = true;
            _run.AutoSize = true;

            _status = new Label();
            _status.Text = "";
            _status.ForeColor = Color.FromArgb(107, 114, 128);
            _status.AutoSize = false;

            _bar = new ProgressBar();
            _bar.Minimum = 0;
            _bar.Maximum = 100;
            _bar.Visible = false;

            _install = new Button();
            _install.Text = "安装";
            _install.FlatStyle = FlatStyle.Flat;
            _install.BackColor = Color.FromArgb(59, 111, 224);
            _install.ForeColor = Color.White;
            _install.FlatAppearance.BorderSize = 0;
            _install.Click += delegate { DoInstall(); };

            _cancel = new Button();
            _cancel.Text = "取消";
            _cancel.FlatStyle = FlatStyle.Flat;
            _cancel.BackColor = Color.White;
            _cancel.Click += delegate { Close(); };

            Controls.Add(title); Controls.Add(sub);
            Controls.Add(dirLabel); Controls.Add(_dir); Controls.Add(browse);
            Controls.Add(_desktop); Controls.Add(_autostart); Controls.Add(_run);
            Controls.Add(_bar); Controls.Add(_status);
            Controls.Add(_install); Controls.Add(_cancel);

            const int pad = 28;
            int w = ClientSize.Width - pad * 2;
            title.SetBounds(pad, 24, w, 32);
            sub.SetBounds(pad, 56, w, 22);
            dirLabel.SetBounds(pad, 96, w, 20);
            _dir.SetBounds(pad, 118, w - 92, 27);
            browse.SetBounds(pad + w - 84, 117, 84, 29);
            _desktop.SetBounds(pad, 164, w, 22);
            _autostart.SetBounds(pad, 190, w, 22);
            _run.SetBounds(pad, 216, w, 22);
            _status.SetBounds(pad, 252, w, 20);
            _bar.SetBounds(pad, 274, w, 6);
            _install.SetBounds(ClientSize.Width - pad - 104, ClientSize.Height - 26 - 34, 104, 34);
            _cancel.SetBounds(_install.Left - 12 - 88, _install.Top + 2, 88, 30);

            AcceptButton = _install;
            CancelButton = _cancel;
        }

        private void SetBusy(bool busy)
        {
            _busy = busy;
            _install.Enabled = !busy;
            _cancel.Enabled = !busy;
            _dir.Enabled = !busy;
            _desktop.Enabled = !busy;
            _autostart.Enabled = !busy;
            _run.Enabled = !busy;
            _bar.Visible = true;
            if (busy) { _bar.Value = 0; _status.Text = "正在安装…"; }
        }

        private void Report(string message, int percent)
        {
            if (InvokeRequired)
            {
                BeginInvoke((MethodInvoker)delegate { Report(message, percent); });
                return;
            }
            _status.Text = message;
            if (percent >= 0)
            {
                _bar.Visible = true;
                _bar.Value = Math.Min(100, Math.Max(0, percent));
            }
        }

        private void DoInstall()
        {
            string dir = _dir.Text.Trim();
            if (dir.Length == 0)
            {
                MessageBox.Show("请填写安装位置", "族谱客户端", MessageBoxButtons.OK, MessageBoxIcon.Warning);
                return;
            }

            SetBusy(true);
            string installedTo = dir;
            Exception failure = null;

            Thread t = new Thread(delegate ()
            {
                try
                {
                    installedTo = Installer.Install(dir, _desktop.Checked, _autostart.Checked, Report);
                }
                catch (Exception ex)
                {
                    failure = ex;
                    Program.Log("安装失败：" + ex);
                }
                try
                {
                    BeginInvoke((MethodInvoker)delegate { Finish(failure, installedTo); });
                }
                catch { }
            });
            t.IsBackground = true;
            t.Start();
        }

        private void Finish(Exception failure, string installedTo)
        {
            SetBusy(false);
            if (failure != null)
            {
                _status.Text = "安装失败";
                MessageBox.Show("安装失败：\r\n\r\n" + failure.Message, "族谱客户端",
                    MessageBoxButtons.OK, MessageBoxIcon.Error);
                _bar.Visible = false;
                return;
            }

            _status.Text = "安装完成";
            _bar.Value = 100;
            if (_run.Checked)
            {
                try
                {
                    Process.Start(new ProcessStartInfo
                    {
                        FileName = Installer.ExeIn(installedTo),
                        WorkingDirectory = installedTo,
                        UseShellExecute = true
                    });
                }
                catch { }
            }
            Close();
        }

        protected override void OnFormClosing(FormClosingEventArgs e)
        {
            if (_busy)
            {
                DialogResult r = MessageBox.Show("正在安装，确定要退出吗？", "族谱客户端",
                    MessageBoxButtons.YesNo, MessageBoxIcon.Question);
                if (r != DialogResult.Yes) e.Cancel = true;
            }
            base.OnFormClosing(e);
        }
    }
}
