package handler

import (
	"database/sql"
	"log"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"familytree/internal/config"

	"github.com/gin-gonic/gin"
)

// UploadHandler 文件上传/附件处理器
type UploadHandler struct {
	DB  *sql.DB
	Cfg *config.Config
}

// Attachment 附件信息
type Attachment struct {
	ID          int64  `json:"id"`
	MemberID    int64  `json:"member_id"`
	SpouseID    int64  `json:"spouse_id"`
	Category    string `json:"category"`
	Path        string `json:"path"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

// Upload 上传附件
func (h *UploadHandler) Upload(c *gin.Context) {
	memberID, _ := strconv.ParseInt(c.PostForm("member_id"), 10, 64)
	spouseID, _ := strconv.ParseInt(c.PostForm("spouse_id"), 10, 64)
	description := c.PostForm("description")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择文件"})
		return
	}
	defer file.Close()

	// 上传时不选分类：按文件类型自动归类（照片/视频/音频）
	category := c.PostForm("category")
	if category == "" {
		category = detectCategory(header.Filename)
	}

	uploadDir := filepath.Join(h.Cfg.DataDir, "upload")
	os.MkdirAll(uploadDir, 0o755)

	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%d_%d%s", time.Now().UnixMilli(), memberID, ext)
	savePath := filepath.Join(uploadDir, filename)

	out, err := os.Create(savePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入文件失败"})
		return
	}
	out.Close()

	// 写入数据库
	relPath := "/uploads/" + filename

	// 上传后立即预生成缩略图
	if isImageExt(ext) {
		GenerateThumbnail(h.Cfg.DataDir, relPath, 120, 240, 400)
	} else if isVideoExt(ext) {
		go GenerateVideoThumbnail(h.Cfg.DataDir, relPath, 120, 400)
	}

	res, err := h.DB.Exec(`INSERT INTO attachments (member_id, spouse_id, category, path, description) VALUES (?,?,?,?,?)`,
		memberID, spouseID, category, relPath, description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 如果 category 是"头像"，更新对应头像字段
	if category == "头像" && memberID > 0 {
		h.DB.Exec(`UPDATE members SET portrait = ? WHERE id = ?`, relPath, memberID)
	}
	if category == "配偶头像" && spouseID > 0 {
		h.DB.Exec(`UPDATE spouses SET portrait = ? WHERE id = ?`, relPath, spouseID)
	}

	id, _ := res.LastInsertId()
	c.JSON(http.StatusOK, Attachment{
		ID:       id,
		MemberID: memberID,
		SpouseID: spouseID,
		Category: category,
		Path:     relPath,
		Description: description,
	})
}

// ListByMember 获取某成员的附件列表
func (h *UploadHandler) ListByMember(c *gin.Context) {
	memberID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	rows, err := h.DB.Query(`SELECT id, member_id, COALESCE(spouse_id,0), category, path, description, created_at
		FROM attachments WHERE member_id = ? AND (spouse_id IS NULL OR spouse_id = 0) ORDER BY created_at DESC`, memberID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	list := []Attachment{}
	for rows.Next() {
		var a Attachment
		rows.Scan(&a.ID, &a.MemberID, &a.SpouseID, &a.Category, &a.Path, &a.Description, &a.CreatedAt)
		list = append(list, a)
	}
	c.JSON(http.StatusOK, list)
}

// ListBySpouse 获取某配偶的附件列表
// GET /api/spouses/:id/attachments
func (h *UploadHandler) ListBySpouse(c *gin.Context) {
	spouseID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	rows, err := h.DB.Query(`SELECT id, COALESCE(member_id,0), COALESCE(spouse_id,0), category, path, description, created_at
		FROM attachments WHERE spouse_id = ? ORDER BY created_at DESC`, spouseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	list := []Attachment{}
	for rows.Next() {
		var a Attachment
		rows.Scan(&a.ID, &a.MemberID, &a.SpouseID, &a.Category, &a.Path, &a.Description, &a.CreatedAt)
		list = append(list, a)
	}
	c.JSON(http.StatusOK, list)
}

// detectCategory 按文件扩展名自动归类多媒体分类
func detectCategory(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	imageExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".bmp": true, ".svg": true, ".heic": true, ".heif": true}
	videoExts := map[string]bool{".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".webm": true, ".flv": true, ".wmv": true, ".m4v": true, ".mpeg": true, ".mpg": true}
	audioExts := map[string]bool{".mp3": true, ".wav": true, ".m4a": true, ".aac": true, ".flac": true, ".ogg": true, ".wma": true, ".opus": true}
	if imageExts[ext] {
		return "照片"
	}
	if videoExts[ext] {
		return "视频"
	}
	if audioExts[ext] {
		return "音频"
	}
	return "照片"
}

func isImageExt(ext string) bool {
	imageExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".bmp": true}
	return imageExts[strings.ToLower(ext)]
}

func isVideoExt(ext string) bool {
	videoExts := map[string]bool{".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".webm": true, ".flv": true, ".wmv": true, ".m4v": true, ".mpeg": true, ".mpg": true}
	return videoExts[strings.ToLower(ext)]
}

// Delete 删除附件
func (h *UploadHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	var filePath string
	h.DB.QueryRow(`SELECT path FROM attachments WHERE id = ?`, id).Scan(&filePath)

	_, err := h.DB.Exec(`DELETE FROM attachments WHERE id = ?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 尝试删除物理文件（带引用检查：同一 path 可能被其它附件/多媒体记录/头像引用 → 只删记录）
	if filePath != "" {
		refs := CountFileRefs(h.DB, filePath)
		if refs > 0 {
			c.JSON(http.StatusOK, gin.H{"ok": true, "file_deleted": false, "refs": refs,
				"note": "该文件仍被其它记录引用，已只删记录、保留源文件"})
			return
		}
		DeleteThumbnailsForSource(h.Cfg.DataDir, filePath)
		absPath := filepath.Join(h.Cfg.DataDir, "upload", filepath.Base(filePath))
		if err := os.Remove(absPath); err != nil {
			log.Printf("[附件删除] 物理文件删除失败 %s: %v", absPath, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}