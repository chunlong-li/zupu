package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// 祠宇志 / 茔墓志
// ============================================================================
//
// 设计：不建平行主表。两条线都落在既有 cemeteries（「祠堂与祖茔」共用表）上，
// 用 kind 区分：
//
//	kind = 'shrine' → 祠宇志（祖厝 / 祠堂 / 家庙 / 其它，真实建筑档案）
//	kind = 'grave'  → 茔墓志（每座祖茔一个独立条目）
//	kind = ''       → 旧「祠墓」记录（不猜类型，CultureView 的「祠墓」tab 仍照旧列出）
//
// 只有真正 1:N 的多条内容（始建重修 / 楹联匾额 / 族产公田）才拆子表 shrine_history /
// shrine_couplets / shrine_properties，按 cemetery_id 关联（删除主记录时 handler 显式清子表）。
//
// 写权限：RequirePriv("member_edit")（与 /cemeteries、房支、字辈派语同口径）。
// 直写、不进「理事提交审核」草稿队列 —— 旧草稿链（diff.go 的 fetchCemeteryRow/cemeteryChanges +
// applyDraft）只认识 name/location/description/photo 四个字段，新字段 + 子表硬塞进去会静默丢数据。
// 如需理事审核，必须同时扩 diff.go 的 fetch/changes 与 applyDraft 两处。

// ShrineHistory 祠宇志·始建与重修历史（一条）
type ShrineHistory struct {
	ID        int64  `json:"id"`
	Year      string `json:"year"`
	Event     string `json:"event"`
	Note      string `json:"note"`
	SortOrder int    `json:"sort_order"`
}

// ShrineCouplet 祠宇志·楹联匾额（一条）
type ShrineCouplet struct {
	ID        int64  `json:"id"`
	Kind      string `json:"kind"`     // 匾额 / 对联 / 其他（空 = 未分类）
	Position  string `json:"position"` // 自由文本：大门额 / 大门联 / 神龛联 / 厅柱联 / 正厅横匾 …
	Content   string `json:"content"`
	SortOrder int    `json:"sort_order"`
}

// 楹联匾额的合法类型（空串 = 未分类，兼容老数据）
var shrineCoupletKinds = map[string]bool{"": true, "匾额": true, "对联": true, "其他": true}

// normCoupletKind 归一化楹联匾额类型：只允许 匾额/对联/其他，其它一律当「其他」
func normCoupletKind(k string) string {
	k = strings.TrimSpace(k)
	if shrineCoupletKinds[k] {
		return k
	}
	return "其他"
}

// ShrineProperty 祠宇志·族产公田（一条）
type ShrineProperty struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Area      string `json:"area"`
	Note      string `json:"note"`
	SortOrder int    `json:"sort_order"`
}

// Shrine 祠宇志条目
type Shrine struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	ShrineType   string   `json:"shrine_type"`
	Address      string   `json:"address"`
	Lng          float64  `json:"lng"`
	Lat          float64  `json:"lat"`
	Architecture string   `json:"architecture"`
	SpiritLayout string   `json:"spirit_layout"`
	Ritual       string   `json:"ritual"`
	Remark       string   `json:"remark"`
	Photos       []string `json:"photos"`
	PlanImages   []string `json:"plan_images"`
	SortOrder    int      `json:"sort_order"`
	UpdatedAt    string   `json:"updated_at"`

	History    []ShrineHistory  `json:"history"`
	Couplets   []ShrineCouplet  `json:"couplets"`
	Properties []ShrineProperty `json:"properties"`
}

// ShrineListItem 祠宇志列表行（不含正文，只带封面与计数）
type ShrineListItem struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	ShrineType     string  `json:"shrine_type"`
	Address        string  `json:"address"`
	Lng            float64 `json:"lng"`
	Lat            float64 `json:"lat"`
	Cover          string  `json:"cover"`
	PhotoCount     int     `json:"photo_count"`
	HistoryCount   int     `json:"history_count"`
	CoupletCount   int     `json:"couplet_count"`
	PropertyCount  int     `json:"property_count"`
	CoupletPreview string  `json:"couplet_preview"` // 首条楹联匾额，如「大门联（对联）陇西绵世泽；东坑振家声」
	SortOrder      int     `json:"sort_order"`
	UpdatedAt      string  `json:"updated_at"`
}

