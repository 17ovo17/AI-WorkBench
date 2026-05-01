package handler

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const chatStreamMaxScanSize = 1024 * 1024

func streamChatResponse(c *gin.Context, body io.Reader, sessionID, modelName string) {
	c.Header("X-Session-ID", sessionID)
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	c.Writer.Flush()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), chatStreamMaxScanSize)
	var answer strings.Builder
	for scanner.Scan() {
		done, err := writeChatStreamLine(c, scanner.Text(), &answer)
		if err != nil {
			logrus.WithError(err).Warn("chat stream write failed")
			return
		}
		if done {
			persistAssistantMessage(sessionID, modelName, "assistant", normalizeReportText(answer.String()))
			return
		}
	}
	if err := scanner.Err(); err != nil {
		logrus.WithError(err).Warn("chat stream scan failed")
		if writeErr := writeChatStreamError(c, "AI provider stream interrupted"); writeErr != nil {
			logrus.WithError(writeErr).Warn("chat stream error write failed")
		}
		return
	}
	persistAssistantMessage(sessionID, modelName, "assistant", normalizeReportText(answer.String()))
	if _, err := c.Writer.WriteString("data: [DONE]\n\n"); err != nil {
		logrus.WithError(err).Warn("chat stream done write failed")
		return
	}
	c.Writer.Flush()
}

func writeChatStreamLine(c *gin.Context, line string, answer *strings.Builder) (bool, error) {
	done := false
	if data, ok := chatStreamData(line); ok {
		if data == "[DONE]" {
			done = true
		} else {
			appendChatStreamDelta(data, answer)
		}
	}
	suffix := "\n"
	if done {
		suffix = "\n\n"
	}
	if _, err := c.Writer.WriteString(line + suffix); err != nil {
		return false, err
	}
	c.Writer.Flush()
	return done, nil
}

func chatStreamData(line string) (string, bool) {
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(line, "data:")), true
}

func appendChatStreamDelta(data string, answer *strings.Builder) {
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		logrus.WithError(err).Warn("chat stream chunk parse failed")
		return
	}
	if len(chunk.Choices) > 0 {
		answer.WriteString(chunk.Choices[0].Delta.Content)
	}
}

func writeChatStreamError(c *gin.Context, message string) error {
	payload, err := json.Marshal(map[string]string{"error": message})
	if err != nil {
		return err
	}
	if _, err := c.Writer.WriteString("event: error\ndata: " + string(payload) + "\n\n"); err != nil {
		return err
	}
	c.Writer.Flush()
	return nil
}
