<template>
  <div class="doc-manager">
    <div class="toolbar">
      <el-upload
        :action="UPLOAD_URL"
        :headers="uploadHeaders"
        :before-upload="beforeUpload"
        :on-success="onUploadSuccess"
        :on-error="onUploadError"
        :show-file-list="false"
        accept=".md,.txt,.pdf,.docx,.html"
      >
        <el-button type="primary" :icon="Upload">上传文档</el-button>
      </el-upload>
      <el-select v-model="docType" placeholder="类型筛选" clearable style="width:160px" @change="reload">
        <el-option v-for="t in DOC_TYPES" :key="t.value" :label="t.label" :value="t.value" />
      </el-select>
      <el-input v-model="keyword" placeholder="搜索文档" clearable style="width:220px" :prefix-icon="Search" @keyup.enter="reload" @clear="reload" />
      <el-button @click="reload">查询</el-button>
      <span class="result-count">共 {{ total }} 条</span>
    </div>

    <el-table :data="docs" v-loading="loading" stripe style="width:100%" empty-text="暂无文档">
      <el-table-column prop="title" label="标题" min-width="220" show-overflow-tooltip>
        <template #default="{ row }">
          <el-link type="primary" :underline="false" @click="viewDoc(row)">{{ row.title }}</el-link>
        </template>
      </el-table-column>
      <el-table-column label="类型" width="110">
        <template #default="{ row }">
          <el-tag :type="typeTagColor(row.doc_type)" size="small">{{ typeLabel(row.doc_type) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="category" label="分类" width="140" show-overflow-tooltip />
      <el-table-column label="大小" width="100">
        <template #default="{ row }">{{ formatSize(row.file_size) }}</template>
      </el-table-column>
      <el-table-column label="创建时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button size="small" link type="primary" @click="viewDoc(row)">查看</el-button>
          <el-button size="small" link type="danger" @click="removeDoc(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager-bar">
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="limit"
        :total="total"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next"
        background
        @current-change="reload"
        @size-change="reload"
      />
    </div>

    <DocDetailDialog v-model="detailVisible" :data="detailData" />
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Upload, Search } from '@element-plus/icons-vue'
import DocDetailDialog from './DocDetailDialog.vue'

const API_BASE = '/api/v1/knowledge/documents'
const UPLOAD_URL = `${API_BASE}/upload`

const DOC_TYPES = [
  { value: 'case', label: '案例' },
  { value: 'runbook', label: 'Runbook' },
  { value: 'document', label: '文档' },
  { value: 'faq', label: 'FAQ' },
]

const docs = ref([])
const total = ref(0)
const page = ref(1)
const limit = ref(20)
const docType = ref('')
const keyword = ref('')
const loading = ref(false)

const detailVisible = ref(false)
const detailData = ref(null)

const uploadHeaders = computed(() => {
  const token = localStorage.getItem('aiw-token')
  return token ? { Authorization: `Bearer ${token}` } : {}
})

const MAX_FILE_SIZE = 20 * 1024 * 1024

const beforeUpload = (file) => {
  if (file.size > MAX_FILE_SIZE) {
    ElMessage.warning('文件大小不能超过 20MB')
    return false
  }
  return true
}

const onUploadSuccess = () => {
  ElMessage.success('上传成功')
  reload()
}

const onUploadError = () => {
  ElMessage.error('上传失败')
}

const reload = async () => {
  loading.value = true
  try {
    const { data } = await axios.get(API_BASE, {
      params: {
        page: page.value,
        limit: limit.value,
        doc_type: docType.value || undefined,
        keyword: keyword.value || undefined,
      },
    })
    docs.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '加载文档失败')
  } finally {
    loading.value = false
  }
}

const viewDoc = async (row) => {
  try {
    const { data } = await axios.get(`${API_BASE}/${row.id}`)
    detailData.value = data
    detailVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '加载文档详情失败')
  }
}

const removeDoc = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除文档「${row.title}」？`, '删除确认', { type: 'warning' })
  } catch { return }
  try {
    await axios.delete(`${API_BASE}/${row.id}`)
    ElMessage.success('已删除')
    reload()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '删除失败')
  }
}

const typeLabel = (t) => DOC_TYPES.find(d => d.value === t)?.label || t || '-'

const typeTagColor = (t) => {
  const map = { case: 'danger', runbook: 'warning', document: '', faq: 'success' }
  return map[t] ?? 'info'
}

const formatSize = (bytes) => {
  if (!bytes) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

const formatTime = (iso) => {
  if (!iso) return '-'
  try { return new Date(iso).toLocaleString('zh-CN', { hour12: false }) } catch { return iso }
}

onMounted(reload)
</script>

<style scoped>
.doc-manager { display: flex; flex-direction: column; gap: 14px; }
.toolbar { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.result-count { color: var(--muted); font-size: 12px; margin-left: auto; }
.pager-bar { display: flex; justify-content: flex-end; padding: 8px 0 0; }
:deep(.el-table) { background: transparent; }
:deep(.el-table tr), :deep(.el-table th.el-table__cell) { background: transparent !important; }
</style>
