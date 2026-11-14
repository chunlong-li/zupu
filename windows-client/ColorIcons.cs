using System.Drawing.Drawing2D;

namespace ZupuClient;

/// <summary>
/// 侧栏彩色矢量图标。不依赖任何字体/emoji，画出来就是彩色，
/// 所以精简版系统也不会缺字或变成黑白。
/// 每个图标画在给定的正方形里（内部按 0..1 归一化坐标）。
/// </summary>
internal static class ColorIcons
{
    // 调色板（取自网页 emoji 的主色）
    static readonly Color Amber = ColorTranslator.FromHtml("#E0A33E");
    static readonly Color Red = ColorTranslator.FromHtml("#D9614C");
    static readonly Color Green = ColorTranslator.FromHtml("#5FA25E");
    static readonly Color Blue = ColorTranslator.FromHtml("#4E8FC7");
    static readonly Color Purple = ColorTranslator.FromHtml("#8C6BB1");
    static readonly Color Brown = ColorTranslator.FromHtml("#A9744F");
    static readonly Color Gray = ColorTranslator.FromHtml("#9C9488");
    static readonly Color Gold = ColorTranslator.FromHtml("#D2A93B");
    static readonly Color Cream = ColorTranslator.FromHtml("#F2E7CE");
    static readonly Color Ink = ColorTranslator.FromHtml("#4A4034");

    private static RectangleF N(RectangleF b, float x, float y, float w, float h)
        => new(b.X + b.Width * x, b.Y + b.Height * y, b.Width * w, b.Height * h);

    private static SolidBrush B(Color c) => new(c);

    public static bool Has(string? key) => key is not null && Map.ContainsKey(key);

    public static void Draw(Graphics g, string? key, RectangleF box)
    {
        if (key is null || !Map.TryGetValue(key, out var draw)) return;
        var old = g.SmoothingMode;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        try { draw(g, box); }
        finally { g.SmoothingMode = old; }
    }

    // ==================== 图标注册表 ====================

    private static readonly Dictionary<string, Action<Graphics, RectangleF>> Map = new()
    {
        // 数据看板
        ["group:📊 数据看板"] = BarChart,
        ["/dashboard"] = LineChart,
        ["/birthday"] = Cake,
        ["/deathday"] = Candle,
        ["/traditions"] = (g, b) => Calendar(g, b, Red),
        ["/heatmap"] = (g, b) => MapIcon(g, b, Green),
        ["/migrations"] = Walker,
        ["/contacts"] = AddressBook,

        // 世系可视化
        ["group:🌳 世系可视化"] = (g, b) => Tree(g, b, Green),
        ["/tree"] = Pagoda,
        ["/sutree"] = Chime,
        ["/susi"] = Web,
        ["/suzhu"] = Beads,
        ["/gentree"] = Ladder,
        ["/demo-dieji"] = (g, b) => Scroll(g, b, Amber),
        ["/demo-dieji-land"] = (g, b) => Scroll(g, b, Brown),
        ["/cover-design"] = CoverBook,
        ["/test-zupu"] = Pine,

        // 常用
        ["/members"] = People,
        ["/branches"] = House,
        ["/media"] = Picture,
        ["/spouses-list"] = Ring,
        ["/generation"] = (g, b) => Scroll(g, b, Gold),

        // 谱头文化 / 祠墓
        ["group:🏛️ 谱头文化"] = Columns,
        ["/culture/events"] = (g, b) => Calendar(g, b, Purple),
        ["/culture/articles"] = PenIcon,
        ["/culture/bios"] = Person,
        ["group:🏮 祠墓"] = Shrine,
        ["/hall"] = Shrine,
        ["/shrines"] = Building,
        ["/graves"] = Tomb,

        // 更多
        ["/knowledge"] = Books,
        ["/accounts"] = MoneyBag,
        ["/merit"] = Tomb,
        ["/ai-assistant"] = Robot,
        ["/review"] = Check,

        // 系统设置（网页）
        ["group:⚙️ 系统设置"] = (g, b) => Gear(g, b, Gray),
        ["/settings"] = Tag,
        ["/data-check"] = Pulse,
        ["/backup"] = Floppy,
        ["/update"] = ArrowUp,
        ["/server-settings"] = Monitor,
        ["/map-offline"] = (g, b) => MapIcon(g, b, Amber),
        ["/usage-guide"] = OpenBook,
        ["/about"] = Info,

        // 客户端设置（原生）
        ["group:⚙️ 设置"] = (g, b) => Gear(g, b, Brown),
        [ClientSettings.Prefix + "datadir"] = Folder,
        [ClientSettings.Prefix + "cache"] = Broom,
        [ClientSettings.Prefix + "about"] = Info,
        [ClientSettings.Prefix + "update"] = Download,
        [ClientSettings.Prefix + "server"] = ServerBox,
        [ClientSettings.Prefix + "startup"] = Rocket,
        [ClientSettings.Prefix + "close"] = (g, b) => Power(g, b, Gray),
        [ClientSettings.Prefix + "settings"] = (g, b) => Gear(g, b, Brown),
    };

    // ==================== 具体图形 ====================

