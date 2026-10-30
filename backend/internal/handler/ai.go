package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// AIHandler 修谱助手（对接任意 OpenAI 兼容端点：本地 llama.cpp/Ollama/vLLM 与云端服务）
type AIHandler struct {
	DB *sql.DB
}

// 默认配置：本地 llama.cpp llama-server（provider=local）与 火山方舟（Ark，存量兼容）
const (
	defaultAIBaseURL = "http://127.0.0.1:18110"
	defaultAIModel   = "qwen2.5-3b"

	defaultAIArkBaseURL = "https://ark.cn-beijing.volces.com/api/v3"
	defaultAIArkModel   = "doubao-1-5-lite-32k-250115"

	providerLocal = "local"
	providerArk   = "ark"

	// 默认生成参数（settings 键 ai_max_tokens / ai_timeout_sec）
	defaultAIMaxTokens  = 2048
	defaultAITimeoutSec = 180
	maxAIMaxTokens      = 32768
	maxAITimeoutSec     = 3600

	// defaultChatMaxTokens 现有对话/润色端点的输出上限（保持原行为不变）
	defaultChatMaxTokens = 1024
)

// AISettings AI 配置（可选供应商预设 + 可覆盖地址/模型/密钥 + 生成参数）
type AISettings struct {
	Enabled    bool
	Provider   string // 预设 id：local|ollama|vllm|llamacpp|openai|deepseek|dashscope|ark|custom（也允许自定义标识）
	BaseURL    string
	Model      string
	APIKey     string
	Path       string // 可选：chat/completions 后缀覆盖（settings 键 ai_path）
	MaxTokens  int
	TimeoutSec int
}

// endpoint 按统一规则返回 OpenAI 兼容的 chat/completions 完整地址
func (s AISettings) endpoint() string {
	return endpointURL(s.BaseURL, s.Path)
}

// timeout 返回 HTTP 超时（秒，未配置时用默认 180）
func (s AISettings) timeout() time.Duration {
	sec := s.TimeoutSec
	if sec <= 0 {
		sec = defaultAITimeoutSec
	}
	return time.Duration(sec) * time.Second
}

// aiSystemPrompt 修谱助手系统提示词（含红线：不编造事实数据）
const aiSystemPrompt = `你是「族谱修谱助手」，专门协助修撰族谱。你只做以下四类工作：
① 校验：检查用户提供文字中的世系接续、生卒年月、父子年龄、辈分等是否矛盾或可疑；
② 语病提醒：指出病句、错别字、用词不当、标点问题；
③ 语言美化：把谱序、传记、小传等内容润色为庄重、典雅的文言或书面语；
④ 修谱建议：就体例、内容、资料补全等给出专业建议。

【红线】严禁编造任何事实数据：不得虚构人名、世系、生卒年月、配偶、籍贯、堂号、字辈等。一切事实必须基于用户提供的文本；凡是不确定的必须明确指出，不得杜撰。回答一律使用简体中文，语言庄重、简洁。`

// modePrompt 根据模式生成附加指令
func modePrompt(mode string) string {
	switch mode {
	case "check":
		return "请对下面这段族谱文字做「校验」：重点检查世系接续是否连贯、生卒年月是否矛盾、父子/辈分年龄是否合理。逐条列出可疑之处，没问题的项可简要说明。"
	case "grammar":
		return "请对下面这段族谱文字做「语病提醒」：指出错别字、病句、用词不当、标点问题，并给出修改建议。"
	case "polish":
		return "请对下面这段族谱文字做「语言美化」：润色为庄重典雅的文言/书面语，保持原意，不增删事实。先给出润色后的全文，再简要说明主要改动。"
	case "advice":
		return "请对下面这段族谱文字给出「修谱建议」：从体例、内容完整性、资料补全等角度提出专业、可操作的建议。"
	default:
		return "请协助处理下面这段族谱文字。"
	}
}

