package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// postBodyName 从 POST/PUT 请求体读取名字（成员/配偶/用户的 name/username），
// 读取后会把 body 重置回去，不影响后续 handler 解析。
func postBodyName(c *gin.Context) string {
	if c.Request.Body == nil {
		return ""
	}
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	// 重置 body 供 handler 读取
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	var m map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		return ""
	}
	for _, k := range []string{"name", "username", "given_name", "member_name", "owner_name"} {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

// AuditLog 记录登录用户对系统的关键操作（新增/修改/删除/登录等）。
// 通过中间件自动采集，无需在各 handler 中埋点。
type AuditLog struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	Action    string `json:"action"` // 登录/登录失败/新增/修改/删除/审核/备份/还原/更新 等
	Target    string `json:"target"` // 操作对象类型，如"成员""配偶""用户"
	Detail    string `json:"detail"` // 可读描述
	IP        string `json:"ip"`
	CreatedAt string `json:"created_at"`
}

// LogRetentionKey 操作日志保留天数的 settings 键；0/负值表示关闭自动清理。
const LogRetentionKey = "oplog_retention_days"

// getLogRetentionDays 读取保留天数设置：默认 180；<=0 表示关闭自动清理。
func getLogRetentionDays(db *sql.DB) int {
	var v string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`, LogRetentionKey).Scan(&v)
	if err != nil {
		return 180 // 默认保留 180 天
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 180
	}
	return n
}

// purgeOldLogs 删除超过保留天数的日志；低频触发（仅在写日志时、距上次清理>1h 才执行）。
var lastPurge time.Time

func purgeOldLogs(db *sql.DB) {
	if time.Since(lastPurge) < time.Hour {
		return
	}
	lastPurge = time.Now()
	days := getLogRetentionDays(db)
	if days <= 0 {
		return // 关闭自动清理
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	_, _ = db.Exec(`DELETE FROM operation_logs WHERE created_at < ?`,
		cutoff.UTC().Format("2006-01-02 15:04:05"))
}

// writeLog 写入一条操作日志，并顺带触发低频过期清理。
// 存储格式：detail = "业务描述|用户名|IP"（复用 detail 列，无独立 username 列）；ip 列单独存 IP。
func writeLog(db *sql.DB, userID int, action, target, detail, uname, ip string) {
	purgeOldLogs(db)
	_, _ = db.Exec(
		`INSERT INTO operation_logs (user_id, action, target, detail, ip, created_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		userID, action, target, detail+"|"+uname+"|"+ip, ip,
	)
}

// entityName 按对象类型与 ID 查询可读名字，查不到返回空。
func entityName(db *sql.DB, kind, id string) string {
	if id == "" || id == "0" {
		return ""
	}
	var name string
	var q string
	switch kind {
	case "member":
		q = `SELECT COALESCE(NULLIF(name,''), '#'||id) FROM members WHERE id = ?`
	case "spouse":
		q = `SELECT COALESCE(NULLIF(name,''), '#'||id) FROM spouses WHERE id = ?`
	case "user":
		q = `SELECT COALESCE(NULLIF(username,''), '#'||id) FROM users WHERE id = ?`
	case "shrine":
		// 祠宇志：名称
		q = `SELECT COALESCE(NULLIF(name,''), '#'||id) FROM cemeteries WHERE id = ? AND kind = 'shrine'`
	case "grave":
		// 茔墓志：优先墓主姓名，未填则用条目名称
		q = `SELECT COALESCE(NULLIF(owner_name,''), NULLIF(name,''), '#'||id) FROM cemeteries WHERE id = ? AND kind = 'grave'`
	default:
		return ""
	}
	if err := db.QueryRow(q, id).Scan(&name); err != nil {
		return ""
	}
	return name
}

