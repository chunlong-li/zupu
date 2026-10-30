package handler

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// 角色定义
// 游客：未登录，仅数据看板
// 族员：查看所有数据
// 理事：族员 + 添加修改成员数据
// 理事长：理事 + 任命罢免理事
// 超级管理员：所有权限 + 任命罢免理事长、理事
const (
	RoleMember      = "族员"
	RoleElder       = "理事"
	RoleBranchElder = "房支理事"
	RoleFinance     = "财务管理"
	RoleChief       = "理事长"
	RoleSuperAdmin  = "超级管理员"
)

// roleLevel 角色层级（数字越大权限越高）
// 财务管理：专项职位（收支公示的审核），层级与族员相当，额外权限由具体接口单独判定
var roleLevel = map[string]int{
	RoleMember:      1,
	RoleElder:       2,
	RoleBranchElder: 2,
	RoleFinance:     1,
	RoleChief:       3,
	RoleSuperAdmin:  4,
}

// AuthHandler 认证相关
type AuthHandler struct{ DB *sql.DB }

// 登录防爆破：内存级失败计数 + 锁定（外网开放后防暴力破解）
// 阈值与锁定时长可从 settings 表配置（login_max_failures / login_lock_minutes）
const (
	defaultLoginMaxFailures = 5                // 默认：同一 IP 连续失败次数阈值
	defaultLoginLockMinutes = 15               // 默认：锁定时长（分钟）
	loginResetWindow        = 15 * time.Minute // 失败计数有效期（超时自动清零）
)

// loginConfig 防爆破配置（运行时从 settings 表加载，默认 5 次/15 分钟）
var loginConfig = struct {
	sync.RWMutex
	maxFailures int
	lockWindow  time.Duration
}{
	maxFailures: defaultLoginMaxFailures,
	lockWindow:  defaultLoginLockMinutes * time.Minute,
}

// LoadLoginConfig 从 settings 表加载防爆破配置
func LoadLoginConfig(db *sql.DB) {
	var maxF string
	var lockM string
	db.QueryRow(`SELECT value FROM settings WHERE key = 'login_max_failures'`).Scan(&maxF)
	db.QueryRow(`SELECT value FROM settings WHERE key = 'login_lock_minutes'`).Scan(&lockM)

	loginConfig.Lock()
	defer loginConfig.Unlock()
	if n, err := strconv.Atoi(maxF); err == nil && n >= 1 && n <= 100 {
		loginConfig.maxFailures = n
	} else {
		loginConfig.maxFailures = defaultLoginMaxFailures
	}
	if n, err := strconv.Atoi(lockM); err == nil && n >= 1 && n <= 1440 {
		loginConfig.lockWindow = time.Duration(n) * time.Minute
	} else {
		loginConfig.lockWindow = defaultLoginLockMinutes * time.Minute
	}
}

// GetLoginConfig 返回当前防爆破配置（供设置页读取）
func GetLoginConfig() (int, int) {
	loginConfig.RLock()
	defer loginConfig.RUnlock()
	return loginConfig.maxFailures, int(loginConfig.lockWindow / time.Minute)
}

type loginAttempt struct {
	failures    int
	lastFail    time.Time
	lockedUntil time.Time
}

var loginLimiter = struct {
	sync.Mutex
	m map[string]*loginAttempt
}{m: make(map[string]*loginAttempt)}

// recordLoginFailure 记录一次登录失败，达到阈值则锁定
func recordLoginFailure(ip string) {
	loginLimiter.Lock()
	defer loginLimiter.Unlock()
	att, ok := loginLimiter.m[ip]
	if !ok {
		att = &loginAttempt{}
		loginLimiter.m[ip] = att
	}
	// 超过重置窗口则清零重新计数
	if !att.lastFail.IsZero() && time.Since(att.lastFail) > loginResetWindow {
		att.failures = 0
	}
	att.failures++
	att.lastFail = time.Now()
	loginConfig.RLock()
	maxF := loginConfig.maxFailures
	lockW := loginConfig.lockWindow
	loginConfig.RUnlock()
	if att.failures >= maxF {
		att.lockedUntil = time.Now().Add(lockW)
	}
}

