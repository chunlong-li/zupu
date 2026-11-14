using System.Diagnostics;
using System.Text;

namespace ZupuUpdater;

/// <summary>
/// 族谱客户端更新器。
///
/// 由客户端在退出前拉起，负责：
///   1) 等客户端进程真正退出（释放 exe / dll 的占用）
///   2) 把已解压的新版本目录覆盖到程序目录（保留数据文件，不删除多余文件）
///   3) 启动新版客户端
///   4) 清理暂存目录与下载的压缩包
///
/// 全部用 .NET 文件 API，路径是 Unicode，不受 cmd 代码页影响（中文路径不会乱码）。
/// </summary>
internal static class Program
{
    private sealed class Options
    {
        public int WaitPid;
        public string? Stage;
        public string? App;
        public string? Log;
        public string? CleanZip;
    }

    private static string _log = "";

    private static int Main(string[] args)
    {
        var o = Parse(args);
        _log = string.IsNullOrWhiteSpace(o.Log)
            ? Path.Combine(Path.GetTempPath(), "zupu-updater.log")
            : o.Log!;

        Log($"==== 更新开始 v{typeof(Program).Assembly.GetName().Version} ====");
        Log($"stage={o.Stage}  app={o.App}  waitPid={o.WaitPid}");

        try
        {
            if (string.IsNullOrWhiteSpace(o.Stage) || !Directory.Exists(o.Stage))
                throw new DirectoryNotFoundException("暂存目录不存在: " + o.Stage);
            if (string.IsNullOrWhiteSpace(o.App) || !Directory.Exists(o.App))
                throw new DirectoryNotFoundException("程序目录不存在: " + o.App);

            WaitForClient(o.WaitPid);
            var copied = CopyTree(o.Stage!, o.App!);
            Log($"已更新 {copied} 个文件");

            StartClient(o.App!);
            Cleanup(o.Stage!, o.CleanZip);
            Log("==== 更新完成 ====");
            return 0;
        }
        catch (Exception ex)
        {
            Log("更新失败: " + ex);
            return 1;
        }
    }

    private static Options Parse(string[] args)
    {
        var o = new Options();
        for (var i = 0; i < args.Length - 1; i++)
        {
            var key = args[i];
            var val = args[i + 1];
            switch (key)
            {
                case "--wait-pid": if (int.TryParse(val, out var pid)) o.WaitPid = pid; i++; break;
                case "--stage": o.Stage = val; i++; break;
                case "--app": o.App = val; i++; break;
                case "--log": o.Log = val; i++; break;
                case "--clean-zip": o.CleanZip = val; i++; break;
            }
        }
        return o;
    }

    /// <summary>等客户端退出；最多等 90 秒</summary>
    private static void WaitForClient(int pid)
    {
        if (pid <= 0) { Thread.Sleep(1200); return; }
        try
        {
            var p = Process.GetProcessById(pid);
            Log("等待客户端退出…");
            if (!p.WaitForExit(90000))
                Log("警告：客户端 90 秒内未退出，仍继续（可能失败）");
            else
                Log("客户端已退出");
        }
        catch (ArgumentException)
        {
            Log("客户端进程已不存在，直接继续");
        }
        // 再给文件句柄一点释放时间
        Thread.Sleep(600);
    }

    /// <summary>把 stage 覆盖到 app：不删除 app 里多出来的文件（比如 settings.json / 缓存）</summary>
    private static int CopyTree(string stage, string app)
    {
        var root = Path.GetFullPath(app);
        var count = 0;

        foreach (var src in Directory.EnumerateFiles(stage, "*", SearchOption.AllDirectories))
        {
            var rel = Path.GetRelativePath(stage, src);
            if (rel.Equals("ZupuUpdater.exe", StringComparison.OrdinalIgnoreCase)) continue; // 别覆盖正在运行的自己

            var dst = Path.GetFullPath(Path.Combine(root, rel));
            if (!dst.StartsWith(root, StringComparison.OrdinalIgnoreCase)) continue;   // 防目录穿越

            Directory.CreateDirectory(Path.GetDirectoryName(dst)!);

            var info = new FileInfo(src);
            var target = new FileInfo(dst);
            if (target.Exists && target.Length == info.Length
                && target.LastWriteTimeUtc == info.LastWriteTimeUtc)
                continue;   // 完全一样，跳过

            CopyWithRetry(src, dst);
            count++;
        }
        return count;
    }

    private static void CopyWithRetry(string src, string dst)
    {
        for (var attempt = 0; attempt < 8; attempt++)
        {
            try
            {
                File.Copy(src, dst, overwrite: true);
                return;
            }
            catch (IOException) when (attempt < 7)
            {
                Thread.Sleep(250 * (attempt + 1));
            }
            catch (UnauthorizedAccessException) when (attempt < 7)
            {
                Thread.Sleep(250 * (attempt + 1));
            }
        }
        // 最后一次不吞异常
        File.Copy(src, dst, overwrite: true);
    }

    private static void StartClient(string app)
    {
        var exe = Path.Combine(app, "ZupuClient.exe");
        if (!File.Exists(exe)) throw new FileNotFoundException("找不到 ZupuClient.exe", exe);

        Log("启动新版客户端");
        Process.Start(new ProcessStartInfo
        {
            FileName = exe,
            WorkingDirectory = app,
            UseShellExecute = true,
        });
    }

    private static void Cleanup(string stage, string? zip)
    {
        // 更新器自己可能就住在 stage 里（包内自带），所以先删内容、最后试删目录
        try
        {
            var self = Environment.ProcessPath;
            foreach (var f in Directory.EnumerateFiles(stage, "*", SearchOption.AllDirectories))
            {
                if (self is not null && string.Equals(f, self, StringComparison.OrdinalIgnoreCase)) continue;
                try { File.Delete(f); } catch { }
            }
            foreach (var d in Directory.EnumerateDirectories(stage, "*", SearchOption.AllDirectories)
                         .OrderByDescending(p => p.Length))
            {
                try { Directory.Delete(d, true); } catch { }
            }
            try { Directory.Delete(stage, true); } catch { }
            Log("已清理暂存目录");
        }
        catch (Exception ex) { Log("清理暂存目录失败（可忽略）: " + ex.Message); }

        try
        {
            if (!string.IsNullOrWhiteSpace(zip) && File.Exists(zip))
            {
                File.Delete(zip);
                Log("已清理下载包");
            }
        }
        catch (Exception ex) { Log("清理下载包失败（可忽略）: " + ex.Message); }
    }

    private static void Log(string message)
    {
        try
        {
            var dir = Path.GetDirectoryName(_log);
            if (!string.IsNullOrEmpty(dir)) Directory.CreateDirectory(dir);
            File.AppendAllText(_log, $"{DateTime.Now:HH:mm:ss} {message}{Environment.NewLine}", Encoding.UTF8);
        }
        catch { /* 日志失败不影响更新 */ }
    }
}
