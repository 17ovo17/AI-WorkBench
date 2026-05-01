<template>
  <div class="oncall-page">
    <div class="page-head">
      <div>
        <div class="panel-kicker">On-call & Notifications</div>
        <h2>值班与通知配置</h2>
        <p>值班组、通知渠道、升级策略和发送记录都走真实配置链路；测试发送会写入后端记录并可重试追踪。</p>
      </div>
      <div class="head-actions">
        <el-button :loading="loading" @click="reloadAll">刷新</el-button>
        <el-button type="primary" @click="openGroupDialog()">新增值班组</el-button>
        <el-button type="primary" plain @click="openChannelDialog()">新增渠道</el-button>
      </div>
    </div>

    <div class="summary-strip">
      <div><b>{{ groups.length }}</b><span>值班组</span></div>
      <div><b>{{ enabledChannels }}</b><span>已启用渠道</span></div>
      <div><b>{{ enabledEscalation }}</b><span>已启用升级步骤</span></div>
      <div><b>{{ records.length }}</b><span>可追踪发送记录</span></div>
    </div>

    <div class="layout-grid">
      <section class="panel-card">
        <div class="section-head">
          <div><h3>值班组</h3><span>配置负责人、时间窗口和升级角色</span></div>
          <el-button size="small" @click="openGroupDialog()">新增</el-button>
        </div>
        <div class="panel-scroll">
          <div v-for="group in groups" :key="group.id" class="row-card">
            <div>
              <b>{{ group.name }}</b>
              <span>{{ formatMembers(group.members) }}</span>
              <em>{{ group.schedule || '未配置时间窗口' }} · {{ group.role || 'primary' }}</em>
            </div>
            <div class="row-actions">
              <el-tag size="small" :type="group.enabled ? 'success' : 'info'">{{ group.enabled ? '启用' : '停用' }}</el-tag>
              <el-button size="small" link @click="openGroupDialog(group)">编辑</el-button>
              <el-button size="small" link type="danger" @click="removeGroup(group)">删除</el-button>
            </div>
          </div>
          <el-empty v-if="!groups.length" description="还没有值班组，请新增主值组。" />
        </div>
      </section>

      <section class="panel-card">
        <div class="section-head">
          <div><h3>通知渠道</h3><span>每个渠道可配置、可测试、可重试</span></div>
          <el-button size="small" @click="openChannelDialog()">新增</el-button>
        </div>
        <div class="panel-scroll">
          <div v-for="channel in channels" :key="channel.id" class="row-card channel-row">
            <div>
              <b>{{ channel.name }}</b>
              <span>{{ channelTypeLabel(channel.type) }}</span>
              <em>{{ displayEndpoint(channel) }}</em>
            </div>
            <div class="row-actions">
              <el-tag size="small" :type="channel.enabled ? 'success' : 'info'">{{ channel.enabled ? '已启用' : '待配置' }}</el-tag>
              <el-button size="small" link @click="openChannelDialog(channel)">配置</el-button>
              <el-button size="small" link :disabled="!channel.enabled" @click="testChannel(channel)">测试</el-button>
              <el-button size="small" link type="danger" :disabled="isBuiltinChannel(channel)" @click="removeChannel(channel)">删除</el-button>
            </div>
          </div>
          <el-empty v-if="!channels.length" description="还没有通知渠道，请新增 Webhook、Flashduty 或 PagerDuty。" />
        </div>
      </section>

      <section class="panel-card">
        <div class="section-head">
          <div><h3>升级策略</h3><span>按未确认、未恢复等条件逐级通知</span></div>
          <el-button size="small" @click="openStepDialog()">新增</el-button>
        </div>
        <div class="panel-scroll">
          <el-timeline>
            <el-timeline-item v-for="step in escalation" :key="step.id" :timestamp="`${step.delay_min || 0} 分钟`">
              <div class="timeline-card">
                <b>{{ step.action }}</b>
                <span>{{ step.condition || '告警触发' }} → {{ step.target }}</span>
                <div>
                  <el-tag size="small" :type="step.enabled ? 'success' : 'info'">{{ step.enabled ? '启用' : '停用' }}</el-tag>
                  <el-button size="small" link @click="openStepDialog(step)">编辑</el-button>
                  <el-button size="small" link type="danger" @click="removeStep(step)">删除</el-button>
                </div>
              </div>
            </el-timeline-item>
          </el-timeline>
          <el-empty v-if="!escalation.length" description="还没有升级步骤。" />
        </div>
      </section>

      <section class="panel-card">
        <div class="section-head">
          <div><h3>发送记录</h3><span>测试通知和重试会生成可追踪记录</span></div>
          <el-button size="small" :loading="recordsLoading" @click="loadRecords">刷新记录</el-button>
        </div>
        <div class="panel-scroll">
          <div v-for="item in records" :key="item.id" class="record-row">
            <div>
              <b>{{ item.channel }}</b>
              <span>{{ item.receiver || '未指定接收人' }}</span>
              <em>{{ formatTime(item.created_at) }}</em>
            </div>
            <div class="record-detail">{{ item.detail || item.trace_id }}</div>
            <div class="row-actions">
              <el-tag size="small" :type="item.status === 'success' ? 'success' : 'danger'">{{ item.status }}</el-tag>
              <el-button size="small" link @click="retryRecord(item)">重试</el-button>
            </div>
          </div>
          <el-empty v-if="!records.length" description="暂无发送记录。配置渠道后点击测试，会生成可追踪记录。" />
        </div>
      </section>
    </div>

    <el-dialog v-model="groupDialog" :title="groupDraft.id ? '编辑值班组' : '新增值班组'" width="520px">
      <el-form label-position="top">
        <el-form-item label="组名"><el-input v-model="groupDraft.name" placeholder="例如：SRE 主值" /></el-form-item>
        <el-form-item label="成员（逗号或换行分隔）"><el-input v-model="groupDraft.membersText" placeholder="admin,sre-primary,dba" /></el-form-item>
        <el-form-item label="值班时间"><el-input v-model="groupDraft.schedule" placeholder="00:00-24:00 / 工作日 09:00-18:00" /></el-form-item>
        <el-form-item label="角色"><el-input v-model="groupDraft.role" placeholder="primary / expert / owner" /></el-form-item>
        <el-form-item><el-switch v-model="groupDraft.enabled" active-text="启用" inactive-text="停用" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveGroup">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="channelDialog" :title="channelDraft.id ? '配置通知渠道' : '新增通知渠道'" width="560px">
      <el-form label-position="top">
        <el-form-item label="渠道名称"><el-input v-model="channelDraft.name" placeholder="例如：生产 Webhook" /></el-form-item>
        <el-form-item label="渠道类型">
          <el-select v-model="channelDraft.type" style="width: 100%">
            <el-option v-for="item in channelTypes" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="接收人/值班组"><el-input v-model="channelDraft.receiver" placeholder="SRE 主值 / oncall@example.com" /></el-form-item>
        <el-form-item label="Endpoint / Webhook"><el-input v-model="channelDraft.endpoint" placeholder="https://example.com/oncall/webhook" /></el-form-item>
        <el-form-item label="Token / Secret"><el-input v-model="channelDraft.secret" type="password" show-password placeholder="仅保存，不在列表明文展示" /></el-form-item>
        <el-form-item label="重试策略"><el-input v-model="channelDraft.retry_policy" placeholder="3 次，每次间隔 60s" /></el-form-item>
        <el-form-item><el-switch v-model="channelDraft.enabled" active-text="启用" inactive-text="停用" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="channelDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveChannel">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="stepDialog" :title="stepDraft.id ? '编辑升级步骤' : '新增升级步骤'" width="520px">
      <el-form label-position="top">
        <el-form-item label="延迟分钟"><el-input-number v-model="stepDraft.delay_min" :min="0" :max="1440" style="width: 100%" /></el-form-item>
        <el-form-item label="触发条件"><el-input v-model="stepDraft.condition" placeholder="未认领 / 未恢复 / P0/P1" /></el-form-item>
        <el-form-item label="动作"><el-input v-model="stepDraft.action" placeholder="重复提醒 / 升级通知" /></el-form-item>
        <el-form-item label="目标"><el-input v-model="stepDraft.target" placeholder="SRE 主值 / 专家组 / 负责人" /></el-form-item>
        <el-form-item><el-switch v-model="stepDraft.enabled" active-text="启用" inactive-text="停用" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="stepDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveStep">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import axios from 'axios'

