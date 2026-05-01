<template>
  <div class="kb-page">
    <div class="page-head glass-panel">
      <div>
        <div class="panel-kicker">Knowledge Base</div>
        <h2>知识库管理</h2>
        <p class="page-desc">维护诊断案例库、文档管理与语义搜索</p>
      </div>
      <div class="head-actions">
        <template v-if="activeTab === 'cases'">
          <el-button :icon="Upload" @click="triggerImport">导入</el-button>
          <el-button :icon="Download" :loading="exporting" @click="exportAll">导出</el-button>
          <el-button type="primary" :icon="Plus" @click="openCreate">新建案例</el-button>
          <input ref="fileInput" type="file" accept=".json,application/json" hidden @change="handleImport" />
        </template>
      </div>
    </div>

    <div class="tabs-wrap glass-panel">
      <el-tabs v-model="activeTab" class="kb-tabs">
        <el-tab-pane label="案例库" name="cases">
          <CaseTable
            :cases="cases" :total="total" :loading="loading"
            v-model:page="page" v-model:limit="limit" v-model:keyword="keyword" v-model:category="category"
            @reload="reload" @detail="openDetail" @edit="openEdit" @remove="removeOne"
          />
        </el-tab-pane>
        <el-tab-pane label="文档管理" name="docs">
          <DocManager />
        </el-tab-pane>
        <el-tab-pane label="语义搜索" name="search">
          <SemanticSearch />
        </el-tab-pane>
      </el-tabs>
    </div>

    <CaseDetailDialog v-model="detailVisible" :data="detailData" />
    <CaseEditDialog v-model="editVisible" :editing="editing" @saved="reload" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Upload, Download, Plus } from '@element-plus/icons-vue'
import CaseDetailDialog from '../components/CaseDetailDialog.vue'
import CaseEditDialog from '../components/CaseEditDialog.vue'
import CaseTable from '../components/knowledge/CaseTable.vue'
import DocManager from '../components/knowledge/DocManager.vue'
import SemanticSearch from '../components/knowledge/SemanticSearch.vue'

const activeTab = ref('cases')
const cases = ref([])
const total = ref(0)
const page = ref(1)
const limit = ref(20)
const keyword = ref('')
const category = ref('')
const loading = ref(false)
const exporting = ref(false)

const detailVisible = ref(false)
const detailData = ref(null)
const editVisible = ref(false)
const editing = ref(null)
const fileInput = ref(null)

const reload = async () => {
  loading.value = true
  try {
    const { data } = await axios.get('/api/v1/knowledge/cases', {
      params: { page: page.value, limit: limit.value, keyword: keyword.value || undefined, category: category.value || undefined }
    })
    cases.value = Array.isArray(data?.items) ? data.items : (Array.isArray(data) ? data : [])
    total.value = Number(data?.total ?? cases.value.length) || 0
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '加载案例失败')
  } finally {
    loading.value = false
  }
}

const openDetail = async (row) => {
  try {
    const { data } = await axios.get(`/api/v1/knowledge/cases/${row.id}`)
    detailData.value = data || row
  } catch { detailData.value = row }
  detailVisible.value = true
}
const openCreate = () => { editing.value = null; editVisible.value = true }
const openEdit = (row) => { editing.value = row; editVisible.value = true }

const removeOne = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除案例「${row.root_cause_description?.slice(0, 24) || row.id}」？`, '二次确认', { type: 'warning' })
  } catch { return }
  try {
    await axios.delete(`/api/v1/knowledge/cases/${row.id}`)
    ElMessage.success('已删除')
    reload()
  } catch (e) { ElMessage.error(e.response?.data?.error || '删除失败') }
}

const triggerImport = () => fileInput.value?.click()

const handleImport = async (ev) => {
  const file = ev.target.files?.[0]
  ev.target.value = ''
  if (!file) return
  let parsed
  try {
    const text = await file.text()
    parsed = JSON.parse(text)
    if (!Array.isArray(parsed)) throw new Error('文件需为 JSON 数组')
  } catch (e) { ElMessage.error(`解析失败：${e.message}`); return }
  try {
    const { data } = await axios.post('/api/v1/knowledge/cases/import', parsed)
    ElMessage.success(`导入完成，共处理 ${data?.imported ?? parsed.length} 条`)
    reload()
  } catch (e) { ElMessage.error(e.response?.data?.error || '导入失败') }
}

const exportAll = async () => {
  exporting.value = true
  try {
    const { data } = await axios.get('/api/v1/knowledge/cases/export')
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = `knowledge-cases-${Date.now()}.json`; a.click()
    URL.revokeObjectURL(url)
  } catch (e) { ElMessage.error(e.response?.data?.error || '导出失败') }
  finally { exporting.value = false }
}

onMounted(reload)
</script>

<style scoped>
.kb-page { padding: 28px 32px; min-height: 100vh; color: #243553; display: flex; flex-direction: column; gap: 18px; }
.glass-panel { background: linear-gradient(145deg, rgba(255,255,255,.58), rgba(225,236,255,.42)); border: 1px solid rgba(255,255,255,.72); border-radius: 24px; box-shadow: 0 20px 54px rgba(63,100,160,.16), inset 0 1px 0 rgba(255,255,255,.78); backdrop-filter: blur(24px); }
.page-head { padding: 20px 26px; display: flex; align-items: center; justify-content: space-between; gap: 16px; flex-wrap: wrap; }
.page-head h2 { margin: 6px 0 4px; font-size: 26px; letter-spacing: -.03em; color: #263653; }
.page-desc { font-size: 13px; color: var(--muted); }
.panel-kicker { font-size: 12px; color: #247cff; text-transform: uppercase; letter-spacing: .06em; font-weight: 800; }
.head-actions { display: flex; gap: 10px; flex-wrap: wrap; }
.tabs-wrap { padding: 16px 20px 8px; }
.kb-tabs :deep(.el-tabs__header) { margin-bottom: 16px; }
</style>
