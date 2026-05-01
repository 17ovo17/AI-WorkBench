<template>
  <div class="topology-page">
    <header class="topology-head glass-card">
      <div>
        <span>BUSINESS TOPOLOGY</span>
        <h1>业务拓扑</h1>
        <p>按用户定义的业务、服务器 IP 和端口进行自动发现；不会混入其他无关主机。</p>
      </div>
      <div class="actions">
        <el-button type="primary" @click="openBusinessDialog()">新增业务端口</el-button>
        <el-button type="success" :loading="inspectionLoading" :disabled="!activeBusiness" @click="inspectActive">业务巡检</el-button>
        <el-button :disabled="!activeBusiness" @click="openNodeDialog()">添加节点</el-button>
        <el-button :loading="loading" :disabled="!activeBusiness" @click="rediscoverActive">重新发现</el-button>
        <el-button :disabled="!activeBusiness" @click="showObservability = !showObservability">{{ showObservability ? '折叠观测层' : '显示观测层' }}</el-button>
        <el-button :disabled="!activeBusiness" @click="saveActive">保存当前业务</el-button>
      </div>
    </header>

    <section class="topology-layout">
      <aside class="business-panel glass-card">
        <div class="panel-title">
          <h3>业务列表</h3>
          <em>{{ businesses.length }} 项</em>
        </div>
        <div class="business-list">
          <div
            v-for="biz in businesses"
            :key="biz.id"
            class="business-item"
            :class="{ active: activeBusiness?.id === biz.id }"
            @click="selectBusiness(biz)"
          >
            <button class="business-main" type="button">
              <strong>{{ biz.name }}</strong>
              <span>{{ (biz.hosts || []).length }} 台服务器 · {{ (biz.endpoints || []).length }} 个端口</span>
            </button>
            <div v-if="activeBusiness?.id === biz.id" class="business-hosts-inline">
              <el-tag v-for="host in biz.hosts" :key="host" size="small">{{ host }}</el-tag>
              <small v-if="biz.attributes?.owner">负责人：{{ biz.attributes.owner }}</small>
            </div>
            <div class="business-actions" @click.stop>
              <el-button size="small" text @click="openBusinessDialog(biz)">编辑</el-button>
              <el-button size="small" text type="danger" @click="deleteBusiness(biz)">删除</el-button>
            </div>
          </div>
        </div>
        <div v-if="!businesses.length" class="hint-card">先点击"新增业务端口"，定义业务名称、服务器 IP 和业务端口后再自动发现。</div>
        <div v-if="activeBusiness" class="inspection-records">
          <div class="inspection-record-head">
            <h3>巡检记录</h3>
            <el-button size="small" text @click="loadInspectionRecords">刷新</el-button>
            <el-button size="small" text type="danger" :disabled="!inspectionRecords.length" @click="cleanupInspectionRecords">清理当前业务</el-button>
          </div>
          <button
            v-for="record in inspectionRecords"
            :key="record.id"
            type="button"
            class="inspection-record-item"
            @click="openInspectionRecord(record)"
          >
            <strong>{{ record.alert_title || record.target_ip }}</strong>
            <span>{{ formatTime(record.create_time) }} · {{ statusLabel(record.status) }}</span>
          </button>
          <div v-if="!inspectionRecords.length" class="inspection-record-empty">暂无业务巡检记录</div>
        </div>
      </aside>

      <main class="canvas glass-card">
        <div v-if="!activeBusiness" class="empty-state">
          <h2>请选择或新增一个业务</h2>
          <p>每个业务独立保存自己的服务器、端口和自动发现结果。</p>
        </div>
        <BusinessTopologyCanvas
          v-else
          :graph="aiGraph"
          :hosts="activeBusiness.hosts || []"
          :title="activeBusiness.name"
          :hideHostIndex="true"
          @select-node="handleCanvasSelect"
          @host-missing="handleHostMissing"
        />
      </main>
    </section>

    <el-drawer v-model="detailDrawerOpen" title="节点详情" direction="rtl" size="400px" :destroy-on-close="false">
      <template v-if="selected || canvasSelectedNode">
        <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px">
          <strong style="font-size:16px">{{ canvasSelectedNode?.hostname || canvasSelectedNode?.id || selected?.name || '节点' }}</strong>
          <el-button v-if="selected" size="small" type="danger" plain @click="deleteNode(selected)">删除节点</el-button>
        </div>

        <div v-if="canvasSelectedNode" style="margin-bottom:16px">
          <span :style="{ padding:'4px 10px', borderRadius:'8px', fontSize:'12px', fontWeight:700, background: canvasSelectedNode.health?.status === 'healthy' ? '#dcfce7' : canvasSelectedNode.health?.status === 'warning' ? '#fef3c7' : '#fee2e2', color: canvasSelectedNode.health?.status === 'healthy' ? '#166534' : canvasSelectedNode.health?.status === 'warning' ? '#92400e' : '#991b1b' }">{{ canvasSelectedNode.health?.score ?? '--' }} · {{ canvasSelectedNode.health?.status || '未知' }}</span>
          <p style="color:#64748b;margin-top:8px">{{ canvasSelectedNode.layer || '未知层' }} · {{ canvasSelectedNode.ip }}</p>
          <div v-if="canvasSelectedNode.services?.length" style="margin-top:10px">
            <b style="font-size:13px">服务端口</b>
            <div style="display:flex;flex-wrap:wrap;gap:6px;margin-top:6px">
              <el-tag v-for="svc in canvasSelectedNode.services" :key="`${svc.name}-${svc.port}`" size="small">{{ svc.name }}:{{ svc.port }}</el-tag>
            </div>
          </div>
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:8px;margin-top:12px;font-size:13px">
            <span>CPU {{ (canvasSelectedNode.metrics?.cpu || 0).toFixed(1) }}%</span>
            <span>内存 {{ (canvasSelectedNode.metrics?.mem || 0).toFixed(1) }}%</span>
            <span>磁盘 {{ (canvasSelectedNode.metrics?.disk || 0).toFixed(1) }}%</span>
            <span>Load {{ (canvasSelectedNode.metrics?.load || 0).toFixed(1) }}</span>
          </div>
        </div>

        <div v-if="selected" style="border-top:1px solid rgba(0,0,0,.06);padding-top:12px">
          <b style="font-size:13px">发现详情</b>
          <p style="margin-top:6px">{{ typeLabel(selected.type) }} · {{ statusLabel(selected.status) }}</p>
          <ul style="margin:8px 0;padding-left:18px;font-size:13px">
            <li v-if="selected.ip">IP：{{ selected.ip }}</li>
            <li v-if="selected.port">端口：{{ selected.port }}</li>
            <li v-if="selected.service_name">业务/组件：{{ selected.service_name }}</li>
            <li v-if="selected.meta">来源：{{ displaySource(selected.meta) }}</li>
          </ul>
        </div>

        <div v-if="relatedEdges.length" style="border-top:1px solid rgba(0,0,0,.06);padding-top:12px;margin-top:12px">
          <b style="font-size:13px">关联连线</b>
          <div v-for="edge in relatedEdges" :key="edge.id" style="padding:6px 0;border-bottom:1px solid rgba(0,0,0,.04);font-size:12px">
            <span :class="['line-state', edge.status]"></span>
            {{ nodeName(edge.source_id) }} → {{ nodeName(edge.target_id) }}
            <small style="color:#64748b;display:block">{{ edgeText(edge) }}</small>
          </div>
        </div>
      </template>
      <p v-else style="color:#94a3b8">点击节点查看详情</p>
    </el-drawer>

    <el-dialog v-model="businessDialog" :title="editingBusiness ? '编辑业务端口' : '新增业务端口'" width="680px">
      <el-form label-position="top">
        <el-form-item label="业务名称">
          <el-input v-model="draft.name" placeholder="例如：订单系统 / AI WorkBench / 数据库集群" />
        </el-form-item>
        <el-form-item label="业务属性（负责人、用途、SLO、等级等）">
          <el-input v-model="draft.attributesText" type="textarea" :rows="4" placeholder="owner=张三&#10;purpose=订单交易核心链路&#10;slo=99.9%&#10;level=P1" />
        </el-form-item>
        <el-form-item label="服务器 IP（每行一个）">
          <el-input v-model="draft.hostsText" type="textarea" :rows="4" placeholder="198.18.20.11&#10;198.18.20.123&#10;192.168.1.7" />
        </el-form-item>
        <el-form-item label="业务端口（每行：IP:端口 服务名 协议，可选）">
          <el-input v-model="draft.endpointsText" type="textarea" :rows="5" placeholder="198.18.20.11:8081 Web入口 HTTP&#10;198.18.20.146:3306 MySQL MySQL&#10;198.18.20.229:8123 ClickHouse HTTP" />
        </el-form-item>
        <div class="hint-card">保存后只发现该业务定义的主机与端口；Prometheus target 即使 up=0 也可作为测试发现数据纳入拓扑，红线仅表示健康断开。</div>
      </el-form>
      <template #footer>
        <el-button @click="businessDialog = false">取消</el-button>
        <el-button type="primary" :loading="loading" @click="saveBusinessDraft">{{ editingBusiness ? '保存并重新发现' : '创建并发现' }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="nodeDialog" title="添加拓扑节点" width="520px">
      <el-form label-position="top">
        <el-form-item label="节点类型">
          <el-select v-model="nodeDraft.type" style="width: 100%">
            <el-option label="服务器" value="host" />
            <el-option label="业务端口" value="service" />
            <el-option label="前端" value="frontend" />
            <el-option label="后端" value="backend" />
            <el-option label="数据库" value="database" />
            <el-option label="缓存" value="cache" />
            <el-option label="监控" value="monitor" />
          </el-select>
        </el-form-item>
        <el-form-item label="名称"><el-input v-model="nodeDraft.name" /></el-form-item>
        <el-form-item label="IP"><el-input v-model="nodeDraft.ip" placeholder="例如：198.18.20.20" /></el-form-item>
        <el-form-item label="端口"><el-input-number v-model="nodeDraft.port" :min="0" :max="65535" style="width: 100%" /></el-form-item>
        <el-form-item label="协议"><el-input v-model="nodeDraft.protocol" placeholder="TCP / HTTP / MySQL / Redis" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="nodeDialog = false">取消</el-button>
        <el-button type="primary" @click="addNode">添加</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="inspectionDialog" title="业务巡检与业务监控" width="980px">
      <div v-if="inspection" style="max-height:calc(100vh - 260px);overflow-y:auto">
        <InspectionDashboard :inspection="inspection" />
      </div>
      <template #footer>
        <el-button @click="inspectionDialog = false">关闭</el-button>
        <el-button type="primary" :loading="inspectionLoading" @click="inspectActive">重新巡检</el-button>
        <el-button type="success" @click="downloadInspectionReport">下载报告</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import BusinessTopologyCanvas from '../components/BusinessTopologyCanvas.vue'
import InspectionDashboard from '../components/InspectionDashboard.vue'

const canvasWidth = 1600
const canvasHeight = 920
const businesses = ref([])
const activeBusiness = ref(null)
const graph = reactive({ nodes: [], edges: [], discovery: null })
const selected = ref(null)
const detailDrawerOpen = ref(false)
const canvasSelectedNode = ref(null)
const showObservability = ref(false)
const loading = ref(false)
const inspectionLoading = ref(false)
const businessDialog = ref(false)
const nodeDialog = ref(false)
const inspectionDialog = ref(false)
const inspection = ref(null)
const inspectionRecords = ref([])
const inspectionRecordsLoading = ref(false)
const editingBusiness = ref(null)
const aiGraph = ref({ nodes: [], links: [], risks: [], summary: {} })
const draft = reactive({ name: '', hostsText: '', endpointsText: '', attributesText: '' })
const nodeDraft = reactive({ type: 'service', name: '', ip: '', port: 0, protocol: 'TCP' })
const layerLabels = [
  { key: 'entry', title: '入口层', desc: 'Nginx / Gateway', x: 300 },
  { key: 'app', title: '应用层', desc: 'JVM / App', x: 530 },
  { key: 'middleware', title: '中间件层', desc: 'Redis / Sentinel / MQ', x: 760 },
  { key: 'database', title: '数据库层', desc: 'Oracle / MySQL / PostgreSQL', x: 990 },
  { key: 'observe', title: '观测层', desc: 'Prometheus / Exporter', x: 1220 }
]

const loadBusinesses = async () => {
  const res = await axios.get('/api/v1/topology/businesses')
  businesses.value = res.data || []
  if (!activeBusiness.value && businesses.value.length) selectBusiness(businesses.value[0])
  if (activeBusiness.value) {
    const fresh = businesses.value.find(item => item.id === activeBusiness.value.id)
    if (fresh) selectBusiness(fresh)
  }
}

const loadInspectionRecords = async () => {
  if (!activeBusiness.value?.id) {
    inspectionRecords.value = []
    return
  }
  inspectionRecordsLoading.value = true
  try {
    const { data } = await axios.get('/api/v1/diagnose', { params: { source: 'business_inspection', business_id: activeBusiness.value.id } })
    inspectionRecords.value = data || []
  } catch (error) {
    inspectionRecords.value = []
    ElMessage.error(`\u52a0\u8f7d\u5de1\u68c0\u8bb0\u5f55\u5931\u8d25\uff1a${error.response?.data?.error || error.message || '\u8bf7\u7a0d\u540e\u91cd\u8bd5'}`)
  } finally {
    inspectionRecordsLoading.value = false
  }
}


const cleanupInspectionRecords = async () => {
  if (!activeBusiness.value?.id) return
  try {
    await ElMessageBox.confirm(`确认清理"${activeBusiness.value.name}"的业务巡检记录？只删除诊断中心内该业务巡检记录，不删除业务拓扑。`, '清理当前业务巡检', {
      type: 'warning', confirmButtonText: '确认清理', cancelButtonText: '取消'
    })
    const { data } = await axios.delete('/api/v1/diagnose', { params: { scope: 'business_inspection', business_id: activeBusiness.value.id } })
    inspectionRecords.value = []
    ElMessage.success(`已清理 ${data?.deleted || 0} 条当前业务巡检记录`)
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error('清理业务巡检记录失败')
  }
}

const selectBusiness = biz => {
  activeBusiness.value = biz
  const normalized = normalizeGraph((biz.graph && biz.graph.nodes) || [], (biz.graph && biz.graph.edges) || [])
  graph.nodes.splice(0, graph.nodes.length, ...normalized.nodes)
  graph.edges.splice(0, graph.edges.length, ...normalized.edges)
  graph.discovery = biz.graph?.discovery || null
  selected.value = visibleNodes.value[0] || graph.nodes[0] || null
  aiGraph.value = graphToAIJSON(biz)
  loadAITopology(biz)
  loadInspectionRecords()
}

const loadAITopology = async biz => {
  if (!biz?.id) return
  const payload = { business_id: biz.id, service_name: biz.name, hosts: biz.hosts || [], endpoints: biz.endpoints || [] }
  const endpoints = ['/api/v1/aiops/topology/generate', '/api/v1/topology/ai/generate']
  let lastError = null
  for (const endpoint of endpoints) {
    try {
      const { data } = await axios.post(endpoint, payload)
      const normalized = normalizeAIGraph(data)
      if (normalized.nodes.length > 0) { aiGraph.value = normalized; return }

    } catch (error) {
      lastError = error
      if (error.response?.status && error.response.status !== 404) break
    }
  }
  aiGraph.value = graphToAIJSON(biz)
  ElMessage.warning(`Topo-Architect ${'\u6682\u4e0d\u53ef\u7528\uff0c\u5df2\u4f7f\u7528\u672c\u5730\u62d3\u6251\u8f6c\u6362\uff1a'}${lastError?.response?.data?.error || lastError?.message || '\u63a5\u53e3\u4e0d\u53ef\u7528'}`)
}
const normalizeGraph = (nodes, edges) => {
  const nodeMap = new Map()
  nodes
    .filter(node => !String(node.id || '').startsWith('svc-') && !['ai_agent', 'catpaw_agent'].includes(node.type))
    .forEach(node => {
      const normalized = { ...node }
      normalized.layer = defaultLayer(normalized)
      nodeMap.set(normalized.id, normalized)
    })
  const filteredNodes = [...nodeMap.values()]
  const topologyNodes = filteredNodes.filter(node => node.type !== 'host')
  const ids = new Set(topologyNodes.map(node => node.id))
  const edgeMap = new Map()
  edges
    .filter(edge => ids.has(edge.source_id) && ids.has(edge.target_id))
    .forEach(edge => edgeMap.set(edge.id || `${edge.source_id}-${edge.target_id}-${edge.protocol}`, edge))
  const filteredEdges = [...edgeMap.values()]
  return { nodes: topologyNodes, edges: filteredEdges }
}

const normalizeAIGraph = data => ({
  nodes: Array.isArray(data?.nodes) ? data.nodes : [],
  links: Array.isArray(data?.links) ? data.links : [],
  risks: Array.isArray(data?.risks) ? data.risks : [],
  summary: data?.summary || {}
})

const graphToAIJSON = biz => {
  const normalized = normalizeGraph((biz?.graph && biz.graph.nodes) || [], (biz?.graph && biz.graph.edges) || [])
  const nodes = normalized.nodes
    .filter(node => !['host', 'ai_agent', 'catpaw_agent'].includes(node.type))
    .map(node => ({
      id: node.id,
      ip: node.ip || '',
      hostname: node.name || node.ip || node.id,
      layer: aiLayerFromNode(node),
      services: [{ name: node.service_name || node.name || 'service', port: Number(node.port || 0), role: typeLabel(node.type) }],
      health: healthFromNode(node),
      metrics: { cpu: 0, mem: 0, disk: 0, load: 0 },
      alerts: node.status === 'offline' || node.status === 'disconnected' ? ['端口或进程状态异常'] : []
    }))
  const ids = new Set(nodes.map(node => node.id))
  const links = normalized.edges
    .filter(edge => ids.has(edge.source_id) && ids.has(edge.target_id))
    .map(edge => ({
      source: edge.source_id,
      target: edge.target_id,
      type: edge.protocol || 'TCP',
      label: translateOpsText(edge.label || edge.protocol || '依赖'),
      dashed: edge.protocol === 'Metrics' || /sync|replication|注册|发现/i.test(edge.label || '')
    }))
  return { nodes, links, risks: detectLocalTopologyRisks(nodes, links), summary: { planner: 'frontend_legacy_adapter', node_count: nodes.length, link_count: links.length } }
}

const aiLayerFromNode = node => {
  const type = String(node.type || '').toLowerCase()
  const name = String(node.service_name || node.name || '').toLowerCase()
  const port = Number(node.port || 0)
  if (['frontend', 'entry', 'gateway'].includes(type) || name.includes('nginx') || [80, 443, 8443].includes(port)) return 'gateway'
  if (type === 'cache' || name.includes('redis') || [6379, 11211].includes(port)) return 'cache'
  if (type === 'middleware' && (name.includes('kafka') || name.includes('mq') || [9092, 5672, 9876].includes(port))) return 'mq'
  if (['database', 'db'].includes(type) || /oracle|mysql|postgres|mongo|elastic/.test(name) || [3306, 5432, 1521, 9200].includes(port)) return 'db'
  if (/etcd|zookeeper|consul/.test(name) || [2379, 2181, 8300].includes(port)) return 'infra'
  if (type === 'monitor' || /prometheus|categraf|exporter/.test(name) || [9090, 9100, 9101].includes(port)) return 'monitor'
  return 'app'
}

const healthFromNode = node => {
  if (['offline', 'disconnected'].includes(node.status)) return { score: 45, status: 'danger' }
  if (node.status === 'unknown') return { score: 0, status: 'unknown' }
  return { score: 90, status: 'healthy' }
}

const detectLocalTopologyRisks = (nodes, links) => {
  const risks = []
  const layerCounts = nodes.reduce((acc, node) => ({ ...acc, [node.layer]: (acc[node.layer] || 0) + 1 }), {})
  ;['gateway', 'cache', 'mq', 'db', 'infra'].forEach(layer => {
    if (layerCounts[layer] === 1) risks.push({ type: 'single_point', title: '单点风险', description: `${layer} 层仅 1 个节点` })
  })
  const degree = new Map(nodes.map(node => [node.id, 0]))
  links.forEach(link => {
    degree.set(link.source, (degree.get(link.source) || 0) + 1)
    degree.set(link.target, (degree.get(link.target) || 0) + 1)
  })
  nodes.forEach(node => { if ((degree.get(node.id) || 0) === 0) risks.push({ type: 'island', title: '孤岛节点', description: `${node.id} 无依赖连线` }) })
  return risks
}

const openBusinessDialog = (biz = null) => {
  editingBusiness.value = biz
  draft.name = biz?.name || ''
  draft.hostsText = (biz?.hosts || []).join('\n')
  draft.endpointsText = (biz?.endpoints || []).map(ep => `${ep.ip}:${ep.port} ${ep.service_name || ''} ${ep.protocol || 'TCP'}`.trim()).join('\n')
  draft.attributesText = attrsToText(biz?.attributes || {})
  businessDialog.value = true
}

const parseHosts = text => [...new Set(text.split(/\n|,|\s+/).map(x => x.trim()).filter(Boolean).map(x => x.replace(/^https?:\/\//, '').split('/')[0].split(':')[0]))]
const parseEndpoints = text => text.split('\n').map(line => {
  const parts = line.trim().split(/\s+/).filter(Boolean)
  if (!parts.length) return null
  const [address, serviceName = '', protocol = 'TCP'] = parts
  const [ip, portText] = address.replace(/^https?:\/\//, '').split('/')[0].split(':')
  const port = Number(portText)
  if (!ip || !port) return null
  return { ip, port, service_name: serviceName, protocol }
}).filter(Boolean)

const parseAttributes = text => Object.fromEntries(text.split('\n').map(line => line.trim()).filter(Boolean).map(line => {
  const index = line.indexOf('=')
  if (index < 0) return [line, '']
  return [line.slice(0, index).trim(), line.slice(index + 1).trim()]
}).filter(([key]) => key))

const attrsToText = attrs => Object.entries(attrs || {}).map(([key, value]) => `${key}=${value}`).join('\n')

const discoverGraph = async (hosts, endpoints, includePlatform = false) => {
  const res = await axios.post('/api/v1/topology/discover', { hosts, endpoints, include_platform: includePlatform, use_ai: true })
  return res.data || { nodes: [], edges: [] }
}

const saveBusinessDraft = async () => {
  const name = draft.name.trim()
  const hosts = parseHosts(draft.hostsText)
  const endpoints = parseEndpoints(draft.endpointsText)
  const attributes = parseAttributes(draft.attributesText)
  for (const ep of endpoints) if (!hosts.includes(ep.ip)) hosts.push(ep.ip)
  if (!name) return ElMessage.warning('请填写业务名称')
  if (!hosts.length && !endpoints.length) return ElMessage.warning('请至少填写一台服务器或一个业务端口')
  loading.value = true
  try {
    const discovered = await discoverGraph(hosts, endpoints, false)
    const payload = { ...(editingBusiness.value || {}), name, hosts, endpoints, attributes, graph: discovered }
    const saved = await axios.post('/api/v1/topology/businesses', payload)
    businessDialog.value = false
    activeBusiness.value = saved.data
    await loadBusinesses()
    selectBusiness(saved.data)
    ElMessage.success(editingBusiness.value ? '业务拓扑已更新' : '业务拓扑已创建')
  } catch (error) {
    const message = error.response?.data?.error || error.message || '创建并发现失败'
    ElMessage.error(`创建并发现失败：${message}`)
    console.error(error)
  } finally {
    loading.value = false
  }
}

const inspectActive = async () => {
  if (!activeBusiness.value?.id) return
  inspectionLoading.value = true
  try {
    const { data } = await axios.get(`/api/v1/topology/businesses/${activeBusiness.value.id}/inspect`)
    inspection.value = data
    inspectionDialog.value = true
    await loadInspectionRecords()
    ElMessage.success('\u4e1a\u52a1\u5de1\u68c0\u5b8c\u6210\uff0c\u8bb0\u5f55\u5df2\u5199\u5165\u667a\u80fd\u8bca\u65ad\u4e2d\u5fc3')
  } catch (error) {
    ElMessage.error(`\u4e1a\u52a1\u5de1\u68c0\u5931\u8d25\uff1a${error.response?.data?.error || error.message || '\u8bf7\u68c0\u67e5\u540e\u7aef\u3001AI \u6216\u6570\u636e\u6e90\u72b6\u6001'}`)
  } finally {
    inspectionLoading.value = false
  }
}

const downloadInspectionReport = () => {
  if (!inspection.value) return
  const ins = inspection.value
  const lines = [
    `# ${ins.business_name || '\u4e1a\u52a1'} \u5de1\u68c0\u62a5\u544a`,
    `- \u72b6\u6001\uff1a${ins.status || '\u672a\u77e5'}`,
    `- \u8bc4\u5206\uff1a${ins.score ?? '--'}`,
    `- \u6570\u636e\u6e90\uff1a${(ins.data_sources || []).join('\u3001') || '\u672a\u77e5'}`,
    `- \u65f6\u95f4\uff1a${ins.generated_at || new Date().toISOString()}`,
    '',
    '## AI \u5206\u6790\u62a5\u544a',
    '',
    ins.ai_analysis || '\u65e0 AI \u5206\u6790',
  ]
  if (ins.ai_suggestions?.length) {
    lines.push('', '## AI \u5efa\u8bae')
    ins.ai_suggestions.forEach(s => lines.push(`- ${s}`))
  }
  if (ins.metrics?.length) {
    lines.push('', '## \u6307\u6807\u6982\u89c8', '', '| IP | \u6307\u6807 | \u503c | \u72b6\u6001 |', '|---|---|---|---|')
    ins.metrics.forEach(m => lines.push(`| ${m.ip} | ${m.name} | ${Number(m.value || 0).toFixed(2)}${m.unit || ''} | ${m.status} |`))
  }
  if (ins.processes?.length) {
    lines.push('', '## \u8fdb\u7a0b/\u670d\u52a1', '', '| IP | \u540d\u79f0 | \u7aef\u53e3 | \u72b6\u6001 |', '|---|---|---|---|')
    ins.processes.forEach(p => lines.push(`| ${p.ip} | ${p.name} | ${p.port || '-'} | ${p.status} |`))
  }
  if (ins.alerts?.length) {
    lines.push('', '## \u544a\u8b66', '', '| \u6807\u9898 | IP | \u7ea7\u522b | \u72b6\u6001 |', '|---|---|---|---|')
    ins.alerts.forEach(a => lines.push(`| ${a.title} | ${a.target_ip} | ${a.severity} | ${a.status} |`))
  }
  const content = lines.join('\n')
  const blob = new Blob([content], { type: 'text/markdown;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${ins.business_name || 'business'}-inspection-${new Date().toISOString().slice(0, 10)}.md`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
  ElMessage.success('\u62a5\u544a\u5df2\u4e0b\u8f7d')
}


const deleteBusiness = async biz => {
  await ElMessageBox.confirm(`删除业务"${biz.name}"后，其拓扑节点和连线也会删除，是否继续？`, '二次确认', {
    type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消'
  })
  await axios.delete(`/api/v1/topology/businesses/${biz.id}`)
  if (activeBusiness.value?.id === biz.id) {
    activeBusiness.value = null
    graph.nodes.splice(0, graph.nodes.length)
    graph.edges.splice(0, graph.edges.length)
    selected.value = null
  }
  await loadBusinesses()
  ElMessage.success('业务已删除')
}

const rediscoverActive = async () => {
  if (!activeBusiness.value) return
  loading.value = true
  try {
    const discovered = await discoverGraph(activeBusiness.value.hosts || [], activeBusiness.value.endpoints || [], false)
    activeBusiness.value.graph = discovered
    graph.nodes.splice(0, graph.nodes.length, ...(discovered.nodes || []))
    graph.edges.splice(0, graph.edges.length, ...(discovered.edges || []))
    graph.discovery = discovered.discovery || null
    selected.value = visibleNodes.value[0] || graph.nodes[0] || null
    ElMessage.success('业务拓扑已重新发现')
  } catch (error) {
    const message = error.response?.data?.error || error.message || '重新发现失败'
    ElMessage.error(`重新发现失败：${message}`)
    console.error(error)
  } finally {
    loading.value = false
  }
}

const openInspectionRecord = record => {
  try {
    inspection.value = record.raw_report ? JSON.parse(record.raw_report) : null
  } catch {
    inspection.value = null
  }
  if (!inspection.value) {
    inspection.value = {
      executive_summary: record.summary_report || record.report || record.alert_title,
      risk_level: record.status === 'failed' ? 'critical' : 'unknown',
      ai_recommendations: [],
      evidence_refs: [],
      diagnose_record_id: record.id
    }
  }
  inspectionDialog.value = true
}

const saveActive = async () => {
  if (!activeBusiness.value) return
  activeBusiness.value.graph = { nodes: graph.nodes, edges: graph.edges, discovery: graph.discovery }
  const saved = await axios.post('/api/v1/topology/businesses', activeBusiness.value)
  activeBusiness.value = saved.data
  await loadBusinesses()
  ElMessage.success('当前业务已保存')
}

const openNodeDialog = () => {
  nodeDraft.type = 'service'
  nodeDraft.name = ''
  nodeDraft.ip = activeBusiness.value?.hosts?.[0] || ''
  nodeDraft.port = 0
  nodeDraft.protocol = 'TCP'
  nodeDialog.value = true
}

const addNode = () => {
  if (!activeBusiness.value) return
  const ip = nodeDraft.ip.trim()
  const name = nodeDraft.name.trim() || (nodeDraft.type === 'host' ? ip : `${ip}:${nodeDraft.port}`)
  if (!name) return ElMessage.warning('请填写节点名称或 IP')
  if (nodeDraft.type !== 'host' && (!ip || !nodeDraft.port)) return ElMessage.warning('业务端口节点需要 IP 和端口')
  if (nodeDraft.type === 'host' && !ip) return ElMessage.warning('服务器节点需要 IP')
  const hostID = `host-${sanitizeID(ip)}`
  if (nodeDraft.type === 'host') {
    if (graph.nodes.some(node => node.id === hostID)) return ElMessage.warning('该服务器节点已存在')
    graph.nodes.push({ id: hostID, name, type: 'host', ip, status: 'unknown', x: 80, y: 120 + graph.nodes.filter(n => n.type === 'host').length * 220, meta: '用户手工添加' })
    if (!activeBusiness.value.hosts.includes(ip)) activeBusiness.value.hosts.push(ip)
  } else {
    if (!graph.nodes.some(node => node.id === hostID)) {
      graph.nodes.push({ id: hostID, name: ip, type: 'host', ip, status: 'unknown', x: 80, y: 120 + graph.nodes.filter(n => n.type === 'host').length * 220, meta: '用户手工添加' })
      if (!activeBusiness.value.hosts.includes(ip)) activeBusiness.value.hosts.push(ip)
    }
    const nodeID = `manual-${sanitizeID(ip)}-${nodeDraft.port}`
    if (graph.nodes.some(node => node.id === nodeID)) return ElMessage.warning('该端口节点已存在')
    graph.nodes.push({ id: nodeID, name, type: nodeDraft.type, ip, port: nodeDraft.port, service_name: name, status: 'unknown', x: 360, y: 65 + relatedServiceCount(hostID) * 95, meta: '用户手工添加' })
    graph.edges.push({ id: `edge-${hostID}-${nodeID}`, source_id: hostID, target_id: nodeID, protocol: nodeDraft.protocol || 'TCP', direction: 'forward', label: name, status: 'unknown' })
    activeBusiness.value.endpoints.push({ ip, port: nodeDraft.port, service_name: name, protocol: nodeDraft.protocol || 'TCP' })
  }
  layoutLocal()
  nodeDialog.value = false
  ElMessage.success('节点已添加，记得保存当前业务')
}

const deleteNode = async node => {
  await ElMessageBox.confirm(`确认删除节点"${node.name}"？相关连线也会同步删除。`, '二次确认', {
    type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消'
  })
  graph.edges.splice(0, graph.edges.length, ...graph.edges.filter(edge => edge.source_id !== node.id && edge.target_id !== node.id))
  graph.nodes.splice(0, graph.nodes.length, ...graph.nodes.filter(item => item.id !== node.id))
  if (node.ip && node.port) {
    activeBusiness.value.endpoints = (activeBusiness.value.endpoints || []).filter(ep => !(ep.ip === node.ip && ep.port === node.port))
  }
  if (node.type === 'host' && node.ip) {
    activeBusiness.value.hosts = (activeBusiness.value.hosts || []).filter(host => host !== node.ip)
    activeBusiness.value.endpoints = (activeBusiness.value.endpoints || []).filter(ep => ep.ip !== node.ip)
  }
  selected.value = visibleNodes.value[0] || graph.nodes[0] || null
  layoutLocal()
  ElMessage.success('节点已删除，记得保存当前业务')
}

const relatedServiceCount = hostID => graph.edges.filter(edge => edge.source_id === hostID).length
const sanitizeID = value => String(value || '').toLowerCase().replace(/[.:_\s\[\]]+/g, '-')

const layoutLocal = () => {
  const layers = new Map()
  graph.nodes.forEach(node => {
    const layer = Number(node.layer || defaultLayer(node))
    node.layer = layer
    if (!layers.has(layer)) layers.set(layer, [])
    layers.get(layer).push(node)
  })
  ;[...layers.keys()].sort((a, b) => a - b).forEach(layer => {
    layers.get(layer).forEach((node, index) => {
      node.x = 70 + layer * 230
      node.y = 120 + index * 140
    })
  })
}

const defaultLayer = node => ({ host: 0, frontend: 1, application: 2, app: 2, backend: 2, service: 2, cache: 3, middleware: 3, database: 4, monitor: 5, management: 5 }[node.type] ?? semanticLayer(node))

const semanticLayer = node => {
  const name = String(node.service_name || node.name || '').toLowerCase()
  const port = Number(node.port || 0)
  if (node.type === 'host') return 0
  if (name.includes('nginx') || name.includes('gateway') || port === 80 || port === 443) return 1
  if (name.includes('redis') || name.includes('sentinel') || name.includes('mq') || port === 6379 || port === 6375 || port === 26379) return 3
  if (name.includes('oracle') || name.includes('mysql') || name.includes('postgres') || port === 1521 || port === 3306 || port === 5432) return 4
  if (name.includes('jvm') || name.includes('app') || name.includes('tomcat') || port === 8080 || port === 8081 || port === 8000) return 2
  if (port === 9090 || port === 9091 || port === 9100 || port === 9101) return 5
  return 2
}


const nodeName = id => graph.nodes.find(node => node.id === id)?.name || id
const getNode = id => graph.nodes.find(node => node.id === id) || { x: 0, y: 0 }
const nodeCenter = id => {
  const node = getNode(id)
  return { x: Number(node.x || 0) + 96, y: Number(node.y || 0) + 44 }
}
const edgePath = edge => {
  const a = nodeCenter(edge.source_id)
  const b = nodeCenter(edge.target_id)
  const mid = Math.max(a.x + 70, (a.x + b.x) / 2)
  return `M ${a.x} ${a.y} L ${mid} ${a.y} L ${mid} ${b.y} L ${b.x} ${b.y}`
}

const edgeLabel = edge => {
  const a = nodeCenter(edge.source_id)
  const b = nodeCenter(edge.target_id)
  return { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 - 8 }
}

const translateOpsText = value => {
  const text = String(value || '')
  const dictionary = {
    'User-defined business endpoint with automatic connectivity check': '用户定义业务端口，已执行自动连通性检查',
    'Nginx to application': '入口到应用',
    'Application to middleware': '应用到中间件',
    'Application to database': '应用到数据库',
    'One endpoint is unreachable; offline target does not block historical metric discovery': '存在端点不可达；离线目标不影响历史指标发现',
    'Both endpoints passed protocol-aware health check': '两端协议化健康检查均通过'
  }
  return dictionary[text] || text
}
const displaySource = value => translateOpsText(value)
const edgeText = edge => [translateOpsText(edge.label || edge.protocol || '\u8fde\u7ebf'), edge.error ? translateOpsText(edge.error) : ''].filter(Boolean).join(' / ')
const visibleNodes = computed(() => graph.nodes.filter(node => !['host', 'ai_agent', 'catpaw_agent'].includes(node.type) && (showObservability.value || node.type !== 'monitor')))
const visibleNodeIDs = computed(() => new Set(visibleNodes.value.map(node => node.id)))
const visibleEdges = computed(() => graph.edges.filter(edge => visibleNodeIDs.value.has(edge.source_id) && visibleNodeIDs.value.has(edge.target_id) && (showObservability.value || edge.protocol !== 'Metrics')))
const relatedEdges = computed(() => selected.value ? visibleEdges.value.filter(edge => edge.source_id === selected.value.id || edge.target_id === selected.value.id) : [])

const businessHostIndex = computed(() => {
  const hosts = new Set(activeBusiness.value?.hosts || [])
  ;(activeBusiness.value?.endpoints || []).forEach(endpoint => { if (endpoint?.ip) hosts.add(endpoint.ip) })
  graph.nodes.forEach(node => { if (node.ip && node.type !== 'monitor') hosts.add(node.ip) })
  return [...hosts].map(ip => {
    const services = graph.nodes.filter(node => node.ip === ip && node.type !== 'host' && node.type !== 'monitor')
    const agentOnline = services.some(node => ['online', 'connected'].includes(node.status))
    return { ip, name: ip, serviceCount: services.length, agentOnline }
  })
})

const selectHostScope = ip => {
  const target = visibleNodes.value.find(node => node.ip === ip) || graph.nodes.find(node => node.ip === ip && !['host', 'ai_agent', 'catpaw_agent'].includes(node.type))
  if (target) {
    selected.value = target
    ElMessage.success(`已定位 ${ip}：${target.name}`)
  } else {
    ElMessage.warning(`${ip} 暂无业务组件，请添加端口或重新发现`)
  }
}

const handleCanvasSelect = node => {
  canvasSelectedNode.value = node
  selected.value = visibleNodes.value.find(item => item.id === node.id || (item.ip === node.ip && item.port === node.services?.[0]?.port)) || selected.value
  detailDrawerOpen.value = true
}

const handleHostMissing = host => {
  ElMessage.warning(`${host} 暂无业务组件，请添加端口或重新发现`)
}
const typeLabel = type => ({ host: '\u4e1a\u52a1\u4e3b\u673a', frontend: '\u5165\u53e3\u5c42', application: '\u5e94\u7528\u5c42', app: '\u5e94\u7528\u5c42', backend: '\u5e94\u7528\u5c42', database: '\u6570\u636e\u5e93\u5c42', cache: '\u4e2d\u95f4\u4ef6\u5c42', middleware: '\u4e2d\u95f4\u4ef6\u5c42', monitor: '\u89c2\u6d4b\u5c42', service: '\u4e1a\u52a1\u7aef\u53e3', management: '\u7ba1\u7406\u534f\u8bae' }[type] || type || '\u7ec4\u4ef6')
const statusLabel = status => ({ online: '\u5728\u7ebf', offline: '\u79bb\u7ebf', connected: '\u8fde\u901a', disconnected: '\u65ad\u5f00', unknown: '\u672a\u77e5', pending: '\u7b49\u5f85\u4e2d', running: '\u8bca\u65ad\u4e2d', done: '\u5df2\u5b8c\u6210', failed: '\u5931\u8d25' }[status] || status || '\u672a\u77e5')
const formatTime = value => value ? new Date(value).toLocaleString('zh-CN') : '-'

onMounted(loadBusinesses)
</script>

<style scoped>
.topology-page { height: 100%; min-height: 0; padding: 14px 22px; color: #243553; overflow: hidden; display: flex; flex-direction: column; }
.glass-card { background: linear-gradient(145deg, rgba(255,255,255,.68), rgba(225,236,255,.48)); border: 1px solid rgba(255,255,255,.74); box-shadow: 0 22px 60px rgba(63,100,160,.15), inset 0 1px 0 rgba(255,255,255,.8); backdrop-filter: blur(22px); }
.topology-head { flex: 0 0 auto; min-height: 88px; border-radius: 24px; padding: 14px 20px; display:flex; align-items:center; justify-content:space-between; margin-bottom: 12px; }
.topology-head span { color:#2f7cff; font-weight:800; font-size:12px; }
.topology-head h1 { font-size:25px; margin:2px 0; }
.topology-head p { color:#6a7b95; font-size:12px; margin:0; }
.actions { display:flex; gap:10px; flex-wrap:wrap; justify-content:flex-end; }
.topology-layout { flex: 1; min-height: 0; display:grid; grid-template-columns: 280px minmax(0, 1fr); gap:12px; }
.business-panel,.detail-panel,.canvas { border-radius:24px; padding:15px; min-height:0; overflow:auto; }
.panel-title,.detail-head { display:flex; align-items:center; justify-content:space-between; gap:8px; margin-bottom:10px; }
.panel-title h3,.detail-panel h3 { margin:0; }
.panel-title em { font-style:normal; color:#2f7cff; font-size:12px; }
.business-list { display:flex; flex-direction:column; gap:9px; }
.business-item { border:1px solid rgba(215,226,246,.9); background:rgba(255,255,255,.55); border-radius:15px; padding:10px; cursor:pointer; color:#243553; }
.business-item.active { border-color:#2f7cff; background:rgba(47,124,255,.12); box-shadow:0 12px 24px rgba(47,124,255,.12); }
.business-main { all:unset; display:block; width:100%; cursor:pointer; }
.business-main strong,.business-main span { display:block; }
.business-main span { color:#72839d; font-size:11px; margin-top:5px; }
.business-hosts-inline { display:flex; flex-wrap:wrap; gap:4px; margin-top:8px; }
.business-hosts-inline small { width:100%; color:#64748b; font-size:11px; margin-top:4px; }
.business-actions { margin-top:8px; display:flex; justify-content:flex-end; gap:4px; }
.business-meta { margin-top:16px; display:flex; flex-wrap:wrap; gap:8px; }
.business-meta h3,.business-meta p { width:100%; margin:0; }
.business-meta small { border-radius:999px; padding:4px 8px; background:rgba(47,124,255,.1); color:#426188; }
.inspection-records { margin-top:16px; padding-top:14px; border-top:1px solid rgba(104,128,164,.16); display:flex; flex-direction:column; gap:8px; }
.inspection-record-head { display:flex; justify-content:space-between; align-items:center; gap:8px; }
.inspection-record-head h3 { margin:0; }
.inspection-record-item { width:100%; border:1px solid rgba(215,226,246,.9); background:rgba(255,255,255,.56); border-radius:14px; padding:10px; color:#243553; text-align:left; cursor:pointer; }
.inspection-record-item strong,.inspection-record-item span { display:block; }
.inspection-record-item strong { font-size:12px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.inspection-record-item span,.inspection-record-empty { margin-top:4px; color:#72839d; font-size:11px; }
.hint-card { border-radius:18px; padding:12px; background:rgba(255,255,255,.55); color:#657792; font-size:12px; line-height:1.55; }
.canvas { position:relative; overflow:auto; background-image: radial-gradient(rgba(47,124,255,.15) 1px, transparent 1px); background-size: 22px 22px; }
.canvas-toolbar { position:absolute; left:14px; right:14px; top:12px; z-index:4; display:flex; justify-content:space-between; pointer-events:none; color:#667895; font-size:12px; }
.legend,.summary { background:rgba(255,255,255,.78); border-radius:999px; padding:7px 11px; }
.legend { display:flex; align-items:center; gap:7px; }
.dot,.line-state { width:9px; height:9px; border-radius:999px; display:inline-block; }
.green,.connected { background:#24bd70; }
.red,.disconnected { background:#ef5454; }
.blue { background:#2f7cff; }
.empty-state { height:100%; display:flex; flex-direction:column; align-items:center; justify-content:center; text-align:center; color:#6b7d96; }
.empty-state h2 { color:#243553; margin:0 0 8px; }
.topology-map { position:relative; margin-top:42px; }

.host-scope-column { position:absolute; left:16px; top:14px; width:220px; min-height:980px; z-index:5; pointer-events:auto; border-right:1px solid rgba(47,124,255,.18); padding:10px 12px 0 0; }
.host-scope-column b { display:block; color:#24415f; font-size:15px; }
.host-scope-column small { display:block; color:#7a8aa1; font-size:11px; margin:3px 0 12px; line-height:1.45; }
.host-scope-item { width:196px; border:1px solid rgba(216,226,246,.92); border-radius:16px; background:rgba(255,255,255,.72); color:#243553; text-align:left; padding:11px 12px; margin-bottom:10px; cursor:pointer; box-shadow:0 12px 26px rgba(68,104,160,.12); transition:.18s ease; }
.host-scope-item:hover,.host-scope-item.active { transform:translateY(-2px); border-color:rgba(47,124,255,.55); box-shadow:0 16px 34px rgba(47,124,255,.18); }
.host-scope-item strong,.host-scope-item span,.host-scope-item em { display:block; }
.host-scope-item strong { font-size:13px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.host-scope-item span { color:#4d75a5; font-size:12px; margin-top:4px; }
.host-scope-item em { color:#7c8da6; font-size:10px; font-style:normal; margin-top:7px; }

.layer-column { position:absolute; top:14px; width:204px; min-height:980px; border-left:1px dashed rgba(47,124,255,.22); background:linear-gradient(90deg, rgba(47,124,255,.05), rgba(255,255,255,0)); z-index:0; pointer-events:none; }
.layer-column b { display:block; color:#24415f; font-size:13px; padding-left:10px; }
.layer-column small { display:block; color:#7a8aa1; font-size:11px; padding-left:10px; margin-top:2px; }
.edge-layer { position:absolute; inset:0; width:100%; height:100%; z-index:1; }
.edge-line { fill:none; stroke:#9aa8bc; stroke-width:2.4; stroke-dasharray:7 5; opacity:.72; }
.edge-line.business { stroke-width:3; opacity:.62; }
.edge-line.connected { stroke:#25bd71; stroke-dasharray:none; }
.edge-line.disconnected { stroke:#ef5454; stroke-dasharray:8 6; }
.edge-line.management { stroke:#7da7ff; stroke-width:2; stroke-dasharray:4 8; opacity:.35; }
.edge-text { display:none; }
.edge-text.connected { fill:#15965a; }
.edge-text.disconnected { fill:#d94343; }
.topo-node { position:absolute; z-index:3; width:176px; min-height:82px; border-radius:18px; padding:12px; background:rgba(255,255,255,.84); border:1px solid rgba(255,255,255,.92); box-shadow:0 16px 34px rgba(68,104,160,.18); cursor:pointer; transition:.18s ease; }
.topo-node:hover,.topo-node.selected { transform: translateY(-2px); box-shadow:0 18px 40px rgba(68,104,160,.24); }
.topo-node.selected { outline:2px solid rgba(47,124,255,.35); }
.topo-node strong { display:block; font-size:14px; margin-bottom:6px; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
.topo-node span { display:block; font-size:11px; color:#6b7d96; }
.topo-node em { display:inline-block; margin-top:8px; font-size:10px; border-radius:999px; padding:3px 8px; background:#edf3ff; color:#6a7b95; font-style:normal; }
.topo-node.online,.topo-node.connected { border-color:rgba(37,189,113,.45); }
.topo-node.offline,.topo-node.disconnected { border-color:rgba(239,84,84,.45); }
.topo-node.target { border-color:rgba(47,124,255,.42); }
.topo-node.ai_agent { background:linear-gradient(145deg, rgba(28,49,86,.94), rgba(47,124,255,.82)); color:#fff; }
.topo-node.ai_agent span,.topo-node.ai_agent em { color:#dce8ff; }
.topo-node.catpaw_agent { background:rgba(232,241,255,.92); border-color:rgba(47,124,255,.45); }
.discovery-card { margin-top:14px; border-radius:16px; padding:12px; background:rgba(255,255,255,.62); border:1px solid rgba(216,226,246,.9); }
.discovery-card h4 { margin:0 0 8px; color:#243553; }
.discovery-card p { margin:0 0 8px; line-height:1.55; }
.discovery-card ul { margin:8px 0 0; padding-left:16px; }
.discovery-card small { display:block; margin-top:8px; color:#d97706; word-break:break-all; }
.detail-panel { font-size:13px; color:#5f718c; }
.detail-panel strong { display:block; color:#243553; font-size:16px; margin-bottom:6px; }
.detail-panel ul { padding-left:16px; line-height:1.9; }
.edge-detail { margin-top:9px; border-radius:14px; background:rgba(255,255,255,.58); padding:9px; color:#33445e; }
.edge-detail small { display:block; color:#7b8aa2; margin-top:4px; word-break:break-all; }
.line-state { margin-right:6px; vertical-align:middle; }
.inspection-mini { margin:10px 0; border-radius:14px; background:rgba(47,124,255,.08); padding:10px; display:flex; flex-direction:column; gap:4px; color:#41607e; }

.md { line-height:1.8; color:#2c3e50; max-width:100%; }
.md h1 { font-size:26px; margin:20px 0 16px; color:#1a2332; border-bottom:3px solid #2f7cff; padding-bottom:10px; }
.md h2 { font-size:20px; margin:24px 0 14px; color:#243553; background:linear-gradient(135deg, rgba(47,124,255,.08), rgba(47,124,255,.02)); padding:10px 14px; border-left:4px solid #2f7cff; border-radius:6px; }
.md h3 { font-size:17px; margin:18px 0 12px; color:#34506f; padding-left:12px; border-left:3px solid #7da7ff; }
.md p { margin:10px 0; line-height:1.9; }
.md ul { margin:10px 0; padding-left:28px; }
.md li { margin:8px 0; line-height:1.8; position:relative; }
.md li::marker { color:#2f7cff; }
.md strong { color:#ef5454; font-weight:600; background:rgba(239,84,84,.08); padding:2px 6px; border-radius:4px; }
.md code { background:rgba(47,124,255,.08); padding:3px 8px; border-radius:4px; font-family:'Consolas','Monaco',monospace; font-size:13px; color:#2f7cff; }
.md pre { background:rgba(47,124,255,.06); padding:14px; border-radius:8px; overflow-x:auto; border:1px solid rgba(47,124,255,.15); }
.md blockquote { border-left:4px solid #2f7cff; padding:10px 14px; margin:14px 0; background:rgba(47,124,255,.04); border-radius:0 6px 6px 0; color:#5e718c; }
.md table { width:100%; border-collapse:collapse; margin:14px 0; }
.md th,.md td { padding:10px; border:1px solid rgba(216,226,246,.9); text-align:left; }
.md th { background:rgba(47,124,255,.08); color:#243553; font-weight:600; }
.md tr:hover { background:rgba(47,124,255,.02); }
.md hr { border:none; border-top:2px solid rgba(216,226,246,.6); margin:20px 0; }
.md em { color:#7da7ff; font-style:normal; }
</style>

