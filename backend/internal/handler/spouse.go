package handler

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"familytree/internal/lunar"

	"github.com/gin-gonic/gin"
)

// Spouse 配偶结构体
type Spouse struct {
	ID             int64  `json:"id"`
	MemberID       int64  `json:"member_id"`
	Surname        string `json:"surname"`
	GivenName      string `json:"given_name"`
	Name           string `json:"name"`
	Type           string `json:"type"` // 正配/继配/侧室
	Family         string `json:"family"`
	BirthCalendar  string `json:"birth_calendar"` // "solar" or "lunar"
	BirthGregorian string `json:"birth_gregorian"`
	BirthLunar     string `json:"birth_lunar"`
	BirthGanzhi    string `json:"birth_ganzhi"`
	BirthEra       string `json:"birth_era"`
	DeathGregorian string `json:"death_gregorian"`
	DeathLunar     string `json:"death_lunar"`
	DeathGanzhi    string `json:"death_ganzhi"`
	DeathEra       string `json:"death_era"`
	Occupation     string `json:"occupation"`
	Education      string `json:"education"`
	Biography      string `json:"biography"`
	Remark         string `json:"remark"`
	Portrait       string `json:"portrait"`
	Phone          string `json:"phone"`
	DNAHaploid     string `json:"dna_haploid"`
	DNADiploid     string `json:"dna_diploid"`
	DNAMito        string `json:"dna_mito"`
	HasBio         bool   `json:"has_bio"` // 是否有名人传记
}

// SpouseHandler 配偶处理器
type SpouseHandler struct {
	DB     *sql.DB
	EncKey string // AES-256 加密密钥（hex 编码）
}

// scanSpouses 通用：扫描配偶 rows（含解密与按角色脱敏）
func scanSpouses(rows *sql.Rows, encKey, role string) ([]Spouse, error) {
	defer rows.Close()
	list := []Spouse{}
	for rows.Next() {
		var s Spouse
		if err := rows.Scan(&s.ID, &s.MemberID, &s.Surname, &s.GivenName, &s.Name, &s.Type, &s.Family,
			&s.BirthCalendar, &s.BirthGregorian, &s.BirthLunar, &s.BirthGanzhi, &s.BirthEra,
			&s.DeathGregorian, &s.DeathLunar, &s.DeathGanzhi, &s.DeathEra,
			&s.Occupation, &s.Education, &s.Biography, &s.Remark, &s.Portrait,
			&s.Phone, &s.DNAHaploid, &s.DNADiploid, &s.DNAMito); err != nil {
			return nil, err
		}
		_ = s.DecryptSensitive(encKey)
		sanitizeSpouseByRole(&s, role)
		list = append(list, s)
	}
	return list, nil
}

