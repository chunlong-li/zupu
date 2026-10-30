package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
)

// FieldChange 一次修改中的单个字段变更（旧值 → 新值）
type FieldChange struct {
	Field string `json:"field"` // 字段中文名
	From  string `json:"from"`  // 原值（空串显示为「空」）
	To    string `json:"to"`    // 新值（空串显示为「空」）
}

// val 空值归一：空字符串统一返回 "空"
func val(s string) string {
	if s == "" {
		return "空"
	}
	return s
}

// scmp 比较两个字符串，返回（旧值、新值、是否变化）
func scmp(from, to string) (string, string, bool) {
	if from == to {
		return "", "", false
	}
	return val(from), val(to), true
}

// icmp 比较两个整数
func icmp(from, to int) (string, string, bool) {
	if from == to {
		return "", "", false
	}
	return strconv.Itoa(from), strconv.Itoa(to), true
}

// appendChange 追加一条变更
func appendChange(list []FieldChange, label, from, to string, changed bool) []FieldChange {
	if !changed {
		return list
	}
	return append(list, FieldChange{Field: label, From: from, To: to})
}

// genderText 性别数字转文本
func genderText(g int) string {
	if g == 2 {
		return "女"
	}
	return "男"
}

// addSet 仅当值非空时追加「空→值」
func addSet(list []FieldChange, label, v string) []FieldChange {
	if v == "" {
		return list
	}
	return append(list, FieldChange{Field: label, From: "空", To: v})
}

// memberCreateChanges 新增成员：列出所有已填写的字段
func memberCreateChanges(m *Member) []FieldChange {
	var out []FieldChange
	if m.Generation > 0 {
		out = append(out, FieldChange{Field: "世代", From: "空", To: strconv.Itoa(m.Generation)})
	}
	out = addSet(out, "房支", m.Branch)
	out = addSet(out, "姓氏", m.Surname)
	out = addSet(out, "名", m.GivenName)
	out = addSet(out, "字", m.Zi)
	out = addSet(out, "号", m.Hao)
	out = addSet(out, "字辈", m.GenerationWord)
	out = addSet(out, "排行", m.Rank)
	if m.Gender != 0 {
		out = append(out, FieldChange{Field: "性别", From: "空", To: genderText(m.Gender)})
	}
	if m.FatherID > 0 {
		out = append(out, FieldChange{Field: "父亲", From: "空", To: strconv.FormatInt(m.FatherID, 10)})
	}
	if m.MotherID > 0 {
		out = append(out, FieldChange{Field: "母亲", From: "空", To: strconv.FormatInt(m.MotherID, 10)})
	}
	out = addSet(out, "族类", m.ClanType)
	out = addSet(out, "出生公历", m.BirthGregorian)
	out = addSet(out, "出生农历", m.BirthLunar)
	out = addSet(out, "出生年号", m.BirthEra)
	out = addSet(out, "逝世公历", m.DeathGregorian)
	out = addSet(out, "逝世农历", m.DeathLunar)
	out = addSet(out, "逝世年号", m.DeathEra)
	if m.Lifespan > 0 {
		out = append(out, FieldChange{Field: "享年", From: "空", To: strconv.Itoa(m.Lifespan)})
	}
	out = addSet(out, "籍贯", m.NativePlace)
	out = addSet(out, "现居地", m.CurrentPlace)
	out = addSet(out, "头衔", m.Title)
	out = addSet(out, "职业", m.Occupation)
	out = addSet(out, "学历", m.Education)
	out = addSet(out, "生平", m.Biography)
	out = addSet(out, "葬地", m.BurialPlace)
	out = addSet(out, "出嫁", m.MarriedOut)
	out = addSet(out, "曾用名", m.FormerName)
	return out
}

// spouseCreateChanges 新增配偶：列出所有已填写的字段
func spouseCreateChanges(s *Spouse) []FieldChange {
	var out []FieldChange
	out = addSet(out, "姓氏", s.Surname)
	out = addSet(out, "名", s.GivenName)
	out = addSet(out, "类型", s.Type)
	out = addSet(out, "家世", s.Family)
	out = addSet(out, "出生公历", s.BirthGregorian)
	out = addSet(out, "出生农历", s.BirthLunar)
	out = addSet(out, "逝世公历", s.DeathGregorian)
	out = addSet(out, "逝世农历", s.DeathLunar)
	out = addSet(out, "职业", s.Occupation)
	out = addSet(out, "学历", s.Education)
	out = addSet(out, "生平", s.Biography)
	out = addSet(out, "备注", s.Remark)
	return out
}