    private static void BarChart(Graphics g, RectangleF b)
    {
        using var br = B(Amber);
        g.FillRectangle(br, N(b, 0.18f, 0.52f, 0.18f, 0.34f));
        g.FillRectangle(br, N(b, 0.41f, 0.34f, 0.18f, 0.52f));
        g.FillRectangle(br, N(b, 0.64f, 0.20f, 0.18f, 0.66f));
        using var pen = new Pen(Amber, Math.Max(1f, b.Width * 0.08f));
        g.DrawLine(pen, N(b, 0.12f, 0.88f, 0.76f, 0f).Left, N(b, 0.12f, 0.88f, 0f, 0f).Top,
                        N(b, 0.88f, 0.88f, 0f, 0f).Left, N(b, 0.88f, 0.88f, 0f, 0f).Top);
    }

    private static void LineChart(Graphics g, RectangleF b)
    {
        using var pen = new Pen(Amber, Math.Max(1.4f, b.Width * 0.10f)) { LineJoin = LineJoin.Round };
        g.DrawLines(pen, new[]
        {
            N(b, 0.12f, 0.74f, 0f, 0f).Location,
            N(b, 0.34f, 0.50f, 0f, 0f).Location,
            N(b, 0.54f, 0.62f, 0f, 0f).Location,
            N(b, 0.86f, 0.24f, 0f, 0f).Location,
        });
        using var br = B(Amber);
        foreach (var p in new[] { (0.12f, 0.74f), (0.34f, 0.50f), (0.54f, 0.62f), (0.86f, 0.24f) })
        {
            var r = N(b, p.Item1 - 0.05f, p.Item2 - 0.05f, 0.10f, 0.10f);
            g.FillEllipse(br, r);
        }
    }

    private static void Cake(Graphics g, RectangleF b)
    {
        using var body = B(ColorTranslator.FromHtml("#C97B5A"));
        g.FillRectangle(body, N(b, 0.14f, 0.55f, 0.72f, 0.32f));
        using var cream = B(Cream);
        g.FillRectangle(cream, N(b, 0.14f, 0.47f, 0.72f, 0.12f));
        using var candle = B(Blue);
        g.FillRectangle(candle, N(b, 0.33f, 0.26f, 0.07f, 0.22f));
        g.FillRectangle(candle, N(b, 0.60f, 0.26f, 0.07f, 0.22f));
        using var flame = B(Gold);
        g.FillEllipse(flame, N(b, 0.30f, 0.16f, 0.13f, 0.14f));
        g.FillEllipse(flame, N(b, 0.57f, 0.16f, 0.13f, 0.14f));
    }

    private static void Candle(Graphics g, RectangleF b)
    {
        using var body = B(Cream);
        g.FillRectangle(body, N(b, 0.34f, 0.40f, 0.32f, 0.48f));
        using var flame = B(Amber);
        g.FillEllipse(flame, N(b, 0.36f, 0.14f, 0.28f, 0.28f));
        using var wick = new Pen(Ink, Math.Max(1f, b.Width * 0.06f));
        g.DrawLine(wick, N(b, 0.50f, 0.36f, 0f, 0f).Location, N(b, 0.50f, 0.42f, 0f, 0f).Location);
    }

    private static void Calendar(Graphics g, RectangleF b, Color accent)
    {
        using var body = B(Color.White);
        g.FillRectangle(body, N(b, 0.12f, 0.20f, 0.76f, 0.68f));
        using var head = B(accent);
        g.FillRectangle(head, N(b, 0.12f, 0.20f, 0.76f, 0.22f));
        using var pen = new Pen(ColorTranslator.FromHtml("#C9C2B6"), Math.Max(1f, b.Width * 0.05f));
        g.DrawRectangle(pen, N(b, 0.12f, 0.20f, 0.76f, 0.68f).X, N(b, 0.12f, 0.20f, 0.76f, 0.68f).Y,
                             N(b, 0.12f, 0.20f, 0.76f, 0.68f).Width - 1, N(b, 0.12f, 0.20f, 0.76f, 0.68f).Height - 1);
        using var dot = B(accent);
        for (var i = 0; i < 3; i++)
            for (var j = 0; j < 2; j++)
                g.FillRectangle(dot, N(b, 0.22f + i * 0.22f, 0.54f + j * 0.16f, 0.10f, 0.09f));
    }

    private static void MapIcon(Graphics g, RectangleF b, Color accent)
    {
        using var body = B(ColorTranslator.FromHtml("#EDE3CF"));
        var pts = new[]
        {
            N(b, 0.10f, 0.24f, 0f, 0f).Location, N(b, 0.36f, 0.16f, 0f, 0f).Location,
            N(b, 0.64f, 0.26f, 0f, 0f).Location, N(b, 0.90f, 0.18f, 0f, 0f).Location,
            N(b, 0.90f, 0.78f, 0f, 0f).Location, N(b, 0.64f, 0.86f, 0f, 0f).Location,
            N(b, 0.36f, 0.76f, 0f, 0f).Location, N(b, 0.10f, 0.84f, 0f, 0f).Location,
        };
        g.FillPolygon(body, pts);
        using var pen = new Pen(accent, Math.Max(1.2f, b.Width * 0.07f));
        g.DrawLines(pen, pts);
        g.DrawLine(pen, N(b, 0.36f, 0.16f, 0f, 0f).Location, N(b, 0.36f, 0.76f, 0f, 0f).Location);
        g.DrawLine(pen, N(b, 0.64f, 0.26f, 0f, 0f).Location, N(b, 0.64f, 0.86f, 0f, 0f).Location);
    }

