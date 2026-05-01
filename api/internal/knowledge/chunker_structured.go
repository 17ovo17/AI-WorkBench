package knowledge

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var yamlTopKeyPattern = regexp.MustCompile(`(?m)^[A-Za-z0-9_.-]+\s*:`)

func chunkByTopLevelKeys(content, strategy string, cfg ChunkConfig) []Chunk {
	cfg = normalizeConfig(cfg)
	if strategy == "json" {
		return chunkJSONTopKeys(content, cfg)
	}
	indices := yamlTopKeyPattern.FindAllStringIndex(content, -1)
	if len(indices) == 0 {
		return nil
	}
	return chunkByRanges(content, indices, cfg)
}

func chunkJSONTopKeys(content string, cfg ChunkConfig) []Chunk {
	var obj map[string]json.RawMessage
	if json.Unmarshal([]byte(content), &obj) != nil || len(obj) == 0 {
		return nil
	}
	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return buildJSONKeyChunks(keys, obj, cfg)
}

func buildJSONKeyChunks(keys []string, obj map[string]json.RawMessage, cfg ChunkConfig) []Chunk {
	chunks := []Chunk{}
	for _, key := range keys {
		body := string(obj[key])
		var pretty bytes.Buffer
		if json.Indent(&pretty, obj[key], "", "  ") == nil {
			body = pretty.String()
		}
		text := key + ":\n```json\n" + body + "\n```"
		chunks = appendTextChunks(chunks, text, cfg)
	}
	return reindexChunks(chunks)
}

func chunkByRanges(content string, ranges [][]int, cfg ChunkConfig) []Chunk {
	chunks := []Chunk{}
	for i, r := range ranges {
		end := len(content)
		if i+1 < len(ranges) {
			end = ranges[i+1][0]
		}
		text := strings.TrimSpace(content[r[0]:end])
		chunks = appendTextChunks(chunks, text, cfg)
	}
	return reindexChunks(chunks)
}

func appendTextChunks(chunks []Chunk, text string, cfg ChunkConfig) []Chunk {
	if text == "" {
		return chunks
	}
	if utf8.RuneCountInString(text) <= cfg.MaxChunkSize {
		return append(chunks, Chunk{Content: text})
	}
	for _, c := range mergeWithOverlap(splitByParagraph(text, cfg), cfg) {
		chunks = append(chunks, Chunk{Content: c.Content})
	}
	return chunks
}
