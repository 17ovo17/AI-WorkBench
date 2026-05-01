<template>
  <div class="aiops-page">
    <aside class="session-rail glass-card">
      <div class="rail-head">
        <div><span>AI WorkBench</span><h2>智能问诊</h2></div>
        <button type="button" aria-label="新建会话" @click="createSession"><el-icon><Plus /></el-icon></button>
      </div>
      <div class="mode-pills">
        <button v-for="mode in modes" :key="mode.key" :class="{ active: sessionDraft.mode === mode.key }" type="button" @click="sessionDraft.mode = mode.key">{{ mode.label }}</button>
      </div>
      <div class="ws-state" :class="wsStatus">WS：{{ wsStatusLabel }}</div>
      <div class="session-list">
        <div v-for="session in sessions" :key="session.id" class="session-item" :class="{ active: activeSessionId === session.id }" @click="selectSession(session)">
          <strong>{{ session.title }}</strong>
          <span>{{ modeLabel(session.mode) }} · {{ formatTime(session.updated_at || session.updatedAt || session.created_at || session.createdAt) }}</span>
          <button class="session-rename" type="button" aria-label="重命名会话" title="重命名会话" @click.stop="renameSession(session)"><el-icon><Edit /></el-icon></button>
          <button class="session-delete" type="button" aria-label="删除会话" @click.stop="deleteSession(session)" title="删除会话"><el-icon><Delete /></el-icon></button>
        </div>
      </div>
      <div class="source-card">
        <b>数据来源</b>
        <span>实时监控指标</span>
        <span>探针巡检报告</span>
        <span>用户上报信息</span>
      </div>
    </aside>

    <main class="chat-workspace glass-card">
      <header class="chat-head">
        <div><span>智能诊断引擎</span><h1>{{ activeTitle }}</h1></div>
        <div style="display:flex;align-items:center;gap:12px">
          <el-select v-model="currentModel" placeholder="选择 AI 模型" size="small" style="width:160px" @change="saveModel">
            <el-option v-for="model in modelOptions" :key="model" :label="model" :value="model" />
          </el-select>
          <el-button size="small" plain @click="router.push({ path: '/knowledge', query: { tab: 'diagnosis' } })">诊断归档</el-button>
          <div class="chat-state"><i :class="{ loading }"></i>{{ loading ? '实时推理中...' : '等待问诊' }}</div>
        </div>
      </header>

      <section ref="msgBox" class="messages">
        <div v-if="!messages.length" class="empty-state">
          <div class="empty-orb"></div>
          <h3>问诊、指标、拓扑联动</h3>
          <p>试试：10.10.1.21 CPU 飙高了 / AI WorkBench 巡检 / 10.10.1.21 和 10.10.1.22 CPU 都很高。</p>
        </div>
        <article v-for="message in messages" :key="message.messageId || message.id" class="message" :class="message.role">
          <div class="bubble">
            <SummaryCard v-if="message.summaryCard?.problem" :card="message.summaryCard" />
            <HandoffCard v-if="activeAudience === 'oncall' && message.handoffNote?.summary" :note="message.handoffNote" @copy="copyHandoff(message.handoffNote)" />
            <div v-if="shouldShowContent(message)" class="md" v-html="renderMd(message.content || '')"></div>
            <ReasoningBlock v-if="shouldShowReasoning(message)" :steps="message.reasoningChain" />
            <div v-if="shouldShowDataSources(message)" class="source-usage">
              <span v-for="source in message.dataSources" :key="`${message.messageId}-${source.source}-${source.queries}`">{{ source.source }} · {{ source.queries }} 次 · {{ source.latency_ms || source.latencyMs || 0 }}ms</span>
            </div>
            <div v-if="message.suggestedActions?.length" class="actions-row">
              <button v-for="action in message.suggestedActions" :key="action.id || action.label" type="button" @click="runAction(action)">
                <el-icon><Operation /></el-icon>{{ action.label }}
              </button>
            </div>
            <div v-if="message.role === 'assistant'" class="diagnosis-actions">
              <button type="button" :disabled="feedbackSent[message.messageId || message.id]" @click="sendMessageFeedback(message, 'accurate')">准确</button>
              <button type="button" :disabled="feedbackSent[message.messageId || message.id]" @click="sendMessageFeedback(message, 'partial')">部分准确</button>
              <button type="button" :disabled="feedbackSent[message.messageId || message.id]" @click="sendMessageFeedback(message, 'inaccurate')">不准确</button>
              <button type="button" @click="archiveMessageCase(message)">归档为案例</button>
            </div>
          </div>
        </article>
      </section>

      <footer class="composer">
        <div class="quick-row">
          <button v-for="prompt in quickPrompts" :key="prompt" type="button" @click="inputText = prompt">{{ prompt }}</button>
        </div>
        <el-input v-model="inputText" type="textarea" :rows="3" resize="none" placeholder="输入 AIOps 问诊问题，Enter 发送，Shift+Enter 换行" @keydown.enter.exact.prevent="sendMessage" />
        <div class="composer-foot">
          <span>WS 实时推理；断开时自动 HTTP fallback</span>
          <div class="composer-buttons">
            <el-button :disabled="!topologyGraph.nodes.length" @click="topologyOpen = !topologyOpen">{{ topologyOpen ? '收起拓扑' : '展开拓扑' }}</el-button>
            <el-button type="primary" :disabled="loading || !inputText.trim()" @click="sendMessage"><el-icon><Top /></el-icon>发送</el-button>
          </div>
        </div>
      </footer>
    </main>

    <el-dialog v-model="topologyOpen" title="拓扑联动面板" width="85%" top="5vh" destroy-on-close>
      <BusinessTopologyCanvas :graph="topologyGraph" :hosts="topologyHosts" title="AIOps 联动拓扑" @select-node="handleTopologyNode" @host-missing="fillQuestion" style="min-height:520px" />
    </el-dialog>

    <RealtimePanel
      :metric-updates="metricUpdates"
      :latest-prom-steps="latestPromSteps"
      :topology-nodes="topologyNodes"
      :topology-hosts="topologyHosts"
      :alerts="alerts"
      :data-source-status="dataSourceStatus"
      :action-result="actionResult"
      @refresh="refreshSidePanel"
      @fill-question="fillQuestion"
    />
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { marked } from 'marked'
import hljs from 'highlight.js'
import { sanitizeHtml } from '../utils/sanitizeHtml'
import BusinessTopologyCanvas from '../components/BusinessTopologyCanvas.vue'
import SummaryCard from '../components/workbench/SummaryCard.vue'
import HandoffCard from '../components/workbench/HandoffCard.vue'
import ReasoningBlock from '../components/workbench/ReasoningBlock.vue'
import RealtimePanel from '../components/workbench/RealtimePanel.vue'
import { useAiopsWebSocket } from '../composables/useAiopsWebSocket'

