using System.Text.Json;
using CefSharp;
using CefSharp.WinForms;

namespace ZupuClient;

public class MainForm : Form
{
    // ---- 网页 ----
    private readonly ChromiumWebBrowser _web;
    private readonly Panel _content = new() { Dock = DockStyle.Fill, BackColor = Theme.Mibai };
    private bool _injected;

    /// <summary>true = 真的要退出（不是隐藏到托盘），避免关闭时又被拦一次</summary>
    private bool _exiting;
    private NotifyIcon? _tray;

    // ---- 原生外壳 ----
    private readonly Panel _topBar = new() { Dock = DockStyle.Top, Height = Theme.TopBarHeight, BackColor = Theme.Ink };
    private readonly SpacedLabel _logo = new() { Text = "族谱", Width = 120, Height = 34 };
    private readonly BranchPicker _branch = new();
    private readonly FlatCheck _include = new() { Text = "含下级", Width = 74 };
    private readonly AvatarButton _avatar = new();
    private readonly FlatTextButton _logout = new() { Text = "退出" };
    private readonly Panel _centerArea = new();

    private readonly Panel _sidebar = new() { Dock = DockStyle.Left, Width = Theme.SidebarWidth, BackColor = Theme.Ink };
    private readonly FlowLayoutPanel _menuHost = new()
    {
        Dock = DockStyle.Fill,
        FlowDirection = FlowDirection.TopDown,
        WrapContents = false,
        AutoScroll = true,
        BackColor = Theme.Ink,
        Padding = new Padding(0),
    };
    private readonly Label _version = new()
    {
        Dock = DockStyle.Fill,
        TextAlign = ContentAlignment.MiddleCenter,
        ForeColor = Theme.FooterText,
        BackColor = Theme.Ink,
        Font = Theme.UiFont(7.5f),
    };

    private readonly Dictionary<string, MenuRow> _rowsByRoute = new();
    private readonly Dictionary<string, List<MenuRow>> _childrenByGroup = new();
    private readonly Dictionary<string, MenuRow> _groupRows = new();

    private readonly System.Windows.Forms.Timer _routePoll = new() { Interval = 900 };
    private string _currentPath = "";
    private bool _syncing;

    public MainForm()
    {
        Text = "族谱客户端";
        Width = Math.Max(900, Prefs.Current.WinW);
        Height = Math.Max(600, Prefs.Current.WinH);
        BackColor = Theme.Mibai;
        if (Prefs.Current.WinX >= 0)
        {
            StartPosition = FormStartPosition.Manual;
            Location = new Point(Prefs.Current.WinX, Prefs.Current.WinY);
        }
        else
        {
            StartPosition = FormStartPosition.CenterScreen;
        }

        _web = new ChromiumWebBrowser { Dock = DockStyle.Fill };
        _content.Controls.Add(_web);

        BuildTopBar();
        ApplyIdentity();

        // 停靠顺序：最后加的 dock 最先 → 顶栏占满整行，侧栏在其下方
        Controls.Add(_content);
        Controls.Add(_sidebar);
        Controls.Add(_topBar);
        MainMenuStrip = new MenuStrip(); // 保留系统菜单栏占位

        Load += (_, _) => InitWeb();
        FormClosing += (_, e) =>
        {
            // 只有用户点 ✕ 才询问；系统关机/注销、程序自己退出都照常关
            if (!_exiting && e.CloseReason == CloseReason.UserClosing)
            {
                var p = Prefs.Current;
                var action = p.CloseAction == "tray" ? "tray" : "exit";

                if (p.CloseAsk)
                {
                    using var dlg = new ClosePromptDialog(action, false);
                    if (dlg.ShowDialog(this) != DialogResult.OK)
                    {
                        e.Cancel = true;
                        return;
                    }
                    action = dlg.Action;
                    if (dlg.NoMoreAsk)
                    {
                        p.CloseAction = action;
                        p.CloseAsk = false;
                        Prefs.Save();
                    }
                }

                if (action == "tray")
                {
                    e.Cancel = true;
                    HideToTray();
                    return;
                }
                _exiting = true;
            }

            Prefs.Current.WinW = Width; Prefs.Current.WinH = Height;
            Prefs.Current.WinX = Location.X; Prefs.Current.WinY = Location.Y;
            Prefs.Save();
            _routePoll.Stop();
        };
        FormClosed += (_, _) =>
        {
            try { if (_tray is not null) { _tray.Visible = false; _tray.Dispose(); _tray = null; } } catch { }
            try { _web.Dispose(); } catch { /* 关闭阶段忽略 */ }
        };
    }

