<template>
  <div class="credentials-page">
    <header class="section-head">
      <div>
        <h2>凭证管理</h2>
        <p>用于远程执行、探针安装和巡检动作，不展示真实密码或私钥。</p>
      </div>
      <el-button type="primary" @click="openCreate">新增凭证</el-button>
    </header>

    <el-table :data="credentials" stripe style="width:100%">
      <el-table-column prop="name" label="名称" min-width="160" />
      <el-table-column prop="protocol" label="协议" width="120" />
      <el-table-column prop="username" label="用户名" width="160" />
      <el-table-column label="密码" width="120">
        <template #default>******</template>
      </el-table-column>
      <el-table-column label="SSH Key" width="120">
        <template #default="{ row }">{{ row.ssh_key ? '已配置' : '未配置' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="120">
        <template #default="{ row }">
          <el-button text type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-if="!credentials.length" description="暂无凭证" />

    <el-alert v-if="msg" :title="msg" :type="msgType" show-icon class="feedback" />

    <el-dialog v-model="dialogVisible" title="新增凭证" width="520px" @close="resetForm">
      <el-form :model="form" label-width="90px">
        <el-form-item label="名称">
          <el-input v-model="form.name" placeholder="如：生产 SSH 凭证" />
        </el-form-item>
        <el-form-item label="协议">
          <el-select v-model="form.protocol" style="width:100%">
            <el-option label="SSH 密码" value="ssh" />
            <el-option label="SSH Key" value="ssh_key" />
            <el-option label="WinRM" value="winrm" />
          </el-select>
        </el-form-item>
        <el-form-item label="用户名">
          <el-input v-model="form.username" placeholder="用户名" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" show-password placeholder="密码" />
        </el-form-item>
        <el-form-item label="SSH Key">
          <el-input v-model="form.ssh_key" type="textarea" :rows="3" placeholder="SSH 私钥（测试中使用占位值）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import axios from 'axios'
import { ElMessageBox } from 'element-plus'

const credentials = ref([])
const dialogVisible = ref(false)
const msg = ref('')
const msgType = ref('success')
const form = ref({ name: '', protocol: 'ssh', username: '', password: '', ssh_key: '' })

const load = async () => {
  try {
    const { data } = await axios.get('/api/v1/credentials')
    credentials.value = Array.isArray(data) ? data : []
  } catch (e) {
    msg.value = e.response?.data?.error || '凭证加载失败'
    msgType.value = 'error'
  }
}

const openCreate = () => {
  resetForm()
  dialogVisible.value = true
}

const resetForm = () => {
  form.value = { name: '', protocol: 'ssh', username: '', password: '', ssh_key: '' }
}

const save = async () => {
  if (!form.value.name || !form.value.username) {
    msg.value = '名称和用户名为必填'
    msgType.value = 'warning'
    return
  }
  try {
    await axios.post('/api/v1/credentials', { ...form.value })
    dialogVisible.value = false
    msg.value = '保存成功'
    msgType.value = 'success'
    await load()
  } catch (e) {
    msg.value = e.response?.data?.error || '保存失败'
    msgType.value = 'error'
  }
}

const remove = async row => {
  try {
    await ElMessageBox.confirm(`确认删除凭证 ${row.name}？`, '删除凭证', { type: 'warning', confirmButtonText: '确认', cancelButtonText: '取消' })
    await axios.delete(`/api/v1/credentials/${row.id}`)
    msg.value = '已删除'
    msgType.value = 'success'
    await load()
  } catch (e) {
    if (e !== 'cancel') {
      msg.value = e.response?.data?.error || '删除失败'
      msgType.value = 'error'
    }
  }
}

onMounted(load)
</script>

<style scoped>
.credentials-page { padding: 24px; color: #243553; }
.section-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 18px; margin-bottom: 18px; }
.section-head h2 { margin: 0 0 6px; font-size: 22px; color: #1e3a5f; }
.section-head p { margin: 0; color: #60728e; font-size: 13px; }
.feedback { margin-top: 14px; max-width: 720px; }
</style>
