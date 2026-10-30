package handler

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type IncenseHandler struct {
	DB *sql.DB
}

// IncenseBurnSeconds 一炷香时长（秒）= 古代 30 分钟
const IncenseBurnSeconds = 1800

// Offer 上香：同一人香未烧完不能再次上香；username 仅供留痕存档，不在界面显示
func (h *IncenseHandler) Offer(c *gin.Context) {
	userID := c.GetInt("user_id")
	username := c.GetString("username")
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	now := time.Now().Unix()

	// 香烧完即删：清理已过期的记录
	h.DB.Exec(`DELETE FROM incense_offers WHERE expires_at <= ?`, now)

	// 同一人香未烧完，不能再上（后台不限制上香总数，仅限同一人）
	var existing int
	h.DB.QueryRow(`SELECT COUNT(*) FROM incense_offers WHERE user_id = ?`, userID).Scan(&existing)
	if existing > 0 {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": "香还未烧完，请稍后再上"})
		return
	}

	expires := now + IncenseBurnSeconds
	_, err := h.DB.Exec(`INSERT INTO incense_offers (user_id, username, count, offered_at, expires_at) VALUES (?,?,?,?,?)`,
		userID, username, 3, now, expires)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "offered_at": now, "expires_at": expires})
}

// List 返回活跃（未烧完）的上香记录；顺带清理已烧完的香
func (h *IncenseHandler) List(c *gin.Context) {
	now := time.Now().Unix()
	h.DB.Exec(`DELETE FROM incense_offers WHERE expires_at <= ?`, now)

	rows, err := h.DB.Query(`SELECT id, user_id, COALESCE(username,''), count, offered_at, expires_at FROM incense_offers ORDER BY offered_at`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type Offer struct {
		ID        int64  `json:"id"`
		UserID    int    `json:"user_id"`
		Username  string `json:"username"`
		Count     int    `json:"count"`
		OfferedAt int64  `json:"offered_at"`
		ExpiresAt int64  `json:"expires_at"`
	}
	offers := []Offer{}
	for rows.Next() {
		var o Offer
		if err := rows.Scan(&o.ID, &o.UserID, &o.Username, &o.Count, &o.OfferedAt, &o.ExpiresAt); err != nil {
			continue
		}
		offers = append(offers, o)
	}
	c.JSON(http.StatusOK, gin.H{"offers": offers})
}
