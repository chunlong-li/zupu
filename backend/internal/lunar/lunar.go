package lunar

import (
	"fmt"
	"strconv"

	lunar "github.com/6tail/lunar-go/calendar"
)

// DateInfo 日期信息（公历 + 农历 + 干支 + 年号 + 生肖，全中文）
type DateInfo struct {
	Gregorian string `json:"gregorian"` // 公历，如 1900-08-01
	Lunar     string `json:"lunar"`     // 农历中文，如 二〇二〇年六月十五
	Ganzhi    string `json:"ganzhi"`    // 干支，如 庚子年癸未月
	YearCn    string `json:"yearCn"`    // 年号中文，如 二〇二〇年
	Animal    string `json:"animal"`    // 生肖，如 鼠
	Era       string `json:"era"`       // 年号，如 "光绪"、"民国"
	EraYear   string `json:"eraYear"`   // 年号年份，如 "二十六年"
}

// 年号映射表（起始年份 -> 年号名称）
// 顺序很重要，从最近的开始匹配
var eraList = []struct {
	StartYear int
	Name      string
}{
	{1949, "共和"},
	{1912, "民国"},
	{1909, "宣统"},
	{1875, "光绪"},
	{1862, "同治"},
	{1851, "咸丰"},
	{1821, "道光"},
	{1796, "嘉庆"},
	{1736, "乾隆"},
	{1723, "雍正"},
	{1662, "康熙"},
	{1644, "顺治"},
	{1628, "崇祯"},
	{1621, "天启"},
	{1573, "万历"},
	{1567, "隆庆"},
	{1522, "嘉靖"},
	{1506, "正德"},
	{1488, "弘治"},
	{1465, "成化"},
	{1457, "天顺"},
	{1450, "景泰"},
	{1436, "正统"},
	{1426, "宣德"},
	{1403, "永乐"},
	{1399, "建文"},
	{1368, "洪武"},
}

// 数字转中文
var cnNums = []string{"零", "一", "二", "三", "四", "五", "六", "七", "八", "九"}

func numberToChinese(n int) string {
	if n == 0 {
		return "零"
	}
	if n < 10 {
		return cnNums[n]
	}
	if n == 10 {
		return "十"
	}
	if n < 20 {
		return "十" + cnNums[n%10]
	}
	if n < 100 {
		s := cnNums[n/10] + "十"
		if n%10 > 0 {
			s += cnNums[n%10]
		}
		return s
	}
	// 100+
	return strconv.Itoa(n)
}

// getEra 根据公历年份获取年号信息
func getEra(year int) (era string, eraYear string) {
	for _, e := range eraList {
		if year >= e.StartYear {
			if year == e.StartYear {
				return e.Name, "元年"
			}
			eraYearNum := year - e.StartYear + 1
			return e.Name, numberToChinese(eraYearNum)
		}
	}
	return "", ""
}

// FromGregorian 根据公历日期换算全部中文日期
func FromGregorian(y, m, d int) DateInfo {
	// NewSolar 新版 API: year, month, day, hour, minute, second
	solar := lunar.NewSolar(y, m, d, 0, 0, 0)
	l := solar.GetLunar()

	era, eraYear := getEra(y)

	return DateInfo{
		Gregorian: fmt.Sprintf("%04d-%02d-%02d", y, m, d),
		Lunar:     fmt.Sprintf("%s年%s月%s", l.GetYearInChinese(), l.GetMonthInChinese(), l.GetDayInChinese()),
		Ganzhi:    fmt.Sprintf("%s年%s月", l.GetYearInGanZhi(), l.GetMonthInGanZhi()),
		YearCn:    l.GetYearInChinese(),
		Animal:    l.GetYearShengXiao(),
		Era:       era,
		EraYear:   eraYear,
	}
}

// FromGregorianYear 根据公历年份换算年号（无月日时只算年号，农历/干支留空）
func FromGregorianYear(y int) DateInfo {
	era, eraYear := getEra(y)
	return DateInfo{
		Gregorian: fmt.Sprintf("%04d", y),
		Era:       era,
		EraYear:   eraYear,
	}
}

// LunarToGregorian 农历转公历
// 农历月份用正数，闰月用负数（如闰八月传 -8）
func LunarToGregorian(y, m, leap, d int) DateInfo {
	lunarMonth := m
	if leap == 1 {
		lunarMonth = -m // 负数表示闰月
	}
	l := lunar.NewLunar(y, lunarMonth, d, 0, 0, 0)
	solar := l.GetSolar()

	era, eraYear := getEra(solar.GetYear())

	return DateInfo{
		Gregorian: fmt.Sprintf("%04d-%02d-%02d", solar.GetYear(), solar.GetMonth(), solar.GetDay()),
		Lunar:     fmt.Sprintf("%s年%s月%s", l.GetYearInChinese(), l.GetMonthInChinese(), l.GetDayInChinese()),
		Ganzhi:    fmt.Sprintf("%s年%s月", l.GetYearInGanZhi(), l.GetMonthInGanZhi()),
		YearCn:    l.GetYearInChinese(),
		Animal:    l.GetYearShengXiao(),
		Era:       era,
		EraYear:   eraYear,
	}
}