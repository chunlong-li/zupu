package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"familytree/internal/config"

	"github.com/gin-gonic/gin"
)

// ======================== 收支公示（公开账目） ========================
//
// 角色分工（用户 2026-09-25 定）：
//   提交：理事 / 房支理事（须由管理员加入「理账名单」授权）
//   审核：财务管理（新增职位）/ 理事长 / 超级管理员（单人通过即可）
//   管理账本：财务管理 / 理事长 / 超级管理员；管理「理账名单」：理事长 / 超级管理员
//   查看：族员及以上（只看已审核公开的账目）
// 账目记录带「审核人」；捐款必须关联成员，默认实名，可单独关闭（匿名）。

// AccountHandler 收支公示处理器
type AccountHandler struct {
	DB  *sql.DB
	Cfg *config.Config
}

// AccountBook 账本
type AccountBook struct {
	ID             int64   `json:"id"`
	Title          string  `json:"title"`
	Year           int     `json:"year"`
	Project        string  `json:"project"`
	BookType       string  `json:"book_type"` // project 项目账本 | donation 捐款账簿
	SortOrder      int     `json:"sort_order"`
	IsPinned       bool    `json:"is_pinned"`
	Description    string  `json:"description"`
	Status         string  `json:"status"` // open | closed
	IsPublic       bool    `json:"is_public"`
	InitialBalance float64 `json:"initial_balance"`
	CreatedByName  string  `json:"created_by_name"`
	CreatedAt      string  `json:"created_at"`
	TotalIn        float64 `json:"total_in"`
	TotalOut       float64 `json:"total_out"`
	Balance        float64 `json:"balance"`
	EntryCount     int     `json:"entry_count"`
	PendingCount   int     `json:"pending_count"`
	IsFund         bool    `json:"is_fund"`
	MeritTitle     string  `json:"merit_title"`
	MeritThreshold float64 `json:"merit_threshold"`
	MeritNumeral   string  `json:"merit_numeral"` // arabic 阿拉伯数字 | chinese 中文数字
	MeritShowAmt   bool    `json:"merit_show_amount"`
	MeritFooter    string  `json:"merit_footer"`
}

// AccountEntry 账目明细
type AccountEntry struct {
	ID              int64    `json:"id"`
	BookID          int64    `json:"book_id"`
	Kind            string   `json:"kind"` // in 收入 | out 支出
	Amount          float64  `json:"amount"`
	Category        string   `json:"category"`
	EntryDate       string   `json:"entry_date"`
	Summary         string   `json:"summary"`
	Handler         string   `json:"handler"`
	Vouchers        []string `json:"vouchers"`
	MemberID        int64    `json:"member_id"`
	MemberName      string   `json:"member_name"`
	DonorPublic     bool     `json:"donor_public"`
	Status          string   `json:"status"` // pending | approved | rejected
	SubmittedBy     int      `json:"submitted_by"`
	SubmittedByName string   `json:"submitted_by_name"`
	ReviewerBy      int      `json:"reviewer_by"`
	ReviewerName    string   `json:"reviewer_name"`
	Reason          string   `json:"reason"`
	CreatedAt       string   `json:"created_at"`
	ReviewedAt      string   `json:"reviewed_at"`
}

// ---- 权限 / 配置判定 ----

func accountsEnabled(db *sql.DB) bool {
	var v string
	db.QueryRow(`SELECT value FROM settings WHERE key = 'account_enabled'`).Scan(&v)
	return v == "1"
}

func accountSubmitterIDs(db *sql.DB) []int64 {
	var raw string
	db.QueryRow(`SELECT value FROM settings WHERE key = 'account_submitters'`).Scan(&raw)
	ids := []int64{}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &ids)
	}
	return ids
}

func isAccountSubmitter(db *sql.DB, uid int) bool {
	for _, id := range accountSubmitterIDs(db) {
		if int(id) == uid {
			return true
		}
	}
	return false
}

// canManageBooks 管理账本 / 审核：财务管理、理事长、超级管理员
func canManageBooks(c *gin.Context) bool {
	role := c.GetString("role")
	return role == RoleFinance || role == RoleChief || role == RoleSuperAdmin
}

// canReviewAccounts 审核账目：财务管理、理事长、超级管理员
func canReviewAccounts(c *gin.Context) bool { return PrivAllowed("finance_audit", c.GetString("role")) }

// canManageAccounts 管理「理账名单」：理事长、超级管理员
func canManageAccounts(c *gin.Context) bool {
	role := c.GetString("role")
	return role == RoleChief || role == RoleSuperAdmin
}

