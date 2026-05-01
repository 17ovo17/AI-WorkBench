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

func performJSON(handler gin.HandlerFunc, body any) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/", handler)
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	} else {
		payload = []byte("{")
	}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestWhiteboxCleanIPBranches(t *testing.T) {
	valid, ok := cleanIP("198.18.20.11")
	if !ok || valid != "198.18.20.11" {
		t.Fatalf("expected valid test IP, got %q ok=%v", valid, ok)
	}
	invalids := []string{"", "999.1.1.1", "0.0.0.0", "224.0.0.1", "169.254.1.1", "198.18.20.11:9100"}
	for _, input := range invalids {
		if _, ok := cleanIP(input); ok {
			t.Fatalf("expected invalid IP rejected: %q", input)
		}
	}
}

func TestWhiteboxCatpawHeartbeatBindAndValidation(t *testing.T) {
	badJSON := performJSON(CatpawHeartbeat, nil)
	if badJSON.Code != http.StatusBadRequest {
		t.Fatalf("bad JSON should be 400, got %d", badJSON.Code)
	}
	badIP := performJSON(CatpawHeartbeat, map[string]any{"ip": "999.1.1.1"})
	if badIP.Code != http.StatusBadRequest {
		t.Fatalf("bad IP should be 400, got %d body=%s", badIP.Code, badIP.Body.String())
	}
	good := performJSON(CatpawHeartbeat, map[string]any{"ip": "198.18.20.11", "hostname": "whitebox", "version": "test"})
	if good.Code != http.StatusOK {
		t.Fatalf("good heartbeat should be 200, got %d body=%s", good.Code, good.Body.String())
	}
}

func TestWhiteboxCatpawReportSummarySanitizesMojibake(t *testing.T) {
	report := "# Catpaw Windows Health\n\n## plugin windows.eventlog\n```json\n[{\"TimeCreated\":\"/Date(1777071358851)/\",\"LogName\":\"Application\",\"ProviderName\":\"Microsoft-Windows-Security-SPP\",\"Id\":1014,\"LevelDisplayName\":\"????\",\"Message\":\"???????????????????hr=0xC004C060\"}]\n```\n"
	summary := summarizeCatpawReport(report)
	if strings.Contains(summary, "????") || strings.Contains(summary, "/Date(") {
		t.Fatalf("summary still contains mojibake/date wrapper: %s", summary)
	}
	if !strings.Contains(summary, "Security-SPP") || !strings.Contains(summary, "2026-") {
		t.Fatalf("summary should contain readable Security-SPP and converted date, got: %s", summary)
	}
}

func TestWhiteboxAIProviderPlaceholderNotUsable(t *testing.T) {
	if usableSecret("${AI_WORKBENCH_API_KEY}") != "" {
		t.Fatal("placeholder key must not be treated as usable")
	}
	if usableSecret("******") != "" {
		t.Fatal("masked key must not be treated as usable")
	}
}

func TestWhiteboxLoadAIProvidersAliasesAndMasking(t *testing.T) {
	viper.Reset()
	viper.Set("ai_providers", []map[string]any{{"id": "p1", "name": "alias", "baseurl": "https://example.com/v1", "apikey": "secret", "models": []string{"m1"}, "default": true}})
	providers := loadAIProviders()
	if len(providers) != 1 || providers[0].BaseURL != "https://example.com/v1" || providers[0].APIKey != "secret" {
		t.Fatalf("alias provider not normalized: %#v", providers)
	}
}

func TestWhiteboxConfiguredModelIDsUseSavedProviders(t *testing.T) {
	viper.Reset()
	viper.Set("ai_providers", []model.AIProvider{{ID: "p1", Name: "saved", Models: []string{"gpt-5.5", " gpt-5.5 ", ""}, Default: true}})
	models := configuredModelIDs()
	if len(models) != 1 || models[0] != "gpt-5.5" {
		t.Fatalf("saved provider models should be normalized and deduplicated, got %#v", models)
	}
}

