package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"familytree/internal/config"

	"github.com/gin-gonic/gin"
)

// KBHandler 传承服务器（家族知识库）反代 + 健康探测
type KBHandler struct {
	DB  *sql.DB
	Cfg *config.Config
	km  kbModeCache
}

// kbModeCache 模式探测缓存（决定是否注入 SSO 身份）
type kbModeCache struct {
	mu     sync.Mutex
	last   time.Time
	online bool
	inject bool // true=族谱登录(SSO注入)；false=知识库自带登录(透明反代)
	info   map[string]any
}

func (h *KBHandler) enabled() bool { return strings.TrimSpace(h.Cfg.KBURL) != "" }

func kbBase() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

// ping 探测传承服务器 /api/health 是否在线
func (h *KBHandler) ping() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(h.Cfg.KBURL, "/")+"/api/health", nil)
	resp, err := kbBase().Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// kbProbe 决定登录源（缓存 20s）：
//   - FAMILY_TREE_KB_LOGIN_MODE=zupu  → 族谱登录（注入 SSO）
//   - =local                          → 知识库自带登录（透明反代）
//   - =auto（默认）                   → 跟随知识库上报的模式（知识库按“先部署为准”自行决定）
func (h *KBHandler) kbProbe() (online bool, inject bool, info map[string]any) {
	h.km.mu.Lock()
	defer h.km.mu.Unlock()
	if !h.km.last.IsZero() && time.Since(h.km.last) < 20*time.Second {
		return h.km.online, h.km.inject, h.km.info
	}
	h.km.last = time.Now()
	h.km.online, h.km.inject, h.km.info = false, true, nil
	if !h.enabled() {
		return false, true, nil
	}

	switch strings.ToLower(strings.TrimSpace(h.Cfg.KBLoginMode)) {
	case "zupu", "integrated":
		h.km.online, h.km.inject = h.ping(), true
		return h.km.online, true, nil
	case "local", "kb":
		h.km.online, h.km.inject = h.ping(), false
		return h.km.online, false, nil
	}

	// auto：问知识库 /api/kb/mode
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(h.Cfg.KBURL, "/")+"/api/kb/mode", nil)
	resp, err := kbBase().Do(req)
	if err != nil {
		return false, true, nil
	}
	defer resp.Body.Close()
	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	h.km.online = resp.StatusCode == 200
	h.km.info = m
	// 知识库上报模式：integrated => 注入 SSO；standalone/未知 => 透明反代
	if md, ok := m["mode"].(string); ok && md == "standalone" {
		h.km.inject = false
	} else {
		h.km.inject = true
	}
	return h.km.online, h.km.inject, h.km.info
}

// kbClaims 与传承服务器对齐的 JWT 载荷
type kbClaims struct {
	Sub    int64  `json:"sub"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Tenant string `json:"tenant"`
	Exp    int64  `json:"exp"`
}

func kbSign(secret string, c kbClaims) string {
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	p, _ := json.Marshal(c)
	body := h + "." + base64.RawURLEncoding.EncodeToString(p)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Status 探测传承服务器是否在线 + 登录源模式（族谱据此门控菜单）
// GET /api/kb/status
func (h *KBHandler) Status(c *gin.Context) {
	if !h.enabled() {
		c.JSON(http.StatusOK, gin.H{"enabled": false, "online": false})
		return
	}
	online, inject, info := h.kbProbe()
	mode := "local"
	if inject {
		mode = "zupu"
	}
	c.JSON(http.StatusOK, gin.H{"enabled": true, "online": online, "mode": mode, "login": !inject, "info": info})
}

var hopHeaders = map[string]bool{
	"Connection": true, "Keep-Alive": true, "Proxy-Authenticate": true, "Proxy-Authorization": true,
	"Te": true, "Trailer": true, "Transfer-Encoding": true, "Upgrade": true,
}

// Proxy 反代 /knowledge/* → 传承服务器
//   - inject=true：复用族谱登录（校族谱 token → 签短时 JWT 注入 X-KB-Auth）
//   - inject=false：透明反代（传承服务器用自带登录，族谱不注入身份）
//
// 支持通过 ?token= 首次进入并落 Cookie（仅 SSO 模式需要）
func (h *KBHandler) Proxy(c *gin.Context) {
	if !h.enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "知识库未启用"})
		return
	}
	_, inject, _ := h.kbProbe()

	jwt := ""
	if inject {
		tok := c.GetHeader("Authorization")
		tok = strings.TrimSpace(strings.TrimPrefix(tok, "Bearer "))
		if tok == "" {
			tok = c.Query("token")
		}
		if tok == "" {
			tok, _ = c.Cookie("kb_token")
		}
		var uid int
		var uname, role string
		err := h.DB.QueryRow(`SELECT id, username, role FROM users WHERE token = ? AND token != ''`, tok).
			Scan(&uid, &uname, &role)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			return
		}
		// 首次带 token 进入：落 Cookie，供 iframe 子请求（无自定义头）续用
		if c.Query("token") != "" {
			c.SetCookie("kb_token", tok, 8*3600, "/knowledge", "", false, true)
		}
		jwt = kbSign(h.Cfg.KBSSOSecret, kbClaims{Sub: int64(uid), Name: uname, Role: role, Exp: time.Now().Add(time.Hour).Unix()})
	}

	path := strings.TrimPrefix(c.Request.URL.Path, "/knowledge")
	if path == "" {
		path = "/"
	}
	target := strings.TrimRight(h.Cfg.KBURL, "/") + path
	if c.Request.URL.RawQuery != "" {
		target += "?" + c.Request.URL.RawQuery
	}

	var body io.Reader
	if c.Request.Body != nil {
		body = c.Request.Body
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, target, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for k, vs := range c.Request.Header {
		if hopHeaders[k] {
			continue
		}
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	if inject {
		req.Header.Set("X-KB-Auth", jwt)
	}
	req.Host = "" // 由 http.Client 依 target 设置

	resp, err := kbBase().Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "传承服务器不可达：" + err.Error()})
		return
	}
	defer resp.Body.Close()
	for k, vs := range resp.Header {
		if hopHeaders[k] {
			continue
		}
		for _, v := range vs {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}
