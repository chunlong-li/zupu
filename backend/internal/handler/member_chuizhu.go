package handler

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	chuiZhuStep = 4 // 边界世代间隔（1→5→9→13→17…，每隔 4 世「一提」）
	chuiZhuGens = 5 // 每页 5 世（五世一图，横版，世代自上而下）
)

// ChuiZhuSpouse 行传中配偶一列（配 / 继配 / 侧室）
type ChuiZhuSpouse struct {
	Name       string `json:"name"`        // 王氏
	Rel        string `json:"rel"`         // 配 / 繼配 / 側室
	BirthDeath string `json:"birth_death"` // 生X卒Y
	Bio        string `json:"bio"`         // 生平条目正文
}

// ChuiZhuMember 垂珠体单个成员
type ChuiZhuMember struct {
	ID              int64           `json:"id"`
	Name            string          `json:"name"`
	Rel             string          `json:"rel"` // 關係格：長子/次子/三子…（父僅一子作「子」）
	Rank            string          `json:"rank"`
	Zihao           string          `json:"zihao"`
	BirthDeath      string          `json:"birth_death"`
	BirthDeathShort string          `json:"birth_death_short"` // 圖上生卒簡式（只到年）
	Spouse          string          `json:"spouse"`
	Burial          string          `json:"burial"`
	Children        string          `json:"children"`     // 生子N
	Sons            []ChuiZhuSon    `json:"sons"`         // 诸子（名 + id，供图上列名/跳转）
	Bio             string          `json:"bio"`          // 生平条目正文（行传用）
	Spouses         []ChuiZhuSpouse `json:"spouses"`      // 行传中配偶各占一列
	Span            int             `json:"span"`         // 横向宽度权重（页内叶子数）
	NoHeir          int             `json:"no_heir"`      // 无嗣
	LostContact     int             `json:"lost_contact"` // 迁居外地失联
	BirthKnown      bool            `json:"birth_known"`
	DeathKnown      bool            `json:"death_known"`
	BurialKnown     bool            `json:"burial_known"`
	BirthStr        string          `json:"birth_str"` // 已格式化生（如「生雍正十年…」），未詳為空
	DeathStr        string          `json:"death_str"` // 已格式化殊（如「殊民國十年」），未詳為空
}

// ChuiZhuSon 垂珠体图上「生子N」下要列的某个儿子
type ChuiZhuSon struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ChuiZhuGeneration 一个世代（横排）
type ChuiZhuGeneration struct {
	Gen     int             `json:"gen"`
	Members []ChuiZhuMember `json:"members"`
}

// ChuiZhuPage 一页（五世一图：一位「提」头 + 其后代四世）
type ChuiZhuPage struct {
	PageIndex   int                 `json:"page_index"`
	HeadID      int64               `json:"head_id"`
	HeadName    string              `json:"head_name"`
	StartGen    int                 `json:"start_gen"`   // 头所在世（最上排）
	EndGen      int                 `json:"end_gen"`     // 最下排世（头世+4）
	Generations []ChuiZhuGeneration `json:"generations"` // 5 个横排，第一排仅头一人
}

// ChuiZhuResponse 垂珠体全部页面（连续滚动，一次返回）
type ChuiZhuResponse struct {
	TotalPages int             `json:"total_pages"`
	Ancestors  []ChuiZhuMember `json:"ancestors"` // 外纪（暂不渲染）
	Pages      []ChuiZhuPage   `json:"pages"`
}

type chuiZhuNode struct {
	id, generation, gender, fatherID                    int64
	givenName, name, surname, zi, hao, rank, recordType string
	clanType                                            string
	birth, death, burial                                string
	birthLunar, birthGanzhi, birthEra                   string
	deathLunar, deathGanzhi, deathEra                   string
	noHeir, lostContact                                 int
	bio                                                 string
	rel                                                 string // 長子/次子/…
	spouses                                             []chuiZhuSpouse
	children                                            []*chuiZhuNode
	span                                                int
}

type chuiZhuSpouse struct {
	id                                int64
	surname, name, typ                string
	birth, death                      string
	birthLunar, birthGanzhi, birthEra string
	deathLunar, deathGanzhi, deathEra string
	bio                               string
}

