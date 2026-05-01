<template>
  <div class="wf-canvas" ref="canvasRef">
    <div v-if="!nodes.length" class="empty-hint">
      <el-empty description="暂无可视化节点，请确认 DSL 格式正确" :image-size="60" />
    </div>
    <template v-else>
      <svg class="canvas-svg" :width="svgWidth" :height="svgHeight">
        <defs>
          <marker id="arrow" viewBox="0 0 10 10" refX="10" refY="5"
            markerWidth="8" markerHeight="8" orient="auto-start-reverse">
            <path d="M 0 0 L 10 5 L 0 10 z" fill="#247cff" />
          </marker>
        </defs>
        <path v-for="(edge, i) in edges" :key="'e'+i"
          :d="edge.path" fill="none" stroke="#247cff" stroke-width="2"
          stroke-dasharray="6,3" marker-end="url(#arrow)" />
      </svg>
      <div v-for="node in nodes" :key="node.id" class="wf-node glass-node"
        :class="{ active: selectedId === node.id }"
        :style="{ left: node.x + 'px', top: node.y + 'px' }"
        @click="selectNode(node)">
        <div class="node-icon">{{ nodeIcon(node.type) }}</div>
        <div class="node-body">
          <div class="node-title">{{ node.title || node.id }}</div>
          <div class="node-type">{{ node.type }}</div>
        </div>
      </div>
    </template>

    <!-- 节点详情面板 -->
    <transition name="slide">
      <div v-if="selectedNode" class="detail-panel glass-node">
        <div class="panel-head">
          <span class="panel-title">{{ selectedNode.title || selectedNode.id }}</span>
          <el-button size="small" link @click="selectedId = ''">关闭</el-button>
        </div>
        <el-descriptions :column="1" size="small" border>
          <el-descriptions-item label="节点 ID">{{ selectedNode.id }}</el-descriptions-item>
          <el-descriptions-item label="类型">{{ selectedNode.type }}</el-descriptions-item>
          <el-descriptions-item v-for="(val, key) in selectedNode.config" :key="key" :label="key">
            <pre class="config-val">{{ typeof val === 'object' ? JSON.stringify(val, null, 2) : String(val) }}</pre>
          </el-descriptions-item>
        </el-descriptions>
      </div>
    </transition>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'

const props = defineProps({ dsl: { type: [String, Object], default: '' } })

const NODE_W = 180
const NODE_H = 64
const GAP_X = 80
const GAP_Y = 20
const PAD = 40

const canvasRef = ref(null)
const selectedId = ref('')

const ICON_MAP = {
  start: '▶', end: '⏹', llm: '🤖', code: '⚙', http: '🌐',
  condition: '🔀', loop: '🔁', tool: '🔧', knowledge: '📚',
  template: '📝', variable: '📦', default: '📌',
}

const nodeIcon = (type) => ICON_MAP[type] || ICON_MAP.default

/* --- 解析 DSL --- */
const parsedGraph = computed(() => {
  let raw = props.dsl
  if (!raw) return { nodes: [], edges: [] }
  if (typeof raw === 'string') {
    try { raw = JSON.parse(raw) } catch { return parseYamlLike(raw) }
  }
  return extractGraph(raw)
})

const extractGraph = (obj) => {
  const nodes = []
  const edgeList = []
  const steps = obj?.nodes || obj?.steps || obj?.graph?.nodes || []
  if (Array.isArray(steps)) {
    for (const s of steps) {
      const id = s.id || s.name || `node_${nodes.length}`
      const { id: _, name: __, next: ___, ...config } = s
      nodes.push({ id, title: s.title || s.name || id, type: s.type || 'default', config })
      if (s.next) {
        const targets = Array.isArray(s.next) ? s.next : [s.next]
        for (const t of targets) edgeList.push({ from: id, to: t })
      }
    }
  }
  if (!edgeList.length && nodes.length > 1) {
    for (let i = 0; i < nodes.length - 1; i++) {
      edgeList.push({ from: nodes[i].id, to: nodes[i + 1].id })
    }
  }
  return { nodes, edges: edgeList }
}
/* PLACEHOLDER_PART2 */

