<template>
  <div class="alerts-page">
    <div class="page-head">
      <div>
        <div class="panel-kicker">SRE Event Inbox</div>
        <h2>{{ T.title }}</h2>
        <p>{{ T.subtitle }}</p>
      </div>
      <div class="head-actions">
        <el-button @click="load">{{ T.refresh }}</el-button>
        <el-button type="primary" @click="router.push({ path: '/settings', query: { tab: 'oncall' } })">{{ T.oncallConfig }}</el-button>
      </div>
    </div>
    <div class="stats-grid">
      <div class="stat"><b>{{ stats.active }}</b><span>{{ T.activeEvents }}</span></div>
      <div class="stat"><b>{{ stats.owned }}</b><span>{{ T.ownedEvents }}</span></div>
      <div class="stat"><b>{{ stats.muted }}</b><span>{{ T.mutedEvents }}</span></div>
      <div class="stat"><b>{{ stats.folded }}</b><span>{{ T.foldedEvents }}</span></div>
    </div>
    <div class="filters glass-line">
      <el-select v-model="query.status" :placeholder="T.status" clearable size="small"><el-option v-for="s in statuses" :key="s.value" :label="s.label" :value="s.value" /></el-select>
      <el-select v-model="query.severity" :placeholder="T.severity" clearable size="small"><el-option value="critical" label="critical" /><el-option value="warning" label="warning" /><el-option value="info" label="info" /></el-select>
      <el-input v-model="query.target_ip" :placeholder="T.hostIp" size="small" clearable />
      <el-input v-model="query.business_id" :placeholder="T.businessId" size="small" clearable />
      <el-input v-model="query.test_batch_id" placeholder="test_batch_id" size="small" clearable />
      <el-input v-model="keyword" :placeholder="T.search" size="small" clearable />
    </div>
    <div class="event-workspace">
      <div class="event-list">
        <div v-for="a in filtered" :key="a.id" class="event-card" :class="[a.severity, a.status]" @click="selected = a">
          <div class="event-main">
            <div class="event-title"><span class="pulse"></span>{{ a.title || T.unnamed }}</div>
            <div class="event-meta">
              <el-tag size="small" :type="severityType(a.severity)">{{ a.severity || 'unknown' }}</el-tag>
              <el-tag size="small" :type="statusType(a.status)">{{ statusLabel(a.status) }}</el-tag>
              <span>{{ a.target_ip || T.noHost }}</span><span>{{ a.source || 'unknown' }}</span><span>{{ T.repeat }} {{ a.count || 1 }}</span>
              <span>{{ fmt(a.first_seen || a.create_time) }} -> {{ fmt(a.last_seen || a.create_time) }}</span>
            </div>
            <div class="event-owner"><span>{{ T.ackBy }}: {{ a.ack_by || T.none }}</span><span>{{ T.assignee }}: {{ a.assignee || T.none }}</span><span v-if="a.muted_until">{{ T.mutedUntil }}: {{ fmt(a.muted_until) }}</span></div>
          </div>
          <div class="event-actions" @click.stop>
            <el-button size="small" @click="diagnose(a)">AI {{ T.diagnose }}</el-button>
            <el-button size="small" @click="action(a, 'acknowledge')">{{ T.ack }}</el-button>
            <el-button size="small" @click="assign(a)">{{ T.assign }}</el-button>
            <el-button size="small" @click="mute(a)">{{ T.mute }}</el-button>
            <el-button size="small" type="success" plain @click="resolve(a)">{{ T.resolve }}</el-button>
            <el-button size="small" @click="action(a, 'archive')">{{ T.archive }}</el-button>
            <el-button size="small" type="danger" plain @click="remove(a)">{{ T.delete }}</el-button>
          </div>
        </div>
        <div v-if="!filtered.length" class="empty">{{ T.empty }}</div>
      </div>
      <aside class="detail-panel" v-if="selected">
        <div class="detail-head"><b>{{ T.detail }}</b><el-button size="small" text @click="selected = null">{{ T.close }}</el-button></div>
        <h3>{{ selected.title }}</h3>
        <div class="detail-tags"><el-tag :type="severityType(selected.severity)">{{ selected.severity }}</el-tag><el-tag :type="statusType(selected.status)">{{ statusLabel(selected.status) }}</el-tag><el-tag>{{ T.fingerprint }} {{ short(selected.fingerprint) }}</el-tag></div>
        <section><h4>{{ T.scope }}</h4><p>{{ T.host }}: {{ selected.target_ip || '-' }}; {{ T.business }}: {{ selected.business_id || selected.linked_business_id || T.none }}</p></section>
        <section><h4>{{ T.notificationTrail }}</h4><div v-for="n in selected.notification_trail || []" :key="n.id" class="timeline-row"><b>{{ n.channel }}</b><span>{{ n.receiver }}</span><em>{{ n.status }}</em><small>{{ n.detail }}</small></div><p v-if="!(selected.notification_trail||[]).length">{{ T.noNotification }}</p></section>
        <section><h4>{{ T.actionTimeline }}</h4><div v-for="item in selected.action_log || []" :key="item.created_at + item.action" class="timeline-row"><b>{{ actionLabel(item.action) }}</b><span>{{ item.actor || 'system' }}</span><em>{{ item.from }} -> {{ item.to }}</em><small>{{ item.reason }}</small></div></section>
        <section><h4>{{ T.labels }}</h4><pre>{{ safeLabels(selected.labels) }}</pre></section>
      </aside>
    </div>
  </div>
