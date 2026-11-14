using System.Drawing.Drawing2D;
using System.Runtime.InteropServices;
using CefSharp;

namespace ZupuClient;

/// <summary>设置弹窗的配色与字体（对齐 DSH 的设置界面：浅灰导航 + 白色内容 + 蓝色强调）</summary>
internal static class Ds
{
    public static readonly Color Surface = ColorTranslator.FromHtml("#FFFFFF");
    public static readonly Color NavBg = ColorTranslator.FromHtml("#FAFAFB");
    public static readonly Color Border = ColorTranslator.FromHtml("#E7E8EC");
    public static readonly Color Divider = ColorTranslator.FromHtml("#F1F1F4");
    public static readonly Color Text = ColorTranslator.FromHtml("#17181A");
    public static readonly Color Muted = ColorTranslator.FromHtml("#6B7280");
    public static readonly Color Accent = ColorTranslator.FromHtml("#3B6FE0");
    public static readonly Color NavSel = ColorTranslator.FromHtml("#EDEFF3");
    public static readonly Color Field = ColorTranslator.FromHtml("#FBFBFC");

    public static Font Ui(float size, FontStyle style = FontStyle.Regular)
        => new("Microsoft YaHei UI", size, style, GraphicsUnit.Point);

    public static GraphicsPath Round(RectangleF r, float radius)
    {
        var p = new GraphicsPath();
        var d = radius * 2;
        p.AddArc(r.X, r.Y, d, d, 180, 90);
        p.AddArc(r.Right - d, r.Y, d, d, 270, 90);
        p.AddArc(r.Right - d, r.Bottom - d, d, d, 0, 90);
        p.AddArc(r.X, r.Bottom - d, d, d, 90, 90);
        p.CloseFigure();
        return p;
    }
}

/// <summary>圆角白卡片</summary>
internal sealed class RoundedPanel : Panel
{
    public int Radius { get; set; } = 10;
    public Color BorderColor { get; set; } = Ds.Border;

    public RoundedPanel()
    {
        SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.SupportsTransparentBackColor
               | ControlStyles.UserPaint | ControlStyles.ResizeRedraw, true);
        BackColor = Color.White;
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        e.Graphics.SmoothingMode = SmoothingMode.AntiAlias;
        using var path = Ds.Round(new RectangleF(0.5f, 0.5f, Width - 1.5f, Height - 1.5f), Radius);
        using var bg = new SolidBrush(BackColor);
        e.Graphics.FillPath(bg, path);
        using var pen = new Pen(BorderColor, 1f);
        e.Graphics.DrawPath(pen, path);
    }
}

/// <summary>开关（DSH 设置里的开关样式）</summary>
internal sealed class ToggleSwitch : Control
{
    private bool _on;

    public event EventHandler? CheckedChanged;

    public bool Checked
    {
        get => _on;
        set
        {
            if (_on == value) return;
            _on = value;
            Invalidate();
            CheckedChanged?.Invoke(this, EventArgs.Empty);
        }
    }

    public ToggleSwitch()
    {
        SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.SupportsTransparentBackColor
               | ControlStyles.UserPaint | ControlStyles.ResizeRedraw, true);
        Size = new Size(40, 22);
        Cursor = Cursors.Hand;
        BackColor = Color.Transparent;
    }

    protected override void OnClick(EventArgs e) { Checked = !Checked; base.OnClick(e); }

    protected override void OnPaint(PaintEventArgs e)
    {
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        var track = new RectangleF(0, (Height - 22) / 2f, 40, 22);
        using var path = Ds.Round(track, 11);
        using var fill = new SolidBrush(_on ? Ds.Accent : ColorTranslator.FromHtml("#D3D6DC"));
        g.FillPath(fill, path);
        var knobX = _on ? track.Right - 19 : track.X + 3;
        using var knob = new SolidBrush(Color.White);
        g.FillEllipse(knob, knobX, track.Y + 3, 16, 16);
    }
}

/// <summary>设置项导航行（左侧分区）</summary>
internal sealed class SettingsNavRow : Control
{
    private bool _hot;
    public bool Active { get; set; }
    public string IconKey { get; set; } = "";

