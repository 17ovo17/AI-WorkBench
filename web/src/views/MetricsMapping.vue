<template>
  <div class="mm-page">
    <div class="page-head glass-panel">
      <div>
        <div class="panel-kicker">Metrics Mapping</div>
        <h2>指标映射管理</h2>
        <p class="page-desc">扫描 Prometheus 指标 → AI 自动适配 → 用户确认 → 同步到知识库</p>
      </div>
      <div class="head-actions">
        <el-button :icon="Refresh" :loading="scanning" @click="scanMetrics">扫描指标</el-button>
        <el-button type="primary" :icon="MagicStick" :loading="adapting" @click="autoAdapt">AI 自动适配</el-button>
        <el-button :icon="Check" :loading="confirming" @click="confirmAuto">批量确认 AI 结果</el-button>
      </div>
    </div>

    <div class="filter-bar glass-panel">
      <el-select v-model="datasourceId" placeholder="选择数据源" style="width:240px" @change="reload">
        <el-option v-for="ds in datasources" :key="ds.id" :label="ds.name" :value="ds.id" />
      </el-select>
      <el-select v-model="status" placeholder="按状态筛选" clearable style="width:180px" @change="reload">
        <el-option label="未适配" value="unmapped" />
        <el-option label="AI 自动适配" value="auto" />
        <el-option label="已确认" value="confirmed" />
        <el-option label="自定义" value="custom" />
      </el-select>
      <el-button @click="reload">查询</el-button>
      <span class="result-count">共 {{ total }} 条</span>
    </div>

    <div class="table-wrap glass-panel">
      <el-table :data="mappings" :max-height="tableMaxHeight" v-loading="loading" stripe style="width:100%" empty-text="暂无数据，请先点击扫描指标">
        <el-table-column prop="raw_name" label="原始指标名" min-width="220" show-overflow-tooltip />
        <el-table-column prop="standard_name" label="标准名" min-width="200">
          <template #default="{ row }">
            <span v-if="row.standard_name" class="std-name">{{ row.standard_name }}</span>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column prop="exporter" label="来源" width="140">
          <template #default="{ row }">
            <el-tag v-if="row.exporter" :type="exporterType(row.exporter)" size="small" effect="plain">{{ row.exporter }}</el-tag>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button size="small" link type="primary" @click="openEdit(row)">编辑</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination
        background
        layout="total, sizes, prev, pager, next, jumper"
        :total="total"
        v-model:current-page="page"
        v-model:page-size="limit"
        :page-sizes="[20, 50, 100, 200]"
        @current-change="reload"
        @size-change="reload"
        style="margin-top:16px;justify-content:flex-end"
      />
    </div>

    <el-dialog v-model="editVisible" title="编辑指标映射" width="560px">
      <el-form :model="editing" label-width="100px">
        <el-form-item label="原始名">
          <el-input v-model="editing.raw_name" disabled />
        </el-form-item>
        <el-form-item label="标准名">
          <el-input v-model="editing.standard_name" placeholder="格式：domain.resource.metric，如 host.cpu.usage" />
        </el-form-item>
        <el-form-item label="Exporter">
          <el-select v-model="editing.exporter" placeholder="选择来源">
            <el-option label="categraf" value="categraf" />
            <el-option label="node_exporter" value="node_exporter" />
            <el-option label="mysqld_exporter" value="mysqld_exporter" />
            <el-option label="redis_exporter" value="redis_exporter" />
            <el-option label="oracledb_exporter" value="oracledb_exporter" />
            <el-option label="其他" value="other" />
          </el-select>
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="editing.description" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="PromQL">
          <el-input v-model="editing.transform" placeholder="支持 {ip} 占位符" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveEdit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, MagicStick, Check } from '@element-plus/icons-vue'

const datasources = ref([])
const datasourceId = ref('')
const status = ref('')
const mappings = ref([])
const total = ref(0)
const page = ref(1)
const limit = ref(50)
const loading = ref(false)
const scanning = ref(false)
const adapting = ref(false)
const confirming = ref(false)

const editVisible = ref(false)
const editing = ref({})
const saving = ref(false)

const STATUS_LABELS = { unmapped: '未适配', auto: 'AI 适配', confirmed: '已确认', custom: '自定义' }
const STATUS_TYPES = { unmapped: 'info', auto: 'warning', confirmed: 'success', custom: 'primary' }
const EXPORTER_TYPES = { categraf: 'success', node_exporter: 'primary', mysqld_exporter: 'warning', redis_exporter: 'danger' }

const statusLabel = (s) => STATUS_LABELS[s] || s
const statusType = (s) => STATUS_TYPES[s] || 'info'
const exporterType = (e) => EXPORTER_TYPES[e] || 'info'

