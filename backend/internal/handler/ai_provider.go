package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// providerPreset 供应商预设（前端「一键选择」用；needs_key=false 表示本地部署通常无需密钥）
type providerPreset struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	BaseURL  string `json:"base_url"`
	Model    string `json:"model"`
	NeedsKey bool   `json:"needs_key"`
	Hint     string `json:"hint"`
}

// aiProviderPresets 返回全部内置供应商预设（本地部署 + 云端 OpenAI 兼容端点 + 自定义）
func aiProviderPresets() []providerPreset {
	return []providerPreset{
		{ID: providerLocal, Label: "本地 llama.cpp (llama-server)", BaseURL: defaultAIBaseURL, Model: defaultAIModel,
			NeedsKey: false, Hint: "本地部署，无需密钥"},
		{ID: "ollama", Label: "本地 Ollama", BaseURL: "http://127.0.0.1:11434/v1", Model: "qwen2.5:7b",
			NeedsKey: false, Hint: "先执行 ollama pull qwen2.5:7b，无需密钥"},
		{ID: "vllm", Label: "本地 vLLM", BaseURL: "http://127.0.0.1:8000/v1", Model: "Qwen/Qwen2.5-7B-Instruct",
			NeedsKey: false, Hint: "vLLM 需以 --served-model-name 指定模型名，通常无需密钥"},
		{ID: "llamacpp", Label: "本地 llama.cpp 其它端口", BaseURL: "http://127.0.0.1:8080/v1", Model: defaultAIModel,
			NeedsKey: false, Hint: "llama-server 默认 8080 端口，无需密钥"},
		{ID: "openai", Label: "OpenAI", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini",
			NeedsKey: true, Hint: "需填写 OpenAI API Key（sk- 开头）"},
		{ID: "deepseek", Label: "DeepSeek", BaseURL: "https://api.deepseek.com/v1", Model: "deepseek-chat",
			NeedsKey: true, Hint: "需填写 DeepSeek API Key"},
		{ID: "dashscope", Label: "阿里通义(DashScope 兼容模式)", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Model: "qwen-plus",
			NeedsKey: true, Hint: "需填写阿里云百炼 API Key（sk- 开头）"},
		{ID: providerArk, Label: "火山方舟(Ark)", BaseURL: defaultAIArkBaseURL, Model: defaultAIArkModel,
			NeedsKey: true, Hint: "需填写方舟 API Key；model 可填接入点 ID（ep- 开头）"},
		{ID: "custom", Label: "自定义(OpenAI 兼容)", BaseURL: "", Model: "",
			NeedsKey: false, Hint: "任意 OpenAI 兼容端点，填写 Base URL 与模型名即可"},
	}
}

// presetByID 按 id 查预设
func presetByID(id string) (providerPreset, bool) {
	id = strings.TrimSpace(id)
	for _, p := range aiProviderPresets() {
		if p.ID == id {
			return p, true
		}
	}
	return providerPreset{}, false
}

// normalizeAIPath 归一化自定义后缀（ai_path），保证以 "/" 开头
func normalizeAIPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// endpointURL 按统一规则拼接 chat/completions 地址：
//  1. 配置了 ai_path 时用 base + path 覆盖；
//  2. base 已以 /chat/completions 结尾 → 原样；
//  3. base 以 /v1 或 /api/v3 结尾（含 .../compatible-mode/v1）→ + /chat/completions；
//  4. 其它 → + /v1/chat/completions（本地 llama-server 等）。
//
// 注意：火山方舟的 base 是 /api/v3，不能盲目加 /v1。
func endpointURL(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return ""
	}
	if np := normalizeAIPath(path); np != "" {
		return base + np
	}
	lower := strings.ToLower(base)
	if strings.HasSuffix(lower, "/chat/completions") {
		return base
	}
	if strings.HasSuffix(lower, "/v1") || strings.HasSuffix(lower, "/api/v3") {
		return base + "/chat/completions"
	}
	return base + "/v1/chat/completions"
}

