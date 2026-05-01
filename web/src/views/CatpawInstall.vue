<template>
  <div class="catpaw-page">
    <div class="page-head">
      <div><div class="panel-kicker">Catpaw Agent</div><h2>探针管理</h2></div>
    </div>

    <div class="workspace">
      <!-- 左：在线探针列表 -->
      <div class="agents-panel panel-card">
        <div class="panel-title">在线探针</div>
        <div v-if="!agents.length" class="empty">暂无探针</div>
        <div v-for="a in agents" :key="a.ip" class="agent-item">
          <div class="agent-row" @click="a._expanded = !a._expanded">
            <span class="dot" :class="{ online: a.online }"></span>
            <div class="agent-info">
              <span class="agent-ip">{{ a.ip }}</span>
              <span class="agent-host">{{ a.hostname }}</span>
            </div>
            <el-tag :type="a.online ? 'success' : 'danger'" size="small">{{ a.online ? '在线' : '离线' }}</el-tag>
            <el-icon style="color:#8b949e"><ArrowDown /></el-icon>
          </div>
          <div v-if="a._expanded" class="agent-actions">
            <div class="action-row">
              <span class="action-label">切换模式</span>
              <el-select v-model="a._mode" size="small" style="width:140px">
                <el-option value="run" label="run — 持续监控" />
                <el-option value="inspect cpu" label="inspect — 巡检" />
                <el-option value="selftest" label="selftest — 测试" />
              </el-select>
              <el-button size="small" type="primary" plain @click.stop="switchMode(a)">切换</el-button>
            </div>
            <div class="action-row">
              <span class="action-label">插件配置</span>
              <el-button size="small" @click.stop="openPluginConfig(a)">配置插件</el-button>
            </div>
            <div class="action-row">
              <el-button size="small" type="danger" plain @click.stop="uninstall(a)">卸载探针</el-button>
              <el-button size="small" type="danger" text @click.stop="deleteAgentRecord(a)">删除主机</el-button>
            </div>
          </div>
        </div>
      </div>

      <!-- 右：功能 tabs -->
      <div class="install-panel panel-card">
        <el-tabs v-model="tab">

          <!-- 凭证管理 -->
          <el-tab-pane label="凭证管理" name="creds">
            <el-button size="small" type="primary" @click="credDialog = true; editCred = {protocol:'ssh',port:22}">新增凭证</el-button>
            <div class="cred-list">
              <div v-for="c in credentials" :key="c.id" class="cred-row">
                <el-tag size="small" :type="c.protocol === 'ssh' ? 'success' : 'warning'">{{ c.protocol.toUpperCase() }}</el-tag>
                <span class="cred-name">{{ c.name }}</span>
                <span class="cred-user">{{ c.username }}@:{{ c.port || (c.protocol==='ssh'?22:5985) }}</span>
                <span class="cred-remark">{{ c.remark }}</span>
                <el-button size="small" @click="editCred = {...c}; credDialog = true">编辑</el-button>
                <el-button size="small" type="danger" plain @click="delCred(c.id)">删除</el-button>
              </div>
              <div v-if="!credentials.length" class="empty">暂无凭证</div>
            </div>
          </el-tab-pane>

          <!-- SSH 一键安装 -->
          <el-tab-pane label="远程安装" name="ssh">
            <el-form :model="sshForm" label-width="90px" size="small">
              <el-form-item label="目标系统">
                <el-radio-group v-model="sshForm.os" @change="v => { sshForm.protocol = v==='linux'?'ssh':'winrm'; sshForm.port = v==='linux'?22:5985; sshForm.mode = v==='linux'?'run':'inspect cpu' }">
                  <el-radio-button value="linux">Linux (SSH)</el-radio-button>
                  <el-radio-button value="windows">Windows (WinRM/WMI)</el-radio-button>
                </el-radio-group>
              </el-form-item>
              <el-form-item label="认证方式">
                <el-radio-group v-model="sshAuthMode">
                  <el-radio-button value="cred">使用凭证</el-radio-button>
                  <el-radio-button value="manual">手动输入</el-radio-button>
                </el-radio-group>
              </el-form-item>
              <el-form-item v-if="sshAuthMode==='cred'" label="选择凭证">
                <el-select v-model="selectedCred" placeholder="选择凭证" clearable @change="applyCred" style="width:100%">
                  <el-option v-for="c in credentials" :key="c.id" :label="c.name" :value="c.id" />
                </el-select>
              </el-form-item>
              <template v-else>
                <el-form-item label="用户名"><el-input v-model="sshForm.username" /></el-form-item>
                <el-form-item label="密码"><el-input v-model="sshForm.password" type="password" show-password /></el-form-item>
                <el-form-item v-if="sshForm.os==='linux'" label="SSH 密钥"><el-input v-model="sshForm.ssh_key" type="textarea" :rows="2" placeholder="粘贴私钥（可选）" /></el-form-item>
              </template>
              <el-form-item label="目标 IP"><el-input v-model="sshForm.ip" /></el-form-item>
              <el-form-item label="端口"><el-input v-model.number="sshForm.port" :placeholder="sshForm.os==='linux'?'22':'5985'" /></el-form-item>
              <el-form-item v-if="sshForm.os==='windows'" label="Windows协议">
                <el-radio-group v-model="sshForm.protocol" @change="v => sshForm.port = v==='wmi'?135:5985">
                  <el-radio-button value="winrm">WinRM</el-radio-button>
                  <el-radio-button value="wmi">WMI/DCOM</el-radio-button>
                </el-radio-group>
              </el-form-item>
              <el-form-item label="平台地址"><el-input v-model="reportURL" /></el-form-item>
              <el-form-item v-if="sshForm.os==='windows'" label="RDP引导">
                <div class="rdp-guide">
                  <div class="rdp-guide-title">首次接入 Windows 时，若 WinRM 5985 不通，请先 RDP 登录目标机并以管理员 PowerShell 执行引导命令。</div>
                  <div class="rdp-guide-actions">
                    <el-button size="small" @click="checkWinRMPort">检测 5985</el-button>
                    <el-button size="small" type="primary" plain @click="genRdpBootstrap">生成 RDP 引导命令</el-button>
                  </div>
                  <div v-if="winrmCheckResult" class="rdp-check">{{ winrmCheckResult }}</div>
                </div>
              </el-form-item>
              <el-form-item label="启动模式">
                <el-select v-model="sshForm.mode" style="width:100%">
                  <template v-if="sshForm.os==='linux'">
                    <el-option value="run" label="run — 持续监控采集，后台运行，推荐生产环境" />
                    <el-option value="inspect cpu" label="inspect — 主动健康巡检" />
                    <el-option value="diagnose list" label="diagnose — 查看历史诊断记录" />
                    <el-option value="selftest" label="selftest — 冒烟测试所有诊断工具" />
                  </template>
                  <template v-else>
                    <el-option value="inspect cpu" label="inspect — 主动健康巡检" />
                    <el-option value="diagnose list" label="diagnose — 查看历史诊断记录" />
                    <el-option value="selftest" label="selftest — 冒烟测试所有诊断工具" />
                  </template>
                </el-select>
              </el-form-item>
              <el-form-item>
                <el-button type="primary" :loading="installing" @click="installSSH">一键安装</el-button>
              </el-form-item>
            </el-form>
            <pre v-if="installOutput" class="code-block">{{ installOutput }}</pre>
            <pre v-if="rdpBootstrapCmd" class="code-block">{{ rdpBootstrapCmd }}</pre>
          </el-tab-pane>

          <!-- 生成命令 -->
          <el-tab-pane label="生成命令" name="cmd">
            <el-radio-group v-model="cmdProtocol" size="small" style="margin-bottom:12px">
              <el-radio-button value="curl">Linux (curl)</el-radio-button>
              <el-radio-button value="winrm">Windows (WinRM)</el-radio-button>
              <el-radio-button value="rdp-winrm">RDP 启用 WinRM</el-radio-button>
              <el-radio-button value="wmi">Windows (WMI)</el-radio-button>
            </el-radio-group>
            <el-input v-model="reportURL" placeholder="平台地址" size="small" style="margin-bottom:8px" />
            <el-button size="small" @click="genCmd">生成</el-button>
            <pre v-if="generatedCmd" class="code-block">{{ generatedCmd }}</pre>
          </el-tab-pane>

          <!-- 远程执行 -->
          <el-tab-pane label="远程执行" name="exec">
            <el-form :model="execForm" label-width="90px" size="small">
              <el-form-item label="认证方式">
                <el-radio-group v-model="execAuthMode">
                  <el-radio-button value="cred">使用凭证</el-radio-button>
                  <el-radio-button value="manual">手动输入</el-radio-button>
                </el-radio-group>
              </el-form-item>
              <el-form-item v-if="execAuthMode==='cred'" label="选择凭证">
                <el-select v-model="selectedExecCred" placeholder="选择凭证" clearable @change="applyExecCred" style="width:100%">
                  <el-option v-for="c in credentials" :key="c.id" :label="c.name" :value="c.id" />
                </el-select>
              </el-form-item>
              <template v-if="execAuthMode==='manual'">
                <el-form-item label="协议">
                  <el-radio-group v-model="execForm.protocol" @change="v => execForm.port = v==='ssh'?22:(v==='wmi'?135:5985)">
                    <el-radio-button value="ssh">SSH</el-radio-button>
                    <el-radio-button value="winrm">WinRM</el-radio-button>
                    <el-radio-button value="wmi">WMI/DCOM</el-radio-button>
                  </el-radio-group>
                </el-form-item>
                <el-form-item label="用户名"><el-input v-model="execForm.username" /></el-form-item>
                <el-form-item label="密码"><el-input v-model="execForm.password" type="password" show-password /></el-form-item>
                <el-form-item label="SSH 密钥"><el-input v-model="execForm.ssh_key" type="textarea" :rows="2" placeholder="可选" /></el-form-item>
              </template>
              <el-form-item label="目标 IP"><el-input v-model="execForm.ip" /></el-form-item>
              <el-form-item label="端口"><el-input v-model.number="execForm.port" :placeholder="execForm.protocol==='ssh'?'22':'5985'" /></el-form-item>
              <el-form-item label="执行类型">
                <el-radio-group v-model="execType">
                  <el-radio-button value="cmd">自定义命令</el-radio-button>
                  <el-radio-button value="catpaw">catpaw 模式</el-radio-button>
                </el-radio-group>
              </el-form-item>
              <el-form-item v-if="execType==='cmd'" label="命令">
                <el-input v-model="execForm.command" type="textarea" :rows="3" />
              </el-form-item>
              <el-form-item v-else label="catpaw 模式">
                <el-select v-model="catpawMode" style="width:100%">
                  <el-option value="run" label="run — 持续监控采集，后台运行" />
                  <el-option value="chat" label="chat — 交互式 AI 排障" />
                  <el-option value="inspect cpu" label="inspect — 主动健康巡检" />
                  <el-option value="diagnose list" label="diagnose — 查看历史诊断记录" />
                  <el-option value="selftest" label="selftest — 冒烟测试所有诊断工具" />
                </el-select>
              </el-form-item>
              <el-form-item>
                <el-button type="primary" :loading="executing" @click="execRemote">执行</el-button>
              </el-form-item>
            </el-form>
            <pre v-if="execOutput" class="code-block">{{ execOutput }}</pre>
          </el-tab-pane>

          <!-- 探针对话 -->
          <el-tab-pane label="探针对话" name="catpaw-chat">
            <CatpawChatPanel :credentials="credentials" />
          </el-tab-pane>

        </el-tabs>
      </div>
    </div>

    <!-- 插件配置弹窗 -->
    <el-dialog v-model="pluginDialogVisible" :title="`插件配置 - ${pluginDialogAgent?.ip}`" width="700px">
      <PluginConfig v-if="pluginDialogAgent" :preset-agent="pluginDialogAgent" :credentials="credentials" @generate="onPluginConfig" />
    </el-dialog>

    <!-- 凭证编辑弹窗 -->
    <el-dialog v-model="credDialog" :title="editCred.id ? '编辑凭证' : '新增凭证'" width="480px">
      <el-form :model="editCred" label-width="80px" size="small">
        <el-form-item label="名称"><el-input v-model="editCred.name" /></el-form-item>
        <el-form-item label="协议">
          <el-radio-group v-model="editCred.protocol" @change="v => editCred.port = v==='ssh'?22:5985">
            <el-radio-button value="ssh">SSH</el-radio-button>
            <el-radio-button value="winrm">WinRM</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="用户名"><el-input v-model="editCred.username" /></el-form-item>
        <el-form-item label="端口"><el-input v-model.number="editCred.port" :placeholder="editCred.protocol==='ssh'?'22':'5985'" /></el-form-item>
        <el-form-item label="密码"><el-input v-model="editCred.password" type="password" show-password /></el-form-item>
        <el-form-item label="SSH 密钥"><el-input v-model="editCred.ssh_key" type="textarea" :rows="3" placeholder="粘贴私钥内容（可选）" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="editCred.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="credDialog = false">取消</el-button>
        <el-button type="primary" @click="saveCred">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import axios from 'axios'
