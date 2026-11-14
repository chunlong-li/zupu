namespace ZupuClient;

/// <summary>
/// 应用数据目录。默认 %LOCALAPPDATA%\ZupuClient；
/// 可用环境变量 ZUPU_DATA_DIR 覆盖（绿色版/便携部署用）。
/// </summary>
public static class AppPaths
{
    private static string? _dataDir;

    public static string DataDir
    {
        get
        {
            if (_dataDir != null) return _dataDir;

            var custom = Environment.GetEnvironmentVariable("ZUPU_DATA_DIR");
            if (!string.IsNullOrWhiteSpace(custom))
            {
                _dataDir = Path.GetFullPath(custom.Trim());
            }
            else
            {
                _dataDir = Path.Combine(
                    Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "ZupuClient");
            }

            try { Directory.CreateDirectory(_dataDir); } catch { /* 由调用方处理 */ }
            return _dataDir;
        }
    }
}