// AuditMiddleware 记录关键操作日志：只记录 POST/PUT/DELETE（写操作），GET 查询不记录。
// 挂在 AuthMiddleware 之后，可从 context 拿到用户身份。
func AuditMiddleware(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method != http.MethodPost && method != http.MethodPut && method != http.MethodDelete {
			c.Next()
			return
		}

		// 预取操作对象名字：修改/删除按 id 从库查（删除后查不到，必须在 c.Next 前取）；
		// 新增从请求体读 name（body 读完会重置，不影响 handler）。
		preName := ""
		id := c.Param("id")
		p := strings.TrimPrefix(c.Request.URL.Path, "/api")
		if id != "" {
			switch {
			case strings.HasPrefix(p, "/spouses") && !strings.Contains(p, "spouse-bio"):
				preName = entityName(db, "spouse", id)
			case strings.HasPrefix(p, "/shrines"):
				preName = entityName(db, "shrine", id)
			case strings.HasPrefix(p, "/graves"):
				preName = entityName(db, "grave", id)
			case strings.Contains(p, "/members"):
				preName = entityName(db, "member", id)
			case strings.HasPrefix(p, "/users"):
				preName = entityName(db, "user", id)
			}
		} else if method == http.MethodPost {
			// 新增成员/配偶/用户时，名字在请求体里
			switch {
			case strings.Contains(p, "/members") && !strings.Contains(p, "/bio") && !strings.Contains(p, "/attachments") && !strings.Contains(p, "/rename"):
				preName = postBodyName(c)
			case strings.HasPrefix(p, "/spouses") && !strings.Contains(p, "spouse-bio"):
				preName = postBodyName(c)
			case strings.HasPrefix(p, "/shrines") || strings.HasPrefix(p, "/graves"):
				preName = postBodyName(c)
			case p == "/users":
				preName = postBodyName(c)
			}
		}

		c.Next()

		if c.Writer.Status() >= 400 {
			return // 失败请求不记录
		}

		uid, _ := c.Get("user_id")
		uname, _ := c.Get("username")
		uidVal, _ := uid.(int)
		unameVal, _ := uname.(string)
		ip := c.ClientIP()

		action, target, detail := describeRoute(c, preName)
		if action == "" {
			return
		}
		writeLog(db, uidVal, action, target, detail, unameVal, ip)
	}
}

