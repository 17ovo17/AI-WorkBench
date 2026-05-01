<template>
  <div class="diagnose-page" :class="{ embedded }">
    <div v-if="!embedded" class="page-head">
      <div><div class="panel-kicker">AI Diagnose</div><h2>智能诊断中心</h2></div>
      <div class="head-actions">
        <el-input
          v-model="targetIP"
          placeholder="选择或输入目标 IP"
          clearable
          style="width:240px"
          @keyup.enter="startDiagnose"
        />
        <el-select v-model="targetIP" filterable placeholder="从监控/探针选择" style="width:220px">
          <el-option v-for="a in agents" :key="`agent-${a.ip}`" :label="`${a.ip} ${a.hostname || 'Catpaw'}`" :value="a.ip" />
          <el-option v-for="i in promInstances" :key="`prom-${i}`" :label="i" :value="i.split(':')[0]" />
        </el-select>
        <el-button type="primary" :loading="launching" @click="startDiagnose">发起诊断</el-button>
        <el-button type="warning" plain @click="cleanupDiagnose('business_inspection')">清理业务巡检</el-button>
        <el-button type="danger" plain @click="cleanupDiagnose('test')">清理测试记录</el-button>
      </div>
    </div>

    <div class="record-toolbar">
      <div class="record-filters">
        <el-select v-model="sourceFilter" clearable placeholder="按来源筛选" style="width:180px">
          <el-option label="业务巡检" value="business_inspection" />
          <el-option label="手动诊断" value="manual" />
          <el-option label="告警触发" value="alert" />
          <el-option label="Catpaw" value="catpaw" />
          <el-option label="Prometheus" value="prometheus" />
        </el-select>
        <el-input v-model="keyword" clearable placeholder="搜索业务、IP、标题、来源" style="width:280px" />
        <el-button @click="resetFilters">重置筛选</el-button>
      </div>
      <div class="record-summary">当前 {{ visibleRecords.length }} / 全部 {{ records.length }} 条；业务巡检按业务统一诊断，原始证据默认折叠。</div>
    </div>

    <div class="records-grid">
      <div v-for="r in visibleRecords" :key="r.id" class="record-card" @click="selected = r">
        <div class="record-head">
          <span class="record-ip">{{ displayTarget(r) }}</span>
          <div style="display:flex;gap:6px;align-items:center">
            <el-tag :type="statusType(r.status)" size="small">{{ statusLabel(r.status) }}</el-tag>
            <el-button size="small" type="danger" plain @click.stop="confirmDelete(r.id)">删除</el-button>
          </div>
        </div>
        <div class="record-meta">
          <span class="source-badge" :class="r.source">{{ sourceLabel(r) }}</span>
          <span class="trigger-badge">{{ r.trigger === 'alert' ? '告警触发' : r.trigger === 'catpaw' ? 'Catpaw 上报' : '手动' }}</span>
          <span class="time">{{ formatTime(r.create_time) }}</span>
        </div>
        <div v-if="r.alert_title" class="alert-title">{{ r.alert_title }}</div>
      </div>
      <div v-if="!records.length" class="empty">暂无诊断记录</div>
    </div>

    <el-drawer v-model="drawerOpen" title="诊断报告" size="60%" class="diagnose-drawer">
      <div v-if="selected" class="report-content">
        <div v-if="selected.status === 'done' && (selected.report || parsedInspection)">
          <div class="report-toolbar">
            <div>
              <div class="report-title">{{ displayTarget(selected) }} 诊断报告</div>
              <div class="report-subtitle">{{ formatTime(selected.create_time) }}</div>
            </div>
            <div class="report-actions">
              <el-button size="small" @click="downloadMarkdown">下载 Markdown</el-button>
              <el-button size="small" @click="downloadHtml">下载 HTML</el-button>
              <el-button size="small" type="success" plain @click="archiveSelected">归档为案例</el-button>
              <el-button size="small" type="danger" plain @click="confirmDelete(selected.id)">删除记录</el-button>
            </div>
          </div>
          <div class="report-scroll">
            <InspectionDashboard
              :inspection="parsedInspection"
              :markdownReport="reportMarkdown"
            />
          </div>
          <el-collapse v-if="selected.raw_report && !parsedInspection" class="raw-collapse">
            <el-collapse-item title="完整原始巡检数据（默认折叠，可展开/下载）" name="raw">
              <pre class="raw-report">{{ selected.raw_report }}</pre>
            </el-collapse-item>
          </el-collapse>
        </div>
        <p v-else-if="selected.status === 'failed' || (selected.status === 'done' && !selected.report)">诊断失败，可能是数据源不可达或 AI 分析出错。</p>
        <p v-else>诊断中，请稍候...</p>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { extractMarkdownText, renderMarkdown } from '../utils/renderMarkdown'