    public SettingsNavRow()
    {
        SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.SupportsTransparentBackColor
               | ControlStyles.UserPaint | ControlStyles.ResizeRedraw, true);
        Height = 34;
        Cursor = Cursors.Hand;
        BackColor = Color.Transparent;
    }

    protected override void OnMouseEnter(EventArgs e) { _hot = true; Invalidate(); base.OnMouseEnter(e); }
    protected override void OnMouseLeave(EventArgs e) { _hot = false; Invalidate(); base.OnMouseLeave(e); }

    protected override void OnPaint(PaintEventArgs e)
    {
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        g.TextRenderingHint = System.Drawing.Text.TextRenderingHint.ClearTypeGridFit;

        if (Active || _hot)
        {
            using var path = Ds.Round(new RectangleF(0, 0, Width - 1, Height - 1), 8);
            using var b = new SolidBrush(Active ? Ds.NavSel : Color.FromArgb(120, 237, 239, 243));
            g.FillPath(b, path);
        }

        var x = 10f;
        if (ColorIcons.Has(IconKey))
        {
            ColorIcons.Draw(g, IconKey, new RectangleF(x, (Height - 16) / 2f, 16, 16));
            x += 16 + 8;
        }

        using var font = Ds.Ui(9.5f, Active ? FontStyle.Bold : FontStyle.Regular);
        using var brush = new SolidBrush(Active ? Ds.Text : ColorTranslator.FromHtml("#4B5563"));
        using var sf = new StringFormat { LineAlignment = StringAlignment.Center, Trimming = StringTrimming.EllipsisCharacter };
        g.DrawString(Text, font, brush, new RectangleF(x, 0, Width - x - 8, Height), sf);
    }
}

/// <summary>
/// 客户端设置弹窗。布局对齐 DSH 的设置：顶部标题条 + 左侧分区导航 + 右侧设置项。
/// </summary>
internal sealed class SettingsDialog : Form
{
    private readonly Panel _header = new() { BackColor = Ds.Surface };
    private readonly Panel _nav = new() { BackColor = Ds.NavBg, Padding = new Padding(10, 10, 10, 10) };
    private readonly Panel _content = new() { BackColor = Ds.Surface, AutoScroll = true, Padding = new Padding(26, 18, 26, 24) };
    private readonly List<SettingsNavRow> _rows = new();
    private readonly List<Func<Control>> _builders = new();
    private readonly CloseButton _close = new();
    private int _active = -1;

    public SettingsDialog()
    {
        Text = "设置";
        FormBorderStyle = FormBorderStyle.None;
        StartPosition = FormStartPosition.CenterParent;
        ShowInTaskbar = false;
        BackColor = Ds.Surface;
        Size = new Size(880, 620);
        MinimumSize = new Size(720, 480);
        DoubleBuffered = true;
        Padding = new Padding(1);

        var title = new Label
        {
            Text = "设置",
            Font = Ds.Ui(11.5f, FontStyle.Bold),
            ForeColor = Ds.Text,
            AutoSize = false,
            TextAlign = ContentAlignment.MiddleLeft,
        };
        var border = new Panel { BackColor = Ds.Border, Height = 1 };
        _header.Controls.Add(title);
        _header.Controls.Add(_close);
        _header.Controls.Add(border);
        _close.Click += (_, _) => Close();
        EnableDrag(_header);
        EnableDrag(title);

        Controls.Add(_content);
        Controls.Add(_nav);
        Controls.Add(_header);

        void LayoutShell()
        {
            const int headerH = 50, navW = 196;
            _header.SetBounds(0, 0, ClientSize.Width, headerH);
            title.SetBounds(20, 0, 260, headerH);
            _close.SetBounds(_header.Width - 12 - _close.Width, (headerH - _close.Height) / 2, _close.Width, _close.Height);
            border.SetBounds(0, headerH - 1, _header.Width, 1);
            _nav.SetBounds(0, headerH, navW, ClientSize.Height - headerH);
            _content.SetBounds(navW, headerH, ClientSize.Width - navW, ClientSize.Height - headerH);
        }
        Resize += (_, _) => LayoutShell();

        BuildNav();
        LayoutShell();
        Select(0);

        KeyPreview = true;
        KeyDown += (_, e) => { if (e.KeyCode == Keys.Escape) Close(); };
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        base.OnPaint(e);
        e.Graphics.SmoothingMode = SmoothingMode.AntiAlias;
        using var pen = new Pen(Ds.Border, 1f);
        e.Graphics.DrawRectangle(pen, 0, 0, Width - 1, Height - 1);
    }

