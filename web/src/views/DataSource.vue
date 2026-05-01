<template>
  <div class="config-page">
    <div class="page-head">
      <div class="panel-kicker">Data Sources</div>
      <h2>数据源配置</h2>
      <p>平台 MySQL 会自动从后端运行配置接入，只读展示并参与健康检查，不会保存或暴露密码。</p>
    </div>

    <div class="ds-list">
      <div v-for="(ds, i) in sources" :key="ds.id" class="panel-card" :class="{ builtin: isPlatformMySQL(ds) }">
        <div class="card-head">
          <el-input v-model="ds.name" placeholder="数据源名称" size="small" style="width:160px" :disabled="isPlatformMySQL(ds)" />
          <el-select v-model="ds.type" size="small" style="width:160px" :disabled="isPlatformMySQL(ds)">
            <el-option value="prometheus" label="Prometheus" />
            <el-option value="pushgateway" label="Pushgateway（告警转发）" />
            <el-option value="mysql" label="MySQL" />
            <el-option value="postgres" label="PostgreSQL" />
            <el-option value="clickhouse" label="ClickHouse" />
          </el-select>
          <el-tag v-if="isPlatformMySQL(ds)" size="small" type="info">平台内置</el-tag>
          <span class="health-dot" :class="healthMap[ds.id] === true ? 'alive' : healthMap[ds.id] === false ? 'dead' : ''"></span>
          <span class="health-text">{{ healthText(ds.id) }}</span>
          <el-button v-if="!isPlatformMySQL(ds)" size="small" type="danger" plain @click="removeSource(i)">删除</el-button>
        </div>
        <el-form :model="ds" label-width="80px" size="small" style="margin-top:12px">
          <el-form-item label="地址">
            <el-input v-model="ds.url" :placeholder="placeholder(ds.type)" :disabled="isPlatformMySQL(ds)" />
          </el-form-item>
          <template v-if="ds.type !== 'prometheus' && ds.type !== 'pushgateway'">
            <el-form-item label="用户名"><el-input v-model="ds.username" :disabled="isPlatformMySQL(ds)" /></el-form-item>
            <el-form-item label="密码"><el-input v-model="ds.password" type="password" show-password :disabled="isPlatformMySQL(ds)" /></el-form-item>
            <el-form-item label="数据库"><el-input v-model="ds.database" :disabled="isPlatformMySQL(ds)" /></el-form-item>
          </template>
        </el-form>
        <div v-if="isPlatformMySQL(ds)" class="builtin-note">该数据源来自后端 mysql.dsn，用于平台自身 MySQL 持久化健康检查；如需修改，请在后端安全配置中调整。</div>
      </div>
    </div>

    <div class="bottom-actions">
      <el-button @click="add">+ 添加数据源</el-button>
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      <el-button :loading="loading" @click="load">刷新健康</el-button>
    </div>
    <el-alert v-if="msg" :title="msg" :type="msgType" show-icon style="margin-top:12px;max-width:760px" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { ElMessageBox } from 'element-plus'

const sources = ref([])
const saving = ref(false)
const loading = ref(false)
const msg = ref('')
const msgType = ref('success')
const healthMap = ref({})
const healthDetail = ref({})
const normalizeList = data => Array.isArray(data) ? data : (Array.isArray(data?.value) ? data.value : [])
const isPlatformMySQL = ds => ds?.id === 'platform-mysql'

const placeholder = (type) => ({
  prometheus: 'http://localhost:9090',
  pushgateway: 'http://localhost:9091',
  mysql: 'localhost:3306 or user:pass@tcp(host:3306)/db',
  postgres: 'localhost:5432',
  clickhouse: 'http://localhost:8123'
}[type] || '')

const healthText = (id) => {
  if (healthMap.value[id] === true) return '健康'
  if (healthMap.value[id] === false) return healthDetail.value[id] || '异常'
  return '未检查'
}

