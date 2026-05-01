import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import axios from 'axios'
import App from './App.vue'
import router from './router'
import store from './store'

const token = localStorage.getItem('aiw-token')
if (token) {
  axios.defaults.headers.common['Authorization'] = `Bearer ${token}`
}

const isLoginPage = () => window.location.pathname.startsWith('/login')
const isSessionAuthRequest = config => {
  const url = String(config?.url || '')
  return url.includes('/auth/me') || url.includes('/auth/change-password')
}

axios.interceptors.response.use(r => r, err => {
  if (err.response?.status === 401 && !isLoginPage()) {
    localStorage.removeItem('aiw-token')
    localStorage.removeItem('aiw-user')
    delete axios.defaults.headers.common['Authorization']
    window.location.href = '/login'
  }
  return Promise.reject(err)
})

const app = createApp(App)
app.use(ElementPlus)
app.use(router)
app.use(store)
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}
app.mount('#app')
