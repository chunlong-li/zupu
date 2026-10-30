package handler

import (
	"database/sql"
	"net/http"
	"strconv"

	"familytree/internal/lunar"

	"github.com/gin-gonic/gin"
)

// LunarHandler 日期换算处理器
type LunarHandler struct {
	DB *sql.DB
}

// Convert 公历转中文日期
// GET /api/lunar/convert?y=1900&m=8&d=1
func (h *LunarHandler) Convert(c *gin.Context) {
	y, _ := strconv.Atoi(c.DefaultQuery("y", "1900"))
	m, _ := strconv.Atoi(c.DefaultQuery("m", "1"))
	d, _ := strconv.Atoi(c.DefaultQuery("d", "1"))

	info := lunar.FromGregorian(y, m, d)
	c.JSON(http.StatusOK, info)
}

// LunarToSolar 农历转公历
// GET /api/lunar/to-solar?y=1900&m=8&d=1
func (h *LunarHandler) LunarToSolar(c *gin.Context) {
	y, _ := strconv.Atoi(c.DefaultQuery("y", "1900"))
	m, _ := strconv.Atoi(c.DefaultQuery("m", "1"))
	d, _ := strconv.Atoi(c.DefaultQuery("d", "1"))
	leap, _ := strconv.Atoi(c.DefaultQuery("leap", "0")) // 是否闰月

	solar := lunar.LunarToGregorian(y, m, leap, d)
	c.JSON(http.StatusOK, solar)
}
