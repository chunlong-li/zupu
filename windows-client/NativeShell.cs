using System.Drawing.Drawing2D;

namespace ZupuClient;

/// <summary>配色与尺寸，对齐网页端 style.css / App.vue</summary>
internal static class Theme
{
    public static readonly Color Ink = ColorTranslator.FromHtml("#3a3226");      // 墨色：顶栏/侧栏底
    public static readonly Color Mibai = ColorTranslator.FromHtml("#f7f3ec");    // 米白：内容底
    public static readonly Color Zheshi = ColorTranslator.FromHtml("#8c5a3c");   // 赭石：头像
    public static readonly Color Border = ColorTranslator.FromHtml("#d9cbb6");   // 边框/菜单文字
    public static readonly Color MenuText = ColorTranslator.FromHtml("#d9cbb6");
    public static readonly Color MenuActive = Color.White;
    public static readonly Color FooterText = ColorTranslator.FromHtml("#5a4a3a");
    public static readonly Color Hover = Color.FromArgb(16, 255, 255, 255);      // rgba(255,255,255,.06)
    public static readonly Color Active = Color.FromArgb(26, 255, 255, 255);     // rgba(255,255,255,.10)

    public const int TopBarHeight = 52;
    public const int SidebarWidth = 200;
    public const int MenuRowHeight = 56;      // Element Plus: --el-menu-item-height
    public const int SubMenuRowHeight = 50;   // Element Plus: --el-menu-sub-item-height
    public const int SidebarFooterHeight = 36;

    public static Font UiFont(float size = 10f, FontStyle style = FontStyle.Regular)
        => new("Microsoft YaHei UI", size, style, GraphicsUnit.Point);

    /// <summary>emoji 字体（GDI 下为单色，但能正确出形，不会变方框）</summary>
    public static Font EmojiFont(float size = 10f)
    {
        foreach (var name in new[] { "Segoe UI Emoji", "Segoe UI Symbol", "Segoe UI" })
        {
            try
            {
                var f = new Font(name, size, FontStyle.Regular, GraphicsUnit.Point);
                if (string.Equals(f.Name, name, StringComparison.OrdinalIgnoreCase)) return f;
                f.Dispose();
            }
            catch { /* 试下一个 */ }
        }
        return new Font("Microsoft YaHei UI", size, FontStyle.Regular, GraphicsUnit.Point);
    }

    public static Font LogoFont()
    {
        // 网页用 Songti SC / SimSun；精简版系统可能没装宋体，按顺序回退
        foreach (var name in new[] { "SimSun", "NSimSun", "宋体", "STSong", "FangSong", "KaiTi" })
        {
            try
            {
                var f = new Font(name, 15f, FontStyle.Regular, GraphicsUnit.Point);
                if (string.Equals(f.Name, name, StringComparison.OrdinalIgnoreCase)) return f;
                f.Dispose();
            }
            catch { /* 试下一个 */ }
        }
        return new Font("Microsoft YaHei UI", 15f, FontStyle.Regular, GraphicsUnit.Point);
    }
}

/// <summary>带字间距的标题（网页 .logo 用 letter-spacing:3px）</summary>
internal sealed class SpacedLabel : Control
{
    public float LetterSpacing { get; set; } = 3f;
    private bool _hot;

    public SpacedLabel()
    {
        SetStyle(ControlStyles.UserPaint | ControlStyles.AllPaintingInWmPaint
               | ControlStyles.OptimizedDoubleBuffer | ControlStyles.SupportsTransparentBackColor, true);
        BackColor = Color.Transparent;
        ForeColor = Color.White;
        Font = Theme.LogoFont();
        Cursor = Cursors.Hand;
        AutoSize = false;
    }

    /// <summary>含字间距的文本宽度</summary>
    public int PreferredWidth()
    {
        var text = Text ?? "";
        if (text.Length == 0) return 0;
        using var g = CreateGraphics();
        float total = 0;
        foreach (var ch in text) total += g.MeasureString(ch.ToString(), Font).Width;
        total += LetterSpacing * Math.Max(0, text.Length - 1);
        return (int)Math.Ceiling(total) + 2;
    }

