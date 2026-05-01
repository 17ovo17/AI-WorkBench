package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"ai-workbench-api/internal/model"

	"github.com/gin-gonic/gin"
)

func respondChatTimeoutFallback(c *gin.Context, sessionID, modelName string, messages []model.Message, stream bool, err error) bool {
	if !isChatTimeoutError(err) {
		return false
	}
	content := buildChatTimeoutFallbackContent(lastChatUserContent(messages))
	persistAssistantMessage(sessionID, modelName, "assistant", content)
	if stream {
		writeChatFallbackStream(c, sessionID, content)
		return true
	}
	c.Header("X-Session-ID", sessionID)
	c.Header("X-AI-Fallback", "timeout")
	c.JSON(http.StatusOK, gin.H{
		"choices":  []gin.H{{"message": model.Message{Role: "assistant", Content: content}}},
		"fallback": gin.H{"reason": "upstream_timeout", "retryAfterSeconds": 60},
	})
	return true
}

func writeChatFallbackStream(c *gin.Context, sessionID, content string) {
	c.Header("X-Session-ID", sessionID)
	c.Header("X-AI-Fallback", "timeout")
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	payload, _ := json.Marshal(gin.H{"choices": []gin.H{{"delta": gin.H{"content": content}}}})
	_, _ = c.Writer.WriteString("data: " + string(payload) + "\n\n")
	_, _ = c.Writer.WriteString("data: [DONE]\n\n")
	c.Writer.Flush()
}

func isChatTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "context deadline exceeded") || strings.Contains(text, "client.timeout") || strings.Contains(text, "timeout awaiting")
}

func lastChatUserContent(messages []model.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
}

func buildChatTimeoutFallbackContent(question string) string {
	if strings.TrimSpace(question) == "" {
		question = "本次 AI 问诊"
	}
	return fmt.Sprintf(`## 诊断状态：AI 正在深度分析中

上游模型在 %d 秒内未返回完整结果，平台已保留本次问诊记录，避免 HTTP 请求因等待过久而失败。

- **当前问题**：%s
- **初步分析**：先按通用 SRE 流程排查资源、日志、依赖和最近变更，等待模型完成后可补充更细的根因判断。
- **处置建议**：先查看诊断记录、Prometheus 指标和 Catpaw 巡检结果；如是 P0/P1 事件，请同步值班同学并保留现场证据。
- **下一步**：稍后刷新诊断记录，或补充目标 IP、服务名、时间范围后重新发起问诊。

> 生成时间：%s（UTC+8）`, int(chatUpstreamTimeout/time.Second), compactAIOpsText(question, 160), time.Now().Format("2006-01-02 15:04:05"))
}