// clearLoginFailures 登录成功后清除该 IP 的失败记录
func clearLoginFailures(ip string) {
	loginLimiter.Lock()
	defer loginLimiter.Unlock()
	delete(loginLimiter.m, ip)
}

// isIPLocked 检查该 IP 是否处于锁定状态（未锁定/已过期返回 false）
func isIPLocked(ip string) bool {
	loginLimiter.Lock()
	defer loginLimiter.Unlock()
	att, ok := loginLimiter.m[ip]
	if !ok {
		return false
	}
	return time.Now().Before(att.lockedUntil)
}

// 生成随机 token
func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// isBcryptHash 判断存储的密码是否为 bcrypt 哈希（60 位，$2a$/$2b$/$2y$ 开头）
func isBcryptHash(stored string) bool {
	return len(stored) == 60 && strings.HasPrefix(stored, "$2")
}

// 密码哈希（bcrypt，cost=12）
func hashPassword(pw string) string {
	// bcrypt 只处理前 72 字节，超长会被拒绝或静默截断，这里显式返回空以触发上层报错
	if len([]byte(pw)) > 72 {
		return ""
	}
	b, err := bcrypt.GenerateFromPassword([]byte(pw), 12)
	if err != nil {
		return ""
	}
	return string(b)
}

// isLegacySHA256 判断存储的密码是否为旧版 sha256 hex（64 位小写十六进制）
func isLegacySHA256(stored string) bool {
	if len(stored) != 64 {
		return false
	}
	for _, c := range stored {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// legacySHA256 计算旧版 sha256 hex（仅用于旧哈希登录比对与无缝迁移）
func legacySHA256(pw string) string {
	h := sha256.Sum256([]byte(pw))
	return hex.EncodeToString(h[:])
}

// verifyPassword 校验密码：优先 bcrypt；若库里是旧 sha256 则先按旧算法比对，命中后返回 needsRehash=true
func verifyPassword(stored, input string) (ok bool, needsRehash bool) {
	if stored == "" {
		return false, false
	}
	if isLegacySHA256(stored) {
		if stored == legacySHA256(input) {
			return true, true
		}
		return false, false
	}
	err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(input))
	return err == nil, false
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// UserInfo 用户信息
type UserInfo struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	RoleDisplay  string `json:"role_display"`
	Branch       string `json:"branch"`
	NeedChangePw bool   `json:"need_change_pw"`
}

// Login 用户登录
// POST /api/auth/login
// loginUser 解析出的登录账号（users 表一行，或按成员姓名自动建号后的结果）
type loginUser struct {
	ID           int
	Username     string
	Password     string // 存储的哈希（bcrypt / 旧 sha256）
	Role         string
	Branch       string
	NeedChangePw int
}

// resolveLoginUser 按用户名取账号；users 里没有时，若 members 表存在同名成员，
// 则按「姓名派生的初始密码」自动建号（角色=族员、首次登录须改密）——与族谱登录保持一致。
// found=false 表示成员表里也没有该名字，调用方按「用户名或密码错误」处理。
func (h *AuthHandler) resolveLoginUser(username string) (loginUser, bool, error) {
	var u loginUser
	err := h.DB.QueryRow(
		`SELECT id, username, password, role, COALESCE(branch,''), COALESCE(need_change_pw,0)
		 FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.Password, &u.Role, &u.Branch, &u.NeedChangePw)
	if err == nil {
		return u, true, nil
	}
	if err != sql.ErrNoRows {
		return u, false, err
	}
	// 用户不存在 → 查成员表，自动创建用户（首次登录）
	var memberName string
	if e := h.DB.QueryRow(`SELECT name FROM members WHERE name = ?`, username).Scan(&memberName); e != nil {
		if e == sql.ErrNoRows {
			return u, false, nil
		}
		return u, false, e
	}
	autoPw := deriveNamePassword(memberName, h.memberPwSalt())
	pwHash := hashPassword(autoPw)
	res, e := h.DB.Exec(`INSERT INTO users (username, password, role, need_change_pw) VALUES (?, ?, ?, 1)`,
		memberName, pwHash, RoleMember)
	if e != nil {
		return u, false, e
	}
	newID, _ := res.LastInsertId()
	return loginUser{ID: int(newID), Username: memberName, Password: pwHash, Role: RoleMember, Branch: "", NeedChangePw: 1}, true, nil
}

func (h *AuthHandler) Login(c *gin.Context) {
	// 防爆破：IP 被锁定则直接拒绝
	clientIP := c.ClientIP()
	if isIPLocked(clientIP) {
		_, lockMin := GetLoginConfig()
		writeLog(h.DB, 0, "登录失败", "账号", "IP 已锁定，拒绝登录尝试", "", clientIP)
		c.JSON(http.StatusTooManyRequests, gin.H{"error": fmt.Sprintf("登录失败次数过多，请%d分钟后再试", lockMin)})
		return
	}

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码不能为空"})
		return
	}

	u, found, err := h.resolveLoginUser(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !found {
		writeLog(h.DB, 0, "登录失败", "账号", "登录失败，用户不存在："+req.Username, req.Username, clientIP)
		recordLoginFailure(clientIP)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	id, username, password, role, branch, needChangePw := u.ID, u.Username, u.Password, u.Role, u.Branch, u.NeedChangePw

	rawPassword := req.Password
	ok, needsRehash := verifyPassword(password, rawPassword)
	if !ok {
		writeLog(h.DB, id, "登录失败", "账号", "登录失败，密码错误："+username, username, clientIP)
		recordLoginFailure(clientIP)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	// 旧 sha256 哈希命中后无缝迁移为 bcrypt
	if needsRehash {
		if nb := hashPassword(rawPassword); nb != "" {
			h.DB.Exec(`UPDATE users SET password = ? WHERE id = ?`, nb, id)
		}
	}

	// 登录成功，清除该 IP 的失败记录
	clearLoginFailures(clientIP)

	// 生成 token
	token := generateToken()
	h.DB.Exec(`UPDATE users SET token = ? WHERE id = ?`, token, id)

	// 记录登录成功日志
	writeLog(h.DB, id, "登录", "账号", "用户登录", username, clientIP)

	roleAliases := LoadRoleAliases(h.DB)

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": UserInfo{
			ID:           id,
			Username:     username,
			Role:         role,
			RoleDisplay:  DisplayRole(roleAliases, role),
			Branch:       branch,
			NeedChangePw: needChangePw == 1,
		},
		"role_aliases": BuildRoleAliasesMap(roleAliases),
	})
}

// VerifyRequest 仅校验账号密码（供传承服务器“用族谱账号登录”，不签发/不覆盖 token，避免顶号）
type VerifyRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Verify 只校验族谱账号密码，返回用户身份，**不修改 users.token**
// POST /api/auth/verify
func (h *AuthHandler) Verify(c *gin.Context) {
	clientIP := c.ClientIP()
	if isIPLocked(clientIP) {
		_, lockMin := GetLoginConfig()
		c.JSON(http.StatusTooManyRequests, gin.H{"error": fmt.Sprintf("登录失败次数过多，请%d分钟后再试", lockMin)})
		return
	}
	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码不能为空"})
		return
	}
	// 与族谱登录同一套解析逻辑：users 里没有时按成员姓名自动建号（否则「没登录过族谱」的成员在知识库登不进来）
	u, found, err := h.resolveLoginUser(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !found {
		recordLoginFailure(clientIP)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	id, username, role, branch := u.ID, u.Username, u.Role, u.Branch
	ok, needsRehash := verifyPassword(u.Password, req.Password)
	if !ok {
		recordLoginFailure(clientIP)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	if needsRehash {
		if nb := hashPassword(req.Password); nb != "" {
			h.DB.Exec(`UPDATE users SET password = ? WHERE id = ?`, nb, id)
		}
	}
	clearLoginFailures(clientIP)
	roleAliases := LoadRoleAliases(h.DB)
	c.JSON(http.StatusOK, gin.H{
		"ok": true,
		"user": gin.H{
			"id": id, "username": username, "role": role,
			"role_display": DisplayRole(roleAliases, role), "branch": branch,
		},
	})
}

// isDemoMode 检查是否为演示模式（禁止修改密码）
func (h *AuthHandler) isDemoMode() bool {
	var v string
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'demo_mode'`).Scan(&v)
	return v == "1" || v == "true" || v == "yes"
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword 修改密码
// POST /api/auth/change-password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := c.GetInt("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.NewPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码不能为空"})
		return
	}
	if len([]byte(req.NewPassword)) > 72 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码过长，最多 72 字节（约 24 个中文字符）"})
		return
	}

	// 演示模式：前端显示修改成功，后端不保存
	if h.isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
		return
	}

	var currentPw string
	err := h.DB.QueryRow(`SELECT password FROM users WHERE id = ?`, userID).Scan(&currentPw)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// need_change_pw=1 允许跳过旧密码（首次登录强制修改密码）
	// need_change_pw=0 必须校验旧密码
	var needChangePw int
	h.DB.QueryRow(`SELECT COALESCE(need_change_pw,0) FROM users WHERE id = ?`, userID).Scan(&needChangePw)
	if needChangePw == 0 {
		ok, _ := verifyPassword(currentPw, req.OldPassword)
		if req.OldPassword == "" || !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "旧密码错误"})
			return
		}
	}

	_, err = h.DB.Exec(`UPDATE users SET password = ?, need_change_pw = 0 WHERE id = ?`,
		hashPassword(req.NewPassword), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}