// describeRoute 根据方法/路径生成可读操作描述。entityName 是预取的对象名字。
func describeRoute(c *gin.Context, name string) (action, target, detail string) {
	p := strings.TrimPrefix(c.Request.URL.Path, "/api")
	p = strings.TrimPrefix(p, "/api")
	m := c.Request.Method
	id := c.Param("id")

	verb := map[string]string{http.MethodPost: "新增", http.MethodPut: "修改", http.MethodDelete: "删除"}[m]
	who := func() string { // 拼对象名字后缀
		if name != "" {
			return "「" + name + "」"
		}
		if id != "" && id != "0" {
			return "#" + id
		}
		return ""
	}

	switch {
	case strings.HasPrefix(p, "/auth/logout"):
		return "", "", ""
	case strings.HasPrefix(p, "/auth/change-password"):
		return "修改", "账号", "修改了登录密码"

	// 用户管理
	case strings.HasPrefix(p, "/users"):
		if p == "/users" && m == http.MethodPost {
			return "新增", "用户", "创建了用户 " + who()
		}
		if strings.Contains(p, "/role") {
			return "修改", "用户", "调整了用户角色 " + who()
		}
		if strings.Contains(p, "/branch") {
			return "修改", "用户", "调整了用户所属支系 " + who()
		}
		if strings.Contains(p, "reset-password") {
			return "修改", "用户", "重置了用户密码 " + who()
		}
		return "删除", "用户", "删除了用户 " + who()

	// 资料来源（成员页区块，独立子表）：与生平条目同口径，单独记一条日志
	case strings.Contains(p, "/sources"):
		return verb, "资料来源", map[string]string{
			"新增": "新增了资料来源", "修改": "编辑了资料来源", "删除": "删除了资料来源"}[verb]

	// 成员
	case strings.Contains(p, "/members") && !strings.Contains(p, "/bio") && !strings.Contains(p, "/attachments"):
		if strings.Contains(p, "/rename") {
			return "修改", "成员", "重命名成员 " + who()
		}
		switch m {
		case http.MethodPost:
			return "新增", "成员", "新增了成员 " + who()
		case http.MethodDelete:
			return "删除", "成员", "删除了成员 " + who()
		default:
			return "修改", "成员", "编辑了成员 " + who()
		}

	// 配偶
	case strings.HasPrefix(p, "/spouses") || strings.HasPrefix(p, "/spouse-bio"):
		switch m {
		case http.MethodPost:
			return "新增", "配偶", "新增了配偶 " + who()
		case http.MethodDelete:
			return "删除", "配偶", "删除了配偶 " + who()
		default:
			return "修改", "配偶", "编辑了配偶 " + who()
		}

	// 生平条目
	case strings.Contains(p, "bio-entries"):
		return verb, "生平", map[string]string{
			"新增": "新增了生平条目", "修改": "编辑了生平条目", "删除": "删除了生平条目"}[verb]

	// 附件
	case p == "/upload" || strings.Contains(p, "/attachments"):
		if m == http.MethodPost {
			return "上传", "附件", "上传了照片/附件"
		}
		return "删除", "附件", "删除了照片/附件"

	// 字体
	case strings.HasPrefix(p, "/fonts"):
		if m == http.MethodPost {
			return "上传", "字体", "上传了自定义字体"
		}
		return "删除", "字体", "删除了自定义字体"

	// 字辈派语
	case strings.HasPrefix(p, "/generation-words"):
		return verb, "字辈", map[string]string{
			"新增": "新增了字辈派语", "修改": "编辑了字辈派语", "删除": "删除了字辈派语"}[verb]

	// 房支
	case strings.HasPrefix(p, "/branches"):
		if strings.Contains(p, "set-members") {
			return "修改", "房支", "批量分配了成员所属房支"
		}
		return verb, "房支", map[string]string{
			"新增": "新增了房支", "修改": "编辑了房支", "删除": "删除了房支"}[verb]

	case strings.HasPrefix(p, "/events"):
		return verb, "大事记", map[string]string{
			"新增": "新增大事记", "修改": "编辑大事记", "删除": "删除大事记"}[verb]
	case strings.HasPrefix(p, "/articles"):
		return verb, "谱序", map[string]string{
			"新增": "新增谱序文章", "修改": "编辑谱序文章", "删除": "删除谱序文章"}[verb]
	case strings.HasPrefix(p, "/cemeteries"):
		// 旧「祠墓」记录归入新模块，单独记一条，别混成「新增祠墓」
		if strings.HasSuffix(p, "/classify") {
			return "归入", "祠墓", "把旧祠墓记录归入祠宇志 / 茔墓志" + who()
		}
		return verb, "祠墓", map[string]string{
			"新增": "新增祠墓", "修改": "编辑祠墓", "删除": "删除祠墓"}[verb]
	// 祠宇志 / 茔墓志（真实祠堂、真实祖茔的档案，独立于电子祠堂 /hall）
	case strings.HasPrefix(p, "/shrines"):
		return verb, "祠宇志", map[string]string{
			"新增": "新增祠宇", "修改": "编辑祠宇", "删除": "删除祠宇"}[verb] + who()
	case strings.HasPrefix(p, "/graves"):
		return verb, "茔墓志", map[string]string{
			"新增": "新增茔墓", "修改": "编辑茔墓", "删除": "删除茔墓"}[verb] + who()

	case strings.Contains(p, "/drafts"):
		return "审核", "协同修谱", "审核了理事提交的修改草稿"
	case strings.Contains(p, "entity-versions") && strings.Contains(p, "/rollback"):
		return "回滚", "成员资料", "回滚了成员/配偶资料到历史版本"
	case strings.Contains(p, "/gedcom/import"):
		return "导入", "族谱数据", "通过 GEDCOM 导入了族谱数据"

	// 备份/还原
	case strings.HasPrefix(p, "/backup") || strings.HasPrefix(p, "/restore"):
		switch {
		case strings.Contains(p, "/reset"):
			return "还原", "数据", "清空并重置了系统数据"
		case strings.HasPrefix(p, "/restore"):
			return "还原", "数据", "从备份还原了数据"
		case strings.Contains(p, "/delete"):
			return "删除", "备份", "删除了备份文件"
		default:
			return "备份", "数据", "执行了数据备份"
		}

	case strings.Contains(p, "/settings/family-name"):
		return "修改", "家族设置", "修改了家族名称"
	case strings.Contains(p, "/settings/family-surname"):
		return "修改", "家族设置", "修改了家族姓氏/谱面设置"
	case strings.Contains(p, "/settings/login-security"):
		return "修改", "服务器设置", "修改了登录安全设置"
	case strings.Contains(p, "/settings/oplog-retention"):
		return "修改", "服务器设置", "修改了操作日志保留策略"
	case strings.Contains(p, "/ai/config"):
		return "修改", "服务器设置", "修改了修谱助手(AI)设置"

	case strings.Contains(p, "/update/"):
		switch {
		case strings.Contains(p, "/rollback"):
			return "回滚", "系统更新", "回滚了系统版本"
		case strings.Contains(p, "/sources") && m == http.MethodPost:
			return "修改", "系统更新", "修改了更新源配置"
		default:
			return "更新", "系统", "执行了系统在线更新"
		}
	}
	return "", "", ""
}

