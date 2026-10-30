package handler

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// setPrivacyForTest 直接替换内存中的隐私配置（不落库），并在用例结束后还原默认值。
func setPrivacyForTest(t *testing.T, mutate func(cfg *privacyConfig)) {
	t.Helper()
	cfg := defaultPrivacyConfig()
	if mutate != nil {
		mutate(&cfg)
	}
	privMu.Lock()
	privCfg = cfg
	privMu.Unlock()
	t.Cleanup(func() {
		privMu.Lock()
		privCfg = defaultPrivacyConfig()
		privMu.Unlock()
	})
}

// sampleSensitiveMember 一个「敏感字段全都有」的成员（默认矩阵下族员看到的应为打码/隐藏）
func sampleSensitiveMember() *Member {
	return &Member{
		ID:             100,
		Name:           "李测试",
		Branch:         "长房",
		BirthGregorian: "1977-09-07",
		NativePlace:    "福建省泉州市安溪县",
		CurrentPlace:   "福建省厦门市思明区",
		Title:          "高级教师",
		Occupation:     "教师",
		Education:      "本科",
		Biography:      "生平文本",
		BurialPlace:    "凤山",
		Portrait:       "/uploads/x.png",
		Phone:          "13812345678",
		DNAHaploid:     "O-M122-002611",
		DNADiploid:     "D8S1179/12/14",
		DNAMito:        "M9a1b2c3d4e5",
	}
}

func sameMember(a, b *Member) bool {
	return a.BirthGregorian == b.BirthGregorian &&
		a.NativePlace == b.NativePlace && a.CurrentPlace == b.CurrentPlace &&
		a.Title == b.Title && a.Occupation == b.Occupation && a.Education == b.Education &&
		a.Biography == b.Biography && a.BurialPlace == b.BurialPlace && a.Portrait == b.Portrait &&
		a.Phone == b.Phone && a.DNAHaploid == b.DNAHaploid && a.DNADiploid == b.DNADiploid && a.DNAMito == b.DNAMito
}

// 默认（public_all=false）：族员看五服内成员仍是「手机打码 + DNA 隐藏 + 籍贯隐藏」
func TestSanitizeMemberDefaultKeepsPrivacy(t *testing.T) {
	setPrivacyForTest(t, nil)
	m := sampleSensitiveMember()
	SanitizeMember(m, nil, RoleMember, 1, "", nil) // db=nil → 无法判定五服 → hide
	if m.Phone != "" {
		t.Fatalf("默认应隐藏手机号，实际 %q", m.Phone)
	}
	if m.DNAHaploid != "" || m.BirthGregorian != "" || m.NativePlace != "" || m.Biography != "" {
		t.Fatalf("默认应清空敏感字段，实际 %+v", m)
	}
}

// 总开关打开：同一成员、同一访问者，所有字段原样返回（服务端不脱敏）
func TestSanitizeMemberPublicAllRevealsEverything(t *testing.T) {
	setPrivacyForTest(t, func(cfg *privacyConfig) { cfg.Switches["public_all"] = true })
	want := sampleSensitiveMember()
	m := sampleSensitiveMember()
	SanitizeMember(m, nil, RoleMember, 1, "", nil)
	if !sameMember(m, want) {
		t.Fatalf("全公开后应原样返回\n got=%+v\nwant=%+v", m, want)
	}
	// 财务管理（默认恒 hidden 的分支）同样放开
	m2 := sampleSensitiveMember()
	SanitizeMember(m2, nil, RoleFinance, 1, "", nil)
	if !sameMember(m2, want) {
		t.Fatalf("全公开后财务管理也应全可见，got=%+v", m2)
	}
}

// 全公开只对「已登录用户」生效：游客（role == ""）仍按游客列脱敏
func TestSanitizeMemberPublicAllKeepsVisitorRestricted(t *testing.T) {
	setPrivacyForTest(t, func(cfg *privacyConfig) { cfg.Switches["public_all"] = true })
	m := sampleSensitiveMember()
	SanitizeMember(m, nil, "", 0, "", nil)
	if m.Phone != "" || m.BirthGregorian != "" || m.DNAHaploid != "" || m.NativePlace != "" {
		t.Fatalf("游客不应因全公开而看到敏感字段，实际 %+v", m)
	}
}

// 配偶脱敏同样受总开关控制
func TestSanitizeSpouseByRolePublicAll(t *testing.T) {
	setPrivacyForTest(t, func(cfg *privacyConfig) { cfg.Switches["public_all"] = true })
	s := &Spouse{Name: "王测试", Phone: "13987654321", Occupation: "医生", DNAHaploid: "N1a2b3c4d5", Biography: "文本"}
	orig := *s
	sanitizeSpouseByRole(s, RoleMember)
	if *s != orig {
		t.Fatalf("全公开后配偶字段应原样返回，got=%+v", *s)
	}
	// 关闭后恢复脱敏
	setPrivacyForTest(t, nil)
	s2 := &Spouse{Name: "王测试", Phone: "13987654321", Occupation: "医生", DNAHaploid: "N1a2b3c4d5"}
	sanitizeSpouseByRole(s2, RoleMember)
	if s2.DNAHaploid != "" {
		t.Fatalf("关闭总开关后 DNA 应隐藏，实际 %q", s2.DNAHaploid)
	}
	if s2.Phone != "139****4321" {
		t.Fatalf("关闭总开关后手机应打码，实际 %q", s2.Phone)
	}
}

// 默认值与保存/加载：public_all 默认 false；PUT 打开后落库并能被重新加载
func TestPrivacyPublicAllDefaultAndPersist(t *testing.T) {
	if defaultPrivacyConfig().Switches["public_all"] {
		t.Fatal("public_all 默认值必须为 false（默认保持隐私保护）")
	}
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatal(err)
	}
	cfg := defaultPrivacyConfig()
	cfg.Switches["public_all"] = true
	if err := SavePrivacy(db, cfg); err != nil {
		t.Fatal(err)
	}
	if !PrivacyPublicAll() {
		t.Fatal("保存 public_all=true 后应生效")
	}
	// 模拟重启：从 settings 重新加载
	privMu.Lock()
	privCfg = defaultPrivacyConfig()
	privMu.Unlock()
	LoadPrivacyConfig(db)
	if !PrivacyPublicAll() {
		t.Fatal("重启后从 settings 重新加载应保持 public_all=true")
	}
	// 复原
	cfg.Switches["public_all"] = false
	if err := SavePrivacy(db, cfg); err != nil {
		t.Fatal(err)
	}
	if PrivacyPublicAll() {
		t.Fatal("保存 public_all=false 后应关闭")
	}
}
