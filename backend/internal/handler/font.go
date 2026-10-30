package handler

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"familytree/internal/config"

	"github.com/gin-gonic/gin"
)

// FontHandler 谱书自定义字体：上传到系统数据目录，换浏览器/设备登录都能用
type FontHandler struct {
	Cfg *config.Config
}

type fontInfo struct {
	Name      string `json:"name"`       // 文件名（含扩展名，作为唯一标识）
	Label     string `json:"label"`      // 显示名（去扩展名）
	Family    string `json:"family"`     // CSS font-family 名（ZupuFont_+安全名）
	URL       string `json:"url"`        // 字体文件访问地址
	Size      int64  `json:"size"`       // 字节
	CreatedAt int64  `json:"created_at"` // Unix
}

var fontExtRe = regexp.MustCompile(`\.(ttf|otf|woff2?|ttc)$`)

func (h *FontHandler) fontDir() string {
	return filepath.Join(h.Cfg.DataDir, "fonts")
}

// ===== 字体内容 sha256 缓存（上传去重用） =====
// 服务器上可能有 200MB+ 字体，不能每次上传都全量重算哈希：
// 缓存键 = 文件全路径，值 = {size, mtime, sha256}，只在文件新增/变更时重算；
// 旁挂 fonts/.sha256.json 持久化，进程重启后也能直接用（List 会按扩展名过滤掉它）。
type fontHashEntry struct {
	Size    int64  `json:"size"`
	ModTime int64  `json:"mtime"`
	Sum     string `json:"sha256"`
}

var (
	fontHashMu     sync.Mutex
	fontHashCache  = map[string]fontHashEntry{}
	fontHashLoaded bool
)

func (h *FontHandler) hashCachePath() string { return filepath.Join(h.fontDir(), ".sha256.json") }

func (h *FontHandler) loadHashCache() {
	if fontHashLoaded {
		return
	}
	fontHashLoaded = true
	b, err := os.ReadFile(h.hashCachePath())
	if err != nil {
		return
	}
	m := map[string]fontHashEntry{}
	if json.Unmarshal(b, &m) == nil && m != nil {
		fontHashCache = m
	}
}

func (h *FontHandler) saveHashCache() {
	b, err := json.Marshal(fontHashCache)
	if err != nil {
		return
	}
	_ = os.WriteFile(h.hashCachePath(), b, 0o644)
}

func (h *FontHandler) setHash(path string, fi os.FileInfo, sum string) {
	fontHashMu.Lock()
	fontHashCache[path] = fontHashEntry{Size: fi.Size(), ModTime: fi.ModTime().UnixNano(), Sum: sum}
	fontHashMu.Unlock()
	h.saveHashCache()
}

// 文件 sha256（带缓存：size+mtime 未变就直接返回）
func (h *FontHandler) fileSum(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	fontHashMu.Lock()
	e, ok := fontHashCache[path]
	fontHashMu.Unlock()
	if ok && e.Size == fi.Size() && e.ModTime == fi.ModTime().UnixNano() && e.Sum != "" {
		return e.Sum, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hsh := sha256.New()
	if _, err := io.Copy(hsh, f); err != nil {
		return "", err
	}
	sum := hex.EncodeToString(hsh.Sum(nil))
	h.setHash(path, fi, sum)
	return sum, nil
}

// 在字体目录里找内容与 sum 相同的字体文件（返回文件名）；skipPath 用于排除临时文件
func (h *FontHandler) findBySum(dir, sum, skipPath string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !fontExtRe.MatchString(strings.ToLower(name)) {
			continue
		}
		p := filepath.Join(dir, name)
		if p == skipPath {
			continue
		}
		s, err := h.fileSum(p)
		if err != nil {
			continue
		}
		if s == sum {
			return name, true
		}
	}
	return "", false
}

// fontMagicOK 校验文件头是否为合法字体魔数（ttf/otf/ttc/woff/woff2）。
// 防止把误下载的网页 HTML（非字体）当字体上传，导致前端 FontFace 加载失败。
func fontMagicOK(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	hdr := make([]byte, 4)
	if n, _ := io.ReadFull(f, hdr); n < 4 {
		return false
	}
	switch string(hdr) {
	case "\x00\x01\x00\x00", "true", "typ1", "OTTO", "ttcf", "wOFF", "wOF2":
		return true
	}
	return false
}