    private static void Walker(Graphics g, RectangleF b)
    {
        using var br = B(Blue);
        g.FillEllipse(br, N(b, 0.38f, 0.10f, 0.22f, 0.22f));
        using var pen = new Pen(Blue, Math.Max(1.6f, b.Width * 0.11f)) { StartCap = LineCap.Round, EndCap = LineCap.Round };
        g.DrawLine(pen, N(b, 0.48f, 0.34f, 0f, 0f).Location, N(b, 0.46f, 0.60f, 0f, 0f).Location);
        g.DrawLine(pen, N(b, 0.46f, 0.60f, 0f, 0f).Location, N(b, 0.30f, 0.86f, 0f, 0f).Location);
        g.DrawLine(pen, N(b, 0.46f, 0.60f, 0f, 0f).Location, N(b, 0.66f, 0.84f, 0f, 0f).Location);
        g.DrawLine(pen, N(b, 0.47f, 0.42f, 0f, 0f).Location, N(b, 0.72f, 0.34f, 0f, 0f).Location);
    }

    private static void AddressBook(Graphics g, RectangleF b)
    {
        using var cover = B(Brown);
        g.FillRectangle(cover, N(b, 0.16f, 0.14f, 0.64f, 0.74f));
        using var page = B(Cream);
        g.FillRectangle(page, N(b, 0.26f, 0.20f, 0.46f, 0.62f));
        using var line = new Pen(Gray, Math.Max(1f, b.Width * 0.05f));
        for (var i = 0; i < 3; i++)
            g.DrawLine(line, N(b, 0.32f, 0.32f + i * 0.16f, 0f, 0f).Location, N(b, 0.66f, 0.32f + i * 0.16f, 0f, 0f).Location);
    }

    private static void Tree(Graphics g, RectangleF b, Color leaf)
    {
        using var trunk = B(Brown);
        g.FillRectangle(trunk, N(b, 0.45f, 0.55f, 0.10f, 0.34f));
        using var br = B(leaf);
        g.FillEllipse(br, N(b, 0.26f, 0.12f, 0.48f, 0.46f));
        g.FillEllipse(br, N(b, 0.10f, 0.24f, 0.34f, 0.34f));
        g.FillEllipse(br, N(b, 0.56f, 0.24f, 0.34f, 0.34f));
    }

    private static void Pine(Graphics g, RectangleF b)
    {
        using var trunk = B(Brown);
        g.FillRectangle(trunk, N(b, 0.45f, 0.72f, 0.10f, 0.18f));
        using var br = B(Green);
        g.FillPolygon(br, new[]
        {
            N(b, 0.50f, 0.10f, 0f, 0f).Location,
            N(b, 0.22f, 0.44f, 0f, 0f).Location,
            N(b, 0.78f, 0.44f, 0f, 0f).Location,
        });
        g.FillPolygon(br, new[]
        {
            N(b, 0.50f, 0.30f, 0f, 0f).Location,
            N(b, 0.14f, 0.74f, 0f, 0f).Location,
            N(b, 0.86f, 0.74f, 0f, 0f).Location,
        });
    }

    private static void Pagoda(Graphics g, RectangleF b)
    {
        using var roof = B(Red);
        using var body = B(Cream);
        for (var i = 0; i < 3; i++)
        {
            var y = 0.22f + i * 0.24f;
            g.FillPolygon(roof, new[]
            {
                N(b, 0.50f, y - 0.10f, 0f, 0f).Location,
                N(b, 0.12f, y + 0.10f, 0f, 0f).Location,
                N(b, 0.88f, y + 0.10f, 0f, 0f).Location,
            });
            g.FillRectangle(body, N(b, 0.30f, y + 0.10f, 0.40f, 0.14f));
        }
    }

    private static void Chime(Graphics g, RectangleF b)
    {
        using var pen = new Pen(Gray, Math.Max(1f, b.Width * 0.06f));
        g.DrawLine(pen, N(b, 0.16f, 0.18f, 0f, 0f).Location, N(b, 0.84f, 0.18f, 0f, 0f).Location);
        using var br = B(Green);
        foreach (var x in new[] { 0.30f, 0.50f, 0.70f })
        {
            g.DrawLine(pen, N(b, x, 0.18f, 0f, 0f).Location, N(b, x, 0.42f, 0f, 0f).Location);
            g.FillPolygon(br, new[]
            {
                N(b, x, 0.42f, 0f, 0f).Location,
                N(b, x - 0.09f, 0.64f, 0f, 0f).Location,
                N(b, x + 0.09f, 0.64f, 0f, 0f).Location,
            });
        }
    }

    private static void Web(Graphics g, RectangleF b)
    {
        using var pen = new Pen(Gray, Math.Max(0.9f, b.Width * 0.05f));
        var c = N(b, 0.50f, 0.50f, 0f, 0f).Location;
        for (var i = 0; i < 8; i++)
        {
            var a = Math.PI * 2 * i / 8;
            g.DrawLine(pen, c, new PointF(c.X + (float)(Math.Cos(a) * b.Width * 0.40f),
                                             c.Y + (float)(Math.Sin(a) * b.Width * 0.40f)));
        }
        for (var r = 0.16f; r < 0.42f; r += 0.13f)
        {
            var rect = N(b, 0.50f - r, 0.50f - r, r * 2, r * 2);
            g.DrawEllipse(pen, rect);
        }
    }

