package model

import "time"

// KnowledgeDocument 知识库文档，支持多格式文档管理与分块向量化。
type KnowledgeDocument struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Content        string    `json:"content"`
	DocType        string    `json:"doc_type"`        // case/runbook/document/faq
	FileType       string    `json:"file_type"`       // md/pdf/docx/txt/html
	FileName       string    `json:"file_name"`
	FileSize       int       `json:"file_size"`
	Category       string    `json:"category"`
	Tags           string    `json:"tags"`
	SourceID       string    `json:"source_id"`       // 关联的 case_id 或 runbook_id
	EmbeddingModel string    `json:"embedding_model"`
	ChunkIndex     int       `json:"chunk_index"`
	ParentID       string    `json:"parent_id"`       // 分块时指向原文档
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
