package handler

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 在线更新（清单式）：从多个更新源（FAMILY_TREE_UPDATE_URL 内置源 + 数据库可配置源）读取 latest.json，
// 按顺序逐个拉取，取版本号最高者作为最新版本；确认后从该源下载更新包 → 校验 sha256 → 部署 → 重启。
// 取代早期逆向飞牛分享 API 的方案（fshare.go 已移除）。

const manifestTimeout = 30 * time.Second

// manifestHTTPClient 构建更新源 HTTP 客户端（支持跳过 TLS 证书校验，适配自签名证书）
func (h *UpdateHandler) manifestHTTPClient() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: h.Cfg.UpdateInsecure}, // #nosec G402 -- 由 FAMILY_TREE_UPDATE_INSECURE 显式开启
	}
	return &http.Client{Timeout: manifestTimeout, Transport: tr}
}

// updateManifest latest.json 清单结构
type updateManifest struct {
	Version    string `json:"version"`    // 版本号（x.y.z 或 x.y.z.mmdd.n）
	URL        string `json:"url"`        // 更新包地址（绝对 URL 或相对 latest.json 的相对路径）
	Size       int64  `json:"size"`       // 字节数（可选，用于提示）
	SHA256     string `json:"sha256"`     // 更新包 sha256（可选，为空则跳过校验）
	Notes      string `json:"notes"`      // 更新说明
	MinVersion string `json:"minVersion"` // 最低可升级版本（可选）
	ReleaseAt  string `json:"releaseAt"`  // 发布日期（可选）
}

// sourceScanResult 单个更新源的扫描结果
type sourceScanResult struct {
	Source  string `json:"source"`
	Ok      bool   `json:"ok"`
	Version string `json:"version"`
	Notes   string `json:"notes"`
	Error   string `json:"error"`
}