    private void BuildNav()
    {
        var entries = new (string key, string text, Func<Control> build)[]
        {
            (ClientSettings.Prefix + "datadir", "数据目录", BuildDataDir),
            (ClientSettings.Prefix + "update", "客户端更新", BuildClientUpdate),
            (ClientSettings.Prefix + "server", "服务端", BuildServer),
            (ClientSettings.Prefix + "startup", "开机自启", BuildStartup),
            (ClientSettings.Prefix + "close", "关闭行为", BuildClose),
            (ClientSettings.Prefix + "cache", "缓存与日志", BuildCache),
            (ClientSettings.Prefix + "about", "关于", BuildAbout),
        };

        var top = 4;
        for (var i = 0; i < entries.Length; i++)
        {
            var index = i;
            var row = new SettingsNavRow
            {
                Text = entries[i].text,
                IconKey = entries[i].key,
                Left = 10,
                Top = top,
                Width = _nav.ClientSize.Width - 20,
                Anchor = AnchorStyles.Top | AnchorStyles.Left | AnchorStyles.Right,
            };
            row.Click += (_, _) => Select(index);
            _nav.Controls.Add(row);
            _rows.Add(row);
            _builders.Add(entries[i].build);
            top += row.Height + 2;
        }
        _nav.Resize += (_, _) =>
        {
            foreach (var r in _rows) r.Width = _nav.ClientSize.Width - 20;
        };
    }

    private void Select(int index)
    {
        if (index < 0 || index >= _builders.Count) return;
        _active = index;
        for (var i = 0; i < _rows.Count; i++)
        {
            _rows[i].Active = i == index;
            _rows[i].Invalidate();
        }
        _content.SuspendLayout();
        foreach (Control c in _content.Controls) c.Dispose();
        _content.Controls.Clear();
        var page = _builders[index]();
        page.Dock = DockStyle.Top;
        _content.Controls.Add(page);

        // 按内容算高度：避免容器按子控件撑宽后冒出横向滚动条
        var height = page.Padding.Vertical;
        foreach (Control c in page.Controls) height += c.Height + c.Margin.Vertical;
        page.Height = Math.Max(10, height + 10);

        _content.ResumeLayout();
        _content.AutoScrollPosition = new Point(0, 0);
    }

    // ==================== 通用构件 ====================

    private static FlowLayoutPanel Page()
    {
        // 宽度跟随父容器、高度由 Select() 按内容算；滚动交给外层 _content
        var flow = new FlowLayoutPanel
        {
            Dock = DockStyle.Top,
            AutoSize = false,
            FlowDirection = FlowDirection.TopDown,
            WrapContents = false,
            AutoScroll = false,
            BackColor = Ds.Surface,
            Padding = Padding.Empty,
        };
        flow.Resize += (_, _) =>
        {
            var w = Math.Max(280, flow.ClientSize.Width - 4);
            foreach (Control c in flow.Controls) c.Width = w;
        };
        return flow;
    }

    private static Label H(string text, float size, Color color, FontStyle style, int height) => new()
    {
        Text = text,
        AutoSize = false,
        Height = height,
        Margin = new Padding(0, 0, 0, 0),
        Font = Ds.Ui(size, style),
        ForeColor = color,
    };

    private static Control SectionTitle(string title, string? subtitle)
    {
        var panel = new Panel
        {
            Height = string.IsNullOrEmpty(subtitle) ? 38 : 60,
            BackColor = Ds.Surface,
            Margin = new Padding(0, 0, 0, 0),
        };
        var t = new Label
        {
            Text = title,
            Font = Ds.Ui(12f, FontStyle.Bold),
            ForeColor = Ds.Text,
            AutoSize = false,
            Height = 26,
            Left = 0,
            Top = 0,
        };
        panel.Controls.Add(t);

        Label? sub = null;
        if (!string.IsNullOrEmpty(subtitle))
        {
            sub = new Label
            {
                Text = subtitle,
                Font = Ds.Ui(8.5f),
                ForeColor = Ds.Muted,
                AutoSize = false,
                Height = 20,
                Left = 0,
                Top = 28,
            };
            panel.Controls.Add(sub);
        }

        panel.Layout += (_, _) =>
        {
            t.Width = panel.Width;
            if (sub is not null) sub.Width = panel.Width;
        };
        return panel;
    }

