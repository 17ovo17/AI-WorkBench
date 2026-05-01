<template>
  <div class="plugin-config">
    <div class="section-title">基础插件（默认启用，无需配置）</div>
    <div class="plugin-grid">
      <el-checkbox v-for="p in basicPlugins" :key="p.name" v-model="p.enabled" :label="p.name">
        {{ p.name }} <span class="plugin-desc">{{ p.desc }}</span>
      </el-checkbox>
    </div>

    <div class="section-title" style="margin-top:20px">扩展插件（需填写目标地址）</div>
    <div v-for="p in advPlugins" :key="p.name" class="adv-plugin">
      <div class="adv-head">
        <el-checkbox v-model="p.enabled">{{ p.name }}</el-checkbox>
        <span class="plugin-desc">{{ p.desc }}</span>
      </div>
      <div v-if="p.enabled" class="adv-body">
        <div v-for="(target, i) in p.targets" :key="i" class="target-row">
          <el-input v-model="p.targets[i]" :placeholder="p.placeholder" size="small" style="flex:1" />
          <el-button size="small" @click="p.targets.splice(i,1)">-</el-button>
        </div>
        <el-button size="small" @click="p.targets.push('')">+ 添加目标</el-button>
      </div>
    </div>

    <div style="margin-top:16px; display:flex; gap:10px; align-items:center; flex-wrap:wrap">
      <el-select v-model="targetAgent" placeholder="选择目标机器" size="small" style="width:180px">
        <el-option v-for="a in agents" :key="a.ip" :label="`${a.ip} ${a.hostname}`" :value="a.ip" />
      </el-select>
      <el-select v-model="authMode" size="small" style="width:120px">
        <el-option value="cred" label="使用凭证" />
        <el-option value="manual" label="手动输入" />
      </el-select>
      <template v-if="authMode==='cred'">
        <el-select v-model="selectedCred" placeholder="选择凭证" size="small" style="width:140px">
          <el-option v-for="c in credentials" :key="c.id" :label="c.name" :value="c.id" />
        </el-select>
      </template>
      <template v-else>
        <el-input v-model="manualUser" placeholder="用户名" size="small" style="width:90px" />
        <el-input v-model="manualPass" placeholder="密码" type="password" size="small" style="width:100px" />
      </template>
      <el-button type="primary" size="small" :loading="applying" @click="applyRemote">远程应用配置</el-button>
      <el-button size="small" @click="$emit('generate', buildConfig())">生成配置</el-button>
    </div>
    <pre v-if="applyOutput" style="margin-top:10px;background:#0d1117;border:1px solid #30363d;border-radius:6px;padding:10px;font-size:12px;color:#e6edf3;white-space:pre-wrap">{{ applyOutput }}</pre>
  </div>
</template>

<script setup>
import { reactive, ref, onMounted } from 'vue'
import axios from 'axios'
import { ElMessageBox } from 'element-plus'

const emit = defineEmits(['generate'])

const agents = ref([])
const credentials = ref([])
const targetAgent = ref('')
const authMode = ref('cred')
const selectedCred = ref('')
const manualUser = ref('root')
const manualPass = ref('')
const applying = ref(false)
const applyOutput = ref('')

onMounted(async () => {
  const [a, c] = await Promise.all([
    axios.get('/api/v1/catpaw/agents'),
    axios.get('/api/v1/credentials')
  ])
  agents.value = (a.data || []).filter(x => x.online)
  credentials.value = c.data || []
})

const applyRemote = async () => {
  if (!targetAgent.value) return
  applying.value = true
  applyOutput.value = ''
  const config = buildConfig()
  let cred = { ip: targetAgent.value, protocol: 'ssh', port: 22 }
  if (authMode.value === 'cred' && selectedCred.value) {
    const c = credentials.value.find(x => x.id === selectedCred.value)
    if (c) Object.assign(cred, { username: c.username, port: c.port || 22, protocol: c.protocol })
  } else {
    cred.username = manualUser.value
    cred.password = manualPass.value
  }
  const cmd = `mkdir -p /etc/catpaw/conf.d && cat > /etc/catpaw/conf.d/plugins.toml << 'PLUGINEOF'
${config}
PLUGINEOF
pkill -HUP catpaw 2>/dev/null || true && echo "plugins config applied"`
  try {
    await ElMessageBox.confirm(`Apply Catpaw plugin config to ${targetAgent.value}? This writes /etc/catpaw/conf.d/plugins.toml and reloads Catpaw.`, 'Confirm remote config apply', {
      type: 'warning', confirmButtonText: 'Confirm', cancelButtonText: 'Cancel'
    })
    const { data } = await axios.post('/api/v1/remote/exec', { ...cred, command: cmd, safety_confirm: 'ALLOW-L3' })
    applyOutput.value = data.output || 'No output'
  } catch (e) {
    applyOutput.value = e.response?.data?.error || e.message
  } finally {
    applying.value = false
  }
}

