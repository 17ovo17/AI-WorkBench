<template>
  <div class="wf-page">
    <div class="page-head glass-panel">
      <div>
        <div class="panel-kicker">Workflow Manager</div>
        <h2>工作流管理</h2>
        <p class="page-desc">查看、创建和执行自定义工作流</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreate">新建工作流</el-button>
    </div>

    <div class="table-card glass-panel">
      <el-table :data="workflows" v-loading="loading" stripe style="width:100%">
        <el-table-column prop="name" label="名称" min-width="160" />
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            <el-tag :type="row.builtin ? '' : 'success'" size="small">{{ row.builtin ? '内置' : '自定义' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="version" label="版本" width="80" />
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button size="small" link @click="viewDSL(row)">DSL</el-button>
            <el-button size="small" link type="primary" @click="openRun(row)">执行</el-button>
            <el-button size="small" link @click="viewHistory(row)">历史</el-button>
            <el-button v-if="!row.builtin" size="small" link @click="openEdit(row)">编辑</el-button>
            <el-button v-if="!row.builtin" size="small" link type="danger" @click="confirmDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- DSL 查看 -->
    <el-dialog v-model="dslVisible" title="工作流 DSL" width="860px" destroy-on-close>
      <el-tabs v-model="dslTab">
        <el-tab-pane label="源码" name="source">
          <pre class="dsl-block">{{ dslContent }}</pre>
        </el-tab-pane>
        <el-tab-pane label="可视化" name="visual">
          <WorkflowCanvas :dsl="dslRaw" />
        </el-tab-pane>
      </el-tabs>
    </el-dialog>

    <!-- 创建/编辑 -->
    <el-dialog v-model="editVisible" :title="editForm.id ? '编辑工作流' : '新建工作流'" width="700px" destroy-on-close>
      <el-form :model="editForm" label-width="80px">
        <el-form-item label="名称"><el-input v-model="editForm.name" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="editForm.description" /></el-form-item>
        <el-form-item label="DSL (YAML)">
          <el-input v-model="editForm.dsl" type="textarea" :rows="18" spellcheck="false" style="font-family:monospace" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveWorkflow">保存</el-button>
      </template>
    </el-dialog>

    <!-- 执行工作流 -->
    <el-dialog v-model="runVisible" title="执行工作流" width="720px" destroy-on-close>
      <el-form :model="runInputs" label-width="100px" v-if="!runStarted">
        <el-form-item v-for="(val, key) in runInputs" :key="key" :label="key">
          <el-input v-model="runInputs[key]" />
        </el-form-item>
        <el-empty v-if="!Object.keys(runInputs).length" description="该工作流无需输入参数" :image-size="60" />
      </el-form>
      <div v-if="runStarted" class="run-progress">
        <div class="run-nodes">
          <div v-for="n in runNodes" :key="n.id" class="rn-card" :class="`status-${n.status}`">
            <div class="rn-card-head">
              <el-icon v-if="n.status === 'running'" class="spin"><Loading /></el-icon>
              <el-icon v-else-if="n.status === 'done'"><CircleCheckFilled /></el-icon>
              <el-icon v-else-if="n.status === 'error'"><CircleCloseFilled /></el-icon>
              <el-icon v-else><Clock /></el-icon>
              <span class="rn-title">{{ n.title || n.id }}</span>
            </div>
            <div class="rn-status-label">{{ statusText(n.status) }}</div>
          </div>
        </div>
        <div v-if="runResult" class="run-result">
          <div class="section-title">执行结果</div>
          <pre class="result-json">{{ JSON.stringify(runResult, null, 2) }}</pre>
        </div>
      </div>
      <template #footer>
        <el-button @click="runVisible = false">关闭</el-button>
        <el-button v-if="!runStarted" type="primary" :loading="runLoading" @click="executeWorkflow">执行</el-button>
      </template>
    </el-dialog>

    <!-- 执行历史 -->
    <WfHistoryDialog v-model="historyVisible" :workflow-id="historyWfId" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Loading, CircleCheckFilled, CircleCloseFilled, Clock } from '@element-plus/icons-vue'
import WorkflowCanvas from '../components/workflow/WorkflowCanvas.vue'
import WfHistoryDialog from '../components/workflow/WfHistoryDialog.vue'

const API = '/api/v1/workflows'
const workflows = ref([])
const loading = ref(false)

/* --- 列表 --- */
const fetchList = async () => {
  loading.value = true
  try { const res = (await axios.get(API)).data; workflows.value = res.items || res || [] }
  catch (e) { ElMessage.error(e.response?.data?.error || '获取工作流列表失败') }
  finally { loading.value = false }
}
onMounted(fetchList)

/* --- DSL 查看 --- */
const dslVisible = ref(false)
const dslContent = ref('')
const dslRaw = ref(null)
const dslTab = ref('source')
const viewDSL = async (row) => {
  try {
    const { data } = await axios.get(`${API}/${row.id}`)
    dslRaw.value = data.dsl || data
    dslContent.value = typeof data.dsl === 'string' ? data.dsl : JSON.stringify(data.dsl, null, 2)
    dslTab.value = 'source'
    dslVisible.value = true
  } catch (e) { ElMessage.error('获取 DSL 失败') }
}

/* --- 创建 / 编辑 --- */
const editVisible = ref(false)
const saving = ref(false)
const editForm = ref({ id: '', name: '', description: '', dsl: '' })

const openCreate = () => {
  editForm.value = { id: '', name: '', description: '', dsl: '' }
  editVisible.value = true
}
const openEdit = async (row) => {
  try {
    const { data } = await axios.get(`${API}/${row.id}`)
    editForm.value = {
      id: data.id, name: data.name, description: data.description || '',
      dsl: typeof data.dsl === 'string' ? data.dsl : JSON.stringify(data.dsl, null, 2)
    }
    editVisible.value = true
  } catch (e) { ElMessage.error('获取工作流详情失败') }
}
const saveWorkflow = async () => {
  if (!editForm.value.name.trim()) { ElMessage.warning('请输入名称'); return }
  saving.value = true
  try {
    const payload = { name: editForm.value.name, description: editForm.value.description, dsl: editForm.value.dsl }
    if (editForm.value.id) await axios.put(`${API}/${editForm.value.id}`, payload)
    else await axios.post(API, payload)
    ElMessage.success('保存成功')
    editVisible.value = false
    fetchList()
  } catch (e) { ElMessage.error(e.response?.data?.error || '保存失败') }
  finally { saving.value = false }
}

/* --- 删除 --- */
const confirmDelete = (row) => {
  ElMessageBox.confirm(`确定删除工作流「${row.name}」？`, '删除确认', { type: 'warning' })
    .then(async () => {
      try { await axios.delete(`${API}/${row.id}`); ElMessage.success('已删除'); fetchList() }
      catch (e) { ElMessage.error(e.response?.data?.error || '删除失败') }
    }).catch(() => {})
}

/* --- 执行工作流 --- */
const runVisible = ref(false)
const runLoading = ref(false)
const runStarted = ref(false)
const runInputs = ref({})
const runNodes = ref([])
const runResult = ref(null)
let currentRunId = ''

const openRun = async (row) => {
  currentRunId = row.id
  runStarted.value = false
  runLoading.value = false
  runNodes.value = []
  runResult.value = null
  try {
    const { data } = await axios.get(`${API}/${row.id}`)
    const inputs = data.dsl?.start?.inputs || data.inputs || {}
    runInputs.value = Object.fromEntries(Object.keys(inputs).map(k => [k, '']))
  } catch { runInputs.value = {} }
  runVisible.value = true
}

const executeWorkflow = async () => {
  runLoading.value = true
  runStarted.value = true
  const token = localStorage.getItem('aiw-token') || ''
  try {
    const resp = await fetch(`${API}/${currentRunId}/stream`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
      body: JSON.stringify({ inputs: runInputs.value })
    })
    if (!resp.ok) { ElMessage.error(`执行失败 (${resp.status})`); return }
    const reader = resp.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''
      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        try { handleRunSSE(JSON.parse(line.slice(6))) } catch {}
      }
    }
  } catch (e) { ElMessage.error(e.message || '执行异常') }
  finally { runLoading.value = false }
}