const load = async () => {
  loading.value = true
  try {
    const { data } = await axios.get('/api/v1/data-sources')
    sources.value = normalizeList(data)
    const { data: health } = await axios.get('/api/v1/health/datasources').catch(() => ({ data: [] }))
    const map = {}
    const detail = {}
    ;(health || []).forEach(h => {
      map[h.id] = h.alive
      detail[h.id] = h.detail
    })
    healthMap.value = map
    healthDetail.value = detail
  } catch (error) {
    msg.value = error.response?.data?.error || '数据源加载失败'
    msgType.value = 'error'
  } finally {
    loading.value = false
  }
}

const add = () => {
  sources.value.push({ id: Date.now().toString(), name: '新数据源', type: 'prometheus', url: '', username: '', password: '', database: '' })
}

const removeSource = async (i) => {
  const source = sources.value[i]
  if (isPlatformMySQL(source)) {
    msg.value = '平台 MySQL 为内置数据源，不能从页面删除'
    msgType.value = 'warning'
    return
  }
  const name = source?.name || '该数据源'
  await ElMessageBox.confirm(`确认删除 ${name}？删除后需点击保存才会生效。`, '二次确认', { type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消' })
  sources.value.splice(i, 1)
}

const save = async () => {
  saving.value = true
  try {
    await axios.post('/api/v1/data-sources', sources.value)
    msg.value = '保存成功，平台 MySQL 已保持为运行配置自动接入'
    msgType.value = 'success'
    await load()
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
.config-page { padding: 32px 36px; min-height: 100vh; color: #243553; }
.page-head { margin-bottom: 22px; }
.page-head h2 { margin: 8px 0 0; font-size: 30px; letter-spacing: -.04em; color: #263653; }
.page-head p { margin: 8px 0 0; color: #60728e; font-size: 13px; }
.panel-kicker { font-size: 13px; color: #247cff; text-transform: uppercase; letter-spacing: .06em; font-weight: 800; }
.ds-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(360px, 420px)); gap: 14px; align-items: start; max-height: calc(100vh - 178px); overflow: auto; padding-right: 6px; }
.panel-card { background: linear-gradient(145deg, rgba(255,255,255,.58), rgba(225,236,255,.42)); border: 1px solid rgba(255,255,255,.72); border-radius: 24px; padding: 16px; box-shadow: 0 20px 54px rgba(63,100,160,.16), inset 0 1px 0 rgba(255,255,255,.78); backdrop-filter: blur(24px); }
.panel-card.builtin { border-color: rgba(36,124,255,.28); background: linear-gradient(145deg, rgba(239,247,255,.75), rgba(225,236,255,.52)); }
.card-head { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; flex-wrap: wrap; }
.health-dot { width: 10px; height: 10px; border-radius: 50%; background: #9aa9be; flex-shrink: 0; }
.health-dot.alive { background: #36d08a; box-shadow: 0 0 0 5px rgba(54,208,138,.12); }
.health-dot.dead { background: #ff5b6b; box-shadow: 0 0 0 5px rgba(255,91,107,.12); }
.health-text { color: #60728e; font-size: 12px; max-width: 160px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.builtin-note { margin-top: 8px; color: #60728e; font-size: 12px; line-height: 1.6; }
.bottom-actions { display: flex; gap: 12px; margin-top: 14px; }
:deep(.el-form) { margin-top: 10px !important; }
:deep(.el-form-item) { margin-bottom: 10px; }
:deep(.el-form-item__label) { width: 64px !important; color: #60728e; font-weight: 700; font-size: 12px; }
:deep(.el-form-item__content) { margin-left: 64px !important; }
:deep(.el-input__wrapper), :deep(.el-select__wrapper) { min-height: 32px; border-radius: 999px !important; }
:deep(.el-input__inner) { font-size: 12px; }
:deep(.el-button--small) { height: 28px; padding: 0 10px; font-size: 12px; }
</style>
