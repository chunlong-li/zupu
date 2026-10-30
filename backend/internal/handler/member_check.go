package handler

// 数据体检报告（族谱数据质量检查）
//
// 路由：GET /api/members/check
// 一次返回全部检查项（带 member_id，前端可点击跳到该成员修正）：
//   1. parent_child_age  父子年龄倒挂：子出生早于父/母，或父母与子女年龄差 < 12 岁
//   2. birth_death       生卒矛盾：卒年早于生年
//   3. duplicate         重复人物：同名同父 / 同名同生日
//   4. missing_required  缺失必填：无父无母且无生年的孤点
//   5. spouse_abnormal   配偶关系异常：同一配偶重复登记 / 多个正配（离婚、续弦未标注）
//   6. generation_gap    世代断档：某房支世代不连续 / 父子世代差不是 1
//
// 说明：检查范围为全库（不受顶部房支下拉影响），因为数据质量要按整谱判断。

import (
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const maxCheckItems = 500 // 每类最多返回的明细条数（Count 仍是真实总数）

type checkRelated struct {
	MemberID int64  `json:"member_id"`
	Name     string `json:"name"`
	Note     string `json:"note,omitempty"`
}

type checkItem struct {
	MemberID   int64          `json:"member_id"`
	Name       string         `json:"name"`
	Generation int            `json:"generation"`
	Branch     string         `json:"branch"`
	Detail     string         `json:"detail"`
	Related    []checkRelated `json:"related,omitempty"`
}

type checkCategory struct {
	Key         string      `json:"key"`
	Label       string      `json:"label"`
	Description string      `json:"description"`
	Count       int         `json:"count"`
	Items       []checkItem `json:"items"`
	Truncated   bool        `json:"truncated"`
}

type checkMember struct {
	ID             int64
	Name           string
	Generation     int
	Gender         int
	FatherID       int64
	MotherID       int64
	MotherSpouseID int64
	BranchID       int64
	Branch         string
	RecordType     string
	Birth          string
	Death          string
	BirthLunar     string
	DeathLunar     string
	BirthEra       string
	DeathEra       string
}

type checkSpouse struct {
	ID       int64
	MemberID int64
	Name     string
	Type     string
}

// Check 数据体检
// GET /api/members/check
func (h *MemberIOHandler) Check(c *gin.Context) {
	members, err := loadCheckMembers(h.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取成员失败：" + err.Error()})
		return
	}
	branches := map[int64]string{}
	if rows, err := h.DB.Query(`SELECT id, name FROM branches`); err == nil {
		for rows.Next() {
			var id int64
			var name string
			if rows.Scan(&id, &name) == nil {
				branches[id] = name
			}
		}
		rows.Close()
	}
	spouses, err := loadCheckSpouses(h.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取配偶失败：" + err.Error()})
		return
	}

	byID := map[int64]*checkMember{}
	byBranch := map[int64][]*checkMember{}
	branchName := func(m *checkMember) string {
		if n, ok := branches[m.BranchID]; ok && n != "" {
			return n
		}
		return m.Branch
	}
	for i := range members {
		m := &members[i]
		byID[m.ID] = m
		if m.BranchID > 0 {
			byBranch[m.BranchID] = append(byBranch[m.BranchID], m)
		}
	}

	// 1) 父子年龄倒挂
	parentAge := []checkItem{}
	addParentAge := func(m *checkMember, detail string, rel checkRelated) {
		parentAge = append(parentAge, checkItem{
			MemberID: m.ID, Name: m.Name, Generation: m.Generation,
			Branch: branchName(m), Detail: detail, Related: []checkRelated{rel},
		})
	}
	for i := range members {
		m := &members[i]
		by := yearOfDate(m.Birth)
		if by <= 0 {
			continue
		}
		if f := byID[m.FatherID]; f != nil {
			if fy := yearOfDate(f.Birth); fy > 0 {
				rel := checkRelated{MemberID: f.ID, Name: f.Name, Note: "生父"}
				if by < fy {
					addParentAge(m, fmt.Sprintf("子 %d 年生，早于生父 %s（%d 年生）", by, f.Name, fy), rel)
				} else if by-fy < 12 {
					addParentAge(m, fmt.Sprintf("与生父 %s 年龄差仅 %d 岁（<12）", f.Name, by-fy), rel)
				}
			}
		}
		if mo := byID[m.MotherID]; mo != nil {
			if my := yearOfDate(mo.Birth); my > 0 {
				rel := checkRelated{MemberID: mo.ID, Name: mo.Name, Note: "生母"}
				if by < my {
					addParentAge(m, fmt.Sprintf("子 %d 年生，早于生母 %s（%d 年生）", by, mo.Name, my), rel)
				} else if by-my < 12 {
					addParentAge(m, fmt.Sprintf("与生母 %s 年龄差仅 %d 岁（<12）", mo.Name, by-my), rel)
				}
			}
		}
	}

	// 2) 生卒矛盾
	birthDeath := []checkItem{}
	for i := range members {
		m := &members[i]
		if strings.TrimSpace(m.Birth) == "" || strings.TrimSpace(m.Death) == "" {
			continue
		}
		if dateBefore(m.Death, m.Birth) {
			birthDeath = append(birthDeath, checkItem{
				MemberID: m.ID, Name: m.Name, Generation: m.Generation,
				Branch: branchName(m),
				Detail: fmt.Sprintf("卒于 %s，早于生年 %s", m.Death, m.Birth),
			})
		}
	}

	// 3) 重复人物（同名同父 / 同名同生日）
	type dupGroup struct {
		key     string
		members []*checkMember
		detail  string
	}
	groups := map[string]*dupGroup{}
	addGroup := func(key, detail string, m *checkMember) {
		g := groups[key]
		if g == nil {
			g = &dupGroup{key: key, detail: detail}
			groups[key] = g
		}
		g.members = append(g.members, m)
	}
	for i := range members {
		m := &members[i]
		if strings.TrimSpace(m.Name) == "" {
			continue
		}
		fatherNote := "父不详"
		if f := byID[m.FatherID]; f != nil {
			fatherNote = "父：" + f.Name
		}
		addGroup("n\x1f"+m.Name+"\x1f"+strconv.FormatInt(m.FatherID, 10),
			fmt.Sprintf("同名同父「%s」（%s）", m.Name, fatherNote), m)
		if b := normalizeCheckDate(m.Birth); b != "" {
			addGroup("b\x1f"+m.Name+"\x1f"+b,
				fmt.Sprintf("同名同生日「%s」（%s）", m.Name, b), m)
		}
	}
	dup := []checkItem{}
	seenSets := map[string]bool{}
	groupKeys := make([]string, 0, len(groups))
	for k := range groups {
		groupKeys = append(groupKeys, k)
	}
	sort.Strings(groupKeys)
	for _, k := range groupKeys {
		g := groups[k]
		if len(g.members) < 2 {
			continue
		}
		ids := make([]int64, 0, len(g.members))
		for _, m := range g.members {
			ids = append(ids, m.ID)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		setKey := make([]string, len(ids))
		for i, id := range ids {
			setKey[i] = strconv.FormatInt(id, 10)
		}
		sk := strings.Join(setKey, ",")
		if seenSets[sk] {
			continue // 同一组人既同名同父又同生日，只报一次
		}
		seenSets[sk] = true
		rep := byID[ids[0]]
		related := []checkRelated{}
		names := []string{}
		for _, id := range ids {
			mm := byID[id]
			names = append(names, fmt.Sprintf("%s(ID:%d)", mm.Name, id))
			if id == rep.ID {
				continue
			}
			related = append(related, checkRelated{MemberID: id, Name: mm.Name, Note: "重复"})
		}
		dup = append(dup, checkItem{
			MemberID: rep.ID, Name: rep.Name, Generation: rep.Generation,
			Branch:  branchName(rep),
			Detail:  fmt.Sprintf("%s，共 %d 人：%s", g.detail, len(ids), strings.Join(names, "、")),
			Related: related,
		})
	}

	// 4) 缺失必填（无父无母无生年）
	missing := []checkItem{}
	for i := range members {
		m := &members[i]
		if m.FatherID != 0 || m.MotherID != 0 {
			continue
		}
		if strings.TrimSpace(m.Birth) != "" || strings.TrimSpace(m.BirthLunar) != "" || strings.TrimSpace(m.BirthEra) != "" {
			continue
		}
		detail := "无生父、无生母、无生年（孤点）"
		if m.RecordType == "外纪" {
			detail += "；类型为外纪，请确认是否需要补充来源"
		}
		missing = append(missing, checkItem{
			MemberID: m.ID, Name: m.Name, Generation: m.Generation,
			Branch: branchName(m), Detail: detail,
		})
	}

	// 5) 配偶关系异常
	spouseAbnormal := []checkItem{}
	spousesByMember := map[int64][]checkSpouse{}
	for _, s := range spouses {
		spousesByMember[s.MemberID] = append(spousesByMember[s.MemberID], s)
	}
	memberIDs := make([]int64, 0, len(spousesByMember))
	for id := range spousesByMember {
		memberIDs = append(memberIDs, id)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })
	for _, id := range memberIDs {
		m := byID[id]
		if m == nil {
			continue
		}
		list := spousesByMember[id]
		// 同名同类型重复
		counts := map[string][]checkSpouse{}
		for _, s := range list {
			key := s.Name + "\x1f" + s.Type
			counts[key] = append(counts[key], s)
		}
		dupKeys := make([]string, 0, len(counts))
		for k := range counts {
			if len(counts[k]) > 1 {
				dupKeys = append(dupKeys, k)
			}
		}
		sort.Strings(dupKeys)
		for _, k := range dupKeys {
			s := counts[k][0]
			spouseAbnormal = append(spouseAbnormal, checkItem{
				MemberID: m.ID, Name: m.Name, Generation: m.Generation,
				Branch: branchName(m),
				Detail: fmt.Sprintf("配偶「%s」（%s）重复登记 %d 次（配偶ID: %d 等）",
					emptyAs(s.Name, "（无姓名）"), emptyAs(s.Type, "未标类型"), len(counts[k]), s.ID),
			})
		}
		// 多个正配
		zhengpei := []checkSpouse{}
		for _, s := range list {
			if strings.TrimSpace(s.Type) == "正配" {
				zhengpei = append(zhengpei, s)
			}
		}
		if len(zhengpei) > 1 {
			names := make([]string, 0, len(zhengpei))
			for _, s := range zhengpei {
				names = append(names, emptyAs(s.Name, "（无姓名）"))
			}
			spouseAbnormal = append(spouseAbnormal, checkItem{
				MemberID: m.ID, Name: m.Name, Generation: m.Generation,
				Branch: branchName(m),
				Detail: fmt.Sprintf("有 %d 位「正配」（%s），疑似续弦/离异未标注（应改为继配/侧室或补标记）",
					len(zhengpei), strings.Join(names, "、")),
			})
		}
		// 无姓名
		for _, s := range list {
			if strings.TrimSpace(s.Name) == "" {
				spouseAbnormal = append(spouseAbnormal, checkItem{
					MemberID: m.ID, Name: m.Name, Generation: m.Generation,
					Branch: branchName(m),
					Detail: fmt.Sprintf("配偶记录缺少姓名（配偶ID: %d，类型：%s）", s.ID, emptyAs(s.Type, "未标")),
				})
			}
		}
	}

	// 6) 世代断档
	genGap := []checkItem{}
	// 6.1 房支内世代不连续
	branchIDs := make([]int64, 0, len(byBranch))
	for id := range byBranch {
		branchIDs = append(branchIDs, id)
	}
	sort.Slice(branchIDs, func(i, j int) bool { return branchIDs[i] < branchIDs[j] })
	for _, bid := range branchIDs {
		list := byBranch[bid]
		byGen := map[int][]*checkMember{}
		for _, m := range list {
			if m.RecordType == "外纪" || m.Generation < 1 {
				continue
			}
			byGen[m.Generation] = append(byGen[m.Generation], m)
		}
		gens := make([]int, 0, len(byGen))
		for g := range byGen {
			gens = append(gens, g)
		}
		sort.Ints(gens)
		for i := 1; i < len(gens); i++ {
			a, b := gens[i-1], gens[i]
			if b <= a+1 {
				continue
			}
			next := byGen[b][0]
			prev := byGen[a][0]
			rel := checkRelated{MemberID: prev.ID, Name: prev.Name, Note: "第" + strconv.Itoa(a) + "世"}
			genGap = append(genGap, checkItem{
				MemberID: next.ID, Name: next.Name, Generation: next.Generation,
				Branch: branchName(next),
				Detail: fmt.Sprintf("房支「%s」第 %d 世 → 第 %d 世之间缺第 %d 世（共 %d 人）",
					emptyAs(branchName(next), "未分房"), a, b, a+1, len(list)),
				Related: []checkRelated{rel},
			})
		}
	}
	// 6.2 父子世代差不是 1
	for i := range members {
		m := &members[i]
		if m.RecordType == "外纪" || m.Generation < 1 || m.FatherID == 0 {
			continue
		}
		f := byID[m.FatherID]
		if f == nil || f.RecordType == "外纪" || f.Generation < 1 {
			continue
		}
		if m.Generation != f.Generation+1 {
			genGap = append(genGap, checkItem{
				MemberID: m.ID, Name: m.Name, Generation: m.Generation,
				Branch: branchName(m),
				Detail: fmt.Sprintf("父子世代不连续：生父 %s 为第 %d 世，本人为第 %d 世（应相差 1）",
					f.Name, f.Generation, m.Generation),
				Related: []checkRelated{{MemberID: f.ID, Name: f.Name, Note: "生父"}},
			})
		}
	}

	cats := []checkCategory{
		makeCategory("parent_child_age", "父子年龄倒挂", "子出生早于父/母，或父母与子女年龄差 < 12 岁", parentAge),
		makeCategory("birth_death", "生卒矛盾", "去世日期早于出生日期", birthDeath),
		makeCategory("duplicate", "重复人物", "同名同父、或同名同生日，疑似重复录入", dup),
		makeCategory("missing_required", "缺失必填", "既无父母、又无生年的孤立点", missing),
		makeCategory("spouse_abnormal", "配偶关系异常", "配偶重复登记、多个「正配」未标续弦/离异、配偶缺姓名", spouseAbnormal),
		makeCategory("generation_gap", "世代断档", "某房支世代不连续，或父子世代差不是 1", genGap),
	}
	summary := map[string]int{}
	total := 0
	for _, cat := range cats {
		summary[cat.Key] = cat.Count
		total += cat.Count
	}
	c.JSON(http.StatusOK, gin.H{
		"generated_at":  time.Now().Format("2006-01-02 15:04:05"),
		"total_members": len(members),
		"total_issues":  total,
		"summary":       summary,
		"categories":    cats,
	})
}

// makeCategory 生成一个类目（排序 + 截断）
func makeCategory(key, label, desc string, list []checkItem) checkCategory {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Generation != list[j].Generation {
			return list[i].Generation < list[j].Generation
		}
		return list[i].MemberID < list[j].MemberID
	})
	count := len(list)
	truncated := false
	if len(list) > maxCheckItems {
		list = list[:maxCheckItems]
		truncated = true
	}
	if list == nil {
		list = []checkItem{}
	}
	return checkCategory{Key: key, Label: label, Description: desc, Count: count, Items: list, Truncated: truncated}
}

