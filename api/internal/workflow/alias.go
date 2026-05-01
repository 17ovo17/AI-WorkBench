package workflow

// WorkflowAlias 旧名称到新名称的映射及默认路由参数
type WorkflowAlias struct {
	NewName       string
	DefaultInputs map[string]string
}

var aliasMap = map[string]WorkflowAlias{
	// diagnosis 类
	"diagnosis":       {NewName: "smart_diagnosis", DefaultInputs: map[string]string{"trigger": "manual"}},
	"alert_diagnosis": {NewName: "smart_diagnosis", DefaultInputs: map[string]string{"trigger": "alert"}},

	// domain 类
	"container_diagnosis":  {NewName: "domain_diagnosis", DefaultInputs: map[string]string{"domain": "container"}},
	"jvm_diagnosis":        {NewName: "domain_diagnosis", DefaultInputs: map[string]string{"domain": "jvm"}},
	"slow_query_diagnosis": {NewName: "domain_diagnosis", DefaultInputs: map[string]string{"domain": "db"}},
	"db_lock_analysis":     {NewName: "domain_diagnosis", DefaultInputs: map[string]string{"domain": "db"}},
	"log_analysis":         {NewName: "domain_diagnosis", DefaultInputs: map[string]string{"domain": "log"}},

	// health 类
	"business_inspection":  {NewName: "health_inspection", DefaultInputs: map[string]string{"scope": "business"}},
	"dependency_health":    {NewName: "health_inspection", DefaultInputs: map[string]string{"scope": "dependency"}},
	"storage_health_check": {NewName: "health_inspection", DefaultInputs: map[string]string{"scope": "storage"}},
	"middleware_inspection": {NewName: "health_inspection", DefaultInputs: map[string]string{"scope": "middleware"}},

	// metrics 类
	"metrics_analysis":       {NewName: "metrics_insight", DefaultInputs: map[string]string{"analysis_type": "anomaly"}},
	"capacity_forecast":      {NewName: "metrics_insight", DefaultInputs: map[string]string{"analysis_type": "forecast"}},
	"slo_compliance":         {NewName: "metrics_insight", DefaultInputs: map[string]string{"analysis_type": "slo"}},
	"traffic_anomaly_detect": {NewName: "metrics_insight", DefaultInputs: map[string]string{"analysis_type": "traffic"}},

	// security 类
	"security_audit":      {NewName: "security_compliance", DefaultInputs: map[string]string{"audit_type": "security"}},
	"ssl_audit":           {NewName: "security_compliance", DefaultInputs: map[string]string{"audit_type": "ssl"}},
	"config_drift_detect": {NewName: "security_compliance", DefaultInputs: map[string]string{"audit_type": "config"}},

	// incident 类
	"incident_timeline":   {NewName: "incident_review", DefaultInputs: map[string]string{"mode": "timeline"}},
	"incident_postmortem": {NewName: "incident_review", DefaultInputs: map[string]string{"mode": "postmortem"}},

	// change 类
	"change_rollback": {NewName: "runbook_execute", DefaultInputs: map[string]string{"action": "rollback"}},
}

// ResolveAlias 解析别名，返回新名称和需要注入的默认参数。
// 若 name 不在别名表中，原样返回且 defaultInputs 为 nil。
func ResolveAlias(name string) (resolvedName string, defaultInputs map[string]string) {
	if alias, ok := aliasMap[name]; ok {
		return alias.NewName, alias.DefaultInputs
	}
	return name, nil
}