// ListByMember 获取某成员的全部配偶
// GET /api/members/:id/spouses
func (h *SpouseHandler) ListByMember(c *gin.Context) {
	memberID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	rows, err := h.DB.Query(`SELECT id, member_id, surname, given_name, name, type, family,
		birth_calendar, birth_gregorian, birth_lunar, birth_ganzhi, birth_era,
		death_gregorian, death_lunar, death_ganzhi, death_era, COALESCE(occupation,''), COALESCE(education,''), COALESCE(biography,''), COALESCE(remark,''), COALESCE(portrait,''), COALESCE(phone,''), COALESCE(dna_haploid,''), COALESCE(dna_diploid,''), COALESCE(dna_mito,'')
		FROM spouses WHERE member_id = ? ORDER BY
		CASE type WHEN '正配' THEN 1 WHEN '继配' THEN 2 WHEN '侧室' THEN 3 ELSE 4 END, id`, memberID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	list, err := scanSpouses(rows, h.EncKey, c.GetString("role"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// GetAll 获取全部配偶（供前端生母查找用）
// GET /api/spouses
func (h *SpouseHandler) GetAll(c *gin.Context) {
	rows, err := h.DB.Query(`SELECT id, member_id, surname, given_name, name, type, family,
		birth_calendar, birth_gregorian, birth_lunar, birth_ganzhi, birth_era,
		death_gregorian, death_lunar, death_ganzhi, death_era, COALESCE(occupation,''), COALESCE(education,''), COALESCE(biography,''), COALESCE(remark,''), COALESCE(portrait,''), COALESCE(phone,''), COALESCE(dna_haploid,''), COALESCE(dna_diploid,''), COALESCE(dna_mito,'')
		FROM spouses ORDER BY id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	list, err := scanSpouses(rows, h.EncKey, c.GetString("role"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// Get 获取单个配偶
// GET /api/spouses/:id
func (h *SpouseHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	row := h.DB.QueryRow(`SELECT id, member_id, surname, given_name, name, type, family,
		birth_calendar, birth_gregorian, birth_lunar, birth_ganzhi, birth_era,
		death_gregorian, death_lunar, death_ganzhi, death_era, COALESCE(occupation,''), COALESCE(education,''), COALESCE(biography,''), COALESCE(remark,''), COALESCE(portrait,''), COALESCE(phone,''), COALESCE(dna_haploid,''), COALESCE(dna_diploid,''), COALESCE(dna_mito,'')
		FROM spouses WHERE id = ?`, id)
	var s Spouse
	if err := row.Scan(&s.ID, &s.MemberID, &s.Surname, &s.GivenName, &s.Name, &s.Type, &s.Family,
		&s.BirthCalendar, &s.BirthGregorian, &s.BirthLunar, &s.BirthGanzhi, &s.BirthEra,
		&s.DeathGregorian, &s.DeathLunar, &s.DeathGanzhi, &s.DeathEra,
		&s.Occupation, &s.Education, &s.Biography, &s.Remark, &s.Portrait, &s.Phone, &s.DNAHaploid, &s.DNADiploid, &s.DNAMito); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到该配偶"})
		return
	}
	_ = s.DecryptSensitive(h.EncKey)
	sanitizeSpouseByRole(&s, c.GetString("role"))
	c.JSON(http.StatusOK, s)
}

// Create 新增配偶
// POST /api/spouses
func (h *SpouseHandler) Create(c *gin.Context) {
	var s Spouse
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 房支隔离：理事只能给本房支成员新增配偶
	if !checkMemberBranchAccess(c, h.DB, s.MemberID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权给其他房支的成员新增配偶"})
		return
	}
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "spouse", "create", 0, s, c)
		return
	}
	uid, uname := currentUser(c)
	created, err := h.applyCreate(&s, uid, uname)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, created)
}

// applyCreate 实际执行新增配偶
func (h *SpouseHandler) applyCreate(s *Spouse, changedBy int, changedByName string) (*Spouse, error) {
	// 自动设置全名 = 姓氏 + 名字
	s.Name = s.Surname + s.GivenName
	// 自动填充农历和年号信息
	s.fillLunarInfo()
	// 加密敏感字段（入库）
	if err := s.EncryptSensitive(h.EncKey); err != nil {
		return nil, err
	}
	res, err := h.DB.Exec(`INSERT INTO spouses (member_id, surname, given_name, name, type, family,
		birth_calendar, birth_gregorian, birth_lunar, birth_ganzhi, birth_era,
		death_gregorian, death_lunar, death_ganzhi, death_era, occupation, education, biography, remark, portrait, phone, dna_haploid, dna_diploid, dna_mito)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		s.MemberID, s.Surname, s.GivenName, s.Name, s.Type, s.Family,
		s.BirthCalendar, s.BirthGregorian, s.BirthLunar, s.BirthGanzhi, s.BirthEra,
		s.DeathGregorian, s.DeathLunar, s.DeathGanzhi, s.DeathEra, s.Occupation, s.Education, s.Biography, s.Remark, s.Portrait, s.Phone, s.DNAHaploid, s.DNADiploid, s.DNAMito)
	if err != nil {
		return nil, err
	}
	s.ID, _ = res.LastInsertId()
	// 记录版本：新增即 v1 快照
	_ = recordVersion(h.DB, "spouse", s.ID, s, changedBy, changedByName, "新增")
	return s, nil
}

// Update 更新配偶
// PUT /api/spouses/:id
func (h *SpouseHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var s Spouse
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 房支隔离：理事只能改本房支成员的配偶
	if !checkMemberBranchAccess(c, h.DB, spouseMemberID(h.DB, id)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权修改其他房支成员的配偶"})
		return
	}
	// DNA：仅超级管理员/理事长可改，其余角色保持库中原值（手机号理事及以上可改）
	if r := c.GetString("role"); r != RoleSuperAdmin && r != RoleChief {
		var e1, e2, e3 string
		if err := h.DB.QueryRow(`SELECT COALESCE(dna_haploid,''), COALESCE(dna_diploid,''), COALESCE(dna_mito,'') FROM spouses WHERE id = ?`, id).Scan(&e1, &e2, &e3); err == nil {
			s.DNAHaploid = DecryptValue(e1, h.EncKey)
			s.DNADiploid = DecryptValue(e2, h.EncKey)
			s.DNAMito = DecryptValue(e3, h.EncKey)
		}
	}
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "spouse", "update", id, s, c)
		return
	}
	uid, uname := currentUser(c)
	if err := h.applyUpdate(id, &s, uid, uname); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

// applyUpdate 实际执行更新配偶
func (h *SpouseHandler) applyUpdate(id int64, s *Spouse, changedBy int, changedByName string) error {
	// 记录修改前快照
	if old, err := fetchSpouseRow(h.DB, id); err == nil {
		_ = recordVersion(h.DB, "spouse", id, old, changedBy, changedByName, "修改")
	}
	// 自动设置全名 = 姓氏 + 名字
	s.Name = s.Surname + s.GivenName
	// 自动填充农历和年号信息
	s.fillLunarInfo()
	// 加密敏感字段（入库）
	if err := s.EncryptSensitive(h.EncKey); err != nil {
		return err
	}
	_, err := h.DB.Exec(`UPDATE spouses SET surname=?, given_name=?, name=?, type=?, family=?,
		birth_calendar=?, birth_gregorian=?, birth_lunar=?, birth_ganzhi=?, birth_era=?,
		death_gregorian=?, death_lunar=?, death_ganzhi=?, death_era=?, occupation=?, education=?, biography=?, remark=?, portrait=?, phone=?, dna_haploid=?, dna_diploid=?, dna_mito=?
		WHERE id=?`,
		s.Surname, s.GivenName, s.Name, s.Type, s.Family,
		s.BirthCalendar, s.BirthGregorian, s.BirthLunar, s.BirthGanzhi, s.BirthEra,
		s.DeathGregorian, s.DeathLunar, s.DeathGanzhi, s.DeathEra, s.Occupation, s.Education, s.Biography, s.Remark, s.Portrait, s.Phone, s.DNAHaploid, s.DNADiploid, s.DNAMito, id)
	if err != nil {
		return err
	}
	s.ID = id
	return nil
}

// Delete 删除配偶
// DELETE /api/spouses/:id
func (h *SpouseHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	// 房支隔离：理事只能删本房支成员的配偶
	if !checkMemberBranchAccess(c, h.DB, spouseMemberID(h.DB, id)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除其他房支成员的配偶"})
		return
	}
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "spouse", "delete", id, gin.H{"id": id}, c)
		return
	}
	uid, uname := currentUser(c)
	if err := h.applyDelete(id, uid, uname); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// applyDelete 实际执行删除配偶
func (h *SpouseHandler) applyDelete(id int64, changedBy int, changedByName string) error {
	// 记录删除前快照
	if old, err := fetchSpouseRow(h.DB, id); err == nil {
		_ = recordVersion(h.DB, "spouse", id, old, changedBy, changedByName, "删除")
	}
	_, err := h.DB.Exec(`DELETE FROM spouses WHERE id = ?`, id)
	return err
}

// fillLunarInfo 根据公历日期自动填充农历、干支和年号信息
func (s *Spouse) fillLunarInfo() {
	// 处理出生日期：有公历日期则重算农历、干支和年号
	if s.BirthGregorian != "" {
		if t, err := time.Parse("2006-01-02", s.BirthGregorian); err == nil {
			info := lunar.FromGregorian(t.Year(), int(t.Month()), t.Day())
			s.BirthLunar = info.Lunar
			s.BirthGanzhi = info.Ganzhi
			s.BirthEra = info.Era + info.EraYear
		} else if y, err2 := strconv.Atoi(s.BirthGregorian); err2 == nil {
			// 只填年份：只算年号（农历/干支需月日，留空）
			info := lunar.FromGregorianYear(y)
			s.BirthEra = info.Era + info.EraYear
		}
	}
	// 处理逝世日期
	if s.DeathGregorian != "" {
		if t, err := time.Parse("2006-01-02", s.DeathGregorian); err == nil {
			info := lunar.FromGregorian(t.Year(), int(t.Month()), t.Day())
			s.DeathLunar = info.Lunar
			s.DeathGanzhi = info.Ganzhi
			s.DeathEra = info.Era + info.EraYear
		} else if y, err2 := strconv.Atoi(s.DeathGregorian); err2 == nil {
			info := lunar.FromGregorianYear(y)
			s.DeathEra = info.Era + info.EraYear
		}
	}
}
