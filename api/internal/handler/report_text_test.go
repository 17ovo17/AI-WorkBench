package handler

import "testing"

func TestNormalizeReportTextExtractsReportField(t *testing.T) {
	input := `{"status":"done","report":"# 诊断报告\n\n- CPU 正常","raw":"{}"}`
	got := normalizeReportText(input)
	if got != "# 诊断报告\n\n- CPU 正常" {
		t.Fatalf("unexpected report text: %q", got)
	}
}

func TestNormalizeReportTextKeepsMarkdown(t *testing.T) {
	input := "# 诊断报告\n\n- 已是 Markdown"
	if got := normalizeReportText(input); got != input {
		t.Fatalf("markdown should remain unchanged: %q", got)
	}
}
