package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// BioEntry 生平条目（成员资料卡的生平由多条「标题+内容」条目组成）
type BioEntry struct {
	ID        int64  `json:"id"`
	MemberID  int64  `json:"member_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	SortOrder int    `json:"sort_order"`
}

// BioHandler 生平条目处理器
type BioHandler struct{ DB *sql.DB }

// ListByMember 列出某成员的生平条目（族员及以上可读）
// GET /api/members/:id/bio-entries
func (h *BioHandler) ListByMember(c *gin.Context) {
	memberID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	rows, err := h.DB.Query(`SELECT id, member_id, title, COALESCE(content,''), COALESCE(sort_order,0)
		FROM member_bio_entries WHERE member_id = ? ORDER BY sort_order, id`, memberID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	entries := []BioEntry{}
	for rows.Next() {
		var e BioEntry
		if err := rows.Scan(&e.ID, &e.MemberID, &e.Title, &e.Content, &e.SortOrder); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		entries = append(entries, e)
	}
	c.JSON(http.StatusOK, entries)
}

// Create 新增一条生平条目（理事提交审核；理事长/超级管理员直接写库）
// POST /api/members/:id/bio-entries
func (h *BioHandler) Create(c *gin.Context) {
	memberID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var e BioEntry
	if err := c.ShouldBindJSON(&e); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	e.Title = strings.TrimSpace(e.Title)
	if e.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标题不能为空"})
		return
	}
	// 房支隔离：理事只能给本房支成员新增生平
	if !checkMemberBranchAccess(c, h.DB, memberID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权给其他房支的成员新增生平"})
		return
	}
	// 理事提交 → 进入审核队列
	if isElder(c) {
		e.MemberID = memberID
		submitDraft(h.DB, "member_bio", "create", memberID, e, c)
		return
	}
	created, err := h.applyCreateMemberBio(memberID, &e)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, created)
}

// applyCreateMemberBio 实际执行新增成员生平条目（供直接写入与审核通过合并共用）
func (h *BioHandler) applyCreateMemberBio(memberID int64, e *BioEntry) (*BioEntry, error) {
	if e.SortOrder <= 0 {
		var maxOrder int
		h.DB.QueryRow(`SELECT COALESCE(MAX(sort_order),0) FROM member_bio_entries WHERE member_id = ?`, memberID).Scan(&maxOrder)
		e.SortOrder = maxOrder + 1
	}
	res, err := h.DB.Exec(`INSERT INTO member_bio_entries (member_id, title, content, sort_order) VALUES (?,?,?,?)`,
		memberID, e.Title, e.Content, e.SortOrder)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	e.ID = id
	e.MemberID = memberID
	return e, nil
}

// Update 更新一条生平条目（理事提交审核；理事长/超级管理员直接写库）
// PUT /api/bio-entries/:id
func (h *BioHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var e BioEntry
	if err := c.ShouldBindJSON(&e); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	e.Title = strings.TrimSpace(e.Title)
	if e.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标题不能为空"})
		return
	}
	// 房支隔离：理事只能改本房支成员的生平
	if !checkMemberBranchAccess(c, h.DB, bioEntryMemberID(h.DB, id)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权修改其他房支成员的生平"})
		return
	}
	if isElder(c) {
		submitDraft(h.DB, "member_bio", "update", id, e, c)
		return
	}
	if err := h.applyUpdateMemberBio(id, &e); err != nil {
		if err.Error() == "条目不存在" {
			c.JSON(http.StatusNotFound, gin.H{"error": "条目不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, e)
}

// applyUpdateMemberBio 实际执行更新成员生平条目
func (h *BioHandler) applyUpdateMemberBio(id int64, e *BioEntry) error {
	res, err := h.DB.Exec(`UPDATE member_bio_entries SET title=?, content=?, sort_order=? WHERE id=?`,
		e.Title, e.Content, e.SortOrder, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("条目不存在")
	}
	return nil
}

// Delete 删除一条生平条目（理事提交审核；理事长/超级管理员直接写库）
// DELETE /api/bio-entries/:id
func (h *BioHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	// 房支隔离：理事只能删本房支成员的生平
	if !checkMemberBranchAccess(c, h.DB, bioEntryMemberID(h.DB, id)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除其他房支成员的生平"})
		return
	}
	if isElder(c) {
		submitDraft(h.DB, "member_bio", "delete", id, gin.H{"id": id}, c)
		return
	}
	if err := h.applyDeleteMemberBio(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// applyDeleteMemberBio 实际执行删除成员生平条目
func (h *BioHandler) applyDeleteMemberBio(id int64) error {
	_, err := h.DB.Exec(`DELETE FROM member_bio_entries WHERE id = ?`, id)
	return err
}


// —— 配偶生平条目 ——

// SpouseBioEntry 配偶生平条目
type SpouseBioEntry struct {
	ID        int64  `json:"id"`
	SpouseID  int64  `json:"spouse_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	SortOrder int    `json:"sort_order"`
}

