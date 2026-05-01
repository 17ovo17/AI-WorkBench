<template>
  <div class="change-pwd-page">
    <div class="change-pwd-card glass-card">
      <div class="card-logo">AI WorkBench</div>
      <h2>首次登录，请修改密码</h2>
      <el-form @submit.prevent="handleChange">
        <el-form-item>
          <el-input v-model="form.old_password" type="password" placeholder="当前密码" prefix-icon="Lock" size="large" show-password />
        </el-form-item>
        <el-form-item>
          <el-input v-model="form.new_password" type="password" placeholder="新密码（至少 6 位）" prefix-icon="Key" size="large" show-password />
        </el-form-item>
        <el-form-item>
          <el-input v-model="confirmPwd" type="password" placeholder="确认新密码" prefix-icon="Key" size="large" show-password @keydown.enter="handleChange" />
        </el-form-item>
        <el-button type="primary" size="large" :loading="loading" style="width:100%" @click="handleChange">确认修改</el-button>
      </el-form>
      <p v-if="error" class="pwd-error">{{ error }}</p>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import { ElMessage } from 'element-plus'

const router = useRouter()
const form = ref({ old_password: '', new_password: '' })
const confirmPwd = ref('')
const loading = ref(false)
const error = ref('')

const handleChange = async () => {
  error.value = ''
  if (form.value.new_password.length < 6) { error.value = '新密码至少 6 位'; return }
  if (form.value.new_password !== confirmPwd.value) { error.value = '两次密码不一致'; return }
  loading.value = true
  try {
    await axios.post('/api/v1/auth/change-password', form.value)
    const user = JSON.parse(localStorage.getItem('aiw-user') || '{}')
    user.must_change_pwd = false
    localStorage.setItem('aiw-user', JSON.stringify(user))
    ElMessage.success('密码已修改')
    router.push('/')
  } catch (e) {
    error.value = e.response?.data?.error || '修改失败'
  } finally { loading.value = false }
}
</script>

<style scoped>
.change-pwd-page { min-height: 100vh; display: flex; align-items: center; justify-content: center; }
.change-pwd-card { width: 400px; padding: 40px 32px; border-radius: 24px; text-align: center; }
.card-logo { font-size: 22px; font-weight: 800; color: #236cff; margin-bottom: 8px; }
.change-pwd-card h2 { font-size: 16px; color: #1e3a5f; margin-bottom: 24px; }
.pwd-error { color: #ef4444; font-size: 13px; margin-top: 12px; }
</style>