// canSubmitAccounts 提交账目：理事 / 房支理事，且已在「理账名单」中
func canSubmitAccounts(c *gin.Context, db *sql.DB) bool {
	role := c.GetString("role")
	if role != RoleElder && role != RoleBranchElder {
		return false
	}
	return isAccountSubmitter(db, c.GetInt("user_id"))
}

func canViewAccounts(c *gin.Context) bool { return PrivAllowed("accounts_view", c.GetString("role")) }

// splitVouchers 逗号分隔的凭证路径 → 切片
func splitVouchers(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// accountFundBase 家族资金基数（初始注入额，settings.account_fund_base）
func accountFundBase(db *sql.DB) float64 {
	var raw string
	db.QueryRow(`SELECT value FROM settings WHERE key = 'account_fund_base'`).Scan(&raw)
	f, _ := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	return f
}

// ---- 家族基金系统账本 ----
//
// 「家族基金」是一本固定的系统账本（settings.account_fund_book 存其 id），不可编辑/删除/封账，
// 其条目全部由系统自动生成（不可手动增删改）：
//   注入 / 提取（理事长手工操作，必须注明）
//   划拨启动资金给某账本 / 该账本启动资金退回 / 封账结余归还
// 家族资金余额 = 该账本余额。

// fundBookID 家族基金账本 id
func fundBookID(db *sql.DB) int64 {
	var raw string
	db.QueryRow(`SELECT value FROM settings WHERE key = 'account_fund_book'`).Scan(&raw)
	id, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return id
}

// isFundBook 是否家族基金系统账本
func isFundBook(db *sql.DB, id int64) bool {
	f := fundBookID(db)
	return f != 0 && f == id
}

// ensureFundBook 确保「家族基金」账本存在并返回 id
func ensureFundBook(db *sql.DB) int64 {
	if id := fundBookID(db); id > 0 {
		var one int
		if db.QueryRow(`SELECT 1 FROM account_books WHERE id = ?`, id).Scan(&one) == nil {
			return id
		}
	}
	res, err := db.Exec(`INSERT INTO account_books (title, year, project, sort_order, description, status, is_public, initial_balance, created_by_name)
		VALUES ('家族基金', 0, '家族资金', -9999, '系统账本：记录家族资金的注入、提取，以及各账本启动资金的划拨与封账结余归还', 'open', 1, 0, '系统')`)
	if err != nil {
		return 0
	}
	id, _ := res.LastInsertId()
	db.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('account_fund_book', ?)`, strconv.FormatInt(id, 10))
	// 迁移：旧版本存的「家族资金基数」记为一笔期初注入
	if base := accountFundBase(db); base != 0 {
		insertFundEntry(db, "in", base, "期初结转（家族资金初始金额）", 0, "系统")
	}
	return id
}

// insertFundEntry 向家族基金账本自动补一条已审核条目
func insertFundEntry(db *sql.DB, kind string, amount float64, summary string, uid int, uname string) {
	id := fundBookID(db)
	if id == 0 || amount == 0 {
		return
	}
	today := time.Now().Format("2006-01-02")
	db.Exec(`INSERT INTO account_entries (book_id, kind, amount, category, entry_date, summary, handler, status, submitted_by, submitted_by_name, reviewer_by, reviewer_name, reviewed_at)
		VALUES (?,?,?,?,?,?, '系统', 'approved', ?,?, ?,?, datetime('now','localtime'))`,
		id, kind, amount, "家族资金", today, summary, uid, uname, uid, uname)
}

// accountFund 家族资金余额 = 家族基金账本余额（无基金账本时兼容用旧「基数」）
func accountFund(db *sql.DB) float64 {
	id := fundBookID(db)
	if id == 0 {
		return accountFundBase(db)
	}
	var in, out float64
	db.QueryRow(`SELECT COALESCE(SUM(CASE WHEN kind='in' THEN amount ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN kind='out' THEN amount ELSE 0 END),0)
		FROM account_entries WHERE book_id = ? AND status = 'approved'`, id).Scan(&in, &out)
	return in - out
}

// accountBookExists 账本是否存在（及状态）
func accountBookExists(db *sql.DB, id int64) (bool, string) {
	var status string
	err := db.QueryRow(`SELECT COALESCE(status,'open') FROM account_books WHERE id = ?`, id).Scan(&status)
	if err != nil {
		return false, ""
	}
	return true, status
}

// ---- 接口实现 ----

// GetConfig 当前用户的能力与开关状态
// GET /api/accounts/config
func (h *AccountHandler) GetConfig(c *gin.Context) {
	enabled := accountsEnabled(h.DB)
	resp := gin.H{
		"enabled":          enabled,
		"role":             c.GetString("role"),
		"can_submit":       canSubmitAccounts(c, h.DB),
		"can_review":       canReviewAccounts(c),
		"can_manage_books": canManageBooks(c),
		"can_manage_list":  canManageAccounts(c),
		"fund":             accountFund(h.DB),
		"fund_book_id":     ensureFundBook(h.DB),
	}
	if canManageAccounts(c) {
		users := []gin.H{}
		list := accountSubmitterIDs(h.DB)
		inList := func(uid int64) bool {
			for _, x := range list {
				if x == uid {
					return true
				}
			}
			return false
		}
		rows, err := h.DB.Query(`SELECT id, username, role FROM users WHERE role IN (?,?) ORDER BY id`, RoleElder, RoleBranchElder)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int64
				var un, rl string
				if rows.Scan(&id, &un, &rl) == nil {
					users = append(users, gin.H{"id": id, "username": un, "role": rl, "in_list": inList(id)})
				}
			}
		}
		resp["submitter_users"] = users
		resp["submitter_ids"] = list
	}
	c.JSON(http.StatusOK, resp)
}

// SaveSettings 开启 / 关闭「收支公示」功能
// PUT /api/accounts/settings  body: {"enabled":true}
func (h *AccountHandler) SaveSettings(c *gin.Context) {
	if !canManageAccounts(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足：仅理事长/超级管理员可开启"})
		return
	}
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
	h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('account_enabled', ?)`, v)
	c.JSON(http.StatusOK, gin.H{"enabled": body.Enabled})
}

