package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// SettingsHandler 系统设置
type SettingsHandler struct {
	DB *sql.DB
}

// FamilyName 获取/设置家族名称
func (h *SettingsHandler) GetFamilyName(c *gin.Context) {
	var val string
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'family_name'`).Scan(&val)
	c.JSON(http.StatusOK, gin.H{"family_name": val})
}

// UpdateFamilyName 更新家族名称
func (h *SettingsHandler) UpdateFamilyName(c *gin.Context) {
	var body struct {
		FamilyName string `json:"family_name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('family_name', ?)`, body.FamilyName)
	c.JSON(http.StatusOK, gin.H{"family_name": body.FamilyName})
}

// GetLoginSecurity 获取登录防爆破配置
func (h *SettingsHandler) GetLoginSecurity(c *gin.Context) {
	maxF, lockMin := GetLoginConfig()
	c.JSON(http.StatusOK, gin.H{"max_failures": maxF, "lock_minutes": lockMin})
}

// UpdateLoginSecurity 更新登录防爆破配置
func (h *SettingsHandler) UpdateLoginSecurity(c *gin.Context) {
	var body struct {
		MaxFailures int `json:"max_failures"`
		LockMinutes int `json:"lock_minutes"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.MaxFailures < 1 || body.MaxFailures > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "失败次数阈值需在 1~100 之间"})
		return
	}
	if body.LockMinutes < 1 || body.LockMinutes > 1440 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "锁定时长需在 1~1440 分钟之间"})
		return
	}
	h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('login_max_failures', ?)`, strconv.Itoa(body.MaxFailures))
	h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('login_lock_minutes', ?)`, strconv.Itoa(body.LockMinutes))
	// 重新加载到内存
	LoadLoginConfig(h.DB)
	c.JSON(http.StatusOK, gin.H{"max_failures": body.MaxFailures, "lock_minutes": body.LockMinutes})
}

// GetLocalNoAuth 读取「本机（127.0.0.1）访问免登录」开关（默认关）
func (h *SettingsHandler) GetLocalNoAuth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"enabled": LocalNoAuthEnabled(h.DB)})
}

// UpdateLocalNoAuth 开/关「本机免登录」。仅理事长与超级管理员可改（路由用 system_settings 权限项）。
func (h *SettingsHandler) UpdateLocalNoAuth(c *gin.Context) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	v := "0"
	if body.Enabled {
		v = "1"
	}
	h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('local_no_auth', ?)`, v)
	c.JSON(http.StatusOK, gin.H{"enabled": body.Enabled})
}

// GetVersion 获取版本号
func (h *SettingsHandler) GetVersion(c *gin.Context) {
	var val string
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'version'`).Scan(&val)
	if val == "" {
		val = "1.3.9"
	}
	var sinceStr string
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'deployed_at'`).Scan(&sinceStr)
	since, _ := strconv.ParseInt(sinceStr, 10, 64)
	c.JSON(http.StatusOK, gin.H{"version": val, "since": since})
}