// generationWordCreateChanges 新增字辈派语
func generationWordCreateChanges(g *GenerationWord) []FieldChange {
	var out []FieldChange
	out = addSet(out, "支系", g.Branch)
	out = addSet(out, "派语诗文", g.Poem)
	if g.StartGeneration > 0 {
		out = append(out, FieldChange{Field: "起始世代", From: "空", To: strconv.Itoa(g.StartGeneration)})
	}
	return out
}

// bioChanges 生平条目字段差异（成员/配偶通用）
func bioChanges(oldTitle, oldContent string, oldSort int, newTitle, newContent string, newSort int) []FieldChange {
	var out []FieldChange
	f, t, c := scmp(oldTitle, newTitle)
	out = appendChange(out, "标题", f, t, c)
	f, t, c = scmp(oldContent, newContent)
	out = appendChange(out, "内容", f, t, c)
	f, t, c = icmp(oldSort, newSort)
	out = appendChange(out, "排序", f, t, c)
	return out
}

// bioCreateChanges 新增生平条目：列出已填字段
func bioCreateChanges(title, content string, sort int) []FieldChange {
	var out []FieldChange
	out = addSet(out, "标题", title)
	out = addSet(out, "内容", content)
	if sort > 0 {
		out = append(out, FieldChange{Field: "排序", From: "空", To: strconv.Itoa(sort)})
	}
	return out
}

// memberChanges 成员：old 与 new 的字段差异
func memberChanges(old, new *Member) []FieldChange {
	var out []FieldChange
	f, t, c := icmp(old.Generation, new.Generation)
	out = appendChange(out, "世代", f, t, c)
	f, t, c = scmp(old.Branch, new.Branch)
	out = appendChange(out, "房支", f, t, c)
	f, t, c = scmp(old.Surname, new.Surname)
	out = appendChange(out, "姓氏", f, t, c)
	f, t, c = scmp(old.GivenName, new.GivenName)
	out = appendChange(out, "名", f, t, c)
	f, t, c = scmp(old.Zi, new.Zi)
	out = appendChange(out, "字", f, t, c)
	f, t, c = scmp(old.Hao, new.Hao)
	out = appendChange(out, "号", f, t, c)
	f, t, c = scmp(old.GenerationWord, new.GenerationWord)
	out = appendChange(out, "字辈", f, t, c)
	f, t, c = scmp(old.Rank, new.Rank)
	out = appendChange(out, "排行", f, t, c)
	if old.Gender != new.Gender {
		out = appendChange(out, "性别", genderText(old.Gender), genderText(new.Gender), true)
	}
	if old.FatherID != new.FatherID {
		out = appendChange(out, "父亲", strconv.FormatInt(old.FatherID, 10), strconv.FormatInt(new.FatherID, 10), true)
	}
	if old.MotherID != new.MotherID {
		out = appendChange(out, "母亲", strconv.FormatInt(old.MotherID, 10), strconv.FormatInt(new.MotherID, 10), true)
	}
	f, t, c = scmp(old.ClanType, new.ClanType)
	out = appendChange(out, "族类", f, t, c)
	f, t, c = scmp(old.BirthGregorian, new.BirthGregorian)
	out = appendChange(out, "出生公历", f, t, c)
	f, t, c = scmp(old.BirthLunar, new.BirthLunar)
	out = appendChange(out, "出生农历", f, t, c)
	f, t, c = scmp(old.BirthEra, new.BirthEra)
	out = appendChange(out, "出生年号", f, t, c)
	f, t, c = scmp(old.DeathGregorian, new.DeathGregorian)
	out = appendChange(out, "逝世公历", f, t, c)
	f, t, c = scmp(old.DeathLunar, new.DeathLunar)
	out = appendChange(out, "逝世农历", f, t, c)
	f, t, c = scmp(old.DeathEra, new.DeathEra)
	out = appendChange(out, "逝世年号", f, t, c)
	f, t, c = icmp(old.Lifespan, new.Lifespan)
	out = appendChange(out, "享年", f, t, c)
	f, t, c = scmp(old.NativePlace, new.NativePlace)
	out = appendChange(out, "籍贯", f, t, c)
	f, t, c = scmp(old.CurrentPlace, new.CurrentPlace)
	out = appendChange(out, "现居地", f, t, c)
	f, t, c = scmp(old.Title, new.Title)
	out = appendChange(out, "头衔", f, t, c)
	f, t, c = scmp(old.Occupation, new.Occupation)
	out = appendChange(out, "职业", f, t, c)
	f, t, c = scmp(old.Education, new.Education)
	out = appendChange(out, "学历", f, t, c)
	f, t, c = scmp(old.Biography, new.Biography)
	out = appendChange(out, "生平", f, t, c)
	f, t, c = scmp(old.BurialPlace, new.BurialPlace)
	out = appendChange(out, "葬地", f, t, c)
	f, t, c = scmp(old.MarriedOut, new.MarriedOut)
	out = appendChange(out, "出嫁", f, t, c)
	f, t, c = scmp(old.FormerName, new.FormerName)
	out = appendChange(out, "曾用名", f, t, c)
	return out
}

