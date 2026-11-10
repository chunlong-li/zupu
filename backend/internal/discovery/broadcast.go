// Package discovery 提供局域网广播发现：手机 App 发 UDP 广播，本服务应答。
//
// 协议（App 与后端约定）：
//
//	App → 广播到 255.255.255.255:45789，内容 "ZUPU_DISCOVER"
//	服务 → 回给来源地址：{"name":"族谱","version":"1.3.16","port":8080}
package discovery

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	Port  = 45789
	magic = "ZUPU_DISCOVER"
)

// Start 在后台启动 UDP 广播应答（失败只记日志，不影响主服务）。
func Start(servicePort int, version string) {
	addr := &net.UDPAddr{IP: net.IPv4zero, Port: Port}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		log.Printf("[发现] UDP 广播监听失败（%d）：%v", Port, err)
		return
	}
	go func() {
		defer conn.Close()
		buf := make([]byte, 256)
		for {
			n, src, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if string(buf[:n]) != magic {
				continue
			}
			port := advertisePort(servicePort)
			if p := observed(); p > 0 {
				port = p // 从真实请求里学到的外部端口最准
			}
			resp, _ := json.Marshal(map[string]any{
				"name":    "族谱",
				"version": version,
				"port":    port,
			})
			_, _ = conn.WriteToUDP(resp, src)
		}
	}()
}

// 记录最近一次 HTTP 请求里看到的「外部端口」——容器无法直接知道宿主机映射，
// 但请求的 Host / X-Forwarded-Port 头里带着它（例如 192.168.1.190:8888 → 8888）。
var (
	observedMu   sync.RWMutex
	observedPort int
)

// ObserveRequest 从请求头里学习对外端口（由路由中间件调用，零配置）。
func ObserveRequest(r *http.Request) {
	port := 0
	if v := r.Header.Get("X-Forwarded-Port"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			port = n
		}
	}
	if port == 0 {
		host := r.Host
		if v := r.Header.Get("X-Forwarded-Host"); v != "" {
			host = v
		}
		if i := strings.LastIndex(host, ":"); i > 0 {
			if n, err := strconv.Atoi(host[i+1:]); err == nil {
				port = n
			}
		}
	}
	if port <= 0 {
		return
	}
	observedMu.Lock()
	observedPort = port
	observedMu.Unlock()
}

// observed 返回学习到的对外端口（0 = 还没学到）
func observed() int {
	observedMu.RLock()
	defer observedMu.RUnlock()
	return observedPort
}

// advertisePort 对外可达端口：容器里监听 8080、宿主机映射成 8888 时，
// 必须告诉手机「用 8888 连」，否则手机去连 8080 连不上。
func advertisePort(fallback int) int {
	if v := os.Getenv("FAMILY_TREE_ADVERTISE_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

// Enabled 允许用环境变量关掉（FAMILY_TREE_DISCOVERY=0），默认开启。
func Enabled() bool {
	v := os.Getenv("FAMILY_TREE_DISCOVERY")
	return v != "0" && v != "false"
}
