<template>
  <aside class="realtime-panel glass-card">
    <div class="panel-head">
      <div><span>LIVE CONTEXT</span><h3>实时指标面板</h3></div>
      <button type="button" @click="emit('refresh')"><el-icon><Refresh /></el-icon></button>
    </div>
    <div class="metric-tabs">
      <button v-for="tab in METRIC_TABS" :key="tab" :class="{ active: activeTab === tab }" type="button" @click="activeTab = tab">{{ tab }}</button>
    </div>
    <div class="metric-card" @click="emit('fill-question', metricQuestion)">
      <b>{{ activeTab }} 实时证据</b>
      <p v-if="metricUpdates.length">{{ latestMetricText }}</p>
      <p v-else-if="latestPromSteps.length">{{ latestPromSteps[0].output || latestPromSteps[0].result || '已有查询结果' }}</p>
      <p v-else>发送问诊或点击 PromQL 建议后自动订阅。</p>
      <div v-if="metricUpdates.length" class="sparkline">
        <i v-for="(point, index) in metricUpdates.slice(-16)" :key="index" :style="{ height: sparkHeight(point) }"></i>
      </div>
    </div>
    <div class="panel-section">
      <b>拓扑高亮</b>
      <div v-if="topologyNodes.length" class="node-list">
        <button v-for="node in topologyNodes" :key="node" type="button" class="node-pill" @click="emit('fill-question', `帮我诊断 ${node}`)">{{ node }}</button>
      </div>
      <p v-else>暂无拓扑高亮节点</p>
    </div>
    <div class="panel-section">
      <b>活跃告警</b>
      <button v-for="alert in groupedAlerts.slice(0, 5)" :key="alert.id" type="button" class="alert-item" @click="emit('fill-question', `帮我诊断 ${alert.target_ip || alert.title} 告警：${alert.title}`)">
        <strong>{{ alert.title }}<em v-if="alert.group_count > 1"> ×{{ alert.group_count }}</em></strong><span>{{ alert.target_ip }} · {{ alert.severity }} · {{ alert.status }}</span>
      </button>
      <p v-if="!alerts.length">暂无告警数据</p>
    </div>
    <div class="panel-section">
      <b>数据源状态</b>
      <div class="health-grid">
        <span v-for="item in dataSourceStatus" :key="item.name || item.source">
          {{ item.name || item.source || item.id }}
          <em :class="{ ok: item.alive || item.status === 'healthy' }">{{ item.alive ? 'healthy' : (item.status || 'unknown') }}</em>
        </span>
      </div>
    </div>
    <div v-if="actionResult" class="action-result"><b>操作结果</b><pre>{{ actionResult }}</pre></div>
  </aside>
</template>

<script setup>
import { computed, ref } from 'vue'
import { Refresh } from '@element-plus/icons-vue'

const props = defineProps({
  metricUpdates: { type: Array, default: () => [] },
  latestPromSteps: { type: Array, default: () => [] },
  topologyNodes: { type: Array, default: () => [] },
  topologyHosts: { type: Array, default: () => [] },
  alerts: { type: Array, default: () => [] },
  dataSourceStatus: { type: Array, default: () => [] },
  actionResult: { type: String, default: '' }
})

const emit = defineEmits(['refresh', 'fill-question'])

const METRIC_TABS = ['CPU', '内存', '磁盘', '网络']
const activeTab = ref('CPU')

const formatTime = value => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
const sparkHeight = point => `${Math.max(8, Math.min(42, Number(point.value || 0) / 2 || 12))}px`

const latestMetricText = computed(() => {
  const point = props.metricUpdates[props.metricUpdates.length - 1]
  return point ? `${point.metric || point.query} = ${point.value ?? point.raw ?? '--'} @ ${formatTime(point.timestamp)}` : ''
})

const metricQuestion = computed(() =>
  props.topologyHosts[0]
    ? `帮我诊断 ${props.topologyHosts[0]} 的 ${activeTab.value}`
    : `帮我诊断 ${activeTab.value} 指标异常`
)

const normalizedAlertTitle = alert => String(alert.title || alert.labels?.alertname || '')
  .replace(/-\d+$/g, '')
  .replace(/\b\d{1,2}:\d{2}:\d{2}\b/g, '')
  .trim()
  .toLowerCase()

const groupedAlerts = computed(() => {
  const groups = new Map()
  for (const alert of props.alerts || []) {
    const key = [alert.target_ip || alert.labels?.instance || '', alert.severity || '', alert.status || '', normalizedAlertTitle(alert)].join('|')
    const existing = groups.get(key)
    if (!existing) {
      groups.set(key, { ...alert, group_count: Number(alert.count || 1), grouped_titles: [alert.title] })
    } else {
      existing.group_count += Number(alert.count || 1)
      existing.grouped_titles.push(alert.title)
      existing.last_seen = alert.last_seen || existing.last_seen
    }
  }
  return [...groups.values()]
})
</script>

<style scoped>
.realtime-panel { padding: 14px; overflow: auto; }
.panel-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; }
.panel-head span { color: #247cff; font-size: 10px; font-weight: 900; letter-spacing: .08em; }
.panel-head h3 { margin: 4px 0 0; color: #233653; }
.panel-head button { border: 0; min-width: 32px; height: 32px; border-radius: 12px; background: #257cff; color: white; cursor: pointer; padding: 0 10px; }
.metric-tabs { display: flex; gap: 6px; flex-wrap: wrap; margin: 12px 0; }
.metric-tabs button { border: 1px solid rgba(101, 125, 160, .18); border-radius: 999px; background: rgba(255, 255, 255, .55); color: #51637d; padding: 6px 10px; font-size: 12px; cursor: pointer; }
.metric-tabs button.active { color: white; background: #247cff; border-color: #247cff; }
.metric-card, .panel-section, .action-result { margin-top: 12px; border-radius: 18px; background: rgba(255, 255, 255, .52); padding: 12px; }
.metric-card { cursor: pointer; }
.metric-card b, .panel-section b, .action-result b { display: block; margin-bottom: 7px; }
.metric-card p, .panel-section p { color: #64748b; font-size: 12px; line-height: 1.55; }
.sparkline { height: 46px; display: flex; align-items: end; gap: 3px; margin-top: 8px; }
.sparkline i { width: 8px; border-radius: 5px 5px 0 0; background: #247cff; opacity: .78; }
.node-list { display: flex; flex-wrap: wrap; gap: 6px; }
.node-pill { border: 0; border-radius: 999px; background: #eff6ff; color: #1d4ed8; padding: 5px 8px; font-size: 12px; cursor: pointer; }
.alert-item { width: 100%; border: 0; border-radius: 12px; background: rgba(254, 242, 242, .72); color: #991b1b; padding: 8px; margin-bottom: 7px; text-align: left; cursor: pointer; }
.alert-item strong, .alert-item span { display: block; }
.alert-item em { margin-left: 4px; color: #ef4444; font-style: normal; }
.alert-item span { font-size: 11px; color: #b45309; margin-top: 3px; }
.health-grid { display: grid; gap: 6px; }
.health-grid span { display: flex; justify-content: space-between; border-radius: 12px; background: rgba(241, 245, 249, .75); padding: 7px 9px; font-size: 12px; }
.health-grid em { color: #247cff; font-style: normal; }
.action-result pre { max-height: 190px; overflow: auto; white-space: pre-wrap; font-size: 11px; color: #475569; }
</style>