// familyName 生成稳定且唯一的 ASCII 字体名：
// 中文文件名直接进 CSS font-family 会全变下划线且互相冲突，
// 改用文件名 md5 前 12 位，保证 ASCII、唯一、稳定。
func familyName(fname string) string {
	sum := md5.Sum([]byte(fname))
	return "ZupuFont_" + hex.EncodeToString(sum[:])[:12]
}

func (h *FontHandler) toInfo(fname string, fi os.FileInfo) fontInfo {
	ext := filepath.Ext(fname)
	label := strings.TrimSuffix(fname, ext)
	return fontInfo{
		Name:      fname,
		Label:     label,
		Family:    familyName(fname),
		URL:       "/fonts/" + fname,
		Size:      fi.Size(),
		CreatedAt: fi.ModTime().Unix(),
	}
}

// List GET /api/fonts 列出系统里已上传的字体
func (h *FontHandler) List(c *gin.Context) {
	dir := h.fontDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, []fontInfo{})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	list := []fontInfo{}
	for _, e := range entries {
		if e.IsDir() || !fontExtRe.MatchString(strings.ToLower(e.Name())) {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		// 跳过非字体文件（如误下载的 HTML），避免前端 FontFace 加载失败
		if !fontMagicOK(filepath.Join(dir, e.Name())) {
			continue
		}
		list = append(list, h.toInfo(e.Name(), fi))
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt < list[j].CreatedAt })
	c.JSON(http.StatusOK, list)
}

// Upload POST /api/fonts 上传字体（multipart: file）
func (h *FontHandler) Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择字体文件"})
		return
	}
	defer file.Close()

	fname := filepath.Base(header.Filename)
	if !fontExtRe.MatchString(strings.ToLower(fname)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 .ttf/.otf/.woff/.woff2/.ttc 字体文件"})
		return
	}
	// 限制 30MB
	if header.Size > 30*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "字体文件过大（上限 30MB）"})
		return
	}

	dir := h.fontDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目录失败"})
		return
	}
	h.loadHashCache()

	// 先写临时文件，边写边算 sha256（避免为了取哈希再全量读一遍）
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}
	tmpPath := tmp.Name()
	hsh := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hsh), file); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入文件失败"})
		return
	}
	tmp.Close()
	sum := hex.EncodeToString(hsh.Sum(nil))

	// 校验是否为真实字体（非 HTML 网页等），不是则删除并报错
	if !fontMagicOK(tmpPath) {
		os.Remove(tmpPath)
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件内容不是有效字体（可能是误下载的网页），请确认后再上传"})
		return
	}

	// ① 内容去重：与服务器上任一已存在字体完全相同 → 拒绝（不再自动改名成 -2/-3）
	if dup, ok := h.findBySum(dir, sum, tmpPath); ok {
		os.Remove(tmpPath)
		c.JSON(http.StatusConflict, gin.H{
			"error":    "该字体已存在：`" + dup + "`（内容完全相同），请直接使用它，或先删除旧字体再上传",
			"code":     "duplicate_content",
			"existing": dup,
		})
		return
	}
	// ② 同名但内容不同 → 拒绝（不覆盖、不自动改名）
	if _, err := os.Stat(filepath.Join(dir, fname)); err == nil {
		os.Remove(tmpPath)
		c.JSON(http.StatusConflict, gin.H{
			"error":    "同名文件已存在：`" + fname + "`（内容不同）。如需替换请先删除旧字体，或改名后上传",
			"code":     "duplicate_name",
			"existing": fname,
		})
		return
	}

	// ③ 正常保存（rename 原子落盘）
	savePath := filepath.Join(dir, fname)
	if err := os.Rename(tmpPath, savePath); err != nil {
		os.Remove(tmpPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}
	fi, err := os.Stat(savePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}
	h.setHash(savePath, fi, sum)
	c.JSON(http.StatusOK, h.toInfo(fname, fi))
}

// Delete DELETE /api/fonts/:name 删除字体
func (h *FontHandler) Delete(c *gin.Context) {
	name := filepath.Base(c.Param("name"))
	if !fontExtRe.MatchString(strings.ToLower(name)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法文件名"})
		return
	}
	p := filepath.Join(h.fontDir(), name)
	h.loadHashCache()
	if err := os.Remove(p); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "字体不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	fontHashMu.Lock()
	delete(fontHashCache, p)
	fontHashMu.Unlock()
	h.saveHashCache()
	c.JSON(http.StatusOK, gin.H{"ok": true, "name": name, "at": time.Now().Unix()})
}
