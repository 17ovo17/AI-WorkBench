<template>
  <div class="diag-panel">
    <div class="input-card">
      <el-form :model="form" label-width="100px" size="default">
        <div class="form-grid">
          <el-form-item label="诊断模式">
            <el-radio-group v-model="form.mode" @change="onModeChange">
              <el-radio value="single">单机诊断</el-radio>
              <el-radio value="business">业务巡检</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item v-if="form.mode === 'single'" label="主机名/IP">
            <el-input v-model="form.hostname" placeholder="如：web-01 或 10.0.0.12" clearable />
          </el-form-item>
          <el-form-item v-if="form.mode === 'business'" label="选择业务">
            <el-select v-model="form.business_id" placeholder="选择业务拓扑" style="width:100%" @change="onBusinessChange">
              <el-option v-for="b in businesses" :key="b.id" :label="b.name" :value="b.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="时间范围">
            <el-select v-model="form.time_range" style="width:100%">
              <el-option v-for="t in TIME_RANGES" :key="t.value" :label="t.label" :value="t.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="响应模式">
            <el-radio-group v-model="form.response_mode">
              <el-radio value="streaming">流式</el-radio>
              <el-radio value="blocking">阻塞</el-radio>
            </el-radio-group>
          </el-form-item>
        </div>
        <el-form-item label="补充问题">
          <el-input v-model="form.user_question" type="textarea" :rows="2" placeholder="可选：让 AI 重点关注的问题" />
        </el-form-item>
        <el-form-item label-width="0">
          <el-button type="primary" :loading="running" :icon="Promotion" @click="startDiagnosis">发起诊断</el-button>
          <el-button :disabled="running" @click="resetAll">重置</el-button>
          <span v-if="lastDuration" class="run-meta">上次耗时 {{ lastDuration }}s</span>
        </el-form-item>
      </el-form>
    </div>

    <div class="workflow-card">
      <div class="workflow-head">
        <span class="section-title">工作流执行</span>
        <span class="run-status">{{ overallText }}</span>
      </div>
      <div class="nodes-row">
        <template v-for="(n, idx) in VISUAL_NODES" :key="n.key || idx">
          <template v-if="n.parallel">
            <div class="parallel-group">
              <div v-for="pn in n.items" :key="pn.key" class="node-card" :class="`status-${nodeState[pn.key]?.status || 'pending'}`">
                <div class="node-icon">
                  <el-icon v-if="nodeState[pn.key]?.status === 'running'" class="spin"><Loading /></el-icon>
                  <el-icon v-else-if="nodeState[pn.key]?.status === 'done'"><CircleCheckFilled /></el-icon>
                  <el-icon v-else-if="nodeState[pn.key]?.status === 'error'"><CircleCloseFilled /></el-icon>
                  <el-icon v-else><Clock /></el-icon>
                </div>
                <div class="node-name">{{ pn.label }}</div>
                <div class="node-key">{{ pn.key }}</div>
              </div>
            </div>
          </template>
          <template v-else>
            <div class="node-card" :class="`status-${nodeState[n.key]?.status || 'pending'}`">
              <div class="node-icon">
                <el-icon v-if="nodeState[n.key]?.status === 'running'" class="spin"><Loading /></el-icon>
                <el-icon v-else-if="nodeState[n.key]?.status === 'done'"><CircleCheckFilled /></el-icon>
                <el-icon v-else-if="nodeState[n.key]?.status === 'error'"><CircleCloseFilled /></el-icon>
                <el-icon v-else><Clock /></el-icon>
              </div>
              <div class="node-name">{{ n.label }}</div>
              <div class="node-key">{{ n.key }}</div>
            </div>
          </template>
          <div v-if="idx < VISUAL_NODES.length - 1" class="node-arrow">&rarr;</div>
        </template>
      </div>
    </div>
    <el-alert v-if="fallbackHint" type="warning" :closable="false" show-icon style="border-radius:18px;margin-top:12px">
      <template #title>
        工作流引擎不可用，已降级。请使用
        <router-link to="/workbench" style="color:#247cff;font-weight:700;text-decoration:underline">智能对话</router-link>
        进行诊断。
      </template>
    </el-alert>

    <DiagnosisReport v-if="report" :report="report" @archive="archiveReport" @feedback="sendFeedback" />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'
import { Promotion, Loading, CircleCheckFilled, CircleCloseFilled, Clock } from '@element-plus/icons-vue'
import DiagnosisReport from '../DiagnosisReport.vue'