import InspectionDashboard from '../components/InspectionDashboard.vue'

defineProps({ embedded: { type: Boolean, default: false } })

const records = ref([])
const selected = ref(null)
const targetIP = ref('')
const launching = ref(false)
const agents = ref([])
const promInstances = ref([])
const sourceFilter = ref('')
const keyword = ref('')
const drawerOpen = computed({ get: () => !!selected.value, set: v => { if (!v) selected.value = null } })

const parsedInspection = computed(() => {
  const r = selected.value
  if (!r || !r.raw_report) return null
  try {
    const parsed = JSON.parse(r.raw_report)
    if (parsed.business_name && typeof parsed.score === 'number') return parsed
  } catch {}
  return null
})

const reportMarkdown = computed(() => extractMarkdownText(selected.value?.summary_report || selected.value?.report || ''))
const renderMd = t => renderMarkdown(t)
const formatTime = t => new Date(t).toLocaleString('zh-CN')
const statusType = s => ({ pending: 'info', running: 'warning', done: 'success', failed: 'danger' }[s] || 'info')
const statusLabel = s => ({ pending: '等待中', running: '诊断中', done: '已完成', failed: '失败' }[s] || s)
const sourceLabel = r => ({ business_inspection: '业务巡检', prometheus: 'Prometheus', catpaw: 'Catpaw', manual: '手动诊断', alert: '告警触发' }[r?.source] || { business_inspection: '业务巡检', prometheus: 'Prometheus', catpaw: 'Catpaw', manual: '手动诊断', alert: '告警触发' }[r?.data_source] || r?.source || r?.data_source || '未知目标')
const triggerLabel = trigger => ({ business_inspection: '业务巡检', alert: '告警触发', catpaw: 'Catpaw 上报', manual: '手动' }[trigger] || trigger || '手动')
const displayTarget = r => String(r?.target_ip || '').startsWith('business:') ? '业务级统一诊断' : (r?.target_ip || '未知目标')
const searchable = r => [r.id, r.target_ip, r.trigger, r.source, r.data_source, r.alert_title, r.summary_report].join(' ').toLowerCase()
const visibleRecords = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  return records.value.filter(r => !q || searchable(r).includes(q))
})
const resetFilters = () => { sourceFilter.value = ''; keyword.value = '' }

const load = async () => {
  const params = {}
  if (sourceFilter.value) params.source = sourceFilter.value
  const { data } = await axios.get('/api/v1/diagnose', { params })
  records.value = data || []
  if (selected.value) {
    const fresh = records.value.find(r => r.id === selected.value.id)
    if (fresh) selected.value = fresh
  }
}

const startDiagnose = async () => {
  const ip = targetIP.value.trim()
  if (!/^((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.|$)){4}$/.test(ip)) {
    ElMessageBox.alert('请输入合法的 IPv4 地址', '输入错误', { type: 'warning' })
    return
  }
  launching.value = true
  try {
    const { data } = await axios.post('/api/v1/diagnose', { ip })
    await load()
    const created = records.value.find(r => r.id === data?.id)
    if (created) selected.value = created
    ElMessage.success('\u8bca\u65ad\u4efb\u52a1\u5df2\u521b\u5efa\uff0c\u6b63\u5728\u6536\u96c6\u6307\u6807\u548c\u751f\u6210\u62a5\u544a')
  } catch (error) {
    ElMessage.error(`\u53d1\u8d77\u8bca\u65ad\u5931\u8d25\uff1a${error.response?.data?.error || error.message || '\u8bf7\u68c0\u67e5\u540e\u7aef\u548c\u6570\u636e\u6e90\u72b6\u6001'}`)
  } finally {
    launching.value = false
  }
}

