package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"familytree/internal/lunar"

	"github.com/gin-gonic/gin"
)

// Member 成员结构体（对应 members 表）
type Member struct {
	ID               int64      `json:"id"`
	Generation       int        `json:"generation"`
	Branch           string     `json:"branch"`
	BranchID         int64      `json:"branch_id"`
	Surname          string     `json:"surname"`
	GivenName        string     `json:"given_name"`
	Name             string     `json:"name"`
	Zi               string     `json:"zi"`
	Hao              string     `json:"hao"`
	GenerationWord   string     `json:"generation_word"`
	Rank             string     `json:"rank"`
	Gender           int        `json:"gender"`
	FatherID         int64      `json:"father_id"`
	MotherID         int64      `json:"mother_id"`
	MotherSpouseID   int64      `json:"mother_spouse_id"`
	MotherSpouseName string     `json:"mother_spouse_name"`
	ClanType         string     `json:"clan_type"`
	RecordType       string     `json:"record_type"`
	FosterFatherID   int64      `json:"foster_father_id"`           // 兼容旧逻辑：= 第一个「嗣父」的成员 id（图谱「出」标记/谱书仍读它）
	BirthFatherText  string     `json:"birth_father_text"`          // 入继：生父在谱外时的文本（如「城东王氏 王某某」）
	Adoptions        []Adoption `json:"adoptions"`                  // 过继/收养关系（嗣父/养父，可多个；兼祧＝多个嗣父）
	AdoptFatherID    int64      `json:"adopt_father_id"`            // 承嗣父（首个「嗣父」，没有则「养父」）——树图双向挂载用
	AdoptKind        string     `json:"adopt_kind"`                 // 承嗣关系：「嗣父」/「养父」
	SonID            int64      `json:"son_id"`                     // 外纪：选儿子联动，非DB字段
	AccountPassword  string     `json:"account_password,omitempty"` // 新增成员时按姓名算出的初始密码（非DB字段，仅在创建响应里返回；账号在首次登录时才建）
	BirthCalendar    string     `json:"birth_calendar"`             // "solar" or "lunar"
	BirthGregorian   string     `json:"birth_gregorian"`
	BirthLunar       string     `json:"birth_lunar"`
	BirthGanzhi      string     `json:"birth_ganzhi"`
	BirthEra         string     `json:"birth_era"`
	DeathGregorian   string     `json:"death_gregorian"`
	DeathLunar       string     `json:"death_lunar"`
	DeathGanzhi      string     `json:"death_ganzhi"`
	DeathEra         string     `json:"death_era"`
	Lifespan         int        `json:"lifespan"`
	NativePlace      string     `json:"native_place"`
	CurrentPlace     string     `json:"current_place"`
	Title            string     `json:"title"`
	Occupation       string     `json:"occupation"`
	Education        string     `json:"education"`
	Biography        string     `json:"biography"`
	BurialPlace      string     `json:"burial_place"`
	Portrait         string     `json:"portrait"`
	MarriedOut       string     `json:"married_out"`
	FormerName       string     `json:"former_name"`
	Phone            string     `json:"phone"`
	DNAHaploid       string     `json:"dna_haploid"`
	DNADiploid       string     `json:"dna_diploid"`
	DNAMito          string     `json:"dna_mito"`
	HasBio           bool       `json:"has_bio"`      // 是否有名人传记（世系图右上角金色卷轴标记）
	NoHeir           int        `json:"no_heir"`      // 无嗣（1=无嗣，禁止新增子女，图谱标注「无嗣」）
	LostContact      int        `json:"lost_contact"` // 迁居外地失联（1=存殁未详，不进生日/祭日，图谱标注「存殁未详，子女未详」）
}

// MemberHandler 成员处理器
type MemberHandler struct {
	DataDir string
	DB      *sql.DB
	EncKey  string // AES-256 加密密钥（hex 编码）
}