// aiSettings 读取 AI 配置：先取供应商预设默认值，再用 settings 表中的非空值覆盖。
// 这样既支持任意 OpenAI 兼容端点，也保证存量 provider=local|ark 的行为不变。
func (h *AIHandler) aiSettings() AISettings {
	s := AISettings{
		Provider:   providerLocal,
		BaseURL:    defaultAIBaseURL,
		Model:      defaultAIModel,
		MaxTokens:  defaultAIMaxTokens,
		TimeoutSec: defaultAITimeoutSec,
	}

	// 预设打底（provider 缺失/为空时按 local 处理）
	var p string
	h.DB.QueryRow(`SELECT value FROM settings WHERE key='ai_provider'`).Scan(&p)
	if p = strings.TrimSpace(p); p != "" {
		s.Provider = p
	}
	if preset, ok := presetByID(s.Provider); ok {
		if preset.BaseURL != "" {
			s.BaseURL = preset.BaseURL
		}
		if preset.Model != "" {
			s.Model = preset.Model
		}
	}

	var en string
	h.DB.QueryRow(`SELECT value FROM settings WHERE key='ai_assistant_enabled'`).Scan(&en)
	s.Enabled = en == "1" || strings.EqualFold(en, "true")

	// DB 非空值覆盖预设
	var b string
	if h.DB.QueryRow(`SELECT value FROM settings WHERE key='ai_base_url'`).Scan(&b); strings.TrimSpace(b) != "" {
		s.BaseURL = strings.TrimSpace(b)
	}

	var m string
	if h.DB.QueryRow(`SELECT value FROM settings WHERE key='ai_model'`).Scan(&m); strings.TrimSpace(m) != "" {
		s.Model = strings.TrimSpace(m)
	}

	var k string
	h.DB.QueryRow(`SELECT value FROM settings WHERE key='ai_api_key'`).Scan(&k)
	s.APIKey = strings.TrimSpace(k)

	var path string
	h.DB.QueryRow(`SELECT value FROM settings WHERE key='ai_path'`).Scan(&path)
	s.Path = strings.TrimSpace(path)

	s.MaxTokens = h.aiIntSetting("ai_max_tokens", defaultAIMaxTokens)
	s.TimeoutSec = h.aiIntSetting("ai_timeout_sec", defaultAITimeoutSec)

	if s.BaseURL == "" {
		s.BaseURL = defaultAIBaseURL
	}
	if s.Model == "" {
		s.Model = defaultAIModel
	}
	return s
}

// aiIntSetting 读取整型配置（缺失或非法时返回默认值）
func (h *AIHandler) aiIntSetting(key string, def int) int {
	var v string
	if err := h.DB.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v); err == nil {
		if n, e := strconv.Atoi(strings.TrimSpace(v)); e == nil && n > 0 {
			return n
		}
	}
	return def
}

// setSetting 写入 settings 表（不存在则插入，存在则覆盖）
func (h *AIHandler) setSetting(key, value string) {
	h.DB.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`, key, value)
}

// configPayload 组装 AI 配置返回体（GET / PUT /ai/config 共用）
func (h *AIHandler) configPayload(s AISettings) gin.H {
	return gin.H{
		"enabled":          s.Enabled,
		"provider":         s.Provider,
		"base_url":         s.BaseURL,
		"model":            s.Model,
		"api_key_masked":   maskAPIKey(s.APIKey),
		"has_api_key":      s.APIKey != "",
		"max_tokens":       s.MaxTokens,
		"timeout_sec":      s.TimeoutSec,
		"path":             s.Path,
		"provider_presets": aiProviderPresets(),
	}
}

// maskAPIKey 打码显示 API Key（如 sk-****abcd）
func maskAPIKey(k string) string {
	if k == "" {
		return ""
	}
	if len(k) <= 8 {
		return "****"
	}
	return k[:4] + "****" + k[len(k)-4:]
}

// GetStatus 助手可用状态（理事长及以上可见，不暴露完整地址与密钥）
// GET /api/ai/status
func (h *AIHandler) GetStatus(c *gin.Context) {
	s := h.aiSettings()
	c.JSON(http.StatusOK, gin.H{"enabled": s.Enabled, "model": s.Model, "provider": s.Provider})
}

// GetConfig 读取 AI 配置（理事长及以上，API Key 打码返回）
// GET /api/ai/config
func (h *AIHandler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, h.configPayload(h.aiSettings()))
}

// UpdateConfig 更新 AI 配置（理事长及以上）
// PUT /api/ai/config
// 语义：未提供的字段不改；clear_api_key=true 时清空 API Key。
func (h *AIHandler) UpdateConfig(c *gin.Context) {
	var body struct {
		Enabled     *bool   `json:"enabled"`
		Provider    *string `json:"provider"`
		BaseURL     *string `json:"base_url"`
		Model       *string `json:"model"`
		APIKey      *string `json:"api_key"`
		ClearAPIKey bool    `json:"clear_api_key"`
		MaxTokens   *int    `json:"max_tokens"`
		TimeoutSec  *int    `json:"timeout_sec"`
		Path        *string `json:"path"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cur := h.aiSettings()

	if body.Enabled != nil {
		v := "0"
		if *body.Enabled {
			v = "1"
		}
		h.setSetting("ai_assistant_enabled", v)
	}

	if body.Provider != nil {
		if p := strings.TrimSpace(*body.Provider); p != "" {
			h.setSetting("ai_provider", p)
			// 切换供应商且未同时给出地址/模型时，用该预设默认值填充
			if p != cur.Provider {
				if preset, ok := presetByID(p); ok {
					if body.BaseURL == nil || strings.TrimSpace(*body.BaseURL) == "" {
						if preset.BaseURL != "" {
							h.setSetting("ai_base_url", preset.BaseURL)
						}
					}
					if body.Model == nil || strings.TrimSpace(*body.Model) == "" {
						if preset.Model != "" {
							h.setSetting("ai_model", preset.Model)
						}
					}
				}
			}
		}
	}
	if body.BaseURL != nil {
		if v := strings.TrimSpace(*body.BaseURL); v != "" {
			h.setSetting("ai_base_url", v)
		}
	}
	if body.Model != nil {
		if v := strings.TrimSpace(*body.Model); v != "" {
			h.setSetting("ai_model", v)
		}
	}
	if body.Path != nil {
		// ai_path 允许清空（清空即回到统一拼接规则）
		h.setSetting("ai_path", strings.TrimSpace(*body.Path))
	}
	if body.MaxTokens != nil && *body.MaxTokens > 0 {
		h.setSetting("ai_max_tokens", strconv.Itoa(clampInt(*body.MaxTokens, 256, maxAIMaxTokens)))
	}
	if body.TimeoutSec != nil && *body.TimeoutSec > 0 {
		h.setSetting("ai_timeout_sec", strconv.Itoa(clampInt(*body.TimeoutSec, 5, maxAITimeoutSec)))
	}
	if body.ClearAPIKey {
		h.setSetting("ai_api_key", "")
	}
	if body.APIKey != nil {
		if v := strings.TrimSpace(*body.APIKey); v != "" {
			h.setSetting("ai_api_key", v)
		}
	}

	c.JSON(http.StatusOK, h.configPayload(h.aiSettings()))
}

