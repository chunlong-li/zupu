using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace ZupuClient;

/// <summary>本地配置：地址历史、记住密码、token、窗口位置</summary>
public static class Prefs
{
    private static string Dir => AppPaths.DataDir;
    private static string File_ => Path.Combine(Dir, "settings.json");

    /// <summary>DPAPI 附加熵：同一用户下隔离本应用的密文</summary>
    private static readonly byte[] Entropy = Encoding.UTF8.GetBytes("ZupuClient.Settings.v1");

    public class Data
    {
        public string Address { get; set; } = "";
        public string User { get; set; } = "";

        /// <summary>DPAPI 保护后的密码（base64）。空表示未保存密码。</summary>
        public string PassEnc { get; set; } = "";

        /// <summary>旧版本明文字段，仅用于一次性迁移，保存时清空。</summary>
        [JsonPropertyName("Pass")]
        [JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
        public string? LegacyPass { get; set; }

        /// <summary>运行时明文密码，不落盘。</summary>
        [JsonIgnore]
        public string Pass { get; set; } = "";

        public bool Remember { get; set; } = true;
        public string Token { get; set; } = "";
        public string UserJson { get; set; } = "";
        public List<string> Addresses { get; set; } = new();
        public int WinW { get; set; } = 1200;
        public int WinH { get; set; } = 820;
        public int WinX { get; set; } = -1;
        public int WinY { get; set; } = -1;
        public string BackMode { get; set; } = "web";

        /// <summary>客户端更新源（空 = 用默认源）</summary>
        public string ClientFeed { get; set; } = "";

        /// <summary>服务端更新源（空 = 用默认源）</summary>
        public string ServerFeed { get; set; } = "";

        /// <summary>服务端安装目录（空 = %LOCALAPPDATA%\ZupuServer）</summary>
        public string ServerDir { get; set; } = "";

        /// <summary>点 ✕ 关闭时的行为：exit=直接退出 / tray=后台运行（托盘）</summary>
        public string CloseAction { get; set; } = "exit";

        /// <summary>关闭时是否每次都弹出选择框</summary>
        public bool CloseAsk { get; set; } = true;

        /// <summary>自签证书指纹固定（TOFU）：host -> sha256(hex)</summary>
        public Dictionary<string, string> CertPins { get; set; } = new();
    }

    private static Data? _cache;

    public static Data Current
    {
        get
        {
            if (_cache != null) return _cache;
            try
            {
                if (File.Exists(File_))
                    _cache = JsonSerializer.Deserialize<Data>(File.ReadAllText(File_)) ?? new Data();
            }
            catch { /* 配置损坏就用默认 */ }

            _cache ??= new Data();

            // 解密密码；若只有旧版明文则迁移到密文
            if (!string.IsNullOrEmpty(_cache.PassEnc))
                _cache.Pass = Unprotect(_cache.PassEnc);
            else if (!string.IsNullOrEmpty(_cache.LegacyPass))
                _cache.Pass = _cache.LegacyPass!;
            _cache.LegacyPass = null;

            return _cache;
        }
    }

    public static void Save()
    {
        try
        {
            Directory.CreateDirectory(Dir);
            var d = Current;
            // 明文进密文，绝不落盘明文
            d.PassEnc = string.IsNullOrEmpty(d.Pass) ? "" : Protect(d.Pass);
            d.LegacyPass = null;
            File.WriteAllText(File_, JsonSerializer.Serialize(d, new JsonSerializerOptions { WriteIndented = true }));
        }
        catch { /* 忽略 */ }
    }

    public static void RememberAddress(string addr)
    {
        if (string.IsNullOrWhiteSpace(addr)) return;
        Current.Addresses.Remove(addr);
        Current.Addresses.Insert(0, addr);
        if (Current.Addresses.Count > 8) Current.Addresses = Current.Addresses.Take(8).ToList();
        Current.Address = addr;
        Save();
    }

    /// <summary>读取/记录某主机的证书指纹（首次信任，之后必须一致）</summary>
    public static string? GetPin(string host)
        => Current.CertPins.TryGetValue(host, out var v) ? v : null;

    public static void SetPin(string host, string fingerprintHex)
    {
        Current.CertPins[host] = fingerprintHex;
        Save();
    }

    private static string Protect(string plain)
    {
        try
        {
            var bytes = ProtectedData.Protect(Encoding.UTF8.GetBytes(plain), Entropy, DataProtectionScope.CurrentUser);
            return Convert.ToBase64String(bytes);
        }
        catch { return ""; }   // 保护失败则不保存密码，避免退化成明文
    }

    private static string Unprotect(string b64)
    {
        try
        {
            var bytes = ProtectedData.Unprotect(Convert.FromBase64String(b64), Entropy, DataProtectionScope.CurrentUser);
            return Encoding.UTF8.GetString(bytes);
        }
        catch { return ""; }   // 换了用户/机器就解不开，按未保存处理
    }
}