import PluginConfig from './PluginConfig.vue'
import CatpawChatPanel from './CatpawChatPanel.vue'

const agents = ref([])
const credentials = ref([])
const tab = ref('creds')
const reportURL = ref(window.location.origin)

const loadPlatformIP = async () => {
  try {
    const { data } = await axios.get('/api/v1/platform/ip')
    if (data.ip) reportURL.value = `http://${data.ip}:8080`
  } catch {}
}
const cmdProtocol = ref('curl')
const generatedCmd = ref('')
const installOutput = ref('')
const execOutput = ref('')
const installing = ref(false)
const executing = ref(false)
const credDialog = ref(false)
const editCred = ref({ protocol: 'ssh', port: 22 })
const selectedCred = ref('')
const selectedExecCred = ref('')
const pluginConfigOutput = ref('')
const rdpBootstrapCmd = ref('')
const winrmCheckResult = ref('')
const execAuthMode = ref('cred')
const execType = ref('cmd')
const catpawMode = ref('run')
const sshAuthMode = ref('cred')

const onPluginConfig = (config) => { pluginConfigOutput.value = config }

const sshForm = ref({ protocol: 'ssh', ip: '', port: 22, username: 'root', password: '', ssh_key: '', mode: 'run', os: 'linux' })
const execForm = ref({ protocol: 'ssh', ip: '', port: 22, username: 'root', password: '', ssh_key: '', command: '' })

