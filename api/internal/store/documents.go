package store

import (
	"sort"
	"strings"
	"time"

	"ai-workbench-api/internal/model"
)

// SaveDocument 保存知识库文档到内存和 MySQL。
func SaveDocument(doc *model.KnowledgeDocument) {
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = time.Now()
	}
	doc.UpdatedAt = time.Now()
	mu.Lock()
	knowledgeDocs[doc.ID] = doc
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(
			`REPLACE INTO knowledge_documents (id,title,content,doc_type,file_type,file_name,file_size,category,tags,source_id,embedding_model,chunk_index,parent_id,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			doc.ID, doc.Title, doc.Content, doc.DocType, doc.FileType,
			doc.FileName, doc.FileSize, doc.Category, doc.Tags, doc.SourceID,
			doc.EmbeddingModel, doc.ChunkIndex, doc.ParentID,
			doc.CreatedAt, doc.UpdatedAt,
		)
	}
}

// GetDocument 通过 ID 获取单个知识库文档。
func GetDocument(id string) (*model.KnowledgeDocument, bool) {
	if mysqlOK {
		row := db.QueryRow(
			`SELECT id,title,content,doc_type,COALESCE(file_type,''),COALESCE(file_name,''),file_size,COALESCE(category,''),COALESCE(tags,''),COALESCE(source_id,''),COALESCE(embedding_model,''),chunk_index,COALESCE(parent_id,''),created_at,updated_at FROM knowledge_documents WHERE id=?`, id)
		var doc model.KnowledgeDocument
		if err := row.Scan(&doc.ID, &doc.Title, &doc.Content, &doc.DocType,
			&doc.FileType, &doc.FileName, &doc.FileSize, &doc.Category,
			&doc.Tags, &doc.SourceID, &doc.EmbeddingModel, &doc.ChunkIndex,
			&doc.ParentID, &doc.CreatedAt, &doc.UpdatedAt); err == nil {
			return &doc, true
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	doc, ok := knowledgeDocs[id]
	if !ok {
		return nil, false
	}
	cp := *doc
	return &cp, true
}

// DeleteDocument 通过 ID 删除知识库文档。
func DeleteDocument(id string) {
	mu.Lock()
	delete(knowledgeDocs, id)
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`DELETE FROM knowledge_documents WHERE id=?`, id)
	}
}

// DeleteDocumentsByParent 删除指定父文档的所有子块。
func DeleteDocumentsByParent(parentID string) {
	mu.Lock()
	for id, doc := range knowledgeDocs {
		if doc.ParentID == parentID {
			delete(knowledgeDocs, id)
		}
	}
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`DELETE FROM knowledge_documents WHERE parent_id=?`, parentID)
	}
}

// ListSiblingChunks 按 chunk_index 返回同一父文档下的所有分块。
func ListSiblingChunks(parentID string) []model.KnowledgeDocument {
	if parentID == "" {
		return nil
	}
	if mysqlOK {
		rows, err := db.Query(selectDocsSQL()+` WHERE parent_id=? ORDER BY chunk_index`, parentID)
		if err == nil {
			defer rows.Close()
			return scanDocRows(rows)
		}
	}
	return listSiblingChunksMemory(parentID)
}

func listSiblingChunksMemory(parentID string) []model.KnowledgeDocument {
	mu.RLock()
	defer mu.RUnlock()
	out := []model.KnowledgeDocument{}
	for _, doc := range knowledgeDocs {
		if doc.ParentID == parentID {
			out = append(out, *doc)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ChunkIndex < out[j].ChunkIndex })
	return out
}

// ListChunkWindow 返回命中分块前后 radius 个分块。
func ListChunkWindow(parentID string, chunkIndex, radius int) []model.KnowledgeDocument {
	chunks := ListSiblingChunks(parentID)
	if len(chunks) == 0 {
		return nil
	}
	out := []model.KnowledgeDocument{}
	for _, chunk := range chunks {
		if chunk.ChunkIndex >= chunkIndex-radius && chunk.ChunkIndex <= chunkIndex+radius {
			out = append(out, chunk)
		}
	}
	return out
}

// ListDocuments 分页查询知识库文档，支持按类型、分类、关键词筛选。
func ListDocuments(page, limit int, docType, category, keyword string) ([]model.KnowledgeDocument, int) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if mysqlOK {
		return listDocsMySQL(page, limit, docType, category, keyword)
	}
	return listDocsMemory(page, limit, docType, category, keyword)
}

func listDocsMySQL(page, limit int, docType, category, keyword string) ([]model.KnowledgeDocument, int) {
	where, args := buildDocWhere(docType, category, keyword)
	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM knowledge_documents"+where, args...).Scan(&total); err != nil {
		return []model.KnowledgeDocument{}, 0
	}
	offset := (page - 1) * limit
	args = append(args, limit, offset)
	rows, err := db.Query(
		selectDocsSQL()+where+" ORDER BY updated_at DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return []model.KnowledgeDocument{}, 0
	}
	defer rows.Close()
	return scanDocRows(rows), total
}

func selectDocsSQL() string {
	return "SELECT id,title,content,doc_type,COALESCE(file_type,''),COALESCE(file_name,''),file_size,COALESCE(category,''),COALESCE(tags,''),COALESCE(source_id,''),COALESCE(embedding_model,''),chunk_index,COALESCE(parent_id,''),created_at,updated_at FROM knowledge_documents"
}

func buildDocWhere(docType, category, keyword string) (string, []any) {
	var clauses []string
	var args []any
	if docType != "" {
		clauses = append(clauses, "doc_type=?")
		args = append(args, docType)
	}
	if category != "" {
		clauses = append(clauses, "category=?")
		args = append(args, category)
	}
	if keyword != "" {
		clauses = append(clauses, "MATCH(title,content) AGAINST(? IN BOOLEAN MODE)")
		args = append(args, keyword)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func scanDocRows(rows interface {
	Next() bool
	Scan(...any) error
}) []model.KnowledgeDocument {
	out := []model.KnowledgeDocument{}
	for rows.Next() {
		var doc model.KnowledgeDocument
		_ = rows.Scan(&doc.ID, &doc.Title, &doc.Content, &doc.DocType,
			&doc.FileType, &doc.FileName, &doc.FileSize, &doc.Category,
			&doc.Tags, &doc.SourceID, &doc.EmbeddingModel, &doc.ChunkIndex,
			&doc.ParentID, &doc.CreatedAt, &doc.UpdatedAt)
		out = append(out, doc)
	}
	return out
}

func listDocsMemory(page, limit int, docType, category, keyword string) ([]model.KnowledgeDocument, int) {
	mu.RLock()
	defer mu.RUnlock()
	kw := strings.ToLower(keyword)
	filtered := make([]model.KnowledgeDocument, 0, len(knowledgeDocs))
	for _, doc := range knowledgeDocs {
		if docType != "" && doc.DocType != docType {
			continue
		}
		if category != "" && doc.Category != category {
			continue
		}
		if kw != "" && !docMatchesKeyword(doc, kw) {
			continue
		}
		filtered = append(filtered, *doc)
	}
	total := len(filtered)
	start := (page - 1) * limit
	if start >= total {
		return []model.KnowledgeDocument{}, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return filtered[start:end], total
}

func docMatchesKeyword(doc *model.KnowledgeDocument, kw string) bool {
	return strings.Contains(strings.ToLower(doc.Title), kw) ||
		strings.Contains(strings.ToLower(doc.Content), kw) ||
		strings.Contains(strings.ToLower(doc.Tags), kw)
}

// ListAllDocuments 获取所有文档（用于重建索引）。
func ListAllDocuments() []model.KnowledgeDocument {
	if mysqlOK {
		rows, err := db.Query(selectDocsSQL() + " ORDER BY created_at")
		if err != nil {
			return []model.KnowledgeDocument{}
		}
		defer rows.Close()
		return scanDocRows(rows)
	}
	mu.RLock()
	defer mu.RUnlock()
	out := make([]model.KnowledgeDocument, 0, len(knowledgeDocs))
	for _, doc := range knowledgeDocs {
		out = append(out, *doc)
	}
	return out
}

// FindDocumentBySourceID 根据 source_id 查找文档（用于去重）。
func FindDocumentBySourceID(sourceID string) *model.KnowledgeDocument {
	if sourceID == "" {
		return nil
	}
	if mysqlOK {
		row := db.QueryRow("SELECT id FROM knowledge_documents WHERE source_id=? AND (parent_id='' OR parent_id IS NULL) LIMIT 1", sourceID)
		var id string
		if row.Scan(&id) == nil && id != "" {
			doc, _ := GetDocument(id)
			return doc
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, doc := range knowledgeDocs {
		if doc.SourceID == sourceID && doc.ParentID == "" {
			cp := *doc
			return &cp
		}
	}
	return nil
}
