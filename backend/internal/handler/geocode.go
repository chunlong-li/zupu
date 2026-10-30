package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// GeocodeHandler 地址地理编码（百度/高德，Key 由用户在系统内自行填写）
type GeocodeHandler struct {
	DB *sql.DB
}

type geoConfigBody struct {
	Provider string `json:"provider"` // none|amap|baidu
	Key      string `json:"key"`
}

func (h *GeocodeHandler) readConfig() (string, string) {
	var p, k string
	h.DB.QueryRow(`SELECT value FROM settings WHERE key='geo_provider'`).Scan(&p)
	h.DB.QueryRow(`SELECT value FROM settings WHERE key='geo_key'`).Scan(&k)
	if p == "" {
		p = "none"
	}
	return p, k
}

func maskKey(k string) string {
	r := []rune(k)
	if len(r) <= 8 {
		if len(r) == 0 {
			return ""
		}
		return "****"
	}
	return string(r[:4]) + "****" + string(r[len(r)-4:])
}

// GetConfig 读取地理编码配置
// GET /api/geo/config
func (h *GeocodeHandler) GetConfig(c *gin.Context) {
	p, k := h.readConfig()
	// 缓存统计
	var cached int
	h.DB.QueryRow(`SELECT COUNT(*) FROM geocode_cache`).Scan(&cached)
	c.JSON(http.StatusOK, gin.H{
		"provider":   p,
		"key_set":    k != "",
		"key_masked": maskKey(k),
		"cached":     cached,
	})
}

// SaveConfig 保存地理编码配置（Key 留空表示不改）
// PUT /api/geo/config
func (h *GeocodeHandler) SaveConfig(c *gin.Context) {
	var body geoConfigBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p := strings.TrimSpace(body.Provider)
	if p != "amap" && p != "baidu" && p != "none" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "服务商仅支持 none / amap / baidu"})
		return
	}
	h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('geo_provider', ?)`, p)
	if k := strings.TrimSpace(body.Key); k != "" {
		h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('geo_key', ?)`, k)
	}
	if p == "none" {
		h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('geo_key', '')`)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "provider": p})
}

// ClearCache 清空坐标缓存
// DELETE /api/geo/cache
func (h *GeocodeHandler) ClearCache(c *gin.Context) {
	if _, err := h.DB.Exec(`DELETE FROM geocode_cache`); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ------------------------- 第三方地理编码 -------------------------

var geoHTTP = &http.Client{Timeout: 8 * time.Second}

// bd09ToGcj02 百度坐标(BD09) → 高德坐标(GCJ02)，避免热力点偏移
func bd09ToGcj02(lng, lat float64) (float64, float64) {
	x, y := lng-0.0065, lat-0.006
	z := math.Sqrt(x*x+y*y) - 0.00002*math.Sin(y*math.Pi*3000.0/180.0)
	theta := math.Atan2(y, x) - 0.000003*math.Cos(x*math.Pi*3000.0/180.0)
	return z * math.Cos(theta), z * math.Sin(theta)
}

func geoFetch(rawURL string) ([]byte, error) {
	resp, err := geoHTTP.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("HTTP " + strconv.Itoa(resp.StatusCode))
	}
	return b, nil
}

// geocodeAmap 高德：/v3/geocode/geo
func geocodeAmap(place, key string) (float64, float64, error) {
	u := "https://restapi.amap.com/v3/geocode/geo?address=" + url.QueryEscape(place) + "&key=" + url.QueryEscape(key)
	b, err := geoFetch(u)
	if err != nil {
		return 0, 0, err
	}
	var out struct {
		Status   string `json:"status"`
		Info     string `json:"info"`
		Geocodes []struct {
			Location string `json:"location"`
		} `json:"geocodes"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return 0, 0, err
	}
	if out.Status != "1" {
		return 0, 0, errors.New("高德返回：" + out.Info + "（" + out.Status + "）")
	}
	if len(out.Geocodes) == 0 {
		return 0, 0, errors.New("未匹配到地址")
	}
	parts := strings.Split(out.Geocodes[0].Location, ",")
	if len(parts) != 2 {
		return 0, 0, errors.New("坐标格式异常")
	}
	lng, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lat, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil {
		return 0, 0, errors.New("坐标解析失败")
	}
	return lng, lat, nil
}

