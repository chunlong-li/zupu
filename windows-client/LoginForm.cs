namespace ZupuClient;

public class LoginForm : Form
{
    private readonly ComboBox _addr = new() { Dock = DockStyle.Fill, DropDownStyle = ComboBoxStyle.DropDown };
    private readonly TextBox _user = new() { Dock = DockStyle.Fill };
    private readonly TextBox _pass = new() { Dock = DockStyle.Fill, UseSystemPasswordChar = true };
    private readonly CheckBox _remember = new() { Text = "记住密码", Checked = true, AutoSize = true };
    private readonly Button _login = new() { Text = "登 录", Height = 44, Dock = DockStyle.Fill };
    private readonly Button _scan = new() { Text = "发现局域网内设备", Height = 36, Dock = DockStyle.Fill };
    private readonly Label _tip = new() { AutoSize = true, ForeColor = Color.DimGray };
    private readonly ProgressBar _bar = new() { Style = ProgressBarStyle.Marquee, Visible = false, Height = 4, Dock = DockStyle.Top };

    public LoginForm()
    {
        Text = "族谱客户端 · 登录";
        Width = 420; Height = 566;
        StartPosition = FormStartPosition.CenterScreen;
        FormBorderStyle = FormBorderStyle.FixedDialog;
        MaximizeBox = false;

        var root = new TableLayoutPanel { Dock = DockStyle.Fill, Padding = new Padding(28), ColumnCount = 1, RowCount = 13 };
        Label Title() => new() { Text = "族 谱", Font = new Font("Microsoft YaHei UI", 22, FontStyle.Bold), AutoSize = true, TextAlign = ContentAlignment.MiddleCenter, Margin = new Padding(0, 4, 0, 10) };

        root.Controls.Add(Title());
        root.Controls.Add(new Label { Text = "服务器地址", AutoSize = true, Margin = new Padding(0, 18, 0, 4) });
        foreach (var a in Prefs.Current.Addresses) _addr.Items.Add(a);
        _addr.Text = Prefs.Current.Address;
        _addr.Height = 32;
        root.Controls.Add(_addr);
        root.Controls.Add(new Label { Text = "账号", AutoSize = true, Margin = new Padding(0, 12, 0, 4) });
        _user.Text = Prefs.Current.User;
        root.Controls.Add(_user);
        root.Controls.Add(new Label { Text = "密码", AutoSize = true, Margin = new Padding(0, 12, 0, 4) });
        _pass.Text = Prefs.Current.Pass;
        root.Controls.Add(_pass);
        _remember.Checked = Prefs.Current.Remember;
        root.Controls.Add(_remember);
        root.Controls.Add(_bar);
        root.Controls.Add(_login);
        root.Controls.Add(_scan);
        root.Controls.Add(_tip);

        // 登录前也能进设置：装服务端、改更新源、开机自启都在这
        var settings = new LinkLabel
        {
            Text = "⚙ 设置（安装服务端 / 客户端更新）",
            AutoSize = true,
            Anchor = AnchorStyles.Right,
            Margin = new Padding(0, 12, 0, 0),
            LinkColor = Color.FromArgb(140, 90, 60),
            ActiveLinkColor = Color.FromArgb(58, 50, 38),
            VisitedLinkColor = Color.FromArgb(140, 90, 60),
        };
        void OpenSettings()
        {
            using var dlg = new SettingsDialog();
            dlg.ShowDialog(this);
            // 设置里可能装了本地服务端、或改了服务器地址，顺手回填
            if (_addr.Text.Trim().Length == 0 && !string.IsNullOrWhiteSpace(Prefs.Current.Address))
                _addr.Text = Prefs.Current.Address;
        }
        settings.LinkClicked += (_, _) => OpenSettings();
        settings.Click += (_, _) => OpenSettings();
        root.RowCount += 1;
        root.Controls.Add(settings);

        Controls.Add(root);

        _login.Click += async (_, _) => await DoLogin();
        _scan.Click += async (_, _) => await DoScan();
        AcceptButton = _login;
    }

    private async Task DoLogin()
    {
        var addr = Net.Normalize(_addr.Text);
        if (addr.Length == 0) { Tip("请填服务器地址"); return; }
        if (_user.Text.Trim().Length == 0) { Tip("请填账号"); return; }
        if (_pass.Text.Length == 0) { Tip("请填密码"); return; }

        SetBusy(true);
        try
        {
            var r = await Net.LoginAsync(addr, _user.Text.Trim(), _pass.Text);
            var p = Prefs.Current;
            p.Address = addr; p.User = _user.Text.Trim();
            p.Pass = _remember.Checked ? _pass.Text : "";
            p.Remember = _remember.Checked;
            p.Token = r.Token; p.UserJson = r.UserJson;
            Prefs.RememberAddress(addr);
            Prefs.Save();
            Hide();
            new MainForm().ShowDialog();
            Close();
        }
        catch (Exception ex) { Tip("登录失败：" + ex.Message); }
        finally { SetBusy(false); }
    }

    private async Task DoScan()
    {
        SetBusy(true);
        _scan.Text = "正在扫描局域网…";
        var found = new List<string>();
        int hits = 0;
        try
        {
            found = await Net.DiscoverAsync(new Progress<string>(_ => Tip($"已发现 {++hits} 个设备…")));
            if (found.Count == 0) Tip("没找到族谱服务，请确认在同一局域网或手动填地址");
            else
            {
                _addr.Items.Clear();
                foreach (var f in found) _addr.Items.Add(f);
                _addr.Text = found[0];
                Tip($"找到 {found.Count} 个：{string.Join("，", found)}");
            }
        }
        catch (Exception ex) { Tip("扫描失败：" + ex.Message); }
        finally { SetBusy(false); _scan.Text = "发现局域网内设备"; }
    }

    private void SetBusy(bool busy) { _bar.Visible = busy; _login.Enabled = !busy; _scan.Enabled = !busy; }
    private void Tip(string s) { _tip.Text = s; }
}