const router = useRouter()
const route = useRoute()
const sessions = ref([])
const messages = ref([])
const models = ref([])
const currentModel = ref(localStorage.getItem('selectedModel') || '')
const activeSessionId = ref('')
const loading = ref(false)
const inputText = ref('')
const msgBox = ref(null)
const alerts = ref([])
const dataSourceStatus = ref([])
const actionResult = ref('')
const sessionDraft = ref({ mode: 'diagnostic' })
const pendingAssistantId = ref('')
const metricUpdates = ref([])
const topologyOpen = ref(false)
const topologyGraph = ref({ nodes: [], links: [], risks: [], summary: {} })
const topologyHighlight = ref({ highlightNodes: [], nodes: [] })
const activeAudience = ref(localStorage.getItem('aiops-audience') || 'user')
const routeDiagnosisStarted = ref(false)
const feedbackSent = ref({})
const modes = [{ key: 'diagnostic', label: '诊断' }, { key: 'inspection', label: '巡检' }, { key: 'report', label: '上报' }, { key: 'topology', label: '拓扑' }]
const quickPrompts = ['10.10.1.21 CPU 飙高了', 'AI WorkBench 巡检', '生成 AI WorkBench 拓扑', '10.10.1.21 和 10.10.1.22 CPU 都很高']
marked.setOptions({ gfm: true, breaks: true, highlight(code, lang) { return hljs.highlight(code, { language: hljs.getLanguage(lang) ? lang : 'plaintext' }).value } })

const scrollBottom = () => nextTick(() => { if (msgBox.value) msgBox.value.scrollTop = msgBox.value.scrollHeight })
const normalizeTopologyGraph = graph => ({ nodes: graph?.nodes || [], links: graph?.links || [], risks: graph?.risks || [], summary: graph?.summary || {} })
const subscribeSuggestedMetrics = actions => {
  const metrics = actions.filter(item => item.type === 'promql').map(item => item.query || item.params?.query).filter(Boolean).slice(0, 3)
  if (metrics.length) sendWS({ type: 'subscribe_metrics', metrics, interval: 10, timestamp: new Date().toISOString() })
}