// spouseChanges 配偶：old 与 new 的字段差异
func spouseChanges(old, new *Spouse) []FieldChange {
	var out []FieldChange
	f, t, c := scmp(old.Surname, new.Surname)
	out = appendChange(out, "姓氏", f, t, c)
	f, t, c = scmp(old.GivenName, new.GivenName)
	out = appendChange(out, "名", f, t, c)
	f, t, c = scmp(old.Type, new.Type)
	out = appendChange(out, "类型", f, t, c)
	f, t, c = scmp(old.Family, new.Family)
	out = appendChange(out, "家世", f, t, c)
	f, t, c = scmp(old.BirthGregorian, new.BirthGregorian)
	out = appendChange(out, "出生公历", f, t, c)
	f, t, c = scmp(old.BirthLunar, new.BirthLunar)
	out = appendChange(out, "出生农历", f, t, c)
	f, t, c = scmp(old.DeathGregorian, new.DeathGregorian)
	out = appendChange(out, "逝世公历", f, t, c)
	f, t, c = scmp(old.DeathLunar, new.DeathLunar)
	out = appendChange(out, "逝世农历", f, t, c)
	f, t, c = scmp(old.Occupation, new.Occupation)
	out = appendChange(out, "职业", f, t, c)
	f, t, c = scmp(old.Education, new.Education)
	out = appendChange(out, "学历", f, t, c)
	f, t, c = scmp(old.Biography, new.Biography)
	out = appendChange(out, "生平", f, t, c)
	f, t, c = scmp(old.Remark, new.Remark)
	out = appendChange(out, "备注", f, t, c)
	return out
}

// eventChanges 大事记字段差异
func eventChanges(old, new *Event) []FieldChange {
	var out []FieldChange
	f, t, c := scmp(old.Year, new.Year)
	out = appendChange(out, "年份", f, t, c)
	f, t, c = scmp(old.Title, new.Title)
	out = appendChange(out, "标题", f, t, c)
	f, t, c = scmp(old.Content, new.Content)
	out = appendChange(out, "详情", f, t, c)
	return out
}

// articleChanges 文章字段差异
func articleChanges(old, new *Article) []FieldChange {
	var out []FieldChange
	f, t, c := scmp(old.Category, new.Category)
	out = appendChange(out, "分类", f, t, c)
	f, t, c = scmp(old.Title, new.Title)
	out = appendChange(out, "标题", f, t, c)
	f, t, c = scmp(old.Author, new.Author)
	out = appendChange(out, "作者", f, t, c)
	f, t, c = scmp(old.Year, new.Year)
	out = appendChange(out, "年代", f, t, c)
	f, t, c = scmp(old.Content, new.Content)
	out = appendChange(out, "内容", f, t, c)
	return out
}

// bioPersonTypeText 人员类型中文
func bioPersonTypeText(t string) string {
	switch t {
	case "member":
		return "族内成员"
	case "spouse":
		return "配偶"
	case "other":
		return "其他人员"
	}
	return t
}

// bioLifeText 生/殁展示文本
func bioLifeText(era, lunar, greg string) string {
	for _, s := range []string{era, lunar, greg} {
		if s != "" {
			return s
		}
	}
	return ""
}