// SetSubmitters 设置「理账名单」
// PUT /api/accounts/submitters  body: {"ids":[1,2,3]}
func (h *AccountHandler) SetSubmitters(c *gin.Context) {
	if !canManageAccounts(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足：仅理事长/超级管理员可管理理账名单"})
		return
	}
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	raw, _ := json.Marshal(body.IDs)
	h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('account_submitters', ?)`, string(raw))
	c.JSON(http.StatusOK, gin.H{"ids": body.IDs})
}

// FundFlow 家族资金注入 / 提取（自动记入「家族基金」系统账本，必须填备注）
// POST /api/accounts/fund/flow  body: {"action":"in|out","amount":1000,"note":"用途/来源"}
func (h *AccountHandler) FundFlow(c *gin.Context) {
	if !canManageAccounts(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足：仅理事长/超级管理员可注入或提取家族资金"})
		return
	}
	var body struct {
		Action string  `json:"action"`
		Amount float64 `json:"amount"`
		Note   string  `json:"note"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if body.Action != "in" && body.Action != "out" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if body.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "金额需大于 0"})
		return
	}
	if strings.TrimSpace(body.Note) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写备注（来源 / 用途）"})
		return
	}
	ensureFundBook(h.DB)
	if body.Action == "out" {
		if cur := accountFund(h.DB); body.Amount > cur+1e-6 {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("提取金额超过家族资金余额（¥%.2f）", cur)})
			return
		}
	}
	uid, uname := currentUser(c)
	label := "注入"
	if body.Action == "out" {
		label = "提取"
	}
	insertFundEntry(h.DB, body.Action, body.Amount, fmt.Sprintf("基金%s：%s", label, strings.TrimSpace(body.Note)), uid, uname)
	c.JSON(http.StatusOK, gin.H{"fund": accountFund(h.DB), "action": body.Action})
}

// bookBalance 账本结余 = 启动资金 + 已审核收入 − 已审核支出
func bookBalance(db *sql.DB, id int64) float64 {
	var start, in, out float64
	db.QueryRow(`SELECT COALESCE(initial_balance,0) FROM account_books WHERE id = ?`, id).Scan(&start)
	db.QueryRow(`SELECT COALESCE(SUM(CASE WHEN kind='in' THEN amount ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN kind='out' THEN amount ELSE 0 END),0)
		FROM account_entries WHERE book_id = ? AND status='approved'`, id).Scan(&in, &out)
	return start + in - out
}

func (h *AccountHandler) fillerBook(b *AccountBook) {
	h.DB.QueryRow(`SELECT
		COALESCE(SUM(CASE WHEN kind='in' THEN amount ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN kind='out' THEN amount ELSE 0 END),0),
		COUNT(*) FROM account_entries WHERE book_id = ? AND status='approved'`, b.ID).
		Scan(&b.TotalIn, &b.TotalOut, &b.EntryCount)
	h.DB.QueryRow(`SELECT COUNT(*) FROM account_entries WHERE book_id = ? AND status='pending'`, b.ID).Scan(&b.PendingCount)
	b.Balance = b.InitialBalance + b.TotalIn - b.TotalOut
}

