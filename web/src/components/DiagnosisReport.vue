<template>
  <div class="report-card glass-panel">
    <div class="report-head">
      <div>
        <div class="panel-kicker">Diagnosis Report</div>
        <h3 class="report-title">诊断报告</h3>
      </div>
      <div class="report-meta">
        <el-tag :type="confidenceType" size="small">置信度 {{ confidenceLabel }}</el-tag>
        <el-tag v-if="report.root_cause_category" :type="categoryType(report.root_cause_category)" size="small">
          {{ report.root_cause_category }}
        </el-tag>
      </div>
    </div>

    <div class="section">
      <div class="section-title">根因结论</div>
      <p class="root-cause">{{ report.root_cause_description || '（未给出根因描述）' }}</p>
    </div>

    <div v-if="evidenceRows.length" class="section">
      <div class="section-title">证据指标</div>
      <el-table :data="evidenceRows" size="small" stripe style="width:100%">
        <el-table-column prop="metric" label="指标" min-width="160" />
        <el-table-column prop="current_value" label="当前值" width="140" />
        <el-table-column prop="threshold" label="阈值" width="120" />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">{{ row.status || '-' }}</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div v-if="treatmentList.length" class="section">
      <div class="section-title">处置步骤</div>
      <ol class="treatment-list">
        <li v-for="(step, i) in treatmentList" :key="i">{{ step }}</li>
      </ol>
    </div>

    <div v-if="verifyQueries.length" class="section">
      <el-collapse>
        <el-collapse-item title="验证查询（PromQL）" name="v">
          <div v-for="(q, i) in verifyQueries" :key="i" class="verify-row">
            <code>{{ q }}</code>
            <el-button size="small" link @click="copy(q)">复制</el-button>
          </div>
        </el-collapse-item>
      </el-collapse>
    </div>

    <div class="footer-actions">
      <el-button type="success" plain :icon="CircleCheck" :disabled="feedbackSent" @click="sendFeedback('accurate')">准确</el-button>
      <el-button type="warning" plain :icon="Warning" :disabled="feedbackSent" @click="sendFeedback('partial')">部分准确</el-button>
      <el-button type="danger" plain :icon="CircleClose" :disabled="feedbackSent" @click="sendFeedback('inaccurate')">不准确</el-button>
      <el-button :icon="Files" @click="$emit('archive')">归档为案例</el-button>
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { CircleCheck, CircleClose, Warning, Files } from '@element-plus/icons-vue'
import { categoryType } from './caseHelpers.js'

const props = defineProps({ report: { type: Object, required: true } })
const emit = defineEmits(['archive', 'feedback'])

const feedbackSent = ref(false)

const confidenceLabel = computed(() => (props.report.confidence || '').toUpperCase() || 'N/A')
const confidenceType = computed(() => {
  const c = confidenceLabel.value
  if (c === 'HIGH') return 'success'
  if (c === 'MEDIUM') return 'warning'
  if (c === 'LOW') return 'danger'
  return 'info'
})

const evidenceRows = computed(() => Array.isArray(props.report.evidence) ? props.report.evidence : [])
const treatmentList = computed(() => {
  const t = props.report.treatment_steps
  if (Array.isArray(t)) return t.filter(Boolean)
  if (typeof t === 'string') return t.split('\n').map(s => s.trim()).filter(Boolean)
  return []
})
const verifyQueries = computed(() => Array.isArray(props.report.verify_queries) ? props.report.verify_queries : [])

const statusType = (s) => {
  const v = String(s || '').toLowerCase()
  if (v === 'critical' || v === 'fail' || v === 'fired') return 'danger'
  if (v === 'warn' || v === 'warning' || v === 'degraded') return 'warning'
  if (v === 'ok' || v === 'normal' || v === 'pass') return 'success'
  return 'info'
}

const copy = async (text) => {
  try { await navigator.clipboard.writeText(text); ElMessage.success('已复制') }
  catch { ElMessage.warning('复制失败，请手动选择') }
}

const sendFeedback = (kind) => {
  feedbackSent.value = true
  emit('feedback', kind)
}
</script>

<style scoped>
.glass-panel { background: linear-gradient(145deg, rgba(255,255,255,.58), rgba(225,236,255,.42)); border: 1px solid rgba(255,255,255,.72); border-radius: 24px; box-shadow: 0 20px 54px rgba(63,100,160,.16), inset 0 1px 0 rgba(255,255,255,.78); backdrop-filter: blur(24px); }
.report-card { padding: 22px 26px; display: flex; flex-direction: column; gap: 16px; }
.report-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; flex-wrap: wrap; }
.report-title { margin: 4px 0 0; font-size: 22px; color: #263653; letter-spacing: -.02em; }
.panel-kicker { font-size: 12px; color: #247cff; font-weight: 800; letter-spacing: .06em; text-transform: uppercase; }
.report-meta { display: flex; gap: 8px; flex-wrap: wrap; }
.section { background: rgba(255,255,255,.42); border: 1px solid rgba(255,255,255,.6); border-radius: 16px; padding: 14px 18px; }
.section-title { font-size: 13px; color: #243553; font-weight: 800; margin-bottom: 8px; }
.root-cause { font-size: 14px; color: #2d3e5d; line-height: 1.7; }
.treatment-list { padding-left: 22px; color: #3a4a6a; font-size: 13px; line-height: 1.9; }
.verify-row { display: flex; align-items: center; gap: 12px; padding: 6px 0; border-bottom: 1px dashed rgba(120,140,180,.18); }
.verify-row code { flex: 1; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; color: #2c3a55; word-break: break-all; }
.footer-actions { display: flex; gap: 10px; flex-wrap: wrap; }
</style>
