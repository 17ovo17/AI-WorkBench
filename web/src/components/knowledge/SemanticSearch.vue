<template>
  <div class="semantic-search">
    <div class="search-bar">
      <el-input
        v-model="query"
        placeholder="输入自然语言查询，如：CPU 使用率过高怎么处理"
        clearable
        style="flex:1"
        :prefix-icon="Search"
        @keyup.enter="doSearch"
      />
      <el-select v-model="docType" placeholder="类型" clearable style="width:130px">
        <el-option v-for="t in DOC_TYPES" :key="t.value" :label="t.label" :value="t.value" />
      </el-select>
      <el-input-number v-model="topK" :min="1" :max="20" style="width:120px" />
      <el-button type="primary" :icon="Search" :loading="searching" @click="doSearch">搜索</el-button>
    </div>

    <div v-if="results.length" class="result-list">
      <div v-for="(item, idx) in results" :key="idx" class="result-card glass-card">
        <div class="card-head">
          <span class="card-title">{{ item.title || '未命名文档' }}</span>
          <el-tag :type="typeTagColor(item.doc_type)" size="small">{{ typeLabel(item.doc_type) }}</el-tag>
          <el-tag v-if="item.parent_id" size="small" effect="plain">Chunk {{ item.chunk_index }}</el-tag>
          <span class="score">相关度 {{ scorePercent(item) }}</span>
        </div>
        <div class="card-content">{{ item.content || item.snippet || '-' }}</div>
        <div v-if="item.context_chunks?.length" class="context-list">
          <div v-for="ctx in item.context_chunks" :key="ctx.doc_id" class="context-item">
            <span class="context-label">{{ ctx.position === 'previous' ? '上文' : '下文' }}</span>
            <span>{{ ctx.content }}</span>
          </div>
        </div>
        <div class="card-actions">
          <el-button size="small" plain type="danger" @click="markBadcase(item)">不相关</el-button>
        </div>
      </div>
    </div>

    <el-empty v-else-if="searched && !searching" description="未找到相关结果" :image-size="80" />
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'
import { Search } from '@element-plus/icons-vue'

const SEARCH_URL = '/api/v1/knowledge/search'
const BADCASE_URL = '/api/v1/knowledge/search/badcase'
const DOC_TYPES = [
  { value: 'case', label: '案例' },
  { value: 'runbook', label: 'Runbook' },
  { value: 'document', label: '文档' },
  { value: 'faq', label: 'FAQ' },
]

const query = ref('')
const docType = ref('')
const topK = ref(5)
const searching = ref(false)
const searched = ref(false)
const results = ref([])

const props = defineProps({
  initialResults: { type: Array, default: () => [] },
  initialQuery: { type: String, default: '' },
})

watch(() => props.initialResults, (items) => {
  if (Array.isArray(items) && items.length) {
    results.value = items
    searched.value = true
  }
}, { immediate: true })

watch(() => props.initialQuery, (value) => {
  if (value && !query.value) query.value = value
}, { immediate: true })

const doSearch = async () => {
  if (!query.value.trim()) {
    ElMessage.warning('请输入搜索内容')
    return
  }
  searching.value = true
  searched.value = true
  try {
    const { data } = await axios.post(SEARCH_URL, {
      query: query.value,
      top_k: topK.value,
      doc_type: docType.value || undefined,
    })
    results.value = data.items || data.results || data || []
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '搜索失败')
    results.value = []
  } finally {
    searching.value = false
  }
}

const markBadcase = async (item) => {
  try {
    await axios.post(BADCASE_URL, {
      query: query.value || '顶部搜索',
      doc_id: item.doc_id || item.id,
      reason: '用户标记为不相关',
    })
    ElMessage.success('已记录不相关反馈')
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '反馈提交失败')
  }
}

const typeLabel = (t) => DOC_TYPES.find(d => d.value === t)?.label || t || '-'

const scorePercent = (item) => Number.isFinite(Number(item.score)) ? `${(Number(item.score) * 100).toFixed(1)}%` : '-'

const typeTagColor = (t) => {
  const map = { case: 'danger', runbook: 'warning', document: '', faq: 'success' }
  return map[t] ?? 'info'
}
</script>

<style scoped>
.semantic-search { display: flex; flex-direction: column; gap: 16px; }
.search-bar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.result-list { display: flex; flex-direction: column; gap: 12px; }
.glass-card {
  background: rgba(255,255,255,0.08);
  backdrop-filter: blur(12px);
  border: 1px solid rgba(255,255,255,0.12);
  border-radius: 12px;
  padding: 16px 18px;
}
.card-head { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; }
.card-title { font-weight: 700; font-size: 14px; color: #243553; }
.score { margin-left: auto; font-size: 12px; color: #247cff; font-weight: 600; }
.card-content {
  font-size: 13px;
  color: #3a4a6a;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 120px;
  overflow: hidden;
}
.context-list { margin-top: 10px; display: flex; flex-direction: column; gap: 8px; }
.context-item { font-size: 12px; color: #5b6b86; background: rgba(36,124,255,.06); border-radius: 8px; padding: 8px 10px; white-space: pre-wrap; max-height: 72px; overflow: hidden; }
.context-label { color: #247cff; font-weight: 700; margin-right: 8px; }
.card-actions { display: flex; justify-content: flex-end; margin-top: 10px; }
</style>
