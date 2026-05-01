<template>
  <div class="rb-page">
    <div class="page-head glass-panel">
      <div>
        <div class="panel-kicker">Runbooks</div>
        <h2>运维手册</h2>
        <p class="page-desc">标准化故障处置流程（SOP），用于诊断后的快速响应</p>
      </div>
      <div class="head-actions">
        <el-button type="primary" :icon="Plus" @click="openCreate">新建 Runbook</el-button>
      </div>
    </div>

    <div class="filter-bar glass-panel">
      <el-select v-model="category" placeholder="按分类筛选" clearable style="width:200px" @change="reload">
        <el-option v-for="c in CATEGORIES" :key="c" :label="c" :value="c" />
      </el-select>
      <el-button @click="reload">查询</el-button>
      <span class="result-count">共 {{ total }} 条</span>
    </div>

    <div class="table-wrap glass-panel">
      <el-table :data="runbooks" v-loading="loading" stripe style="width:100%" empty-text="暂无 Runbook">
        <el-table-column prop="title" label="标题" min-width="200" show-overflow-tooltip />
        <el-table-column label="分类" width="140">
          <template #default="{ row }">
            <el-tag :type="categoryType(row.category)" size="small">{{ row.category }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="version" label="版本" width="70" align="center" />
        <el-table-column label="严重级别" width="100" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.severity" :type="severityColor(row.severity)" size="small">{{ row.severity }}</el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="预计时间" width="90" align="center">
          <template #default="{ row }">{{ row.estimated_time || '-' }}</template>
        </el-table-column>
        <el-table-column label="执行次数" width="80" align="center">
          <template #default="{ row }">{{ row.execution_count ?? 0 }}</template>
        </el-table-column>
        <el-table-column label="成功率" width="80" align="center">
          <template #default="{ row }">{{ row.success_rate != null ? `${(row.success_rate * 100).toFixed(0)}%` : '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <el-button size="small" link @click="openDetail(row)">查看</el-button>
            <el-button size="small" link type="success" @click="openExecute(row)">执行</el-button>
            <el-button size="small" link @click="openHistory(row)">历史</el-button>
            <el-button size="small" link @click="openEdit(row)">编辑</el-button>
            <el-button size="small" link type="danger" @click="removeOne(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="limit"
        :total="total"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next, jumper"
        background
        @current-change="reload"
        @size-change="reload"
        style="margin-top:16px;justify-content:flex-end"
      />
    </div>

    <el-dialog v-model="detailVisible" :title="detailData?.title || '查看 Runbook'" width="780px" top="6vh">
      <div v-if="detailData" class="detail">
        <div class="detail-row"><span class="lbl">分类：</span><el-tag size="small">{{ detailData.category }}</el-tag></div>
        <div v-if="detailData.version" class="detail-row"><span class="lbl">版本：</span>{{ detailData.version }}</div>
        <div v-if="detailData.severity" class="detail-row">
          <span class="lbl">严重级别：</span>
          <el-tag :type="severityColor(detailData.severity)" size="small">{{ detailData.severity }}</el-tag>
        </div>
        <div v-if="detailData.estimated_time" class="detail-row"><span class="lbl">预计时间：</span>{{ detailData.estimated_time }}</div>
        <div class="detail-row"><span class="lbl">触发条件：</span><pre class="json-pre">{{ detailData.trigger_conditions }}</pre></div>
        <div v-if="detailData.prerequisites" class="detail-row">
          <span class="lbl">前置条件：</span>
          <pre class="md-pre">{{ detailData.prerequisites }}</pre>
        </div>
        <div class="detail-row"><span class="lbl">处置步骤：</span></div>
        <pre class="md-pre">{{ detailData.steps }}</pre>
        <div v-if="detailData.rollback_steps" class="detail-row">
          <span class="lbl">回滚步骤：</span>
          <pre class="md-pre rollback-pre">{{ detailData.rollback_steps }}</pre>
        </div>
      </div>
    </el-dialog>

    <el-dialog v-model="editVisible" :title="editing?.id ? '编辑 Runbook' : '新建 Runbook'" width="720px" top="6vh">
      <el-form :model="editForm" label-width="120px">
        <el-form-item label="标题" required>
          <el-input v-model="editForm.title" />
        </el-form-item>
        <el-form-item label="分类" required>
          <el-select v-model="editForm.category" filterable allow-create>
            <el-option v-for="c in CATEGORIES" :key="c" :label="c" :value="c" />
          </el-select>
        </el-form-item>
        <el-form-item label="触发条件 (JSON)">
          <el-input v-model="editForm.trigger_conditions" type="textarea" :rows="3"
            placeholder='{"metric":"cpu_usage_active","operator":">","threshold":90}' />
        </el-form-item>
        <el-form-item label="处置步骤 (Markdown)" required>
          <el-input v-model="editForm.steps" type="textarea" :rows="12" />
        </el-form-item>
        <el-form-item label="可自动执行">
          <el-switch v-model="editForm.auto_executable" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveEdit">保存</el-button>
      </template>
    </el-dialog>

    <RunbookExecuteDialog v-model="executeVisible" :runbook="executeTarget" />
    <RunbookHistoryDialog v-model="historyVisible" :runbook-id="historyTargetId" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { CATEGORIES, categoryType } from '../components/caseHelpers.js'
import RunbookExecuteDialog from '../components/runbook/RunbookExecuteDialog.vue'
import RunbookHistoryDialog from '../components/runbook/RunbookHistoryDialog.vue'

const SEVERITY_COLORS = { critical: 'danger', high: 'warning', medium: '', low: 'success' }
const severityColor = (s) => SEVERITY_COLORS[s] ?? 'info'

const runbooks = ref([])
const total = ref(0)
const page = ref(1)
const limit = ref(20)
const category = ref('')
const loading = ref(false)

const detailVisible = ref(false)
const detailData = ref(null)
const editVisible = ref(false)
const editing = ref(null)
const editForm = ref(emptyForm())
const saving = ref(false)

const executeVisible = ref(false)
const executeTarget = ref(null)
const historyVisible = ref(false)
const historyTargetId = ref('')

function emptyForm() {
  return { title: '', category: '', trigger_conditions: '', steps: '', auto_executable: false }
}

const formatTime = (iso) => {
  if (!iso) return '-'
  try { return new Date(iso).toLocaleString('zh-CN', { hour12: false }) } catch { return iso }
}

const reload = async () => {
  loading.value = true
  try {
    const { data } = await axios.get('/api/v1/knowledge/runbooks', {
      params: { page: page.value, limit: limit.value, category: category.value || undefined }
    })
    runbooks.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '加载失败')
  } finally {
    loading.value = false
  }
}