// modelsURL 推导 GET /models 探测地址（用于「测试连接」）
func modelsURL(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return ""
	}
	if np := normalizeAIPath(path); np != "" {
		base = strings.TrimSuffix(base, np)
	}
	lower := strings.ToLower(base)
	if strings.HasSuffix(lower, "/chat/completions") {
		base = strings.TrimRight(base[:len(base)-len("/chat/completions")], "/")
		lower = strings.ToLower(base)
	}
	if strings.HasSuffix(lower, "/v1") || strings.HasSuffix(lower, "/api/v3") {
		return base + "/models"
	}
	return base + "/v1/models"
}

// Test 探测 AI 服务可用性（理事长及以上）
// POST /api/ai/test  → {"ok":bool,"message":string,"models":[]string}
// 失败一律返回 ok=false + 简短原因，不抛 500。
func (h *AIHandler) Test(c *gin.Context) {
	// 允许请求体覆盖当前配置（便于「填完先测再存」），缺省用已保存配置
	var body struct {
		Provider *string `json:"provider"`
		BaseURL  *string `json:"base_url"`
		Model    *string `json:"model"`
		APIKey   *string `json:"api_key"`
		Path     *string `json:"path"`
	}
	_ = c.ShouldBindJSON(&body)

	s := h.aiSettings()
	if body.Provider != nil && strings.TrimSpace(*body.Provider) != "" {
		p := strings.TrimSpace(*body.Provider)
		s.Provider = p
		// 只给了供应商时用预设填充地址/模型（显式入参优先）
		if preset, ok := presetByID(p); ok {
			if body.BaseURL == nil || strings.TrimSpace(*body.BaseURL) == "" {
				if preset.BaseURL != "" {
					s.BaseURL = preset.BaseURL
				}
			}
			if body.Model == nil || strings.TrimSpace(*body.Model) == "" {
				if preset.Model != "" {
					s.Model = preset.Model
				}
			}
		}
	}
	if body.BaseURL != nil && strings.TrimSpace(*body.BaseURL) != "" {
		s.BaseURL = strings.TrimSpace(*body.BaseURL)
	}
	if body.Model != nil && strings.TrimSpace(*body.Model) != "" {
		s.Model = strings.TrimSpace(*body.Model)
	}
	// 密钥/后缀按「显式给出即生效」处理（允许传空串表示本次不带密钥）
	if body.APIKey != nil {
		s.APIKey = strings.TrimSpace(*body.APIKey)
	}
	if body.Path != nil {
		s.Path = strings.TrimSpace(*body.Path)
	}

	url := modelsURL(s.BaseURL, s.Path)
	if url == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "未配置服务地址（Base URL）", "models": []string{}})
		return
	}

	ok, message, models := probeModels(url, s.APIKey)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": message, "models": []string{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": message, "models": models})
}

// probeModels 请求 {base}/models 探测可用性（3 秒超时）
func probeModels(url, apiKey string) (bool, string, []string) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, "地址无效：" + err.Error(), nil
	}
	if key := strings.TrimSpace(apiKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, "无法连接：" + firstLine([]byte(err.Error())), nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode != http.StatusOK {
		return false, "服务返回错误（HTTP " + strconv.Itoa(resp.StatusCode) + "）：" + firstLine(body), nil
	}

	models := parseModelList(body)
	if len(models) == 0 {
		return true, "连接成功（服务未返回模型列表，请确认模型名是否正确）", models
	}
	return true, "连接成功，发现 " + strconv.Itoa(len(models)) + " 个模型", models
}

// parseModelList 兼容 {"data":[{"id":...}]} 与 {"models":[...]} 两种返回
func parseModelList(body []byte) []string {
	var a struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	models := []string{}
	if err := json.Unmarshal(body, &a); err == nil {
		for _, d := range a.Data {
			if s := strings.TrimSpace(d.ID); s != "" {
				models = append(models, s)
			}
		}
	}
	if len(models) > 0 {
		return models
	}
	var b struct {
		Models []struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &b); err == nil {
		for _, m := range b.Models {
			v := strings.TrimSpace(m.ID)
			if v == "" {
				v = strings.TrimSpace(m.Name)
			}
			if v != "" {
				models = append(models, v)
			}
		}
	}
	return models
}
