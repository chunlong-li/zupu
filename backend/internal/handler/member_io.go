package handler

// 成员档案 Excel / CSV 导入导出
//
// 路由（见 internal/router/router.go）：
//   GET  /api/members/export                 导出（xlsx / csv，支持当前房支/筛选范围）
//   GET  /api/members/import/template        下载导入模板（xlsx / csv）
//   POST /api/members/import/preview         上传后预览校验（不写库）
//   POST /api/members/import                 只导入校验通过的行（事务）
//   POST /api/members/import/failures        下载失败清单 CSV
//
// 设计要点：
//  1. 列名全部中文，导出/模板/导入共用同一套列定义（ioCols），可「导出 → 改 → 导入」往返。
//  2. 日期统一写成 YYYY-MM-DD（只知年份时写 YYYY），农历列名带「农历」标记且原样保留。
//  3. 更新语义：**导入单元格为空 = 保持库中原值**（不提供清空），只有填了值才覆盖；
//     预览会列出「将更新」的具体字段。
//  4. 匹配顺序：ID 精确 → 姓名+生父姓名（无生父列时要求姓名唯一）；同文件内可引用
//     尚未入库的父母（按姓名+父名解析），按世代从高到低插入后再回填父子关系。
//  5. 事务策略：默认 mode=abort —— 全部通过校验的行在一个事务里写入，任何数据库错误整体
//     回滚；mode=skip —— 同事务内逐行跳过出错行，最后提交成功行（若一行都没成功则回滚）。
//     校验不通过的行永远不参与写入，逐行给出原因，可下载失败清单 CSV。

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const (
	importMaxBytes = 32 << 20 // 单个导入文件上限 32MB
	importMaxRows  = 20000    // 单次导入行数上限（防误传超大文件）
)

// MemberIOHandler 成员档案导入导出处理器
type MemberIOHandler struct {
	DB      *sql.DB
	EncKey  string
	DataDir string
}

// ioColumn 导入/导出一列
type ioColumn struct {
	Key      string   // 导入字段键；空 = 只读列（导入时忽略）
	Label    string   // 中文列名（导出/模板表头）
	Aliases  []string // 导入时可识别的别名
	Priv     string   // 需要的隐私权限项（空 = 不限）
	ReadOnly bool
}

// ioCols 全部列（导出顺序 / 模板顺序 / 导入识别）
var ioCols = []ioColumn{
	{Key: "id", Label: "ID", Aliases: []string{"成员ID", "编号", "id", "ID号"}},
	{Key: "name", Label: "姓名", Aliases: []string{"全名", "成员姓名"}},
	{Key: "surname", Label: "姓", Aliases: []string{"姓氏"}},
	{Key: "given_name", Label: "名", Aliases: []string{"名讳"}},
	{Key: "generation", Label: "世代", Aliases: []string{"第几世", "世次"}},
	{Key: "gender", Label: "性别", Aliases: []string{}},
	{Key: "branch", Label: "房支", Aliases: []string{"房份", "支系", "房支名称"}},
	{Key: "zi", Label: "字"},
	{Key: "hao", Label: "号"},
	{Key: "generation_word", Label: "字辈", Aliases: []string{"辈分"}},
	{Key: "rank", Label: "排行", Aliases: []string{"行第"}},
	{Key: "clan_type", Label: "宗法", Aliases: []string{"入嗣方式"}},
	{Key: "record_type", Label: "类型", Aliases: []string{"纪别"}},
	{Key: "father_id", Label: "生父ID", Aliases: []string{"父ID", "父亲ID"}},
	{Key: "father_name", Label: "生父姓名", Aliases: []string{"父姓名", "父亲姓名", "父名"}},
	{Key: "father_father_name", Label: "生父之父姓名", Aliases: []string{"祖父姓名", "父之父", "祖父"}},
	{Key: "mother_id", Label: "生母ID", Aliases: []string{"母ID", "母亲ID"}},
	{Key: "mother_name", Label: "生母姓名", Aliases: []string{"母姓名", "母亲姓名", "母名"}},
	{Key: "mother_father_name", Label: "生母之父姓名", Aliases: []string{"外祖父姓名", "母之父"}},
	{Key: "birth_calendar", Label: "出生历法", Aliases: []string{"历法"}},
	{Key: "birth_gregorian", Label: "出生日期(公历)", Aliases: []string{"出生日期", "出生公历", "生年"}},
	{Key: "birth_lunar", Label: "出生日期(农历)", Aliases: []string{"出生农历", "农历生日"}},
	{Key: "birth_ganzhi", Label: "出生干支", Aliases: []string{"生干支"}, ReadOnly: true},
	{Key: "birth_era", Label: "出生年号", Aliases: []string{"生年号"}},
	{Key: "death_gregorian", Label: "去世日期(公历)", Aliases: []string{"去世日期", "死亡日期", "卒年", "逝世日期"}},
	{Key: "death_lunar", Label: "去世日期(农历)", Aliases: []string{"去世农历", "卒农历"}},
	{Key: "death_ganzhi", Label: "去世干支", Aliases: []string{"卒干支"}, ReadOnly: true},
	{Key: "death_era", Label: "去世年号", Aliases: []string{"卒年号"}},
	{Key: "lifespan", Label: "享年", Aliases: []string{"年龄"}, ReadOnly: true},
	{Key: "native_place", Label: "籍贯", Aliases: []string{}},
	{Key: "current_place", Label: "现居地", Aliases: []string{"居住地", "现住址"}},
	{Key: "title", Label: "头衔", Aliases: []string{"称号"}},
	{Key: "occupation", Label: "职业", Aliases: []string{}},
	{Key: "education", Label: "学历", Aliases: []string{}},
	{Key: "biography", Label: "生平", Aliases: []string{"传记", "简历"}},
	{Key: "burial_place", Label: "葬地", Aliases: []string{"墓地", "墓葬"}},
	{Key: "former_name", Label: "曾用名", Aliases: []string{"原名"}},
	{Key: "married_out", Label: "出嫁", Aliases: []string{"适"}},
	{Key: "phone", Label: "电话", Aliases: []string{"手机号", "联系电话"}, Priv: "phone"},
	{Key: "no_heir", Label: "无嗣", Aliases: []string{}},
	{Key: "lost_contact", Label: "失联", Aliases: []string{"存殁未详"}},
	{Key: "spouse_name", Label: "配偶", Aliases: []string{"配偶姓名"}},
	{Key: "spouse_type", Label: "配偶类型", Aliases: []string{"配偶关系"}},
	{Key: "children_count", Label: "子女数", ReadOnly: true},
	{Key: "portrait", Label: "头像", ReadOnly: true},
	{Key: "dna_haploid", Label: "DNA单倍体", Aliases: []string{"单倍体", "Y-DNA"}, Priv: "dna", ReadOnly: true},
	{Key: "dna_diploid", Label: "DNA二倍体", Aliases: []string{"二倍体", "STR"}, Priv: "dna", ReadOnly: true},
	{Key: "dna_mito", Label: "DNA线粒体", Aliases: []string{"线粒体", "mtDNA"}, Priv: "dna", ReadOnly: true},
	{Key: "created_at", Label: "创建时间", ReadOnly: true},
	{Key: "updated_at", Label: "更新时间", ReadOnly: true},
}

func ioColumnByLabel(label string) (ioColumn, bool) {
	for _, c := range ioCols {
		if c.Label == label {
			return c, true
		}
	}
	return ioColumn{}, false
}

// ioColumnAllowed 该列在当前角色下是否可见
func ioColumnAllowed(c ioColumn, role string) bool {
	if c.Priv == "" {
		return true
	}
	return PrivAllowed(c.Priv, role)
}

// ---------------------------------------------------------------------------
// 导出
// ---------------------------------------------------------------------------

// exportMember 导出所需的成员全字段（含派生列）
type exportMember struct {
	Member
	CreatedAt        string
	UpdatedAt        string
	BranchName       string
	FatherName       string
	MotherName       string
	MotherSpouseName string
	FatherFatherName string
	MotherFatherName string
	SpouseName       string
	SpouseType       string
	ChildrenCount    int
}

// Export 导出成员档案（xlsx / csv）
// GET /api/members/export?format=xlsx&branch_id=&include_children=&kw=&scope=all
func (h *MemberIOHandler) Export(c *gin.Context) {
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "xlsx")))
	if format != "csv" {
		format = "xlsx"
	}
	role := c.GetString("role")

	list, err := h.queryExportRows(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	cols := make([]ioColumn, 0, len(ioCols))
	for _, col := range ioCols {
		if !ioColumnAllowed(col, role) {
			continue
		}
		cols = append(cols, col)
	}
	header := make([]string, len(cols))
	widths := make([]float64, len(cols))
	for i, col := range cols {
		header[i] = col.Label
		widths[i] = 12
		switch col.Label {
		case "ID":
			widths[i] = 8
		case "姓名", "姓", "名":
			widths[i] = 10
		case "世代", "性别", "字辈", "排行", "类型", "宗法", "无嗣", "失联", "享年", "子女数":
			widths[i] = 8
		case "生平":
			widths[i] = 42
		case "出生日期(公历)", "去世日期(公历)", "出生日期(农历)", "去世日期(农历)":
			widths[i] = 16
		case "籍贯", "现居地", "葬地", "曾用名", "出嫁", "房支":
			widths[i] = 16
		case "电话", "DNA单倍体", "DNA二倍体", "DNA线粒体":
			widths[i] = 18
		case "创建时间", "更新时间":
			widths[i] = 19
		}
	}

	// 家族姓氏（用于生成文件名）
	familySurname := ""
	_ = h.DB.QueryRow(`SELECT value FROM settings WHERE key='family_surname'`).Scan(&familySurname)
	baseName := strings.TrimSpace(familySurname) + "族谱-成员档案"
	if baseName == "族谱-成员档案" {
		baseName = "成员档案"
	}
	stamp := time.Now().Format("20060102_1504")

	if format == "csv" {
		var buf bytes.Buffer
		buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM：Excel 直接双击打开不乱码
		w := csv.NewWriter(&buf)
		_ = w.Write(header)
		for _, m := range list {
			row := make([]string, len(cols))
			for i, col := range cols {
				row[i] = exportCell(m, col.Key)
			}
			_ = w.Write(row)
		}
		w.Flush()
		h.attachment(c, baseName+"-"+stamp+".csv", "text/csv; charset=utf-8", buf.Bytes())
		return
	}

	rows := make([][]string, 0, len(list))
	for _, m := range list {
		row := make([]string, len(cols))
		for i, col := range cols {
			row[i] = exportCell(m, col.Key)
		}
		rows = append(rows, row)
	}
	var buf bytes.Buffer
	if err := writeXLSX(&buf, []xlsxSheet{{
		Name:   "成员档案",
		Header: header,
		Rows:   rows,
		Widths: widths,
	}}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 Excel 失败：" + err.Error()})
		return
	}
	h.attachment(c, baseName+"-"+stamp+".xlsx",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

// attachment 以附件形式返回字节流（中文文件名用 RFC 5987 编码）
func (h *MemberIOHandler) attachment(c *gin.Context, filename, contentType string, body []byte) {
	ascii := "members-" + time.Now().Format("20060102_1504") + filepath.Ext(filename)
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, ascii, url.PathEscape(filename)))
	c.Header("Content-Length", strconv.Itoa(len(body)))
	c.Header("Cache-Control", "no-store")
	_, _ = c.Writer.Write(body)
}