// Grave 茔墓志条目（每座祖茔一个）
type Grave struct {
	ID                  int64    `json:"id"`
	Name                string   `json:"name"`
	OwnerName           string   `json:"owner_name"`
	MemberID            int64    `json:"member_id"`
	MemberName          string   `json:"member_name"`
	Generation          int      `json:"generation"`
	Birth               string   `json:"birth"`
	Death               string   `json:"death"`
	Epitaph             string   `json:"epitaph"`
	MountainOrientation string   `json:"mountain_orientation"`
	Address             string   `json:"address"`
	Lng                 float64  `json:"lng"`
	Lat                 float64  `json:"lat"`
	Photos              []string `json:"photos"`
	BoundaryImage       string   `json:"boundary_image"`
	Remark              string   `json:"remark"`
	UpdatedAt           string   `json:"updated_at"`
}

// ShrineGraveHandler 祠宇志 / 茔墓志处理器
type ShrineGraveHandler struct {
	DB *sql.DB
}

// ---------- 通用小工具 ----------

// decodeImageList 解析图片 URL 列表：正常是 JSON 数组；兼容历史上直接存单条路径的情况。
func decodeImageList(raw string) []string {
	raw = strings.TrimSpace(raw)
	out := []string{}
	if raw == "" {
		return out
	}
	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err == nil {
		for _, p := range arr {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	// 兼容：不是 JSON（例如旧数据里直接存了一段路径）
	if strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "uploads/") {
		return []string{raw}
	}
	return out
}

// encodeImageList 序列化图片 URL 列表（去空白、去重、限 60 张，空则存空串）
func encodeImageList(list []string) string {
	out := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, p := range list {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
		if len(out) >= 60 {
			break
		}
	}
	if len(out) == 0 {
		return ""
	}
	b, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(b)
}

// normCoord 归一化经纬度：越界或只填了一半 → 视为未填写（0,0）
func normCoord(lng, lat float64) (float64, float64) {
	if lng < -180 || lng > 180 || lat < -90 || lat > 90 {
		return 0, 0
	}
	if lng == 0 || lat == 0 {
		return 0, 0
	}
	return lng, lat
}

// nowStamp 写入 updated_at 的时间戳（VARCHAR，避免 SQLite「ADD COLUMN 默认值必须是常量」限制）
func nowStamp() string { return time.Now().Format("2006-01-02 15:04:05") }

// ---------- 祠宇志 ----------

// ListShrines 祠宇志列表
// GET /api/shrines?keyword=&type=
// 排序：sort_order → 名称
func (h *ShrineGraveHandler) ListShrines(c *gin.Context) {
	kw := strings.TrimSpace(c.Query("keyword"))
	typ := strings.TrimSpace(c.Query("type"))
	where := []string{`kind = 'shrine'`}
	args := []interface{}{}
	if kw != "" {
		like := "%" + kw + "%"
		where = append(where, `(COALESCE(name,'') LIKE ? OR COALESCE(location,'') LIKE ?
			OR COALESCE(description,'') LIKE ? OR COALESCE(architecture,'') LIKE ?)`)
		args = append(args, like, like, like, like)
	}
	if typ != "" {
		where = append(where, `COALESCE(shrine_type,'') = ?`)
		args = append(args, typ)
	}
	q := `SELECT id, COALESCE(name,''), COALESCE(shrine_type,''), COALESCE(location,''), COALESCE(lng,0), COALESCE(lat,0),
		COALESCE(photos,''), COALESCE(sort_order,0), COALESCE(updated_at,''),
		(SELECT COUNT(*) FROM shrine_history x WHERE x.cemetery_id = cemeteries.id),
		(SELECT COUNT(*) FROM shrine_couplets x WHERE x.cemetery_id = cemeteries.id),
		(SELECT COUNT(*) FROM shrine_properties x WHERE x.cemetery_id = cemeteries.id),
		(SELECT COALESCE(kind,'') || '|' || COALESCE(position,'') || '|' || COALESCE(content,'')
		   FROM shrine_couplets x WHERE x.cemetery_id = cemeteries.id ORDER BY sort_order, id LIMIT 1)
		FROM cemeteries WHERE ` + strings.Join(where, ` AND `) + `
		ORDER BY COALESCE(sort_order,0), COALESCE(name,''), id`
	rows, err := h.DB.Query(q, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	list := []ShrineListItem{}
	for rows.Next() {
		var it ShrineListItem
		var photos string
		var firstCouplet sql.NullString
		if err := rows.Scan(&it.ID, &it.Name, &it.ShrineType, &it.Address, &it.Lng, &it.Lat,
			&photos, &it.SortOrder, &it.UpdatedAt,
			&it.HistoryCount, &it.CoupletCount, &it.PropertyCount, &firstCouplet); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if firstCouplet.Valid && firstCouplet.String != "" {
			parts := strings.SplitN(firstCouplet.String, "|", 3)
			if len(parts) == 3 {
				kind := parts[0]
				if kind == "" {
					kind = "未分类"
				}
				it.CoupletPreview = parts[1] + "（" + kind + "）" + parts[2]
			}
		}
		imgs := decodeImageList(photos)
		it.PhotoCount = len(imgs)
		if len(imgs) > 0 {
			it.Cover = imgs[0]
		}
		list = append(list, it)
	}
	c.JSON(http.StatusOK, gin.H{"items": list, "total": len(list)})
}

// GetShrine 祠宇志详情（含始建重修 / 楹联匾额 / 族产公田）
// GET /api/shrines/:id
func (h *ShrineGraveHandler) GetShrine(c *gin.Context) {
	id := parsePathID(c)
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	s, err := h.loadShrine(id)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "祠宇记录不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

// CreateShrine 新增祠宇
// POST /api/shrines
func (h *ShrineGraveHandler) CreateShrine(c *gin.Context) {
	var s Shrine
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.normalize()
	if s.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写祠宇名称"})
		return
	}
	tx, err := h.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer tx.Rollback()

	photos := encodeImageList(s.Photos)
	cover := ""
	if len(s.Photos) > 0 {
		cover = strings.TrimSpace(s.Photos[0])
	}
	res, err := tx.Exec(`INSERT INTO cemeteries
		(kind, name, shrine_type, location, description, photo, photos, plan_images,
		 lng, lat, architecture, spirit_layout, ritual, sort_order, updated_at)
		VALUES ('shrine',?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		s.Name, s.ShrineType, s.Address, s.Remark, cover, photos, encodeImageList(s.PlanImages),
		s.Lng, s.Lat, s.Architecture, s.SpiritLayout, s.Ritual, s.SortOrder, nowStamp())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	id, _ := res.LastInsertId()
	if err := h.replaceShrineChildren(tx, id, &s); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out, err := h.loadShrine(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

// UpdateShrine 修改祠宇（整条覆盖：主表字段 + 三类子项全量替换）
// PUT /api/shrines/:id
func (h *ShrineGraveHandler) UpdateShrine(c *gin.Context) {
	id := parsePathID(c)
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var s Shrine
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.normalize()
	if s.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写祠宇名称"})
		return
	}
	tx, err := h.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer tx.Rollback()

	photos := encodeImageList(s.Photos)
	cover := ""
	if len(s.Photos) > 0 {
		cover = strings.TrimSpace(s.Photos[0])
	}
	res, err := tx.Exec(`UPDATE cemeteries SET
		name=?, shrine_type=?, location=?, description=?, photos=?, plan_images=?,
		lng=?, lat=?, architecture=?, spirit_layout=?, ritual=?, sort_order=?, updated_at=?,
		photo = CASE WHEN ? <> '' THEN ? ELSE photo END
		WHERE id=? AND kind='shrine'`,
		s.Name, s.ShrineType, s.Address, s.Remark, photos, encodeImageList(s.PlanImages),
		s.Lng, s.Lat, s.Architecture, s.SpiritLayout, s.Ritual, s.SortOrder, nowStamp(),
		cover, cover, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "祠宇记录不存在"})
		return
	}
	if err := h.replaceShrineChildren(tx, id, &s); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out, err := h.loadShrine(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

// DeleteShrine 删除祠宇（连带删除三类子项）
// DELETE /api/shrines/:id
func (h *ShrineGraveHandler) DeleteShrine(c *gin.Context) {
	id := parsePathID(c)
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	tx, err := h.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer tx.Rollback()
	for _, t := range []string{"shrine_history", "shrine_couplets", "shrine_properties"} {
		if _, err := tx.Exec(`DELETE FROM `+t+` WHERE cemetery_id=?`, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	res, err := tx.Exec(`DELETE FROM cemeteries WHERE id=? AND kind='shrine'`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "祠宇记录不存在"})
		return
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": id})
}

// normalize 归一化祠宇字段（去空白 / 补默认类型 / 坐标合法性 / 图片列表清洗）
func (s *Shrine) normalize() {
	s.Name = strings.TrimSpace(s.Name)
	s.ShrineType = strings.TrimSpace(s.ShrineType)
	if s.ShrineType == "" {
		s.ShrineType = "祠堂"
	}
	s.Address = strings.TrimSpace(s.Address)
	s.Architecture = strings.TrimSpace(s.Architecture)
	s.SpiritLayout = strings.TrimSpace(s.SpiritLayout)
	s.Ritual = strings.TrimSpace(s.Ritual)
	s.Remark = strings.TrimSpace(s.Remark)
	s.Lng, s.Lat = normCoord(s.Lng, s.Lat)
	if s.Photos == nil {
		s.Photos = []string{}
	}
	if s.PlanImages == nil {
		s.PlanImages = []string{}
	}
}

// loadShrine 读取祠宇详情（含子表）
func (h *ShrineGraveHandler) loadShrine(id int64) (*Shrine, error) {
	var s Shrine
	var photos, planImages, remark string
	err := h.DB.QueryRow(`SELECT id, COALESCE(name,''), COALESCE(shrine_type,''), COALESCE(location,''),
		COALESCE(lng,0), COALESCE(lat,0), COALESCE(architecture,''), COALESCE(spirit_layout,''),
		COALESCE(ritual,''), COALESCE(description,''), COALESCE(photos,''), COALESCE(plan_images,''),
		COALESCE(sort_order,0), COALESCE(updated_at,'')
		FROM cemeteries WHERE id=? AND kind='shrine'`, id).
		Scan(&s.ID, &s.Name, &s.ShrineType, &s.Address, &s.Lng, &s.Lat, &s.Architecture,
			&s.SpiritLayout, &s.Ritual, &remark, &photos, &planImages, &s.SortOrder, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.Remark = remark
	s.Photos = decodeImageList(photos)
	s.PlanImages = decodeImageList(planImages)
	s.History = []ShrineHistory{}
	s.Couplets = []ShrineCouplet{}
	s.Properties = []ShrineProperty{}

	if rows, err := h.DB.Query(`SELECT id, COALESCE(year,''), COALESCE(event,''), COALESCE(note,''), COALESCE(sort_order,0)
		FROM shrine_history WHERE cemetery_id=? ORDER BY sort_order, id`, id); err == nil {
		for rows.Next() {
			var x ShrineHistory
			if rows.Scan(&x.ID, &x.Year, &x.Event, &x.Note, &x.SortOrder) == nil {
				s.History = append(s.History, x)
			}
		}
		rows.Close()
	}
	if rows, err := h.DB.Query(`SELECT id, COALESCE(kind,''), COALESCE(position,''), COALESCE(content,''), COALESCE(sort_order,0)
		FROM shrine_couplets WHERE cemetery_id=? ORDER BY sort_order, id`, id); err == nil {
		for rows.Next() {
			var x ShrineCouplet
			if rows.Scan(&x.ID, &x.Kind, &x.Position, &x.Content, &x.SortOrder) == nil {
				s.Couplets = append(s.Couplets, x)
			}
		}
		rows.Close()
	}
	if rows, err := h.DB.Query(`SELECT id, COALESCE(name,''), COALESCE(area,''), COALESCE(note,''), COALESCE(sort_order,0)
		FROM shrine_properties WHERE cemetery_id=? ORDER BY sort_order, id`, id); err == nil {
		for rows.Next() {
			var x ShrineProperty
			if rows.Scan(&x.ID, &x.Name, &x.Area, &x.Note, &x.SortOrder) == nil {
				s.Properties = append(s.Properties, x)
			}
		}
		rows.Close()
	}
	return &s, nil
}

// replaceShrineChildren 全量替换三类子项（先删后插，顺序按提交顺序）
func (h *ShrineGraveHandler) replaceShrineChildren(tx *sql.Tx, id int64, s *Shrine) error {
	for _, t := range []string{"shrine_history", "shrine_couplets", "shrine_properties"} {
		if _, err := tx.Exec(`DELETE FROM `+t+` WHERE cemetery_id=?`, id); err != nil {
			return err
		}
	}
	for i, x := range s.History {
		if _, err := tx.Exec(`INSERT INTO shrine_history (cemetery_id, year, event, note, sort_order)
			VALUES (?,?,?,?,?)`, id, strings.TrimSpace(x.Year), strings.TrimSpace(x.Event),
			strings.TrimSpace(x.Note), i); err != nil {
			return err
		}
	}
	for i, x := range s.Couplets {
		if _, err := tx.Exec(`INSERT INTO shrine_couplets (cemetery_id, kind, position, content, sort_order)
			VALUES (?,?,?,?,?)`, id, normCoupletKind(x.Kind), strings.TrimSpace(x.Position), strings.TrimSpace(x.Content), i); err != nil {
			return err
		}
	}
	for i, x := range s.Properties {
		if _, err := tx.Exec(`INSERT INTO shrine_properties (cemetery_id, name, area, note, sort_order)
			VALUES (?,?,?,?,?)`, id, strings.TrimSpace(x.Name), strings.TrimSpace(x.Area),
			strings.TrimSpace(x.Note), i); err != nil {
			return err
		}
	}
	return nil
}

// ---------- 茔墓志 ----------

// ListGraves 茔墓志列表
// GET /api/graves?keyword=&generation=&has_coord=1
// 排序：世次 → 姓名（未填世次的排最后）
func (h *ShrineGraveHandler) ListGraves(c *gin.Context) {
	kw := strings.TrimSpace(c.Query("keyword"))
	genStr := strings.TrimSpace(c.Query("generation"))
	hasCoord := strings.TrimSpace(c.Query("has_coord"))

	where := []string{`c.kind = 'grave'`}
	args := []interface{}{}
	if kw != "" {
		like := "%" + kw + "%"
		where = append(where, `(COALESCE(c.name,'') LIKE ? OR COALESCE(c.owner_name,'') LIKE ?
			OR COALESCE(c.location,'') LIKE ? OR COALESCE(c.epitaph,'') LIKE ?
			OR COALESCE(c.mountain_orientation,'') LIKE ? OR COALESCE(m.name,'') LIKE ?)`)
		args = append(args, like, like, like, like, like, like)
	}
	if genStr != "" {
		if gen, err := strconv.Atoi(genStr); err == nil {
			where = append(where, `COALESCE(c.generation,0) = ?`)
			args = append(args, gen)
		}
	}
	if hasCoord == "1" || hasCoord == "true" {
		where = append(where, `COALESCE(c.lng,0) <> 0 AND COALESCE(c.lat,0) <> 0`)
	}

	q := `SELECT c.id, COALESCE(c.name,''), COALESCE(c.owner_name,''), COALESCE(c.member_id,0),
		COALESCE(m.name,''), COALESCE(c.generation,0), COALESCE(c.birth,''), COALESCE(c.death,''),
		COALESCE(c.mountain_orientation,''), COALESCE(c.location,''), COALESCE(c.lng,0), COALESCE(c.lat,0),
		COALESCE(c.photos,''), COALESCE(c.plan_images,''), COALESCE(c.updated_at,'')
		FROM cemeteries c LEFT JOIN members m ON m.id = c.member_id
		WHERE ` + strings.Join(where, ` AND `) + `
		ORDER BY (COALESCE(c.generation,0) > 0) DESC, COALESCE(c.generation,0) ASC,
		         COALESCE(NULLIF(c.owner_name,''), c.name) ASC, c.id ASC`
	rows, err := h.DB.Query(q, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	list := []Grave{}
	for rows.Next() {
		var g Grave
		var photos, planImages string
		if err := rows.Scan(&g.ID, &g.Name, &g.OwnerName, &g.MemberID, &g.MemberName, &g.Generation,
			&g.Birth, &g.Death, &g.MountainOrientation, &g.Address, &g.Lng, &g.Lat,
			&photos, &planImages, &g.UpdatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		g.Photos = decodeImageList(photos)
		if imgs := decodeImageList(planImages); len(imgs) > 0 {
			g.BoundaryImage = imgs[0]
		}
		list = append(list, g)
	}
	c.JSON(http.StatusOK, gin.H{"items": list, "total": len(list)})
}

// GetGrave 茔墓志详情
// GET /api/graves/:id
func (h *ShrineGraveHandler) GetGrave(c *gin.Context) {
	id := parsePathID(c)
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	g, err := h.loadGrave(id)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "茔墓记录不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, g)
}

// CreateGrave 新增茔墓
// POST /api/graves
func (h *ShrineGraveHandler) CreateGrave(c *gin.Context) {
	var g Grave
	if err := c.ShouldBindJSON(&g); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.normalizeGrave(&g)
	if g.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写茔墓名称或墓主"})
		return
	}
	photos := encodeImageList(g.Photos)
	cover := ""
	if len(g.Photos) > 0 {
		cover = strings.TrimSpace(g.Photos[0])
	}
	boundary := ""
	if g.BoundaryImage != "" {
		boundary = encodeImageList([]string{g.BoundaryImage})
	}
	res, err := h.DB.Exec(`INSERT INTO cemeteries
		(kind, name, owner_name, member_id, generation, birth, death, epitaph, mountain_orientation,
		 location, description, lng, lat, photos, photo, plan_images, updated_at)
		VALUES ('grave',?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		g.Name, g.OwnerName, g.MemberID, g.Generation, g.Birth, g.Death, g.Epitaph, g.MountainOrientation,
		g.Address, g.Remark, g.Lng, g.Lat, photos, cover, boundary, nowStamp())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	id, _ := res.LastInsertId()
	out, err := h.loadGrave(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

// UpdateGrave 修改茔墓
// PUT /api/graves/:id
func (h *ShrineGraveHandler) UpdateGrave(c *gin.Context) {
	id := parsePathID(c)
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var g Grave
	if err := c.ShouldBindJSON(&g); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.normalizeGrave(&g)
	if g.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写茔墓名称或墓主"})
		return
	}
	photos := encodeImageList(g.Photos)
	cover := ""
	if len(g.Photos) > 0 {
		cover = strings.TrimSpace(g.Photos[0])
	}
	boundary := ""
	if g.BoundaryImage != "" {
		boundary = encodeImageList([]string{g.BoundaryImage})
	}
	res, err := h.DB.Exec(`UPDATE cemeteries SET
		name=?, owner_name=?, member_id=?, generation=?, birth=?, death=?, epitaph=?, mountain_orientation=?,
		location=?, description=?, lng=?, lat=?, photos=?, plan_images=?, updated_at=?,
		photo = CASE WHEN ? <> '' THEN ? ELSE photo END
		WHERE id=? AND kind='grave'`,
		g.Name, g.OwnerName, g.MemberID, g.Generation, g.Birth, g.Death, g.Epitaph, g.MountainOrientation,
		g.Address, g.Remark, g.Lng, g.Lat, photos, boundary, nowStamp(),
		cover, cover, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "茔墓记录不存在"})
		return
	}
	out, err := h.loadGrave(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

// DeleteGrave 删除茔墓
// DELETE /api/graves/:id
func (h *ShrineGraveHandler) DeleteGrave(c *gin.Context) {
	id := parsePathID(c)
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	res, err := h.DB.Exec(`DELETE FROM cemeteries WHERE id=? AND kind='grave'`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "茔墓记录不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": id})
}

// normalizeGrave 归一化茔墓字段（墓主回填 / 名称兜底 / 坐标 / 图片）
func (h *ShrineGraveHandler) normalizeGrave(g *Grave) {
	g.Name = strings.TrimSpace(g.Name)
	g.OwnerName = strings.TrimSpace(g.OwnerName)
	g.Birth = strings.TrimSpace(g.Birth)
	g.Death = strings.TrimSpace(g.Death)
	g.Epitaph = strings.TrimSpace(g.Epitaph)
	g.MountainOrientation = strings.TrimSpace(g.MountainOrientation)
	g.Address = strings.TrimSpace(g.Address)
	g.Remark = strings.TrimSpace(g.Remark)
	g.BoundaryImage = strings.TrimSpace(g.BoundaryImage)
	g.Lng, g.Lat = normCoord(g.Lng, g.Lat)
	if g.MemberID < 0 {
		g.MemberID = 0
	}
	// 关联了成员但没写墓主姓名 → 用成员姓名补上（只补空，不改用户填的）
	if g.MemberID > 0 && g.OwnerName == "" && h.DB != nil {
		var n string
		if h.DB.QueryRow(`SELECT COALESCE(name,'') FROM members WHERE id=?`, g.MemberID).Scan(&n) == nil {
			g.OwnerName = strings.TrimSpace(n)
		}
	}
	if g.Name == "" {
		if g.OwnerName != "" {
			g.Name = g.OwnerName
			if !strings.HasSuffix(g.Name, "墓") {
				g.Name += "墓"
			}
		} else {
			g.Name = "未名祖茔"
		}
	}
	if g.Photos == nil {
		g.Photos = []string{}
	}
	if g.Generation < 0 {
		g.Generation = 0
	}
}

// loadGrave 读取茔墓单条
func (h *ShrineGraveHandler) loadGrave(id int64) (*Grave, error) {
	var g Grave
	var photos, planImages string
	err := h.DB.QueryRow(`SELECT c.id, COALESCE(c.name,''), COALESCE(c.owner_name,''), COALESCE(c.member_id,0),
		COALESCE(m.name,''), COALESCE(c.generation,0), COALESCE(c.birth,''), COALESCE(c.death,''),
		COALESCE(c.epitaph,''), COALESCE(c.mountain_orientation,''), COALESCE(c.location,''),
		COALESCE(c.description,''), COALESCE(c.lng,0), COALESCE(c.lat,0),
		COALESCE(c.photos,''), COALESCE(c.plan_images,''), COALESCE(c.updated_at,'')
		FROM cemeteries c LEFT JOIN members m ON m.id = c.member_id
		WHERE c.id=? AND c.kind='grave'`, id).
		Scan(&g.ID, &g.Name, &g.OwnerName, &g.MemberID, &g.MemberName, &g.Generation, &g.Birth, &g.Death,
			&g.Epitaph, &g.MountainOrientation, &g.Address, &g.Remark, &g.Lng, &g.Lat,
			&photos, &planImages, &g.UpdatedAt)
	if err != nil {
		return nil, err
	}
	g.Photos = decodeImageList(photos)
	if imgs := decodeImageList(planImages); len(imgs) > 0 {
		g.BoundaryImage = imgs[0]
	}
	return &g, nil
}

// ClassifyLegacy 把旧「祠墓」记录（kind 为空，来自原 CultureView「祠墓」tab）归入
// 祠宇志或茔墓志；顺手把旧的单张 photo 回填成 photos 数组，归入后即可在新模块里继续编辑。
// POST /api/cemeteries/:id/classify  body: {"kind":"shrine"|"grave"}
// 只允许操作 kind 为空的记录（已归入的不再改动，避免误改）。
func (h *ShrineGraveHandler) ClassifyLegacy(c *gin.Context) {
	id := parsePathID(c)
	if id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var req struct {
		Kind string `json:"kind"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	kind := strings.TrimSpace(req.Kind)
	if kind != "shrine" && kind != "grave" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind 只能是 shrine（祠宇志）或 grave（茔墓志）"})
		return
	}
	res, err := h.DB.Exec(`UPDATE cemeteries SET
		kind = ?,
		shrine_type = CASE WHEN ? = 'shrine' AND COALESCE(shrine_type,'') = '' THEN '祠堂' ELSE shrine_type END,
		photos = CASE WHEN COALESCE(photos,'') = '' AND COALESCE(photo,'') <> ''
		              THEN '["' || photo || '"]' ELSE photos END,
		updated_at = ?
		WHERE id = ? AND COALESCE(kind,'') = ''`, kind, kind, nowStamp(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在，或已归入祠宇志/茔墓志"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": id, "kind": kind})
}

// parsePathID 解析 :id 路径参数（非法返回 0）
func parsePathID(c *gin.Context) int64 {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	return id
}
