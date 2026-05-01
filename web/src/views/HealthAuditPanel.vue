<template>
  <div class="health-audit-page">
    <header class="section-head">
      <div>
        <h2>存储健康与审计日志</h2>
        <p>展示 MySQL、Redis 健康状态，并抽样验证敏感操作审计留痕。</p>
      </div>
      <el-button :loading="loading" @click="load">刷新</el-button>
    </header>

    <section class="health-grid">
      <div class="health-card">
        <span>MySQL</span>
        <b>{{ storage.mysql || '需确认' }}</b>
      </div>
      <div class="health-card">
        <span>Redis</span>
        <b>{{ storage.redis || '需确认' }}</b>
      </div>
    </section>

    <section class="audit-section">
      <h3>审计日志</h3>
      <el-table :data="events" stripe style="width:100%">
        <el-table-column prop="action" label="动作" min-width="160" />
        <el-table-column prop="operator" label="操作人" width="100" />
        <el-table-column label="时间" width="170">
          <template #default="{ row }">{{ row.timestamp ? new Date(row.timestamp).toLocaleString('zh-CN') : '-' }}</template>
        </el-table-column>
        <el-table-column prop="target" label="对象" min-width="140" />
        <el-table-column prop="risk" label="风险" width="80" />
        <el-table-column prop="decision" label="决策" width="80" />
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column prop="test_batch_id" label="批次" min-width="150" />
      </el-table>
      <el-empty v-if="!events.length" description="暂无审计日志" />
    </section>

    <el-alert v-if="msg" :title="msg" :type="msgType" show-icon class="feedback" />
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import axios from 'axios'

const loading = ref(false)
const storage = ref({})
const events = ref([])
const msg = ref('')
const msgType = ref('success')

const load = async () => {
  loading.value = true
  msg.value = ''
  try {
    const [storageRes, auditRes] = await Promise.all([
      axios.get('/api/v1/health/storage'),
      axios.get('/api/v1/audit/events'),
    ])
    storage.value = storageRes.data || {}
    events.value = Array.isArray(auditRes.data) ? auditRes.data : []
  } catch (e) {
    msg.value = e.response?.data?.error || '健康或审计日志加载失败'
    msgType.value = 'error'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.health-audit-page { padding: 24px; color: #243553; }
.section-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 18px; margin-bottom: 18px; }
.section-head h2 { margin: 0 0 6px; font-size: 22px; color: #1e3a5f; }
.section-head p { margin: 0; color: #60728e; font-size: 13px; }
.health-grid { display: grid; grid-template-columns: repeat(2, minmax(180px, 240px)); gap: 14px; margin-bottom: 20px; }
.health-card { padding: 18px; border-radius: 18px; background: rgba(255,255,255,.56); border: 1px solid rgba(255,255,255,.72); box-shadow: 0 14px 36px rgba(63,100,160,.12); }
.health-card span { display: block; color: #60728e; font-size: 12px; margin-bottom: 8px; }
.health-card b { color: #166534; font-size: 22px; }
.audit-section h3 { margin: 0 0 12px; color: #1e3a5f; }
.feedback { margin-top: 14px; max-width: 720px; }
</style>
