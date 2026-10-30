package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"familytree/internal/config"

	"github.com/gin-gonic/gin"
)

// 造一个「魔数合法」的假字体：fontMagicOK 只看头 4 字节
func fakeFont(fill byte, n int) []byte {
	b := make([]byte, 0, n+4)
	b = append(b, 0x00, 0x01, 0x00, 0x00)
	for i := 0; i < n; i++ {
		b = append(b, fill)
	}
	return b
}

func newFontTestRouter(t *testing.T) (*gin.Engine, *FontHandler, string) {
	t.Helper()
	dir := t.TempDir()
	h := &FontHandler{Cfg: &config.Config{DataDir: dir}}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/fonts", h.Upload)
	r.DELETE("/api/fonts/:name", h.Delete)
	// 每个测试用全新的哈希缓存（全局变量，测试间要清）
	fontHashMu.Lock()
	fontHashCache = map[string]fontHashEntry{}
	fontHashLoaded = false
	fontHashMu.Unlock()
	return r, h, filepath.Join(dir, "fonts")
}

func upload(t *testing.T, r *gin.Engine, name string, content []byte) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", name)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write: %v", err)
	}
	w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/fonts", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

func fontFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := []string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if fontExtRe.MatchString(strings.ToLower(e.Name())) {
			out = append(out, e.Name())
		}
	}
	return out
}

func TestFontUploadDedup(t *testing.T) {
	r, h, dir := newFontTestRouter(t)
	contentX := fakeFont(0x11, 2048)
	contentY := fakeFont(0x22, 2048)
	contentZ := fakeFont(0x33, 2048)

	// 1) 首次上传 → 成功
	if code, body := upload(t, r, "a.ttf", contentX); code != 200 {
		t.Fatalf("首次上传应 200，得 %d %v", code, body)
	}
	if got := fontFiles(t, dir); len(got) != 1 || got[0] != "a.ttf" {
		t.Fatalf("首次上传后磁盘应有且仅有 a.ttf，得 %v", got)
	}

	// 2) 同一文件再传一次 → 409 duplicate_content，磁盘不变
	code, body := upload(t, r, "a.ttf", contentX)
	if code != http.StatusConflict || body["code"] != "duplicate_content" {
		t.Fatalf("同文件重传应 409 duplicate_content，得 %d %v", code, body)
	}
	if body["existing"] != "a.ttf" {
		t.Fatalf("existing 应为 a.ttf，得 %v", body["existing"])
	}
	if msg, _ := body["error"].(string); !strings.Contains(msg, "该字体已存在") || !strings.Contains(msg, "a.ttf") {
		t.Fatalf("文案不对：%v", msg)
	}
	if got := fontFiles(t, dir); len(got) != 1 {
		t.Fatalf("重传被拒后磁盘仍应只有 1 个字体，得 %v", got)
	}

	// 3) 同内容改名 → 仍 409 duplicate_content，指向现有文件名
	code, body = upload(t, r, "b.ttf", contentX)
	if code != http.StatusConflict || body["code"] != "duplicate_content" || body["existing"] != "a.ttf" {
		t.Fatalf("同内容改名应 409 duplicate_content(existing=a.ttf)，得 %d %v", code, body)
	}
	if got := fontFiles(t, dir); len(got) != 1 {
		t.Fatalf("同内容改名被拒后磁盘应仍只有 1 个，得 %v", got)
	}

	// 4) 同名不同内容 → 409 duplicate_name
	code, body = upload(t, r, "a.ttf", contentY)
	if code != http.StatusConflict || body["code"] != "duplicate_name" || body["existing"] != "a.ttf" {
		t.Fatalf("同名不同内容应 409 duplicate_name，得 %d %v", code, body)
	}
	if msg, _ := body["error"].(string); !strings.Contains(msg, "同名文件已存在") {
		t.Fatalf("同名文案不对：%v", msg)
	}
	if got := fontFiles(t, dir); len(got) != 1 {
		t.Fatalf("同名被拒后磁盘应仍只有 1 个，得 %v", got)
	}

	// 5) 全新字体 → 成功
	if code, body := upload(t, r, "c.ttf", contentZ); code != 200 {
		t.Fatalf("新字体应 200，得 %d %v", code, body)
	}
	got := fontFiles(t, dir)
	if len(got) != 2 {
		t.Fatalf("应为 2 个字体文件，得 %v", got)
	}

	// 6) 不允许出现 -2/-3 这类自动改名产物
	re := regexp.MustCompile(`-\d+\.(ttf|otf|woff2?|ttc)$`)
	for _, n := range got {
		if re.MatchString(n) {
			t.Fatalf("不应出现自动改名文件：%v", got)
		}
	}
	// 7) 临时文件不能残留
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".upload-") {
			t.Fatalf("临时上传文件残留：%v", e.Name())
		}
	}

	// 8) 哈希缓存旁挂文件应存在；清掉内存缓存（模拟进程重启）后结论不变
	if _, err := os.Stat(h.hashCachePath()); err != nil {
		t.Fatalf("哈希缓存文件未生成：%v", err)
	}
	fontHashMu.Lock()
	fontHashCache = map[string]fontHashEntry{}
	fontHashLoaded = false
	fontHashMu.Unlock()
	if code, body := upload(t, r, "b.ttf", contentX); code != http.StatusConflict || body["code"] != "duplicate_content" {
		t.Fatalf("冷缓存（走 .sha256.json）后仍应 409 duplicate_content，得 %d %v", code, body)
	}
	if got := fontFiles(t, dir); len(got) != 2 {
		t.Fatalf("最终应仍为 2 个字体，得 %v", got)
	}
}

func TestFontUploadValidationStillWorks(t *testing.T) {
	r, _, dir := newFontTestRouter(t)

	// 扩展名白名单
	if code, _ := upload(t, r, "x.txt", fakeFont(0x44, 64)); code != http.StatusBadRequest {
		t.Fatalf("非法扩展名应 400，得 %d", code)
	}
	// 魔数校验（HTML 冒充字体）
	html := append([]byte("<html>not a font</html>"), fakeFont(0x00, 8)...)
	if code, body := upload(t, r, "fake.ttf", html); code != http.StatusBadRequest {
		t.Fatalf("非字体内容应 400，得 %d %v", code, body)
	}
	if got := fontFiles(t, dir); len(got) != 0 {
		t.Fatalf("被拒的上传不应落盘，得 %v", got)
	}
	// 正常字体仍可上传
	if code, _ := upload(t, r, "ok.ttf", fakeFont(0x55, 128)); code != 200 {
		t.Fatalf("正常字体应 200，得 %d", code)
	}
	// 删除后同内容可再传（缓存要跟着删）
	if rec := httptest.NewRecorder(); true {
		req := httptest.NewRequest(http.MethodDelete, "/api/fonts/ok.ttf", nil)
		r.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("删除应 200，得 %d", rec.Code)
		}
	}
	if code, _ := upload(t, r, "ok2.ttf", fakeFont(0x55, 128)); code != 200 {
		t.Fatalf("删除旧字体后同内容应可再传，得 %d", code)
	}
}
