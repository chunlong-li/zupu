package handler

import (
	"database/sql"
	"encoding/json"
	"sync"
)

// ============================================================================
// 隐私与权限体系
//   2026-09-13 初版：三维模型（字段敏感度 × 人物状态 × 访问者关系）
//   2026-10-02 改为「可配置矩阵」：项目 × 职位，前台可勾选（settings: privacy_matrix）
//   2026-10-17 新增总开关 public_all「取消隐私 · 全公开」：默认关闭；
//              打开后对**任何已登录用户**（role != ""）一律不脱敏——在世者脱敏、五服分级、
//              已故者职位门限、矩阵中「信息可见性」各项（含 DNA / 手机号）全部失效。
//              未登录游客（role == ""）不受该开关影响，仍按矩阵「游客」列；
//              「操作权限」（op 组：审核/备份/账号管理…）也不受影响，避免借全公开提权。
// ============================================================================

// FieldSensitivity 访问者可见等级（保留：审计日志等处仍用）
type FieldSensitivity int

const (
	SensPublic        FieldSensitivity = 0 // 公开
	SensFamily        FieldSensitivity = 1 // 家族可见
	SensCloseRelative FieldSensitivity = 2 // 直系近亲可见
	SensAdmin         FieldSensitivity = 3 // 仅管理员
)

// viewerSensitivity 访问者可见等级（用于审计日志是否记录）
func viewerSensitivity(role string) FieldSensitivity {
	switch role {
	case RoleSuperAdmin, RoleChief, RoleElder:
		return SensAdmin
	case RoleBranchElder, RoleMember:
		return SensFamily
	default:
		return SensPublic
	}
}

// ============================================================================
// 隐私矩阵
// ============================================================================

// 游客列（未登录）用固定 key
const PrivVisitor = "游客"

// PrivacyColumns 矩阵列顺序（职位）
var PrivacyColumns = []string{PrivVisitor, RoleMember, RoleBranchElder, RoleElder, RoleFinance, RoleChief, RoleSuperAdmin}

// 布尔项取值：0=不可见，2=可见；分级项额外有 1=打码
const (
	PrivHidden = 0
	PrivMasked = 1
	PrivFull   = 2
)

// privItemMeta 矩阵项目定义
type privItemMeta struct {
	Key   string
	Label string
	Group string // info=信息可见性 / op=操作权限
	Level bool   // true=0/1/2 三级（可打码）；false=0/2 两级
}

// PrivItemMetas 矩阵项目（顺序即前台展示顺序）
var PrivItemMetas = []privItemMeta{
	{"base", "世系框架（姓名/性别/世代/字辈/房支/字号）", "info", false},
	{"dates", "生卒日期 · 年龄", "info", false},
	{"bio", "生平 · 传记", "info", false},
	{"photo", "照片 · 头像", "info", false},
	{"occupation", "职业 · 学历 · 称号", "info", false},
	{"burial", "葬地 · 墓址", "info", false},
	{"spouse", "配偶信息", "info", false},
	{"places", "籍贯 · 现居地", "info", false},
	{"phone", "手机号", "info", true},
	{"spouse_phone", "配偶手机号", "info", true},
	{"dna", "DNA 编码", "info", true},
	{"accounts_view", "收支账目（查看）", "op", false},
	{"member_edit", "提交新增 / 修改", "op", false},
	{"review", "审核中心", "op", false},
	{"finance_audit", "收支公示审核", "op", false},
	{"export", "成员导出 / GEDCOM", "op", false},
	{"backup", "数据备份 / 恢复", "op", false},
	{"account_view", "账号列表（查看）", "op", false},
	{"account_create", "创建账号", "op", false},
	{"account_manage", "任命 / 罢免 / 重置密码", "op", false},
	{"account_delete", "删除账号", "op", false},
	{"system_settings", "系统 / 家族设置", "op", false},
	{"kb", "家族知识库", "op", false},
	{"audit_log", "操作日志 / 审计", "op", false},
}