const TIME_RANGES = [
  { label: '最近 15 分钟', value: '15m' }, { label: '最近 30 分钟', value: '30m' },
  { label: '最近 1 小时', value: '1h' }, { label: '最近 3 小时', value: '3h' },
  { label: '最近 6 小时', value: '6h' }, { label: '最近 12 小时', value: '12h' },
  { label: '最近 24 小时', value: '24h' }
]
const NODE_DEFS = [
  { key: 'start', label: '接收请求' },
  { key: 'knowledge_retrieval', label: '检索案例' },
  { key: 'http_metrics', label: '实时指标' },
  { key: 'http_alerts', label: '告警记录' },
  { key: 'code_correlate', label: '关联分析' },
  { key: 'llm_diagnosis', label: 'LLM 诊断' },
  { key: 'http_runbook_match', label: '匹配 Runbook' },
  { key: 'end', label: '输出结果' }
]

// 可视化布局：并行节点合并为一组
const VISUAL_NODES = [
  { key: 'start', label: '接收请求' },
  { key: 'knowledge_retrieval', label: '检索案例' },
  { parallel: true, items: [
    { key: 'http_metrics', label: '实时指标' },
    { key: 'http_alerts', label: '告警记录' }
  ]},
  { key: 'code_correlate', label: '关联分析' },
  { key: 'llm_diagnosis', label: 'LLM 诊断' },
  { key: 'http_runbook_match', label: '匹配 Runbook' },
  { key: 'end', label: '输出结果' }
]

const form = reactive({ mode: 'single', hostname: '', business_id: '', time_range: '1h', user_question: '', response_mode: 'streaming' })
const businesses = ref([])
const nodeState = reactive(initNodes())
const running = ref(false)
const report = ref(null)
const fallbackHint = ref(false)
const lastDuration = ref(0)
const startTs = ref(0)

async function loadBusinesses() {
  try {
    const { data } = await axios.get('/api/v1/topology/businesses')
    businesses.value = Array.isArray(data) ? data : data.items || []
  } catch { businesses.value = [] }
}

function onModeChange() {
  form.hostname = ''
  form.business_id = ''
}

function onBusinessChange(id) {
  const biz = businesses.value.find(b => b.id === id)
  if (biz && biz.hosts?.length) {
    form.hostname = biz.hosts[0]
  }
}

onMounted(loadBusinesses)

function initNodes() {
  const obj = {}
  NODE_DEFS.forEach(n => { obj[n.key] = { status: 'pending' } })
  return obj
}

const overallText = computed(() => {
  if (running.value) return '运行中...'
  const errors = NODE_DEFS.filter(n => nodeState[n.key]?.status === 'error').length
  const dones = NODE_DEFS.filter(n => nodeState[n.key]?.status === 'done').length
  if (errors) return `失败（${errors} 个节点报错）`
  if (dones === NODE_DEFS.length) return '已完成'
  return '空闲'
})

const resetAll = () => {
  Object.assign(nodeState, initNodes())
  report.value = null; fallbackHint.value = false; lastDuration.value = 0
}
const setNode = (key, status, extra = {}) => {
  if (nodeState[key]) nodeState[key] = { ...nodeState[key], ...extra, status }
}
const startDiagnosis = async () => {
  if (form.mode === 'single' && !form.hostname.trim()) { ElMessage.warning('请输入主机名/IP'); return }
  if (form.mode === 'business' && !form.business_id) { ElMessage.warning('请选择业务'); return }
  if (running.value) return
  resetAll(); running.value = true; startTs.value = Date.now(); setNode('start', 'running')

  if (form.mode === 'business') {
    form.user_question = form.user_question || '业务巡检'
  }

  try {
    if (form.response_mode === 'streaming') await runStreaming()
    else await runBlocking()
  } catch (e) {
    ElMessage.error(e.message || '诊断失败')
  } finally {
    running.value = false
    lastDuration.value = ((Date.now() - startTs.value) / 1000).toFixed(1)
  }
}

const runBlocking = async () => {
  try {
    const { data } = await axios.post('/api/v1/diagnosis/start', { ...form, response_mode: 'blocking' })
    NODE_DEFS.forEach(n => setNode(n.key, 'done'))
    report.value = normalizeReport(data)
  } catch (e) { handleError(e.response?.status, e.response?.data) }
}

const runStreaming = async () => {
  const token = localStorage.getItem('aiw-token') || ''
  const resp = await fetch('/api/v1/diagnosis/start', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify({ ...form, response_mode: 'streaming' })
  })
  if (resp.status === 503) { handleError(503, await safeJson(resp)); return }
  if (!resp.ok) { handleError(resp.status, await safeJson(resp)); return }
  if (!resp.body) { await runBlocking(); return }
  const reader = resp.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n'); buffer = lines.pop() || ''
    for (const line of lines) {
      if (!line.startsWith('data: ')) continue
      try { handleSSE(JSON.parse(line.slice(6))) } catch { /* skip */ }
    }
  }
  NODE_DEFS.forEach(n => { if (nodeState[n.key].status === 'running') setNode(n.key, 'done') })
}