const basicPlugins = reactive([
  { name: 'cpu',        desc: 'CPU 使用率和负载',         enabled: true },
  { name: 'mem',        desc: '内存使用率',               enabled: true },
  { name: 'disk',       desc: '磁盘使用率和 inode',       enabled: true },
  { name: 'diskio',     desc: '磁盘 IO 读写速率',         enabled: true },
  { name: 'net',        desc: '网络流量和丢包',           enabled: true },
  { name: 'netif',      desc: '网卡详细统计',             enabled: false },
  { name: 'uptime',     desc: '系统运行时长',             enabled: true },
  { name: 'conntrack',  desc: 'TCP 连接跟踪表使用率',     enabled: true },
  { name: 'tcpstate',   desc: 'TCP 连接状态分布',         enabled: false },
  { name: 'sockstat',   desc: 'Socket 统计',              enabled: false },
  { name: 'sysctl',     desc: '内核参数监控',             enabled: false },
  { name: 'zombie',     desc: '僵尸进程检测',             enabled: true },
  { name: 'procnum',    desc: '进程数量监控',             enabled: false },
  { name: 'procfd',     desc: '进程文件描述符',           enabled: false },
  { name: 'filefd',     desc: '系统文件描述符',           enabled: false },
  { name: 'mount',      desc: '挂载点检测',               enabled: false },
  { name: 'systemd',    desc: 'Systemd 服务状态',         enabled: false },
  { name: 'ntp',        desc: 'NTP 时钟同步',             enabled: false },
  { name: 'sysdiag',    desc: '70+ 系统诊断工具（AI用）', enabled: true },
  { name: 'docker',     desc: 'Docker 容器监控',          enabled: false },
  { name: 'secmod',     desc: 'SELinux/AppArmor 状态',    enabled: false },
  { name: 'neigh',      desc: 'ARP 邻居表',               enabled: false },
])

const advPlugins = reactive([
  { name: 'redis',         desc: 'Redis 实例监控',           enabled: false, placeholder: '127.0.0.1:6379', targets: [] },
  { name: 'redis_sentinel',desc: 'Redis Sentinel 监控',      enabled: false, placeholder: '127.0.0.1:26379', targets: [] },
  { name: 'http',          desc: 'HTTP 接口可用性探测',      enabled: false, placeholder: 'http://example.com/health', targets: [] },
  { name: 'ping',          desc: 'ICMP Ping 延迟检测',       enabled: false, placeholder: '8.8.8.8', targets: [] },
  { name: 'dns',           desc: 'DNS 解析检测',             enabled: false, placeholder: 'example.com', targets: [] },
  { name: 'cert',          desc: 'TLS 证书到期检测',         enabled: false, placeholder: 'example.com:443', targets: [] },
  { name: 'etcd',          desc: 'etcd 集群监控',            enabled: false, placeholder: 'http://127.0.0.1:2379', targets: [] },
  { name: 'exec',          desc: '自定义脚本采集',           enabled: false, placeholder: '/path/to/script.sh', targets: [] },
  { name: 'logfile',       desc: '日志文件关键字告警',       enabled: false, placeholder: '/var/log/app.log', targets: [] },
  { name: 'filecheck',     desc: '文件存在性检测',           enabled: false, placeholder: '/etc/app/config.yaml', targets: [] },
])

const buildConfig = () => {
  const lines = []
  for (const p of basicPlugins) {
    if (!p.enabled) continue
    lines.push(`# ${p.name}: ${p.desc}`)
    lines.push(`[[${p.name}.instances]]`)
    lines.push('')
  }
  for (const p of advPlugins) {
    if (!p.enabled || !p.targets.filter(Boolean).length) continue
    lines.push(`# ${p.name}: ${p.desc}`)
    for (const t of p.targets.filter(Boolean)) {
      lines.push(`[[${p.name}.instances]]`)
      lines.push(`targets = ["${t}"]`)
      lines.push('')
    }
  }
  return lines.join('\n')
}
</script>

<style scoped>
.plugin-config { padding: 4px 0; }
.section-title { font-size: 12px; color: #8b949e; text-transform: uppercase; letter-spacing: 1px; margin-bottom: 10px; }
.plugin-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
.plugin-desc { font-size: 11px; color: #8b949e; margin-left: 4px; }
.adv-plugin { margin-bottom: 10px; }
.adv-head { display: flex; align-items: center; gap: 8px; }
.adv-body { margin: 8px 0 0 24px; }
.target-row { display: flex; gap: 6px; margin-bottom: 6px; align-items: center; }
</style>