const TEST_BATCH_ID = 'aiw-a3f7b2c1'
const loading = ref(false)
const saving = ref(false)
const recordsLoading = ref(false)
const groups = ref([])
const channels = ref([])
const escalation = ref([])
const records = ref([])
const groupDialog = ref(false)
const channelDialog = ref(false)
const stepDialog = ref(false)

const channelTypes = [
  { value: 'console', label: '平台内通知' },
  { value: 'webhook', label: 'Webhook' },
  { value: 'email', label: 'Email' },
  { value: 'flashduty', label: 'Flashduty' },
  { value: 'pagerduty', label: 'PagerDuty' },
  { value: 'dingtalk', label: '钉钉' },
  { value: 'feishu', label: '飞书' },
  { value: 'wecom', label: '企业微信' },
]

const groupDraft = reactive({ id: '', name: '', membersText: '', schedule: '', role: 'primary', enabled: true })
const channelDraft = reactive({ id: '', name: '', type: 'webhook', endpoint: '', webhook: '', receiver: '', secret: '', retry_policy: '3 次，每次间隔 60s', enabled: true })
const stepDraft = reactive({ id: '', delay_min: 0, target: '', action: '', condition: '', enabled: true })

const enabledChannels = computed(() => channels.value.filter(item => item.enabled).length)
const enabledEscalation = computed(() => escalation.value.filter(item => item.enabled).length)