// privacyConfig 持久化结构
type privacyConfig struct {
	Items    map[string]map[string]int `json:"items"`
	Switches map[string]bool           `json:"switches"`
}

var (
	privMu  sync.RWMutex
	privCfg = defaultPrivacyConfig()
)

// 默认矩阵（列序：游客 族员 房支理事 理事 财务管理 理事长 超级管理员）
var defaultPrivRows = map[string][]int{
	"base":            {2, 2, 2, 2, 2, 2, 2},
	"dates":           {0, 2, 2, 2, 0, 2, 2},
	"bio":             {0, 2, 2, 2, 0, 2, 2},
	"photo":           {0, 2, 2, 2, 0, 2, 2},
	"occupation":      {0, 2, 2, 2, 0, 2, 2},
	"burial":          {0, 2, 2, 2, 0, 2, 2},
	"spouse":          {0, 2, 2, 2, 0, 2, 2},
	"places":          {0, 0, 2, 2, 0, 2, 2},
	"phone":           {0, 1, 1, 1, 0, 2, 2},
	"spouse_phone":    {0, 1, 1, 1, 0, 2, 2},
	"dna":             {0, 0, 0, 0, 0, 2, 2},
	"accounts_view":   {0, 2, 2, 2, 2, 2, 2},
	"member_edit":     {0, 0, 2, 2, 0, 2, 2},
	"review":          {0, 0, 0, 0, 0, 2, 2},
	"finance_audit":   {0, 0, 0, 0, 2, 2, 2},
	"export":          {0, 0, 0, 0, 0, 2, 2},
	"backup":          {0, 0, 0, 0, 0, 2, 2},
	"account_view":    {0, 0, 0, 0, 0, 2, 2},
	"account_create":  {0, 0, 0, 0, 0, 2, 2},
	"account_manage":  {0, 0, 0, 0, 0, 2, 2},
	"account_delete":  {0, 0, 0, 0, 0, 0, 2},
	"system_settings": {0, 0, 0, 0, 0, 2, 2},
	"kb":              {0, 2, 2, 2, 2, 2, 2},
	"audit_log":       {0, 0, 0, 0, 0, 2, 2},
}

func defaultPrivacyConfig() privacyConfig {
	items := map[string]map[string]int{}
	for k, row := range defaultPrivRows {
		m := map[string]int{}
		for i, col := range PrivacyColumns {
			m[col] = row[i]
		}
		items[k] = m
	}
	return privacyConfig{
		Items: items,
		// five_fu / dead_public 默认开启（＝保持隐私保护）；
		// public_all「取消隐私 · 全公开」默认关闭（＝维持既有隐私行为）。
		Switches: map[string]bool{"five_fu": true, "dead_public": true, "public_all": false},
	}
}

// LoadPrivacyConfig 从 settings 加载矩阵（未配置则用默认）
func LoadPrivacyConfig(db *sql.DB) {
	cfg := defaultPrivacyConfig()
	if db != nil {
		var raw string
		db.QueryRow(`SELECT value FROM settings WHERE key = 'privacy_matrix'`).Scan(&raw)
		if raw != "" {
			var saved privacyConfig
			if json.Unmarshal([]byte(raw), &saved) == nil {
				for k, row := range saved.Items {
					if _, ok := cfg.Items[k]; !ok {
						continue
					}
					for col, v := range row {
						if _, ok := cfg.Items[k][col]; !ok {
							continue
						}
						if v < 0 {
							v = 0
						}
						if v > 2 {
							v = 2
						}
						cfg.Items[k][col] = v
					}
				}
				for k, v := range saved.Switches {
					if _, ok := cfg.Switches[k]; ok {
						cfg.Switches[k] = v
					}
				}
			}
		}
	}
	// 锁定：世系框架 & 超级管理员列
	forceLockPrivacy(&cfg)
	privMu.Lock()
	privCfg = cfg
	privMu.Unlock()
}