    private static void Beads(Graphics g, RectangleF b)
    {
        using var br = B(Brown);
        var c = N(b, 0.50f, 0.20f, 0f, 0f).Location;
        using var pen = new Pen(Gray, Math.Max(0.9f, b.Width * 0.05f));
        g.DrawArc(pen, N(b, 0.14f, 0.14f, 0.72f, 0.72f), 200, 140);
        for (var i = 0; i < 5; i++)
        {
            var a = Math.PI * (200 + i * 35) / 180;
            var r = b.Width * 0.34f;
            g.FillEllipse(br, new RectangleF(
                c.X + 0.24f * b.Width + (float)(Math.Cos(a) * r) - b.Width * 0.06f,
                c.Y + 0.33f * b.Width + (float)(Math.Sin(a) * r) - b.Width * 0.06f,
                b.Width * 0.12f, b.Width * 0.12f));
        }
    }

    private static void Ladder(Graphics g, RectangleF b)
    {
        using var pen = new Pen(Brown, Math.Max(1.4f, b.Width * 0.08f));
        g.DrawLine(pen, N(b, 0.30f, 0.10f, 0f, 0f).Location, N(b, 0.30f, 0.90f, 0f, 0f).Location);
        g.DrawLine(pen, N(b, 0.70f, 0.10f, 0f, 0f).Location, N(b, 0.70f, 0.90f, 0f, 0f).Location);
        for (var i = 0; i < 3; i++)
            g.DrawLine(pen, N(b, 0.30f, 0.30f + i * 0.22f, 0f, 0f).Location, N(b, 0.70f, 0.30f + i * 0.22f, 0f, 0f).Location);
    }

    private static void Scroll(Graphics g, RectangleF b, Color accent)
    {
        using var paper = B(Cream);
        g.FillRectangle(paper, N(b, 0.18f, 0.14f, 0.64f, 0.72f));
        using var edge = B(accent);
        g.FillRectangle(edge, N(b, 0.10f, 0.10f, 0.80f, 0.10f));
        g.FillRectangle(edge, N(b, 0.10f, 0.80f, 0.80f, 0.10f));
        using var line = new Pen(Gray, Math.Max(0.9f, b.Width * 0.05f));
        for (var i = 0; i < 3; i++)
            g.DrawLine(line, N(b, 0.28f, 0.34f + i * 0.16f, 0f, 0f).Location, N(b, 0.72f, 0.34f + i * 0.16f, 0f, 0f).Location);
    }

    private static void CoverBook(Graphics g, RectangleF b)
    {
        using var cover = B(Red);
        g.FillRectangle(cover, N(b, 0.18f, 0.10f, 0.64f, 0.80f));
        using var page = B(Cream);
        g.FillRectangle(page, N(b, 0.26f, 0.16f, 0.48f, 0.68f));
        using var pen = new Pen(Gold, Math.Max(1f, b.Width * 0.05f));
        g.DrawRectangle(pen, N(b, 0.18f, 0.10f, 0.64f, 0.80f).X, N(b, 0.18f, 0.10f, 0.64f, 0.80f).Y,
                             N(b, 0.18f, 0.10f, 0.64f, 0.80f).Width, N(b, 0.18f, 0.10f, 0.64f, 0.80f).Height);
    }

    private static void People(Graphics g, RectangleF b)
    {
        using var br = B(Blue);
        g.FillEllipse(br, N(b, 0.16f, 0.20f, 0.26f, 0.26f));
        g.FillEllipse(br, N(b, 0.56f, 0.16f, 0.28f, 0.28f));
        g.FillEllipse(br, N(b, 0.08f, 0.50f, 0.44f, 0.40f));
        using var br2 = B(ColorTranslator.FromHtml("#6BA6D6"));
        g.FillEllipse(br2, N(b, 0.50f, 0.46f, 0.44f, 0.40f));
    }

    private static void Person(Graphics g, RectangleF b)
    {
        using var br = B(Blue);
        g.FillEllipse(br, N(b, 0.34f, 0.12f, 0.32f, 0.32f));
        g.FillEllipse(br, N(b, 0.20f, 0.50f, 0.60f, 0.42f));
    }

    private static void House(Graphics g, RectangleF b)
    {
        using var roof = B(Red);
        g.FillPolygon(roof, new[]
        {
            N(b, 0.50f, 0.12f, 0f, 0f).Location,
            N(b, 0.94f, 0.46f, 0f, 0f).Location,
            N(b, 0.06f, 0.46f, 0f, 0f).Location,
        });
        using var body = B(Amber);
        g.FillRectangle(body, N(b, 0.20f, 0.46f, 0.60f, 0.42f));
        using var door = B(Brown);
        g.FillRectangle(door, N(b, 0.42f, 0.62f, 0.16f, 0.26f));
    }

    private static void Picture(Graphics g, RectangleF b)
    {
        using var frame = B(Blue);
        g.FillRectangle(frame, N(b, 0.10f, 0.18f, 0.80f, 0.64f));
        using var sky = B(ColorTranslator.FromHtml("#DCEBF7"));
        g.FillRectangle(sky, N(b, 0.18f, 0.26f, 0.64f, 0.48f));
        using var hill = B(Green);
        g.FillPolygon(hill, new[]
        {
            N(b, 0.18f, 0.74f, 0f, 0f).Location,
            N(b, 0.42f, 0.48f, 0f, 0f).Location,
            N(b, 0.64f, 0.74f, 0f, 0f).Location,
        });
        using var sun = B(Gold);
        g.FillEllipse(sun, N(b, 0.62f, 0.32f, 0.12f, 0.12f));
    }

