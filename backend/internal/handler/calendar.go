package handler

import (
	"database/sql"
	"net/http"
	"sort"
	"strings"
	"time"

	"familytree/internal/lunar"

	"github.com/gin-gonic/gin"
)

// CalendarMember 日历视图成员（生日/祭日）
type CalendarMember struct {
	ID             int64  `json:"id"`
	Kind           string `json:"kind,omitempty"`      // "member" 或 "spouse"
	MemberID       int64  `json:"member_id,omitempty"` // spouse 所属成员 id
	Name           string `json:"name"`
	Generation     int    `json:"generation"`
	Gender         int    `json:"gender"`
	Portrait       string `json:"portrait"`
	BirthGregorian string `json:"birth_gregorian"`
	BirthLunar     string `json:"birth_lunar"`
	LunarMonth     int    `json:"lunar_month"`
	LunarDay       int    `json:"lunar_day"`
	IsLeapMonth    bool   `json:"is_leap_month"`
	ThisYearSolar  string `json:"this_year_solar"`
	Age            int    `json:"age"`
	// 祭日专用
	DeathGregorian string `json:"death_gregorian,omitempty"`
	DeathLunar     string `json:"death_lunar,omitempty"`
	Lifespan       int    `json:"lifespan,omitempty"`
}

// CalendarHandler 日历处理器
type CalendarHandler struct {
	DB *sql.DB
}

var lunarMonthMap = map[string]int{
	"正": 1, "一": 1,
	"二": 2, "三": 3, "四": 4, "五": 5, "六": 6,
	"七": 7, "八": 8, "九": 9, "十": 10,
	"冬": 11, "十一": 11,
	"腊": 12, "十二": 12,
}

var lunarDayMap = map[string]int{
	"初一": 1, "初二": 2, "初三": 3, "初四": 4, "初五": 5, "初六": 6, "初七": 7, "初八": 8, "初九": 9, "初十": 10,
	"十一": 11, "十二": 12, "十三": 13, "十四": 14, "十五": 15, "十六": 16, "十七": 17, "十八": 18, "十九": 19,
	"二十": 20, "廿一": 21, "廿二": 22, "廿三": 23, "廿四": 24, "廿五": 25, "廿六": 26, "廿七": 27, "廿八": 28, "廿九": 29,
	"三十": 30,
}

// parseLunarMD 从农历字符串中解析月日。格式如 "二〇二〇年六月十五"、"一八〇三年闰二月廿二"
func parseLunarMD(lunarStr string) (month, day int, isLeap bool, ok bool) {
	yi := strings.Index(lunarStr, "年")
	mi := strings.Index(lunarStr, "月")
	if yi == -1 || mi == -1 || yi >= mi {
		return 0, 0, false, false
	}
	monthStr := lunarStr[yi+len("年") : mi]
	dayStr := lunarStr[mi+len("月"):]
	if strings.HasPrefix(monthStr, "闰") {
		isLeap = true
		monthStr = monthStr[len("闰"):]
	}
	m, ok := lunarMonthMap[monthStr]
	if !ok {
		return 0, 0, false, false
	}
	d, ok := lunarDayMap[dayStr]
	if !ok {
		return 0, 0, false, false
	}
	return m, d, isLeap, true
}

// safeLunarToSolar 安全转换，农历日不存在时返回空；isLeap 表示闰月
func safeLunarToSolar(y, m int, isLeap bool, d int) string {
	defer func() { recover() }()
	leap := 0
	if isLeap {
		leap = 1
	}
	info := lunar.LunarToGregorian(y, m, leap, d)
	return info.Gregorian
}