    /// <summary>一行设置项：左标题+说明，右控件</summary>
    private static Control Row(string title, string? desc, Control? control)
    {
        var hasDesc = !string.IsNullOrEmpty(desc);
        var panel = new Panel
        {
            Height = hasDesc ? 64 : 48,
            BackColor = Color.White,
            Margin = new Padding(0),
        };
        var t = new Label
        {
            Text = title,
            Font = Ds.Ui(9.5f),
            ForeColor = Ds.Text,
            AutoSize = false,
            Height = 22,
            Left = 16,
            Top = hasDesc ? 11 : 13,
            AutoEllipsis = true,
        };
        panel.Controls.Add(t);

        Label? d = null;
        if (hasDesc)
        {
            d = new Label
            {
                Text = desc,
                Font = Ds.Ui(8.5f),
                ForeColor = Ds.Muted,
                AutoSize = false,
                Height = 18,
                Left = 16,
                Top = 33,
                AutoEllipsis = true,
            };
            panel.Controls.Add(d);
        }

        if (control is not null)
        {
            control.Anchor = AnchorStyles.Right | AnchorStyles.Top;
            panel.Controls.Add(control);
        }

        void Layout()
        {
            if (control is not null)
            {
                // 初始布局时 panel.Width 还可能是 0，不能把控件排到负坐标（会让 AutoScroll 撑出横向滚动条）
                var left = panel.Width - 16 - control.Width;
                control.Left = Math.Max(0, left);
                control.Top = Math.Max(0, (panel.Height - control.Height) / 2);
            }
            var rightEdge = control is null ? panel.Width - 16 : control.Left - 16;
            var textWidth = Math.Max(60, rightEdge - 16);
            t.Width = textWidth;
            if (d is not null) d.Width = textWidth;
        }

        panel.Layout += (_, _) => Layout();
        panel.HandleCreated += (_, _) => Layout();
        return panel;
    }

    private static Control Divider() => new Panel
    {
        Height = 1,
        BackColor = Ds.Divider,
        Margin = new Padding(0),
    };

    /// <summary>把若干行装进一张圆角白卡片</summary>
    private static Control Card(params Control[] rows)
    {
        var inner = new FlowLayoutPanel
        {
            Dock = DockStyle.Fill,
            FlowDirection = FlowDirection.TopDown,
            WrapContents = false,
            AutoScroll = false,
            BackColor = Color.White,
            Padding = new Padding(0, 6, 0, 6),
        };
        var list = new List<Control>();
        for (var i = 0; i < rows.Length; i++)
        {
            list.Add(rows[i]);
            if (i < rows.Length - 1) list.Add(Divider());
        }
        foreach (var c in list) inner.Controls.Add(c);

        var height = 12;
        foreach (var c in list) height += c.Height;

        var card = new RoundedPanel { Height = height, BackColor = Color.White, Margin = new Padding(0, 4, 0, 18) };
        card.Controls.Add(inner);
        card.Resize += (_, _) => { foreach (Control c in inner.Controls) c.Width = Math.Max(200, inner.ClientSize.Width); };
        inner.Resize += (_, _) => { foreach (Control c in inner.Controls) c.Width = Math.Max(200, inner.ClientSize.Width); };
        return card;
    }

    private static Button Btn(string text, EventHandler? onClick, bool primary = false, int width = 96)
    {
        var b = new Button
        {
            Text = text,
            Width = width,
            Height = 30,
            FlatStyle = FlatStyle.Flat,
            BackColor = primary ? Ds.Accent : Color.White,
            ForeColor = primary ? Color.White : Ds.Text,
            Font = Ds.Ui(9f),
            Cursor = Cursors.Hand,
            Margin = new Padding(0),
        };
        b.FlatAppearance.BorderSize = primary ? 0 : 1;
        b.FlatAppearance.BorderColor = Ds.Border;
        if (onClick is not null) b.Click += onClick;
        return b;
    }

    private static TextBox Field(string text) => new()
    {
        Text = text,
        Width = 300,
        Height = 28,
        BorderStyle = BorderStyle.FixedSingle,
        Font = Ds.Ui(9f),
        ForeColor = Ds.Text,
        BackColor = Ds.Field,
    };

    private static Label Value(string text) => new()
    {
        Text = text,
        AutoSize = false,
        Height = 22,
        Width = 360,
        TextAlign = ContentAlignment.MiddleRight,
        Font = Ds.Ui(9f),
        ForeColor = Ds.Muted,
    };

    private static Label Status() => new()
    {
        Text = "",
        AutoSize = false,
        Height = 44,
        Font = Ds.Ui(8.5f),
        ForeColor = Ds.Muted,
        Margin = new Padding(0, 0, 0, 6),
    };

    private static void SetText(Label label, string text)
    {
        try
        {
            if (label.IsHandleCreated) label.BeginInvoke(new Action(() => label.Text = text));
            else label.Text = text;
        }
        catch { }
    }

    private static void OpenPath(string path)
    {
        try
        {
            if (File.Exists(path) || Directory.Exists(path))
                System.Diagnostics.Process.Start(new System.Diagnostics.ProcessStartInfo(path) { UseShellExecute = true });
        }
        catch { }
    }

    // ==================== 各分区 ====================

