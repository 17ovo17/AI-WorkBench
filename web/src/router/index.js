import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: () => import('../views/Login.vue'), meta: { public: true } },
    { path: '/settings/change-password', component: () => import('../views/ChangePassword.vue') },
    { path: '/', component: () => import('../views/Dashboard.vue') },
    { path: '/workbench', component: () => import('../views/Workbench.vue') },
    { path: '/knowledge', component: () => import('../views/KnowledgeCenter.vue') },
    { path: '/workflows', component: () => import('../views/WorkflowHub.vue') },
    { path: '/alerts', component: () => import('../views/Alerts.vue') },
    { path: '/topology', component: () => import('../views/TopologyHub.vue') },
    { path: '/settings/ai', component: () => import('../views/AIConfig.vue') },
    { path: '/settings', component: () => import('../views/SystemSettings.vue') },
    { path: '/diagnose', redirect: '/knowledge?tab=diagnosis' },
    { path: '/catpaw', redirect: '/topology?tab=catpaw' },
    { path: '/settings/profiles', redirect: '/settings?tab=profiles' },
    { path: '/settings/oncall', redirect: '/settings?tab=oncall' },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ]
})

router.beforeEach((to, from, next) => {
  if (to.meta?.public) return next()
  const token = localStorage.getItem('aiw-token')
  if (!token) return next('/login')
  next()
})

export default router