// List 返回全部成员
func (h *MemberHandler) List(c *gin.Context) {
	branchID, _ := strconv.ParseInt(c.Query("branch_id"), 10, 64)
	includeChildren := c.Query("include_children") == "1" || c.Query("include_children") == "true"

	q := `SELECT id, generation, branch, branch_id, surname, given_name, name, zi, hao, generation_word,
		rank, gender, father_id, mother_id, mother_spouse_id, clan_type, record_type, foster_father_id, birth_father_text,
		birth_calendar, birth_gregorian, birth_lunar, birth_ganzhi, birth_era, death_gregorian, death_lunar, death_ganzhi, death_era,
		lifespan, native_place, current_place, title, occupation, education, biography, burial_place, portrait, married_out, former_name, phone, dna_haploid, dna_diploid, dna_mito
		FROM members`
	args := []interface{}{}
	if branchID > 0 {
		done := false
		if includeChildren {
			var p string
			if err := h.DB.QueryRow(`SELECT COALESCE(path,'') FROM branches WHERE id=?`, branchID).Scan(&p); err == nil && p != "" {
				q += ` WHERE branch_id IN (SELECT id FROM branches WHERE path = ? OR path LIKE ?)`
				args = append(args, p, p+".%")
				done = true
			}
		}
		if !done {
			q += ` WHERE branch_id = ?`
			args = append(args, branchID)
		}
	}
	q += ` ORDER BY generation, id`
	rows, err := h.DB.Query(q, args...)
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
			&m.ClanType, &m.RecordType, &m.FosterFatherID, &m.BirthFatherText, &m.BirthCalendar, &m.BirthGregorian, &m.BirthLunar, &m.BirthGanzhi, &m.BirthEra,
			&m.DeathGregorian, &m.DeathLunar, &m.DeathGanzhi, &m.DeathEra, &m.Lifespan,
			&m.NativePlace, &m.CurrentPlace, &m.Title, &m.Occupation, &m.Education, &m.Biography, &m.BurialPlace, &m.Portrait, &m.MarriedOut, &m.FormerName, &m.Phone, &m.DNAHaploid, &m.DNADiploid, &m.DNAMito); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		members = append(members, m)
	}
	// 批量附加过继关系（树图要用「承嗣父」做双向挂载）
	if len(members) > 0 {
		adoptByMember := map[int64][]Adoption{}
		if ars, aerr := h.DB.Query(`SELECT a.member_id, a.father_id, COALESCE(f.name,''), COALESCE(a.father_text,''), COALESCE(a.kind,'嗣父')
			FROM member_adoptions a LEFT JOIN members f ON f.id = a.father_id ORDER BY a.member_id, a.sort_order, a.id`); aerr == nil {
			for ars.Next() {
				var mid int64
				var a Adoption
				if ars.Scan(&mid, &a.FatherID, &a.FatherName, &a.FatherText, &a.MotherID, &a.MotherName, &a.Kind) == nil {
					adoptByMember[mid] = append(adoptByMember[mid], a)
				}
			}
			ars.Close()
		}
		for i := range members {
			if ads, ok := adoptByMember[members[i].ID]; ok {
				members[i].Adoptions = ads
			}
			members[i].fillAdoptFather()
		}
	}

	// 解密敏感字段（出库后解密，再脱敏）
	DecryptMembers(members, h.EncKey)
	attachMemberFlags(h.DB, members)
	bioSet := bioFlagSet(h.DB)
	for i := range members {
		members[i].HasBio = bioSet[members[i].ID]
	}
	viewerID := FindMemberByName(h.DB, c.GetString("username"))
	SanitizeMembers(members, c.GetString("role"), viewerID, c.GetString("branch"), h.DB)
	c.JSON(http.StatusOK, members)
}