    // ================= 顶栏 =================

    private void BuildTopBar()
    {
        _centerArea.BackColor = Color.Transparent;
        _centerArea.Controls.Add(_logo);
        _centerArea.Controls.Add(_branch);
        _centerArea.Controls.Add(_include);

        _avatar.Anchor = AnchorStyles.Right;
        _logout.Anchor = AnchorStyles.Right;
        _topBar.Controls.Add(_centerArea);
        _topBar.Controls.Add(_avatar);
        _topBar.Controls.Add(_logout);

        _logo.Click += (_, _) => Navigate("📈 数据看板", "/dashboard");
        _branch.BranchPicked += async (_, id) => await ApplyBranchAsync(id, _include.Checked);
        _include.CheckedChanged += async (_, _) =>
        {
            if (_syncing) return;
            await ApplyBranchAsync(_branch.SelectedId, _include.Checked);
        };
        _logout.Click += (_, _) => Logout();

        _topBar.Resize += (_, _) => LayoutTopBar();
        LayoutTopBar();
    }

    private void LayoutTopBar()
    {
        // 右侧 auth-area：距右 20px（网页 .topbar-inner padding: 0 20px）
        const int pad = 20;
        _logout.Size = new Size(52, 30);
        _logout.Left = _topBar.Width - pad - _logout.Width;
        _logout.Top = (Theme.TopBarHeight - _logout.Height) / 2 - 4;
        _avatar.Left = _logout.Left - 8 - _avatar.Width;
        _avatar.Top = (Theme.TopBarHeight - _avatar.Height) / 2;

        // 中间区：logo + 房支下拉 + 含下级，整体水平居中
        const int gap = 12;
        _logo.Height = 34;
        _logo.Top = 0;
        var logoW = Math.Max(60, _logo.PreferredWidth());
        _logo.Width = logoW;
        _logo.Left = 0;

        var showBranch = !string.IsNullOrWhiteSpace(Prefs.Current.Token);
        _branch.Visible = showBranch;
        _include.Visible = showBranch;

        var w = logoW;
        if (showBranch) w += gap + _branch.Width + 6 + _include.Width;

        _branch.Left = logoW + gap;
        _branch.Top = (Theme.TopBarHeight - _branch.Height) / 2;
        _include.Left = _branch.Left + _branch.Width + 6;
        _include.Top = (Theme.TopBarHeight - _include.Height) / 2;

        _centerArea.Size = new Size(w, Theme.TopBarHeight);
        _centerArea.Left = Math.Max(pad, (_topBar.Width - w) / 2);
        _centerArea.Top = 0;
        _logo.Top = (Theme.TopBarHeight - _logo.Height) / 2;
    }

    private void ApplyIdentity()
    {
        var name = "";
        var role = "";
        long id = 0;
        try
        {
            using var doc = JsonDocument.Parse(string.IsNullOrWhiteSpace(Prefs.Current.UserJson) ? "{}" : Prefs.Current.UserJson);
            var root = doc.RootElement;
            if (root.TryGetProperty("username", out var u)) name = u.GetString() ?? "";
            if (root.TryGetProperty("role", out var r)) role = r.GetString() ?? "";
            if (root.TryGetProperty("id", out var i) && i.TryGetInt64(out var v)) id = v;
        }
        catch { /* 用户信息坏了就按空处理 */ }

        _avatar.Initial = string.IsNullOrEmpty(name) ? "族" : name.Substring(0, 1);
        _userId = id;
        _role = role;

        _sidebar.Controls.Clear();
        _rowsByRoute.Clear();
        _childrenByGroup.Clear();
        _groupRows.Clear();
        _menuHost.Controls.Clear();

        var footer = new Panel { Dock = DockStyle.Bottom, Height = Theme.SidebarFooterHeight, BackColor = Theme.Ink };
        footer.Controls.Add(_version);
        _sidebar.Controls.Add(_menuHost);
        _sidebar.Controls.Add(footer);

        BuildMenuRows(BuildNav(role));
        LayoutRows();
        _menuHost.Resize += (_, _) => LayoutRows();
        _menuHost.ClientSizeChanged += (_, _) => LayoutRows();
        _menuHost.ControlAdded += (_, _) => LayoutRows();
    }

