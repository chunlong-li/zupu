package handler

import (
	"bufio"
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"familytree/internal/gedcom"

	"github.com/gin-gonic/gin"
)

// GedcomHandler GEDCOM 导入导出
type GedcomHandler struct {
	DB *sql.DB
}

// gedcomParser 解析 GEDCOM 5.5.1 文本
type gedcomParser struct {
	lines    []string
	pos      int
	warnings []string
}

func newGedcomParser(text string) *gedcomParser {
	return &gedcomParser{lines: strings.Split(text, "\n"), pos: 0}
}

func (p *gedcomParser) warn(msg string) {
	p.warnings = append(p.warnings, msg)
}

// Import 导入 GEDCOM 文件
// POST /api/gedcom/import
func (h *GedcomHandler) Import(c *gin.Context) {
	content, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取上传内容失败"})
		return
	}

	parser := newGedcomParser(string(content))
	people := parser.parse()

	salt := memberPwSalt(h.DB)
	type namePw struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	accounts := make([]namePw, 0, len(people))
	imported, skipped, restored, linked := 0, 0, 0, 0

	// @I..@ → 解析对象（用于世代推算）
	byXid := map[string]*gedcomPerson{}
	for i := range people {
		if people[i].xid != "" {
			byXid[people[i].xid] = &people[i]
		}
	}
	// 世代：优先 _GEN，否则按父母链递推（无父母 = 第一世）
	genMemo := map[string]int{}
	var calcGen func(xid string, stack map[string]bool) int
	calcGen = func(xid string, stack map[string]bool) int {
		if g, ok := genMemo[xid]; ok {
			return g
		}
		pp := byXid[xid]
		if pp == nil || stack[xid] {
			return 0
		}
		if pp.gen > 0 {
			genMemo[xid] = pp.gen
			return pp.gen
		}
		stack[xid] = true
		g := 0
		for _, px := range []string{pp.fatherX, pp.motherX} {
			if px == "" {
				continue
			}
			if pg := calcGen(px, stack); pg > g {
				g = pg
			}
		}
		delete(stack, xid)
		if g == 0 {
			g = 1
		}
		genMemo[xid] = g
		return g
	}
	for i := range people {
		if people[i].xid != "" {
			calcGen(people[i].xid, map[string]bool{})
		}
	}

	// @I..@ → 本库成员 id
	idByXid := map[string]int64{}
	for i := range people {
		person := &people[i]
		m := personToMember(*person)
		if m.Name == "" {
			continue
		}
		if m.Generation <= 0 {
			if g := genMemo[person.xid]; g > 0 {
				m.Generation = g
			} else {
				m.Generation = 1
			}
		}

		// 同名去重：库里已有同名成员 → 复用其 id，不重复插入
		var existID int64
		if e := h.DB.QueryRow(`SELECT id FROM members WHERE name = ? LIMIT 1`, m.Name).Scan(&existID); e == nil && existID > 0 {
			skipped++
			if person.xid != "" {
				idByXid[person.xid] = existID
			}
		} else {
			res, err := h.DB.Exec(`INSERT INTO members (generation, branch, rank, surname, given_name, name, zi, gender, father_id, mother_id,
				clan_type, birth_gregorian, birth_lunar, birth_ganzhi,
				death_gregorian, death_lunar, death_ganzhi,
				native_place, title, biography)
				VALUES (?,?,?,?,?,?,?,?,0,0,?,?,?,?,?,?,?,?,?,?)`,
				m.Generation, m.Branch, m.Rank, m.Surname, m.GivenName, m.Name, m.Zi, m.Gender,
				m.ClanType, m.BirthGregorian, m.BirthLunar, m.BirthGanzhi,
				m.DeathGregorian, m.DeathLunar, m.DeathGanzhi,
				m.NativePlace, m.Title, m.Biography)
			if err != nil {
				parser.warn(fmt.Sprintf("导入 %s 失败: %s", m.Name, err.Error()))
				continue
			}
			id, _ := res.LastInsertId()
			if person.xid != "" {
				idByXid[person.xid] = id
			}
			imported++
			// 导入配偶
			for _, s := range person.spouses {
				h.DB.Exec(`INSERT INTO spouses (member_id, name, type, family) VALUES (?,?,?,?)`,
					id, s.Name, s.Type, s.Family)
			}
		}
		// 账号处理：
		//   1) 备份文件里带账号（_ACCT/_PWHASH）→ 直接还原原密码哈希，不重新算；
		//   2) 本库已有该账号 → 保留原密码，不重新算、也不进入初始密码列表；
		//   3) 都没有 → 给出按姓名派生的初始密码（首次登录时才建号）。
		username := m.Name
		if person.acct != "" {
			username = person.acct
		}
		var exists int
		h.DB.QueryRow(`SELECT COUNT(1) FROM users WHERE username = ?`, username).Scan(&exists)
		if person.acct != "" && person.pwHash != "" {
			if exists == 0 {
				role := restorableRole(person.acRole)
				nc := person.pwChg
				if nc < 0 {
					nc = 0
				}
				if _, e := h.DB.Exec(`INSERT INTO users (username, password, role, branch, need_change_pw) VALUES (?,?,?,'',?)`,
					username, person.pwHash, role, nc); e == nil {
					restored++
				} else {
					parser.warn(fmt.Sprintf("恢复 %s 账号失败: %s", username, e.Error()))
				}
			}
		} else if exists == 0 {
			accounts = append(accounts, namePw{Name: username, Password: deriveNamePassword(m.Name, salt)})
		}
	}

	// 回填父母关系（仅对当前无父母的新建记录，避免覆盖已有数据）
	for i := range people {
		person := &people[i]
		if person.xid == "" {
			continue
		}
		mid, ok := idByXid[person.xid]
		if !ok || mid == 0 {
			continue
		}
		var fid, moid int64
		if person.fatherX != "" {
			fid = idByXid[person.fatherX]
		}
		if person.motherX != "" {
			moid = idByXid[person.motherX]
		}
		if fid == 0 && moid == 0 {
			continue
		}
		if _, e := h.DB.Exec(`UPDATE members SET father_id=?, mother_id=? WHERE id=? AND father_id=0 AND mother_id=0`,
			fid, moid, mid); e == nil {
			linked++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"imported": imported,
		"skipped":  skipped,
		"restored": restored,
		"linked":   linked,
		"warnings": parser.warnings,
		"accounts": accounts,
	})
}

