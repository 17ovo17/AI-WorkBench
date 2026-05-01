<template>
  <div class="catpaw-chat">
    <div class="chat-head">
      <div>
        <div class="panel-kicker">Catpaw Chat</div>
        <h3>探针交互诊断</h3>
      </div>
      <div class="head-right">
        <el-select v-model="selectedAgent" placeholder="选择在线探针" size="small" style="width:160px">
          <el-option v-for="a in agents" :key="a.ip" :label="`${a.ip} ${a.hostname || ''}`" :value="a.ip" />
        </el-select>
        <el-button size="small" type="primary" :disabled="!selectedAgent || connected" @click="connect">连接</el-button>
        <el-button size="small" type="danger" plain :disabled="!connected" @click="disconnect">断开</el-button>
      </div>
    </div>

    <div class="terminal" ref="termBox">
      <div v-if="!lines.length" class="term-empty">
        选择在线探针后连接，可发送只读巡检问题；危险命令必须经过平台安全确认。
      </div>
      <div v-for="(line, i) in lines" :key="i" class="term-line" v-html="ansi(line)"></div>
    </div>

    <div class="input-row">
      <el-input
        v-model="inputText"
        placeholder="输入巡检问题或只读排查命令，发送给 Catpaw 子 Agent..."
        :disabled="!connected"
        @keydown.enter.exact.prevent="send"
        size="small"
      />
      <el-button size="small" type="primary" :disabled="!connected || !inputText.trim()" @click="send">发送</el-button>
    </div>
  </div>
</template>

<script setup>
import { ref, nextTick, onUnmounted } from 'vue'
import axios from 'axios'

const props = defineProps({ credentials: Array })

const agents = ref([])
const selectedAgent = ref('')
const connected = ref(false)
const lines = ref([])
const inputText = ref('')
const termBox = ref(null)
let ws = null

const ansi = (text) => String(text || '')
  .replace(/&/g, '&amp;')
  .replace(/</g, '&lt;')
  .replace(/>/g, '&gt;')
  .replace(/\x1b\[(\d+)m/g, '')

const scrollBottom = () => nextTick(() => {
  if (termBox.value) termBox.value.scrollTop = termBox.value.scrollHeight
})

const loadAgents = async () => {
  try {
    const { data } = await axios.get('/api/v1/catpaw/agents')
    agents.value = (data || []).filter(a => a.online)
  } catch (error) {
    lines.value.push(`[加载探针失败] ${error?.message || error}`)
  }
}

const connect = () => {
  const agent = agents.value.find(a => a.ip === selectedAgent.value)
  if (!agent) {
    lines.value.push('[连接失败] 请先选择一个在线探针。')
    return
  }
  const cred = props.credentials?.find(c => c.ip === agent.ip) || {}
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/api/v1/catpaw/chat-ws`)
  ws.onopen = () => {
    connected.value = true
    lines.value = [`[已连接] ${agent.ip} ${agent.hostname || ''}`]
    ws.send(JSON.stringify({
      ip: agent.ip,
      port: cred.port || 22,
      username: cred.username || 'root',
      password: cred.password || '',
      ssh_key: cred.ssh_key || ''
    }))
  }
  ws.onmessage = (event) => {
    const newLines = String(event.data || '').split('\n')
    if (lines.value.length && newLines[0]) {
      lines.value[lines.value.length - 1] += newLines[0]
      lines.value.push(...newLines.slice(1))
    } else {
      lines.value.push(...newLines)
    }
    if (lines.value.length > 500) lines.value = lines.value.slice(-500)
    scrollBottom()
  }
  ws.onclose = () => {
    connected.value = false
    lines.value.push('[连接已断开]')
  }
  ws.onerror = () => {
    connected.value = false
    lines.value.push('[连接错误] 请检查探针在线状态、凭证和网络连通性。')
  }
}

const disconnect = () => ws?.close()

const send = () => {
  if (!inputText.value.trim() || !ws || !connected.value) return
  ws.send(inputText.value + '\n')
  inputText.value = ''
}

loadAgents()
onUnmounted(() => ws?.close())
</script>

<style scoped>
.catpaw-chat { display: flex; flex-direction: column; height: 100%; }
.chat-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; gap: 12px; }
.chat-head h3 { margin: 0; font-size: 14px; }
.panel-kicker { font-size: 10px; color: #58a6ff; text-transform: uppercase; letter-spacing: 1px; }
.head-right { display: flex; gap: 8px; align-items: center; }
.terminal { flex: 1; background: #0d1117; border: 1px solid #30363d; border-radius: 6px; padding: 12px; font-family: monospace; font-size: 12px; overflow-y: auto; min-height: 300px; max-height: 500px; color: #e6edf3; }
.term-empty { color: #8b949e; line-height: 1.6; }
.term-line { line-height: 1.5; white-space: pre-wrap; word-break: break-all; }
.input-row { display: flex; gap: 8px; margin-top: 10px; }
</style>
