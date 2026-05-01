package knowledge

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	defaultMaxChunkSize = 800
	defaultOverlap      = 100
	defaultSeparator    = "\n\n"
)

// ChunkConfig 分块配置。
type ChunkConfig struct {
	MaxChunkSize int    // 每块最大字符数，默认 800
	Overlap      int    // 重叠字符数，默认 100
	Separator    string // 分隔符，默认按段落
}

// Chunk 表示文档的一个分块。
type Chunk struct {
	Index   int    `json:"index"`
	Content string `json:"content"`
	Start   int    `json:"start"` // 在原文中的起始位置（字符）
	End     int    `json:"end"`   // 在原文中的结束位置（字符）
}

// ChunkDocument 将长文档分成多个块。
// 分块策略：先按段落分割，超长段落按行再分，超长行按句子或词边界切分。
func ChunkDocument(content string, cfg ChunkConfig) []Chunk {
	cfg = normalizeConfig(cfg)
	if utf8.RuneCountInString(content) <= cfg.MaxChunkSize {
		return []Chunk{{Index: 0, Content: content, Start: 0, End: len(content)}}
	}
	segments := splitByParagraph(content, cfg)
	return mergeWithOverlap(segments, cfg)
}

func normalizeConfig(cfg ChunkConfig) ChunkConfig {
	if cfg.MaxChunkSize <= 0 {
		cfg.MaxChunkSize = defaultMaxChunkSize
	}
	if cfg.Overlap < 0 {
		cfg.Overlap = defaultOverlap
	}
	if cfg.Overlap >= cfg.MaxChunkSize {
		cfg.Overlap = cfg.MaxChunkSize / 5
	}
	if cfg.Separator == "" {
		cfg.Separator = defaultSeparator
	}
	return cfg
}

// splitByParagraph 按段落分割，超长段落按行再分，超长行硬切。
func splitByParagraph(content string, cfg ChunkConfig) []string {
	paragraphs := strings.Split(content, cfg.Separator)
	var segments []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if utf8.RuneCountInString(p) <= cfg.MaxChunkSize {
			segments = append(segments, p)
			continue
		}
		segments = append(segments, splitByLine(p, cfg)...)
	}
	return segments
}

// splitByLine 按行分割超长段落。
func splitByLine(paragraph string, cfg ChunkConfig) []string {
	lines := strings.Split(paragraph, "\n")
	var segments []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) <= cfg.MaxChunkSize {
			segments = append(segments, line)
			continue
		}
		segments = append(segments, hardSplit(line, cfg.MaxChunkSize)...)
	}
	return segments
}

// hardSplit 按 MaxChunkSize 硬切，尽量保留完整词。
func hardSplit(text string, maxSize int) []string {
	runes := []rune(text)
	var segments []string
	for len(runes) > 0 {
		end := maxSize
		if end > len(runes) {
			end = len(runes)
		}
		if end < len(runes) {
			end = findChunkBoundary(runes, end)
		}
		segments = append(segments, string(runes[:end]))
		runes = runes[end:]
	}
	return segments
}

func findChunkBoundary(runes []rune, pos int) int {
	if boundary := findSentenceBoundary(runes, pos); boundary > 0 {
		return boundary
	}
	return findWordBoundary(runes, pos)
}

func findSentenceBoundary(runes []rune, pos int) int {
	for i := pos; i > pos-120 && i > 0; i-- {
		if strings.ContainsRune("。！？.!?", runes[i-1]) {
			return i
		}
	}
	return 0
}

// findWordBoundary 在 runes 中从 pos 向前找空格边界。
func findWordBoundary(runes []rune, pos int) int {
	for i := pos; i > pos-50 && i > 0; i-- {
		if runes[i] == ' ' || runes[i] == '\t' {
			return i + 1
		}
	}
	return pos
}