// queryExportRows 读取成员（支持房支范围 / 关键词 / 范围）
func (h *MemberIOHandler) queryExportRows(c *gin.Context) ([]exportMember, error) {
	q := `SELECT m.id, m.generation, COALESCE(m.branch,''), COALESCE(m.branch_id,0),
		COALESCE(m.surname,''), COALESCE(m.given_name,''), m.name, COALESCE(m.zi,''), COALESCE(m.hao,''),
		COALESCE(m.generation_word,''), COALESCE(m.rank,''), COALESCE(m.gender,1),
		COALESCE(m.father_id,0), COALESCE(m.mother_id,0), COALESCE(m.mother_spouse_id,0),
		COALESCE(m.clan_type,''), COALESCE(m.record_type,''), COALESCE(m.foster_father_id,0),
		COALESCE(m.birth_calendar,'solar'), COALESCE(m.birth_gregorian,''), COALESCE(m.birth_lunar,''),
		COALESCE(m.birth_ganzhi,''), COALESCE(m.birth_era,''),
		COALESCE(m.death_gregorian,''), COALESCE(m.death_lunar,''), COALESCE(m.death_ganzhi,''), COALESCE(m.death_era,''),
		COALESCE(m.lifespan,0), COALESCE(m.native_place,''), COALESCE(m.current_place,''), COALESCE(m.title,''),
		COALESCE(m.occupation,''), COALESCE(m.education,''), COALESCE(m.biography,''), COALESCE(m.burial_place,''),
		COALESCE(m.portrait,''), COALESCE(m.married_out,''), COALESCE(m.former_name,''), COALESCE(m.phone,''),
		COALESCE(m.no_heir,0), COALESCE(m.lost_contact,0),
		COALESCE(m.dna_haploid,''), COALESCE(m.dna_diploid,''), COALESCE(m.dna_mito,''),
		COALESCE(m.created_at,''), COALESCE(m.updated_at,'')
		FROM members m WHERE 1=1`
	args := []interface{}{}
	scope := strings.ToLower(strings.TrimSpace(c.Query("scope")))
	if scope != "all" {
		if cond, a := BranchScope(c, h.DB, "m.branch_id"); cond != "" {
			q += " AND " + cond
			args = append(args, a...)
		}
	}
	if kw := strings.TrimSpace(c.Query("kw")); kw != "" {
		like := "%" + kw + "%"
		q += ` AND (m.name LIKE ? OR m.zi LIKE ? OR m.hao LIKE ? OR m.generation_word LIKE ?
			OR m.native_place LIKE ? OR m.current_place LIKE ? OR m.former_name LIKE ?)`
		for i := 0; i < 7; i++ {
			args = append(args, like)
		}
	}
	if g := strings.TrimSpace(c.Query("generation")); g != "" {
		if n, err := strconv.Atoi(g); err == nil {
			q += " AND m.generation = ?"
			args = append(args, n)
		}
	}
	if g := strings.TrimSpace(c.Query("gender")); g != "" {
		if n, err := strconv.Atoi(g); err == nil && (n == 1 || n == 2) {
			q += " AND m.gender = ?"
			args = append(args, n)
		}
	}
	q += " ORDER BY m.generation, m.id"

	rows, err := h.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exportMember{}
	for rows.Next() {
		var m exportMember
		if err := rows.Scan(&m.ID, &m.Generation, &m.Branch, &m.BranchID, &m.Surname, &m.GivenName, &m.Name,
			&m.Zi, &m.Hao, &m.GenerationWord, &m.Rank, &m.Gender, &m.FatherID, &m.MotherID, &m.MotherSpouseID,
			&m.ClanType, &m.RecordType, &m.FosterFatherID, &m.BirthCalendar,
			&m.BirthGregorian, &m.BirthLunar, &m.BirthGanzhi, &m.BirthEra,
			&m.DeathGregorian, &m.DeathLunar, &m.DeathGanzhi, &m.DeathEra, &m.Lifespan,
			&m.NativePlace, &m.CurrentPlace, &m.Title, &m.Occupation, &m.Education, &m.Biography,
			&m.BurialPlace, &m.Portrait, &m.MarriedOut, &m.FormerName, &m.Phone,
			&m.NoHeir, &m.LostContact, &m.DNAHaploid, &m.DNADiploid, &m.DNAMito,
			&m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		// 解密敏感字段（手机号/DNA）
		_ = m.DecryptSensitive(h.EncKey)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 附加派生列：父母姓名、祖父母姓名、配偶、子女数
	branchNames := map[int64]string{}
	if brs, err := h.DB.Query(`SELECT id, name FROM branches`); err == nil {
		for brs.Next() {
			var id int64
			var name string
			if brs.Scan(&id, &name) == nil {
				branchNames[id] = name
			}
		}
		brs.Close()
	}
	lite := map[int64]*memberLite{}
	if all, err := loadMemberLite(h.DB); err == nil {
		lite = all
	}
	spouses := map[int64][]string{}
	spouseTypes := map[int64][]string{}
	spouseByID := map[int64]string{}
	if srs, err := h.DB.Query(`SELECT id, member_id, COALESCE(name,''), COALESCE(type,'') FROM spouses ORDER BY
		CASE type WHEN '正配' THEN 1 WHEN '继配' THEN 2 WHEN '侧室' THEN 3 ELSE 4 END, id`); err == nil {
		for srs.Next() {
			var sid, mid int64
			var name, typ string
			if srs.Scan(&sid, &mid, &name, &typ) == nil && name != "" {
				spouseByID[sid] = name
				spouses[mid] = append(spouses[mid], name)
				spouseTypes[mid] = append(spouseTypes[mid], typ)
			}
		}
		srs.Close()
	}
	children := map[int64]int{}
	if crs, err := h.DB.Query(`SELECT father_id, COUNT(*) FROM members WHERE father_id > 0 GROUP BY father_id`); err == nil {
		for crs.Next() {
			var fid int64
			var n int
			if crs.Scan(&fid, &n) == nil {
				children[fid] = n
			}
		}
		crs.Close()
	}
	for i := range out {
		m := &out[i]
		m.BranchName = branchNames[m.BranchID]
		m.FatherName = liteName(lite, m.FatherID)
		m.MotherName = liteName(lite, m.MotherID)
		m.MotherSpouseName = spouseByID[m.MotherSpouseID]
		if f := lite[m.FatherID]; f != nil {
			m.FatherFatherName = liteName(lite, f.FatherID)
		}
		if mo := lite[m.MotherID]; mo != nil {
			m.MotherFatherName = liteName(lite, mo.FatherID)
		}
		m.SpouseName = strings.Join(spouses[m.ID], "、")
		m.SpouseType = strings.Join(spouseTypes[m.ID], "、")
		m.ChildrenCount = children[m.ID]
	}
	return out, nil
}

// liteName 按 id 取成员姓名（不存在返回空）
func liteName(m map[int64]*memberLite, id int64) string {
	if id <= 0 {
		return ""
	}
	if v := m[id]; v != nil {
		return v.Name
	}
	return ""
}

// exportCell 取某一列的导出值
func exportCell(m exportMember, key string) string {
	switch key {
	case "id":
		return strconv.FormatInt(m.ID, 10)
	case "name":
		return m.Name
	case "surname":
		return m.Surname
	case "given_name":
		return m.GivenName
	case "generation":
		return strconv.Itoa(m.Generation)
	case "gender":
		if m.Gender == 2 {
			return "女"
		}
		return "男"
	case "branch":
		if m.BranchName != "" {
			return m.BranchName
		}
		return m.Branch
	case "zi":
		return m.Zi
	case "hao":
		return m.Hao
	case "generation_word":
		return m.GenerationWord
	case "rank":
		return m.Rank
	case "clan_type":
		return m.ClanType
	case "record_type":
		return m.RecordType
	case "father_id":
		if m.FatherID > 0 {
			return strconv.FormatInt(m.FatherID, 10)
		}
		return ""
	case "father_name":
		return m.FatherName
	case "father_father_name":
		return m.FatherFatherName
	case "mother_id":
		if m.MotherID > 0 {
			return strconv.FormatInt(m.MotherID, 10)
		}
		return ""
	case "mother_name":
		if m.MotherName != "" {
			return m.MotherName
		}
		return m.MotherSpouseName
	case "mother_father_name":
		return m.MotherFatherName
	case "birth_calendar":
		if m.BirthCalendar == "lunar" {
			return "农历"
		}
		return "公历"
	case "birth_gregorian":
		return m.BirthGregorian
	case "birth_lunar":
		return m.BirthLunar
	case "birth_ganzhi":
		return m.BirthGanzhi
	case "birth_era":
		return m.BirthEra
	case "death_gregorian":
		return m.DeathGregorian
	case "death_lunar":
		return m.DeathLunar
	case "death_ganzhi":
		return m.DeathGanzhi
	case "death_era":
		return m.DeathEra
	case "lifespan":
		if m.Lifespan > 0 {
			return strconv.Itoa(m.Lifespan)
		}
		return ""
	case "native_place":
		return m.NativePlace
	case "current_place":
		return m.CurrentPlace
	case "title":
		return m.Title
	case "occupation":
		return m.Occupation
	case "education":
		return m.Education
	case "biography":
		return m.Biography
	case "burial_place":
		return m.BurialPlace
	case "former_name":
		return m.FormerName
	case "married_out":
		return m.MarriedOut
	case "phone":
		return m.Phone
	case "no_heir":
		if m.NoHeir == 1 {
			return "是"
		}
		return "否"
	case "lost_contact":
		if m.LostContact == 1 {
			return "是"
		}
		return "否"
	case "spouse_name":
		return m.SpouseName
	case "spouse_type":
		return m.SpouseType
	case "portrait":
		return m.Portrait
	case "created_at":
		return m.CreatedAt
	case "updated_at":
		return m.UpdatedAt
	case "dna_haploid":
		return m.DNAHaploid
	case "dna_diploid":
		return m.DNADiploid
	case "dna_mito":
		return m.DNAMito
	case "children_count":
		return strconv.Itoa(m.ChildrenCount)
	case "":
		return ""
	}
	return ""
}

// Template 下载导入模板
// GET /api/members/import/template?format=xlsx
func (h *MemberIOHandler) Template(c *gin.Context) {
	role := c.GetString("role")
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "xlsx")))
	if format != "csv" {
		format = "xlsx"
	}
	header := []string{}
	for _, col := range ioCols {
		if !ioColumnAllowed(col, role) {
			continue
		}
		header = append(header, col.Label)
	}
	notes := [][]string{
		{"列名", "填写说明"},
		{"ID", "可选。填写后按 ID 精确更新；不填则按「姓名 + 生父姓名」匹配，匹配不到就新增。"},
		{"姓名", "必填，写全名（如「李国栋」）。也可分别填「姓」「名」两列。"},
		{"性别", "填「男」或「女」，留空默认男。"},
		{"世代", "填数字（如 12）。留空时：有生父则 = 生父世代 + 1，否则 = 1。"},
		{"房支", "填房支名称（须已在「房支管理」中存在），留空则跟随生父房支。"},
		{"出生日期(公历)", "统一写 YYYY-MM-DD（如 1950-08-01）；只知年份可只写 1950。"},
		{"去世日期(公历)", "统一写 YYYY-MM-DD；留空 = 在世。"},
		{"出生/去世日期(农历)", "农历文本原样保留（如「一九五〇年七月初一」）。"},
		{"出生历法", "填「公历」或「农历」，留空默认公历。"},
		{"生父ID / 生父姓名", "二选一：ID 最准确；用姓名时按「姓名 + 生父之父姓名」区分同名。"},
		{"生母ID / 生母姓名", "可填女性成员；若填的是生父的配偶（不在成员表中），会自动关联为生母（配偶）。"},
		{"配偶 / 配偶类型", "配偶姓名多个用「、」分隔；类型可填 正配/继配/侧室，留空默认正配。"},
		{"无嗣 / 失联", "填「是」或「否」。"},
		{"导入语义", "更新时：单元格留空 = 保持库中原值；只有填了值才会覆盖。"},
		{"只读列", "享年、干支、子女数、头像、创建/更新时间、DNA 等列只用于导出查看，导入时忽略。"},
	}
	if format == "csv" {
		var buf bytes.Buffer
		buf.WriteString("\xEF\xBB\xBF")
		w := csv.NewWriter(&buf)
		_ = w.Write(header)
		w.Flush()
		h.attachment(c, "成员导入模板.csv", "text/csv; charset=utf-8", buf.Bytes())
		return
	}
	var buf bytes.Buffer
	if err := writeXLSX(&buf, []xlsxSheet{
		{Name: "成员档案", Header: header, Rows: nil},
		{Name: "填写说明", Header: []string{"列名", "填写说明"}, Rows: notes[1:], Widths: []float64{22, 90}},
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成模板失败：" + err.Error()})
		return
	}
	h.attachment(c, "成员导入模板.xlsx",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

// ---------------------------------------------------------------------------
// 导入：读文件 → 校验 → 预览 / 执行
// ---------------------------------------------------------------------------

type importColumnView struct {
	Index  int    `json:"index"`
	Header string `json:"header"`
	Field  string `json:"field"`
	Label  string `json:"label"`
	Status string `json:"status"` // ok / readonly / unknown / duplicate
}

type importRow struct {
	RowNo int
	Raw   map[string]string

	Action  string // create / update / skip / error
	Reasons []string
	Warns   []string

	MemberID   int64
	Name       string
	Surname    string
	GivenName  string
	Generation int
	Gender     int
	BranchID   int64
	BranchName string

	FatherID       int64
	MotherID       int64
	MotherSpouseID int64
	FatherPending  *importRow
	MotherPending  *importRow

	SpouseNames []string
	SpouseTypes []string
	SpouseType  string
	SpouseAdd   int
	Changes     []string

	Member *Member
}

func (r *importRow) fail(reason string) {
	r.Reasons = append(r.Reasons, reason)
	r.Action = "error"
}

type importAnalysis struct {
	FileType  string
	Columns   []importColumnView
	Rows      []*importRow
	Total     int
	Valid     int
	Invalid   int
	Create    int
	Update    int
	Skip      int
	SpouseAdd int
}

type importRowView struct {
	RowNo      int      `json:"row_no"`
	Action     string   `json:"action"`
	MemberID   int64    `json:"member_id"`
	Name       string   `json:"name"`
	Generation int      `json:"generation"`
	Gender     string   `json:"gender"`
	Branch     string   `json:"branch"`
	Father     string   `json:"father"`
	Mother     string   `json:"mother"`
	Reasons    []string `json:"reasons,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
	Changes    []string `json:"changes,omitempty"`
	SpouseAdd  int      `json:"spouse_add"`
}

func (r *importRow) view() importRowView {
	gender := ""
	switch r.Gender {
	case 1:
		gender = "男"
	case 2:
		gender = "女"
	}
	father, mother := "", ""
	if r.FatherPending != nil {
		father = r.FatherPending.Name + "（本表新增）"
	} else if r.FatherID > 0 {
		father = "ID:" + strconv.FormatInt(r.FatherID, 10)
	}
	if r.MotherPending != nil {
		mother = r.MotherPending.Name + "（本表新增）"
	} else if r.MotherID > 0 {
		mother = "ID:" + strconv.FormatInt(r.MotherID, 10)
	} else if r.MotherSpouseID > 0 {
		mother = "配偶ID:" + strconv.FormatInt(r.MotherSpouseID, 10)
	}
	return importRowView{
		RowNo: r.RowNo, Action: r.Action, MemberID: r.MemberID, Name: r.Name,
		Generation: r.Generation, Gender: gender, Branch: r.BranchName,
		Father: father, Mother: mother,
		Reasons: r.Reasons, Warnings: r.Warns, Changes: r.Changes, SpouseAdd: r.SpouseAdd,
	}
}

// Preview 上传预览校验
// POST /api/members/import/preview  (multipart: file)
func (h *MemberIOHandler) Preview(c *gin.Context) {
	table, fileType, err := h.readUploadedTable(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a, err := h.analyzeTable(table, c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	views := make([]importRowView, 0, len(a.Rows))
	for _, r := range a.Rows {
		views = append(views, r.view())
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":        a.Invalid == 0,
		"file_type": fileType,
		"columns":   a.Columns,
		"stats": gin.H{
			"total": a.Total, "valid": a.Valid, "invalid": a.Invalid,
			"create": a.Create, "update": a.Update, "skip": a.Skip, "spouse_add": a.SpouseAdd,
		},
		"rows":     views,
		"warnings": a.Warnings(),
	})
}

// Import 执行导入（只写校验通过的行）
// POST /api/members/import  (multipart: file, mode=abort|skip)
func (h *MemberIOHandler) Import(c *gin.Context) {
	table, fileType, err := h.readUploadedTable(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a, err := h.analyzeTable(table, c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	mode := strings.ToLower(strings.TrimSpace(c.PostForm("mode")))
	if mode != "skip" {
		mode = "abort"
	}
	if a.Valid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "没有任何一行通过校验，未写入数据库",
			"mode":  mode, "file_type": fileType,
			"stats": gin.H{"total": a.Total, "valid": 0, "invalid": a.Invalid,
				"create": 0, "update": 0, "skip": 0},
			"failures": failureViews(a.Rows, nil),
		})
		return
	}

	uid, uname := currentUser(c)
	res := &importApplyResult{}
	if err := h.applyImport(a, mode, uid, uname, res); err != nil {
		status := http.StatusInternalServerError
		c.JSON(status, gin.H{
			"error":       err.Error(),
			"mode":        mode,
			"file_type":   fileType,
			"rolled_back": mode == "abort",
			"created":     res.Created, "updated": res.Updated, "skipped": res.Skipped,
			"spouse_add": res.SpouseAdd,
			"failures":   failureViews(a.Rows, res.ApplyFailures),
			"stats": gin.H{"total": a.Total, "valid": a.Valid, "invalid": a.Invalid,
				"create": a.Create, "update": a.Update, "skip": a.Skip},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok": true, "mode": mode, "file_type": fileType,
		"created": res.Created, "updated": res.Updated, "skipped": res.Skipped,
		"spouse_add": res.SpouseAdd, "invalid": a.Invalid,
		"created_ids": res.CreatedIDs, "updated_ids": res.UpdatedIDs,
		"failures": failureViews(a.Rows, res.ApplyFailures),
		"stats": gin.H{"total": a.Total, "valid": a.Valid, "invalid": a.Invalid,
			"create": a.Create, "update": a.Update, "skip": a.Skip},
	})
}

// FailuresCSV 下载失败清单 CSV（重新解析上传文件，不写库）
// POST /api/members/import/failures  (multipart: file)
func (h *MemberIOHandler) FailuresCSV(c *gin.Context) {
	table, _, err := h.readUploadedTable(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a, err := h.analyzeTable(table, c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"行号", "姓名", "失败原因", "原始数据（JSON）"})
	n := 0
	for _, r := range a.Rows {
		if r.Action != "error" {
			continue
		}
		n++
		raw, _ := json.Marshal(r.Raw)
		_ = w.Write([]string{strconv.Itoa(r.RowNo), r.Name, strings.Join(r.Reasons, "；"), string(raw)})
	}
	w.Flush()
	if n == 0 {
		_ = w.Write([]string{"", "", "（没有失败行）", ""})
		w.Flush()
	}
	filename := "导入失败清单_" + time.Now().Format("20060102_1504") + ".csv"
	h.attachment(c, filename, "text/csv; charset=utf-8", buf.Bytes())
}

type applyFailure struct {
	RowNo  int    `json:"row_no"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type importApplyResult struct {
	Created       int
	Updated       int
	Skipped       int
	SpouseAdd     int
	CreatedIDs    []int64
	UpdatedIDs    []int64
	ApplyFailures []applyFailure
}

func failureViews(rows []*importRow, applyFails []applyFailure) []importRowView {
	out := []importRowView{}
	for _, r := range rows {
		if r.Action == "error" {
			out = append(out, r.view())
		}
	}
	for _, f := range applyFails {
		out = append(out, importRowView{RowNo: f.RowNo, Name: f.Name, Action: "error", Reasons: []string{f.Reason}})
	}
	return out
}

// a.Warnings 表级提示
func (a *importAnalysis) Warnings() []string {
	w := []string{}
	unknown := 0
	for _, c := range a.Columns {
		if c.Status == "unknown" {
			unknown++
		}
	}
	if unknown > 0 {
		w = append(w, fmt.Sprintf("有 %d 个列名未识别，导入时会忽略（可对照模板检查表头）", unknown))
	}
	if a.Invalid > 0 {
		w = append(w, fmt.Sprintf("有 %d 行未通过校验，导入时会被跳过", a.Invalid))
	}
	return w
}

// readUploadedTable 读取上传文件并解析为二维表
func (h *MemberIOHandler) readUploadedTable(c *gin.Context) ([][]string, string, error) {
	fh, err := c.FormFile("file")
	if err != nil {
		return nil, "", fmt.Errorf("请选择要上传的 Excel/CSV 文件")
	}
	if fh.Size > importMaxBytes {
		return nil, "", fmt.Errorf("文件过大（%.1fMB），上限 %dMB", float64(fh.Size)/(1<<20), importMaxBytes>>20)
	}
	f, err := fh.Open()
	if err != nil {
		return nil, "", fmt.Errorf("无法读取上传文件：%w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, importMaxBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("读取上传文件失败：%w", err)
	}
	if len(data) > importMaxBytes {
		return nil, "", fmt.Errorf("文件过大，上限 %dMB", importMaxBytes>>20)
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("文件内容为空")
	}

	// xlsx：zip 魔数 PK\x03\x04
	if len(data) > 4 && data[0] == 'P' && data[1] == 'K' {
		rows, err := readXLSX(data)
		if err != nil {
			return nil, "", fmt.Errorf("解析 Excel 失败：%w", err)
		}
		return rows, "xlsx", nil
	}
	// CSV / TXT：先按 UTF-8，失败则按 GB18030（Windows 中文 Excel 常见编码）
	body := data
	if !utf8.Valid(body) {
		if dec, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), body); err == nil {
			body = dec
		}
	}
	body = bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})
	text := string(body)
	comma := ','
	if !strings.Contains(text, ",") && strings.Contains(text, "\t") {
		comma = '\t'
	}
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = comma
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, "", fmt.Errorf("解析 CSV 失败：%w", err)
	}
	return records, "csv", nil
}

