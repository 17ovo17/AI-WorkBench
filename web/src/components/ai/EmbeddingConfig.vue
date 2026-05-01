<template>
  <div class="embed-panel">
    <el-form :model="form" label-width="120px" size="default" style="max-width:600px">
      <el-form-item label="Provider">
        <el-select v-model="form.provider" style="width:100%">
          <el-option value="builtin" label="内置 BM25" />
          <el-option value="api" label="外部 API" />
          <el-option value="hybrid" label="混合模式 (BM25 + API)" />
        </el-select>
      </el-form-item>

      <template v-if="form.provider === 'api' || form.provider === 'hybrid'">
        <el-form-item label="API URL">
          <el-input v-model="form.api_url" placeholder="https://api.openai.com/v1/embeddings" />
        </el-form-item>
        <el-form-item label="API Key">
          <el-input v-model="form.api_key" type="password" show-password placeholder="sk-..." />
        </el-form-item>
        <el-form-item label="模型名称">
          <el-input v-model="form.model" placeholder="text-embedding-3-small" />
        </el-form-item>
        <el-form-item label="向量维度">
          <el-input-number v-model="form.dimensions" :min="64" :max="4096" :step="64" />
        </el-form-item>
      </template>

      <el-form-item label-width="0">
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
        <el-button
          v-if="form.provider !== 'builtin'"
          :loading="testing"
          @click="testConnection"
        >测试连接</el-button>
      </el-form-item>
    </el-form>

    <el-alert v-if="msg" :title="msg" :type="msgType" show-icon style="margin-top:12px;max-width:600px" />
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'

const form = reactive({
  provider: 'builtin',
  api_url: '',
  api_key: '',
  model: '',
  dimensions: 1536,
})
const saving = ref(false)
const testing = ref(false)
const msg = ref('')
const msgType = ref('success')

const load = async () => {
  try {
    const { data } = await axios.get('/api/v1/settings/embedding')
    if (data) {
      form.provider = data.provider || 'builtin_bm25'
      form.api_url = data.api_url || ''
      form.api_key = data.api_key || ''
      form.model = data.model || ''
      form.dimensions = data.dimensions || 1536
    }
  } catch { /* use defaults */ }
}

const save = async () => {
  saving.value = true
  try {
    await axios.put('/api/v1/settings/embedding', { ...form })
    msg.value = '保存成功'; msgType.value = 'success'
  } catch (e) {
    msg.value = e.response?.data?.error || '保存失败'; msgType.value = 'error'
  } finally { saving.value = false }
}

const testConnection = async () => {
  testing.value = true
  try {
    const { data } = await axios.post('/api/v1/settings/embedding/test', {
      api_url: form.api_url, api_key: form.api_key, model: form.model,
    })
    ElMessage.success(data?.message || '连接成功')
    msg.value = '测试通过'; msgType.value = 'success'
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '连接失败')
    msg.value = e.response?.data?.error || '测试失败'; msgType.value = 'error'
  } finally { testing.value = false }
}

onMounted(load)
</script>

<style scoped>
.embed-panel { padding: 12px 0; }
</style>