const channelTypeLabel = type => channelTypes.find(item => item.value === type)?.label || type || '未知渠道'
const formatTime = value => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
const isBuiltinChannel = channel => ['console', 'webhook', 'flashduty', 'pagerduty'].includes(channel.id)
const formatMembers = members => (members || []).join('、') || '未配置成员'
const primaryReceiver = () => groups.value.find(item => item.enabled)?.name || '值班人员'

const displayEndpoint = channel => {
  if (channel.type === 'console') return '平台内留痕'
  return channel.endpoint || channel.webhook || '未配置 Endpoint'
}

const normalizeChannels = items => {
  const defaults = [
    { id: 'console', name: 'Console', type: 'console', receiver: '平台值班人员', enabled: true, endpoint: '' },
    { id: 'webhook', name: 'Webhook', type: 'webhook', receiver: '', enabled: false, endpoint: '' },
    { id: 'flashduty', name: 'Flashduty', type: 'flashduty', receiver: '', enabled: false, endpoint: '' },
    { id: 'pagerduty', name: 'PagerDuty', type: 'pagerduty', receiver: '', enabled: false, endpoint: '' },
  ]
  const byID = new Map(defaults.map(item => [item.id, item]))
  ;(items || []).forEach(item => byID.set(item.id || `${item.type}-${item.name}`, { ...byID.get(item.id), ...item }))
  return [...byID.values()]
}

const loadConfig = async () => {
  const { data } = await axios.get('/api/v1/oncall/config')
  escalation.value = (data.escalation || []).sort((left, right) => Number(left.delay_min || 0) - Number(right.delay_min || 0))
}

const loadGroups = async () => {
  const { data } = await axios.get('/api/v1/oncall/groups')
  groups.value = data.items || data || []
}

const loadChannels = async () => {
  const { data } = await axios.get('/api/v1/oncall/channels')
  channels.value = normalizeChannels(data.items || data || [])
}

const loadRecords = async () => {
  recordsLoading.value = true
  try {
    const { data } = await axios.get('/api/v1/oncall/records')
    records.value = data.items || data || []
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '加载通知发送记录失败')
  } finally {
    recordsLoading.value = false
  }
}

const reloadAll = async () => {
  loading.value = true
  try {
    await Promise.all([loadConfig(), loadGroups(), loadChannels(), loadRecords()])
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '加载值班通知配置失败')
  } finally {
    loading.value = false
  }
}

const saveConfig = async () => {
  await axios.post('/api/v1/oncall/config', { groups: groups.value, escalation: escalation.value })
}

const openGroupDialog = group => {
  Object.assign(groupDraft, {
    id: group?.id || '',
    name: group?.name || '',
    membersText: (group?.members || []).join(','),
    schedule: group?.schedule || '',
    role: group?.role || 'primary',
    enabled: group?.enabled ?? true,
  })
  groupDialog.value = true
}