const fmt = t => t ? new Date(t).toLocaleString('zh-CN') : '-'
const dangerCommandPattern = /(rm\s+-rf|Remove-Item\s+.*-Recurse|\bdel\s+|\brd\s+\/s|DROP\s+DATABASE|TRUNCATE\s+TABLE|FLUSHALL|\bformat\s+|\bmkfs|Stop-Process|\bpkill\b|schtasks\s+\/Delete)/i

const confirmDangerAction = async ({ title, target, command, risk = 'L3', scope = '白名单 Catpaw 测试范围' }) => {
  const escaped = (command || '').replace(/[<>&]/g, c => ({ '<': '&lt;', '>': '&gt;', '&': '&amp;' }[c]))
  await ElMessageBox.confirm(
    `<div style="line-height:1.7"><p><b>目标：</b>${target || '-'}</p><p><b>风险：</b>${risk}，仅允许${scope}。</p>${escaped ? `<pre style="white-space:pre-wrap;max-height:160px;overflow:auto">${escaped}</pre>` : ''}<p>确认结果会回传后端，后端仍会二次校验主机、路径和命令安全边界。</p></div>`,
    title || '确认高风险操作',
    { type: 'warning', dangerouslyUseHTMLString: true, confirmButtonText: '确认执行', cancelButtonText: '取消' }
  )
  return `ALLOW-${risk}`
}