// forceLockPrivacy 强制约束（防锁死）
func forceLockPrivacy(cfg *privacyConfig) {
	if row, ok := cfg.Items["base"]; ok {
		for _, col := range PrivacyColumns {
			row[col] = PrivFull
		}
	}
	// 超级管理员列全开（系统设置/账号删除等，避免唯一超管被锁死）
	for _, it := range PrivItemMetas {
		if row, ok := cfg.Items[it.Key]; ok {
			row[RoleSuperAdmin] = PrivFull
		}
	}
}

// PrivRoleKey 角色 → 矩阵列 key（"" 视为游客）
func PrivRoleKey(role string) string {
	if role == "" {
		return PrivVisitor
	}
	return role
}

// privLevel 取某项目对某职位的可见级别
func privLevel(item, role string) int {
	privMu.RLock()
	defer privMu.RUnlock()
	if row, ok := privCfg.Items[item]; ok {
		if v, ok := row[PrivRoleKey(role)]; ok {
			return v
		}
	}
	return 0
}

// PrivAllowed 操作权限：是否允许
func PrivAllowed(item, role string) bool {
	return privLevel(item, role) > 0
}

func privSwitch(name string) bool {
	privMu.RLock()
	defer privMu.RUnlock()
	return privCfg.Switches[name]
}

// PrivacyPublicAll 总开关「取消隐私 · 全公开」当前是否打开
// 打开后服务端所有「信息可见性」脱敏一律跳过（对已登录用户），详见 SanitizeMember。
func PrivacyPublicAll() bool { return privSwitch("public_all") }

// privPublicAllFor 该访问者是否适用「全公开」：
// 仅对已登录用户（role != ""）生效；未登录游客仍按矩阵「游客」列脱敏。
func privPublicAllFor(role string) bool {
	return role != "" && privSwitch("public_all")
}

// CurrentPrivacy 返回当前矩阵副本（供保存/展示）
func CurrentPrivacy() privacyConfig {
	privMu.RLock()
	defer privMu.RUnlock()
	out := privacyConfig{Items: map[string]map[string]int{}, Switches: map[string]bool{}}
	for k, row := range privCfg.Items {
		m := map[string]int{}
		for c, v := range row {
			m[c] = v
		}
		out.Items[k] = m
	}
	for k, v := range privCfg.Switches {
		out.Switches[k] = v
	}
	return out
}

// ============================================================================
// 脱敏
// ============================================================================

// IsAlive 判断成员是否在世（无卒年 = 在世）
func (m *Member) IsAlive() bool {
	return m.DeathGregorian == ""
}

// MaskPhone 手机号脱敏：138****1234
func MaskPhone(phone string) string {
	if phone == "" {
		return phone
	}
	runes := []rune(phone)
	n := len(runes)
	if n <= 7 {
		return phone
	}
	return string(runes[:3]) + "****" + string(runes[n-4:])
}

// MaskDNA DNA 编码脱敏：保留前 4 位与后 4 位，中间打码
func MaskDNA(code string) string {
	if code == "" {
		return code
	}
	r := []rune(code)
	n := len(r)
	if n <= 8 {
		return "********"
	}
	return string(r[:4]) + "****" + string(r[n-4:])
}

