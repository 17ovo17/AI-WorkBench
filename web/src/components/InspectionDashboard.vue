<template>
  <div v-if="inspection" class="inspection-dashboard">
    <section class="inspection-hero" :class="inspection.status">
      <div>
        <b>{{ inspection.business_name }}</b>
        <h2>{{ inspection.score }}</h2>
        <p>{{ cleanSummary }}</p>
      </div>
      <div class="inspection-kpis">
        <span>资源 {{ inspection.resources?.length || 0 }}</span>
        <span>进程 {{ inspection.processes?.length || 0 }}</span>
        <span>指标 {{ inspection.metrics?.length || 0 }}</span>
        <span>告警 {{ inspection.alerts?.length || 0 }}</span>
      </div>
    </section>
    <el-tabs lazy>
      <el-tab-pane label="业务巡检" name="inspect">
        <div class="ai-advice" v-if="inspection.ai_analysis || inspection.ai_suggestions?.length">
          <b>AI 巡检建议</b>
          <div v-if="inspection.ai_analysis" class="md" v-html="renderMarkdown(inspection.ai_analysis)"></div>
          <ul>
            <li v-for="item in inspection.ai_suggestions" :key="item">{{ item }}</li>
          </ul>
        </div>
        <ul class="inspection-list">
          <li v-for="item in topFindings" :key="item">{{ item }}</li>
        </ul>
      </el-tab-pane>
      <el-tab-pane label="业务分析" name="metrics">
        <el-table :data="inspection.metrics" size="small" height="320">
          <el-table-column prop="ip" label="IP" width="130" />
          <el-table-column prop="name" label="指标" />
          <el-table-column label="值" width="120">
            <template #default="s">{{ Number(s.row.value || 0).toFixed(2) }}{{ s.row.unit }}</template>
          </el-table-column>
          <el-table-column prop="status" label="状态" width="110" />
          <el-table-column prop="source" label="来源" width="120" />
        </el-table>
      </el-tab-pane>
      <el-tab-pane label="业务进程" name="processes">
        <el-table :data="inspection.processes" size="small" height="320">
          <el-table-column prop="ip" label="IP" width="130" />
          <el-table-column prop="name" label="进程/服务" />
          <el-table-column prop="description" label="描述" />
          <el-table-column prop="path" label="路径" />
          <el-table-column prop="port" label="端口" width="90" />
          <el-table-column prop="status" label="状态" width="100" />
        </el-table>
      </el-tab-pane>
      <el-tab-pane label="业务属性" name="attrs">
        <div class="attr-grid">
          <div v-for="(value, key) in inspection.attributes" :key="key"><b>{{ key }}</b><span>{{ value }}</span></div>
        </div>
      </el-tab-pane>
      <el-tab-pane label="业务资源" name="resources">
        <el-table :data="inspection.resources" size="small" height="320">
          <el-table-column prop="ip" label="IP" width="130" />
          <el-table-column prop="name" label="资源" />
          <el-table-column prop="type" label="类型" width="120" />
          <el-table-column prop="owner" label="负责人" width="120" />
          <el-table-column prop="purpose" label="用途" />
          <el-table-column prop="status" label="状态" width="100" />
        </el-table>
      </el-tab-pane>
      <el-tab-pane label="告警情况" name="alerts">
        <el-table :data="inspection.alerts" size="small" height="320">
          <el-table-column prop="title" label="告警" />
          <el-table-column prop="target_ip" label="IP" width="130" />
          <el-table-column prop="severity" label="级别" width="100" />
          <el-table-column prop="status" label="状态" width="100" />
          <el-table-column prop="source" label="来源" width="120" />
        </el-table>
      </el-tab-pane>
    </el-tabs>
  </div>
  <div v-else-if="markdownReport" class="markdown-body" v-html="renderMarkdown(markdownReport)"></div>
  <p v-else class="empty-hint">暂无报告数据</p>
</template>

<script setup>
import { computed } from 'vue'
import { renderMarkdown } from '../utils/renderMarkdown'

const props = defineProps({
  inspection: { type: Object, default: null },
  markdownReport: { type: String, default: '' }
})

const cleanSummary = computed(() => {
  const raw = props.inspection?.summary || ''
  if (!raw.includes('{') && !raw.includes('"evidence"')) return raw
  const lines = raw.split('\n').filter(line => {
    const t = line.trim()
    return t && !t.startsWith('{') && !t.startsWith('"') && !t.includes('{"') && !t.includes('"evidence"') && !t.includes('"status"')
  })
  if (lines.length) return lines.join(' ')
  const statusMap = { healthy: '整体健康', warning: '需要关注', critical: '存在风险' }
  const s = props.inspection?.status
  return `${props.inspection?.business_name || ''} 评分 ${props.inspection?.score || '--'}，${statusMap[s] || s || '未知'}`
})

const topFindings = computed(() => {
  const items = props.inspection?.topology_findings || []
  const seen = new Set()
  const out = []
  for (const raw of items) {
    const item = String(raw || '').trim()
    if (!item || seen.has(item) || item.startsWith('{') || item.includes('"evidence"')) continue
    seen.add(item)
    out.push(item)
    if (out.length >= 8) break
  }
  return out
})
</script>