// restorableRole 导入时允许还原的角色（不还原超级管理员，避免通过导入文件提权；账号非成员绑定）
func restorableRole(role string) string {
	switch role {
	case RoleMember, RoleElder, RoleBranchElder, RoleFinance, RoleChief:
		return role
	default:
		return RoleMember
	}
}

// GenerateGedcom 生成 GEDCOM 5.5.1 文本（不含账号，兼容旧调用，委托到 gedcom 包）
func GenerateGedcom(db *sql.DB) (string, error) {
	return gedcom.Generate(db)
}

// GenerateGedcomWithAccounts 生成含成员账号（密码哈希）的 GEDCOM，用于带账号密码的备份
func GenerateGedcomWithAccounts(db *sql.DB) (string, error) {
	return gedcom.GenerateWithAccounts(db, true)
}

// Export 导出 GEDCOM 5.5.1（直接下载）
// GET /api/gedcom/export
func (h *GedcomHandler) Export(c *gin.Context) {
	// 仅理事长/超级管理员导出时附带账号密码哈希（能导也能导回；普通成员导出不含账号信息）
	role := c.GetString("role")
	withAccounts := role == RoleChief || role == RoleSuperAdmin
	text, err := gedcom.GenerateWithAccounts(h.DB, withAccounts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=familytree_%s.ged", time.Now().Format("20060102")))
	c.String(http.StatusOK, text)
}

// gedcomPerson 解析后的个人记录
type gedcomPerson struct {
	xid     string // 记录标识 @I..@
	name    string
	zi      string
	hao     string
	sex     string
	birth   string
	death   string
	note    string
	occup   string
	place   string
	famc    string   // 父母家庭 @F..@
	fams    []string // 自己组建的家庭 @F..@
	fatherX string   // 解析出的父亲 @I..@
	motherX string   // 解析出的母亲 @I..@
	gen     int      // _GEN 自定义世代
	branch  string   // _BRAN
	rank    string   // _RANK
	surname string   // NAME/2 SURN
	given   string   // NAME/2 GIVN
	acct    string
	pwHash  string
	acRole  string
	pwChg   int
	spouses []memberSpouse
}

// gedcomFamily GEDCOM 家庭记录（用于还原父母与配偶）
type gedcomFamily struct {
	xid       string
	husb      string   // @I..@
	wife      string   // @I..@（母亲）
	wivesText []string // 以姓名文本给出的配偶
	chil      []string // @I..@
}

// 简易 GEDCOM 解析器：解析个人 + 家庭记录，回填父母与世代
func (p *gedcomParser) parse() []gedcomPerson {
	var people []gedcomPerson
	var current *gedcomPerson
	var curFam *gedcomFamily
	fams := map[string]*gedcomFamily{}
	flushFam := func() {
		if curFam != nil && curFam.xid != "" {
			fams[curFam.xid] = curFam
		}
	}
	re := regexp.MustCompile(`^(\d+)\s+(@[^@]+@)?\s*(\w+)\s*(.*)$`)

	var lastName string
	var prevName string
	scanner := bufio.NewScanner(strings.NewReader(strings.Join(p.lines, "\n")))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		level, _ := strconv.Atoi(m[1])
		xref := strings.TrimSpace(m[2])
		tag := m[3]
		value := strings.TrimSpace(m[4])

		switch tag {
		case "INDI":
			if current != nil {
				people = append(people, *current)
			}
			flushFam()
			curFam = nil
			current = &gedcomPerson{pwChg: -1, xid: xref}
		case "FAM":
			if current != nil {
				people = append(people, *current)
				current = nil
			}
			flushFam()
			curFam = &gedcomFamily{xid: xref}
		case "NAME":
			if current != nil && level == 1 {
				prevName = current.name
				lastName = value
				current.name = value
			}
		case "GIVN":
			if current != nil {
				current.given = value
			}
		case "SURN":
			if current != nil {
				current.surname = value
			}
		case "SEX":
			if current != nil {
				current.sex = value
			}
		case "DATE":
			if level == 2 && current != nil {
				// 判断上下文：BIRT 或 DEAT
				// 简化处理：根据上一行判断
				current.birth = appendIfEmpty(current.birth, value)
			}
		case "OCCU":
			if current != nil {
				current.occup = value
			}
		case "NOTE":
			if current != nil {
				current.note = value
			}
		case "PLAC":
			if current != nil {
				current.place = value
			}
		case "FAMC":
			if current != nil {
				current.famc = value
			}
		case "FAMS":
			if current != nil && value != "" {
				current.fams = append(current.fams, value)
			}
		case "_GEN":
			if current != nil {
				current.gen, _ = strconv.Atoi(value)
			}
		case "_BRAN":
			if current != nil {
				current.branch = value
			}
		case "_RANK":
			if current != nil {
				current.rank = value
			}
		case "_ACCT":
			if current != nil {
				current.acct = value
			}
		case "_PWHASH":
			if current != nil {
				current.pwHash = value
			}
		case "_ACROLE":
			if current != nil {
				current.acRole = value
			}
		case "_PWCHG":
			if current != nil {
				current.pwChg, _ = strconv.Atoi(value)
			}
		case "HUSB":
			if curFam != nil {
				curFam.husb = value
			}
		case "WIFE":
			if curFam != nil && strings.HasPrefix(value, "@") && curFam.wife == "" {
				curFam.wife = value
			} else if curFam != nil && value != "" {
				curFam.wivesText = append(curFam.wivesText, value)
			}
		case "CHIL":
			if curFam != nil && value != "" {
				curFam.chil = append(curFam.chil, value)
			}
		case "TYPE":
			// 本系统导出的 NAME 后跟 "2 TYPE zi/hao" 表示该 NAME 是字/号，需回滚姓名
			if level == 2 && current != nil && lastName != "" {
				if value == "zi" {
					current.zi = lastName
					current.name = prevName
				} else if value == "hao" {
					current.hao = lastName
					current.name = prevName
				}
				lastName = ""
			}
		}
	}
	if current != nil {
		people = append(people, *current)
	}
	flushFam()

	// 回填父母（FAMC → 家庭 → HUSB/WIFE）与配偶（WIFE 文本行）
	byX := map[string]*gedcomPerson{}
	for i := range people {
		if people[i].xid != "" {
			byX[people[i].xid] = &people[i]
		}
	}
	for i := range people {
		if people[i].famc == "" {
			continue
		}
		if fam := fams[people[i].famc]; fam != nil {
			if fam.husb != "" {
				people[i].fatherX = fam.husb
			}
			if fam.wife != "" {
				people[i].motherX = fam.wife
			}
		}
	}
	for _, fam := range fams {
		if fam.husb == "" || len(fam.wivesText) == 0 {
			continue
		}
		hp := byX[fam.husb]
		if hp == nil {
			continue
		}
		for _, wn := range fam.wivesText {
			hp.spouses = append(hp.spouses, memberSpouse{Name: wn})
		}
	}
	return people
}

