package handler

import (
	"database/sql"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	_ "image/gif"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/image/draw"
)

// ThumbnailHandler 缩略图服务
type ThumbnailHandler struct {
	DB      *sql.DB
	DataDir string
}

const thumbQuality = 75

// ServeThumbnail GET /api/thumbnail?path=/uploads/xxx.jpg&size=120
func (h *ThumbnailHandler) ServeThumbnail(c *gin.Context) {
	imgPath := c.Query("path")
	sizeStr := c.DefaultQuery("size", "120")

	if imgPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 path 参数"})
		return
	}

	size, err := strconv.Atoi(sizeStr)
	if err != nil || size < 16 || size > 512 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "size 需在 16~512 之间"})
		return
	}

	// URL 路径统一按 "/" 处理：Windows 上 filepath.Clean 会把 "/uploads/x" 变成 "\uploads\x"，
	// 导致下面的前缀校验失败并返回 400（前端 el-image 显示 FAILED）。必须用 path.Clean。
	cleanPath := path.Clean(strings.ReplaceAll(imgPath, "\\", "/"))
	if !strings.HasPrefix(cleanPath, "/uploads/") && !strings.HasPrefix(cleanPath, "uploads/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径不合法"})
		return
	}
	cleanPath = strings.TrimPrefix(cleanPath, "/")

	srcPath := filepath.Join(h.DataDir, "upload", filepath.Base(cleanPath))
	srcInfo, err := os.Stat(srcPath)
	if os.IsNotExist(err) {
		c.Status(http.StatusNotFound)
		return
	}

	cacheDir := filepath.Join(h.DataDir, "thumbnails")
	os.MkdirAll(cacheDir, 0755)

	cacheKey := thumbCacheKey(srcPath, size, srcInfo.ModTime().Unix())
	cachePath := filepath.Join(cacheDir, cacheKey)

	etag := `"` + cacheKey + `"`
	if fi, err := os.Stat(cachePath); err == nil && fi.Size() > 0 {
		c.Header("ETag", etag)
		c.Header("Cache-Control", "public, max-age=604800")
		c.Header("Content-Type", "image/jpeg")
		if match := c.GetHeader("If-None-Match"); match == etag {
			c.Status(http.StatusNotModified)
			return
		}
		c.File(cachePath)
		return
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer srcFile.Close()

	img, fmtName, err := image.Decode(srcFile)
	if err != nil {
		// 图片解码失败 → 尝试用 ffmpeg 生成视频缩略图
		log.Printf("[thumbnail] image decode failed %s (%s), trying ffmpeg...", srcPath, fmtName)
		srcFile.Close()
		if _, lookErr := exec.LookPath("ffmpeg"); lookErr != nil {
			c.Status(http.StatusUnsupportedMediaType)
			return
		}
		cmd := exec.Command("ffmpeg", "-y", "-ss", "1", "-i", srcPath,
			"-vframes", "1", "-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", size, size),
			cachePath)
		if cmd.Run() != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Header("ETag", etag)
		c.Header("Cache-Control", "public, max-age=604800")
		c.Header("Content-Type", "image/jpeg")
		c.File(cachePath)
		return
	}

	thumb := resize(img, size)

	out, err := os.Create(cachePath)
	if err != nil {
		c.Header("ETag", etag)
		c.Header("Cache-Control", "public, max-age=604800")
		c.Header("Content-Type", "image/jpeg")
		jpeg.Encode(c.Writer, thumb, &jpeg.Options{Quality: thumbQuality})
		return
	}
	defer out.Close()

	c.Header("ETag", etag)
	c.Header("Cache-Control", "public, max-age=604800")
	c.Header("Content-Type", "image/jpeg")
	jpeg.Encode(io.MultiWriter(out, c.Writer), thumb, &jpeg.Options{Quality: thumbQuality})
}

func resize(src image.Image, size int) image.Image {
	b := src.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		return src
	}

	// 缩放到短边 ≤ size（保持比例）
	scale := float64(size) / float64(srcW)
	if hScale := float64(size) / float64(srcH); hScale < scale {
		scale = hScale
	}
	if scale >= 1.0 {
		return src
	}

	newW := int(float64(srcW)*scale + 0.5)
	newH := int(float64(srcH)*scale + 0.5)
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