func TestWhiteboxPlatformMySQLDataSourceIsVirtual(t *testing.T) {
	viper.Reset()
	viper.Set("mysql.dsn", "root@unix(/var/run/mysqld/mysqld.sock)/ai_workbench?charset=utf8mb4&parseTime=true&loc=Local")
	viper.Set("prometheus.url", "http://localhost:9090")
	viper.Set("data_sources", []map[string]any{{"id": "custom-prom", "name": "Custom Prom", "type": "prometheus", "url": "http://prom:9090"}})

	ds := loadDataSources()
	foundMySQL := false
	for _, item := range ds {
		if item.ID == "platform-mysql" {
			foundMySQL = true
			if item.Type != "mysql" || item.Name == "" || item.URL != "unix:/var/run/mysqld/mysqld.sock" || item.Database != "ai_workbench" {
				t.Fatalf("platform mysql datasource not normalized: %#v", item)
			}
		}
	}
	if !foundMySQL {
		t.Fatalf("platform mysql datasource missing: %#v", ds)
	}
}

func TestWhiteboxAlertDeleteLifecycle(t *testing.T) {
	id := "whitebox-alert-delete-" + time.Now().Format("20060102150405.000000000")
	store.AddAlert(&model.AlertRecord{ID: id, Title: "whitebox delete", TargetIP: "198.18.20.11", Severity: "warning", Status: "firing", Source: "whitebox", CreateTime: time.Now()})
	if !store.ResolveAlert(id) {
		t.Fatal("expected alert resolve to find record")
	}
	if !store.DeleteAlert(id) {
		t.Fatal("expected alert delete to find record")
	}
	if store.DeleteAlert(id) {
		t.Fatal("second delete should report missing record")
	}
}

func TestWhiteboxAlertIncidentLifecycleAndDedup(t *testing.T) {
	base := time.Now().Format("20060102150405.000000000")
	first := &model.AlertRecord{ID: "whitebox-incident-" + base + "-1", Title: "HighRedisLatency", TargetIP: "198.18.20.20", Severity: "warning", Status: "firing", Source: "whitebox", Labels: map[string]string{"business_id": "clims", "test_batch_id": "whitebox"}, CreateTime: time.Now()}
	second := &model.AlertRecord{ID: "whitebox-incident-" + base + "-2", Title: "HighRedisLatency", TargetIP: "198.18.20.20", Severity: "warning", Status: "firing", Source: "whitebox", Labels: map[string]string{"business_id": "clims", "test_batch_id": "whitebox"}, CreateTime: time.Now().Add(time.Second)}
	if !store.AddAlert(first) {
		t.Fatal("first incident should be treated as new")
	}
	if store.AddAlert(second) {
		t.Fatal("duplicate incident should be folded instead of treated as new")
	}
	var merged *model.AlertRecord
	for _, alert := range store.ListAlerts() {
		if alert.ID == first.ID {
			merged = alert
			break
		}
	}
	if merged == nil || merged.Count < 2 || merged.Fingerprint == "" || len(merged.NotificationTrail) == 0 {
		t.Fatalf("expected deduplicated incident with notification trail, got %#v", merged)
	}
	if !store.UpdateAlertAction(first.ID, model.AlertAction{Action: "acknowledge", Actor: "oncall"}, func(a *model.AlertRecord) { a.Status = "acknowledged"; a.AckBy = "oncall" }) {
		t.Fatal("acknowledge should update alert")
	}
	if !store.UpdateAlertAction(first.ID, model.AlertAction{Action: "assign", Actor: "oncall"}, func(a *model.AlertRecord) { a.Status = "assigned"; a.Assignee = "DBA" }) {
		t.Fatal("assign should update alert")
	}
	if !store.ResolveAlert(first.ID) || !store.DeleteAlert(first.ID) {
		t.Fatal("resolve and soft delete should succeed")
	}
}

func TestWhiteboxAlertActionRejectsUnknownAction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	id := "whitebox-alert-action-" + time.Now().Format("20060102150405.000000000")
	store.AddAlert(&model.AlertRecord{ID: id, Title: "whitebox action", TargetIP: "198.18.20.11", Severity: "warning", Status: "firing", Source: "whitebox", CreateTime: time.Now()})
	r := gin.New()
	r.PUT("/alerts/:id/:action", AlertAction)
	req := httptest.NewRequest(http.MethodPut, "/alerts/"+id+"/unsupported", strings.NewReader(`{"actor":"tester"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown alert action should return 400, got %d body=%s", w.Code, w.Body.String())
	}
	_ = store.DeleteAlert(id)
}

func TestWhiteboxOnCallTestSend(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/oncall/test-send", TestOnCallNotification)
	req := httptest.NewRequest(http.MethodPost, "/oncall/test-send", strings.NewReader(`{"channel":"console","receiver":"SRE","business_id":"clims","alert_title":"unit"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("test send should pass, code=%d body=%s", w.Code, w.Body.String())
	}
	bad := httptest.NewRecorder()
	badReq := httptest.NewRequest(http.MethodPost, "/oncall/test-send", strings.NewReader(`{"channel":"unsupported"}`))
	badReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(bad, badReq)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("unsupported channel should be rejected, got %d", bad.Code)
	}
}

func TestWhiteboxDiagnoseCleanupByScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stamp := time.Now().Format("20060102150405.000000000")
	id := "biz-inspect-whitebox-" + stamp
	otherID := "biz-inspect-whitebox-other-" + stamp
	store.AddRecord(&model.DiagnoseRecord{ID: id, TargetIP: "business:whitebox", Trigger: "business_inspection", Source: "business_inspection", DataSource: "business_inspection", Status: model.StatusDone, AlertTitle: "business inspection whitebox", RawReport: `{"business_id":"whitebox"}`, CreateTime: time.Now()})
	store.AddRecord(&model.DiagnoseRecord{ID: otherID, TargetIP: "business:keep", Trigger: "business_inspection", Source: "business_inspection", DataSource: "business_inspection", Status: model.StatusDone, AlertTitle: "business inspection keep", RawReport: `{"business_id":"keep"}`, CreateTime: time.Now()})
	r := gin.New()
	r.DELETE("/diagnose", CleanupDiagnose)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/diagnose?scope=business_inspection&business_id=whitebox", nil))
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), `"deleted":0`) {
		t.Fatalf("business inspection cleanup should delete scoped record, code=%d body=%s", w.Code, w.Body.String())
	}
	if records := store.ListRecords(); !recordExists(records, otherID) {
		t.Fatal("scoped business inspection cleanup should not delete other business records")
	}
	store.DeleteRecord(otherID)
}

func recordExists(records []*model.DiagnoseRecord, id string) bool {
	for _, record := range records {
		if record.ID == id {
			return true
		}
	}
	return false
}

func TestWhiteboxListDiagnoseBusinessFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	id := "biz-inspect-filter-" + time.Now().Format("20060102150405.000000000")
	store.AddRecord(&model.DiagnoseRecord{ID: id, TargetIP: "business:clims", Trigger: "business_inspection", Source: "business_inspection", DataSource: "business_inspection", Status: model.StatusDone, AlertTitle: "业务巡检：clims", RawReport: `{"business_id":"clims"}`, CreateTime: time.Now()})
	r := gin.New()
	r.GET("/diagnose", ListDiagnose)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/diagnose?source=business_inspection&business_id=clims", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), id) {
		t.Fatalf("business filter should include record, code=%d body=%s", w.Code, w.Body.String())
	}
	store.DeleteRecord(id)
}

func TestWhiteboxAITopologyLayerClassification(t *testing.T) {
	cases := []struct {
		endpoint model.TopologyEndpoint
		want     string
	}{
		{model.TopologyEndpoint{IP: "198.18.20.20", Port: 80, ServiceName: "nginx"}, "gateway"},
		{model.TopologyEndpoint{IP: "198.18.20.11", Port: 8081, ServiceName: "jvm"}, "app"},
		{model.TopologyEndpoint{IP: "198.18.20.20", Port: 6379, ServiceName: "redis"}, "cache"},
		{model.TopologyEndpoint{IP: "198.18.20.30", Port: 9092, ServiceName: "kafka"}, "mq"},
		{model.TopologyEndpoint{IP: "198.18.22.11", Port: 1521, ServiceName: "oracle"}, "db"},
		{model.TopologyEndpoint{IP: "198.18.20.40", Port: 2379, ServiceName: "etcd"}, "infra"},
	}
	for _, tc := range cases {
		if got := classifyAITopologyLayer(tc.endpoint); got != tc.want {
			t.Fatalf("classifyAITopologyLayer(%+v)=%s want %s", tc.endpoint, got, tc.want)
		}
	}
}

