package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

// Config 应用配置
type Config struct {
	Port        string // 监听端口
	DataDir     string // 数据根目录（宿主机映射）
	AdminUser   string // 管理员账号
	AdminPass   string // 管理员密码
	Timezone    string // 时区
	MaxUploadMB int64  // 上传大小限制（MB）

	// 在线更新配置
	AppDir         string // 应用目录（二进制所在，默认 /data/runtime）
	WebDir         string // 前端静态目录（默认 /data/runtime/web）
	EnableUpdate   bool   // 是否启用在线更新
	UpdateDirHost  string // 宿主机上的更新包目录路径（仅用于前端展示提示，默认空）
	UpdateURL      string // 在线更新清单 URL（latest.json），为空则禁用「在线更新」
	UpdateInsecure bool   // 跳过更新源 TLS 证书校验（自签名证书场景，默认 false）

	// 传承服务器（家族知识库）反代
	KBURL       string // 传承服务器地址，如 http://knowledge:8090；为空则隐藏知识库入口
	KBSSOSecret string // 与传承服务器共享的密钥（签发短时 JWT 做 SSO）
	KBLoginMode string // 知识库登录源：auto（跟随知识库模式）| zupu（族谱登录）| local（知识库自带登录，透明反代）
}

// exeDir 返回可执行文件所在目录（用于 Windows 相对路径定位）
func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// defaultDataDir 按平台返回默认数据根目录
func defaultDataDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(exeDir(), "data")
	}
	return "/data"
}

// defaultAppDir 按平台返回默认应用目录（在线更新运行时目录）
func defaultAppDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(exeDir(), "runtime")
	}
	return "/data/runtime"
}

// defaultWebDir 按平台返回默认前端静态目录
func defaultWebDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(exeDir(), "web")
	}
	return "/data/runtime/web"
}

// Load 从环境变量加载配置
func Load() *Config {
	return &Config{
		Port:        getEnv("FAMILY_TREE_PORT", "8080"),
		DataDir:     getEnv("FAMILY_TREE_DATA_DIR", defaultDataDir()),
		AdminUser:   getEnv("FAMILY_TREE_ADMIN_USER", "admin"),
		AdminPass:   getEnv("FAMILY_TREE_ADMIN_PASS", "admin123"),
		Timezone:    getEnv("FAMILY_TREE_TIMEZONE", "Asia/Shanghai"),
		MaxUploadMB: getEnvInt64("FAMILY_TREE_MAX_UPLOAD_MB", 20),

		AppDir:       getEnv("FAMILY_TREE_APP_DIR", defaultAppDir()),
		WebDir:       getEnv("FAMILY_TREE_WEB_DIR", defaultWebDir()),
		EnableUpdate: getEnvBool("FAMILY_TREE_ENABLE_UPDATE", true),

		UpdateDirHost:  getEnv("FAMILY_TREE_UPDATE_DIR_HOST", ""),
		UpdateURL:      getEnv("FAMILY_TREE_UPDATE_URL", "https://zupu-update.lijiacore.com:4440"),
		UpdateInsecure: getEnvBool("FAMILY_TREE_UPDATE_INSECURE", true),

		KBURL:       getEnv("FAMILY_TREE_KB_URL", ""),
		KBSSOSecret: getEnv("FAMILY_TREE_KB_SSO_SECRET", "zupu-kb-sso-dev"),
		KBLoginMode: getEnv("FAMILY_TREE_KB_LOGIN_MODE", "auto"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