// analyzeTable 校验整张表并给出每行动作（不写库）
func (h *MemberIOHandler) analyzeTable(table [][]string, c *gin.Context) (*importAnalysis, error) {
	if len(table) == 0 {
		return nil, fmt.Errorf("文件里没有内容")
	}
	// 表头 = 第一个非空行
	headerIdx := -1
	for i, row := range table {
		for _, v := range row {
			if strings.TrimSpace(v) != "" {
				headerIdx = i
				break
			}
		}
		if headerIdx >= 0 {
			break
		}
	}
	if headerIdx < 0 {
		return nil, fmt.Errorf("文件里没有表头")
	}
	header := table[headerIdx]
	columns, colOf, err := mapImportColumns(header)
	if err != nil {
		return nil, err
	}

	a := &importAnalysis{Columns: columns}
	a.FileType = ""
	// 逐行收集原始值
	for i := headerIdx + 1; i < len(table) && len(a.Rows) < importMaxRows; i++ {
		row := table[i]
		vals := map[string]string{}
		nonEmpty := false
		for ci, key := range colOf {
			if key == "" || ci >= len(row) {
				continue
			}
			v := strings.TrimSpace(row[ci])
			if v == "" {
				continue
			}
			// 同一字段只取第一列
			if _, exists := vals[key]; exists {
				continue
			}
			vals[key] = v
			nonEmpty = true
		}
		if !nonEmpty {
			continue
		}
		r := &importRow{RowNo: i + 1, Raw: vals}
		a.Rows = append(a.Rows, r)
	}
	if len(a.Rows) == 0 {
		return nil, fmt.Errorf("表头下面没有数据行")
	}
	a.Total = len(a.Rows)

	// 基础校验
	seenID := map[int64]int{}
	for _, r := range a.Rows {
		h.validateBasics(r, seenID)
	}

	// 数据库索引
	all, err := loadMemberLite(h.DB)
	if err != nil {
		return nil, err
	}
	byName := map[string][]*memberLite{}
	for _, m := range all {
		if m.Name != "" {
			byName[m.Name] = append(byName[m.Name], m)
		}
	}
	branches := map[string]int64{}
	if rows, err := h.DB.Query(`SELECT id, name FROM branches`); err == nil {
		for rows.Next() {
			var id int64
			var name string
			if rows.Scan(&id, &name) == nil {
				branches[strings.TrimSpace(name)] = id
			}
		}
		rows.Close()
	}

	// 生父/生母引用（先解析到 ID 或本表待插入行）
	pendingByName := map[string][]*importRow{}
	pendingChildren := map[*importRow][]*importRow{}
	for _, r := range a.Rows {
		if r.Name != "" {
			pendingByName[r.Name] = append(pendingByName[r.Name], r)
		}
	}
	for _, r := range a.Rows {
		if r.Action == "error" {
			continue
		}
		h.resolveParents(r, all, byName, pendingByName)
		if r.FatherPending != nil {
			pendingChildren[r.FatherPending] = append(pendingChildren[r.FatherPending], r)
		}
		if r.MotherPending != nil {
			pendingChildren[r.MotherPending] = append(pendingChildren[r.MotherPending], r)
		}
	}

	// 世代：填写优先；否则父世代+1（支持本表内父子）
	for _, r := range a.Rows {
		if r.Action == "error" {
			continue
		}
		if r.Generation <= 0 && r.Raw["generation"] == "" {
			r.Generation = h.resolveGeneration(r, all, map[*importRow]bool{})
			if r.Generation < 1 {
				r.Generation = 1
			}
		}
	}

	// 房支：名称/ID → branch_id；为空时跟随生父
	for _, r := range a.Rows {
		if r.Action == "error" {
			continue
		}
		if v := r.Raw["branch"]; v != "" {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
				found := false
				for _, bid := range branches {
					if bid == id {
						found = true
						break
					}
				}
				if !found {
					r.fail("房支不存在：ID " + v)
					continue
				}
				r.BranchID = id
			} else if id, ok := branches[v]; ok {
				r.BranchID = id
			} else {
				r.fail("房支不存在：「" + v + "」（请先在「房支管理」中添加）")
				continue
			}
		}
		if r.BranchID > 0 {
			for name, id := range branches {
				if id == r.BranchID {
					r.BranchName = name
					break
				}
			}
		}
	}
	// 房支继承：父/母在本表新增时递归跟随（可能多代）
	var inheritBranch func(r *importRow, seen map[*importRow]bool) int64
	inheritBranch = func(r *importRow, seen map[*importRow]bool) int64 {
		if r.BranchID > 0 {
			return r.BranchID
		}
		if seen[r] {
			return 0
		}
		seen[r] = true
		if r.FatherPending != nil {
			return inheritBranch(r.FatherPending, seen)
		}
		if r.MotherPending != nil {
			return inheritBranch(r.MotherPending, seen)
		}
		if r.FatherID > 0 {
			if f := liteByID(all, r.FatherID); f != nil {
				return f.BranchID
			}
		}
		return 0
	}
	for _, r := range a.Rows {
		if r.Action == "error" || r.BranchID > 0 {
			continue
		}
		if bid := inheritBranch(r, map[*importRow]bool{}); bid > 0 {
			r.BranchID = bid
		}
		if r.BranchID > 0 {
			for name, id := range branches {
				if id == r.BranchID {
					r.BranchName = name
					break
				}
			}
		}
	}

	// 动作：更新（ID 或 姓名+生父姓名） / 新增
	for _, r := range a.Rows {
		if r.Action == "error" {
			continue
		}
		r.Action = h.decideAction(r, all, byName)
	}

	// 同文件重复（新增行：姓名+父）
	seenKey := map[string]*importRow{}
	for _, r := range a.Rows {
		if r.Action != "create" {
			continue
		}
		key := r.Name + "\x1f"
		if r.FatherPending != nil {
			key += "pending:" + strconv.Itoa(r.FatherPending.RowNo)
		} else {
			key += strconv.FormatInt(r.FatherID, 10)
		}
		if prev, ok := seenKey[key]; ok {
			r.fail(fmt.Sprintf("文件内重复：与第 %d 行同名同父，请合并后再导入", prev.RowNo))
			continue
		}
		seenKey[key] = r
	}

	// 组装 Member / 计算更新字段
	for _, r := range a.Rows {
		if r.Action == "error" || r.Action == "skip" {
			continue
		}
		if r.Action == "update" {
			if err := h.prepareUpdate(r); err != nil {
				r.fail(err.Error())
			}
		} else {
			if err := h.prepareCreate(r); err != nil {
				r.fail(err.Error())
			}
		}
	}

	// 统计
	for _, r := range a.Rows {
		switch r.Action {
		case "create":
			a.Create++
		case "update":
			a.Update++
		case "skip":
			a.Skip++
		default:
			a.Invalid++
			r.Action = "error"
		}
		if r.Action != "error" {
			a.Valid++
			if len(r.SpouseNames) > 0 {
				a.SpouseAdd += len(r.SpouseNames)
			}
		}
	}
	return a, nil
}