const agentOS = agent => {
  const text = `${agent?.os || ''} ${agent?.platform || ''} ${agent?.hostname || ''} ${agent?.version || ''}`.toLowerCase()
  if (text.includes('windows') || text.includes('desktop')) return 'windows'
  return 'linux'
}

const uninstallPlan = agent => agentOS(agent) === 'windows'
  ? { protocol: 'winrm', port: 5985, command: '停止 Catpaw 计划任务/进程；仅删除 C:\\catpaw', scope: 'Windows 目标的 C:\\catpaw、Catpaw 任务和 Catpaw 进程' }
  : { protocol: 'ssh', port: 22, command: '停止 catpaw；仅删除 /usr/local/bin/catpaw、/etc/catpaw、/var/log/catpaw*', scope: 'Linux 目标的 /usr/local/bin/catpaw、/etc/catpaw、/var/log/catpaw*' }

const loadAgents = async () => {
  const { data } = await axios.get('/api/v1/catpaw/agents')
  agents.value = data || []
}

const loadCreds = async () => {
  const { data } = await axios.get('/api/v1/credentials')
  credentials.value = data || []
}

const saveCred = async () => {
  await axios.post('/api/v1/credentials', editCred.value)
  credDialog.value = false
  loadCreds()
}

const delCred = async (id) => {
  await ElMessageBox.confirm('确认删除该凭证？', '提示', { type: 'warning' })
  await axios.delete(`/api/v1/credentials/${id}`)
  loadCreds()
}