func TestWhiteboxAITopologyFallbackLinksAndRisks(t *testing.T) {
	graph := buildAITopologyGraph(aiTopologyGenerateRequest{ServiceName: "clims", Endpoints: []model.TopologyEndpoint{
		{IP: "198.18.20.20", Port: 80, ServiceName: "nginx"},
		{IP: "198.18.20.11", Port: 8081, ServiceName: "jvm"},
		{IP: "198.18.20.20", Port: 6379, ServiceName: "redis"},
		{IP: "198.18.22.11", Port: 1521, ServiceName: "oracle"},
	}}, "heuristic_fallback", "test")
	if graph.Summary.Planner != "heuristic_fallback" {
		t.Fatalf("unexpected planner: %s", graph.Summary.Planner)
	}
	if len(graph.Nodes) != 4 {
		t.Fatalf("expected 4 business nodes, got %d", len(graph.Nodes))
	}
	mustHave := map[string]bool{"gateway->app": false, "app->cache": false, "app->db": false}
	layers := map[string]string{}
	for _, node := range graph.Nodes {
		layers[node.ID] = node.Layer
	}
	for _, link := range graph.Links {
		key := layers[link.Source] + "->" + layers[link.Target]
		if _, ok := mustHave[key]; ok {
			mustHave[key] = true
		}
	}
	for key, ok := range mustHave {
		if !ok {
			t.Fatalf("missing inferred link %s in %+v", key, graph.Links)
		}
	}
	if len(graph.Risks) == 0 {
		t.Fatal("expected single point risks for clims-like topology")
	}
}

func TestWhiteboxAITopologyExcludesAgentNodes(t *testing.T) {
	graph := buildAITopologyGraph(aiTopologyGenerateRequest{Endpoints: []model.TopologyEndpoint{
		{IP: "198.18.20.10", Port: 9101, ServiceName: "catpaw-agent"},
		{IP: "198.18.20.11", Port: 8080, ServiceName: "order-api"},
	}}, "heuristic_fallback", "test")
	if len(graph.Nodes) != 1 {
		t.Fatalf("expected only one business node, got %+v", graph.Nodes)
	}
	if graph.Nodes[0].Layer != "app" {
		t.Fatalf("expected remaining node to be app, got %+v", graph.Nodes[0])
	}
}

func TestWhiteboxAIOpsIntentRouting(t *testing.T) {
	cases := map[string]string{
		"10.0.1.21 CPU 很高": "diagnostic",
		"clims 巡检一下":       "inspection",
		"我上报一个 ERROR 日志":   "report",
		"生成 clims 业务拓扑图":   "topology",
	}
	for input, want := range cases {
		if got := detectAIOpsMode(input, nil); got != want {
			t.Fatalf("detectAIOpsMode(%q)=%s want %s", input, got, want)
		}
	}
}

func TestWhiteboxAIOpsPromQLChainOrder(t *testing.T) {
	ids := listPromQLTemplateIDsForChain("cpu_high")
	want := []string{"cpu-001", "cpu-003", "cpu-002", "cpu-004", "mem-001", "disk-002"}
	if len(ids) < len(want) {
		t.Fatalf("cpu_high chain too short: %+v", ids)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("cpu_high step %d=%s want %s full=%+v", i, ids[i], want[i], ids)
		}
	}
	query := renderPromQLTemplate(`cpu_usage_active{ident=~"{{hosts}}"}`, []string{"10.0.1.21", "10.0.1.22"})
	if !strings.Contains(query, `10.0.1.21|10.0.1.22`) {
		t.Fatalf("hosts were not rendered into promql: %s", query)
	}
}

func TestWhiteboxAIOpsActionSafety(t *testing.T) {
	blocked := performJSON(AIOpsExecuteAction, map[string]any{"actionType": "restart", "params": map[string]any{"command": "systemctl restart nginx"}})
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("write action should be forbidden, got %d body=%s", blocked.Code, blocked.Body.String())
	}
	copyOnly := performJSON(AIOpsExecuteAction, map[string]any{"actionType": "command", "params": map[string]any{"command": "df -h"}})
	if copyOnly.Code != http.StatusOK || !strings.Contains(copyOnly.Body.String(), "copyOnly") {
		t.Fatalf("command action should be copy-only, got %d body=%s", copyOnly.Code, copyOnly.Body.String())
	}
}

func TestWhiteboxAIOpsSanitizesAttachments(t *testing.T) {
	text := sanitizeAIOpsText("password=abc token:xyz api_key 123 normal")
	if strings.Contains(text, "abc") || strings.Contains(text, "xyz") || strings.Contains(text, "123") {
		t.Fatalf("sensitive values were not redacted: %s", text)
	}
}