// Birthdays 族人生日（按农历月日排序，正月初一开始）
// GET /api/members/birthdays
func (h *CalendarHandler) Birthdays(c *gin.Context) {
	var list []CalendarMember
	now := time.Now()
	bcond, bargs := BranchScope(c, h.DB, "branch_id")

	// 1) 族人生日（仅在世）
	rows, err := h.DB.Query(`SELECT id, name, generation, gender, COALESCE(portrait,''),
		COALESCE(birth_gregorian,''), COALESCE(birth_lunar,'')
		FROM members WHERE birth_lunar != '' AND COALESCE(death_gregorian,'') = '' AND COALESCE(lost_contact,0) = 0`+AndCond(bcond), bargs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for rows.Next() {
		var m CalendarMember
		if err := rows.Scan(&m.ID, &m.Name, &m.Generation, &m.Gender, &m.Portrait,
			&m.BirthGregorian, &m.BirthLunar); err != nil {
			continue
		}
		m.Kind = "member"
		appendBirthday(&m, &list, now)
	}
	rows.Close()

	// 2) 配偶生日（仅在世）
	spW := ""
	if bcond != "" {
		spW = " AND member_id IN (SELECT id FROM members WHERE " + bcond + ")"
	}
	srows, err := h.DB.Query(`SELECT id, member_id,
		COALESCE(NULLIF(name,''), surname || given_name) AS name,
		COALESCE(birth_gregorian,''), COALESCE(birth_lunar,'')
		FROM spouses WHERE birth_lunar != '' AND COALESCE(death_gregorian,'') = ''`+spW, bargs...)
	if err == nil {
		for srows.Next() {
			var m CalendarMember
			if err := srows.Scan(&m.ID, &m.MemberID, &m.Name, &m.BirthGregorian, &m.BirthLunar); err != nil {
				continue
			}
			m.Kind = "spouse"
			appendBirthday(&m, &list, now)
		}
		srows.Close()
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].LunarMonth != list[j].LunarMonth {
			return list[i].LunarMonth < list[j].LunarMonth
		}
		return list[i].LunarDay < list[j].LunarDay
	})

	c.JSON(http.StatusOK, list)
}

// appendBirthday 将一条生日记录解析并加入列表
func appendBirthday(m *CalendarMember, list *[]CalendarMember, now time.Time) {
	month, day, isLeap, ok := parseLunarMD(m.BirthLunar)
	if !ok {
		return
	}
	m.LunarMonth = month
	m.LunarDay = day
	m.IsLeapMonth = isLeap
	m.ThisYearSolar = safeLunarToSolar(now.Year(), month, isLeap, day)
	if m.BirthGregorian != "" {
		if bt, err := time.Parse("2006-01-02", m.BirthGregorian); err == nil {
			m.Age = now.Year() - bt.Year()
		}
	}
	*list = append(*list, *m)
}

// Deathdays 先人祭日（按农历月日排序）
// GET /api/members/deathdays
func (h *CalendarHandler) Deathdays(c *gin.Context) {
	var list []CalendarMember
	now := time.Now()
	bcond, bargs := BranchScope(c, h.DB, "branch_id")

	// 1) 族人祭日
	rows, err := h.DB.Query(`SELECT id, name, generation, gender, COALESCE(portrait,''),
		COALESCE(birth_gregorian,''), COALESCE(birth_lunar,''),
		COALESCE(death_gregorian,''), COALESCE(death_lunar,''), lifespan
		FROM members WHERE death_lunar != '' AND COALESCE(lost_contact,0) = 0`+AndCond(bcond), bargs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for rows.Next() {
		var m CalendarMember
		if err := rows.Scan(&m.ID, &m.Name, &m.Generation, &m.Gender, &m.Portrait,
			&m.BirthGregorian, &m.BirthLunar, &m.DeathGregorian, &m.DeathLunar, &m.Lifespan); err != nil {
			continue
		}
		m.Kind = "member"
		appendDeathday(&m, &list, now)
	}
	rows.Close()

	// 2) 配偶祭日
	spW := ""
	if bcond != "" {
		spW = " AND member_id IN (SELECT id FROM members WHERE " + bcond + ")"
	}
	srows, err := h.DB.Query(`SELECT id, member_id,
		COALESCE(NULLIF(name,''), surname || given_name) AS name,
		COALESCE(death_gregorian,''), COALESCE(death_lunar,'')
		FROM spouses WHERE death_lunar != ''`+spW, bargs...)
	if err == nil {
		for srows.Next() {
			var m CalendarMember
			if err := srows.Scan(&m.ID, &m.MemberID, &m.Name, &m.DeathGregorian, &m.DeathLunar); err != nil {
				continue
			}
			m.Kind = "spouse"
			appendDeathday(&m, &list, now)
		}
		srows.Close()
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].LunarMonth != list[j].LunarMonth {
			return list[i].LunarMonth < list[j].LunarMonth
		}
		return list[i].LunarDay < list[j].LunarDay
	})

	c.JSON(http.StatusOK, list)
}

// appendDeathday 将一条祭日记录解析并加入列表
func appendDeathday(m *CalendarMember, list *[]CalendarMember, now time.Time) {
	month, day, isLeap, ok := parseLunarMD(m.DeathLunar)
	if !ok {
		return
	}
	m.LunarMonth = month
	m.LunarDay = day
	m.IsLeapMonth = isLeap
	m.ThisYearSolar = safeLunarToSolar(now.Year(), month, isLeap, day)
	*list = append(*list, *m)
}