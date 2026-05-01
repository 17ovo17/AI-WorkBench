import { ref } from 'vue'
import { ElMessage } from 'element-plus'

/**
 * AIOps WebSocket 连接管理 composable
 * 负责 WS 连接、重连、消息分发
 */
export function useAiopsWebSocket({
  messages,
  loading,
  topologyGraph,
  topologyHighlight,
  metricUpdates,
  actionResult,
  pendingAssistantId,
  scrollBottom,
  normalizeTopologyGraph,
  subscribeSuggestedMetrics,
}) {
  const wsStatus = ref('disconnected')
  const wsRef = ref(null)
  const reconnectTimer = ref(null)
  const reconnectAttempt = ref(0)

  const sendWS = payload => {
    if (wsRef.value?.readyState === WebSocket.OPEN) {
      wsRef.value.send(JSON.stringify(payload))
      return true
    }
    return false
  }

  const ensureAssistantPlaceholder = () => {
    let msg = messages.value.find(item => item.messageId === pendingAssistantId.value)
    if (!msg) {
      pendingAssistantId.value = `assistant-${Date.now()}`
      msg = {
        messageId: pendingAssistantId.value,
        role: 'assistant',
        content: '正在执行 AIOps 推理链...',
        reasoningChain: [],
        createdAt: new Date().toISOString(),
      }
      messages.value.push(msg)
    }
    return msg
  }
// PLACEHOLDER_REST

  const upsertReasoningStep = step => {
    const msg = ensureAssistantPlaceholder()
    const list = msg.reasoningChain || []
    const index = list.findIndex(item => item.step === step.step)
    const normalized = { ...step, timestamp: step.timestamp || new Date().toISOString() }
    if (index >= 0) list[index] = { ...list[index], ...normalized }
    else list.push(normalized)
    msg.reasoningChain = [...list].sort((a, b) => a.step - b.step)
    scrollBottom()
  }

  const finishAssistantMessage = message => {
    const index = messages.value.findIndex(item => item.messageId === pendingAssistantId.value)
    if (index >= 0) messages.value[index] = message
    else messages.value.push(message)
    pendingAssistantId.value = ''
    if (message?.topology) topologyHighlight.value = message.topology
    subscribeSuggestedMetrics(message?.suggestedActions || [])
    loading.value = false
    scrollBottom()
  }

  const handleWSMessage = data => {
    if (data.type === 'pong' || data.type === 'connected') { wsStatus.value = 'connected'; return }
    if (data.type === 'reasoning_step') upsertReasoningStep(data)
    else if (data.type === 'diagnosis') finishAssistantMessage(data.message)
    else if (data.type === 'topology_update') {
      topologyGraph.value = normalizeTopologyGraph(data.topology)
      topologyHighlight.value = data.highlight || {}
    }
    else if (data.type === 'metric_update') metricUpdates.value.push(data)
    else if (data.type === 'action_result') actionResult.value = JSON.stringify(data.result, null, 2)
    else if (data.type === 'error') ElMessage.error(data.message || 'WebSocket 错误')
    else if (data.type === 'complete') { loading.value = false; scrollBottom() }
  }

  const closeWS = () => {
    if (reconnectTimer.value) clearTimeout(reconnectTimer.value)
    reconnectTimer.value = null
    if (wsRef.value) {
      const ws = wsRef.value
      wsRef.value = null
      ws.onclose = null
      ws.close()
    }
  }

  const scheduleReconnect = sessionId => {
    if (!sessionId || wsRef.value === null) return
    wsStatus.value = 'disconnected'
    const delay = Math.min(30000, 1000 * Math.pow(2, reconnectAttempt.value++))
    reconnectTimer.value = setTimeout(() => connectWS(sessionId), delay)
  }

  const connectWS = sessionId => {
    closeWS()
    if (!sessionId) return
    wsStatus.value = 'connecting'
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${protocol}//${window.location.host}/api/v1/aiops/ws/sessions/${sessionId}`)
    wsRef.value = ws
    ws.onopen = () => {
      wsStatus.value = 'connected'
      reconnectAttempt.value = 0
      sendWS({ type: 'ping', timestamp: new Date().toISOString() })
    }
    ws.onmessage = event => handleWSMessage(JSON.parse(event.data))
    ws.onerror = () => { wsStatus.value = 'disconnected' }
    ws.onclose = () => scheduleReconnect(sessionId)
  }

  return { wsStatus, connectWS, closeWS, sendWS }
}
