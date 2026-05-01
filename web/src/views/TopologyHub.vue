<template>
  <div class="topo-hub-page">
    <div class="page-head glass-panel">
      <div>
        <div class="panel-kicker">Topology</div>
        <h2>业务拓扑</h2>
        <p class="page-desc">业务拓扑图与探针管理</p>
      </div>
    </div>

    <div class="tabs-wrap glass-panel">
      <el-tabs v-model="activeTab" class="topo-tabs">
        <el-tab-pane label="业务拓扑" name="topology">
          <TopologyView />
        </el-tab-pane>
        <el-tab-pane label="探针管理" name="catpaw">
          <CatpawView />
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup>
import { ref, defineAsyncComponent, watch } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const activeTab = ref(route.query.tab === 'catpaw' ? 'catpaw' : 'topology')

watch(() => route.query.tab, value => {
  if (value === 'catpaw' || value === 'topology') activeTab.value = value
})

const TopologyView = defineAsyncComponent(() =>
  import('./Topology.vue')
)
const CatpawView = defineAsyncComponent(() =>
  import('./CatpawInstall.vue')
)
</script>

<style scoped>
.topo-hub-page { padding: 28px 32px; height: 100%; min-height: 0; color: #243553; display: flex; flex-direction: column; gap: 18px; overflow: hidden; }
.glass-panel { background: linear-gradient(145deg, rgba(255,255,255,.58), rgba(225,236,255,.42)); border: 1px solid rgba(255,255,255,.72); border-radius: 24px; box-shadow: 0 20px 54px rgba(63,100,160,.16), inset 0 1px 0 rgba(255,255,255,.78); backdrop-filter: blur(24px); }
.page-head { padding: 20px 26px; }
.page-head h2 { margin: 6px 0 4px; font-size: 26px; letter-spacing: -.03em; color: #263653; }
.page-desc { font-size: 13px; color: var(--muted); }
.panel-kicker { font-size: 12px; color: #247cff; text-transform: uppercase; letter-spacing: .06em; font-weight: 800; }
.tabs-wrap { padding: 16px 20px 8px; flex: 1; min-height: 0; overflow: hidden; display: flex; flex-direction: column; }
:deep(.el-tabs) { flex: 1; min-height: 0; display: flex; flex-direction: column; }
:deep(.el-tabs__content) { flex: 1; min-height: 0; overflow: hidden; }
:deep(.el-tab-pane) { height: 100%; min-height: 0; overflow: hidden; }
.topo-tabs :deep(.el-tabs__header) { margin-bottom: 16px; }
</style>
