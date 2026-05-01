<template>
  <div class="handoff-card">
    <div class="handoff-head">
      <b>值班交接 · {{ note.status || '待确认' }}</b>
      <button type="button" @click="emit('copy')">复制交接</button>
    </div>
    <p>{{ note.summary || '' }}</p>
    <ul v-if="note.verifiedFacts?.length">
      <li v-for="(fact, index) in note.verifiedFacts" :key="index">{{ fact }}</li>
    </ul>
    <small v-if="note.escalationPolicy">升级条件：{{ note.escalationPolicy }}</small>
  </div>
</template>

<script setup>
const props = defineProps({
  note: { type: Object, default: () => ({}) }
})
const emit = defineEmits(['copy'])
</script>

<style scoped>
.handoff-card { margin-bottom: 10px; border-radius: 16px; padding: 12px; background: #f8fbff; border: 1px solid rgba(37, 124, 255, .16); }
.handoff-head { display: flex; justify-content: space-between; gap: 8px; align-items: center; margin-bottom: 6px; }
.handoff-head button { border: 0; border-radius: 10px; background: #247cff; color: white; padding: 5px 9px; cursor: pointer; }
.handoff-card p { margin: 5px 0; color: #334155; line-height: 1.55; }
.handoff-card ul { margin: 6px 0; padding-left: 18px; color: #475569; }
.handoff-card small { color: #b45309; }
</style>