    private static void Ring(Graphics g, RectangleF b)
    {
        using var pen = new Pen(Gold, Math.Max(1.6f, b.Width * 0.10f));
        g.DrawEllipse(pen, N(b, 0.22f, 0.40f, 0.56f, 0.46f));
        using var gem = B(Blue);
        g.FillPolygon(gem, new[]
        {
            N(b, 0.50f, 0.10f, 0f, 0f).Location,
            N(b, 0.64f, 0.26f, 0f, 0f).Location,
            N(b, 0.50f, 0.42f, 0f, 0f).Location,
            N(b, 0.36f, 0.26f, 0f, 0f).Location,
        });
    }

    private static void Columns(Graphics g, RectangleF b)
    {
        using var roof = B(Blue);
        g.FillPolygon(roof, new[]
        {
            N(b, 0.50f, 0.08f, 0f, 0f).Location,
            N(b, 0.96f, 0.30f, 0f, 0f).Location,
            N(b, 0.04f, 0.30f, 0f, 0f).Location,
        });
        using var body = B(Cream);
        g.FillRectangle(body, N(b, 0.12f, 0.30f, 0.76f, 0.10f));
        foreach (var x in new[] { 0.22f, 0.44f, 0.66f })
            g.FillRectangle(body, N(b, x, 0.40f, 0.12f, 0.40f));
        g.FillRectangle(body, N(b, 0.12f, 0.82f, 0.76f, 0.08f));
    }

    private static void PenIcon(Graphics g, RectangleF b)
    {
        // 笔杆
        using var body = B(Ink);
        g.FillPolygon(body, new[]
        {
            N(b, 0.78f, 0.10f, 0f, 0f).Location,
            N(b, 0.92f, 0.24f, 0f, 0f).Location,
            N(b, 0.34f, 0.82f, 0f, 0f).Location,
            N(b, 0.20f, 0.68f, 0f, 0f).Location,
        });
        // 笔尖
        using var nib = B(Gold);
        g.FillPolygon(nib, new[]
        {
            N(b, 0.20f, 0.68f, 0f, 0f).Location,
            N(b, 0.34f, 0.82f, 0f, 0f).Location,
            N(b, 0.10f, 0.90f, 0f, 0f).Location,
        });
    }

    private static void Shrine(Graphics g, RectangleF b)
    {
        using var roof = B(Red);
        g.FillPolygon(roof, new[]
        {
            N(b, 0.50f, 0.10f, 0f, 0f).Location,
            N(b, 0.96f, 0.40f, 0f, 0f).Location,
            N(b, 0.04f, 0.40f, 0f, 0f).Location,
        });
        using var body = B(ColorTranslator.FromHtml("#EBD9C0"));
        g.FillRectangle(body, N(b, 0.18f, 0.40f, 0.64f, 0.42f));
        using var door = B(Brown);
        g.FillRectangle(door, N(b, 0.42f, 0.56f, 0.16f, 0.26f));
    }

    private static void Building(Graphics g, RectangleF b)
    {
        using var body = B(ColorTranslator.FromHtml("#E9DECA"));
        g.FillRectangle(body, N(b, 0.20f, 0.34f, 0.60f, 0.52f));
        using var roof = B(Red);
        g.FillPolygon(roof, new[]
        {
            N(b, 0.50f, 0.10f, 0f, 0f).Location,
            N(b, 0.90f, 0.34f, 0f, 0f).Location,
            N(b, 0.10f, 0.34f, 0f, 0f).Location,
        });
        using var win = B(Blue);
        g.FillRectangle(win, N(b, 0.30f, 0.48f, 0.14f, 0.16f));
        g.FillRectangle(win, N(b, 0.56f, 0.48f, 0.14f, 0.16f));
    }

    private static void Tomb(Graphics g, RectangleF b)
    {
        using var stone = B(Gray);
        g.FillRectangle(stone, N(b, 0.28f, 0.28f, 0.44f, 0.56f));
        g.FillEllipse(stone, N(b, 0.28f, 0.14f, 0.44f, 0.28f));
        using var line = new Pen(Color.White, Math.Max(1f, b.Width * 0.06f));
        g.DrawLine(line, N(b, 0.50f, 0.38f, 0f, 0f).Location, N(b, 0.50f, 0.62f, 0f, 0f).Location);
        g.DrawLine(line, N(b, 0.40f, 0.46f, 0f, 0f).Location, N(b, 0.60f, 0.46f, 0f, 0f).Location);
    }

    private static void Books(Graphics g, RectangleF b)
    {
        using var b1 = B(Brown);
        using var b2 = B(Green);
        using var b3 = B(Blue);
        g.FillRectangle(b1, N(b, 0.14f, 0.62f, 0.72f, 0.16f));
        g.FillRectangle(b2, N(b, 0.18f, 0.44f, 0.64f, 0.16f));
        g.FillRectangle(b3, N(b, 0.22f, 0.26f, 0.56f, 0.16f));
    }

    private static void MoneyBag(Graphics g, RectangleF b)
    {
        using var bag = B(Gold);
        g.FillEllipse(bag, N(b, 0.14f, 0.32f, 0.72f, 0.58f));
        using var neck = B(Brown);
        g.FillRectangle(neck, N(b, 0.36f, 0.18f, 0.28f, 0.18f));
        using var sign = new Pen(Ink, Math.Max(1f, b.Width * 0.06f));
        g.DrawLine(sign, N(b, 0.50f, 0.48f, 0f, 0f).Location, N(b, 0.50f, 0.74f, 0f, 0f).Location);
        g.DrawLine(sign, N(b, 0.40f, 0.56f, 0f, 0f).Location, N(b, 0.60f, 0.56f, 0f, 0f).Location);
    }

