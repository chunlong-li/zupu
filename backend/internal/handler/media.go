package handler

import (
	"crypto/sha256"
	"log"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"familytree/internal/config"

	"github.com/gin-gonic/gin"
)

// MediaHandler 通用多媒体库（不属于某个成员的公共素材：祠堂照片、传统节日图片/视频等）
type MediaHandler struct {
	DB  *sql.DB
	Cfg *config.Config
}

func isVideoExtMedia(ext string) bool {
	switch strings.ToLower(ext) {
	case ".mp4", ".mov", ".m4v", ".webm", ".avi", ".mkv", ".ogv":
		return true
	}
	return false
}

// detectMediaType 按扩展名判定 image / video
func detectMediaType(name string) string {
	if isVideoExtMedia(filepath.Ext(name)) {
		return "video"
	}
	return "image"
}


// ===== 素材可见范围（按调用者的授权房支，后端强制）=====
// 判定依据：认证中间件写入 context 的 role / branch（＝users.branch，由「用户管理→分配房支」设置）。
//   all  ＝不限制（超级管理员）；deny ＝什么都不许（分配了不存在的房支）；否则 ids＝允许的房支（总谱素材恒允许）
//   ② 未分配房支 → 只看总谱素材（不含任何房支）
type mediaScope struct {
	all  bool
	deny bool
	ids  []int64
}

func MediaScopeOf(c *gin.Context, db *sql.DB) mediaScope {
	if c.GetString("role") == RoleSuperAdmin {
		return mediaScope{all: true}
	}
	branch := strings.TrimSpace(c.GetString("branch"))
	if branch == "" {
		return mediaScope{}
	}
	var id int64
	var path string
	if err := db.QueryRow(`SELECT id, COALESCE(path,'') FROM branches WHERE name = ?`, branch).Scan(&id, &path); err != nil {
		return mediaScope{deny: true}
	}
	ids := []int64{id}
	if c.Query("include_children") == "1" || c.Query("include_children") == "true" {
		if path != "" {
			if rows, err := db.Query(`SELECT id FROM branches WHERE path = ? OR path LIKE ?`, path, path+".%"); err == nil {
				defer rows.Close()
				for rows.Next() {
					var x int64
					if rows.Scan(&x) == nil && x != id {
						ids = append(ids, x)
					}
				}
			}
		}
	}
	return mediaScope{ids: ids}
}

// MediaVisibleCond 返回媒体素材可见性 SQL 片段与参数；空串＝不限制
func MediaVisibleCond(c *gin.Context, db *sql.DB) (string, []interface{}) {
	sc := MediaScopeOf(c, db)
	if sc.all {
		return "", nil
	}
	if sc.deny {
		return "1=0", nil
	}
	if len(sc.ids) == 0 {
		return "(COALESCE(branch_id,0) = 0)", nil
	}
	ph := make([]string, 0, len(sc.ids))
	args := make([]interface{}, 0, len(sc.ids))
	for _, v := range sc.ids {
		ph = append(ph, "?")
		args = append(args, v)
	}
	return "(COALESCE(branch_id,0) = 0 OR branch_id IN (" + strings.Join(ph, ",") + "))", args
}

// MediaBranchAllowed 调用者是否可读/写 branch_id 归属的素材（branchID=0 即总谱素材，恒允许）
func MediaBranchAllowed(c *gin.Context, db *sql.DB, branchID int64) bool {
	sc := MediaScopeOf(c, db)
	if sc.all || branchID == 0 {
		return true
	}
	for _, v := range sc.ids {
		if v == branchID {
			return true
		}
	}
	return false
}