func loadCheckMembers(db *sql.DB) ([]checkMember, error) {
	rows, err := db.Query(`SELECT id, COALESCE(name,''), COALESCE(generation,1), COALESCE(gender,1),
		COALESCE(father_id,0), COALESCE(mother_id,0), COALESCE(mother_spouse_id,0),
		COALESCE(branch_id,0), COALESCE(branch,''), COALESCE(record_type,''),
		COALESCE(birth_gregorian,''), COALESCE(death_gregorian,''),
		COALESCE(birth_lunar,''), COALESCE(death_lunar,''), COALESCE(birth_era,''), COALESCE(death_era,'')
		FROM members ORDER BY generation, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []checkMember{}
	for rows.Next() {
		var m checkMember
		if err := rows.Scan(&m.ID, &m.Name, &m.Generation, &m.Gender, &m.FatherID, &m.MotherID,
			&m.MotherSpouseID, &m.BranchID, &m.Branch, &m.RecordType,
			&m.Birth, &m.Death, &m.BirthLunar, &m.DeathLunar, &m.BirthEra, &m.DeathEra); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func loadCheckSpouses(db *sql.DB) ([]checkSpouse, error) {
	rows, err := db.Query(`SELECT id, member_id, COALESCE(name,''), COALESCE(type,'') FROM spouses ORDER BY member_id, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []checkSpouse{}
	for rows.Next() {
		var s checkSpouse
		if err := rows.Scan(&s.ID, &s.MemberID, &s.Name, &s.Type); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// yearOfDate 从 YYYY-MM-DD / YYYY 里取年份；非法返回 0
func yearOfDate(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if len(s) >= 4 {
		if y, err := strconv.Atoi(s[:4]); err == nil && y > 0 && y < 10000 {
			return y
		}
	}
	return 0
}

// normalizeCheckDate 统一成 YYYY-MM-DD / YYYY（用于同名同生日分组）
func normalizeCheckDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if d, ok := normalizeImportDate(s); ok {
		return d
	}
	return ""
}

// dateBefore 判断 a 是否早于 b（支持 YYYY-MM-DD 与 YYYY，年份不同时按年比）
func dateBefore(a, b string) bool {
	an, aok := normalizeImportDate(a)
	bn, bok := normalizeImportDate(b)
	if !aok || !bok {
		ay, by := yearOfDate(a), yearOfDate(b)
		if ay > 0 && by > 0 {
			return ay < by
		}
		return false
	}
	if len(an) == 4 || len(bn) == 4 {
		return yearOfDate(an) < yearOfDate(bn)
	}
	return an < bn
}

func emptyAs(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
