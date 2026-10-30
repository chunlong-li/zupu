package handler

import (
	"time"
	"strings"
	"sort"
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Search 全文检索成员
// GET /api/members/search?q=xxx
func (h *MemberHandler) Search(c *gin.Context) {
	q := "%" + c.Query("q") + "%"
	rows, err := h.DB.Query(`SELECT id, generation, branch, branch_id, surname, given_name, name, zi, hao, generation_word,
		rank, gender, father_id, mother_id, mother_spouse_id, clan_type, record_type, foster_father_id,
		birth_calendar, birth_gregorian, birth_lunar, birth_ganzhi, birth_era, death_gregorian, death_lunar, death_ganzhi, death_era,
		lifespan, native_place, current_place, title, occupation, education, biography, burial_place, portrait, married_out, former_name, phone
		FROM members WHERE
		name LIKE ? OR surname LIKE ? OR given_name LIKE ? OR zi LIKE ? OR hao LIKE ? OR generation_word LIKE ? OR
		native_place LIKE ? OR current_place LIKE ? OR title LIKE ? OR occupation LIKE ? OR education LIKE ? OR biography LIKE ? OR
		birth_gregorian LIKE ? OR death_gregorian LIKE ?
		ORDER BY generation, id
		LIMIT 50`, q, q, q, q, q, q, q, q, q, q, q, q, q, q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	members := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Generation, &m.Branch, &m.BranchID, &m.Surname, &m.GivenName, &m.Name, &m.Zi, &m.Hao,
			&m.GenerationWord, &m.Rank, &m.Gender, &m.FatherID, &m.MotherID, &m.MotherSpouseID,
			&m.ClanType, &m.RecordType, &m.FosterFatherID, &m.BirthCalendar, &m.BirthGregorian, &m.BirthLunar, &m.BirthGanzhi, &m.BirthEra,
			&m.DeathGregorian, &m.DeathLunar, &m.DeathGanzhi, &m.DeathEra, &m.Lifespan,
			&m.NativePlace, &m.CurrentPlace, &m.Title, &m.Occupation, &m.Education, &m.Biography, &m.BurialPlace, &m.Portrait, &m.MarriedOut, &m.FormerName, &m.Phone); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		members = append(members, m)
	}
	viewerID := FindMemberByName(h.DB, c.GetString("username"))
	SanitizeMembers(members, c.GetString("role"), viewerID, c.GetString("branch"), h.DB)
	c.JSON(http.StatusOK, members)
}

// Children 获取某成员的子女列表（按长幼排序，含嫡庶标记）
// GET /api/members/:id/children
func (h *MemberHandler) Children(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	rows, err := h.DB.Query(`SELECT id, generation, branch, branch_id, surname, given_name, name, zi, hao, generation_word,
		rank, gender, father_id, mother_id, mother_spouse_id, clan_type, record_type, foster_father_id,
		birth_calendar, birth_gregorian, birth_lunar, birth_ganzhi, birth_era, death_gregorian, death_lunar, death_ganzhi, death_era,
		lifespan, native_place, current_place, title, occupation, education, biography, burial_place, portrait, married_out, former_name, phone
		FROM members WHERE father_id = ? ORDER BY gender DESC, id`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	children := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Generation, &m.Branch, &m.BranchID, &m.Surname, &m.GivenName, &m.Name, &m.Zi, &m.Hao,
			&m.GenerationWord, &m.Rank, &m.Gender, &m.FatherID, &m.MotherID, &m.MotherSpouseID,
			&m.ClanType, &m.RecordType, &m.FosterFatherID, &m.BirthCalendar, &m.BirthGregorian, &m.BirthLunar, &m.BirthGanzhi, &m.BirthEra,
			&m.DeathGregorian, &m.DeathLunar, &m.DeathGanzhi, &m.DeathEra, &m.Lifespan,
			&m.NativePlace, &m.CurrentPlace, &m.Title, &m.Occupation, &m.Education, &m.Biography, &m.BurialPlace, &m.Portrait, &m.MarriedOut, &m.FormerName, &m.Phone); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		children = append(children, m)
	}
	viewerID := FindMemberByName(h.DB, c.GetString("username"))
	SanitizeMembers(children, c.GetString("role"), viewerID, c.GetString("branch"), h.DB)
	c.JSON(http.StatusOK, children)
}