const handleSSE = (ev) => {
  const event = ev?.event || '', node = ev?.node_id
  if (event === 'workflow_started') setNode('start', 'done')
  else if (event === 'node_started' && node) setNode(node, 'running')
  else if (event === 'node_finished' && node) setNode(node, 'done')
  else if (event === 'node_error' && node) {
    setNode(node, 'error')
    ElMessage.error(ev?.data?.error || `节点 ${ev?.node_title || node} 执行失败`)
  } else if (event === 'workflow_finished') {
    const out = ev?.data?.outputs
    if (out) report.value = normalizeReport(out)
    NODE_DEFS.forEach(n => { if (nodeState[n.key].status === 'pending') setNode(n.key, 'done') })
  } else if (event === 'workflow_failed') {
    ElMessage.error(ev?.data?.error || '工作流执行失败')
    NODE_DEFS.forEach(n => { if (nodeState[n.key].status === 'running') setNode(n.key, 'error') })
  }
}

const safeJson = async (resp) => { try { return await resp.json() } catch { return {} } }

const handleError = (status, data) => {
  if (status === 503 && data?.fallback) {
    fallbackHint.value = true
    NODE_DEFS.forEach(n => { if (nodeState[n.key].status !== 'done') setNode(n.key, 'error') })
    return
  }
  ElMessage.error(data?.error || `诊断失败 (${status || 'network'})`)
  NODE_DEFS.forEach(n => { if (nodeState[n.key].status === 'running') setNode(n.key, 'error') })
}

const normalizeReport = (raw) => raw?.report || raw || null

const sendFeedback = async (kind) => {
  try {
    await axios.post('/api/v1/diagnosis/feedback', { report_id: report.value?.id || '', rating: kind })
    ElMessage.success('反馈已提交')
  } catch (e) { ElMessage.warning(e.response?.data?.error || '反馈接口暂不可用') }
}

const archiveReport = async () => {
  if (!report.value) return
  try {
    await axios.post('/api/v1/knowledge/cases', {
      root_cause_category: report.value.root_cause_category || 'unknown',
      root_cause_description: report.value.root_cause_description || '',
      keywords: (report.value.keywords || []).join?.(',') || report.value.keywords || '',
      treatment_steps: Array.isArray(report.value.treatment_steps)
        ? report.value.treatment_steps.join('\n') : (report.value.treatment_steps || ''),
      metric_snapshot: report.value.metric_snapshot || {}
    })
    ElMessage.success('已归档到知识库')
  } catch (e) { ElMessage.error(e.response?.data?.error || '归档失败') }
}
</script>

<style scoped>
.diag-panel { display: flex; flex-direction: column; gap: 16px; }
.input-card { padding: 18px 22px; background: rgba(255,255,255,.36); border-radius: 18px; border: 1px solid rgba(255,255,255,.6); }
.form-grid { display: grid; grid-template-columns: repeat(3, minmax(220px, 1fr)); gap: 0 24px; }
.run-meta { color: var(--muted); font-size: 12px; margin-left: 12px; }
.workflow-card { padding: 18px 22px; background: rgba(255,255,255,.36); border-radius: 18px; border: 1px solid rgba(255,255,255,.6); }
.workflow-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 14px; }
.section-title { font-size: 14px; color: #243553; font-weight: 800; }
.run-status { color: var(--muted); font-size: 12px; }
.nodes-row { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.node-card { flex: 1 1 110px; min-width: 110px; padding: 14px 10px; border-radius: 16px; border: 1px solid rgba(255,255,255,.7); background: rgba(255,255,255,.42); display: flex; flex-direction: column; align-items: center; gap: 6px; }
.node-card .node-icon { font-size: 22px; color: #98a3b8; }
.node-name { font-size: 13px; font-weight: 700; color: #243553; }
.node-key { font-size: 11px; color: var(--muted); }
.node-arrow { color: #b6c3d8; font-size: 18px; flex: 0 0 auto; }
.parallel-group { display: flex; flex-direction: column; gap: 6px; }
.parallel-group .node-card { flex: none; min-width: 110px; }
.status-running { border-color: rgba(47,124,255,.55); box-shadow: 0 0 0 4px rgba(47,124,255,.12); }
.status-running .node-icon { color: #247cff; }
.status-running .spin { animation: spin 1.1s linear infinite; }
.status-done { border-color: rgba(54,208,138,.55); }
.status-done .node-icon { color: #36b07a; }
.status-error { border-color: rgba(255,91,107,.55); background: rgba(255,236,238,.6); }
.status-error .node-icon { color: #ff5b6b; }
@keyframes spin { from { transform: rotate(0); } to { transform: rotate(360deg); } }
</style>
