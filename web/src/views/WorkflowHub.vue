<template>
  <div class="wfh-page">
    <div class="page-head glass-panel">
      <div>
        <div class="panel-kicker">Workflows</div>
        <h2>工作流</h2>
        <p class="page-desc">诊断工作流与自定义工作流管理</p>
      </div>
    </div>

    <div class="tabs-wrap glass-panel">
      <el-tabs v-model="activeTab" class="wfh-tabs">
        <el-tab-pane label="诊断工作流" name="diagnosis">
          <DiagnosisPanel />
        </el-tab-pane>
        <el-tab-pane label="工作流管理" name="manager">
          <ManagerPanel />
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup>
import { ref, defineAsyncComponent } from 'vue'

const activeTab = ref('diagnosis')

const DiagnosisPanel = defineAsyncComponent(() =>
  import('../components/workflow/DiagnosisPanel.vue')
)
const ManagerPanel = defineAsyncComponent(() =>
  import('../components/workflow/ManagerPanel.vue')
)
</script>

<style scoped>
.wfh-page { padding: 28px 32px; min-height: 100vh; color: #243553; display: flex; flex-direction: column; gap: 18px; }
.glass-panel { background: linear-gradient(145deg, rgba(255,255,255,.58), rgba(225,236,255,.42)); border: 1px solid rgba(255,255,255,.72); border-radius: 24px; box-shadow: 0 20px 54px rgba(63,100,160,.16), inset 0 1px 0 rgba(255,255,255,.78); backdrop-filter: blur(24px); }
.page-head { padding: 20px 26px; }
.page-head h2 { margin: 6px 0 4px; font-size: 26px; letter-spacing: -.03em; color: #263653; }
.page-desc { font-size: 13px; color: var(--muted); }
.panel-kicker { font-size: 12px; color: #247cff; text-transform: uppercase; letter-spacing: .06em; font-weight: 800; }
.tabs-wrap { padding: 16px 20px 8px; }
.wfh-tabs :deep(.el-tabs__header) { margin-bottom: 16px; }
</style>