const applyCred = (id) => {
  const c = credentials.value.find(x => x.id === id)
  if (!c) return
  sshForm.value.username = c.username
  sshForm.value.port = c.port || (c.protocol === 'wmi' ? 135 : (c.protocol === 'winrm' ? 5985 : 22))
  sshForm.value.os = (c.protocol === 'winrm' || c.protocol === 'wmi') ? 'windows' : 'linux'
  sshForm.value.protocol = c.protocol || (sshForm.value.os === 'windows' ? 'winrm' : 'ssh')
}

const applyExecCred = (id) => {
  const c = credentials.value.find(x => x.id === id)
  if (!c) return
  execForm.value.username = c.username
  execForm.value.port = c.port || (c.protocol === 'wmi' ? 135 : (c.protocol === 'winrm' ? 5985 : 22))
  execForm.value.protocol = c.protocol
}

const installSSH = async () => {
  installing.value = true
  installOutput.value = ''
  try {
    // 构建凭证
    let payload = { ...sshForm.value }
    if (sshAuthMode.value === 'cred' && selectedCred.value) {
      const c = credentials.value.find(x => x.id === selectedCred.value)
      if (c) Object.assign(payload, { username: c.username, port: c.port, protocol: c.protocol })
      payload.credential_id = selectedCred.value
      delete payload.password
      delete payload.ssh_key
    }
    // Windows 用 WinRM 安装
    if (sshForm.value.os === 'windows' && !payload.protocol) {
      payload.protocol = 'winrm'
    }
    await confirmDangerAction({ title: 'Confirm Catpaw remote install', target: `${payload.ip}:${payload.port || ''}`, command: `install catpaw (${payload.protocol || 'ssh'})`, risk: 'L3' })
    const { data } = await axios.post('/api/v1/remote/install-catpaw', payload, {
      headers: { 'X-Platform-URL': reportURL.value }
    })
    installOutput.value = data.output || '安装成功'
    loadAgents()
  } catch (e) {
    installOutput.value = e.response?.data?.error || e.message
  } finally {
    installing.value = false
  }
}

const uninstall = async (agent) => {
  const ip = agent?.ip || agent
  const plan = uninstallPlan(agent)
  const safetyConfirm = await confirmDangerAction({ title: '确认卸载 Catpaw 探针', target: `${ip}（${agentOS(agent) === 'windows' ? 'Windows' : 'Linux'}）`, command: plan.command, risk: 'L3', scope: plan.scope })
  const cred = { ...sshForm.value }
  cred.protocol = plan.protocol
  cred.port = cred.port || plan.port
  if (selectedCred.value) {
    cred.credential_id = selectedCred.value
    delete cred.password
    delete cred.ssh_key
  }
  try {
    const { data } = await axios.post('/api/v1/remote/uninstall-catpaw', { ...cred, ip, safety_confirm: safetyConfirm })
    ElMessage.success(data.output || '卸载已完成')
    loadAgents()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || e.message)
  }
}