// MemberWithSpouses 成员详情（含配偶列表）
type MemberWithSpouses struct {
	Member
	Spouses []Spouse `json:"spouses"`
}

// GetWithSpouses 获取成员详情（含配偶列表）
// GET /api/members/:id/detail
func (h *MemberHandler) GetWithSpouses(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	var m Member
	err := h.DB.QueryRow(`SELECT id, generation, branch, branch_id, surname, given_name, name, zi, hao, generation_word,
		rank, gender, father_id, mother_id, mother_spouse_id, clan_type, record_type, foster_father_id,
		birth_calendar, birth_gregorian, birth_lunar, birth_ganzhi, birth_era, death_gregorian, death_lunar, death_ganzhi, death_era,
		lifespan, native_place, current_place, title, occupation, education, biography, burial_place, portrait, married_out, former_name, phone, dna_haploid, dna_diploid, dna_mito
		FROM members WHERE id = ?`, id).Scan(&m.ID, &m.Generation, &m.Branch, &m.BranchID, &m.Surname, &m.GivenName, &m.Name, &m.Zi, &m.Hao,
		&m.GenerationWord, &m.Rank, &m.Gender, &m.FatherID, &m.MotherID, &m.MotherSpouseID,
		&m.ClanType, &m.RecordType, &m.FosterFatherID, &m.BirthCalendar, &m.BirthGregorian, &m.BirthLunar, &m.BirthGanzhi, &m.BirthEra,
		&m.DeathGregorian, &m.DeathLunar, &m.DeathGanzhi, &m.DeathEra, &m.Lifespan,
		&m.NativePlace, &m.CurrentPlace, &m.Title, &m.Occupation, &m.Education, &m.Biography, &m.BurialPlace, &m.Portrait, &m.MarriedOut, &m.FormerName, &m.Phone, &m.DNAHaploid, &m.DNADiploid, &m.DNAMito)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "成员不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 无嗣 / 迁居失联 标记
	h.DB.QueryRow(`SELECT COALESCE(no_heir,0), COALESCE(lost_contact,0) FROM members WHERE id=?`, id).
		Scan(&m.NoHeir, &m.LostContact)

	// 解密敏感字段
	m.DecryptSensitive(h.EncKey)

	// 隐私脱敏（超管/理事长/理事全可见，房支理事按房支，族员按五服）
	role := c.GetString("role")
	viewerID := FindMemberByName(h.DB, c.GetString("username"))
	SanitizeMember(&m, nil, role, viewerID, c.GetString("branch"), h.DB)

	// 访问审计：仅记录会触发脱敏的角色
	if viewerID > 0 && viewerSensitivity(role) < SensAdmin {
		distance := RelationDistance(h.DB, viewerID, id)
		LogPrivacyAccess(h.DB, c.GetString("username"), viewerID, id, distance)
	}

	// 查询配偶
	spouseRows, _ := h.DB.Query(`SELECT id, member_id, surname, given_name, name, type, family,
		birth_calendar, birth_gregorian, birth_lunar, birth_ganzhi, birth_era,
		death_gregorian, death_lunar, death_ganzhi, death_era, COALESCE(occupation,''), COALESCE(education,''), COALESCE(biography,''), COALESCE(remark,''), COALESCE(portrait,''), COALESCE(phone,''), COALESCE(dna_haploid,''), COALESCE(dna_diploid,''), COALESCE(dna_mito,'')
		FROM spouses WHERE member_id = ? ORDER BY
		CASE type WHEN '正配' THEN 1 WHEN '继配' THEN 2 WHEN '侧室' THEN 3 ELSE 4 END, id`, id)
	defer spouseRows.Close()

	spouses := []Spouse{}
	for spouseRows.Next() {
		var s Spouse
		spouseRows.Scan(&s.ID, &s.MemberID, &s.Surname, &s.GivenName, &s.Name, &s.Type, &s.Family,
			&s.BirthCalendar, &s.BirthGregorian, &s.BirthLunar, &s.BirthGanzhi, &s.BirthEra,
			&s.DeathGregorian, &s.DeathLunar, &s.DeathGanzhi, &s.DeathEra,
			&s.Occupation, &s.Education, &s.Biography, &s.Remark, &s.Portrait, &s.Phone, &s.DNAHaploid, &s.DNADiploid, &s.DNAMito)
		spouses = append(spouses, s)
	}

	// 解密 + 按角色脱敏配偶敏感字段
	decryptSpousesInPlace(spouses, h.EncKey)
	for i := range spouses {
		sanitizeSpouseByRole(&spouses[i], role)
	}

	c.JSON(http.StatusOK, MemberWithSpouses{Member: m, Spouses: spouses})
}

