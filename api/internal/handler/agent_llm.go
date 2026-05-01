package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)


func agentRequestID() string {
	return fmt.Sprintf("agent-%d", time.Now().UnixNano())
}

type agentLLMRequest struct {
	Messages  []map[string]interface{} `json:"messages"`
	Tools     []map[string]interface{} `json:"tools,omitempty"`
	MaxTokens int                      `json:"max_tokens,omitempty"`
	TimeoutMs int64                    `json:"timeout_ms,omitempty"`
	Metadata  map[string]interface{}   `json:"metadata,omitempty"`
}

type agentLLMEnvelope struct {
	RequestID string                 `json:"request_id"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     *agentLLMError         `json:"error,omitempty"`
}

type agentLLMError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func AgentLLMChat(c *gin.Context) {
	var req agentLLMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, agentLLMEnvelope{RequestID: agentRequestID(), Error: &agentLLMError{Code: "bad_request", Message: err.Error()}})
		return
	}
	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, agentLLMEnvelope{RequestID: agentRequestID(), Error: &agentLLMError{Code: "bad_request", Message: "messages is required"}})
		return
	}

	model := resolveDefaultModel()
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	payload := map[string]interface{}{
		"model":      model,
		"messages":   req.Messages,
		"max_tokens": maxTokens,
	}
	if len(req.Tools) > 0 {
		payload["tools"] = req.Tools
	}

	body, _ := json.Marshal(payload)
	client := http.DefaultClient
	if req.TimeoutMs > 0 {
		client = &http.Client{Timeout: time.Duration(req.TimeoutMs) * time.Millisecond}
	}
	upstreamReq, err := http.NewRequest(http.MethodPost, getBaseURL()+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, agentLLMEnvelope{RequestID: agentRequestID(), Error: &agentLLMError{Code: "request_build_failed", Message: err.Error()}})
		return
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+getAPIKey())

	started := time.Now()
	resp, err := client.Do(upstreamReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, agentLLMEnvelope{RequestID: agentRequestID(), Error: &agentLLMError{Code: "upstream_failed", Message: err.Error()}})
		return
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, agentLLMEnvelope{RequestID: agentRequestID(), Error: &agentLLMError{Code: "upstream_error", Message: string(raw)}})
		return
	}

	var upstream struct {
		ID      string `json:"id"`
		Choices []struct {
			Message      map[string]interface{} `json:"message"`
			FinishReason string                 `json:"finish_reason"`
		} `json:"choices"`
		Usage map[string]interface{} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &upstream); err != nil || len(upstream.Choices) == 0 {
		c.JSON(http.StatusBadGateway, agentLLMEnvelope{RequestID: agentRequestID(), Error: &agentLLMError{Code: "bad_upstream_response", Message: fmt.Sprintf("%v", err)}})
		return
	}

	c.JSON(http.StatusOK, agentLLMEnvelope{
		RequestID: agentRequestID(),
		Data: map[string]interface{}{
			"id":                  upstream.ID,
			"model":               model,
			"message":             upstream.Choices[0].Message,
			"finish_reason":       upstream.Choices[0].FinishReason,
			"usage":               upstream.Usage,
			"provider_latency_ms": time.Since(started).Milliseconds(),
			"attempts":            1,
		},
	})
}
