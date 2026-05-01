<template>
  <div class="config-page">
    <div class="page-head">
      <div class="panel-kicker">AI Config</div>
      <h2>AI 模型配置</h2>
    </div>

    <el-tabs v-model="activeTab" type="border-card" class="ai-tabs">
      <el-tab-pane label="LLM 模型" name="llm">
        <LlmProviders />
      </el-tab-pane>
      <el-tab-pane label="Embedding 模型" name="embedding">
        <EmbeddingConfig />
      </el-tab-pane>
      <el-tab-pane label="Reranker 模型" name="reranker">
        <RerankerConfig />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref, defineAsyncComponent } from 'vue'

const activeTab = ref('llm')

const LlmProviders = defineAsyncComponent(() =>
  import('../components/ai/LlmProviders.vue')
)
const EmbeddingConfig = defineAsyncComponent(() =>
  import('../components/ai/EmbeddingConfig.vue')
)
const RerankerConfig = defineAsyncComponent(() =>
  import('../components/ai/RerankerConfig.vue')
)
</script>

<style scoped>
.config-page { padding: 32px 36px; min-height: 100vh; color: #243553; }
.page-head { margin-bottom: 22px; }
.page-head h2 { margin: 8px 0 0; font-size: 30px; letter-spacing: -.04em; color: #263653; }
.panel-kicker { font-size: 13px; color: #247cff; text-transform: uppercase; letter-spacing: .06em; font-weight: 800; }
.ai-tabs :deep(.el-tabs__header) { margin-bottom: 16px; }
</style>