const confirmDelete = async (id) => {
  try {
    await ElMessageBox.confirm('\u786e\u8ba4\u5220\u9664\u8fd9\u6761\u8bca\u65ad\u8bb0\u5f55\uff1f\u5220\u9664\u540e\u4e0d\u53ef\u6062\u590d\u3002', '\u4e8c\u6b21\u786e\u8ba4', { type: 'warning', confirmButtonText: '\u786e\u8ba4\u5220\u9664', cancelButtonText: '\u53d6\u6d88' })
    await axios.delete(`/api/v1/diagnose/${id}`)
    if (selected.value?.id === id) selected.value = null
    await load()
    ElMessage.success('\u8bca\u65ad\u8bb0\u5f55\u5df2\u5220\u9664')
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error('\u5220\u9664\u5931\u8d25\uff0c\u8bf7\u7a0d\u540e\u91cd\u8bd5')
  }
}


const cleanupDiagnose = async (scope) => {
  const text = scope === 'business_inspection' ? '\u786e\u8ba4\u5220\u9664\u6240\u6709\u4e1a\u52a1\u5de1\u68c0\u8bca\u65ad\u8bb0\u5f55\uff1f\u4e1a\u52a1\u5de1\u68c0\u53ef\u5728\u4e1a\u52a1\u62d3\u6251\u4e2d\u91cd\u65b0\u751f\u6210\u3002' : '\u786e\u8ba4\u5220\u9664\u6d4b\u8bd5\u8bca\u65ad\u8bb0\u5f55\uff1f\u4ec5\u5339\u914d test\u3001whitebox\u3001aiw- \u7b49\u6d4b\u8bd5\u6807\u8bc6\u3002'
  try {
    await ElMessageBox.confirm(text, '\u6279\u91cf\u6e05\u7406\u786e\u8ba4', { type: 'warning', confirmButtonText: '\u786e\u8ba4\u6e05\u7406', cancelButtonText: '\u53d6\u6d88' })
    const { data } = await axios.delete('/api/v1/diagnose', { params: { scope } })
    ElMessage.success(`\u5df2\u6e05\u7406 ${data?.deleted || 0} \u6761\u8bca\u65ad\u8bb0\u5f55`)
    if (selected.value && (scope === 'business_inspection' || String(selected.value.id).includes('test'))) selected.value = null
    await load()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error('\u6e05\u7406\u5931\u8d25\uff0c\u8bf7\u7a0d\u540e\u91cd\u8bd5')
  }
}


const safeFilePart = value => String(value || 'unknown').replace(/[^a-zA-Z0-9._-]/g, '_')

const downloadBlob = (content, type, ext) => {
  if (!selected.value) return
  const a = document.createElement('a')
  a.href = URL.createObjectURL(new Blob([content], { type }))
  const stamp = new Date().toISOString().slice(0,19).replace(/:/g,'-')
  a.download = `diagnose-${safeFilePart(selected.value.target_ip)}-${stamp}.${ext}`
  a.click()
  URL.revokeObjectURL(a.href)
}

const buildInspectionMarkdown = (ins) => {
  const lines = [
    `# ${ins.business_name || '诊断'} 报告`,
    `- 状态：${ins.status || '未知'}`,
    `- 评分：${ins.score ?? '--'}`,
    `- 数据源：${(ins.data_sources || []).join('、') || '未知'}`,
    '', '## AI 分析报告', '', ins.ai_analysis || '无 AI 分析',
  ]
  if (ins.ai_suggestions?.length) {
    lines.push('', '## AI 建议')
    ins.ai_suggestions.forEach(s => lines.push(`- ${s}`))
  }
  if (ins.metrics?.length) {
    lines.push('', '## 指标概览', '', '| IP | 指标 | 值 | 状态 |', '|---|---|---|---|')
    ins.metrics.forEach(m => lines.push(`| ${m.ip} | ${m.name} | ${Number(m.value || 0).toFixed(2)}${m.unit || ''} | ${m.status} |`))
  }
  if (ins.alerts?.length) {
    lines.push('', '## 告警', '', '| 标题 | IP | 级别 | 状态 |', '|---|---|---|---|')
    ins.alerts.forEach(a => lines.push(`| ${a.title} | ${a.target_ip} | ${a.severity} | ${a.status} |`))
  }
  return lines.join('\n')
}

