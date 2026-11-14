using System.Diagnostics;
using System.Net.Http;
using System.Drawing.Drawing2D;

namespace ZupuClient;

/// <summary>
/// 开机启动时的等待逻辑：如果「服务端开机自启」是开着的，客户端先等服务端能响应再进界面。
/// 只在由自启项拉起（--autostart-launch）时才等，手动双击不受影响。
/// </summary>
internal static class BootWait
{
    public static bool IsLogonLaunch(string[] args)
        => args.Any(a => a.Equals("--autostart-launch", StringComparison.OrdinalIgnoreCase));

    /// <summary>要探测的本地地址；连的是远程服务器时返回 null（没什么可等的）</summary>
    public static Uri? ProbeUrl()
    {
        var addr = Prefs.Current.Address;
        if (string.IsNullOrWhiteSpace(addr)) return new Uri("http://127.0.0.1:8080/");

        if (!Uri.TryCreate(addr, UriKind.Absolute, out var u)) return new Uri("http://127.0.0.1:8080/");
        var local = u.Host.Equals("127.0.0.1", StringComparison.OrdinalIgnoreCase)
                 || u.Host.Equals("localhost", StringComparison.OrdinalIgnoreCase)
                 || u.Host.Equals("::1", StringComparison.OrdinalIgnoreCase);
        return local ? u : null;
    }

    /// <summary>开机且服务端自启时，等服务端就绪（最多 90 秒）</summary>
    public static void RunIfNeeded(string[] args, Action<string>? log = null)
    {
        if (!IsLogonLaunch(args)) return;
        if (!Autostart.ServerEnabled)
        {
            log?.Invoke("boot wait: 服务端未设置开机自启，跳过等待");
            return;
        }

        var url = ProbeUrl();
        if (url is null)
        {
            log?.Invoke("boot wait: 连接的是远程服务器，跳过等待");
            return;
        }

        if (BootWaitForm.ProbeOnce(url, 900))
        {
            log?.Invoke("boot wait: 服务端已就绪，无需等待");
            return;
        }

        log?.Invoke("boot wait: 等待服务端 " + url);
        using var form = new BootWaitForm(url);
        form.ShowDialog();
        log?.Invoke("boot wait: 结束，就绪=" + form.Ready);
    }
}

/// <summary>等待服务端启动的提示窗：显示已经等了多久 + 跳过按钮</summary>
internal sealed class BootWaitForm : Form
{
    private readonly Uri _url;
    private readonly System.Windows.Forms.Timer _timer = new() { Interval = 700 };
    private readonly Label _status = new();
    private readonly Label _hint = new();
    private readonly RoundedPanel _track = new() { Radius = 3, BorderColor = Color.Transparent };
    private readonly Panel _bar = new() { BackColor = Ds.Accent };
    private readonly Label _elapsed = new();
    private readonly Button _skip = new();
    private readonly Stopwatch _watch = Stopwatch.StartNew();
    private readonly TimeSpan _limit = TimeSpan.FromSeconds(90);
    private bool _triedStart;
    private bool _done;

    public bool Ready { get; private set; }

