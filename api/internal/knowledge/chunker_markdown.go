package knowledge

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var markdownHeadingPattern = regexp.MustCompile(`^(#{1,3})\s+(.+?)\s*$`)

type markdownSection struct {
	Headings []string
	Body     string
	Start    int
	End      int
}

func chunkMarkdown(content string, cfg ChunkConfig) []Chunk {
	cfg = normalizeConfig(cfg)
	sections := collectMarkdownSections(content)
	chunks := []Chunk{}
	for _, section := range sections {
		chunks = append(chunks, splitMarkdownSection(section, cfg)...)
	}
	return reindexChunks(chunks)
}

func collectMarkdownSections(content string) []markdownSection {
	lines := strings.SplitAfter(content, "\n")
	sections := []markdownSection{}
	headings := map[int]string{}
	body := strings.Builder{}
	start, offset := 0, 0
	for _, raw := range lines {
		if level, title, ok := parseMarkdownHeading(raw); ok {
			sections = appendMarkdownSection(sections, headings, body.String(), start, offset)
			updateMarkdownHeadings(headings, level, title)
			body.Reset()
			start = offset
		} else {
			body.WriteString(raw)
		}
		offset += len(raw)
	}
	return appendMarkdownSection(sections, headings, body.String(), start, len(content))
}

func parseMarkdownHeading(line string) (int, string, bool) {
	match := markdownHeadingPattern.FindStringSubmatch(strings.TrimSpace(line))
	if len(match) != 3 {
		return 0, "", false
	}
	return len(match[1]), strings.TrimSpace(match[2]), true
}

func updateMarkdownHeadings(headings map[int]string, level int, title string) {
	for i := level; i <= 3; i++ {
		delete(headings, i)
	}
	headings[level] = strings.Repeat("#", level) + " " + title
}

func appendMarkdownSection(items []markdownSection, headings map[int]string, body string, start, end int) []markdownSection {
	prefix := activeMarkdownHeadings(headings)
	if strings.TrimSpace(body) == "" && len(prefix) == 0 {
		return items
	}
	return append(items, markdownSection{Headings: prefix, Body: strings.TrimSpace(body), Start: start, End: end})
}

func activeMarkdownHeadings(headings map[int]string) []string {
	out := []string{}
	for i := 1; i <= 3; i++ {
		if title := headings[i]; title != "" {
			out = append(out, title)
		}
	}
	return out
}

func splitMarkdownSection(section markdownSection, cfg ChunkConfig) []Chunk {
	prefix := strings.Join(section.Headings, "\n")
	text := joinPrefixAndBody(prefix, section.Body)
	if utf8.RuneCountInString(text) <= cfg.MaxChunkSize {
		return []Chunk{{Content: text, Start: section.Start, End: section.End}}
	}
	return splitLongMarkdownSection(prefix, section, cfg)
}

func splitLongMarkdownSection(prefix string, section markdownSection, cfg ChunkConfig) []Chunk {
	bodyCfg := cfg
	bodyCfg.MaxChunkSize = cfg.MaxChunkSize - utf8.RuneCountInString(prefix) - 2
	if bodyCfg.MaxChunkSize < 200 {
		bodyCfg.MaxChunkSize = 200
	}
	chunks := mergeWithOverlap(splitByParagraph(section.Body, bodyCfg), bodyCfg)
	out := make([]Chunk, 0, len(chunks))
	for _, chunk := range chunks {
		out = append(out, Chunk{Content: joinPrefixAndBody(prefix, chunk.Content), Start: section.Start, End: section.End})
	}
	return out
}

func joinPrefixAndBody(prefix, body string) string {
	body = strings.TrimSpace(body)
	if prefix == "" {
		return body
	}
	if body == "" {
		return prefix
	}
	return prefix + "\n\n" + body
}
