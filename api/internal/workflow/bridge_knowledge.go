package workflow

import (
	"context"

	"ai-workbench-api/internal/embedding"
	"ai-workbench-api/internal/store"
	"ai-workbench-api/internal/workflow/node"

	"github.com/sirupsen/logrus"
)

// KnowledgeBridge bridges the node.KnowledgeSearcher interface to the store layer.
type KnowledgeBridge struct{}

func (b *KnowledgeBridge) Search(ctx context.Context, query string, topK int, category string) ([]node.KnowledgeResult, error) {
	if topK <= 0 {
		topK = 5
	}

	// 尝试使用混合搜索
	searcher := embedding.GetSearcher()
	if searcher != nil {
		results, err := searcher.Search(ctx, query, topK*3)
		if err == nil && len(results) > 0 {
			filtered := filterEmbeddingResults(results, category)
			if len(filtered) > topK {
				filtered = filtered[:topK]
			}
			return convertEmbeddingResults(filtered), nil
		}
		if err != nil {
			logrus.WithError(err).Warn("workflow: hybrid search failed, falling back to FULLTEXT")
		}
	}

	// fallback 到 FULLTEXT 搜索
	return b.searchFallback(query, topK, category)
}

// searchFallback 使用 FULLTEXT 搜索（原有逻辑）
func (b *KnowledgeBridge) searchFallback(query string, topK int, category string) ([]node.KnowledgeResult, error) {
	cases, _ := store.ListCases(1, topK, query, category)
	results := make([]node.KnowledgeResult, 0, len(cases))
	for _, c := range cases {
		results = append(results, node.KnowledgeResult{
			ID:          c.ID,
			Score:       c.EvaluationAvg,
			Category:    c.RootCauseCategory,
			Description: c.RootCauseDescription,
			Treatment:   c.TreatmentSteps,
			Keywords:    c.Keywords,
		})
	}
	return results, nil
}

// filterEmbeddingResults 按 category 过滤搜索结果
func filterEmbeddingResults(results []embedding.SearchResult, category string) []embedding.SearchResult {
	if category == "" {
		return results
	}
	filtered := make([]embedding.SearchResult, 0, len(results))
	for _, r := range results {
		if r.Category == category {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) == 0 {
		return results // 无匹配时返回全部
	}
	return filtered
}

// convertEmbeddingResults 将 embedding.SearchResult 转换为 node.KnowledgeResult
func convertEmbeddingResults(results []embedding.SearchResult) []node.KnowledgeResult {
	out := make([]node.KnowledgeResult, 0, len(results))
	for _, r := range results {
		out = append(out, node.KnowledgeResult{
			ID:          r.DocID,
			Score:       r.Score,
			Category:    r.Category,
			Description: r.Content,
		})
	}
	return out
}