    private long _userId;
    private string _role = "";

    private static NavNode[] BuildNav(string role)
    {
        var admin = role is "理事长" or "超级管理员" or "admin" or "superadmin";
        var list = new List<NavNode>
        {
            new("📊 数据看板", "", new NavNode[]
            {
                new("📈 数据看板", "/dashboard"),
                new("🎂 生日", "/birthday"),
                new("🕯️ 祭日", "/deathday"),
                new("🎏 传统日程", "/traditions"),
                new("🗺️ 热力分布", "/heatmap"),
                new("🚶 迁徙记录", "/migrations"),
                new("📇 通讯录", "/contacts"),
            }),
            new("🌳 世系可视化", "", new NavNode[]
            {
                new("🏯 宝塔式", "/tree"),
                new("🎐 苏式垂吊图", "/sutree"),
                new("🕸️ 苏式垂丝图", "/susi"),
                new("📿 苏氏垂珠体", "/suzhu"),
                new("🪜 世代分层图", "/gentree"),
                new("📃 竖排牒记式", "/demo-dieji"),
                new("📄 横版牒记式", "/demo-dieji-land"),
                new("📕 谱书封面", "/cover-design"),
                new("🌲 传代树", "/test-zupu"),
            }),
            new("👥 成员管理", "/members"),
            new("🏡 房支管理", "/branches"),
            new("🖼️ 家族多媒体", "/media"),
            new("💍 配偶资料", "/spouses-list"),
            new("📜 字辈派语", "/generation"),
            new("🏛️ 谱头文化", "", new NavNode[]
            {
                new("🗓️ 家族大事记", "/culture/events"),
                new("✒️ 谱序", "/culture/articles"),
                new("👤 名人传记", "/culture/bios"),
            }),
            new("🏮 祠墓", "", new NavNode[]
            {
                new("🛕 祠堂", "/hall"),
                new("🏯 祠宇志", "/shrines"),
                new("🪦 茔墓志", "/graves"),
            }),
            new("📚 家族知识库", "/knowledge"),
            new("💰 收支公示", "/accounts"),
            new("🪦 功德碑", "/merit"),
        };

        if (admin)
        {
            list.Add(new NavNode("🤖 修谱助手", "/ai-assistant"));
            list.Add(new NavNode("✅ 审核中心", "/review"));
            list.Add(new NavNode("⚙️ 系统设置", "", new NavNode[]
            {
                new("🏷️ 家族设置", "/settings"),
                new("🩺 数据体检", "/data-check"),
                new("💾 备份管理", "/backup"),
                new("⬆️ 版本更新", "/update"),
                new("🖥️ 服务器设置", "/server-settings"),
                new("🗺️ 离线地图", "/map-offline"),
                new("📖 使用说明", "/usage-guide"),
                new("ℹ️ 关于我们", "/about"),
            }));
        }

        // 客户端自己的设置：一个入口，点开是设置弹窗（对照 DSH 的设置界面）
        list.Add(new NavNode("⚙️ 设置", ClientSettings.SettingsRoute));

        return list.ToArray();
    }