    private static void Robot(Graphics g, RectangleF b)
    {
        using var body = B(ColorTranslator.FromHtml("#B9C2CC"));
        g.FillRectangle(body, N(b, 0.18f, 0.30f, 0.64f, 0.46f));
        using var head = B(ColorTranslator.FromHtml("#8FA0B0"));
        g.FillRectangle(head, N(b, 0.26f, 0.10f, 0.48f, 0.20f));
        using var eye = B(Ink);
        g.FillEllipse(eye, N(b, 0.34f, 0.16f, 0.08f, 0.08f));
        g.FillEllipse(eye, N(b, 0.58f, 0.16f, 0.08f, 0.08f));
        using var line = new Pen(Ink, Math.Max(0.9f, b.Width * 0.05f));
        for (var i = 0; i < 3; i++)
            g.DrawLine(line, N(b, 0.32f, 0.44f + i * 0.10f, 0f, 0f).Location, N(b, 0.68f, 0.44f + i * 0.10f, 0f, 0f).Location);
    }

    private static void Check(Graphics g, RectangleF b)
    {
        using var circle = B(Green);
        g.FillEllipse(circle, N(b, 0.08f, 0.08f, 0.84f, 0.84f));
        using var pen = new Pen(Color.White, Math.Max(1.8f, b.Width * 0.13f))
        { StartCap = LineCap.Round, EndCap = LineCap.Round, LineJoin = LineJoin.Round };
        g.DrawLines(pen, new[]
        {
            N(b, 0.26f, 0.52f, 0f, 0f).Location,
            N(b, 0.44f, 0.70f, 0f, 0f).Location,
            N(b, 0.74f, 0.32f, 0f, 0f).Location,
        });
    }


    /// <summary>没登记图标时的兜底：按 key 取稳定颜色，画圆角块 + 首字（GDI+，纯彩色，不依赖 WPF）</summary>
    public static void DrawFallback(Graphics g, RectangleF b, string? key, string label)
    {
        var palette = new[] { Amber, Red, Green, Blue, Purple, Brown, Gold, Ink };
        var h = 7;
        foreach (var ch in key ?? "") h = (h * 31 + ch) & 0x7fffffff;
        var color = palette[h % palette.Length];

        g.SmoothingMode = SmoothingMode.AntiAlias;
        var r = b.Width * 0.24f;
        using var path = new GraphicsPath();
        path.AddArc(b.X, b.Y, r * 2, r * 2, 180, 90);
        path.AddArc(b.Right - r * 2, b.Y, r * 2, r * 2, 270, 90);
        path.AddArc(b.Right - r * 2, b.Bottom - r * 2, r * 2, r * 2, 0, 90);
        path.AddArc(b.X, b.Bottom - r * 2, r * 2, r * 2, 90, 90);
        path.CloseFigure();

        using (var bg = new SolidBrush(Color.FromArgb(235, color)))
            g.FillPath(bg, path);

        var ch0 = string.IsNullOrWhiteSpace(label) ? "?" : label.Trim().Substring(0, 1);
        using var font = new Font("Microsoft YaHei UI", Math.Max(7f, b.Height * 0.62f), FontStyle.Bold, GraphicsUnit.Pixel);
        using var white = new SolidBrush(Color.White);
        using var sf = new StringFormat { Alignment = StringAlignment.Center, LineAlignment = StringAlignment.Center };
        g.DrawString(ch0, font, white, b, sf);
    }

    private static void Power(Graphics g, RectangleF b, Color color)
    {
        using var pen = new Pen(color, Math.Max(1.6f, b.Width * 0.12f))
        {
            StartCap = LineCap.Round,
            EndCap = LineCap.Round,
        };
        var r = new RectangleF(b.X + b.Width * 0.17f, b.Y + b.Height * 0.19f, b.Width * 0.66f, b.Height * 0.66f);
        g.DrawArc(pen, r, -55, 290);
        g.DrawLine(pen,
            b.X + b.Width * 0.5f, b.Y + b.Height * 0.08f,
            b.X + b.Width * 0.5f, b.Y + b.Height * 0.46f);
    }

    private static void Gear(Graphics g, RectangleF b, Color color)
    {
        using var br = B(color);
        var c = N(b, 0.50f, 0.50f, 0f, 0f).Location;
        for (var i = 0; i < 8; i++)
        {
            var a = Math.PI * 2 * i / 8;
            var x = c.X + (float)Math.Cos(a) * b.Width * 0.34f;
            var y = c.Y + (float)Math.Sin(a) * b.Width * 0.34f;
            g.FillEllipse(br, new RectangleF(x - b.Width * 0.10f, y - b.Width * 0.10f, b.Width * 0.20f, b.Width * 0.20f));
        }
        g.FillEllipse(br, N(b, 0.20f, 0.20f, 0.60f, 0.60f));
        using var hole = B(ColorTranslator.FromHtml("#5C5449"));
        g.FillEllipse(hole, N(b, 0.38f, 0.38f, 0.24f, 0.24f));
    }