// applyMemberLevel 按级别处理成员敏感字段
func applyMemberLevel(m *Member, item string, lvl int) {
	switch item {
	case "dates":
		if lvl == 0 {
			m.BirthCalendar = ""
			m.BirthGregorian = ""
			m.BirthLunar = ""
			m.BirthGanzhi = ""
			m.BirthEra = ""
			m.DeathGregorian = ""
			m.DeathLunar = ""
			m.DeathGanzhi = ""
			m.DeathEra = ""
			m.Lifespan = 0
		}
	case "bio":
		if lvl == 0 {
			m.Biography = ""
			m.MotherID = 0
			m.MotherSpouseID = 0
			m.FosterFatherID = 0
			m.MarriedOut = ""
		}
	case "photo":
		if lvl == 0 {
			m.Portrait = ""
		}
	case "occupation":
		if lvl == 0 {
			m.Title = ""
			m.Occupation = ""
			m.Education = ""
		}
	case "burial":
		if lvl == 0 {
			m.BurialPlace = ""
		}
	case "places":
		if lvl == 0 {
			m.NativePlace = ""
			m.CurrentPlace = ""
		}
	case "phone":
		switch lvl {
		case 0:
			m.Phone = ""
		case 1:
			m.Phone = MaskPhone(m.Phone)
		}
	case "dna":
		switch lvl {
		case 0:
			m.DNAHaploid = ""
			m.DNADiploid = ""
			m.DNAMito = ""
		case 1:
			m.DNAHaploid = MaskDNA(m.DNAHaploid)
			m.DNADiploid = MaskDNA(m.DNADiploid)
			m.DNAMito = MaskDNA(m.DNAMito)
		}
	}
}

// SanitizeMember 统一隐私脱敏（列表 + 详情共用）
// spouses 可选：传入需同步脱敏的配偶列表
// 规则来源：隐私矩阵（settings.privacy_matrix）
// 关系维度：族员/房支理事(非本房支) 叠加「五服」判断（开关 five_fu）
// 在世维度：逝世者信息一律可见（开关 dead_public）
// 总开关 public_all 打开时：已登录用户直接全量返回（不脱敏任何字段，含配偶），
// 未登录游客仍按矩阵「游客」列处理。
func SanitizeMember(m *Member, spouses *[]Spouse, viewerRole string, viewerID int64, viewerBranch string, db *sql.DB) {
	role := viewerRole

	// 总开关：「取消隐私 · 全公开」—— 在世者脱敏、五服分级、已故者门限、矩阵信息项全部失效。
	if privPublicAllFor(role) {
		return
	}

	hide := false
	forceFull := false

	ownBranch := role == RoleBranchElder && viewerBranch != "" && m.Branch != "" && viewerBranch == m.Branch
	fiveFuRole := role == RoleMember || (role == RoleBranchElder && !ownBranch)

	if role == "" || role == RoleFinance {
		hide = true
	}
	if fiveFuRole {
		if !privSwitch("five_fu") {
			// 关闭五服分级：整列放开（不再按关系隐藏）
		} else if viewerID <= 0 || m.ID <= 0 || db == nil || !WithinFiveFu(db, viewerID, m.ID) {
			hide = true
		} else if !m.IsAlive() && privSwitch("dead_public") {
			forceFull = true
		}
	}

	lvl := func(item string) int {
		if hide {
			return PrivHidden
		}
		l := privLevel(item, role)
		if forceFull && item != "dna" && item != "phone" && item != "spouse_phone" {
			l = PrivFull
		}
		return l
	}

	applyMemberLevel(m, "dates", lvl("dates"))
	applyMemberLevel(m, "bio", lvl("bio"))
	applyMemberLevel(m, "photo", lvl("photo"))
	applyMemberLevel(m, "occupation", lvl("occupation"))
	applyMemberLevel(m, "burial", lvl("burial"))
	applyMemberLevel(m, "places", lvl("places"))
	applyMemberLevel(m, "phone", lvl("phone"))
	applyMemberLevel(m, "dna", lvl("dna"))

	if spouses != nil {
		spLvl := lvl("spouse")
		spPhone := lvl("spouse_phone")
		dnaLvl := lvl("dna")
		for j := range *spouses {
			s := &(*spouses)[j]
			if spLvl == 0 {
				s.Portrait = ""
				s.Occupation = ""
				s.Education = ""
				s.Biography = ""
			}
			switch spPhone {
			case 0:
				s.Phone = ""
			case 1:
				s.Phone = MaskPhone(s.Phone)
			}
			switch dnaLvl {
			case 0:
				s.DNAHaploid = ""
				s.DNADiploid = ""
				s.DNAMito = ""
			case 1:
				s.DNAHaploid = MaskDNA(s.DNAHaploid)
				s.DNADiploid = MaskDNA(s.DNADiploid)
				s.DNAMito = MaskDNA(s.DNAMito)
			}
		}
	}
}