// GetFamilySurname 获取家族姓氏
func (h *SettingsHandler) GetFamilySurname(c *gin.Context) {
	var surname, familyName, tanghao, junwang, branchDesignation, zuji string
	var shizuName, shizuBio, shiqianzuName, shiqianzuPlace string
	var diejiShowSisters string
	var chuizhuShowDaughters string
	var xiuci, titleWithXiuci string
	var totemImage, totemStyle, totemFontSize string
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'family_surname'`).Scan(&surname)
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'family_name'`).Scan(&familyName)
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'tanghao'`).Scan(&tanghao)
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'junwang'`).Scan(&junwang)
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'branch_designation'`).Scan(&branchDesignation)
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'zuji'`).Scan(&zuji)
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'shizu_name'`).Scan(&shizuName)
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'shizu_bio'`).Scan(&shizuBio)
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'shiqianzu_name'`).Scan(&shiqianzuName)
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'shiqianzu_place'`).Scan(&shiqianzuPlace)
	// 牒记式本支条目是否显示姐妹/女儿：默认开（值为 '0' 时关闭）
	h.DB.QueryRow(`SELECT COALESCE((SELECT value FROM settings WHERE key = 'dieji_show_sisters'), '1')`).Scan(&diejiShowSisters)
	// 苏氏垂珠体是否显示女儿：默认开（值为 '0' 时关闭）
	h.DB.QueryRow(`SELECT COALESCE((SELECT value FROM settings WHERE key = 'chuizhu_show_daughters'), '1')`).Scan(&chuizhuShowDaughters)
	// 修次（創修/初修/二修/三修/四修/五修/重修/續修/自定义）：默认「重修」，保持改动前外观
	h.DB.QueryRow(`SELECT COALESCE((SELECT value FROM settings WHERE key = 'xiuci'), '重修')`).Scan(&xiuci)
	// 题名是否含修次（默认关）：勾上后谱名如「李氏創修族譜」
	h.DB.QueryRow(`SELECT COALESCE((SELECT value FROM settings WHERE key = 'title_with_xiuci'), '0')`).Scan(&titleWithXiuci)
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'totem_image'`).Scan(&totemImage)
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'totem_style'`).Scan(&totemStyle)
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'totem_font_size'`).Scan(&totemFontSize)
	c.JSON(http.StatusOK, gin.H{
		"family_surname": surname, "family_name": familyName,
		"tanghao": tanghao, "junwang": junwang, "branch_designation": branchDesignation, "zuji": zuji,
		"shizu_name": shizuName, "shizu_bio": shizuBio,
		"shiqianzu_name": shiqianzuName, "shiqianzu_place": shiqianzuPlace,
		"totem_image": totemImage, "totem_style": totemStyle,
		"totem_font_size":        totemFontSize,
		"dieji_show_sisters":     diejiShowSisters != "0",
		"chuizhu_show_daughters": chuizhuShowDaughters != "0",
		"xiuci":                  xiuci,
		"title_with_xiuci":       titleWithXiuci == "1",
	})
}

// UpdateFamilySurname 更新家族姓氏
func (h *SettingsHandler) UpdateFamilySurname(c *gin.Context) {
	// 全部字段用指针：区分「未提交该字段」（保持原值）与「提交空串」（显式清空）。
	// 这样部分更新（如谱书封面页只提交 xiuci / title_with_xiuci）不会把堂号/郡望/祖籍等清空。
	var body struct {
		Surname              *string `json:"surname"`
		FamilyName           *string `json:"family_name"`
		Tanghao              *string `json:"tanghao"`
		Junwang              *string `json:"junwang"`
		BranchDesignation    *string `json:"branch_designation"`
		Zuji                 *string `json:"zuji"`
		ShizuName            *string `json:"shizu_name"`
		ShizuBio             *string `json:"shizu_bio"`
		ShiqianzuName        *string `json:"shiqianzu_name"`
		ShiqianzuPlace       *string `json:"shiqianzu_place"`
		TotemImage           *string `json:"totem_image"`
		TotemStyle           *string `json:"totem_style"`
		TotemFontSize        *int    `json:"totem_font_size"`
		DiejiShowSisters     *bool   `json:"dieji_show_sisters"`
		ChuizhuShowDaughters *bool   `json:"chuizhu_show_daughters"`
		Xiuci                string  `json:"xiuci"`
		TitleWithXiuci       *bool   `json:"title_with_xiuci"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 仅当该字段被提交时才写入（未提交 → 保持原值，实现字段级部分更新）
	setStr := func(key string, v *string, allowEmpty bool) {
		if v == nil {
			return
		}
		if *v == "" && !allowEmpty {
			return
		}
		h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`, key, *v)
	}
	setStr("family_surname", body.Surname, false) // 空串不覆盖（避免误清姓氏）
	setStr("family_name", body.FamilyName, false)
	setStr("tanghao", body.Tanghao, true) // 允许显式清空
	setStr("junwang", body.Junwang, true)
	setStr("branch_designation", body.BranchDesignation, true)
	setStr("zuji", body.Zuji, true)
	setStr("shizu_name", body.ShizuName, true)
	setStr("shizu_bio", body.ShizuBio, true)
	setStr("shiqianzu_name", body.ShiqianzuName, true)
	setStr("shiqianzu_place", body.ShiqianzuPlace, true)
	setStr("totem_image", body.TotemImage, true)
	setStr("totem_style", body.TotemStyle, true)
	if body.TotemFontSize != nil && *body.TotemFontSize > 0 {
		h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('totem_font_size', ?)`, strconv.Itoa(*body.TotemFontSize))
	}
	// 牒记式本支显示姐妹/女儿开关（不传则保持原值）
	if body.DiejiShowSisters != nil {
		v := "1"
		if !*body.DiejiShowSisters {
			v = "0"
		}
		h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('dieji_show_sisters', ?)`, v)
	}
	// 修次（空串表示不改动，保持原值）
	if body.Xiuci != "" {
		h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('xiuci', ?)`, body.Xiuci)
	}
	// 题名含修次开关（不传则保持原值）
	if body.TitleWithXiuci != nil {
		v := "0"
		if *body.TitleWithXiuci {
			v = "1"
		}
		h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('title_with_xiuci', ?)`, v)
	}
	// 苏氏垂珠体显示女儿开关（不传则保持原值）
	if body.ChuizhuShowDaughters != nil {
		v := "1"
		if !*body.ChuizhuShowDaughters {
			v = "0"
		}
		h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('chuizhu_show_daughters', ?)`, v)
	}
	deref := func(v *string) string {
		if v == nil {
			return ""
		}
		return *v
	}
	c.JSON(http.StatusOK, gin.H{
		"family_surname": deref(body.Surname), "family_name": deref(body.FamilyName),
		"tanghao": deref(body.Tanghao), "junwang": deref(body.Junwang), "branch_designation": deref(body.BranchDesignation), "zuji": deref(body.Zuji),
		"shizu_name": deref(body.ShizuName), "shizu_bio": deref(body.ShizuBio),
		"shiqianzu_name": deref(body.ShiqianzuName), "shiqianzu_place": deref(body.ShiqianzuPlace),
		"totem_image": deref(body.TotemImage), "totem_style": deref(body.TotemStyle),
	})
}

