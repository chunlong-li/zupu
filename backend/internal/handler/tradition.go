package handler

import (
	"database/sql"
	"familytree/internal/lunar"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
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
		// 除夕：腊月最后一天（廿九或三十），每年自动算，不用手工填临时日期
		// 可按日历自动算的「不固定」节日：除夕=腊月最后一天；清明=节气
		if t.LunarDay == 0 {
			auto := ""
			switch {
			case strings.Contains(t.Title, "除夕"):
				auto = lunar.LastDayOfLunarYear(now.Year())
			case strings.Contains(t.Title, "清明"):
				auto = lunar.JieQiGregorian(now.Year(), "清明")
			}
			if auto != "" {
				t.ThisYearSolar = auto
				list = append(list, t)
				continue
			}
		}
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
	// 排序：按农历月日；「不固定」但能自动算的节日按实际位置排
	//   除夕 = 腊月最后一天 → 排在腊月所有节日之后
	//   清明 = 节气在 3 月初 → 排在三月节日之前
	sort.SliceStable(list, func(i, j int) bool {
		return tradOrderKey(list[i]) < tradOrderKey(list[j])
	})
	c.JSON(http.StatusOK, list)
}

// SetTraditionTempDate 为「不固定」的传统节日设置/清除本年临时日期
// PUT /api/traditions/:id/temp-date
// 请求：{"year": 2026, "date": "2026-10-22"}；date 传空串表示清除该年临时日期
func (h *TraditionHandler) SetTraditionTempDate(c *gin.Context) {
	ensureTraditionTempDates(h.DB)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Year       int    `json:"year"`
		Date       string `json:"date"`
		LunarMonth int    `json:"lunar_month"` // 传农历月/日时优先（前端改成农历选择）
		LunarDay   int    `json:"lunar_day"`
		IsLeap     bool   `json:"is_leap"`
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
	// 前端现在选的是「农历月/日」，这里转成公历再存（后端 lunar 包已具备能力）
	if req.LunarMonth > 0 && req.LunarDay > 0 {
		leap := 0
		if req.IsLeap {
			leap = 1
		}
		info := lunar.LunarToGregorian(req.Year, req.LunarMonth, leap, req.LunarDay)
		if info.Gregorian == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "农历日期无效"})
			return
		}
		req.Date = info.Gregorian
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

// SeedDefaultTraditions 一键补全「通用节日」：只插入库里还没有的（按标题去重），不动已有条目。
// POST /api/traditions/seed-defaults
//
// 除夕、清明这类「每年日期不定」的节日，农历日填 0（不固定），
// 由用户在传统日程里按年份填临时日期（可选农历月/日，后端换算成公历）。
func (h *TraditionHandler) SeedDefaultTraditions(c *gin.Context) {
	ensureTraditionTempDates(h.DB)

	defaults := []struct {
		Title string
		Month int
		Day   int // 0 = 不固定
		Desc  string
	}{
		{"春节", 1, 1, "农历正月初一，岁首"},
		{"除夕", 12, 0, "农历腊月最后一天（廿九或三十），日期每年不同，请按年填写临时日期"},
		{"元宵节", 1, 15, "农历正月十五，上元节"},
		{"龙抬头", 2, 2, "农历二月初二"},
		{"清明节", 3, 0, "按节气，公历 4 月 4~6 日，每年不同，请按年填写临时日期"},
		{"端午节", 5, 5, "农历五月初五"},
		{"七夕", 7, 7, "农历七月初七"},
		{"中元节", 7, 15, "农历七月十五，七月半"},
		{"中秋节", 8, 15, "农历八月十五"},
		{"重阳节", 9, 9, "农历九月初九"},
		{"寒衣节", 10, 1, "农历十月初一"},
		{"下元节", 10, 15, "农历十月十五"},
		{"腊八节", 12, 8, "农历十二月初八"},
		{"小年", 12, 23, "农历腊月廿三（南方多为廿四）"},
	}

	added := 0
	for _, d := range defaults {
		var exists int
		_ = h.DB.QueryRow(`SELECT COUNT(*) FROM traditions WHERE title = ?`, d.Title).Scan(&exists)
		if exists > 0 {
			continue
		}
		if _, err := h.DB.Exec(
			`INSERT INTO traditions (title, lunar_month, lunar_day, is_leap_month, description) VALUES (?,?,?,0,?)`,
			d.Title, d.Month, d.Day, d.Desc,
		); err == nil {
			added++
		}
	}
	c.JSON(http.StatusOK, gin.H{"added": added, "message": fmt.Sprintf("已补充 %d 个通用节日", added)})
}

// tradOrderKey 排序列：农历月 * 100 + 日；不固定节日用推断位置
func tradOrderKey(t Tradition) int {
	day := t.LunarDay
	if day == 0 {
		switch {
		case strings.Contains(t.Title, "除夕"):
			day = 99 // 腊月最末
		case strings.Contains(t.Title, "清明"):
			day = 1 // 三月初
		default:
			day = 50 // 其它不固定：放月中
		}
	}
	return t.LunarMonth*100 + day
}
