<template>
  <div class="login-page">
    <div class="login-card glass-card">
      <div class="login-logo">AI WorkBench</div>
      <h2>登录</h2>
      <el-form @submit.prevent="handleLogin">
        <el-form-item>
          <el-input v-model="form.username" placeholder="用户名" prefix-icon="User" size="large" />
        </el-form-item>
        <el-form-item>
          <el-input v-model="form.password" type="password" placeholder="密码" prefix-icon="Lock" size="large" show-password @keydown.enter="handleLogin" />
        </el-form-item>
        <el-button type="primary" size="large" :loading="loading" style="width:100%" @click="handleLogin">登录</el-button>
      </el-form>
      <p v-if="error" class="login-error">{{ error }}</p>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'

const router = useRouter()
const form = ref({ username: '', password: '' })
const loading = ref(false)
const error = ref('')

const handleLogin = async () => {
  if (!form.value.username || !form.value.password) { error.value = '请输入用户名和密码'; return }
  loading.value = true; error.value = ''
  try {
    const { data } = await axios.post('/api/v1/auth/login', form.value)
    localStorage.setItem('aiw-token', data.token)
    localStorage.setItem('aiw-user', JSON.stringify(data.user))
    axios.defaults.headers.common['Authorization'] = `Bearer ${data.token}`
    if (data.user.must_change_pwd) {
      router.push('/settings/change-password')
    } else {
      router.push('/')
    }
  } catch (e) {
    error.value = e.response?.data?.error || '登录失败'
  } finally { loading.value = false }
}
</script>

<style scoped>
.login-page { min-height: 100vh; display: flex; align-items: center; justify-content: center; }
.login-card { width: 380px; padding: 40px 32px; border-radius: 24px; text-align: center; }
.login-logo { font-size: 22px; font-weight: 800; color: #236cff; margin-bottom: 8px; }
.login-card h2 { font-size: 18px; color: #1e3a5f; margin-bottom: 24px; }
.login-error { color: #ef4444; font-size: 13px; margin-top: 12px; }
</style>