    protected override void OnMouseEnter(EventArgs e) { _hot = true; Invalidate(); base.OnMouseEnter(e); }
    protected override void OnMouseLeave(EventArgs e) { _hot = false; Invalidate(); base.OnMouseLeave(e); }

    protected override void OnPaint(PaintEventArgs e)
    {
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        g.TextRenderingHint = System.Drawing.Text.TextRenderingHint.ClearTypeGridFit;

        var text = Text ?? "";
        var color = _hot ? Color.FromArgb(217, 255, 255, 255) : ForeColor;
        using var brush = new SolidBrush(color);

        // 逐字绘制以实现字间距
        float total = 0;
        var widths = new float[text.Length];
        for (var i = 0; i < text.Length; i++)
        {
            var s = text[i].ToString();
            widths[i] = g.MeasureString(s, Font).Width;
            total += widths[i];
        }
        total += LetterSpacing * Math.Max(0, text.Length - 1);

        var x = (Width - total) / 2f;
        var y = (Height - Font.GetHeight(g)) / 2f;
        for (var i = 0; i < text.Length; i++)
        {
            g.DrawString(text[i].ToString(), Font, brush, x, y);
            x += widths[i] + LetterSpacing;
        }
    }
}

/// <summary>图片里的圆形头像（网页 .user-avatar：30px，赭石底，白字）</summary>
internal sealed class AvatarButton : Control
{
    private bool _hot;
    public string Initial { get; set; } = "族";

    public AvatarButton()
    {
        SetStyle(ControlStyles.UserPaint | ControlStyles.AllPaintingInWmPaint
               | ControlStyles.OptimizedDoubleBuffer | ControlStyles.SupportsTransparentBackColor, true);
        Size = new Size(30, 30);
        BackColor = Color.Transparent;
        Cursor = Cursors.Hand;
    }

    protected override void OnMouseEnter(EventArgs e) { _hot = true; Invalidate(); base.OnMouseEnter(e); }
    protected override void OnMouseLeave(EventArgs e) { _hot = false; Invalidate(); base.OnMouseLeave(e); }

    protected override void OnPaint(PaintEventArgs e)
    {
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        var rect = new Rectangle(0, 0, Width - 1, Height - 1);
        using var fill = new SolidBrush(_hot ? ColorTranslator.FromHtml("#9a6644") : Theme.Zheshi);
        g.FillEllipse(fill, rect);
        using var font = Theme.UiFont(9.5f);
        using var sf = new StringFormat { Alignment = StringAlignment.Center, LineAlignment = StringAlignment.Center };
        g.DrawString(Initial, font, Brushes.White, new RectangleF(0, 0, Width, Height), sf);
    }
}

/// <summary>顶栏右侧的纯文字按钮（网页 .logout-btn：白字，悬停 rgba(255,255,255,.1)）</summary>
internal sealed class FlatTextButton : Control
{
    private bool _hot;

    public FlatTextButton()
    {
        SetStyle(ControlStyles.UserPaint | ControlStyles.AllPaintingInWmPaint
               | ControlStyles.OptimizedDoubleBuffer | ControlStyles.SupportsTransparentBackColor, true);
        BackColor = Color.Transparent;
        ForeColor = Color.White;
        Font = Theme.UiFont(9.5f);
        Cursor = Cursors.Hand;
        Size = new Size(56, 30);
    }

    protected override void OnMouseEnter(EventArgs e) { _hot = true; Invalidate(); base.OnMouseEnter(e); }
    protected override void OnMouseLeave(EventArgs e) { _hot = false; Invalidate(); base.OnMouseLeave(e); }