const openDetail = async (row) => {
  try {
    const { data } = await axios.get(`/api/v1/knowledge/runbooks/${row.id}`)
    detailData.value = data || row
  } catch {
    detailData.value = row
  }
  detailVisible.value = true
}

const openCreate = () => {
  editing.value = null
  editForm.value = emptyForm()
  editVisible.value = true
}

const openEdit = (row) => {
  editing.value = row
  editForm.value = {
    title: row.title || '',
    category: row.category || '',
    trigger_conditions: typeof row.trigger_conditions === 'string'
      ? row.trigger_conditions
      : JSON.stringify(row.trigger_conditions || {}),
    steps: row.steps || '',
    auto_executable: !!row.auto_executable,
  }
  editVisible.value = true
}

const saveEdit = async () => {
  if (!editForm.value.title || !editForm.value.category || !editForm.value.steps) {
    ElMessage.warning('标题、分类、步骤为必填')
    return
  }
  saving.value = true
  try {
    const payload = {
      title: editForm.value.title,
      category: editForm.value.category,
      trigger_conditions: editForm.value.trigger_conditions || '{}',
      steps: editForm.value.steps,
      auto_executable: editForm.value.auto_executable,
    }
    if (editing.value?.id) {
      await axios.put(`/api/v1/knowledge/runbooks/${editing.value.id}`, payload)
    } else {
      await axios.post('/api/v1/knowledge/runbooks', payload)
    }
    ElMessage.success('保存成功')
    editVisible.value = false
    await reload()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}

const removeOne = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除「${row.title}」？`, '二次确认', { type: 'warning' })
  } catch { return }
  try {
    await axios.delete(`/api/v1/knowledge/runbooks/${row.id}`)
    ElMessage.success('已删除')
    reload()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '删除失败')
  }
}

const openExecute = (row) => {
  executeTarget.value = row
  executeVisible.value = true
}

const openHistory = (row) => {
  historyTargetId.value = row.id
  historyVisible.value = true
}

onMounted(reload)
</script>

<style scoped>
.rb-page { padding: 28px 32px; min-height: 100vh; color: var(--ink); display: flex; flex-direction: column; gap: 18px; }
.glass-panel { background: linear-gradient(145deg, rgba(255,255,255,.58), rgba(225,236,255,.42)); border: 1px solid rgba(255,255,255,.72); border-radius: 24px; box-shadow: 0 20px 54px rgba(63,100,160,.16), inset 0 1px 0 rgba(255,255,255,.78); backdrop-filter: blur(24px); }
.page-head { padding: 20px 26px; display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.page-head h2 { margin: 6px 0 4px; font-size: 24px; letter-spacing: -.03em; }
.page-desc { font-size: 13px; color: var(--muted); }
.panel-kicker { font-size: 12px; color: #247cff; text-transform: uppercase; letter-spacing: .06em; font-weight: 800; }
.head-actions { display: flex; gap: 10px; }
.filter-bar { padding: 14px 20px; display: flex; align-items: center; gap: 12px; }
.result-count { color: var(--muted); font-size: 12px; margin-left: auto; }
.table-wrap { padding: 16px 20px; }
.detail { padding: 4px 0; }
.detail-row { margin-bottom: 12px; }
.lbl { font-weight: 700; color: var(--ink); margin-right: 8px; }
.json-pre { background: rgba(255,255,255,.5); border-radius: 12px; padding: 10px 14px; font-size: 12px; white-space: pre-wrap; word-break: break-all; margin: 6px 0 0; }
.md-pre { background: rgba(255,255,255,.5); border-radius: 12px; padding: 14px 18px; font-size: 12.5px; line-height: 1.65; white-space: pre-wrap; max-height: 50vh; overflow-y: auto; margin: 6px 0 0; }
.rollback-pre { border-left: 3px solid #ff9f43; }
:deep(.el-table) { background: transparent; }
:deep(.el-table tr), :deep(.el-table th.el-table__cell) { background: transparent !important; }
</style>