// List 列表（可按 category 筛选）
func (h *MediaHandler) List(c *gin.Context) {
	cat := strings.TrimSpace(c.Query("category"))
	vis, vargs := MediaVisibleCond(c, h.DB)
	where := []string{}
	args := []interface{}{}
	if cat != "" && cat != "全部" {
		where = append(where, "category = ?")
		args = append(args, cat)
	}
	if vis != "" {
		where = append(where, vis)
		args = append(args, vargs...)
	}
	q := `SELECT id, path, type, category, title, description, created_at, COALESCE(branch_id,0) FROM media_assets`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY id DESC"
	rows, err := h.DB.Query(q, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var path, typ, category, title, desc, created string
		var branchID int64
		if err := rows.Scan(&id, &path, &typ, &category, &title, &desc, &created, &branchID); err != nil {
			continue
		}
		out = append(out, gin.H{"id": id, "path": path, "type": typ, "category": category,
			"title": title, "description": desc, "created_at": created, "branch_id": branchID})
	}
	c.JSON(http.StatusOK, out)
}

// Categories 已用分类（供筛选下拉）
func (h *MediaHandler) Categories(c *gin.Context) {
	vis, vargs := MediaVisibleCond(c, h.DB)
	cq := `SELECT DISTINCT category FROM media_assets WHERE category <> ''`
	if vis != "" {
		cq += " AND " + vis
	}
	cq += " ORDER BY category"
	rows, err := h.DB.Query(cq, vargs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if rows.Scan(&v) == nil && v != "" {
			out = append(out, v)
		}
	}
	c.JSON(http.StatusOK, out)
}

// Upload 上传通用媒体（图片/视频）；落在既有 upload/ 目录 → 自动纳入既有备份
func (h *MediaHandler) Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择文件"})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	typ := detectMediaType(header.Filename)
	if typ == "image" {
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg":
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的图片格式：" + ext})
			return
		}
	} else if !isVideoExtMedia(ext) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的文件格式：" + ext})
		return
	}

	uploadDir := filepath.Join(h.Cfg.DataDir, "upload")
	os.MkdirAll(uploadDir, 0o755)
	filename := fmt.Sprintf("media_%d%s", time.Now().UnixMilli(), ext)
	savePath := filepath.Join(uploadDir, filename)

	// 先写临时文件并计算 sha256（上传大小上限由 FAMILY_TREE_MAX_UPLOAD_MB 控制，成本可控）
	tmp, err := os.CreateTemp(uploadDir, ".upload-*.tmp")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}
	hsh := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, hsh), file)
	if err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入文件失败"})
		return
	}
	tmp.Close()
	sum := hex.EncodeToString(hsh.Sum(nil))

	// 内容查重：同一份内容（先按字节大小筛，再比 sha256）若已在 upload/ 里存在 → 复用它，不再复制一份。
	// 这样"删除后又重新上传同一个文件"永远不会因为"文件已存在"而报错。
	relPath, reused := findSameContentFile(h.Cfg.DataDir, size, sum)
	if reused {
		os.Remove(tmp.Name())
	} else {
		if err := os.Rename(tmp.Name(), savePath); err != nil {
			// 跨设备等极端情况下退回 io.Copy
			src, e1 := os.Open(tmp.Name())
			if e1 != nil {
				os.Remove(tmp.Name())
				c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
				return
			}
			dst, e2 := os.Create(savePath)
			if e2 != nil {
				src.Close()
				os.Remove(tmp.Name())
				c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
				return
			}
			io.Copy(dst, src)
			dst.Close()
			src.Close()
			os.Remove(tmp.Name())
		}
		relPath = "/uploads/" + filename
	}
	// 复用来的路径：若该文件仍被别的记录引用，提示"已存在"；若无人引用（是孤儿）提示"已复用"。两种都成功，不报错。
	refs := CountFileRefs(h.DB, relPath)
	if typ == "image" {
		GenerateThumbnail(h.Cfg.DataDir, relPath, 120, 240, 400)
	} else {
		go GenerateVideoThumbnail(h.Cfg.DataDir, relPath, 120, 400)
	}

	category := strings.TrimSpace(c.PostForm("category"))
	if category == "" {
		category = "其他"
	}
	title := strings.TrimSpace(c.PostForm("title"))
	description := c.PostForm("description")
	// 归属：由前端按顶部「宗谱/房支」选择器自动带上；0＝总谱（缺省也是总谱，不强制前端手填）
	branchID, _ := strconv.ParseInt(strings.TrimSpace(c.PostForm("branch_id")), 10, 64)
	if branchID < 0 {
		branchID = 0
	}
	if !MediaBranchAllowed(c, h.DB, branchID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权把素材归属到该房支（仅可上传到总谱或你被授权的房支）"})
		return
	}

	res, err := h.DB.Exec(`INSERT INTO media_assets (path, type, category, title, description, branch_id) VALUES (?,?,?,?,?,?)`,
		relPath, typ, category, title, description, branchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	id, _ := res.LastInsertId()
	note := ""
	if reused {
		if refs > 0 {
			note = "该文件已存在（已被 " + strconv.Itoa(refs) + " 条记录引用），已直接为你复用，未重复占盘"
		} else {
			note = "检测到内容相同的文件（此前未被引用），已复用之，未重复占盘"
		}
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "path": relPath, "type": typ, "category": category,
		"title": title, "description": description, "branch_id": branchID, "reused": reused, "note": note})
}