func appendIfEmpty(existing, new string) string {
	if existing == "" {
		return new
	}
	return existing
}

// memberSpouse 简化配偶
type memberSpouse struct {
	Name   string
	Type   string
	Family string
}

// splitGedcomName 解析 GEDCOM 姓名：标准格式 "名 /姓/"，本系统导出为纯全名
func splitGedcomName(v string) (surname, given, full string) {
	v = strings.TrimSpace(v)
	if strings.Contains(v, "/") {
		i := strings.Index(v, "/")
		given = strings.TrimSpace(v[:i])
		rest := v[i+1:]
		if j := strings.Index(rest, "/"); j >= 0 {
			surname = strings.TrimSpace(rest[:j])
		} else {
			surname = strings.TrimSpace(rest)
		}
		return surname, given, strings.TrimSpace(surname + given)
	}
	return "", "", v
}

// personToMember 将解析后的 GEDCOM 个人转为 Member
func personToMember(p gedcomPerson) Member {
	gender := 1
	if p.sex == "F" {
		gender = 2
	}
	surname, given, full := splitGedcomName(p.name)
	if p.surname != "" {
		surname = p.surname
	}
	if p.given != "" {
		given = p.given
	}
	return Member{
		Generation:     p.gen, // 0 表示待按父母链推算
		Surname:        surname,
		GivenName:      given,
		Name:           full,
		Zi:             p.zi,
		Hao:            p.hao,
		Gender:         gender,
		Branch:         p.branch,
		Rank:           p.rank,
		BirthGregorian: p.birth,
		Title:          p.occup,
		Biography:      p.note,
		NativePlace:    p.place,
		ClanType:       "本生",
	}
}
