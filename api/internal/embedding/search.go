package embedding

import (
	"context"
	"sort"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

const (
	rrfK             = 60 // RRF 融合常数
	defaultSearchTop = 50 // 默认粗筛数量
)

// HybridSearcher 混合搜索编排器
type HybridSearcher struct {
	mu       sync.RWMutex
	bm25     *BM25Engine
	vector   *VectorEngine // 可能为 nil（未配置外部 embedding）
	reranker Reranker      // 可能为 nil（未启用）
	provider string        // "builtin" | "api" | "hybrid"
}

// globalSearcher 全局搜索实例
var (
	globalSearcher *HybridSearcher
	searcherMu     sync.RWMutex
)

// Init 全局初始化函数，启动时调用
func Init() {
	globalSearcher = NewHybridSearcher()
	log.WithField("provider", globalSearcher.provider).Info("embedding: search engine initialized")
}

// GetSearcher 获取全局搜索实例
func GetSearcher() *HybridSearcher {
	searcherMu.RLock()
	defer searcherMu.RUnlock()
	return globalSearcher
}

// ReloadSearcher 重新加载配置并重建搜索器（前端修改配置后调用）
func ReloadSearcher() {
	searcherMu.Lock()
	defer searcherMu.Unlock()
	globalSearcher = NewHybridSearcher()
	log.Info("embedding: searcher reloaded with new config")
}

// NewHybridSearcher 根据配置初始化混合搜索器
func NewHybridSearcher() *HybridSearcher {
	cfg := LoadConfig()
	rcfg := LoadRerankerConfig()

	provider := cfg.Provider
	if provider == "" {
		provider = "builtin"
	}

	h := &HybridSearcher{
		provider: provider,
	}

	// BM25 在 builtin 和 hybrid 模式下启用
	if provider == "builtin" || provider == "hybrid" {
		h.bm25 = NewBM25Engine()
		log.Info("embedding: BM25 engine enabled")
	}

	// 向量引擎在 api 和 hybrid 模式下启用
	if provider == "api" || provider == "hybrid" {
		embedder := NewAPIEmbedderFromConfig(cfg)
		h.vector = NewVectorEngine(embedder)
		log.Info("embedding: vector engine enabled")
	}

	// Reranker 可选
	if rcfg.Enabled {
		h.reranker = NewRerankerFromConfig(rcfg)
		log.WithField("provider", rcfg.Provider).
			Info("embedding: reranker enabled")
	}

	return h
}

// Search 混合搜索入口
func (h *HybridSearcher) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var results []SearchResult
	var err error

	switch h.provider {
	case "builtin":
		results, err = h.searchBM25(query, topK)
	case "api":
		results, err = h.searchVector(query, topK)
	case "hybrid":
		results, err = h.searchHybrid(query, topK)
	default:
		results, err = h.searchBM25(query, topK)
	}

	if err != nil {
		return nil, err
	}

	// 如果启用了 reranker，对结果做重排
	if h.reranker != nil && len(results) > 0 {
		reranked, rerankErr := h.reranker.Rerank(ctx, query, results, topK)
		if rerankErr != nil {
			log.WithError(rerankErr).Warn("embedding: rerank failed, using original results")
			return limitResults(boostSearchResults(query, results), topK), nil
		}
		return limitResults(boostSearchResults(query, reranked), topK), nil
	}

	return limitResults(boostSearchResults(query, results), topK), nil
}

// Index 索引文档到所有启用的引擎
func (h *HybridSearcher) Index(doc Document) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.bm25 != nil {
		if err := h.bm25.Index(doc); err != nil {
			return err
		}
	}
	if h.vector != nil {
		if err := h.vector.Index(doc); err != nil {
			return err
		}
	}
	return nil
}

// Remove 从所有引擎中删除文档
func (h *HybridSearcher) Remove(docID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.bm25 != nil {
		if err := h.bm25.Remove(docID); err != nil {
			return err
		}
	}
	if h.vector != nil {
		if err := h.vector.Remove(docID); err != nil {
			return err
		}
	}
	return nil
}

