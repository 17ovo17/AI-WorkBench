<template>
  <div class="app-layout" :class="{ 'no-sidebar': isLoginPage }">
    <div class="ambient ambient-one"></div>
    <div class="ambient ambient-two"></div>
    <aside v-if="!isLoginPage" class="sidebar glass-panel">
      <div class="sidebar-brand">AI WorkBench</div>
      <nav class="sidebar-nav">
        <div class="nav-group-label">核心功能</div>
        <router-link to="/" class="nav-item" exact>
          <el-icon><DataBoard /></el-icon><span>运维总览</span>
        </router-link>
        <router-link to="/workbench" class="nav-item">
          <el-icon><ChatDotRound /></el-icon><span>智能诊断</span>
        </router-link>
        <router-link to="/knowledge" class="nav-item">
          <el-icon><Collection /></el-icon><span>知识中心</span>
        </router-link>
        <router-link to="/workflows" class="nav-item">
          <el-icon><Operation /></el-icon><span>工作流</span>
        </router-link>
        <div class="nav-divider"></div>
        <div class="nav-group-label">监控告警</div>
        <router-link to="/alerts" class="nav-item">
          <el-icon><Bell /></el-icon><span>告警中心</span>
        </router-link>
        <router-link to="/topology" class="nav-item">
          <el-icon><Share /></el-icon><span>业务拓扑</span>
        </router-link>
        <div class="nav-divider"></div>
        <div class="nav-group-label">系统设置</div>
        <router-link to="/settings/ai" class="nav-item">
          <el-icon><MagicStick /></el-icon><span>AI 模型</span>
        </router-link>
        <router-link to="/settings" class="nav-item">
          <el-icon><Setting /></el-icon><span>系统配置</span>
        </router-link>
      </nav>
      <el-dropdown trigger="click" @command="handleUserCommand">
        <div class="sidebar-user" style="cursor:pointer">
          <div class="avatar-orb"></div>
          <span>{{ currentUser.username || '用户' }}</span>
          <el-icon><ArrowDown /></el-icon>
        </div>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="change-password"><el-icon><Key /></el-icon>修改密码</el-dropdown-item>
            <el-dropdown-item command="logout" divided><el-icon><SwitchButton /></el-icon>退出登录</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
      <div class="sidebar-tools">
        <button type="button" aria-label="切换到日间模式" :class="{ active: theme === 'light' }" @click="setTheme('light')"><el-icon><Sunny /></el-icon></button>
        <button type="button" aria-label="切换到夜间模式" :class="{ active: theme === 'dark' }" @click="setTheme('dark')"><el-icon><Moon /></el-icon></button>
      </div>
    </aside>
    <main class="main-content">
      <router-view />
    </main>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const isLoginPage = computed(() => route.path === '/login' || route.path === '/settings/change-password')
const theme = ref(localStorage.getItem('aiw-theme') || 'light')

const currentUser = computed(() => {
  try { return JSON.parse(localStorage.getItem('aiw-user') || '{}') } catch { return {} }
})

const handleLogout = () => {
  localStorage.removeItem('aiw-token')
  localStorage.removeItem('aiw-user')
  router.push('/login')
}

const handleUserCommand = (cmd) => {
  if (cmd === 'logout') handleLogout()
  else if (cmd === 'change-password') router.push('/settings/change-password')
}

const setTheme = mode => {
  theme.value = mode
}

watch(theme, value => {
  const mode = value === 'dark' ? 'dark' : 'light'
  document.documentElement.dataset.theme = mode
  localStorage.setItem('aiw-theme', mode)
}, { immediate: true })

onMounted(() => setTheme(theme.value))
</script>

