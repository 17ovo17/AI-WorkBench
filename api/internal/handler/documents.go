package handler

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"ai-workbench-api/internal/embedding"
	"ai-workbench-api/internal/knowledge"
	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const maxUploadSize = 20 << 20 // 20MB

// docListResponse 文档分页列表响应。
type docListResponse struct {
	Items []model.KnowledgeDocument `json:"items"`
	Total int                       `json:"total"`
	Page  int                       `json:"page"`
	Limit int                       `json:"limit"`
}

// UploadDocument POST /api/v1/knowledge/documents/upload
// 接收 multipart/form-data，解析文件，分块，存储。
func UploadDocument(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件上传失败: " + err.Error()})
		return
	}
	defer file.Close()

	if header.Size > maxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件大小超过 20MB 限制"})
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败: " + err.Error()})
		return
	}

	content, err := knowledge.ParseDocument(header.Filename, data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件解析失败: " + err.Error()})
		return
	}

	doc := buildUploadDoc(c, header.Filename, int(header.Size), content)
	if err := knowledge.IndexDocument(doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "索引文档失败: " + err.Error()})
		return
	}

	auditEvent(c, "knowledge.document.upload", doc.ID, "low", "ok",
		"file="+header.Filename, c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, doc)
}

func buildUploadDoc(c *gin.Context, fileName string, fileSize int, content string) *model.KnowledgeDocument {
	now := time.Now()
	title := c.PostForm("title")
	if title == "" {
		title = fileName
	}
	return &model.KnowledgeDocument{
		ID:        store.NewID(),
		Title:     title,
		Content:   content,
		DocType:   c.DefaultPostForm("doc_type", "document"),
		FileType:  knowledge.FileTypeFromName(fileName),
		FileName:  fileName,
		FileSize:  fileSize,
		Category:  c.PostForm("category"),
		Tags:      c.PostForm("tags"),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// ListDocumentsHandler GET /api/v1/knowledge/documents
func ListDocumentsHandler(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	docType := c.Query("doc_type")
	category := c.Query("category")
	keyword := c.Query("keyword")
	items, total := store.ListDocuments(page, limit, docType, category, keyword)
	c.JSON(http.StatusOK, docListResponse{Items: items, Total: total, Page: page, Limit: limit})
}

// GetDocumentHandler GET /api/v1/knowledge/documents/:id
func GetDocumentHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	doc, ok := store.GetDocument(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}
	c.JSON(http.StatusOK, doc)
}

// DeleteDocumentHandler DELETE /api/v1/knowledge/documents/:id
func DeleteDocumentHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	if _, ok := store.GetDocument(id); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}
	if err := knowledge.RemoveDocument(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	auditEvent(c, "knowledge.document.delete", id, "medium", "ok", "", c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ReindexDocumentHandler POST /api/v1/knowledge/documents/:id/reindex
func ReindexDocumentHandler(c *gin.Context) {
	id := c.Param("id")
	doc, ok := store.GetDocument(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}
	if err := knowledge.ReindexDocument(doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	auditEvent(c, "knowledge.document.reindex", id, "low", "ok", "", c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// searchRequest 知识库搜索请求体。
type searchRequest struct {
	Query   string `json:"query"`
	TopK    int    `json:"top_k"`
	DocType string `json:"doc_type"`
}

type searchBadcaseRequest struct {
	Query  string `json:"query"`
	DocID  string `json:"doc_id"`
	Reason string `json:"reason"`
}

// SearchKnowledge POST /api/v1/knowledge/search
func SearchKnowledge(c *gin.Context) {
	var req searchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query required"})
		return
	}
	knowledge.EnsureDomainSeedDocuments()
	if req.TopK <= 0 || req.TopK > 50 {
		req.TopK = 5
	}

	searcher := embedding.GetSearcher()
	if searcher != nil {
		results, err := searcher.Search(c.Request.Context(), req.Query, searchLimit(req))
		if err != nil {
			logrus.WithError(err).Warn("hybrid search failed, fallback to fulltext")
		} else if len(results) > 0 {
			results = limitSearchResults(filterSearchResults(results, req.DocType), req.TopK)
			enriched := enrichSearchResults(results)
			recordKnowledgeSearch(req.Query, len(enriched), topSearchScore(enriched), "hybrid")
			c.JSON(http.StatusOK, gin.H{"items": enriched, "results": enriched, "total": len(enriched), "query": req.Query, "engine": "hybrid"})
			return
		}
	} else {
		logrus.Warn("search engine not initialized, using fulltext")
	}

	items, total := store.ListDocuments(1, req.TopK, req.DocType, "", req.Query)
	recordKnowledgeSearch(req.Query, len(items), topDocumentScore(items), "fulltext")
	c.JSON(http.StatusOK, gin.H{"items": items, "results": items, "total": total, "query": req.Query, "engine": "fulltext"})
}

// ReindexAllHandler POST /api/v1/knowledge/reindex-all
func ReindexAllHandler(c *gin.Context) {
	go func() {
		if err := knowledge.RebuildAll(); err != nil {
			logrus.Errorf("全量重建索引失败: %v", err)
		}
	}()
	auditEvent(c, "knowledge.document.reindex-all", "all", "medium", "ok", "", c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "全量重建索引已启动"})
}

type enrichedSearchResult struct {
	DocID       string         `json:"doc_id"`
	Title       string         `json:"title"`
	Content     string         `json:"content"`
	Score       float64        `json:"score"`
	DocType     string         `json:"doc_type"`
	Category    string         `json:"category"`
	ChunkIndex  int            `json:"chunk_index,omitempty"`
	ParentID    string         `json:"parent_id,omitempty"`
	ParentTitle string         `json:"parent_title,omitempty"`
	Context     []chunkContext `json:"context_chunks,omitempty"`
}

type chunkContext struct {
	DocID      string `json:"doc_id"`
	Content    string `json:"content"`
	ChunkIndex int    `json:"chunk_index"`
	Position   string `json:"position"`
}

func enrichSearchResults(results []embedding.SearchResult) []enrichedSearchResult {
	enriched := make([]enrichedSearchResult, 0, len(results))
	for _, r := range results {
		er := enrichedSearchResult{
			DocID: r.DocID, Title: r.Title, Content: r.Content,
			Score: r.Score, DocType: r.DocType, Category: r.Category,
			ParentID: r.ParentID, ChunkIndex: r.ChunkIndex,
		}
		if doc, ok := store.GetDocument(r.DocID); ok && doc.ParentID != "" {
			er.ParentID = doc.ParentID
			er.ChunkIndex = doc.ChunkIndex
			if parent, pok := store.GetDocument(doc.ParentID); pok {
				er.ParentTitle = parent.Title
			}
		}
		if er.ParentID != "" {
			er.Context = buildChunkContext(er.ParentID, er.ChunkIndex, er.DocID)
		}
		enriched = append(enriched, er)
	}
	return enriched
}

func searchLimit(req searchRequest) int {
	if req.DocType == "" {
		return req.TopK
	}
	limit := req.TopK * 4
	if limit < 20 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	return limit
}

func filterSearchResults(results []embedding.SearchResult, docType string) []embedding.SearchResult {
	if docType == "" {
		return results
	}
	filtered := make([]embedding.SearchResult, 0, len(results))
	for _, result := range results {
		if result.DocType == docType {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

func limitSearchResults(results []embedding.SearchResult, topK int) []embedding.SearchResult {
	if topK >= len(results) {
		return results
	}
	return results[:topK]
}

func buildChunkContext(parentID string, chunkIndex int, docID string) []chunkContext {
	window := store.ListChunkWindow(parentID, chunkIndex, 1)
	out := []chunkContext{}
	for _, chunk := range window {
		if chunk.ID == docID {
			continue
		}
		position := "next"
		if chunk.ChunkIndex < chunkIndex {
			position = "previous"
		}
		out = append(out, chunkContext{DocID: chunk.ID, Content: chunk.Content, ChunkIndex: chunk.ChunkIndex, Position: position})
	}
	return out
}

func recordKnowledgeSearch(query string, hitCount int, topScore float64, engine string) {
	store.AddKnowledgeSearchEvent(&model.KnowledgeSearchEvent{Query: query, HitCount: hitCount, TopScore: topScore, Engine: engine})
}

func topSearchScore(items []enrichedSearchResult) float64 {
	if len(items) == 0 {
		return 0
	}
	return items[0].Score
}

func topDocumentScore(items []model.KnowledgeDocument) float64 {
	if len(items) == 0 {
		return 0
	}
	return 1
}

// KnowledgeSearchStats GET /api/v1/knowledge/search/stats
func KnowledgeSearchStats(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	c.JSON(http.StatusOK, store.KnowledgeSearchStats(limit))
}

// SubmitSearchBadcase POST /api/v1/knowledge/search/badcase
func SubmitSearchBadcase(c *gin.Context) {
	var req searchBadcaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Query == "" || req.DocID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query and doc_id required"})
		return
	}
	store.AddKnowledgeSearchBadcase(&model.KnowledgeSearchBadcase{Query: req.Query, DocID: req.DocID, Reason: req.Reason, CreatedBy: currentOperator(c)})
	auditEvent(c, "knowledge.search.badcase", req.DocID, "low", "ok", "query="+req.Query, c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