// Rebuild 全量重建所有引擎的索引
func (h *HybridSearcher) Rebuild(docs []Document) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.bm25 != nil {
		if err := h.bm25.Rebuild(docs); err != nil {
			return err
		}
	}
	if h.vector != nil {
		if err := h.vector.Rebuild(docs); err != nil {
			return err
		}
	}
	return nil
}

// searchBM25 仅使用 BM25 搜索
func (h *HybridSearcher) searchBM25(query string, topK int) ([]SearchResult, error) {
	return h.bm25.Search(query, topK)
}

// searchVector 仅使用向量搜索
func (h *HybridSearcher) searchVector(query string, topK int) ([]SearchResult, error) {
	return h.vector.Search(query, topK)
}

// searchHybrid BM25 + 向量 → RRF 融合
func (h *HybridSearcher) searchHybrid(query string, topK int) ([]SearchResult, error) {
	bm25Results, err := h.bm25.Search(query, defaultSearchTop)
	if err != nil {
		return nil, err
	}

	vectorResults, err := h.vector.Search(query, defaultSearchTop)
	if err != nil {
		return nil, err
	}

	merged := mergeResults(bm25Results, vectorResults)
	return limitResults(merged, topK), nil
}

// mergeResults 使用 RRF（Reciprocal Rank Fusion）融合两路结果
func mergeResults(bm25Results, vectorResults []SearchResult) []SearchResult {
	scores := make(map[string]float64)
	docMap := make(map[string]SearchResult)

	// BM25 结果的 RRF 分数
	for rank, r := range bm25Results {
		scores[r.DocID] += 1.0 / float64(rrfK+rank+1)
		docMap[r.DocID] = r
	}

	// 向量结果的 RRF 分数
	for rank, r := range vectorResults {
		scores[r.DocID] += 1.0 / float64(rrfK+rank+1)
		if _, exists := docMap[r.DocID]; !exists {
			docMap[r.DocID] = r
		}
	}

	// 按 RRF 分数排序
	type scoredDoc struct {
		docID string
		score float64
	}
	ranked := make([]scoredDoc, 0, len(scores))
	for id, s := range scores {
		ranked = append(ranked, scoredDoc{docID: id, score: s})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	results := make([]SearchResult, 0, len(ranked))
	for _, item := range ranked {
		doc := docMap[item.docID]
		doc.Score = item.score
		results = append(results, doc)
	}
	return results
}

func boostSearchResults(query string, results []SearchResult) []SearchResult {
	terms := relevanceTerms(query)
	if len(terms) == 0 || len(results) == 0 {
		return results
	}
	for i := range results {
		text := strings.ToLower(results[i].Title + " " + results[i].Content + " " + results[i].Category + " " + results[i].DocType)
		bonus := 0.0
		for _, term := range terms {
			if strings.Contains(text, strings.ToLower(term)) {
				bonus += 0.35
			}
		}
		if bonus == 0 && isFocusedOpsQuery(query) {
			results[i].Score *= 0.2
			continue
		}
		results[i].Score += bonus
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	return results
}

func isFocusedOpsQuery(query string) bool {
	return len([]rune(strings.TrimSpace(query))) <= 12
}

func relevanceTerms(query string) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	terms := []string{}
	for _, item := range domainSynonyms(q) {
		terms = appendUniqueTerm(terms, item)
	}
	for _, item := range strings.Fields(q) {
		terms = appendUniqueTerm(terms, item)
	}
	return terms
}

func appendUniqueTerm(items []string, term string) []string {
	term = strings.TrimSpace(term)
	if term == "" {
		return items
	}
	for _, item := range items {
		if item == term {
			return items
		}
	}
	return append(items, term)
}

// limitResults 截取前 topK 个结果
func limitResults(results []SearchResult, topK int) []SearchResult {
	if topK >= len(results) {
		return results
	}
	return results[:topK]
}
