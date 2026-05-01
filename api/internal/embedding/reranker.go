package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"ai-workbench-api/internal/aiconfig"

	"github.com/spf13/viper"
)

const (
	rerankerAPITimeout = 30 * time.Second
	contentTruncLen    = 200 // LLM 重排时截取内容长度
)

// Reranker 重排接口
type Reranker interface {
	Rerank(ctx context.Context, query string, docs []SearchResult, topK int) ([]SearchResult, error)
}

// NewReranker 根据 viper 配置创建重排器（兼容旧调用）
func NewReranker() Reranker {
	return NewRerankerFromConfig(LoadRerankerConfig())
}

// NewRerankerFromConfig 根据 RerankerConfig 创建重排器
func NewRerankerFromConfig(cfg RerankerConfig) Reranker {
	switch cfg.Provider {
	case "api":
		return newAPIRerankerFromConfig(cfg)
	default:
		return newLLMReranker()
	}
}

// --- LLM Reranker ---

// LLMReranker 使用 LLM 做文档重排
type LLMReranker struct {
	llmURL string
	apiKey string
	model  string
	client *http.Client
}

func newLLMReranker() *LLMReranker {
	return &LLMReranker{
		llmURL: viper.GetString("ai.base_url"),
		apiKey: viper.GetString("ai.api_key"),
		model:  aiconfig.ResolveDefaultModel(),
		client: &http.Client{Timeout: rerankerAPITimeout},
	}
}

// Rerank 通过 LLM 对候选文档按相关性重排
func (r *LLMReranker) Rerank(ctx context.Context, query string, docs []SearchResult, topK int) ([]SearchResult, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	prompt := buildRerankPrompt(query, docs)
	rankings, err := r.callLLM(ctx, prompt)
	if err != nil {
		// LLM 重排失败时降级：直接截取前 topK
		if topK > len(docs) {
			topK = len(docs)
		}
		return docs[:topK], nil
	}

	return applyRankings(docs, rankings, topK), nil
}

// callLLM 调用 LLM API 获取排序结果
func (r *LLMReranker) callLLM(ctx context.Context, prompt string) ([]rankItem, error) {
	reqBody := map[string]interface{}{
		"model": r.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal LLM request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.llmURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create LLM request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send LLM request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read LLM response: %w", err)
	}

	return parseLLMRankResponse(respBody)
}

// --- API Reranker ---

// APIReranker 调用外部 rerank API（Cohere/Jina 兼容）
type APIReranker struct {
	url    string
	apiKey string
	model  string
	client *http.Client
}

func newAPIReranker() *APIReranker {
	return newAPIRerankerFromConfig(LoadRerankerConfig())
}

func newAPIRerankerFromConfig(cfg RerankerConfig) *APIReranker {
	return &APIReranker{
		url:    cfg.APIURL,
		apiKey: cfg.APIKey,
		model:  cfg.Model,
		client: &http.Client{Timeout: rerankerAPITimeout},
	}
}

// rerankAPIRequest Cohere/Jina 兼容的 rerank 请求
type rerankAPIRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n"`
}

// rerankAPIResponse rerank API 响应
type rerankAPIResponse struct {
	Results []struct {
		Index int     `json:"index"`
		Score float64 `json:"relevance_score"`
	} `json:"results"`
}

// Rerank 调用外部 rerank API 重排
func (r *APIReranker) Rerank(ctx context.Context, query string, docs []SearchResult, topK int) ([]SearchResult, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	texts := make([]string, len(docs))
	for i, d := range docs {
		texts[i] = d.Title + ": " + truncate(d.Content, contentTruncLen)
	}

	reqBody := rerankAPIRequest{
		Model:     r.model,
		Query:     query,
		Documents: texts,
		TopN:      topK,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal rerank request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create rerank request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send rerank request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rerank API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result rerankAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode rerank response: %w", err)
	}

	out := make([]SearchResult, 0, len(result.Results))
	for _, r := range result.Results {
		if r.Index < len(docs) {
			item := docs[r.Index]
			item.Score = r.Score
			out = append(out, item)
		}
	}
	return out, nil
}

// --- 辅助函数 ---

// rankItem LLM 返回的排序项
type rankItem struct {
	Index int     `json:"index"`
	Score float64 `json:"score"`
}

// buildRerankPrompt 构建 LLM 重排 prompt
func buildRerankPrompt(query string, docs []SearchResult) string {
	var buf bytes.Buffer
	buf.WriteString("给定查询和候选文档列表，按相关性从高到低排序，")
	buf.WriteString("返回 JSON 数组 [{\"index\": 0, \"score\": 0.95}, ...]\n")
	buf.WriteString(fmt.Sprintf("查询：%s\n候选文档：\n", query))
	for i, doc := range docs {
		content := truncate(doc.Content, contentTruncLen)
		buf.WriteString(fmt.Sprintf("%d. %s: %s\n", i, doc.Title, content))
	}
	return buf.String()
}

// parseLLMRankResponse 解析 LLM 返回的排序 JSON
func parseLLMRankResponse(body []byte) ([]rankItem, error) {
	// 解析 OpenAI 兼容的 chat completion 响应
	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}

	var items []rankItem
	content := chatResp.Choices[0].Message.Content
	if err := json.Unmarshal([]byte(content), &items); err != nil {
		return nil, fmt.Errorf("parse ranking JSON: %w", err)
	}
	return items, nil
}

// applyRankings 根据排序结果重排文档
func applyRankings(docs []SearchResult, rankings []rankItem, topK int) []SearchResult {
	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].Score > rankings[j].Score
	})

	if topK > len(rankings) {
		topK = len(rankings)
	}

	out := make([]SearchResult, 0, topK)
	for i := 0; i < topK; i++ {
		idx := rankings[i].Index
		if idx >= 0 && idx < len(docs) {
			item := docs[idx]
			item.Score = rankings[i].Score
			out = append(out, item)
		}
	}
	return out
}

// truncate 截取字符串到指定 rune 长度
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