// SanitizeMembers 批量脱敏
func SanitizeMembers(members []Member, viewerRole string, viewerID int64, viewerBranch string, db *sql.DB) {
	for i := range members {
		SanitizeMember(&members[i], nil, viewerRole, viewerID, viewerBranch, db)
	}
}

// sanitizeSpouseByRole 独立配偶接口的脱敏（无成员上下文，不叠加五服）
func sanitizeSpouseByRole(s *Spouse, role string) {
	// 总开关：全公开时对已登录用户不脱敏（游客仍受限）
	if privPublicAllFor(role) {
		return
	}
	if role == "" || role == RoleFinance {
		s.Portrait = ""
		s.Occupation = ""
		s.Education = ""
		s.Biography = ""
		s.Phone = ""
		s.DNAHaploid = ""
		s.DNADiploid = ""
		s.DNAMito = ""
		return
	}
	if privLevel("spouse", role) == 0 {
		s.Portrait = ""
		s.Occupation = ""
		s.Education = ""
		s.Biography = ""
	}
	switch privLevel("spouse_phone", role) {
	case 0:
		s.Phone = ""
	case 1:
		s.Phone = MaskPhone(s.Phone)
	}
	switch privLevel("dna", role) {
	case 0:
		s.DNAHaploid = ""
		s.DNADiploid = ""
		s.DNAMito = ""
	case 1:
		s.DNAHaploid = MaskDNA(s.DNAHaploid)
		s.DNADiploid = MaskDNA(s.DNADiploid)
		s.DNAMito = MaskDNA(s.DNAMito)
	}
}

// PrivacyView 生成前台展示用的矩阵视图
func PrivacyView(db *sql.DB) map[string]interface{} {
	cfg := CurrentPrivacy()
	aliases := LoadRoleAliases(db)
	roleNames := map[string]string{}
	for _, col := range PrivacyColumns {
		if col == PrivVisitor {
			roleNames[col] = "游客"
		} else {
			roleNames[col] = DisplayRole(aliases, col)
		}
	}
	items := []map[string]interface{}{}
	for _, meta := range PrivItemMetas {
		items = append(items, map[string]interface{}{
			"key":    meta.Key,
			"label":  meta.Label,
			"group":  meta.Group,
			"level":  meta.Level,
			"values": cfg.Items[meta.Key],
		})
	}
	// 系统默认值（供前端「恢复默认设置」按钮使用：只回填表单，不落库）
	dft := defaultPrivacyConfig()
	forceLockPrivacy(&dft)
	return map[string]interface{}{
		"columns":     PrivacyColumns,
		"role_names":  roleNames,
		"items":       items,
		"switches":    cfg.Switches,
		"public_all":  cfg.Switches["public_all"],
		"defaults":    map[string]interface{}{"items": dft.Items, "switches": dft.Switches},
		"visitor_col": PrivVisitor,
	}
}

// SavePrivacy 保存矩阵（仅接受已知项目/列；强制约束）
func SavePrivacy(db *sql.DB, in privacyConfig) error {
	cfg := defaultPrivacyConfig()
	for k, row := range in.Items {
		if _, ok := cfg.Items[k]; !ok {
			continue
		}
		for col, v := range row {
			if _, ok := cfg.Items[k][col]; !ok {
				continue
			}
			if v < 0 {
				v = 0
			}
			if v > 2 {
				v = 2
			}
			cfg.Items[k][col] = v
		}
	}
	for k, v := range in.Switches {
		if _, ok := cfg.Switches[k]; ok {
			cfg.Switches[k] = v
		}
	}
	forceLockPrivacy(&cfg)
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('privacy_matrix', ?)`, string(raw)); err != nil {
		return err
	}
	LoadPrivacyConfig(db)
	return nil
}