const saveGroup = async () => {
  const name = groupDraft.name.trim()
  if (!name) return ElMessage.warning('请填写值班组名称')
  const item = {
    id: groupDraft.id || `group_${Date.now()}`,
    name,
    members: groupDraft.membersText.split(/[,，\n]/).map(member => member.trim()).filter(Boolean),
    schedule: groupDraft.schedule.trim(),
    role: groupDraft.role.trim() || 'primary',
    enabled: groupDraft.enabled,
  }
  saving.value = true
  try {
    const { data } = await axios.post('/api/v1/oncall/groups', item)
    groups.value = groups.value.some(group => group.id === data.id)
      ? groups.value.map(group => group.id === data.id ? data : group)
      : [...groups.value, data]
    groupDialog.value = false
    ElMessage.success('值班组已保存')
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '保存值班组失败')
  } finally {
    saving.value = false
  }
}

const removeGroup = async group => {
  await ElMessageBox.confirm(`确认删除值班组「${group.name}」？`, '删除值班组', { type: 'warning' })
  await axios.delete(`/api/v1/oncall/groups/${group.id}`)
  groups.value = groups.value.filter(item => item.id !== group.id)
  ElMessage.success('值班组已删除')
}

const openChannelDialog = channel => {
  Object.assign(channelDraft, {
    id: channel?.id || '',
    name: channel?.name || '',
    type: channel?.type || 'webhook',
    endpoint: channel?.endpoint || channel?.webhook || '',
    webhook: channel?.webhook || channel?.endpoint || '',
    receiver: channel?.receiver || '',
    secret: channel?.secret || '',
    retry_policy: channel?.retry_policy || '3 次，每次间隔 60s',
    enabled: channel?.enabled ?? true,
  })
  channelDialog.value = true
}

const saveChannel = async () => {
  const name = channelDraft.name.trim()
  if (!name) return ElMessage.warning('请填写渠道名称')
  if (channelDraft.enabled && channelDraft.type !== 'console' && !channelDraft.endpoint.trim()) {
    return ElMessage.warning('启用外部通知渠道前必须配置 Endpoint')
  }
  const item = { ...channelDraft, id: channelDraft.id || `${channelDraft.type}_${Date.now()}` }
  item.webhook = item.endpoint
  saving.value = true
  try {
    const { data } = await axios.post('/api/v1/oncall/channels', item)
    channels.value = normalizeChannels(channels.value.map(channel => channel.id === data.id ? data : channel).concat(channels.value.some(channel => channel.id === data.id) ? [] : [data]))
    channelDialog.value = false
    ElMessage.success('通知渠道已保存')
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '保存通知渠道失败')
  } finally {
    saving.value = false
  }
}

const removeChannel = async channel => {
  await ElMessageBox.confirm(`确认删除通知渠道「${channel.name}」？`, '删除通知渠道', { type: 'warning' })
  await axios.delete(`/api/v1/oncall/channels/${channel.id}`)
  channels.value = channels.value.filter(item => item.id !== channel.id)
  ElMessage.success('通知渠道已删除')
}

const testChannel = async channel => {
  try {
    const { data } = await axios.post('/api/v1/oncall/test-send', {
      channel: channel.type,
      channel_id: channel.id,
      receiver: channel.receiver || primaryReceiver(),
      business_id: 'manual-check',
      alert_title: '值班通知链路验证',
      test_batch_id: TEST_BATCH_ID,
    })
    records.value = [data, ...records.value.filter(item => item.id !== data.id)]
    ElMessage.success('测试通知已发送并写入记录')
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '测试通知发送失败')
  }
}

const openStepDialog = step => {
  Object.assign(stepDraft, {
    id: step?.id || '',
    delay_min: step?.delay_min ?? 0,
    target: step?.target || '',
    action: step?.action || '',
    condition: step?.condition || '',
    enabled: step?.enabled ?? true,
  })
  stepDialog.value = true
}

const saveStep = async () => {
  if (!stepDraft.target.trim() || !stepDraft.action.trim()) return ElMessage.warning('请填写升级目标和动作')
  const item = { ...stepDraft, id: stepDraft.id || `esc_${Date.now()}`, delay_min: Number(stepDraft.delay_min || 0) }
  saving.value = true
  try {
    escalation.value = escalation.value.some(step => step.id === item.id)
      ? escalation.value.map(step => step.id === item.id ? item : step)
      : [...escalation.value, item]
    escalation.value.sort((left, right) => Number(left.delay_min || 0) - Number(right.delay_min || 0))
    await saveConfig()
    stepDialog.value = false
    ElMessage.success('升级策略已保存')
  } catch (error) {
    ElMessage.error(error.response?.data?.error || '保存升级策略失败')
  } finally {
    saving.value = false
  }
}