// findSameContentFile 在 <DataDir>/upload/ 里找"同大小 + 同 sha256"的文件；找到返回其 /uploads/ 相对路径。
// 先按大小筛选再算 sha，通常只有 0~2 个候选，代价很低。
func findSameContentFile(dataDir string, size int64, sum string) (string, bool) {
	dir := filepath.Join(dataDir, "upload")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "media_") == false && !strings.Contains(name, "_") {
			// 仍然继续（文件名格式不固定），只跳过目录/隐藏文件
		}
		if e.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		fi, err := e.Info()
		if err != nil || fi.Size() != size {
			continue
		}
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		hh := sha256.New()
		_, err = io.Copy(hh, f)
		f.Close()
		if err != nil {
			continue
		}
		if hex.EncodeToString(hh.Sum(nil)) == sum {
			return "/uploads/" + name, true
		}
	}
	return "", false
}

// CountFileRefs 该 path 被多少条记录引用（media_assets / attachments / members.portrait / spouses.portrait）
func CountFileRefs(db *sql.DB, relPath string) int {
	if strings.TrimSpace(relPath) == "" {
		return 0
	}
	n := 0
	var c int
	if err := db.QueryRow(`SELECT COUNT(*) FROM media_assets WHERE path = ?`, relPath).Scan(&c); err == nil {
		n += c
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM attachments WHERE path = ?`, relPath).Scan(&c); err == nil {
		n += c
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM members WHERE portrait = ?`, relPath).Scan(&c); err == nil {
		n += c
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM spouses WHERE portrait = ?`, relPath).Scan(&c); err == nil {
		n += c
	}
	return n
}

// removeUploadAndThumbs 删除源文件与它的缩略图缓存，返回释放的字节数
func removeUploadAndThumbs(dataDir, relPath string) int64 {
	if strings.TrimSpace(relPath) == "" {
		return 0
	}
	abs := filepath.Join(dataDir, "upload", filepath.Base(relPath))
	var freed int64
	if fi, err := os.Stat(abs); err == nil {
		freed = fi.Size()
	}
	DeleteThumbnailsForSource(dataDir, relPath)
	if err := os.Remove(abs); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[多媒体删除] 源文件删除失败 %s: %v", abs, err)
			return 0
		}
	}
	return freed
}

// Orphans GET /api/media/orphans —— 列出 upload/ 里既不被 media_assets 也不被 attachments 引用（也不做头像）的文件
func (h *MediaHandler) Orphans(c *gin.Context) {
	files, total := listOrphanFiles(h.Cfg.DataDir, h.DB)
	out := []gin.H{}
	for _, f := range files {
		out = append(out, gin.H{"name": f.Name, "path": "/uploads/" + f.Name, "size": f.Size})
	}
	c.JSON(http.StatusOK, gin.H{"count": len(out), "bytes": total, "files": out})
}

// CleanOrphans POST /api/media/orphans/clean —— 删除孤儿文件（连同它们的缩略图缓存）
func (h *MediaHandler) CleanOrphans(c *gin.Context) {
	files, total := listOrphanFiles(h.Cfg.DataDir, h.DB)
	removed := 0
	var freed int64
	for _, f := range files {
		rel := "/uploads/" + f.Name
		DeleteThumbnailsForSource(h.Cfg.DataDir, rel)
		if err := os.Remove(filepath.Join(h.Cfg.DataDir, "upload", f.Name)); err == nil {
			removed++
			freed += f.Size
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "count": removed, "bytes": freed,
		"note": "已清理 " + strconv.Itoa(removed) + " 个孤儿文件，释放 " + humanSize(freed) + "（预计 " + humanSize(total) + "）"})
}

type orphanFile struct {
	Name string
	Size int64
}

// listOrphanFiles 扫 upload/，剔除被 media_assets / attachments / 头像引用的文件
func listOrphanFiles(dataDir string, db *sql.DB) ([]orphanFile, int64) {
	dir := filepath.Join(dataDir, "upload")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0
	}
	referenced := map[string]bool{}
	add := func(q string) {
		rows, err := db.Query(q)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var p string
			if rows.Scan(&p) == nil && p != "" {
				referenced[filepath.Base(p)] = true
			}
		}
	}
	add(`SELECT path FROM media_assets WHERE path <> ''`)
	add(`SELECT path FROM attachments WHERE path <> ''`)
	add(`SELECT portrait FROM members WHERE portrait <> ''`)
	add(`SELECT portrait FROM spouses WHERE portrait <> ''`)

	out := []orphanFile{}
	var total int64
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if referenced[e.Name()] {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, orphanFile{Name: e.Name(), Size: fi.Size()})
		total += fi.Size()
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	return out, total
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// Update 修改分类 / 标题 / 说明
func (h *MediaHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var curBranch int64
	if err := h.DB.QueryRow(`SELECT COALESCE(branch_id,0) FROM media_assets WHERE id = ?`, id).Scan(&curBranch); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	if !MediaBranchAllowed(c, h.DB, curBranch) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权修改其它房支的素材"})
		return
	}
	var body struct {
		Category    *string `json:"category"`
		Title       *string `json:"title"`
		Description *string `json:"description"`
		BranchID    *int64  `json:"branch_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Category != nil {
		h.DB.Exec(`UPDATE media_assets SET category = ? WHERE id = ?`, *body.Category, id)
	}
	if body.Title != nil {
		h.DB.Exec(`UPDATE media_assets SET title = ? WHERE id = ?`, *body.Title, id)
	}
	if body.Description != nil {
		h.DB.Exec(`UPDATE media_assets SET description = ? WHERE id = ?`, *body.Description, id)
	}
	if body.BranchID != nil {
		if !MediaBranchAllowed(c, h.DB, *body.BranchID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权把素材改归到该房支"})
			return
		}
		h.DB.Exec(`UPDATE media_assets SET branch_id = ? WHERE id = ?`, *body.BranchID, id)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Delete 删除记录与文件（带引用检查：仍被别的记录引用 → 只删记录、保留文件）
func (h *MediaHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var path string
	var curBranch int64
	if err := h.DB.QueryRow(`SELECT path, COALESCE(branch_id,0) FROM media_assets WHERE id = ?`, id).Scan(&path, &curBranch); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		return
	}
	if !MediaBranchAllowed(c, h.DB, curBranch) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除其它房支的素材"})
		return
	}
	if _, err := h.DB.Exec(`DELETE FROM media_assets WHERE id = ?`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 删记录后再查引用：media_assets 其它行 / attachments / members.portrait / spouses.portrait
	refs := CountFileRefs(h.DB, path)
	var freed int64
	fileDeleted := false
	if refs == 0 {
		freed = removeUploadAndThumbs(h.Cfg.DataDir, path)
		fileDeleted = true
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "file_deleted": fileDeleted, "freed_bytes": freed, "refs": refs,
		"note": func() string {
			if refs > 0 {
				return "该文件仍被 " + strconv.Itoa(refs) + " 条记录引用，已只删记录、保留源文件"
			}
			if fileDeleted {
				return "已删除记录与源文件（含缩略图缓存，释放 " + humanSize(freed) + "）"
			}
			return "已删除记录（源文件本就不存在）"
		}()})
}