const loadDatasources = async () => {
  try {
    const { data } = await axios.get('/api/v1/data-sources')
    datasources.value = (data || []).filter(d => d.type === 'prometheus')
    if (!datasourceId.value && datasources.value.length > 0) {
      datasourceId.value = datasources.value[0].id
    }
  } catch (e) {
    ElMessage.error('加载数据源失败：' + (e.response?.data?.error || e.message))
  }
}

const reload = async () => {
  if (!datasourceId.value) return
  loading.value = true
  try {
    const { data } = await axios.get('/api/v1/metrics/mappings', {
      params: { datasource_id: datasourceId.value, status: status.value, page: page.value, limit: limit.value }
    })
    mappings.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    ElMessage.error('加载映射失败：' + (e.response?.data?.error || e.message))
  } finally {
    loading.value = false
  }
}

const scanMetrics = async () => {
  if (!datasourceId.value) {
    ElMessage.warning('请先选择数据源')
    return
  }
  scanning.value = true
  try {
    const { data } = await axios.post('/api/v1/metrics/scan', { datasource_id: datasourceId.value })
    ElMessage.success(`扫描完成，新增 ${data.added || 0} 条指标`)
    await reload()
  } catch (e) {
    ElMessage.error('扫描失败：' + (e.response?.data?.error || e.message))
  } finally {
    scanning.value = false
  }
}

const autoAdapt = async () => {
  if (!datasourceId.value) {
    ElMessage.warning('请先选择数据源')
    return
  }
  adapting.value = true
  try {
    const { data } = await axios.post('/api/v1/metrics/auto-adapt', { datasource_id: datasourceId.value, max_batches: 5 })
    ElMessage.success(`AI 适配完成：处理 ${data.processed || 0} 条，成功适配 ${data.adapted || 0} 条`)
    await reload()
  } catch (e) {
    ElMessage.error('AI 适配失败：' + (e.response?.data?.error || e.message))
  } finally {
    adapting.value = false
  }
}

const confirmAuto = async () => {
  if (!datasourceId.value) return
  try {
    await ElMessageBox.confirm('确认将所有 AI 自动适配的映射标记为「已确认」？', '批量确认', { type: 'warning' })
  } catch { return }
  confirming.value = true
  try {
    const { data } = await axios.post('/api/v1/metrics/mappings/confirm', { datasource_id: datasourceId.value })
    ElMessage.success(`已确认 ${data.confirmed || 0} 条`)
    await reload()
  } catch (e) {
    ElMessage.error('确认失败：' + (e.response?.data?.error || e.message))
  } finally {
    confirming.value = false
  }
}

const openEdit = (row) => {
  editing.value = { ...row }
  editVisible.value = true
}

const saveEdit = async () => {
  saving.value = true
  try {
    await axios.put(`/api/v1/metrics/mappings/${editing.value.id}`, editing.value)
    ElMessage.success('保存成功')
    editVisible.value = false
    await reload()
  } catch (e) {
    ElMessage.error('保存失败：' + (e.response?.data?.error || e.message))
  } finally {
    saving.value = false
  }
}

const tableMaxHeight = ref(500)
const calcTableHeight = () => { tableMaxHeight.value = Math.max(300, window.innerHeight - 340) }

onMounted(async () => {
  calcTableHeight()
  window.addEventListener('resize', calcTableHeight)
  await loadDatasources()
  await reload()
})

onUnmounted(() => { window.removeEventListener('resize', calcTableHeight) })
</script>

<style scoped>
.mm-page { padding: 28px 32px; height: 100%; display: flex; flex-direction: column; gap: 16px; color: var(--ink); }
.page-head { display: flex; align-items: center; justify-content: space-between; padding: 22px 28px; border-radius: 22px; flex-shrink: 0; }
.panel-kicker { font-size: 13px; color: #247cff; text-transform: uppercase; letter-spacing: .06em; font-weight: 800; }
.page-head h2 { margin: 6px 0 4px; font-size: 22px; letter-spacing: -.03em; }
.page-desc { font-size: 13px; color: var(--muted); }
.head-actions { display: flex; gap: 10px; }
.filter-bar { display: flex; align-items: center; gap: 14px; padding: 14px 22px; border-radius: 22px; flex-shrink: 0; }
.result-count { margin-left: auto; font-size: 12px; color: var(--muted); font-weight: 700; }
.table-wrap { padding: 16px 22px 22px; border-radius: 22px; flex: 1; min-height: 0; overflow: hidden; display: flex; flex-direction: column; }
.std-name { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; color: #1672ff; font-weight: 700; font-size: 13px; }
.muted { color: var(--muted); }
:deep(.el-table) { background: transparent; }
:deep(.el-table tr) { background: transparent !important; }
</style>
