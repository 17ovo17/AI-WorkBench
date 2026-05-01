<template>
  <div class="reasoning">
    <button type="button" class="reasoning-head" @click="open = !open">
      AI 推理过程（{{ steps.length }} 步） {{ open ? '⌃' : '⌄' }}
    </button>
    <div v-if="open" class="reasoning-list">
      <div v-for="step in steps" :key="step.step" :class="['reasoning-step', step.status]">
        <span class="step-no">{{ step.step }}</span>
        <div>
          <b>{{ actionName(step.action) }}</b>
          <p>{{ step.inference || step.next_step || step.query || '' }}</p>
          <code>{{ compact(JSON.stringify(step.output || step.result || step.query || '', null, 2), 180) }}</code>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'

defineProps({
  steps: { type: Array, default: () => [] }
})

const open = ref(true)

const ACTION_LABELS = {
  entity_extraction: '实体提取',
  prometheus_query: 'Prometheus 查询',
  catpaw_query: 'Catpaw 交叉验证',
  root_cause_inference: '根因推断',
  business_inspection: '业务巡检',
  inspection_alive: '主机存活与采集状态',
  inspection_cpu: 'CPU 分项巡检',
  inspection_memory: '内存分项巡检',
  inspection_disk: '磁盘分项巡检',
  inspection_load: '负载分项巡检',
  inspection_network: '网络分项巡检',
  inspection_process: '进程分项巡检',
  topology_generate: '拓扑生成',
  user_report_extract: '上报提取'
}

const actionName = action => ACTION_LABELS[action] || action
const compact = (text, limit = 120) => text && text.length > limit ? `${text.slice(0, limit)}...` : text
</script>

<style scoped>
.reasoning { margin-top: 10px; border: 1px solid rgba(100, 116, 139, .18); border-radius: 14px; overflow: hidden; }
.reasoning-head { width: 100%; border: 0; padding: 9px 11px; text-align: left; background: rgba(37, 124, 255, .08); color: #1d4ed8; font-weight: 800; cursor: pointer; }
.reasoning-list { padding: 8px 10px; background: rgba(248, 250, 252, .62); }
.reasoning-step { display: grid; grid-template-columns: 22px 1fr; gap: 8px; margin: 7px 0; font-size: 12px; }
.step-no { width: 20px; height: 20px; border-radius: 50%; display: flex; align-items: center; justify-content: center; background: #dbeafe; color: #1d4ed8; font-weight: 800; }
.reasoning-step code { display: block; color: #64748b; white-space: pre-wrap; margin-top: 3px; }
</style>