const deleteAgentRecord = async (agent) => {
  await ElMessageBox.confirm(`确认删除主机 ${agent.ip} 的探针记录？这不会远程卸载，只移除平台里的离线/废弃主机占位。`, '删除主机记录', {
    type: 'warning',
    confirmButtonText: '删除主机',
    cancelButtonText: '取消'
  })
  await axios.delete(`/api/v1/catpaw/agents/${encodeURIComponent(agent.ip)}`)
  ElMessage.success('主机记录已删除')
  loadAgents()
}

const switchMode = async (agent) => {
  if (!agent._mode) return
  // 找匹配凭证
  const cred = credentials.value.find(c => c.ip === agent.ip) || sshForm.value
  const cmd = `pkill -f 'catpaw' 2>/dev/null; nohup catpaw ${agent._mode} --configs /etc/catpaw/conf.d > /var/log/catpaw.log 2>&1 & echo "已切换到 ${agent._mode} 模式"`
  try {
    const safetyConfirm = await confirmDangerAction({ title: 'Confirm Catpaw mode switch', target: agent.ip, command: cmd, risk: 'L3' })
    const { data } = await axios.post('/api/v1/remote/exec', { ...cred, ip: agent.ip, command: cmd, safety_confirm: safetyConfirm })
    ElMessage.success(data.output || `已切换到 ${agent._mode}`)
  } catch (e) {
    ElMessage.error(e.response?.data?.error || e.message)
  }
}

const pluginDialogAgent = ref(null)
const pluginDialogVisible = ref(false)

const openPluginConfig = (agent) => {
  pluginDialogAgent.value = agent
  pluginDialogVisible.value = true
}

const genCmd = async () => {
  const { data } = await axios.post('/api/v1/remote/install-cmd', {
    protocol: cmdProtocol.value,
    report_url: reportURL.value
  })
  generatedCmd.value = data.command
}

const genRdpBootstrap = async () => {
  const { data } = await axios.post('/api/v1/remote/install-cmd', {
    protocol: 'rdp-winrm',
    report_url: reportURL.value
  })
  rdpBootstrapCmd.value = data.command
}

const checkWinRMPort = async () => {
  if (!sshForm.value.ip) {
    winrmCheckResult.value = '请先填写目标 IP'
    return
  }
  const { data } = await axios.post('/api/v1/remote/check-port', {
    ip: sshForm.value.ip,
    port: 5985
  })
  winrmCheckResult.value = data.reachable ? `${data.address} 可达，可以使用 WinRM 安装` : `${data.address} 不可达：${data.error || '连接失败'}`
}

const execRemote = async () => {
  executing.value = true
  execOutput.value = ''
  try {
    // 构建凭证
    let cred = { ...execForm.value }
    if (execAuthMode.value === 'cred' && selectedExecCred.value) {
      const c = credentials.value.find(x => x.id === selectedExecCred.value)
      if (c) Object.assign(cred, { username: c.username, port: c.port, protocol: c.protocol })
      cred.credential_id = selectedExecCred.value
      delete cred.password
      delete cred.ssh_key
    }
    // 构建命令
    if (execType.value === 'catpaw') {
      cred.command = `catpaw ${catpawMode.value} --configs /etc/catpaw/conf.d`
    }
    cred.safety_confirm = await confirmDangerAction({ title: 'Confirm remote command execution', target: cred.ip, command: cred.command, risk: 'L3' })
    const { data } = await axios.post('/api/v1/remote/exec', cred)
    execOutput.value = data.output
  } catch (e) {
    execOutput.value = e.response?.data?.error || e.message
  } finally {
    executing.value = false
  }
}

