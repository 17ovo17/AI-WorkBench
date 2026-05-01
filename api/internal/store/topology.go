package store

import (
	"encoding/json"
	"sort"
	"time"

	"ai-workbench-api/internal/model"
)

// SaveTopology replaces the entire topology graph in memory and MySQL.
func SaveTopology(g model.TopologyGraph) {
	now := time.Now()
	mu.Lock()
	topologyNodes = map[string]*model.TopologyNode{}
	topologyEdges = map[string]*model.TopologyEdge{}
	for i := range g.Nodes {
		if g.Nodes[i].CreatedAt.IsZero() {
			g.Nodes[i].CreatedAt = now
		}
		g.Nodes[i].UpdatedAt = now
		n := g.Nodes[i]
		topologyNodes[n.ID] = &n
	}
	for i := range g.Edges {
		if g.Edges[i].CreatedAt.IsZero() {
			g.Edges[i].CreatedAt = now
		}
		g.Edges[i].UpdatedAt = now
		e := g.Edges[i]
		topologyEdges[e.ID] = &e
	}
	mu.Unlock()
	if mysqlOK {
		tx, _ := db.Begin()
		if tx != nil {
			_, _ = tx.Exec(`DELETE FROM topology_edges`)
			_, _ = tx.Exec(`DELETE FROM topology_nodes`)
			for _, n := range g.Nodes {
				_, _ = tx.Exec(`REPLACE INTO topology_nodes (id,name,type,ip,service_name,port,status,x,y,meta,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, n.ID, n.Name, n.Type, n.IP, n.ServiceName, n.Port, n.Status, n.X, n.Y, n.Meta, n.CreatedAt, n.UpdatedAt)
			}
			for _, e := range g.Edges {
				_, _ = tx.Exec(`REPLACE INTO topology_edges (id,source_id,target_id,protocol,direction,label,status,latency_ms,error,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, e.ID, e.SourceID, e.TargetID, e.Protocol, e.Direction, e.Label, e.Status, e.LatencyMs, e.Error, e.CreatedAt, e.UpdatedAt)
			}
			_ = tx.Commit()
		}
	}
}

// GetTopology returns the full topology graph.
func GetTopology() model.TopologyGraph {
	if mysqlOK {
		g := model.TopologyGraph{Nodes: []model.TopologyNode{}, Edges: []model.TopologyEdge{}}
		if rows, err := db.Query(`SELECT id,name,type,ip,service_name,port,status,x,y,meta,created_at,updated_at FROM topology_nodes ORDER BY id`); err == nil {
			defer rows.Close()
			for rows.Next() {
				n := model.TopologyNode{}
				_ = rows.Scan(&n.ID, &n.Name, &n.Type, &n.IP, &n.ServiceName, &n.Port, &n.Status, &n.X, &n.Y, &n.Meta, &n.CreatedAt, &n.UpdatedAt)
				g.Nodes = append(g.Nodes, n)
			}
		}
		if rows, err := db.Query(`SELECT id,source_id,target_id,protocol,direction,label,COALESCE(status,''),COALESCE(latency_ms,0),COALESCE(error,''),created_at,updated_at FROM topology_edges ORDER BY id`); err == nil {
			defer rows.Close()
			for rows.Next() {
				e := model.TopologyEdge{}
				_ = rows.Scan(&e.ID, &e.SourceID, &e.TargetID, &e.Protocol, &e.Direction, &e.Label, &e.Status, &e.LatencyMs, &e.Error, &e.CreatedAt, &e.UpdatedAt)
				g.Edges = append(g.Edges, e)
			}
		}
		if len(g.Nodes) > 0 {
			return g
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	g := model.TopologyGraph{Nodes: []model.TopologyNode{}, Edges: []model.TopologyEdge{}}
	for _, n := range topologyNodes {
		g.Nodes = append(g.Nodes, *n)
	}
	for _, e := range topologyEdges {
		g.Edges = append(g.Edges, *e)
	}
	return g
}

func seedPlatformTopology() {
	if len(GetTopology().Nodes) > 0 {
		return
	}
	now := time.Now()
	SaveTopology(model.TopologyGraph{
		Nodes: []model.TopologyNode{
			{ID: "platform-web", Name: "AI WorkBench 前端", Type: "frontend", ServiceName: "web", Port: 3000, Status: "unknown", X: 120, Y: 170, CreatedAt: now, UpdatedAt: now},
			{ID: "platform-api", Name: "AI WorkBench 后端", Type: "backend", ServiceName: "api", Port: 8080, Status: "unknown", X: 370, Y: 170, CreatedAt: now, UpdatedAt: now},
			{ID: "platform-mysql", Name: "MySQL", Type: "database", ServiceName: "mysql", Port: 3306, Status: "unknown", X: 650, Y: 100, CreatedAt: now, UpdatedAt: now},
			{ID: "platform-redis", Name: "Redis", Type: "cache", ServiceName: "redis", Port: 6379, Status: "unknown", X: 650, Y: 240, CreatedAt: now, UpdatedAt: now},
			{ID: "platform-prom", Name: "Prometheus", Type: "monitor", ServiceName: "prometheus", Port: 9090, Status: "unknown", X: 370, Y: 320, CreatedAt: now, UpdatedAt: now},
		},
		Edges: []model.TopologyEdge{
			{ID: "edge-web-api", SourceID: "platform-web", TargetID: "platform-api", Protocol: "HTTP", Direction: "forward", Label: "API 调用", Status: "unknown", CreatedAt: now, UpdatedAt: now},
			{ID: "edge-api-mysql", SourceID: "platform-api", TargetID: "platform-mysql", Protocol: "MySQL", Direction: "forward", Label: "持久化", Status: "unknown", CreatedAt: now, UpdatedAt: now},
			{ID: "edge-api-redis", SourceID: "platform-api", TargetID: "platform-redis", Protocol: "Redis", Direction: "forward", Label: "缓存/在线状态", Status: "unknown", CreatedAt: now, UpdatedAt: now},
			{ID: "edge-api-prom", SourceID: "platform-api", TargetID: "platform-prom", Protocol: "HTTP", Direction: "forward", Label: "监控查询", Status: "unknown", CreatedAt: now, UpdatedAt: now},
		},
	})
}

// PLACEHOLDER_TOPOLOGY_BUSINESS

// ListTopologyBusinesses returns all topology businesses.
func ListTopologyBusinesses() []model.TopologyBusiness {
	if mysqlOK {
		rows, err := db.Query(`SELECT id,name,hosts,endpoints,COALESCE(attributes,'{}'),graph,created_at,updated_at FROM topology_businesses ORDER BY updated_at DESC`)
		if err == nil {
			defer rows.Close()
			out := []model.TopologyBusiness{}
			for rows.Next() {
				var b model.TopologyBusiness
				var hostsRaw, endpointsRaw, attributesRaw, graphRaw string
				_ = rows.Scan(&b.ID, &b.Name, &hostsRaw, &endpointsRaw, &attributesRaw, &graphRaw, &b.CreatedAt, &b.UpdatedAt)
				_ = json.Unmarshal([]byte(hostsRaw), &b.Hosts)
				_ = json.Unmarshal([]byte(endpointsRaw), &b.Endpoints)
				_ = json.Unmarshal([]byte(attributesRaw), &b.Attributes)
				_ = json.Unmarshal([]byte(graphRaw), &b.Graph)
				out = append(out, b)
			}
			return out
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	out := []model.TopologyBusiness{}
	for _, b := range topologyBusinesses {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}

// GetTopologyBusiness retrieves a single topology business by id.
func GetTopologyBusiness(id string) (model.TopologyBusiness, bool) {
	if mysqlOK {
		var b model.TopologyBusiness
		var hostsRaw, endpointsRaw, attributesRaw, graphRaw string
		row := db.QueryRow(`SELECT id,name,hosts,endpoints,COALESCE(attributes,'{}'),graph,created_at,updated_at FROM topology_businesses WHERE id=?`, id)
		if row.Scan(&b.ID, &b.Name, &hostsRaw, &endpointsRaw, &attributesRaw, &graphRaw, &b.CreatedAt, &b.UpdatedAt) == nil {
			_ = json.Unmarshal([]byte(hostsRaw), &b.Hosts)
			_ = json.Unmarshal([]byte(endpointsRaw), &b.Endpoints)
			_ = json.Unmarshal([]byte(attributesRaw), &b.Attributes)
			_ = json.Unmarshal([]byte(graphRaw), &b.Graph)
			return b, true
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	b, ok := topologyBusinesses[id]
	if !ok {
		return model.TopologyBusiness{}, false
	}
	return *b, true
}

// SaveTopologyBusiness creates or updates a topology business.
func SaveTopologyBusiness(b model.TopologyBusiness) model.TopologyBusiness {
	now := time.Now()
	if b.ID == "" {
		b.ID = NewID()
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = now
	}
	if b.Attributes == nil {
		b.Attributes = map[string]string{}
	}
	b.UpdatedAt = now
	mu.Lock()
	cp := b
	topologyBusinesses[b.ID] = &cp
	mu.Unlock()
	if mysqlOK {
		hostsRaw, _ := json.Marshal(b.Hosts)
		endpointsRaw, _ := json.Marshal(b.Endpoints)
		attributesRaw, _ := json.Marshal(b.Attributes)
		graphRaw, _ := json.Marshal(b.Graph)
		_, _ = db.Exec(`REPLACE INTO topology_businesses (id,name,hosts,endpoints,attributes,graph,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?)`, b.ID, b.Name, string(hostsRaw), string(endpointsRaw), string(attributesRaw), string(graphRaw), b.CreatedAt, b.UpdatedAt)
	} else {
		persistFallbackSnapshot()
	}
	return b
}

// DeleteTopologyBusiness removes a topology business by id.
func DeleteTopologyBusiness(id string) {
	mu.Lock()
	delete(topologyBusinesses, id)
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`DELETE FROM topology_businesses WHERE id=?`, id)
	} else {
		persistFallbackSnapshot()
	}
}