// thumbCacheKey 生成可预测的缓存键：<文件名base>_<size>_<mtime(hex)>.jpg
// 之所以不用 SHA256 是为了删除时能按文件名前缀快速定位
func thumbCacheKey(srcPath string, size int, mtime int64) string {
	base := strings.TrimSuffix(filepath.Base(srcPath), filepath.Ext(srcPath))
	return fmt.Sprintf("%s_%d_%x", base, size, mtime)
}

// GenerateThumbnail 上传时预生成缩略图（同步，图片快）
func GenerateThumbnail(dataDir, relPath string, sizes ...int) {
	cleanPath := strings.TrimPrefix(path.Clean(strings.ReplaceAll(relPath, "\\", "/")), "/")
	srcPath := filepath.Join(dataDir, "upload", filepath.Base(cleanPath))

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return
	}

	cacheDir := filepath.Join(dataDir, "thumbnails")
	os.MkdirAll(cacheDir, 0755)

	if len(sizes) == 0 {
		sizes = []int{120, 400} // 默认缩略图尺寸：小（头像/列表）+ 中（详情/资料页）
	}

	for _, size := range sizes {
		cacheKey := thumbCacheKey(srcPath, size, srcInfo.ModTime().Unix())
		cachePath := filepath.Join(cacheDir, cacheKey)

		if fi, err := os.Stat(cachePath); err == nil && fi.Size() > 0 {
			continue // 已缓存
		}

		srcFile, err := os.Open(srcPath)
		if err != nil {
			continue
		}

		img, _, err := image.Decode(srcFile)
		srcFile.Close()
		if err != nil {
			log.Printf("[thumbnail] 预生成失败(非图片): %s", relPath)
			return
		}

		thumb := resize(img, size)
		out, err := os.Create(cachePath)
		if err != nil {
			continue
		}
		jpeg.Encode(out, thumb, &jpeg.Options{Quality: thumbQuality})
		out.Close()
	}
}

// GenerateVideoThumbnail 上传时预生成视频缩略图（异步，ffmpeg 较慢）
// ffmpeg 不可用时会静默降级，仅记日志
func GenerateVideoThumbnail(dataDir, relPath string, sizes ...int) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		log.Printf("[thumbnail] ffmpeg 未安装，跳过视频缩略图: %s", relPath)
		return
	}
	cleanPath := strings.TrimPrefix(path.Clean(strings.ReplaceAll(relPath, "\\", "/")), "/")
	srcPath := filepath.Join(dataDir, "upload", filepath.Base(cleanPath))

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return
	}

	if len(sizes) == 0 {
		sizes = []int{120, 400}
	}

	for _, size := range sizes {
		cacheKey := thumbCacheKey(srcPath, size, srcInfo.ModTime().Unix())
		cachePath := filepath.Join(dataDir, "thumbnails", cacheKey)

		if fi, err := os.Stat(cachePath); err == nil && fi.Size() > 0 {
			continue
		}

		// ffmpeg -ss 1 -i video.mp4 -vframes 1 -vf scale=<size>:-1 thumbnail.jpg
		cmd := exec.Command("ffmpeg", "-y", "-ss", "1", "-i", srcPath,
			"-vframes", "1", "-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", size, size),
			cachePath)
		if err := cmd.Run(); err != nil {
			log.Printf("[thumbnail] 视频缩略图生成失败 %s: %v", relPath, err)
		}
	}
}

// DeleteThumbnailsForSource 删除源文件对应的所有缩略图缓存
func DeleteThumbnailsForSource(dataDir, relPath string) {
	cleanPath := strings.TrimPrefix(path.Clean(strings.ReplaceAll(relPath, "\\", "/")), "/")
	srcBase := strings.TrimSuffix(filepath.Base(cleanPath), filepath.Ext(cleanPath))

	cacheDir := filepath.Join(dataDir, "thumbnails")
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return
	}

	prefix := srcBase + "_"
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			os.Remove(filepath.Join(cacheDir, entry.Name()))
		}
	}
}