    private void BuildMenuRows(NavNode[] nodes)
    {
        foreach (var node in nodes)
        {
            var isGroup = node.IsGroup;
            var row = new MenuRow
            {
                Text = node.Label,
                Route = node.Route,
                IconKey = isGroup ? "group:" + node.Label : node.Route,
                IsGroup = isGroup,
                Level = 0,
                Height = Theme.MenuRowHeight,
                Expanded = isGroup && node.Label.Contains("数据看板"),
            };
            _menuHost.Controls.Add(row);

            if (isGroup)
            {
                _groupRows[node.Label] = row;
                var kids = new List<MenuRow>();
                foreach (var child in node.Children!)
                {
                    var cr = new MenuRow
                    {
                        Text = child.Label,
                        Route = child.Route,
                        IconKey = child.Route,
                        IsGroup = false,
                        Level = 1,
                        Height = Theme.SubMenuRowHeight,
                        Visible = row.Expanded,
                    };
                    _menuHost.Controls.Add(cr);
                    _rowsByRoute[child.Route] = cr;
                    kids.Add(cr);

                    // 子项同样要接点击（之前漏了，导致分组里的菜单全是死的）
                    var childRoute = child.Route;
                    var childLabel = child.Label;
                    cr.Activated += (_, _) => Navigate(childLabel, childRoute);
                }
                _childrenByGroup[node.Label] = kids;
                row.Activated += (_, _) => ToggleGroup(node.Label);
            }
            else
            {
                row.Expanded = false;
                _rowsByRoute[node.Route] = row;
                var route = node.Route;
                var label = node.Label;
                row.Activated += (_, _) => Navigate(label, route);
            }
        }
    }

    private void LayoutRows()
    {
        // 预留纵向滚动条宽度，否则行宽超出可视区会再冒出一条横向滚动条
        var w = Math.Max(80, _menuHost.ClientSize.Width - SystemInformation.VerticalScrollBarWidth);
        foreach (Control c in _menuHost.Controls)
        {
            c.Width = w;
            c.Margin = Padding.Empty;
        }
    }

    private void ToggleGroup(string label)
    {
        if (!_groupRows.TryGetValue(label, out var row)) return;
        row.Expanded = !row.Expanded;
        row.Invalidate();
        if (_childrenByGroup.TryGetValue(label, out var kids))
            foreach (var k in kids) k.Visible = row.Expanded;
    }

    private void ExpandGroupFor(string path)
    {
        foreach (var (label, kids) in _childrenByGroup)
        {
            if (!kids.Any(k => k.Route == path)) continue;
            if (_groupRows.TryGetValue(label, out var row) && !row.Expanded)
            {
                row.Expanded = true;
                row.Invalidate();
                foreach (var k in kids) k.Visible = true;
            }
        }
    }

    private void SetActive(string path)
    {
        path = NormalizePath(path);
        if (path.Length == 0) return;
        ExpandGroupFor(path);

        foreach (var (route, row) in _rowsByRoute)
        {
            var on = NormalizePath(route) == path;
            if (row.Active == on) continue;
            row.Active = on;
            row.Invalidate();
        }
    }

    private static string NormalizePath(string? p)
    {
        if (string.IsNullOrEmpty(p)) return "";
        var q = p.IndexOf('?');
        if (q >= 0) p = p[..q];
        var h = p.IndexOf('#');
        if (h >= 0) p = p[..h];
        if (p.Length > 1) p = p.TrimEnd('/');
        return p;
    }

    // ================= 网页 =================

    private void InitWeb()
    {
        _web.FrameLoadStart += async (_, e) =>
        {
            if (e.Frame is null || !e.Frame.IsMain) return;
            await InjectShellChromeAsync();
        };
        _web.FrameLoadEnd += async (_, e) =>
        {
            if (e.Frame is null || !e.Frame.IsMain) return;
            await InjectShellChromeAsync();
            await OnLoadedAsync();
        };
        _web.LoadError += (_, e) =>
        {
            if (e.Frame is null || !e.Frame.IsMain) return;
            Ui(() => _version.Text = "加载失败 " + e.ErrorCode);
        };

        _routePoll.Tick += async (_, _) => await PollRouteAsync();
        _routePoll.Start();

        _web.LoadUrl(Prefs.Current.Address);
    }