// ======================== 职位别名 ========================

// DefaultRoleNames 内部角色 → 默认显示名称
var DefaultRoleNames = map[string]string{
	RoleMember:      "族员",
	RoleElder:       "理事",
	RoleBranchElder: "房支理事",
	RoleFinance:     "财务管理",
	RoleChief:       "理事长",
	RoleSuperAdmin:  "超级管理员",
}

// LoadRoleAliases 从 settings 加载职位别名（仅返回与默认值不同的映射）
func LoadRoleAliases(db *sql.DB) map[string]string {
	var raw string
	db.QueryRow(`SELECT value FROM settings WHERE key = 'role_aliases'`).Scan(&raw)
	aliases := map[string]string{}
	if raw != "" {
		json.Unmarshal([]byte(raw), &aliases)
	}
	return aliases
}

// DisplayRole 返回角色的显示名称（优先别名）
func DisplayRole(aliases map[string]string, internal string) string {
	if a, ok := aliases[internal]; ok && a != "" {
		return a
	}
	if d, ok := DefaultRoleNames[internal]; ok {
		return d
	}
	return internal
}

// BuildRoleAliasesMap 构建完整显示名映射（别名优先，未设置用默认）
func BuildRoleAliasesMap(aliases map[string]string) map[string]string {
	m := make(map[string]string, len(DefaultRoleNames))
	for k := range DefaultRoleNames {
		if a, ok := aliases[k]; ok && a != "" {
			m[k] = a
		} else {
			m[k] = DefaultRoleNames[k]
		}
	}
	return m
}

// GetRoleAliases 获取职位别名（需登录）
func (h *SettingsHandler) GetRoleAliases(c *gin.Context) {
	aliases := LoadRoleAliases(h.DB)
	c.JSON(http.StatusOK, gin.H{
		"aliases":  BuildRoleAliasesMap(aliases),
		"defaults": DefaultRoleNames,
	})
}

// UpdateRoleAliases 更新职位别名
func (h *SettingsHandler) UpdateRoleAliases(c *gin.Context) {
	var body struct {
		Aliases map[string]string `json:"aliases"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	// 仅保存与默认值不同的别名
	filtered := map[string]string{}
	for k, v := range body.Aliases {
		if v == "" {
			continue
		}
		if d, ok := DefaultRoleNames[k]; !ok || d != v {
			filtered[k] = v
		}
	}

	raw, _ := json.Marshal(filtered)
	h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('role_aliases', ?)`, string(raw))
	c.JSON(http.StatusOK, gin.H{
		"message":  "职位别名已更新",
		"aliases":  BuildRoleAliasesMap(filtered),
		"defaults": DefaultRoleNames,
	})
}

// ======================== 隐私与权限矩阵 ========================

// GetPrivacyMatrix 获取隐私与权限矩阵
func (h *SettingsHandler) GetPrivacyMatrix(c *gin.Context) {
	c.JSON(http.StatusOK, PrivacyView(h.DB))
}

// UpdatePrivacyMatrix 保存隐私与权限矩阵
func (h *SettingsHandler) UpdatePrivacyMatrix(c *gin.Context) {
	var body privacyConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := SavePrivacy(h.DB, body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "隐私与权限已保存", "view": PrivacyView(h.DB)})
}

// SetDemoMode 设置演示模式（仅超级管理员）
// PUT /api/settings/demo-mode
func (h *SettingsHandler) SetDemoMode(c *gin.Context) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	v := "0"
	if body.Enabled {
		v = "1"
	}
	h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('demo_mode', ?)`, v)
	c.JSON(http.StatusOK, gin.H{"message": "演示模式已更新", "demo_mode": body.Enabled})
}

// GetDemoMode 获取演示模式状态
// GET /api/settings/demo-mode
func (h *SettingsHandler) GetDemoMode(c *gin.Context) {
	var v string
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'demo_mode'`).Scan(&v)
	enabled := v == "1" || v == "true" || v == "yes"
	c.JSON(http.StatusOK, gin.H{"demo_mode": enabled})
}