let timer
onMounted(() => { loadAgents(); loadCreds(); loadPlatformIP(); timer = setInterval(loadAgents, 15000) })
onUnmounted(() => clearInterval(timer))
</script>


<style scoped>
.catpaw-page { padding: 28px 32px; height: 100vh; overflow: hidden; color: #243553; }
.page-head { margin-bottom: 16px; }
.page-head h2 { margin: 8px 0 0; font-size: 26px; letter-spacing: -.04em; color: #263653; }
.panel-kicker { font-size: 13px; color: #247cff; text-transform: uppercase; letter-spacing: .06em; font-weight: 800; }
.workspace { display: grid; grid-template-columns: minmax(320px, .82fr) minmax(620px, 1.45fr); gap: 16px; height: calc(100vh - 138px); min-height: 0; }
.panel-card { background: linear-gradient(145deg, rgba(255,255,255,.58), rgba(225,236,255,.42)); border: 1px solid rgba(255,255,255,.72); border-radius: 22px; padding: 16px; box-shadow: 0 20px 54px rgba(63,100,160,.16), inset 0 1px 0 rgba(255,255,255,.78); backdrop-filter: blur(24px); min-height: 0; overflow: auto; }
.panel-title { font-size: 13px; font-weight: 900; color: #247cff; text-transform: uppercase; letter-spacing: .06em; margin-bottom: 16px; }
.empty { color: #74849e; font-size: 14px; padding: 34px 0; text-align: center; }
.agent-row { display: flex; align-items: center; gap: 10px; padding: 12px; border-radius: 16px; background: rgba(255,255,255,.36); margin-bottom: 10px; font-size: 13px; cursor: pointer; box-shadow: inset 0 1px 0 rgba(255,255,255,.72); }
.agent-item { margin-bottom: 10px; }
.agent-actions { padding: 12px; background: rgba(255,255,255,.34); border: 1px solid rgba(255,255,255,.62); border-radius: 18px; margin-bottom: 8px; }
.action-row { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; font-size: 12px; }
.action-label { color: #74849e; min-width: 70px; }
.dot { width: 9px; height: 9px; border-radius: 50%; background: #9aa9be; flex-shrink: 0; }
.dot.online { background: #36d08a; box-shadow: 0 0 0 5px rgba(54,208,138,.12); }
.agent-info { flex: 1; min-width: 0; }
.agent-ip { font-weight: 800; color: #253855; margin-right: 8px; }
.agent-host { color: #74849e; font-size: 12px; }
.cred-list { margin-top: 12px; }
.cred-row { display: flex; align-items: center; gap: 8px; padding: 12px; border-radius: 16px; background: rgba(255,255,255,.36); margin-bottom: 8px; font-size: 13px; }
.cred-name { font-weight: 800; min-width: 80px; color: #253855; }
.cred-user { color: #74849e; flex: 1; font-size: 12px; }
.cred-remark { color: #74849e; font-size: 12px; }
.rdp-guide { width: 100%; padding: 12px; border-radius: 16px; background: rgba(255,255,255,.36); border: 1px solid rgba(255,255,255,.62); }
.rdp-guide-title { color: #536783; font-size: 12px; line-height: 1.5; margin-bottom: 10px; }
.rdp-guide-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.rdp-check { margin-top: 8px; font-size: 12px; color: #247cff; font-weight: 800; }
.code-block { background: rgba(34,48,74,.9); border: 1px solid rgba(255,255,255,.3); border-radius: 18px; padding: 14px; font-size: 12px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; white-space: pre-wrap; margin-top: 12px; color: #f8fbff; max-height: 160px; overflow-y: auto; }
:deep(.el-tabs__item) { color: #667895; font-weight: 800; }
:deep(.el-tabs__item.is-active) { color: #247cff; }
:deep(.el-tabs__active-bar) { background: #247cff; }
:deep(.el-tabs__nav-wrap::after) { background: rgba(88,120,168,.16); }
:deep(.el-form-item__label) { color: #60728e; font-weight: 700; }
:deep(.el-dialog) { border-radius: 24px; background: linear-gradient(145deg, #f2f8ff, #dcecff); }
</style>



