package handler

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

//go:embed promql_library.json
var promqlLibraryFS embed.FS

type promQLLibrary struct {
	Version         string                          `json:"version"`
	Description     string                          `json:"description"`
	Templates       []promQLTemplate                `json:"templates"`
	DiagnosisChains map[string]promQLDiagnosisChain `json:"diagnosisChains"`
}

type promQLTemplate struct {
	ID          string             `json:"id"`
	Category    string             `json:"category"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	PromQL      string             `json:"promql"`
	Threshold   map[string]float64 `json:"threshold"`
	Unit        string             `json:"unit"`
	TimeRange   string             `json:"timeRange"`
	Related     []string           `json:"related"`
}

type promQLDiagnosisChain struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Steps       []struct {
		TemplateID string `json:"templateId"`
		Purpose    string `json:"purpose"`
	} `json:"steps"`
}

var (
	promQLLibOnce    sync.Once
	promQLLib        promQLLibrary
	promQLLibErr     error
	aiopsMu          sync.Mutex
	aiopsInspections = map[string]model.AIOpsInspection{}
	aiopsWSUpgrader  = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
)

type aiopsCreateSessionRequest struct {
	Title   string           `json:"title"`
	Scope   model.AIOpsScope `json:"scope"`
	Mode    string           `json:"mode"`
	Context map[string]any   `json:"context"`
}

type aiopsMessageRequest struct {
	Role        string                  `json:"role"`
	Content     string                  `json:"content"`
	Attachments []model.AIOpsAttachment `json:"attachments"`
	Audience    string                  `json:"audience"`
}

type aiopsActionRequest struct {
	ActionType string         `json:"actionType"`
	ActionID   string         `json:"actionId"`
	Params     map[string]any `json:"params"`
}

func loadPromQLLibrary() (promQLLibrary, error) {
	promQLLibOnce.Do(func() {
		data, err := promqlLibraryFS.ReadFile("promql_library.json")
		if err != nil {
			promQLLibErr = err
			return
		}
		promQLLibErr = json.Unmarshal(data, &promQLLib)
	})
	return promQLLib, promQLLibErr
}

func AIOpsCreateSession(c *gin.Context) {
	var req aiopsCreateSessionRequest
	_ = c.ShouldBindJSON(&req)
	now := time.Now()
	mode := normalizeAIOpsMode(req.Mode)
	if mode == "" {
		mode = "diagnostic"
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = modeTitle(mode)
	}
	id := "aiops-" + store.NewID()
	store.SaveChatSession(&model.ChatSession{ID: id, Title: title, Model: resolveDefaultModel(), TargetIP: firstNonEmptyAIOps(req.Scope.Hosts), CreatedAt: now, UpdatedAt: now})
	resp := model.AIOpsSession{SessionID: id, ID: id, Title: title, Mode: mode, Status: "active", Scope: req.Scope, Context: req.Context, ContextSnapshot: req.Scope, CreatedAt: now, UpdatedAt: now}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": resp})
}

func AIOpsPostMessage(c *gin.Context) {
	sessionID := c.Param("id")
	if _, ok := store.GetChatSession(sessionID); !ok {
		now := time.Now()
		store.SaveChatSession(&model.ChatSession{ID: sessionID, Title: "AIOps 智能问诊", Model: resolveDefaultModel(), CreatedAt: now, UpdatedAt: now})
	}
	var req aiopsMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": err.Error()})
		return
	}
	content := sanitizeAIOpsText(req.Content)
	attachments := sanitizeAIOpsAttachments(req.Attachments)
	now := time.Now()
	store.AddChatMessage(model.ChatMessage{ID: store.NewID(), SessionID: sessionID, Role: "user", Content: content, CreatedAt: now})
	answer := runAIOpsDiagnosis(sessionID, content, attachments, req.Audience)
	store.AddChatMessage(model.ChatMessage{ID: answer.MessageID, SessionID: sessionID, Role: "assistant", Content: answer.Content, CreatedAt: answer.CreatedAt})
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": answer})
}

func AIOpsGetMessages(c *gin.Context) {
	sessionID := c.Param("id")
	messages := store.ListChatMessages(sessionID)
	out := make([]model.AIOpsMessage, 0, len(messages))
	for _, msg := range messages {
		out = append(out, model.AIOpsMessage{MessageID: msg.ID, ID: msg.ID, SessionID: msg.SessionID, Role: msg.Role, Content: msg.Content, CreatedAt: msg.CreatedAt})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"messages": out, "hasMore": false}})
}

func AIOpsExecuteAction(c *gin.Context) {
	var req aiopsActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": err.Error()})
		return
	}
	data, status, errMsg := executeAIOpsAction(req)
	if errMsg != "" {
		c.JSON(status, gin.H{"code": status, "error": errMsg})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": data})
}

func executeAIOpsAction(req aiopsActionRequest) (gin.H, int, string) {
	actionType := strings.ToLower(strings.TrimSpace(req.ActionType))
	audit := gin.H{"actionId": req.ActionID, "actionType": actionType, "copyOnly": actionType == "command", "timestamp": time.Now()}
	switch actionType {
	case "promql":
		query := stringParam(req.Params, "query")
		if query == "" {
			return nil, http.StatusBadRequest, "query is required"
		}
		start := time.Now()
		result, err := queryProm(query)
		latency := time.Since(start).Milliseconds()
		if err != nil {
			return gin.H{"actionType": "promql", "query": query, "error": err.Error(), "latency_ms": latency, "audit": audit}, http.StatusOK, ""
		}
		return gin.H{"actionType": "promql", "query": query, "result": result, "latency_ms": latency, "chart": gin.H{"type": "line", "title": "PromQL \u67e5\u8be2"}, "audit": audit}, http.StatusOK, ""
	case "command":
		return gin.H{"actionType": "command", "copyOnly": true, "command": stringParam(req.Params, "command"), "message": "\u53ea\u8fd4\u56de\u590d\u5236\u5185\u5bb9\uff0c\u4e0d\u6267\u884c\u8fdc\u7a0b\u547d\u4ee4", "audit": audit}, http.StatusOK, ""
	case "link", "topology":
		return gin.H{"actionType": actionType, "url": stringParam(req.Params, "url"), "audit": audit}, http.StatusOK, ""
	default:
		return nil, http.StatusForbidden, "AIOps suggested actions are read-only: only promql, command(copy), link, and topology are allowed"
	}
}

func AIOpsCreateInspection(c *gin.Context) {
	var req struct {
		Name  string           `json:"name"`
		Scope model.AIOpsScope `json:"scope"`
		Depth string           `json:"depth"`
	}
	_ = c.ShouldBindJSON(&req)
	id := "insp-" + store.NewID()
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "AIOps 主动巡检"
	}
	inspection := model.AIOpsInspection{InspectionID: id, Name: name, Status: "running", CreatedAt: time.Now(), UpdatedAt: time.Now(), Progress: map[string]any{"totalLayers": 6, "completedLayers": 0, "currentLayer": "topology", "estimatedCompletion": time.Now().Add(2 * time.Minute).Format(time.RFC3339)}}
	if business, ok := matchBusinessByScope(req.Scope); ok {
		report := buildBusinessInspection(business)
		inspection.Status = "completed"
		inspection.Report = report
		inspection.Progress = map[string]any{"totalLayers": 6, "completedLayers": 6, "currentLayer": "completed"}
	}
	aiopsMu.Lock()
	aiopsInspections[id] = inspection
	aiopsMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": inspection})
}

func AIOpsInspectionProgress(c *gin.Context) {
	inspection, ok := getAIOpsInspection(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "error": "inspection not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"inspectionId": inspection.InspectionID, "status": inspection.Status, "progress": inspection.Progress}})
}

func AIOpsInspectionReport(c *gin.Context) {
	inspection, ok := getAIOpsInspection(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "error": "inspection not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": inspection})
}

func AIOpsPrometheusQuery(c *gin.Context) {
	var req struct {
		Query string `json:"query"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Query) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "query is required"})
		return
	}
	start := time.Now()
	result, err := queryProm(req.Query)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"query": req.Query, "result": "", "error": err.Error(), "latency_ms": latency}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"query": req.Query, "result": result, "latency_ms": latency}})
}

