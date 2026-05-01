package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"ai-workbench-api/internal/aiconfig"
	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	chatUpstreamTimeout = 120 * time.Second
)

const systemPrompt = `你是 AI WorkBench 的 SRE 问诊助手。
使用中文 Markdown，先给结论，再给关键证据、排查步骤、处置建议和下一步。
优先引用 Prometheus/Categraf、Catpaw、告警和拓扑数据；没有实时数据时说明数据缺口，不编造指标。
回答保持简洁结构化，避免冗长推理；如给命令，只提供只读检查命令。`

type aiProviderConfig struct {
	BaseURL      string   `mapstructure:"base_url"`
	BaseURLAlias string   `mapstructure:"baseurl"`
	APIKey       string   `mapstructure:"api_key"`
	APIKeyAlias  string   `mapstructure:"apikey"`
	Models       []string `mapstructure:"models"`
	Default      bool     `mapstructure:"default"`
}

func normalizeAIBaseURL(value string) string {
	u := strings.TrimRight(strings.TrimSpace(value), "/")
	return strings.TrimSuffix(u, "/chat/completions")
}

func configuredAIProviders() []aiProviderConfig {
	var providers []aiProviderConfig
	_ = viper.UnmarshalKey("ai_providers", &providers)
	for i := range providers {
		if providers[i].BaseURL == "" {
			providers[i].BaseURL = providers[i].BaseURLAlias
		}
		if providers[i].APIKey == "" {
			providers[i].APIKey = providers[i].APIKeyAlias
		}
		providers[i].BaseURL = normalizeAIBaseURL(providers[i].BaseURL)
		providers[i].APIKey = strings.TrimSpace(providers[i].APIKey)
	}
	return providers
}

func resolveDefaultModel() string {
	return aiconfig.ResolveDefaultModel()
}

func usableSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "******" || strings.Contains(value, "${") || strings.Contains(strings.ToLower(value), "your_") {
		return ""
	}
	return value
}

func getBaseURL() string {
	providers := configuredAIProviders()
	for _, p := range providers {
		if p.Default && p.BaseURL != "" {
			return p.BaseURL
		}
	}
	for _, p := range providers {
		if p.BaseURL != "" {
			return p.BaseURL
		}
	}
	return normalizeAIBaseURL(viper.GetString("ai.base_url"))
}

func getAPIKey() string {
	if key := usableSecret(os.Getenv("AI_WORKBENCH_API_KEY")); key != "" {
		return key
	}
	providers := configuredAIProviders()
	for _, p := range providers {
		if p.Default {
			if key := usableSecret(p.APIKey); key != "" {
				return key
			}
		}
	}
	for _, p := range providers {
		if key := usableSecret(p.APIKey); key != "" {
			return key
		}
	}
	return usableSecret(viper.GetString("ai.api_key"))
}

func Chat(c *gin.Context) {
	var req model.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sessionID := ensureChatSession(req)
	persistLatestUserMessage(sessionID, req)

	messages := buildChatRequestMessages(req.Messages)
	if content, ok := localChatAnswer(messages); ok {
		respondLocalChatAnswer(c, sessionID, req.Model, content)
		return
	}
	enrichLastUserMessage(messages)

	resp, err := callChatCompletion(c.Request.Context(), req.Model, messages, req.Stream)
	if err != nil {
		if respondChatTimeoutFallback(c, sessionID, req.Model, req.Messages, req.Stream, err) {
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer closeChatBody(resp.Body)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		proxyChatUpstreamBody(c, resp, sessionID)
		return
	}
	if req.Stream {
		streamChatResponse(c, resp.Body, sessionID, req.Model)
		return
	}
	proxyChatJSONResponse(c, resp, sessionID, req.Model)
}

func ensureChatSession(req model.ChatRequest) string {
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		now := time.Now()
		newID := store.NewID()
		title := chatSessionTitle(req.Messages)
		store.SaveChatSession(&model.ChatSession{ID: newID, Title: title, Model: req.Model, TargetIP: extractIP(title), CreatedAt: now, UpdatedAt: now})
		return newID
	} else if _, ok := store.GetChatSession(sessionID); !ok {
		now := time.Now()
		store.SaveChatSession(&model.ChatSession{ID: sessionID, Title: "New session", Model: req.Model, CreatedAt: now, UpdatedAt: now})
	}
	return sessionID
}

func chatSessionTitle(messages []model.Message) string {
	title := "New session"
	if len(messages) == 0 || strings.TrimSpace(messages[len(messages)-1].Content) == "" {
		return title
	}
	title = strings.TrimSpace(messages[len(messages)-1].Content)
	if len([]rune(title)) > 28 {
		title = string([]rune(title)[:28]) + "..."
	}
	return title
}

func persistLatestUserMessage(sessionID string, req model.ChatRequest) {
	if len(req.Messages) > 0 {
		last := req.Messages[len(req.Messages)-1]
		if last.Role == "user" {
			store.AddChatMessage(model.ChatMessage{ID: store.NewID(), SessionID: sessionID, Role: last.Role, Content: last.Content, Model: req.Model, TargetIP: extractIP(last.Content), CreatedAt: time.Now()})
		}
	}
}

func buildChatRequestMessages(requestMessages []model.Message) []model.Message {
	messages := make([]model.Message, 0, len(requestMessages)+1)
	messages = append(messages, model.Message{Role: "system", Content: systemPrompt})
	messages = append(messages, requestMessages...)
	return messages
}