    protected override void OnPaint(PaintEventArgs e)
    {
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        if (_hot)
        {
            using var path = Round(new RectangleF(0, 0, Width, Height), 4);
            using var b = new SolidBrush(Theme.Hover);
            g.FillPath(b, path);
        }
        using var font = Theme.UiFont(9.5f);
        using var sf = new StringFormat { Alignment = StringAlignment.Center, LineAlignment = StringAlignment.Center };
        g.DrawString(Text, font, Brushes.White, new RectangleF(0, 0, Width, Height), sf);
    }

    internal static GraphicsPath Round(RectangleF r, float radius)
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

/// <summary>顶栏「含下级」勾选框（网页 .branch-inc：12×12 边框 + #d9cbb6 文字）</summary>
internal sealed class FlatCheck : Control
{
    private bool _checked = true;
    private bool _hot;

    public event EventHandler? CheckedChanged;

    public bool Checked
    {
        get => _checked;
        set { if (_checked == value) return; _checked = value; Invalidate(); CheckedChanged?.Invoke(this, EventArgs.Empty); }
    }

    public FlatCheck()
    {
        SetStyle(ControlStyles.UserPaint | ControlStyles.AllPaintingInWmPaint
               | ControlStyles.OptimizedDoubleBuffer | ControlStyles.SupportsTransparentBackColor, true);
        BackColor = Color.Transparent;
        Cursor = Cursors.Hand;
        Height = 26;
    }

    protected override void OnMouseEnter(EventArgs e) { _hot = true; Invalidate(); base.OnMouseEnter(e); }
    protected override void OnMouseLeave(EventArgs e) { _hot = false; Invalidate(); base.OnMouseLeave(e); }
    protected override void OnClick(EventArgs e) { Checked = !Checked; base.OnClick(e); }

    protected override void OnPaint(PaintEventArgs e)
    {
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        const int box = 14;
        var bx = 0;
        var by = (Height - box) / 2;
        var rect = new Rectangle(bx, by, box, box);

        using var border = new Pen(Theme.Border, 1f);
        using var fill = new SolidBrush(Color.FromArgb(_checked ? 255 : 30, Theme.Border));
        g.FillRectangle(fill, rect);
        g.DrawRectangle(border, rect);

        if (_checked)
        {
            using var pen = new Pen(Color.FromArgb(90, 58, 50, 38), 1.8f);
            g.DrawLines(pen, new[]
            {
                new PointF(bx + 3f, by + 7f),
                new PointF(bx + 6f, by + 10f),
                new PointF(bx + 11f, by + 4f),
            });
        }

        using var font = Theme.UiFont(9f);
        using var sf = new StringFormat { LineAlignment = StringAlignment.Center };
        using var text = new SolidBrush(_hot ? Color.White : Theme.MenuText);
        g.DrawString(Text, font, text, new RectangleF(box + 6, 0, Width - box - 6, Height), sf);
    }
}

/// <summary>顶栏房支下拉（网页 el-select：200×小号、白字、下拉面板白底）</summary>
internal sealed class BranchPicker : Control
{
    private bool _hot;
    private bool _open;
    private string _value = "宗谱（全部）";

    public long SelectedId { get; private set; }
    public event EventHandler<long>? BranchPicked;

    public sealed record Option(long Id, string Name) { public override string ToString() => Name; }

    private readonly List<Option> _options = new();

    public BranchPicker()
    {
        SetStyle(ControlStyles.UserPaint | ControlStyles.AllPaintingInWmPaint
               | ControlStyles.OptimizedDoubleBuffer | ControlStyles.SupportsTransparentBackColor, true);
        BackColor = Color.Transparent;
        Font = Theme.UiFont(9f);
        Cursor = Cursors.Hand;
        Size = new Size(200, 32);
    }

    public void SetOptions(IEnumerable<Option> options, long selectedId)
    {
        _options.Clear();
        _options.Add(new Option(0, "宗谱（全部）"));
        _options.AddRange(options);
        SelectedId = selectedId;
        _value = _options.FirstOrDefault(o => o.Id == selectedId)?.Name ?? "宗谱（全部）";
        Invalidate();
    }