// ListBySpouse 列出某配偶的生平条目（族员及以上可读）
// GET /api/spouses/:id/bio-entries
func (h *BioHandler) ListBySpouse(c *gin.Context) {
	spouseID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	rows, err := h.DB.Query(`SELECT id, spouse_id, title, COALESCE(content,''), COALESCE(sort_order,0)
		FROM spouse_bio_entries WHERE spouse_id = ? ORDER BY sort_order, id`, spouseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	entries := []SpouseBioEntry{}
	for rows.Next() {
		var e SpouseBioEntry
		if err := rows.Scan(&e.ID, &e.SpouseID, &e.Title, &e.Content, &e.SortOrder); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		entries = append(entries, e)
	}
	c.JSON(http.StatusOK, entries)
}

// CreateSpouse 新增一条配偶生平条目（理事提交审核；理事长/超级管理员直接写库）
// POST /api/spouses/:id/bio-entries
func (h *BioHandler) CreateSpouse(c *gin.Context) {
	spouseID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var e SpouseBioEntry
	if err := c.ShouldBindJSON(&e); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	e.Title = strings.TrimSpace(e.Title)
	if e.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标题不能为空"})
		return
	}
	// 房支隔离：理事只能给本房支成员的配偶新增生平
	if !checkMemberBranchAccess(c, h.DB, spouseMemberID(h.DB, spouseID)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权给其他房支成员的配偶新增生平"})
		return
	}
	if isElder(c) {
		e.SpouseID = spouseID
		submitDraft(h.DB, "spouse_bio", "create", spouseID, e, c)
		return
	}
	created, err := h.applyCreateSpouseBio(spouseID, &e)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, created)
}

// applyCreateSpouseBio 实际执行新增配偶生平条目
func (h *BioHandler) applyCreateSpouseBio(spouseID int64, e *SpouseBioEntry) (*SpouseBioEntry, error) {
	if e.SortOrder <= 0 {
		var maxOrder int
		h.DB.QueryRow(`SELECT COALESCE(MAX(sort_order),0) FROM spouse_bio_entries WHERE spouse_id = ?`, spouseID).Scan(&maxOrder)
		e.SortOrder = maxOrder + 1
	}
	res, err := h.DB.Exec(`INSERT INTO spouse_bio_entries (spouse_id, title, content, sort_order) VALUES (?,?,?,?)`,
		spouseID, e.Title, e.Content, e.SortOrder)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	e.ID = id
	e.SpouseID = spouseID
	return e, nil
}

// UpdateSpouse 更新一条配偶生平条目（理事提交审核；理事长/超级管理员直接写库）
// PUT /api/spouse-bio-entries/:id
func (h *BioHandler) UpdateSpouse(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var e SpouseBioEntry
	if err := c.ShouldBindJSON(&e); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	e.Title = strings.TrimSpace(e.Title)
	if e.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标题不能为空"})
		return
	}
	// 房支隔离：理事只能改本房支成员配偶的生平
	if !checkMemberBranchAccess(c, h.DB, spouseBioMemberID(h.DB, id)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权修改其他房支成员配偶的生平"})
		return
	}
	if isElder(c) {
		submitDraft(h.DB, "spouse_bio", "update", id, e, c)
		return
	}
	if err := h.applyUpdateSpouseBio(id, &e); err != nil {
		if err.Error() == "条目不存在" {
			c.JSON(http.StatusNotFound, gin.H{"error": "条目不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, e)
}

// applyUpdateSpouseBio 实际执行更新配偶生平条目
func (h *BioHandler) applyUpdateSpouseBio(id int64, e *SpouseBioEntry) error {
	res, err := h.DB.Exec(`UPDATE spouse_bio_entries SET title=?, content=?, sort_order=? WHERE id=?`,
		e.Title, e.Content, e.SortOrder, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("条目不存在")
	}
	return nil
}

// DeleteSpouse 删除一条配偶生平条目（理事提交审核；理事长/超级管理员直接写库）
// DELETE /api/spouse-bio-entries/:id
func (h *BioHandler) DeleteSpouse(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	// 房支隔离：理事只能删本房支成员配偶的生平
	if !checkMemberBranchAccess(c, h.DB, spouseBioMemberID(h.DB, id)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除其他房支成员配偶的生平"})
		return
	}
	if isElder(c) {
		submitDraft(h.DB, "spouse_bio", "delete", id, gin.H{"id": id}, c)
		return
	}
	if err := h.applyDeleteSpouseBio(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// applyDeleteSpouseBio 实际执行删除配偶生平条目
func (h *BioHandler) applyDeleteSpouseBio(id int64) error {
	_, err := h.DB.Exec(`DELETE FROM spouse_bio_entries WHERE id = ?`, id)
	return err
}