const { wsStatus, connectWS, closeWS, sendWS } = useAiopsWebSocket({
  messages, loading, topologyGraph, topologyHighlight, metricUpdates,
  actionResult, pendingAssistantId, scrollBottom, normalizeTopologyGraph, subscribeSuggestedMetrics,
})
const activeTitle = computed(() => sessions.value.find(s => s.id === activeSessionId.value)?.title || 'AIOps 智能问诊')
const modelOptions = computed(() => models.value.map(item => typeof item === 'string' ? item : (item.id || item.name || item.model)).filter(Boolean))
const latestAssistant = computed(() => [...messages.value].reverse().find(item => item.role === 'assistant'))
const latestPromSteps = computed(() => (latestAssistant.value?.reasoningChain || []).filter(step => step.action === 'prometheus_query'))
const topologyNodes = computed(() => latestAssistant.value?.topology?.highlightNodes || latestAssistant.value?.topology?.nodes || topologyHighlight.value.highlightNodes || topologyHighlight.value.nodes || [])
const topologyHosts = computed(() => [...new Set((topologyGraph.value.nodes || []).map(node => node.ip).filter(Boolean))])
const wsStatusLabel = computed(() => ({ connected: '已连接', connecting: '连接中', disconnected: '未连接' }[wsStatus.value] || wsStatus.value))
const cleanLLMActions = text => {
  if (!text) return ''
  return text.replace(/^action:\s*(.+?)(?:\s*;\s*priority:\s*(\d+))?$/gm, (_, desc, pri) => `${pri || '-'}. **[P${pri || '?'}]** ${desc.trim()}`)
}
const renderMd = text => sanitizeHtml(marked.parse(cleanLLMActions(text || '')))
const shouldShowReasoning = message => {
  const steps = message.reasoningChain || []
  return !!steps.length && (activeAudience.value === 'ops' || steps.some(step => String(step.action || '').startsWith('inspection_')))
}
const shouldShowDataSources = message => ['ops', 'oncall'].includes(activeAudience.value) && message.dataSources?.length
const shouldShowContent = message => !!(message.content && message.content.trim())
const modeLabel = mode => ({ diagnostic: '诊断', inspection: '巡检', report: '上报', topology: '拓扑' }[mode] || '问诊')
const formatTime = value => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
const loadSessions = async () => {
  const { data } = await axios.get('/api/v1/chat/sessions')
  sessions.value = (data || []).map(item => ({ ...item, id: item.id || item.sessionId, mode: item.mode || 'diagnostic' }))
  if (!activeSessionId.value && sessions.value.length) selectSession(sessions.value[0])
}

const loadModels = async () => {
  try {
    const { data } = await axios.get('/api/v1/models')
    models.value = normalizeModelsResponse(data)
    if (!modelOptions.value.includes(currentModel.value) && modelOptions.value.length) {
      localStorage.removeItem('selectedModel')
      saveModel(modelOptions.value[0])
    }
  } catch { models.value = currentModel.value ? [currentModel.value] : [] }
}

const normalizeModelsResponse = data => {
  const raw = Array.isArray(data) ? data : (Array.isArray(data?.models) ? data.models : (Array.isArray(data?.data) ? data.data : []))
  return raw.map(item => typeof item === 'string' ? item : (item?.id || item?.name || item?.model)).filter(Boolean)
}

const saveModel = value => {
  currentModel.value = value
  if (value) localStorage.setItem('selectedModel', value)
  else localStorage.removeItem('selectedModel')
}

const createSessionWithTitle = async (title = 'AIOps 智能问诊', mode = sessionDraft.value.mode) => {
  const { data } = await axios.post('/api/v1/aiops/sessions', { title, mode, scope: {}, model: currentModel.value })
  const session = { id: data.data.sessionId, title: data.data.title, mode: data.data.mode, createdAt: data.data.createdAt, updatedAt: data.data.createdAt }
  sessions.value.unshift(session)
  activeSessionId.value = session.id
  messages.value = []
  connectWS(session.id)
  return session
}