<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
:root {
  color-scheme: light;
  --ink: #23334f;
  --muted: #74849e;
  --blue: #2f7cff;
  --blue-2: #72a9ff;
  --glass: rgba(239, 246, 255, .54);
  --glass-strong: rgba(248, 252, 255, .74);
  --line: rgba(255, 255, 255, .58);
  --shadow: 0 26px 80px rgba(55, 99, 170, .22), inset 0 1px 0 rgba(255,255,255,.7);
}
[data-theme="dark"] {
  color-scheme: dark;
  --ink: #e6edf7;
  --muted: #9dafc8;
  --blue: #65a5ff;
  --blue-2: #93c5fd;
  --glass: rgba(19, 30, 50, .66);
  --glass-strong: rgba(25, 39, 64, .82);
  --line: rgba(140, 164, 205, .26);
  --shadow: 0 26px 80px rgba(0, 10, 28, .42), inset 0 1px 0 rgba(255,255,255,.08);
}
html, body, #app { width: 100%; height: 100%; min-height: 100%; }
body {
  min-width: 1180px;
  background:
    radial-gradient(circle at 80% -10%, rgba(255,255,255,.92), transparent 28%),
    radial-gradient(circle at 18% 95%, rgba(255,255,255,.66), transparent 24%),
    linear-gradient(135deg, #eef5ff 0%, #d9e8fb 42%, #b7cceb 100%);
  color: var(--ink);
  font-family: Inter, ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", "Microsoft YaHei", sans-serif;
  overflow: hidden;
}
[data-theme="dark"] body {
  background:
    radial-gradient(circle at 80% -10%, rgba(52,76,118,.48), transparent 28%),
    radial-gradient(circle at 18% 95%, rgba(45,92,155,.28), transparent 24%),
    linear-gradient(135deg, #07111f 0%, #111e33 48%, #172742 100%);
}
body::before, body::after {
  content: "";
  position: fixed;
  inset: auto;
  pointer-events: none;
  z-index: 0;
}
body::before {
  width: 980px; height: 980px; right: -140px; top: -180px;
  background: radial-gradient(ellipse at center, rgba(255,255,255,.72), rgba(255,255,255,.2) 35%, transparent 66%);
  filter: blur(4px);
}
body::after {
  width: 1250px; height: 420px; left: -170px; bottom: 35px;
  border-radius: 50%;
  border-top: 3px solid rgba(255,255,255,.5);
  transform: rotate(-26deg);
  box-shadow: 0 -18px 60px rgba(255,255,255,.28);
}
.app-layout { position: relative; z-index: 1; display: flex; height: 100vh; min-height: 0; padding: 0; overflow: hidden; }
.app-layout.no-sidebar { justify-content: center; }
.ambient { position: fixed; border-radius: 999px; pointer-events: none; filter: blur(18px); opacity: .55; z-index: 0; }
.ambient-one { width: 420px; height: 420px; right: 16%; top: 12%; background: rgba(255,255,255,.78); }
.ambient-two { width: 320px; height: 320px; left: 14%; bottom: 8%; background: rgba(75,142,255,.22); }
.glass-panel {
  background: linear-gradient(145deg, rgba(255,255,255,.62), rgba(224,236,255,.42));
  border: 1px solid rgba(255,255,255,.66);
  box-shadow: var(--shadow);
  backdrop-filter: blur(24px) saturate(140%);
}
[data-theme="dark"] .glass-panel {
  background: linear-gradient(145deg, rgba(24,38,62,.76), rgba(16,28,48,.56));
  border-color: rgba(150, 174, 214, .22);
}
.sidebar {
  width: 188px;
  height: 100vh;
  border-radius: 0 28px 28px 0;
  padding: 24px 14px 12px;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}
.sidebar-brand { color: #236cff; font-size: 18px; font-weight: 800; letter-spacing: -.03em; margin: 0 8px 24px; }
.sidebar-nav { display: flex; flex-direction: column; gap: 8px; flex: 1; }
.nav-item {
  min-height: 38px;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 0 16px;
  color: #2b3b57;
  text-decoration: none;
  font-size: 13px;
  font-weight: 650;
  border-radius: 24px;
  transition: transform .24s ease, box-shadow .24s ease, background .24s ease, color .24s ease;
}
[data-theme="dark"] .nav-item { color: #c9d7ea; }
.nav-item .el-icon { font-size: 25px; }
.nav-item:hover { transform: translateX(4px); color: var(--blue); background: rgba(255,255,255,.38); }
.nav-item.router-link-active {
  color: #1672ff;
  background: rgba(255,255,255,.62);
  box-shadow: 0 16px 38px rgba(74,119,184,.18), inset 0 1px 0 rgba(255,255,255,.9);
}
[data-theme="dark"] .nav-item.router-link-active {
  color: #ffffff;
  background: rgba(76, 130, 215, .24);
  box-shadow: 0 16px 38px rgba(0,0,0,.28), inset 0 1px 0 rgba(255,255,255,.08);
}
.nav-divider { height: 1px; background: rgba(78,105,146,.18); margin: 18px 12px; }
.nav-group-label { font-size: 10px; color: #8a9bb5; font-weight: 800; letter-spacing: .08em; text-transform: uppercase; padding: 0 16px; margin-bottom: -2px; }
.sidebar-user {
  height: 54px;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 0 16px;
  border-radius: 24px;
  background: rgba(255,255,255,.28);
  color: var(--ink);
  font-size: 15px;
  font-weight: 700;
  box-shadow: inset 0 1px 0 rgba(255,255,255,.54);
}
.sidebar-user .el-icon { margin-left: auto; color: #536987; }
.avatar-orb { width: 38px; height: 38px; border-radius: 50%; background: radial-gradient(circle at 32% 28%, #d9f0ff, #5f91ff 48%, #826cff); box-shadow: 0 12px 22px rgba(55,117,255,.28); }
.sidebar-tools { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-top: 18px; }
.sidebar-tools button {
  height: 38px; border: 0; border-radius: 22px; color: #243751; font-size: 20px; cursor: pointer;
  background: rgba(255,255,255,.42); box-shadow: inset 0 1px 0 rgba(255,255,255,.75), 0 12px 26px rgba(78,112,164,.12);
}
.sidebar-tools button.active { color: white; background: linear-gradient(135deg, #5aa0ff, #1668ff); }
.main-content { position: relative; z-index: 1; flex: 1; height: 100vh; min-width: 0; overflow: auto; }
.main-content::-webkit-scrollbar { width: 10px; }
.main-content::-webkit-scrollbar-thumb { background: rgba(67,103,156,.22); border-radius: 999px; }
.el-button { border-radius: 999px; font-weight: 700; }
.el-button--primary { background: linear-gradient(135deg, #5aa0ff, #1668ff); border: 0; box-shadow: 0 14px 28px rgba(35,110,255,.28); }
.el-input__wrapper, .el-textarea__inner, .el-select__wrapper {
  border-radius: 18px !important;
  background: rgba(255,255,255,.58) !important;
  box-shadow: inset 0 1px 0 rgba(255,255,255,.78), 0 10px 24px rgba(78,112,164,.10) !important;
  border: 1px solid rgba(255,255,255,.72) !important;
}
@media (max-width: 1280px) {
  body { min-width: 1024px; }
  .sidebar { width: 220px; }
}
</style>
