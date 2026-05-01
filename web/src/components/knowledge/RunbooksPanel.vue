<template>
  <div class="rb-panel">
    <div class="filter-bar">
      <el-select v-model="category" placeholder="按分类筛选" clearable style="width:200px" @change="reload">
        <el-option v-for="c in CATEGORIES" :key="c" :label="c" :value="c" />
      </el-select>
      <el-button @click="reload">查询</el-button>
      <span class="result-count">共 {{ total }} 条</span>
    </div>

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
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <el-button size="small" link @click="openDetail(row)">查看</el-button>
          <el-button size="small" link @click="openEdit(row)">编辑</el-button>
          <el-button size="small" link type="danger" @click="removeOne(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination
      v-model:current-page="page" v-model:page-size="limit" :total="total"
      :page-sizes="[10, 20, 50]" layout="total, sizes, prev, pager, next"
      background @current-change="reload" @size-change="reload"
      style="margin-top:16px;justify-content:flex-end"
    />

    <!-- 查看对话框（含执行和历史标签页） -->
    <el-dialog v-model="detailVisible" :title="detailData?.title || '查看 Runbook'" width="780px" top="6vh">
      <el-tabs v-if="detailData" v-model="detailTab">
        <el-tab-pane label="详情" name="info">
          <RunbookDetailInfo :data="detailData" />
        </el-tab-pane>
        <el-tab-pane label="执行" name="execute">
          <RunbookExecuteDialog v-model="executeInline" :runbook="detailData" inline />
        </el-tab-pane>
        <el-tab-pane label="历史" name="history">
          <RunbookHistoryDialog v-model="historyInline" :runbook-id="detailData?.id" inline />
        </el-tab-pane>
      </el-tabs>
    </el-dialog>

    <!-- 编辑对话框 -->
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
          <el-input v-model="editForm.steps" type="textarea" :rows="10" />
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
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CATEGORIES, categoryType } from '../caseHelpers.js'
import RunbookExecuteDialog from '../runbook/RunbookExecuteDialog.vue'
import RunbookHistoryDialog from '../runbook/RunbookHistoryDialog.vue'
import RunbookDetailInfo from './RunbookDetailInfo.vue'

const SEVERITY_COLORS = { critical: 'danger', high: 'warning', medium: '', low: 'success' }
const severityColor = (s) => SEVERITY_COLORS[s] ?? 'info'

const runbooks = ref([])
const total = ref(0)
const page = ref(1)
const limit = ref(20)
const category = ref('')
const loading = ref(false)
const saving = ref(false)

const detailVisible = ref(false)
const detailData = ref(null)
const detailTab = ref('info')
const executeInline = ref(true)
const historyInline = ref(true)

const editVisible = ref(false)
const editing = ref(null)
const editForm = ref(emptyForm())

function emptyForm() {
  return { title: '', category: '', trigger_conditions: '', steps: '', auto_executable: false }
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
  } catch { detailData.value = row }
  detailTab.value = 'info'
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
      ? row.trigger_conditions : JSON.stringify(row.trigger_conditions || {}),
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
      title: editForm.value.title, category: editForm.value.category,
      trigger_conditions: editForm.value.trigger_conditions || '{}',
      steps: editForm.value.steps, auto_executable: editForm.value.auto_executable,
    }
    if (editing.value?.id) await axios.put(`/api/v1/knowledge/runbooks/${editing.value.id}`, payload)
    else await axios.post('/api/v1/knowledge/runbooks', payload)
    ElMessage.success('保存成功')
    editVisible.value = false
    await reload()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '保存失败')
  } finally { saving.value = false }
}

const removeOne = async (row) => {
  try { await ElMessageBox.confirm(`确认删除「${row.title}」？`, '二次确认', { type: 'warning' }) }
  catch { return }
  try {
    await axios.delete(`/api/v1/knowledge/runbooks/${row.id}`)
    ElMessage.success('已删除')
    reload()
  } catch (e) { ElMessage.error(e.response?.data?.error || '删除失败') }
}

defineExpose({ openCreate })
onMounted(reload)
</script>

<style scoped>
.rb-panel { display: flex; flex-direction: column; gap: 12px; }
.filter-bar { display: flex; align-items: center; gap: 12px; margin-bottom: 8px; }
.result-count { color: var(--muted); font-size: 12px; margin-left: auto; }
:deep(.el-table) { background: transparent; }
:deep(.el-table tr), :deep(.el-table th.el-table__cell) { background: transparent !important; }
</style>
