package handler

import (
	"archive/tar"
	"compress/gzip"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"familytree/internal/config"
	"familytree/internal/proc"

	"github.com/gin-gonic/gin"
)

// UpdateHandler 在线更新（页面上传更新包）
type UpdateHandler struct {
	DB  *sql.DB
	Cfg *config.Config
}

// currentVersion 从数据库读取当前版本号
func (h *UpdateHandler) currentVersion() string {
	var v string
	h.DB.QueryRow(`SELECT value FROM settings WHERE key = 'version'`).Scan(&v)
	if v == "" {
		return "1.3.9"
	}
	return v
}

// betaAuthorized 开源版：测试版通道不再需要授权。
func (h *UpdateHandler) betaAuthorized() bool { return true }

// parseVersionFromFile 从文件名解析版本号
// 支持正式版 x.y.z（如 0.5.13）与测试版 x.y.z.mmdd.n（如 0.5.13.0829.3），共 3~5 段
func parseVersionFromFile(name string) string {
	re := regexp.MustCompile(`v?(\d+(?:\.\d+){2,4})`)
	m := re.FindStringSubmatch(name)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

// versionCmp 比较版本号 a 与 b，a>b 返回 1，a==b 返回 0，a<b 返回 -1
// 支持变长段（正式版 x.y.z / 测试版 x.y.z.mmdd.n），缺失段按 0 处理
func versionCmp(a, b string) int {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		var na, nb int
		if i < len(pa) {
			na, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			nb, _ = strconv.Atoi(pb[i])
		}
		if na > nb {
			return 1
		}
		if na < nb {
			return -1
		}
	}
	return 0
}

// UploadUpdate 上传更新包并执行更新
// POST /api/update/upload（multipart/form-data，字段 file）
func (h *UpdateHandler) UploadUpdate(c *gin.Context) {
	if !h.Cfg.EnableUpdate {
		c.JSON(http.StatusBadRequest, gin.H{"error": "在线更新未启用"})
		return
	}

	// 接收上传文件
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未收到上传文件，请选择 .tar.gz 更新包"})
		return
	}

	// 校验文件名格式（用 filepath.Base 去掉可能的前导路径）
	name := filepath.Base(fileHeader.Filename)
	if !isTarGz(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "更新包必须是 .tar.gz 格式"})
		return
	}

	// 保存上传文件到 staging
	stagingRoot := filepath.Join(h.Cfg.AppDir, ".update")
	os.MkdirAll(stagingRoot, 0o755)
	dlPath := filepath.Join(stagingRoot, "upload.tar.gz")

	src, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取上传文件失败"})
		return
	}
	defer src.Close()

	dst, err := os.Create(dlPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存上传文件失败"})
		return
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存上传文件失败"})
		return
	}
	dst.Close()

	needRestart, message, err := h.deployUpdate(name, dlPath, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !needRestart {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": message})
		return
	}
	// 异步自重启（延迟 1 秒，给响应留时间）
	go func() {
		time.Sleep(1 * time.Second)
		h.restartSelf()
	}()
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": message})
}

// ListServerPackages 列出更新服务器上的更新包（从 packages.json 清单获取）
// GET /api/update/server-packages?mode=beta|version
// 从更新源（内置 + 数据库源）的 packages.json 拉取，按 channel 过滤（beta/version）
func (h *UpdateHandler) ListServerPackages(c *gin.Context) {
	mode := c.Query("mode")
	client := h.manifestHTTPClient()

	type pkg struct {
		Name    string `json:"name"`
		Size    int64  `json:"size"`
		Version string `json:"version"`
		URL     string `json:"url"`
		SHA256  string `json:"sha256"`
		Channel string `json:"channel"`
		Notes   string `json:"notes"`
	}
	list := []pkg{}

	// 逐个更新源拉 packages.json，合并结果（去重按 name）
	seen := map[string]bool{}
	for _, s := range h.getUpdateSources() {
		pu := packagesJSONURL(s)
		if pu == "" {
			continue
		}
		m, err := fetchPackagesManifest(client, pu)
		if err != nil {
			continue
		}
		for _, e := range m.Packages {
			if e.Name == "" || seen[e.Name] {
				continue
			}
			// 平台过滤：指定了 platform 的包只匹配当前平台（Windows 版不显示 Linux 包，反之亦然；支持 GOOS-GOARCH 区分 x86/arm）
			if e.Platform != "" && !platformMatch(e.Platform) {
				continue
			}
			// 按 mode 过滤 channel：只接受明确勾选的类型，mode 为空（未勾选）时不返回任何包，避免混乱
			switch mode {
			case "beta":
				if e.Channel != "beta" {
					continue
				}
			case "version":
				if e.Channel != "version" && e.Channel != "stable" {
					continue
				}
				// 旧版本只展示正式版（3 段版本号），排除历史测试版（5 段 mmdd.n）
				if strings.Count(e.Version, ".") != 2 {
					continue
				}
			default:
				continue
			}
			seen[e.Name] = true
			list = append(list, pkg{
				Name:    e.Name,
				Size:    e.Size,
				Version: e.Version,
				URL:     e.URL,
				SHA256:  e.SHA256,
				Channel: e.Channel,
				Notes:   e.Notes,
			})
		}
	}

	// 按版本号降序排列
	sort.Slice(list, func(i, j int) bool { return versionCmp(list[i].Version, list[j].Version) > 0 })
	c.JSON(http.StatusOK, gin.H{"packages": list, "mode": mode, "empty": len(list) == 0, "beta_authorized": h.betaAuthorized()})
}