// mapImportColumns 表头 → 字段映射
func mapImportColumns(header []string) ([]importColumnView, []string, error) {
	columns := make([]importColumnView, len(header))
	colOf := make([]string, len(header))
	seenKey := map[string]int{}
	seenHeader := map[string]int{}
	for i, raw := range header {
		name := strings.TrimSpace(raw)
		name = strings.TrimPrefix(name, "\uFEFF")
		cv := importColumnView{Index: i + 1, Header: name}
		if name == "" {
			cv.Status = "unknown"
			columns[i] = cv
			continue
		}
		if j, ok := seenHeader[name]; ok {
			cv.Status = "duplicate"
			columns[i] = cv
			_ = j
			continue
		}
		seenHeader[name] = i

		col, ok := ioColumnByLabel(name)
		if !ok {
			// 别名
			for _, cand := range ioCols {
				for _, a := range cand.Aliases {
					if a == name {
						col, ok = cand, true
						break
					}
				}
				if ok {
					break
				}
			}
		}
		if !ok {
			cv.Status = "unknown"
			columns[i] = cv
			continue
		}
		cv.Field = col.Key
		cv.Label = col.Label
		if col.Key == "" || col.ReadOnly {
			cv.Status = "readonly"
			columns[i] = cv
			continue
		}
		if prev, ok := seenKey[col.Key]; ok {
			cv.Status = "duplicate"
			columns[i] = cv
			_ = prev
			continue
		}
		seenKey[col.Key] = i
		cv.Status = "ok"
		colOf[i] = col.Key
		columns[i] = cv
	}
	// 姓名/姓/名至少要有一个
	if _, ok := seenKey["name"]; !ok {
		if _, ok2 := seenKey["given_name"]; !ok2 {
			return columns, colOf, fmt.Errorf("表头缺少必填列「姓名」（或「姓」+「名」）")
		}
	}
	return columns, colOf, nil
}

