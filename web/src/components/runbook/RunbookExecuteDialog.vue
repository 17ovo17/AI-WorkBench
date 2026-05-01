<template>
  <el-dialog v-model="visible" title="执行 Runbook" width="640px" destroy-on-close>
    <div v-if="!executed" class="exec-form">
      <el-form :model="form" label-width="110px">
        <el-form-item label="Runbook">
          <span class="rb-title">{{ runbook?.title || '-' }}</span>
        </el-form-item>
        <el-form-item label="目标 IP" required>
          <el-input v-model="form.target_ip" placeholder="如 192.168.1.100" />
        </el-form-item>
        <template v-if="variableKeys.length">
          <el-divider content-position="left">模板变量</el-divider>
          <el-form-item v-for="v in variableKeys" :key="v.key" :label="v.label">
            <el-input v-model="form.variables[v.key]" :placeholder="v.default || ''" />
          </el-form-item>
        </template>
      </el-form>
    </div>

    <div v-else class="exec-result">
      <el-result
        :icon="resultIcon"
        :title="resultTitle"
      />
      <div v-if="resultOutput" class="output-block">
        <div class="output-label">执行输出</div>
        <pre class="output-pre">{{ resultOutput }}</pre>
      </div>
    </div>

    <template #footer>
      <el-button @click="visible = false">关闭</el-button>
      <el-button v-if="!executed" type="primary" :loading="executing" @click="doExecute">
        执行
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'

const props = defineProps({ modelValue: Boolean, runbook: Object })
const emit = defineEmits(['update:modelValue'])

const visible = computed({ get: () => props.modelValue, set: v => emit('update:modelValue', v) })

const form = ref({ target_ip: '', variables: {} })
const executing = ref(false)
const executed = ref(false)
const resultStatus = ref('')
const resultOutput = ref('')

const variableKeys = computed(() => {
  const vars = props.runbook?.variables
  if (!vars || typeof vars !== 'object') return []
  return Object.entries(vars).map(([key, def]) => ({
    key,
    label: def?.label || key,
    default: def?.default || '',
  }))
})

const resultIcon = computed(() => {
  if (resultStatus.value === 'succeeded') return 'success'
  if (resultStatus.value === 'failed') return 'error'
  return 'info'
})

const resultTitle = computed(() => {
  if (resultStatus.value === 'succeeded') return '执行成功'
  if (resultStatus.value === 'failed') return '执行失败'
  return '执行完成'
})

watch(() => props.modelValue, (open) => {
  if (!open) return
  executed.value = false
  resultStatus.value = ''
  resultOutput.value = ''
  const vars = {}
  for (const v of variableKeys.value) {
    vars[v.key] = v.default
  }
  form.value = { target_ip: '', variables: vars }
})

const doExecute = async () => {
  if (!form.value.target_ip.trim()) {
    ElMessage.warning('请输入目标 IP')
    return
  }
  executing.value = true
  try {
    const { data } = await axios.post(
      `/api/v1/knowledge/runbooks/${props.runbook.id}/execute`,
      { target_ip: form.value.target_ip, variables: form.value.variables }
    )
    resultStatus.value = data.status || 'succeeded'
    resultOutput.value = data.output || JSON.stringify(data, null, 2)
    executed.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '执行失败')
    resultStatus.value = 'failed'
    resultOutput.value = e.response?.data?.error || e.message
    executed.value = true
  } finally {
    executing.value = false
  }
}
</script>

<style scoped>
.rb-title { font-weight: 700; color: #243553; }
.exec-result { display: flex; flex-direction: column; gap: 12px; }
.output-block { background: rgba(255,255,255,.42); border-radius: 12px; padding: 14px 16px; border: 1px solid rgba(255,255,255,.6); }
.output-label { font-weight: 700; font-size: 13px; color: #243553; margin-bottom: 8px; }
.output-pre { font-family: ui-monospace, monospace; font-size: 12px; background: rgba(35,53,83,.06); border-radius: 10px; padding: 10px 12px; max-height: 300px; overflow: auto; white-space: pre-wrap; word-break: break-word; color: #2c3a55; }
</style>