const downloadMarkdown = () => {
  const ins = parsedInspection.value
  if (ins) {
    downloadBlob(buildInspectionMarkdown(ins), 'text/markdown;charset=utf-8', 'md')
  } else {
    const title = `# 诊断报告 - ${selected.value.target_ip}\n\n生成时间：${formatTime(selected.value.create_time)}\n\n`
    downloadBlob(title + reportMarkdown.value, 'text/markdown;charset=utf-8', 'md')
  }
}

const downloadHtml = () => {
  const mdContent = parsedInspection.value ? buildInspectionMarkdown(parsedInspection.value) : reportMarkdown.value
  const html = `<!DOCTYPE html><html><head><meta charset="utf-8"><title>诊断报告 - ${selected.value.target_ip}</title>
<style>body{background:#0d1117;color:#f0f6fc;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;padding:24px;line-height:1.7}h1,h2,h3,h4{color:#fff}p,li,td,th,blockquote{color:#f0f6fc}a{color:#79c0ff}pre{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:12px;overflow-x:auto}table{border-collapse:collapse;width:100%;margin:12px 0}th,td{border:1px solid #30363d;padding:8px 10px}code{background:#161b22;color:#79c0ff;padding:2px 5px;border-radius:4px}blockquote{border-left:3px solid #58a6ff;background:#161b22;margin:12px 0;padding:8px 12px}</style>
</head><body>${renderMd(mdContent)}</body></html>`
  downloadBlob(html, 'text/html;charset=utf-8', 'html')
}