// validateBasics 单行基础校验（姓名、ID、性别、日期、布尔）
func (h *MemberIOHandler) validateBasics(r *importRow, seenID map[int64]int) {
	if v := r.Raw["id"]; v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			r.fail("ID 不是正整数：" + v)
		} else {
			if prev, ok := seenID[id]; ok {
				r.fail(fmt.Sprintf("文件内重复：ID %d 已在第 %d 行出现", id, prev))
			} else {
				seenID[id] = r.RowNo
			}
			r.MemberID = id
		}
	}

	name := strings.TrimSpace(r.Raw["name"])
	surname := strings.TrimSpace(r.Raw["surname"])
	given := strings.TrimSpace(r.Raw["given_name"])
	if name == "" {
		// 允许「姓 + 名」拼出姓名
		if given != "" {
			name = surname + given
		}
	}
	if name == "" {
		r.fail("姓名为空（必填）")
		return
	}
	if name == strings.TrimSpace(surname) && given == "" {
		r.fail("「姓名」只有姓，缺少「名」")
		return
	}
	if surname != "" && given != "" {
		if name != surname+given {
			r.fail(fmt.Sprintf("「姓名」(%s) 与「姓」+「名」(%s+%s) 不一致", name, surname, given))
			return
		}
	}
	r.Name = name
	r.Surname = surname
	r.GivenName = given

	if v := r.Raw["gender"]; v != "" {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "男", "1", "m", "male", "男性":
			r.Gender = 1
		case "女", "2", "f", "female", "女性":
			r.Gender = 2
		default:
			r.fail("性别只能是「男」或「女」：" + v)
		}
	}

	if v := r.Raw["generation"]; v != "" {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 0 || n > 999 {
			r.fail("世代必须是 0-999 的整数：" + v)
		} else {
			r.Generation = n
		}
	}

	// 日期校验/规范化
	for _, key := range []string{"birth_gregorian", "death_gregorian"} {
		v := r.Raw[key]
		if v == "" {
			continue
		}
		norm, ok := normalizeImportDate(v)
		if !ok {
			label := "出生日期(公历)"
			if key == "death_gregorian" {
				label = "去世日期(公历)"
			}
			r.fail(fmt.Sprintf("%s格式非法：%s（应为 YYYY-MM-DD 或 YYYY）", label, v))
			continue
		}
		r.Raw[key] = norm
	}

	// 历法
	if v := r.Raw["birth_calendar"]; v != "" {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "公历", "阳历", "solar", "g":
			r.Raw["birth_calendar"] = "solar"
		case "农历", "阴历", "lunar", "l":
			r.Raw["birth_calendar"] = "lunar"
		default:
			r.fail("出生历法只能是「公历」或「农历」：" + v)
		}
	}
	// 布尔
	for _, key := range []string{"no_heir", "lost_contact"} {
		if v := r.Raw[key]; v != "" {
			b, ok := parseImportBool(v)
			if !ok {
				r.fail("「" + key + "」只能填 是/否：" + v)
				continue
			}
			if b {
				r.Raw[key] = "1"
			} else {
				r.Raw[key] = "0"
			}
		}
	}
	if r.Action == "" {
		r.Action = "pending"
	}
}

// normalizeImportDate 接受 YYYY-MM-DD / YYYY/M/D / YYYY.M.D / YYYY年M月D日 / YYYY
func normalizeImportDate(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	s = strings.NewReplacer("年", "-", "月", "-", "日", "").Replace(s)
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ".", "-")
	s = strings.Trim(s, "-")
	parts := strings.Split(s, "-")
	toInt := func(x string) (int, bool) {
		n, err := strconv.Atoi(x)
		return n, err == nil
	}
	switch len(parts) {
	case 1:
		y, ok := toInt(parts[0])
		if !ok || y < 1 || y > 9999 || len(parts[0]) != 4 {
			return "", false
		}
		return fmt.Sprintf("%04d", y), true
	case 3:
		y, ok1 := toInt(parts[0])
		m, ok2 := toInt(parts[1])
		d, ok3 := toInt(parts[2])
		if !ok1 || !ok2 || !ok3 || len(parts[0]) != 4 || y < 1 || y > 9999 || m < 1 || m > 12 || d < 1 || d > 31 {
			return "", false
		}
		t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
		if t.Year() != y || int(t.Month()) != m || t.Day() != d {
			return "", false
		}
		return t.Format("2006-01-02"), true
	}
	return "", false
}

func parseImportBool(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "是", "1", "true", "y", "yes", "有", "无嗣", "失联", "存殁未详":
		return true, true
	case "否", "0", "false", "n", "no", "无", "":
		return false, true
	}
	return false, false
}

// resolveParents 解析生父/生母引用（DB 或本表待插入行）
func (h *MemberIOHandler) resolveParents(r *importRow, all map[int64]*memberLite, byName map[string][]*memberLite, pendingByName map[string][]*importRow) {
	// 生父
	if v := r.Raw["father_id"]; v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			r.fail("生父ID 不是正整数：" + v)
		} else if liteByID(all, id) == nil {
			r.fail(fmt.Sprintf("生父ID 不存在：%d", id))
		} else {
			r.FatherID = id
		}
	} else if name := strings.TrimSpace(r.Raw["father_name"]); name != "" {
		grand := strings.TrimSpace(r.Raw["father_father_name"])
		if id := findMemberIDByName(all, byName, name, grand, r.MemberID); id > 0 {
			r.FatherID = id
		} else {
			if cands := findPendingRows(pendingByName[name], r, grand, all); len(cands) == 1 {
				r.FatherPending = cands[0]
			} else if len(cands) > 1 {
				r.fail("生父「" + name + "」在本表中有多人同名，请填写「生父之父姓名」区分")
			} else {
				r.fail("生父不存在：" + name + "（可填生父ID，或先导入父辈）")
			}
		}
	}

	// 生母
	if v := r.Raw["mother_id"]; v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			r.fail("生母ID 不是正整数：" + v)
		} else if m := liteByID(all, id); m == nil {
			r.fail(fmt.Sprintf("生母ID 不存在：%d", id))
		} else if m.Gender == 1 {
			r.fail(fmt.Sprintf("生母ID %d 对应成员是男性，不能作为生母", id))
		} else {
			r.MotherID = id
		}
	} else if name := strings.TrimSpace(r.Raw["mother_name"]); name != "" {
		grand := strings.TrimSpace(r.Raw["mother_father_name"])
		if id := findMemberIDByName(all, byName, name, grand, r.MemberID); id > 0 {
			r.MotherID = id
		} else if r.FatherID > 0 && h.spouseIDOf(r.FatherID, name) > 0 {
			// 生母记在生父的配偶里（未建成员档案）
			r.MotherSpouseID = h.spouseIDOf(r.FatherID, name)
		} else if cands := findPendingRows(pendingByName[name], r, grand, all); len(cands) == 1 {
			r.MotherPending = cands[0]
		} else if len(cands) > 1 {
			r.fail("生母「" + name + "」在本表中有多人同名，请填写「生母之父姓名」区分")
		} else {
			r.fail("生母不存在：" + name + "（可填生母ID，或先导入母辈）")
		}
	}
}

