package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// EntityVersion 实体版本快照（成员/配偶的修改历史，支持一键回滚）
type EntityVersion struct {
	ID            int64           `json:"id"`
	EntityType    string          `json:"entity_type"` // member | spouse
	EntityID      int64           `json:"entity_id"`
	VersionNo     int             `json:"version_no"`
	Snapshot      json.RawMessage `json:"snapshot"`
	ChangedBy     int             `json:"changed_by"`
	ChangedByName string          `json:"changed_by_name"`
	Note          string          `json:"note"`
	CreatedAt     string          `json:"created_at"`
}

// VersionLogHandler 版本历史处理器
type VersionLogHandler struct {
	DB *sql.DB
}

// currentUser 从上下文取当前登录用户（id + 用户名）
func currentUser(c *gin.Context) (int, string) {
	return c.GetInt("user_id"), c.GetString("username")
}

// nextVersionNo 计算某实体下一个版本号
func nextVersionNo(db *sql.DB, entityType string, entityID int64) int {
	var n int
	db.QueryRow(`SELECT COALESCE(MAX(version_no),0) FROM entity_versions WHERE entity_type=? AND entity_id=?`,
		entityType, entityID).Scan(&n)
	return n + 1
}

// recordVersion 写入一条版本快照。snapshot 为实体结构体（会自动 JSON 序列化）。
// note 为空时用 fallback。
func recordVersion(db *sql.DB, entityType string, entityID int64, snapshot interface{}, changedBy int, changedByName, note string) error {
	b, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if note == "" {
		note = "修改"
	}
	vno := nextVersionNo(db, entityType, entityID)
	_, err = db.Exec(`INSERT INTO entity_versions
		(entity_type, entity_id, version_no, snapshot, changed_by, changed_by_name, note)
		VALUES (?,?,?,?,?,?,?)`,
		entityType, entityID, vno, string(b), changedBy, changedByName, note)
	return err
}