    /// <summary>只同步显示，不触发 BranchPicked</summary>
    public void SyncSelection(long id)
    {
        SelectedId = id;
        var hit = _options.FirstOrDefault(o => o.Id == id);
        if (hit != null) _value = hit.Name;
        Invalidate();
    }

    protected override void OnMouseEnter(EventArgs e) { _hot = true; Invalidate(); base.OnMouseEnter(e); }
    protected override void OnMouseLeave(EventArgs e) { _hot = false; Invalidate(); base.OnMouseLeave(e); }

    protected override void OnClick(EventArgs e)
    {
        base.OnClick(e);
        ShowDrop();
    }

    private void ShowDrop()
    {
        var menu = new ContextMenuStrip
        {
            Renderer = new DarkMenuRenderer(),
            ShowImageMargin = false,
            BackColor = Color.White,
            Font = Theme.UiFont(9.5f),
            MinimumSize = new Size(Width, 0),
        };
        foreach (var o in _options)
        {
            var item = new ToolStripMenuItem(o.Name) { Padding = new Padding(6, 4, 6, 4) };
            if (o.Id == SelectedId) item.Font = Theme.UiFont(9.5f, FontStyle.Bold);
            var id = o.Id;
            item.Click += (_, _) =>
            {
                SyncSelection(id);
                BranchPicked?.Invoke(this, id);
            };
            menu.Items.Add(item);
        }
        menu.Closed += (_, _) => { _open = false; Invalidate(); };
        _open = true;
        Invalidate();
        menu.Show(this, new Point(0, Height + 2));
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;

        var r = new RectangleF(0, 0, Width - 1, Height - 1);
        using (var path = FlatTextButton.Round(r, 4))
        using (var fill = new SolidBrush(Color.FromArgb(_open || _hot ? 40 : 22, 255, 255, 255)))
        using (var border = new Pen(Color.FromArgb(90, 255, 255, 255), 1f))
        {
            g.FillPath(fill, path);
            g.DrawPath(border, path);
        }

        using var font = Theme.UiFont(9f);
        using var textBrush = new SolidBrush(Color.White);
        using var sf = new StringFormat { LineAlignment = StringAlignment.Center, Trimming = StringTrimming.EllipsisCharacter };
        g.DrawString(_value, font, textBrush, new RectangleF(10, 1, Width - 32, Height - 2), sf);

        // 右侧箭头
        var cx = Width - 16f;
        var cy = Height / 2f;
        using var arrow = new Pen(Color.FromArgb(200, 255, 255, 255), 1.6f);
        if (_open)
            g.DrawLines(arrow, new[] { new PointF(cx - 4, cy + 2), new PointF(cx, cy - 2), new PointF(cx + 4, cy + 2) });
        else
            g.DrawLines(arrow, new[] { new PointF(cx - 4, cy - 2), new PointF(cx, cy + 2), new PointF(cx + 4, cy - 2) });
    }
}

internal sealed class DarkMenuRenderer : ToolStripProfessionalRenderer
{
    protected override void OnRenderMenuItemBackground(ToolStripItemRenderEventArgs e)
    {
        var g = e.Graphics;
        var rect = new Rectangle(0, 0, e.Item.Width, e.Item.Height);
        if (e.Item.Selected)
        {
            using var b = new SolidBrush(ColorTranslator.FromHtml("#f2ece2"));
            g.FillRectangle(b, rect);
        }
        else
        {
            using var b = new SolidBrush(Color.White);
            g.FillRectangle(b, rect);
        }
    }

    protected override void OnRenderItemText(ToolStripItemTextRenderEventArgs e)
    {
        e.TextColor = Theme.Ink;
        base.OnRenderItemText(e);
    }
}

/// <summary>侧栏一行：分组行（可折叠）或页面项</summary>
internal sealed class MenuRow : Control
{
    private bool _hot;