    private static void Tag(Graphics g, RectangleF b)
    {
        using var br = B(Amber);
        g.FillPolygon(br, new[]
        {
            N(b, 0.20f, 0.20f, 0f, 0f).Location,
            N(b, 0.58f, 0.20f, 0f, 0f).Location,
            N(b, 0.86f, 0.50f, 0f, 0f).Location,
            N(b, 0.58f, 0.80f, 0f, 0f).Location,
            N(b, 0.20f, 0.80f, 0f, 0f).Location,
        });
        using var hole = B(Color.White);
        g.FillEllipse(hole, N(b, 0.30f, 0.42f, 0.14f, 0.14f));
    }

    private static void Pulse(Graphics g, RectangleF b)
    {
        using var br = B(Red);
        g.FillEllipse(br, N(b, 0.34f, 0.30f, 0.34f, 0.34f));
        using var pen = new Pen(Red, Math.Max(1.5f, b.Width * 0.09f)) { LineJoin = LineJoin.Round };
        g.DrawLines(pen, new[]
        {
            N(b, 0.10f, 0.52f, 0f, 0f).Location,
            N(b, 0.34f, 0.52f, 0f, 0f).Location,
            N(b, 0.44f, 0.34f, 0f, 0f).Location,
            N(b, 0.56f, 0.70f, 0f, 0f).Location,
            N(b, 0.66f, 0.52f, 0f, 0f).Location,
            N(b, 0.90f, 0.52f, 0f, 0f).Location,
        });
    }

    private static void Floppy(Graphics g, RectangleF b)
    {
        using var body = B(Blue);
        g.FillRectangle(body, N(b, 0.16f, 0.16f, 0.68f, 0.68f));
        using var label = B(Cream);
        g.FillRectangle(label, N(b, 0.30f, 0.50f, 0.40f, 0.30f));
        using var shutter = B(ColorTranslator.FromHtml("#D8DEE4"));
        g.FillRectangle(shutter, N(b, 0.36f, 0.16f, 0.28f, 0.24f));
    }

    private static void ArrowUp(Graphics g, RectangleF b)
    {
        using var br = B(Blue);
        g.FillPolygon(br, new[]
        {
            N(b, 0.50f, 0.12f, 0f, 0f).Location,
            N(b, 0.80f, 0.46f, 0f, 0f).Location,
            N(b, 0.60f, 0.46f, 0f, 0f).Location,
            N(b, 0.60f, 0.86f, 0f, 0f).Location,
            N(b, 0.40f, 0.86f, 0f, 0f).Location,
            N(b, 0.40f, 0.46f, 0f, 0f).Location,
            N(b, 0.20f, 0.46f, 0f, 0f).Location,
        });
    }

    private static void Monitor(Graphics g, RectangleF b)
    {
        using var frame = B(Gray);
        g.FillRectangle(frame, N(b, 0.10f, 0.16f, 0.80f, 0.54f));
        using var screen = B(ColorTranslator.FromHtml("#DCEBF7"));
        g.FillRectangle(screen, N(b, 0.16f, 0.22f, 0.68f, 0.42f));
        using var stand = B(Gray);
        g.FillRectangle(stand, N(b, 0.42f, 0.70f, 0.16f, 0.12f));
        g.FillRectangle(stand, N(b, 0.26f, 0.82f, 0.48f, 0.08f));
    }

    private static void OpenBook(Graphics g, RectangleF b)
    {
        using var page = B(Cream);
        g.FillPolygon(page, new[]
        {
            N(b, 0.08f, 0.26f, 0f, 0f).Location,
            N(b, 0.50f, 0.20f, 0f, 0f).Location,
            N(b, 0.50f, 0.80f, 0f, 0f).Location,
            N(b, 0.08f, 0.86f, 0f, 0f).Location,
        });
        g.FillPolygon(page, new[]
        {
            N(b, 0.92f, 0.26f, 0f, 0f).Location,
            N(b, 0.50f, 0.20f, 0f, 0f).Location,
            N(b, 0.50f, 0.80f, 0f, 0f).Location,
            N(b, 0.92f, 0.86f, 0f, 0f).Location,
        });
        using var pen = new Pen(Blue, Math.Max(1f, b.Width * 0.06f));
        g.DrawPolygon(pen, new[]
        {
            N(b, 0.08f, 0.26f, 0f, 0f).Location, N(b, 0.50f, 0.20f, 0f, 0f).Location,
            N(b, 0.50f, 0.80f, 0f, 0f).Location, N(b, 0.08f, 0.86f, 0f, 0f).Location,
        });
        g.DrawPolygon(pen, new[]
        {
            N(b, 0.92f, 0.26f, 0f, 0f).Location, N(b, 0.50f, 0.20f, 0f, 0f).Location,
            N(b, 0.50f, 0.80f, 0f, 0f).Location, N(b, 0.92f, 0.86f, 0f, 0f).Location,
        });
    }

    private static void Info(Graphics g, RectangleF b)
    {
        using var br = B(Blue);
        g.FillEllipse(br, N(b, 0.08f, 0.08f, 0.84f, 0.84f));
        using var f = new Font("Microsoft YaHei UI", Math.Max(6f, b.Width * 0.62f), FontStyle.Bold, GraphicsUnit.Pixel);
        using var sf = new StringFormat { Alignment = StringAlignment.Center, LineAlignment = StringAlignment.Center };
        g.DrawString("i", f, Brushes.White, new RectangleF(b.X, b.Y - b.Height * 0.02f, b.Width, b.Height), sf);
    }

