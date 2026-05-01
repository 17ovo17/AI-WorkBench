package embedding

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
)

// VectorEngine 基于余弦相似度的内存向量搜索引擎
type VectorEngine struct {
	mu       sync.RWMutex
	embedder EmbeddingProvider
	docs     map[string]*vectorDoc
}

// vectorDoc 已向量化的文档
type vectorDoc struct {
	ID         string
	Title      string
	Content    string
	DocType    string
	Category   string
	ParentID   string
	ChunkIndex int
	Embedding  []float64
}

// NewVectorEngine 创建向量搜索引擎
func NewVectorEngine(embedder EmbeddingProvider) *VectorEngine {
	return &VectorEngine{
		embedder: embedder,
		docs:     make(map[string]*vectorDoc),
	}
}

// Index 向量化文档并存储
func (e *VectorEngine) Index(doc Document) error {
	text := doc.Title + " " + doc.Content
	vec, err := e.embedder.Embed(context.Background(), text)
	if err != nil {
		return fmt.Errorf("vector index: embed failed: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.docs[doc.ID] = &vectorDoc{
		ID:         doc.ID,
		Title:      doc.Title,
		Content:    doc.Content,
		DocType:    doc.DocType,
		Category:   doc.Category,
		ParentID:   doc.ParentID,
		ChunkIndex: doc.ChunkIndex,
		Embedding:  vec,
	}
	return nil
}

// Remove 删除文档
func (e *VectorEngine) Remove(docID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.docs, docID)
	return nil
}

// Search 使用余弦相似度检索最相关的文档
func (e *VectorEngine) Search(query string, topK int) ([]SearchResult, error) {
	queryVec, err := e.embedder.Embed(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("vector search: embed query failed: %w", err)
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	type scored struct {
		doc   *vectorDoc
		score float64
	}

	results := make([]scored, 0, len(e.docs))
	for _, doc := range e.docs {
		sim := cosineSimilarity(queryVec, doc.Embedding)
		if sim > 0 {
			results = append(results, scored{doc: doc, score: sim})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if topK > len(results) {
		topK = len(results)
	}

	out := make([]SearchResult, topK)
	for i := 0; i < topK; i++ {
		out[i] = SearchResult{
			DocID:      results[i].doc.ID,
			Title:      results[i].doc.Title,
			Content:    results[i].doc.Content,
			Score:      results[i].score,
			DocType:    results[i].doc.DocType,
			Category:   results[i].doc.Category,
			ParentID:   results[i].doc.ParentID,
			ChunkIndex: results[i].doc.ChunkIndex,
		}
	}
	return out, nil
}

// Rebuild 全量重建向量索引
func (e *VectorEngine) Rebuild(docs []Document) error {
	texts := make([]string, len(docs))
	for i, doc := range docs {
		texts[i] = doc.Title + " " + doc.Content
	}

	vectors, err := e.embedder.EmbedBatch(context.Background(), texts)
	if err != nil {
		return fmt.Errorf("vector rebuild: batch embed failed: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.docs = make(map[string]*vectorDoc, len(docs))
	for i, doc := range docs {
		e.docs[doc.ID] = &vectorDoc{
			ID:         doc.ID,
			Title:      doc.Title,
			Content:    doc.Content,
			DocType:    doc.DocType,
			Category:   doc.Category,
			ParentID:   doc.ParentID,
			ChunkIndex: doc.ChunkIndex,
			Embedding:  vectors[i],
		}
	}
	return nil
}

// cosineSimilarity 计算两个向量的余弦相似度
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}
