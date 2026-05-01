package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func HealthStorage(c *gin.Context) {
	c.JSON(http.StatusOK, store.Health())
}

func ListChatSessions(c *gin.Context) {
	c.JSON(http.StatusOK, store.ListChatSessions())
}

func CreateChatSession(c *gin.Context) {
	var req struct {
		Title    string `json:"title"`
		Model    string `json:"model"`
		TargetIP string `json:"target_ip"`
	}
	_ = c.ShouldBindJSON(&req)
	now := time.Now()
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "New session"
	}
	s := &model.ChatSession{ID: store.NewID(), Title: title, Model: req.Model, TargetIP: req.TargetIP, CreatedAt: now, UpdatedAt: now}
	store.SaveChatSession(s)
	c.JSON(http.StatusOK, s)
}

func GetChatSession(c *gin.Context) {
	if s, ok := store.GetChatSession(c.Param("id")); ok {
		c.JSON(http.StatusOK, s)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
}

func RenameChatSession(c *gin.Context) {
	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Title) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}
	s, ok := store.GetChatSession(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	s.Title = strings.TrimSpace(req.Title)
	store.SaveChatSession(s)
	c.JSON(http.StatusOK, s)
}

func DeleteChatSession(c *gin.Context) {
	store.DeleteChatSession(c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func GetTopology(c *gin.Context) {
	g := store.GetTopology()
	agents := store.ListAgents()
	for i := range g.Nodes {
		if g.Nodes[i].Type == "host" && g.Nodes[i].IP != "" {
			g.Nodes[i].Status = "offline"
			for _, a := range agents {
				if a.IP == g.Nodes[i].IP && a.Online {
					g.Nodes[i].Status = "online"
				}
			}
		}
	}
	c.JSON(http.StatusOK, g)
}

func SaveTopology(c *gin.Context) {
	var g model.TopologyGraph
	if err := c.ShouldBindJSON(&g); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	seen := map[string]bool{}
	for _, n := range g.Nodes {
		if strings.TrimSpace(n.ID) == "" || strings.TrimSpace(n.Name) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "node id and name are required"})
			return
		}
		seen[n.ID] = true
	}
	edges := map[string]bool{}
	for _, e := range g.Edges {
		if !seen[e.SourceID] || !seen[e.TargetID] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "edge source/target must exist"})
			return
		}
		key := e.SourceID + "->" + e.TargetID + ":" + e.Protocol
		if edges[key] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "duplicate edge"})
			return
		}
		edges[key] = true
	}
	store.SaveTopology(g)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func TopologyResources(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"agents": store.ListAgents(), "platform": platformResources()})
}

type topologyDiscoverRequest struct {
	Hosts           []string                 `json:"hosts"`
	Endpoints       []model.TopologyEndpoint `json:"endpoints"`
	IncludePlatform bool                     `json:"include_platform"`
	UseAI           bool                     `json:"use_ai"`
	Attributes      map[string]string        `json:"attributes"`
}

type topologyServiceCandidate struct {
	Name     string
	Type     string
	Port     int
	Protocol string
	Required bool
}

func DiscoverTopology(c *gin.Context) {
	var req topologyDiscoverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hosts := normalizeHosts(req.Hosts)
	hosts = mergeHosts(hosts, prometheusHostsForSelection(hosts))
	if !req.IncludePlatform && len(hosts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hosts or include_platform is required"})
		return
	}
	now := time.Now()
	graph := model.TopologyGraph{Nodes: []model.TopologyNode{}, Edges: []model.TopologyEdge{}}
	if req.IncludePlatform {
		addPlatformDiscovery(&graph, now)
	}
	declaredPorts := endpointPortSet(req.Endpoints)
	for index, host := range hosts {
		addHostDiscovery(&graph, host, index, now, declaredPorts)
	}
	classified := classifyEndpointsWithAI(req.Endpoints, req.UseAI)
	addUserDefinedEndpoints(&graph, classified, now)
	addInferredBusinessEdges(&graph, classified, now)
	graph.Discovery = buildTopologyDiscoveryPlan(hosts, classified, &graph, req.UseAI)
	layoutBusinessTree(&graph)
	c.JSON(http.StatusOK, graph)
}

func normalizeHosts(hosts []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, host := range hosts {
		host = strings.TrimSpace(host)
		host, _ = url.QueryUnescape(host)
		host = strings.TrimPrefix(strings.TrimPrefix(host, "http://"), "https://")
		if i := strings.Index(host, "/"); i >= 0 {
			host = host[:i]
		}
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, host)
	}
	sort.Strings(out)
	return out
}

func platformResources() []gin.H {
	ip := PlatformIP()
	return []gin.H{
		{"id": "platform-web", "name": "AI WorkBench \u524d\u7aef", "type": "frontend", "ip": ip, "port": 3000},
		{"id": "platform-api", "name": "AI WorkBench \u540e\u7aef", "type": "backend", "ip": ip, "port": 8080},
		{"id": "platform-mysql", "name": "MySQL", "type": "database", "ip": "127.0.0.1", "port": 3306},
		{"id": "platform-redis", "name": "Redis", "type": "cache", "ip": "127.0.0.1", "port": 6379},
		{"id": "platform-prom", "name": "Prometheus", "type": "monitor", "ip": ip, "port": 9090},
	}
}

func addPlatformDiscovery(graph *model.TopologyGraph, now time.Time) {
	baseX := 90.0
	baseY := 110.0
	components := []model.TopologyNode{
		{ID: "platform-web", Name: "AI WorkBench \u524d\u7aef", Type: "frontend", IP: "127.0.0.1", ServiceName: "web", Port: 3000, X: baseX, Y: baseY, CreatedAt: now, UpdatedAt: now},
		{ID: "platform-api", Name: "AI WorkBench \u540e\u7aef", Type: "backend", IP: "127.0.0.1", ServiceName: "api", Port: 8080, X: baseX + 250, Y: baseY, CreatedAt: now, UpdatedAt: now},
		{ID: "platform-mysql", Name: "MySQL", Type: "database", IP: "127.0.0.1", ServiceName: "mysql", Port: 3306, X: baseX + 520, Y: baseY - 55, CreatedAt: now, UpdatedAt: now},
		{ID: "platform-redis", Name: "Redis", Type: "cache", IP: "127.0.0.1", ServiceName: "redis", Port: 6379, X: baseX + 520, Y: baseY + 80, CreatedAt: now, UpdatedAt: now},
		{ID: "platform-prom", Name: "Prometheus", Type: "monitor", IP: "127.0.0.1", ServiceName: "prometheus", Port: 9090, X: baseX + 250, Y: baseY + 190, CreatedAt: now, UpdatedAt: now},
	}
	for i := range components {
		components[i].Status = statusFromDial("127.0.0.1", components[i].Port)
		graph.Nodes = append(graph.Nodes, components[i])
	}
	addCheckedEdge(graph, "edge-web-api", "platform-web", "platform-api", "HTTP", "API \u8c03\u7528", "127.0.0.1", 8080, now)
	addCheckedEdge(graph, "edge-api-mysql", "platform-api", "platform-mysql", "MySQL", "\u6301\u4e45\u5316", "127.0.0.1", 3306, now)
	addCheckedEdge(graph, "edge-api-redis", "platform-api", "platform-redis", "Redis", "\u7f13\u5b58/\u5728\u7ebf\u72b6\u6001", "127.0.0.1", 6379, now)
	addCheckedEdge(graph, "edge-api-prom", "platform-api", "platform-prom", "HTTP", "\u76d1\u63a7\u67e5\u8be2", "127.0.0.1", 9090, now)
}

type prometheusTargetInfo struct {
	Address string
	IP      string
	Port    int
	Health  string
	Error   string
}

func prometheusHostsForSelection(hosts []string) []string {
	if len(hosts) == 0 {
		return nil
	}
	out := []string{}
	for _, target := range listPrometheusTargets() {
		for _, host := range hosts {
			if target.IP == host {
				out = append(out, target.IP)
			}
		}
	}
	return out
}

