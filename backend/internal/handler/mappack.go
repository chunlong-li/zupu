package handler

import (
	"archive/tar"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"familytree/internal/config"

	"github.com/gin-gonic/gin"
)

// ===== 离线底图包（导入 + 管理）=====
// 导入的离线包解压到 <DataDir>/map/<包名>/<层>/<z>/<x>/<y>.<ext>，包名 = 上传的文件名（去掉扩展名）。
// 本系统不提供「从在线地图批量下载瓦片」，也不提供导出/镜像分发（涉及批量获取地图数据，不符合地图服务条款）。

// 中国大致范围（历史遗留常量，保留）
const (
	cnMinLon = 73.0
	cnMaxLon = 136.0
	cnMinLat = 17.0
	cnMaxLat = 54.0
)

type MapPackStatus struct {
	State      string `json:"state"` // none|downloading|done|partial|error
	Total      int    `json:"total"`
	Done       int    `json:"done"`
	Failed     int    `json:"failed"`
	Bytes      int64  `json:"bytes"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	Error      string `json:"error"`
	Source     string `json:"source"` // import
	Unit       string `json:"unit"`   // tiles | bytes
	Phase      string `json:"phase"`  // 下载中 | 解压中
}

type mapPackMeta struct {
	Status MapPackStatus     `json:"status"`
	Layers []string          `json:"layers"`
	Ext    map[string]string `json:"ext"`
}

type MapPackHandler struct {
	DB         *sql.DB
	DataDir    string
	Cfg        *config.Config
	mu         sync.Mutex
	quotaMu    sync.Mutex
	quotaUntil time.Time // 天地图服务端 Key 额度用尽 -> 该时间前视为不可用（前端自动走浏览器端 API）
	status     map[string]*MapPackStatus
	onMu       sync.Mutex
	onCache    map[string]onTile
}

// onTile 在线瓦片内存缓存（代理天地图用）
type onTile struct {
	ct string
	b  []byte
}

// EnsureStart 供启动时调用：初始化并开启后台检查（额度用尽自动续传 / 定时保存）
func (h *MapPackHandler) EnsureStart() { h.ensureInit() }

func (h *MapPackHandler) ensureInit() {
	if h.status == nil {
		h.status = map[string]*MapPackStatus{}
	}
}

func (h *MapPackHandler) mapRoot() string { return filepath.Join(h.DataDir, "map") }

func (h *MapPackHandler) packDir(id string) string { return filepath.Join(h.mapRoot(), id) }

// safePackName 规范化包名（= 导入时的文件名）：只取单层名字，去掉路径分隔符与首尾点
func safePackName(s string) string {
	s = strings.TrimSpace(filepath.Base(filepath.ToSlash(s)))
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.Trim(s, ".")
	if s == "" {
		s = "offline-" + time.Now().Format("20060102-150405")
	}
	return s
}

// listPackDirs 列出 map 目录下已导入的包（一个子目录 = 一个包）
func (h *MapPackHandler) listPackDirs() []string {
	ents, err := os.ReadDir(h.mapRoot())
	if err != nil {
		return nil
	}
	out := []string{}
	for _, e := range ents {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// scanZoomRange 扫描包目录里实际存在的缩放级别范围（看 base/label 下的数字目录）
func scanZoomRange(dir string) (int, int) {
	minZ, maxZ := 0, 0
	for _, layer := range []string{"base", "label"} {
		ents, err := os.ReadDir(filepath.Join(dir, layer))
		if err != nil {
			continue
		}
		for _, e := range ents {
			if !e.IsDir() {
				continue
			}
			z, err := strconv.Atoi(e.Name())
			if err != nil {
				continue
			}
			if minZ == 0 || z < minZ {
				minZ = z
			}
			if z > maxZ {
				maxZ = z
			}
		}
	}
	if minZ == 0 {
		minZ, maxZ = 3, 8
	}
	return minZ, maxZ
}

// promoteRoot 归一化导入目录：若解压后顶层没有 base/label、且只有一个子目录，
// 就把该子目录的内容提升到顶层（兼容「文件名/包ID/base/...」这类多层包裹）。
func promoteRoot(dest string) {
	for i := 0; i < 4; i++ {
		ents, err := os.ReadDir(dest)
		if err != nil {
			return
		}
		hasLayer := false
		var dirs []string
		for _, e := range ents {
			if e.Name() == "base" || e.Name() == "label" {
				hasLayer = true
			}
			if e.IsDir() {
				dirs = append(dirs, e.Name())
			}
		}
		if hasLayer || len(dirs) != 1 {
			return
		}
		sub := filepath.Join(dest, dirs[0])
		ses, err := os.ReadDir(sub)
		if err != nil {
			return
		}
		moved := false
		for _, e := range ses {
			src := filepath.Join(sub, e.Name())
			dst := filepath.Join(dest, e.Name())
			if _, err := os.Stat(dst); err == nil {
				continue
			}
			if os.Rename(src, dst) == nil {
				moved = true
			}
		}
		if !moved {
			return
		}
		os.Remove(sub)
	}
}

// ---- 瓦片源（天地图，需 token）----

func lonLatToTile(lon, lat float64, z int) (int, int) {
	n := math.Pow(2, float64(z))
	x := int((lon + 180.0) / 360.0 * n)
	latRad := lat * math.Pi / 180.0
	y := int((1.0 - math.Log(math.Tan(latRad)+1.0/math.Cos(latRad))/math.Pi) / 2.0 * n)
	return x, y
}

// tiandituTileURL 天地图瓦片（需 token）；layerBase: vec | cva | img | cia | ter | cta
func tiandituTileURL(layerBase string, z, x, y int, tk string) string {
	sub := []string{"0", "1", "2", "3", "4", "5", "6", "7"}[(x+y)%8]
	return fmt.Sprintf("https://t%s.tianditu.gov.cn/%s_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=%s&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILEMATRIX=%d&TILEROW=%d&TILECOL=%d&tk=%s",
		sub, layerBase, layerBase, z, y, x, tk)
}

func (h *MapPackHandler) mapToken() string {
	var v string
	h.DB.QueryRow(`SELECT value FROM settings WHERE key='map_token'`).Scan(&v)
	return strings.TrimSpace(v)
}

// sourceFor 返回当前源与图层列表（统一天地图：底图 vec + 注记 cva）
func (h *MapPackHandler) sourceFor() (string, []string) {
	return "tianditu", []string{"base", "label"}
}

// ---- 天地图额度：限流检测 / 自动暂停 / 次日恢复 ----

// errTkLimited 天地图返回「该tk已限流」(HTTP 429 / code 302010)
var errTkLimited = errors.New("天地图该 Key 已限流")

// ---- meta 持久化 ----

func (h *MapPackHandler) readMeta(id string) *mapPackMeta {
	b, err := os.ReadFile(filepath.Join(h.packDir(id), "meta.json"))
	if err != nil {
		return nil
	}
	var m mapPackMeta
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	return &m
}

func (h *MapPackHandler) writeMeta(id string, m *mapPackMeta) {
	_ = os.MkdirAll(h.packDir(id), 0o755)
	b, _ := json.MarshalIndent(m, "", "  ")
	_ = os.WriteFile(filepath.Join(h.packDir(id), "meta.json"), b, 0o644)
}

// ---- HTTP handlers ----

type packInfo struct {
	ID         string        `json:"id"`   // = 目录名 = 导入时的文件名（去扩展名）
	Name       string        `json:"name"` // 列表显示名
	Tiles      int           `json:"tiles"`
	Bytes      int64         `json:"bytes"`
	ImportedAt string        `json:"imported_at"`
	MinZoom    int           `json:"min_zoom"`
	MaxZoom    int           `json:"max_zoom"`
	HasTiles   bool          `json:"has_tiles"` // 本地是否已有瓦片（可用于热力图离线底图）
	Status     MapPackStatus `json:"status"`
}

// GetPacks 列出已导入的离线包（按文件名列，像「备份」那样的列表）
// GET /api/map/packs
func (h *MapPackHandler) GetPacks(c *gin.Context) {
	h.ensureInit()
	out := []packInfo{}
	for _, id := range h.listPackDirs() {
		dir := h.packDir(id)
		h.mu.Lock()
		status := MapPackStatus{}
		if s, ok := h.status[id]; ok && s != nil {
			status = *s
		}
		h.mu.Unlock()
		if status.Done == 0 {
			if m := h.readMeta(id); m != nil && m.Status.Done > status.Done {
				status = m.Status
			}
		}
		tiles, bytes := status.Done, status.Bytes
		if tiles == 0 {
			tiles, bytes = countPackTiles(dir)
		}
		if status.State == "" || status.State == "none" {
			status.State = "done"
		}
		status.Done, status.Total, status.Bytes, status.Unit = tiles, tiles, bytes, "tiles"
		imp := status.FinishedAt
		if imp == "" {
			if fi, err := os.Stat(dir); err == nil {
				imp = fi.ModTime().Format("2006-01-02 15:04:05")
			}
		}
		mn, mx := scanZoomRange(dir)
		out = append(out, packInfo{ID: id, Name: id, Tiles: tiles, Bytes: bytes, ImportedAt: imp, MinZoom: mn, MaxZoom: mx, HasTiles: h.hasTiles(id), Status: status})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ImportedAt > out[j].ImportedAt })
	c.JSON(http.StatusOK, gin.H{"packs": out, "token_configured": h.mapToken() != ""})
}

// GetConfig 读取地图配置
// GET /api/map/config
func (h *MapPackHandler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"token_set":    h.mapToken() != "",
		"token_masked": maskKey(h.mapToken()),
	})
}

// SaveConfig 保存地图配置（token；仅更新传入的字段）
// PUT /api/map/config  { token? }
func (h *MapPackHandler) SaveConfig(c *gin.Context) {
	var body struct {
		Token *string `json:"token"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Token != nil {
		if _, err := h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('map_token', ?)`, strings.TrimSpace(*body.Token)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// onlineConfig 读取「在线底图」配置（免 Key 源 + 天地图【浏览器端】Key，供热力图页面直接加载瓦片）
func (h *MapPackHandler) onlineConfig() (string, string, string) {
	var tk, layer, source string
	h.DB.QueryRow(`SELECT value FROM settings WHERE key='map_online_tk'`).Scan(&tk)
	h.DB.QueryRow(`SELECT value FROM settings WHERE key='map_online_layer'`).Scan(&layer)
	h.DB.QueryRow(`SELECT value FROM settings WHERE key='map_online_source'`).Scan(&source)
	if layer != "vec" && layer != "img" && layer != "ter" {
		layer = "vec"
	}
	if !isOnlineSource(source) {
		source = "tianditu"
	}
	return source, tk, layer
}

// 可选的在线底图源；除了天地图外都是无需 Key 的免费源（高德 / Esri，均实测大陆可直达）
var mapOnlineSources = []string{"tianditu", "amap", "amap_sat", "esri_sat"}

func isOnlineSource(s string) bool {
	for _, v := range mapOnlineSources {
		if v == s {
			return true
		}
	}
	return false
}

// GetOnlineConfig 读取在线底图配置
// GET /api/map/online-config
func (h *MapPackHandler) GetOnlineConfig(c *gin.Context) {
	source, tkBrowser, layer := h.onlineConfig()
	stk := h.mapToken()
	c.JSON(http.StatusOK, gin.H{
		"source":           source,
		"sources":          mapOnlineSources,
		"tianditu":         source == "tianditu",
		"layer":            layer,
		"tk":               tkBrowser, // 浏览器端 Key（直连渲染需要；仅登录用户可取）
		"tk_set":           tkBrowser != "",
		"tk_masked":        maskKey(tkBrowser),
		"server_tk_set":    stk != "",
		"server_tk_masked": maskKey(stk),
		"proxy":            h.onlineProxy(),
		"quota_exhausted":  h.quotaExhausted(),
	})
}

// onlineProxy 在线天地图瓦片是否经服务器中转（true=Key 不下发浏览器）
func (h *MapPackHandler) onlineProxy() bool {
	var v string
	h.DB.QueryRow(`SELECT value FROM settings WHERE key='map_online_proxy'`).Scan(&v)
	return v == "1"
}

// ---- 天地图服务端 Key 额度用尽：标记 + 对外暴露（前端据此自动改用浏览器端 API 直连）----

func (h *MapPackHandler) markQuotaExhausted() {
	h.quotaMu.Lock()
	h.quotaUntil = time.Now().Add(30 * time.Minute)
	h.quotaMu.Unlock()
}

func (h *MapPackHandler) clearQuotaExhausted() {
	h.quotaMu.Lock()
	h.quotaUntil = time.Time{}
	h.quotaMu.Unlock()
}

// quotaExhausted 服务端 Key 是否处于「额度用尽」状态
func (h *MapPackHandler) quotaExhausted() bool {
	h.quotaMu.Lock()
	defer h.quotaMu.Unlock()
	return !h.quotaUntil.IsZero() && time.Now().Before(h.quotaUntil)
}

// onlineLayerToPack 天地图在线图层 → 包内层名 + 该层是否与智能包（天地图 vec/cva）同内容
func onlineLayerToPack(layer string) (string, bool) {
	switch layer {
	case "vec":
		return "base", true
	case "cva":
		return "label", true
	case "img", "ter":
		return "base", false
	case "cia", "cta":
		return "label", false
	}
	return "", false
}

// ServeOnlineTile 天地图在线瓦片代理（用服务端 Key，浏览器拿不到 Key）
// GET /api/map/online/:layer/:z/:x/:y?token=xxx
func (h *MapPackHandler) ServeOnlineTile(c *gin.Context) {
	layer := c.Param("layer")
	switch layer {
	case "vec", "img", "ter", "cva", "cia", "cta":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的图层"})
		return
	}
	z, err1 := strconv.Atoi(c.Param("z"))
	x, err2 := strconv.Atoi(c.Param("x"))
	y, err3 := strconv.Atoi(c.Param("y"))
	n := 1 << uint(maxInt(z, 0))
	if err1 != nil || err2 != nil || err3 != nil || z < 1 || z > 18 || x < 0 || y < 0 || x >= n || y >= n {
		c.JSON(http.StatusBadRequest, gin.H{"error": "瓦片坐标不合法"})
		return
	}
	key := layer + "/" + strconv.Itoa(z) + "/" + strconv.Itoa(x) + "/" + strconv.Itoa(y)
	// ① 内存缓存
	if ct, b, ok := h.onCacheGet(key); ok {
		c.Header("Cache-Control", "public, max-age=86400")
		c.Data(http.StatusOK, ct, b)
		return
	}
	// ② 本地已有就不再向天地图请求（省额度，断网/额度用尽也能用）
	tk := h.mapToken()
	if tk == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未配置天地图「服务端」Key"})
		return
	}
	u := tiandituTileURL(layer, z, x, y, tk)
	b, ct, err := fetchTileBytes(&http.Client{Timeout: 15 * time.Second}, u)
	if err != nil {
		if errors.Is(err, errTkLimited) {
			// 服务端 Key 额度用尽：标记后让前端自动改用「浏览器端」Key 直连天地图
			h.markQuotaExhausted()
			c.Header("X-Map-Quota", "exhausted")
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "天地图服务端 Key 额度用尽", "quota_exhausted": true})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "瓦片获取失败：" + err.Error()})
		return
	}
	h.clearQuotaExhausted()
	h.onCachePut(key, ct, b)
	c.Header("Cache-Control", "public, max-age=86400")
	c.Data(http.StatusOK, ct, b)
}

func (h *MapPackHandler) onCacheGet(k string) (string, []byte, bool) {
	h.onMu.Lock()
	defer h.onMu.Unlock()
	v, ok := h.onCache[k]
	if !ok {
		return "", nil, false
	}
	return v.ct, v.b, true
}

func (h *MapPackHandler) onCachePut(k, ct string, b []byte) {
	h.onMu.Lock()
	defer h.onMu.Unlock()
	if h.onCache == nil {
		h.onCache = map[string]onTile{}
	}
	if len(h.onCache) > 3000 {
		h.onCache = map[string]onTile{}
	}
	h.onCache[k] = onTile{ct: ct, b: b}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// SaveOnlineConfig 保存在线底图配置
// PUT /api/map/online-config  { source?, tk?, layer?, clear? }
func (h *MapPackHandler) SaveOnlineConfig(c *gin.Context) {
	var body struct {
		Source      *string `json:"source"`
		TK          *string `json:"tk"`
		ServerTK    *string `json:"server_tk"`
		Layer       *string `json:"layer"`
		Proxy       *bool   `json:"proxy"`
		Clear       bool    `json:"clear"`
		ClearServer bool    `json:"clear_server"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Clear {
		h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('map_online_tk', '')`)
	} else if body.TK != nil {
		if tk := strings.TrimSpace(*body.TK); tk != "" {
			if _, err := h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('map_online_tk', ?)`, tk); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
	}
	if body.ClearServer {
		h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('map_token', '')`)
	} else if body.ServerTK != nil {
		if tk := strings.TrimSpace(*body.ServerTK); tk != "" {
			if _, err := h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('map_token', ?)`, tk); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
	}
	if body.Proxy != nil {
		v := "0"
		if *body.Proxy {
			v = "1"
		}
		h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('map_online_proxy', ?)`, v)
	}
	if body.Source != nil {
		s := strings.TrimSpace(*body.Source)
		if !isOnlineSource(s) {
			s = "tianditu"
		}
		if _, err := h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('map_online_source', ?)`, s); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if body.Layer != nil {
		l := strings.TrimSpace(*body.Layer)
		if l != "vec" && l != "img" && l != "ter" {
			l = "vec"
		}
		if _, err := h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('map_online_layer', ?)`, l); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// extractTarGz 解压 tar.gz 到 dest；strip 为需要去掉的顶层目录名（如 "A"）
func extractMapTarGz(r io.Reader, dest, strip string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	destClean := filepath.Clean(dest)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		name = strings.TrimPrefix(name, "./")
		if strip != "" {
			name = strings.TrimPrefix(name, strip+"/")
		}
		if name == "" || name == "." || filepath.IsAbs(name) || strings.HasPrefix(name, "..") || name == "meta.json" {
			continue
		}
		target := filepath.Join(destClean, name)
		if target != destClean && !strings.HasPrefix(target, destClean+string(os.PathSeparator)) {
			continue
		}
		if hdr.Typeflag == tar.TypeDir {
			os.MkdirAll(target, 0o755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		out.Close()
	}
	return nil
}

// hasTiles 该包目录下是否已有瓦片文件
func (h *MapPackHandler) hasTiles(id string) bool {
	dir := h.packDir(id)
	found := false
	filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !fi.IsDir() && filepath.Base(p) != "meta.json" {
			found = true
			return io.EOF
		}
		return nil
	})
	return found
}

// countPackTiles 统计包目录下的瓦片文件数与字节数（不含 meta.json）
func countPackTiles(dir string) (int, int64) {
	n := 0
	var bytes int64
	filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || filepath.Base(p) == "meta.json" {
			return nil
		}
		n++
		bytes += fi.Size()
		return nil
	})
	return n, bytes
}

// ImportPack 导入离线包：上传 .tar.gz，按「文件名」建立条目并解压到 map/<文件名>/
// POST /api/map/import-pack  （仅理事长/超级管理员）
func (h *MapPackHandler) ImportPack(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要导入的离线包文件（.tar.gz）"})
		return
	}
	defer file.Close()
	if header != nil && header.Size > 10<<30 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件过大（上限 10GB）"})
		return
	}

	// 包名 = 上传的文件名（去掉 .tar.gz/.tgz/.tar/.gz 扩展名）
	name := ""
	if header != nil {
		base := filepath.Base(filepath.ToSlash(header.Filename))
		low := strings.ToLower(base)
		for _, ext := range []string{".tar.gz", ".tgz", ".tar", ".gz"} {
			if strings.HasSuffix(low, ext) {
				base = base[:len(base)-len(ext)]
				break
			}
		}
		name = base
	}
	name = safePackName(name)

	tmp, err := os.CreateTemp("", "mappack-import-*.tar.gz")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取上传内容失败：" + err.Error()})
		return
	}
	tmp.Close()

	f, err := os.Open(tmpPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()
	dest := h.packDir(name)
	os.MkdirAll(dest, 0o755)
	if err := extractMapTarGz(f, dest, ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解压失败：" + err.Error()})
		return
	}
	promoteRoot(dest) // 兼容「文件名/包ID/base/...」多层包裹

	tiles, bytes := countPackTiles(dest)
	if tiles == 0 {
		os.RemoveAll(dest)
		c.JSON(http.StatusBadRequest, gin.H{"error": "归档内没有瓦片文件（需含 base/ 或 label/ 目录），导入已中止"})
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	_, layers := h.sourceFor()
	h.writeMeta(name, &mapPackMeta{
		Status: MapPackStatus{State: "done", Done: tiles, Total: tiles, Bytes: bytes, Unit: "tiles", Source: "import", StartedAt: now, FinishedAt: now},
		Layers: layers,
		Ext:    map[string]string{},
	})
	h.mu.Lock()
	h.status[name] = &MapPackStatus{State: "done", Done: tiles, Total: tiles, Bytes: bytes, Unit: "tiles", Source: "import", StartedAt: now, FinishedAt: now}
	h.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"ok": true, "id": name, "name": name, "tiles": tiles, "bytes": bytes})
}

// DeletePack 删除已导入的地图包
// DELETE /api/map/packs/:id
func (h *MapPackHandler) DeletePack(c *gin.Context) {
	h.ensureInit()
	id := safePackName(c.Param("id"))
	dir := h.packDir(id)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		c.JSON(http.StatusNotFound, gin.H{"error": "地图包不存在"})
		return
	}
	h.mu.Lock()
	delete(h.status, id)
	h.mu.Unlock()
	if err := os.RemoveAll(dir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ServeTile 提供已下载的本地瓦片
// GET /api/map/tiles/:pack/:layer/:z/:x/:y
func (h *MapPackHandler) ServeTile(c *gin.Context) {
	pack := safePackName(c.Param("pack"))
	layer := c.Param("layer")
	z, e1 := strconv.Atoi(c.Param("z"))
	x, e2 := strconv.Atoi(c.Param("x"))
	y, e3 := strconv.Atoi(c.Param("y"))
	if e1 != nil || e2 != nil || e3 != nil || !validXYZ(z, x, y) {
		c.Status(http.StatusBadRequest)
		return
	}
	if pack == "" || (layer != "base" && layer != "label") {
		c.Status(http.StatusBadRequest)
		return
	}
	if fi, err := os.Stat(h.packDir(pack)); err != nil || !fi.IsDir() {
		c.Status(http.StatusNotFound)
		return
	}
	dir := filepath.Join(h.packDir(pack), layer, strconv.Itoa(z), strconv.Itoa(x))
	m, err := filepath.Glob(filepath.Join(dir, strconv.Itoa(y)+".*"))
	if err != nil || len(m) == 0 {
		c.Status(http.StatusNotFound)
		return
	}
	c.File(m[0])
}

func validXYZ(z, x, y int) bool {
	if z < 0 || z > 20 || x < 0 || y < 0 {
		return false
	}
	n := 1 << uint(z)
	return x < n && y < n
}

// tilePath 返回瓦片文件路径（磁盘上是 y.png / y.jpg；不存在返回空串）
func tilePath(root, layer string, z, x, y int) string {
	m, err := filepath.Glob(filepath.Join(root, layer, strconv.Itoa(z), strconv.Itoa(x), strconv.Itoa(y)+".*"))
	if err != nil || len(m) == 0 {
		return ""
	}
	return m[0]
}

func tileExists(root, layer string, z, x, y int) bool {
	dir := filepath.Join(root, layer, strconv.Itoa(z), strconv.Itoa(x))
	m, _ := filepath.Glob(filepath.Join(dir, strconv.Itoa(y)+".*"))
	return len(m) > 0
}

func extForCT(ct string) string {
	ct = strings.ToLower(ct)
	switch {
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
		return "jpg"
	default:
		return "png"
	}
}

func saveTile(root, layer string, z, x, y int, ext string, b []byte) error {
	if ext == "" {
		ext = "png"
	}
	dir := filepath.Join(root, layer, strconv.Itoa(z), strconv.Itoa(x))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(dir, strconv.Itoa(y)+"."+ext+".tmp")
	final := filepath.Join(dir, strconv.Itoa(y)+"."+ext)
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

func fetchTileBytes(client *http.Client, url string) ([]byte, string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	// 注意：不能用浏览器 UA，否则天地图会把“服务端 Key”误判为浏览器访问而拒绝(301013)
	req.Header.Set("User-Agent", "familytree-offline-map/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
			if s := string(b); strings.Contains(s, "302010") || strings.Contains(s, "限流") {
				return nil, "", errTkLimited
			}
		}
		return nil, "", fmt.Errorf("http %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, "", err
	}
	if len(b) < 100 {
		return nil, "", fmt.Errorf("空响应")
	}
	ct = strings.ToLower(strings.TrimSpace(ct))
	if !strings.HasPrefix(ct, "image/") {
		return nil, "", fmt.Errorf("非图片响应(%s)", strings.TrimSpace(strings.SplitN(string(b), "\n", 2)[0]))
	}
	return b, ct, nil
}