    public BootWaitForm(Uri url)
    {
        _url = url;
        FormBorderStyle = FormBorderStyle.None;
        StartPosition = FormStartPosition.CenterScreen;
        ShowInTaskbar = true;
        BackColor = Ds.Surface;
        Size = new Size(420, 176);
        DoubleBuffered = true;
        Padding = new Padding(1);
        Text = "族谱客户端";

        _status.Text = "正在等待本地服务端启动…";
        _status.Font = Ds.Ui(10.5f, FontStyle.Bold);
        _status.ForeColor = Ds.Text;
        _status.AutoSize = false;

        _hint.Text = "开机时服务端要先起来，通常需要 5～20 秒。";
        _hint.Font = Ds.Ui(8.5f);
        _hint.ForeColor = Ds.Muted;
        _hint.AutoSize = false;

        _elapsed.Font = Ds.Ui(8.5f);
        _elapsed.ForeColor = Ds.Muted;
        _elapsed.TextAlign = ContentAlignment.MiddleRight;
        _elapsed.AutoSize = false;

        _skip.Text = "跳过等待";
        _skip.Font = Ds.Ui(9f);
        _skip.FlatStyle = FlatStyle.Flat;
        _skip.BackColor = Color.White;
        _skip.ForeColor = Ds.Text;
        _skip.Cursor = Cursors.Hand;
        _skip.FlatAppearance.BorderSize = 1;
        _skip.FlatAppearance.BorderColor = Ds.Border;
        _skip.Click += (_, _) => Finish(false);

        Controls.Add(_status);
        Controls.Add(_hint);
        Controls.Add(_elapsed);
        Controls.Add(_track);
        Controls.Add(_skip);
        _track.Controls.Add(_bar);

        void Layout()
        {
            const int pad = 26;
            _status.SetBounds(pad, 26, ClientSize.Width - pad * 2, 24);
            _hint.SetBounds(pad, 52, ClientSize.Width - pad * 2, 20);
            _track.SetBounds(pad, 84, ClientSize.Width - pad * 2, 6);
            _bar.SetBounds(0, 0, 0, 6);
            _elapsed.SetBounds(pad, 96, ClientSize.Width - pad * 2 - 4, 20);
            _skip.SetBounds(ClientSize.Width - pad - 88, ClientSize.Height - 18 - 30, 88, 30);
        }
        Resize += (_, _) => Layout();
        Layout();

        _timer.Tick += (_, _) => Tick();
        Shown += (_, _) => _timer.Start();
        FormClosed += (_, _) => _timer.Stop();
    }

    private void Tick()
    {
        if (_done) return;

        if (ProbeOnce(_url, 1200))
        {
            _status.Text = "服务端已就绪，正在打开…";
            _bar.Width = _track.Width;
            Finish(true, delay: 320);
            return;
        }

        // 等了几秒还没起来，且进程都不在：主动拉一把（自启项可能被推迟执行）
        if (!_triedStart && _watch.Elapsed.TotalSeconds > 6)
        {
            _triedStart = true;
            try
            {
                if (!ServerInstaller.IsRunning() && ServerInstaller.IsInstalled)
                {
                    ServerInstaller.Start();
                    _hint.Text = "已尝试启动服务端，继续等待…";
                }
            }
            catch { /* 拉不起来就继续等/超时 */ }
        }

        var frac = Math.Min(1.0, _watch.Elapsed.TotalSeconds / _limit.TotalSeconds);
        _bar.Width = (int)(_track.Width * frac);
        _elapsed.Text = $"已等待 {_watch.Elapsed.TotalSeconds:0} 秒";

        if (_watch.Elapsed > _limit)
        {
            _status.Text = "服务端还没起来，先打开客户端";
            _hint.Text = "可以在 设置 → 服务端 里手动启动，或检查服务端是否已安装。";
            Finish(false, delay: 900);
        }
    }

    private void Finish(bool ready, int delay = 0)
    {
        if (_done) return;
        _done = true;
        Ready = ready;
        _timer.Stop();

        if (delay <= 0)
        {
            DialogResult = ready ? DialogResult.OK : DialogResult.Ignore;
            Close();
            return;
        }

        var t = new System.Windows.Forms.Timer { Interval = delay };
        t.Tick += (_, _) =>
        {
            t.Stop(); t.Dispose();
            DialogResult = ready ? DialogResult.OK : DialogResult.Ignore;
            Close();
        };
        t.Start();
    }

    /// <summary>探一次：能拿到任何 HTTP 响应就算服务端活着</summary>
    public static bool ProbeOnce(Uri url, int timeoutMs)
    {
        try
        {
            using var handler = new HttpClientHandler { ServerCertificateCustomValidationCallback = (_, _, _, _) => true };
            using var client = new HttpClient(handler) { Timeout = TimeSpan.FromMilliseconds(timeoutMs) };
            using var resp = client.GetAsync(url, HttpCompletionOption.ResponseHeadersRead).GetAwaiter().GetResult();
            return true;
        }
        catch { return false; }
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        base.OnPaint(e);
        using var pen = new Pen(Ds.Border, 1f);
        e.Graphics.DrawRectangle(pen, 0, 0, Width - 1, Height - 1);
    }
}
