package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Biography 名人传记：为「做出重大贡献或成就」的人立传。
// 不限于族内成员，配偶、姻亲、外族/地方名人均可；member_id / spouse_id 可为 0（独立立传）。
type Biography struct {
	ID             int64  `json:"id"`
	PersonType     string `json:"person_type"` // member | spouse | other
	MemberID       int64  `json:"member_id"`
	SpouseID       int64  `json:"spouse_id"`
	Name           string `json:"name"`
	Zi             string `json:"zi"`
	Hao            string `json:"hao"`
	Gender         string `json:"gender"`
	BirthGregorian string `json:"birth_gregorian"`
	BirthLunar     string `json:"birth_lunar"`
	BirthEra       string `json:"birth_era"`
	DeathGregorian string `json:"death_gregorian"`
	DeathLunar     string `json:"death_lunar"`
	DeathEra       string `json:"death_era"`
	NativePlace    string `json:"native_place"`
	CurrentPlace   string `json:"current_place"`
	Title          string `json:"title"`      // 身份/头衔
	Occupation     string `json:"occupation"` // 职业
	Honors         string `json:"honors"`     // 荣誉
	Portrait       string `json:"portrait"`
	Summary        string `json:"summary"` // 摘要（列表展示）
	Content        string `json:"content"` // 正文
	Reason         string `json:"reason"`  // 入选理由/事迹
	SortYear       int    `json:"sort_year"`
	SortOrder      int    `json:"sort_order"`
	Public         int    `json:"public"`
	CreatedBy      string `json:"created_by"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// BiographyHandler 名人传记处理器
type BiographyHandler struct {
	DB *sql.DB
}

const biographyCols = `person_type, member_id, spouse_id, name, zi, hao, gender,
	birth_gregorian, birth_lunar, birth_era, death_gregorian, death_lunar, death_era,
	native_place, current_place, title, occupation, honors, portrait, summary, content,
	reason, sort_year, sort_order, public`

// ListBiographies 传记列表
// GET /api/biographies?q=&person_type=&member_id=&spouse_id=
func (h *BiographyHandler) ListBiographies(c *gin.Context) {
	where := []string{}
	args := []interface{}{}
	if role := c.GetString("role"); role == RoleMember {
		where = append(where, "public = 1")
	}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		where = append(where, "(name LIKE ? OR zi LIKE ? OR hao LIKE ? OR title LIKE ? OR honors LIKE ? OR summary LIKE ? OR content LIKE ?)")
		kw := "%" + q + "%"
		args = append(args, kw, kw, kw, kw, kw, kw, kw)
	}
	if pt := c.Query("person_type"); pt != "" && pt != "all" {
		where = append(where, "person_type = ?")
		args = append(args, pt)
	}
	if mid, _ := strconv.ParseInt(c.Query("member_id"), 10, 64); mid > 0 {
		where = append(where, "member_id = ?")
		args = append(args, mid)
	}
	if sid, _ := strconv.ParseInt(c.Query("spouse_id"), 10, 64); sid > 0 {
		where = append(where, "spouse_id = ?")
		args = append(args, sid)
	}
	query := `SELECT id, ` + biographyCols + `, COALESCE(created_by,''), COALESCE(created_at,''), COALESCE(updated_at,'') FROM biographies`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	// 主排序不在这里做：sort_year 不是主键，改由 sortBiographies 按
	// 「世代 → 排行 → 生年 → id」在 Go 侧重排（rank 是自由文本，SQL 无法按序数排）。
	// 这里只保证取数顺序稳定，便于后端排序失败时仍有确定结果。
	query += " ORDER BY id"

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	list := []Biography{}
	for rows.Next() {
		var b Biography
		if err := rows.Scan(&b.ID, &b.PersonType, &b.MemberID, &b.SpouseID, &b.Name, &b.Zi, &b.Hao, &b.Gender,
			&b.BirthGregorian, &b.BirthLunar, &b.BirthEra, &b.DeathGregorian, &b.DeathLunar, &b.DeathEra,
			&b.NativePlace, &b.CurrentPlace, &b.Title, &b.Occupation, &b.Honors, &b.Portrait, &b.Summary, &b.Content,
			&b.Reason, &b.SortYear, &b.SortOrder, &b.Public, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		list = append(list, b)
	}
	// 名人传记默认顺序：世代（辈分高者在前）→ 排行 → 生年 → id；
	// 配偶随夫、关联不到成员者排最后（详见 biography_sort.go）。
	sortBiographies(h.DB, list)
	c.JSON(http.StatusOK, list)
}

// GetBiography 传记详情
// GET /api/biographies/:id
func (h *BiographyHandler) GetBiography(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var b Biography
	err := h.DB.QueryRow(`SELECT id, `+biographyCols+`, COALESCE(created_by,''), COALESCE(created_at,''), COALESCE(updated_at,'')
		FROM biographies WHERE id = ?`, id).Scan(&b.ID, &b.PersonType, &b.MemberID, &b.SpouseID, &b.Name, &b.Zi, &b.Hao, &b.Gender,
		&b.BirthGregorian, &b.BirthLunar, &b.BirthEra, &b.DeathGregorian, &b.DeathLunar, &b.DeathEra,
		&b.NativePlace, &b.CurrentPlace, &b.Title, &b.Occupation, &b.Honors, &b.Portrait, &b.Summary, &b.Content,
		&b.Reason, &b.SortYear, &b.SortOrder, &b.Public, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "传记不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, b)
}

// CreateBiography 新增传记（理事提交 → 审核；理事长/超管直接发布）
func (h *BiographyHandler) CreateBiography(c *gin.Context) {
	var b Biography
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(b.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写姓名"})
		return
	}
	if isElder(c) {
		submitDraft(h.DB, "biography", "create", 0, b, c)
		return
	}
	_, uname := currentUser(c)
	b.CreatedBy = uname
	id, err := insertBiography(h.DB, &b)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	b.ID = id
	c.JSON(http.StatusOK, b)
}

// UpdateBiography 修改传记
func (h *BiographyHandler) UpdateBiography(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var b Biography
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(b.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写姓名"})
		return
	}
	if isElder(c) {
		submitDraft(h.DB, "biography", "update", id, b, c)
		return
	}
	if err := updateBiography(h.DB, id, &b); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	b.ID = id
	c.JSON(http.StatusOK, b)
}

// DeleteBiography 删除传记
func (h *BiographyHandler) DeleteBiography(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if isElder(c) {
		submitDraft(h.DB, "biography", "delete", id, gin.H{"id": id}, c)
		return
	}
	if _, err := h.DB.Exec(`DELETE FROM biographies WHERE id = ?`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetBiographyCriteria 入传标准
// GET /api/biographies/criteria
func (h *BiographyHandler) GetBiographyCriteria(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"criteria": getSetting(h.DB, "biography_criteria")})
}

// UpdateBiographyCriteria 设置入传标准（理事长/超级管理员）
func (h *BiographyHandler) UpdateBiographyCriteria(c *gin.Context) {
	var body struct {
		Criteria string `json:"criteria"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, err := h.DB.Exec(`INSERT INTO settings (key, value) VALUES ('biography_criteria', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, body.Criteria); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "criteria": body.Criteria})
}

// --- 内部：入库 ---

func insertBiography(db *sql.DB, b *Biography) (int64, error) {
	res, err := db.Exec(`INSERT INTO biographies (`+biographyCols+`, created_by) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		b.PersonType, b.MemberID, b.SpouseID, b.Name, b.Zi, b.Hao, b.Gender,
		b.BirthGregorian, b.BirthLunar, b.BirthEra, b.DeathGregorian, b.DeathLunar, b.DeathEra,
		b.NativePlace, b.CurrentPlace, b.Title, b.Occupation, b.Honors, b.Portrait, b.Summary, b.Content,
		b.Reason, b.SortYear, b.SortOrder, b.Public, b.CreatedBy)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func updateBiography(db *sql.DB, id int64, b *Biography) error {
	_, err := db.Exec(`UPDATE biographies SET person_type=?, member_id=?, spouse_id=?, name=?, zi=?, hao=?, gender=?,
		birth_gregorian=?, birth_lunar=?, birth_era=?, death_gregorian=?, death_lunar=?, death_era=?,
		native_place=?, current_place=?, title=?, occupation=?, honors=?, portrait=?, summary=?, content=?,
		reason=?, sort_year=?, sort_order=?, public=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		b.PersonType, b.MemberID, b.SpouseID, b.Name, b.Zi, b.Hao, b.Gender,
		b.BirthGregorian, b.BirthLunar, b.BirthEra, b.DeathGregorian, b.DeathLunar, b.DeathEra,
		b.NativePlace, b.CurrentPlace, b.Title, b.Occupation, b.Honors, b.Portrait, b.Summary, b.Content,
		b.Reason, b.SortYear, b.SortOrder, b.Public, id)
	return err
}

// applyDraft 审核通过后落库
func (h *BiographyHandler) applyDraft(d *EditDraft) error {
	switch d.Action {
	case "create":
		var b Biography
		if err := json.Unmarshal([]byte(d.Payload), &b); err != nil {
			return err
		}
		_, err := insertBiography(h.DB, &b)
		return err
	case "update":
		var b Biography
		if err := json.Unmarshal([]byte(d.Payload), &b); err != nil {
			return err
		}
		return updateBiography(h.DB, d.EntityID, &b)
	case "delete":
		_, err := h.DB.Exec(`DELETE FROM biographies WHERE id = ?`, d.EntityID)
		return err
	}
	return nil
}

// getSetting 读取 settings 表（不存在返回空串）
func getSetting(db *sql.DB, key string) string {
	var v string
	if err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v); err != nil {
		return ""
	}
	return v
}