    private Control BuildDataDir()
    {
        var flow = Page();
        flow.Controls.Add(SectionTitle("数据目录", "登录信息、缓存与日志都放在这里。绿色版可用环境变量 ZUPU_DATA_DIR 指到别处。"));
        var open = Btn("打开", (_, _) => OpenPath(AppPaths.DataDir), width: 60);
        var copy = Btn("复制路径", (_, _) => { try { Clipboard.SetText(AppPaths.DataDir); } catch { } }, width: 84);
        var value = Value(AppPaths.DataDir);
        var buttons = new FlowLayoutPanel { FlowDirection = FlowDirection.LeftToRight, WrapContents = false, AutoSize = true, Height = 30 };
        buttons.Controls.Add(open); buttons.Controls.Add(copy);

        flow.Controls.Add(Card(
            Row("当前目录", null, value),
            Row("操作", "打开文件夹，或复制完整路径", buttons)));
        return flow;
    }

    private Control BuildClientUpdate()
    {
        var flow = Page();
        UpdateManifest? latest = null;

        var status = Status();
        var feed = Field(ClientUpdater.FeedUrl);
        feed.Width = 320;
        var check = Btn("检查更新", null, width: 84);
        var apply = Btn("下载并更新", null, primary: true, width: 96);
        apply.Enabled = false;
        var bar = new ProgressBar { Height = 6, Width = 300, Minimum = 0, Maximum = 100, Style = ProgressBarStyle.Continuous, Visible = false };

        var rowButtons = new FlowLayoutPanel { FlowDirection = FlowDirection.LeftToRight, WrapContents = false, AutoSize = true, Height = 30 };
        rowButtons.Controls.Add(check); rowButtons.Controls.Add(apply);

        check.Click += async (_, _) =>
        {
            check.Enabled = false;
            status.Text = "正在检查…";
            try
            {
                ClientUpdater.FeedUrl = feed.Text.Trim();
                latest = await ClientUpdater.CheckAsync();
                var newer = UpdateFeed.CompareVersion(latest.Version, ClientUpdater.CurrentVersion) > 0;
                status.Text = newer
                    ? $"发现新版本 {latest.Version}（当前 {ClientUpdater.CurrentVersion}），{latest.SizeText}"
                    : $"已是最新版本（{ClientUpdater.CurrentVersion}）";
                apply.Enabled = newer;
            }
            catch (Exception ex) { status.Text = "检查失败：" + ex.Message; apply.Enabled = false; }
            finally { check.Enabled = true; }
        };

        apply.Click += async (_, _) =>
        {
            if (latest is null) return;
            if (MessageBox.Show($"更新到 {latest.Version}？\n客户端会关闭，替换完成后自动重新打开。",
                    "客户端更新", MessageBoxButtons.OKCancel, MessageBoxIcon.Question) != DialogResult.OK) return;
            check.Enabled = apply.Enabled = false;
            try
            {
                status.Text = "正在下载…";
                bar.Visible = true;
                bar.Value = 0;
                var stage = await ClientUpdater.DownloadAndStageAsync(latest,
                    new Progress<int>(p => { if (p >= 0) bar.Value = Math.Min(100, p); }));
                status.Text = "正在应用更新，客户端即将重启…";
                ClientUpdater.ApplyAndRestart(stage, latest.Version);
                Application.Exit();
            }
            catch (Exception ex) { status.Text = "更新失败：" + ex.Message; check.Enabled = apply.Enabled = true; }
        };

        flow.Controls.Add(SectionTitle("客户端更新", "只覆盖程序文件，登录信息与缓存（数据目录）都会保留。"));
        flow.Controls.Add(Card(
            Row("当前版本", null, Value(ClientUpdater.CurrentVersion)),
            Row("更新源", "留空使用默认源", feed),
            Row("检查并更新", "下载后会自动重启客户端", rowButtons)));
        flow.Controls.Add(bar);
        flow.Controls.Add(status);
        return flow;
    }

