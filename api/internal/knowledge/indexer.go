package knowledge

import (
	"encoding/json"
	"time"

	"ai-workbench-api/internal/embedding"
	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/sirupsen/logrus"
)

// IndexDocument 索引一个文档：分块 + 存储。
// 向量化由后续 embedding 包异步处理，此处仅完成分块入库。
func IndexDocument(doc *model.KnowledgeDocument) error {
	cfg := ChunkConfig{
		MaxChunkSize: defaultMaxChunkSize,
		Overlap:      defaultOverlap,
	}
	chunks := ChunkDocumentByTypeAndFile(doc.Content, doc.DocType, doc.FileType, cfg)
	if len(chunks) <= 1 {
		store.SaveDocument(doc)
		indexToSearchEngine(doc)
		return nil
	}
	return indexChunks(doc, chunks)
}

// indexChunks 将分块后的文档逐块存储，parent_id 指向原文档。
func indexChunks(doc *model.KnowledgeDocument, chunks []Chunk) error {
	// 保存原文档（chunk_index = -1 表示原文）
	parent := *doc
	parent.ChunkIndex = -1
	store.SaveDocument(&parent)

	now := time.Now()
	for _, chunk := range chunks {
		chunkDoc := model.KnowledgeDocument{
			ID:             store.NewID(),
			Title:          doc.Title,
			Content:        chunk.Content,
			DocType:        doc.DocType,
			FileType:       doc.FileType,
			FileName:       doc.FileName,
			FileSize:       doc.FileSize,
			Category:       doc.Category,
			Tags:           doc.Tags,
			SourceID:       doc.SourceID,
			EmbeddingModel: doc.EmbeddingModel,
			ChunkIndex:     chunk.Index,
			ParentID:       doc.ID,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		store.SaveDocument(&chunkDoc)
	}
	indexChunksToSearchEngine(doc.ID)
	logrus.Infof("文档 %s 已分块索引，共 %d 块", doc.ID, len(chunks))
	return nil
}

// RemoveDocument 删除文档及其所有子块。
func RemoveDocument(docID string) error {
	removeFromSearchEngine(docID)
	store.DeleteDocumentsByParent(docID)
	store.DeleteDocument(docID)
	logrus.Infof("文档 %s 及其子块已删除", docID)
	return nil
}

// ReindexDocument 重建单个文档的分块和搜索索引。
func ReindexDocument(doc *model.KnowledgeDocument) error {
	target := normalizeReindexTarget(doc)
	removeFromSearchEngine(target.ID)
	store.DeleteDocumentsByParent(target.ID)
	return IndexDocument(target)
}

func normalizeReindexTarget(doc *model.KnowledgeDocument) *model.KnowledgeDocument {
	target := *doc
	if doc.ParentID != "" {
		if parent, ok := store.GetDocument(doc.ParentID); ok {
			target = *parent
		}
	}
	target.ParentID = ""
	target.ChunkIndex = 0
	return &target
}

// RebuildAll 全量重建索引：同步案例和 Runbook 到文档层，然后重建分块和搜索引擎索引。
func RebuildAll() error {
	syncAllCasesAndRunbooks()
	docs := store.ListAllDocuments()

	rebuilt := 0
	for i := range docs {
		if docs[i].ParentID != "" {
			continue
		}
		store.DeleteDocumentsByParent(docs[i].ID)
		doc := docs[i]
		if err := indexDocumentChunksOnly(&doc); err != nil {
			logrus.Warnf("重建文档 %s 索引失败: %v", doc.ID, err)
			continue
		}
		rebuilt++
	}
	rebuildSearchEngineFromChunks()
	logrus.Infof("全量重建索引完成，共处理 %d 个文档", rebuilt)
	return nil
}

func syncAllCasesAndRunbooks() {
	cases, _ := store.ListCases(1, 1000, "", "")
	synced := 0
	for i := range cases {
		c := cases[i]
		existing := store.FindDocumentBySourceID(c.ID)
		if existing != nil {
			continue
		}
		if err := SyncCaseToDocument(&c); err != nil {
			logrus.Warnf("同步案例 %s 到文档层失败: %v", c.ID, err)
			continue
		}
		synced++
	}

	runbooks, _ := store.ListRunbooks("", 1, 1000)
	for i := range runbooks {
		r := runbooks[i]
		existing := store.FindDocumentBySourceID(r.ID)
		if existing != nil {
			continue
		}
		if err := SyncRunbookToDocument(&r); err != nil {
			logrus.Warnf("同步 Runbook %s 到文档层失败: %v", r.ID, err)
			continue
		}
		synced++
	}
	logrus.Infof("同步案例和 Runbook 到文档层完成，新增 %d 条", synced)
}

// SyncCaseToDocument 将 DiagnosisCase 同步为 KnowledgeDocument。
func SyncCaseToDocument(c *model.DiagnosisCase) error {
	content := buildCaseContent(c)
	doc := &model.KnowledgeDocument{
		ID:        store.NewID(),
		Title:     "案例: " + c.RootCauseCategory + " - " + truncate(c.RootCauseDescription, 50),
		Content:   content,
		DocType:   "case",
		FileType:  "md",
		Category:  c.RootCauseCategory,
		Tags:      c.Keywords,
		SourceID:  c.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := IndexDocument(doc); err != nil {
		return err
	}
	return nil
}

func buildCaseContent(c *model.DiagnosisCase) string {
	snapshot := string(c.MetricSnapshot)
	if snapshot == "" || snapshot == "null" {
		snapshot = "{}"
	}
	var pretty []byte
	var raw json.RawMessage
	if json.Unmarshal(c.MetricSnapshot, &raw) == nil {
		if p, err := json.MarshalIndent(raw, "", "  "); err == nil {
			pretty = p
		}
	}
	if pretty == nil {
		pretty = c.MetricSnapshot
	}
	return "# 根因分类\n" + c.RootCauseCategory + "\n\n" +
		"# 根因描述\n" + c.RootCauseDescription + "\n\n" +
		"# 处置方案\n" + c.TreatmentSteps + "\n\n" +
		"# 指标快照\n```json\n" + string(pretty) + "\n```"
}

// SyncRunbookToDocument 将 Runbook 同步为 KnowledgeDocument。
func SyncRunbookToDocument(r *model.Runbook) error {
	content := buildRunbookContent(r)
	doc := &model.KnowledgeDocument{
		ID:        store.NewID(),
		Title:     "Runbook: " + r.Title,
		Content:   content,
		DocType:   "runbook",
		FileType:  "md",
		Category:  r.Category,
		SourceID:  r.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := IndexDocument(doc); err != nil {
		return err
	}
	return nil
}

func buildRunbookContent(r *model.Runbook) string {
	triggers := string(r.TriggerConditions)
	if triggers == "" || triggers == "null" {
		triggers = "{}"
	}
	return "# " + r.Title + "\n\n" +
		"## 分类\n" + r.Category + "\n\n" +
		"## 触发条件\n```json\n" + triggers + "\n```\n\n" +
		"## 处置步骤\n" + r.Steps
}

// truncate 截断字符串到指定长度。
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// LoadExistingDocsToSearchEngine 启动时将已有文档加载到搜索引擎。
func LoadExistingDocsToSearchEngine() {
	searcher := embedding.GetSearcher()
	if searcher == nil {
		return
	}
	EnsureDomainSeedDocuments()
	docs := store.ListAllDocuments()
	loaded := 0
	embDocs := make([]embedding.Document, 0, len(docs))
	for i := range docs {
		if docs[i].ChunkIndex == -1 {
			continue
		}
		embDocs = append(embDocs, toEmbeddingDocument(&docs[i]))
		loaded++
	}
	if err := searcher.Rebuild(embDocs); err != nil {
		logrus.WithError(err).Warn("加载文档到搜索引擎失败")
	}
	logrus.Infof("已加载 %d 个文档到搜索引擎", loaded)
}

// indexDocumentChunksOnly 仅做分块入库，不触发搜索引擎索引（RebuildAll 专用）。
func indexDocumentChunksOnly(doc *model.KnowledgeDocument) error {
	cfg := ChunkConfig{MaxChunkSize: defaultMaxChunkSize, Overlap: defaultOverlap}
	chunks := ChunkDocumentByTypeAndFile(doc.Content, doc.DocType, doc.FileType, cfg)
	if len(chunks) <= 1 {
		store.SaveDocument(doc)
		return nil
	}
	parent := *doc
	parent.ChunkIndex = -1
	store.SaveDocument(&parent)
	now := time.Now()
	for _, chunk := range chunks {
		chunkDoc := model.KnowledgeDocument{
			ID: store.NewID(), Title: doc.Title, Content: chunk.Content,
			DocType: doc.DocType, FileType: doc.FileType, FileName: doc.FileName,
			FileSize: doc.FileSize, Category: doc.Category, Tags: doc.Tags,
			SourceID: doc.SourceID, EmbeddingModel: doc.EmbeddingModel,
			ChunkIndex: chunk.Index, ParentID: doc.ID, CreatedAt: now, UpdatedAt: now,
		}
		store.SaveDocument(&chunkDoc)
	}
	return nil
}

// indexToSearchEngine 将文档索引到混合搜索引擎
func indexToSearchEngine(doc *model.KnowledgeDocument) {
	searcher := embedding.GetSearcher()
	if searcher == nil {
		return
	}
	embDoc := toEmbeddingDocument(doc)
	if err := searcher.Index(embDoc); err != nil {
		logrus.WithError(err).Warnf("knowledge: index doc %s to search engine failed", doc.ID)
	}
}

func removeFromSearchEngine(docID string) {
	searcher := embedding.GetSearcher()
	if searcher == nil {
		return
	}
	for _, chunk := range store.ListSiblingChunks(docID) {
		removeIndexedDoc(searcher, chunk.ID)
	}
	removeIndexedDoc(searcher, docID)
}

func removeIndexedDoc(searcher *embedding.HybridSearcher, docID string) {
	if err := searcher.Remove(docID); err != nil {
		logrus.WithError(err).Warnf("knowledge: remove doc %s from search engine failed", docID)
	}
}

func indexChunksToSearchEngine(parentID string) {
	chunks := store.ListSiblingChunks(parentID)
	if len(chunks) == 0 {
		return
	}
	for i := range chunks {
		indexToSearchEngine(&chunks[i])
	}
}

func rebuildSearchEngineFromChunks() {
	searcher := embedding.GetSearcher()
	if searcher == nil {
		return
	}
	docs := store.ListAllDocuments()
	embDocs := make([]embedding.Document, 0, len(docs))
	for i := range docs {
		if docs[i].ChunkIndex == -1 {
			continue
		}
		embDocs = append(embDocs, toEmbeddingDocument(&docs[i]))
	}
	if err := searcher.Rebuild(embDocs); err != nil {
		logrus.WithError(err).Warn("重建搜索引擎索引失败")
	}
}

func toEmbeddingDocument(doc *model.KnowledgeDocument) embedding.Document {
	return embedding.Document{
		ID:         doc.ID,
		Title:      doc.Title,
		Content:    doc.Content,
		DocType:    doc.DocType,
		Category:   doc.Category,
		ParentID:   doc.ParentID,
		ChunkIndex: doc.ChunkIndex,
	}
}