const createSession = async () => {
  let title = 'AIOps 智能问诊'
  try {
    const { value } = await ElMessageBox.prompt('请输入会话标题', '新建会话', {
      inputValue: title,
      confirmButtonText: '确认',
      cancelButtonText: '取消'
    })
    title = String(value || title).trim() || title
  } catch { return }
  await createSessionWithTitle(title, sessionDraft.value.mode)
}

const renameSession = async session => {
  try {
    const { value } = await ElMessageBox.prompt('请输入新的会话标题', '重命名会话', {
      inputValue: session.title,
      confirmButtonText: '确认',
      cancelButtonText: '取消'
    })
    const title = String(value || '').trim()
    if (!title) return
    await axios.put(`/api/v1/chat/sessions/${session.id}`, { title })
    session.title = title
    ElMessage.success('已重命名')
  } catch { /* 用户取消或接口失败时保持原状 */ }
}

const deleteSession = async session => {
  try {
    await ElMessageBox.confirm(`确认删除会话「${session.title}」？`, '删除会话', {
      type: 'warning',
      confirmButtonText: '确认',
      cancelButtonText: '取消'
    })
    await axios.delete(`/api/v1/chat/sessions/${session.id}`)
    sessions.value = sessions.value.filter(s => s.id !== session.id)
    if (activeSessionId.value === session.id) {
      if (sessions.value.length) selectSession(sessions.value[0])
      else { activeSessionId.value = null; messages.value = [] }
    }
  } catch { /* 用户取消或删除失败时保持原状 */ }
}

const selectSession = async session => {
  activeSessionId.value = session.id || session.sessionId
  connectWS(activeSessionId.value)
  try {
    const { data } = await axios.get(`/api/v1/aiops/sessions/${activeSessionId.value}/messages`)
    messages.value = data.data?.messages || []
    scrollBottom()
  } catch { messages.value = session.messages || [] }
}

const ensureSession = async () => { if (!activeSessionId.value) await createSession() }

const sendMessage = async () => {
  const text = inputText.value.trim()
  if (!text || loading.value) return
  await ensureSession()
  messages.value.push({ messageId: `local-${Date.now()}`, role: 'user', content: text, createdAt: new Date().toISOString() })
  inputText.value = ''
  loading.value = true
  scrollBottom()
  if (sendWS({ type: 'chat', content: text, audience: activeAudience.value, attachments: [], timestamp: new Date().toISOString() })) return
  try {
    const { data } = await axios.post(`/api/v1/aiops/sessions/${activeSessionId.value}/messages`, { role: 'user', content: text, audience: activeAudience.value, model: currentModel.value })
    messages.value.push(data.data)
    if (data.data?.topology) topologyHighlight.value = data.data.topology
    subscribeSuggestedMetrics(data.data?.suggestedActions || [])
    await loadSessions()
  } catch (error) { ElMessage.error(`问诊失败：${error.response?.data?.error || error.message}`) }
  finally { loading.value = false; scrollBottom() }
}

const startRouteDiagnosis = async () => {
  if (routeDiagnosisStarted.value) return
  const ip = String(route.query.ip || '').trim()
  if (!ip) return
  routeDiagnosisStarted.value = true
  const title = String(route.query.title || '').trim() || `${ip} 告警诊断`
  await createSessionWithTitle(`告警诊断：${title}`, 'diagnostic')
  inputText.value = `${ip} ${title}，请生成诊断报告`
  await sendMessage()
}

const runAction = async action => {
  if (action.type === 'command') {
    const command = action.command || action.params?.command || ''
    await navigator.clipboard?.writeText(command)
    actionResult.value = command
    ElMessage.success('已复制命令；平台不会执行写操作')
    return
  }
  if (action.type === 'link' || action.type === 'topology') {
    const url = action.url || action.params?.url || '/topology'
    if (url.startsWith('/topology')) topologyOpen.value = true
    router.push(url)
    return
  }
  if (action.type === 'promql') {
    const query = action.query || action.params?.query
    if (sendWS({ type: 'execute_action', actionType: 'promql', actionId: action.id, params: { query }, timestamp: new Date().toISOString() })) {
      sendWS({ type: 'subscribe_metrics', metrics: [query], interval: 8, timestamp: new Date().toISOString() })
      return
    }
    const { data } = await axios.post(`/api/v1/aiops/sessions/${activeSessionId.value}/actions/execute`, { actionType: 'promql', actionId: action.id, params: { query } })
    actionResult.value = JSON.stringify(data.data, null, 2)
  }
}