// Get 返回单个成员
func (h *MemberHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var m Member
	err := h.DB.QueryRow(`SELECT id, generation, branch, branch_id, surname, given_name, name, zi, hao, generation_word,
		rank, gender, father_id, mother_id, mother_spouse_id, clan_type, record_type, foster_father_id, birth_father_text,
		birth_calendar, birth_gregorian, birth_lunar, birth_ganzhi, birth_era, death_gregorian, death_lunar, death_ganzhi, death_era,
		lifespan, native_place, current_place, title, occupation, education, biography, burial_place, portrait, married_out, former_name, phone, dna_haploid, dna_diploid, dna_mito
		FROM members WHERE id = ?`, id).Scan(&m.ID, &m.Generation, &m.Branch, &m.BranchID, &m.Surname, &m.GivenName, &m.Name, &m.Zi, &m.Hao,
		&m.GenerationWord, &m.Rank, &m.Gender, &m.FatherID, &m.MotherID, &m.MotherSpouseID,
		&m.ClanType, &m.RecordType, &m.FosterFatherID, &m.BirthFatherText, &m.BirthCalendar, &m.BirthGregorian, &m.BirthLunar, &m.BirthGanzhi, &m.BirthEra,
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

	// 解密敏感字段
	m.DecryptSensitive(h.EncKey)

	// 过继/收养关系（嗣父/养父，可多个）
	m.Adoptions = loadAdoptions(h.DB, m.ID)
	m.fillAdoptFather()

	// 隐私脱敏（超管/理事长/理事全可见，房支理事按房支，族员按五服）
	role := c.GetString("role")
	viewerID := FindMemberByName(h.DB, c.GetString("username"))
	SanitizeMember(&m, nil, role, viewerID, c.GetString("branch"), h.DB)

	// 访问审计：仅记录会触发脱敏的角色（非超管/理事长/理事）的敏感信息查看
	if viewerID > 0 && viewerSensitivity(role) < SensAdmin {
		distance := RelationDistance(h.DB, viewerID, id)
		LogPrivacyAccess(h.DB, c.GetString("username"), viewerID, id, distance)
	}

	c.JSON(http.StatusOK, m)
}

// Create 新增成员
func (h *MemberHandler) Create(c *gin.Context) {
	var m Member
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 房支隔离：理事只能新增到自己房支或未分配
	if !checkCreateBranchAccess(c, h.DB, m.BranchID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权向其他房支新增成员"})
		return
	}
	// DNA 编码仅超级管理员/理事长可录入
	if r := c.GetString("role"); r != RoleSuperAdmin && r != RoleChief {
		m.DNAHaploid = ""
		m.DNADiploid = ""
		m.DNAMito = ""
	}
	// 理事提交 → 进入审核队列（理事长/超级管理员直接写库）
	if isElder(c) {
		submitDraft(h.DB, "member", "create", 0, m, c)
		return
	}
	uid, uname := currentUser(c)
	created, err := h.applyCreate(&m, uid, uname)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, created)
}

// Adoption 过继/收养关系（一人可多条：兼祧＝多个嗣父；养子＝养父）
type Adoption struct {
	ID         int64  `json:"id"`
	FatherID   int64  `json:"father_id"`   // 嗣父/养父（本谱成员 id，0＝谱外）
	FatherName string `json:"father_name"` // 只读：联表取的名字
	FatherText string `json:"father_text"` // 谱外对象的姓名/出处
	MotherID   int64  `json:"mother_id"`   // 嗣母/养母（本谱成员 id，0=未填）
	MotherName string `json:"mother_name"` // 只读：联表取的名字
	Kind       string `json:"kind"`        // 嗣父 / 养父
	Note       string `json:"note"`
}

// fillAdoptFather 依据过继关系算出「承嗣父」：优先第一个本谱「嗣父」，其次「养父」
func (m *Member) fillAdoptFather() {
	m.AdoptFatherID = 0
	m.AdoptKind = ""
	for _, a := range m.Adoptions {
		if a.FatherID > 0 && a.Kind == "嗣父" {
			m.AdoptFatherID, m.AdoptKind = a.FatherID, "嗣父"
			return
		}
	}
	for _, a := range m.Adoptions {
		if a.FatherID > 0 && a.Kind == "养父" {
			m.AdoptFatherID, m.AdoptKind = a.FatherID, "养父"
			return
		}
	}
}

// saveAdoptions 用提交上来的列表整体替换某成员的过继/收养关系，
// 并把「第一个本谱嗣父」回写到 members.foster_father_id（老逻辑：图谱「出」标记、谱书都读它）。
func saveAdoptions(db *sql.DB, memberID int64, list []Adoption) {
	db.Exec(`DELETE FROM member_adoptions WHERE member_id = ?`, memberID)
	firstFoster := int64(0)
	order := 0
	for _, a := range list {
		kind := strings.TrimSpace(a.Kind)
		if kind == "" {
			kind = "嗣父"
		}
		ft := strings.TrimSpace(a.FatherText)
		if a.FatherID == 0 && ft == "" {
			continue // 空行
		}
		db.Exec(`INSERT INTO member_adoptions (member_id, father_id, father_text, kind, mother_id, note, sort_order)
			VALUES (?,?,?,?,?,?,?)`, memberID, a.FatherID, ft, kind, a.MotherID, strings.TrimSpace(a.Note), order)
		order++
		if kind == "嗣父" && a.FatherID > 0 && firstFoster == 0 {
			firstFoster = a.FatherID
		}
	}
	db.Exec(`UPDATE members SET foster_father_id = ? WHERE id = ?`, firstFoster, memberID)
}

