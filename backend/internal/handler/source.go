package handler

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ============================ 资料来源（member_sources） ============================
// 一个成员可以有**多条**资料来源，每条带类型（族谱旧谱／口述／墓碑／家书／方志／其他，可自定义），
// 用于「据：光緒壬辰劉氏族譜·卷三·第12頁」这类引证。之所以用独立子表而不是 members 上加一列
// `source_note`：单列存不下多条、也分不出类型，且与既有 member_bio_entries（生平条目）同一套
// 「子表 + 成员页区块」模式，便于维护与整库备份（备份是整库 zip，新表天然在内）。
const (
	SourceKindDefault  = "其他" // 未填类型时的默认值
	SourceTextMaxRunes = 500  // 单条来源内容上限（字）
	SourceKindMaxRunes = 32   // 类型名上限（字）
)

// MemberSource 资料来源条目
type MemberSource struct {
	ID        int64  `json:"id"`
	MemberID  int64  `json:"member_id"`
	Text      string `json:"text"`
	Kind      string `json:"kind"`
	CreatedAt string `json:"created_at,omitempty"`
}

// SourceHandler 资料来源处理器
type SourceHandler struct{ DB *sql.DB }

// sourceMemberID 资料来源所属成员 id（查不到返回 0）
func sourceMemberID(db *sql.DB, id int64) int64 {
	var mid int64
	if err := db.QueryRow(`SELECT member_id FROM member_sources WHERE id = ?`, id).Scan(&mid); err != nil {
		return 0
	}
	return mid
}

// ListAll 列出全部资料来源（谱书渲染用：一次取全量，前端按 member_id 分组）
// GET /api/member-sources
func (h *SourceHandler) ListAll(c *gin.Context) {
	rows, err := h.DB.Query(`SELECT id, member_id, COALESCE("text",''), COALESCE(kind,''), COALESCE(created_at,'')
		FROM member_sources ORDER BY member_id, id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	list := []MemberSource{}
	for rows.Next() {
		var s MemberSource
		if err := rows.Scan(&s.ID, &s.MemberID, &s.Text, &s.Kind, &s.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		list = append(list, s)
	}
	c.JSON(http.StatusOK, list)
}

// ListByMember 列出某成员的资料来源（族员及以上可读）
// GET /api/members/:id/sources
func (h *SourceHandler) ListByMember(c *gin.Context) {
	memberID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	rows, err := h.DB.Query(`SELECT id, member_id, COALESCE("text",''), COALESCE(kind,''), COALESCE(created_at,'')
		FROM member_sources WHERE member_id = ? ORDER BY id`, memberID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	list := []MemberSource{}
	for rows.Next() {
		var s MemberSource
		if err := rows.Scan(&s.ID, &s.MemberID, &s.Text, &s.Kind, &s.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		list = append(list, s)
	}
	c.JSON(http.StatusOK, list)
}

// Create 新增一条资料来源（member_edit 权限；与其他成员资料一致，含房支隔离）
// POST /api/members/:id/sources
func (h *SourceHandler) Create(c *gin.Context) {
	memberID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var s MemberSource
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	s.Text = strings.TrimSpace(s.Text)
	s.Kind = strings.TrimSpace(s.Kind)
	if s.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "来源内容不能为空"})
		return
	}
	if n := len([]rune(s.Text)); n > SourceTextMaxRunes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "来源内容过长（最多 " + strconv.Itoa(SourceTextMaxRunes) + " 字）"})
		return
	}
	if s.Kind == "" {
		s.Kind = SourceKindDefault
	}
	if n := len([]rune(s.Kind)); n > SourceKindMaxRunes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "来源类型过长（最多 " + strconv.Itoa(SourceKindMaxRunes) + " 字）"})
		return
	}
	// 房支隔离：理事只能给本房支成员加来源
	if !checkMemberBranchAccess(c, h.DB, memberID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权给其他房支的成员添加资料来源"})
		return
	}
	res, err := h.DB.Exec(`INSERT INTO member_sources (member_id, "text", kind) VALUES (?,?,?)`,
		memberID, s.Text, s.Kind)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.ID, _ = res.LastInsertId()
	s.MemberID = memberID
	if err := h.DB.QueryRow(`SELECT COALESCE(created_at,'') FROM member_sources WHERE id = ?`, s.ID).Scan(&s.CreatedAt); err != nil {
		s.CreatedAt = ""
	}
	c.JSON(http.StatusOK, s)
}

// Delete 删除一条资料来源（member_edit 权限；与其他成员资料一致，含房支隔离）
// DELETE /api/sources/:id
func (h *SourceHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	mid := sourceMemberID(h.DB, id)
	if mid == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "条目不存在"})
		return
	}
	if !checkMemberBranchAccess(c, h.DB, mid) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除其他房支成员的资料来源"})
		return
	}
	res, err := h.DB.Exec(`DELETE FROM member_sources WHERE id = ?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "条目不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
