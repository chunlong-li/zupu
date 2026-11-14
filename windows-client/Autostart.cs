using Microsoft.Win32;

namespace ZupuClient;

/// <summary>
/// 开机自启。两种机制，都不用管理员权限：
///  1) 首选 HKCU\...\Run 注册表（最标准，官方文档 Run and RunOnce Registry Keys）
///  2) 注册表被策略/ACL 拦住时，退回到「启动」文件夹里的 .cmd
///
/// 注意 Windows 侧的坑（都会让「开关打开但其实没启动」）：
///  · 任务管理器「启动」页把某项禁用后，会写 StartupApproved\Run 的二进制标志位，
///    注册表值还在但系统会跳过它 —— 这里检测并提示，重新打开开关时会顺手把这个标志清掉。
///  · Run 项启动时当前目录是 C:\Windows\System32，客户端必须在启动时自己切回程序目录（见 Program.Main）。
/// </summary>
internal static class Autostart
{
    private const string RunKey = @"Software\Microsoft\Windows\CurrentVersion\Run";
    private const string ApprovedKey =
        @"Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run";
    private const string ClientValue = "ZupuClient";
    private const string ServerValue = "ZupuServer";

    public static string ClientCommand =>
        "\"" + Path.Combine(AppContext.BaseDirectory, "ZupuClient.exe") + "\" --autostart-launch";

    /// <summary>
    /// 把已登记的 Run 值刷新成当前命令行：程序被移动过、或者老版本没带
    /// --autostart-launch 时自动补上（客户端靠它判断要不要等服务端）。
    /// </summary>
    public static void Refresh()
    {
        try
        {
            var v = Read(ClientValue);
            if (v is not null && v != ClientCommand) TryWrite(ClientValue, ClientCommand);
        }
        catch { /* 忽略 */ }
    }

    public static string ServerCommand => ServerInstaller.IsInstalled
        ? $"cmd /c start \"\" /min \"{ServerInstaller.StartBatPath}\""
        : "";

    private static string StartupDir =>
        Environment.GetFolderPath(Environment.SpecialFolder.Startup);

    private static string ClientCmdFile => Path.Combine(StartupDir, "族谱客户端.cmd");
    private static string ServerCmdFile => Path.Combine(StartupDir, "族谱服务端.cmd");

    public static bool ClientEnabled => Read(ClientValue) is not null || File.Exists(ClientCmdFile);
    public static bool ServerEnabled => Read(ServerValue) is not null || File.Exists(ServerCmdFile);

    /// <summary>当前生效的方式，给界面显示用</summary>
    public static string ClientMechanism
    {
        get
        {
            var v = Read(ClientValue);
            if (v is not null)
                return IsDisabledByWindows(ClientValue)
                    ? "已登记，但被「任务管理器 → 启动」禁用：" + v
                    : "注册表：" + v;
            if (File.Exists(ClientCmdFile)) return "启动文件夹：" + ClientCmdFile;
            return "未登记";
        }
    }

    public static string ServerMechanism
    {
        get
        {
            var v = Read(ServerValue);
            if (v is not null)
                return IsDisabledByWindows(ServerValue)
                    ? "已登记，但被「任务管理器 → 启动」禁用：" + v
                    : "注册表：" + v;
            if (File.Exists(ServerCmdFile)) return "启动文件夹：" + ServerCmdFile;
            return ServerInstaller.IsInstalled ? "未登记" : "未安装服务端";
        }
    }

    public static void SetClient(bool on)
    {
        if (!on)
        {
            TryWrite(ClientValue, null);
            TryDelete(ClientCmdFile);
            return;
        }
        var exe = Path.Combine(AppContext.BaseDirectory, "ZupuClient.exe");
        if (!File.Exists(exe)) throw new FileNotFoundException("找不到 ZupuClient.exe", exe);

        if (TryWrite(ClientValue, ClientCommand))
        {
            ClearDisabled(ClientValue);     // 顺手解除“任务管理器里被禁用”的标志
            TryDelete(ClientCmdFile);       // 两种机制只留一种
            return;
        }
        WriteStartupCmd(ClientCmdFile, "族谱客户端", $"start \"\" \"{exe}\" --autostart-launch");
    }

    public static void SetServer(bool on)
    {
        if (!on)
        {
            TryWrite(ServerValue, null);
            TryDelete(ServerCmdFile);
            return;
        }
        if (!ServerInstaller.IsInstalled)
            throw new InvalidOperationException("还没安装服务端，先到「服务端」页面安装");

        if (TryWrite(ServerValue, ServerCommand))
        {
            ClearDisabled(ServerValue);
            TryDelete(ServerCmdFile);
            return;
        }
        WriteStartupCmd(ServerCmdFile, "族谱服务端",
            $"start \"\" /min \"{ServerInstaller.StartBatPath}\"");
    }

    // ==================== 内部 ====================

    private static string? Read(string name)
    {
        try
        {
            using var key = Registry.CurrentUser.OpenSubKey(RunKey);
            return key?.GetValue(name) as string;
        }
        catch { return null; }
    }

    /// <summary>Windows 是否在「任务管理器 → 启动」里把这一项禁用了（值还在，但系统会跳过）</summary>
    private static bool IsDisabledByWindows(string name)
    {
        try
        {
            using var key = Registry.CurrentUser.OpenSubKey(ApprovedKey);
            if (key?.GetValue(name) is not byte[] bytes || bytes.Length == 0) return false;
            return (bytes[0] & 1) != 0;
        }
        catch { return false; }
    }

    /// <summary>删掉禁用标志，恢复「启用」</summary>
    private static void ClearDisabled(string name)
    {
        try
        {
            using var key = Registry.CurrentUser.OpenSubKey(ApprovedKey, writable: true);
            key?.DeleteValue(name, throwOnMissingValue: false);
        }
        catch { /* 清不掉也不影响，Windows 下次自己会重建 */ }
    }

    /// <summary>写注册表；被拒时返回 false 交给兜底</summary>
    private static bool TryWrite(string name, string? value)
    {
        try
        {
            using var key = Registry.CurrentUser.CreateSubKey(RunKey, writable: true);
            if (value is null) key.DeleteValue(name, throwOnMissingValue: false);
            else key.SetValue(name, value, RegistryValueKind.String);
            return true;
        }
        catch (UnauthorizedAccessException) { return false; }
        catch (System.Security.SecurityException) { return false; }
        catch (IOException) { return false; }
    }

    private static void WriteStartupCmd(string path, string title, string body)
    {
        Directory.CreateDirectory(Path.GetDirectoryName(path)!);
        // cmd 按当前代码页解析脚本，路径含中文时用 UTF-8 + chcp 65001，避免乱码
        var text = "@echo off\r\nchcp 65001 >nul\r\nrem " + title + " 开机自启（由族谱客户端写入）\r\n" + body + "\r\n";
        File.WriteAllText(path, text, new System.Text.UTF8Encoding(false));
    }

    private static void TryDelete(string path)
    {
        try { if (File.Exists(path)) File.Delete(path); } catch { }
    }
}
