<template>
  <el-dialog v-model="visible" :title="title" width="760px" destroy-on-close>
    <div v-if="data" class="detail-body">
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="文档类型">
          <el-tag :type="typeTagColor(data.doc_type)" size="small">{{ typeLabel(data.doc_type) }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="文件格式">
          <el-tag size="small" effect="plain">{{ data.file_type || '-' }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="分类">{{ data.category || '-' }}</el-descriptions-item>
        <el-descriptions-item label="文件名">{{ data.file_name || '-' }}</el-descriptions-item>
        <el-descriptions-item label="大小">{{ formatSize(data.file_size) }}</el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ formatTime(data.created_at) }}</el-descriptions-item>
        <el-descriptions-item label="更新时间">{{ formatTime(data.updated_at) }}</el-descriptions-item>
        <el-descriptions-item label="标签">
          <template v-if="tagList.length">
            <el-tag v-for="t in tagList" :key="t" size="small" effect="plain" style="margin-right:4px">{{ t }}</el-tag>
          </template>
          <span v-else>-</span>
        </el-descriptions-item>
      </el-descriptions>

      <div class="detail-section">
        <div class="section-title">文档内容</div>
        <pre class="content-pre">{{ data.content || '暂无内容' }}</pre>
      </div>
    </div>
  </el-dialog>
</template>

<script setup>
import { computed } from 'vue'

const DOC_TYPES = [
  { value: 'case', label: '案例' },
  { value: 'runbook', label: 'Runbook' },
  { value: 'document', label: '文档' },
  { value: 'faq', label: 'FAQ' },
]

const props = defineProps({ modelValue: Boolean, data: Object })
const emit = defineEmits(['update:modelValue'])

const visible = computed({ get: () => props.modelValue, set: v => emit('update:modelValue', v) })
const title = computed(() => props.data ? `文档详情 - ${props.data.title}` : '文档详情')
const tagList = computed(() => (props.data?.tags || '').split(',').map(s => s.trim()).filter(Boolean))

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
</script>

<style scoped>
.detail-body { display: flex; flex-direction: column; gap: 16px; }
.detail-section { background: rgba(255,255,255,.42); border-radius: 14px; padding: 14px 16px; border: 1px solid rgba(255,255,255,.6); }
.section-title { font-weight: 800; color: #243553; margin-bottom: 8px; font-size: 13px; }
.content-pre { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; background: rgba(35,53,83,.06); border-radius: 10px; padding: 10px 12px; max-height: 400px; overflow: auto; color: #2c3a55; white-space: pre-wrap; word-break: break-word; }
</style>
