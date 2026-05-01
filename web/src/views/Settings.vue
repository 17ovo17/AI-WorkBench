<template>
  <div class="settings-page">
    <div class="page-head"><div class="panel-kicker">Settings</div><h2>系统配置</h2></div>
    <div class="settings-grid">
      <div class="panel-card">
        <div class="panel-title">AI 模型配置</div>
        <el-form :model="form" label-width="100px" size="small">
          <el-form-item label="API 地址">
            <el-input v-model="form.base_url" placeholder="https://api.openai.com/v1（不含 /chat/completions）" />
          </el-form-item>
          <el-form-item label="API Key">
            <el-input v-model="form.api_key" type="password" show-password placeholder="sk-..." />
          </el-form-item>
          <el-form-item label="可用模型">
            <div v-for="(m, i) in form.models" :key="i" class="model-row">
              <el-input v-model="form.models[i]" placeholder="gpt-4o" size="small" style="flex:1" />
              <el-button size="small" @click="form.models.splice(i,1)">-</el-button>
            </div>
            <el-button size="small" @click="form.models.push('')">+ 添加模型</el-button>
          </el-form-item>
          <el-form-item label="默认模型">
            <el-select v-model="form.default_model" style="width:100%">
              <el-option v-for="m in form.models.filter(Boolean)" :key="m" :label="m" :value="m" />
            </el-select>
          </el-form-item>
          <el-form-item label="诊断模型">
            <el-select v-model="form.diagnose_model" style="width:100%">
              <el-option value="" label="同默认模型" />
              <el-option v-for="m in form.models.filter(Boolean)" :key="m" :label="m" :value="m" />
            </el-select>
          </el-form-item>
        </el-form>
      </div>

      <div class="panel-card" style="margin-top:16px">
        <div class="panel-title">Prometheus 配置</div>
        <el-form :model="form" label-width="100px" size="small">
          <el-form-item label="地址">
            <el-input v-model="form.prometheus_url" placeholder="http://localhost:9090" />
          </el-form-item>
          <el-form-item label="Instance格式">
            <el-input v-model="form.instance_format" placeholder="{ip}:9100" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="saving" @click="save">保存配置</el-button>
            <el-button @click="load">重置</el-button>
          </el-form-item>
        </el-form>
        <el-alert v-if="msg" :title="msg" :type="msgType" show-icon style="margin-top:12px" />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'

const form = ref({ base_url: '', api_key: '', models: [''], default_model: '', diagnose_model: '', prometheus_url: '', instance_format: '' })
const saving = ref(false)
const msg = ref('')
const msgType = ref('success')

const load = async () => {
  const { data } = await axios.get('/api/v1/settings')
  form.value = { ...form.value, ...data, models: data.models?.length ? data.models : [data.default_model || ''] }
}

const save = async () => {
  saving.value = true
  try {
    await axios.post('/api/v1/settings', { ...form.value, models: form.value.models.filter(Boolean) })
    msg.value = '保存成功，配置已生效'
    msgType.value = 'success'
  } catch (e) {
    msg.value = e.response?.data?.error || '保存失败'
    msgType.value = 'error'
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.settings-page { padding: 24px; background: #0d1117; min-height: 100vh; color: #e6edf3; }
.page-head { margin-bottom: 24px; }
.page-head h2 { margin: 0; font-size: 18px; }
.panel-kicker { font-size: 10px; color: #58a6ff; text-transform: uppercase; letter-spacing: 1px; margin-bottom: 2px; }
.settings-grid { max-width: 600px; }
.panel-card { background: #161b22; border: 1px solid #30363d; border-radius: 10px; padding: 20px; }
.panel-title { font-size: 12px; font-weight: 600; color: #8b949e; text-transform: uppercase; letter-spacing: 1px; margin-bottom: 16px; }
.model-row { display: flex; gap: 6px; margin-bottom: 6px; }
</style>
