using System.Drawing.Drawing2D;

namespace ZupuClient;

/// <summary>单选项行：左边一个圆点，右边标题 + 说明。点整行都能选。</summary>
internal sealed class ChoiceRow : Control
{
    private bool _selected;
    private bool _hot;

    public string Title { get; set; } = "";
    public string Desc { get; set; } = "";

    public event EventHandler? Picked;

    public bool Selected
    {
        get => _selected;
        set { if (_selected == value) return; _selected = value; Invalidate(); }
    }

    public ChoiceRow()
    {
        SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer
               | ControlStyles.UserPaint | ControlStyles.ResizeRedraw
               | ControlStyles.SupportsTransparentBackColor, true);
        Height = 58;
        Cursor = Cursors.Hand;
        BackColor = Color.Transparent;
    }

    protected override void OnMouseEnter(EventArgs e) { _hot = true; Invalidate(); base.OnMouseEnter(e); }
    protected override void OnMouseLeave(EventArgs e) { _hot = false; Invalidate(); base.OnMouseLeave(e); }

    protected override void OnClick(EventArgs e)
    {
        Selected = true;
        Picked?.Invoke(this, EventArgs.Empty);
        base.OnClick(e);
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        g.TextRenderingHint = System.Drawing.Text.TextRenderingHint.ClearTypeGridFit;

        if (_hot && !_selected)
        {
            using var hover = new SolidBrush(Color.FromArgb(110, Ds.NavSel));
            using var path = Ds.Round(new RectangleF(0, 2, Width - 1, Height - 5), 8);
            g.FillPath(hover, path);
        }

        const float cx = 20f;
        var cy = Height / 2f - 8;
        var ring = new RectangleF(cx - 8, cy, 16, 16);
        using (var pen = new Pen(_selected ? Ds.Accent : ColorTranslator.FromHtml("#B9BEC7"), 1.6f))
            g.DrawEllipse(pen, ring);
        if (_selected)
            using (var dot = new SolidBrush(Ds.Accent))
                g.FillEllipse(dot, cx - 4, cy + 4, 8, 8);

        using var titleFont = Ds.Ui(9.5f, _selected ? FontStyle.Bold : FontStyle.Regular);
        using var titleBrush = new SolidBrush(Ds.Text);
        g.DrawString(Title, titleFont, titleBrush, new PointF(42, 9));

        if (!string.IsNullOrEmpty(Desc))
        {
            using var descFont = Ds.Ui(8.5f);
            using var descBrush = new SolidBrush(Ds.Muted);
            g.DrawString(Desc, descFont, descBrush, new PointF(42, 32));
        }
    }
}

/// <summary>「以后不再提醒」那种勾选框</summary>
internal sealed class CheckRow : Control
{
    private bool _checked;
    private bool _hot;

    public event EventHandler? CheckedChanged;

    public bool Checked
    {
        get => _checked;
        set { if (_checked == value) return; _checked = value; Invalidate(); CheckedChanged?.Invoke(this, EventArgs.Empty); }
    }

    public CheckRow()
    {
        SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer
               | ControlStyles.UserPaint | ControlStyles.ResizeRedraw
               | ControlStyles.SupportsTransparentBackColor, true);
        Height = 26;
        Cursor = Cursors.Hand;
        BackColor = Color.Transparent;
    }

    protected override void OnMouseEnter(EventArgs e) { _hot = true; Invalidate(); base.OnMouseEnter(e); }
    protected override void OnMouseLeave(EventArgs e) { _hot = false; Invalidate(); base.OnMouseLeave(e); }
    protected override void OnClick(EventArgs e) { Checked = !Checked; base.OnClick(e); }

    protected override void OnPaint(PaintEventArgs e)
    {
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        var box = new RectangleF(2, (Height - 16) / 2f, 16, 16);
        using (var path = Ds.Round(box, 4))
        {
            using var fill = new SolidBrush(_checked ? Ds.Accent : Color.White);
            g.FillPath(fill, path);
            using var pen = new Pen(_checked ? Ds.Accent : ColorTranslator.FromHtml("#B9BEC7"), 1.4f);
            g.DrawPath(pen, path);
        }
        if (_checked)
        {
            using var pen = new Pen(Color.White, 1.8f) { StartCap = LineCap.Round, EndCap = LineCap.Round };
            g.DrawLines(pen, new[]
            {
                new PointF(box.X + 4, box.Y + 8.2f),
                new PointF(box.X + 6.8f, box.Y + 11f),
                new PointF(box.X + 12, box.Y + 5f),
            });
        }
        using var font = Ds.Ui(9f);
        using var brush = new SolidBrush(_hot ? Ds.Text : ColorTranslator.FromHtml("#4B5563"));
        g.DrawString(Text, font, brush, new PointF(26, (Height - 18) / 2f));
    }
}