func localChatAnswer(messages []model.Message) (string, bool) {
	idx := lastUserMessageIndex(messages)
	if idx < 0 {
		return "", false
	}
	handled, content := businessInspectionChatAnswer(messages[idx].Content)
	return content, handled
}

func respondLocalChatAnswer(c *gin.Context, sessionID, modelName, content string) {
	content = normalizeReportText(content)
	persistAssistantMessage(sessionID, modelName, "assistant", content)
	c.Header("X-Session-ID", sessionID)
	c.JSON(http.StatusOK, gin.H{"choices": []gin.H{{"message": model.Message{Role: "assistant", Content: content}}}})
}

func enrichLastUserMessage(messages []model.Message) {
	idx := lastUserMessageIndex(messages)
	if idx < 0 {
		return
	}
	ctx := buildMonitorContext(extractIP(messages[idx].Content), messages[idx].Content)
	if ctx != "" {
		messages[idx].Content += ctx
	}
}

func lastUserMessageIndex(messages []model.Message) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return i
		}
	}
	return -1
}

func callChatCompletion(ctx context.Context, modelName string, messages []model.Message, stream bool) (*http.Response, error) {
	baseURL, apiKey := getBaseURL(), getAPIKey()
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("AI provider is not configured: base_url or api_key is missing")
	}
	payload := map[string]interface{}{"model": modelName, "messages": messages, "stream": stream}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal chat payload: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create chat request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: chatUpstreamTimeout}
	return client.Do(httpReq)
}

func proxyChatUpstreamBody(c *gin.Context, resp *http.Response, sessionID string) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("read AI provider response: %v", err)})
		return
	}
	c.Header("X-Session-ID", sessionID)
	c.Data(resp.StatusCode, responseContentType(resp), data)
}

func proxyChatJSONResponse(c *gin.Context, resp *http.Response, sessionID, modelName string) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("read AI provider response: %v", err)})
		return
	}
	persistAssistantFromJSON(sessionID, modelName, data)
	c.Header("X-Session-ID", sessionID)
	c.Data(resp.StatusCode, responseContentType(resp), data)
}

func responseContentType(resp *http.Response) string {
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		return "application/json; charset=utf-8"
	}
	return contentType
}

func persistAssistantFromJSON(sessionID, modelName string, data []byte) {
	var upstream struct {
		Choices []struct {
			Message model.Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &upstream); err != nil {
		logrus.WithError(err).Warn("chat upstream JSON parse failed")
		return
	}
	if len(upstream.Choices) == 0 {
		return
	}
	msg := upstream.Choices[0].Message
	persistAssistantMessage(sessionID, modelName, msg.Role, msg.Content)
}

func persistAssistantMessage(sessionID, modelName, role, content string) {
	content = normalizeReportText(content)
	if strings.TrimSpace(content) == "" {
		return
	}
	if strings.TrimSpace(role) == "" {
		role = "assistant"
	}
	store.AddChatMessage(model.ChatMessage{ID: store.NewID(), SessionID: sessionID, Role: role, Content: content, Model: modelName, CreatedAt: time.Now()})
}

func closeChatBody(body io.Closer) {
	if err := body.Close(); err != nil {
		logrus.WithError(err).Warn("chat upstream body close failed")
	}
}

func businessInspectionChatAnswer(text string) (bool, string) {
	query := strings.ToLower(strings.TrimSpace(text))
	if query == "" || !(strings.Contains(query, "巡检") || strings.Contains(query, "inspection") || strings.Contains(query, "inspect")) {
		return false, ""
	}
	if business, ok := matchBusinessByNameOrHosts(text, nil); ok {
		inspection := buildBusinessInspection(business)
		lines := []string{
			fmt.Sprintf("**%s 业务巡检结论**", business.Name),
			fmt.Sprintf("- 状态：%s，评分：%d", inspection.Status, inspection.Score),
			fmt.Sprintf("- 摘要：%s", inspection.Summary),
			fmt.Sprintf("- 数据源：%s", strings.Join(inspection.DataSources, "、")),
		}
		if len(inspection.AISuggestions) > 0 {
			lines = append(lines, "- AI 建议：")
			for _, item := range limitStrings(inspection.AISuggestions, 5) {
				lines = append(lines, "  - "+item)
			}
		}
		if len(inspection.TopologyFindings) > 0 {
			lines = append(lines, "- 关键发现：")
			for _, item := range limitStrings(inspection.TopologyFindings, 5) {
				lines = append(lines, "  - "+item)
			}
		}
		return true, strings.Join(lines, "\n")
	}
	return false, ""
}

func Models(c *gin.Context) {
	items := []map[string]string{}
	for _, modelID := range configuredModelIDs() {
		items = append(items, map[string]string{"id": modelID})
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func configuredModelIDs() []string {
	seen := map[string]bool{}
	var models []string
	for _, p := range loadAIProviders() {
		for _, m := range p.Models {
			m = strings.TrimSpace(m)
			if m != "" && !seen[m] {
				seen[m] = true
				models = append(models, m)
			}
		}
	}
	if len(models) == 0 {
		if m := resolveDefaultModel(); m != "" {
			models = append(models, m)
		}
	}
	return models
}