// loadAdoptions 读取某成员的全部过继/收养关系（含嗣父姓名）
func loadAdoptions(db *sql.DB, memberID int64) []Adoption {
	out := []Adoption{}
	rows, err := db.Query(`SELECT a.id, a.father_id, COALESCE(m.name,''), a.father_text, COALESCE(a.mother_id,0), COALESCE(mo.name,''), a.kind, a.note
		FROM member_adoptions a
		LEFT JOIN members m ON m.id = a.father_id
		LEFT JOIN members mo ON mo.id = a.mother_id
		WHERE a.member_id = ? ORDER BY a.sort_order, a.id`, memberID)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var a Adoption
		if rows.Scan(&a.ID, &a.FatherID, &a.FatherName, &a.FatherText, &a.MotherID, &a.MotherName, &a.Kind, &a.Note) == nil {
			out = append(out, a)
		}
	}
	return out
}

// applyAdoptions 保存过继关系：请求里带了 adoptions（哪怕是空数组）才整体替换；
// 没带（例如成员列表那种精简表单）就保持原样，避免把已有关系清空。
// 新增时如果只给了旧的 foster_father_id，也补一条「嗣父」。
func applyAdoptions(db *sql.DB, memberID int64, m *Member) {
	if m.Adoptions == nil {
		if m.FosterFatherID > 0 {
			saveAdoptions(db, memberID, []Adoption{{FatherID: m.FosterFatherID, Kind: "嗣父"}})
		}
		return
	}
	saveAdoptions(db, memberID, m.Adoptions)
}

// applyCreate 实际执行新增成员（供直接写入与审核通过合并共用）
func (h *MemberHandler) applyCreate(m *Member, changedBy int, changedByName string) (*Member, error) {
	// 如果姓氏为空，从 settings 表读取 family_surname 并自动填充
	if m.Surname == "" {
		var familySurname string
		h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'family_surname'`).Scan(&familySurname)
		m.Surname = familySurname
	}
	// 自动设置全名 = 姓氏 + 名字
	m.Name = m.Surname + m.GivenName
	// 自动继承父亲的房支：新增成员未指定房支时，继承其生父的房支归属
	if m.FatherID > 0 && m.BranchID == 0 && strings.TrimSpace(m.Branch) == "" {
		var fbid int64
		if err := h.DB.QueryRow(`SELECT COALESCE(branch_id,0) FROM members WHERE id = ?`, m.FatherID).Scan(&fbid); err == nil && fbid > 0 {
			m.BranchID = fbid
		}
	}
	// 同步房支：若 branch_id 给定则回填 branch 名称；否则若 branch 文本能匹配则回填 branch_id
	m.syncBranch(h.DB)
	// 自动根据名字和字辈派语适配字辈
	if m.GenerationWord == "" {
		m.GenerationWord = h.detectGenerationWord(m.GivenName, m.Branch, m.Generation)
	}
	// 自动填充农历和年号信息
	m.fillLunarInfo()
	// 加密敏感字段（入库）
	m.EncryptSensitive(h.EncKey)
	res, err := h.DB.Exec(`INSERT INTO members (generation, branch, branch_id, surname, given_name, name, zi, hao, generation_word,
		rank, gender, father_id, mother_id, mother_spouse_id, clan_type, record_type, foster_father_id, birth_father_text,
		birth_calendar, birth_gregorian, birth_lunar, birth_ganzhi, birth_era, death_gregorian, death_lunar, death_ganzhi, death_era,
		lifespan, native_place, current_place, title, occupation, education, biography, burial_place, portrait, married_out, former_name, phone, dna_haploid, dna_diploid, dna_mito)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.Generation, m.Branch, m.BranchID, m.Surname, m.GivenName, m.Name, m.Zi, m.Hao, m.GenerationWord,
		m.Rank, m.Gender, m.FatherID, m.MotherID, m.MotherSpouseID, m.ClanType, m.RecordType, m.FosterFatherID, m.BirthFatherText,
		m.BirthCalendar, m.BirthGregorian, m.BirthLunar, m.BirthGanzhi, m.BirthEra, m.DeathGregorian, m.DeathLunar, m.DeathGanzhi, m.DeathEra,
		m.Lifespan, m.NativePlace, m.CurrentPlace, m.Title, m.Occupation, m.Education, m.Biography, m.BurialPlace, m.Portrait, m.MarriedOut, m.FormerName, m.Phone, m.DNAHaploid, m.DNADiploid, m.DNAMito)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	m.ID = id
	// 外纪：选儿子联动，自动回填儿子的生父 + 生母
	if m.SonID > 0 {
		h.DB.Exec(`UPDATE members SET father_id=? WHERE id=?`, id, m.SonID)
		// 同时回填生母：取外纪成员的第一个正配
		var motherSpouseID sql.NullInt64
		h.DB.QueryRow(`SELECT id FROM spouses WHERE member_id=? AND type='正配' ORDER BY id LIMIT 1`, id).Scan(&motherSpouseID)
		if motherSpouseID.Valid && motherSpouseID.Int64 > 0 {
			h.DB.Exec(`UPDATE members SET mother_spouse_id=? WHERE id=?`, motherSpouseID.Int64, m.SonID)
		}
	}
	// 过继/收养关系（嗣父/养父，可多个）
	applyAdoptions(h.DB, id, m)
	// 记录版本：新增即 v1 快照
	_ = recordVersion(h.DB, "member", id, m, changedBy, changedByName, "新增")
	// 新增成员：只按姓名算出初始密码返回给前端展示（不建号；首次登录时才自动建号）
	if n := strings.TrimSpace(m.Name); n != "" {
		m.AccountPassword = deriveNamePassword(n, memberPwSalt(h.DB))
	}
	return m, nil
}