func liteByID(all map[int64]*memberLite, id int64) *memberLite {
	if id <= 0 {
		return nil
	}
	return all[id]
}

// findMemberIDByName 按姓名（+父名）在库中查找唯一成员
func findMemberIDByName(all map[int64]*memberLite, byName map[string][]*memberLite, name, fatherName string, excludeID int64) int64 {
	cands := byName[name]
	if len(cands) == 0 {
		return 0
	}
	filtered := make([]*memberLite, 0, len(cands))
	for _, m := range cands {
		if m.ID == excludeID {
			continue
		}
		if fatherName != "" {
			fname := ""
			if f := all[m.FatherID]; f != nil {
				fname = f.Name
			}
			if fname != fatherName {
				continue
			}
		}
		filtered = append(filtered, m)
	}
	if len(filtered) == 1 {
		return filtered[0].ID
	}
	return 0
}

// findPendingRows 在本表待插入行里按姓名+父名找父母
func findPendingRows(cands []*importRow, self *importRow, fatherName string, all map[int64]*memberLite) []*importRow {
	out := []*importRow{}
	for _, c := range cands {
		if c == self || c.Name == "" {
			continue
		}
		if fatherName != "" {
			fname := ""
			if c.FatherPending != nil {
				fname = c.FatherPending.Name
			} else if f := all[c.FatherID]; f != nil {
				fname = f.Name
			}
			if fname != fatherName {
				continue
			}
		}
		out = append(out, c)
	}
	// 父名不唯一时若只传姓名，仍允许唯一命中
	return out
}

// spouseIDOf 查某成员配偶里是否有该姓名
func (h *MemberIOHandler) spouseIDOf(memberID int64, name string) int64 {
	var id int64
	_ = h.DB.QueryRow(`SELECT id FROM spouses WHERE member_id=? AND name=? LIMIT 1`, memberID, name).Scan(&id)
	return id
}

// resolveGeneration 递归计算世代（父/母在本表内也能算）
func (h *MemberIOHandler) resolveGeneration(r *importRow, all map[int64]*memberLite, visiting map[*importRow]bool) int {
	if r.Generation > 0 {
		return r.Generation
	}
	if visiting[r] {
		return 0
	}
	visiting[r] = true
	defer delete(visiting, r)
	base := 0
	if r.FatherPending != nil {
		base = h.resolveGeneration(r.FatherPending, all, visiting)
	} else if r.FatherID > 0 {
		if f := all[r.FatherID]; f != nil {
			base = f.Generation
		}
	}
	if base == 0 {
		if r.MotherPending != nil {
			base = h.resolveGeneration(r.MotherPending, all, visiting)
		} else if r.MotherID > 0 {
			if m := all[r.MotherID]; m != nil {
				base = m.Generation
			}
		}
	}
	if base > 0 {
		return base + 1
	}
	return 1
}

// decideAction 决定 create / update
func (h *MemberIOHandler) decideAction(r *importRow, all map[int64]*memberLite, byName map[string][]*memberLite) string {
	if r.MemberID > 0 {
		if liteByID(all, r.MemberID) == nil {
			r.fail(fmt.Sprintf("ID 不存在：%d", r.MemberID))
			return "error"
		}
		// ID + 姓名必须一致（避免误改他人）
		existing := liteByID(all, r.MemberID)
		if r.Name != "" && existing.Name != r.Name {
			r.fail(fmt.Sprintf("ID %d 对应成员是「%s」，与填写的姓名「%s」不一致（改名请用成员编辑）",
				r.MemberID, existing.Name, r.Name))
			return "error"
		}
		return "update"
	}
	// 姓名匹配
	cands := byName[r.Name]
	if len(cands) == 0 {
		return "create"
	}
	filtered := make([]*memberLite, 0, len(cands))
	for _, m := range cands {
		if r.FatherPending != nil {
			// 生父是本表新增 → 库里不可能已有同父同名的人，视为新增
			continue
		}
		if strings.TrimSpace(r.Raw["father_name"]) != "" || r.FatherID > 0 {
			if m.FatherID != r.FatherID {
				continue
			}
		}
		filtered = append(filtered, m)
	}
	if r.FatherPending != nil {
		return "create"
	}
	switch len(filtered) {
	case 1:
		r.MemberID = filtered[0].ID
		return "update"
	case 0:
		return "create"
	default:
		ids := make([]string, 0, len(filtered))
		for _, m := range filtered {
			ids = append(ids, strconv.FormatInt(m.ID, 10))
		}
		r.fail("同名「" + r.Name + "」有 " + strconv.Itoa(len(filtered)) + " 人（ID: " + strings.Join(ids, ",") + "），请填写「生父姓名」或 ID 区分")
		return "error"
	}
}

// prepareCreate 组装新增成员
func (h *MemberIOHandler) prepareCreate(r *importRow) error {
	// 父/母是本表待插入行且校验失败 → 本行也失败
	if r.FatherPending != nil && r.FatherPending.Action == "error" {
		return fmt.Errorf("生父「%s」所在第 %d 行校验失败", r.FatherPending.Name, r.FatherPending.RowNo)
	}
	if r.MotherPending != nil && r.MotherPending.Action == "error" {
		return fmt.Errorf("生母「%s」所在第 %d 行校验失败", r.MotherPending.Name, r.MotherPending.RowNo)
	}

	surname := r.Surname
	given := r.GivenName
	name := r.Name
	if surname == "" || given == "" {
		// 拆分：姓氏优先用设置里的家族姓氏
		fam := ""
		_ = h.DB.QueryRow(`SELECT value FROM settings WHERE key='family_surname'`).Scan(&fam)
		fam = strings.TrimSpace(fam)
		if surname == "" {
			surname = fam
		}
		if given == "" {
			if surname != "" && strings.HasPrefix(name, surname) {
				given = strings.TrimPrefix(name, surname)
			} else {
				given = name
				if surname == "" {
					surname = fam
				}
			}
		}
		if given == "" {
			return fmt.Errorf("无法从「姓名」拆出「名」，请分别填写「姓」「名」")
		}
	}
	r.Surname, r.GivenName, r.Name = surname, given, surname+given

	m := &Member{
		Generation:     r.Generation,
		BranchID:       r.BranchID,
		Branch:         r.BranchName,
		Surname:        surname,
		GivenName:      given,
		Name:           surname + given,
		Gender:         r.Gender,
		ClanType:       "本生",
		RecordType:     "内纪",
		BirthCalendar:  "solar",
		FatherID:       r.FatherID,
		MotherID:       r.MotherID,
		MotherSpouseID: r.MotherSpouseID,
	}
	if m.Gender == 0 {
		m.Gender = 1
	}
	// 文本字段
	setStr := func(key string, dst *string) {
		if v := r.Raw[key]; v != "" {
			*dst = v
		}
	}
	setStr("zi", &m.Zi)
	setStr("hao", &m.Hao)
	setStr("generation_word", &m.GenerationWord)
	setStr("rank", &m.Rank)
	setStr("clan_type", &m.ClanType)
	setStr("record_type", &m.RecordType)
	setStr("birth_calendar", &m.BirthCalendar)
	setStr("birth_gregorian", &m.BirthGregorian)
	setStr("birth_lunar", &m.BirthLunar)
	setStr("birth_era", &m.BirthEra)
	setStr("death_gregorian", &m.DeathGregorian)
	setStr("death_lunar", &m.DeathLunar)
	setStr("death_era", &m.DeathEra)
	setStr("native_place", &m.NativePlace)
	setStr("current_place", &m.CurrentPlace)
	setStr("title", &m.Title)
	setStr("occupation", &m.Occupation)
	setStr("education", &m.Education)
	setStr("biography", &m.Biography)
	setStr("burial_place", &m.BurialPlace)
	setStr("married_out", &m.MarriedOut)
	setStr("former_name", &m.FormerName)
	setStr("phone", &m.Phone)
	if v := r.Raw["no_heir"]; v == "1" {
		m.NoHeir = 1
	}
	if v := r.Raw["lost_contact"]; v == "1" {
		m.LostContact = 1
	}
	if m.RecordType == "外纪" && m.Generation < 0 {
		m.Generation = 0
	}
	// 房支：branch_id 有值时同步名称
	if m.BranchID > 0 && m.Branch == "" {
		_ = h.DB.QueryRow(`SELECT name FROM branches WHERE id=?`, m.BranchID).Scan(&m.Branch)
	}
	if m.GenerationWord == "" {
		m.GenerationWord = (&MemberHandler{DB: h.DB}).detectGenerationWord(m.GivenName, m.Branch, m.Generation)
	}
	m.fillLunarInfo()

	// 配偶
	r.SpouseNames, r.SpouseTypes = parseSpouseRefs(r.Raw["spouse_name"], r.Raw["spouse_type"])
	r.SpouseAdd = len(r.SpouseNames)
	r.Member = m
	return nil
}

