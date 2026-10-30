package handler

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// MeritStele 「功德碑」——可存在多个，各自独立配置，最新在前
type MeritStele struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`      // 切换标签显示名
	BookID    int64   `json:"book_id"`   // 关联的「捐款账簿」
	Title     string  `json:"title"`     // 碑额标题
	Threshold float64 `json:"threshold"` // 入碑门槛
	PerColumn int     `json:"per_column"`
	Numeral   string  `json:"numeral"` // arabic | chinese
	Style     string  `json:"style"`   // 配色：stone 墨底金字 | plain 白底黑字 | jade 汉白玉描金 | gold 朱漆描金
	Border    string  `json:"border"`  // 边框纹样：none 无 | hui1 回纹·单线 | hui2 回纹·带框线
	ShowAmt   bool    `json:"show_amount"`
	Footer    string  `json:"footer"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	BookTitle string  `json:"book_title,omitempty"` // 只读：关联账簿名（列表展示）
}

func meritNormalize(s *MeritStele) {
	if s.Threshold < 0 {
		s.Threshold = 0
	}
	if s.PerColumn < 4 {
		s.PerColumn = 4
	}
	if s.PerColumn > 40 {
		s.PerColumn = 40
	}
	if s.Numeral != "chinese" {
		s.Numeral = "arabic"
	}
	if !meritStyleOK(s.Style) {
		s.Style = "stone"
	}
	if !meritBorderOK(s.Border) {
		s.Border = "none"
	}
	s.Name = strings.TrimSpace(s.Name)
	s.Title = strings.TrimSpace(s.Title)
	s.Footer = strings.TrimSpace(s.Footer)
}

// meritStyleOK 合法配色（底色 / 字色）
func meritStyleOK(v string) bool {
	switch v {
	case "stone", "plain", "jade", "gold":
		return true
	}
	return false
}

// meritBorderOK 合法边框纹样
func meritBorderOK(v string) bool {
	switch v {
	case "none", "hui1", "hui2":
		return true
	}
	return false
}

const meritSelectCols = `s.id, COALESCE(s.name,''), COALESCE(s.book_id,0), COALESCE(s.title,''),
	COALESCE(s.threshold,0), COALESCE(s.per_column,12), COALESCE(s.numeral,'arabic'), COALESCE(s.style,'stone'),
	COALESCE(s.border,'none'),
	COALESCE(s.show_amount,1), COALESCE(s.footer,''), COALESCE(s.created_at,''), COALESCE(s.updated_at,''),
	COALESCE(b.title,'')`

func scanMeritStele(scan func(...interface{}) error) (MeritStele, error) {
	var s MeritStele
	var show int
	err := scan(&s.ID, &s.Name, &s.BookID, &s.Title, &s.Threshold, &s.PerColumn, &s.Numeral, &s.Style, &s.Border,
		&show, &s.Footer, &s.CreatedAt, &s.UpdatedAt, &s.BookTitle)
	s.ShowAmt = show == 1
	if s.Numeral != "chinese" {
		s.Numeral = "arabic"
	}
	if !meritStyleOK(s.Style) {
		s.Style = "stone"
	}
	if !meritBorderOK(s.Border) {
		s.Border = "none"
	}
	return s, err
}

// ListMeritSteles 功德碑列表（最新在前）
// GET /api/merit/steles
func (h *AccountHandler) ListMeritSteles(c *gin.Context) {
	if !canViewAccounts(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}
	q := `SELECT ` + meritSelectCols + ` FROM merit_steles s LEFT JOIN account_books b ON b.id = s.book_id
		ORDER BY datetime(s.updated_at) DESC, s.id DESC`
	rows, err := h.DB.Query(q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	list := []MeritStele{}
	for rows.Next() {
		s, err := scanMeritStele(rows.Scan)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		list = append(list, s)
	}
	c.JSON(http.StatusOK, list)
}

// GetMeritStele 单个功德碑
// GET /api/merit/steles/:id
func (h *AccountHandler) GetMeritStele(c *gin.Context) {
	if !canViewAccounts(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	row := h.DB.QueryRow(`SELECT `+meritSelectCols+` FROM merit_steles s LEFT JOIN account_books b ON b.id = s.book_id WHERE s.id = ?`, id)
	s, err := scanMeritStele(row.Scan)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "功德碑不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

// CreateMeritStele 新增功德碑
// POST /api/merit/steles
func (h *AccountHandler) CreateMeritStele(c *gin.Context) {
	if !canManageBooks(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}
	var s MeritStele
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	meritNormalize(&s)
	if s.Name == "" {
		if s.Title != "" {
			s.Name = s.Title
		} else {
			s.Name = "功德碑"
		}
	}
	uid, uname := currentUser(c)
	_ = uid
	res, err := h.DB.Exec(`INSERT INTO merit_steles (name, book_id, title, threshold, per_column, numeral, style, border, show_amount, footer, created_by, created_by_name)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		s.Name, s.BookID, s.Title, s.Threshold, s.PerColumn, s.Numeral, s.Style, s.Border, boolToInt(s.ShowAmt), s.Footer, uid, uname)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.ID, _ = res.LastInsertId()
	row := h.DB.QueryRow(`SELECT `+meritSelectCols+` FROM merit_steles s LEFT JOIN account_books b ON b.id = s.book_id WHERE s.id = ?`, s.ID)
	out, _ := scanMeritStele(row.Scan)
	c.JSON(http.StatusOK, out)
}

// UpdateMeritStele 编辑功德碑
// PUT /api/merit/steles/:id
func (h *AccountHandler) UpdateMeritStele(c *gin.Context) {
	if !canManageBooks(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if _, err := strconv.Atoi(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var exists int
	if err := h.DB.QueryRow(`SELECT COUNT(*) FROM merit_steles WHERE id = ?`, id).Scan(&exists); err != nil || exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "功德碑不存在"})
		return
	}
	var s MeritStele
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	meritNormalize(&s)
	if s.Name == "" {
		if s.Title != "" {
			s.Name = s.Title
		} else {
			s.Name = "功德碑"
		}
	}
	_, err := h.DB.Exec(`UPDATE merit_steles SET name=?, book_id=?, title=?, threshold=?, per_column=?, numeral=?, style=?, border=?, show_amount=?, footer=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		s.Name, s.BookID, s.Title, s.Threshold, s.PerColumn, s.Numeral, s.Style, s.Border, boolToInt(s.ShowAmt), s.Footer, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	row := h.DB.QueryRow(`SELECT `+meritSelectCols+` FROM merit_steles s LEFT JOIN account_books b ON b.id = s.book_id WHERE s.id = ?`, id)
	out, _ := scanMeritStele(row.Scan)
	c.JSON(http.StatusOK, out)
}

// DeleteMeritStele 删除功德碑
// DELETE /api/merit/steles/:id
func (h *AccountHandler) DeleteMeritStele(c *gin.Context) {
	if !canManageBooks(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, err := h.DB.Exec(`DELETE FROM merit_steles WHERE id = ?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "功德碑不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