const handleRunSSE = (ev) => {
  const event = ev?.event || ''
  if (event === 'node_started') {
    runNodes.value.push({ id: ev.node_id, title: ev.node_title || ev.node_id, status: 'running' })
  } else if (event === 'node_finished') {
    const n = runNodes.value.find(x => x.id === ev.node_id)
    if (n) n.status = 'done'
  } else if (event === 'node_error') {
    const n = runNodes.value.find(x => x.id === ev.node_id)
    if (n) n.status = 'error'
  } else if (event === 'workflow_finished') {
    runResult.value = ev?.data?.outputs || ev?.data || {}
  } else if (event === 'workflow_failed') {
    ElMessage.error(ev?.data?.error || '工作流执行失败')
  }
}

/* --- 执行历史 --- */
const historyVisible = ref(false)
const historyWfId = ref('')

const viewHistory = (row) => {
  historyWfId.value = row.id
  historyVisible.value = true
}

const formatTs = (ts) => {
  if (!ts) return '-'
  const d = new Date(typeof ts === 'number' ? ts * 1000 : ts)
  return d.toLocaleString('zh-CN')
}

const STATUS_TEXT = { running: '执行中', done: '已完成', error: '失败', pending: '等待中' }
const statusText = (s) => STATUS_TEXT[s] || s || '等待中'
</script>