// Logout 登出
// POST /api/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	userID := c.GetInt("user_id")
	if userID > 0 {
		h.DB.Exec(`UPDATE users SET token = '' WHERE id = ?`, userID)
	}
	c.JSON(http.StatusOK, gin.H{"message": "已登出"})
}

// CheckAuth 检查登录状态
// GET /api/auth/check
func (h *AuthHandler) CheckAuth(c *gin.Context) {
	token := c.GetHeader("Authorization")
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimSpace(token)
	if token == "" {
		token = c.Query("token")
	}
	if token == "" {
		// 本机免登录：直接回一个「族员」身份，前端据此进入（无需输入账号密码）
		if isLocalGuest(c, h.DB) {
			roleAliases := LoadRoleAliases(h.DB)
			c.JSON(http.StatusOK, gin.H{
				"user": UserInfo{
					ID: 0, Username: "本机", Role: RoleMember,
					RoleDisplay: DisplayRole(roleAliases, RoleMember), Branch: "",
				},
				"role_aliases": roleAliases,
				"local_guest":  true,
			})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	var userID int
	var username, role, branch string
	var needChangePw int
	err := h.DB.QueryRow(
		`SELECT id, username, role, COALESCE(branch,''), COALESCE(need_change_pw,0)
		 FROM users WHERE token = ? AND token != ''`, token,
	).Scan(&userID, &username, &role, &branch, &needChangePw)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	roleAliases := LoadRoleAliases(h.DB)

	c.JSON(http.StatusOK, gin.H{
		"user": UserInfo{
			ID:           userID,
			Username:     username,
			Role:         role,
			RoleDisplay:  DisplayRole(roleAliases, role),
			Branch:       branch,
			NeedChangePw: needChangePw == 1,
		},
		"role_aliases": BuildRoleAliasesMap(roleAliases),
	})
}

// ListUsers 用户列表（族员及以上可查看）
// GET /api/users
func (h *AuthHandler) ListUsers(c *gin.Context) {
	rows, err := h.DB.Query(`SELECT id, username, role, COALESCE(branch,''), COALESCE(need_change_pw,0), created_at
		FROM users ORDER BY id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type UserRow struct {
		ID           int    `json:"id"`
		Username     string `json:"username"`
		Role         string `json:"role"`
		Branch       string `json:"branch"`
		NeedChangePw int    `json:"need_change_pw"`
		CreatedAt    string `json:"created_at"`
	}

	users := []UserRow{}
	for rows.Next() {
		var u UserRow
		rows.Scan(&u.ID, &u.Username, &u.Role, &u.Branch, &u.NeedChangePw, &u.CreatedAt)
		users = append(users, u)
	}
	c.JSON(http.StatusOK, users)
}

// memberPwSalt 本机专属密钥（首次使用时随机生成并持久化）—— 用于「按姓名派生密码」
// memberPwSalt 读取本机成员密码派生密钥（不存在则懒生成并持久化）
func memberPwSalt(db *sql.DB) string {
	var v string
	db.QueryRow(`SELECT value FROM settings WHERE key='member_pw_salt'`).Scan(&v)
	if v != "" {
		return v
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "familytree-default-salt"
	}
	v = hex.EncodeToString(b)
	db.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('member_pw_salt', ?)`, v)
	return v
}

func (h *AuthHandler) memberPwSalt() string { return memberPwSalt(h.DB) }

// deriveNamePassword 由姓名派生 12 位纯数字密码（同机同名结果固定；需本机密钥才能复算）
func deriveNamePassword(name, salt string) string {
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(strings.TrimSpace(name)))
	sum := mac.Sum(nil)
	out := make([]byte, 0, 12)
	for _, b := range sum {
		out = append(out, byte('0'+int(b)%10))
		if len(out) == 12 {
			break
		}
	}
	return string(out)
}

// randomNumeric 生成 n 位随机纯数字密码
func randomNumeric(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return strings.Repeat("0", n)
	}
	out := make([]byte, n)
	for i, b := range buf {
		out[i] = byte('0' + int(b)%10)
	}
	return string(out)
}