func TestWhiteboxAIOpsV2ActionTopologyAndBlockedWrites(t *testing.T) {
	allowed, status, errMsg := executeAIOpsAction(aiopsActionRequest{ActionType: "topology", Params: map[string]any{"url": "/topology?highlight=api-01"}})
	if status != http.StatusOK || errMsg != "" || allowed["url"] != "/topology?highlight=api-01" {
		t.Fatalf("topology action should be allowed as read-only link, status=%d err=%s data=%+v", status, errMsg, allowed)
	}
	for _, actionType := range []string{"restart", "delete", "write", "remote_exec"} {
		_, status, errMsg = executeAIOpsAction(aiopsActionRequest{ActionType: actionType, Params: map[string]any{"command": "rm -rf /"}})
		if status != http.StatusForbidden || errMsg == "" {
			t.Fatalf("%s should be forbidden, status=%d err=%s", actionType, status, errMsg)
		}
	}
}

func TestWhiteboxAIOpsV2MetricParsingHelpers(t *testing.T) {
	if got := metricNameFromPromQL(`cpu_usage_active{ident="10.0.1.21"}`); got != "cpu_usage_active" {
		t.Fatalf("metricNameFromPromQL returned %q", got)
	}
	value, ok := firstFloatFromText(`cpu_usage_active 95.7 @ now`)
	if !ok || value != 95.7 {
		t.Fatalf("firstFloatFromText returned value=%v ok=%v", value, ok)
	}
}

func TestWhiteboxAIOpsV2TopologyTriggerByQuestion(t *testing.T) {
	cases := []string{
		"10.0.1.21 和 10.0.1.22 CPU 都很高",
		"Redis 异常影响哪些链路",
		"生成 clims 拓扑",
	}
	for _, input := range cases {
		graph, ok := aiopsTopologyGraphForQuestion(input)
		if !ok {
			t.Fatalf("expected topology trigger for %q", input)
		}
		if graph.Summary.Planner == "" {
			t.Fatalf("topology summary planner should be set for %q: %+v", input, graph.Summary)
		}
	}
	if _, ok := aiopsTopologyGraphForQuestion("单台机器 CPU 高"); ok {
		t.Fatal("generic single-machine diagnosis should not force topology update")
	}
}

func TestWhiteboxAIOpsAudienceSummaryAndHandoff(t *testing.T) {
	msg := runAIOpsDiagnosis("audience-whitebox", "10.0.1.21 CPU ?????????", nil, "oncall")
	if msg.Audience != "oncall" {
		t.Fatalf("expected oncall audience, got %s", msg.Audience)
	}
	if msg.SummaryCard.Problem == "" || msg.SummaryCard.NextStep == "" || msg.SummaryCard.Severity == "" {
		t.Fatalf("summaryCard should be populated: %+v", msg.SummaryCard)
	}
	if msg.HandoffNote.Summary == "" || len(msg.HandoffNote.OpenQuestions) == 0 || msg.HandoffNote.EscalationPolicy == "" {
		t.Fatalf("handoffNote should be populated for oncall: %+v", msg.HandoffNote)
	}
	foundSummary := false
	foundHandoff := false
	for _, action := range msg.SuggestedActions {
		if action.ID == "copy-summary" && action.Type == "command" && action.Command != "" {
			foundSummary = true
		}
		if action.ID == "copy-handoff" && action.Type == "command" && action.Command != "" {
			foundHandoff = true
		}
	}
	if !foundSummary || !foundHandoff {
		t.Fatalf("expected copy summary and handoff actions, got %+v", msg.SuggestedActions)
	}
}

func TestWhiteboxAIOpsAudienceAutoDetection(t *testing.T) {
	cases := map[string]string{
		"manager business impact": "manager",
		"show PromQL evidence":    "ops",
		"oncall escalation p1":    "oncall",
		"is service affected":     "user",
	}
	for input, want := range cases {
		if got := normalizeAIOpsAudience("", input, "diagnostic"); got != want {
			t.Fatalf("normalizeAIOpsAudience(%q)=%s want %s", input, got, want)
		}
	}
	if got := normalizeAIOpsAudience("", "\u7ed9\u9886\u5bfc\u770b\u4e1a\u52a1\u5f71\u54cd", "diagnostic"); got != "manager" {
		t.Fatalf("expected Chinese manager intent, got %s", got)
	}
	if got := normalizeAIOpsAudience("", "\u9700\u8981\u503c\u73ed\u4ea4\u63a5", "diagnostic"); got != "oncall" {
		t.Fatalf("expected Chinese oncall intent, got %s", got)
	}
}