// platformMatch 判断包声明的 platform 是否匹配当前运行平台。
// 支持两种格式：
//   - 旧格式（GOOS）：linux / windows，匹配同 GOOS 的所有架构（向后兼容旧 packages.json）
//   - 新格式（GOOS-GOARCH）：linux-amd64 / linux-arm64 / windows-amd64，精确匹配架构
//
// 空 platform 视为通用包（不过滤）。
func platformMatch(platform string) bool {
	if strings.Contains(platform, "-") {
		return platform == runtime.GOOS+"-"+runtime.GOARCH
	}
	// 旧格式纯 GOOS（linux/windows）视为历史 x86/amd64 包，仅 amd64 架构匹配
	return platform == runtime.GOOS && runtime.GOARCH == "amd64"
}

// UpdateFromServer 从更新服务器下载更新包并执行更新
// POST /api/update/from-server  body: {"file": "族谱系统vX.update.tar.gz", "url": "beta/xxx.tar.gz", "sha256": "..."}
// url 为相对 packages.json 所在目录的路径（或绝对 URL）；未传 url 时按文件名校验并向后兼容
func (h *UpdateHandler) UpdateFromServer(c *gin.Context) {
	if !h.Cfg.EnableUpdate {
		c.JSON(http.StatusBadRequest, gin.H{"error": "在线更新未启用"})
		return
	}
	var req struct {
		File    string `json:"file"`
		URL     string `json:"url"`
		SHA256  string `json:"sha256"`
		Channel string `json:"channel"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.File == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未指定更新包文件名"})
		return
	}

	name := filepath.Base(filepath.FromSlash(req.File))
	if !isTarGz(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "更新包必须是 .tar.gz 格式"})
		return
	}

	stagingRoot := filepath.Join(h.Cfg.AppDir, ".update")
	os.MkdirAll(stagingRoot, 0o755)
	dlPath := filepath.Join(stagingRoot, "upload.tar.gz")

	// 后台执行更新，前端轮询进度（下载 % → 解压中 → 重启中）
	task := newUpdateTask()
	go func() {
		if req.URL != "" {
			// 相对路径 → 基于第一个可用更新源解析；绝对 URL 直接用
			dlURL := req.URL
			if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
				sources := h.getUpdateSources()
				if len(sources) == 0 {
					setUpdateError(task, "未配置更新源，无法解析下载地址")
					return
				}
				dlURL = resolveManifestURL(sources[0], req.URL)
			}
			client := h.manifestHTTPClient()
			if err := downloadFile(client, dlURL, dlPath, func(p int) {
				setUpdateProgress(task, stageDownloading, p, fmt.Sprintf("正在下载 %d%%", p))
			}); err != nil {
				setUpdateError(task, "下载更新包失败："+err.Error())
				return
			}
			if req.SHA256 != "" {
				if err := verifySHA256(dlPath, req.SHA256); err != nil {
					os.Remove(dlPath)
					setUpdateError(task, "更新包校验失败（sha256 不匹配，已丢弃）："+err.Error())
					return
				}
			}
		} else {
			// 向后兼容：无 url 时尝试从本地挂载目录读取（旧行为）
			rel := filepath.Clean(filepath.FromSlash(req.File))
			if rel == ".." || strings.HasPrefix(rel, "../") || filepath.IsAbs(rel) || strings.HasPrefix(rel, "..") {
				setUpdateError(task, "非法的文件路径")
				return
			}
			srcPath := filepath.Join("/data/updates", rel)
			if _, err := os.Stat(srcPath); err != nil {
				setUpdateError(task, "未找到该更新包，请重新刷新列表")
				return
			}
			if err := copyFile(srcPath, dlPath); err != nil {
				setUpdateError(task, "复制更新包失败: "+err.Error())
				return
			}
		}

		needRestart, msg, err := h.deployUpdate(name, dlPath, func(stage string, p int, m string) {
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

// isTarGz 判断文件是否为 tar.gz/tgz
func isTarGz(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz")
}

// applyUpdate 执行部署：校验版本 → 解压 → 备份旧版 → 部署 → 更新版本号 → 自重启
// deployUpdate 执行部署：校验版本 → 解压 → 备份旧版 → 部署 → 更新版本号。
// 返回 needRestart（是否需重启）、message（提示文案）、err（失败原因）。
// onStage 可选，用于上报进度阶段（解压中等）。
func (h *UpdateHandler) deployUpdate(name, dlPath string, onStage func(stage string, percent int, message string)) (needRestart bool, message string, err error) {
	// 从文件名解析版本号
	newVer := parseVersionFromFile(name)
	if newVer == "" {
		return false, "", fmt.Errorf("文件名中未找到版本号")
	}

	cur := h.currentVersion()
	if versionCmp(newVer, cur) == 0 {
		return false, "更新包版本 v" + newVer + " 与当前版本相同", nil
	}

	stagingRoot := filepath.Join(h.Cfg.AppDir, ".update")

	// 解压到临时目录，校验内容
	tmpDir := filepath.Join(stagingRoot, "staging")
	os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir, 0o755)
	if onStage != nil {
		onStage(stageExtracting, 100, "正在解压更新包...")
	}
	if err := extractTarGz(dlPath, tmpDir); err != nil {
		return false, "", fmt.Errorf("解压失败: %v", err)
	}

	// 定位二进制和 web 目录
	binPath := findBinary(tmpDir)
	if binPath == "" {
		return false, "", fmt.Errorf("更新包中未找到 familytree 二进制")
	}
	webSrc := findWebDir(tmpDir)
	if webSrc == "" {
		return false, "", fmt.Errorf("更新包中未找到 web 目录（需含 index.html）")
	}

	// 备份旧版本（二进制 + web）
	// 旧版可能来自数据卷 /data/runtime（已在线更新过），或镜像内置 /app（首次在线更新前）
	backupDir := filepath.Join(stagingRoot, "backup_"+strings.ReplaceAll(cur, ".", "_"))
	os.RemoveAll(backupDir)
	os.MkdirAll(backupDir, 0o755)

	// 旧二进制：优先数据卷/runtime，回退容器内 /app（镜像内置版），再回退当前运行 exe（Windows 独立版）
	oldBinCandidates := []string{
		filepath.Join(h.Cfg.AppDir, binFileName()),
		"/app/" + binFileName(),
	}
	if exe, err := os.Executable(); err == nil {
		oldBinCandidates = append(oldBinCandidates, exe)
	}
	binBackedUp := false
	for _, src := range oldBinCandidates {
		if fi, err := os.Stat(src); err == nil && !fi.IsDir() {
			if copyFile(src, filepath.Join(backupDir, binFileName())) == nil {
				binBackedUp = true
				break
			}
		}
	}

	// 旧前端：优先数据卷，回退容器内 /app/web
	oldWebCandidates := []string{
		h.Cfg.WebDir,
		"/app/web",
	}
	webBackedUp := false
	for _, src := range oldWebCandidates {
		if fi, err := os.Stat(src); err == nil && fi.IsDir() {
			if copyDir(src, filepath.Join(backupDir, "web")) == nil {
				webBackedUp = true
				break
			}
		}
	}
	if !binBackedUp || !webBackedUp {
		return false, "", fmt.Errorf("备份旧版本失败，已中止更新")
	}

	// 清理多余的旧版本回滚备份，只保留最近 3 份
	pruneOldBackups(stagingRoot, 3)

	// 部署目标：数据卷中的二进制与前端
	oldBin := filepath.Join(h.Cfg.AppDir, binFileName())
	oldWeb := h.Cfg.WebDir

	// 部署：覆盖二进制 + web
	if err := copyFile(binPath, oldBin); err != nil {
		return false, "", fmt.Errorf("替换二进制失败: %v", err)
	}
	os.Chmod(oldBin, 0o755)
	os.RemoveAll(oldWeb)
	if err := copyDir(webSrc, oldWeb); err != nil {
		return false, "", fmt.Errorf("替换前端失败: %v", err)
	}

	// 更新版本号到数据库
	h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('version', ?)`, newVer)

	// 清理本次解压临时目录与 .update 下残留的历史更新包，释放空间
	os.RemoveAll(tmpDir)
	cleanStaleUpdatePackages(stagingRoot)

	return true, "更新完成，服务即将重启", nil
}

