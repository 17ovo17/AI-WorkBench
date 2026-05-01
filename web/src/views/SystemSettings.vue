<template>
  <div class="sys-page">
    <div class="page-head glass-panel">
      <div>
        <div class="panel-kicker">System Settings</div>
        <h2>系统配置</h2>
        <p class="page-desc">数据源、指标映射、凭证、常用地址、健康审计与值班通知</p>
      </div>
    </div>

    <div class="tabs-wrap glass-panel">
      <el-tabs v-model="activeTab" class="sys-tabs">
        <el-tab-pane label="数据源" name="datasource">
          <DataSourceView />
        </el-tab-pane>
        <el-tab-pane label="指标映射" name="metrics">
          <MetricsMappingView />
        </el-tab-pane>
        <el-tab-pane label="常用地址" name="profiles">
          <UserProfilesView />
        </el-tab-pane>
        <el-tab-pane label="凭证管理" name="credentials">
          <CredentialsView />
        </el-tab-pane>
        <el-tab-pane label="健康审计" name="health-audit">
          <HealthAuditView />
        </el-tab-pane>
        <el-tab-pane label="值班通知" name="oncall">
          <OnCallView />
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup>
import { ref, defineAsyncComponent, watch } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const validTabs = new Set(['datasource', 'metrics', 'profiles', 'credentials', 'health-audit', 'oncall'])
const activeTab = ref(validTabs.has(route.query.tab) ? route.query.tab : 'datasource')

watch(() => route.query.tab, value => {
  if (validTabs.has(value)) activeTab.value = value
})

const DataSourceView = defineAsyncComponent(() =>
  import('./DataSource.vue')
)
const MetricsMappingView = defineAsyncComponent(() =>
  import('./MetricsMapping.vue')
)
const UserProfilesView = defineAsyncComponent(() =>
  import('./UserProfiles.vue')
)
const CredentialsView = defineAsyncComponent(() =>
  import('./CredentialsPanel.vue')
)
const HealthAuditView = defineAsyncComponent(() =>
  import('./HealthAuditPanel.vue')
)
const OnCallView = defineAsyncComponent(() =>
  import('./OnCallConfig.vue')
)
</script>

<style scoped>
.sys-page { padding: 28px 32px; height: 100%; color: #243553; display: flex; flex-direction: column; gap: 18px; }
.glass-panel { background: linear-gradient(145deg, rgba(255,255,255,.58), rgba(225,236,255,.42)); border: 1px solid rgba(255,255,255,.72); border-radius: 24px; box-shadow: 0 20px 54px rgba(63,100,160,.16), inset 0 1px 0 rgba(255,255,255,.78); backdrop-filter: blur(24px); }
.page-head { padding: 20px 26px; }
.page-head h2 { margin: 6px 0 4px; font-size: 26px; letter-spacing: -.03em; color: #263653; }
.page-desc { font-size: 13px; color: var(--muted); }
.panel-kicker { font-size: 12px; color: #247cff; text-transform: uppercase; letter-spacing: .06em; font-weight: 800; }
.tabs-wrap { padding: 16px 20px 8px; flex: 1; min-height: 0; overflow: hidden; display: flex; flex-direction: column; }
:deep(.el-tabs) { display: flex; flex-direction: column; height: 100%; }
:deep(.el-tabs__content) { flex: 1; min-height: 0; overflow-y: auto; }
.sys-tabs :deep(.el-tabs__header) { margin-bottom: 16px; }
</style>
