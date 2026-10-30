package handler

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// memberFlagsMap 返回 id -> {无嗣, 迁居失联}。单独一次查询，避免改动各处成员查询冗长的列清单。
func memberFlagsMap(db *sql.DB) map[int64][2]int {
	m := map[int64][2]int{}
	rows, err := db.Query(`SELECT id, COALESCE(no_heir,0), COALESCE(lost_contact,0) FROM members`)
	if err != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var a, b int
		if err := rows.Scan(&id, &a, &b); err == nil {
			m[id] = [2]int{a, b}
		}
	}
	return m
}

// attachMemberFlags 批量补齐「无嗣 / 迁居失联」两个标记。
func attachMemberFlags(db *sql.DB, members []Member) {
	if len(members) == 0 {
		return
	}
	m := memberFlagsMap(db)
	for i := range members {
		if v, ok := m[members[i].ID]; ok {
			members[i].NoHeir = v[0]
			members[i].LostContact = v[1]
		}
	}
}

// bioFlagSet 有传记的成员 id 集合（member_id > 0）。
func bioFlagSet(db *sql.DB) map[int64]bool {
	set := map[int64]bool{}
	rows, err := db.Query(`SELECT DISTINCT member_id FROM biographies WHERE COALESCE(member_id,0) > 0`)
	if err != nil {
		return set
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			set[id] = true
		}
	}
	return set
}

// bioSpouseFlagSet 有传记的配偶 id 集合（spouse_id > 0）。
func bioSpouseFlagSet(db *sql.DB) map[int64]bool {
	set := map[int64]bool{}
	rows, err := db.Query(`SELECT DISTINCT spouse_id FROM biographies WHERE COALESCE(spouse_id,0) > 0`)
	if err != nil {
		return set
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			set[id] = true
		}
	}
	return set
}

// UpdateFlags 设置成员的「无嗣 / 迁居失联」标记（单独接口，避免整表 UPDATE）。
// PUT /api/members/:id/flags  { "no_heir": 0|1, "lost_contact": 0|1 }
func (h *MemberHandler) UpdateFlags(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		NoHeir      *int `json:"no_heir"`
		LostContact *int `json:"lost_contact"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var cur struct{ NoHeir, LostContact int }
	if err := h.DB.QueryRow(`SELECT COALESCE(no_heir,0), COALESCE(lost_contact,0) FROM members WHERE id=?`, id).
		Scan(&cur.NoHeir, &cur.LostContact); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "成员不存在"})
		return
	}
	noHeir := cur.NoHeir
	lost := cur.LostContact
	if req.NoHeir != nil {
		noHeir = *req.NoHeir
	}
	if req.LostContact != nil {
		lost = *req.LostContact
	}
	if _, err := h.DB.Exec(
		`UPDATE members SET no_heir=?, lost_contact=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		noHeir, lost, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "no_heir": noHeir, "lost_contact": lost})
}