// ListChuiZhu 组装苏氏垂珠体（横版，五世一提，世代自上而下，人从右往左）
// GET /api/members/chuizhu
// 结构：始祖页 = 世1~5；之后第5世每人单独一页（自己排第一、后代排 6/7/8/9 世）；
// 第9世每人再单独一页（9~13世），以此类推（边界世代 1,5,9,13,17…）
func (h *MemberHandler) ListChuiZhu(c *gin.Context) {
	// 家族设置：苏氏垂珠体是否显示女儿（默认开；女儿只显示到配偶，不列其子女）
	showDaughters := true
	{
		var v string
		if err := h.DB.QueryRow(`SELECT COALESCE((SELECT value FROM settings WHERE key = 'chuizhu_show_daughters'), '1')`).Scan(&v); err == nil {
			showDaughters = v != "0"
		}
	}

	bcond, bargs := BranchScope(c, h.DB, "branch_id")
	mq := `SELECT id, generation, surname, given_name, name, zi, hao, rank, gender, father_id,
		COALESCE(record_type,''), COALESCE(clan_type,''), COALESCE(birth_gregorian,''), COALESCE(death_gregorian,''), COALESCE(burial_place,''),
		COALESCE(birth_lunar,''), COALESCE(birth_ganzhi,''), COALESCE(birth_era,''),
		COALESCE(death_lunar,''), COALESCE(death_ganzhi,''), COALESCE(death_era,''),
		COALESCE(no_heir,0), COALESCE(lost_contact,0)
		FROM members`
	if bcond != "" {
		mq += " WHERE " + bcond
	}
	mq += " ORDER BY generation, id"
	rows, err := h.DB.Query(mq, bargs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var nodes []*chuiZhuNode
	byID := map[int64]*chuiZhuNode{}
	for rows.Next() {
		n := &chuiZhuNode{}
		if err := rows.Scan(&n.id, &n.generation, &n.surname, &n.givenName, &n.name, &n.zi, &n.hao, &n.rank, &n.gender, &n.fatherID, &n.recordType, &n.clanType, &n.birth, &n.death, &n.burial,
			&n.birthLunar, &n.birthGanzhi, &n.birthEra, &n.deathLunar, &n.deathGanzhi, &n.deathEra,
			&n.noHeir, &n.lostContact); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		nodes = append(nodes, n)
		byID[n.id] = n
	}

	spq := `SELECT id, member_id, COALESCE(surname,''), COALESCE(name,''), COALESCE(type,''),
		COALESCE(birth_gregorian,''), COALESCE(death_gregorian,''),
		COALESCE(birth_lunar,''), COALESCE(birth_ganzhi,''), COALESCE(birth_era,''),
		COALESCE(death_lunar,''), COALESCE(death_ganzhi,''), COALESCE(death_era,'')
		FROM spouses`
	if bcond != "" {
		spq += " WHERE member_id IN (SELECT id FROM members WHERE " + bcond + ")"
	}
	spq += ` ORDER BY CASE type WHEN '正配' THEN 1 WHEN '继配' THEN 2 WHEN '侧室' THEN 3 ELSE 4 END, id`
	spRows, err := h.DB.Query(spq, bargs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer spRows.Close()
	for spRows.Next() {
		var mid int64
		s := chuiZhuSpouse{}
		if err := spRows.Scan(&s.id, &mid, &s.surname, &s.name, &s.typ, &s.birth, &s.death,
			&s.birthLunar, &s.birthGanzhi, &s.birthEra, &s.deathLunar, &s.deathGanzhi, &s.deathEra); err != nil {
			continue
		}
		if n, ok := byID[mid]; ok {
			n.spouses = append(n.spouses, s)
		}
	}

	// 配偶生平条目（行传配偶列正文）
	spBio := map[int64][]string{}
	if spBioRows, sberr := h.DB.Query(`SELECT spouse_id, COALESCE(content,'') FROM spouse_bio_entries ORDER BY spouse_id, sort_order, id`); sberr == nil {
		defer spBioRows.Close()
		for spBioRows.Next() {
			var sid int64
			var content string
			if err := spBioRows.Scan(&sid, &content); err != nil {
				continue
			}
			content = strings.TrimSpace(content)
			if content == "" {
				continue
			}
			spBio[sid] = append(spBio[sid], content)
		}
	}
	for _, n := range nodes {
		for i := range n.spouses {
			if parts := spBio[n.spouses[i].id]; len(parts) > 0 {
				n.spouses[i].bio = strings.Join(parts, "\n")
			}
		}
	}

	// 生平条目（行传正文）
	if bioRows, berr := h.DB.Query(`SELECT member_id, COALESCE(content,'') FROM member_bio_entries ORDER BY member_id, sort_order, id`); berr == nil {
		defer bioRows.Close()
		bioMap := map[int64][]string{}
		for bioRows.Next() {
			var mid int64
			var content string
			if err := bioRows.Scan(&mid, &content); err != nil {
				continue
			}
			content = strings.TrimSpace(content)
			if content == "" {
				continue
			}
			bioMap[mid] = append(bioMap[mid], content)
		}
		for id, parts := range bioMap {
			if n, ok := byID[id]; ok {
				n.bio = strings.Join(parts, "\n")
			}
		}
	}

	// 招赘续房：女儿若①自身 clan_type 标了「入赘/招赘/赘婿」，②或名下有子女（孩子的 father_id 指向她），
	// 说明这一支是招赘续香火——即使开关关闭也保留（按男丁世系显示，子女照常入图）
	childCount := map[int64]int{}
	for _, n := range nodes {
		if n.fatherID != 0 {
			childCount[n.fatherID]++
		}
	}
	keepFemale := map[int64]bool{}
	for _, n := range nodes {
		if n.gender != 2 {
			continue
		}
		ct := strings.TrimSpace(n.clanType)
		if ct == "入赘" || ct == "招赘" || ct == "贄婿" || strings.Contains(ct, "贄") || strings.Contains(ct, "招贄") || childCount[n.id] > 0 {
			keepFemale[n.id] = true
		}
	}
	includeNode := func(n *chuiZhuNode) bool {
		if n.gender != 2 {
			return true
		}
		return showDaughters || keepFemale[n.id]
	}

	// 建树（女儿：开关关且非招赘续房时整支不进树；招赘支正常入图）
	for _, n := range nodes {
		if !includeNode(n) {
			continue
		}
		if n.fatherID != 0 {
			if p, ok := byID[n.fatherID]; ok && includeNode(p) {
				p.children = append(p.children, n)
			}
		}
	}
	for _, n := range nodes {
		sortChildren(n.children)
	}

	// 關係格：長子/次子/三子…（父僅一子作「子」；女作「女」）
	for _, n := range nodes {
		if n.fatherID == 0 {
			continue
		}
		p, ok := byID[n.fatherID]
		if !ok || p == nil {
			continue
		}
		sibs := []*chuiZhuNode{}
		for _, c := range p.children {
			if (c.gender == 2) == (n.gender == 2) {
				sibs = append(sibs, c)
			}
		}
		idx := 0
		for i, c := range sibs {
			if c.id == n.id {
				idx = i + 1
				break
			}
		}
		n.rel = relLabel(int(n.gender), idx, len(sibs))
		if n.rel != "" {
			n.rel = chuiZhuName(p) + n.rel
		}
	}

	// 外纪 + 内纪始祖 + 内纪最大世代
	// 过滤模式：以范围内最小内纪世代为「起世」，该世代无范围内父辈者即各页之「提」
	baseGen := 1
	if bcond != "" {
		minGen := 0
		for _, n := range nodes {
			if !includeNode(n) || n.recordType == "外纪" || int(n.generation) <= 0 {
				continue
			}
			g := int(n.generation)
			if minGen == 0 || g < minGen {
				minGen = g
			}
		}
		if minGen > 0 {
			baseGen = minGen
		}
	}

	var ancestors []*chuiZhuNode
	var innerRoots []*chuiZhuNode
	innerMaxGen := 0
	for _, n := range nodes {
		if !includeNode(n) {
			continue
		}
		if n.recordType == "外纪" || n.generation <= 0 {
			ancestors = append(ancestors, n)
		} else {
			if int(n.generation) == baseGen {
				innerRoots = append(innerRoots, n)
			}
			if int(n.generation) > innerMaxGen {
				innerMaxGen = int(n.generation)
			}
		}
	}
	sortChildren(innerRoots)
	if len(innerRoots) > 0 {
		if baseGen > 1 {
			innerRoots[0].rel = "房祖"
		} else {
			innerRoots[0].rel = "始祖" // 行傳：世1 第一人關係格寫「始祖」
		}
	}

	// 全树 DFS（长子优先），用于按世代取成员
	var order []*chuiZhuNode
	var dfs func(n *chuiZhuNode)
	dfs = func(n *chuiZhuNode) {
		order = append(order, n)
		for _, c := range n.children {
			dfs(c)
		}
	}
	for _, r := range innerRoots {
		dfs(r)
	}

	// 生成全部页面（广度优先：「起世」各「提」→ 起世+4各人 → +4各人 → …）
	var pages []ChuiZhuPage
	pageIdx := 0
	for _, r := range innerRoots {
		pageIdx++
		page, _ := buildChuiZhuPage(r, baseGen, pageIdx)
		pages = append(pages, page)
	}
	for gen := baseGen + chuiZhuStep; gen <= innerMaxGen; gen += chuiZhuStep {
		for _, n := range order {
			if int(n.generation) == gen && len(n.children) > 0 {
				pageIdx++
				page, _ := buildChuiZhuPage(n, gen, pageIdx)
				pages = append(pages, page)
			}
		}
	}

	resp := ChuiZhuResponse{
		TotalPages: len(pages),
		Ancestors:  []ChuiZhuMember{},
		Pages:      pages,
	}
	for _, a := range ancestors {
		resp.Ancestors = append(resp.Ancestors, chuiZhuToMember(a))
	}

	c.JSON(http.StatusOK, resp)
}

// buildChuiZhuPage 以 head 为「提」头，组装一页（head + 其后代四世），返回页 + 末排成员（供递归）
func buildChuiZhuPage(head *chuiZhuNode, startGen, idx int) (ChuiZhuPage, []*chuiZhuNode) {
	gens := make([][]*chuiZhuNode, chuiZhuGens)
	gens[0] = []*chuiZhuNode{head}

	var walk func(n *chuiZhuNode)
	walk = func(n *chuiZhuNode) {
		for _, c := range n.children {
			ci := int(c.generation) - startGen
			if ci >= 1 && ci < chuiZhuGens {
				gens[ci] = append(gens[ci], c)
				if ci < chuiZhuGens-1 {
					walk(c)
				}
			}
		}
	}
	walk(head)

	// 页内叶子权重（父 span = 诸子 span 之和；页末排 span=1）
	leafGen := startGen + chuiZhuGens - 1
	var layout func(n *chuiZhuNode) int
	layout = func(n *chuiZhuNode) int {
		if len(n.children) == 0 || int(n.generation) >= leafGen {
			n.span = 1
			return 1
		}
		total := 0
		for _, c := range n.children {
			total += layout(c)
		}
		n.span = total
		return total
	}
	layout(head)

	page := ChuiZhuPage{
		PageIndex:   idx,
		HeadID:      head.id,
		HeadName:    chuiZhuName(head),
		StartGen:    startGen,
		EndGen:      startGen + chuiZhuGens - 1,
		Generations: []ChuiZhuGeneration{},
	}
	for gi := 0; gi < chuiZhuGens; gi++ {
		members := []ChuiZhuMember{}
		for _, n := range gens[gi] {
			members = append(members, chuiZhuToMember(n))
		}
		page.Generations = append(page.Generations, ChuiZhuGeneration{Gen: startGen + gi, Members: members})
	}
	return page, gens[chuiZhuGens-1]
}

func chuiZhuToMember(n *chuiZhuNode) ChuiZhuMember {
	sons := []ChuiZhuSon{}
	for _, c := range n.children {
		if c.gender != 2 { // 2=女，其余（男/未知）计为子
			sons = append(sons, ChuiZhuSon{ID: c.id, Name: chuiZhuName(c)})
		}
	}
	spouses := []ChuiZhuSpouse{}
	for _, s := range n.spouses {
		spouses = append(spouses, ChuiZhuSpouse{
			Name: chuiZhuSpouseName(s),
			Rel:  spouseRelLabel(s.typ),
			BirthDeath: formatBirthDeathCN(s.birth, s.birthLunar, s.birthGanzhi, s.birthEra,
				s.death, s.deathLunar, s.deathGanzhi, s.deathEra),
			Bio: s.bio,
		})
	}
	return ChuiZhuMember{
		ID:    n.id,
		Name:  chuiZhuName(n),
		Rel:   n.rel,
		Rank:  n.rank,
		Zihao: formatZihao(n.zi, n.hao),
		BirthDeath: formatBirthDeathCN(n.birth, n.birthLunar, n.birthGanzhi, n.birthEra,
			n.death, n.deathLunar, n.deathGanzhi, n.deathEra),
		BirthDeathShort: formatBirthDeathShort(n.birth, n.birthEra, n.death, n.deathEra),
		Spouse:          formatSpouseOld(n.spouses),
		Burial:          formatBurial(n.burial),
		Children:        formatChildren(len(sons)),
		Sons:            sons,
		Bio:             n.bio,
		Spouses:         spouses,
		Span:            n.span,
		NoHeir:          n.noHeir,
		LostContact:     n.lostContact,
		BirthKnown:      n.birth != "",
		DeathKnown:      n.death != "",
		BurialKnown:     n.burial != "",
		BirthStr:        birthOnlyCN(n.birth, n.birthLunar, n.birthGanzhi, n.birthEra),
		DeathStr:        deathOnlyCN(n.death, n.deathLunar, n.deathGanzhi, n.deathEra),
	}
}

// 行傳配偶列名：王氏 / 某氏
func chuiZhuSpouseName(s chuiZhuSpouse) string {
	if s.surname != "" {
		return s.surname + "氏"
	}
	return s.name
}

// 行傳配偶列關係字：配 / 繼配 / 側室
func spouseRelLabel(t string) string {
	switch t {
	case "继配", "繼配":
		return "繼配"
	case "侧室", "側室":
		return "側室"
	}
	return "配"
}

// 關係字：長子/次子/三子…
func relLabel(gender, idx, total int) string {
	if idx <= 0 {
		return ""
	}
	suffix := "子"
	if gender == 2 {
		suffix = "女"
	}
	if total <= 1 {
		return "之" + suffix
	}
	return cnRank(idx) + suffix
}

func cnRank(n int) string {
	if n == 1 {
		return "長"
	}
	if n == 2 {
		return "次"
	}
	d := []string{"", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
	if n < 10 {
		return d[n]
	}
	if n < 20 {
		return "十" + d[n-10]
	}
	return d[n/10] + "十" + d[n%10]
}

func chuiZhuName(n *chuiZhuNode) string {
	if n.givenName != "" {
		return n.givenName
	}
	return n.name
}

func formatZihao(zi, hao string) string {
	parts := []string{}
	if zi != "" {
		parts = append(parts, "字"+zi)
	}
	if hao != "" {
		parts = append(parts, "號"+hao)
	}
	return strings.Join(parts, "")
}

func yearOf(s string) string {
	if s == "" {
		return ""
	}
	if i := strings.Index(s, "-"); i > 0 {
		return s[:i]
	}
	return s
}

// 生卒（農曆、中文數字）：生雍正十年壬子十月廿四日。卒嘉慶十八年癸酉七月十三日
// 生（已格式化）：如「生雍正十年壬子十月廿四日」「生一九五〇年」；未詳返回 ""
func birthOnlyCN(gBirth, lBirth, zBirth, eBirth string) string {
	if s := lunarPart(eBirth, lBirth, zBirth); s != "" {
		return "生" + s
	}
	if y := yearOf(gBirth); y != "" {
		return "生" + cnDigits(y)
	}
	return ""
}

// 殊（已格式化）：如「殊民國十年」；未詳返回 ""
func deathOnlyCN(gDeath, lDeath, zDeath, eDeath string) string {
	if s := lunarPart(eDeath, lDeath, zDeath); s != "" {
		return "殊" + s
	}
	if y := yearOf(gDeath); y != "" {
		return "殊" + cnDigits(y)
	}
	return ""
}

func formatBirthDeathCN(gBirth, lBirth, zBirth, eBirth, gDeath, lDeath, zDeath, eDeath string) string {
	parts := []string{}
	if s := lunarPart(eBirth, lBirth, zBirth); s != "" {
		parts = append(parts, "生"+s)
	} else if y := yearOf(gBirth); y != "" {
		parts = append(parts, "生"+cnDigits(y))
	}
	if s := lunarPart(eDeath, lDeath, zDeath); s != "" {
		parts = append(parts, "卒"+s)
	} else if y := yearOf(gDeath); y != "" {
		parts = append(parts, "卒"+cnDigits(y))
	}
	return strings.Join(parts, "。")
}

// 農曆日期串：年號 + 年 + 干支年 + 月日（如 雍正十年壬子十月廿四日）
func lunarPart(era, lunar, ganzhi string) string {
	era = strings.TrimSpace(era)
	lunar = strings.TrimSpace(lunar)
	ganzhi = strings.TrimSpace(ganzhi)
	if era == "" && ganzhi == "" {
		if lunar == "" {
			return ""
		}
		return cnDigits(lunar) + "日"
	}
	out := era
	if out != "" {
		out += "年"
	}
	if ganzhi != "" {
		if i := strings.Index(ganzhi, "年"); i > 0 {
			ganzhi = ganzhi[:i]
		}
		out += ganzhi
	}
	if lunar != "" {
		if i := strings.Index(lunar, "年"); i >= 0 {
			lunar = lunar[i+len("年"):]
		}
		out += lunar + "日"
	}
	return out
}

// 阿拉伯數字 → 中文數字（後備年份用）
func cnDigits(s string) string {
	d := []string{"〇", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteString(d[r-'0'])
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// 圖上用的生卒簡式：只到年（年號+年），無年號則用中文公歷年
func formatBirthDeathShort(gBirth, eBirth, gDeath, eDeath string) string {
	parts := []string{}
	if s := shortYear(eBirth, gBirth); s != "" {
		parts = append(parts, "生"+s)
	}
	if s := shortYear(eDeath, gDeath); s != "" {
		parts = append(parts, "卒"+s)
	}
	return strings.Join(parts, "。")
}

func shortYear(era, greg string) string {
	if e := strings.TrimSpace(era); e != "" {
		return e + "年"
	}
	if y := yearOf(greg); y != "" {
		return cnDigits(y)
	}
	return ""
}

func formatBirthDeath(birth, death string) string {
	parts := []string{}
	if y := yearOf(birth); y != "" {
		parts = append(parts, "生"+y)
	}
	if y := yearOf(death); y != "" {
		parts = append(parts, "卒"+y)
	}
	return strings.Join(parts, "")
}

func formatSpouseOld(spouses []chuiZhuSpouse) string {
	parts := []string{}
	for _, s := range spouses {
		label := spouseRelLabel(s.typ)
		if disp := chuiZhuSpouseName(s); disp != "" {
			parts = append(parts, label+disp)
		}
	}
	return strings.Join(parts, "")
}

func formatSpouse(spouses []Spouse) string {
	parts := []string{}
	for _, s := range spouses {
		label := "配"
		if s.Type == "继配" || s.Type == "繼配" {
			label = "繼配"
		} else if s.Type == "侧室" || s.Type == "側室" {
			label = "側室"
		}
		if s.Surname != "" {
			parts = append(parts, label+s.Surname+"氏")
		} else if s.Name != "" {
			parts = append(parts, label+s.Name)
		}
	}
	return strings.Join(parts, "")
}

func formatBurial(burial string) string {
	if burial == "" {
		return ""
	}
	return "葬" + burial
}

func formatChildren(sons int) string {
	if sons <= 0 {
		return ""
	}
	return "生子" + strconv.Itoa(sons)
}

func rankNumFor(rank string) int {
	r := strings.TrimSpace(rank)
	if r == "" {
		return 0
	}
	if strings.HasPrefix(r, "行") {
		if n, err := strconv.Atoi(strings.TrimPrefix(r, "行")); err == nil {
			return n
		}
	}
	cn := map[string]int{
		"长": 1, "次": 2, "三": 3, "四": 4, "五": 5,
		"六": 6, "七": 7, "八": 8, "九": 9, "十": 10,
		"长子": 1, "次子": 2, "三子": 3, "四子": 4, "五子": 5,
		"六子": 6, "七子": 7, "八子": 8, "九子": 9, "十子": 10,
	}
	if n, ok := cn[r]; ok {
		return n
	}
	return 0
}

func sortChildren(nodes []*chuiZhuNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		ri, rj := rankNumFor(nodes[i].rank), rankNumFor(nodes[j].rank)
		if ri != rj {
			if ri == 0 {
				return false
			}
			if rj == 0 {
				return true
			}
			return ri < rj
		}
		if nodes[i].birth != nodes[j].birth {
			return nodes[i].birth < nodes[j].birth
		}
		return nodes[i].id < nodes[j].id
	})
}
