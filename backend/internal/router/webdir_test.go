package router

import (
	"os"
	"path/filepath"
	"testing"
)

// 回归测试：空目录里的 FAMILY_TREE_WEB_DIR 不能赢过真正含 index.html 的目录。
// 曾经的表现：全新数据卷 + FAMILY_TREE_WEB_DIR=/data/runtime/web（空）→ 首页 404、/health 200。
func TestResolveWebDir(t *testing.T) {
	mk := func(t *testing.T, dir string, withIndex bool) string {
		t.Helper()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if withIndex {
			if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}

	t.Run("配置目录含 index.html 时直接用", func(t *testing.T) {
		dir := mk(t, filepath.Join(t.TempDir(), "configured"), true)
		got, ok := resolveWebDir(dir)
		if !ok || got != dir {
			t.Fatalf("got (%q,%v), want (%q,true)", got, ok, dir)
		}
	})

	t.Run("空目录要让位给含 index.html 的 ./web", func(t *testing.T) {
		empty := mk(t, filepath.Join(t.TempDir(), "empty"), false)
		wd, _ := os.Getwd()
		tmp := t.TempDir()
		mk(t, filepath.Join(tmp, "web"), true)
		if err := os.Chdir(tmp); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(wd) }()

		got, ok := resolveWebDir(empty)
		if !ok {
			t.Fatalf("got ok=false, want true（应回落到 ./web）")
		}
		if filepath.Base(got) != "web" {
			t.Fatalf("got %q, want .../web", got)
		}
	})

	t.Run("哪里都没有 index.html 时返回 ok=false 并给出候选路径", func(t *testing.T) {
		empty := mk(t, filepath.Join(t.TempDir(), "empty"), false)
		wd, _ := os.Getwd()
		tmp := t.TempDir() // 空目录，没有 web/
		if err := os.Chdir(tmp); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Chdir(wd) }()

		got, ok := resolveWebDir(empty)
		if ok {
			t.Fatalf("got ok=true (%q), want false", got)
		}
		if got != empty {
			t.Fatalf("got %q, want 回落到存在的空目录 %q（便于日志暴露）", got, empty)
		}
	})
}