const plainReportText = r => {
  const value = r?.summary_report || r?.report || r?.raw_report || r?.alert_title || ''
  if (typeof value === 'string') return value.replace(/[{}"'[\]]/g, ' ').replace(/\s+/g, ' ').trim()
  try { return JSON.stringify(value) } catch { return String(value || '') }
}

const inferCategory = r => {
  const text = [r?.alert_title, r?.source, r?.data_source, plainReportText(r)].join(' ').toLowerCase()
  if (text.includes('cpu')) return 'CPU'
  if (text.includes('memory') || text.includes('内存')) return 'Memory'
  if (text.includes('disk') || text.includes('磁盘')) return 'Disk'
  if (text.includes('network') || text.includes('网络')) return 'Network'
  return sourceLabel(r)
}

const archiveSelected = async () => {
  if (!selected.value) return
  const description = plainReportText(selected.value).slice(0, 800) || `${displayTarget(selected.value)} 诊断结论`
  try {
    await axios.post('/api/v1/diagnosis/archive', {
      diagnosis_id: selected.value.id,
      root_cause_category: inferCategory(selected.value),
      root_cause_description: description,
      treatment_steps: '查看诊断报告，按报告建议确认指标、进程、日志并执行处置。',
      keywords: [displayTarget(selected.value), selected.value.source || selected.value.data_source, selected.value.trigger, 'diagnosis'].filter(Boolean).join(',')
    })
    ElMessage.success('已归档到知识中心案例库')
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '归档失败')
  }
}

let timer
onMounted(async () => {
  load()
  watch(sourceFilter, load)
  timer = setInterval(load, 15000)
  const [a, p] = await Promise.all([
    axios.get('/api/v1/catpaw/agents').catch(() => ({ data: [] })),
    axios.get('/api/v1/prometheus/instances').catch(() => ({ data: { data: [] } }))
  ])
  agents.value = (a.data || []).filter(x => x.online)
  promInstances.value = p.data?.data || []
})
onUnmounted(() => clearInterval(timer))
</script>


<style scoped>
.diagnose-page { padding: 32px 36px; height: 100%; min-height: 0; color: #243553; display: flex; flex-direction: column; overflow: hidden; }
.diagnose-page.embedded { padding: 0; }
.page-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 22px; }
.page-head h2 { margin: 8px 0 0; font-size: 30px; letter-spacing: -.04em; color: #263653; }
.panel-kicker { font-size: 13px; color: #247cff; text-transform: uppercase; letter-spacing: .06em; font-weight: 800; }
.head-actions { display: flex; gap: 12px; align-items: center; padding: 12px; border-radius: 22px; background: rgba(255,255,255,.34); border: 1px solid rgba(255,255,255,.65); backdrop-filter: blur(18px); }
.record-toolbar { display:flex; justify-content:space-between; align-items:center; gap:14px; margin:-8px 0 18px; padding:12px 14px; border-radius:18px; background:rgba(255,255,255,.36); border:1px solid rgba(255,255,255,.66); backdrop-filter:blur(16px); }
.record-filters { display:flex; gap:10px; align-items:center; flex-wrap:wrap; }
.record-summary { color:#667995; font-size:12px; }
.records-grid { flex: 1; min-height: 0; display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 16px; overflow: auto; padding-right: 6px; align-content: start; }
.record-card { background: linear-gradient(145deg, rgba(255,255,255,.58), rgba(225,236,255,.42)); border: 1px solid rgba(255,255,255,.72); border-radius: 24px; padding: 20px; cursor: pointer; transition: transform .22s ease, box-shadow .22s ease; box-shadow: 0 20px 54px rgba(63,100,160,.16), inset 0 1px 0 rgba(255,255,255,.78); backdrop-filter: blur(24px); }
.record-card:hover { transform: translateY(-3px); box-shadow: 0 28px 70px rgba(63,100,160,.22), inset 0 1px 0 rgba(255,255,255,.8); }
.record-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; gap: 10px; }
.record-ip { font-weight: 800; font-size: 18px; color: #253855; }
.record-meta { display: flex; gap: 8px; align-items: center; font-size: 12px; color: #74849e; flex-wrap: wrap; }
.source-badge { padding: 5px 10px; border-radius: 999px; font-size: 11px; font-weight: 800; background: rgba(255,255,255,.5); }
.source-badge.prometheus { color: #247cff; }
.source-badge.catpaw { color: #22b96d; }
.alert-title { margin-top: 12px; font-size: 13px; color: #6b7890; }
.empty { color: #74849e; text-align: center; padding: 80px; grid-column: 1/-1; background: rgba(255,255,255,.34); border-radius: 24px; border: 1px solid rgba(255,255,255,.68); }
.report-content { padding: 18px; color: #243553; min-height: 100%; background: linear-gradient(145deg, #eef6ff, #dbeaff); }
.report-toolbar { display: flex; justify-content: space-between; gap: 16px; align-items: flex-start; margin-bottom: 16px; padding: 18px; background: rgba(255,255,255,.58); border: 1px solid rgba(255,255,255,.72); border-radius: 22px; box-shadow: inset 0 1px 0 rgba(255,255,255,.78); }
.report-title { color: #253855; font-weight: 800; font-size: 18px; }
.report-subtitle { color: #74849e; font-size: 12px; margin-top: 4px; }
.report-actions { display: flex; flex-wrap: wrap; gap: 8px; justify-content: flex-end; }
.report-scroll { max-height: calc(100vh - 220px); overflow-y: auto; border-radius: 22px; }
.raw-collapse { margin-top: 14px; border-radius: 18px; overflow: hidden; }
.raw-report { max-height: 360px; overflow: auto; white-space: pre-wrap; word-break: break-word; background: rgba(20,31,52,.94); color: #f8fbff; border-radius: 14px; padding: 14px; font-size: 12px; }
:global(.diagnose-drawer .el-drawer__body) { background: linear-gradient(145deg, #eef6ff, #dbeaff); color: #243553; }
:global(.diagnose-drawer .el-drawer__header) { background: rgba(245,250,255,.85); color: #243553; border-bottom: 1px solid rgba(255,255,255,.7); margin-bottom: 0; padding-bottom: 16px; }
</style>


