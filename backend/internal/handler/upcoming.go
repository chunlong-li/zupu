package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// 首页「未来 30 天」提醒：传统节日 + 成员生日 + 忌日（逝世周年）
//
//   2026-10-17 新增。数据只对**已登录账号**开放：游客（无 token）由
//   AuthMiddleware 直接 401；本机免登录（local_guest，user_id=0）同样不放行，
//   保证「未登录拿不到这些数据」。
//
//   传统节日里「不固定」（农历日 = 0，如清明/端午/中秋/除夕）的条目，
//   按本年临时日期（tradition_temp_dates，见 tradition.go）参与计算；
//   临时日期已过期则视为「不固定」，不进入 30 天窗口。
// ============================================================================

// UpcomingWindowDays 首页提醒窗口天数（含今天）
const UpcomingWindowDays = 30

// UpcomingItem 首页「未来 30 天」条目
type UpcomingItem struct {
	Date      string `json:"date"`               // 公历 YYYY-MM-DD
	DateMD    string `json:"date_md"`            // MM-DD
	Weekday   string `json:"weekday"`            // 周一…周日
	Days      int    `json:"days"`               // 距今天数，0 = 今天
	Kind      string `json:"kind"`               // festival | birthday | deathday
	Title     string `json:"title"`              // 节日名 / 成员姓名
	Subtitle  string `json:"subtitle,omitempty"` // 农历日期 / 逝世周年
	Lunar     string `json:"lunar,omitempty"`    // 农历日期原文
	MemberID  int64  `json:"member_id,omitempty"`
	SpouseID  int64  `json:"spouse_id,omitempty"`
	Years     int    `json:"years,omitempty"`      // 生日=年龄，忌日=逝世周年
	TempFixed bool   `json:"temp_fixed,omitempty"` // 节日：按临时日期计算
}

// UpcomingHandler 首页 30 天提醒处理器
type UpcomingHandler struct {
	DB *sql.DB
}

var weekdayCN = []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

// parseSolarDate 严格解析 YYYY-MM-DD（按本地时区零点）
func parseSolarDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if len(s) != 10 {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	// 防止 2026-02-31 这类被正常化后的假日期
	if t.Format("2006-01-02") != s {
		return time.Time{}, false
	}
	return t, true
}

// solarOccurrence 取今年（若已过则明年）的「公历月-日」对应的日期
func solarOccurrence(month, day int, today time.Time) (time.Time, bool) {
	for y := today.Year(); y <= today.Year()+1; y++ {
		s := fmt.Sprintf("%04d-%02d-%02d", y, month, day)
		if t, ok := parseSolarDate(s); ok && !t.Before(today) {
			return t, true
		}
	}
	return time.Time{}, false
}

// solarMD 从 birth_gregorian / death_gregorian 中解析月日（兼容只有年份的旧数据）
func solarMD(s string) (month, day, year int, ok bool) {
	s = strings.TrimSpace(s)
	if len(s) != 10 {
		return 0, 0, 0, false
	}
	t, ok2 := parseSolarDate(s)
	if !ok2 {
		return 0, 0, 0, false
	}
	return int(t.Month()), t.Day(), t.Year(), true
}

// lunarOccurrence 取今年（若已过则明年）的农历月日对应公历日期
func lunarOccurrence(month int, isLeap bool, day int, today time.Time) (time.Time, bool) {
	for y := today.Year(); y <= today.Year()+1; y++ {
		if t, ok := parseSolarDate(safeLunarToSolar(y, month, isLeap, day)); ok && !t.Before(today) {
			return t, true
		}
	}
	return time.Time{}, false
}