<style scoped>
.wf-page { padding: 28px 32px; min-height: 100vh; color: #243553; display: flex; flex-direction: column; gap: 18px; }
.glass-panel { background: linear-gradient(145deg, rgba(255,255,255,.58), rgba(225,236,255,.42)); border: 1px solid rgba(255,255,255,.72); border-radius: 24px; box-shadow: 0 20px 54px rgba(63,100,160,.16), inset 0 1px 0 rgba(255,255,255,.78); backdrop-filter: blur(24px); }
.page-head { padding: 20px 26px; display: flex; justify-content: space-between; align-items: center; }
.page-head h2 { margin: 6px 0 4px; font-size: 26px; letter-spacing: -.03em; color: #263653; }
.page-desc { font-size: 13px; color: var(--muted); }
.panel-kicker { font-size: 12px; color: #247cff; font-weight: 800; letter-spacing: .06em; text-transform: uppercase; }
.table-card { padding: 22px 26px; }
.section-title { font-size: 13px; color: #243553; font-weight: 800; margin-bottom: 8px; }
.dsl-block { background: rgba(0,0,0,.04); border-radius: 12px; padding: 16px; font-size: 12px; font-family: ui-monospace, monospace; white-space: pre-wrap; word-break: break-all; max-height: 480px; overflow: auto; }
.run-progress { display: flex; flex-direction: column; gap: 14px; }
.run-nodes { display: flex; flex-wrap: wrap; gap: 10px; }
.rn-card { display: flex; flex-direction: column; gap: 4px; padding: 12px 16px; border-radius: 12px; border: 1px solid rgba(255,255,255,.7); background: rgba(255,255,255,.42); min-width: 140px; }
.rn-card-head { display: flex; align-items: center; gap: 6px; }
.rn-card-head .el-icon { font-size: 18px; }
.rn-title { font-size: 13px; font-weight: 600; color: #243553; }
.rn-status-label { font-size: 11px; color: #98a3b8; padding-left: 24px; }
.status-running { border-color: rgba(47,124,255,.55); }
.status-running .el-icon { color: #247cff; }
.status-done { border-color: rgba(54,208,138,.55); }
.status-done .el-icon { color: #36b07a; }
.status-error { border-color: rgba(255,91,107,.55); background: rgba(255,236,238,.6); }
.status-error .el-icon { color: #ff5b6b; }
.spin { animation: spin 1.1s linear infinite; }
@keyframes spin { from { transform: rotate(0); } to { transform: rotate(360deg); } }
.result-json { background: rgba(0,0,0,.04); border-radius: 12px; padding: 14px; font-size: 12px; font-family: ui-monospace, monospace; white-space: pre-wrap; word-break: break-all; max-height: 360px; overflow: auto; }
</style>