// ListBooks 账本列表（年度内按项目先后排序）
// GET /api/accounts/books
func (h *AccountHandler) ListBooks(c *gin.Context) {
	if !canViewAccounts(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}
	reviewer := canReviewAccounts(c)
	fid := ensureFundBook(h.DB)
	q := `SELECT id, COALESCE(title,''), COALESCE(year,0), COALESCE(project,''), COALESCE(book_type,'project'), COALESCE(sort_order,0), COALESCE(is_pinned,0),
		COALESCE(description,''), COALESCE(status,'open'), COALESCE(is_public,1), COALESCE(initial_balance,0),
		COALESCE(merit_title,''), COALESCE(merit_threshold,0), COALESCE(merit_numeral,'arabic'), COALESCE(merit_show_amount,1), COALESCE(merit_footer,''),
		COALESCE(created_by_name,''), COALESCE(created_at,'') FROM account_books`
	if !reviewer {
		q += ` WHERE is_public = 1`
	}
	q += ` ORDER BY is_pinned DESC, CASE WHEN status='open' THEN 0 ELSE 1 END, id DESC`
	rows, err := h.DB.Query(q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	list := []AccountBook{}
	for rows.Next() {
		var b AccountBook
		var isPub int
		var pinned int
		var meritShow int
		if err := rows.Scan(&b.ID, &b.Title, &b.Year, &b.Project, &b.BookType, &b.SortOrder, &pinned, &b.Description,
			&b.Status, &isPub, &b.InitialBalance, &b.MeritTitle, &b.MeritThreshold, &b.MeritNumeral, &meritShow, &b.MeritFooter,
			&b.CreatedByName, &b.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if b.BookType != "donation" {
			b.BookType = "project"
		}
		if b.MeritNumeral != "chinese" {
			b.MeritNumeral = "arabic"
		}
		b.MeritShowAmt = meritShow == 1
		b.IsPinned = pinned == 1
		b.IsPublic = isPub == 1
		b.IsFund = b.ID == fid
		h.fillerBook(&b)
		list = append(list, b)
	}
	c.JSON(http.StatusOK, list)
}

// CreateBook 新建账本
// POST /api/accounts/books
func (h *AccountHandler) CreateBook(c *gin.Context) {
	if !canManageBooks(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}
	var b AccountBook
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if strings.TrimSpace(b.Title) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写账本名称"})
		return
	}
	if b.InitialBalance < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "启动资金不能为负数"})
		return
	}
	// 启动资金从家族资金划拨，不能超过当前家族资金
	ensureFundBook(h.DB)
	if fund := accountFund(h.DB); b.InitialBalance > fund+1e-6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("启动资金不能超过当前家族资金（¥%.2f）", fund)})
		return
	}
	if b.Status == "" {
		b.Status = "open"
	}
	if b.BookType != "donation" {
		b.BookType = "project"
	}
	uid, uname := currentUser(c)
	res, err := h.DB.Exec(`INSERT INTO account_books (title, year, project, book_type, description, status, is_public, initial_balance, created_by, created_by_name)
		VALUES (?,?,?,?,?,?,?,?,?,?)`, b.Title, b.Year, b.Project, b.BookType, b.Description, b.Status, boolToInt(b.IsPublic), b.InitialBalance, uid, uname)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	id, _ := res.LastInsertId()
	b.ID = id
	b.CreatedByName = uname
	if b.MeritNumeral != "chinese" {
		b.MeritNumeral = "arabic"
	}
	b.MeritShowAmt = true
	// 划拨启动资金 → 在家族基金账本记一笔支出
	insertFundEntry(h.DB, "out", b.InitialBalance, fmt.Sprintf("划拨启动资金给「%s」", b.Title), uid, uname)
	c.JSON(http.StatusOK, b)
}