// fetchMemberRow 读取成员整行（与 Member.Get 同列），用于记录修改前快照
func fetchMemberRow(db *sql.DB, id int64) (*Member, error) {
	var m Member
	err := db.QueryRow(`SELECT id, generation, branch, branch_id, surname, given_name, name, zi, hao, generation_word,
		rank, gender, father_id, mother_id, mother_spouse_id, clan_type, record_type, foster_father_id,
		birth_calendar, birth_gregorian, birth_lunar, birth_ganzhi, birth_era, death_gregorian, death_lunar, death_ganzhi, death_era,
		lifespan, native_place, current_place, title, occupation, education, biography, burial_place, portrait, married_out, former_name
		FROM members WHERE id = ?`, id).Scan(&m.ID, &m.Generation, &m.Branch, &m.BranchID, &m.Surname, &m.GivenName, &m.Name, &m.Zi, &m.Hao,
		&m.GenerationWord, &m.Rank, &m.Gender, &m.FatherID, &m.MotherID, &m.MotherSpouseID,
		&m.ClanType, &m.RecordType, &m.FosterFatherID, &m.BirthCalendar, &m.BirthGregorian, &m.BirthLunar, &m.BirthGanzhi, &m.BirthEra,
		&m.DeathGregorian, &m.DeathLunar, &m.DeathGanzhi, &m.DeathEra, &m.Lifespan,
		&m.NativePlace, &m.CurrentPlace, &m.Title, &m.Occupation, &m.Education, &m.Biography, &m.BurialPlace, &m.Portrait, &m.MarriedOut, &m.FormerName)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// fetchSpouseRow 读取配偶整行（与 Spouse.Get 同列），用于记录修改前快照
func fetchSpouseRow(db *sql.DB, id int64) (*Spouse, error) {
	var s Spouse
	err := db.QueryRow(`SELECT id, member_id, surname, given_name, name, type, family,
		birth_calendar, birth_gregorian, birth_lunar, birth_ganzhi, birth_era,
		death_gregorian, death_lunar, death_ganzhi, death_era, COALESCE(occupation,''), COALESCE(education,''), COALESCE(biography,''), COALESCE(remark,''), COALESCE(portrait,''), COALESCE(phone,''), COALESCE(dna_haploid,''), COALESCE(dna_diploid,''), COALESCE(dna_mito,'')
		FROM spouses WHERE id = ?`, id).Scan(&s.ID, &s.MemberID, &s.Surname, &s.GivenName, &s.Name, &s.Type, &s.Family,
		&s.BirthCalendar, &s.BirthGregorian, &s.BirthLunar, &s.BirthGanzhi, &s.BirthEra,
		&s.DeathGregorian, &s.DeathLunar, &s.DeathGanzhi, &s.DeathEra,
		&s.Occupation, &s.Education, &s.Biography, &s.Remark, &s.Portrait, &s.Phone, &s.DNAHaploid, &s.DNADiploid, &s.DNAMito)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ListMemberVersions 列出成员版本历史
// GET /api/members/:id/versions
func (h *VersionLogHandler) ListMemberVersions(c *gin.Context) {
	h.listVersions(c, "member", c.Param("id"))
}

// ListSpouseVersions 列出配偶版本历史
// GET /api/spouses/:id/versions
func (h *VersionLogHandler) ListSpouseVersions(c *gin.Context) {
	h.listVersions(c, "spouse", c.Param("id"))
}

func (h *VersionLogHandler) listVersions(c *gin.Context, entityType, idStr string) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	rows, err := h.DB.Query(`SELECT id, entity_type, entity_id, version_no, snapshot, changed_by, changed_by_name, note, created_at
		FROM entity_versions WHERE entity_type=? AND entity_id=? ORDER BY version_no DESC`, entityType, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	list := []EntityVersion{}
	for rows.Next() {
		var v EntityVersion
		var snap string
		if err := rows.Scan(&v.ID, &v.EntityType, &v.EntityID, &v.VersionNo, &snap, &v.ChangedBy, &v.ChangedByName, &v.Note, &v.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		v.Snapshot = json.RawMessage(snap)
		list = append(list, v)
	}
	c.JSON(http.StatusOK, list)
}

// VersionChanges 返回某版本相对其下一版本（更新后状态）的字段级变更
// GET /api/entity-versions/:id/changes
// 语义：
//   - note=新增：快照即新建后的实体，展示「空 → 快照」
//   - note=修改/改名/回滚：快照为改动前的状态，展示「快照 → 下一版本快照（若无则当前库状态）」
//   - note=删除：无字段对比，由前端提示「删除该记录」
func (h *VersionLogHandler) VersionChanges(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var entityType string
	var entityID int64
	var versionNo int
	var snap, note string
	err = h.DB.QueryRow(`SELECT entity_type, entity_id, version_no, COALESCE(snapshot,''), COALESCE(note,'') FROM entity_versions WHERE id = ?`, id).
		Scan(&entityType, &entityID, &versionNo, &snap, &note)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "版本不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 删除：无字段对比
	if note == "删除" {
		c.JSON(http.StatusOK, gin.H{"changes": []FieldChange{}, "note": note, "deleted": true})
		return
	}

	// 找下一更新版本（version_no 更大的最小者）作为「改动后」状态
	var afterSnap string
	err = h.DB.QueryRow(`SELECT COALESCE(snapshot,'') FROM entity_versions WHERE entity_type=? AND entity_id=? AND version_no > ? ORDER BY version_no ASC LIMIT 1`,
		entityType, entityID, versionNo).Scan(&afterSnap)
	hasAfter := (err == nil)
	if err != nil && err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var changes []FieldChange
	switch entityType {
	case "member":
		var beforeM Member
		if json.Unmarshal([]byte(snap), &beforeM) != nil {
			c.JSON(http.StatusOK, gin.H{"changes": []FieldChange{}, "note": note})
			return
		}
		var afterM Member
		if note == "新增" {
			changes = memberCreateChanges(&beforeM)
		} else {
			if hasAfter {
				if json.Unmarshal([]byte(afterSnap), &afterM) != nil {
					c.JSON(http.StatusOK, gin.H{"changes": []FieldChange{}, "note": note})
					return
				}
			} else {
				// 最新版本：与当前库状态对比
				if cur, err := fetchMemberRow(h.DB, entityID); err == nil && cur != nil {
					afterM = *cur
				}
			}
			changes = memberChanges(&beforeM, &afterM)
		}
	case "spouse":
		var beforeS Spouse
		if json.Unmarshal([]byte(snap), &beforeS) != nil {
			c.JSON(http.StatusOK, gin.H{"changes": []FieldChange{}, "note": note})
			return
		}
		var afterS Spouse
		if note == "新增" {
			changes = spouseCreateChanges(&beforeS)
		} else {
			if hasAfter {
				if json.Unmarshal([]byte(afterSnap), &afterS) != nil {
					c.JSON(http.StatusOK, gin.H{"changes": []FieldChange{}, "note": note})
					return
				}
			} else {
				if cur, err := fetchSpouseRow(h.DB, entityID); err == nil && cur != nil {
					afterS = *cur
				}
			}
			changes = spouseChanges(&beforeS, &afterS)
		}
	default:
		changes = []FieldChange{}
	}

	c.JSON(http.StatusOK, gin.H{"changes": changes, "note": note})
}

// Rollback 回滚到指定版本快照（回滚前会先把当前状态再存一版，原始数据不丢）
// POST /api/entity-versions/:id/rollback  body: {"note":"回滚原因"}
func (h *VersionLogHandler) Rollback(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var body struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&body)

	var entityType string
	var entityID int64
	var snap string
	err = h.DB.QueryRow(`SELECT entity_type, entity_id, snapshot FROM entity_versions WHERE id = ?`, id).
		Scan(&entityType, &entityID, &snap)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "版本不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	uid, uname := currentUser(c)
	note := body.Note
	if note == "" {
		note = "回滚到版本 " + strconv.FormatInt(id, 10)
	}

	switch entityType {
	case "member":
		// 先存当前状态快照
		if cur, err := fetchMemberRow(h.DB, entityID); err == nil {
			_ = recordVersion(h.DB, "member", entityID, cur, uid, uname, note)
		}
		var m Member
		if err := json.Unmarshal([]byte(snap), &m); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "快照解析失败：" + err.Error()})
			return
		}
		_, err := h.DB.Exec(`UPDATE members SET generation=?, branch=?, branch_id=?, surname=?, given_name=?, name=?, zi=?, hao=?, generation_word=?,
			rank=?, gender=?, father_id=?, mother_id=?, mother_spouse_id=?, clan_type=?, record_type=?, foster_father_id=?,
			birth_calendar=?, birth_gregorian=?, birth_lunar=?, birth_ganzhi=?, birth_era=?, death_gregorian=?, death_lunar=?, death_ganzhi=?, death_era=?,
			lifespan=?, native_place=?, current_place=?, title=?, occupation=?, education=?, biography=?, burial_place=?, portrait=?, married_out=?, former_name=?,
			updated_at=CURRENT_TIMESTAMP WHERE id=?`,
			m.Generation, m.Branch, m.BranchID, m.Surname, m.GivenName, m.Name, m.Zi, m.Hao, m.GenerationWord,
			m.Rank, m.Gender, m.FatherID, m.MotherID, m.MotherSpouseID, m.ClanType, m.RecordType, m.FosterFatherID,
			m.BirthCalendar, m.BirthGregorian, m.BirthLunar, m.BirthGanzhi, m.BirthEra, m.DeathGregorian, m.DeathLunar, m.DeathGanzhi, m.DeathEra,
			m.Lifespan, m.NativePlace, m.CurrentPlace, m.Title, m.Occupation, m.Education, m.Biography, m.BurialPlace, m.Portrait, m.MarriedOut, m.FormerName, entityID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "entity_type": entityType, "entity_id": entityID})

	case "spouse":
		if cur, err := fetchSpouseRow(h.DB, entityID); err == nil {
			_ = recordVersion(h.DB, "spouse", entityID, cur, uid, uname, note)
		}
		var s Spouse
		if err := json.Unmarshal([]byte(snap), &s); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "快照解析失败：" + err.Error()})
			return
		}
		_, err := h.DB.Exec(`UPDATE spouses SET surname=?, given_name=?, name=?, type=?, family=?,
			birth_calendar=?, birth_gregorian=?, birth_lunar=?, birth_ganzhi=?, birth_era=?,
			death_gregorian=?, death_lunar=?, death_ganzhi=?, death_era=?, occupation=?, education=?, biography=?, remark=?, portrait=?
			WHERE id=?`,
			s.Surname, s.GivenName, s.Name, s.Type, s.Family,
			s.BirthCalendar, s.BirthGregorian, s.BirthLunar, s.BirthGanzhi, s.BirthEra,
			s.DeathGregorian, s.DeathLunar, s.DeathGanzhi, s.DeathEra, s.Occupation, s.Education, s.Biography, s.Remark, s.Portrait, entityID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "entity_type": entityType, "entity_id": entityID})

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的实体类型"})
	}
}
