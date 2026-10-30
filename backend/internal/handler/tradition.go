package handler

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Tradition 家族传统日程
type Tradition struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	LunarMonth    int    `json:"lunar_month"`
	LunarDay      int    `json:"lunar_day"`
	IsLeapMonth   bool   `json:"is_leap_month"`
	Description   string `json:"description"`
	ThisYearSolar string `json:"this_year_solar,omitempty"`
	// 农历日 = 0 即「不固定」（清明按节气、除夕按年末等）
	Unfixed bool `json:"unfixed"`
	// 本年临时日期（仅「不固定」且未过期时有值，YYYY-MM-DD）
	TempSolarDate string `json:"temp_solar_date,omitempty"`
	TempYear      int    `json:"temp_year,omitempty"`
	TempDays      int    `json:"temp_days"` // 距今天数，0 = 今天
	// 本年临时日期已过期（仅用于界面提示，此时按「不固定」处理，自动失效）
	TempExpiredDate string `json:"temp_expired_date,omitempty"`
}

// TraditionHandler 传统日程处理器
type TraditionHandler struct {
	DB *sql.DB
}

// ensureTraditionTempDates 建立传统节日「临时日期」表（按 节日 + 年份 存，保留历史年份）。
// 放在 handler 里惰性建表，安装升级后无需额外迁移步骤即可生效。
func ensureTraditionTempDates(db *sql.DB) {
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS tradition_temp_dates (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tradition_id INT NOT NULL,
		year INT NOT NULL,
		solar_date VARCHAR(16) NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(tradition_id, year)
	)`)
}

// ListTraditions 列出所有传统日程（按农历月日排序）
// GET /api/traditions
func (h *TraditionHandler) ListTraditions(c *gin.Context) {
	ensureTraditionTempDates(h.DB)
	rows, err := h.DB.Query(`SELECT id, title, lunar_month, lunar_day, is_leap_month, description
		FROM traditions ORDER BY lunar_month, lunar_day, id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// 本年临时日期（不固定节日）
	tempDates := map[int64]string{}
	if trows, terr := h.DB.Query(`SELECT tradition_id, solar_date FROM tradition_temp_dates WHERE year = ?`, now.Year()); terr == nil {
		for trows.Next() {
			var id int64
			var ds string
			if trows.Scan(&id, &ds) == nil {
				tempDates[id] = ds
			}
		}
		trows.Close()
	}

	list := []Tradition{}
	for rows.Next() {
		var t Tradition
		var leap int
		if err := rows.Scan(&t.ID, &t.Title, &t.LunarMonth, &t.LunarDay, &leap, &t.Description); err != nil {
			continue
		}
		t.IsLeapMonth = leap != 0
		t.Unfixed = t.LunarDay == 0
		if t.Unfixed {
			// 不固定：只有本年临时日期且未过期才显示具体日期；过期即回到「不固定」，不沿用到下一年
			if ds, ok := tempDates[t.ID]; ok && ds != "" {
				if d, ok2 := parseSolarDate(ds); ok2 {
					if !d.Before(today) {
						t.TempSolarDate = ds
						t.TempYear = now.Year()
						t.TempDays = int(d.Sub(today).Hours()/24 + 0.5)
						t.ThisYearSolar = ds
					} else {
						t.TempExpiredDate = ds
					}
				}
			}
		} else {
			t.ThisYearSolar = safeLunarToSolar(now.Year(), t.LunarMonth, t.IsLeapMonth, t.LunarDay)
		}
		list = append(list, t)
	}
	c.JSON(http.StatusOK, list)
}

