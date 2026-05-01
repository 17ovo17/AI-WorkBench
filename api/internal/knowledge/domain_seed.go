package knowledge

import (
	"strings"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"
)

type domainSeedDoc struct {
	ID      string
	Title   string
	Content string
	Tags    string
}

var domainSeedDocs = []domainSeedDoc{
	{"domain-prometheus-metrics", "Prometheus 指标缺失排查", "Prometheus 指标缺失时，先检查 scrape target、up、label、relabel、Categraf/exporter 状态和 PromQL 查询。处理步骤：确认数据源可用，检查目标标签，恢复采集任务，补齐监控指标和告警。", "Prometheus,PromQL,指标缺失,监控,采集,数据源"},
	{"domain-business-topology", "业务拓扑与服务依赖排查", "业务拓扑用于描述入口层、应用层、中间件层、数据库层和服务依赖链路。排查业务拓扑时需要核对 topology nodes、links、入口 nginx、应用 api/worker、Redis/MQ、MySQL/Oracle 数据库。", "业务拓扑,拓扑,服务依赖,入口层,应用层,数据库层,topology"},
	{"domain-workflow-route", "工作流路由命中规则", "工作流路由根据用户问题识别诊断、巡检、指标洞察、网络检查、知识增强、复盘等 workflow。路由异常时检查 route-preview、workflow_name、confidence、关键词和工作流 DSL。", "工作流路由,工作流,路由,workflow,route,DSL"},
	{"domain-network-latency", "网络延迟和丢包排查", "网络延迟排查关注 RTT、P99、丢包、TCP 重传、带宽饱和、网卡错误、连接数和上下游链路。建议先看 Prometheus 网络指标，再结合拓扑定位影响范围。", "网络延迟,网络,延迟,丢包,重传,RTT,network"},
	{"domain-alert-storm", "告警风暴收敛处置", "告警风暴需要先按业务拓扑、服务、实例和指纹聚合，识别根因告警，抑制重复告警，保留 P0/P1 通知，并在恢复后复盘告警规则。", "告警风暴,告警,报警,alert,聚合,收敛"},
	{"domain-catpaw-inspection", "Catpaw 巡检和主机探针", "Catpaw 巡检用于采集主机进程、端口、服务状态和系统快照。探针巡检异常时检查 agent 心跳、报告上报、端口连通、进程状态，并与 Prometheus 指标交叉验证。", "Catpaw,巡检,探针,主机巡检,agent,心跳"},
}

func EnsureDomainSeedDocuments() {
	for _, seed := range domainSeedDocs {
		if hasDomainSeed(seed.ID) {
			continue
		}
		now := time.Now()
		doc := &model.KnowledgeDocument{
			ID:        "kb-" + seed.ID,
			Title:     seed.Title,
			Content:   seed.Content,
			DocType:   "runbook",
			FileType:  "md",
			Category:  "domain-index",
			Tags:      seed.Tags,
			SourceID:  seed.ID,
			CreatedAt: now,
			UpdatedAt: now,
		}
		store.SaveDocument(doc)
		indexToSearchEngine(doc)
	}
}

func hasDomainSeed(sourceID string) bool {
	if store.FindDocumentBySourceID(sourceID) != nil {
		return true
	}
	for _, doc := range store.ListAllDocuments() {
		if strings.EqualFold(doc.SourceID, sourceID) || strings.EqualFold(doc.ID, "kb-"+sourceID) {
			return true
		}
	}
	return false
}
