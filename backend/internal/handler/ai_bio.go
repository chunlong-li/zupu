package handler

import (
	"database/sql"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// genBioPrompt AI 辅助生成小传的系统指令（素材仅为后端查库得到的结构化事实）
const genBioPrompt = `你是「族谱修谱助手」，正在依据系统提供的【结构化事实】撰写一篇人物小传。
【素材范围】只能使用【结构化事实】中给出的信息，以及【补充说明】中的行文要求；其它一律视为「待考」。
【铁律】
1. 严禁编造：不得虚构或推测任何未提供的人名、地名、字号、官职、年月日、学历、职业、亲属关系；
2. 标注「待考」的项不得写入正文，也不得用「大约」「可能」「据说」等猜测性说法补足；
3. 文中形如 __FACT_n__、__NOTE_n__ 的占位符是系统保护的日期/纪年，凡需写入必须原样照抄，一字不改、不删、不增、不换算；
4. 不得重复使用同一占位符，也不得自行编造占位符；
5. 采用生平叙述体、第三人称，庄重平实，避免空洞评价与溢美之词；
6. 默认篇幅 300~600 字，可读性优先。

【输出要求】只输出一个 JSON 对象，不要输出任何解释或其他文字，格式如下：
{"title":"小传标题","content":"小传正文"}`

// GenerateBio 依据库中结构化资料辅助生成小传（理事长及以上，需已启用）
// POST /api/ai/generate-bio
// 入参：{"member_id":1,"spouse_id":0,"style":"传记体|简洁|文言","length":"短|中|长","level":50,"degree":50,"extra_notes":""}
// 返回：{"title":"...","content":"...","protected":bool}（不回写数据库，由前端走既有生平条目保存流程）
func (h *AIHandler) GenerateBio(c *gin.Context) {
	s := h.aiSettings()
	if !s.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "修谱助手未启用，请先到系统设置中开启并配置模型"})
		return
	}

	var body struct {
		MemberID   int64  `json:"member_id"`
		SpouseID   int64  `json:"spouse_id"`
		Style      string `json:"style"`
		Length     string `json:"length"`
		Level      *int   `json:"level"`
		Degree     *int   `json:"degree"`
		ExtraNotes string `json:"extra_notes"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if body.MemberID <= 0 && body.SpouseID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少成员 ID"})
		return
	}

	var facts, defTitle string
	var err error
	if body.SpouseID > 0 {
		facts, defTitle, err = h.spouseBioFacts(body.SpouseID)
	} else {
		facts, defTitle, err = h.memberBioFacts(body.MemberID)
	}
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到该成员"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取成员资料失败"})
		return
	}

	degree := 50
	if body.Degree != nil {
		degree = *body.Degree
	}
	level := 50
	if body.Level != nil {
		level = *body.Level
	}

	// 发送前掩码：结构化事实与补充说明里的日期都换成占位符，模型看不到原文就改不了
	maskedFacts, factDates := maskDates("FACT", facts)
	extraNotes := strings.TrimSpace(body.ExtraNotes)
	maskedNotes, noteDates := maskDates("NOTE", extraNotes)

	prompt := genBioPrompt +
		"\n\n【文体】" + genBioStyleLine(body.Style) +
		"\n【篇幅】" + genBioLengthLine(body.Length) +
		"\n【叙述尺度】" + genBioLevelLine(level)

	userPrompt := "【结构化事实】\n" + maskedFacts
	if maskedNotes != "" {
		userPrompt += "\n\n【补充说明】\n" + maskedNotes
	}

	messages := []map[string]string{
		{"role": "system", "content": aiSystemPrompt},
		{"role": "user", "content": prompt + "\n\n" + userPrompt},
	}

	// 小传篇幅长，输出上限至少 2048（不受现有对话/润色端点 1024 限制影响）
	maxTokens := s.MaxTokens
	if maxTokens < defaultAIMaxTokens {
		maxTokens = defaultAIMaxTokens
	}

	reply, err := callModelLimit(s, messages, degreeTemp(degree), maxTokens)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	title, content := parseGeneratedBio(reply, defTitle)
	protected := false

	// 保护①：占位符校验（允许遗漏未写入的事实，但不允许臆造/越界；校验收到的实际引用）
	if !maskRefsValid(title, "FACT", len(factDates)) || !maskRefsValid(content, "FACT", len(factDates)) ||
		!maskRefsValid(title, "NOTE", len(noteDates)) || !maskRefsValid(content, "NOTE", len(noteDates)) {
		protected = true
	}

	// 保护②：还原占位符为真实日期，并清理可能残留的占位符
	title = unmaskDates(title, "FACT", factDates)
	title = unmaskDates(title, "NOTE", noteDates)
	content = unmaskDates(content, "FACT", factDates)
	content = unmaskDates(content, "NOTE", noteDates)
	if v := clearMaskResidue(title); v != title {
		title = v
		protected = true
	}
	if v := clearMaskResidue(content); v != content {
		content = v
		protected = true
	}

	// 保护③：年份兜底 —— 正文不得出现「结构化事实」之外的年份/纪年（防编造日期）
	known := facts
	if extraNotes != "" {
		known += "\n" + extraNotes
	}
	if hasExtraYears(known, title) || hasExtraYears(known, content) {
		protected = true
	}

	if strings.TrimSpace(title) == "" {
		title = defTitle
	}
	if strings.TrimSpace(content) == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "模型未返回有效内容"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"title": title, "content": content, "protected": protected})
}

// genBioStyleLine 文体指令（前端可传中文或留空）
func genBioStyleLine(style string) string {
	switch strings.TrimSpace(style) {
	case "文言":
		return "文言：使用浅近文言（近代谱牒笔法），句法简练、少用长句，不用生僻典故。"
	case "简洁":
		return "简洁：以简明书面语直述生平，少修饰、少铺陈，事实清楚即可。"
	default:
		return "传记体：现代书面语，庄重典雅，按家世—生卒—行迹—德业—亲属的脉络自然成文。"
	}
}

// genBioLengthLine 篇幅指令（映射字数区间，默认 300~600 字）
func genBioLengthLine(length string) string {
	switch strings.TrimSpace(length) {
	case "短":
		return "短：200~300 字，只保留最主要的事实。"
	case "长":
		return "长：600~1000 字，可充分展开行迹与德业，但不得新增事实。"
	default:
		return "中：300~600 字。"
	}
}

// genBioLevelLine 叙述尺度指令（等级越低越保守，越高越允许铺陈，但都不许新增事实）
func genBioLevelLine(level int) string {
	switch {
	case level <= 0:
		return "最严格：只陈述事实，逐项成句，不加任何铺陈、评价与连接性想象。"
	case level <= 25:
		return "严格：以事实陈述为主，仅用必要的连接语使文句通顺。"
	case level <= 50:
		return "适中：事实为主，可适度组织语言与过渡，使行文连贯自然。"
	case level <= 75:
		return "较宽松：可适度文学化铺陈（环境、时序的概括性描述），但不得新增任何具体事实。"
	default:
		return "宽松：可着力文采与结构，仍不得新增任何具体事实。"
	}
}

// parseGeneratedBio 解析模型返回的 {title,content}；解析失败时把全文当正文，标题用兜底标题
func parseGeneratedBio(reply, fallbackTitle string) (string, string) {
	title, content := parseBioPolish(reply, fallbackTitle, "")
	if strings.TrimSpace(content) == "" {
		content = strings.TrimSpace(reply)
	}
	if strings.TrimSpace(title) == "" {
		title = fallbackTitle
	}
	return strings.TrimSpace(title), strings.TrimSpace(content)
}

// maskRefsValid 校验文本中出现的占位符是否都在合法范围内（允许遗漏，不允许越界/臆造）
func maskRefsValid(s, prefix string, n int) bool {
	if n == 0 {
		return !strings.Contains(s, "__"+prefix+"_")
	}
	re := regexp.MustCompile(`__` + prefix + `_(\d+)__`)
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		idx, err := strconv.Atoi(m[1])
		if err != nil || idx < 1 || idx > n {
			return false
		}
	}
	return true
}

// maskResidueRe 匹配残留的占位符（含被模型写坏的形态，如 __FACT_3_）
var maskResidueRe = regexp.MustCompile(`__[A-Za-z]+_\d+_?`)

// clearMaskResidue 清除还原后仍残留的占位符，避免把系统标记写进谱文
func clearMaskResidue(s string) string {
	return maskResidueRe.ReplaceAllString(s, "")
}

// hasExtraYears 判断 out 中是否出现了 orig 之外的 4 位及以上数字（视为编造年份）
func hasExtraYears(orig, out string) bool {
	known := map[string]bool{}
	for _, y := range yearRe.FindAllString(orig, -1) {
		known[y] = true
	}
	for _, y := range yearRe.FindAllString(out, -1) {
		if !known[y] {
			return true
		}
	}
	return false
}

var yearRe = regexp.MustCompile(`\d{4,}`)

// memberBioFacts 读取成员的结构化事实，拼成「标签：值」文本（缺失写「待考」）
func (h *AIHandler) memberBioFacts(id int64) (string, string, error) {
	var (
		name, zi, hao, genWord, branch, rank                  string
		gender, generation, lifespan                          int
		birthG, birthL, birthZ, birthE                        string
		deathG, deathL, deathZ, deathE                        string
		native, current, title, occupation, education, burial string
		marriedOut, biography                                 string
		fatherID, motherID, motherSpouseID                    int64
	)
	err := h.DB.QueryRow(`SELECT COALESCE(name,''), COALESCE(zi,''), COALESCE(hao,''), COALESCE(generation_word,''),
			COALESCE(generation,0), COALESCE(gender,1), COALESCE(branch,''), COALESCE(rank,''),
			COALESCE(birth_gregorian,''), COALESCE(birth_lunar,''), COALESCE(birth_ganzhi,''), COALESCE(birth_era,''),
			COALESCE(death_gregorian,''), COALESCE(death_lunar,''), COALESCE(death_ganzhi,''), COALESCE(death_era,''),
			COALESCE(lifespan,0), COALESCE(native_place,''), COALESCE(current_place,''), COALESCE(title,''),
			COALESCE(occupation,''), COALESCE(education,''), COALESCE(burial_place,''), COALESCE(married_out,''),
			COALESCE(father_id,0), COALESCE(mother_id,0), COALESCE(mother_spouse_id,0), COALESCE(biography,'')
		FROM members WHERE id = ?`, id).
		Scan(&name, &zi, &hao, &genWord, &generation, &gender, &branch, &rank,
			&birthG, &birthL, &birthZ, &birthE, &deathG, &deathL, &deathZ, &deathE,
			&lifespan, &native, &current, &title, &occupation, &education, &burial, &marriedOut,
			&fatherID, &motherID, &motherSpouseID, &biography)
	if err != nil {
		return "", "", err
	}

	lines := []string{
		factLine("姓名", name),
		factLine("字", zi),
		factLine("号", hao),
		factLine("字辈", genWord),
		factLine("世代", genText(generation)),
		"性别：" + genderText(gender),
		factLine("房支", branch),
		factLine("排行", rank),
		factLine("出生", dateFact(birthG, birthL, birthZ, birthE)),
		factLine("去世", dateFact(deathG, deathL, deathZ, deathE)),
		factLine("享年", lifespanText(lifespan)),
		factLine("籍贯", native),
		factLine("现居", current),
		factLine("功名/职称", title),
		factLine("职业", occupation),
		factLine("学历", education),
		factLine("葬地", burial),
		factLine("适/出嫁出", marriedOut),
		factLine("父亲", h.personName(fatherID)),
		factLine("母亲", h.personName(motherIDOrSpouse(motherID, motherSpouseID))),
		factLine("配偶", strings.Join(h.spouseNamesOf(id), "、")),
	}
	if bio := strings.TrimSpace(biography); bio != "" {
		lines = append(lines, "备注（已录，可整理引用）："+clipText(bio, 1200))
	}
	if entries := h.bioEntriesText(id); entries != "" {
		lines = append(lines, "已有生平条目（可整理引用）：\n"+clipText(entries, 1200))
	}
	return strings.Join(lines, "\n"), name + "小传", nil
}

// spouseBioFacts 读取配偶的结构化事实（家世、生卒、职业等）
func (h *AIHandler) spouseBioFacts(id int64) (string, string, error) {
	var (
		memberID                         int64
		name, typ, family, remark        string
		birthG, birthL, birthZ, birthE   string
		deathG, deathL, deathZ, deathE   string
		occupation, education, biography string
	)
	err := h.DB.QueryRow(`SELECT COALESCE(member_id,0), COALESCE(name,''), COALESCE(type,''), COALESCE(family,''),
			COALESCE(birth_gregorian,''), COALESCE(birth_lunar,''), COALESCE(birth_ganzhi,''), COALESCE(birth_era,''),
			COALESCE(death_gregorian,''), COALESCE(death_lunar,''), COALESCE(death_ganzhi,''), COALESCE(death_era,''),
			COALESCE(occupation,''), COALESCE(education,''), COALESCE(remark,''), COALESCE(biography,'')
		FROM spouses WHERE id = ?`, id).
		Scan(&memberID, &name, &typ, &family, &birthG, &birthL, &birthZ, &birthE,
			&deathG, &deathL, &deathZ, &deathE, &occupation, &education, &remark, &biography)
	if err != nil {
		return "", "", err
	}

	// 从所属成员补齐世代/房支/籍贯
	var generation int
	var branch, native, memberName string
	if memberID > 0 {
		h.DB.QueryRow(`SELECT COALESCE(generation,0), COALESCE(branch,''), COALESCE(native_place,''), COALESCE(name,'')
			FROM members WHERE id = ?`, memberID).Scan(&generation, &branch, &native, &memberName)
	}

	lines := []string{
		factLine("姓名", name),
		factLine("身份", spouseTypeText(typ)),
		factLine("娘家", family),
		factLine("世代", genText(generation)),
		factLine("房支", branch),
		factLine("籍贯", native),
		factLine("出生", dateFact(birthG, birthL, birthZ, birthE)),
		factLine("去世", dateFact(deathG, deathL, deathZ, deathE)),
		factLine("职业", occupation),
		factLine("学历", education),
		factLine("备注", remark),
		factLine("夫/妻", memberName),
	}
	if bio := strings.TrimSpace(biography); bio != "" {
		lines = append(lines, "备注（已录，可整理引用）："+clipText(bio, 1200))
	}
	return strings.Join(lines, "\n"), name + "小传", nil
}

// factLine 组装「标签：值」，值为空时写「待考」
func factLine(label, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "待考"
	}
	return label + "：" + value
}

// dateFact 组装公历/农历/干支/年号四项日期描述
func dateFact(gregorian, lunar, ganzhi, era string) string {
	parts := []string{}
	if v := strings.TrimSpace(gregorian); v != "" {
		parts = append(parts, "公历"+v)
	}
	if v := strings.TrimSpace(lunar); v != "" {
		parts = append(parts, "农历"+v)
	}
	if v := strings.TrimSpace(ganzhi); v != "" {
		parts = append(parts, "干支"+v)
	}
	if v := strings.TrimSpace(era); v != "" {
		parts = append(parts, "年号"+v)
	}
	return strings.Join(parts, "；")
}

// genText 世代文本（0 视为未知）
func genText(generation int) string {
	if generation <= 0 {
		return ""
	}
	return "第" + strconv.Itoa(generation) + "世"
}

// lifespanText 享年文本
func lifespanText(n int) string {
	if n <= 0 {
		return ""
	}
	return strconv.Itoa(n) + " 岁"
}

// spouseTypeText 配偶身份文本
func spouseTypeText(t string) string {
	t = strings.TrimSpace(t)
	if t == "" {
		return "配偶"
	}
	return t
}

// motherIDOrSpouse 母亲可能是「女性成员」也可能是「配偶」记录，取有效的一个
func motherIDOrSpouse(motherID, motherSpouseID int64) int64 {
	if motherID > 0 {
		return motherID
	}
	return motherSpouseID
}

// personName 按 id 取姓名（先查成员，再查配偶；查不到返回空串）
func (h *AIHandler) personName(id int64) string {
	if id <= 0 {
		return ""
	}
	var name string
	if err := h.DB.QueryRow(`SELECT COALESCE(name,'') FROM members WHERE id = ?`, id).Scan(&name); err == nil && strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	h.DB.QueryRow(`SELECT COALESCE(name,'') FROM spouses WHERE id = ?`, id).Scan(&name)
	return strings.TrimSpace(name)
}

// spouseNamesOf 取某成员的配偶姓名列表（正配在前）
func (h *AIHandler) spouseNamesOf(memberID int64) []string {
	rows, err := h.DB.Query(`SELECT COALESCE(name,'') FROM spouses WHERE member_id = ?
		ORDER BY CASE type WHEN '正配' THEN 1 WHEN '继配' THEN 2 WHEN '侧室' THEN 3 ELSE 4 END, id`, memberID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var n string
		if rows.Scan(&n) == nil && strings.TrimSpace(n) != "" {
			names = append(names, strings.TrimSpace(n))
		}
	}
	return names
}

// bioEntriesText 拼接成员已有的生平条目（供模型整理引用）
func (h *AIHandler) bioEntriesText(memberID int64) string {
	rows, err := h.DB.Query(`SELECT COALESCE(title,''), COALESCE(content,'') FROM member_bio_entries
		WHERE member_id = ? ORDER BY sort_order, id LIMIT 8`, memberID)
	if err != nil {
		return ""
	}
	defer rows.Close()
	parts := []string{}
	for rows.Next() {
		var t, ct string
		if rows.Scan(&t, &ct) != nil {
			continue
		}
		ct = strings.TrimSpace(ct)
		if ct == "" {
			continue
		}
		if t = strings.TrimSpace(t); t != "" {
			parts = append(parts, t+"："+ct)
		} else {
			parts = append(parts, ct)
		}
	}
	return strings.Join(parts, "\n")
}

// clipText 截断过长文本（按 rune 计，避免截坏多字节字符）
func clipText(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}

// ===== 名人传记：AI 生成传记（素材 = 传记表单资料 + 关联档案 + 生平条目）=====

// genBiographyPrompt AI 生成传记正文的系统指令
const genBiographyPrompt = `你是「族谱修谱助手」，正在依据系统提供的材料撰写一篇【名人传记】的正文。

【材料】由三部分组成：
· 【传记资料】——立传表单里已填写或已选择的信息（姓名字号、生卒、籍贯现居、身份职衔、荣誉、摘要、入选理由等）；
· 【档案资料】——若该传主关联了族内成员/配偶，则附上其族谱档案字段；
· 【生平条目】——该传主已录入的生平事迹条目（可能为空）。

写作要求：
① 只使用材料中出现的事实。材料里没有的人名、地名、年份、职衔、事迹一律不写；材料不足处宁可略写（如「早年事迹待考」），绝不编造、不推测、不补充。
② 可依据材料做合理的书面化组织（如：家世籍贯 → 生平经历 → 品德事迹 → 家族贡献 → 时人评价），但不得添加材料之外的具体事实。
③ 语言庄重典雅、第三人称书面语；不使用 Markdown 标记、不用小标题与编号，分段用自然段即可。
④ 材料中的时间一律沿用原文写法，不换算、不补充、不改动。
⑤ 只输出传记正文本身，不要写「正文」「传记」之类前缀，也不要输出解释性文字。`

// spouseBioEntriesText 读取配偶的生平条目（与 bioEntriesText 同构，表名不同）
func (h *AIHandler) spouseBioEntriesText(spouseID int64) string {
	rows, err := h.DB.Query(`SELECT COALESCE(title,''), COALESCE(content,'') FROM spouse_bio_entries
		WHERE spouse_id = ? ORDER BY sort_order, id LIMIT 8`, spouseID)
	if err != nil {
		return ""
	}
	defer rows.Close()
	parts := []string{}
	for rows.Next() {
		var t, ct string
		if rows.Scan(&t, &ct) != nil {
			continue
		}
		if ct = strings.TrimSpace(ct); ct == "" {
			continue
		}
		if t = strings.TrimSpace(t); t != "" {
			parts = append(parts, t+"："+ct)
		} else {
			parts = append(parts, ct)
		}
	}
	return strings.Join(parts, "\n")
}

// GenerateBiography POST /api/ai/generate-biography
// 名人传记「按资料 + 生平生成传记」：前端把当前表单值传上来（含未保存的编辑），
// 后端再补上关联成员/配偶的档案与其生平条目，一起作为素材。
func (h *AIHandler) GenerateBiography(c *gin.Context) {
	s := h.aiSettings()
	if !s.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "修谱助手未启用，请先到系统设置中开启并配置模型"})
		return
	}

	var body struct {
		MemberID      int64  `json:"member_id"`
		SpouseID      int64  `json:"spouse_id"`
		Name          string `json:"name"`
		Zi            string `json:"zi"`
		Hao           string `json:"hao"`
		Gender        string `json:"gender"`
		BirthEra      string `json:"birth_era"`
		BirthLunar    string `json:"birth_lunar"`
		BirthGreg     string `json:"birth_gregorian"`
		DeathEra      string `json:"death_era"`
		DeathLunar    string `json:"death_lunar"`
		DeathGreg     string `json:"death_gregorian"`
		NativePlace   string `json:"native_place"`
		CurrentPlace  string `json:"current_place"`
		Title         string `json:"title"`
		Occupation    string `json:"occupation"`
		Honors        string `json:"honors"`
		Summary       string `json:"summary"`
		Reason        string `json:"reason"`
		Style         string `json:"style"`
		Length        string `json:"length"`
		Level         *int   `json:"level"`
		Degree        *int   `json:"degree"`
		ExtraNotes    string `json:"extra_notes"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	// ① 传记资料（表单值）
	var lb strings.Builder
	addLine := func(label, v string) {
		if v = strings.TrimSpace(v); v != "" {
			lb.WriteString(factLine(label, v))
			lb.WriteString("\n")
		}
	}
	addLine("姓名", body.Name)
	addLine("字", body.Zi)
	addLine("号", body.Hao)
	addLine("性别", body.Gender)
	addLine("生（年号/干支）", body.BirthEra)
	addLine("生（农历）", body.BirthLunar)
	addLine("生（公历）", body.BirthGreg)
	addLine("卒（年号/干支）", body.DeathEra)
	addLine("卒（农历）", body.DeathLunar)
	addLine("卒（公历）", body.DeathGreg)
	addLine("籍贯", body.NativePlace)
	addLine("现居", body.CurrentPlace)
	addLine("身份/职衔", body.Title)
	addLine("职业", body.Occupation)
	addLine("荣誉", body.Honors)
	addLine("已有摘要", body.Summary)
	addLine("入选理由/事迹", body.Reason)

	facts := "【传记资料】\n" + lb.String()
	defTitle := strings.TrimSpace(body.Name)

	// ② 关联人的档案资料
	if body.SpouseID > 0 {
		if f, t, err := h.spouseBioFacts(body.SpouseID); err == nil {
			facts += "\n【档案资料】\n" + f
			if defTitle == "" {
				defTitle = t
			}
		}
	} else if body.MemberID > 0 {
		if f, t, err := h.memberBioFacts(body.MemberID); err == nil {
			facts += "\n【档案资料】\n" + f
			if defTitle == "" {
				defTitle = t
			}
		}
	}

	// ③ 生平条目
	if body.MemberID > 0 {
		if t := h.bioEntriesText(body.MemberID); t != "" {
			facts += "\n【生平条目】\n" + t
		}
	}
	if body.SpouseID > 0 {
		if t := h.spouseBioEntriesText(body.SpouseID); t != "" {
			facts += "\n【生平条目】\n" + t
		}
	}

	if strings.TrimSpace(lb.String()) == "" && body.MemberID <= 0 && body.SpouseID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先填写传记资料，或为该传主关联一位成员/配偶"})
		return
	}
	if defTitle == "" {
		defTitle = "传记"
	}

	degree, level := 50, 50
	if body.Degree != nil {
		degree = *body.Degree
	}
	if body.Level != nil {
		level = *body.Level
	}

	// 发送前掩码（与 generate-bio 同一套三道保护）
	maskedFacts, factDates := maskDates("FACT", facts)
	extraNotes := strings.TrimSpace(body.ExtraNotes)
	maskedNotes, noteDates := maskDates("NOTE", extraNotes)

	prompt := genBiographyPrompt +
		"\n\n【文体】" + genBioStyleLine(body.Style) +
		"\n【篇幅】" + genBioLengthLine(body.Length) +
		"\n【叙述尺度】" + genBioLevelLine(level)

	userPrompt := "【材料】\n" + maskedFacts
	if maskedNotes != "" {
		userPrompt += "\n\n【补充说明】\n" + maskedNotes
	}
	messages := []map[string]string{
		{"role": "system", "content": aiSystemPrompt},
		{"role": "user", "content": prompt + "\n\n" + userPrompt},
	}

	maxTokens := s.MaxTokens
	if maxTokens < defaultAIMaxTokens {
		maxTokens = defaultAIMaxTokens
	}
	reply, err := callModelLimit(s, messages, degreeTemp(degree), maxTokens)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	title, content := parseGeneratedBio(reply, defTitle)
	protected := false
	if !maskRefsValid(title, "FACT", len(factDates)) || !maskRefsValid(content, "FACT", len(factDates)) ||
		!maskRefsValid(title, "NOTE", len(noteDates)) || !maskRefsValid(content, "NOTE", len(noteDates)) {
		protected = true
	}
	title = unmaskDates(title, "FACT", factDates)
	title = unmaskDates(title, "NOTE", noteDates)
	content = unmaskDates(content, "FACT", factDates)
	content = unmaskDates(content, "NOTE", noteDates)
	if v := clearMaskResidue(title); v != title {
		title = v
		protected = true
	}
	if v := clearMaskResidue(content); v != content {
		content = v
		protected = true
	}
	known := facts
	if extraNotes != "" {
		known += "\n" + extraNotes
	}
	if hasExtraYears(known, content) {
		protected = true
	}
	if strings.TrimSpace(content) == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "模型未返回有效内容"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"title": title, "content": content, "protected": protected})
}