</template>
<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
const T = { title: '\u544a\u8b66\u4e8b\u4ef6\u5de5\u4f5c\u53f0', subtitle: '\u805a\u5408\u91cd\u590d\u544a\u8b66\uff0c\u4e32\u8054\u8ba4\u9886\u3001\u8f6c\u6d3e\u3001\u9759\u9ed8\u3001\u8bca\u65ad\u3001\u6062\u590d\u3001\u5f52\u6863\u548c\u5220\u9664\u5ba1\u8ba1\u3002', refresh: '\u5237\u65b0', oncallConfig: '\u503c\u73ed\u901a\u77e5\u914d\u7f6e', activeEvents: '\u5f85\u5904\u7406\u4e8b\u4ef6', ownedEvents: '\u5df2\u8ba4\u9886/\u8f6c\u6d3e', mutedEvents: '\u9759\u9ed8\u4e2d', foldedEvents: '\u91cd\u590d\u6298\u53e0', status: '\u72b6\u6001', severity: '\u7ea7\u522b', hostIp: '\u4e3b\u673a IP', businessId: '\u4e1a\u52a1 ID', search: '\u641c\u7d22\u6807\u9898/\u5904\u7406\u4eba', unnamed: '\u672a\u547d\u540d\u544a\u8b66', noHost: '\u672a\u5173\u8054\u4e3b\u673a', repeat: '\u91cd\u590d', ackBy: '\u8ba4\u9886', assignee: '\u5904\u7406\u4eba', mutedUntil: '\u9759\u9ed8\u81f3', none: '\u65e0', diagnose: '\u8bca\u65ad', ack: '\u8ba4\u9886', assign: '\u8f6c\u6d3e', mute: '\u9759\u9ed8', resolve: '\u6062\u590d', archive: '\u5f52\u6863', delete: '\u5220\u9664', empty: '\u6682\u65e0\u7b26\u5408\u6761\u4ef6\u7684\u544a\u8b66\u4e8b\u4ef6', detail: '\u4e8b\u4ef6\u8be6\u60c5', close: '\u5173\u95ed', fingerprint: '\u6307\u7eb9', scope: '\u5f71\u54cd\u8303\u56f4', host: '\u4e3b\u673a', business: '\u4e1a\u52a1', notificationTrail: '\u901a\u77e5\u8f68\u8ff9', noNotification: '\u6682\u65e0\u901a\u77e5\u8bb0\u5f55', actionTimeline: '\u5904\u7f6e\u65f6\u95f4\u7ebf', labels: '\u6807\u7b7e\u6458\u8981\uff08\u5df2\u8131\u654f\uff09', loadFailed: '\u544a\u8b66\u52a0\u8f7d\u5931\u8d25', updated: '\u4e8b\u4ef6\u72b6\u6001\u5df2\u66f4\u65b0', failed: '\u64cd\u4f5c\u5931\u8d25\uff0c\u8bf7\u67e5\u770b\u540e\u7aef\u65e5\u5fd7\u6216\u91cd\u8bd5', assignPrompt: '\u8bf7\u8f93\u5165\u5904\u7406\u4eba\u6216\u503c\u73ed\u7ec4\uff081-40 \u4e2a\u5b57\u7b26\uff09', mutePrompt: '\u9759\u9ed8\u5206\u949f\u6570\uff081-1440\uff09', confirm: '\u4e8c\u6b21\u786e\u8ba4' }
const router = useRouter(); const alerts = ref([]); const selected = ref(null); const keyword = ref('')
const query = reactive({ status: '', severity: '', target_ip: '', business_id: '', test_batch_id: '' })
const statuses = [{ value: 'firing', label: '\u544a\u8b66\u4e2d' }, { value: 'acknowledged', label: '\u5df2\u8ba4\u9886' }, { value: 'assigned', label: '\u5df2\u8f6c\u6d3e' }, { value: 'diagnosing', label: '\u8bca\u65ad\u4e2d' }, { value: 'muted', label: '\u9759\u9ed8\u4e2d' }, { value: 'mitigated', label: '\u5df2\u7f13\u89e3' }, { value: 'resolved', label: '\u5df2\u6062\u590d' }, { value: 'archived', label: '\u5df2\u5f52\u6863' }]
const normalizedAlertTitle = a => String(a.title || a.labels?.alertname || '').replace(/-\d+$/g, '').trim().toLowerCase()
const groupedAlerts = computed(() => {
  const groups = new Map()
  alerts.value.forEach(a => {
    const key = [a.target_ip || a.labels?.instance || '', a.severity || '', a.status || '', normalizedAlertTitle(a)].join('|')
    const hit = groups.get(key)
    if (!hit) groups.set(key, { ...a, count: Number(a.count || 1), grouped_ids: [a.id] })
    else {
      hit.count += Number(a.count || 1)
      hit.grouped_ids.push(a.id)
      hit.title = normalizedAlertTitle(a) ? a.title.replace(/-\d+$/g, '') : hit.title
      hit.last_seen = a.last_seen || hit.last_seen
    }
  })
  return [...groups.values()]
})
const filtered = computed(() => groupedAlerts.value.filter(a => (!query.status || a.status === query.status) && (!query.severity || a.severity === query.severity) && (!query.target_ip || String(a.target_ip || '').includes(query.target_ip)) && (!query.business_id || a.business_id === query.business_id || a.linked_business_id === query.business_id) && (!query.test_batch_id || a.test_batch_id === query.test_batch_id) && (!keyword.value.trim() || [a.title, a.target_ip, a.assignee, a.ack_by, a.source].join(' ').toLowerCase().includes(keyword.value.trim().toLowerCase()))))
const stats = computed(() => ({ active: groupedAlerts.value.filter(a => !['resolved', 'archived'].includes(a.status)).length, owned: groupedAlerts.value.filter(a => a.ack_by || a.assignee).length, muted: groupedAlerts.value.filter(a => a.status === 'muted').length, folded: alerts.value.length - groupedAlerts.value.length + alerts.value.reduce((sum, a) => sum + Math.max(0, (a.count || 1) - 1), 0) }))
const fmt = t => t ? new Date(t).toLocaleString('zh-CN') : '-'; const short = v => v ? String(v).slice(0, 8) : '-'
const safeLabels = labels => JSON.stringify(Object.fromEntries(Object.entries(labels || {}).sort(([a], [b]) => a.localeCompare(b)).map(([key, value]) => [key, /token|secret|password|api[_-]?key|authorization|credential|private|bearer/i.test(key) ? '******' : value])), null, 2)
const severityType = s => ({ critical: 'danger', warning: 'warning', info: 'info' }[s] || 'info'); const statusType = s => ({ firing: 'danger', acknowledged: 'warning', assigned: 'warning', diagnosing: 'primary', muted: 'info', mitigated: 'warning', resolved: 'success', archived: 'info' }[s] || 'info')
const statusLabel = s => statuses.find(x => x.value === s)?.label || s || 'unknown'; const actionLabel = a => ({ created: '\u521b\u5efa', deduplicated: '\u91cd\u590d\u6298\u53e0', acknowledge: '\u8ba4\u9886', assign: '\u8f6c\u6d3e', mute: '\u9759\u9ed8', diagnosing: '\u8bca\u65ad', mitigate: '\u7f13\u89e3', resolved: '\u6062\u590d', archive: '\u5f52\u6863', deleted: '\u5220\u9664' }[a] || a)
const load = async () => { try { const { data } = await axios.get('/api/v1/alerts'); alerts.value = Array.isArray(data) ? data : []; if (selected.value) selected.value = alerts.value.find(a => a.id === selected.value.id) || selected.value } catch (e) { ElMessage.error(e.response?.data?.error || T.loadFailed) } }
const putAction = async (alert, name, payload = {}) => { try { await axios.put(`/api/v1/alerts/${alert.id}/${name}`, payload); ElMessage.success(T.updated); await load() } catch (e) { ElMessage.error(e.response?.data?.error || T.failed) } }
const action = async (alert, name) => putAction(alert, name, { actor: '\u5f53\u524d\u503c\u73ed\u4eba', reason: actionLabel(name) })
const assign = async alert => { const { value } = await ElMessageBox.prompt(T.assignPrompt, T.assign, { confirmButtonText: T.assign, cancelButtonText: '\u53d6\u6d88', inputValue: alert.assignee || 'DBA', inputPattern: /^.{1,40}$/, inputErrorMessage: T.assignPrompt }); await putAction(alert, 'assign', { actor: '\u5f53\u524d\u503c\u73ed\u4eba', assignee: value.trim(), reason: T.assign }) }
const mute = async alert => { const { value } = await ElMessageBox.prompt(T.mutePrompt, T.mute, { confirmButtonText: T.mute, cancelButtonText: '\u53d6\u6d88', inputValue: '60', inputPattern: /^([1-9]|[1-9]\d{1,2}|1[0-3]\d{2}|14[0-3]\d|1440)$/, inputErrorMessage: T.mutePrompt }); await putAction(alert, 'mute', { actor: '\u5f53\u524d\u503c\u73ed\u4eba', muted_minutes: Number(value), reason: T.mute }) }
const resolve = async alert => { try { await ElMessageBox.confirm(`${T.resolve} ${alert.title || alert.id}?`, T.confirm, { type: 'warning' }); await axios.put(`/api/v1/alerts/${alert.id}/resolve`); ElMessage.success(T.updated); await load() } catch (e) { if (e !== 'cancel') ElMessage.error(e.response?.data?.error || T.failed) } }
const remove = async alert => { try { await ElMessageBox.confirm(`${T.delete} ${alert.title || alert.id}?`, T.confirm, { type: 'warning' }); await axios.delete(`/api/v1/alerts/${alert.id}`); ElMessage.success(T.updated); await load() } catch (e) { if (e !== 'cancel') ElMessage.error(e.response?.data?.error || T.failed) } }
const diagnose = async a => { await putAction(a, 'diagnosing', { actor: '\u5f53\u524d\u503c\u73ed\u4eba', reason: 'AI diagnose' }); router.push({ path: '/workbench', query: { ip: a.target_ip, title: a.title, alert_id: a.id } }) }
let timer; onMounted(() => { load(); timer = setInterval(load, 10000) }); onUnmounted(() => clearInterval(timer))
</script>
<style scoped>
.alerts-page { padding: 28px 32px; min-height: 100vh; color: #243553; }.page-head { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px; }.page-head h2 { margin: 6px 0 0; font-size: 30px; letter-spacing: -.04em; color: #263653; }.page-head p { margin-top: 8px; color: #60728e; font-size: 13px; }.panel-kicker { font-size: 13px; color: #247cff; text-transform: uppercase; letter-spacing: .06em; font-weight: 800; }.head-actions { display: flex; gap: 10px; }.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 12px; }.stat { padding: 14px 16px; border-radius: 20px; background: rgba(255,255,255,.52); border: 1px solid rgba(255,255,255,.68); box-shadow: 0 12px 34px rgba(63,100,160,.12); }.stat b { display: block; font-size: 24px; color: #1f70ff; }.stat span { font-size: 12px; color: #60728e; }.filters { display: grid; grid-template-columns: 130px 120px 140px 140px 160px 1fr; gap: 10px; margin-bottom: 12px; }.glass-line { padding: 12px; border-radius: 20px; background: rgba(255,255,255,.42); border: 1px solid rgba(255,255,255,.68); }.event-workspace { display: grid; grid-template-columns: minmax(0, 1fr) 420px; gap: 14px; }.event-list { display: flex; flex-direction: column; gap: 10px; max-height: calc(100vh - 250px); overflow: auto; padding-right: 4px; }.event-card { display: flex; flex-direction: column; gap: 10px; padding: 14px 16px; border-radius: 20px; background: linear-gradient(145deg, rgba(255,255,255,.62), rgba(226,238,255,.46)); border: 1px solid rgba(255,255,255,.72); box-shadow: 0 14px 38px rgba(63,100,160,.13); cursor: pointer; }.event-card.firing { box-shadow: 0 16px 44px rgba(255,87,87,.12); }.event-main { min-width: 0; flex: 1; }.event-title { font-weight: 850; font-size: 16px; color: #253855; display: flex; align-items: center; gap: 8px; }.pulse { width: 10px; height: 10px; border-radius: 50%; background: #ff5b6b; box-shadow: 0 0 0 6px rgba(255,91,107,.12); }.event-meta, .event-owner { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 8px; color: #60728e; font-size: 12px; }.event-actions { display: flex; flex-wrap: wrap; gap: 6px; }.detail-panel { max-height: calc(100vh - 250px); overflow: auto; padding: 16px; border-radius: 22px; background: rgba(255,255,255,.56); border: 1px solid rgba(255,255,255,.72); box-shadow: 0 16px 44px rgba(63,100,160,.14); }.detail-head { display: flex; justify-content: space-between; align-items: center; }.detail-panel h3 { margin: 10px 0; line-height: 1.35; word-break: break-all; }.detail-tags { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 12px; }.detail-panel section { margin-top: 14px; }.detail-panel h4 { margin-bottom: 8px; color: #263653; }.timeline-row { display: grid; grid-template-columns: 86px 1fr; gap: 4px 8px; padding: 8px 0; border-bottom: 1px solid rgba(96,114,142,.14); font-size: 12px; }.timeline-row em, .timeline-row small { color: #74849e; font-style: normal; }pre { white-space: pre-wrap; word-break: break-all; padding: 10px; border-radius: 14px; background: rgba(36,124,255,.06); color: #40506a; font-size: 12px; }.empty { color: #74849e; text-align: center; padding: 80px; background: rgba(255,255,255,.34); border-radius: 24px; border: 1px solid rgba(255,255,255,.68); }
</style>