// Update 更新成员
func (h *MemberHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var m Member
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 房支隔离：理事只能改自己房支的成员，且不能把成员转到其他房支
	if !checkMemberBranchAccess(c, h.DB, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权修改其他房支的成员"})
		return
	}
	if !checkCreateBranchAccess(c, h.DB, m.BranchID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权将成员转入其他房支"})
		return
	}
	// DNA 编码仅超级管理员/理事长可修改，其余角色保持库中原值
	if r := c.GetString("role"); r != RoleSuperAdmin && r != RoleChief {
		var e1, e2, e3 string
		if err := h.DB.QueryRow(`SELECT COALESCE(dna_haploid,''), COALESCE(dna_diploid,''), COALESCE(dna_mito,'') FROM members WHERE id = ?`, id).Scan(&e1, &e2, &e3); err == nil {
			m.DNAHaploid = DecryptValue(e1, h.EncKey)
			m.DNADiploid = DecryptValue(e2, h.EncKey)
			m.DNAMito = DecryptValue(e3, h.EncKey)
		}
	}
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "member", "update", id, m, c)
		return
	}
	uid, uname := currentUser(c)
	if err := h.applyUpdate(id, &m, uid, uname); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, m)
}

// applyUpdate 实际执行更新成员
func (h *MemberHandler) applyUpdate(id int64, m *Member, changedBy int, changedByName string) error {
	var oldBranchID int64
	// 记录修改前快照（若有旧记录）
	if old, err := fetchMemberRow(h.DB, id); err == nil {
		oldBranchID = old.BranchID
		_ = recordVersion(h.DB, "member", id, old, changedBy, changedByName, "修改")
	}
	// 自动设置全名 = 姓氏 + 名字
	m.Name = m.Surname + m.GivenName
	// 同步房支：若 branch_id 给定则回填 branch 名称；否则若 branch 文本能匹配则回填 branch_id
	m.syncBranch(h.DB)
	// 自动根据名字和字辈派语适配字辈
	if m.GenerationWord == "" {
		m.GenerationWord = h.detectGenerationWord(m.GivenName, m.Branch, m.Generation)
	}
	// 自动填充农历和年号信息
	m.fillLunarInfo()
	// 加密敏感字段（入库）
	m.EncryptSensitive(h.EncKey)
	_, err := h.DB.Exec(`UPDATE members SET generation=?, branch=?, branch_id=?, surname=?, given_name=?, name=?, zi=?, hao=?, generation_word=?,
		rank=?, gender=?, father_id=?, mother_id=?, mother_spouse_id=?, clan_type=?, record_type=?, foster_father_id=?, birth_father_text=?,
		birth_calendar=?, birth_gregorian=?, birth_lunar=?, birth_ganzhi=?, birth_era=?, death_gregorian=?, death_lunar=?, death_ganzhi=?, death_era=?,
		lifespan=?, native_place=?, current_place=?, title=?, occupation=?, education=?, biography=?, burial_place=?, portrait=?, married_out=?, former_name=?, phone=?, dna_haploid=?, dna_diploid=?, dna_mito=?,
		updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		m.Generation, m.Branch, m.BranchID, m.Surname, m.GivenName, m.Name, m.Zi, m.Hao, m.GenerationWord,
		m.Rank, m.Gender, m.FatherID, m.MotherID, m.MotherSpouseID, m.ClanType, m.RecordType, m.FosterFatherID, m.BirthFatherText,
		m.BirthCalendar, m.BirthGregorian, m.BirthLunar, m.BirthGanzhi, m.BirthEra, m.DeathGregorian, m.DeathLunar, m.DeathGanzhi, m.DeathEra,
		m.Lifespan, m.NativePlace, m.CurrentPlace, m.Title, m.Occupation, m.Education, m.Biography, m.BurialPlace, m.Portrait, m.MarriedOut, m.FormerName, m.Phone, m.DNAHaploid, m.DNADiploid, m.DNAMito, id)
	if err != nil {
		return err
	}
	m.ID = id
	// 外纪：选儿子联动，自动回填儿子的生父 + 生母
	if m.SonID > 0 {
		h.DB.Exec(`UPDATE members SET father_id=? WHERE id=?`, id, m.SonID)
		// 同时回填生母：取外纪成员的第一个正配
		var motherSpouseID sql.NullInt64
		h.DB.QueryRow(`SELECT id FROM spouses WHERE member_id=? AND type='正配' ORDER BY id LIMIT 1`, id).Scan(&motherSpouseID)
		if motherSpouseID.Valid && motherSpouseID.Int64 > 0 {
			h.DB.Exec(`UPDATE members SET mother_spouse_id=? WHERE id=?`, motherSpouseID.Int64, m.SonID)
		}
	}
	// 过继/收养关系（嗣父/养父，可多个）
	applyAdoptions(h.DB, id, m)
	// 房支变更：级联到后代（跟随旧房支或未分房的后代一起改）
	if oldBranchID != m.BranchID {
		cascadeBranchToDescendants(h.DB, id, oldBranchID, m.BranchID, m.Branch)
	}
	return nil
}

// Rename 改名：仅可修改名字（姓氏不可改），同步更新同名登录账号，旧名存入 former_name
// PUT /api/members/:id/rename
// 请求体：{ "given_name": "新名" }（surname 字段忽略，强制沿用原姓）
func (h *MemberHandler) Rename(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		GivenName string `json:"given_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	req.GivenName = strings.TrimSpace(req.GivenName)
	if req.GivenName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名不能为空"})
		return
	}
	// 房支隔离：理事只能改自己房支的成员
	if !checkMemberBranchAccess(c, h.DB, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权修改其他房支的成员"})
		return
	}
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "member", "rename", id, req, c)
		return
	}
	uid, uname := currentUser(c)
	res, err := h.applyRename(id, req.GivenName, uid, uname)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "成员不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// applyRename 实际执行改名，返回结果 map
