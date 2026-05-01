<template>
  <div :class="['summary-card', `sev-${card.severity || 'P3'}`]">
    <div class="summary-top">
      <b>结论卡 · {{ card.severity || 'P3' }}</b>
      <span>{{ card.escalationNeeded ? '需要升级' : '持续观察' }}</span>
    </div>
    <p><strong>问题：</strong>{{ card.problem || '-' }}</p>
    <p><strong>影响：</strong>{{ card.impact || '-' }}</p>
    <p><strong>下一步：</strong>{{ card.nextStep || '-' }}</p>
  </div>
</template>

<script setup>
defineProps({
  card: { type: Object, default: () => ({}) }
})
</script>

<style scoped>
.summary-card { margin-bottom: 10px; border-radius: 16px; padding: 12px; background: #f8fbff; border: 1px solid rgba(37, 124, 255, .16); }
.summary-card.sev-P0, .summary-card.sev-P1 { background: #fff1f2; border-color: rgba(220, 38, 38, .22); }
.summary-top { display: flex; justify-content: space-between; gap: 8px; align-items: center; margin-bottom: 6px; }
.summary-top span { border-radius: 999px; background: #e0f2fe; color: #0369a1; padding: 3px 8px; font-size: 11px; }
.summary-card.sev-P0 .summary-top span, .summary-card.sev-P1 .summary-top span { background: #fee2e2; color: #b91c1c; }
.summary-card p { margin: 5px 0; color: #334155; line-height: 1.55; }
</style>