// packageEntry packages.json 中的单个更新包
type packageEntry struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Version  string `json:"version"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
	Channel  string `json:"channel"` // beta | version
	Notes    string `json:"notes"`
	Platform string `json:"platform"` // linux | windows（空=通用/旧包，不按平台过滤）
}

// packagesManifest packages.json 清单结构
type packagesManifest struct {
	Packages []packageEntry `json:"packages"`
}

// packagesJSONURL 从 latest.json 源地址推导 packages.json 地址
func packagesJSONURL(sourceURL string) string {
	u, err := url.Parse(sourceURL)
	if err != nil {
		return ""
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/packages.json"
	} else {
		dir := path.Dir(u.Path)
		u.Path = dir + "/packages.json"
	}
	return u.String()
}

// fetchPackagesManifest 拉取并解析 packages.json
func fetchPackagesManifest(client *http.Client, rawURL string) (*packagesManifest, error) {
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // 限制 4MB
	if err != nil {
		return nil, err
	}
	var m packagesManifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("清单格式错误：%v", err)
	}
	return &m, nil
}

// getUpdateSources 合并数据库可配置源 + 内置环境变量源（去重、去空白、保序）
func (h *UpdateHandler) getUpdateSources() []string {
	var list []string
	var raw string
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'update_sources'`).Scan(&raw)
	if raw != "" {
		var arr []string
		if json.Unmarshal([]byte(raw), &arr) == nil {
			list = append(list, arr...)
		}
	}
	if h.Cfg.UpdateURL != "" {
		list = append(list, h.Cfg.UpdateURL)
	}
	seen := map[string]bool{}
	out := []string{}
	for _, s := range list {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if u, err := url.Parse(s); err == nil && (u.Path == "" || u.Path == "/") {
			s = strings.TrimRight(s, "/") + "/latest.json"
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// saveUpdateSources 保存数据库可配置源（清理空白/去重），返回清理后的列表
func (h *UpdateHandler) saveUpdateSources(sources []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range sources {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	b, _ := json.Marshal(out)
	h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('update_sources', ?)`, string(b))
	return out
}

// ListUpdateSources 获取更新源配置
// GET /api/update/sources
func (h *UpdateHandler) ListUpdateSources(c *gin.Context) {
	var dbSources []string
	var raw string
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'update_sources'`).Scan(&raw)
	if raw != "" {
		json.Unmarshal([]byte(raw), &dbSources)
	}
	if dbSources == nil {
		dbSources = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"sources": dbSources, "builtin": h.Cfg.UpdateURL})
}

// SaveUpdateSources 保存更新源配置
// POST /api/update/sources   body: {"sources": ["url1", "url2"]}
func (h *UpdateHandler) SaveUpdateSources(c *gin.Context) {
	var body struct {
		Sources []string `json:"sources"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cleaned := h.saveUpdateSources(body.Sources)
	c.JSON(http.StatusOK, gin.H{"ok": true, "sources": cleaned})
}

// explainUpdateError 把 TLS/证书类错误翻成可读原因
func explainUpdateError(err error) string {
	if err == nil {
		return ""
	}
	e := strings.ToLower(err.Error())
	if strings.Contains(e, "certificate") || strings.Contains(e, "x509") || strings.Contains(e, "tls") {
		return "更新源证书无法验证（自签名）：" + err.Error()
	}
	return err.Error()
}

// scanSources 按顺序扫描所有更新源，返回版本最高的源与其清单、以及各源结果
func (h *UpdateHandler) scanSources() (best *updateManifest, bestSource string, results []sourceScanResult) {
	client := h.manifestHTTPClient()
	results = []sourceScanResult{}
	for _, s := range h.getUpdateSources() {
		r := sourceScanResult{Source: s}
		m, err := fetchManifest(client, s)
		if err != nil {
			r.Error = explainUpdateError(err)
			results = append(results, r)
			continue
		}
		if m.Version == "" {
			r.Error = "清单缺少版本号"
			results = append(results, r)
			continue
		}
		r.Ok = true
		r.Version = m.Version
		r.Notes = m.Notes
		results = append(results, r)
		if best == nil || versionCmp(m.Version, best.Version) > 0 {
			best = m
			bestSource = s
		}
	}
	return
}

// bestPackageForPlatform 从各更新源的 packages.json 中，按当前平台过滤后选出版本最高的包。
// latest.json 只有单一 url（通常指向 Linux 包）且无平台字段，Windows/ARM 客户端直接按 latest.json
// 下载会下错平台；这里改为按 packages.json 的 platform 字段（复用 update.go 的 platformMatch）
// 做 GOOS+GOARCH 分流，确保各平台客户端都下载到本平台的更新包。
func (h *UpdateHandler) bestPackageForPlatform() (best *packageEntry, bestSource string) {
	client := h.manifestHTTPClient()
	for _, s := range h.getUpdateSources() {
		pu := packagesJSONURL(s)
		if pu == "" {
			continue
		}
		m, err := fetchPackagesManifest(client, pu)
		if err != nil {
			continue
		}
		for i := range m.Packages {
			e := &m.Packages[i]
			if e.Version == "" || e.URL == "" {
				continue
			}
			// 平台过滤：指定了 platform 的包只匹配当前平台；空 platform 视为通用包（不过滤）
			if e.Platform != "" && !platformMatch(e.Platform) {
				continue
			}
			// 通道过滤：一键更新只推正式版（stable/version）；测试版（beta）由「测试版列表」手动选择下载
			if e.Channel == "beta" {
				continue
			}
			if best == nil || versionCmp(e.Version, best.Version) > 0 {
				best = e
				bestSource = s
			}
		}
	}
	return
}

// CheckManifestUpdate 检查各更新源中的最新版本
// GET /api/update/check
func (h *UpdateHandler) CheckManifestUpdate(c *gin.Context) {
	cur := h.currentVersion()

	sources := h.getUpdateSources()
	if len(sources) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"ok": false, "checked": true, "current": cur, "sources": []sourceScanResult{},
			"error": "未配置更新源，请先点击「更新地址」添加更新源地址",
		})
		return
	}

	// 各源 latest.json 状态（用于前端展示「各更新源检测结果」）
	_, _, results := h.scanSources()

	// 按当前平台从 packages.json 选版本最高的包（避免 latest.json 单一 url 下错平台）
	best, bestSource := h.bestPackageForPlatform()
	if best == nil {
		msg := "所有更新源均无法获取（请检查更新地址与网络）"
		for _, r := range results {
			if strings.Contains(r.Error, "证书") {
				msg = r.Error
				break
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"ok": false, "checked": true, "current": cur, "sources": results,
			"error": msg,
		})
		return
	}

	cmp := versionCmp(best.Version, cur)
	hasUpdate := cmp > 0    // 有更高版本才提示「有新版本」
	canDowngrade := cmp < 0 // 服务器版本低于当前（可回退）

	c.JSON(http.StatusOK, gin.H{
		"ok":           true,
		"checked":      true,
		"current":      cur,
		"latest":       best.Version,
		"latestURL":    best.URL,
		"latestSize":   best.Size,
		"notes":        best.Notes,
		"hasUpdate":    hasUpdate,
		"canDowngrade": canDowngrade,
		"minVersion":   "",
		"releaseAt":    "",
		"source":       bestSource,
		"sources":      results,
		"updateURL":    bestSource,
	})
}

// UpdateFromManifest 从版本最高的更新源下载更新包并执行更新
// POST /api/update/from-manifest
func (h *UpdateHandler) UpdateFromManifest(c *gin.Context) {
	if !h.Cfg.EnableUpdate {
		c.JSON(http.StatusBadRequest, gin.H{"error": "在线更新未启用"})
		return
	}

	best, bestSource := h.bestPackageForPlatform()
	if best == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "所有更新源均无法获取，无法执行更新"})
		return
	}
	if best.Version == "" || best.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "更新清单不完整（缺 version 或 url）"})
		return
	}

	cur := h.currentVersion()
	if versionCmp(best.Version, cur) == 0 {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "更新包版本 v" + best.Version + " 与当前版本相同"})
		return
	}

	// 解析下载地址（支持相对 latest.json 的相对路径）
	dlURL := resolveManifestURL(bestSource, best.URL)

	// 下载到 staging
	stagingRoot := filepath.Join(h.Cfg.AppDir, ".update")
	os.MkdirAll(stagingRoot, 0o755)
	dlPath := filepath.Join(stagingRoot, "manifest_download.tar.gz")

	// 部署时用包文件名携带版本号（deployUpdate 从文件名解析版本；无名字时回退到按版本生成）
	fileName := best.Name
	if fileName == "" {
		fileName = manifestFileName(best.Version)
	}
	renamePath := filepath.Join(stagingRoot, fileName)
	sha := best.SHA256

	// 后台执行更新，前端轮询进度（下载 % → 解压中 → 重启中）
	task := newUpdateTask()
	go func() {
		client := h.manifestHTTPClient()
		// 下载（带进度）
		if err := downloadFile(client, dlURL, dlPath, func(p int) {
			setUpdateProgress(task, stageDownloading, p, fmt.Sprintf("正在下载 %d%%", p))
		}); err != nil {
			setUpdateError(task, "下载更新包失败："+err.Error())
			return
		}
		// 校验 sha256
		if sha != "" {
			if err := verifySHA256(dlPath, sha); err != nil {
				os.Remove(dlPath)
				setUpdateError(task, "更新包校验失败（sha256 不匹配，已丢弃）："+err.Error())
				return
			}
		}
		os.Remove(renamePath)
		if err := os.Rename(dlPath, renamePath); err != nil {
			renamePath = dlPath
		}
		// 解压 + 部署（带进度）
		needRestart, msg, err := h.deployUpdate(fileName, renamePath, func(stage string, p int, m string) {
			setUpdateProgress(task, stage, p, m)
		})
		if err != nil {
			setUpdateError(task, err.Error())
			return
		}
		if !needRestart {
			setUpdateProgress(task, stageDone, 100, msg)
			return
		}
		setUpdateProgress(task, stageRestarting, 100, "更新完成，正在重启...")
		time.Sleep(1 * time.Second)
		h.restartSelf()
	}()

	c.JSON(http.StatusOK, gin.H{"ok": true, "task": task, "message": "更新已开始"})
}

// fetchManifest 拉取并解析 latest.json
func fetchManifest(client *http.Client, rawURL string) (*updateManifest, error) {
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 限制 1MB
	if err != nil {
		return nil, err
	}
	var m updateManifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("清单格式错误：%v", err)
	}
	return &m, nil
}

// resolveManifestURL 把清单里的 url 解析成绝对地址
func resolveManifestURL(manifestURL, fileURL string) string {
	u, err := url.Parse(fileURL)
	if err == nil && u.IsAbs() {
		return fileURL
	}
	base, err := url.Parse(manifestURL)
	if err != nil {
		return fileURL
	}
	dir := path.Dir(base.Path)
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	base.Path = dir
	return base.ResolveReference(u).String()
}

// downloadFile 下载文件到本地（onProgress 可选，回调下载百分比 0-100）
func downloadFile(client *http.Client, rawURL, dest string, onProgress func(percent int)) error {
	resp, err := client.Get(rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	total := resp.ContentLength
	if total <= 0 {
		// 未知总大小：一次性拷贝，完成后报 100
		if _, err = io.Copy(out, resp.Body); err != nil {
			return err
		}
		if onProgress != nil {
			onProgress(100)
		}
		return nil
	}

	var written int64
	buf := make([]byte, 64*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			if onProgress != nil {
				p := int(written * 100 / total)
				if p > 100 {
					p = 100
				}
				onProgress(p)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	return nil
}

// verifySHA256 校验文件 sha256
func verifySHA256(file, expect string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	exp := strings.ToLower(strings.TrimSpace(expect))
	if got != exp {
		return fmt.Errorf("期望 %s，实际 %s", exp, got)
	}
	return nil
}

// manifestFileName 生成带版本号的文件名，供 applyUpdate 解析版本
func manifestFileName(ver string) string {
	return "族谱系统v" + ver + ".update.tar.gz"
}