// geocodeBaidu 百度：/geocoding/v3/
func geocodeBaidu(place, key string) (float64, float64, error) {
	u := "https://api.map.baidu.com/geocoding/v3/?address=" + url.QueryEscape(place) + "&output=json&ak=" + url.QueryEscape(key)
	b, err := geoFetch(u)
	if err != nil {
		return 0, 0, err
	}
	var out struct {
		Status  int    `json:"status"`
		Msg     string `json:"msg"`
		Message string `json:"message"`
		Result  struct {
			Location struct {
				Lng float64 `json:"lng"`
				Lat float64 `json:"lat"`
			} `json:"location"`
		} `json:"result"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return 0, 0, err
	}
	if out.Status != 0 {
		m := out.Message
		if m == "" {
			m = out.Msg
		}
		if m == "" {
			m = "status " + strconv.Itoa(out.Status)
		}
		return 0, 0, errors.New("百度返回：" + m + "（" + strconv.Itoa(out.Status) + "）")
	}
	if out.Result.Location.Lng == 0 && out.Result.Location.Lat == 0 {
		return 0, 0, errors.New("未匹配到地址")
	}
	lng, lat := bd09ToGcj02(out.Result.Location.Lng, out.Result.Location.Lat)
	return lng, lat, nil
}

func geocodePlace(place, provider, key string) (float64, float64, error) {
	switch provider {
	case "amap":
		return geocodeAmap(place, key)
	case "baidu":
		return geocodeBaidu(place, key)
	}
	return 0, 0, errors.New("未配置地理编码服务商")
}

// ------------------------- 热力点 -------------------------

// geoResolve 批量把地名解析为经纬度：先读缓存，未命中的按需调用第三方编码（最多 maxNew 个），
// 结果写回缓存。未配置地理编码服务或解析失败的地名不会出现在返回的 map 中。
func geoResolve(db *sql.DB, places []string, maxNew int) map[string][2]float64 {
	out := map[string][2]float64{}
	if len(places) == 0 {
		return out
	}
	// 1) 缓存
	if rows, err := db.Query(`SELECT place, lng, lat FROM geocode_cache`); err == nil {
		for rows.Next() {
			var p string
			var lng, lat float64
			if err := rows.Scan(&p, &lng, &lat); err == nil {
				out[p] = [2]float64{lng, lat}
			}
		}
		rows.Close()
	}
	// 2) 未命中的按需解析
	var provider, key string
	db.QueryRow(`SELECT value FROM settings WHERE key='geo_provider'`).Scan(&provider)
	db.QueryRow(`SELECT value FROM settings WHERE key='geo_key'`).Scan(&key)
	if provider != "none" && key != "" {
		calls := 0
		for _, p := range places {
			if _, ok := out[p]; ok {
				continue
			}
			if calls >= maxNew {
				break
			}
			calls++
			lng, lat, err := geocodePlace(p, provider, key)
			if err != nil {
				continue
			}
			out[p] = [2]float64{lng, lat}
			db.Exec(`INSERT OR REPLACE INTO geocode_cache (place, lng, lat, provider, updated_at) VALUES (?,?,?,?,CURRENT_TIMESTAMP)`,
				p, lng, lat, provider)
		}
	}
	res := map[string][2]float64{}
	for _, p := range places {
		if c, ok := out[p]; ok {
			res[p] = c
		}
	}
	return res
}

type heatPoint struct {
	Name  string  `json:"name"`
	Lng   float64 `json:"lng"`
	Lat   float64 `json:"lat"`
	Count int     `json:"count"`
	// 详情行（成员姓名，供图例/提示直观展示「谁被算进来了」）
	Names []string `json:"names,omitempty"`
}

// heatPointBatch 一类落点（native / current / current_fallback / burial）
type heatPointBatch struct {
	Kind   string      `json:"kind"`
	Label  string      `json:"label"`
	Points []heatPoint `json:"points"`
	// 人数（按成员去重）与人数上限内的地名数
	Total int `json:"total"`
	// 该类是否为「现居地落回祖籍（未迁出）」
	Fallback bool `json:"fallback,omitempty"`
}

// heatPlaceStats 按类别统计成员的地名（现居地为空/仅填祖籍者计入 fallback）
type heatPlaceStats struct {
	native      map[string]int
	current     map[string]int
	fallback    map[string]int
	burial      map[string]int
	total       map[string]int // 类别 → 人数
	fallbackNum int
	// 当前点选开关下命中的成员数（按人去重）
	visibleMembers int
}

// heatPlaceQuery 按筛选条件取成员地名并分类（供落点接口用）
func heatPlaceQuery(c *gin.Context, db *sql.DB) (*heatPlaceStats, error) {
	q := `SELECT COALESCE(native_place,''), COALESCE(current_place,''), COALESCE(burial_place,'') FROM members WHERE 1=1`
	args := []any{}
	if b := strings.TrimSpace(c.Query("branch")); b != "" {
		q += " AND branch = ?"
		args = append(args, b)
	}
	if cond, cargs := BranchScope(c, db, "branch_id"); cond != "" {
		q += " AND " + cond
		args = append(args, cargs...)
	}
	if v, err := strconv.Atoi(c.Query("gen_min")); err == nil && v > 0 {
		q += " AND generation >= ?"
		args = append(args, v)
	}
	if v, err := strconv.Atoi(c.Query("gen_max")); err == nil && v > 0 {
		q += " AND generation <= ?"
		args = append(args, v)
	}
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	st := &heatPlaceStats{
		native: map[string]int{}, current: map[string]int{}, fallback: map[string]int{},
		burial: map[string]int{}, total: map[string]int{},
	}
	// 图例点选开关（cats）：关掉的类别不参与统计，返回的 points_by_field 里也就没有它
	cats := heatCatsOf(c)
	fallbackMembers, currentMembers, visibleMembers := 0, 0, 0
	for rows.Next() {
		var np, cp, bp string
		if err := rows.Scan(&np, &cp, &bp); err != nil {
			continue
		}
		np, cp, bp = strings.TrimSpace(np), strings.TrimSpace(cp), strings.TrimSpace(bp)
		// 该成员在当前点选开关下是否还有可见类别（按人去重，与明细口径一致）
		visible := false
		if np != "" && heatCatOn(cats, heatCatNative) {
			st.native[np]++
			st.total["native"]++
			visible = true
		}
		// 现居地为空（或填了但识别不出省份）→ 落回祖籍坐标（未迁出）
		cpItem := cp
		cProv := detectProvince(cp)
		if cProv == "" && np != "" {
			cpItem = np
			if heatCatOn(cats, heatCatFallback) {
				st.fallback[cpItem]++
				st.total[heatCatFallback]++
				fallbackMembers++
				visible = true
			}
		} else if cp != "" && heatCatOn(cats, heatCatCurrent) {
			st.current[cpItem]++
			st.total["current"]++
			currentMembers++
			visible = true
		}
		if bp != "" && heatCatOn(cats, heatCatBurial) {
			st.burial[bp]++
			st.total["burial"]++
			visible = true
		}
		if visible {
			visibleMembers++
		}
	}
	st.total["current_declared"] = currentMembers
	st.total["current_fallback"] = fallbackMembers
	st.visibleMembers = visibleMembers
	st.fallbackNum = fallbackMembers
	return st, nil
}

// heatPointNames 把成员姓名按地名归类（详情行用，最多 30 个名字）
// 返回 (姓名索引, 命中类别的成员数, error)
func heatPointNames(c *gin.Context, db *sql.DB, limit int) (map[string][]string, int, error) {
	q := `SELECT COALESCE(name,''), COALESCE(native_place,''), COALESCE(current_place,''), COALESCE(burial_place,'') FROM members WHERE 1=1`
	args := []any{}
	if b := strings.TrimSpace(c.Query("branch")); b != "" {
		q += " AND branch = ?"
		args = append(args, b)
	}
	if cond, cargs := BranchScope(c, db, "branch_id"); cond != "" {
		q += " AND " + cond
		args = append(args, cargs...)
	}
	if v, err := strconv.Atoi(c.Query("gen_min")); err == nil && v > 0 {
		q += " AND generation >= ?"
		args = append(args, v)
	}
	if v, err := strconv.Atoi(c.Query("gen_max")); err == nil && v > 0 {
		q += " AND generation <= ?"
		args = append(args, v)
	}
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := map[string][]string{}
	// 图例点选开关：关掉的类别不收集姓名（详情行与落点口径一致）
	cats := heatCatsOf(c)
	visible := 0
	for rows.Next() {
		var name, np, cp, bp string
		if err := rows.Scan(&name, &np, &cp, &bp); err != nil {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		keys := map[string]bool{}
		if np = strings.TrimSpace(np); np != "" {
			if heatCatOn(cats, heatCatNative) {
				keys["n|"+np] = true
			}
			if detectProvince(cp) == "" && heatCatOn(cats, heatCatFallback) {
				keys["c|"+np] = true // 未迁出：现居地落回祖籍
			}
		}
		if cp = strings.TrimSpace(cp); cp != "" && detectProvince(cp) != "" && heatCatOn(cats, heatCatCurrent) {
			keys["c|"+cp] = true
		}
		if bp = strings.TrimSpace(bp); bp != "" && heatCatOn(cats, heatCatBurial) {
			keys["b|"+bp] = true
		}
		if len(keys) > 0 {
			visible++
		}
		for k := range keys {
			if len(out[k]) < limit {
				out[k] = append(out[k], name)
			}
		}
	}
	return out, visible, nil
}

// heatBatchLabel 类别显示名
func heatBatchLabel(kind string, fallback bool) string {
	switch kind {
	case "native":
		return "祖籍"
	case "current":
		if fallback {
			return "现居地同祖籍（未迁出）"
		}
		return "现居地"
	case "burial":
		return "葬地"
	}
	return kind
}

// heatResolvePlaces 地名 → 坐标（先读缓存，未命中的按需调用第三方，最多 maxNew 个）
func heatResolvePlaces(db *sql.DB, places []string, maxNew int) (map[string][2]float64, int, string) {
	cache := map[string][2]float64{}
	if rows, err := db.Query(`SELECT place, lng, lat FROM geocode_cache`); err == nil {
		for rows.Next() {
			var p string
			var lng, lat float64
			if err := rows.Scan(&p, &lng, &lat); err == nil {
				cache[p] = [2]float64{lng, lat}
			}
		}
		rows.Close()
	}
	var provider, key string
	db.QueryRow(`SELECT value FROM settings WHERE key='geo_provider'`).Scan(&provider)
	db.QueryRow(`SELECT value FROM settings WHERE key='geo_key'`).Scan(&key)
	calls, failed := 0, 0
	lastErr := ""
	if provider != "none" && key != "" {
		for _, p := range places {
			if _, ok := cache[p]; ok {
				continue
			}
			if calls >= maxNew {
				break
			}
			calls++
			lng, lat, err := geocodePlace(p, provider, key)
			if err != nil {
				failed++
				lastErr = err.Error()
				continue
			}
			cache[p] = [2]float64{lng, lat}
			db.Exec(`INSERT OR REPLACE INTO geocode_cache (place, lng, lat, provider, updated_at) VALUES (?,?,?,?,CURRENT_TIMESTAMP)`,
				p, lng, lat, provider)
		}
	}
	return cache, failed, lastErr
}

// heatBatchFromPlaceCount 一族地名 + 计数 → 聚合落点（约 1km 网格合并）
// names 的 key 形如 "n|地名"/"c|地名"/"b|地名"（见 heatPointNames）
func heatBatchFromPlaceCount(placeCount map[string]int, coords map[string][2]float64, names map[string][]string, kind string, fallback bool) heatPointBatch {
	pre := "n|"
	if kind == "burial" {
		pre = "b|"
	} else if kind == "current" {
		pre = "c|"
	}
	agg := map[string]*heatPoint{}
	for p, n := range placeCount {
		cd, ok := coords[p]
		if !ok {
			continue
		}
		ns := names[pre+p]
		k := strconv.FormatFloat(math.Round(cd[0]*100)/100, 'f', 2, 64) + "," + strconv.FormatFloat(math.Round(cd[1]*100)/100, 'f', 2, 64)
		if pt, ok := agg[k]; ok {
			pt.Count += n
			if len(pt.Names) < 30 {
				pt.Names = append(pt.Names, ns...)
			}
		} else {
			if len(ns) > 30 {
				ns = ns[:30]
			}
			agg[k] = &heatPoint{Name: p, Lng: math.Round(cd[0]*100) / 100, Lat: math.Round(cd[1]*100) / 100, Count: n, Names: ns}
		}
	}
	pts := make([]heatPoint, 0, len(agg))
	for _, pt := range agg {
		pts = append(pts, *pt)
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].Count > pts[j].Count })
	total := 0
	for _, n := range placeCount {
		total += n
	}
	return heatPointBatch{Kind: kind, Label: heatBatchLabel(kind, fallback), Points: pts, Total: total}
}

// Points 返回经纬度热力点（用缓存 + 按需调用第三方 API）
// GET /api/heatmap/points?field=native|current|burial|all&branch=&gen_min=&gen_max=&limit=40
// field=current 时，现居地为空的成员落回祖籍（未迁出），返回的 kind=current 批次带 fallback=true；
// field=all 时同时返回 native / current / burial 三类批次（前端按类别上色 + 出图例）。
func (h *GeocodeHandler) Points(c *gin.Context) {
	provider, key := h.readConfig()
	if provider == "none" || key == "" {
		c.JSON(http.StatusOK, gin.H{
			"configured": false, "points": []any{}, "pending": 0, "total_places": 0,
			"hint": "未配置地理编码服务，可在本页「地理编码设置」中填写高德/百度 Key",
		})
		return
	}

	field := c.Query("field")
	switch field {
	case "current", "burial", "all":
	default:
		field = "native"
	}
	// 图例点选开关（cats）：与 /heatmap 同源，关掉的类别不参与解析与返回
	cats := heatCatsOf(c)

	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 || limit > 100 {
		limit = 40
	}

	st, err := heatPlaceQuery(c, h.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	names, _, _ := heatPointNames(c, h.DB, 30)

	// 参与解析的地名集合（按需解析总量仍受 limit 限制，避免一次请求打爆第三方配额）
	var want []map[string]int
	switch field {
	case "all":
		want = []map[string]int{st.native, st.current, st.fallback, st.burial}
	case "current":
		want = []map[string]int{st.current, st.fallback}
	case "burial":
		want = []map[string]int{st.burial}
	default:
		want = []map[string]int{st.native}
	}
	placeSet := map[string]bool{}
	places := []string{}
	for _, m := range want {
		for p := range m {
			if !placeSet[p] {
				placeSet[p] = true
				places = append(places, p)
			}
		}
	}
	sort.Strings(places)

	cache, failed, lastErr := heatResolvePlaces(h.DB, places, limit)

	// 各类别落点
	batches := []heatPointBatch{}
	addBatch := func(m map[string]int, kind string, fallback bool) {
		if len(m) == 0 {
			return
		}
		b := heatBatchFromPlaceCount(m, cache, names, kind, fallback)
		b.Fallback = fallback
		if b.Total > 0 {
			batches = append(batches, b)
		}
	}
	switch field {
	case "all":
		addBatch(st.native, "native", false)
		addBatch(st.current, "current", false)
		addBatch(st.fallback, heatCatFallback, true)
		addBatch(st.burial, "burial", false)
	case "current":
		addBatch(st.current, "current", false)
		addBatch(st.fallback, heatCatFallback, true)
	case "burial":
		addBatch(st.burial, "burial", false)
	default:
		addBatch(st.native, "native", false)
	}

	// 兼容旧前端：points = 主类落点；current 模式把「未迁出」的也算进来
	main := heatPointBatch{}
	for _, b := range batches {
		if (field == "current" && b.Kind == "current") || (field == "native" && b.Kind == "native") ||
			(field == "burial" && b.Kind == "burial") || (field == "all" && b.Kind == "native") {
			main = b
			break
		}
	}
	mainPts := main.Points
	if mainPts == nil {
		mainPts = []heatPoint{}
	}
	if field == "current" {
		// 未迁出者的落点也并入 points（坐标 = 祖籍），fallback 标记在 points_by_field 里
		for _, b := range batches {
			if b.Fallback {
				mainPts = append(append([]heatPoint{}, mainPts...), b.Points...)
			}
		}
	}

	pending := 0
	for _, p := range places {
		if _, ok := cache[p]; !ok {
			pending++
		}
	}

	// 人数口径（按成员去重）
	memberTotals := gin.H{
		"native":           st.total["native"],
		"current":          st.total["current"],
		"current_fallback": st.total["current_fallback"],
		"burial":           st.total["burial"],
	}
	resp := gin.H{
		"configured":      true,
		"provider":        provider,
		"points":          mainPts,
		"points_by_field": batches,
		"member_totals":   memberTotals,
		"pending":         pending,
		"total_places":    len(places),
		"failed":          failed,
		// 当前点选开关下命中的成员数（按人算，去掉关掉的类别后重新统计）
		"visible_members": st.visibleMembers,
		"cats":            heatCatsParam(cats),
		"current_fallback": gin.H{
			"enabled": true,
			"count":   st.fallbackNum,
			"note":    "现居地为空的成员按祖籍坐标显示（未迁出）",
		},
	}
	if failed > 0 {
		hint := strconv.Itoa(failed) + " 个地址解析失败"
		if lastErr != "" {
			hint += "：" + lastErr
		} else {
			hint += "（请检查 Key 是否有效、对应服务是否已开通）"
		}
		resp["hint"] = hint
		resp["error"] = lastErr
	}
	c.JSON(http.StatusOK, resp)
}