    private Control BuildServer()
    {
        var flow = Page();
        UpdateManifest? latest = null;

        var state = Value("");
        var feed = Field(ServerInstaller.FeedUrl);
        feed.Width = 320;
        var dir = Field(ServerInstaller.InstallDir);
        dir.Width = 300;
        dir.Height = 26;
        dir.Margin = new Padding(0, 2, 6, 0);

        var pickDir = Btn("选择…", (_, _) =>
        {
            using var dlg = new FolderBrowserDialog
            {
                Description = "选择服务端安装目录",
                UseDescriptionForTitle = true,
                ShowNewFolderButton = true,
            };
            var current = dir.Text.Trim();
            if (Directory.Exists(current)) dlg.SelectedPath = current;
            else
            {
                try { dlg.SelectedPath = Path.GetDirectoryName(current) ?? ServerInstaller.DefaultDir; } catch { }
            }
            if (dlg.ShowDialog(this) == DialogResult.OK) dir.Text = dlg.SelectedPath;
        }, width: 64);
        pickDir.Height = 28;
        pickDir.Margin = new Padding(0);

        var dirPicker = new FlowLayoutPanel
        {
            FlowDirection = FlowDirection.LeftToRight,
            WrapContents = false,
            AutoSize = true,
            Height = 28,
        };
        dirPicker.Controls.Add(dir);
        dirPicker.Controls.Add(pickDir);
        var status = Status();
        var bar = new ProgressBar { Height = 6, Width = 300, Minimum = 0, Maximum = 100, Style = ProgressBarStyle.Continuous, Visible = false };

        void RefreshState() => state.Text = ServerInstaller.IsInstalled
            ? $"已安装 v{ServerInstaller.InstalledVersion}　{(ServerInstaller.IsRunning() ? "运行中" : "未运行")}"
            : "未安装";

        var check = Btn("检查更新", null, width: 84);
        var install = Btn("下载并安装", null, primary: true, width: 96);
        var local = Btn("本地包…", null, width: 80);
        var row1 = new FlowLayoutPanel { FlowDirection = FlowDirection.LeftToRight, WrapContents = false, AutoSize = true, Height = 30 };
        row1.Controls.Add(check); row1.Controls.Add(install); row1.Controls.Add(local);

        var start = Btn("启动服务端", null, width: 96);
        var openDir = Btn("打开目录", null, width: 80);
        var row2 = new FlowLayoutPanel { FlowDirection = FlowDirection.LeftToRight, WrapContents = false, AutoSize = true, Height = 30 };
        row2.Controls.Add(start); row2.Controls.Add(openDir);

        check.Click += async (_, _) =>
        {
            check.Enabled = false;
            status.Text = "正在检查…";
            try
            {
                ServerInstaller.FeedUrl = feed.Text.Trim();
                latest = await ServerInstaller.CheckLatestAsync();
                status.Text = $"源上最新 v{latest.Version}　{latest.SizeText}";
            }
            catch (Exception ex) { status.Text = "检查失败：" + ex.Message; }
            finally { check.Enabled = true; }
        };

        install.Click += async (_, _) =>
        {
            if (latest is null) { status.Text = "请先点「检查更新」"; return; }
            if (MessageBox.Show($"把服务端 v{latest.Version} 安装到：\n{dir.Text}\n\ndata 数据不会被覆盖。",
                    "安装服务端", MessageBoxButtons.OKCancel, MessageBoxIcon.Question) != DialogResult.OK) return;
            check.Enabled = install.Enabled = false;
            try
            {
                ServerInstaller.InstallDir = dir.Text.Trim();
                status.Text = "正在下载全量包…";
                bar.Visible = true;
                bar.Value = 0;
                var pkg = await ServerInstaller.DownloadAsync(latest,
                    new Progress<int>(p => { if (p >= 0) bar.Value = Math.Min(100, p); }));
                var target = ServerInstaller.InstallDir;
                var ver = latest.Version;
                await Task.Run(() => ServerInstaller.InstallFromPackage(pkg, target, m => SetText(status, m), ver));
                status.Text = "安装完成";
                RefreshState();
            }
            catch (Exception ex) { status.Text = "安装失败：" + ex.Message; }
            finally { check.Enabled = install.Enabled = true; }
        };

        local.Click += async (_, _) =>
        {
            using var dlg = new OpenFileDialog
            {
                Title = "选择服务端全量包",
                Filter = "更新包 (*.zip;*.tar.gz;*.tgz)|*.zip;*.tar.gz;*.tgz|所有文件 (*.*)|*.*",
            };
            if (dlg.ShowDialog(this) != DialogResult.OK) return;
            try
            {
                ServerInstaller.InstallDir = dir.Text.Trim();
                var target = ServerInstaller.InstallDir;
                var file = dlg.FileName;
                status.Text = "正在安装本地包…";
                await Task.Run(() => ServerInstaller.InstallFromPackage(file, target, m => SetText(status, m)));
                status.Text = "安装完成";
                RefreshState();
            }
            catch (Exception ex) { status.Text = "安装失败：" + ex.Message; }
        };

        start.Click += (_, _) =>
        {
            try
            {
                ServerInstaller.Start();
                status.Text = "已启动（最小化窗口），稍等几秒再访问 http://127.0.0.1:8080";
                RefreshState();
            }
            catch (Exception ex) { status.Text = "启动失败：" + ex.Message; }
        };

        openDir.Click += (_, _) => OpenPath(ServerInstaller.InstallDir);

        RefreshState();

        flow.Controls.Add(SectionTitle("服务端", "全量包：下载解压即安装/更新；data 与 knowledge 目录永不覆盖。"));
        flow.Controls.Add(Card(
            Row("安装状态", null, state),
            Row("安装目录", "可点「选择…」更换位置", dirPicker),
            Row("更新源", "包清单地址，留空用默认", feed),
            Row("安装与更新", null, row1),
            Row("运行", null, row2)));
        flow.Controls.Add(bar);
        flow.Controls.Add(status);
        return flow;
    }

