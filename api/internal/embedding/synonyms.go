package embedding

import "strings"

var opsSynonymGroups = [][]string{
	{"cpu", "处理器", "中央处理器", "cpu使用率", "cpu usage", "load", "负载", "高负载"},
	{"memory", "mem", "内存", "内存使用率", "内存不足", "rss", "heap"},
	{"disk", "磁盘", "硬盘", "存储", "容量", "inode", "磁盘空间"},
	{"io", "i/o", "磁盘io", "读写", "iops", "吞吐", "io等待", "iowait"},
	{"network", "网络", "网卡", "带宽", "丢包", "重传", "rtt", "latency", "延迟"},
	{"tcp", "传输控制协议", "连接", "半连接", "time_wait", "close_wait", "syn"},
	{"oom", "内存溢出", "out of memory", "内存耗尽", "被杀", "oomkill", "oom killed"},
	{"gc", "垃圾回收", "full gc", "young gc", "停顿", "stw", "gc pause"},
	{"jvm", "java", "堆内存", "线程", "类加载", "jvm诊断"},
	{"prometheus", "promql", "指标", "监控", "数据源", "采集", "时序数据"},
	{"alert", "告警", "报警", "告警风暴", "风暴", "firing", "告警收敛"},
	{"runbook", "预案", "运维手册", "处置步骤", "应急流程", "操作手册"},
	{"workflow", "工作流", "流程", "编排", "路由", "workflow route", "工作流路由"},
	{"topology", "拓扑", "业务拓扑", "链路", "依赖", "调用关系", "服务关系"},
	{"database", "db", "数据库", "mysql", "慢查询", "锁等待", "死锁", "连接池"},
	{"redis", "缓存", "缓存命中", "key", "过期", "redis延迟", "内存碎片"},
	{"kubernetes", "k8s", "容器", "pod", "deployment", "namespace", "节点"},
	{"container", "docker", "镜像", "容器重启", "crashloopbackoff", "imagepullbackoff"},
	{"nginx", "网关", "反向代理", "upstream", "502", "504", "ingress"},
	{"http", "https", "接口", "api", "状态码", "响应时间", "错误率", "qps"},
	{"slo", "sla", "可用性", "错误预算", "burn rate", "服务等级"},
	{"capacity", "容量", "容量预测", "扩容", "水位", "资源规划"},
	{"security", "安全", "漏洞", "合规", "审计", "基线", "风险"},
	{"catpaw", "巡检", "探针", "agent", "主机巡检", "远程执行"},
	{"log", "日志", "错误日志", "堆栈", "异常", "trace", "链路追踪"},
	{"dns", "域名", "解析", "dns解析", "域名解析", "nxdomain"},
	{"ssl", "tls", "证书", "证书过期", "握手", "https证书"},
}

var shortQueryExpansions = map[string]string{
	"cpu":        "CPU 使用率异常排查 处理器 高负载",
	"处理器":        "CPU 使用率异常排查 处理器 高负载",
	"内存":         "内存使用率异常排查 OOM 内存溢出",
	"memory":     "Memory 内存使用率异常排查 OOM",
	"mem":        "Memory 内存使用率异常排查 OOM",
	"磁盘":         "磁盘空间和 IO 异常排查",
	"disk":       "Disk 磁盘空间和 IO 异常排查",
	"io":         "磁盘 IO 延迟 iowait 异常排查",
	"网络":         "网络延迟 丢包 TCP 重传异常排查",
	"network":    "Network 网络延迟 丢包 TCP 重传异常排查",
	"tcp":        "TCP 传输控制协议 连接异常 重传排查",
	"oom":        "OOM 内存溢出 out of memory 排查",
	"gc":         "GC 垃圾回收 Full GC 停顿排查",
	"jvm":        "JVM 堆内存 GC 线程异常排查",
	"redis":      "Redis 缓存延迟 内存 命中率异常排查",
	"mysql":      "MySQL 慢查询 锁等待 连接池异常排查",
	"k8s":        "Kubernetes Pod 容器异常排查",
	"pod":        "Pod 重启 OOM CrashLoopBackOff 排查",
	"502":        "HTTP 502 网关 upstream 异常排查",
	"504":        "HTTP 504 网关超时 upstream 异常排查",
	"证书":         "SSL TLS 证书过期 握手异常排查",
	"prometheus": "Prometheus PromQL 指标采集异常排查",
}

func domainSynonyms(text string) []string {
	text = strings.ToLower(text)
	out := []string{}
	for _, group := range opsSynonymGroups {
		if groupMatches(text, group) {
			out = append(out, group...)
		}
	}
	return out
}

func groupMatches(text string, group []string) bool {
	for _, term := range group {
		if strings.Contains(text, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func expandShortOpsQuery(query string) string {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return query
	}
	if expanded, ok := shortQueryExpansions[q]; ok {
		return query + " " + expanded
	}
	if len([]rune(q)) <= 8 && len(strings.Fields(q)) <= 2 {
		if synonyms := domainSynonyms(q); len(synonyms) > 0 {
			return query + " " + strings.Join(synonyms, " ") + " 异常排查 处置建议"
		}
	}
	return query
}