// GetUpcoming 首页「未来 30 天」（登录可见）
// GET /api/dashboard/upcoming
func (h *UpcomingHandler) GetUpcoming(c *gin.Context) {
	// 双重保险：路由已挂 AuthMiddleware + RequireRole(族员)，
	// 这里再拒一次未登录 / 本机免登录访客，确保未登录拿不到节日/生日/忌日数据
	if c.GetString("role") == "" || c.GetInt("user_id") <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	windowEnd := today.AddDate(0, 0, UpcomingWindowDays-1)

	items := []UpcomingItem{}
	add := func(date time.Time, kind, title, subtitle, lunar string, memberID, spouseID int64, years int, tempFixed bool) {
		if date.Before(today) || date.After(windowEnd) {
			return
		}
		items = append(items, UpcomingItem{
			Date:      date.Format("2006-01-02"),
			DateMD:    date.Format("01-02"),
			Weekday:   weekdayCN[int(date.Weekday())],
			Days:      int(date.Sub(today).Hours()/24 + 0.5),
			Kind:      kind,
			Title:     title,
			Subtitle:  subtitle,
			Lunar:     lunar,
			MemberID:  memberID,
			SpouseID:  spouseID,
			Years:     years,
			TempFixed: tempFixed,
		})
	}

	// ---------- 1) 传统节日 ----------
	ensureTraditionTempDates(h.DB)
	// 本年/明年的临时日期（不固定节日专用）
	tempDates := map[int64]map[int]string{}
	if trows, err := h.DB.Query(`SELECT tradition_id, year, solar_date FROM tradition_temp_dates WHERE year IN (?,?)`,
		today.Year(), today.Year()+1); err == nil {
		for trows.Next() {
			var id int64
			var y int
			var ds string
			if trows.Scan(&id, &y, &ds) == nil {
				if tempDates[id] == nil {
					tempDates[id] = map[int]string{}
				}
				tempDates[id][y] = ds
			}
		}
		trows.Close()
	}
	if rows, err := h.DB.Query(`SELECT id, title, lunar_month, lunar_day, is_leap_month FROM traditions ORDER BY id`); err == nil {
		for rows.Next() {
			var id int64
			var title string
			var month, day, leap int
			if rows.Scan(&id, &title, &month, &day, &leap) != nil {
				continue
			}
			if day == 0 {
				// 不固定：有临时日期且未过期才参与 30 天窗口
				for y := today.Year(); y <= today.Year()+1; y++ {
					ds := tempDates[id][y]
					t, ok := parseSolarDate(ds)
					if !ok || t.Before(today) {
						continue
					}
					add(t, "festival", title, "传统节日 · 临时日期", "", 0, 0, 0, true)
					break
				}
				continue
			}
			// 固定农历日：清明/端午之外按农历月日每年换算
			if t, ok := lunarOccurrence(month, leap != 0, day, today); ok {
				add(t, "festival", title, "传统节日", "", 0, 0, 0, false)
			}
		}
		rows.Close()
	}

	// ---------- 2) 族人生日 / 忌日 ----------
	bcond, bargs := BranchScope(c, h.DB, "branch_id")
	members, err := h.DB.Query(`SELECT id, name,
		COALESCE(birth_gregorian,''), COALESCE(birth_lunar,''),
		COALESCE(death_gregorian,''), COALESCE(death_lunar,'')
		FROM members
		WHERE COALESCE(lost_contact,0) = 0
		  AND (COALESCE(birth_gregorian,'') != '' OR COALESCE(birth_lunar,'') != ''
		       OR COALESCE(death_gregorian,'') != '' OR COALESCE(death_lunar,'') != '')`+AndCond(bcond), bargs...)
	if err == nil {
		for members.Next() {
			var id int64
			var name, bg, bl, dg, dl string
			if members.Scan(&id, &name, &bg, &bl, &dg, &dl) != nil {
				continue
			}
			// 在世 → 生日；已故 → 忌日
			if dg == "" && dl == "" {
				if t, years, lunar, ok := birthdayOf(bg, bl, today); ok {
					add(t, "birthday", name, ageText(years), lunar, id, 0, years, false)
				}
			} else {
				if t, years, lunar, ok := deathdayOf(dg, dl, today); ok {
					add(t, "deathday", name, anniversaryText(years), lunar, id, 0, years, false)
				}
			}
		}
		members.Close()
	}

	// ---------- 3) 配偶生日 / 忌日 ----------
	spWhere := ""
	if bcond != "" {
		spWhere = " AND member_id IN (SELECT id FROM members WHERE " + bcond + ")"
	}
	spouses, err := h.DB.Query(`SELECT id, member_id,
		COALESCE(NULLIF(name,''), surname || given_name) AS name,
		COALESCE(birth_gregorian,''), COALESCE(birth_lunar,''),
		COALESCE(death_gregorian,''), COALESCE(death_lunar,'')
		FROM spouses
		WHERE (COALESCE(birth_gregorian,'') != '' OR COALESCE(birth_lunar,'') != ''
		       OR COALESCE(death_gregorian,'') != '' OR COALESCE(death_lunar,'') != '')`+spWhere, bargs...)
	if err == nil {
		for spouses.Next() {
			var id, memberID int64
			var name, bg, bl, dg, dl string
			if spouses.Scan(&id, &memberID, &name, &bg, &bl, &dg, &dl) != nil {
				continue
			}
			if dg == "" && dl == "" {
				if t, years, lunar, ok := birthdayOf(bg, bl, today); ok {
					add(t, "birthday", name, ageText(years), lunar, 0, id, years, false)
				}
			} else {
				if t, years, lunar, ok := deathdayOf(dg, dl, today); ok {
					add(t, "deathday", name, anniversaryText(years), lunar, 0, id, years, false)
				}
			}
		}
		spouses.Close()
	}

	// 排序：日期 → 节日/生日/忌日 → 名称
	kindRank := map[string]int{"festival": 0, "birthday": 1, "deathday": 2}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Date != items[j].Date {
			return items[i].Date < items[j].Date
		}
		if kindRank[items[i].Kind] != kindRank[items[j].Kind] {
			return kindRank[items[i].Kind] < kindRank[items[j].Kind]
		}
		return items[i].Title < items[j].Title
	})

	c.JSON(http.StatusOK, gin.H{
		"start":  today.Format("2006-01-02"),
		"end":    windowEnd.Format("2006-01-02"),
		"days":   UpcomingWindowDays,
		"items":  items,
		"count":  len(items),
		"today":  today.Format("2006-01-02"),
		"window": "未来 30 天",
	})
}