// CreateUser 创建用户（理事及以上可创建，角色不能高于自己）
// POST /api/users
func (h *AuthHandler) CreateUser(c *gin.Context) {
	callerRole := c.GetString("role")

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Branch   string `json:"branch"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名不能为空"})
		return
	}

	pw := strings.TrimSpace(req.Password)
	autoPw := pw == ""
	if autoPw {
		// 未填密码：按用户名（姓名）派生 12 位数字临时密码（需本机密钥才能复算）
		pw = deriveNamePassword(req.Username, h.memberPwSalt())
	}
	if len([]byte(pw)) > 72 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码过长，最多 72 字节（约 24 个中文字符）"})
		return
	}
	role := req.Role
	if role == "" {
		role = RoleMember
	}

	// 校验角色合法性
	if _, ok := roleLevel[role]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的角色"})
		return
	}
	// 不能创建高于自己层级的角色
	if roleLevel[role] >= roleLevel[callerRole] {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权创建该角色用户"})
		return
	}

	// 支系只能指定给理事/房支理事，且房支名需存在；指定支系的理事自动升级为房支理事
	branch := strings.TrimSpace(req.Branch)
	if branch != "" && role != RoleElder && role != RoleBranchElder {
		c.JSON(http.StatusBadRequest, gin.H{"error": "支系只能指定给理事"})
		return
	}
	if branch != "" && role == RoleElder {
		role = RoleBranchElder
	}
	if branch != "" {
		var cnt int
		if err := h.DB.QueryRow(`SELECT COUNT(*) FROM branches WHERE name = ?`, branch).Scan(&cnt); err != nil || cnt == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "房支不存在"})
			return
		}
	}

	if _, err := h.DB.Exec(`INSERT INTO users (username, password, role, branch, need_change_pw)
		VALUES (?, ?, ?, ?, 1)`, req.Username, hashPassword(pw), role, branch); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "用户创建成功", "password": pw, "auto_generated": autoPw})
}

// SetRole 任命/罢免：更新用户角色（理事长可任命罢免理事，超级管理员可任命罢免理事长、理事）
// PUT /api/users/:id/role
func (h *AuthHandler) SetRole(c *gin.Context) {
	callerRole := c.GetString("role")
	callerID := c.GetInt("user_id")
	targetID := c.Param("id")

	var req struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if _, ok := roleLevel[req.Role]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的角色"})
		return
	}

	// 不能操作自己
	if targetID == strconv.Itoa(callerID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能修改自己的角色"})
		return
	}

	// 查询目标用户当前角色
	var targetRole string
	err := h.DB.QueryRow(`SELECT role FROM users WHERE id = ?`, targetID).Scan(&targetRole)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 权限校验：调用者层级必须高于目标角色层级（即只能任命比自己低的角色）
	if roleLevel[req.Role] >= roleLevel[callerRole] {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权任命该角色"})
		return
	}

	// 防止超级管理员降级其他超级管理员
	if targetRole == RoleSuperAdmin && callerRole == RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "不能修改其他超级管理员的角色"})
		return
	}

	_, err = h.DB.Exec(`UPDATE users SET role = ? WHERE id = ?`, req.Role, targetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "角色已更新"})
}

// SetBranch 设置用户所属房支（理事长及以上）
// 给理事分配支系 → 自动升级为房支理事；给房支理事清除支系 → 自动降回理事
// PUT /api/users/:id/branch   body: {"branch": "房支名"}
func (h *AuthHandler) SetBranch(c *gin.Context) {
	callerRole := c.GetString("role")
	targetID := c.Param("id")

	var req struct {
		Branch string `json:"branch"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	branch := strings.TrimSpace(req.Branch)

	// 仅理事长及以上可设置支系
	if roleLevel[callerRole] < roleLevel[RoleChief] {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅理事长及以上可设置支系"})
		return
	}

	// 支系只能指定给理事/房支理事
	var targetRole string
	if err := h.DB.QueryRow(`SELECT role FROM users WHERE id = ?`, targetID).Scan(&targetRole); err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if branch != "" && targetRole != RoleElder && targetRole != RoleBranchElder {
		c.JSON(http.StatusBadRequest, gin.H{"error": "支系只能指定给理事"})
		return
	}

	// 若指定支系，校验房支名存在
	if branch != "" {
		var cnt int
		if err := h.DB.QueryRow(`SELECT COUNT(*) FROM branches WHERE name = ?`, branch).Scan(&cnt); err != nil || cnt == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "房支不存在"})
			return
		}
	}

	// 分配支系 → 自动升级理事为房支理事；清除支系 → 自动降回理事
	newRole := targetRole
	if branch != "" && targetRole == RoleElder {
		newRole = RoleBranchElder
	} else if branch == "" && targetRole == RoleBranchElder {
		newRole = RoleElder
	}
	if newRole != targetRole {
		_, err := h.DB.Exec(`UPDATE users SET role = ?, branch = ? WHERE id = ?`, newRole, branch, targetID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		_, err := h.DB.Exec(`UPDATE users SET branch = ? WHERE id = ?`, branch, targetID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "支系已设置"})
}