    /// <summary>隐藏网页自带的顶栏/侧栏（原生外壳替代它们）</summary>
    private async Task InjectShellChromeAsync()
    {
        const string js = """
            (function(){
              try{
                var id='zupu-client-shell';
                var st=document.getElementById(id);
                if(!st){ st=document.createElement('style'); st.id=id; (document.head||document.documentElement).appendChild(st); }
                st.textContent =
                  'header.topbar{display:none!important}'+
                  'aside.sidebar{display:none!important}'+
                  'html,body{overflow:hidden!important;height:100%!important}'+
                  '.layout{height:100vh!important}';
                return 'ok';
              }catch(e){ return 'no:'+e.message; }
            })()
            """;

        // 重载后偶尔会拿不到 document（脚本失败/页面未就绪），重试几次
        for (var attempt = 0; attempt < 6; attempt++)
        {
            var r = await EvalAsync(js);
            if (r == "ok")
            {
                if (attempt > 0) ShellLog($"inject ok (retried {attempt}x)");   // 只有重试过才记一行
                return;
            }
            await Task.Delay(150);
        }
        ShellLog("inject FAILED after 6 attempts");
    }

    private static void ShellLog(string msg)
    {
        try
        {
            File.AppendAllText(
                Path.Combine(AppPaths.DataDir, "shell.log"),
                DateTime.Now.ToString("HH:mm:ss.fff") + "  " + msg + Environment.NewLine);
        }
        catch { /* 忽略 */ }
    }

    /// <summary>一次取回外壳需要的信息：当前路由、家族名、版本号、房支上下文</summary>
    private async Task<ShellInfo?> ReadShellAsync()
    {
        const string js = """
            (function(){
              try{
                var logo=document.querySelector('.topbar .logo');
                var ver=document.querySelector('.sidebar-footer .version');
                var ctx=null;
                try{ ctx=JSON.parse(localStorage.getItem('branchCtx')||'null'); }catch(e){}
                return JSON.stringify({
                  path: location.pathname,
                  logo: logo ? (logo.textContent||'').trim() : '',
                  version: ver ? (ver.textContent||'').trim() : '',
                  branch: ctx && ctx.current ? ctx.current : 0,
                  include: !(ctx && ctx.includeChildren === false)
                });
              }catch(e){ return ''; }
            })()
            """;
        var raw = await EvalAsync(js);
        if (string.IsNullOrWhiteSpace(raw)) return null;
        try
        {
            using var doc = JsonDocument.Parse(raw);
            var r = doc.RootElement;
            return new ShellInfo(
                r.TryGetProperty("path", out var p) ? p.GetString() ?? "" : "",
                r.TryGetProperty("logo", out var l) ? l.GetString() ?? "" : "",
                r.TryGetProperty("version", out var v) ? v.GetString() ?? "" : "",
                r.TryGetProperty("branch", out var b) && b.TryGetInt64(out var bi) ? bi : 0,
                !(r.TryGetProperty("include", out var i) && i.ValueKind == JsonValueKind.False));
        }
        catch { return null; }
    }

    private sealed record ShellInfo(string Path, string Logo, string Version, long Branch, bool Include);

    private async Task OnLoadedAsync()
    {
        if (!_injected && !string.IsNullOrWhiteSpace(Prefs.Current.Token))
        {
            _injected = true;
            var token = JsString(Prefs.Current.Token);
            var user = JsString(Prefs.Current.UserJson);
            await EvalAsync(
                $"try{{localStorage.setItem('token',{token});localStorage.setItem('user',{user});localStorage.setItem('zupu_sponsor_shown','1');}}catch(e){{}}");
            Ui(() => _web.Reload());
            return;
        }

        var info = await ReadShellAsync();
        if (info is null) return;

        Ui(() =>
        {
            if (!string.IsNullOrWhiteSpace(info.Logo) && _logo.Text != info.Logo)
            {
                _logo.Text = info.Logo;
                LayoutTopBar();
            }
            if (!string.IsNullOrWhiteSpace(info.Version)) _version.Text = info.Version;

            _syncing = true;
            try
            {
                _include.Checked = info.Include;
                _branch.SyncSelection(info.Branch);
            }
            finally { _syncing = false; }

            _currentPath = info.Path;
            SetActive(info.Path);
        });

        await LoadBranchesAsync(info.Branch);
    }

