package handler

import (
	"database/sql"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// HeatmapHandler 家族热力分布处理器
type HeatmapHandler struct {
	DB *sql.DB
}

// 省级行政区（全称 + 常用简称/别称），全称需与前端 GeoJSON 的 name 一致
var heatProvinces = []struct {
	Name string
	Keys []string
}{
	{"北京市", []string{"北京"}},
	{"天津市", []string{"天津"}},
	{"河北省", []string{"河北"}},
	{"山西省", []string{"山西"}},
	{"内蒙古自治区", []string{"内蒙古", "内蒙"}},
	{"辽宁省", []string{"辽宁"}},
	{"吉林省", []string{"吉林"}},
	{"黑龙江省", []string{"黑龙江"}},
	{"上海市", []string{"上海"}},
	{"江苏省", []string{"江苏"}},
	{"浙江省", []string{"浙江"}},
	{"安徽省", []string{"安徽"}},
	{"福建省", []string{"福建"}},
	{"江西省", []string{"江西"}},
	{"山东省", []string{"山东"}},
	{"河南省", []string{"河南"}},
	{"湖北省", []string{"湖北"}},
	{"湖南省", []string{"湖南"}},
	{"广东省", []string{"广东"}},
	{"广西壮族自治区", []string{"广西"}},
	{"海南省", []string{"海南"}},
	{"重庆市", []string{"重庆"}},
	{"四川省", []string{"四川"}},
	{"贵州省", []string{"贵州"}},
	{"云南省", []string{"云南"}},
	{"西藏自治区", []string{"西藏"}},
	{"陕西省", []string{"陕西"}},
	{"甘肃省", []string{"甘肃"}},
	{"青海省", []string{"青海"}},
	{"宁夏回族自治区", []string{"宁夏"}},
	{"新疆维吾尔自治区", []string{"新疆"}},
	{"台湾省", []string{"台湾"}},
	{"香港特别行政区", []string{"香港"}},
	{"澳门特别行政区", []string{"澳门"}},
}

// detectProvince 从自由文本地名中识别省级行政区（取最长匹配，避免「山西/陕西」等歧义）
func detectProvince(place string) string {
	if place == "" {
		return ""
	}
	best, bestLen := "", 0
	for _, p := range heatProvinces {
		for _, k := range p.Keys {
			if strings.Contains(place, k) && len(k) > bestLen {
				best, bestLen = p.Name, len(k)
			}
		}
	}
	return best
}

// 截到「市/州/地区/盟」为止：生者住址只保留到市一级
var heatCityCutRe = regexp.MustCompile(`(?s)^(.{0,24}?(?:市|自治州|地区|盟))`)

// truncatePlace 地址脱敏：只保留到市（无市名则退回省级）
func truncatePlace(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if m := heatCityCutRe.FindString(p); m != "" {
		return m
	}
	if prov := detectProvince(p); prov != "" {
		return prov
	}
	r := []rune(p)
	if len(r) > 12 {
		return string(r[:12])
	}
	return p
}

var heatYearRe = regexp.MustCompile(`(1[0-9]{3}|20[0-9]{2})`)

// 直辖市：下钻时按「区」聚合，普通省份按「市」聚合
var heatMunicipalities = map[string]bool{"北京市": true, "天津市": true, "上海市": true, "重庆市": true}

var heatDistrictRe = regexp.MustCompile(`(?s)^(.{0,24}?(?:区|县|旗))`)

// stripProvincePrefix 去掉地名开头的省名（全称或简称）
func stripProvincePrefix(prov, s string) string {
	if prov == "" {
		return s
	}
	if strings.HasPrefix(s, prov) {
		return strings.TrimPrefix(s, prov)
	}
	for _, p := range heatProvinces {
		if p.Name != prov {
			continue
		}
		for _, k := range p.Keys {
			if strings.HasPrefix(s, k) {
				return strings.TrimPrefix(s, k)
			}
		}
	}
	return s
}

// cityKey 取地级关键字用于省市下钻聚合（直辖市取区级），无则返回空（只统计到省）
func cityKey(prov, place string) string {
	s := strings.TrimSpace(place)
	if s == "" {
		return ""
	}
	if heatMunicipalities[prov] {
		rest := stripProvincePrefix(prov, s)
		if m := heatDistrictRe.FindString(rest); m != "" {
			return m
		}
	}
	t := truncatePlace(s)
	if prov != "" && t == prov { // 只识别到省，没有市名
		return ""
	}
	return stripProvincePrefix(prov, t)
}

// heatYearOf 从生卒日期字符串里取 4 位年份
func heatYearOf(s string) int {
	m := heatYearRe.FindString(s)
	if m == "" {
		return 0
	}
	y, _ := strconv.Atoi(m)
	return y
}

type heatGeoItem struct {
	Province string `json:"province"`
	Count    int    `json:"count"`
}

type heatBranchItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type heatCityItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func addCity(m map[string]map[string]int, prov, city string) {
	addCityN(m, prov, city, 1)
}

// addCityN 按次数累加到「省 → 城市」计数（现居地落回祖籍时用得上）
func addCityN(m map[string]map[string]int, prov, city string, n int) {
	if prov == "" || city == "" {
		return
	}
	if m[prov] == nil {
		m[prov] = map[string]int{}
	}
	m[prov][city] += n
}

// sortCityMap 省 → 城市计数（按人数降序）
func sortCityMap(m map[string]map[string]int) map[string][]heatCityItem {
	out := map[string][]heatCityItem{}
	for prov, cm := range m {
		list := make([]heatCityItem, 0, len(cm))
		for k, v := range cm {
			list = append(list, heatCityItem{Name: k, Count: v})
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].Count != list[j].Count {
				return list[i].Count > list[j].Count
			}
			return list[i].Name < list[j].Name
		})
		out[prov] = list
	}
	return out
}