// mergeWithOverlap 将小段合并为不超过 MaxChunkSize 的块，相邻块有重叠。
func mergeWithOverlap(segments []string, cfg ChunkConfig) []Chunk {
	var chunks []Chunk
	var buf strings.Builder
	byteOffset := 0
	chunkStart := 0
	idx := 0

	for _, seg := range segments {
		segRunes := utf8.RuneCountInString(seg)
		bufRunes := utf8.RuneCountInString(buf.String())
		if bufRunes > 0 && bufRunes+1+segRunes > cfg.MaxChunkSize {
			text := buf.String()
			chunks = append(chunks, Chunk{
				Index:   idx,
				Content: text,
				Start:   chunkStart,
				End:     chunkStart + len(text),
			})
			idx++
			overlap := extractOverlap(text, cfg.Overlap)
			byteOffset = chunkStart + len(text) - len(overlap)
			chunkStart = byteOffset
			buf.Reset()
			buf.WriteString(overlap)
		}
		if buf.Len() > 0 {
			buf.WriteString("\n\n")
		}
		buf.WriteString(seg)
	}
	if buf.Len() > 0 {
		text := buf.String()
		chunks = append(chunks, Chunk{
			Index:   idx,
			Content: text,
			Start:   chunkStart,
			End:     chunkStart + len(text),
		})
	}
	return chunks
}

// extractOverlap 从文本末尾提取 overlap 个字符的重叠内容，优先在句子边界开始。
func extractOverlap(text string, overlap int) string {
	runes := []rune(text)
	if len(runes) <= overlap {
		return text
	}
	start := len(runes) - overlap
	window := string(runes[start:])
	if cut := sentenceOverlapCut(window); cut > 0 {
		return strings.TrimSpace(window[cut:])
	}
	return strings.TrimSpace(string(runes[start:]))
}

var (
	stepPattern        = regexp.MustCompile(`(?m)^(\d+[\.\)、]|步骤\s*\d+|Step\s*\d+)`)
	sentenceEndPattern = regexp.MustCompile(`[。！？.!?]\s*`)
)

func ChunkDocumentByType(content string, docType string, cfg ChunkConfig) []Chunk {
	return ChunkDocumentByTypeAndFile(content, docType, "", cfg)
}

func ChunkDocumentByTypeAndFile(content string, docType, fileType string, cfg ChunkConfig) []Chunk {
	strategy := normalizeChunkStrategy(docType, fileType)
	if strategy == "runbook" {
		chunks := chunkBySteps(content, cfg)
		if len(chunks) > 1 {
			return chunks
		}
	}
	if strategy == "json" || strategy == "yaml" {
		chunks := chunkByTopLevelKeys(content, strategy, cfg)
		if len(chunks) > 0 {
			return chunks
		}
	}
	if strategy == "markdown" {
		chunks := chunkMarkdown(content, cfg)
		if len(chunks) > 0 {
			return chunks
		}
	}
	return ChunkDocument(content, cfg)
}

func normalizeChunkStrategy(docType, fileType string) string {
	docType = strings.ToLower(strings.TrimSpace(docType))
	fileType = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(fileType), "."))
	if docType == "runbook" {
		return "runbook"
	}
	if docType == "md" || docType == "markdown" || fileType == "md" || fileType == "markdown" {
		return "markdown"
	}
	if docType == "json" || fileType == "json" {
		return "json"
	}
	if docType == "yaml" || docType == "yml" || fileType == "yaml" || fileType == "yml" {
		return "yaml"
	}
	return "default"
}

func chunkBySteps(content string, cfg ChunkConfig) []Chunk {
	cfg = normalizeConfig(cfg)
	indices := stepPattern.FindAllStringIndex(content, -1)
	if len(indices) < 2 {
		return nil
	}
	var chunks []Chunk
	for i, idx := range indices {
		end := len(content)
		if i+1 < len(indices) {
			end = indices[i+1][0]
		}
		text := strings.TrimSpace(content[idx[0]:end])
		if len(text) > 0 {
			chunks = append(chunks, Chunk{Content: text, Index: i, Start: idx[0], End: end})
		}
	}
	return chunks
}

func sentenceOverlapCut(window string) int {
	matches := sentenceEndPattern.FindAllStringIndex(window, -1)
	for _, m := range matches {
		if len([]rune(window[m[1]:])) >= 20 {
			return m[1]
		}
	}
	return 0
}

func reindexChunks(chunks []Chunk) []Chunk {
	for i := range chunks {
		chunks[i].Index = i
	}
	return chunks
}