// ResetPassword 重置密码（理事长及以上可操作，超级管理员可重置理事长）
// POST /api/users/:id/reset-password
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	// 演示模式：前端显示成功，后端不保存
	if h.isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "密码已重置", "password": randomNumeric(12)})
		return
	}

	id := c.Param("id")

	// 不允许越权重置超级管理员密码（除非调用者本人也是超级管理员）
	callerRole := c.GetString("role")
	var targetRole string
	if err := h.DB.QueryRow(`SELECT role FROM users WHERE id = ?`, id).Scan(&targetRole); err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if targetRole == RoleSuperAdmin && callerRole != RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权重置超级管理员的密码"})
		return
	}

	// 随机生成 12 位数字临时密码，首次登录需修改
	pw := randomNumeric(12)
	if _, err := h.DB.Exec(`UPDATE users SET password = ?, need_change_pw = 1 WHERE id = ?`,
		hashPassword(pw), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "密码已重置", "password": pw})
}

// DeleteUser 删除用户（仅超级管理员可操作，不能删除自己或其他超级管理员）
// DELETE /api/users/:id
func (h *AuthHandler) DeleteUser(c *gin.Context) {
	callerRole := c.GetString("role")
	callerID := c.GetInt("user_id")
	targetID := c.Param("id")

	// 仅超级管理员可删除
	if callerRole != RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅超级管理员可删除用户"})
		return
	}

	// 不能删除自己
	if targetID == strconv.Itoa(callerID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除自己的账号"})
		return
	}

	// 查询目标用户角色
	var targetRole string
	err := h.DB.QueryRow(`SELECT role FROM users WHERE id = ?`, targetID).Scan(&targetRole)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 不能删除其他超级管理员
	if targetRole == RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "不能删除超级管理员账号"})
		return
	}

	_, err = h.DB.Exec(`DELETE FROM users WHERE id = ?`, targetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "用户已删除"})
}