type heatMigItem struct {
	From        string `json:"from"`
	To          string `json:"to"`
	FromProv    string `json:"from_province,omitempty"`
	ToProv      string `json:"to_province,omitempty"`
	Count       int    `json:"count"`
	Era         string `json:"era,omitempty"`
	Year        int    `json:"year,omitempty"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"` // event=手工迁徙记录 / computed=籍贯→葬地推算
	// 落点级坐标（把「迁出地/迁入地」地址地理编码得到；失败则为 0，前端回退按省级画）
	FromLng float64 `json:"from_lng,omitempty"`
	FromLat float64 `json:"from_lat,omitempty"`
	ToLng   float64 `json:"to_lng,omitempty"`
	ToLat   float64 `json:"to_lat,omitempty"`
}

type heatMemberRow struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Generation   int    `json:"generation"`
	Branch       string `json:"branch"`
	NativePlace  string `json:"native_place"`
	CurrentPlace string `json:"current_place"`
	BurialPlace  string `json:"burial_place"`
	Alive        bool   `json:"alive"`
	// 该成员命中的热力类别（field=all 时用）：native / current / burial
	Categories []string `json:"categories,omitempty"`
	// 现居地为空、按祖籍落点显示（未迁出）
	CurrentFromNative bool `json:"current_from_native,omitempty"`
}