// clampInt 把 n 限制在 [lo, hi] 区间
func clampInt(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

// Chat 修谱助手对话（理事长及以上，需先启用）
// POST /api/ai/chat  body: {"mode":"check|grammar|polish|advice","content":"..."}
func (h *AIHandler) Chat(c *gin.Context) {
	s := h.aiSettings()
	if !s.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "修谱助手未启用，请先到系统设置中开启并配置模型"})
		return
	}

	var body struct {
		Mode    string `json:"mode"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	content := strings.TrimSpace(body.Content)
	if content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入要处理的文字"})
		return
	}
	if len(content) > 20000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文字过长（最多 20000 字），请分段处理"})
		return
	}

	messages := []map[string]string{
		{"role": "system", "content": aiSystemPrompt},
		{"role": "user", "content": modePrompt(body.Mode) + "\n\n" + content},
	}

	reply, err := callModel(s, messages)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if reply == "" {
		reply = "（模型未返回有效内容，请稍后重试）"
	}
	c.JSON(http.StatusOK, gin.H{"reply": reply})
}

// ValidateSave 保存前校验（理事长及以上，需已启用）
// POST /api/ai/validate-save  body: {"content":"待校验的人物资料文本"}
// 返回: {"ok":bool,"problems":[]string}
func (h *AIHandler) ValidateSave(c *gin.Context) {
	s := h.aiSettings()
	if !s.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "修谱助手未启用"})
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	content := strings.TrimSpace(body.Content)
	if content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内容为空"})
		return
	}
	if len(content) > 20000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内容过长"})
		return
	}

	const validatePrompt = `你是「族谱修谱助手」，正在做保存前校验。下面是一份待保存的族谱人物资料（可能是族人、配偶或子女）。
只检查「姓名缺漏」一项：姓或名为空。
注意：姓名一栏若已同时包含姓和名（例如「李小明」「王氏」），视为完整，不要报告缺漏。其余内容（生卒日期、世代、年龄等）一律不检查、不报告。
【输出要求】只输出一个 JSON 对象，不要输出任何解释或其他文字，格式如下：
没有问题：{"ok": true, "problems": []}
有问题：{"ok": false, "problems": ["问题1", "问题2"]}`

	messages := []map[string]string{
		{"role": "system", "content": "你是族谱修谱助手，负责保存前校验。只检查姓名是否完整，不检查其他内容，不编造事实。"},
		{"role": "user", "content": validatePrompt + "\n\n待校验资料：\n" + content},
	}

	reply, err := callModel(s, messages)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ok, problems := parseValidateReply(reply)
	c.JSON(http.StatusOK, gin.H{"ok": ok, "problems": problems})
}

// PolishBio 生平条目「病句排查 + 语言美化」（理事长及以上，需已启用）
// POST /api/ai/polish-bio  body: {"title":"...","content":"..."}
// 返回: {"title":"润色后标题","content":"润色后内容"}
func (h *AIHandler) PolishBio(c *gin.Context) {
	s := h.aiSettings()
	if !s.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "修谱助手未启用"})
		return
	}

	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Level   int    `json:"level"`  // 约束等级 0~100（越低约束越严）
		Degree  int    `json:"degree"` // 润色程度 0~100（越高改动越大）
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	content := strings.TrimSpace(body.Content)
	if content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入要优化的内容"})
		return
	}
	if len(content) > 20000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内容过长"})
		return
	}
	title := strings.TrimSpace(body.Title)

	const bioPrompt = `你是「族谱修谱助手」，正在润色一条族谱生平条目。
请做「语病排查 + 语言美化」：只允许修正错别字、病句、用词不当、标点问题，并把语序、用词调整为庄重典雅的书面语。

【铁律】
1. 文本中出现形如 __TITLE_n__、__CONTENT_n__ 的占位符，是系统预留下的原文字段（日期/纪年/数字），必须原样保留、一字不改、不删、不增、不换算，原位照抄；
2. 所有人名、地名、称谓一律不改；
3. 不得增删任何事实，不得发挥、不得凭空补写。

同时给出一条更简洁得体的条目标题。

【输出要求】只输出一个 JSON 对象，不要输出任何解释或其他文字，格式如下：
{"title":"润色后的标题","content":"润色后的内容"}
若标题无需改动，title 照抄原标题；若内容无需改动，content 照抄原内容。`

	// 动态约束与润色程度
	prompt := bioPrompt + "\n\n【约束等级】" + constraintLine(body.Level)
	prompt += "\n【润色程度】" + degreeLine(body.Degree)
	temperature := degreeTemp(body.Degree)

	// 发送前掩码：把日期/纪年换成占位符，AI 看不到原文就改不了
	maskedTitle, titleDates := maskDates("TITLE", title)
	maskedContent, contentDates := maskDates("CONTENT", content)

	userPrompt := ""
	if title != "" {
		userPrompt = "条目标题：" + maskedTitle + "\n"
	}
	userPrompt += "条目内容：" + maskedContent
	messages := []map[string]string{
		{"role": "system", "content": aiSystemPrompt},
		{"role": "user", "content": prompt + "\n\n" + userPrompt},
	}

	reply, err := callModelWithTemp(s, messages, temperature)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	pt, pc := parseBioPolish(reply, title, content)
	// 校验：AI 必须把所有占位符原样保留齐全，缺一即退回原文
	if !maskIntact(pt, "TITLE", len(titleDates)) || !maskIntact(pc, "CONTENT", len(contentDates)) {
		c.JSON(http.StatusOK, gin.H{"title": title, "content": content, "protected": true})
		return
	}
	// 还原占位符为原文日期
	newPt := unmaskDates(pt, "TITLE", titleDates)
	newPc := unmaskDates(pc, "CONTENT", contentDates)
	// 二次兜底：还原后仍比对年份级改动
	guarded := false
	if guardDates(title, newPt) != newPt {
		newPt = title
		guarded = true
	}
	if guardDates(content, newPc) != newPc {
		newPc = content
		guarded = true
	}
	c.JSON(http.StatusOK, gin.H{"title": newPt, "content": newPc, "protected": guarded})
}

// callModel 调用模型（默认温度 0.7，输出上限 1024）
func callModel(s AISettings, messages []map[string]string) (string, error) {
	return callModelLimit(s, messages, 0.7, defaultChatMaxTokens)
}

// callModelWithTemp 指定温度调用模型（温度越低越忠实原文，适合润色/日期敏感场景）
func callModelWithTemp(s AISettings, messages []map[string]string, temperature float64) (string, error) {
	return callModelLimit(s, messages, temperature, defaultChatMaxTokens)
}

// callModelLimit 指定温度与输出上限调用模型（任意 OpenAI 兼容 /chat/completions 端点）
func callModelLimit(s AISettings, messages []map[string]string, temperature float64, maxTokens int) (string, error) {
	if maxTokens <= 0 {
		maxTokens = defaultChatMaxTokens
	}
	reqBody := map[string]interface{}{
		"model":       s.Model,
		"messages":    messages,
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"stream":      false,
	}
	payload, _ := json.Marshal(reqBody)

	req, err := http.NewRequest(http.MethodPost, s.endpoint(), bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("构建请求失败：%v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// 鉴权：任意供应商只要配置了 API Key 就下发 Bearer；本地无 key 部署则不带鉴权头
	if secret := strings.TrimSpace(s.APIKey); secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}

	client := &http.Client{Timeout: s.timeout()}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("无法连接模型服务：%v", err)
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", fmt.Errorf("读取模型返回失败：%v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("模型服务返回错误（HTTP %d）：%s", resp.StatusCode, firstLine(respBytes))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", fmt.Errorf("解析模型返回失败：%v", err)
	}
	reply := ""
	if len(parsed.Choices) > 0 {
		reply = strings.TrimSpace(parsed.Choices[0].Message.Content)
	}
	if reply == "" {
		return "", fmt.Errorf("模型未返回有效内容")
	}
	return reply, nil
}

// constraintLine 根据约束等级生成提示词（等级越低越保守，只修错别字标点；越高越允许调整用词句式）
func constraintLine(level int) string {
	switch {
	case level <= 0:
		return "最严格：只修正错别字、错别标点，不改动任何用词、语序、句式，不得增删字句。"
	case level <= 25:
		return "严格：只修正错别字、标点与明显病句，用词、语序、句式尽量保持原样。"
	case level <= 50:
		return "适中：修正错别字、标点、病句，可微调用词，语序句式基本保持。"
	case level <= 75:
		return "较宽松：可调整用词、语序，使表达更通顺得体，但不增删事实、不改原意。"
	default:
		return "宽松：在保留事实与原意的前提下，可适当调整句式、语序、用词，使行文更典雅流畅。"
	}
}

// degreeLine 根据润色程度生成提示词（程度越高越允许改写幅度大）
func degreeLine(degree int) string {
	switch {
	case degree <= 0:
		return "尽量不做美化，只做必要的纠错，输出尽可能贴近原文。"
	case degree <= 25:
		return "轻度美化：在保留原句的基础上小幅润色。"
	case degree <= 50:
		return "中度美化：适当调整用词与句式，使文字更庄重得体。"
	case degree <= 75:
		return "较高美化：较明显地优化用词、句式，追求典雅书面语。"
	default:
		return "重度美化：大幅润色改写，在保留原意与事实的前提下使文字更典雅、更简洁。"
	}
}

// degreeTemp 根据润色程度映射采样温度：0→0.1 … 100→0.9（润色越大越允许发挥）
func degreeTemp(degree int) float64 {
	if degree < 0 {
		degree = 0
	}
	if degree > 100 {
		degree = 100
	}
	return 0.1 + 0.008*float64(degree)
}

// firstLine 取错误响应体首行，便于排查（火山方舟/模型服务返回的 JSON 错误）
func firstLine(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}

// parseValidateReply 解析模型返回的校验结果；解析失败时用启发式兜底
func parseValidateReply(reply string) (bool, []string) {
	s := strings.TrimSpace(reply)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	// 提取 JSON 对象
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		var vr struct {
			OK       bool     `json:"ok"`
			Problems []string `json:"problems"`
		}
		if err := json.Unmarshal([]byte(s[start:end+1]), &vr); err == nil {
			return vr.OK, vr.Problems
		}
	}

	// 启发式兜底：模型没输出 JSON 时，根据文本判断
	lower := strings.ToLower(reply)
	if strings.Contains(lower, "无问题") || strings.Contains(lower, "没有问题") ||
		strings.Contains(lower, "未发现") || strings.Contains(lower, "无不妥") ||
		strings.Contains(lower, "无误") || strings.Contains(lower, "未发现明显") {
		return true, nil
	}
	return false, []string{reply}
}

// parseBioPolish 解析模型返回的 {title,content}；解析失败时回退：标题用原标题，内容用全文
func parseBioPolish(reply, origTitle, origContent string) (string, string) {
	s := strings.TrimSpace(reply)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		var pr struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(s[start:end+1]), &pr); err == nil {
			t := strings.TrimSpace(pr.Title)
			c := strings.TrimSpace(pr.Content)
			if c == "" {
				c = origContent
			}
			if t == "" {
				t = origTitle
			}
			return t, c
		}
	}
	return origTitle, strings.TrimSpace(reply)
}

// dateMaskRe 匹配需要原样保护的「日期/纪年」片段：
// 公历（2026年8月31日 / 2026-08-31 / 2026年 / 8月31日 / 8月 / 裸四位年份），
// 年号纪年（光绪三十四年 / 民国38年），干支纪年（丙午年）
var dateMaskRe = regexp.MustCompile(
	`\d{4}\s*年\s*\d{1,2}\s*月(?:\s*\d{1,2}\s*日)?` + // 2026年8月31日
		`|\d{4}\s*[-/.]\s*\d{1,2}\s*[-/.]\s*\d{1,2}(?:\s*日)?` + // 2026-08-31
		`|\d{4}\s*年` + // 2026年
		`|(?:光绪|宣统|同治|咸丰|嘉庆|乾隆|雍正|康熙|顺治|崇祯|万历|隆庆|嘉靖|正德|弘治|成化|天顺|景泰|正统|宣德|洪熙|永乐|建文|洪武|民国|共和)\s*[一二三四五六七八九十百零]+\s*年` + // 光绪三十四年
		`|(?:光绪|宣统|同治|咸丰|嘉庆|乾隆|雍正|康熙|顺治|崇祯|万历|隆庆|嘉靖|正德|弘治|成化|天顺|景泰|正统|宣德|洪熙|永乐|建文|洪武|民国|共和)\s*\d{1,3}\s*年` + // 民国38年
		`|[甲乙丙丁戊己庚辛壬癸][子丑寅卯辰巳午未申酉戌亥]\s*年` + // 丙午年
		`|\d{1,2}\s*月\s*\d{1,2}\s*日` + // 8月31日
		`|\d{1,2}\s*月` + // 8月
		`|\d{4,}`, // 裸四位年份 2026
)

// maskDates 把文本中的日期/纪年片段替换为 <prefix>_n 占位符，返回掩码文本与原文片段
func maskDates(prefix, text string) (string, []string) {
	locs := dateMaskRe.FindAllStringIndex(text, -1)
	if len(locs) == 0 {
		return text, nil
	}
	var sb strings.Builder
	var originals []string
	last := 0
	for i, loc := range locs {
		sb.WriteString(text[last:loc[0]])
		sb.WriteString(fmt.Sprintf("__%s_%d__", prefix, i+1))
		originals = append(originals, text[loc[0]:loc[1]])
		last = loc[1]
	}
	sb.WriteString(text[last:])
	return sb.String(), originals
}

// unmaskDates 把 <prefix>_n 占位符还原为原文日期片段
func unmaskDates(masked, prefix string, originals []string) string {
	for i, orig := range originals {
		masked = strings.ReplaceAll(masked, fmt.Sprintf("__%s_%d__", prefix, i+1), orig)
	}
	return masked
}

// maskIntact 校验所有占位符是否原样齐全（个数必须正好等于 n，不多不少，且序号恰好 1..n 各一次）
func maskIntact(s, prefix string, n int) bool {
	re := regexp.MustCompile(`__` + prefix + `_(\d+)__`)
	ms := re.FindAllStringSubmatch(s, -1)
	if len(ms) != n {
		return false
	}
	if n == 0 {
		return len(ms) == 0
	}
	seen := make(map[int]bool)
	for _, m := range ms {
		var idx int
		fmt.Sscanf(m[1], "%d", &idx)
		if idx < 1 || idx > n || seen[idx] {
			return false
		}
		seen[idx] = true
	}
	return len(seen) == n
}

// guardDates 防御性兜底（降级版）：只保护「年份」（4 位及以上数字）。
// 原文没有年份则不设防；原文有年份且模型改动/删除/新增了年份才退回原文。
// 月份、日期、排行、世代等小数字的微调不再触发，避免把正常润色（如“第三”→“3”）误判为改日期。
func guardDates(orig, polished string) string {
	po := strings.TrimSpace(polished)
	if po == "" {
		return orig
	}
	oy := extractYears(orig)
	if oy == "" {
		return po // 原文无年份，不设防
	}
	if oy == extractYears(po) {
		return po
	}
	return orig
}

// extractYears 抽取文本中所有 4 位及以上数字（视为年份），排序后拼接用于比较
func extractYears(s string) string {
	re := regexp.MustCompile(`\d{4,}`)
	yrs := re.FindAllString(s, -1)
	sort.Strings(yrs)
	return strings.Join(yrs, "|")
}