// prepareUpdate 合并更新字段（空单元格不覆盖）
func (h *MemberIOHandler) prepareUpdate(r *importRow) error {
	old, err := fetchImportMemberRow(h.DB, r.MemberID)
	if err != nil {
		return fmt.Errorf("读取 ID %d 失败：%v", r.MemberID, err)
	}
	if old == nil {
		return fmt.Errorf("ID %d 不存在", r.MemberID)
	}
	old.DecryptSensitive(h.EncKey)
	r.Name = old.Name
	r.Surname = old.Surname
	r.GivenName = old.GivenName
	r.BranchID = old.BranchID
	r.BranchName = old.Branch
	if r.Gender == 0 {
		r.Gender = old.Gender
	}

	origFatherID, origMotherID, origMotherSpouseID := old.FatherID, old.MotherID, old.MotherSpouseID
	fatherGiven := r.Raw["father_id"] != "" || r.Raw["father_name"] != "" || r.FatherPending != nil
	motherGiven := r.Raw["mother_id"] != "" || r.Raw["mother_name"] != "" || r.MotherPending != nil
	// 父母：只有明确给出引用时才改
	if fatherGiven {
		if r.FatherPending != nil {
			// 本表新增的父亲：插入后回填，先置 0
			old.FatherID = 0
		} else {
			old.FatherID = r.FatherID
		}
	}
	if motherGiven {
		if r.MotherPending != nil {
			old.MotherID = 0
			old.MotherSpouseID = r.MotherSpouseID
		} else if r.MotherID > 0 {
			old.MotherID = r.MotherID
			old.MotherSpouseID = 0
		} else if r.MotherSpouseID > 0 {
			old.MotherSpouseID = r.MotherSpouseID
		}
	}

	changes := []string{}
	setStr := func(key, label string, dst *string) {
		v := r.Raw[key]
		if v == "" {
			return
		}
		if *dst != v {
			changes = append(changes, label)
			*dst = v
		}
	}
	setInt := func(key, label string, dst *int) {
		v := r.Raw[key]
		if v == "" {
			return
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return
		}
		if *dst != n {
			changes = append(changes, label)
			*dst = n
		}
	}
	if r.Raw["generation"] != "" && old.Generation != r.Generation {
		changes = append(changes, "世代")
		old.Generation = r.Generation
	}
	if r.Gender != old.Gender && r.Raw["gender"] != "" {
		changes = append(changes, "性别")
		old.Gender = r.Gender
	}
	if id := r.BranchID; id > 0 && id != old.BranchID {
		changes = append(changes, "房支")
		old.BranchID = id
		old.Branch = r.BranchName
	}
	setStr("zi", "字", &old.Zi)
	setStr("hao", "号", &old.Hao)
	setStr("generation_word", "字辈", &old.GenerationWord)
	setStr("rank", "排行", &old.Rank)
	setStr("clan_type", "宗法", &old.ClanType)
	setStr("record_type", "类型", &old.RecordType)
	setStr("birth_gregorian", "出生日期", &old.BirthGregorian)
	setStr("birth_lunar", "出生农历", &old.BirthLunar)
	setStr("birth_era", "出生年号", &old.BirthEra)
	setStr("death_gregorian", "去世日期", &old.DeathGregorian)
	setStr("death_lunar", "去世农历", &old.DeathLunar)
	setStr("death_era", "去世年号", &old.DeathEra)
	setStr("native_place", "籍贯", &old.NativePlace)
	setStr("current_place", "现居地", &old.CurrentPlace)
	setStr("title", "头衔", &old.Title)
	setStr("occupation", "职业", &old.Occupation)
	setStr("education", "学历", &old.Education)
	setStr("biography", "生平", &old.Biography)
	setStr("burial_place", "葬地", &old.BurialPlace)
	setStr("married_out", "出嫁", &old.MarriedOut)
	setStr("former_name", "曾用名", &old.FormerName)
	setStr("phone", "电话", &old.Phone)
	setInt("no_heir", "无嗣", &old.NoHeir)
	setInt("lost_contact", "失联", &old.LostContact)

	// 出生历法：库里空值等价于「公历」，避免导出→导入产生无意义的变更
	if v := r.Raw["birth_calendar"]; v != "" {
		oldCal := old.BirthCalendar
		if oldCal == "" {
			oldCal = "solar"
		}
		if oldCal != v {
			changes = append(changes, "出生历法")
		}
		old.BirthCalendar = v
	}
	if fatherGiven && (origFatherID != old.FatherID) {
		changes = append(changes, "生父")
	}
	if motherGiven && (origMotherID != old.MotherID || origMotherSpouseID != old.MotherSpouseID) {
		changes = append(changes, "生母")
	}

	old.Name = old.Surname + old.GivenName
	if old.GenerationWord == "" {
		old.GenerationWord = (&MemberHandler{DB: h.DB}).detectGenerationWord(old.GivenName, old.Branch, old.Generation)
	}
	old.fillLunarInfo()

	// 配偶：只新增缺失的
	if v := r.Raw["spouse_name"]; v != "" {
		names, types := parseSpouseRefs(v, r.Raw["spouse_type"])
		for i, p := range names {
			if h.spouseIDOf(r.MemberID, p) == 0 {
				r.SpouseNames = append(r.SpouseNames, p)
				t := "正配"
				if i < len(types) && types[i] != "" {
					t = types[i]
				}
				r.SpouseTypes = append(r.SpouseTypes, t)
				changes = append(changes, "配偶+"+p)
			}
		}
	}

	r.SpouseAdd = len(r.SpouseNames)
	if len(changes) == 0 && len(r.SpouseNames) == 0 {
		r.Action = "skip"
		r.Warns = append(r.Warns, "与库中数据一致，无需更新")
	}
	r.Changes = changes
	r.Member = old
	return nil
}

// applyImport 在事务中写入；mode=abort 出错整体回滚，mode=skip 逐行跳过
func (h *MemberIOHandler) applyImport(a *importAnalysis, mode string, uid int, uname string, res *importApplyResult) error {
	tx, err := h.DB.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败：%w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// 按世代升序写入，保证本表内父辈先入库
	ordered := make([]*importRow, 0, len(a.Rows))
	for _, r := range a.Rows {
		if r.Action == "create" || r.Action == "update" || r.Action == "skip" {
			ordered = append(ordered, r)
		}
	}
	for i := 1; i < len(ordered); i++ {
		for j := i; j > 0 && ordered[j].Generation < ordered[j-1].Generation; j-- {
			ordered[j], ordered[j-1] = ordered[j-1], ordered[j]
		}
	}

	// 本表新增行 → 新 ID
	newID := map[*importRow]int64{}

	handleErr := func(r *importRow, err error) error {
		if mode == "skip" {
			res.ApplyFailures = append(res.ApplyFailures, applyFailure{RowNo: r.RowNo, Name: r.Name, Reason: err.Error()})
			return nil
		}
		return fmt.Errorf("第 %d 行「%s」写入失败：%v", r.RowNo, r.Name, err)
	}

	for _, r := range ordered {
		switch r.Action {
		case "skip":
			res.Skipped++
			continue
		case "create":
			id, err := h.insertMemberTx(tx, r, uid, uname)
			if err != nil {
				if e := handleErr(r, err); e != nil {
					return e
				}
				continue
			}
			newID[r] = id
			r.MemberID = id
			res.Created++
			res.CreatedIDs = append(res.CreatedIDs, id)
			n, err := h.insertSpousesTx(tx, id, r)
			if err != nil {
				if e := handleErr(r, err); e != nil {
					return e
				}
			}
			res.SpouseAdd += n
		case "update":
			if err := h.updateMemberTx(tx, r, uid, uname); err != nil {
				if e := handleErr(r, err); e != nil {
					return e
				}
				continue
			}
			res.Updated++
			res.UpdatedIDs = append(res.UpdatedIDs, r.MemberID)
			n, err := h.insertSpousesTx(tx, r.MemberID, r)
			if err != nil {
				if e := handleErr(r, err); e != nil {
					return e
				}
			}
			res.SpouseAdd += n
		}
	}

	// 回填「本表新增的父母」引用
	for _, r := range a.Rows {
		if r.Action == "error" {
			continue
		}
		if r.FatherPending == nil && r.MotherPending == nil {
			continue
		}
		mid := r.MemberID
		if mid == 0 {
			mid = newID[r]
		}
		if mid == 0 {
			continue
		}
		fid, moid := r.FatherID, r.MotherID
		if r.FatherPending != nil {
			fid = newID[r.FatherPending]
			if fid == 0 {
				if e := handleErr(r, fmt.Errorf("生父「%s」未能插入成功", r.FatherPending.Name)); e != nil {
					return e
				}
				continue
			}
		}
		if r.MotherPending != nil {
			moid = newID[r.MotherPending]
			if moid == 0 {
				if e := handleErr(r, fmt.Errorf("生母「%s」未能插入成功", r.MotherPending.Name)); e != nil {
					return e
				}
				continue
			}
		}
		if _, err := tx.Exec(`UPDATE members SET father_id=?, mother_id=?, mother_spouse_id=? WHERE id=?`,
			fid, moid, r.MotherSpouseID, mid); err != nil {
			if e := handleErr(r, fmt.Errorf("回填父母关系失败：%v", err)); e != nil {
				return e
			}
		}
	}

	if res.Created == 0 && res.Updated == 0 {
		_ = tx.Rollback()
		committed = true
		if len(res.ApplyFailures) > 0 {
			return fmt.Errorf("全部行都写入失败，已整体回滚：%s", res.ApplyFailures[0].Reason)
		}
		return fmt.Errorf("没有任何行需要写入")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败：%w", err)
	}
	committed = true
	return nil
}

