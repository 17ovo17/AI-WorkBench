<template>
  <div class="dashboard-page">
    <header class="dash-header">
      <h1>运维总览</h1>
      <div class="dash-actions">
        <el-button size="small" @click="load">刷新</el-button>
        <span class="dash-time">{{ currentTime }}</span>
      </div>
    </header>

    <section class="stats-grid">
      <div
        class="stat-card glass-card"
        v-for="stat in stats"
        :key="stat.label"
        :role="stat.to ? 'button' : undefined"
        :tabindex="stat.to ? 0 : undefined"
        @click="goStat(stat)"
        @keydown.enter="goStat(stat)"
      >
        <div class="stat-icon" :style="{ background: stat.color }">
          <el-icon :size="24"><component :is="stat.icon" /></el-icon>
        </div>
        <div class="stat-body">
          <span class="stat-value">{{ stat.value }}</span>
          <span class="stat-label">{{ stat.label }}</span>
        </div>
      </div>
    </section>

    <section class="dash-grid">
      <div class="glass-card alert-summary">
        <h3>告警分布</h3>
        <div class="alert-bars">
          <div class="alert-bar" v-for="item in alertBars" :key="item.label">
            <span class="bar-label">{{ item.label }}</span>
            <div class="bar-track"><div class="bar-fill" :style="{ width: item.pct + '%', background: item.color }"></div></div>
            <span class="bar-count">{{ item.count }}</span>
          </div>
        </div>
      </div>

      <div class="glass-card recent-diagnoses">
        <h3>最近诊断</h3>
        <div class="diag-list">
          <div v-if="!summary.recent_diagnoses?.length" class="empty-hint">暂无诊断记录</div>
          <div v-for="d in summary.recent_diagnoses" :key="d.id" class="diag-item" role="button" tabindex="0" @click="openDiagnosisArchive(d)" @keydown.enter="openDiagnosisArchive(d)">
            <span class="diag-ip">{{ d.target_ip }}</span>
            <el-tag :type="d.status === 'done' ? 'success' : d.status === 'failed' ? 'danger' : 'warning'" size="small">{{ d.status }}</el-tag>
            <span class="diag-source">{{ d.source }}</span>
          </div>
        </div>
      </div>

      <div class="glass-card quick-actions">
        <h3>快捷入口</h3>
        <div class="action-grid">
          <button class="action-btn" @click="$router.push('/workbench')"><el-icon><ChatDotRound /></el-icon>智能问诊</button>
          <button class="action-btn" @click="$router.push('/topology')"><el-icon><Share /></el-icon>业务拓扑</button>
          <button class="action-btn" @click="$router.push({ path: '/topology', query: { tab: 'catpaw' } })"><el-icon><Connection /></el-icon>探针管理</button>
          <button class="action-btn" @click="$router.push({ path: '/settings', query: { tab: 'profiles' } })"><el-icon><User /></el-icon>常用地址</button>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import axios from 'axios'
import { useRouter } from 'vue-router'

const summary = ref({})
const currentTime = ref('')
const router = useRouter()
let timer = null

const stats = computed(() => [
  { label: '在线探针', value: summary.value.agents?.online || 0, icon: 'Connection', color: 'linear-gradient(135deg,#22c55e,#16a34a)' },
  { label: '活跃告警', value: summary.value.alerts?.firing || 0, icon: 'Bell', color: 'linear-gradient(135deg,#ef4444,#dc2626)', to: '/alerts' },
  { label: '业务系统', value: summary.value.businesses || 0, icon: 'Share', color: 'linear-gradient(135deg,#3b82f6,#2563eb)' },
  { label: '常用配置', value: summary.value.profiles || 0, icon: 'User', color: 'linear-gradient(135deg,#8b5cf6,#7c3aed)' },
])

const alertBars = computed(() => {
  const a = summary.value.alerts || {}
  const total = Math.max(a.total || 1, 1)
  return [
    { label: '严重', count: a.critical || 0, pct: ((a.critical || 0) / total) * 100, color: '#ef4444' },
    { label: '警告', count: a.warning || 0, pct: ((a.warning || 0) / total) * 100, color: '#f59e0b' },
    { label: '信息', count: a.info || 0, pct: ((a.info || 0) / total) * 100, color: '#3b82f6' },
  ]
})

const load = async () => {
  try {
    const { data } = await axios.get('/api/v1/dashboard/summary')
    summary.value = data
  } catch {}
}

const updateTime = () => { currentTime.value = new Date().toLocaleString('zh-CN') }
const goStat = (stat) => { if (stat.to) router.push(stat.to) }
const openDiagnosisArchive = diagnosis => {
  router.push({ path: '/knowledge', query: { tab: 'diagnosis', diagnosis_id: diagnosis.id || '' } })
}

onMounted(() => { load(); updateTime(); timer = setInterval(() => { load(); updateTime() }, 30000) })
onBeforeUnmount(() => clearInterval(timer))
</script>

<style scoped>
.dashboard-page { padding: 24px; max-width: 1200px; margin: 0 auto; }
.dash-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px; }
.dash-header h1 { font-size: 24px; color: #1e3a5f; }
.dash-actions { display: flex; align-items: center; gap: 12px; }
.dash-time { color: #64748b; font-size: 14px; }
.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 24px; }
.stat-card { display: flex; align-items: center; gap: 16px; padding: 20px; border-radius: 20px; }
.stat-card[role="button"] { cursor: pointer; }
.stat-card[role="button"]:focus-visible { outline: 2px solid #247cff; outline-offset: 3px; }
.stat-icon { width: 48px; height: 48px; border-radius: 14px; display: flex; align-items: center; justify-content: center; color: white; }
.stat-value { font-size: 28px; font-weight: 800; color: #1e3a5f; display: block; }
.stat-label { font-size: 13px; color: #64748b; }
.dash-grid { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 16px; }
.dash-grid .glass-card { padding: 20px; border-radius: 20px; }
.dash-grid h3 { font-size: 15px; color: #1e3a5f; margin-bottom: 16px; }
.alert-bars { display: flex; flex-direction: column; gap: 12px; }
.alert-bar { display: flex; align-items: center; gap: 10px; }
.bar-label { width: 36px; font-size: 13px; color: #64748b; }
.bar-track { flex: 1; height: 8px; background: #e2e8f0; border-radius: 4px; overflow: hidden; }
.bar-fill { height: 100%; border-radius: 4px; transition: width .3s; }
.bar-count { width: 28px; text-align: right; font-size: 14px; font-weight: 700; color: #334155; }
.diag-list { display: flex; flex-direction: column; gap: 8px; }
.diag-item { display: flex; align-items: center; gap: 10px; padding: 8px 12px; border-radius: 12px; background: rgba(255,255,255,.5); cursor: pointer; }
.diag-item:hover { background: rgba(255,255,255,.8); }
.diag-ip { font-weight: 700; font-size: 13px; color: #1e3a5f; }
.diag-source { font-size: 12px; color: #94a3b8; margin-left: auto; }
.empty-hint { color: #94a3b8; font-size: 13px; text-align: center; padding: 20px; }
.action-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.action-btn { display: flex; align-items: center; gap: 8px; padding: 12px 16px; border: 1px solid rgba(255,255,255,.6); border-radius: 14px; background: rgba(255,255,255,.5); color: #334155; font-size: 13px; font-weight: 600; cursor: pointer; transition: all .2s; }
.action-btn:hover { background: #247cff; color: white; border-color: #247cff; }
</style>