// biographyChanges 名人传记字段差异
func biographyChanges(old, new *Biography) []FieldChange {
	var out []FieldChange
	add := func(label, a, b string) {
		f, t, c := scmp(a, b)
		out = appendChange(out, label, f, t, c)
	}
	add("人员类型", bioPersonTypeText(old.PersonType), bioPersonTypeText(new.PersonType))
	add("姓名", old.Name, new.Name)
	add("字", old.Zi, new.Zi)
	add("号", old.Hao, new.Hao)
	add("性别", old.Gender, new.Gender)
	add("生", bioLifeText(old.BirthEra, old.BirthLunar, old.BirthGregorian), bioLifeText(new.BirthEra, new.BirthLunar, new.BirthGregorian))
	add("殁", bioLifeText(old.DeathEra, old.DeathLunar, old.DeathGregorian), bioLifeText(new.DeathEra, new.DeathLunar, new.DeathGregorian))
	add("籍贯", old.NativePlace, new.NativePlace)
	add("现居", old.CurrentPlace, new.CurrentPlace)
	add("身份", old.Title, new.Title)
	add("职业", old.Occupation, new.Occupation)
	add("荣誉", old.Honors, new.Honors)
	add("摘要", old.Summary, new.Summary)
	add("正文", old.Content, new.Content)
	add("入选理由", old.Reason, new.Reason)
	return out
}

// cemeteryChanges 祠墓字段差异
func cemeteryChanges(old, new *Cemetery) []FieldChange {
	var out []FieldChange
	f, t, c := scmp(old.Name, new.Name)
	out = appendChange(out, "名称", f, t, c)
	f, t, c = scmp(old.Location, new.Location)
	out = appendChange(out, "位置", f, t, c)
	f, t, c = scmp(old.Description, new.Description)
	out = appendChange(out, "描述", f, t, c)
	f, t, c = scmp(old.Photo, new.Photo)
	out = appendChange(out, "照片", f, t, c)
	return out
}

// generationWordChanges 字辈派语字段差异
func generationWordChanges(old, new *GenerationWord) []FieldChange {
	var out []FieldChange
	f, t, c := scmp(old.Branch, new.Branch)
	out = appendChange(out, "支系", f, t, c)
	f, t, c = scmp(old.Poem, new.Poem)
	out = appendChange(out, "派语诗文", f, t, c)
	f, t, c = icmp(old.StartGeneration, new.StartGeneration)
	out = appendChange(out, "起始世代", f, t, c)
	return out
}