// ListOperationLogs 分页查询操作日志（理事长及以上）
func ListOperationLogs(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		page := atoiDefault(c.Query("page"), 1)
		size := atoiDefault(c.Query("size"), 50)
		if page < 1 {
			page = 1
		}
		if size < 1 || size > 200 {
			size = 50
		}
		offset := (page - 1) * size

		action := strings.TrimSpace(c.Query("action"))
		keyword := strings.TrimSpace(c.Query("keyword"))

		where := "WHERE 1=1"
		args := []interface{}{}
		if action != "" {
			where += " AND action = ?"
			args = append(args, action)
		}
		if keyword != "" {
			where += " AND (detail LIKE ?)"
			args = append(args, "%"+keyword+"%")
		}

		var total int
		_ = db.QueryRow(`SELECT COUNT(*) FROM operation_logs `+where, args...).Scan(&total)

		q := `SELECT id, COALESCE(user_id,0), COALESCE(action,''), COALESCE(target,''), COALESCE(detail,''), COALESCE(created_at,'')
		      FROM operation_logs ` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
		args = append(args, size, offset)
		rows, err := db.Query(q, args...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询操作日志失败"})
			return
		}
		defer rows.Close()

		list := []AuditLog{}
		for rows.Next() {
			var l AuditLog
			var rawDetail string
			if err := rows.Scan(&l.ID, &l.UserID, &l.Action, &l.Target, &rawDetail, &l.CreatedAt); err != nil {
				continue
			}
			parts := strings.SplitN(rawDetail, "|", 3)
			l.Detail = parts[0]
			if len(parts) >= 2 {
				l.Username = parts[1]
			}
			if len(parts) >= 3 {
				l.IP = parts[2]
			}
			list = append(list, l)
		}

		c.JSON(http.StatusOK, gin.H{
			"list":           list,
			"total":          total,
			"page":           page,
			"size":           size,
			"retention_days": getLogRetentionDays(db),
		})
	}
}

// ClearOperationLogs 清空操作日志（理事长及以上）
func ClearOperationLogs(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := db.Exec(`DELETE FROM operation_logs`); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "清空失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// GetLogRetention 读取操作日志保留策略（理事长及以上）
func GetLogRetention(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"retention_days": getLogRetentionDays(db)})
	}
}

// UpdateLogRetention 修改保留天数；days<=0 表示关闭自动清理（理事长及以上）
func UpdateLogRetention(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			RetentionDays *int `json:"retention_days"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.RetentionDays == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
			return
		}
		days := *req.RetentionDays
		if days < 0 || days > 3650 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "保留天数需在 0~3650 之间（0=关闭自动清理）"})
			return
		}
		if _, err := db.Exec(
			`INSERT INTO settings (key,value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
			LogRetentionKey, strconv.Itoa(days)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
			return
		}
		lastPurge = time.Time{} // 立即允许下次写日志时清理
		c.JSON(http.StatusOK, gin.H{"ok": true, "retention_days": days})
	}
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
