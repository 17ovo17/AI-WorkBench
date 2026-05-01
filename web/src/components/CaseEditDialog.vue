<template>
  <el-dialog v-model="visible" :title="dialogTitle" width="640px" destroy-on-close @close="handleClose">
    <el-form ref="formRef" :model="form" :rules="rules" label-width="110px" size="default">
      <el-form-item label="根因分类" prop="root_cause_category">
        <el-select v-model="form.root_cause_category" placeholder="选择根因分类" style="width:100%">
          <el-option v-for="c in CATEGORIES" :key="c" :label="c" :value="c" />
        </el-select>
      </el-form-item>
      <el-form-item label="根因描述" prop="root_cause_description">
        <el-input v-model="form.root_cause_description" type="textarea" :rows="2" placeholder="一句话描述根因" />
      </el-form-item>
      <el-form-item label="关键词" prop="keywords">
        <el-input v-model="form.keywords" placeholder="逗号分隔，如：cpu,过载,sar" />
      </el-form-item>
      <el-form-item label="处置步骤" prop="treatment_steps">
        <el-input v-model="form.treatment_steps" type="textarea" :rows="5" placeholder="每行一步处置动作" />
      </el-form-item>
      <el-form-item label="指标快照" prop="metric_snapshot">
        <el-input v-model="form.metric_snapshot" type="textarea" :rows="6" placeholder='例如：{"cpu_usage": 95.6, "load1": 12.3}' />
        <div class="form-tip">JSON 对象格式，提交时会校验</div>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submit">{{ isEdit ? '更新' : '创建' }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import axios from 'axios'
import { CATEGORIES } from './caseHelpers.js'

const props = defineProps({ modelValue: Boolean, editing: Object })
const emit = defineEmits(['update:modelValue', 'saved'])

const visible = computed({ get: () => props.modelValue, set: v => emit('update:modelValue', v) })
const isEdit = computed(() => !!props.editing?.id)
const dialogTitle = computed(() => isEdit.value ? '编辑案例' : '新建案例')

const formRef = ref(null)
const submitting = ref(false)
const form = ref(emptyForm())

function emptyForm() {
  return { id: '', root_cause_category: '', root_cause_description: '', keywords: '', treatment_steps: '', metric_snapshot: '{}' }
}

const rules = {
  root_cause_category: [{ required: true, message: '请选择根因分类', trigger: 'change' }],
  root_cause_description: [{ required: true, message: '请输入根因描述', trigger: 'blur' }],
  treatment_steps: [{ required: true, message: '请输入处置步骤', trigger: 'blur' }],
  metric_snapshot: [{ required: true, message: '请输入指标快照（JSON）', trigger: 'blur' }]
}

watch(() => [props.modelValue, props.editing], ([open, editing]) => {
  if (!open) return
  if (editing && editing.id) {
    const snap = editing.metric_snapshot
    let snapText = '{}'
    try { snapText = typeof snap === 'string' ? snap : JSON.stringify(snap || {}, null, 2) } catch {}
    form.value = {
      id: editing.id,
      root_cause_category: editing.root_cause_category || '',
      root_cause_description: editing.root_cause_description || '',
      keywords: editing.keywords || '',
      treatment_steps: editing.treatment_steps || '',
      metric_snapshot: snapText
    }
  } else {
    form.value = emptyForm()
  }
}, { immediate: true })

const handleClose = () => { formRef.value?.resetFields?.() }

const submit = async () => {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return
  let snapshotObj
  try { snapshotObj = JSON.parse(form.value.metric_snapshot || '{}') } catch {
    ElMessage.error('指标快照不是合法 JSON')
    return
  }
  submitting.value = true
  const payload = {
    root_cause_category: form.value.root_cause_category,
    root_cause_description: form.value.root_cause_description,
    keywords: form.value.keywords,
    treatment_steps: form.value.treatment_steps,
    metric_snapshot: snapshotObj
  }
  try {
    if (isEdit.value) {
      await axios.put(`/api/v1/knowledge/cases/${form.value.id}`, payload)
      ElMessage.success('案例已更新')
    } else {
      await axios.post('/api/v1/knowledge/cases', payload)
      ElMessage.success('案例已创建')
    }
    visible.value = false
    emit('saved')
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '保存失败')
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.form-tip { font-size: 12px; color: #98a3b8; margin-top: 4px; }
</style>