// Stats 数据统计
// GET /api/members/stats
func (h *MemberHandler) Stats(c *gin.Context) {
	bcond, bargs := BranchScope(c, h.DB, "branch_id")
	w := ""
	if bcond != "" {
		w = " WHERE " + bcond
	}
	and := func(extra string) string {
		if bcond == "" {
			return " WHERE " + extra
		}
		return " WHERE " + extra + " AND " + bcond
	}

	var total, alive, male, female, spouseCount, spouseAlive int
	h.DB.QueryRow(`SELECT COUNT(*) FROM members`+w, bargs...).Scan(&total)
	h.DB.QueryRow(`SELECT COUNT(*) FROM members`+and("(death_gregorian = '' OR death_gregorian IS NULL)"), bargs...).Scan(&alive)
	h.DB.QueryRow(`SELECT COUNT(*) FROM members`+and("gender = 1"), bargs...).Scan(&male)
	h.DB.QueryRow(`SELECT COUNT(*) FROM members`+and("gender = 2"), bargs...).Scan(&female)

	// 配偶：随其所属成员一并过滤
	spW, spAliveW := "", ""
	if bcond != "" {
		sub := "member_id IN (SELECT id FROM members WHERE " + bcond + ")"
		spW = " WHERE " + sub
		spAliveW = " WHERE " + sub + " AND (death_gregorian = '' OR death_gregorian IS NULL)"
	} else {
		spAliveW = " WHERE (death_gregorian = '' OR death_gregorian IS NULL)"
	}
	h.DB.QueryRow(`SELECT COUNT(*) FROM spouses`+spW, bargs...).Scan(&spouseCount)
	h.DB.QueryRow(`SELECT COUNT(*) FROM spouses`+spAliveW, bargs...).Scan(&spouseAlive)

	// 各世代人数
	genRows, _ := h.DB.Query(`SELECT generation, COUNT(*) as cnt FROM members`+w+` GROUP BY generation ORDER BY generation`, bargs...)
	defer genRows.Close()
	genStats := []map[string]interface{}{}
	for genRows.Next() {
		var gen, cnt int
		genRows.Scan(&gen, &cnt)
		genStats = append(genStats, map[string]interface{}{"generation": gen, "count": cnt})
	}

	// 平均寿命
	var avgLifespan float64
	h.DB.QueryRow(`SELECT AVG(lifespan) FROM members`+and("lifespan > 0"), bargs...).Scan(&avgLifespan)

	// ===== 职业占比（成员 + 配偶合并；未填职业归「未知」）=====
	// 默认只统计在世者；前端可切「全部」（含已逝）。两套数据一次返回，切换无需重新请求。
	type nameCnt struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	normOcc := func(o string) string {
		if o = strings.TrimSpace(o); o == "" {
			return "未知"
		}
		return o
	}
	collectOcc := func(livingOnly bool) ([]nameCnt, int, int) {
		occMap := map[string]int{}
		mq := `SELECT COALESCE(occupation,'') o FROM members`
		if livingOnly {
			mq += and("(death_gregorian = '' OR death_gregorian IS NULL)")
		} else {
			mq += w
		}
		if rows, err := h.DB.Query(mq, bargs...); err == nil {
			for rows.Next() {
				var o string
				if rows.Scan(&o) == nil {
					occMap[normOcc(o)]++
				}
			}
			rows.Close()
		}
		sq := `SELECT COALESCE(occupation,'') o FROM spouses`
		conds := []string{}
		if livingOnly {
			conds = append(conds, "(death_gregorian = '' OR death_gregorian IS NULL)")
		}
		if bcond != "" {
			conds = append(conds, "member_id IN (SELECT id FROM members"+w+")")
		}
		if len(conds) > 0 {
			sq += " WHERE " + strings.Join(conds, " AND ")
		}
		if rows, err := h.DB.Query(sq, bargs...); err == nil {
			for rows.Next() {
				var o string
				if rows.Scan(&o) == nil {
					occMap[normOcc(o)]++
				}
			}
			rows.Close()
		}
		list := []nameCnt{}
		for k, v := range occMap {
			list = append(list, nameCnt{k, v})
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].Count != list[j].Count {
				return list[i].Count > list[j].Count
			}
			return list[i].Name < list[j].Name
		})
		total, unknown := 0, 0
		for _, o := range list {
			total += o.Count
			if o.Name == "未知" {
				unknown = o.Count
			}
		}
		const occTop = 12
		out := []nameCnt{}
		if len(list) > occTop {
			rest := 0
			for _, o := range list[occTop:] {
				rest += o.Count
			}
			out = append(out, list[:occTop]...)
			out = append(out, nameCnt{"其他", rest})
		} else {
			out = list
		}
		return out, total, unknown
	}
	occOut, occTotal, occUnknown := collectOcc(true)          // 在世（默认）
	occAllOut, occAllTotal, occAllUnknown := collectOcc(false) // 全部

	// ===== 年龄结构（在世者，按出生公历年份估算）=====
	type ageBucket struct {
		Label string `json:"label"`
		Count int    `json:"count"`
	}
	bounds := []struct {
		Label string
		Lo    int
		Hi    int
	}{
		{"0-17 岁", 0, 17}, {"18-30 岁", 18, 30}, {"31-45 岁", 31, 45},
		{"46-60 岁", 46, 60}, {"61-75 岁", 61, 75}, {"76 岁以上", 76, 200},
	}
	counts := make([]int, len(bounds))
	ageKnown, ageUnknown, ageSum := 0, 0, 0
	curYear := time.Now().Year()
	addAge := func(birth string) {
		y := birthYearOf(birth)
		if y <= 0 {
			ageUnknown++
			return
		}
		age := curYear - y
		if age < 0 || age > 130 {
			ageUnknown++
			return
		}
		ageKnown++
		ageSum += age
		for i, b := range bounds {
			if age >= b.Lo && age <= b.Hi {
				counts[i]++
				break
			}
		}
	}
	if rows, err := h.DB.Query(`SELECT COALESCE(birth_gregorian,'') b FROM members`+and("(death_gregorian = '' OR death_gregorian IS NULL)"), bargs...); err == nil {
		for rows.Next() {
			var b string
			if rows.Scan(&b) == nil {
				addAge(b)
			}
		}
		rows.Close()
	}
	{
		sq := `SELECT COALESCE(birth_gregorian,'') b FROM spouses WHERE (death_gregorian = '' OR death_gregorian IS NULL)`
		if bcond != "" {
			sq += " AND member_id IN (SELECT id FROM members" + w + ")"
		}
		if rows, err := h.DB.Query(sq, bargs...); err == nil {
			for rows.Next() {
				var b string
				if rows.Scan(&b) == nil {
					addAge(b)
				}
			}
			rows.Close()
		}
	}
	ageGroups := []ageBucket{}
	for i, b := range bounds {
		ageGroups = append(ageGroups, ageBucket{b.Label, counts[i]})
	}
	avgAge := 0.0
	if ageKnown > 0 {
		avgAge = float64(ageSum) / float64(ageKnown)
	}

	// 字辈数
	var uniqueSurnames int
	h.DB.QueryRow(`SELECT COUNT(DISTINCT generation_word) FROM members`+and("generation_word != ''"), bargs...).Scan(&uniqueSurnames)

	// 支系数（当前房支范围内的房支数）
	var uniqueBranches int
	if branchID, _ := strconv.ParseInt(c.Query("branch_id"), 10, 64); branchID > 0 {
		if bcond2, bargs2 := BranchScope(c, h.DB, "id"); bcond2 != "" {
			h.DB.QueryRow(`SELECT COUNT(*) FROM branches WHERE `+bcond2, bargs2...).Scan(&uniqueBranches)
		}
	} else {
		h.DB.QueryRow(`SELECT COUNT(*) FROM branches`).Scan(&uniqueBranches)
	}

	c.JSON(http.StatusOK, gin.H{
		"total":           total + spouseCount,
		"alive":           alive + spouseAlive,
		"deceased":        (total - alive) + (spouseCount - spouseAlive),
		"male":            male,
		"female":          female,
		"spouses":         spouseCount,
		"spouse_alive":    spouseAlive,
		"spouse_deceased": spouseCount - spouseAlive,
		"generations":     genStats,
		"avg_lifespan":    avgLifespan,
		"unique_surnames": uniqueSurnames,
		"unique_branches": uniqueBranches,
		"occupations":     occOut,
		"occupation_total":       occTotal,
		"occupation_unknown":     occUnknown,
		"occupations_all":        occAllOut,
		"occupation_total_all":   occAllTotal,
		"occupation_unknown_all": occAllUnknown,
		"age_groups":      ageGroups,
		"age_known":       ageKnown,
		"age_unknown":     ageUnknown,
		"avg_age":         avgAge,
	})
}