    private async Task LoadBranchesAsync(long selected)
    {
        try
        {
            var req = new HttpRequestMessage(HttpMethod.Get, $"{Prefs.Current.Address}/api/branches");
            req.Headers.Add("Authorization", "Bearer " + Prefs.Current.Token);
            var resp = await Net.Http.SendAsync(req);
            if (!resp.IsSuccessStatusCode) return;
            using var doc = JsonDocument.Parse(await resp.Content.ReadAsStringAsync());
            var opts = new List<BranchPicker.Option>();
            foreach (var b in doc.RootElement.EnumerateArray())
            {
                if (!b.TryGetProperty("id", out var idEl) || !idEl.TryGetInt64(out var id)) continue;
                var name = b.TryGetProperty("name", out var n) ? n.GetString() ?? "" : "";
                if (b.TryGetProperty("subtitle", out var sub) && sub.ValueKind == JsonValueKind.String)
                {
                    var s = sub.GetString();
                    if (!string.IsNullOrWhiteSpace(s)) name += " · " + s;
                }
                opts.Add(new BranchPicker.Option(id, name));
            }
            Ui(() => { _branch.SetOptions(opts, selected); LayoutTopBar(); });
        }
        catch { /* 忽略 */ }
    }

    /// <summary>写 localStorage 的房支上下文后重载页面（网页自己没有对外的房支接口）</summary>
    private async Task ApplyBranchAsync(long id, bool includeChildren)
    {
        var uid = _userId;
        var js = $$"""
            (function(){
              try{
                var cur={{id}}, uid={{uid}}, inc={{includeChildren.ToString().ToLowerInvariant()}};
                localStorage.setItem('branchCtx', JSON.stringify({ current: cur, includeChildren: inc, uid: uid }));
                if (uid > 0) localStorage.setItem('branchCtx:u'+uid, JSON.stringify({ v: 2, current: cur, includeChildren: inc, picked: true }));
                return 'ok';
              }catch(e){ return 'no'; }
            })()
            """;
        var r = await EvalAsync(js);
        if (r == "ok") Ui(() => _web.Reload());
    }

    private async Task PollRouteAsync()
    {
        if (!_web.Visible || _web.IsLoading) return;   // 显示的是原生设置页时不跟随
        var path = await EvalAsync("location.pathname");
        if (string.IsNullOrEmpty(path) || path == _currentPath) return;
        _currentPath = path;
        Ui(() => SetActive(path));
    }

    private async void Navigate(string label, string route)
    {
        ShellLog($"navigate '{label}' -> {route} (from {_currentPath})");

        // 客户端自己的设置：弹窗，不占用内容区
        if (ClientSettings.IsClientRoute(route))
        {
            OpenSettingsDialog();
            return;
        }

        ShowWeb();
        var want = NormalizePath(route);
        SetActive(route);
        if (NormalizePath(_currentPath) == want) return;   // 已经在这一页

        // 1) 点网页里文本相同的菜单项 → 走 vue-router，不整页刷新
        var clickJs = $$"""
            (function(){
              try{
                var want={{JsString(label)}};
                var items=document.querySelectorAll('li.el-menu-item');
                for(var i=0;i<items.length;i++){
                  var t=(items[i].textContent||'').replace(/\s+/g,' ').trim();
                  if(t===want){ items[i].click(); return 'ok'; }
                }
                return 'no';
              }catch(e){ return 'no'; }
            })()
            """;
        if (await EvalAsync(clickJs) == "ok" && await PathBecomesAsync(want, 600)) return;

        // 2) 直接调网页的 vue-router（菜单项拿不到时）
        var routerJs = $$"""
            (function(){
              try{
                var el=document.getElementById('app');
                var app=el&&el.__vue_app__;
                var rt=app&&app.config&&app.config.globalProperties&&app.config.globalProperties.$router;
                if(!rt) return 'norouter';
                rt.push({{JsString(route)}});
                return 'ok';
              }catch(e){ return 'err'; }
            })()
            """;
        if (await EvalAsync(routerJs) == "ok" && await PathBecomesAsync(want, 600)) return;

        // 3) 整页跳转兜底
        ShellLog($"navigate fallback -> LoadUrl {route}");
        Ui(() => _web.LoadUrl(Prefs.Current.Address + route));
    }