// UpdateBook 编辑账本 / 封账
// PUT /api/accounts/books/:id
func (h *AccountHandler) UpdateBook(c *gin.Context) {
	if !canManageBooks(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ok, curStatus := accountBookExists(h.DB, id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "账本不存在"})
		return
	}
	if isFundBook(h.DB, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "「家族基金」为系统账本，不可编辑（余额变动请用注入/提取）"})
		return
	}
	// 封账不可取消：已封账的账本连同其信息一起定稿，任何修改（含改回「进行中」）均拒绝
	if curStatus == "closed" {
		c.JSON(http.StatusForbidden, gin.H{"error": "该账本已封账定稿，不可再修改（封账不可取消）"})
		return
	}
	var b AccountBook
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if b.Status == "" {
		b.Status = "open"
	}
	if b.InitialBalance < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "启动资金不能为负数"})
		return
	}
	// 启动资金不能超过「家族资金 + 本账本原先占用的启动资金」（原先那笔退还后再重新划拨）
	var oldStart float64
	h.DB.QueryRow(`SELECT COALESCE(initial_balance,0) FROM account_books WHERE id = ?`, id).Scan(&oldStart)
	if avail := accountFund(h.DB) + oldStart; b.InitialBalance > avail+1e-6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("启动资金不能超过当前家族资金（¥%.2f）", avail)})
		return
	}
	// 封账前必须先处理完待审核账目（封账后无法再审）
	if b.Status == "closed" {
		var pend int
		h.DB.QueryRow(`SELECT COUNT(*) FROM account_entries WHERE book_id = ? AND status = 'pending'`, id).Scan(&pend)
		if pend > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("该账本还有 %d 笔待审核账目，请先审核完再封账", pend)})
			return
		}
	}
	_, err := h.DB.Exec(`UPDATE account_books SET title=?, year=?, project=?, book_type=?, description=?, status=?, is_public=?, initial_balance=? WHERE id=?`,
		b.Title, b.Year, b.Project, b.BookType, b.Description, b.Status, boolToInt(b.IsPublic), b.InitialBalance, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	b.ID = id
	if b.BookType != "donation" && b.BookType != "project" {
		var t string
		h.DB.QueryRow(`SELECT COALESCE(book_type,'project') FROM account_books WHERE id = ?`, id).Scan(&t)
		b.BookType = t
	}
	uid, uname := currentUser(c)
	// 启动资金变动 → 在家族基金账本补记（多划 = 支出，退回 = 收入）
	if b.InitialBalance > oldStart {
		insertFundEntry(h.DB, "out", b.InitialBalance-oldStart, fmt.Sprintf("追加划拨启动资金给「%s」", b.Title), uid, uname)
	} else if b.InitialBalance < oldStart {
		insertFundEntry(h.DB, "in", oldStart-b.InitialBalance, fmt.Sprintf("「%s」启动资金退回基金", b.Title), uid, uname)
	}
	// 封账 → 结余归还家族基金
	if b.Status == "closed" && curStatus != "closed" {
		insertFundEntry(h.DB, "in", bookBalance(h.DB, id), fmt.Sprintf("「%s」封账，结余归还基金", b.Title), uid, uname)
	}
	c.JSON(http.StatusOK, b)
}