// computeChanges 根据草稿计算「查看修改」的字段变更列表。
// 返回 nil 表示该草稿无法逐字段对比（前端另行展示概要）。
func computeChanges(db *sql.DB, entityType, action string, entityID int64, payload string) []FieldChange {
	// 删除：无字段级对比，由前端展示「删除该记录」
	if action == "delete" {
		return nil
	}
	// 改名：单独展示「名 旧→新」
	if action == "rename" {
		var req struct {
			GivenName string `json:"given_name"`
		}
		if json.Unmarshal([]byte(payload), &req) == nil {
			oldName := ""
			if entityType == "member" {
				if m, err := fetchMemberRow(db, entityID); err == nil && m != nil {
					oldName = m.Name
				}
			}
			from := oldName
			if from == "" {
				from = "原名字"
			}
			return []FieldChange{{Field: "名", From: from, To: val(req.GivenName)}}
		}
		return nil
	}

	switch entityType {
	case "member":
		var newM Member
		if json.Unmarshal([]byte(payload), &newM) != nil {
			return nil
		}
		if action == "create" {
			return memberCreateChanges(&newM)
		}
		oldM, _ := fetchMemberRow(db, entityID)
		if oldM == nil {
			oldM = &Member{}
		}
		return memberChanges(oldM, &newM)

	case "spouse":
		var newS Spouse
		if json.Unmarshal([]byte(payload), &newS) != nil {
			return nil
		}
		if action == "create" {
			return spouseCreateChanges(&newS)
		}
		oldS, _ := fetchSpouseRow(db, entityID)
		if oldS == nil {
			oldS = &Spouse{}
		}
		return spouseChanges(oldS, &newS)

	case "event":
		var newE Event
		if json.Unmarshal([]byte(payload), &newE) != nil {
			return nil
		}
		if action == "create" {
			return eventChanges(&Event{}, &newE)
		}
		var oldE Event
		_ = fetchEventRow(db, entityID, &oldE)
		return eventChanges(&oldE, &newE)

	case "article":
		var newA Article
		if json.Unmarshal([]byte(payload), &newA) != nil {
			return nil
		}
		if action == "create" {
			return articleChanges(&Article{}, &newA)
		}
		var oldA Article
		_ = fetchArticleRow(db, entityID, &oldA)
		return articleChanges(&oldA, &newA)

	case "biography":
		var newB Biography
		if json.Unmarshal([]byte(payload), &newB) != nil {
			return nil
		}
		if action == "create" {
			return biographyChanges(&Biography{}, &newB)
		}
		var oldB Biography
		_ = fetchBiographyRow(db, entityID, &oldB)
		return biographyChanges(&oldB, &newB)

	case "cemetery":
		var newC Cemetery
		if json.Unmarshal([]byte(payload), &newC) != nil {
			return nil
		}
		if action == "create" {
			return cemeteryChanges(&Cemetery{}, &newC)
		}
		var oldC Cemetery
		_ = fetchCemeteryRow(db, entityID, &oldC)
		return cemeteryChanges(&oldC, &newC)

	case "generation_word":
		var newG GenerationWord
		if json.Unmarshal([]byte(payload), &newG) != nil {
			return nil
		}
		if action == "create" {
			return generationWordCreateChanges(&newG)
		}
		var oldG GenerationWord
		_ = fetchGenerationWordRow(db, entityID, &oldG)
		return generationWordChanges(&oldG, &newG)

	case "member_bio":
		var newB BioEntry
		if json.Unmarshal([]byte(payload), &newB) != nil {
			return nil
		}
		if action == "create" {
			return bioCreateChanges(newB.Title, newB.Content, newB.SortOrder)
		}
		var oldT, oldC string
		var oldSort int
		_ = db.QueryRow(`SELECT COALESCE(title,''), COALESCE(content,''), COALESCE(sort_order,0) FROM member_bio_entries WHERE id = ?`, entityID).
			Scan(&oldT, &oldC, &oldSort)
		return bioChanges(oldT, oldC, oldSort, newB.Title, newB.Content, newB.SortOrder)

	case "spouse_bio":
		var newSB SpouseBioEntry
		if json.Unmarshal([]byte(payload), &newSB) != nil {
			return nil
		}
		if action == "create" {
			return bioCreateChanges(newSB.Title, newSB.Content, newSB.SortOrder)
		}
		var oldT, oldC string
		var oldSort int
		_ = db.QueryRow(`SELECT COALESCE(title,''), COALESCE(content,''), COALESCE(sort_order,0) FROM spouse_bio_entries WHERE id = ?`, entityID).
			Scan(&oldT, &oldC, &oldSort)
		return bioChanges(oldT, oldC, oldSort, newSB.Title, newSB.Content, newSB.SortOrder)
	}
	return nil
}

// fetchEventRow 读取大事记单行
func fetchEventRow(db *sql.DB, id int64, out *Event) error {
	return db.QueryRow(`SELECT id, COALESCE(year,''), COALESCE(title,''), COALESCE(content,'') FROM events WHERE id = ?`, id).
		Scan(&out.ID, &out.Year, &out.Title, &out.Content)
}

// fetchArticleRow 读取文章单行
func fetchArticleRow(db *sql.DB, id int64, out *Article) error {
	return db.QueryRow(`SELECT id, COALESCE(category,''), COALESCE(title,''), COALESCE(author,''), COALESCE(year,''), COALESCE(content,'') FROM articles WHERE id = ?`, id).
		Scan(&out.ID, &out.Category, &out.Title, &out.Author, &out.Year, &out.Content)
}

// fetchBiographyRow 读取传记单行
func fetchBiographyRow(db *sql.DB, id int64, out *Biography) error {
	return db.QueryRow(`SELECT id, COALESCE(person_type,''), COALESCE(member_id,0), COALESCE(spouse_id,0),
		COALESCE(name,''), COALESCE(zi,''), COALESCE(hao,''), COALESCE(gender,''),
		COALESCE(birth_gregorian,''), COALESCE(birth_lunar,''), COALESCE(birth_era,''),
		COALESCE(death_gregorian,''), COALESCE(death_lunar,''), COALESCE(death_era,''),
		COALESCE(native_place,''), COALESCE(current_place,''), COALESCE(title,''), COALESCE(occupation,''),
		COALESCE(honors,''), COALESCE(portrait,''), COALESCE(summary,''), COALESCE(content,''),
		COALESCE(reason,''), COALESCE(sort_year,0), COALESCE(sort_order,0), COALESCE(public,1)
		FROM biographies WHERE id = ?`, id).
		Scan(&out.ID, &out.PersonType, &out.MemberID, &out.SpouseID, &out.Name, &out.Zi, &out.Hao, &out.Gender,
			&out.BirthGregorian, &out.BirthLunar, &out.BirthEra, &out.DeathGregorian, &out.DeathLunar, &out.DeathEra,
			&out.NativePlace, &out.CurrentPlace, &out.Title, &out.Occupation, &out.Honors, &out.Portrait,
			&out.Summary, &out.Content, &out.Reason, &out.SortYear, &out.SortOrder, &out.Public)
}

