package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	embeddingAPITimeout = 30 * time.Second
	defaultBatchSize    = 10
	maxAPIBatchSize     = 10
)

// APIEmbedder 外部 Embedding API 客户端（OpenAI /embeddings 兼容格式）
type APIEmbedder struct {
	url        string
	apiKey     string
	model      string
	dimensions int
	batchSize  int
	client     *http.Client
}

// embeddingRequest OpenAI 兼容的请求体
type embeddingRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

// embeddingResponse OpenAI 兼容的响应体
type embeddingResponse struct {
	Data []embeddingData `json:"data"`
}

// embeddingData 单条向量结果
type embeddingData struct {
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// NewAPIEmbedder 从 viper 读取配置创建 API 客户端（兼容旧调用）
func NewAPIEmbedder() *APIEmbedder {
	return NewAPIEmbedderFromConfig(LoadConfig())
}

// NewAPIEmbedderFromConfig 从 EmbedConfig 创建 API 客户端
func NewAPIEmbedderFromConfig(cfg EmbedConfig) *APIEmbedder {
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	if batchSize > maxAPIBatchSize {
		batchSize = maxAPIBatchSize
	}

	return &APIEmbedder{
		url:        cfg.APIURL,
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		dimensions: cfg.Dimensions,
		batchSize:  batchSize,
		client:     &http.Client{Timeout: embeddingAPITimeout},
	}
}

// Embed 对单条文本进行向量化
func (e *APIEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	results, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("embedding: empty response from API")
	}
	return results[0], nil
}

// EmbedBatch 批量向量化，自动按 batchSize 分批
func (e *APIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	allResults := make([][]float64, len(texts))

	for i := 0; i < len(texts); i += e.batchSize {
		end := i + e.batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		embeddings, err := e.callAPI(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("embedding: batch %d-%d failed: %w", i, end, err)
		}

		for _, item := range embeddings {
			idx := i + item.Index
			if idx < len(allResults) {
				allResults[idx] = item.Embedding
			}
		}
	}

	return allResults, nil
}

// Dimensions 返回向量维度
func (e *APIEmbedder) Dimensions() int {
	return e.dimensions
}

// callAPI 调用外部 embedding API
func (e *APIEmbedder) callAPI(ctx context.Context, texts []string) ([]embeddingData, error) {
	reqBody := embeddingRequest{
		Model: e.model,
		Input: texts,
	}
	if e.dimensions > 0 {
		reqBody.Dimensions = e.dimensions
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := strings.TrimRight(e.url, "/") + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Data, nil
}
