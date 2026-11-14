namespace ZupuClient;

/// <summary>客户端原生页面的路由约定（/__client/* 一律由客户端自己渲染）</summary>
internal static class ClientSettings
{
    public const string Prefix = "/__client/";

    /// <summary>设置弹窗（对照 DSH 的设置界面：左侧分区 + 右侧设置项）</summary>
    public const string SettingsRoute = Prefix + "settings";

    public static bool IsClientRoute(string? route)
        => !string.IsNullOrEmpty(route) && route.StartsWith(Prefix, StringComparison.Ordinal);
}
