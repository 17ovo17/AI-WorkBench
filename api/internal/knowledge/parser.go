package knowledge

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// ParseDocument 根据文件类型解析文档内容，返回纯文本。
func ParseDocument(fileName string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".md", ".markdown":
		return parseMD(data), nil
	case ".txt", ".log", ".csv":
		return parseTXT(data), nil
	case ".pdf":
		return parsePDF(data)
	case ".docx":
		return parseDOCX(data)
	case ".html", ".htm":
		return parseHTML(data), nil
	default:
		return "", fmt.Errorf("不支持的文件类型: %s", ext)
	}
}

// FileTypeFromName 从文件名提取文件类型标识。
func FileTypeFromName(fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	ext = strings.TrimPrefix(ext, ".")
	switch ext {
	case "markdown":
		return "md"
	case "htm":
		return "html"
	case "log":
		return "txt"
	default:
		return ext
	}
}

// parseMD 直接返回 Markdown 内容。
func parseMD(data []byte) string {
	return strings.TrimSpace(string(data))
}

// parseTXT 直接返回纯文本内容。
func parseTXT(data []byte) string {
	return strings.TrimSpace(string(data))
}

// parsePDF 简单提取 PDF 中的文本流。
// 不依赖 pdfcpu，仅做基础文本流提取；复杂 PDF 可能提取不完整。
func parsePDF(data []byte) (string, error) {
	content := string(data)
	var result strings.Builder
	extracted := extractPDFTextStreams(content, &result)
	if !extracted {
		return "", fmt.Errorf("PDF 文本提取失败：未找到可解析的文本流，建议转换为 TXT 或 MD 后上传")
	}
	text := strings.TrimSpace(result.String())
	if text == "" {
		return "", fmt.Errorf("PDF 文本提取为空：文档可能为扫描件或纯图片 PDF")
	}
	return text, nil
}

// extractPDFTextStreams 从 PDF 原始内容中提取 BT...ET 文本块。
func extractPDFTextStreams(content string, result *strings.Builder) bool {
	found := false
	re := regexp.MustCompile(`BT\s([\s\S]*?)ET`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		text := extractTextFromBT(m[1])
		if text != "" {
			found = true
			result.WriteString(text)
			result.WriteString("\n")
		}
	}
	return found
}

// extractTextFromBT 从 BT 块中提取括号内的文本。
func extractTextFromBT(block string) string {
	re := regexp.MustCompile(`\(([^)]*)\)`)
	matches := re.FindAllStringSubmatch(block, -1)
	var parts []string
	for _, m := range matches {
		if len(m) >= 2 && strings.TrimSpace(m[1]) != "" {
			parts = append(parts, m[1])
		}
	}
	return strings.Join(parts, " ")
}

// parseDOCX 解析 Word 文档，用 archive/zip 解压 + 解析 word/document.xml。
func parseDOCX(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("DOCX 解压失败: %w", err)
	}
	for _, f := range reader.File {
		if f.Name != "word/document.xml" {
			continue
		}
		return extractDOCXText(f)
	}
	return "", fmt.Errorf("DOCX 中未找到 word/document.xml")
}

// extractDOCXText 从 word/document.xml 中提取纯文本。
func extractDOCXText(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", fmt.Errorf("打开 document.xml 失败: %w", err)
	}
	defer rc.Close()

	var result strings.Builder
	decoder := xml.NewDecoder(rc)
	var inText bool
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			inText = isDocxTextElement(t)
		case xml.EndElement:
			if t.Name.Local == "p" {
				result.WriteString("\n")
			}
			if t.Name.Local == "t" {
				inText = false
			}
		case xml.CharData:
			if inText {
				result.Write(t)
			}
		}
	}
	return strings.TrimSpace(result.String()), nil
}

// isDocxTextElement 判断是否为 DOCX 文本元素。
func isDocxTextElement(el xml.StartElement) bool {
	return el.Name.Local == "t"
}

// parseHTML 提取 HTML 纯文本，去除标签。
func parseHTML(data []byte) string {
	text := string(data)
	text = stripHTMLComments(text)
	text = stripHTMLTags(text)
	text = decodeHTMLEntities(text)
	// 合并多余空行
	re := regexp.MustCompile(`\n{3,}`)
	text = re.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

// stripHTMLComments 去除 HTML 注释。
func stripHTMLComments(s string) string {
	re := regexp.MustCompile(`<!--[\s\S]*?-->`)
	return re.ReplaceAllString(s, "")
}

// stripHTMLTags 去除 HTML 标签，块级标签替换为换行。
func stripHTMLTags(s string) string {
	blockRe := regexp.MustCompile(`</(p|div|br|h[1-6]|li|tr|blockquote)[^>]*>`)
	s = blockRe.ReplaceAllString(s, "\n")
	tagRe := regexp.MustCompile(`<[^>]+>`)
	return tagRe.ReplaceAllString(s, "")
}

// decodeHTMLEntities 解码常见 HTML 实体。
func decodeHTMLEntities(s string) string {
	r := strings.NewReplacer(
		"&amp;", "&", "&lt;", "<", "&gt;", ">",
		"&quot;", "\"", "&#39;", "'", "&nbsp;", " ",
	)
	return r.Replace(s)
}