func (h *MemberHandler) applyRename(id int64, givenName string, changedBy int, changedByName string) (gin.H, error) {
	// 读取当前成员
	var oldName, oldSurname, formerName string
	err := h.DB.QueryRow(`SELECT name, surname, COALESCE(former_name,'') FROM members WHERE id = ?`, id).
		Scan(&oldName, &oldSurname, &formerName)
	if err == sql.ErrNoRows {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	// 姓氏不可改，沿用原姓
	newName := oldSurname + givenName

	// 曾用名：旧名若与新名不同且尚未记录，则追加
	if oldName != "" && oldName != newName {
		if formerName == "" {
			formerName = oldName
		} else {
			parts := strings.Split(formerName, "、")
			exists := false
			for _, p := range parts {
				if strings.TrimSpace(p) == oldName {
					exists = true
					break
				}
			}
			if !exists {
				formerName = formerName + "、" + oldName
			}
		}
	}

	// 记录改名前的快照
	if old, err := fetchMemberRow(h.DB, id); err == nil {
		_ = recordVersion(h.DB, "member", id, old, changedBy, changedByName, "改名")
	}

	// 更新成员姓名与曾用名
	_, err = h.DB.Exec(`UPDATE members SET surname=?, given_name=?, name=?, former_name=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		oldSurname, givenName, newName, formerName, id)
	if err != nil {
		return nil, err
	}

	// 同步更新同名登录账号（若有）
	res, err := h.DB.Exec(`UPDATE users SET username=? WHERE username=?`, newName, oldName)
	userRenamed := false
	if err == nil {
		if n, _ := res.RowsAffected(); n > 0 {
			userRenamed = true
		}
	}

	return gin.H{
		"ok":           true,
		"id":           id,
		"name":         newName,
		"former_name":  formerName,
		"user_renamed": userRenamed,
	}, nil
}

// Delete 删除成员
func (h *MemberHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	// 房支隔离：理事只能删自己房支的成员
	if !checkMemberBranchAccess(c, h.DB, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除其他房支的成员"})
		return
	}
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "member", "delete", id, gin.H{"id": id}, c)
		return
	}
	uid, uname := currentUser(c)
	if err := h.applyDelete(id, uid, uname); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// applyDelete 实际执行删除成员（含级联）
func (h *MemberHandler) applyDelete(id int64, changedBy int, changedByName string) error {
	// 记录删除前快照
	if old, err := fetchMemberRow(h.DB, id); err == nil {
		_ = recordVersion(h.DB, "member", id, old, changedBy, changedByName, "删除")
	}
	// 级联删除配偶
	h.DB.Exec(`DELETE FROM spouses WHERE member_id = ?`, id)
	// 级联删除资料来源（成员页区块，独立子表；不留孤儿行）
	h.DB.Exec(`DELETE FROM member_sources WHERE member_id = ?`, id)
	// 级联删除附件（含物理文件）
	rows, _ := h.DB.Query(`SELECT path FROM attachments WHERE member_id = ?`, id)
	if rows != nil {
		for rows.Next() {
			var p string
			if rows.Scan(&p) == nil && p != "" {
				absPath := filepath.Join(h.DataDir, "upload", filepath.Base(p))
				os.Remove(absPath)
			}
		}
		rows.Close()
	}
	h.DB.Exec(`DELETE FROM attachments WHERE member_id = ?`, id)
	_, err := h.DB.Exec(`DELETE FROM members WHERE id = ?`, id)
	return err
}

// syncBranch 同步房支字段：
//   - 若 branch_id > 0：用该房支名称回填 branch 文本
//   - 否则若 branch 文本非空：尝试按名称匹配房支表回填 branch_id
func (m *Member) syncBranch(db *sql.DB) {
	if m.BranchID > 0 {
		var name string
		if err := db.QueryRow(`SELECT name FROM branches WHERE id = ?`, m.BranchID).Scan(&name); err == nil && name != "" {
			m.Branch = name
		}
		return
	}
	b := strings.TrimSpace(m.Branch)
	if b != "" {
		var id int64
		if err := db.QueryRow(`SELECT id FROM branches WHERE name = ?`, b).Scan(&id); err == nil {
			m.BranchID = id
		}
	}
}

// cascadeBranchToDescendants 将某成员的房支变更级联到其后代：
// 从 memberID 出发，凡「跟随旧房支(branch_id==oldBid)或未分房(branch_id==0)」的后代都改为新分支；
// 遇到已是其它房支的后代则停止该支（分房保护）。返回被更新的后代数。
func cascadeBranchToDescendants(db *sql.DB, memberID, oldBid, newBid int64, newName string) int {
	count := 0
	queue := []int64{memberID}
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		rows, err := db.Query(`SELECT id, COALESCE(branch_id,0) FROM members WHERE father_id = ?`, parent)
		if err != nil {
			continue
		}
		type child struct {
			id  int64
			bid int64
		}
		var children []child
		for rows.Next() {
			var c child
			if rows.Scan(&c.id, &c.bid) == nil {
				children = append(children, c)
			}
		}
		rows.Close()
		for _, c := range children {
			if c.bid == oldBid || c.bid == 0 {
				db.Exec(`UPDATE members SET branch_id=?, branch=? WHERE id=?`, newBid, newName, c.id)
				count++
				queue = append(queue, c.id)
			}
		}
	}
	return count
}

// detectGenerationWord 根据名字和字辈派语自动适配字辈
// 规则：
//  1. 根据成员的世代（generation）和字辈派语的起始世代（start_generation），
//     算出该成员应使用的第 N 个字辈字；
//  2. 如果名字（given_name）的首字正好是该字辈字，则返回该字；
//  3. 否则扫描所有字辈派语，若名字首字匹配某个字辈字，返回该字；
//  4. 都不匹配则返回空字符串。
func (h *MemberHandler) detectGenerationWord(givenName, branch string, generation int) string {
	if givenName == "" {
		return ""
	}
	firstRune := []rune(givenName)[0]

	rows, err := h.DB.Query(`SELECT branch, words_array, start_generation FROM generation_words ORDER BY id`)
	if err != nil {
		return ""
	}
	defer rows.Close()

	type gw struct {
		Branch          string
		WordsArray      string
		StartGeneration int
	}
	var list []gw
	for rows.Next() {
		var g gw
		if err := rows.Scan(&g.Branch, &g.WordsArray, &g.StartGeneration); err != nil {
			continue
		}
		list = append(list, g)
	}

	for _, g := range list {
		// 支系过滤：如果字辈派语有支系且与成员支系不匹配，跳过
		if g.Branch != "" && branch != "" && g.Branch != branch {
			continue
		}
		words := parseWordsArray(g.WordsArray)
		if len(words) == 0 {
			continue
		}
		// 根据世代计算应使用的字辈字索引
		idx := generation - g.StartGeneration
		if idx >= 0 && idx < len(words) {
			if string(firstRune) == words[idx] {
				return words[idx]
			}
		}
		// 兜底：扫描该派语所有字，看名字首字是否匹配
		for _, w := range words {
			if string(firstRune) == w {
				return w
			}
		}
	}
	return ""
}

// parseWordsArray 解析字辈数组（JSON 数组或逗号分隔字符串）
func parseWordsArray(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var arr []string
	if strings.HasPrefix(s, "[") {
		if err := json.Unmarshal([]byte(s), &arr); err == nil {
			return arr
		}
	}
	// 逗号/顿号分隔
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '，' || r == '、' || r == ' ' || r == '\n'
	})
	return fields
}

// fillLunarInfo 根据公历日期自动填充农历、干支和年号信息
func (m *Member) fillLunarInfo() {
	m.BirthGregorian = strings.TrimSpace(m.BirthGregorian)
	m.DeathGregorian = strings.TrimSpace(m.DeathGregorian)
	// 处理出生日期：有公历日期则重算农历、干支和年号（保证改公历后关联字段同步更新）
	if m.BirthGregorian != "" {
		if t, err := time.Parse("2006-01-02", m.BirthGregorian); err == nil {
			info := lunar.FromGregorian(t.Year(), int(t.Month()), t.Day())
			m.BirthLunar = info.Lunar
			m.BirthGanzhi = info.Ganzhi
			m.BirthEra = info.Era + info.EraYear
		} else if y, err2 := strconv.Atoi(m.BirthGregorian); err2 == nil {
			// 只填年份：只算年号（农历/干支需月日，留空）
			info := lunar.FromGregorianYear(y)
			m.BirthEra = info.Era + info.EraYear
		}
	}
	// 处理逝世日期
	if m.DeathGregorian != "" {
		if t, err := time.Parse("2006-01-02", m.DeathGregorian); err == nil {
			info := lunar.FromGregorian(t.Year(), int(t.Month()), t.Day())
			m.DeathLunar = info.Lunar
			m.DeathGanzhi = info.Ganzhi
			m.DeathEra = info.Era + info.EraYear
		} else if y, err2 := strconv.Atoi(m.DeathGregorian); err2 == nil {
			info := lunar.FromGregorianYear(y)
			m.DeathEra = info.Era + info.EraYear
		}
	}
	// 自动计算享年：逝世年份 - 出生年份 + 1（虚岁，中国传统）
	// 生年或卒年任一缺失时清空享年（避免删除生卒年后享年残留）
	// 支持完整日期（YYYY-MM-DD）或纯年份（YYYY）
	m.Lifespan = 0
	by, ok1 := extractYear(m.BirthGregorian)
	dy, ok2 := extractYear(m.DeathGregorian)
	if ok1 && ok2 && dy >= by {
		m.Lifespan = dy - by + 1
	}
}

// extractYear 从公历日期字符串提取年份（支持 YYYY-MM-DD 或纯 YYYY）
func extractYear(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Year(), true
	}
	if y, err := strconv.Atoi(s); err == nil {
		return y, true
	}
	return 0, false
}