func (h *MemberIOHandler) insertMemberTx(tx *sql.Tx, r *importRow, uid int, uname string) (int64, error) {
	m := r.Member
	m.Name = m.Surname + m.GivenName
	if err := m.EncryptSensitive(h.EncKey); err != nil {
		return 0, err
	}
	res, err := tx.Exec(`INSERT INTO members (generation, branch, branch_id, surname, given_name, name, zi, hao, generation_word,
		rank, gender, father_id, mother_id, mother_spouse_id, clan_type, record_type, foster_father_id,
		birth_calendar, birth_gregorian, birth_lunar, birth_ganzhi, birth_era, death_gregorian, death_lunar, death_ganzhi, death_era,
		lifespan, native_place, current_place, title, occupation, education, biography, burial_place, portrait, married_out, former_name, phone,
		no_heir, lost_contact, dna_haploid, dna_diploid, dna_mito)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.Generation, m.Branch, m.BranchID, m.Surname, m.GivenName, m.Name, m.Zi, m.Hao, m.GenerationWord,
		m.Rank, m.Gender, m.FatherID, m.MotherID, m.MotherSpouseID, m.ClanType, m.RecordType, m.FosterFatherID,
		m.BirthCalendar, m.BirthGregorian, m.BirthLunar, m.BirthGanzhi, m.BirthEra,
		m.DeathGregorian, m.DeathLunar, m.DeathGanzhi, m.DeathEra, m.Lifespan,
		m.NativePlace, m.CurrentPlace, m.Title, m.Occupation, m.Education, m.Biography,
		m.BurialPlace, m.Portrait, m.MarriedOut, m.FormerName, m.Phone, m.NoHeir, m.LostContact,
		m.DNAHaploid, m.DNADiploid, m.DNAMito)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	m.ID = id
	// 版本快照（插入后的明文快照）
	snap := *m
	plain := snap
	if err := plain.DecryptSensitive(h.EncKey); err == nil {
		snap = plain
	}
	if err := recordVersionTx(tx, "member", id, snap, uid, uname, "导入新增"); err != nil {
		return 0, err
	}
	return id, nil
}

func (h *MemberIOHandler) updateMemberTx(tx *sql.Tx, r *importRow, uid int, uname string) error {
	m := r.Member
	if err := m.EncryptSensitive(h.EncKey); err != nil {
		return err
	}
	_, err := tx.Exec(`UPDATE members SET generation=?, branch=?, branch_id=?, surname=?, given_name=?, name=?, zi=?, hao=?, generation_word=?,
		rank=?, gender=?, father_id=?, mother_id=?, mother_spouse_id=?, clan_type=?, record_type=?, foster_father_id=?,
		birth_calendar=?, birth_gregorian=?, birth_lunar=?, birth_ganzhi=?, birth_era=?, death_gregorian=?, death_lunar=?, death_ganzhi=?, death_era=?,
		lifespan=?, native_place=?, current_place=?, title=?, occupation=?, education=?, biography=?, burial_place=?, portrait=?, married_out=?, former_name=?, phone=?,
		no_heir=?, lost_contact=?, dna_haploid=?, dna_diploid=?, dna_mito=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`,
		m.Generation, m.Branch, m.BranchID, m.Surname, m.GivenName, m.Name, m.Zi, m.Hao, m.GenerationWord,
		m.Rank, m.Gender, m.FatherID, m.MotherID, m.MotherSpouseID, m.ClanType, m.RecordType, m.FosterFatherID,
		m.BirthCalendar, m.BirthGregorian, m.BirthLunar, m.BirthGanzhi, m.BirthEra,
		m.DeathGregorian, m.DeathLunar, m.DeathGanzhi, m.DeathEra, m.Lifespan,
		m.NativePlace, m.CurrentPlace, m.Title, m.Occupation, m.Education, m.Biography,
		m.BurialPlace, m.Portrait, m.MarriedOut, m.FormerName, m.Phone, m.NoHeir, m.LostContact,
		m.DNAHaploid, m.DNADiploid, m.DNAMito, m.ID)
	if err != nil {
		return err
	}
	snap := *m
	if err := snap.DecryptSensitive(h.EncKey); err != nil {
		snap = *m
	}
	return recordVersionTx(tx, "member", m.ID, snap, uid, uname, "导入修改")
}

func (h *MemberIOHandler) insertSpousesTx(tx *sql.Tx, memberID int64, r *importRow) (int, error) {
	n := 0
	for i, name := range r.SpouseNames {
		var exists int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM spouses WHERE member_id=? AND name=?`, memberID, name).Scan(&exists); err != nil {
			return n, err
		}
		if exists > 0 {
			continue
		}
		surname, given := "", name
		if rs := []rune(name); len(rs) > 1 {
			surname = string(rs[0])
			given = string(rs[1:])
		}
		typ := ""
		if i < len(r.SpouseTypes) {
			typ = r.SpouseTypes[i]
		}
		if typ == "" {
			typ = r.SpouseType
		}
		if typ == "" {
			typ = "正配"
		}
		if _, err := tx.Exec(`INSERT INTO spouses (member_id, surname, given_name, name, type) VALUES (?,?,?,?,?)`,
			memberID, surname, given, name, typ); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// splitImportList 按中英文逗号/顿号/分号/竖线拆分单元格
func splitImportList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	out := []string{}
	for _, part := range strings.FieldsFunc(s, func(ru rune) bool {
		return ru == '、' || ru == '，' || ru == ',' || ru == ';' || ru == '；' || ru == '|'
	}) {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// splitSpouseTypeSuffix 拆出「王氏（正配）」这种带类型的写法
func splitSpouseTypeSuffix(p string) (name, typ string) {
	p = strings.TrimSpace(p)
	for _, pair := range [][2]string{{"（", "）"}, {"(", ")"}} {
		open, closeCh := pair[0], pair[1]
		if strings.HasSuffix(p, closeCh) {
			if i := strings.LastIndex(p, open); i > 0 {
				return strings.TrimSpace(p[:i]), strings.TrimSpace(p[i+len(open) : len(p)-len(closeCh)])
			}
		}
	}
	return p, ""
}

// parseSpouseRefs 解析配偶姓名 + 配偶类型两列。
// 支持：多个配偶用 、，,;；| 分隔；姓名后带（正配）类型；类型列与姓名一一对应或只给一个。
func parseSpouseRefs(namesRaw, typesRaw string) (names, types []string) {
	parts := splitImportList(namesRaw)
	if len(parts) == 0 {
		return nil, nil
	}
	typeList := splitImportList(typesRaw)
	seen := map[string]bool{}
	for i, part := range parts {
		name, inlineType := splitSpouseTypeSuffix(part)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		typ := inlineType
		if typ == "" {
			if len(typeList) == len(parts) && i < len(typeList) {
				typ = typeList[i]
			} else if len(typeList) == 1 {
				typ = typeList[0]
			}
		}
		if typ == "" {
			typ = "正配"
		}
		names = append(names, name)
		types = append(types, typ)
	}
	return names, types
}

func recordVersionTx(tx *sql.Tx, entityType string, entityID int64, snapshot interface{}, changedBy int, changedByName, note string) error {
	b, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if note == "" {
		note = "修改"
	}
	var n int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(version_no),0) FROM entity_versions WHERE entity_type=? AND entity_id=?`,
		entityType, entityID).Scan(&n); err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO entity_versions
		(entity_type, entity_id, version_no, snapshot, changed_by, changed_by_name, note)
		VALUES (?,?,?,?,?,?,?)`,
		entityType, entityID, n+1, string(b), changedBy, changedByName, note)
	return err
}

// memberLite 检查/导入共用的成员轻量索引
type memberLite struct {
	ID         int64
	Name       string
	Generation int
	Gender     int
	FatherID   int64
	MotherID   int64
	BranchID   int64
	RecordType string
	Birth      string
	Death      string
}

func loadMemberLite(db *sql.DB) (map[int64]*memberLite, error) {
	rows, err := db.Query(`SELECT id, COALESCE(name,''), COALESCE(generation,1), COALESCE(gender,1),
		COALESCE(father_id,0), COALESCE(mother_id,0), COALESCE(branch_id,0), COALESCE(record_type,''),
		COALESCE(birth_gregorian,''), COALESCE(death_gregorian,'')
		FROM members ORDER BY generation, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]*memberLite{}
	for rows.Next() {
		m := &memberLite{}
		if err := rows.Scan(&m.ID, &m.Name, &m.Generation, &m.Gender, &m.FatherID, &m.MotherID,
			&m.BranchID, &m.RecordType, &m.Birth, &m.Death); err != nil {
			return nil, err
		}
		out[m.ID] = m
	}
	return out, rows.Err()
}

// fetchImportMemberRow 读取成员整行（含敏感字段/标记，导入更新合并用）
func fetchImportMemberRow(db *sql.DB, id int64) (*Member, error) {
	var m Member
	err := db.QueryRow(`SELECT id, COALESCE(generation,1), COALESCE(branch,''), COALESCE(branch_id,0),
		COALESCE(surname,''), COALESCE(given_name,''), COALESCE(name,''), COALESCE(zi,''), COALESCE(hao,''),
		COALESCE(generation_word,''), COALESCE(rank,''), COALESCE(gender,1),
		COALESCE(father_id,0), COALESCE(mother_id,0), COALESCE(mother_spouse_id,0),
		COALESCE(clan_type,''), COALESCE(record_type,''), COALESCE(foster_father_id,0),
		COALESCE(birth_calendar,'solar'), COALESCE(birth_gregorian,''), COALESCE(birth_lunar,''),
		COALESCE(birth_ganzhi,''), COALESCE(birth_era,''),
		COALESCE(death_gregorian,''), COALESCE(death_lunar,''), COALESCE(death_ganzhi,''), COALESCE(death_era,''),
		COALESCE(lifespan,0), COALESCE(native_place,''), COALESCE(current_place,''), COALESCE(title,''),
		COALESCE(occupation,''), COALESCE(education,''), COALESCE(biography,''), COALESCE(burial_place,''),
		COALESCE(portrait,''), COALESCE(married_out,''), COALESCE(former_name,''), COALESCE(phone,''),
		COALESCE(no_heir,0), COALESCE(lost_contact,0),
		COALESCE(dna_haploid,''), COALESCE(dna_diploid,''), COALESCE(dna_mito,'')
		FROM members WHERE id = ?`, id).Scan(&m.ID, &m.Generation, &m.Branch, &m.BranchID,
		&m.Surname, &m.GivenName, &m.Name, &m.Zi, &m.Hao, &m.GenerationWord, &m.Rank, &m.Gender,
		&m.FatherID, &m.MotherID, &m.MotherSpouseID, &m.ClanType, &m.RecordType, &m.FosterFatherID,
		&m.BirthCalendar, &m.BirthGregorian, &m.BirthLunar, &m.BirthGanzhi, &m.BirthEra,
		&m.DeathGregorian, &m.DeathLunar, &m.DeathGanzhi, &m.DeathEra, &m.Lifespan,
		&m.NativePlace, &m.CurrentPlace, &m.Title, &m.Occupation, &m.Education, &m.Biography,
		&m.BurialPlace, &m.Portrait, &m.MarriedOut, &m.FormerName, &m.Phone,
		&m.NoHeir, &m.LostContact, &m.DNAHaploid, &m.DNADiploid, &m.DNAMito)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}
