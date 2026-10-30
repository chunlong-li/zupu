// update-gateway：开源版更新网关
// 只做三件事：健康检查、静态 manifest、更新包静态下载。
// 最小可运行：健康检查 + 静态清单 + 更新包下载。
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var (
	rootDir    = getEnv("GATEWAY_ROOT", "/data/updates")
	listenAddr = ":" + getEnv("GATEWAY_PORT", "8080")
)

func getEnv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func main() {
	log.Printf("update-gateway 启动：root=%s listen=%s", rootDir, listenAddr)
	http.HandleFunc("/", route)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

func route(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}
	rel := strings.TrimPrefix(filepath.Clean("/"+r.URL.Path), "/")
	if rel == "" || rel == "." {
		http.NotFound(w, r)
		return
	}
	full := filepath.Join(rootDir, filepath.FromSlash(rel))
	rootClean := filepath.Clean(rootDir)
	if full != rootClean && !strings.HasPrefix(full, rootClean+string(os.PathSeparator)) {
		http.Error(w, "非法路径", http.StatusBadRequest)
		return
	}
	http.ServeFile(w, r, full)
}