const parseYamlLike = (text) => {
  const nodes = []
  const lines = text.split('\n')
  let currentNode = null
  for (const line of lines) {
    const trimmed = line.trim()
    if (trimmed.startsWith('- id:') || trimmed.startsWith('- name:')) {
      if (currentNode) nodes.push(currentNode)
      const val = trimmed.split(':').slice(1).join(':').trim()
      currentNode = { id: val, title: val, type: 'default', config: {} }
    } else if (currentNode && trimmed.startsWith('type:')) {
      currentNode.type = trimmed.split(':').slice(1).join(':').trim()
    } else if (currentNode && trimmed.startsWith('title:')) {
      currentNode.title = trimmed.split(':').slice(1).join(':').trim()
    }
  }
  if (currentNode) nodes.push(currentNode)
  const edgeList = []
  for (let i = 0; i < nodes.length - 1; i++) {
    edgeList.push({ from: nodes[i].id, to: nodes[i + 1].id })
  }
  return { nodes, edges: edgeList }
}

/* --- 布局计算 --- */
const nodes = computed(() => {
  const raw = parsedGraph.value.nodes
  return raw.map((n, i) => ({
    ...n,
    x: PAD + i * (NODE_W + GAP_X),
    y: PAD + (i % 2) * GAP_Y,
  }))
})

const nodeMap = computed(() => {
  const m = {}
  for (const n of nodes.value) m[n.id] = n
  return m
})

const edges = computed(() => {
  return parsedGraph.value.edges.map(e => {
    const from = nodeMap.value[e.from]
    const to = nodeMap.value[e.to]
    if (!from || !to) return null
    const x1 = from.x + NODE_W
    const y1 = from.y + NODE_H / 2
    const x2 = to.x
    const y2 = to.y + NODE_H / 2
    const cx = (x1 + x2) / 2
    return { path: `M${x1},${y1} C${cx},${y1} ${cx},${y2} ${x2},${y2}` }
  }).filter(Boolean)
})

const svgWidth = computed(() => {
  if (!nodes.value.length) return 0
  return PAD * 2 + nodes.value.length * (NODE_W + GAP_X)
})

const svgHeight = computed(() => PAD * 2 + NODE_H + GAP_Y + 40)

const selectedNode = computed(() => nodes.value.find(n => n.id === selectedId.value) || null)

const selectNode = (node) => {
  selectedId.value = selectedId.value === node.id ? '' : node.id
}

watch(() => props.dsl, () => { selectedId.value = '' })
</script>

<style scoped>
.wf-canvas { position: relative; min-height: 200px; overflow: auto; padding: 10px; }
.canvas-svg { position: absolute; top: 0; left: 0; pointer-events: none; }
.empty-hint { display: flex; justify-content: center; padding: 30px 0; }
.glass-node {
  background: rgba(255,255,255,0.08);
  backdrop-filter: blur(12px);
  border: 1px solid rgba(255,255,255,0.12);
  border-radius: 12px;
}
.wf-node {
  position: absolute;
  width: 180px;
  height: 64px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 14px;
  cursor: pointer;
  transition: box-shadow .2s, border-color .2s;
}
.wf-node:hover { border-color: rgba(36,124,255,.5); box-shadow: 0 4px 16px rgba(36,124,255,.15); }
.wf-node.active { border-color: #247cff; box-shadow: 0 4px 20px rgba(36,124,255,.25); }
.node-icon { font-size: 22px; flex-shrink: 0; }
.node-body { overflow: hidden; }
.node-title { font-size: 13px; font-weight: 700; color: #243553; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.node-type { font-size: 11px; color: #98a3b8; }
.detail-panel { position: absolute; top: 10px; right: 10px; width: 280px; padding: 14px; z-index: 10; }
.panel-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.panel-title { font-weight: 700; font-size: 14px; color: #243553; }
.config-val { font-family: ui-monospace, monospace; font-size: 11px; white-space: pre-wrap; word-break: break-all; margin: 0; }
.slide-enter-active, .slide-leave-active { transition: transform .25s ease, opacity .25s ease; }
.slide-enter-from, .slide-leave-to { transform: translateX(20px); opacity: 0; }
</style>
