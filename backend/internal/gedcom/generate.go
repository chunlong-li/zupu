package gedcom

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Row 一条成员导出数据
type Row struct {
	id, fatherID, motherID, motherSpouseID      int64
	gen, gender                                 int
	surname, givenName, name, zi, hao, clanType string
	branch, rank                                string
	birthG, birthL, deathG, deathL              string
	nativePlace, title, occupation, education   string
	bio, burial                                 string
}

// Spouse 配偶导出数据
type Spouse struct {
	ID         int64
	MemberID   int64
	Name       string
	Type       string
	Birth      string
	Death      string
	Occupation string
	Education  string
}

// Account 成员账号（用于备份/恢复登录密码）
type Account struct {
	Username string
	Hash     string
	Role     string
	NeedChg  int
}

// Generate 生成 GEDCOM 5.5.1 文本（不含账号，供导出下载与普通备份共用）
func Generate(db *sql.DB) (string, error) {
	return GenerateWithAccounts(db, false)
}

// GenerateWithAccounts 生成 GEDCOM 5.5.1 文本；withAccounts 为 true 时附带成员账号
// （用户名 / 密码哈希 / 角色），便于备份后在其它系统恢复原登录密码。
func GenerateWithAccounts(db *sql.DB, withAccounts bool) (string, error) {
	rows, err := db.Query(`SELECT id, generation, surname, given_name, name, zi, hao, gender, father_id, mother_id, mother_spouse_id,
		clan_type, branch, rank, birth_gregorian, birth_lunar, death_gregorian, death_lunar,
		native_place, title, occupation, education, biography, burial_place
		FROM members ORDER BY generation, id`)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	// 读取家族姓氏/名称，用于生成 HEAD 备注
	var famSurname, famName string
	db.QueryRow(`SELECT value FROM settings WHERE key = 'family_surname'`).Scan(&famSurname)
	db.QueryRow(`SELECT value FROM settings WHERE key = 'family_name'`).Scan(&famName)
	if famName == "" {
		famName = famSurname
	}

	// 读取所有配偶，建 member_id → []Spouse 映射
	spRows, _ := db.Query(`SELECT id, member_id, name, type, COALESCE(birth_gregorian,''), COALESCE(death_gregorian,''), COALESCE(occupation,''), COALESCE(education,'') FROM spouses ORDER BY member_id, id`)
	defer spRows.Close()
	spMap := map[int64][]Spouse{}
	spouseNameByID := map[int64]string{}
	for spRows.Next() {
		var s Spouse
		spRows.Scan(&s.ID, &s.MemberID, &s.Name, &s.Type, &s.Birth, &s.Death, &s.Occupation, &s.Education)
		spMap[s.MemberID] = append(spMap[s.MemberID], s)
		spouseNameByID[s.ID] = s.Name
	}

	// 成员账号（用户名=成员姓名）→ 供备份/恢复
	acctMap := map[string]Account{}
	if withAccounts {
		if uRows, uErr := db.Query(`SELECT username, password, COALESCE(role,''), COALESCE(need_change_pw,1) FROM users`); uErr == nil {
			defer uRows.Close()
			for uRows.Next() {
				var a Account
				uRows.Scan(&a.Username, &a.Hash, &a.Role, &a.NeedChg)
				if a.Username != "" && a.Hash != "" {
					acctMap[a.Username] = a
				}
			}
		}
	}

	var members []Row
	for rows.Next() {
		var m Row
		rows.Scan(&m.id, &m.gen, &m.surname, &m.givenName, &m.name, &m.zi, &m.hao, &m.gender, &m.fatherID, &m.motherID, &m.motherSpouseID,
			&m.clanType, &m.branch, &m.rank, &m.birthG, &m.birthL, &m.deathG, &m.deathL,
			&m.nativePlace, &m.title, &m.occupation, &m.education, &m.bio, &m.burial)
		members = append(members, m)
	}

	memberByID := map[int64]Row{}
	for _, m := range members {
		memberByID[m.id] = m
	}

	// 分配 FAM id：每个「有配偶 或 有孩子」的成员分配一个家庭记录
	famOfMember := map[int64]int{}
	famID := 1
	for _, m := range members {
		if len(spMap[m.id]) > 0 || hasChildren(m.id, members) {
			famOfMember[m.id] = famID
			famID++
		}
	}

	var buf strings.Builder
	buf.WriteString("0 HEAD\n")
	buf.WriteString("1 SOUR 中式族谱系统\n")
	if famName != "" {
		buf.WriteString(fmt.Sprintf("1 SUBM %s族谱\n", famName))
	}
	buf.WriteString("1 GEDC\n2 VERS 5.5.1\n1 CHAR UTF-8\n")
	buf.WriteString("1 DATE " + time.Now().Format("02 JAN 2006") + "\n")

	// 1) 个人记录 INDI
	for _, m := range members {
		sex := "M"
		if m.gender == 2 {
			sex = "F"
		}

		buf.WriteString(fmt.Sprintf("0 @I%d@ INDI\n", m.id))
		buf.WriteString(fmt.Sprintf("1 NAME %s\n", m.name))
		if m.givenName != "" && m.surname != "" {
			buf.WriteString(fmt.Sprintf("2 GIVN %s\n2 SURN %s\n", m.givenName, m.surname))
		}
		buf.WriteString(fmt.Sprintf("1 SEX %s\n", sex))

		if m.birthG != "" {
			buf.WriteString(fmt.Sprintf("1 BIRT\n2 DATE %s\n", m.birthG))
		} else if m.birthL != "" {
			buf.WriteString(fmt.Sprintf("1 BIRT\n2 DATE %s\n", m.birthL))
		}
		if m.deathG != "" {
			buf.WriteString(fmt.Sprintf("1 DEAT\n2 DATE %s\n", m.deathG))
		} else if m.deathL != "" {
			buf.WriteString(fmt.Sprintf("1 DEAT\n2 DATE %s\n", m.deathL))
		}

		if m.zi != "" {
			buf.WriteString(fmt.Sprintf("1 NAME %s\n2 TYPE zi\n", m.zi))
		}
		if m.hao != "" {
			buf.WriteString(fmt.Sprintf("1 NAME %s\n2 TYPE hao\n", m.hao))
		}
		if m.title != "" {
			buf.WriteString(fmt.Sprintf("1 TITL %s\n", m.title))
		}
		if m.occupation != "" {
			buf.WriteString(fmt.Sprintf("1 OCCU %s\n", m.occupation))
		}
		if m.education != "" {
			buf.WriteString(fmt.Sprintf("1 EDUC %s\n", m.education))
		}
		if m.bio != "" {
			buf.WriteString(fmt.Sprintf("1 NOTE %s\n", m.bio))
		}
		if m.nativePlace != "" {
			buf.WriteString(fmt.Sprintf("1 RESI\n2 PLAC %s\n", m.nativePlace))
		}
		if m.burial != "" {
			buf.WriteString(fmt.Sprintf("1 BURI\n2 PLAC %s\n", m.burial))
		}

		// 世代/房支/辈分（GEDCOM 自定义扩展标签）
		if m.gen > 0 {
			buf.WriteString(fmt.Sprintf("1 _GEN %d\n", m.gen))
		}
		if m.branch != "" {
			buf.WriteString(fmt.Sprintf("1 _BRAN %s\n", m.branch))
		}
		if m.rank != "" {
			buf.WriteString(fmt.Sprintf("1 _RANK %s\n", m.rank))
		}

		// 账号信息（自定义扩展标签）：恢复后成员仍用原密码登录
		if acct, ok := acctMap[m.name]; ok {
			buf.WriteString(fmt.Sprintf("1 _ACCT %s\n", acct.Username))
			buf.WriteString(fmt.Sprintf("1 _PWHASH %s\n", acct.Hash))
			if acct.Role != "" {
				buf.WriteString(fmt.Sprintf("1 _ACROLE %s\n", acct.Role))
			}
			buf.WriteString(fmt.Sprintf("1 _PWCHG %d\n", acct.NeedChg))
		}

		// FAMS：自己组建的家庭
		if fam, ok := famOfMember[m.id]; ok {
			buf.WriteString(fmt.Sprintf("1 FAMS @F%d@\n", fam))
		}
		// FAMC：父母家庭（父亲优先，父亲缺失时回退到母亲）
		parentID := m.fatherID
		if parentID == 0 {
			parentID = m.motherID
		}
		if parentID > 0 {
			if fam, ok := famOfMember[parentID]; ok {
				buf.WriteString(fmt.Sprintf("1 FAMC @F%d@\n", fam))
			}
		}
	}

	// 2) 家庭记录 FAM
	for _, m := range members {
		fam, ok := famOfMember[m.id]
		if !ok {
			continue
		}
		buf.WriteString(fmt.Sprintf("0 @F%d@ FAM\n", fam))
		buf.WriteString(fmt.Sprintf("1 HUSB @I%d@\n", m.id))

		emitted := map[string]bool{}
		// 配偶记录
		for _, sp := range spMap[m.id] {
			buf.WriteString(fmt.Sprintf("1 WIFE %s\n", sp.Name))
			if sp.Type != "" {
				buf.WriteString(fmt.Sprintf("2 NOTE %s\n", sp.Type))
			}
			if sp.Occupation != "" {
				buf.WriteString(fmt.Sprintf("2 OCCU %s\n", sp.Occupation))
			}
			if sp.Education != "" {
				buf.WriteString(fmt.Sprintf("2 EDUC %s\n", sp.Education))
			}
			emitted[sp.Name] = true
		}
		// 孩子 + 母亲链接
		for _, child := range members {
			if child.fatherID != m.id {
				continue
			}
			buf.WriteString(fmt.Sprintf("1 CHIL @I%d@\n", child.id))
			// 母亲链接：优先成员表母亲（@I引用），其次配偶表母亲
			if child.motherID > 0 {
				if _, ok := memberByID[child.motherID]; ok {
					ref := fmt.Sprintf("@I%d@", child.motherID)
					if !emitted[ref] {
						buf.WriteString(fmt.Sprintf("1 WIFE %s\n", ref))
						emitted[ref] = true
					}
				}
			} else if child.motherSpouseID > 0 {
				if nm, ok := spouseNameByID[child.motherSpouseID]; ok && nm != "" && !emitted[nm] {
					buf.WriteString(fmt.Sprintf("1 WIFE %s\n", nm))
					emitted[nm] = true
				}
			}
		}
	}

	buf.WriteString("0 TRLR\n")
	return buf.String(), nil
}

// hasChildren 判断成员是否有孩子
func hasChildren(id int64, members []Row) bool {
	for _, m := range members {
		if m.fatherID == id {
			return true
		}
	}
	return false
}
