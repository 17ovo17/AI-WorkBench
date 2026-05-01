<template>
  <el-dialog v-model="visible" title="执行历史" width="700px" destroy-on-close>
    <el-table :data="historyList" v-loading="loading" stripe size="small" empty-text="暂无执行记录">
      <el-table-column prop="id" label="运行 ID" width="200" show-overflow-tooltip />
      <el-table-column prop="status" label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 'succeeded' ? 'success' : row.status === 'failed' ? 'danger' : 'info'" size="small">
            {{ row.status }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="elapsed_ms" label="耗时(ms)" width="100" />
      <el-table-column prop="created_at" label="时间" min-width="180">
        <template #default="{ row }">{{ formatTs(row.created_at) }}</template>
      </el-table-column>
    </el-table>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'

const props = defineProps({ modelValue: Boolean, workflowId: String })
const emit = defineEmits(['update:modelValue'])

const visible = computed({ get: () => props.modelValue, set: v => emit('update:modelValue', v) })
const historyList = ref([])
const loading = ref(false)

const formatTs = (ts) => {
  if (!ts) return '-'
  const d = new Date(typeof ts === 'number' ? ts * 1000 : ts)
  return d.toLocaleString('zh-CN')
}

watch(() => props.modelValue, async (open) => {
  if (!open || !props.workflowId) return
  loading.value = true
  try {
    historyList.value = (await axios.get(`/api/v1/workflows/${props.workflowId}/runs`)).data || []
  } catch (e) {
    ElMessage.error('获取执行历史失败')
    historyList.value = []
  } finally {
    loading.value = false
  }
})
</script>
