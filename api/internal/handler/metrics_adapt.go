package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const (
	autoAdaptBatchSize = 30
	autoAdaptTimeout   = 90 * time.Second
)

type adaptResult struct {
	RawName      string `json:"raw_name"`
	Exporter     string `json:"exporter"`
	StandardName string `json:"standard_name"`
	Description  string `json:"description"`
	Transform    string `json:"transform"`
	PromQL       string `json:"promql,omitempty"`
	NormalRange  string `json:"normal_range,omitempty"`
}

// AutoAdaptMetrics POST /api/v1/metrics/auto-adapt
// 璋冪敤 LLM 瀵规湭閫傞厤鐨勬寚鏍囪繘琛屾壒閲忛€傞厤銆?
func AutoAdaptMetrics(c *gin.Context) {
	var req struct {
		DatasourceID string `json:"datasource_id"`
		MaxBatches   int    `json:"max_batches"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.DatasourceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "datasource_id required"})
		return
	}
	if req.MaxBatches <= 0 || req.MaxBatches > 20 {
		req.MaxBatches = 5
	}

	prompt, err := loadAdaptPrompt()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "load prompt failed: " + err.Error()})
		return
	}

	processed, adapted := runAutoAdapt(req.DatasourceID, prompt, req.MaxBatches)
	auditEvent(c, "metrics.auto_adapt", req.DatasourceID, "low", "ok",
		fmt.Sprintf("processed=%d adapted=%d", processed, adapted), c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{
		"processed": processed,
		"adapted":   adapted,
	})
}

// runAutoAdapt 鎵ц澶氳疆鎵归噺閫傞厤锛岃繑鍥炲鐞嗘€绘暟鍜屾垚鍔熼€傞厤鏁般€?
func runAutoAdapt(datasourceID, prompt string, maxBatches int) (int, int) {
	processed := 0
	adapted := 0
	for round := 0; round < maxBatches; round++ {
		batch, _ := store.ListMetricsMappings(datasourceID, "unmapped", 1, autoAdaptBatchSize)
		if len(batch) == 0 {
			break
		}
		results, err := callLLMForAdapt(prompt, batch)
		if err != nil {
			logrus.Warnf("auto adapt round %d failed: %v", round, err)
			processed += len(batch)
			continue
		}
		for _, m := range batch {
			processed++
			r := findAdaptResult(results, m.RawName)
			if r == nil {
				continue
			}
			updated := m
			updated.StandardName = r.StandardName
			updated.Exporter = r.Exporter
			updated.Description = r.Description
			updated.Transform = firstNonEmpty(r.Transform, r.PromQL)
			updated.Status = "auto"
			store.UpdateMetricsMapping(&updated)
			adapted++
		}
	}
	return processed, adapted
}

// loadAdaptPrompt 鍔犺浇鎸囨爣閫傞厤 Prompt 妯℃澘銆?
func loadAdaptPrompt() (string, error) {
	data, err := os.ReadFile("assets/prompts/metrics_adapt.txt")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// callLLMForAdapt 璋冪敤 LLM 澶勭悊涓€鎵规寚鏍囷紝杩斿洖閫傞厤缁撴灉鏁扮粍銆?
func callLLMForAdapt(prompt string, batch []model.MetricsMapping) ([]adaptResult, error) {
	names := make([]string, len(batch))
	for i, m := range batch {
		names[i] = m.RawName
	}
	userMsg := "璇烽€傞厤浠ヤ笅鎸囨爣鍒楄〃锛堜粎杩斿洖 JSON 鏁扮粍锛夛細\n" + joinLines(names)

	model := resolveDefaultModel()

	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": prompt},
			{"role": "user", "content": userMsg},
		},
		"max_tokens": 4096,
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: autoAdaptTimeout}
	req, err := http.NewRequest(http.MethodPost, getBaseURL()+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+getAPIKey())

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(raw))
	}

	var upstream struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &upstream); err != nil || len(upstream.Choices) == 0 {
		return nil, fmt.Errorf("bad upstream response")
	}
	return parseAdaptJSON(upstream.Choices[0].Message.Content)
}

// parseAdaptJSON 浠?LLM 鍝嶅簲涓彁鍙?JSON 鏁扮粍銆?
func parseAdaptJSON(content string) ([]adaptResult, error) {
	var lastErr error
	for _, candidate := range adaptJSONCandidates(content) {
		results, err := decodeAdaptResults(candidate)
		if err == nil {
			return results, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, fmt.Errorf("parse JSON: %w", lastErr)
	}
	return nil, fmt.Errorf("no JSON found in response")
}

func adaptJSONCandidates(content string) []string {
	trimmed := strings.TrimSpace(content)
	out := []string{trimmed}
	out = append(out, fencedJSONBlocks(trimmed)...)
	if start, end := indexOf(trimmed, "["), lastIndexOf(trimmed, "]"); start >= 0 && end > start {
		out = append(out, trimmed[start:end+1])
	}
	if start, end := indexOf(trimmed, "{"), lastIndexOf(trimmed, "}"); start >= 0 && end > start {
		out = append(out, trimmed[start:end+1])
	}
	return inspectionUniqueStrings(out)
}

func fencedJSONBlocks(content string) []string {
	out := []string{}
	parts := strings.Split(content, "```")
	for i := 1; i < len(parts); i += 2 {
		block := strings.TrimSpace(parts[i])
		block = strings.TrimPrefix(block, "json")
		block = strings.TrimPrefix(block, "JSON")
		if block = strings.TrimSpace(block); block != "" {
			out = append(out, block)
		}
	}
	return out
}

func decodeAdaptResults(raw string) ([]adaptResult, error) {
	var results []adaptResult
	if err := json.Unmarshal([]byte(raw), &results); err == nil {
		return normalizeAdaptResults(results), nil
	}
	var wrapper struct {
		Items   []adaptResult `json:"items"`
		Results []adaptResult `json:"results"`
		Data    []adaptResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
		return nil, err
	}
	return normalizeAdaptResults(firstNonEmptyAdaptSlice(wrapper.Items, wrapper.Results, wrapper.Data)), nil
}

func normalizeAdaptResults(items []adaptResult) []adaptResult {
	for i := range items {
		items[i].Transform = firstNonEmpty(items[i].Transform, items[i].PromQL)
	}
	return items
}

func firstNonEmptyAdaptSlice(slices ...[]adaptResult) []adaptResult {
	for _, items := range slices {
		if len(items) > 0 {
			return items
		}
	}
	return nil
}

func findAdaptResult(results []adaptResult, name string) *adaptResult {
	for i := range results {
		if results[i].RawName == name {
			return &results[i]
		}
	}
	return nil
}

func joinLines(items []string) string {
	out := ""
	for _, s := range items {
		out += "- " + s + "\n"
	}
	return out
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func lastIndexOf(s, sub string) int {
	for i := len(s) - len(sub); i >= 0; i-- {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
