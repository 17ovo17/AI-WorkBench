/**
 * 内置工作流参数模板
 * key = builtin 工作流名称，value = 参数定义数组
 * - key: 参数名
 * - label: 表单标签
 * - placeholder: 输入提示
 * - required: 是否必填
 * - default: 默认值
 */
export const BUILTIN_PARAMS = {
  smart_diagnosis: [
    { key: 'hostname', label: '主机名/IP', placeholder: '如：web-01 或 10.0.0.12', required: true },
    { key: 'time_range', label: '时间范围', placeholder: '1h', default: '1h' },
    { key: 'user_question', label: '补充问题', placeholder: '可选：让 AI 重点关注的问题' },
    { key: 'trigger', label: '触发方式', placeholder: 'manual / alert', default: 'manual' },
  ],
  domain_diagnosis: [
    { key: 'hostname', label: '主机名/IP', placeholder: '如：web-01 或 10.0.0.12', required: true },
    { key: 'domain', label: '诊断领域', placeholder: 'jvm / container / db / log', required: true },
    { key: 'time_range', label: '时间范围', placeholder: '1h', default: '1h' },
    { key: 'app_name', label: '应用名', placeholder: '可选' },
  ],
  health_inspection: [
    { key: 'hostname', label: '主机名/IP', placeholder: '如：web-01 或 10.0.0.12', required: true },
    { key: 'scope', label: '巡检范围', placeholder: 'full / business / middleware / storage / dependency', default: 'full' },
    { key: 'business_id', label: '业务 ID', placeholder: '可选' },
  ],
  metrics_insight: [
    { key: 'hostname', label: '主机名/IP', placeholder: '如：web-01 或 10.0.0.12', required: true },
    { key: 'analysis_type', label: '分析类型', placeholder: 'anomaly / forecast / slo / traffic', default: 'anomaly' },
    { key: 'time_range', label: '时间范围', placeholder: '6h', default: '6h' },
    { key: 'slo_target', label: 'SLO 目标', placeholder: '99.9', default: '99.9' },
  ],
  security_compliance: [
    { key: 'audit_type', label: '审计类型', placeholder: 'security / ssl / config', default: 'security' },
    { key: 'time_range', label: '时间范围', placeholder: '24h', default: '24h' },
    { key: 'min_risk', label: '最低风险等级', placeholder: 'low / medium / high', default: 'low' },
  ],
  incident_review: [
    { key: 'hostname', label: '主机名/IP', placeholder: '如：web-01 或 10.0.0.12', required: true },
    { key: 'mode', label: '回顾模式', placeholder: 'timeline / postmortem', default: 'timeline' },
    { key: 'time_range', label: '时间范围', placeholder: '6h', default: '6h' },
    { key: 'incident_summary', label: '故障摘要', placeholder: '可选：故障简要描述' },
  ],
  network_check: [
    { key: 'target_ips', label: '目标 IP', placeholder: '逗号分隔多个 IP', required: true },
    { key: 'business_id', label: '业务 ID', placeholder: '可选：关联业务拓扑' },
  ],
  runbook_execute: [
    { key: 'runbook_id', label: 'Runbook ID', placeholder: '要执行的 Runbook ID', required: true },
    { key: 'target_ip', label: '目标主机', placeholder: '执行目标 IP', required: true },
    { key: 'dry_run', label: '模拟执行', placeholder: 'true / false', default: 'false' },
  ],
  knowledge_enrich: [
    { key: 'diagnosis_id', label: '诊断记录 ID', placeholder: '要沉淀的诊断记录 ID', required: true },
  ],
}
