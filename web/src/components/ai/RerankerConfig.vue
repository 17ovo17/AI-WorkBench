<template>
  <div class="reranker-panel">
    <el-form :model="form" label-width="120px" size="default" style="max-width:600px">
      <el-form-item label="启用 Reranker">
        <el-switch v-model="form.enabled" />
      </el-form-item>

      <template v-if="form.enabled">
        <el-form-item label="Provider">
          <el-select v-model="form.provider" style="width:100%">
            <el-option value="llm" label="LLM 重排" />
            <el-option value="api" label="外部 API" />
          </el-select>
        </el-form-item>

        <template v-if="form.provider === 'api'">
          <el-form-item label="API URL">
            <el-input v-model="form.api_url" placeholder="https://api.cohere.ai/v1/rerank" />
          </el-form-item>
          <el-form-item label="API Key">
            <el-input v-model="form.api_key" type="password" show-password />
          </el-form-item>
          <el-form-item label="模型名称">
            <el-input v-model="form.model" placeholder="rerank-english-v3.0" />
          </el-form-item>
        </template>

        <el-form-item label="Top-K">
          <el-input-number v-model="form.top_k" :min="1" :max="50" />
        </el-form-item>
      </template>

      <el-form-item label-width="0">
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </el-form-item>
    </el-form>

    <el-alert v-if="msg" :title="msg" :type="msgType" show-icon style="margin-top:12px;max-width:600px" />
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import axios from 'axios'

const form = reactive({
  enabled: false,
  provider: 'llm',
  api_url: '',
  api_key: '',
  model: '',
  top_k: 5,
})
const saving = ref(false)
const msg = ref('')
const msgType = ref('success')

const load = async () => {
  try {
    const { data } = await axios.get('/api/v1/settings/reranker')
    if (data) {
      form.enabled = !!data.enabled
      form.provider = data.provider || 'llm'
      form.api_url = data.api_url || ''
      form.api_key = data.api_key || ''
      form.model = data.model || ''
      form.top_k = data.top_k || 5
    }
  } catch { /* use defaults */ }
}

const save = async () => {
  saving.value = true
  try {
    await axios.put('/api/v1/settings/reranker', { ...form })
    msg.value = '保存成功'; msgType.value = 'success'
  } catch (e) {
    msg.value = e.response?.data?.error || '保存失败'; msgType.value = 'error'
  } finally { saving.value = false }
}

onMounted(load)
</script>

<style scoped>
.reranker-panel { padding: 12px 0; }
</style>