// birthdayOf 计算本年（或明年）生日：优先农历，农历不可用时退回公历月日
func birthdayOf(birthGregorian, birthLunar string, today time.Time) (time.Time, int, string, bool) {
	if birthLunar != "" {
		if m, d, leap, ok := parseLunarMD(birthLunar); ok {
			if t, ok2 := lunarOccurrence(m, leap, d, today); ok2 {
				return t, 0, birthLunar, true
			}
		}
	}
	if m, d, by, ok := solarMD(birthGregorian); ok {
		if t, ok2 := solarOccurrence(m, d, today); ok2 {
			return t, t.Year() - by, "", true
		}
	}
	return time.Time{}, 0, "", false
}

// deathdayOf 计算本年（或明年）忌日：优先农历，农历不可用时退回公历月日
func deathdayOf(deathGregorian, deathLunar string, today time.Time) (time.Time, int, string, bool) {
	dy := 0
	if _, _, y, ok := solarMD(deathGregorian); ok {
		dy = y
	}
	if deathLunar != "" {
		if m, d, leap, ok := parseLunarMD(deathLunar); ok {
			if t, ok2 := lunarOccurrence(m, leap, d, today); ok2 {
				years := 0
				if dy > 0 && t.Year() >= dy {
					years = t.Year() - dy
				}
				return t, years, deathLunar, true
			}
		}
	}
	if m, d, _, ok := solarMD(deathGregorian); ok {
		if t, ok2 := solarOccurrence(m, d, today); ok2 {
			years := 0
			if dy > 0 && t.Year() >= dy {
				years = t.Year() - dy
			}
			return t, years, "", true
		}
	}
	return time.Time{}, 0, "", false
}

func ageText(years int) string {
	if years > 0 {
		return fmt.Sprintf("%d 岁", years)
	}
	return ""
}

func anniversaryText(years int) string {
	if years > 0 {
		return fmt.Sprintf("逝世 %d 周年", years)
	}
	return "忌日"
}