// AuthMiddleware 认证中间件：验证 token，设置 user_id 和 role
func AuthMiddleware(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			token = c.Query("token")
		}
		if token == "" {
			// 本机（127.0.0.1/::1）免登录：按「族员」身份放行，权限与普通族员一致（只读为主）
			if isLocalGuest(c, db) {
				c.Set("user_id", 0)
				c.Set("role", RoleMember)
				c.Set("username", "本机")
				c.Set("branch", "")
				c.Set("local_guest", true)
				c.Next()
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			c.Abort()
			return
		}

		token = strings.TrimPrefix(token, "Bearer ")
		token = strings.TrimSpace(token)

		var userID int
		var role string
		var username string
		var branch string
		err := db.QueryRow(`SELECT id, role, COALESCE(username,''), COALESCE(branch,'') FROM users WHERE token = ? AND token != ''`, token).Scan(&userID, &role, &username, &branch)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "登录已过期，请重新登录"})
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Set("role", role)
		c.Set("username", username)
		c.Set("branch", branch)
		c.Next()
	}
}

// isLoopback 判断请求是否来自本机。
// 只用 TCP 对端地址（RemoteAddr），不采信可伪造的 X-Forwarded-For；经反向代理转发的请求对端是代理地址（非 127.0.0.1）→ 不会误判为本机。
func isLoopback(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		host = strings.TrimSpace(remoteAddr)
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// LocalNoAuthEnabled 本机免登录开关。取值优先级：
//
//	① 环境变量 FAMILY_TREE_LOCAL_NO_AUTH：=1 强制开、=0 强制关（便于运维临时覆盖）；
//	② 否则读设置表 local_no_auth —— 由「服务器设置 → 本机访问」页的开关写入（仅理事长/超管可改），**默认关**。
//
// **默认关闭**的原因：若族谱被同机反向代理（如 Lucky）转发，代理过来的请求对端就是 127.0.0.1，
// 一旦开启会把所有经代理访问的远端用户都当作本机访客（免登录的「族员」）——存在越权风险。
func LocalNoAuthEnabled(db *sql.DB) bool {
	switch os.Getenv("FAMILY_TREE_LOCAL_NO_AUTH") {
	case "1":
		return true
	case "0":
		return false
	}
	if db == nil {
		return false
	}
	var v string
	if err := db.QueryRow(`SELECT value FROM settings WHERE key = 'local_no_auth'`).Scan(&v); err != nil {
		return false
	}
	return v == "1"
}

// isLocalGuest 本次请求是否应作为「本机访客」放行（本机 + 未带 token + 开关打开）
func isLocalGuest(c *gin.Context, db *sql.DB) bool {
	return LocalNoAuthEnabled(db) && isLoopback(c.Request.RemoteAddr)
}

// RequireRole 角色权限中间件：要求角色层级 >= minRole
func RequireRole(minRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		if roleLevel[role] < roleLevel[minRole] {
			c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequirePriv 隐私与权限矩阵中间件：要求用户在矩阵中启用该项目
func RequirePriv(item string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !PrivAllowed(item, c.GetString("role")) {
			c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// EnsureUserTable 确保用户表包含必要列，并做角色迁移
func EnsureUserTable(db *sql.DB) {
	db.Exec(`ALTER TABLE users ADD COLUMN token VARCHAR(128) DEFAULT ''`)
	db.Exec(`ALTER TABLE users ADD COLUMN need_change_pw INTEGER DEFAULT 1`)

	// 角色迁移：旧的"普通编辑"映射为"族员"
	db.Exec(`UPDATE users SET role = ? WHERE role = ?`, RoleMember, "普通编辑")
	// 角色迁移：旧的"长老"映射为"理事"，"族长"映射为"理事长"
	db.Exec(`UPDATE users SET role = ? WHERE role = ?`, RoleElder, "长老")
	db.Exec(`UPDATE users SET role = ? WHERE role = ?`, RoleChief, "族长")
	// 角色迁移：旧的"家主"更名为"理事长"
	db.Exec(`UPDATE users SET role = ? WHERE role = ?`, RoleChief, "家主")

	// 确保默认管理员密码已哈希（仅当既非 bcrypt 也非旧 sha256 时才重置，避免误改已迁移/自定义密码）
	var pw string
	err := db.QueryRow(`SELECT password FROM users WHERE username = 'admin'`).Scan(&pw)
	if err == nil && !isBcryptHash(pw) && !isLegacySHA256(pw) {
		db.Exec(`UPDATE users SET password = ?, need_change_pw = 1 WHERE username = 'admin'`,
			hashPassword("admin123"))
	}
}