// Progress 查询更新任务进度
// GET /api/update/progress?task=<id>
func (h *UpdateHandler) Progress(c *gin.Context) {
	task := c.Query("task")
	if task == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 task 参数"})
		return
	}
	p := getUpdateProgress(task)
	if p == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "更新任务不存在或已结束"})
		return
	}
	c.JSON(http.StatusOK, p)
}

// Rollback 回滚到上一个版本
// POST /api/update/rollback
func (h *UpdateHandler) Rollback(c *gin.Context) {
	base := filepath.Join(h.Cfg.AppDir, ".update")
	entries, err := os.ReadDir(base)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "没有可回滚的备份"})
		return
	}
	var backups []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "backup_") {
			backups = append(backups, e.Name())
		}
	}
	if len(backups) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "没有可回滚的备份"})
		return
	}

	// 跳过无效备份（空目录缺二进制/前端），选最近的有效备份
	sort.Strings(backups)
	var last, ver string
	for i := len(backups) - 1; i >= 0; i-- {
		name := backups[i]
		bdir := filepath.Join(base, name)
		if fi, err := os.Stat(filepath.Join(bdir, binFileName())); err != nil || fi.IsDir() {
			continue // 缺二进制，无效备份
		}
		if _, err := os.Stat(filepath.Join(bdir, "web", "index.html")); err != nil {
			continue // 缺前端，无效备份
		}
		last = name
		break
	}
	if last == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "没有有效的回滚备份（备份数据不完整）"})
		return
	}
	backupDir := filepath.Join(base, last)
	ver = strings.TrimPrefix(last, "backup_")
	ver = strings.ReplaceAll(ver, "_", ".")

	// 回滚前校验备份完整性，避免空备份导致假成功
	oldBin := filepath.Join(backupDir, binFileName())
	oldWeb := filepath.Join(backupDir, "web")
	if fi, err := os.Stat(oldBin); err != nil || fi.IsDir() {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "备份数据不完整（缺二进制），无法回滚"})
		return
	}
	if _, err := os.Stat(filepath.Join(oldWeb, "index.html")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "备份数据不完整（缺前端），无法回滚"})
		return
	}

	// 恢复二进制和 web
	if err := copyFile(oldBin, filepath.Join(h.Cfg.AppDir, binFileName())); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复二进制失败: " + err.Error()})
		return
	}
	os.Chmod(filepath.Join(h.Cfg.AppDir, binFileName()), 0o755)
	if _, err := os.Stat(oldWeb); err == nil {
		os.RemoveAll(h.Cfg.WebDir)
		if err := copyDir(oldWeb, h.Cfg.WebDir); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复前端失败: " + err.Error()})
			return
		}
	}
	h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('version', ?)`, ver)

	go func() {
		time.Sleep(1 * time.Second)
		h.restartSelf()
	}()

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "已回滚到 " + ver + "，服务即将重启"})
}

// binFileName 返回二进制文件名（Windows 带 .exe 扩展名，Linux 无扩展名）
func binFileName() string {
	if runtime.GOOS == "windows" {
		return "familytree.exe"
	}
	return "familytree"
}

// restartSelf 自重启：
// - Linux/Docker：退出当前进程，由 Docker restart=unless-stopped 自动拉起新二进制
// - Windows：启动 runtime 新二进制作为替换器（--self-replace），替换主 exe 后自动重启
func (h *UpdateHandler) restartSelf() {
	if runtime.GOOS == "windows" {
		newBin := filepath.Join(h.Cfg.AppDir, binFileName())
		mainExe, err := os.Executable()
		if err != nil {
			log.Printf("[更新] 获取主程序路径失败: %v", err)
			os.Exit(1)
			return
		}
		// 替换器必须**脱离当前控制台**运行：否则用户按「Press any key」关掉旧窗口时，
		// 替换器/新服务会被系统一起杀掉（旧版就是这个原因导致「更新后没有再启动」）。
		logPath := filepath.Join(filepath.Dir(mainExe), "update-restart.log")
		args := []string{"--self-replace", mainExe}
		if lp := proc.LauncherPID(); lp > 0 {
			args = append(args, strconv.Itoa(lp)) // 让替换器把承载本服务的旧窗口一起关掉
		}
		if err := proc.StartDetached(newBin, args, filepath.Dir(newBin), logPath); err != nil {
			log.Printf("[更新] 启动替换器失败: %v", err)
		}
		os.Exit(0)
		return
	}
	// Linux/Docker：familytree 是容器 PID 1，退出后 Docker 自动重启容器
	os.Exit(0)
}

// extractTarGz 解压 tar.gz 到目标目录
func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// 防止路径穿越
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			continue
		}
		target := filepath.Join(dest, clean)
		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0o755)
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
			os.Chmod(target, os.FileMode(hdr.Mode))
		}
	}
	return nil
}

// findBinary 在解压目录中查找 familytree 二进制
func findBinary(dir string) string {
	var found string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if found != "" {
			return filepath.SkipAll
		}
		if !info.IsDir() && (info.Name() == "familytree" || info.Name() == "familytree.exe") {
			// Windows 无 Unix 执行权限位（mode&0111 恒为 0），直接按文件名匹配
			if runtime.GOOS == "windows" || info.Mode()&0111 != 0 {
				found = path
			}
		}
		return nil
	})
	return found
}

// findWebDir 在解压目录中查找 web 目录
func findWebDir(dir string) string {
	var found string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if found != "" {
			return filepath.SkipAll
		}
		if info.IsDir() && info.Name() == "web" {
			// 确认包含 index.html
			if _, err := os.Stat(filepath.Join(path, "index.html")); err == nil {
				found = path
			}
		}
		return nil
	})
	return found
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	os.MkdirAll(filepath.Dir(dst), 0o755)
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// copyDir 递归复制目录
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

// pruneOldBackups 清理多余的旧版本回滚备份，按修改时间只保留最近 keep 份
// 回滚只用最新一份有效备份，更早的备份纯属占用空间
func pruneOldBackups(stagingRoot string, keep int) {
	entries, err := os.ReadDir(stagingRoot)
	if err != nil {
		return
	}
	type bdir struct {
		name    string
		modTime time.Time
	}
	var backups []bdir
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "backup_") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		backups = append(backups, bdir{name: e.Name(), modTime: info.ModTime()})
	}
	if len(backups) <= keep {
		return
	}
	// 按修改时间降序（最新的在前）
	sort.Slice(backups, func(i, j int) bool { return backups[i].modTime.After(backups[j].modTime) })
	for i := keep; i < len(backups); i++ {
		p := filepath.Join(stagingRoot, backups[i].name)
		if err := os.RemoveAll(p); err == nil {
			log.Printf("[更新] 清理旧回滚备份: %s", backups[i].name)
		}
	}
}

// cleanStaleUpdatePackages 清理 .update 目录下残留的历史更新包（*.tar.gz），释放空间
func cleanStaleUpdatePackages(stagingRoot string) {
	entries, err := os.ReadDir(stagingRoot)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".tar.gz") || strings.HasSuffix(e.Name(), ".tgz") {
			p := filepath.Join(stagingRoot, e.Name())
			if err := os.Remove(p); err == nil {
				log.Printf("[更新] 清理残留更新包: %s", e.Name())
			}
		}
	}
}

// PruneUpdateArtifacts 启动时清理更新残留：回滚备份只保留最近 keep 份、删除残留更新包
// 在 main 启动时调用，确保历史遗留的冗余备份在升级后第一次启动即被清掉，无需等到下一次更新
func PruneUpdateArtifacts(appDir string, keep int) {
	stagingRoot := filepath.Join(appDir, ".update")
	pruneOldBackups(stagingRoot, keep)
	cleanStaleUpdatePackages(stagingRoot)
}
