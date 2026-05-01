package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"ai-workbench-api/internal/embedding"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
)

// GetEmbeddingSettings GET /api/v1/settings/embedding
func GetEmbeddingSettings(c *gin.Context) {
	cfg := embedding.LoadConfig()
	c.JSON(http.StatusOK, gin.H{
		"provider":   cfg.Provider,
		"api_url":    cfg.APIURL,
		"api_key":    maskSensitive(cfg.APIKey),
		"model":      cfg.Model,
		"dimensions": cfg.Dimensions,
		"batch_size": cfg.BatchSize,
	})
}

// UpdateEmbeddingSettings PUT /api/v1/settings/embedding
func UpdateEmbeddingSettings(c *gin.Context) {
	var req struct {
		Provider   string `json:"provider"`
		APIURL     string `json:"api_url"`
		APIKey     string `json:"api_key"`
		Model      string `json:"model"`
		Dimensions int    `json:"dimensions"`
		BatchSize  int    `json:"batch_size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	prov := req.Provider
	if prov == "builtin_bm25" {
		prov = "builtin"
	}
	cfg := embedding.EmbedConfig{
		Provider:   prov,
		APIURL:     req.APIURL,
		APIKey:     keepIfMasked(req.APIKey, "embedding.api.key"),
		Model:      req.Model,
		Dimensions: req.Dimensions,
		BatchSize:  req.BatchSize,
	}
	embedding.SaveEmbedConfig(cfg)
	embedding.ReloadSearcher()
	auditEvent(c, "settings.embedding.update", "embedding", "low", "ok",
		"provider="+cfg.Provider, c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TestEmbeddingConnection POST /api/v1/settings/embedding/test
func TestEmbeddingConnection(c *gin.Context) {
	var req struct {
		Text   string `json:"text"`
		APIURL string `json:"api_url"`
		APIKey string `json:"api_key"`
		Model  string `json:"model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		req.Text = "embedding connectivity test"
	}

	cfg := embedding.LoadConfig()
	if req.APIURL != "" {
		cfg.APIURL = req.APIURL
	}
	if req.APIKey != "" && !strings.HasSuffix(req.APIKey, "...") {
		cfg.APIKey = req.APIKey
	}
	if req.Model != "" {
		cfg.Model = req.Model
	}

	if cfg.APIURL == "" || cfg.APIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API URL 和 API Key 不能为空", "ok": false})
		return
	}

	embedder := embedding.NewAPIEmbedderFromConfig(cfg)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	vec, err := embedder.Embed(ctx, req.Text)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error(), "ok": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "dimensions": len(vec), "message": "连接成功"})
}

// GetRerankerSettings GET /api/v1/settings/reranker
func GetRerankerSettings(c *gin.Context) {
	cfg := embedding.LoadRerankerConfig()
	c.JSON(http.StatusOK, gin.H{
		"enabled":  cfg.Enabled,
		"provider": cfg.Provider,
		"api_url":  cfg.APIURL,
		"api_key":  maskSensitive(cfg.APIKey),
		"model":    cfg.Model,
		"top_k":    cfg.TopK,
	})
}

// UpdateRerankerSettings PUT /api/v1/settings/reranker
func UpdateRerankerSettings(c *gin.Context) {
	var req struct {
		Enabled  bool   `json:"enabled"`
		Provider string `json:"provider"`
		APIURL   string `json:"api_url"`
		APIKey   string `json:"api_key"`
		Model    string `json:"model"`
		TopK     int    `json:"top_k"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cfg := embedding.RerankerConfig{
		Enabled:  req.Enabled,
		Provider: req.Provider,
		APIURL:   req.APIURL,
		APIKey:   keepIfMasked(req.APIKey, "reranker.api.key"),
		Model:    req.Model,
		TopK:     req.TopK,
	}
	embedding.SaveRerankerConfig(cfg)
	embedding.ReloadSearcher()
	auditEvent(c, "settings.reranker.update", "reranker", "low", "ok",
		"provider="+cfg.Provider, c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func maskSensitive(val string) string {
	if len(val) <= 6 {
		return strings.Repeat("*", len(val))
	}
	return val[:6] + "..."
}

func keepIfMasked(newVal, settingKey string) string {
	if newVal == "" {
		return ""
	}
	if strings.HasSuffix(newVal, "...") || strings.Contains(newVal, "***") {
		if s, ok := store.GetAISetting(settingKey); ok && s.SettingValue != "" {
			return s.SettingValue
		}
	}
	return newVal
}