const removeStep = async step => {
  await ElMessageBox.confirm(`确认删除升级步骤「${step.action}」？`, '删除升级步骤', { type: 'warning' })
  escalation.value = escalation.value.filter(item => item.id !== step.id)
  await saveConfig()
  ElMessage.success('升级步骤已删除')
}

const retryRecord = async record => {
  await axios.post('/api/v1/oncall/test-send', {
    channel: record.channel || 'console',
    receiver: record.receiver || primaryReceiver(),
    retry_of: record.id,
    business_id: 'manual-retry',
    alert_title: '值班通知重试',
    test_batch_id: TEST_BATCH_ID,
  }).then(({ data }) => {
    records.value = [data, ...records.value]
    ElMessage.success('重试已写入发送记录')
  }).catch(error => ElMessage.error(error.response?.data?.error || '重试失败'))
}

onMounted(reloadAll)
</script>

<style scoped>
.oncall-page { height: 100%; min-height: 0; color: #243553; display: flex; flex-direction: column; gap: 14px; overflow: hidden; }
.page-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 18px; }
.page-head h2 { margin: 6px 0 0; font-size: 24px; letter-spacing: -.03em; color: #263653; }
.page-head p { margin-top: 6px; color: #60728e; font-size: 13px; max-width: 860px; line-height: 1.6; }
.head-actions { display: flex; gap: 10px; flex-wrap: wrap; justify-content: flex-end; }
.panel-kicker { font-size: 12px; color: #247cff; text-transform: uppercase; letter-spacing: .06em; font-weight: 800; }
.summary-strip { display: grid; grid-template-columns: repeat(4, minmax(120px, 1fr)); gap: 12px; }
.summary-strip div { padding: 14px 16px; border-radius: 18px; background: rgba(255,255,255,.58); border: 1px solid rgba(255,255,255,.72); box-shadow: 0 12px 32px rgba(63,100,160,.11); }
.summary-strip b { display: block; font-size: 24px; color: #182844; }
.summary-strip span { color: #60728e; font-size: 12px; }
.layout-grid { flex: 1; min-height: 0; display: grid; grid-template-columns: repeat(2, minmax(360px, 1fr)); gap: 14px; }
.panel-card { min-height: 0; padding: 16px; border-radius: 20px; background: linear-gradient(145deg, rgba(255,255,255,.58), rgba(225,236,255,.42)); border: 1px solid rgba(255,255,255,.72); box-shadow: 0 16px 44px rgba(63,100,160,.13); display: flex; flex-direction: column; }
.section-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 14px; margin-bottom: 10px; }
.section-head h3 { margin: 0 0 4px; }
.section-head span { color: #60728e; font-size: 12px; }
.panel-scroll { flex: 1; min-height: 0; overflow: auto; padding-right: 4px; }
.row-card, .record-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 12px; align-items: center; padding: 12px 0; border-bottom: 1px solid rgba(96,114,142,.12); }
.row-card b, .record-row b { display: block; color: #223553; margin-bottom: 4px; }
.row-card span, .row-card em, .record-row span, .record-row em { display: block; color: #60728e; font-style: normal; font-size: 12px; line-height: 1.5; overflow-wrap: anywhere; }
.row-actions { display: flex; flex-wrap: wrap; gap: 4px; justify-content: flex-end; align-items: center; }
.record-row { grid-template-columns: minmax(180px, .9fr) minmax(220px, 1.2fr) auto; }
.record-detail { color: #405875; font-size: 12px; line-height: 1.55; overflow-wrap: anywhere; }
.timeline-card { display: grid; gap: 5px; color: #405875; }
.timeline-card b { color: #223553; }
.timeline-card span { color: #60728e; font-size: 12px; }
@media (max-width: 1180px) {
  .summary-strip, .layout-grid { grid-template-columns: 1fr; }
  .record-row { grid-template-columns: 1fr; }
}
</style>
