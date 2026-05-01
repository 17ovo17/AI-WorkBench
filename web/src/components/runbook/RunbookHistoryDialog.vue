<template>
  <el-dialog v-model="visible" title="执行历史" width="760px" destroy-on-close>
    <el-table :data="historyList" v-loading="loading" stripe size="small" empty-text="暂无执行记录">
      <el-table-column label="执行时间" width="180">
        <template #default="{ row }">{{ formatTime(row.executed_at || row.created_at) }}</template>
      </el-table-column>
      <el-table-column prop="executor" label="执行者" width="120" show-overflow-tooltip />
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="statusColor(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="target_ip" label="目标 IP" width="150" />
      <el-table-column prop="duration_ms" label="耗时" width="100">
        <template #default="{ row }">{{ row.duration_ms ? `${row.duration_ms}ms` : '-' }}</template>
      </el-table-column>
      <el-table-column label="输出" min-width="200" show-overflow-tooltip>
        <template #default="{ row }">{{ row.output || '-' }}</template>
      </el-table-column>
    </el-table>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'

const props = defineProps({ modelValue: Boolean, runbookId: String })
const emit = defineEmits(['update:modelValue'])

const visible = computed({ get: () => props.modelValue, set: v => emit('update:modelValue', v) })
const historyList = ref([])
const loading = ref(false)

const STATUS_MAP = {
  succeeded: { color: 'success', label: '成功' },
  failed: { color: 'danger', label: '失败' },
  running: { color: 'primary', label: '执行中' },
  manual: { color: 'warning', label: '手动' },
}

const statusColor = (s) => STATUS_MAP[s]?.color ?? 'info'
const statusLabel = (s) => STATUS_MAP[s]?.label ?? s ?? '-'

const formatTime = (iso) => {
  if (!iso) return '-'
  try { return new Date(iso).toLocaleString('zh-CN', { hour12: false }) } catch { return iso }
}

watch(() => props.modelValue, async (open) => {
  if (!open || !props.runbookId) return
  loading.value = true
  try {
    const { data } = await axios.get(`/api/v1/knowledge/runbooks/${props.runbookId}/history`)
    historyList.value = data.items || data || []
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '获取执行历史失败')
    historyList.value = []
  } finally {
    loading.value = false
  }
})
</script>
