<template>
  <div class="bt-canvas">
    <div class="bt-toolbar">
      <div class="bt-title">
        <b>{{ title || '业务拓扑画布' }}</b>
        <span>{{ summaryText }}</span>
      </div>
      <div class="bt-actions">
        <button :class="{ active: layoutMode === 'force' }" type="button" @click="switchLayout('force')">力导向图</button>
        <button :class="{ active: layoutMode === 'tree' }" type="button" @click="switchLayout('tree')">树形布局</button>
        <button :class="{ active: riskOpen }" type="button" @click="riskOpen = !riskOpen">风险视图</button>
        <button type="button" @click="resetZoom">重置视图</button>
      </div>
    </div>

    <div class="bt-body" :class="{ 'with-sidebar': !hideHostIndex }">
      <aside v-if="!hideHostIndex" class="host-index">
        <b>业务主机清单</b>
        <small>主机只作为索引，不绘制为拓扑节点</small>
        <button
          v-for="host in hosts"
          :key="host"
          type="button"
          :class="{ active: selectedNode?.ip === host }"
          @click="focusHost(host)"
        >
          <strong>{{ host }}</strong>
          <span>{{ hostServiceCount(host) }} 个业务组件</span>
        </button>
      </aside>

      <section ref="containerRef" class="graph-shell">
        <div v-if="riskOpen" class="risk-banner">
          <b>拓扑结构风险</b>
          <span v-if="!normalizedRisks.length">未发现结构风险</span>
          <p v-for="risk in normalizedRisks" :key="`${risk.type}-${risk.title}-${risk.description}`">
            {{ risk.title || risk.type }}：{{ risk.description }}
          </p>
        </div>
        <svg ref="svgRef" class="graph-svg"></svg>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as d3 from 'd3'

const props = defineProps({
  graph: { type: Object, default: () => ({ nodes: [], links: [], risks: [], summary: {} }) },
  hosts: { type: Array, default: () => [] },
  title: { type: String, default: '' },
  hideHostIndex: { type: Boolean, default: false }
})

const emit = defineEmits(['select-node', 'host-missing'])
const containerRef = ref(null)
const svgRef = ref(null)
const layoutMode = ref('force')
const riskOpen = ref(false)
const selectedNode = ref(null)
const nodeDialogOpen = ref(false)
let simulation = null
let zoomBehavior = null
let zoomLayer = null

const normalizedNodes = computed(() => (props.graph?.nodes || []).filter(node => node && node.id && !['host', 'ai_agent', 'catpaw_agent'].includes(node.layer)))
const normalizedLinks = computed(() => (props.graph?.links || []).filter(link => link?.source && link?.target))
const normalizedRisks = computed(() => props.graph?.risks || [])
const summaryText = computed(() => `${normalizedNodes.value.length} 节点 · ${normalizedLinks.value.length} 连线 · ${normalizedRisks.value.length} 风险`)

const layerConfig = {
  gateway: { label: '入口层', color: '#f59e0b', order: 0 },
  app: { label: '应用层', color: '#3b82f6', order: 1 },
  cache: { label: '缓存层', color: '#8b5cf6', order: 2 },
  mq: { label: '消息层', color: '#ec4899', order: 2 },
  db: { label: '数据层', color: '#10b981', order: 3 },
  infra: { label: '基础设施', color: '#64748b', order: 4 },
  monitor: { label: '观测层', color: '#06b6d4', order: 5 }
}

const healthColor = status => ({ healthy: '#22c55e', warning: '#f59e0b', danger: '#ef4444', unknown: '#94a3b8' }[status] || '#94a3b8')
const healthLabel = status => ({ healthy: '健康', warning: '告警', danger: '危险', unknown: '未知' }[status] || '未知')
const layerLabel = layer => layerConfig[layer]?.label || layer || '业务组件'
const metricValue = value => Number.isFinite(Number(value)) ? Number(value).toFixed(1) : '--'
const downstreamCount = id => normalizedLinks.value.filter(link => link.source === id).length
const upstreamCount = id => normalizedLinks.value.filter(link => link.target === id).length
const hostServiceCount = host => normalizedNodes.value.filter(node => node.ip === host).length

const switchLayout = mode => {
  layoutMode.value = mode
  renderGraph()
}

const resetZoom = () => {
  if (!svgRef.value || !zoomBehavior) return
  d3.select(svgRef.value).transition().duration(250).call(zoomBehavior.transform, d3.zoomIdentity)
}

const focusHost = host => {
  const node = normalizedNodes.value.find(item => item.ip === host)
  if (!node) {
    emit('host-missing', host)
    return
  }
  selectNode(node)
}

const selectNode = node => {
  selectedNode.value = node
  emit('select-node', node)
  highlightRelated(node.id)
}

