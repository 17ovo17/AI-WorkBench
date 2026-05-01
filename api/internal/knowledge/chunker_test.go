package knowledge

import (
	"strings"
	"testing"
)

func TestChunkMarkdownKeepsHeadingContext(t *testing.T) {
	content := "# CPU 排查\n总览\n\n## 使用率过高\n第一句。第二句。\n\n### 火焰图\n采集 pprof。"
	chunks := ChunkDocumentByTypeAndFile(content, "document", "md", ChunkConfig{MaxChunkSize: 80, Overlap: 20})
	if len(chunks) < 2 {
		t.Fatalf("expected markdown chunks, got %d", len(chunks))
	}
	if !strings.Contains(chunks[1].Content, "# CPU 排查") {
		t.Fatalf("chunk missing heading context: %q", chunks[1].Content)
	}
	if !strings.Contains(chunks[len(chunks)-1].Content, "### 火焰图") {
		t.Fatalf("nested heading missing: %q", chunks[len(chunks)-1].Content)
	}
}

func TestChunkStructuredByTopLevelKeys(t *testing.T) {
	jsonChunks := ChunkDocumentByTypeAndFile(`{"cpu":{"usage":95},"memory":{"used":80}}`, "document", "json", ChunkConfig{})
	if len(jsonChunks) != 2 || !strings.Contains(jsonChunks[0].Content, "cpu:") {
		t.Fatalf("unexpected json chunks: %#v", jsonChunks)
	}
	yamlChunks := ChunkDocumentByTypeAndFile("cpu:\n  usage: 95\nmemory:\n  used: 80\n", "document", "yaml", ChunkConfig{})
	if len(yamlChunks) != 2 || !strings.Contains(yamlChunks[1].Content, "memory:") {
		t.Fatalf("unexpected yaml chunks: %#v", yamlChunks)
	}
}