    public string Route = "";
    public string IconKey = "";
    public bool IsGroup;
    public bool Expanded;
    public bool Active;
    public int Level;

    public event EventHandler? Activated;

    public MenuRow()
    {
        SetStyle(ControlStyles.UserPaint | ControlStyles.AllPaintingInWmPaint
               | ControlStyles.OptimizedDoubleBuffer | ControlStyles.SupportsTransparentBackColor, true);
        BackColor = Color.Transparent;
        Cursor = Cursors.Hand;
    }

    protected override void OnMouseEnter(EventArgs e) { _hot = true; Invalidate(); base.OnMouseEnter(e); }
    protected override void OnMouseLeave(EventArgs e) { _hot = false; Invalidate(); base.OnMouseLeave(e); }
    protected override void OnClick(EventArgs e) { Activated?.Invoke(this, EventArgs.Empty); base.OnClick(e); }

    /// <summary>把「📈 数据看板」拆成图标和文字两段</summary>
    internal static (string icon, string label) SplitIcon(string text)
    {
        var i = text.IndexOf(' ');
        if (i <= 0) return ("", text);
        var head = text[..i];
        // 开头是符号/emoji（U+2000 以上，含代理对）才当作图标
        if (head.Length == 0 || head[0] < '\u2000') return ("", text);
        return (head, text[(i + 1)..].TrimStart());
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        var g = e.Graphics;
        g.TextRenderingHint = System.Drawing.Text.TextRenderingHint.ClearTypeGridFit;

        // 背景：选中 > 悬停
        if (Active || _hot)
        {
            using var b = new SolidBrush(Active ? Theme.Active : Theme.Hover);
            g.FillRectangle(b, ClientRectangle);
        }

        var indent = Level == 0 ? 20 : 40;
        var size = Level == 0 ? 10f : 9.5f;
        using var font = Theme.UiFont(size, Active ? FontStyle.Bold : FontStyle.Regular);
        using var brush = new SolidBrush(Active ? Theme.MenuActive : Theme.MenuText);
        using var sf = new StringFormat { LineAlignment = StringAlignment.Center, Trimming = StringTrimming.EllipsisCharacter };

        // 图标：优先用内置彩色矢量图标（不依赖字体），否则退回 emoji 位图
        var (icon, label) = SplitIcon(Text ?? "");
        var x = (float)indent;
        var avail = (float)(Width - indent - 24);
        var iconSize = Level == 0 ? 18f : 16f;

        if (ColorIcons.Has(IconKey))
        {
            ColorIcons.Draw(g, IconKey, new RectangleF(x, (Height - iconSize) / 2f, iconSize, iconSize));
            x += iconSize + 8;
            avail -= iconSize + 8;
        }
        else
        {
            // 没登记图标：用 GDI+ 画一个彩色兜底块（不再依赖 WPF 渲染 emoji）
            ColorIcons.DrawFallback(g,
                new RectangleF(x, (Height - iconSize) / 2f, iconSize, iconSize),
                IconKey, label);
            x += iconSize + 8;
            avail -= iconSize + 8;
        }
        if (label.Length > 0)
            g.DrawString(label, font, brush, new RectangleF(x, 0, Math.Max(10, avail), Height), sf);

        if (IsGroup)
        {
            // 折叠箭头（网页里是 el-sub-menu 的 caret）
            var cx = Width - 18f;
            var cy = Height / 2f;
            using var pen = new Pen(Theme.MenuText, 1.4f);
            if (Expanded)
                g.DrawLines(pen, new[] { new PointF(cx - 4, cy - 2), new PointF(cx, cy + 2), new PointF(cx + 4, cy - 2) });
            else
                g.DrawLines(pen, new[] { new PointF(cx - 2, cy - 4), new PointF(cx + 2, cy), new PointF(cx - 2, cy + 4) });
        }
    }
}

internal sealed record NavNode(string Label, string Route, NavNode[]? Children = null)
{
    public bool IsGroup => Children is { Length: > 0 };
}