const highlightRelated = id => {
  if (!zoomLayer) return
  const related = new Set([id])
  normalizedLinks.value.forEach(link => {
    if (link.source === id) related.add(link.target)
    if (link.target === id) related.add(link.source)
  })
  zoomLayer.selectAll('.node').attr('opacity', d => related.has(d.id) ? 1 : 0.22)
  zoomLayer.selectAll('.node-label').attr('opacity', d => related.has(d.id) ? 1 : 0.22)
  zoomLayer.selectAll('.link').attr('opacity', d => d.source.id === id || d.target.id === id ? 1 : 0.12)
}

const renderGraph = async () => {
  await nextTick()
  const container = containerRef.value
  const svgEl = svgRef.value
  if (!container || !svgEl) return
  const width = container.clientWidth || 900
  const height = container.clientHeight || 620
  if (simulation) simulation.stop()

  const nodes = normalizedNodes.value.map(node => ({ ...node }))
  const links = normalizedLinks.value.map(link => ({ ...link }))
  const nodeIDs = new Set(nodes.map(node => node.id))
  const safeLinks = links.filter(link => nodeIDs.has(link.source) && nodeIDs.has(link.target))
  const nodeByID = new Map(nodes.map(node => [node.id, node]))
  safeLinks.forEach(link => {
    link.source = nodeByID.get(link.source) || link.source
    link.target = nodeByID.get(link.target) || link.target
  })

  const svg = d3.select(svgEl)
  svg.selectAll('*').remove()
  svg.attr('viewBox', `0 0 ${width} ${height}`)

  const defs = svg.append('defs')
  defs.append('marker').attr('id', 'arrow-solid').attr('viewBox', '0 -5 10 10').attr('refX', 30).attr('refY', 0).attr('markerWidth', 7).attr('markerHeight', 7).attr('orient', 'auto').append('path').attr('d', 'M0,-5L10,0L0,5').attr('fill', '#64748b')
  defs.append('marker').attr('id', 'arrow-dashed').attr('viewBox', '0 -5 10 10').attr('refX', 30).attr('refY', 0).attr('markerWidth', 7).attr('markerHeight', 7).attr('orient', 'auto').append('path').attr('d', 'M0,-5L10,0L0,5').attr('fill', '#94a3b8')

  zoomLayer = svg.append('g')
  zoomBehavior = d3.zoom().scaleExtent([0.2, 4]).on('zoom', event => zoomLayer.attr('transform', event.transform))
  svg.call(zoomBehavior)

  const link = zoomLayer.selectAll('.link').data(safeLinks).join('line')
    .attr('class', 'link')
    .attr('stroke', d => d.dashed || d.relation === 'replication' ? '#94a3b8' : '#64748b')
    .attr('stroke-width', d => d.relation === 'replication' ? 2.5 : 1.6)
    .attr('stroke-dasharray', d => d.dashed ? '6 5' : null)
    .attr('marker-end', d => d.dashed ? 'url(#arrow-dashed)' : 'url(#arrow-solid)')

  const linkLabel = zoomLayer.selectAll('.link-label').data(safeLinks).join('text')
    .attr('class', 'link-label')
    .attr('font-size', 10)
    .attr('fill', '#64748b')
    .attr('text-anchor', 'middle')
    .text(d => d.label || d.type)

  const packet = zoomLayer.selectAll('.packet').data(safeLinks).join('circle')
    .attr('class', 'packet')
    .attr('r', 3)
    .attr('fill', '#60a5fa')
    .attr('opacity', 0.75)

  const node = zoomLayer.selectAll('.node').data(nodes).join('g')
    .attr('class', 'node')
    .style('cursor', 'pointer')
    .on('click', (_, d) => selectNode(d))

  node.each(function drawNode(d) {
    const group = d3.select(this)
    const layer = layerConfig[d.layer] || layerConfig.app
    const health = healthColor(d.health?.status)
    group.append('circle').attr('r', 25).attr('fill', 'none').attr('stroke', health).attr('stroke-width', d.health?.status === 'danger' ? 4 : 2.5).attr('opacity', 0.85)
    if (d.layer === 'gateway') {
      group.append('polygon').attr('points', hexPoints(18)).attr('fill', layer.color).attr('stroke', '#0f172a').attr('stroke-width', 1.5)
    } else if (d.layer === 'cache') {
      group.append('polygon').attr('points', '0,-19 19,0 0,19 -19,0').attr('fill', layer.color).attr('stroke', '#0f172a').attr('stroke-width', 1.5)
    } else if (d.layer === 'mq') {
      group.append('polygon').attr('points', '-20,-14 15,-14 20,14 -15,14').attr('fill', layer.color).attr('stroke', '#0f172a').attr('stroke-width', 1.5)
    } else if (d.layer === 'db') {
      group.append('rect').attr('x', -18).attr('y', -13).attr('width', 36).attr('height', 28).attr('rx', 4).attr('fill', layer.color).attr('stroke', '#0f172a').attr('stroke-width', 1.5)
      group.append('ellipse').attr('cx', 0).attr('cy', -13).attr('rx', 18).attr('ry', 6).attr('fill', layer.color).attr('stroke', '#0f172a').attr('stroke-width', 1.3)
    } else {
      group.append('rect').attr('x', -20).attr('y', -14).attr('width', 40).attr('height', 28).attr('rx', d.layer === 'infra' ? 10 : 6).attr('fill', layer.color).attr('stroke', '#0f172a').attr('stroke-width', 1.5)
    }
    group.append('circle').attr('cx', 17).attr('cy', -17).attr('r', 5).attr('fill', health).attr('stroke', '#fff').attr('stroke-width', 1.5)
  })

  const label = zoomLayer.selectAll('.node-label').data(nodes).join('text')
    .attr('class', 'node-label')
    .attr('text-anchor', 'middle')
    .attr('font-size', 11)
    .attr('fill', '#1e293b')
    .attr('font-weight', 700)
    .text(d => shortLabel(d))

  const ipLabel = zoomLayer.selectAll('.node-ip').data(nodes).join('text')
    .attr('class', 'node-ip')
    .attr('text-anchor', 'middle')
    .attr('font-size', 9)
    .attr('fill', '#64748b')
    .text(d => d.ip)

  if (layoutMode.value === 'tree') {
    applyTree(nodes, width, height)
    ticked()
  } else {
    simulation = d3.forceSimulation(nodes)
      .force('link', d3.forceLink(safeLinks).id(d => d.id).distance(220))
      .force('charge', d3.forceManyBody().strength(-900))
      .force('center', d3.forceCenter(width / 2, height / 2))
      .force('collision', d3.forceCollide().radius(58))
      .force('y', d3.forceY(d => 90 + (layerConfig[d.layer]?.order ?? 2) * 160).strength(0.12))
      .on('tick', ticked)
    node.call(d3.drag().on('start', dragStart).on('drag', dragged).on('end', dragEnd))
  }

  animatePackets()

  function ticked() {
    link.attr('x1', d => d.source.x).attr('y1', d => d.source.y).attr('x2', d => d.target.x).attr('y2', d => d.target.y)
    linkLabel.attr('x', d => (d.source.x + d.target.x) / 2).attr('y', d => (d.source.y + d.target.y) / 2 - 7)
    node.attr('transform', d => `translate(${d.x},${d.y})`)
    label.attr('x', d => d.x).attr('y', d => d.y + 38)
    ipLabel.attr('x', d => d.x).attr('y', d => d.y + 52)
  }

  function animatePackets() {
    packet.each(function movePacket(d) {
      const el = d3.select(this)
      function repeat() {
        el.attr('cx', d.source.x).attr('cy', d.source.y)
          .transition().duration(1800).ease(d3.easeLinear)
          .attr('cx', d.target.x).attr('cy', d.target.y)
          .on('end', repeat)
      }
      repeat()
    })
  }

  function dragStart(event, d) {
    if (!event.active && simulation) simulation.alphaTarget(0.25).restart()
    d.fx = d.x
    d.fy = d.y
  }
  function dragged(event, d) {
    d.fx = event.x
    d.fy = event.y
  }
  function dragEnd(event, d) {
    if (!event.active && simulation) simulation.alphaTarget(0)
    d.fx = null
    d.fy = null
  }
}

