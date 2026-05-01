<template>
  <div v-if="data" class="detail">
    <div class="detail-row"><span class="lbl">分类：</span><el-tag size="small">{{ data.category }}</el-tag></div>
    <div v-if="data.version" class="detail-row"><span class="lbl">版本：</span>{{ data.version }}</div>
    <div v-if="data.severity" class="detail-row">
      <span class="lbl">严重级别：</span>
      <el-tag :type="severityColor(data.severity)" size="small">{{ data.severity }}</el-tag>
    </div>
    <div v-if="data.estimated_time" class="detail-row">
      <span class="lbl">预计时间：</span>{{ data.estimated_time }}
    </div>
    <div class="detail-row">
      <span class="lbl">触发条件：</span>
      <pre class="json-pre">{{ data.trigger_conditions }}</pre>
    </div>
    <div v-if="data.prerequisites" class="detail-row">
      <span class="lbl">前置条件：</span>
      <pre class="md-pre">{{ data.prerequisites }}</pre>
    </div>
    <div class="detail-row"><span class="lbl">处置步骤：</span></div>
    <pre class="md-pre">{{ data.steps }}</pre>
    <div v-if="data.rollback_steps" class="detail-row">
      <span class="lbl">回滚步骤：</span>
      <pre class="md-pre rollback-pre">{{ data.rollback_steps }}</pre>
    </div>
  </div>
</template>

<script setup>
defineProps({ data: { type: Object, default: null } })

const SEVERITY_COLORS = { critical: 'danger', high: 'warning', medium: '', low: 'success' }
const severityColor = (s) => SEVERITY_COLORS[s] ?? 'info'
</script>

<style scoped>
.detail { padding: 4px 0; }
.detail-row { margin-bottom: 12px; }
.lbl { font-weight: 700; color: var(--ink); margin-right: 8px; }
.json-pre { background: rgba(255,255,255,.5); border-radius: 12px; padding: 10px 14px; font-size: 12px; white-space: pre-wrap; word-break: break-all; margin: 6px 0 0; }
.md-pre { background: rgba(255,255,255,.5); border-radius: 12px; padding: 14px 18px; font-size: 12.5px; line-height: 1.65; white-space: pre-wrap; max-height: 50vh; overflow-y: auto; margin: 6px 0 0; }
.rollback-pre { border-left: 3px solid #ff9f43; }
</style>