// GetSpousesAsMothers 获取某成员的所有配偶（供子女选择生母用）
// GET /api/members/:id/spouses-for-mother
func (h *MemberHandler) GetSpousesAsMothers(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	rows, err := h.DB.Query(`SELECT id, member_id, surname, given_name, name, type, family,
		birth_calendar, birth_gregorian, birth_lunar, birth_ganzhi, birth_era,
		death_gregorian, death_lunar, death_ganzhi, death_era, COALESCE(occupation,''), COALESCE(education,''), COALESCE(biography,''), COALESCE(remark,''), COALESCE(portrait,''), COALESCE(phone,''), COALESCE(dna_haploid,''), COALESCE(dna_diploid,''), COALESCE(dna_mito,'')
		FROM spouses WHERE member_id = ? ORDER BY
		CASE type WHEN '正配' THEN 1 WHEN '继配' THEN 2 WHEN '侧室' THEN 3 ELSE 4 END, id`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	list := []Spouse{}
	for rows.Next() {
		var s Spouse
		rows.Scan(&s.ID, &s.MemberID, &s.Surname, &s.GivenName, &s.Name, &s.Type, &s.Family,
			&s.BirthCalendar, &s.BirthGregorian, &s.BirthLunar, &s.BirthGanzhi, &s.BirthEra,
			&s.DeathGregorian, &s.DeathLunar, &s.DeathGanzhi, &s.DeathEra,
			&s.Occupation, &s.Education, &s.Biography, &s.Remark, &s.Portrait, &s.Phone, &s.DNAHaploid, &s.DNADiploid, &s.DNAMito)
		list = append(list, s)
	}
	decryptSpousesInPlace(list, h.EncKey)
	role := c.GetString("role")
	for i := range list {
		sanitizeSpouseByRole(&list[i], role)
	}
	c.JSON(http.StatusOK, list)
}

// birthYearOf 从出生日期字符串里取年份（注意：与 member_chuizhu.go 里的 yearOf 不同，那个返回字符串）（容错 "1950-03-12"、"1950"、"1950年3月" 等）
func birthYearOf(s string) int {
	digits := make([]byte, 0, 4)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			digits = append(digits, c)
			if len(digits) == 4 {
				y, _ := strconv.Atoi(string(digits))
				if y >= 1000 && y <= 2200 {
					return y
				}
				digits = digits[:0]
			}
		} else {
			digits = digits[:0]
		}
	}
	return 0
}