    private static void Folder(Graphics g, RectangleF b)
    {
        using var back = B(ColorTranslator.FromHtml("#E8B75C"));
        g.FillRectangle(back, N(b, 0.10f, 0.24f, 0.36f, 0.16f));
        using var front = B(Amber);
        g.FillRectangle(front, N(b, 0.10f, 0.34f, 0.80f, 0.48f));
        using var line = new Pen(Color.FromArgb(120, 255, 255, 255), Math.Max(1f, b.Width * 0.05f));
        g.DrawLine(line, N(b, 0.10f, 0.46f, 0f, 0f).Location, N(b, 0.90f, 0.46f, 0f, 0f).Location);
    }

    private static void Download(Graphics g, RectangleF b)
    {
        using var arrow = B(Green);
        g.FillPolygon(arrow, new[]
        {
            N(b, 0.50f, 0.10f, 0f, 0f).Location,
            N(b, 0.72f, 0.42f, 0f, 0f).Location,
            N(b, 0.58f, 0.42f, 0f, 0f).Location,
            N(b, 0.58f, 0.64f, 0f, 0f).Location,
            N(b, 0.42f, 0.64f, 0f, 0f).Location,
            N(b, 0.42f, 0.42f, 0f, 0f).Location,
            N(b, 0.28f, 0.42f, 0f, 0f).Location,
        });
        using var tray = B(Blue);
        g.FillRectangle(tray, N(b, 0.16f, 0.72f, 0.68f, 0.10f));
        g.FillRectangle(tray, N(b, 0.16f, 0.82f, 0.10f, 0.08f));
        g.FillRectangle(tray, N(b, 0.74f, 0.82f, 0.10f, 0.08f));
    }

    private static void ServerBox(Graphics g, RectangleF b)
    {
        using var body = B(Gray);
        g.FillRectangle(body, N(b, 0.20f, 0.16f, 0.60f, 0.24f));
        g.FillRectangle(body, N(b, 0.20f, 0.46f, 0.60f, 0.24f));
        using var led = B(Green);
        g.FillEllipse(led, N(b, 0.28f, 0.25f, 0.08f, 0.07f));
        g.FillEllipse(led, N(b, 0.28f, 0.55f, 0.08f, 0.07f));
        using var slot = B(ColorTranslator.FromHtml("#E8E4DC"));
        g.FillRectangle(slot, N(b, 0.44f, 0.26f, 0.28f, 0.05f));
        g.FillRectangle(slot, N(b, 0.44f, 0.56f, 0.28f, 0.05f));
        using var baseBar = B(Blue);
        g.FillRectangle(baseBar, N(b, 0.16f, 0.74f, 0.68f, 0.10f));
    }

    private static void Rocket(Graphics g, RectangleF b)
    {
        using var body = B(Red);
        g.FillPolygon(body, new[]
        {
            N(b, 0.50f, 0.08f, 0f, 0f).Location,
            N(b, 0.66f, 0.40f, 0f, 0f).Location,
            N(b, 0.66f, 0.66f, 0f, 0f).Location,
            N(b, 0.34f, 0.66f, 0f, 0f).Location,
            N(b, 0.34f, 0.40f, 0f, 0f).Location,
        });
        using var win = B(Cream);
        g.FillEllipse(win, N(b, 0.44f, 0.30f, 0.12f, 0.14f));
        using var fin = B(Brown);
        g.FillPolygon(fin, new[]
        {
            N(b, 0.34f, 0.60f, 0f, 0f).Location,
            N(b, 0.20f, 0.76f, 0f, 0f).Location,
            N(b, 0.34f, 0.76f, 0f, 0f).Location,
        });
        g.FillPolygon(fin, new[]
        {
            N(b, 0.66f, 0.60f, 0f, 0f).Location,
            N(b, 0.80f, 0.76f, 0f, 0f).Location,
            N(b, 0.66f, 0.76f, 0f, 0f).Location,
        });
        using var flame = B(Gold);
        g.FillPolygon(flame, new[]
        {
            N(b, 0.42f, 0.70f, 0f, 0f).Location,
            N(b, 0.58f, 0.70f, 0f, 0f).Location,
            N(b, 0.50f, 0.90f, 0f, 0f).Location,
        });
    }

    private static void Broom(Graphics g, RectangleF b)
    {
        using var handle = B(Brown);
        g.DrawLine(new Pen(handle, Math.Max(1.6f, b.Width * 0.10f)),
            N(b, 0.34f, 0.12f, 0f, 0f).Location, N(b, 0.48f, 0.56f, 0f, 0f).Location);
        using var bristle = B(Amber);
        g.FillPolygon(bristle, new[]
        {
            N(b, 0.40f, 0.52f, 0f, 0f).Location,
            N(b, 0.70f, 0.44f, 0f, 0f).Location,
            N(b, 0.86f, 0.86f, 0f, 0f).Location,
            N(b, 0.40f, 0.80f, 0f, 0f).Location,
        });
        using var line = new Pen(Color.FromArgb(110, 255, 255, 255), Math.Max(0.9f, b.Width * 0.05f));
        for (var i = 1; i < 3; i++)
            g.DrawLine(line, N(b, 0.48f + i * 0.12f, 0.52f, 0f, 0f).Location, N(b, 0.52f + i * 0.12f, 0.84f, 0f, 0f).Location);
    }
}