// fetchCemeteryRow 读取祠墓单行
func fetchCemeteryRow(db *sql.DB, id int64, out *Cemetery) error {
	return db.QueryRow(`SELECT id, COALESCE(name,''), COALESCE(location,''), COALESCE(description,''), COALESCE(photo,'') FROM cemeteries WHERE id = ?`, id).
		Scan(&out.ID, &out.Name, &out.Location, &out.Description, &out.Photo)
}

// fetchGenerationWordRow 读取字辈派语单行
func fetchGenerationWordRow(db *sql.DB, id int64, out *GenerationWord) error {
	return db.QueryRow(`SELECT id, COALESCE(branch,''), COALESCE(poem,''), COALESCE(words_array,''), start_generation FROM generation_words WHERE id = ?`, id).
		Scan(&out.ID, &out.Branch, &out.Poem, &out.WordsArray, &out.StartGeneration)
}

// entitySummary 用于删除草稿展示：返回被删除对象的一句话描述
func entitySummary(db *sql.DB, entityType string, entityID int64) string {
	switch entityType {
	case "member":
		if m, err := fetchMemberRow(db, entityID); err == nil && m != nil && m.Name != "" {
			return fmt.Sprintf("成员「%s」", m.Name)
		}
		return fmt.Sprintf("成员 #%d", entityID)
	case "spouse":
		var name string
		_ = db.QueryRow(`SELECT COALESCE(name,'') FROM spouses WHERE id = ?`, entityID).Scan(&name)
		if name != "" {
			return fmt.Sprintf("配偶「%s」", name)
		}
		return fmt.Sprintf("配偶 #%d", entityID)
	case "event":
		var t string
		_ = db.QueryRow(`SELECT COALESCE(title,'') FROM events WHERE id = ?`, entityID).Scan(&t)
		return fmt.Sprintf("大事记「%s」", t)
	case "article":
		var t string
		_ = db.QueryRow(`SELECT COALESCE(title,'') FROM articles WHERE id = ?`, entityID).Scan(&t)
		return fmt.Sprintf("文章「%s」", t)
	case "biography":
		var n string
		_ = db.QueryRow(`SELECT COALESCE(name,'') FROM biographies WHERE id = ?`, entityID).Scan(&n)
		return fmt.Sprintf("传记「%s」", n)
	case "cemetery":
		var n string
		_ = db.QueryRow(`SELECT COALESCE(name,'') FROM cemeteries WHERE id = ?`, entityID).Scan(&n)
		return fmt.Sprintf("祠墓「%s」", n)
	case "generation_word":
		var b string
		_ = db.QueryRow(`SELECT COALESCE(branch,'') FROM generation_words WHERE id = ?`, entityID).Scan(&b)
		if b == "" {
			b = "总谱"
		}
		return fmt.Sprintf("字辈派语「%s」", b)
	case "member_bio":
		var t string
		_ = db.QueryRow(`SELECT COALESCE(title,'') FROM member_bio_entries WHERE id = ?`, entityID).Scan(&t)
		if t != "" {
			return fmt.Sprintf("成员生平条目「%s」", t)
		}
		return fmt.Sprintf("成员生平条目 #%d", entityID)
	case "spouse_bio":
		var t string
		_ = db.QueryRow(`SELECT COALESCE(title,'') FROM spouse_bio_entries WHERE id = ?`, entityID).Scan(&t)
		if t != "" {
			return fmt.Sprintf("配偶生平条目「%s」", t)
		}
		return fmt.Sprintf("配偶生平条目 #%d", entityID)
	}
	return fmt.Sprintf("#%d", entityID)
}