// SetTraditionTempDate 为「不固定」的传统节日设置/清除本年临时日期
// PUT /api/traditions/:id/temp-date
// 请求：{"year": 2026, "date": "2026-10-22"}；date 传空串表示清除该年临时日期
func (h *TraditionHandler) SetTraditionTempDate(c *gin.Context) {
	ensureTraditionTempDates(h.DB)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Year int    `json:"year"`
		Date string `json:"date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var lunarDay int
	var title string
	if err := h.DB.QueryRow(`SELECT COALESCE(title,''), COALESCE(lunar_day,0) FROM traditions WHERE id = ?`, id).
		Scan(&title, &lunarDay); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "节日不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 只有「不固定」（农历日 = 0）的节日才需要临时日期
	if lunarDay != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该节日有固定农历日，无需临时日期"})
		return
	}

	now := time.Now()
	if req.Year == 0 {
		req.Year = now.Year()
	}
	if req.Year < 1900 || req.Year > 2100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "年份需在 1900~2100 之间"})
		return
	}

	// 清除该年临时日期
	if req.Date == "" {
		if _, err := h.DB.Exec(`DELETE FROM tradition_temp_dates WHERE tradition_id = ? AND year = ?`, id, req.Year); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "tradition_id": id, "year": req.Year, "date": ""})
		return
	}

	d, ok := parseSolarDate(req.Date)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "日期格式需为 YYYY-MM-DD"})
		return
	}
	// 临时日期只针对某一年：日期必须落在所填年份内，避免跨年误用
	if d.Year() != req.Year {
		c.JSON(http.StatusBadRequest, gin.H{"error": "日期年份需与所选年份一致"})
		return
	}

	_, err := h.DB.Exec(`INSERT INTO tradition_temp_dates (tradition_id, year, solar_date) VALUES (?,?,?)
		ON CONFLICT(tradition_id, year) DO UPDATE SET solar_date = excluded.solar_date, created_at = CURRENT_TIMESTAMP`,
		id, req.Year, req.Date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "tradition_id": id, "year": req.Year, "date": req.Date})
}

// CreateTradition 新增传统日程
// POST /api/traditions
func (h *TraditionHandler) CreateTradition(c *gin.Context) {
	var t Tradition
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if t.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标题不能为空"})
		return
	}
	if t.LunarMonth < 1 || t.LunarMonth > 12 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "农历月份需在 1~12 之间"})
		return
	}
	// 农历日：1~30；0 表示「不固定」（如清明按节气、除夕按年末确定，没有固定农历日）
	if t.LunarDay < 0 || t.LunarDay > 30 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "农历日期需为 0（不固定）或 1~30"})
		return
	}
	leap := 0
	if t.IsLeapMonth {
		leap = 1
	}
	res, err := h.DB.Exec(`INSERT INTO traditions (title, lunar_month, lunar_day, is_leap_month, description) VALUES (?,?,?,?,?)`,
		t.Title, t.LunarMonth, t.LunarDay, leap, t.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	t.ID, _ = res.LastInsertId()
	now := time.Now()
	t.Unfixed = t.LunarDay == 0
	if !t.Unfixed {
		t.ThisYearSolar = safeLunarToSolar(now.Year(), t.LunarMonth, t.IsLeapMonth, t.LunarDay)
	}
	c.JSON(http.StatusOK, t)
}

// UpdateTradition 更新传统日程
// PUT /api/traditions/:id
func (h *TraditionHandler) UpdateTradition(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var t Tradition
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if t.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标题不能为空"})
		return
	}
	if t.LunarMonth < 1 || t.LunarMonth > 12 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "农历月份需在 1~12 之间"})
		return
	}
	// 农历日：1~30；0 表示「不固定」（如清明按节气、除夕按年末确定，没有固定农历日）
	if t.LunarDay < 0 || t.LunarDay > 30 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "农历日期需为 0（不固定）或 1~30"})
		return
	}
	leap := 0
	if t.IsLeapMonth {
		leap = 1
	}
	_, err := h.DB.Exec(`UPDATE traditions SET title=?, lunar_month=?, lunar_day=?, is_leap_month=?, description=? WHERE id=?`,
		t.Title, t.LunarMonth, t.LunarDay, leap, t.Description, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	t.ID = id
	now := time.Now()
	t.Unfixed = t.LunarDay == 0
	if !t.Unfixed {
		t.ThisYearSolar = safeLunarToSolar(now.Year(), t.LunarMonth, t.IsLeapMonth, t.LunarDay)
	}
	c.JSON(http.StatusOK, t)
}

// DeleteTradition 删除传统日程
// DELETE /api/traditions/:id
func (h *TraditionHandler) DeleteTradition(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	_, err := h.DB.Exec(`DELETE FROM traditions WHERE id=?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