func sortGeo(m map[string]int) []heatGeoItem {
	out := make([]heatGeoItem, 0, len(m))
	for k, v := range m {
		out = append(out, heatGeoItem{Province: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Province < out[j].Province
	})
	return out
}

// heatQuery 构造成员查询（共用筛选条件）
func heatQuery(c *gin.Context, db *sql.DB, cols string) (string, []any) {
	q := "SELECT " + cols + " FROM members WHERE 1=1"
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
	return q, args
}

// heatCatNative / heatCatFallback / heatCatBurial 热力类别（与前端图例一一对应）
const (
	heatCatNative   = "native"
	heatCatCurrent  = "current"  // 真实填报的现居地
	heatCatFallback = "fallback" // 现居地为空 → 落回祖籍（未迁出）
	heatCatBurial   = "burial"
)

// heatCatAll 默认（不传 cats）时启用的全部类别
var heatCatAll = map[string]bool{heatCatNative: true, heatCatCurrent: true, heatCatFallback: true, heatCatBurial: true}

// heatCatsOf 解析 cats 查询参数（逗号分隔的类别白名单，来自「全部」模式图例的点选开关）。
// 不传 / 传空 = 全部启用；传了但一个都不认识 = 空集（图例全关 → 什么都不画）。
// 只认 native/current/fallback/burial 四个键，避免任意字符串影响查询。
func heatCatsOf(c *gin.Context) map[string]bool {
	raw := strings.TrimSpace(c.Query("cats"))
	if raw == "" {
		return map[string]bool{heatCatNative: true, heatCatCurrent: true, heatCatFallback: true, heatCatBurial: true}
	}
	out := map[string]bool{}
	for _, p := range strings.Split(raw, ",") {
		switch strings.TrimSpace(p) {
		case heatCatNative:
			out[heatCatNative] = true
		case heatCatCurrent:
			out[heatCatCurrent] = true
		case heatCatFallback:
			out[heatCatFallback] = true
		case heatCatBurial:
			out[heatCatBurial] = true
		}
	}
	return out
}

// heatCatOn 某类别是否启用
func heatCatOn(cats map[string]bool, k string) bool { return cats[k] }

// heatCatsParam cats → 查询参数（空集写成 none，避免被当成"未传=全开"）
func heatCatsParam(cats map[string]bool) string {
	if len(cats) == 0 {
		return "none"
	}
	ks := make([]string, 0, 4)
	for _, k := range []string{heatCatNative, heatCatCurrent, heatCatFallback, heatCatBurial} {
		if cats[k] {
			ks = append(ks, k)
		}
	}
	return strings.Join(ks, ",")
}

// heatRowCats 一行的可用类别（省份可识别才算可用）
func heatRowCats(npProv, cpProv, bpProv string, cpFromNative bool) []string {
	out := []string{}
	if npProv != "" {
		out = append(out, heatCatNative)
	}
	if cpFromNative {
		out = append(out, heatCatFallback)
	} else if cpProv != "" {
		out = append(out, heatCatCurrent)
	}
	if bpProv != "" {
		out = append(out, heatCatBurial)
	}
	return out
}

// heatRowHit 该行是否命中「省 + 可选城市」筛选（cats 为启用的类别白名单）
func heatRowHit(prov, city, npProv, bpProv string, np, cp, bp string, cats map[string]bool) bool {
	cpProv := detectProvince(cp)
	cpFromNative := false
	if cpProv == "" && npProv != "" {
		cpProv, cpFromNative = npProv, true
	}
	if heatCatOn(cats, heatCatNative) && provHit(prov, city, npProv, np) {
		return true
	}
	if heatCatOn(cats, heatCatFallback) && cpFromNative && provHit(prov, city, cpProv, cp, np) {
		return true
	}
	if heatCatOn(cats, heatCatCurrent) && !cpFromNative && provHit(prov, city, cpProv, cp) {
		return true
	}
	if heatCatOn(cats, heatCatBurial) && provHit(prov, city, bpProv, bp) {
		return true
	}
	return false
}

// GetHeatmap 家族热力分布聚合数据
// GET /api/heatmap
func (h *HeatmapHandler) GetHeatmap(c *gin.Context) {
	cols := "COALESCE(id,0), COALESCE(name,''), COALESCE(native_place,''), COALESCE(current_place,''), COALESCE(burial_place,''), " +
		"COALESCE(branch,''), COALESCE(generation,0), COALESCE(birth_gregorian,''), COALESCE(death_gregorian,'')"
	q, args := heatQuery(c, h.DB, cols)
	rows, err := h.DB.Query(q, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	nativeCount := map[string]int{}
	currentCount := map[string]int{}
	burialCount := map[string]int{}
	cityNative := map[string]map[string]int{}
	cityCurrent := map[string]map[string]int{}
	cityBurial := map[string]map[string]int{}
	branchCount := map[string]int{}
	birthCount := map[int]int{}
	deathCount := map[int]int{}
	migCount := map[string]int{}
	migFrom, migTo := map[string]string{}, map[string]string{}
	total, nativeUnknown, burialUnknown, currentUnknown := 0, 0, 0, 0
	// 现居地落回祖籍的统计（未迁出者）：currentFromNative = 落回数，currentDeclaredTotal = 真实现居地数
	currentFromNative, currentDeclaredTotal := 0, 0
	// 「纯填报现居地」的省级聚合（图例里「现居地」这一行用它，避免把未迁出的人算进去）
	currentDeclaredCount := map[string]int{}
	// 图例点选开关（cats）：关掉的类别不进聚合，页面上就不会有它的数
	cats := heatCatsOf(c)

	for rows.Next() {
		var id int64
		var name, np, cp, bp, branch, birth, death string
		var gen int
		if err := rows.Scan(&id, &name, &np, &cp, &bp, &branch, &gen, &birth, &death); err != nil {
			continue
		}
		total++
		if branch == "" {
			branch = "未分房"
		}
		branchCount[branch]++

		npProv := detectProvince(np)
		if npProv == "" {
			if strings.TrimSpace(np) != "" {
				nativeUnknown++
			}
		} else if heatCatOn(cats, heatCatNative) {
			nativeCount[npProv]++
		}
		// 现居地：填了按现居地算；没填（或填了但识别不出省份）则落回祖籍
		// （未迁出的人只填祖籍 → 现居地视图里也要有点，见 cpFromNative）
		cpFromNative := false
		cpProv := detectProvince(cp)
		if cpProv == "" {
			if strings.TrimSpace(cp) != "" {
				currentUnknown++
			}
			if npProv != "" {
				cpProv = npProv
				cpFromNative = true
			}
		}
		if cpProv != "" {
			if cpFromNative {
				if heatCatOn(cats, heatCatFallback) {
					currentCount[cpProv]++
					currentFromNative++
				}
			} else if heatCatOn(cats, heatCatCurrent) {
				currentCount[cpProv]++
				currentDeclaredCount[cpProv]++
				currentDeclaredTotal++
			}
		}
		bpProv := detectProvince(bp)
		if bpProv == "" {
			if strings.TrimSpace(bp) != "" {
				burialUnknown++
			}
		} else if heatCatOn(cats, heatCatBurial) {
			burialCount[bpProv]++
		}
		if heatCatOn(cats, heatCatNative) {
			addCity(cityNative, npProv, cityKey(npProv, np))
		}
		// 现居地落回祖籍时，城市也跟着取祖籍城市（下钻能看到这些人）
		cpCity := cityKey(cpProv, cp)
		if cpFromNative {
			cpCity = cityKey(npProv, np)
			if heatCatOn(cats, heatCatFallback) {
				addCityN(cityCurrent, cpProv, cpCity, 1)
			}
		} else if heatCatOn(cats, heatCatCurrent) {
			addCity(cityCurrent, cpProv, cpCity)
		}
		if heatCatOn(cats, heatCatBurial) {
			addCity(cityBurial, bpProv, cityKey(bpProv, bp))
		}
		if npProv != "" && bpProv != "" && npProv != bpProv &&
			heatCatOn(cats, heatCatNative) && heatCatOn(cats, heatCatBurial) {
			key := npProv + "|" + bpProv
			migCount[key]++
			migFrom[key], migTo[key] = npProv, bpProv
		}

		if y := heatYearOf(birth); y > 0 {
			birthCount[y]++
		}
		if y := heatYearOf(death); y > 0 {
			deathCount[y]++
		}
	}

	// 迁徙事件（手工录入的优先）
	migration := []heatMigItem{}
	mrows, mErr := h.DB.Query(`SELECT COALESCE(from_place,''), COALESCE(to_place,''), COALESCE(era,''),
		COALESCE(year,0), COALESCE(member_count,0), COALESCE(description,'')
		FROM migrations ORDER BY year, id`)
	if mErr != nil {
		log.Printf("[heatmap] 迁徙事件查询失败: %v", mErr)
	} else {
		for mrows.Next() {
			var f, t, era, desc string
			var y, cnt int
			if err := mrows.Scan(&f, &t, &era, &y, &cnt, &desc); err != nil {
				log.Printf("[heatmap] 迁徙事件扫描失败: %v", err)
				continue
			}
			migration = append(migration, heatMigItem{
				From: f, To: t, FromProv: detectProvince(f), ToProv: detectProvince(t),
				Count: cnt, Era: era, Year: y, Description: desc, Source: "event",
			})
		}
		mrows.Close()
	}

	// 迁徙事件：尽力把「迁出地/迁入地」解析成经纬度（优先读缓存，未命中的按需调用第三方编码）
	if len(migration) > 0 {
		places := make([]string, 0, len(migration)*2)
		seen := map[string]bool{}
		for _, it := range migration {
			for _, p := range []string{it.From, it.To} {
				p = strings.TrimSpace(p)
				if p != "" && !seen[p] {
					seen[p] = true
					places = append(places, p)
				}
			}
		}
		coords := geoResolve(h.DB, places, 20)
		for i := range migration {
			if c, ok := coords[strings.TrimSpace(migration[i].From)]; ok {
				migration[i].FromLng, migration[i].FromLat = c[0], c[1]
			}
			if c, ok := coords[strings.TrimSpace(migration[i].To)]; ok {
				migration[i].ToLng, migration[i].ToLat = c[0], c[1]
			}
		}
	}

	// 无手工记录时，用「籍贯省 → 葬地省」推算（近似）
	computed := make([]heatMigItem, 0, len(migCount))
	for k, v := range migCount {
		computed = append(computed, heatMigItem{From: migFrom[k], To: migTo[k], FromProv: migFrom[k], ToProv: migTo[k], Count: v, Source: "computed"})
	}
	sort.Slice(computed, func(i, j int) bool {
		if computed[i].Count != computed[j].Count {
			return computed[i].Count > computed[j].Count
		}
		return computed[i].From+computed[i].To < computed[j].From+computed[j].To
	})
	if len(computed) > 40 {
		computed = computed[:40]
	}

	branchList := make([]heatBranchItem, 0, len(branchCount))
	for k, v := range branchCount {
		branchList = append(branchList, heatBranchItem{Name: k, Count: v})
	}
	sort.Slice(branchList, func(i, j int) bool {
		if branchList[i].Count != branchList[j].Count {
			return branchList[i].Count > branchList[j].Count
		}
		return branchList[i].Name < branchList[j].Name
	})

	// 房支下拉选项（不受筛选影响）
	opts := []string{}
	if orows, err := h.DB.Query(`SELECT COALESCE(branch,'') FROM members GROUP BY branch ORDER BY branch`); err == nil {
		for orows.Next() {
			var b string
			if err := orows.Scan(&b); err == nil && strings.TrimSpace(b) != "" {
				opts = append(opts, b)
			}
		}
		orows.Close()
	}

	c.JSON(http.StatusOK, gin.H{
		"total":         total,
		"branchOptions": opts,
		"geo": gin.H{
			"native":         sortGeo(nativeCount),
			"current":        sortGeo(currentCount),
			"burial":         sortGeo(burialCount),
			"nativeUnknown":  nativeUnknown,
			"burialUnknown":  burialUnknown,
			"currentUnknown": currentUnknown,
			// 现居地为空的成员落回祖籍：current 里含这部分人，前端据此标注「现居地同祖籍（未迁出）」
			"currentFromNative": currentFromNative,
			"currentDeclared":   currentDeclaredTotal,
			"currentFallback":   true,
			// 「纯填报现居地」的省级聚合（不含落回者）：图例要分别显示「现居地」与「未迁出」的省级人数
			"currentDeclaredGeo": sortGeo(currentDeclaredCount),
		},
		"city": gin.H{
			"native":  sortCityMap(cityNative),
			"current": sortCityMap(cityCurrent),
			"burial":  sortCityMap(cityBurial),
		},
		"branch":               branchList,
		"timeline":             gin.H{"birth": birthCount, "death": deathCount},
		"migration":            migration,
		"migrationComputed":    computed,
		"hasMigrationEvents":   len(migration) > 0,
		"hasLocation":          len(nativeCount) > 0 || len(burialCount) > 0 || len(currentCount) > 0,
		"hasCurrentPlaceField": true,
		// 本次实际启用的类别（图例点选开关）→ 前端保持一致
		"cats": heatCatsParam(cats),
	})
}

// GetHeatmapMembers 某省成员明细
// GET /api/heatmap/members?field=native|current|burial|all&province=江西省&city=泉州市
// 说明：field=current 时，现居地为空的成员落回祖籍（未迁出），与热力图一致；
// field=all 时返回三类中任意一类命中的成员，并用 categories 标明命中了哪几类。
func (h *HeatmapHandler) GetHeatmapMembers(c *gin.Context) {
	field := c.Query("field")
	if field != "burial" && field != "current" && field != "all" {
		field = "native"
	}
	prov := c.Query("province")
	city := c.Query("city")
	// 超管/理事长/理事可见完整地址；其余角色只到市一级
	full := viewerSensitivity(c.GetString("role")) >= SensAdmin

	cols := "COALESCE(id,0), COALESCE(name,''), COALESCE(generation,0), COALESCE(branch,''), " +
		"COALESCE(native_place,''), COALESCE(current_place,''), COALESCE(burial_place,''), COALESCE(death_gregorian,'')"
	q, args := heatQuery(c, h.DB, cols)
	q += " ORDER BY generation, id LIMIT 1000"

	rows, err := h.DB.Query(q, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	mask := func(s string) string {
		if full {
			return s
		}
		return truncatePlace(s)
	}

	list := []heatMemberRow{}
	// 图例点选开关（cats）：关掉的类别不出现在明细里，保证与地图口径一致
	cats := heatCatsOf(c)
	for rows.Next() {
		var death string
		var m heatMemberRow
		if err := rows.Scan(&m.ID, &m.Name, &m.Generation, &m.Branch, &m.NativePlace, &m.CurrentPlace, &m.BurialPlace, &death); err != nil {
			continue
		}
		m.Alive = strings.TrimSpace(death) == ""
		nProv := detectProvince(m.NativePlace)
		cProv := detectProvince(m.CurrentPlace)
		if cProv == "" && nProv != "" { // 现居地为空 → 落回祖籍
			cProv = nProv
			m.CurrentFromNative = true
		}
		bProv := detectProvince(m.BurialPlace)

		// 该成员命中的类别（未迁出者算 fallback，不算 current）
		all := heatRowCats(nProv, cProv, bProv, m.CurrentFromNative)
		hit := false
		switch field {
		case "all":
			hit = heatRowHit(prov, city, nProv, bProv, m.NativePlace, m.CurrentPlace, m.BurialPlace, cats)
		case "burial":
			hit = heatCatOn(cats, heatCatBurial) && provHit(prov, city, bProv, m.BurialPlace)
		case "current":
			place := m.CurrentPlace
			if m.CurrentFromNative {
				place = m.NativePlace
			}
			if m.CurrentFromNative {
				hit = heatCatOn(cats, heatCatFallback) && provHit(prov, city, cProv, place, m.NativePlace)
			} else {
				hit = heatCatOn(cats, heatCatCurrent) && provHit(prov, city, cProv, place)
			}
		default:
			hit = heatCatOn(cats, heatCatNative) && provHit(prov, city, nProv, m.NativePlace)
		}
		if !hit {
			continue
		}
		m.Categories = all
		m.NativePlace = mask(m.NativePlace)
		m.CurrentPlace = mask(m.CurrentPlace)
		m.BurialPlace = mask(m.BurialPlace)
		list = append(list, m)
	}
	c.JSON(http.StatusOK, gin.H{"province": prov, "city": city, "field": field, "full": full, "cats": heatCatsParam(cats), "list": list})
}

// provHit 判断某条明细是否落在「省 + 可选城市」筛选内。
// placeProv 是该类别用于判断的省；places 用于市一级匹配（现居地落回祖籍时也能对上祖籍城市）。
func provHit(prov, city, placeProv string, places ...string) bool {
	if placeProv != prov {
		return false
	}
	if city == "" {
		return true
	}
	for _, p := range places {
		if strings.TrimSpace(p) != "" && cityKey(placeProv, p) == city {
			return true
		}
	}
	return false
}
