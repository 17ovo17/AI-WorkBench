package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func TestModelsReturnsConfiguredModelsOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5.4"}]}`))
	}))
	defer upstream.Close()
	viper.Reset()
	viper.Set("ai.base_url", upstream.URL+"/v1")
	viper.Set("ai.api_key", "test-key")
	viper.Set("ai_providers", []map[string]any{{"id": "p1", "name": "test", "models": []string{"gpt-5.5", "gpt-5.5-diagnose"}, "default": true}})
	t.Cleanup(viper.Reset)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	Models(c)

	if upstreamCalled {
		t.Fatal("/models 不应请求上游模型列表")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "gpt-5.5") || !strings.Contains(body, "gpt-5.5-diagnose") {
		t.Fatalf("response missing configured models: %s", body)
	}
	if strings.Contains(body, "gpt-5.4") {
		t.Fatalf("response leaked upstream model: %s", body)
	}
}

func TestAutoAdaptUsesDefaultModel(t *testing.T) {
	var requestedModel string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		requestedModel = payload.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"[{\"raw_name\":\"cpu_usage\",\"exporter\":\"node\",\"standard_name\":\"cpu_usage\",\"description\":\"CPU使用率\",\"transform\":\"none\"}]"}}]}`))
	}))
	defer upstream.Close()
	viper.Reset()
	viper.Set("ai.base_url", upstream.URL+"/v1")
	viper.Set("ai.api_key", "test-key")
	viper.Set("ai_providers", []map[string]any{{"id": "p1", "name": "test", "models": []string{"gpt-5.5"}, "default": true}, {"id": "p2", "name": "backup", "models": []string{"gpt-5.4"}}})
	t.Cleanup(viper.Reset)

	_, err := callLLMForAdapt("请返回 JSON", []model.MetricsMapping{{RawName: "cpu_usage"}})
	if err != nil {
		t.Fatalf("callLLMForAdapt failed: %v", err)
	}
	if requestedModel != "gpt-5.5" {
		t.Fatalf("expected default model gpt-5.5, got %q", requestedModel)
	}
}

func TestParseAdaptJSONSupportsMarkdownFence(t *testing.T) {
	content := "模型输出如下：\n```json\n[{\"raw_name\":\"cpu_usage\",\"exporter\":\"categraf\",\"standard_name\":\"host.cpu.usage\",\"description\":\"CPU 使用率\",\"promql\":\"cpu_usage{instance=\\\"{ip}:9100\\\"}\"}]\n```"
	results, err := parseAdaptJSON(content)
	if err != nil {
		t.Fatalf("parseAdaptJSON failed: %v", err)
	}
	if len(results) != 1 || results[0].RawName != "cpu_usage" || results[0].Transform == "" {
		t.Fatalf("unexpected parse result: %+v", results)
	}
}

func TestMappedPromQLUsesPrometheusPlaceholder(t *testing.T) {
	query := mappedPromQL("cpu_usage_active", `cpu_usage_active{instance="{ip}:9100"}`, `ident="10.0.0.1"`)
	if !strings.Contains(query, `10.0.0.1:9100`) || strings.Contains(query, "{ip}") {
		t.Fatalf("placeholder not applied: %s", query)
	}
}

func TestInspectionMetricPromQLPrefersDirectMetric(t *testing.T) {
	spec := inspectionMetricSpec{Metric: "cpu_usage_active"}
	target := promTarget{LabelKey: "ident", LabelVal: "10.10.1.11", Metrics: []string{"cpu_usage_active", "node_cpu_seconds_total"}}
	query := inspectionMetricPromQL(spec, target)
	if query != `max(cpu_usage_active{ident="10.10.1.11"})` {
		t.Fatalf("expected direct cpu usage metric, got %s", query)
	}
}

func TestMappedCounterPromQLUsesRate(t *testing.T) {
	query := mappedPromQL("node_cpu_seconds_total", "none", `ident="10.10.1.11"`)
	if !strings.Contains(query, "rate(node_cpu_seconds_total") || strings.Contains(query, "max(node_cpu_seconds_total") {
		t.Fatalf("expected rate-based cpu counter query, got %s", query)
	}
	if !strings.Contains(query, `mode="idle"`) {
		t.Fatalf("expected idle-mode cpu usage query, got %s", query)
	}
}

func TestMappedCounterTransformUsesRate(t *testing.T) {
	query := mappedPromQL("node_cpu_seconds_total", `max(node_cpu_seconds_total{instance="{instance}"})`, `ident="10.10.1.11"`)
	if !strings.Contains(query, "rate(node_cpu_seconds_total") || strings.Contains(query, "max(node_cpu_seconds_total") {
		t.Fatalf("expected transformed counter to be rate-normalized, got %s", query)
	}
}

func TestNetworkErrorRatePromQLSupportsNodeExporterCounters(t *testing.T) {
	metrics := []string{"node_network_receive_errs_total", "node_network_transmit_errs_total", "node_network_receive_bytes_total"}
	query := networkErrorRatePromQL(metrics, `ident="10.10.1.11"`)
	if !strings.Contains(query, "rate(node_network_receive_errs_total") || query == "" {
		t.Fatalf("expected node exporter error counter query, got %s", query)
	}
	if !strings.Contains(query, `device!~`) {
		t.Fatalf("expected loopback device filter, got %s", query)
	}
}

func TestBusinessNameFuzzyMatch(t *testing.T) {
	id := "test-business-fuzzy-" + time.Now().Format("150405.000000000")
	store.SaveTopologyBusiness(model.TopologyBusiness{ID: id, Name: "AI WorkBench 运维诊断业务链路"})
	t.Cleanup(func() { store.DeleteTopologyBusiness(id) })

	if got := detectBusinessName("AI WorkBench 巡检"); got != "AI WorkBench 运维诊断业务链路" {
		t.Fatalf("unexpected fuzzy business match: %q", got)
	}
	if _, ok := matchBusinessByNameOrHosts("AI WorkBench 巡检", nil); !ok {
		t.Fatal("matchBusinessByNameOrHosts should match fuzzy business name")
	}
}

func TestBusinessInspectionChatAnswerUsesFuzzyName(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	id := "test-chat-business-fuzzy-" + time.Now().Format("150405.000000000")
	store.SaveTopologyBusiness(model.TopologyBusiness{ID: id, Name: "AI WorkBench 运维诊断业务链路", Hosts: []string{"10.254.253.251"}})
	t.Cleanup(func() { store.DeleteTopologyBusiness(id) })

	handled, content := businessInspectionChatAnswer("AI WorkBench 巡检")
	if !handled {
		t.Fatal("businessInspectionChatAnswer should handle fuzzy inspection request")
	}
	if !strings.Contains(content, "AI WorkBench 运维诊断业务链路") || !strings.Contains(content, "业务巡检结论") {
		t.Fatalf("unexpected inspection answer: %s", content)
	}
}

func TestAIOpsInspectionUsesQuestionFallbackForBusinessMatch(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	id := "test-aiops-business-fuzzy-" + time.Now().Format("150405.000000000")
	store.SaveTopologyBusiness(model.TopologyBusiness{ID: id, Name: "AI WorkBench 运维诊断业务链路", Hosts: []string{"10.254.253.250"}})
	t.Cleanup(func() { store.DeleteTopologyBusiness(id) })

	content, steps, _, _, _ := runAIOpsInspectionAnswer("AI WorkBench 巡检", "", nil, nil)
	if strings.Contains(content, "未匹配到具体业务") {
		t.Fatalf("AIOps inspection should match business from question fallback: %s", content)
	}
	if !strings.Contains(content, "AI WorkBench 运维诊断业务链路") {
		t.Fatalf("AIOps inspection answer missing business name: %s", content)
	}
	if len(steps) == 0 || steps[0].Action != "business_inspection" {
		t.Fatalf("expected business_inspection reasoning step, got %+v", steps)
	}
}

func TestEnsureDefaultMonitoringBusinessRegistersUnassignedHosts(t *testing.T) {
	defaultBefore, hadDefault := defaultMonitoringBusiness("默认监控业务")
	t.Cleanup(func() {
		if hadDefault {
			store.SaveTopologyBusiness(defaultBefore)
			return
		}
		if business, ok := defaultMonitoringBusiness("默认监控业务"); ok {
			store.DeleteTopologyBusiness(business.ID)
		}
	})

	assignedID := "test-assigned-business-" + time.Now().Format("150405.000000000")
	store.SaveTopologyBusiness(model.TopologyBusiness{ID: assignedID, Name: "已注册业务", Hosts: []string{"10.254.253.240"}})
	t.Cleanup(func() { store.DeleteTopologyBusiness(assignedID) })

	ensureDefaultMonitoringBusiness([]string{"10.254.253.240", "10.254.253.241", "10.254.253.242"})
	ensureDefaultMonitoringBusiness([]string{"10.254.253.241"})

	business, ok := defaultMonitoringBusiness("默认监控业务")
	if !ok {
		t.Fatal("default monitoring business should be created")
	}
	if countString(business.Hosts, "10.254.253.240") != 0 {
		t.Fatalf("assigned host should not be added to default business: %+v", business.Hosts)
	}
	if countString(business.Hosts, "10.254.253.241") != 1 || countString(business.Hosts, "10.254.253.242") != 1 {
		t.Fatalf("unassigned hosts should be registered once: %+v", business.Hosts)
	}
	if business.Attributes["source"] != "prometheus_auto_discovery" {
		t.Fatalf("default business source attribute missing: %+v", business.Attributes)
	}
	if !hasNode(&business.Graph, "host-10-254-253-241") || !hasNode(&business.Graph, "host-10-254-253-242") {
		t.Fatalf("default business graph missing discovered hosts: %+v", business.Graph.Nodes)
	}
}

func TestIPsFromPrometheusInstances(t *testing.T) {
	ips := ipsFromPrometheusInstances([]string{"10.254.253.231:9100", "node-10.254.253.232", "10.254.253.231"})
	if countString(ips, "10.254.253.231") != 1 || countString(ips, "10.254.253.232") != 1 {
		t.Fatalf("unexpected parsed ips: %+v", ips)
	}
}

func countString(items []string, want string) int {
	count := 0
	for _, item := range items {
		if item == want {
			count++
		}
	}
	return count
}

func TestInspectionReportLocalizesEnglishHeadings(t *testing.T) {
	report := strings.Join([]string{
		"# Inspection Report: AI WorkBench",
		"## Overview",
		"- Health score: 88/100",
		"- Current status: warning",
		"- Data sources: prometheus, catpaw",
		"## Key Findings",
		"- 根因：入口层延迟升高",
		"## Suggested Actions",
		"- 处置建议：复核网关连接数",
	}, "\n")
	got := ensureInspectionReportSections(report, model.BusinessInspection{BusinessName: "AI WorkBench", Score: 88, Status: "warning", DataSources: []string{"prometheus"}})
	for _, forbidden := range []string{"Inspection Report", "Overview", "Key Findings", "Suggested Actions", "Health score", "Current status", "Data sources"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("report still contains English heading %q: %s", forbidden, got)
		}
	}
	if !strings.Contains(got, "# 业务巡检报告") || !strings.Contains(got, "## 处置建议") {
		t.Fatalf("report missing Chinese sections: %s", got)
	}
}

func TestAuditEventIncludesOperatorTimestampDescription(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/audit-test", bytes.NewReader(nil))
	c.Set("username", "admin")

	auditEvent(c, "workflow.run", "wf-1", "low", "ok", "手动触发", "batch-1")
	events := store.ListAuditEvents(1)
	if len(events) != 1 {
		t.Fatal("expected one audit event")
	}
	event := events[0]
	if event.Operator != "admin" || event.Timestamp == "" || event.Description == "" {
		t.Fatalf("audit fields missing: %+v", event)
	}
	if !strings.Contains(event.Description, "workflow.run") || !strings.Contains(event.Description, "wf-1") {
		t.Fatalf("audit description is not specific: %s", event.Description)
	}
}
