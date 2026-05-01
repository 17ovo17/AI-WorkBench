<template>
  <div class="llm-panel">
    <div class="providers-list">
      <div v-for="(p, i) in providers" :key="p.id" class="provider-card panel-card">
        <div class="card-head">
          <el-input v-model="p.name" placeholder="服务名称" size="small" style="width:160px" />
          <el-tag v-if="p.default" type="success" size="small">默认</el-tag>
          <span class="health-dot" :class="healthMap[p.id] === true ? 'alive' : healthMap[p.id] === false ? 'dead' : ''"></span>
          <div class="card-actions">
            <el-button size="small" @click="setDefault(i)">设为默认</el-button>
            <el-button size="small" type="danger" plain @click="removeProvider(i)">删除</el-button>
          </div>
        </div>
        <el-form :model="p" label-width="80px" size="small" style="margin-top:12px">
          <el-form-item label="API 地址">
            <el-input v-model="p.base_url" placeholder="https://api.openai.com/v1" />
          </el-form-item>
          <el-form-item label="API Key">
            <el-input v-model="p.api_key" type="password" show-password placeholder="sk-..." />
          </el-form-item>
          <el-form-item label="模型列表">
            <div v-for="(m, j) in p.models" :key="j" class="model-row">
              <el-input v-model="p.models[j]" placeholder="gpt-4o" size="small" style="flex:1" />
              <el-button size="small" @click="removeModel(p, j)">-</el-button>
            </div>
            <el-button size="small" @click="p.models.push('')">+ 添加模型</el-button>
          </el-form-item>
        </el-form>
      </div>
    </div>

    <div class="bottom-actions">
      <el-button @click="addProvider">+ 添加 AI 服务</el-button>
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
    </div>
    <el-alert v-if="msg" :title="msg" :type="msgType" show-icon style="margin-top:12px;max-width:700px" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { ElMessageBox } from 'element-plus'

const providers = ref([])
const saving = ref(false)
const msg = ref('')
const msgType = ref('success')
const healthMap = ref({})
const normalizeList = data => Array.isArray(data) ? data : (Array.isArray(data?.value) ? data.value : [])
const load = async () => {
  const { data } = await axios.get('/api/v1/ai-providers')
  providers.value = normalizeList(data).map(p => ({
    ...p, base_url: p.base_url || p.baseurl || '', api_key: p.api_key || p.apikey || '', models: p.models || []
  }))
  const { data: health } = await axios.get('/api/v1/health/ai-providers').catch(() => ({ data: [] }))
  const map = {}
  ;(health || []).forEach(h => { map[h.id] = h.alive })
  healthMap.value = map
}

const addProvider = () => {
  providers.value.push({ id: Date.now().toString(), name: '新服务', base_url: '', api_key: '', models: [''], default: false })
}
const setDefault = (i) => { providers.value.forEach((p, j) => p.default = j === i) }

const removeProvider = async (i) => {
  const name = providers.value[i]?.name || '该 AI 服务'
  await ElMessageBox.confirm(`确认删除 ${name}？删除后需点击保存才会生效。`, '二次确认', { type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消' })
  providers.value.splice(i, 1)
}

const removeModel = async (provider, index) => {
  const name = provider.models[index] || '该模型'
  await ElMessageBox.confirm(`确认删除模型 ${name}？删除后需点击保存才会生效。`, '二次确认', { type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消' })
  provider.models.splice(index, 1)
}

const save = async () => {
  saving.value = true
  try {
    await axios.post('/api/v1/ai-providers', providers.value.map(p => ({ ...p, models: p.models.filter(Boolean) })))
    msg.value = '保存成功'; msgType.value = 'success'
  } catch (e) {
    msg.value = e.response?.data?.error || '保存失败'; msgType.value = 'error'
  } finally { saving.value = false }
}

onMounted(load)
</script>

<style scoped>
.llm-panel { padding: 4px 0; }
.providers-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(330px, 380px)); gap: 14px; align-items: start; max-height: calc(100vh - 240px); overflow: auto; padding-right: 6px; }
.panel-card { background: linear-gradient(145deg, rgba(255,255,255,.58), rgba(225,236,255,.42)); border: 1px solid rgba(255,255,255,.72); border-radius: 24px; padding: 16px; box-shadow: 0 20px 54px rgba(63,100,160,.16), inset 0 1px 0 rgba(255,255,255,.78); backdrop-filter: blur(24px); }
.card-head { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; flex-wrap: wrap; }
.health-dot { width: 10px; height: 10px; border-radius: 50%; background: #9aa9be; flex-shrink: 0; }
.health-dot.alive { background: #36d08a; box-shadow: 0 0 0 5px rgba(54,208,138,.12); }
.health-dot.dead { background: #ff5b6b; box-shadow: 0 0 0 5px rgba(255,91,107,.12); }
.card-actions { margin-left: auto; display: flex; gap: 6px; }
.model-row { display: flex; gap: 8px; margin-bottom: 8px; width: 100%; }
.bottom-actions { display: flex; gap: 12px; margin-top: 14px; }
:deep(.el-form) { margin-top: 10px !important; }
:deep(.el-form-item) { margin-bottom: 10px; }
:deep(.el-form-item__label) { width: 64px !important; color: #60728e; font-weight: 700; font-size: 12px; }
:deep(.el-form-item__content) { margin-left: 64px !important; }
</style>