<style scoped>
.inspection-dashboard { display:flex; flex-direction:column; gap:14px; }
.inspection-hero { border-radius:20px; padding:18px; display:flex; align-items:center; justify-content:space-between; color:#18304d; background:linear-gradient(135deg, rgba(37,189,113,.16), rgba(47,124,255,.12)); }
.inspection-hero.warning { background:linear-gradient(135deg, rgba(245,158,11,.18), rgba(47,124,255,.1)); }
.inspection-hero.critical { background:linear-gradient(135deg, rgba(239,84,84,.18), rgba(245,158,11,.12)); }
.inspection-hero b { font-size:14px; }
.inspection-hero h2 { margin:4px 0; font-size:42px; }
.inspection-hero p { margin:0; color:#5e718c; }
.inspection-kpis { display:grid; grid-template-columns:repeat(2, 1fr); gap:8px; }
.inspection-kpis span { border-radius:999px; background:rgba(255,255,255,.7); padding:8px 12px; font-size:12px; color:#34506f; }
.inspection-list { margin:0; padding-left:18px; line-height:1.9; color:#405875; }
.ai-advice { border-radius:18px; padding:13px 15px; margin-bottom:12px; background:linear-gradient(135deg, rgba(47,124,255,.12), rgba(37,189,113,.10)); border:1px solid rgba(47,124,255,.18); color:#294463; }
.ai-advice b { display:block; color:#1d4ed8; margin-bottom:6px; }
.ai-advice :deep(p) { margin:0 0 8px; line-height:1.6; }
.ai-advice :deep(ul) { margin:0; padding-left:18px; line-height:1.8; }
.ai-advice :deep(table) { border-collapse:collapse; width:100%; margin:10px 0; font-size:13px; background:rgba(255,255,255,.6); border-radius:10px; overflow:hidden; }
.ai-advice :deep(th) { background:rgba(47,124,255,.1); color:#1d4ed8; font-weight:700; text-align:left; padding:8px 10px; border:1px solid rgba(47,124,255,.15); }
.ai-advice :deep(td) { padding:7px 10px; border:1px solid rgba(70,93,130,.15); }
.ai-advice :deep(tr:hover td) { background:rgba(47,124,255,.04); }
.ai-advice :deep(h1) { font-size:20px; margin:18px 0 12px; color:#1a2332; border-bottom:2px solid rgba(47,124,255,.3); padding-bottom:8px; }
.ai-advice :deep(h2) { font-size:17px; margin:16px 0 10px; color:#243553; background:rgba(47,124,255,.06); padding:8px 12px; border-left:3px solid #2f7cff; border-radius:4px; }
.ai-advice :deep(h3) { font-size:15px; margin:14px 0 8px; color:#2c4a6e; }
.ai-advice :deep(h4) { font-size:14px; margin:12px 0 6px; color:#34506f; }
.ai-advice :deep(hr) { border:none; border-top:1px solid rgba(47,124,255,.15); margin:16px 0; }
.ai-advice :deep(code) { background:rgba(47,124,255,.08); color:#1d4ed8; padding:1px 5px; border-radius:4px; font-size:12px; }
.ai-advice :deep(ol) { margin:0; padding-left:18px; line-height:1.8; }
.attr-grid { display:grid; grid-template-columns:repeat(2, minmax(0, 1fr)); gap:10px; }
.attr-grid div { border-radius:14px; padding:10px; background:rgba(255,255,255,.7); border:1px solid rgba(216,226,246,.9); }
.attr-grid b,.attr-grid span { display:block; }
.attr-grid b { color:#2f7cff; margin-bottom:4px; }
.markdown-body { color:#253855; font-size:14px; line-height:1.75; background:rgba(255,255,255,.46); border:1px solid rgba(255,255,255,.66); border-radius:22px; padding:20px; }
.markdown-body :deep(h1),.markdown-body :deep(h2),.markdown-body :deep(h3),.markdown-body :deep(h4) { color:#172845; margin:18px 0 10px; line-height:1.35; }
.markdown-body :deep(a),.markdown-body :deep(code) { color:#176eff; }
.markdown-body :deep(strong) { color:#172845; }
.markdown-body :deep(pre) { background:rgba(34,48,74,.92); color:#f8fbff; border-radius:14px; padding:14px; overflow-x:auto; }
.markdown-body :deep(pre code) { color:#f8fbff; padding:0; background:transparent; }
.markdown-body :deep(table) { border-collapse:collapse; width:100%; margin:12px 0; }
.markdown-body :deep(th),.markdown-body :deep(td) { border:1px solid rgba(70,93,130,.22); padding:8px 10px; }
.markdown-body :deep(blockquote) { border-left:3px solid #247cff; background:rgba(255,255,255,.52); margin:12px 0; padding:8px 12px; border-radius:10px; }
.empty-hint { color:#94a3b8; text-align:center; padding:40px; }
</style>