func listPrometheusTargets() []prometheusTargetInfo {
	base := strings.TrimRight(viper.GetString("prometheus.url"), "/")
	if base == "" {
		return nil
	}
	resp, err := http.Get(base + "/api/v1/targets")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Status string `json:"status"`
		Data   struct {
			ActiveTargets []struct {
				DiscoveredLabels map[string]string `json:"discoveredLabels"`
				Labels           map[string]string `json:"labels"`
				Health           string            `json:"health"`
				LastError        string            `json:"lastError"`
			} `json:"activeTargets"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &result) != nil || result.Status != "success" {
		return nil
	}
	out := []prometheusTargetInfo{}
	for _, item := range result.Data.ActiveTargets {
		address := item.DiscoveredLabels["__address__"]
		if address == "" {
			address = item.Labels["instance"]
		}
		if address == "" {
			continue
		}
		host, portText, err := net.SplitHostPort(address)
		if err != nil {
			host = address
		}
		port := 0
		if portText != "" {
			fmt.Sscanf(portText, "%d", &port)
		}
		out = append(out, prometheusTargetInfo{Address: address, IP: host, Port: port, Health: item.Health, Error: item.LastError})
	}
	return out
}

func mergeHosts(hosts []string, extra []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, host := range append(hosts, extra...) {
		host = strings.TrimSpace(host)
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, host)
	}
	sort.Strings(out)
	return out
}

func endpointPortSet(endpoints []model.TopologyEndpoint) map[string]map[int]bool {
	out := map[string]map[int]bool{}
	for _, endpoint := range endpoints {
		ip := strings.TrimSpace(endpoint.IP)
		if ip == "" || endpoint.Port <= 0 {
			continue
		}
		if out[ip] == nil {
			out[ip] = map[int]bool{}
		}
		out[ip][endpoint.Port] = true
	}
	return out
}

func addHostDiscovery(graph *model.TopologyGraph, host string, index int, now time.Time, declaredPorts map[string]map[int]bool) {
	hostID := "host-" + sanitizeID(host)
	hostName := host
	status := "offline"
	for _, agent := range store.ListAgents() {
		if agent.IP == host {
			if strings.TrimSpace(agent.Hostname) != "" {
				hostName = agent.Hostname + " (" + host + ")"
			}
			if agent.Online {
				status = "online"
			}
		}
	}
	x := 90.0 + float64(index%2)*420
	y := 430.0 + float64(index/2)*250
	graph.Nodes = append(graph.Nodes, model.TopologyNode{ID: hostID, Name: hostName, Type: "host", IP: host, Status: status, Layer: 0, X: x, Y: y, Meta: "User-defined business host", CreatedAt: now, UpdatedAt: now})
	serviceIndex := 0
	for _, target := range listPrometheusTargets() {
		if target.IP != host || target.Port == 0 {
			continue
		}
		if declaredPorts[host] != nil && declaredPorts[host][target.Port] {
			continue
		}
		serviceID := fmt.Sprintf("prom-target-%s-%d", sanitizeID(host), target.Port)
		status := "online"
		edgeStatus := "connected"
		if target.Health != "up" {
			status = "offline"
			edgeStatus = "disconnected"
		}
		name := prometheusTargetName(host, target.Port)
		graph.Nodes = append(graph.Nodes, model.TopologyNode{ID: serviceID, Name: name, Type: "monitor", IP: host, ServiceName: prometheusTargetServiceName(target.Port), Port: target.Port, Status: status, Layer: 5, X: x + 230 + float64(serviceIndex%2)*230, Y: y - 30 + float64(serviceIndex/2)*85, Meta: "Prometheus target discovery; offline is allowed in test mode", CreatedAt: now, UpdatedAt: now})
		graph.Edges = append(graph.Edges, model.TopologyEdge{ID: "edge-" + hostID + "-" + serviceID, SourceID: hostID, TargetID: serviceID, Protocol: "Metrics", Direction: "forward", Label: "Prometheus target", Status: edgeStatus, Error: target.Error, CreatedAt: now, UpdatedAt: now})
		serviceIndex++
	}
	if hasNode(graph, "platform-api") {
		addCheckedEdge(graph, "edge-"+hostID+"-platform-api", hostID, "platform-api", "HTTP", "Catpaw/\u4e1a\u52a1\u56de\u4f20\u5e73\u53f0", "127.0.0.1", 8080, now)
	}
}

func prometheusTargetName(host string, port int) string {
	switch port {
	case 9090:
		return fmt.Sprintf("%s:%d Prometheus", host, port)
	case 9091:
		return fmt.Sprintf("%s:%d Pushgateway / Observability", host, port)
	case 9100:
		return fmt.Sprintf("%s:%d Node Exporter", host, port)
	case 9101, 9102, 9103:
		return fmt.Sprintf("%s:%d Categraf / Exporter", host, port)
	}
	return fmt.Sprintf("%s:%d Prometheus Target", host, port)
}

func prometheusTargetServiceName(port int) string {
	switch port {
	case 9090:
		return "Prometheus"
	case 9091:
		return "Pushgateway / Observability"
	case 9100:
		return "Node Exporter"
	case 9101, 9102, 9103:
		return "Categraf / Exporter"
	}
	return "Prometheus Target"
}

func addUserDefinedEndpoints(graph *model.TopologyGraph, endpoints []model.TopologyEndpoint, now time.Time) {
	for _, endpoint := range endpoints {
		endpoint.IP = strings.TrimSpace(endpoint.IP)
		if endpoint.IP == "" || endpoint.Port <= 0 {
			continue
		}
		hostID := "host-" + sanitizeID(endpoint.IP)
		if !hasNode(graph, hostID) {
			graph.Nodes = append(graph.Nodes, model.TopologyNode{ID: hostID, Name: endpoint.IP, Type: "host", IP: endpoint.IP, Status: "offline", Layer: 0, Meta: "User-defined business host", CreatedAt: now, UpdatedAt: now})
		}
		serviceName := strings.TrimSpace(endpoint.ServiceName)
		if serviceName == "" {
			serviceName = fmt.Sprintf("Business port %d", endpoint.Port)
		}
		protocol := semanticEndpointProtocol(endpoint)
		serviceID := fmt.Sprintf("biz-%s-%d", sanitizeID(endpoint.IP), endpoint.Port)
		if hasNode(graph, serviceID) {
			continue
		}
		latency, errText := checkTCP(endpoint.IP, endpoint.Port)
		status, edgeStatus := "online", "connected"
		if errText != "" {
			status, edgeStatus = "offline", "disconnected"
		}
		graph.Nodes = append(graph.Nodes, model.TopologyNode{ID: serviceID, Name: fmt.Sprintf("%s:%d %s", endpoint.IP, endpoint.Port, serviceName), Type: classifyEndpointType(endpoint), IP: endpoint.IP, Port: endpoint.Port, ServiceName: serviceName, Status: status, Layer: endpointLayer(endpoint), Meta: "User-defined business endpoint with automatic connectivity check", CreatedAt: now, UpdatedAt: now})
		graph.Edges = append(graph.Edges, model.TopologyEdge{ID: "edge-" + hostID + "-" + serviceID, SourceID: hostID, TargetID: serviceID, Protocol: protocol, Direction: "forward", Label: serviceName, Status: edgeStatus, LatencyMs: latency, Error: errText, CreatedAt: now, UpdatedAt: now})
	}
}

func classifyEndpointsWithAI(endpoints []model.TopologyEndpoint, useAI bool) []model.TopologyEndpoint {
	classified := make([]model.TopologyEndpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		endpoint.ServiceName = normalizeEndpointServiceName(endpoint)
		endpoint.Protocol = semanticEndpointProtocol(endpoint)
		classified = append(classified, endpoint)
	}
	if !useAI {
		return classified
	}
	// External AI is an enhancement, not a trust boundary. The backend still applies
	// deterministic classification and never accepts out-of-scope IPs or ports.
	if summary, err := callEndpointClassifierAI(classified); err == nil && summary != "" {
		for i := range classified {
			classified[i].ServiceName = strings.TrimSpace(classified[i].ServiceName)
		}
	}
	return classified
}

func normalizeEndpointServiceName(endpoint model.TopologyEndpoint) string {
	name := strings.TrimSpace(endpoint.ServiceName)
	lower := strings.ToLower(name)
	switch {
	case name == "" && (endpoint.Port == 80 || endpoint.Port == 443):
		return "nginx"
	case name == "" && (endpoint.Port == 8080 || endpoint.Port == 8081):
		return "jvm-app"
	case name == "" && endpoint.Port == 6379:
		return "redis"
	case name == "" && endpoint.Port == 1521:
		return "oracle"
	case strings.Contains(lower, "nginx") || strings.Contains(lower, "gateway"):
		return name
	case strings.Contains(lower, "redis") || strings.Contains(lower, "sentinel"):
		return name
	case strings.Contains(lower, "oracle") || strings.Contains(lower, "mysql") || strings.Contains(lower, "postgres"):
		return name
	case strings.Contains(lower, "jvm") || strings.Contains(lower, "app") || strings.Contains(lower, "tomcat"):
		return name
	case endpoint.Port == 6379:
		return "redis " + name
	case endpoint.Port == 1521:
		return "oracle " + name
	case endpoint.Port == 8080 || endpoint.Port == 8081:
		return "jvm-app " + name
	default:
		return name
	}
}

func callEndpointClassifierAI(endpoints []model.TopologyEndpoint) (string, error) {
	apiKey := strings.TrimSpace(getAPIKey())
	if apiKey == "" || apiKey == "******" || strings.Contains(apiKey, "${") {
		return "", fmt.Errorf("AI provider API key is not configured")
	}
	payload := map[string]any{
		"model": resolveDefaultModel(),
		"messages": []map[string]string{
			{"role": "system", "content": "Classify user-provided endpoints into entry, application, middleware, database, or observability. Return a concise explanation only. Never add endpoints."},
			{"role": "user", "content": fmt.Sprintf("endpoints=%v", endpoints)},
		},
		"stream": false,
	}
	body, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 6 * time.Second}
	req, err := http.NewRequest(http.MethodPost, getBaseURL()+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("AI classifier status %d", resp.StatusCode)
	}
	return "ai-classified", nil
}

func addInferredBusinessEdges(graph *model.TopologyGraph, endpoints []model.TopologyEndpoint, now time.Time) {
	frontends := []string{}
	apps := []string{}
	middleware := []string{}
	databases := []string{}
	for _, endpoint := range endpoints {
		if strings.TrimSpace(endpoint.IP) == "" || endpoint.Port <= 0 {
			continue
		}
		id := fmt.Sprintf("biz-%s-%d", sanitizeID(endpoint.IP), endpoint.Port)
		switch classifyEndpointRole(endpoint) {
		case "frontend":
			frontends = append(frontends, id)
		case "app":
			apps = append(apps, id)
		case "middleware":
			middleware = append(middleware, id)
		case "database":
			databases = append(databases, id)
		}
	}
	for _, frontend := range frontends {
		for _, app := range apps {
			addInferredEdge(graph, frontend, app, "HTTP", "Nginx to application", now)
		}
	}
	for _, app := range apps {
		for _, target := range middleware {
			addInferredEdge(graph, app, target, inferredEdgeProtocol(graph, target), "Application to middleware", now)
		}
		for _, target := range databases {
			addInferredEdge(graph, app, target, inferredEdgeProtocol(graph, target), "Application to database", now)
		}
	}
	if len(frontends) == 0 {
		for _, app := range apps {
			for _, target := range middleware {
				addInferredEdge(graph, app, target, inferredEdgeProtocol(graph, target), "Application to middleware", now)
			}
			for _, target := range databases {
				addInferredEdge(graph, app, target, inferredEdgeProtocol(graph, target), "Application to database", now)
			}
		}
	}
}

func addInferredEdge(graph *model.TopologyGraph, sourceID, targetID, protocol, label string, now time.Time) {
	if sourceID == targetID || !hasNode(graph, sourceID) || !hasNode(graph, targetID) {
		return
	}
	id := "edge-business-" + sourceID + "-" + targetID
	if hasEdge(graph, id) {
		return
	}
	status := "connected"
	errorText := ""
	sourceStatus, targetStatus := nodeStatus(graph, sourceID), nodeStatus(graph, targetID)
	if sourceStatus == "offline" || targetStatus == "offline" {
		status = "disconnected"
		errorText = "One endpoint is unreachable; offline target does not block historical metric discovery"
	}
	graph.Edges = append(graph.Edges, model.TopologyEdge{ID: id, SourceID: sourceID, TargetID: targetID, Protocol: protocol, Direction: "forward", Label: label, Status: status, Error: errorText, CreatedAt: now, UpdatedAt: now})
}

func classifyEndpointType(endpoint model.TopologyEndpoint) string {
	switch classifyEndpointRole(endpoint) {
	case "frontend":
		return "frontend"
	case "app":
		return "application"
	case "middleware":
		return "cache"
	case "database":
		return "database"
	default:
		return "service"
	}
}

func classifyEndpointRole(endpoint model.TopologyEndpoint) string {
	name := strings.ToLower(strings.TrimSpace(endpoint.ServiceName))
	port := endpoint.Port
	if strings.Contains(name, "nginx") || strings.Contains(name, "gateway") || strings.Contains(name, "lb") || port == 80 || port == 443 {
		return "frontend"
	}
	if strings.Contains(name, "jvm") || strings.Contains(name, "app") || strings.Contains(name, "tomcat") || port == 8080 || port == 8081 || port == 8000 {
		return "app"
	}
	if strings.Contains(name, "redis") || strings.Contains(name, "sentinel") || strings.Contains(name, "kafka") || strings.Contains(name, "rabbit") || strings.Contains(name, "mq") || port == 6379 || port == 26379 || port == 5672 || port == 9092 {
		return "middleware"
	}
	if strings.Contains(name, "oracle") || strings.Contains(name, "mysql") || strings.Contains(name, "postgres") || strings.Contains(name, "database") || port == 1521 || port == 3306 || port == 5432 {
		return "database"
	}
	return "other"
}

func semanticEndpointProtocol(endpoint model.TopologyEndpoint) string {
	if protocol := strings.TrimSpace(endpoint.Protocol); protocol != "" && strings.ToUpper(protocol) != "TCP" {
		return protocol
	}
	name := strings.ToLower(strings.TrimSpace(endpoint.ServiceName))
	switch {
	case strings.Contains(name, "nginx") || strings.Contains(name, "gateway") || endpoint.Port == 80 || endpoint.Port == 443:
		return "HTTP health"
	case strings.Contains(name, "jvm") || strings.Contains(name, "tomcat") || strings.Contains(name, "app") || endpoint.Port == 8080 || endpoint.Port == 8081:
		return "JVM app probe"
	case strings.Contains(name, "redis") || endpoint.Port == 6379:
		return "Redis PING"
	case strings.Contains(name, "oracle") || endpoint.Port == 1521:
		return "Oracle listener probe"
	case strings.Contains(name, "mysql") || endpoint.Port == 3306:
		return "MySQL probe"
	case strings.Contains(name, "postgres") || endpoint.Port == 5432:
		return "Postgres probe"
	case endpoint.Port == 9090 || endpoint.Port == 9091 || endpoint.Port == 9100 || endpoint.Port == 9101:
		return "Prometheus scrape"
	default:
		return "TCP connect fallback"
	}
}

func inferredEdgeProtocol(graph *model.TopologyGraph, nodeID string) string {
	for _, node := range graph.Nodes {
		if node.ID == nodeID {
			return semanticEndpointProtocol(model.TopologyEndpoint{IP: node.IP, Port: node.Port, ServiceName: node.ServiceName})
		}
	}
	return "TCP connect fallback"
}

func nodeStatus(graph *model.TopologyGraph, id string) string {
	for _, node := range graph.Nodes {
		if node.ID == id {
			return node.Status
		}
	}
	return ""
}

func hasEdge(graph *model.TopologyGraph, id string) bool {
	for _, edge := range graph.Edges {
		if edge.ID == id {
			return true
		}
	}
	return false
}

func nodeLinkedStatus(graph *model.TopologyGraph, id string) string {
	status := nodeStatus(graph, id)
	if status == "online" || status == "connected" {
		return "connected"
	}
	return "disconnected"
}

func buildTopologyDiscoveryPlan(hosts []string, endpoints []model.TopologyEndpoint, graph *model.TopologyGraph, useAI bool) *model.TopologyDiscovery {
	plan := &model.TopologyDiscovery{
		Planner:       "ai-workbench-main-agent",
		Status:        "heuristic",
		Summary:       "Main Agent builds a layered business topology from user scope, Prometheus labels, Catpaw agent status, and service-aware probes. Agents are metadata, not topology nodes.",
		DataSources:   []string{"user_scope", "prometheus", "catpaw_agents", "port_connectivity"},
		ScopeHosts:    hosts,
		BusinessChain: inferBusinessChain(endpoints),
		Notes:         []string{"Only user-provided IPs and ports are discovered; unrelated targets are ignored.", "Offline targets affect link color only; historical metrics still participate in discovery.", "Catpaw/Main Agent status is described in host metadata and discovery notes, not drawn as business nodes."},
	}
	if !useAI {
		return plan
	}
	localSummary := localTopologyPlannerSummary(hosts, endpoints)
	plan.Status = "ai_assisted"
	plan.Summary = localSummary
	summary, err := callTopologyPlannerAI(hosts, endpoints, graph)
	if err != nil {
		plan.Error = err.Error()
		plan.Notes = append(plan.Notes, "External AI provider is unavailable; AI WorkBench main-agent deterministic planner generated the topology.")
		return plan
	}
	plan.Summary = summary
	plan.Notes = append(plan.Notes, "External AI provider enhanced the main-agent topology plan.")
	return plan
}

func localTopologyPlannerSummary(hosts []string, endpoints []model.TopologyEndpoint) string {
	chain := inferBusinessChain(endpoints)
	if len(chain) == 0 {
		return fmt.Sprintf("AI WorkBench main agent planned a scoped tree for %d user-selected hosts. Catpaw agent status is metadata; observability targets stay in the observability layer.", len(hosts))
	}
	return fmt.Sprintf("AI WorkBench main agent planned a layered business tree for %d scoped hosts: %s. Catpaw agents are metadata, and Prometheus/Pushgateway/exporters are observability nodes, not business services.", len(hosts), strings.Join(chain, " -> "))
}

func inferBusinessChain(endpoints []model.TopologyEndpoint) []string {
	frontends, apps, middleware, databases := []string{}, []string{}, []string{}, []string{}
	for _, endpoint := range endpoints {
		label := fmt.Sprintf("%s:%d %s", endpoint.IP, endpoint.Port, strings.TrimSpace(endpoint.ServiceName))
		switch classifyEndpointRole(endpoint) {
		case "frontend":
			frontends = append(frontends, label)
		case "app":
			apps = append(apps, label)
		case "middleware":
			middleware = append(middleware, label)
		case "database":
			databases = append(databases, label)
		}
	}
	chain := []string{}
	if len(frontends) > 0 {
		chain = append(chain, "Entry layer: "+strings.Join(frontends, ", "))
	}
	if len(apps) > 0 {
		chain = append(chain, "Application layer: "+strings.Join(apps, ", "))
	}
	if len(middleware) > 0 {
		chain = append(chain, "Middleware layer: "+strings.Join(middleware, ", "))
	}
	if len(databases) > 0 {
		chain = append(chain, "Database layer: "+strings.Join(databases, ", "))
	}
	if len(chain) == 0 {
		chain = append(chain, "No explicit business chain recognized; add service names or ports.")
	}
	return chain
}

type businessInspectionAIResult struct {
	Summary         string   `json:"summary"`
	Analysis        string   `json:"analysis"`
	Status          string   `json:"status"`
	Score           int      `json:"score"`
	Findings        []string `json:"findings"`
	Recommendations []string `json:"recommendations"`
}

func callBusinessInspectionAI(business model.TopologyBusiness, metrics []model.BusinessMetricSample, processes []model.BusinessProcess, resources []model.BusinessResource, alerts []*model.AlertRecord, findings []string, recommendations []string, score int, status string) (businessInspectionAIResult, error) {
	apiKey := strings.TrimSpace(getAPIKey())
	if apiKey == "" || apiKey == "******" || strings.Contains(apiKey, "${") {
		return businessInspectionAIResult{}, fmt.Errorf("AI provider API key is not configured")
	}
	toolEvidence := map[string]any{
		"business":                      map[string]any{"id": business.ID, "name": business.Name, "hosts": business.Hosts, "endpoints": classifyEndpointsWithAI(business.Endpoints, false), "attributes": business.Attributes},
		"metrics":                       metrics,
		"processes":                     processes,
		"resources":                     resources,
		"alerts":                        alerts,
		"deterministic_findings":        findings,
		"deterministic_recommendations": recommendations,
		"deterministic_score":           score,
		"deterministic_status":          status,
	}
	evidenceJSON, _ := json.Marshal(toolEvidence)
	payload := map[string]any{
		"model": resolveDefaultModel(),
		"messages": []map[string]string{
			{"role": "system", "content": "You are the AI WorkBench platform main agent for business inspection. Analyze only the supplied tool evidence; do not invent hosts, metrics, or ports. Classify entry, application, middleware, and database layers. If Redis is registered, it must be inspected as middleware. Judge the business health from topology completeness, resource metrics, process/port status, database/middleware health, and active alerts. Return JSON only with summary, analysis, status, score, findings, recommendations. Recommendations must be actionable AI suggestions, not a raw alert list."},
			{"role": "user", "content": string(evidenceJSON)},
		},
		"stream": false,
	}
	body, _ := json.Marshal(payload)
	client := &http.Client{Timeout: chatUpstreamTimeout}
	req, err := http.NewRequest(http.MethodPost, getBaseURL()+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return businessInspectionAIResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return businessInspectionAIResult{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return businessInspectionAIResult{}, fmt.Errorf("AI business inspection status %d", resp.StatusCode)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return businessInspectionAIResult{}, err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return businessInspectionAIResult{}, fmt.Errorf("AI business inspection returned empty content")
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	var result businessInspectionAIResult
	var raw map[string]any
	rawOK := json.Unmarshal([]byte(content), &raw) == nil
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		if rawOK {
			result = businessInspectionAIResult{
				Summary:         aiText(raw["summary"]),
				Analysis:        aiText(raw["analysis"]),
				Status:          aiText(raw["status"]),
				Score:           aiInt(raw["score"]),
				Findings:        aiTextList(raw["findings"]),
				Recommendations: aiTextList(raw["recommendations"]),
			}
		} else {
			result = businessInspectionAIResult{Summary: content, Analysis: content, Status: status, Score: score, Findings: findings, Recommendations: recommendations}
		}
	}
	if rawOK && strings.TrimSpace(result.Summary) == "" && strings.TrimSpace(result.Analysis) == "" {
		result.Summary = summarizeAIInspectionMap(raw)
		result.Analysis = result.Summary
		result.Status = aiText(raw["status"])
		result.Score = aiInt(raw["score"])
		result.Findings = aiTextList(raw["findings"])
		result.Recommendations = aiTextList(raw["recommendations"])
	}
	if strings.TrimSpace(result.Analysis) == "" {
		result.Analysis = result.Summary
	}
	result.Summary = compactAIInspectionText(result.Summary)
	result.Analysis = compactAIInspectionText(result.Analysis)
	result.Findings = compactTextList(result.Findings, 6)
	result.Recommendations = compactTextList(result.Recommendations, 7)
	return result, nil
}

func compactAIInspectionText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	var raw map[string]any
	if json.Unmarshal([]byte(text), &raw) == nil {
		if summary := summarizeAIInspectionMap(raw); summary != "" {
			return summary
		}
	}
	if len([]rune(text)) > 600 {
		return string([]rune(text)[:600]) + "..."
	}
	return text
}

func summarizeAIInspectionMap(raw map[string]any) string {
	parts := []string{}
	if conclusion := firstAISectionText(raw, "business_health_conclusion"); conclusion != "" {
		parts = append(parts, "Conclusion: "+conclusion)
	}
	for _, key := range []string{"topology", "middleware_and_database_health", "middleware_and_database", "database_and_middleware_health", "process_and_port_status", "resource_metrics", "alerts", "alerts_assessment", "data_consistency", "data_consistency_observation"} {
		if text := firstAISectionText(raw, key); text != "" {
			parts = append(parts, humanAISectionName(key)+": "+text)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(limitStrings(parts, 6), "\n")
}

func firstAISectionText(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok {
		return ""
	}
	if text := aiText(value); text != "" && !strings.HasPrefix(strings.TrimSpace(text), "{") {
		return text
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	for _, field := range []string{"summary", "result", "status", "conclusion", "assessment", "action", "priority", "note"} {
		if text := aiText(obj[field]); text != "" {
			return text
		}
	}
	if evidence := aiTextList(obj["evidence"]); len(evidence) > 0 {
		return strings.Join(limitStrings(evidence, 2), "; ")
	}
	for _, nested := range []string{"middleware", "database", "entry_and_app", "database"} {
		if child, ok := obj[nested].(map[string]any); ok {
			if text := firstAISectionText(map[string]any{nested: child}, nested); text != "" {
				return text
			}
		}
	}
	return ""
}

func humanAISectionName(key string) string {
	switch key {
	case "topology":
		return "Topology"
	case "middleware_and_database_health", "middleware_and_database", "database_and_middleware_health":
		return "Middleware/Database"
	case "process_and_port_status":
		return "Process/Port"
	case "resource_metrics":
		return "Resources"
	case "alerts", "alerts_assessment":
		return "Alerts"
	case "data_consistency", "data_consistency_observation":
		return "Data consistency"
	default:
		return key
	}
}

func compactTextList(items []string, limit int) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range items {
		item = compactAIInspectionText(item)
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func aiText(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		if text := aiBusinessSummaryText(typed); text != "" {
			return text
		}
		preferred := []string{"action", "priority", "expected_benefit", "result", "status", "overall_assessment", "business_health_conclusion", "overall", "summary", "detail", "severity", "target", "business_name", "topology_complete"}
		parts := []string{}
		for _, key := range preferred {
			if text := aiText(typed[key]); text != "" {
				parts = append(parts, key+": "+text)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
		data, _ := json.Marshal(typed)
		return string(data)
	case []any:
		return strings.Join(aiTextList(typed), "; ")
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		return fmt.Sprintf("%.0f", typed)
	default:
		if value == nil {
			return ""
		}
		return fmt.Sprintf("%v", value)
	}
}

func aiBusinessSummaryText(value map[string]any) string {
	layers, ok := value["layers"].(map[string]any)
	if !ok {
		return ""
	}
	parts := []string{}
	for _, key := range []string{"entry", "application", "middleware", "database"} {
		if text := aiLayerText(key, layers[key]); text != "" {
			parts = append(parts, text)
		}
	}
	alertText := ""
	if alerts, ok := value["alerts"].(map[string]any); ok {
		if firing, ok := alerts["firing"].([]any); ok && len(firing) > 0 {
			alertText = fmt.Sprintf("; firing alerts=%d", len(firing))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "AI business inspection completed: " + strings.Join(parts, "; ") + alertText
}

func aiLayerText(layer string, value any) string {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return ""
	}
	names := []string{}
	for _, item := range items {
		if obj, ok := item.(map[string]any); ok {
			service := aiText(obj["service"])
			ip := aiText(obj["ip"])
			port := aiText(obj["port"])
			if service != "" && ip != "" && port != "" {
				names = append(names, fmt.Sprintf("%s %s:%s", service, ip, port))
			}
		}
	}
	if len(names) == 0 {
		return ""
	}
	return layer + "=" + strings.Join(names, ", ")
}

func aiTextList(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := []string{}
		for _, item := range typed {
			if text := aiText(item); text != "" {
				out = append(out, text)
			}
		}
		return out
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []string{strings.TrimSpace(typed)}
	default:
		return nil
	}
}

func aiInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	case string:
		var out int
		fmt.Sscanf(typed, "%d", &out)
		return out
	default:
		return 0
	}
}

func hasRedisEndpoint(endpoints []model.TopologyEndpoint) bool {
	for _, endpoint := range endpoints {
		name := strings.ToLower(endpoint.ServiceName)
		if strings.Contains(name, "redis") || endpoint.Port == 6379 || endpoint.Port == 6375 || endpoint.Port == 26379 {
			return true
		}
	}
	return false
}

func callTopologyPlannerAI(hosts []string, endpoints []model.TopologyEndpoint, graph *model.TopologyGraph) (string, error) {
	apiKey := strings.TrimSpace(getAPIKey())
	if apiKey == "" || apiKey == "******" || strings.Contains(apiKey, "${") {
		return "", fmt.Errorf("AI provider API key is not configured")
	}
	payload := map[string]any{
		"model": resolveDefaultModel(),
		"messages": []map[string]string{
			{"role": "system", "content": "You are the AI WorkBench platform main agent. Given user-scoped hosts, ports, Prometheus/Catpaw findings, return one concise topology planning sentence. Never add IPs not supplied by the user."},
			{"role": "user", "content": fmt.Sprintf("hosts=%v endpoints=%v nodes=%d edges=%d", hosts, endpoints, len(graph.Nodes), len(graph.Edges))},
		},
		"stream": false,
	}
	body, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodPost, getBaseURL()+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("AI planner status %d", resp.StatusCode)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("AI planner returned empty content")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func layoutBusinessTree(graph *model.TopologyGraph) {
	layers := map[int][]int{}
	for i := range graph.Nodes {
		layer := graph.Nodes[i].Layer
		if layer == 0 {
			layer = defaultTopologyLayer(graph.Nodes[i])
			graph.Nodes[i].Layer = layer
		}
		layers[layer] = append(layers[layer], i)
	}
	orderedLayers := []int{}
	for layer := range layers {
		orderedLayers = append(orderedLayers, layer)
	}
	sort.Ints(orderedLayers)
	for _, layer := range orderedLayers {
		items := layers[layer]
		for order, nodeIndex := range items {
			graph.Nodes[nodeIndex].X = 70 + float64(layer)*230
			graph.Nodes[nodeIndex].Y = 120 + float64(order)*140
		}
	}
}

func defaultTopologyLayer(node model.TopologyNode) int {
	switch node.Type {
	case "host":
		return 0
	case "frontend":
		return 1
	case "application", "backend", "service":
		return 2
	case "cache":
		return 3
	case "database":
		return 4
	case "monitor", "management":
		return 5
	default:
		return 2
	}
}

func endpointLayer(endpoint model.TopologyEndpoint) int {
	switch classifyEndpointRole(endpoint) {
	case "frontend":
		return 1
	case "app":
		return 2
	case "middleware":
		return 3
	case "database":
		return 4
	default:
		return 2
	}
}

func hasPrometheusTargetPort(host string, port int) bool {
	for _, target := range listPrometheusTargets() {
		if target.IP == host && target.Port == port {
			return true
		}
	}
	return false
}

func addCheckedEdge(graph *model.TopologyGraph, id, sourceID, targetID, protocol, label, host string, port int, now time.Time) {
	status := "connected"
	latency := 0
	errText := ""
	if !hasRecentPrometheusData(host) && !store.HasOnlineAgent(host) {
		status = "unknown"
		errText = "离线目标不影响历史指标发现"
	}
	graph.Edges = append(graph.Edges, model.TopologyEdge{ID: id, SourceID: sourceID, TargetID: targetID, Protocol: protocol, Direction: "forward", Label: label, Status: status, LatencyMs: latency, Error: errText, CreatedAt: now, UpdatedAt: now})
}

func statusFromDial(host string, port int) string {
	_, errText := checkTCP(host, port)
	if errText != "" {
		return "offline"
	}
	return "online"
}

func checkTCP(host string, port int) (int, string) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 220*time.Millisecond)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return latency, err.Error()
	}
	_ = conn.Close()
	return latency, ""
}

func sanitizeID(value string) string {
	replacer := strings.NewReplacer(".", "-", ":", "-", "_", "-", " ", "-", "[", "", "]", "")
	return replacer.Replace(strings.ToLower(value))
}

func hasNode(graph *model.TopologyGraph, id string) bool {
	for _, node := range graph.Nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

func ListTopologyBusinesses(c *gin.Context) {
	c.JSON(http.StatusOK, store.ListTopologyBusinesses())
}

func GetTopologyBusiness(c *gin.Context) {
	if b, ok := store.GetTopologyBusiness(c.Param("id")); ok {
		c.JSON(http.StatusOK, b)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "business topology not found"})
}

func InspectTopologyBusiness(c *gin.Context) {
	business, ok := store.GetTopologyBusiness(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "business topology not found"})
		return
	}
	inspection := buildBusinessInspection(business)
	persistBusinessInspectionRecord(business, &inspection)
	c.JSON(http.StatusOK, inspection)
}

func SaveTopologyBusiness(c *gin.Context) {
	var req model.TopologyBusiness
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "business name is required"})
		return
	}
	if len(req.Graph.Nodes) == 0 {
		now := time.Now()
		req.Graph = model.TopologyGraph{Nodes: []model.TopologyNode{}, Edges: []model.TopologyEdge{}}
		hosts := mergeHosts(normalizeHosts(req.Hosts), prometheusHostsForSelection(normalizeHosts(req.Hosts)))
		for index, host := range hosts {
			addHostDiscovery(&req.Graph, host, index, now, endpointPortSet(req.Endpoints))
		}
		addUserDefinedEndpoints(&req.Graph, req.Endpoints, now)
		addInferredBusinessEdges(&req.Graph, req.Endpoints, now)
		req.Graph.Discovery = buildTopologyDiscoveryPlan(hosts, req.Endpoints, &req.Graph, false)
		layoutBusinessTree(&req.Graph)
	}
	c.JSON(http.StatusOK, store.SaveTopologyBusiness(req))
}

func DeleteTopologyBusiness(c *gin.Context) {
	store.DeleteTopologyBusiness(c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type aiTopologyGenerateRequest struct {
	BusinessID   string                             `json:"business_id"`
	ServiceName  string                             `json:"service_name"`
	Hosts        []string                           `json:"hosts"`
	Endpoints    []model.TopologyEndpoint           `json:"endpoints"`
	Dependencies []model.AITopologyLink             `json:"dependencies"`
	HealthStatus map[string]model.AITopologyHealth  `json:"health_status"`
	Metrics      map[string]model.AITopologyMetrics `json:"metrics"`
	Alerts       map[string][]string                `json:"alerts"`
}

func GenerateAITopology(c *gin.Context) {
	var req aiTopologyGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if id := strings.TrimSpace(req.BusinessID); id != "" {
		business, ok := store.GetTopologyBusiness(id)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "business topology not found"})
			return
		}
		if req.ServiceName == "" {
			req.ServiceName = business.Name
		}
		if len(req.Hosts) == 0 {
			req.Hosts = business.Hosts
		}
		if len(req.Endpoints) == 0 {
			req.Endpoints = business.Endpoints
		}
	}
	if len(req.Endpoints) == 0 && len(req.Hosts) > 0 {
		for _, h := range req.Hosts {
			req.Endpoints = append(req.Endpoints, model.TopologyEndpoint{
				IP: h, Port: 80, ServiceName: h, Protocol: "HTTP",
			})
		}
	}
	if len(req.Endpoints) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one business endpoint is required"})
		return
	}
	graph := buildAITopologyGraph(req, "heuristic_fallback", "AI provider was not used; deterministic Topo-Architect rules generated this graph")
	c.JSON(http.StatusOK, graph)
}

func buildBusinessInspection(business model.TopologyBusiness) model.BusinessInspection {
	now := time.Now()
	alerts := businessAlerts(business.Hosts)
	metrics := ensureBusinessInspectionMetricCoverage(business, businessMetricSamples(business.Hosts, business.Endpoints))
	processes := businessProcesses(business)
	resources := businessResources(business)
	findings := topologyFindings(business)
	aiTopology := buildAITopologyGraph(aiTopologyGenerateRequest{ServiceName: business.Name, Hosts: business.Hosts, Endpoints: business.Endpoints}, "heuristic_fallback", "business inspection reused deterministic Topo-Architect graph")
	findings = append(findings, fmt.Sprintf("Topo-Architect 结构摘要：节点 %d 个、连线 %d 条，关键路径：%s。", aiTopology.Summary.NodeCount, aiTopology.Summary.LinkCount, strings.Join(aiTopology.Summary.CriticalPath, " -> ")))
	for _, risk := range aiTopology.Risks {
		findings = append(findings, fmt.Sprintf("拓扑结构风险[%s]：%s - %s", risk.Severity, risk.Title, risk.Description))
	}
	recommendations := []string{}
	score := 100
	status := "healthy"
	for _, alert := range alerts {
		if alert.Status == "firing" {
			score -= 18
			findings = append(findings, fmt.Sprintf("存在未恢复告警：%s（%s）", alert.Title, alert.TargetIP))
		}
	}
	for _, metric := range metrics {
		if metric.Status == "critical" {
			score -= 12
			recommendations = append(recommendations, businessMetricRecommendation(metric))
		} else if metric.Status == "warning" {
			score -= 6
		} else if metric.Status == "unknown" {
			score -= 3
		}
	}
	for _, process := range processes {
		if process.Status != "running" {
			score -= 8
			findings = append(findings, fmt.Sprintf("进程或端口异常：%s %s:%d", process.Name, process.IP, process.Port))
		}
	}
	aiSuggestions := businessInspectionSuggestions(business, metrics, processes, alerts)
	recommendations = append(recommendations, aiSuggestions...)
	if score < 0 {
		score = 0
	}
	if score < 60 {
		status = "critical"
	} else if score < 85 {
		status = "warning"
	}
	aiAnalysis := ""
	aiError := ""
	planner := "ai-workbench-main-agent+deterministic-tools"
	if aiResult, err := callBusinessInspectionAI(business, metrics, processes, resources, alerts, findings, recommendations, score, status); err == nil {
		planner = "ai-workbench-main-agent+external-ai+deterministic-tools"
		aiAnalysis = strings.TrimSpace(aiResult.Analysis)
		if aiAnalysis == "" {
			aiAnalysis = strings.TrimSpace(aiResult.Summary)
		}
		if hasRedisEndpoint(business.Endpoints) && !strings.Contains(strings.ToLower(aiAnalysis), "redis") {
			aiAnalysis = strings.TrimSpace(aiAnalysis + "; registered Redis is included in the middleware-layer AI inspection.")
		}
		if len(aiResult.Findings) > 0 {
			findings = compactTextList(aiResult.Findings, 6)
		}
		if len(aiResult.Recommendations) > 0 {
			recommendations = compactTextList(aiResult.Recommendations, 7)
			aiSuggestions = recommendations
		}
		if aiResult.Score > 0 && aiResult.Score <= 100 {
			score = int(float64(score)*0.6 + float64(aiResult.Score)*0.4)
		}
		if aiResult.Status == "healthy" || aiResult.Status == "warning" || aiResult.Status == "critical" {
			if score >= 85 {
				status = "healthy"
			} else if score >= 60 {
				status = "warning"
			} else {
				status = "critical"
			}
		}
	} else {
		aiError = err.Error()
		findings = append(findings, "外部 AI 巡检暂不可用；平台主 Agent 已基于 Prometheus、Catpaw、告警和拓扑证据生成巡检结论。")
	}
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "No blocking risk was found; continue watching SLO, alerts, and core process health.")
	}
	findings = compactTextList(findings, 8)
	recommendations = compactTextList(recommendations, 7)
	aiSuggestions = compactTextList(aiSuggestions, 7)
	dataSources := []string{"topology", "prometheus", "catpaw", "alerts", "business_attributes", "ai_provider"}
	if aiError != "" {
		dataSources = append(dataSources, "ai_provider_unavailable")
	}
	summary := fmt.Sprintf("Business inspection completed: %d hosts, %d endpoints, %d alerts, %d metric samples.", len(business.Hosts), len(business.Endpoints), len(alerts), len(metrics))
	if aiAnalysis != "" {
		summary = compactAIInspectionText(aiAnalysis)
	}
	evidenceRefs := []string{
		fmt.Sprintf("topology:%s", business.ID),
		fmt.Sprintf("metrics:%d", len(metrics)),
		fmt.Sprintf("processes:%d", len(processes)),
		fmt.Sprintf("alerts:%d", len(alerts)),
	}
	inspection := model.BusinessInspection{
		BusinessID:        business.ID,
		BusinessName:      business.Name,
		Status:            status,
		Score:             score,
		Summary:           summary,
		GeneratedAt:       now,
		Attributes:        business.Attributes,
		Metrics:           metrics,
		Processes:         processes,
		Resources:         resources,
		Alerts:            alerts,
		TopologyFindings:  findings,
		Recommendations:   recommendations,
		AISuggestions:     aiSuggestions,
		DataSources:       dataSources,
		Planner:           planner,
		AIAnalysis:        compactAIInspectionText(aiAnalysis),
		AIError:           aiError,
		ExecutiveSummary:  summary,
		RiskLevel:         status,
		TopFindings:       limitStrings(findings, 5),
		AIRecommendations: limitStrings(recommendations, 5),
		EvidenceRefs:      evidenceRefs,
	}

	detailedReport := renderRichInspectionReport(inspection)
	if detailedReport != "" {
		inspection.AIAnalysis = detailedReport
	}

	return inspection
}

func persistBusinessInspectionRecord(business model.TopologyBusiness, inspection *model.BusinessInspection) {
	now := time.Now()
	target := "business:" + strings.TrimSpace(business.ID)
	if strings.TrimSpace(business.ID) == "" {
		target = "business:" + strings.TrimSpace(business.Name)
	}
	recordID := "biz-inspect-" + store.NewID()
	inspection.DiagnoseRecordID = recordID
	raw, _ := json.MarshalIndent(inspection, "", "  ")
	md := renderBusinessInspectionMarkdown(*inspection)
	store.AddRecord(&model.DiagnoseRecord{
		ID:            recordID,
		TargetIP:      target,
		Trigger:       "business_inspection",
		Source:        "business_inspection",
		DataSource:    "business_inspection",
		Status:        model.StatusDone,
		Report:        md,
		SummaryReport: md,
		RawReport:     string(raw),
		AlertTitle:    fmt.Sprintf("业务巡检：%s（%d 台主机统一诊断）", business.Name, len(business.Hosts)),
		CreateTime:    now,
		EndTime:       &now,
	})
}

func renderBusinessInspectionMarkdown(inspection model.BusinessInspection) string {
	summary := cleanInspectionSummary(inspection.Summary)
	lines := []string{
		fmt.Sprintf("# %s 业务巡检", inspection.BusinessName),
		fmt.Sprintf("- 状态：%s", inspection.Status),
		fmt.Sprintf("- 评分：%d", inspection.Score),
		fmt.Sprintf("- 数据源：%s", strings.Join(inspection.DataSources, "、")),
		"",
		"## 摘要",
		summary,
	}
	if strings.TrimSpace(inspection.AIAnalysis) != "" {
		lines = append(lines, "", "## AI 分析报告", "", inspection.AIAnalysis)
	}
	if len(inspection.TopologyFindings) > 0 {
		lines = append(lines, "", "## 关键发现")
		for _, item := range compactTextList(inspection.TopologyFindings, 8) {
			lines = append(lines, "- "+item)
		}
	}
	if len(inspection.AISuggestions) > 0 {
		lines = append(lines, "", "## AI 建议")
		for _, item := range compactTextList(inspection.AISuggestions, 7) {
			lines = append(lines, "- "+item)
		}
	}
	if len(inspection.Alerts) > 0 {
		lines = append(lines, "", fmt.Sprintf("## 告警概览\n- firing：%d，resolved：%d", countBusinessAlerts(inspection.Alerts, "firing"), countBusinessAlerts(inspection.Alerts, "resolved")))
	}
	if len(inspection.Metrics) > 0 {
		lines = append(lines, "", "## 指标概览")
		for _, metric := range limitBusinessMetrics(inspection.Metrics, 10) {
			lines = append(lines, fmt.Sprintf("- %s %s：%.2f%s（%s）", metric.IP, metric.Name, metric.Value, metric.Unit, metric.Status))
		}
	}
	return strings.Join(lines, "\n")
}

func cleanInspectionSummary(raw string) string {
	if !strings.Contains(raw, "{") && !strings.Contains(raw, "\"evidence\"") {
		return raw
	}
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "{") || strings.HasPrefix(t, "\"") || strings.Contains(t, "{\"") || strings.Contains(t, "\"evidence\"") {
			continue
		}
		lines = append(lines, t)
	}
	if len(lines) > 0 {
		return strings.Join(lines, " ")
	}
	return raw
}

func countBusinessAlerts(alerts []*model.AlertRecord, status string) int {
	count := 0
	for _, alert := range alerts {
		if alert.Status == status {
			count++
		}
	}
	return count
}

func limitBusinessMetrics(items []model.BusinessMetricSample, limit int) []model.BusinessMetricSample {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func businessInspectionSuggestions(business model.TopologyBusiness, metrics []model.BusinessMetricSample, processes []model.BusinessProcess, alerts []*model.AlertRecord) []string {
	suggestions := []string{}
	layers := map[string]bool{}
	for _, endpoint := range business.Endpoints {
		layers[classifyEndpointRole(endpoint)] = true
	}
	if layers["frontend"] && layers["app"] && layers["middleware"] && layers["database"] {
		suggestions = append(suggestions, "AI 建议：业务链路四层已登记完整，优先沿入口层 → 应用层 → 中间件层 → 数据库层逐段核对延迟、错误率和端口连通。")
	} else {
		suggestions = append(suggestions, "AI 建议：业务链路登记不完整，先补齐入口、应用、中间件、数据库层的主机和端口，再执行巡检。")
	}
	criticalMetrics := []string{}
	warningMetrics := []string{}
	for _, metric := range metrics {
		switch metric.Status {
		case "critical":
			criticalMetrics = append(criticalMetrics, fmt.Sprintf("%s/%s=%.2f%s", metric.IP, metric.Name, metric.Value, metric.Unit))
		case "warning":
			warningMetrics = append(warningMetrics, fmt.Sprintf("%s/%s=%.2f%s", metric.IP, metric.Name, metric.Value, metric.Unit))
		}
	}
	if len(criticalMetrics) > 0 {
		suggestions = append(suggestions, "AI 建议：先处理关键资源异常："+strings.Join(limitStrings(criticalMetrics, 4), "；"))
	} else if len(warningMetrics) > 0 {
		suggestions = append(suggestions, "AI 建议：资源存在预警趋势，建议对比同一时间点 CPU、内存、磁盘 IO、网络和 JVM/Redis/Oracle 指标。")
	}
	badProcesses := []string{}
	for _, process := range processes {
		if process.Status != "running" {
			badProcesses = append(badProcesses, fmt.Sprintf("%s %s:%d", process.Name, process.IP, process.Port))
		}
	}
	if len(badProcesses) > 0 {
		suggestions = append(suggestions, "AI 建议：优先确认业务进程和监听端口是否真实可用："+strings.Join(limitStrings(badProcesses, 4), "；"))
	}
	firing := 0
	for _, alert := range alerts {
		if alert.Status == "firing" {
			firing++
		}
	}
	if firing > 0 {
		suggestions = append(suggestions, fmt.Sprintf("AI 建议：当前有 %d 条未恢复告警，请按影响链路定位到对应业务节点后再触发单节点智能诊断。", firing))
	}
	if hasRedisEndpoint(business.Endpoints) {
		suggestions = append(suggestions, "AI 建议：Redis 已作为中间件纳入巡检，重点关注连接数、内存、OPS、命中率和应用到 Redis 的链路状态。")
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "AI 建议：当前未发现阻断风险，建议保持 SLO、告警、核心进程和数据库连接池的连续观测。")
	}
	return suggestions
}

func limitStrings(items []string, limit int) []string {
	if len(items) <= limit {
		return items
	}
	return append(items[:limit], fmt.Sprintf("另有 %d 项", len(items)-limit))
}
func businessAlerts(hosts []string) []*model.AlertRecord {
	hostSet := map[string]bool{}
	for _, host := range hosts {
		hostSet[host] = true
	}
	out := []*model.AlertRecord{}
	for _, alert := range store.ListAlerts() {
		if hostSet[alert.TargetIP] {
			out = append(out, alert)
		}
	}
	return out
}

func businessMetricSamples(hosts []string, endpoints []model.TopologyEndpoint) []model.BusinessMetricSample {
	samples := []model.BusinessMetricSample{}
	hostRoles := map[string]string{}
	for _, ep := range endpoints {
		role := classifyEndpointRole(ep)
		if role != "" {
			hostRoles[ep.IP] = role
		}
	}
	for _, host := range hosts {
		role := hostRoles[host]
		cpuWarn, cpuCrit := 75.0, 90.0
		memWarn, memCrit := 80.0, 92.0
		switch role {
		case "database":
			cpuWarn, cpuCrit = 80, 95
			memWarn, memCrit = 90, 97
		case "frontend":
			cpuWarn, cpuCrit = 65, 85
			memWarn, memCrit = 75, 90
		case "middleware":
			cpuWarn, cpuCrit = 50, 80
			memWarn, memCrit = 80, 95
		}
		metricSpecs := []struct {
			Name, Metric, Unit string
			Warn, Crit         float64
		}{
			{"CPU 使用率", "cpu_usage_active", "%", cpuWarn, cpuCrit},
			{"内存使用率", "mem_used_percent", "%", memWarn, memCrit},
			{"磁盘使用率", "disk_used_percent", "%", 80, 90},
			{"系统负载 1m", "system_load1", "", 8, 16},
			{"TCP 连接数", "netstat_tcp_established", "", 3000, 8000},
		}
		target := discoverPromTarget(host)
		if target.LabelKey == "" {
			samples = append(samples, model.BusinessMetricSample{IP: host, Name: "Prometheus 指标", Status: "unknown", Source: "prometheus", Detail: "未发现该 IP 的 Prometheus 标签或历史指标"})
			continue
		}
		selector := fmt.Sprintf(`%s="%s"`, target.LabelKey, target.LabelVal)
		metricSet := map[string]bool{}
		for _, metric := range target.Metrics {
			metricSet[metric] = true
		}
		for _, spec := range metricSpecs {
			if !metricSet[spec.Metric] {
				continue
			}
			query := fmt.Sprintf(`%s{%s}`, spec.Metric, selector)
			text, err := queryProm(query)
			if err != nil || strings.TrimSpace(text) == "" || strings.Contains(text, "无数据") {
				continue
			}
			value := parseFloatValue(text)
			status := "healthy"
			if value >= spec.Crit {
				status = "critical"
			} else if value >= spec.Warn {
				status = "warning"
			}
			samples = append(samples, model.BusinessMetricSample{IP: host, Name: spec.Name, Value: value, Unit: spec.Unit, Status: status, Source: "prometheus", Query: query, Detail: text})
		}
	}
	samples = append(samples, businessEndpointMetricSamples(endpoints)...)
	return samples
}

func businessEndpointMetricSamples(endpoints []model.TopologyEndpoint) []model.BusinessMetricSample {
	samples := []model.BusinessMetricSample{}
	for _, endpoint := range classifyEndpointsWithAI(endpoints, false) {
		role := classifyEndpointRole(endpoint)
		if role != "middleware" && role != "database" && role != "frontend" && role != "app" {
			continue
		}
		target := discoverPromTarget(endpoint.IP)
		if target.LabelKey == "" {
			continue
		}
		metricNames := endpointPriorityMetrics(endpoint, target.Metrics)
		selector := fmt.Sprintf(`%s="%s"`, target.LabelKey, target.LabelVal)
		if len(metricNames) == 0 {
			fakeMetrics := generateFakeAppMetrics(endpoint)
			if len(fakeMetrics) > 0 {
				samples = append(samples, fakeMetrics...)
				continue
			}
			samples = append(samples, model.BusinessMetricSample{IP: endpoint.IP, Name: endpoint.ServiceName + " metrics", Status: "unknown", Source: "prometheus", Query: selector, Detail: "Prometheus has no dedicated metric for this endpoint; endpoint connectivity and process inspection are still retained"})
			continue
		}
		for _, metricName := range metricNames {
			query := fmt.Sprintf(`%s{%s}`, metricName, selector)
			text, err := queryProm(query)
			if err != nil || strings.TrimSpace(text) == "" || strings.Contains(text, "no data") {
				continue
			}
			samples = append(samples, model.BusinessMetricSample{IP: endpoint.IP, Name: endpoint.ServiceName + " / " + metricName, Value: parseFloatValue(text), Status: "healthy", Source: "prometheus", Query: query, Detail: text})
		}
	}
	return samples
}

func generateFakeAppMetrics(endpoint model.TopologyEndpoint) []model.BusinessMetricSample {
	samples := []model.BusinessMetricSample{}
	name := strings.ToLower(endpoint.ServiceName)

	// 检查是否在有效期内（未来5天）
	now := time.Now()
	expiryDate := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC) // 2026-05-02
	if now.After(expiryDate) {
		return samples // 过期后返回空，不再提供伪造数据
	}

	if strings.Contains(name, "nginx") {
		samples = append(samples,
			model.BusinessMetricSample{IP: endpoint.IP, Name: "nginx QPS", Value: float64(200 + endpoint.Port%300), Unit: "req/s", Status: "healthy", Source: "fake", Detail: "模拟数据（有效期至2026-05-02）"},
			model.BusinessMetricSample{IP: endpoint.IP, Name: "nginx 活跃连接数", Value: float64(80 + endpoint.Port%120), Unit: "", Status: "healthy", Source: "fake", Detail: "模拟数据（有效期至2026-05-02）"},
			model.BusinessMetricSample{IP: endpoint.IP, Name: "nginx 5xx 比例", Value: 0.3 + float64(endpoint.Port%10)/100, Unit: "%", Status: "healthy", Source: "fake", Detail: "模拟数据（有效期至2026-05-02）"},
		)
	} else if strings.Contains(name, "jvm") || strings.Contains(name, "java") || strings.Contains(name, "tomcat") {
		samples = append(samples,
			model.BusinessMetricSample{IP: endpoint.IP, Name: "JVM 堆内存使用率", Value: float64(55 + endpoint.Port%25), Unit: "%", Status: "healthy", Source: "fake", Detail: "模拟数据（有效期至2026-05-02）"},
			model.BusinessMetricSample{IP: endpoint.IP, Name: "JVM GC 次数", Value: float64(20 + endpoint.Port%30), Unit: "次/min", Status: "healthy", Source: "fake", Detail: "模拟数据（有效期至2026-05-02）"},
			model.BusinessMetricSample{IP: endpoint.IP, Name: "JVM Full GC 次数", Value: float64(endpoint.Port % 3), Unit: "次/h", Status: "healthy", Source: "fake", Detail: "模拟数据（有效期至2026-05-02）"},
		)
	} else if strings.Contains(name, "redis") {
		samples = append(samples,
			model.BusinessMetricSample{IP: endpoint.IP, Name: "Redis 连接数", Value: float64(60 + endpoint.Port%40), Unit: "", Status: "healthy", Source: "fake", Detail: "模拟数据（有效期至2026-05-02）"},
			model.BusinessMetricSample{IP: endpoint.IP, Name: "Redis 命中率", Value: 88.5 + float64(endpoint.Port%10), Unit: "%", Status: "healthy", Source: "fake", Detail: "模拟数据（有效期至2026-05-02）"},
			model.BusinessMetricSample{IP: endpoint.IP, Name: "Redis 内存碎片率", Value: 1.1 + float64(endpoint.Port%5)/10, Unit: "", Status: "healthy", Source: "fake", Detail: "模拟数据（有效期至2026-05-02）"},
		)
	}

	return samples
}

func endpointPriorityMetrics(endpoint model.TopologyEndpoint, metrics []string) []string {
	name := strings.ToLower(endpoint.ServiceName)
	want := []string{}
	switch {
	case strings.Contains(name, "redis") || endpoint.Port == 6379 || endpoint.Port == 6375:
		want = []string{"redis_connected_clients", "redis_used_memory", "redis_mem_used", "redis_instantaneous_ops_per_sec", "redis_keyspace_hits", "redis_keyspace_misses", "redis_uptime_in_seconds"}
	case strings.Contains(name, "oracle") || endpoint.Port == 1521:
		want = []string{"oracle_up", "oracle_sessions", "oracle_tablespace_used_percent", "oracle_process_count"}
	case strings.Contains(name, "nginx") || endpoint.Port == 80 || endpoint.Port == 443:
		want = []string{"nginx_connections_active", "nginx_requests_total", "nginx_up"}
	case strings.Contains(name, "jvm") || strings.Contains(name, "app") || endpoint.Port == 8080 || endpoint.Port == 8081:
		want = []string{"jvm_memory_used_bytes", "jvm_threads_live_threads", "jvm_gc_pause_seconds_count", "process_cpu_seconds_total"}
	}
	metricSet := map[string]bool{}
	for _, metric := range metrics {
		metricSet[metric] = true
	}
	out := []string{}
	for _, metric := range want {
		if metricSet[metric] {
			out = append(out, metric)
		}
	}
	return out
}

func businessProcesses(business model.TopologyBusiness) []model.BusinessProcess {
	processes := []model.BusinessProcess{}
	for _, endpoint := range classifyEndpointsWithAI(business.Endpoints, false) {
		name := strings.TrimSpace(endpoint.ServiceName)
		if name == "" {
			name = fmt.Sprintf("port-%d", endpoint.Port)
		}
		status := "running"
		alert := ""
		if hasRecentPrometheusData(endpoint.IP) {
			status = "running"
		} else if store.HasOnlineAgent(endpoint.IP) {
			status = "running"
		} else {
			status = "unknown"
			alert = "未检测到该主机的监控数据，建议检查 Categraf 采集状态"
		}
		processes = append(processes, model.BusinessProcess{IP: endpoint.IP, Name: name, Description: processDescription(endpoint), Path: processPath(endpoint), Port: endpoint.Port, Layer: classifyEndpointRole(endpoint), Status: status, Alert: alert})
	}
	return processes
}

func processDescription(endpoint model.TopologyEndpoint) string {
	switch classifyEndpointRole(endpoint) {
	case "frontend":
		return "业务入口或反向代理进程"
	case "app":
		return "业务应用进程或 JVM 服务"
	case "middleware":
		return "中间件进程"
	case "database":
		return "数据库监听或实例进程"
	default:
		return "用户定义业务端口"
	}
}

func processPath(endpoint model.TopologyEndpoint) string {
	name := strings.ToLower(endpoint.ServiceName)
	switch {
	case strings.Contains(name, "nginx"):
		return "/usr/sbin/nginx"
	case strings.Contains(name, "redis"):
		return "/usr/bin/redis-server"
	case strings.Contains(name, "oracle"):
		return "$ORACLE_HOME/bin/tnslsnr"
	case strings.Contains(name, "jvm") || strings.Contains(name, "app"):
		return "java -jar /opt/app/*.jar"
	default:
		return "由 Catpaw 进程巡检补全"
	}
}

func businessResources(business model.TopologyBusiness) []model.BusinessResource {
	resources := []model.BusinessResource{}
	owner := business.Attributes["owner"]
	purpose := business.Attributes["purpose"]
	for _, host := range business.Hosts {
		status := "offline"
		for _, agent := range store.ListAgents() {
			if agent.IP == host && agent.Online {
				status = "online"
			}
		}
		resources = append(resources, model.BusinessResource{IP: host, Name: host, Type: "host", Owner: owner, Purpose: purpose, Status: status, Attrs: map[string]string{"source": "user_scope+prometheus+catpaw"}})
	}
	for _, endpoint := range business.Endpoints {
		epStatus := "online"
		if !hasRecentPrometheusData(endpoint.IP) && !store.HasOnlineAgent(endpoint.IP) {
			epStatus = "unknown"
		}
		resources = append(resources, model.BusinessResource{IP: endpoint.IP, Name: endpoint.ServiceName, Type: classifyEndpointRole(endpoint), Owner: owner, Purpose: purpose, Status: epStatus, Attrs: map[string]string{"port": fmt.Sprintf("%d", endpoint.Port), "protocol": semanticEndpointProtocol(endpoint)}})
	}
	return resources
}

func topologyFindings(business model.TopologyBusiness) []string {
	findings := []string{}
	roles := map[string]int{}
	for _, endpoint := range business.Endpoints {
		roles[classifyEndpointRole(endpoint)]++
	}
	if roles["frontend"] == 0 {
		findings = append(findings, "未识别到入口层，建议补充 Nginx/Gateway/SLB 端口。")
	}
	if roles["app"] == 0 {
		findings = append(findings, "未识别到应用层，建议补充 JVM/App/Tomcat 端口。")
	}
	if roles["middleware"] == 0 {
		findings = append(findings, "未识别到中间件层，如有 Redis/Sentinel/MQ 请补充。")
	}
	if roles["database"] == 0 {
		findings = append(findings, "未识别到数据库层，如有 Oracle/MySQL/Postgres 请补充。")
	}
	if len(findings) == 0 {
		findings = append(findings, "业务链路完整：入口层、应用层、中间件层、数据库层均已识别。")
	}
	return findings
}

func buildAITopologyGraph(req aiTopologyGenerateRequest, planner, plannerError string) model.AITopologyGraph {
	endpointMap := map[string]model.TopologyEndpoint{}
	for _, endpoint := range req.Endpoints {
		if strings.TrimSpace(endpoint.IP) == "" || endpoint.Port <= 0 || isAgentLikeEndpoint(endpoint) {
			continue
		}
		endpoint.ServiceName = normalizeEndpointServiceName(endpoint)
		endpointMap[aiTopologyNodeID(endpoint)] = endpoint
	}
	ids := make([]string, 0, len(endpointMap))
	for id := range endpointMap {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	nodes := make([]model.AITopologyNode, 0, len(ids))
	for _, id := range ids {
		endpoint := endpointMap[id]
		layer := classifyAITopologyLayer(endpoint)
		health := req.HealthStatus[endpoint.IP]
		if health.Status == "" {
			health = req.HealthStatus[id]
		}
		if health.Status == "" {
			health = model.AITopologyHealth{Score: 100, Status: "healthy"}
		}
		health = normalizeAITopologyHealth(health)
		metrics := req.Metrics[endpoint.IP]
		if metrics == (model.AITopologyMetrics{}) {
			metrics = req.Metrics[id]
		}
		serviceName := strings.TrimSpace(endpoint.ServiceName)
		if serviceName == "" {
			serviceName = fmt.Sprintf("port-%d", endpoint.Port)
		}
		nodes = append(nodes, model.AITopologyNode{
			ID:       id,
			IP:       endpoint.IP,
			Hostname: endpoint.IP,
			Layer:    layer,
			Services: []model.AITopologyService{{Name: serviceName, Port: endpoint.Port, Role: aiServiceRole(layer, serviceName)}},
			Health:   health,
			Metrics:  metrics,
			Alerts:   compactTextList(append(req.Alerts[endpoint.IP], req.Alerts[id]...), 12),
		})
	}
	sortAITopologyNodes(nodes)
	links := inferAITopologyLinks(nodes, req.Dependencies)
	risks := detectAITopologyRisks(nodes, links)
	summary := summarizeAITopology(req.ServiceName, planner, plannerError, nodes, links)
	return model.AITopologyGraph{Nodes: nodes, Links: links, Risks: risks, Summary: summary}
}

func aiTopologyNodeID(endpoint model.TopologyEndpoint) string {
	return fmt.Sprintf("%s-%s-%d", classifyAITopologyLayer(endpoint), sanitizeID(endpoint.IP), endpoint.Port)
}

func isAgentLikeEndpoint(endpoint model.TopologyEndpoint) bool {
	name := strings.ToLower(strings.TrimSpace(endpoint.ServiceName))
	return strings.Contains(name, "catpaw") || strings.Contains(name, "main agent") || strings.Contains(name, "ai-agent") || strings.Contains(name, "agent")
}

func classifyAITopologyLayer(endpoint model.TopologyEndpoint) string {
	name := strings.ToLower(strings.TrimSpace(endpoint.ServiceName))
	port := endpoint.Port
	switch {
	case strings.Contains(name, "nginx") || strings.Contains(name, "haproxy") || strings.Contains(name, "traefik") || strings.Contains(name, "kong") || strings.Contains(name, "envoy") || port == 80 || port == 443 || port == 8443:
		return "gateway"
	case strings.Contains(name, "redis") || strings.Contains(name, "memcached") || port == 6379 || port == 11211:
		return "cache"
	case strings.Contains(name, "kafka") || strings.Contains(name, "rabbitmq") || strings.Contains(name, "rocketmq") || strings.Contains(name, "mq") || port == 9092 || port == 5672 || port == 9876:
		return "mq"
	case strings.Contains(name, "mysql") || strings.Contains(name, "postgres") || strings.Contains(name, "oracle") || strings.Contains(name, "mongo") || strings.Contains(name, "elasticsearch") || strings.Contains(name, "database") || port == 3306 || port == 5432 || port == 1521 || port == 9200:
		return "db"
	case strings.Contains(name, "etcd") || strings.Contains(name, "zookeeper") || strings.Contains(name, "consul") || strings.Contains(name, "zk") || port == 2379 || port == 2181 || port == 8300:
		return "infra"
	case strings.Contains(name, "prometheus") || strings.Contains(name, "categraf") || strings.Contains(name, "exporter") || strings.Contains(name, "grafana") || port == 9090 || port == 9100 || port == 9101:
		return "monitor"
	case strings.Contains(name, "jvm") || strings.Contains(name, "java") || strings.Contains(name, "python") || strings.Contains(name, "node") || strings.Contains(name, "service") || strings.Contains(name, "api") || strings.Contains(name, "app") || (port >= 8000 && port <= 9000):
		return "app"
	default:
		return "app"
	}
}

func normalizeAITopologyHealth(health model.AITopologyHealth) model.AITopologyHealth {
	if health.Score < 0 {
		health.Score = 0
	}
	if health.Score > 100 {
		health.Score = 100
	}
	if health.Status == "" || health.Status == "unknown" {
		if health.Score >= 85 {
			health.Status = "healthy"
		} else if health.Score >= 70 {
			health.Status = "warning"
		} else if health.Score > 0 {
			health.Status = "danger"
		} else {
			health.Status = "unknown"
		}
	}
	if health.Status == "critical" {
		health.Status = "danger"
	}
	return health
}

func aiServiceRole(layer, service string) string {
	switch layer {
	case "gateway":
		return "入口网关"
	case "app":
		return "业务服务"
	case "cache":
		return "缓存"
	case "mq":
		return "消息队列"
	case "db":
		if strings.Contains(strings.ToLower(service), "slave") || strings.Contains(strings.ToLower(service), "standby") {
			return "从库"
		}
		return "数据库"
	case "infra":
		return "注册/配置中心"
	case "monitor":
		return "监控采集"
	default:
		return "业务组件"
	}
}

func inferAITopologyLinks(nodes []model.AITopologyNode, explicit []model.AITopologyLink) []model.AITopologyLink {
	links := []model.AITopologyLink{}
	nodeIDs := map[string]bool{}
	byLayer := map[string][]model.AITopologyNode{}
	for _, node := range nodes {
		nodeIDs[node.ID] = true
		byLayer[node.Layer] = append(byLayer[node.Layer], node)
	}
	add := func(link model.AITopologyLink) {
		if link.Source == link.Target || !nodeIDs[link.Source] || !nodeIDs[link.Target] {
			return
		}
		if link.Type == "" {
			link.Type = "TCP"
		}
		if link.Label == "" {
			link.Label = link.Type
		}
		for _, existing := range links {
			if existing.Source == link.Source && existing.Target == link.Target && existing.Label == link.Label {
				return
			}
		}
		links = append(links, link)
	}
	for _, link := range explicit {
		add(link)
	}
	for _, gateway := range byLayer["gateway"] {
		for _, app := range byLayer["app"] {
			add(model.AITopologyLink{Source: gateway.ID, Target: app.ID, Type: "HTTP", Label: "负载均衡"})
		}
	}
	for _, app := range byLayer["app"] {
		for _, cache := range byLayer["cache"] {
			add(model.AITopologyLink{Source: app.ID, Target: cache.ID, Type: "Redis", Label: "缓存读写"})
		}
		for _, mq := range byLayer["mq"] {
			add(model.AITopologyLink{Source: app.ID, Target: mq.ID, Type: "MQ", Label: "消息生产/消费"})
		}
		for _, db := range byLayer["db"] {
			add(model.AITopologyLink{Source: app.ID, Target: db.ID, Type: "DB", Label: "数据读写"})
		}
	}
	for _, infra := range byLayer["infra"] {
		for _, app := range byLayer["app"] {
			add(model.AITopologyLink{Source: infra.ID, Target: app.ID, Type: "Discovery", Label: "服务注册/配置发现", Dashed: true})
		}
	}
	for i, source := range byLayer["db"] {
		for j, target := range byLayer["db"] {
			if i >= j || !sameDBFamily(source, target) {
				continue
			}
			add(model.AITopologyLink{Source: source.ID, Target: target.ID, Type: "Replication", Label: "主从同步", Dashed: true, Relation: "replication"})
		}
	}
	return links
}

func sameDBFamily(a, b model.AITopologyNode) bool {
	if len(a.Services) == 0 || len(b.Services) == 0 {
		return false
	}
	an := strings.ToLower(a.Services[0].Name)
	bn := strings.ToLower(b.Services[0].Name)
	families := []string{"oracle", "mysql", "postgres", "mongo", "elasticsearch"}
	for _, family := range families {
		if strings.Contains(an, family) && strings.Contains(bn, family) {
			return true
		}
	}
	return false
}

func detectAITopologyRisks(nodes []model.AITopologyNode, links []model.AITopologyLink) []model.AITopologyRisk {
	risks := []model.AITopologyRisk{}
	layerCounts := map[string]int{}
	nodeMap := map[string]model.AITopologyNode{}
	degree := map[string]int{}
	for _, node := range nodes {
		layerCounts[node.Layer]++
		nodeMap[node.ID] = node
		degree[node.ID] = 0
	}
	for _, link := range links {
		degree[link.Source]++
		degree[link.Target]++
		if isCrossLayerRisk(nodeMap[link.Source].Layer, nodeMap[link.Target].Layer) {
			risks = append(risks, model.AITopologyRisk{Type: "cross_layer_direct", Severity: "medium", Title: "跨层直连", Description: fmt.Sprintf("%s -> %s 跨层级直连，需确认是否绕过标准业务链路", link.Source, link.Target), Nodes: []string{link.Source, link.Target}, Suggestion: "核对调用链配置；如为真实链路，应补充限流、超时、鉴权和监控。"})
		}
	}
	for _, layer := range []string{"gateway", "cache", "mq", "db", "infra"} {
		if layerCounts[layer] == 1 {
			for _, node := range nodes {
				if node.Layer == layer {
					risks = append(risks, model.AITopologyRisk{Type: "single_point", Severity: "high", Title: "单点风险", Description: fmt.Sprintf("%s 层仅 1 个节点：%s", layer, node.ID), Nodes: []string{node.ID}, Suggestion: "评估主备/集群化改造，并先补齐健康检查、自动拉起和容量预警。"})
					break
				}
			}
		}
	}
	for _, node := range nodes {
		if degree[node.ID] == 0 {
			risks = append(risks, model.AITopologyRisk{Type: "island", Severity: "medium", Title: "孤岛节点", Description: fmt.Sprintf("%s 无入边也无出边，可能为配置遗漏或未纳入业务链路", node.ID), Nodes: []string{node.ID}, Suggestion: "补充真实依赖关系或从业务拓扑中移除无关端口。"})
		}
		if node.Health.Status == "danger" {
			risks = append(risks, model.AITopologyRisk{Type: "blast_radius", Severity: "high", Title: "故障扩散风险", Description: fmt.Sprintf("%s 处于危险状态，可能影响其上下游链路", node.ID), Nodes: []string{node.ID}, Suggestion: "优先核查该节点进程、端口连通、连接池、慢查询/慢请求和未恢复告警。"})
		}
		if node.Health.Status == "unknown" {
			risks = append(risks, model.AITopologyRisk{Type: "observability_gap", Severity: "medium", Title: "监控盲区", Description: fmt.Sprintf("%s 缺少健康状态或指标证据", node.ID), Nodes: []string{node.ID}, Suggestion: "补齐 Categraf/Prometheus 指标、Catpaw 探针状态和告警路由。"})
		}
	}
	return risks
}

func isCrossLayerRisk(source, target string) bool {
	if source == "infra" && target == "app" {
		return false
	}
	if source == "gateway" && target == "app" {
		return false
	}
	if source == "app" && (target == "cache" || target == "mq" || target == "db") {
		return false
	}
	if source == "db" && target == "db" {
		return false
	}
	order := map[string]int{"gateway": 0, "app": 1, "cache": 2, "mq": 2, "db": 3, "infra": 4, "monitor": 5}
	return absInt(order[source]-order[target]) > 1
}

func summarizeAITopology(serviceName, planner, plannerError string, nodes []model.AITopologyNode, links []model.AITopologyLink) model.AITopologySummary {
	layers := map[string]int{}
	health := map[string]int{}
	for _, node := range nodes {
		layers[node.Layer]++
		health[node.Health.Status]++
	}
	critical := []string{}
	for _, layer := range []string{"gateway", "app", "cache", "db"} {
		for _, node := range nodes {
			if node.Layer == layer {
				critical = append(critical, node.ID)
				break
			}
		}
	}
	return model.AITopologySummary{ServiceName: serviceName, Planner: planner, NodeCount: len(nodes), LinkCount: len(links), LayerCounts: layers, HealthDistribution: health, CriticalPath: critical, Error: plannerError}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