    // ================= 托盘（后台运行） =================

    private void HideToTray()
    {
        EnsureTray();
        Hide();
        ShowInTaskbar = false;
        try
        {
            _tray?.ShowBalloonTip(2000, "族谱客户端", "仍在后台运行，双击托盘图标可以重新打开。", ToolTipIcon.Info);
        }
        catch { /* 气泡提示失败无所谓 */ }
    }

    private void EnsureTray()
    {
        if (_tray is not null) return;

        Icon icon;
        try { icon = Icon.ExtractAssociatedIcon(Application.ExecutablePath) ?? SystemIcons.Application; }
        catch { icon = SystemIcons.Application; }

        var menu = new ContextMenuStrip();
        menu.Items.Add("显示主窗口", null, (_, _) => ShowFromTray());
        menu.Items.Add(new ToolStripSeparator());
        menu.Items.Add("退出族谱客户端", null, (_, _) => QuitFromTray());

        _tray = new NotifyIcon
        {
            Icon = icon,
            Text = "族谱客户端",
            Visible = true,
            ContextMenuStrip = menu,
        };
        _tray.DoubleClick += (_, _) => ShowFromTray();
    }

    private void ShowFromTray()
    {
        ShowInTaskbar = true;
        Show();
        if (WindowState == FormWindowState.Minimized) WindowState = FormWindowState.Normal;
        Activate();
        BringToFront();
    }

    private void QuitFromTray()
    {
        _exiting = true;
        try { if (_tray is not null) { _tray.Visible = false; _tray.Dispose(); _tray = null; } } catch { }
        Close();
    }

    /// <summary>确保网页内容区可见</summary>
    private void ShowWeb()    {
        Ui(() =>
        {
            _web.Visible = true;
            _web.BringToFront();
        });
    }

    /// <summary>打开设置弹窗；关闭后恢复原来的网页高亮</summary>
    private void OpenSettingsDialog()
    {
        var prev = _currentPath;
        using var dlg = new SettingsDialog();
        dlg.ShowDialog(this);
        Ui(() =>
        {
            _currentPath = prev;
            SetActive(prev);
            ShowWeb();
        });
    }

    /// <summary>等 location.pathname 变成目标路径，确认这次跳转真的生效了</summary>
    private async Task<bool> PathBecomesAsync(string want, int timeoutMs)
    {
        var deadline = Environment.TickCount64 + timeoutMs;
        while (Environment.TickCount64 < deadline)
        {
            var p = await EvalAsync("location.pathname");
            if (NormalizePath(p) == want)
            {
                _currentPath = p!;
                return true;
            }
            await Task.Delay(60);
        }
        return false;
    }

    // ================= 工具 =================

    private static string JsString(string? s)
    {
        if (string.IsNullOrEmpty(s)) return "''";
        var sb = new System.Text.StringBuilder("'");
        foreach (var c in s)
        {
            sb.Append(c switch
            {
                '\\' => "\\\\",
                '\'' => "\\'",
                '\r' => " ",
                '\n' => " ",
                '\u2028' => " ",
                '\u2029' => " ",
                _ => c.ToString(),
            });
        }
        return sb.Append('\'').ToString();
    }

    private async Task<string?> EvalAsync(string js)
    {
        try
        {
            var response = await _web.EvaluateScriptAsync(js);
            if (!response.Success) return null;
            return response.Result?.ToString() ?? "";
        }
        catch { return null; }
    }

    private void Ui(Action action)
    {
        if (IsDisposed) return;
        if (InvokeRequired) BeginInvoke(action);
        else action();
    }

    private void Logout()
    {
        Prefs.Current.Token = "";
        Prefs.Save();
        _routePoll.Stop();
        Hide();
        new LoginForm().ShowDialog();
        Close();
    }
}