/// <summary>点 ✕ 时的确认框：直接退出 / 后台运行 + 以后不再提醒</summary>
internal sealed class ClosePromptDialog : Form
{
    private readonly ChoiceRow _exit;
    private readonly ChoiceRow _tray;
    private readonly CheckRow _noAsk;
    private readonly Button _ok;
    private readonly Button _cancel;
    private readonly CloseButton _close = new();

    public string Action => _tray.Selected ? "tray" : "exit";
    public bool NoMoreAsk => _noAsk.Checked;

    public ClosePromptDialog(string currentAction, bool noMoreAsk)
    {
        Text = "关闭族谱客户端";
        FormBorderStyle = FormBorderStyle.None;
        StartPosition = FormStartPosition.CenterParent;
        ShowInTaskbar = false;
        BackColor = Ds.Surface;
        Size = new Size(452, 322);
        DoubleBuffered = true;
        Padding = new Padding(1);

        var header = new Panel { BackColor = Ds.Surface };
        var title = new Label
        {
            Text = "关闭族谱客户端",
            Font = Ds.Ui(11.5f, FontStyle.Bold),
            ForeColor = Ds.Text,
            AutoSize = false,
            TextAlign = ContentAlignment.MiddleLeft,
        };
        header.Controls.Add(title);
        header.Controls.Add(_close);
        _close.Click += (_, _) => { DialogResult = DialogResult.Cancel; Close(); };

        var question = new Label
        {
            Text = "关闭主窗口后要做什么？",
            Font = Ds.Ui(9f),
            ForeColor = Ds.Muted,
            AutoSize = false,
        };

        _exit = new ChoiceRow { Title = "直接退出", Desc = "关闭程序，与服务器的连接一并断开" };
        _tray = new ChoiceRow { Title = "后台运行", Desc = "窗口隐藏到托盘，程序继续运行，双击托盘图标可重新打开" };
        _exit.Picked += (_, _) => _tray.Selected = false;
        _tray.Picked += (_, _) => _exit.Selected = false;
        _exit.Selected = currentAction != "tray";
        _tray.Selected = currentAction == "tray";

        _noAsk = new CheckRow { Text = "以后不再提醒（可在 设置 → 关闭行为 里修改）", Checked = noMoreAsk };

        _ok = new Button
        {
            Text = "确定",
            Width = 84,
            Height = 30,
            FlatStyle = FlatStyle.Flat,
            BackColor = Ds.Accent,
            ForeColor = Color.White,
            Font = Ds.Ui(9f),
            Cursor = Cursors.Hand,
        };
        _ok.FlatAppearance.BorderSize = 0;
        _ok.Click += (_, _) => { DialogResult = DialogResult.OK; Close(); };

        _cancel = new Button
        {
            Text = "取消",
            Width = 84,
            Height = 30,
            FlatStyle = FlatStyle.Flat,
            BackColor = Color.White,
            ForeColor = Ds.Text,
            Font = Ds.Ui(9f),
            Cursor = Cursors.Hand,
        };
        _cancel.FlatAppearance.BorderSize = 1;
        _cancel.FlatAppearance.BorderColor = Ds.Border;
        _cancel.Click += (_, _) => { DialogResult = DialogResult.Cancel; Close(); };

        Controls.Add(header);
        Controls.Add(question);
        Controls.Add(_exit);
        Controls.Add(_tray);
        Controls.Add(_noAsk);
        Controls.Add(_ok);
        Controls.Add(_cancel);

        void Layout()
        {
            const int pad = 24, headerH = 48;
            header.SetBounds(0, 0, ClientSize.Width, headerH);
            title.SetBounds(pad, 0, ClientSize.Width - pad - 56, headerH);
            _close.SetBounds(ClientSize.Width - 12 - _close.Width, (headerH - _close.Height) / 2, _close.Width, _close.Height);

            var y = headerH + 18;
            question.SetBounds(pad, y, ClientSize.Width - pad * 2, 22);
            y += 30;
            _exit.SetBounds(pad, y, ClientSize.Width - pad * 2, _exit.Height); y += _exit.Height + 2;
            _tray.SetBounds(pad, y, ClientSize.Width - pad * 2, _tray.Height); y += _tray.Height + 14;
            _noAsk.SetBounds(pad, y, ClientSize.Width - pad * 2, _noAsk.Height);

            var by = ClientSize.Height - 30 - 18;
            _ok.SetBounds(ClientSize.Width - pad - _ok.Width, by, _ok.Width, _ok.Height);
            _cancel.SetBounds(_ok.Left - 10 - _cancel.Width, by, _cancel.Width, _cancel.Height);
        }
        Resize += (_, _) => Layout();
        Layout();

        AcceptButton = _ok;
        CancelButton = _cancel;
        KeyPreview = true;
        KeyDown += (_, e) => { if (e.KeyCode == Keys.Escape) { DialogResult = DialogResult.Cancel; Close(); } };
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        base.OnPaint(e);
        using var pen = new Pen(Ds.Border, 1f);
        e.Graphics.DrawRectangle(pen, 0, 0, Width - 1, Height - 1);
    }
}