    private Control BuildStartup()
    {
        var flow = Page();
        var detail = Status();
        detail.Height = 60;

        var clientSwitch = new ToggleSwitch { Checked = Autostart.ClientEnabled };
        var serverSwitch = new ToggleSwitch { Checked = Autostart.ServerEnabled };

        void RefreshDetail() => detail.Text = "客户端：" + Autostart.ClientMechanism + "\n服务端：" + Autostart.ServerMechanism;

        clientSwitch.CheckedChanged += (_, _) =>
        {
            try { Autostart.SetClient(clientSwitch.Checked); }
            catch (Exception ex)
            {
                MessageBox.Show("设置失败：" + ex.Message, "开机自启", MessageBoxButtons.OK, MessageBoxIcon.Warning);
                clientSwitch.Checked = Autostart.ClientEnabled;
            }
            RefreshDetail();
        };
        serverSwitch.CheckedChanged += (_, _) =>
        {
            try { Autostart.SetServer(serverSwitch.Checked); }
            catch (Exception ex)
            {
                MessageBox.Show("设置失败：" + ex.Message, "开机自启", MessageBoxButtons.OK, MessageBoxIcon.Warning);
                serverSwitch.Checked = Autostart.ServerEnabled;
            }
            RefreshDetail();
        };

        RefreshDetail();

        flow.Controls.Add(SectionTitle("开机自启", "登录 Windows 后自动启动；服务端需先安装。"));
        flow.Controls.Add(Card(
            Row("客户端开机自启", "打开电脑后自动启动族谱客户端", clientSwitch),
            Row("服务端开机自启", "打开电脑后自动启动本地服务端", serverSwitch)));
        flow.Controls.Add(detail);
        return flow;
    }

    /// <summary>点 ✕ 时的行为：直接退出 / 后台运行，以及是否每次询问</summary>
    private Control BuildClose()
    {
        var flow = Page();

        var exit = new ChoiceRow { Title = "直接退出", Desc = "关闭程序，与服务器的连接一并断开" };
        var tray = new ChoiceRow { Title = "后台运行", Desc = "窗口隐藏到托盘，程序继续运行，双击托盘图标可重新打开" };
        var choices = new[] { exit, tray };

        foreach (var c in choices)
        {
            var self = c;
            self.Picked += (_, _) =>
            {
                foreach (var other in choices) other.Selected = other == self;
                Prefs.Current.CloseAction = self == tray ? "tray" : "exit";
                Prefs.Save();
            };
        }

        var current = Prefs.Current.CloseAction == "tray" ? tray : exit;
        foreach (var c in choices) c.Selected = c == current;

        var ask = new ToggleSwitch { Checked = Prefs.Current.CloseAsk };
        ask.CheckedChanged += (_, _) =>
        {
            Prefs.Current.CloseAsk = ask.Checked;
            Prefs.Save();
        };

        flow.Controls.Add(SectionTitle("关闭行为", "点窗口右上角的 ✕ 时怎么处理。托盘图标里也有「退出族谱客户端」。"));
        flow.Controls.Add(Card(exit, tray));
        flow.Controls.Add(Card(
            Row("每次关闭都询问", "关掉后直接按上面的选择执行，不再弹窗", ask)));
        return flow;
    }