func AIOpsPrometheusQueryRange(c *gin.Context) {
	AIOpsPrometheusQuery(c)
}

func AIOpsCatpawQuery(c *gin.Context) {
	var req struct {
		IP    string `json:"ip"`
		Check string `json:"check"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.IP == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "error": "ip is required"})
		return
	}
	if report, ok := store.LatestCatpawReport(req.IP); ok {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"ip": req.IP, "check": req.Check, "source": "catpaw_report", "result": report.Report, "created_at": report.CreateTime}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"ip": req.IP, "check": req.Check, "source": "catpaw", "result": "no catpaw report found", "agents": store.ListAgents()}})
}

func AIOpsTopologyGenerate(c *gin.Context) {
	GenerateAITopology(c)
}

func AIOpsSessionWS(c *gin.Context) {
	conn, err := aiopsWSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	sessionID := c.Param("id")
	var writeMu sync.Mutex
	metricStops := map[string]chan struct{}{}
	defer func() {
		for _, stop := range metricStops {
			close(stop)
		}
	}()
	writeAIOpsWS(conn, &writeMu, gin.H{"type": "connected", "sessionId": sessionID, "serverTime": time.Now(), "timestamp": time.Now()})
	for {
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		switch strings.ToLower(fmt.Sprint(msg["type"])) {
		case "ping":
			writeAIOpsWS(conn, &writeMu, gin.H{"type": "pong", "serverTime": time.Now(), "timestamp": time.Now()})
		case "interrupt":
			writeAIOpsWS(conn, &writeMu, gin.H{"type": "interrupted", "reason": fmt.Sprint(msg["reason"]), "completedSteps": 0, "totalSteps": 0, "timestamp": time.Now()})
		case "feedback":
			writeAIOpsWS(conn, &writeMu, gin.H{"type": "feedback_ack", "messageId": msg["messageId"], "timestamp": time.Now()})
		case "execute_action":
			req := aiopsActionRequest{ActionType: fmt.Sprint(msg["actionType"]), ActionID: fmt.Sprint(msg["actionId"]), Params: mapStringAny(msg["params"])}
			data, status, errMsg := executeAIOpsAction(req)
			if errMsg != "" {
				writeAIOpsWS(conn, &writeMu, gin.H{"type": "error", "code": status, "message": errMsg, "recoverable": true, "timestamp": time.Now()})
			} else {
				writeAIOpsWS(conn, &writeMu, gin.H{"type": "action_result", "actionId": req.ActionID, "actionType": req.ActionType, "result": data, "timestamp": time.Now()})
			}
		case "subscribe_metrics":
			queries := stringSliceFromAny(msg["metrics"])
			interval := intFromAny(msg["interval"], 5)
			for _, query := range queries {
				if _, exists := metricStops[query]; exists || strings.TrimSpace(query) == "" {
					continue
				}
				stop := make(chan struct{})
				metricStops[query] = stop
				go streamAIOpsMetric(conn, &writeMu, query, time.Duration(interval)*time.Second, stop)
			}
			writeAIOpsWS(conn, &writeMu, gin.H{"type": "metric_subscription", "status": "subscribed", "metrics": queries, "timestamp": time.Now()})
		case "unsubscribe_metrics":
			for _, query := range stringSliceFromAny(msg["metrics"]) {
				if stop, ok := metricStops[query]; ok {
					close(stop)
					delete(metricStops, query)
				}
			}
			writeAIOpsWS(conn, &writeMu, gin.H{"type": "metric_subscription", "status": "unsubscribed", "timestamp": time.Now()})
		case "chat":
			content := sanitizeAIOpsText(fmt.Sprint(msg["content"]))
			attachments := sanitizeAIOpsAttachments(attachmentsFromAny(msg["attachments"]))
			store.AddChatMessage(model.ChatMessage{ID: store.NewID(), SessionID: sessionID, Role: "user", Content: content, CreatedAt: time.Now()})
			answer := runAIOpsDiagnosis(sessionID, content, attachments, fmt.Sprint(msg["audience"]))
			for _, step := range answer.ReasoningChain {
				writeAIOpsWS(conn, &writeMu, gin.H{"type": "reasoning_step", "step": step.Step, "action": step.Action, "status": "running", "input": step.Input, "query": step.Query, "timestamp": time.Now()})
				writeAIOpsWS(conn, &writeMu, gin.H{"type": "reasoning_step", "step": step.Step, "action": step.Action, "status": step.Status, "input": step.Input, "output": firstNonNil(step.Output, step.Result), "query": step.Query, "latencyMs": step.LatencyMs, "timestamp": step.Timestamp})
			}
			store.AddChatMessage(model.ChatMessage{ID: answer.MessageID, SessionID: sessionID, Role: "assistant", Content: answer.Content, CreatedAt: answer.CreatedAt})
			writeAIOpsWS(conn, &writeMu, gin.H{"type": "diagnosis", "message": answer, "timestamp": time.Now()})
			if graph, ok := aiopsTopologyGraphForQuestion(content); ok {
				writeAIOpsWS(conn, &writeMu, gin.H{"type": "topology_update", "topology": graph, "highlight": answer.Topology, "timestamp": time.Now()})
			}
			writeAIOpsWS(conn, &writeMu, gin.H{"type": "complete", "messageId": answer.MessageID, "totalLatencyMs": 0, "timestamp": time.Now()})
		default:
			writeAIOpsWS(conn, &writeMu, gin.H{"type": "error", "code": 400, "message": "unknown websocket message type", "recoverable": true, "timestamp": time.Now()})
		}
	}
}

func writeAIOpsWS(conn *websocket.Conn, mu *sync.Mutex, payload gin.H) {
	mu.Lock()
	defer mu.Unlock()
	_ = conn.WriteJSON(payload)
}

func streamAIOpsMetric(conn *websocket.Conn, mu *sync.Mutex, query string, interval time.Duration, stop <-chan struct{}) {
	if interval < time.Second {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	emit := func() {
		result, err := queryProm(query)
		payload := gin.H{"type": "metric_update", "query": query, "metric": metricNameFromPromQL(query), "timestamp": time.Now()}
		if err != nil {
			payload["error"] = err.Error()
		} else {
			payload["raw"] = result
			if value, ok := firstFloatFromText(result); ok {
				payload["value"] = value
			}
		}
		writeAIOpsWS(conn, mu, payload)
	}
	emit()
	for {
		select {
		case <-ticker.C:
			emit()
		case <-stop:
			return
		}
	}
}
