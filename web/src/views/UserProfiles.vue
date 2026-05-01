<template>
  <div class="profiles-page">
    <header class="page-header">
      <h2>常用地址配置</h2>
      <el-button type="primary" @click="showAdd = true"><el-icon><Plus /></el-icon>新建配置</el-button>
    </header>

    <div class="profiles-grid">
      <div v-if="!profiles.length" class="empty-hint glass-card">
        <p>还没有常用地址配置</p>
        <p>添加常用的 IP、端口、服务，在诊断和拓扑页面快速选择</p>
      </div>
      <div v-for="p in profiles" :key="p.id" class="profile-card glass-card">
        <div class="card-head">
          <strong>{{ p.name }}</strong>
          <div>
            <el-button text @click="editProfile(p)"><el-icon><Edit /></el-icon></el-button>
            <el-button text type="danger" @click="deleteProfile(p.id)"><el-icon><Delete /></el-icon></el-button>
          </div>
        </div>
        <p v-if="p.description" class="card-desc">{{ p.description }}</p>
        <div class="tag-group">
          <el-tag v-for="h in p.hosts" :key="h.ip" size="small">{{ h.label || h.ip }}{{ h.hostname ? ` (${h.hostname})` : '' }}</el-tag>
        </div>
        <div v-if="p.endpoints?.length" class="tag-group">
          <el-tag v-for="e in p.endpoints" :key="`${e.ip}:${e.port}`" type="info" size="small">{{ e.label || e.service || e.ip }}:{{ e.port }}</el-tag>
        </div>
      </div>
    </div>

    <el-dialog v-model="showAdd" :title="editingId ? '编辑配置' : '新建配置'" width="560px" @close="resetForm">
      <el-form label-position="top">
        <el-form-item label="配置名称"><el-input v-model="form.name" placeholder="如：生产环境核心服务" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="主机列表">
          <div v-for="(h, i) in form.hosts" :key="i" class="inline-row">
            <el-input v-model="h.ip" placeholder="IP" style="width:140px" />
            <el-input v-model="h.hostname" placeholder="主机名" style="width:140px" />
            <el-input v-model="h.label" placeholder="备注" style="width:120px" />
            <el-button text type="danger" @click="form.hosts.splice(i, 1)"><el-icon><Delete /></el-icon></el-button>
          </div>
          <el-button text @click="form.hosts.push({ ip: '', hostname: '', label: '' })"><el-icon><Plus /></el-icon>添加主机</el-button>
        </el-form-item>
        <el-form-item label="端点列表">
          <div v-for="(e, i) in form.endpoints" :key="i" class="inline-row">
            <el-input v-model="e.ip" placeholder="IP" style="width:120px" />
            <el-input v-model.number="e.port" placeholder="端口" style="width:80px" />
            <el-input v-model="e.service" placeholder="服务名" style="width:100px" />
            <el-input v-model="e.label" placeholder="备注" style="width:100px" />
            <el-button text type="danger" @click="form.endpoints.splice(i, 1)"><el-icon><Delete /></el-icon></el-button>
          </div>
          <el-button text @click="form.endpoints.push({ ip: '', port: null, service: '', label: '' })"><el-icon><Plus /></el-icon>添加端点</el-button>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAdd = false">取消</el-button>
        <el-button type="primary" @click="saveProfile">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'

const profiles = ref([])
const showAdd = ref(false)
const editingId = ref('')
const form = ref({ name: '', description: '', hosts: [{ ip: '', hostname: '', label: '' }], endpoints: [] })

const load = async () => {
  try { const { data } = await axios.get('/api/v1/user-profiles'); profiles.value = Array.isArray(data) ? data : [] } catch {}
}

const saveProfile = async () => {
  if (!form.value.name) { ElMessage.warning('请输入配置名称'); return }
  const payload = { ...form.value, hosts: form.value.hosts.filter(h => h.ip), endpoints: form.value.endpoints.filter(e => e.ip && e.port) }
  if (editingId.value) payload.id = editingId.value
  await axios.post('/api/v1/user-profiles', payload)
  showAdd.value = false; resetForm(); load()
  ElMessage.success('已保存')
}

const editProfile = p => {
  editingId.value = p.id
  form.value = { name: p.name, description: p.description || '', hosts: p.hosts?.length ? [...p.hosts] : [{ ip: '', hostname: '', label: '' }], endpoints: p.endpoints?.length ? [...p.endpoints] : [] }
  showAdd.value = true
}

const deleteProfile = async id => {
  await ElMessageBox.confirm('确定删除？', '提示', { type: 'warning', confirmButtonText: '确定', cancelButtonText: '取消' })
  await axios.delete(`/api/v1/user-profiles/${id}`)
  load(); ElMessage.success('已删除')
}

const resetForm = () => { editingId.value = ''; form.value = { name: '', description: '', hosts: [{ ip: '', hostname: '', label: '' }], endpoints: [] } }

onMounted(load)
</script>

<style scoped>
.profiles-page { padding: 24px; max-width: 960px; margin: 0 auto; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.page-header h2 { font-size: 20px; color: #1e3a5f; }
.profiles-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 16px; }
.profile-card { padding: 18px; border-radius: 20px; }
.card-head { display: flex; justify-content: space-between; align-items: center; }
.card-head strong { font-size: 15px; color: #1e3a5f; }
.card-desc { font-size: 13px; color: #64748b; margin: 6px 0; }
.tag-group { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 10px; }
.empty-hint { padding: 40px; text-align: center; color: #64748b; border-radius: 20px; }
.inline-row { display: flex; gap: 8px; align-items: center; margin-bottom: 8px; }
</style>
