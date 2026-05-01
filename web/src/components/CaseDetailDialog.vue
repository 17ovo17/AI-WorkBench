<template>
  <el-dialog v-model="visible" :title="title" width="720px" destroy-on-close>
    <div v-if="data" class="detail-body">
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="根因分类">
          <el-tag :type="categoryType(data.root_cause_category)">{{ data.root_cause_category }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ formatTime(data.created_at) }}</el-descriptions-item>
        <el-descriptions-item label="创建人">{{ data.created_by || '-' }}</el-descriptions-item>
        <el-descriptions-item label="评分均值">{{ data.evaluation_avg?.toFixed?.(2) ?? '-' }}</el-descriptions-item>
        <el-descriptions-item label="根因描述" :span="2">{{ data.root_cause_description }}</el-descriptions-item>
        <el-descriptions-item label="关键词" :span="2">
          <el-tag v-for="kw in keywordList" :key="kw" size="small" effect="plain" style="margin-right:4px">{{ kw }}</el-tag>
        </el-descriptions-item>
      </el-descriptions>

      <div class="detail-section">
        <div class="section-title">处置步骤</div>
        <ol v-if="treatmentList.length" class="treatment-list">
          <li v-for="(step, i) in treatmentList" :key="i">{{ step }}</li>
        </ol>
        <div v-else class="empty">无处置步骤</div>
      </div>

      <div class="detail-section">
        <div class="section-title">指标快照</div>
        <pre class="snapshot-pre">{{ snapshotText }}</pre>
      </div>
    </div>
  </el-dialog>
</template>

<script setup>
import { computed } from 'vue'
import { categoryType } from './caseHelpers.js'

const props = defineProps({ modelValue: Boolean, data: Object })
const emit = defineEmits(['update:modelValue'])

const visible = computed({ get: () => props.modelValue, set: v => emit('update:modelValue', v) })
const title = computed(() => props.data ? `案例详情 - ${props.data.id}` : '案例详情')

const keywordList = computed(() => (props.data?.keywords || '').split(',').map(s => s.trim()).filter(Boolean))
const treatmentList = computed(() => (props.data?.treatment_steps || '').split('\n').map(s => s.trim()).filter(Boolean))
const snapshotText = computed(() => {
  const snap = props.data?.metric_snapshot
  if (!snap) return '{}'
  try { return JSON.stringify(typeof snap === 'string' ? JSON.parse(snap) : snap, null, 2) } catch { return String(snap) }
})

const formatTime = (iso) => {
  if (!iso) return '-'
  try { return new Date(iso).toLocaleString('zh-CN', { hour12: false }) } catch { return iso }
}
</script>

<style scoped>
.detail-body { display: flex; flex-direction: column; gap: 16px; }
.detail-section { background: rgba(255,255,255,.42); border-radius: 14px; padding: 14px 16px; border: 1px solid rgba(255,255,255,.6); }
.section-title { font-weight: 800; color: #243553; margin-bottom: 8px; font-size: 13px; }
.treatment-list { padding-left: 22px; color: #3a4a6a; font-size: 13px; line-height: 1.8; }
.snapshot-pre { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; background: rgba(35,53,83,.06); border-radius: 10px; padding: 10px 12px; max-height: 320px; overflow: auto; color: #2c3a55; white-space: pre-wrap; word-break: break-word; }
.empty { color: #98a3b8; font-size: 12px; }
</style>
