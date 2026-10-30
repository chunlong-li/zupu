package handler

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// MigrationEvent 家族迁徙事件
type MigrationEvent struct {
	ID          int64  `json:"id"`
	FromPlace   string `json:"from_place"`
	ToPlace     string `json:"to_place"`
	Era         string `json:"era"`
	Year        int    `json:"year"`
	Branch      string `json:"branch"`
	MemberIDs   string `json:"member_ids"`
	MemberCount int    `json:"member_count"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at,omitempty"`
	// SyncResidence 为 true 时，保存后把涉及成员的现居地更新为「迁入地」（nil 视为 true）
	SyncResidence *bool `json:"sync_residence,omitempty"`
}

// syncMemberResidence 把涉及成员的现居地更新为指定地点，返回更新人数
func (h *MigrationEventHandler) syncMemberResidence(ids string, place string) int {
	place = strings.TrimSpace(place)
	if place == "" {
		return 0
	}
	n := 0
	for _, s := range strings.Split(ids, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		if _, err := h.DB.Exec(`UPDATE members SET current_place = ? WHERE id = ?`, place, id); err == nil {
			n++
		}
	}
	return n
}

// MigrationEventHandler 迁徙事件处理器
type MigrationEventHandler struct {
	DB *sql.DB
}

func (h *MigrationEventHandler) countMembers(ids string) int {
	n := 0
	for _, s := range strings.Split(ids, ",") {
		if strings.TrimSpace(s) != "" {
			n++
		}
	}
	return n
}

// List 迁徙事件列表
// GET /api/migrations
func (h *MigrationEventHandler) List(c *gin.Context) {
	mq := `SELECT id, COALESCE(from_place,''), COALESCE(to_place,''), COALESCE(era,''),
		COALESCE(year,0), COALESCE(branch,''), COALESCE(member_ids,''), COALESCE(member_count,0),
		COALESCE(description,''), COALESCE(created_at,'')
		FROM migrations`
	margs := []interface{}{}
	if bcond, bargs := BranchScope(c, h.DB, "id"); bcond != "" {
		mq += ` WHERE branch IN (SELECT name FROM branches WHERE ` + bcond + `)`
		margs = append(margs, bargs...)
	}
	mq += ` ORDER BY year, id`
	rows, err := h.DB.Query(mq, margs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	list := []MigrationEvent{}
	for rows.Next() {
		var m MigrationEvent
		if err := rows.Scan(&m.ID, &m.FromPlace, &m.ToPlace, &m.Era, &m.Year, &m.Branch, &m.MemberIDs, &m.MemberCount, &m.Description, &m.CreatedAt); err != nil {
			continue
		}
		if m.MemberCount == 0 {
			m.MemberCount = h.countMembers(m.MemberIDs)
		}
		list = append(list, m)
	}
	c.JSON(http.StatusOK, list)
}

// Create 新增迁徙事件
// POST /api/migrations
func (h *MigrationEventHandler) Create(c *gin.Context) {
	var m MigrationEvent
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m.FromPlace = strings.TrimSpace(m.FromPlace)
	m.ToPlace = strings.TrimSpace(m.ToPlace)
	if m.FromPlace == "" || m.ToPlace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "迁出地、迁入地不能为空"})
		return
	}
	m.MemberCount = h.countMembers(m.MemberIDs)
	res, err := h.DB.Exec(`INSERT INTO migrations (from_place, to_place, era, year, branch, member_ids, member_count, description)
		VALUES (?,?,?,?,?,?,?,?)`,
		m.FromPlace, m.ToPlace, m.Era, m.Year, m.Branch, m.MemberIDs, m.MemberCount, m.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	m.ID, _ = res.LastInsertId()
	if m.SyncResidence == nil || *m.SyncResidence {
		h.syncMemberResidence(m.MemberIDs, m.ToPlace)
	}
	c.JSON(http.StatusOK, m)
}

// Update 修改迁徙事件
// PUT /api/migrations/:id
func (h *MigrationEventHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var m MigrationEvent
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m.FromPlace = strings.TrimSpace(m.FromPlace)
	m.ToPlace = strings.TrimSpace(m.ToPlace)
	if m.FromPlace == "" || m.ToPlace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "迁出地、迁入地不能为空"})
		return
	}
	m.MemberCount = h.countMembers(m.MemberIDs)
	_, err := h.DB.Exec(`UPDATE migrations SET from_place=?, to_place=?, era=?, year=?, branch=?, member_ids=?, member_count=?, description=? WHERE id=?`,
		m.FromPlace, m.ToPlace, m.Era, m.Year, m.Branch, m.MemberIDs, m.MemberCount, m.Description, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	m.ID = id
	if m.SyncResidence == nil || *m.SyncResidence {
		h.syncMemberResidence(m.MemberIDs, m.ToPlace)
	}
	c.JSON(http.StatusOK, m)
}

// Delete 删除迁徙事件
// DELETE /api/migrations/:id
func (h *MigrationEventHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if _, err := h.DB.Exec(`DELETE FROM migrations WHERE id = ?`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
