package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMediaDedup(t *testing.T) {
	base := t.TempDir()
	// 真实用法：uploadDir = <data>/upload，文件在其子目录下
	upload := filepath.Join(base, "upload")
	photos := filepath.Join(upload, "photos")
	os.MkdirAll(photos, 0o755)
	os.WriteFile(filepath.Join(photos, "a.jpg"), []byte("AAAA"), 0o644)
	os.WriteFile(filepath.Join(photos, "b.jpg"), []byte("BBBB"), 0o644)

	media := filepath.Join(base, "backup", "media")
	os.MkdirAll(media, 0o755)
	d1 := filepath.Join(media, "20260831")
	if err := snapshotDir(upload, "", d1); err != nil {
		t.Fatal("snapshot d1:", err)
	}

	// day2: change b, add c; a unchanged
	os.WriteFile(filepath.Join(photos, "b.jpg"), []byte("BBBB2"), 0o644)
	os.WriteFile(filepath.Join(photos, "c.jpg"), []byte("CCCC"), 0o644)
	d2 := filepath.Join(media, "20260901")
	if err := snapshotDir(upload, d1, d2); err != nil {
		t.Fatal("snapshot d2:", err)
	}

	// a 未变 → 硬链接共享 inode
	a1, _ := os.Stat(filepath.Join(d1, "photos", "a.jpg"))
	a2, _ := os.Stat(filepath.Join(d2, "photos", "a.jpg"))
	if !os.SameFile(a1, a2) {
		t.Fatalf("a.jpg 未变，应硬链接共享 inode，但未共享")
	}
	// b 变更 → 不复用旧 inode，且内容已更新
	b1, _ := os.Stat(filepath.Join(d1, "photos", "b.jpg"))
	b2, _ := os.Stat(filepath.Join(d2, "photos", "b.jpg"))
	if os.SameFile(b1, b2) {
		t.Fatalf("b.jpg 已变更，不应共享 inode")
	}
	bd, _ := os.ReadFile(filepath.Join(d2, "photos", "b.jpg"))
	if string(bd) != "BBBB2" {
		t.Fatalf("b.jpg 内容未更新: %s", bd)
	}
	// c 新增 → 存在
	if _, err := os.Stat(filepath.Join(d2, "photos", "c.jpg")); err != nil {
		t.Fatalf("c.jpg 应被复制: %v", err)
	}

	// day3: 删除 c → d3 不应包含 c（快照真实反映当天状态）
	os.Remove(filepath.Join(photos, "c.jpg"))
	d3 := filepath.Join(media, "20260902")
	if err := snapshotDir(upload, d2, d3); err != nil {
		t.Fatal("snapshot d3:", err)
	}
	if _, err := os.Stat(filepath.Join(d3, "photos", "c.jpg")); err == nil {
		t.Fatalf("c.jpg 已删除，d3 不应包含")
	}
	// a 仍共享 inode
	a3, _ := os.Stat(filepath.Join(d3, "photos", "a.jpg"))
	if !os.SameFile(a1, a3) {
		t.Fatalf("a.jpg 未变，d3 应仍共享 inode")
	}
	t.Log("dedup OK: 未变硬链接共享、变更/新增正确覆盖、删除不再出现")
}