// DeleteBook 删除账本（连同其明细）
// DELETE /api/accounts/books/:id
func (h *AccountHandler) DeleteBook(c *gin.Context) {
	if !canManageBooks(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ok, status := accountBookExists(h.DB, id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "账本不存在"})
		return
	}
	if isFundBook(h.DB, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "「家族基金」为系统账本，不能删除"})
		return
	}
	if status == "closed" {
		c.JSON(http.StatusForbidden, gin.H{"error": "该账本已封账定稿，不能删除"})
		return
	}
	var title string
	var start float64
	h.DB.QueryRow(`SELECT COALESCE(title,''), COALESCE(initial_balance,0) FROM account_books WHERE id = ?`, id).Scan(&title, &start)
	h.DB.Exec(`DELETE FROM account_entries WHERE book_id = ?`, id)
	if _, err := h.DB.Exec(`DELETE FROM account_books WHERE id = ?`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 启动资金退回家族基金
	uid, uname := currentUser(c)
	insertFundEntry(h.DB, "in", start, fmt.Sprintf("删除「%s」，启动资金退回基金", title), uid, uname)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TogglePin 置顶/取消置顶（封账账本也可用）
// PUT /api/accounts/books/:id/pin
func (h *AccountHandler) TogglePin(c *gin.Context) {
	if !canManageBooks(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ok, _ := accountBookExists(h.DB, id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "账本不存在"})
		return
	}
	if isFundBook(h.DB, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "「家族基金」为系统账本，不支持置顶操作"})
		return
	}
	var body struct {
		IsPinned bool `json:"is_pinned"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	_, err := h.DB.Exec(`UPDATE account_books SET is_pinned = ? WHERE id = ?`, boolToInt(body.IsPinned), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "is_pinned": body.IsPinned})
}

// SaveMeritConfig 保存「功德碑」展示配置（仅捐款账簿，封账后仍可调）
// PUT /api/accounts/books/:id/merit
func (h *AccountHandler) SaveMeritConfig(c *gin.Context) {
	if !canManageBooks(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if ok, _ := accountBookExists(h.DB, id); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "账本不存在"})
		return
	}
	var body struct {
		MeritTitle     string  `json:"merit_title"`
		MeritThreshold float64 `json:"merit_threshold"`
		MeritNumeral   string  `json:"merit_numeral"`
		MeritShowAmt   bool    `json:"merit_show_amount"`
		MeritFooter    string  `json:"merit_footer"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if body.MeritThreshold < 0 {
		body.MeritThreshold = 0
	}
	numeral := "arabic"
	if body.MeritNumeral == "chinese" {
		numeral = "chinese"
	}
	_, err := h.DB.Exec(`UPDATE account_books SET merit_title=?, merit_threshold=?, merit_numeral=?, merit_show_amount=?, merit_footer=? WHERE id=?`,
		strings.TrimSpace(body.MeritTitle), body.MeritThreshold, numeral, boolToInt(body.MeritShowAmt), strings.TrimSpace(body.MeritFooter), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// bookDetail 详情（含汇总）
// GET /api/accounts/books/:id
func (h *AccountHandler) GetBook(c *gin.Context) {
	if !canViewAccounts(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var b AccountBook
	var isPub int
	var pinned int
	var meritShow int
	err := h.DB.QueryRow(`SELECT id, COALESCE(title,''), COALESCE(year,0), COALESCE(project,''), COALESCE(book_type,'project'), COALESCE(sort_order,0), COALESCE(is_pinned,0),
		COALESCE(description,''), COALESCE(status,'open'), COALESCE(is_public,1), COALESCE(initial_balance,0),
		COALESCE(merit_title,''), COALESCE(merit_threshold,0), COALESCE(merit_numeral,'arabic'), COALESCE(merit_show_amount,1), COALESCE(merit_footer,''),
		COALESCE(created_by_name,''), COALESCE(created_at,'') FROM account_books WHERE id = ?`, id).
		Scan(&b.ID, &b.Title, &b.Year, &b.Project, &b.BookType, &b.SortOrder, &pinned, &b.Description, &b.Status, &isPub, &b.InitialBalance,
			&b.MeritTitle, &b.MeritThreshold, &b.MeritNumeral, &meritShow, &b.MeritFooter, &b.CreatedByName, &b.CreatedAt)
	b.IsPinned = pinned == 1
	if b.BookType != "donation" {
		b.BookType = "project"
	}
	if b.MeritNumeral != "chinese" {
		b.MeritNumeral = "arabic"
	}
	b.MeritShowAmt = meritShow == 1
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "账本不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	b.IsPublic = isPub == 1
	b.IsFund = isFundBook(h.DB, b.ID)
	h.fillerBook(&b)
	c.JSON(http.StatusOK, b)
}

// ListEntries 账本明细（族员仅见已审核；提交人可见自己待审的；审核层可见全部）
// GET /api/accounts/books/:id/entries
func (h *AccountHandler) ListEntries(c *gin.Context) {
	if !canViewAccounts(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}
	bookID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	reviewer := canReviewAccounts(c)
	uid := c.GetInt("user_id")
	q := `SELECT e.id, e.book_id, COALESCE(e.kind,'in'), COALESCE(e.amount,0), COALESCE(e.category,''),
		COALESCE(e.entry_date,''), COALESCE(e.summary,''), COALESCE(e.handler,''), COALESCE(e.voucher,''),
		COALESCE(e.member_id,0), COALESCE(m.name,''), COALESCE(e.donor_public,1), COALESCE(e.status,''),
		COALESCE(e.submitted_by,0), COALESCE(e.submitted_by_name,''), COALESCE(e.reviewer_by,0),
		COALESCE(e.reviewer_name,''), COALESCE(e.reason,''), COALESCE(datetime(e.created_at,'localtime'),''), COALESCE(datetime(e.reviewed_at,'localtime'),'')
		FROM account_entries e LEFT JOIN members m ON m.id = e.member_id WHERE e.book_id = ?`
	if !reviewer {
		q += ` AND (e.status='approved' OR (e.status='pending' AND e.submitted_by=` + strconv.Itoa(uid) + `))`
	}
	q += ` ORDER BY e.entry_date DESC, e.id DESC`
	rows, err := h.DB.Query(q, bookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	list := []AccountEntry{}
	for rows.Next() {
		e, err := scanAccountEntry(rows, reviewer)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		list = append(list, *e)
	}
	c.JSON(http.StatusOK, list)
}

// scanAccountEntry 扫描一行账目（rowLike：*sql.Row 或 *sql.Rows）
type rowLike interface {
	Scan(dest ...interface{}) error
}

func scanAccountEntry(r rowLike, reveal bool) (*AccountEntry, error) {
	var e AccountEntry
	var vouchers, mname string
	var pub int
	if err := r.Scan(&e.ID, &e.BookID, &e.Kind, &e.Amount, &e.Category, &e.EntryDate, &e.Summary, &e.Handler,
		&vouchers, &e.MemberID, &mname, &pub, &e.Status, &e.SubmittedBy, &e.SubmittedByName,
		&e.ReviewerBy, &e.ReviewerName, &e.Reason, &e.CreatedAt, &e.ReviewedAt); err != nil {
		return nil, err
	}
	e.Vouchers = splitVouchers(vouchers)
	e.DonorPublic = pub == 1
	if e.MemberID > 0 {
		if !reveal && !e.DonorPublic {
			e.MemberName = "热心宗亲"
		} else {
			e.MemberName = mname
		}
	}
	return &e, nil
}

// ListPending 待审核队列（审核层）
// GET /api/accounts/pending
func (h *AccountHandler) ListPending(c *gin.Context) {
	if !canReviewAccounts(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}
	rows, err := h.DB.Query(`SELECT e.id, e.book_id, COALESCE(e.kind,'in'), COALESCE(e.amount,0), COALESCE(e.category,''),
		COALESCE(e.entry_date,''), COALESCE(e.summary,''), COALESCE(e.handler,''), COALESCE(e.voucher,''),
		COALESCE(e.member_id,0), COALESCE(m.name,''), COALESCE(e.donor_public,1), COALESCE(e.status,''),
		COALESCE(e.submitted_by,0), COALESCE(e.submitted_by_name,''), COALESCE(e.reviewer_by,0),
		COALESCE(e.reviewer_name,''), COALESCE(e.reason,''), COALESCE(e.created_at,''), COALESCE(e.reviewed_at,'')
		FROM account_entries e LEFT JOIN members m ON m.id = e.member_id
		WHERE e.status='pending' ORDER BY e.id ASC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	list := []AccountEntry{}
	for rows.Next() {
		e, err := scanAccountEntry(rows, true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		list = append(list, *e)
	}
	c.JSON(http.StatusOK, list)
}

// CreateEntry 记一笔：理事提交（进审核），审核层直接记账
// POST /api/accounts/books/:id/entries
func (h *AccountHandler) CreateEntry(c *gin.Context) {
	bookID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ok, status := accountBookExists(h.DB, bookID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "账本不存在"})
		return
	}
	reviewer := canReviewAccounts(c)
	if !reviewer && !canSubmitAccounts(c, h.DB) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足：需在「理账名单」中"})
		return
	}
	if status == "closed" {
		c.JSON(http.StatusForbidden, gin.H{"error": "该账本已封账定稿，不能再记账"})
		return
	}
	if isFundBook(h.DB, bookID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "「家族基金」为系统账本，账目由系统自动生成（请用注入/提取）"})
		return
	}
	var e AccountEntry
	if err := c.ShouldBindJSON(&e); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if e.Kind != "in" && e.Kind != "out" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择收入或支出"})
		return
	}
	if e.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "金额需大于 0"})
		return
	}
	if strings.TrimSpace(e.Summary) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写摘要"})
		return
	}
	uid, uname := currentUser(c)
	newStatus := "pending"
	revBy, revName := 0, ""
	if reviewer {
		newStatus = "approved"
		revBy, revName = uid, uname // 审核层直接记账，审核人记为自己
	}
	res, err := h.DB.Exec(`INSERT INTO account_entries
		(book_id, kind, amount, category, entry_date, summary, handler, voucher, member_id, donor_public, status, submitted_by, submitted_by_name, reviewer_by, reviewer_name)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		bookID, e.Kind, e.Amount, e.Category, e.EntryDate, e.Summary, e.Handler,
		strings.Join(e.Vouchers, ","), e.MemberID, boolToInt(e.DonorPublic), newStatus, uid, uname, revBy, revName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	id, _ := res.LastInsertId()
	e.ID = id
	e.BookID = bookID
	e.Status = newStatus
	e.SubmittedBy, e.SubmittedByName = uid, uname
	e.ReviewerBy, e.ReviewerName = revBy, revName
	if newStatus == "pending" {
		c.JSON(http.StatusOK, gin.H{"entry": e, "message": "已提交审核，待财务管理/理事长审核通过后公开"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entry": e})
}

// UpdateEntry 修改账目（提交人可在待审时改；审核层随时改）
// PUT /api/accounts/entries/:id
func (h *AccountHandler) UpdateEntry(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var bookID int64
	var status string
	var submittedBy int
	err := h.DB.QueryRow(`SELECT book_id, COALESCE(status,''), COALESCE(submitted_by,0) FROM account_entries WHERE id = ?`, id).
		Scan(&bookID, &status, &submittedBy)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "账目不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	reviewer := canReviewAccounts(c)
	if !reviewer && !(submittedBy == c.GetInt("user_id") && status == "pending") {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}
	if isFundBook(h.DB, bookID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "「家族基金」系统账目的记录不可修改"})
		return
	}
	if _, st := accountBookExists(h.DB, bookID); st == "closed" {
		c.JSON(http.StatusForbidden, gin.H{"error": "该账本已封账定稿，账目不可修改"})
		return
	}
	var e AccountEntry
	if err := c.ShouldBindJSON(&e); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if e.Kind != "in" && e.Kind != "out" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择收入或支出"})
		return
	}
	if e.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "金额需大于 0"})
		return
	}
	_, err = h.DB.Exec(`UPDATE account_entries SET kind=?, amount=?, category=?, entry_date=?, summary=?, handler=?, voucher=?, member_id=?, donor_public=? WHERE id=?`,
		e.Kind, e.Amount, e.Category, e.EntryDate, e.Summary, e.Handler, strings.Join(e.Vouchers, ","), e.MemberID, boolToInt(e.DonorPublic), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteEntry 删除账目（提交人待审时可撤；审核层可删）
// DELETE /api/accounts/entries/:id
func (h *AccountHandler) DeleteEntry(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var bookID int64
	var status string
	var submittedBy int
	err := h.DB.QueryRow(`SELECT book_id, COALESCE(status,''), COALESCE(submitted_by,0) FROM account_entries WHERE id = ?`, id).Scan(&bookID, &status, &submittedBy)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "账目不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	reviewer := canReviewAccounts(c)
	if !reviewer && !(submittedBy == c.GetInt("user_id") && status == "pending") {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}
	if isFundBook(h.DB, bookID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "「家族基金」系统账目的记录不可删除"})
		return
	}
	if _, st := accountBookExists(h.DB, bookID); st == "closed" {
		c.JSON(http.StatusForbidden, gin.H{"error": "该账本已封账定稿，账目不可删除"})
		return
	}
	h.DB.Exec(`DELETE FROM account_entries WHERE id = ?`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ReviewEntry 审核账目：approve 公开 / reject 驳回
// POST /api/accounts/entries/:id/review  body: {"action":"approve|reject","reason":"..."}
func (h *AccountHandler) ReviewEntry(c *gin.Context) {
	if !canReviewAccounts(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足：仅财务管理/理事长/超级管理员可审核"})
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var body struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Action != "approve" && body.Action != "reject" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action 必须为 approve 或 reject"})
		return
	}
	var status string
	var bookID int64
	err := h.DB.QueryRow(`SELECT COALESCE(status,''), COALESCE(book_id,0) FROM account_entries WHERE id = ?`, id).Scan(&status, &bookID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "账目不存在"})
		return
	}
	if status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该记录已处理，请勿重复审核"})
		return
	}
	if isFundBook(h.DB, bookID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "「家族基金」系统账目的记录不可审核"})
		return
	}
	if _, st := accountBookExists(h.DB, bookID); st == "closed" {
		c.JSON(http.StatusForbidden, gin.H{"error": "该账本已封账定稿，账目不可审核"})
		return
	}
	uid, uname := currentUser(c)
	newStatus := "approved"
	if body.Action == "reject" {
		newStatus = "rejected"
	}
	_, err = h.DB.Exec(`UPDATE account_entries SET status=?, reviewer_by=?, reviewer_name=?, reason=?, reviewed_at=CURRENT_TIMESTAMP WHERE id=?`,
		newStatus, uid, uname, body.Reason, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "status": newStatus, "reviewer_name": uname})
}

// UploadVoucher 上传凭证图片
// POST /api/accounts/upload  (multipart: file)
func (h *AccountHandler) UploadVoucher(c *gin.Context) {
	if !canReviewAccounts(c) && !canSubmitAccounts(c, h.DB) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择文件"})
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "凭证请上传图片（jpg/png/gif/webp）"})
		return
	}
	uploadDir := filepath.Join(h.Cfg.DataDir, "upload")
	os.MkdirAll(uploadDir, 0o755)
	filename := fmt.Sprintf("account_%d%s", time.Now().UnixMilli(), ext)
	out, err := os.Create(filepath.Join(uploadDir, filename))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入文件失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": "/uploads/" + filename})
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
