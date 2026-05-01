package embedding

import (
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/go-ego/gse"
)

const (
	bm25K1 = 1.2  // BM25 词频饱和参数
	bm25B  = 0.75 // BM25 文档长度归一化参数
)

// BM25Engine 基于 BM25 算法的内置搜索引擎
type BM25Engine struct {
	mu    sync.RWMutex
	seg   gse.Segmenter
	docs  map[string]*indexedDoc // docID -> 文档
	idf   map[string]float64     // term -> IDF
	avgDL float64                // 平均文档长度（词数）
}

// indexedDoc 已索引的文档
type indexedDoc struct {
	ID         string
	Title      string
	Content    string
	DocType    string
	Category   string
	ParentID   string
	ChunkIndex int
	TitleTerms map[string]int
	Terms      map[string]int // term -> 词频
	Length     int            // 文档长度（词数）
}

// NewBM25Engine 创建 BM25 搜索引擎，初始化分词器
func NewBM25Engine() *BM25Engine {
	e := &BM25Engine{
		docs: make(map[string]*indexedDoc),
		idf:  make(map[string]float64),
	}
	e.seg.LoadDict()
	return e
}

// Index 索引单个文档：分词并建立倒排索引
func (e *BM25Engine) Index(doc Document) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	terms, titleTerms := e.documentTerms(doc)
	termFreq := buildTermFreq(terms)

	e.docs[doc.ID] = &indexedDoc{
		ID:         doc.ID,
		Title:      doc.Title,
		Content:    doc.Content,
		DocType:    doc.DocType,
		Category:   doc.Category,
		ParentID:   doc.ParentID,
		ChunkIndex: doc.ChunkIndex,
		TitleTerms: buildTermFreq(titleTerms),
		Terms:      termFreq,
		Length:     len(terms),
	}

	e.recomputeStats()
	return nil
}

// Remove 从索引中删除文档
func (e *BM25Engine) Remove(docID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.docs, docID)
	e.recomputeStats()
	return nil
}

// Search 使用 BM25 算法检索文档
func (e *BM25Engine) Search(query string, topK int) ([]SearchResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	expandedQuery := expandShortOpsQuery(query)
	queryTerms := e.expandQueryTerms(e.tokenize(expandedQuery), expandedQuery)
	if len(queryTerms) == 0 {
		return nil, nil
	}

	type scored struct {
		doc   *indexedDoc
		score float64
	}

	results := make([]scored, 0, len(e.docs))
	for _, doc := range e.docs {
		s := e.score(queryTerms, doc)
		if s > 0 {
			results = append(results, scored{doc: doc, score: s})
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

// Rebuild 全量重建索引
func (e *BM25Engine) Rebuild(docs []Document) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.docs = make(map[string]*indexedDoc, len(docs))
	for _, doc := range docs {
		terms, titleTerms := e.documentTerms(doc)
		termFreq := buildTermFreq(terms)

		e.docs[doc.ID] = &indexedDoc{
			ID:         doc.ID,
			Title:      doc.Title,
			Content:    doc.Content,
			DocType:    doc.DocType,
			Category:   doc.Category,
			ParentID:   doc.ParentID,
			ChunkIndex: doc.ChunkIndex,
			TitleTerms: buildTermFreq(titleTerms),
			Terms:      termFreq,
			Length:     len(terms),
		}
	}

	e.recomputeStats()
	return nil
}

func (e *BM25Engine) documentTerms(doc Document) ([]string, []string) {
	titleTerms := e.tokenize(doc.Title)
	contentTerms := e.tokenize(doc.Content)
	terms := append([]string{}, contentTerms...)
	for i := 0; i < 4; i++ {
		terms = append(terms, titleTerms...)
	}
	return terms, titleTerms
}

// tokenize 对文本进行中文分词，过滤停用词和标点
func (e *BM25Engine) tokenize(text string) []string {
	segments := e.seg.Cut(text, true)
	tokens := make([]string, 0, len(segments))
	for _, seg := range segments {
		w := strings.TrimSpace(seg)
		if w == "" {
			continue
		}
		if isStopWord(w) {
			continue
		}
		tokens = append(tokens, strings.ToLower(w))
	}
	for _, term := range domainSynonyms(strings.ToLower(text)) {
		tokens = append(tokens, strings.ToLower(term))
	}
	return tokens
}

func (e *BM25Engine) expandQueryTerms(terms []string, query string) []string {
	out := append([]string{}, terms...)
	for _, term := range domainSynonyms(strings.ToLower(query)) {
		out = append(out, strings.ToLower(term), strings.ToLower(term))
	}
	return out
}

// recomputeStats 重新计算 IDF 和平均文档长度
func (e *BM25Engine) recomputeStats() {
	n := float64(len(e.docs))
	if n == 0 {
		e.avgDL = 0
		e.idf = make(map[string]float64)
		return
	}

	// 统计每个 term 出现在多少文档中
	docFreq := make(map[string]int)
	totalLen := 0
	for _, doc := range e.docs {
		totalLen += doc.Length
		seen := make(map[string]bool)
		for term := range doc.Terms {
			if !seen[term] {
				docFreq[term]++
				seen[term] = true
			}
		}
	}

	e.avgDL = float64(totalLen) / n
	e.idf = make(map[string]float64, len(docFreq))
	for term, df := range docFreq {
		// IDF = ln((N - df + 0.5) / (df + 0.5) + 1)
		e.idf[term] = math.Log((n-float64(df)+0.5)/(float64(df)+0.5) + 1)
	}
}

// score 计算查询与文档的 BM25 得分
func (e *BM25Engine) score(queryTerms []string, doc *indexedDoc) float64 {
	s := 0.0
	if e.avgDL == 0 {
		return s
	}
	dl := float64(doc.Length)
	for _, qt := range queryTerms {
		idf, ok := e.idf[qt]
		if !ok {
			continue
		}
		tf := float64(doc.Terms[qt])
		numerator := tf * (bm25K1 + 1)
		denominator := tf + bm25K1*(1-bm25B+bm25B*dl/e.avgDL)
		s += idf * numerator / denominator
		if doc.TitleTerms[qt] > 0 {
			s += idf * 2.5 * float64(doc.TitleTerms[qt])
		}
	}
	return s
}

// buildTermFreq 统计词频
func buildTermFreq(terms []string) map[string]int {
	freq := make(map[string]int, len(terms))
	for _, t := range terms {
		freq[t]++
	}
	return freq
}

// isStopWord 判断是否为停用词或纯标点
func isStopWord(w string) bool {
	if len([]rune(w)) == 1 {
		r := []rune(w)[0]
		if unicode.IsPunct(r) || unicode.IsSpace(r) || unicode.IsSymbol(r) {
			return true
		}
	}
	// 常见中文停用词
	stops := map[string]bool{
		"的": true, "了": true, "在": true, "是": true, "我": true,
		"有": true, "和": true, "就": true, "不": true, "人": true,
		"都": true, "一": true, "一个": true, "上": true, "也": true,
		"很": true, "到": true, "说": true, "要": true, "去": true,
		"你": true, "会": true, "着": true, "没有": true, "看": true,
		"好": true, "自己": true, "这": true, "他": true, "她": true,
		"它": true, "the": true, "a": true, "an": true, "is": true,
		"are": true, "was": true, "were": true, "be": true, "been": true,
		"of": true, "and": true, "in": true, "to": true, "for": true,
		"with": true, "on": true, "at": true, "by": true, "from": true,
	}
	return stops[w]
}
