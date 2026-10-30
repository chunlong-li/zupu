package handler

import (
	"database/sql"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// 名人传记默认排序：世代(generation) → 排行(rank) → 生年(birth_gregorian) → id。
//
// 规则（与「谱书」各视图共用同一份接口顺序）：
//  1. 能关联到族内成员的传记（member_id）按该成员的 generation 升序（辈分高者在前）；
//  2. 配偶的传记（person_type=spouse，spouse_id → spouses.member_id）跟随其丈夫的
//     generation / rank（传统族谱「配附于夫」），同世代同排行内再按自身 id 兜底；
//  3. 同世代按 rank 升序；没有 rank（或 rank 解析不出序数）的排在有序数者之后，
//     改用生年、再没有则用 id；
//  4. sort_year 不再作为主排序，**只在「同世代 + 同排行」内**作为次级键（未填 0 视为
//     未设置，让位给生年），所以填了很大 sort_year 的晚辈子孙不会跑到先祖前面；
//  5. 关联不到任何成员的传记（member_id / spouse_id 均为 0，或关联记录已不存在）排在
//     最后，内部按 sort_year、id 排序。
//
// 实现说明：rank 是自由文本（「行一」「行12」「长子」「次子」…），SQLite 无法直接按
// 序数排序，故在 Go 侧解析后排序；接口仍先由 SQL 取出（ORDER BY id）再调用本函数。

type bioSortKey struct {
	linked     bool  // 是否关联到族内成员（含配偶→丈夫）
	isSpouse   bool  // 是否为「配偶」传记（跟随丈夫世代/排行）
	generation int   // 世代（1 起；未关联时无意义）
	hasRank    bool  // rank 是否解析出序数
	rank       int   // 排行序数
	sortYear   int   // 传记表 sort_year（人工填；0=未填）
	birth      int64 // 生年键 yyyymmdd（0=未知），来自成员档案，其次传记自身
}

// 中文数字（序数解析用）
var cnDigitValue = map[rune]int{
	'零': 0, '〇': 0, '一': 1, '二': 2, '两': 2, '三': 3, '四': 4,
	'五': 5, '六': 6, '七': 7, '八': 8, '九': 9,
}

var (
	reRankDigits = regexp.MustCompile(`\d+`)
	reBirthDate  = regexp.MustCompile(`(\d{1,4})(?:[-/.年](\d{1,2}))?(?:[-/.月](\d{1,2}))?`)
)

// rankNumber 把「行一 / 行12 / 长子 / 次子 / 十五」等排行文本解析为序数；解析不出返回 0。
func rankNumber(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// ① 阿拉伯数字优先：「行12」→ 12
	if m := reRankDigits.FindString(s); m != "" {
		if n, err := strconv.Atoi(m); err == nil && n > 0 {
			return n
		}
	}
	// ② 传统称谓
	if strings.ContainsAny(s, "长長") {
		return 1
	}
	if strings.Contains(s, "次") {
		return 2
	}
	// ③ 中文数字：「行一」→1、「十五」→15、「廿三」→23
	if n := cnNumeralValue(s); n > 0 {
		return n
	}
	return 0
}

// cnNumeralValue 解析字符串中连续出现的中文数字（忽略「行/第/排行」等前后缀）。
func cnNumeralValue(s string) int {
	total, num := 0, 0
	seen := false
	for _, r := range s {
		switch {
		case r == '零' || r == '〇':
			if num != 0 {
				total += num
				num = 0
			}
		case cnDigitValue[r] > 0:
			num = cnDigitValue[r]
			seen = true
		case r == '十':
			if num == 0 {
				num = 1
			}
			total += num * 10
			num = 0
			seen = true
		case r == '廿':
			total += 20
			num = 0
			seen = true
		case r == '卅':
			total += 30
			num = 0
			seen = true
		case r == '百':
			if num == 0 {
				num = 1
			}
			total += num * 100
			num = 0
			seen = true
		}
	}
	if !seen {
		return 0
	}
	return total + num
}

// birthSortKey 生年键：取字符串里第一组「年[-月[-日]]」，如 1700 / 1732-12-11。
// 生年缺失返回 0（排序时排在有生年者之后）。
func birthSortKey(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	m := reBirthDate.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	y, _ := strconv.Atoi(m[1])
	if y <= 0 {
		return 0
	}
	mo, d := 0, 0
	if m[2] != "" {
		mo, _ = strconv.Atoi(m[2])
	}
	if m[3] != "" {
		d, _ = strconv.Atoi(m[3])
	}
	return int64(y)*10000 + int64(mo)*100 + int64(d)
}

// tieYear 同世代同排行内的「次级年份」：sort_year 有值用 sort_year，否则用生年。
// 0 表示两者皆无（排序时列在有年份者之后）。
func (k bioSortKey) tieYear() int64 {
	if k.sortYear > 0 {
		return int64(k.sortYear) * 10000
	}
	return k.birth
}

// less 传记排序比较：世代 → 排行 → 配偶附夫 → sort_year（同世同排内）→ 生年 → id。
func (k bioSortKey) less(o bioSortKey) bool {
	// 关联不到成员的排最后，内部按 sort_year、id
	if k.linked != o.linked {
		return k.linked // 关联到成员的在前，关联不到者排最后
	}
	if !k.linked {
		// 未关联成员：sort_year、id（id 由调用方兜底）
		return k.sortYear < o.sortYear
	}
	// ① 世代：数字小的（辈分高）在前
	if k.generation != o.generation {
		return k.generation < o.generation
	}
	// ② 排行：有排行的在前，其次按序数
	if k.hasRank != o.hasRank {
		return k.hasRank
	}
	if k.hasRank && k.rank != o.rank {
		return k.rank < o.rank
	}
	// ③ 配偶附于夫：同世代同排行内，本人排在配偶之前
	if k.isSpouse != o.isSpouse {
		return !k.isSpouse
	}
	// 配偶同样跟随丈夫的世代/排行后，按自身 id 兜底（不看自身生年）
	if k.isSpouse {
		return false
	}
	// ④ 同世代同排行内：sort_year（未填则生年）→ 生年 → id
	ya, yb := k.tieYear(), o.tieYear()
	if ya != yb {
		if ya == 0 {
			return false
		}
		if yb == 0 {
			return true
		}
		return ya < yb
	}
	if k.birth != o.birth {
		if k.birth == 0 {
			return false
		}
		if o.birth == 0 {
			return true
		}
		return k.birth < o.birth
	}
	return false // id 由调用方兜底
}

// loadBiographySortKeys 一次性取出所有传记的排序键（成员 / 配偶→丈夫）。
// 成员关联优先于配偶关联：若传记同时填了 member_id 与 spouse_id，以 member_id 本人为准。
func loadBiographySortKeys(db *sql.DB) (map[int64]bioSortKey, bool) {
	keys := map[int64]bioSortKey{}
	if db == nil {
		return keys, false
	}
	// ① 关联成员的传记
	rows, err := db.Query(`SELECT b.id, m.generation, COALESCE(m.rank,''), COALESCE(m.birth_gregorian,'')
		FROM biographies b JOIN members m ON m.id = b.member_id
		WHERE COALESCE(b.member_id,0) > 0`)
	if err != nil {
		return keys, false
	}
	for rows.Next() {
		var id int64
		var gen int
		var rank, birth string
		if err := rows.Scan(&id, &gen, &rank, &birth); err != nil {
			rows.Close()
			return keys, false
		}
		r := rankNumber(rank)
		keys[id] = bioSortKey{
			linked: true, generation: gen,
			hasRank: r > 0, rank: r,
			birth: birthSortKey(birth),
		}
	}
	rows.Close()

	// ② 配偶的传记：跟随丈夫（spouses.member_id）的世代 / 排行
	rows, err = db.Query(`SELECT b.id, m.generation, COALESCE(m.rank,''), COALESCE(m.birth_gregorian,'')
		FROM biographies b JOIN spouses s ON s.id = b.spouse_id JOIN members m ON m.id = s.member_id
		WHERE COALESCE(b.spouse_id,0) > 0`)
	if err != nil {
		return keys, false
	}
	for rows.Next() {
		var id int64
		var gen int
		var rank, birth string
		if err := rows.Scan(&id, &gen, &rank, &birth); err != nil {
			rows.Close()
			return keys, false
		}
		_ = birth                          // 配偶不看生年，只需丈夫的世代 / 排行
		if _, exists := keys[id]; exists { // 成员关联优先
			continue
		}
		r := rankNumber(rank)
		keys[id] = bioSortKey{
			linked: true, isSpouse: true, generation: gen,
			hasRank: r > 0, rank: r,
		}
	}
	rows.Close()
	return keys, true
}

// sortBiographies 按「世代 → 排行 → 生年 → id」就地重排传记列表（默认自动规则）。
// 排序键读取失败时保持原有顺序（不因排序失败而丢内容 / 报错）。
func sortBiographies(db *sql.DB, list []Biography) {
	if len(list) < 2 {
		return
	}
	keys, ok := loadBiographySortKeys(db)
	if !ok {
		return
	}
	sortBiographiesWithKeys(list, keys)
}

// sortBiographiesWithKeys 用已取出的排序键重排（独立成函数便于单测）。
func sortBiographiesWithKeys(list []Biography, keys map[int64]bioSortKey) {
	// 每条传记的完整键：关联信息（查库）+ 自身 sort_year / birth_gregorian
	type item struct {
		b Biography
		k bioSortKey
	}
	items := make([]item, len(list))
	for i, b := range list {
		k, found := keys[b.ID]
		if !found {
			k = bioSortKey{} // 关联不到成员 → 排最后
		}
		k.sortYear = b.SortYear
		if k.birth == 0 { // 成员档案无生年时退回传记本身的生年
			k.birth = birthSortKey(b.BirthGregorian)
		}
		items[i] = item{b: b, k: k}
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.k.less(b.k) {
			return true
		}
		if b.k.less(a.k) {
			return false
		}
		return a.b.ID < b.b.ID
	})
	for i := range items {
		list[i] = items[i].b
	}
}
