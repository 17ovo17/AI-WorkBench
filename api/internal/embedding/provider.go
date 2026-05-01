package embedding

import "context"

// SearchResult 统一的搜索结果
type SearchResult struct {
	DocID      string  `json:"doc_id"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	DocType    string  `json:"doc_type"`
	Category   string  `json:"category"`
	ParentID   string  `json:"parent_id,omitempty"`
	ChunkIndex int     `json:"chunk_index,omitempty"`
}

// Document 待索引的文档
type Document struct {
	ID         string
	Title      string
	Content    string
	DocType    string
	Category   string
	ParentID   string
	ChunkIndex int
}

// EmbeddingProvider 向量化接口
type EmbeddingProvider interface {
	// Embed 对单条文本进行向量化
	Embed(ctx context.Context, text string) ([]float64, error)
	// EmbedBatch 批量向量化
	EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)
	// Dimensions 返回向量维度
	Dimensions() int
}

// SearchProvider 搜索接口
type SearchProvider interface {
	// Index 索引单个文档
	Index(doc Document) error
	// Remove 删除文档
	Remove(docID string) error
	// Search 搜索文档
	Search(query string, topK int) ([]SearchResult, error)
	// Rebuild 全量重建索引
	Rebuild(docs []Document) error
}