    private Control BuildCache()    {
        var flow = Page();
        var cacheDir = Path.Combine(AppPaths.DataDir, "cef-cache");
        var status = Status();
        var size = Value("");

        void Refresh() => size.Text = Fmt(DirSize(cacheDir));

        var clear = Btn("清除缓存", (_, _) =>
        {
            try
            {
                if (Directory.Exists(cacheDir)) { var freed = DirSize(cacheDir); Directory.Delete(cacheDir, true); status.Text = "已清除 " + Fmt(freed); }
                else status.Text = "没有需要清除的缓存";
            }
            catch
            {
                try
                {
                    var old = cacheDir + ".old";
                    if (Directory.Exists(old)) Directory.Delete(old, true);
                    Directory.Move(cacheDir, old);
                    status.Text = "已标记清除，下次启动客户端时生效";
                }
                catch (Exception ex) { status.Text = "清除失败（" + ex.GetType().Name + "），请关闭客户端后重试"; }
            }
            Refresh();
        }, width: 84);

        var openLog = Btn("日志目录", (_, _) => OpenPath(AppPaths.DataDir), width: 84);
        var row = new FlowLayoutPanel { FlowDirection = FlowDirection.LeftToRight, WrapContents = false, AutoSize = true, Height = 30 };
        row.Controls.Add(clear); row.Controls.Add(openLog);

        Refresh();
        flow.Controls.Add(SectionTitle("缓存与日志", "页面显示异常时可先清一次缓存。"));
        flow.Controls.Add(Card(
            Row("网页缓存占用", null, size),
            Row("操作", null, row)));
        flow.Controls.Add(status);
        return flow;
    }

    private Control BuildAbout()
    {
        var flow = Page();
        string chromium;
        try { chromium = Cef.ChromiumVersion; } catch { chromium = "未初始化"; }

        var info = H(
            "族谱客户端  " + ClientUpdater.CurrentVersion + "\n"
            + "内置内核  Chromium " + chromium + "\n"
            + "运行库  " + RuntimeInformation.FrameworkDescription + "\n"
            + "程序目录  " + AppContext.BaseDirectory + "\n"
            + "数据目录  " + AppPaths.DataDir,
            9f, Ds.Muted, FontStyle.Regular, 108);

        var openApp = Btn("打开程序目录", (_, _) => OpenPath(AppContext.BaseDirectory), width: 108);

        flow.Controls.Add(SectionTitle("关于", "客户端只是网页版族谱系统的桌面外壳，所有数据以服务器为准。"));
        flow.Controls.Add(Card(
            Row("版本信息", null, Value(ClientUpdater.CurrentVersion)),
            Row("程序目录", null, openApp)));
        flow.Controls.Add(info);
        return flow;
    }

    private void EnableDrag(Control handle)
    {
        handle.MouseDown += (_, e) =>
        {
            if (e.Button != MouseButtons.Left) return;
            ReleaseCapture();
            SendMessage(Handle, 0xA1, (IntPtr)0x2, IntPtr.Zero);
        };
    }

    [DllImport("user32.dll")]
    private static extern bool ReleaseCapture();

    [DllImport("user32.dll")]
    private static extern IntPtr SendMessage(IntPtr hWnd, int msg, IntPtr wParam, IntPtr lParam);

    private static long DirSize(string dir)
    {
        try
        {
            if (!Directory.Exists(dir)) return 0;
            return new DirectoryInfo(dir).EnumerateFiles("*", SearchOption.AllDirectories).Sum(f => f.Length);
        }
        catch { return 0; }
    }

    private static string Fmt(long bytes)
    {
        if (bytes <= 0) return "0 B";
        string[] units = { "B", "KB", "MB", "GB" };
        double v = bytes; var i = 0;
        while (v >= 1024 && i < units.Length - 1) { v /= 1024; i++; }
        return v.ToString(i == 0 ? "0" : "0.0") + " " + units[i];
    }
}

/// <summary>标题栏右侧的关闭按钮</summary>
internal sealed class CloseButton : Control
{
    private bool _hot;

    public CloseButton()
    {
        SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.SupportsTransparentBackColor
               | ControlStyles.UserPaint | ControlStyles.ResizeRedraw, true);
        Size = new Size(34, 28);
        Cursor = Cursors.Hand;
        BackColor = Color.Transparent;
    }

    protected override void OnMouseEnter(EventArgs e) { _hot = true; Invalidate(); base.OnMouseEnter(e); }
    protected override void OnMouseLeave(EventArgs e) { _hot = false; Invalidate(); base.OnMouseLeave(e); }

    protected override void OnPaint(PaintEventArgs e)
    {
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        if (_hot)
        {
            using var path = Ds.Round(new RectangleF(0, 0, Width - 1, Height - 1), 6);
            using var b = new SolidBrush(Ds.NavSel);
            g.FillPath(b, path);
        }
        using var pen = new Pen(ColorTranslator.FromHtml("#6B7280"), 1.4f);
        var cx = Width / 2f; var cy = Height / 2f;
        g.DrawLine(pen, cx - 5, cy - 5, cx + 5, cy + 5);
        g.DrawLine(pen, cx + 5, cy - 5, cx - 5, cy + 5);
    }
}