const hexPoints = radius => d3.range(6).map(index => {
  const angle = Math.PI / 3 * index - Math.PI / 2
  return `${radius * Math.cos(angle)},${radius * Math.sin(angle)}`
}).join(' ')

const shortLabel = node => {
  const svc = node.services?.[0]?.name || node.hostname || node.id
  return svc.length > 16 ? `${svc.slice(0, 14)}…` : svc
}

const applyTree = (nodes, width, height) => {
  const groups = d3.group(nodes, node => node.layer)
  const layerOrder = ['gateway', 'app', 'cache', 'mq', 'db', 'infra', 'monitor']
  const rowGap = Math.max(78, (height - 110) / Math.max(layerOrder.length, 1))
  layerOrder.forEach((layer, row) => {
    const items = groups.get(layer) || []
    const gap = width / (items.length + 1 || 2)
    items.forEach((node, index) => {
      node.x = gap * (index + 1)
      node.y = 70 + row * rowGap
    })
  })
}

watch(() => props.graph, renderGraph, { deep: true })
watch(layoutMode, renderGraph)
onMounted(renderGraph)
onBeforeUnmount(() => { if (simulation) simulation.stop() })
defineExpose({ loadData: renderGraph, updateHealth: renderGraph, resetZoom })
</script>

<style scoped>
@import '../styles/topology-canvas.css';
</style>