const messagePlainText = message => String(message?.content || '').replace(/[#*_`>\-[\]()]/g, ' ').replace(/\s+/g, ' ').trim()
const sendMessageFeedback = async (message, rating) => {
  const id = message.messageId || message.id || activeSessionId.value
  try {
    await axios.post('/api/v1/diagnosis/feedback', { report_id: id, rating })
    feedbackSent.value = { ...feedbackSent.value, [id]: true }
    ElMessage.success('反馈已提交')
  } catch (e) {
    ElMessage.warning(e.response?.data?.error || '反馈接口暂不可用')
  }
}

const archiveMessageCase = async message => {
  const description = messagePlainText(message) || 'AIOps 诊断结论'
  try {
    await axios.post('/api/v1/knowledge/cases', {
      root_cause_category: message.root_cause_category || 'AI诊断',
      root_cause_description: description,
      keywords: 'aiops,workbench',
      treatment_steps: '查看诊断报告并按建议执行',
      metric_snapshot: {}
    })
    ElMessage.success('已归档到知识库')
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '归档失败')
  }
}

const refreshSidePanel = async () => {
  const [alertRes, healthRes] = await Promise.allSettled([axios.get('/api/v1/alerts'), axios.get('/api/v1/health/datasources')])
  if (alertRes.status === 'fulfilled') alerts.value = alertRes.value.data || []
  if (healthRes.status === 'fulfilled') dataSourceStatus.value = Array.isArray(healthRes.value.data) ? healthRes.value.data : Object.entries(healthRes.value.data || {}).map(([name, status]) => ({ name, status }))
}

const handleTopologyNode = node => fillQuestion(`帮我诊断 ${node.ip || node.id} ${node.services?.[0]?.name || ''}`.trim())
const fillQuestion = value => { inputText.value = String(value || '') }
const copyText = async (text, message = '已复制') => { await navigator.clipboard?.writeText(String(text || '')); ElMessage.success(message) }
const handoffToText = note => [note.summary, `状态：${note.status || '-'}`, note.verifiedFacts?.length ? `已验证事实：${note.verifiedFacts.join('、')}` : '', note.openQuestions?.length ? `待确认问题：${note.openQuestions.join('、')}` : '', note.suggestedNext?.length ? `建议下一步：${note.suggestedNext.join('、')}` : '', note.escalationPolicy ? `升级策略：${note.escalationPolicy}` : ''].filter(Boolean).join('\n')
const copyHandoff = note => copyText(handoffToText(note), '交接单已复制到剪贴板')

watch(activeAudience, value => localStorage.setItem('aiops-audience', value))
onMounted(async () => { await loadModels(); await loadSessions(); await refreshSidePanel(); await startRouteDiagnosis() })
onBeforeUnmount(closeWS)
</script>

<style scoped>
@import '../styles/markdown.css';
.aiops-page { height: 100%; min-height: 0; padding: 16px 18px; display: grid; grid-template-columns: 256px minmax(560px, 1fr) 320px; gap: 12px; color: #22324d; overflow: hidden; }
.glass-card { background: linear-gradient(145deg, rgba(255,255,255,.66), rgba(226,238,255,.46)); border: 1px solid rgba(255,255,255,.72); box-shadow: 0 24px 70px rgba(63,100,160,.17), inset 0 1px 0 rgba(255,255,255,.78); backdrop-filter: blur(24px) saturate(145%); border-radius: 24px; }
.session-rail,.chat-workspace { min-height: 0; overflow: auto; }
.session-rail { padding: 14px; display:flex; flex-direction:column; }
.rail-head,.chat-head { display:flex; align-items:flex-start; justify-content:space-between; gap:10px; }
.rail-head span,.chat-head span { color:#247cff; font-size:10px; font-weight:900; letter-spacing:.08em; }
.rail-head h2,.chat-head h1 { margin:4px 0 0; color:#233653; }
.rail-head button { border:0; min-width:32px; height:32px; border-radius:12px; background:#257cff; color:white; cursor:pointer; padding:0 10px; }
.mode-pills,.quick-row { display:flex; gap:6px; flex-wrap:wrap; margin:12px 0; }
.mode-pills button,.quick-row button { border:1px solid rgba(101,125,160,.18); border-radius:999px; background:rgba(255,255,255,.55); color:#51637d; padding:6px 10px; font-size:12px; cursor:pointer; }
.mode-pills button.active { color:white; background:#247cff; border-color:#247cff; }
.ws-state { border-radius:14px; padding:7px 10px; font-size:12px; font-weight:800; background:#f1f5f9; color:#64748b; }
.ws-state.connected { background:#dcfce7; color:#166534; } .ws-state.connecting { background:#fef3c7; color:#92400e; }
.session-list { flex:1; overflow:auto; padding-right:4px; margin-top:10px; }
.session-item { width:100%; border:0; border-radius:16px; background:rgba(255,255,255,.46); padding:11px; margin-bottom:8px; text-align:left; cursor:pointer; color:#2b3b57; position:relative; }
.session-rename,.session-delete { position:absolute; top:8px; border:0; background:transparent; color:#94a3b8; cursor:pointer; padding:2px; border-radius:6px; opacity:0; transition:opacity .2s; }
.session-rename { right:32px; }
.session-delete { right:8px; }
.session-item:hover .session-rename,.session-item:hover .session-delete { opacity:1; } .session-delete:hover { color:#ef4444; background:rgba(239,68,68,.1); } .session-rename:hover { color:#247cff; background:rgba(36,124,255,.1); }
.session-item.active { background:#fff; box-shadow:0 12px 28px rgba(66,111,180,.18); outline:2px solid rgba(36,124,255,.22); }
.session-item strong,.session-item span { display:block; } .session-item span { color:#71819a; font-size:11px; margin-top:4px; }
.source-card { margin-top:10px; border-radius:18px; background:rgba(255,255,255,.5); padding:12px; font-size:12px; display:flex; flex-direction:column; gap:7px; }
.chat-workspace { display:flex; flex-direction:column; padding:14px; }
.chat-state { display:flex; align-items:center; gap:8px; color:#63748f; font-size:13px; font-weight:700; }
.chat-state i { width:9px; height:9px; border-radius:50%; background:#22c55e; box-shadow:0 0 0 4px rgba(34,197,94,.12); } .chat-state i.loading { background:#f59e0b; box-shadow:0 0 0 4px rgba(245,158,11,.15); }
.messages { flex:1; overflow:auto; padding:12px 4px; }
.empty-state { height:100%; display:flex; align-items:center; justify-content:center; flex-direction:column; text-align:center; color:#6f8098; }
.empty-orb { width:82px; height:82px; border-radius:50%; background:radial-gradient(circle, #72a9ff, transparent 68%); margin-bottom:14px; }
.message { display:flex; margin:12px 0; } .message.user { justify-content:flex-end; }
.bubble { max-width:86%; border-radius:18px; padding:13px 15px; background:rgba(255,255,255,.72); box-shadow:inset 0 1px 0 rgba(255,255,255,.86), 0 14px 34px rgba(73,105,150,.12); overflow:hidden; word-break:break-word; }
.message.user .bubble { background:#247cff; color:white; }
.source-usage,.actions-row,.diagnosis-actions { display:flex; flex-wrap:wrap; gap:8px; margin-top:10px; } .source-usage span { border-radius:999px; padding:4px 8px; background:#eef6ff; color:#2563eb; font-size:11px; }
.actions-row button { border:1px solid rgba(37,124,255,.2); border-radius:12px; background:#fff; color:#1d4ed8; padding:7px 9px; cursor:pointer; display:flex; align-items:center; gap:5px; }
.diagnosis-actions button { border:1px solid rgba(36,124,255,.18); border-radius:999px; background:rgba(255,255,255,.76); color:#1d4ed8; padding:6px 10px; cursor:pointer; font-size:12px; }
.diagnosis-actions button:disabled { cursor:not-allowed; opacity:.55; }
.composer { border-top:1px solid rgba(104,128,165,.16); padding-top:10px; } .composer-foot,.composer-buttons { display:flex; justify-content:space-between; align-items:center; gap:10px; margin-top:8px; color:#71819a; font-size:12px; }
@media (max-width: 1440px) { .aiops-page { grid-template-columns: 230px 1fr 280px; gap: 10px; } }
@media (max-width: 1280px) { .aiops-page { grid-template-columns: 230px 1fr; } :deep(.realtime-panel) { display:none; } }
@media (max-width: 1024px) { .aiops-page { grid-template-columns: 1fr; } .session-rail { display:none; } }
</style>
