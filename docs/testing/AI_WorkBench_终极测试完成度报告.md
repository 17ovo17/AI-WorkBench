# AI WorkBench 终极测试完成度报告

本报告用于每轮测试结束后定调每个基准是否已经测试、是否通过、是否存在遗留问题。后续所有测试默认以 `AI_WorkBench_终极用户闭环测试基准.md` 为最高优先级基准。

| 基准 | 状态 | 结论 | 证据目录 | 遗留问题 |
| --- | --- | --- | --- | --- |
| S0 测试安全预检 | 已完成/通过 | 安全白名单和非破坏性策略已写入基准；本轮未执行真实破坏动作 | `docs/testing/AI_WorkBench_终极用户闭环测试基准.md` | 无 P0/P1；后续真实卸载前仍需确认卡片证据 |
| S1 危险命令沙箱 | 已完成/部分通过 | 白盒/用户旅程未执行真实危险命令；L4/L3 策略作为门禁保留 | `docs/testing/AI_WorkBench_MCP浏览器真实交互基准.md` | 需在后续专批补齐多形态危险命令截图证据 |
| 运行态基线 | 已完成/通过 | 后端重启后 `mysql=true`、`redis=true`；AI/数据源健康可读 | `.test-evidence/aiw-whitebox-20260425-180738` | 无 P0/P1 |
| 可点击元素全量 | 已完成/通过 | Playwright Chrome 10/10 通过，覆盖 7 页面与关键控件可读性 | `web/test-results` | 深层每按钮 3 用例需持续扩展 |
| MCP 浏览器真实交互 | 已完成/通过 | MCP 实测拓扑与 Catpaw，确认无乱码、拓扑无 Agent 节点、探针对话可见 | `.playwright-mcp/`、`aiw-ultimate-catapaw-chat.png` | 网络/控制台 JSON 需后续批次归档 |
| 用户旅程闭环 | 已完成/通过 | 用户旅程 14/14，得分 100 | `.test-evidence/aiw-ultimate-20260425-181127/user-journey` | 无 P0/P1 |
| Windows Catpaw 闭环 | 已完成/有条件通过 | OS 矩阵记录 Windows 远程生命周期必须经确认；页面显示 `192.168.1.7` 在线 | `.test-evidence/aiw-catpaw-os-20260425-180739` | 未在本轮重新执行真实卸载/重装，按安全门禁保留确认项 |
| Linux Catpaw 闭环 | 已完成/通过 | WSL Catpaw OS 矩阵 7/7，得分 100；包含 selftest/read-only probes | `.test-evidence/aiw-catpaw-os-20260425-180739` | 部分插件依赖环境的深测后续按 BLOCKED/PASS 细分 |
| AI 调度闭环 | 已完成/部分通过 | AI Provider 健康为 alive；诊断/API 旅程通过 | `.test-evidence/aiw-ultimate-20260425-181127/user-journey` | 仍需后续补 Chat 中真实触发 Windows/Linux Catpaw 的逐条工具回传截图 |
| 业务拓扑闭环 | 已完成/通过 | 拓扑按业务主机/入口/应用/中间件/数据库/观测分层；无 Main Agent/Catpaw 节点 | MCP 页面快照、`.test-evidence/aiw-ultimate-20260425-181127/user-journey` | 无 P0/P1 |
| Prometheus/Categraf 闭环 | 已完成/通过 | 6 台业务主机 IP 标签查询均有数据计数 | `.test-evidence/aiw-ultimate-20260425-181127/user-journey` | 无 P0/P1 |
| API 安全闭环 | 已完成/基线通过 | 白盒 API 25/25 通过 | `.test-evidence/aiw-whitebox-20260425-180738` | OWASP 深度越权/速率仍需专批扩展 |
| 持久化与重启 | 已完成/通过 | 后端重启后存储健康；用户旅程创建/清理业务和会话通过 | `.test-evidence/aiw-ultimate-20260425-181127/user-journey` | 无 P0/P1 |
| 灾备与降级 | 待执行 | 未在本轮停止 MySQL/Redis/Prometheus/AI | 待补 | 需专批执行，当前不能标记通过 |
| 大数据与长稳 | 待执行 | 未执行 30 分钟长稳 | 待补 | 需专批执行，当前不能标记通过 |
| 最终清理与验收 | 已完成/通过 | 用户旅程清理自身 chat/diagnose/topology 记录；未执行远程破坏操作 | `.test-evidence/aiw-ultimate-20260425-181127/user-journey/teardown-result.json` | 告警 webhook 会触发异步诊断，后续需增加按 batch 清理告警/alert 诊断 |

## 定调规则

- `已完成/通过`：基准用例已执行，证据齐全，无 P0/P1。
- `已完成/有问题`：基准用例已执行，但存在 P2/P3 或环境阻塞。
- `失败`：存在 P0/P1、误删、越权、明文泄露、报告乱码、核心闭环不可用。
- `待执行`：尚未形成足够证据，不能标记通过。

## 后续默认基准

所有“继续测试、继续修复、继续回归、重新全量测试”请求，默认执行：终极用户闭环测试基准 + MCP 浏览器真实交互 + HTML 全量方案 + 白盒逻辑矩阵 + 安全沙箱规范 + Catpaw Windows/Linux 闭环矩阵。

## 2026-04-25 22:35:25 业务拓扑 AI 巡检与 Redis 中间件回归

- **结论**：已修复主机节点误入应用层、Redis 已登记但未进入巡检指标、业务巡检未优先体现 AI 分析的问题。
- **分层基准**：业务主机固定在主机列，Nginx 在入口层，JVM 在应用层，Redis/Sentinel/MQ 在中间件层，Oracle/MySQL/Postgres 在数据库层，Prometheus/Exporter 在观测层。
- **AI 巡检基准**：业务巡检调用外部 AI Provider；AI 只基于 Prometheus、Catpaw、告警、拓扑、业务属性等工具证据分析，不允许编造主机/指标/端口；AI 不可用时必须展示 i_error 和主 Agent 兜底说明。
- **Redis 基准**：登记 Redis 后必须出现在中间件层、业务进程、业务资源和业务巡检指标中；已验证 edis_connected_clients、edis_used_memory、edis_instantaneous_ops_per_sec、edis_keyspace_hits、edis_keyspace_misses 被纳入巡检。
- **执行证据**：go test ./... 通过；
pm run build 通过；web/tests/whitebox-ui.spec.js 11/11 通过；scripts/testing/run_user_journey_regression.py 19/19 通过。
- **自评分**：96/100。扣分项：AI Provider 响应有时接近 45 秒，已把 UI 测试超时纳入基准，但后续仍建议优化异步巡检体验。

## 2026-04-25 23:05:57 业务主机索引列与拓扑节点边界回归

- **结论**：已按用户视角调整业务拓扑：业务主机不再作为拓扑画布节点展示，改为画布左侧 Business Hosts 索引列；拓扑业务组件从入口层/应用层/中间件层/数据库层开始。
- **交互基准**：点击主机索引项会定位该主机上的第一个业务组件；无业务组件时给出明确提示。
- **拓扑基准**：.topo-node 中不允许出现 业务主机 节点；主机仅作为业务范围和筛选/定位入口，不再参与可见拓扑连线。
- **分层基准**：Nginx=入口层，JVM/App=应用层，Redis/Sentinel/MQ=中间件层，Oracle/MySQL/Postgres=数据库层，Prometheus/Exporter=观测层。
- **执行证据**：
pm run build 通过；web/tests/whitebox-ui.spec.js 11/11 通过；scripts/testing/run_user_journey_regression.py 19/19 通过；证据目录 .test-evidence/aiw-host-index-clean-20260425-230330/。
- **自评分**：98/100。扣分项：当前主机索引文案使用英文以规避 Windows PowerShell 写入导致的中文编码风险；后续如需中文化，必须只用 UTF-8 写入并纳入乱码回归。

## 业务主机索引列与拓扑节点边界（最终基准追加）

- 业务主机只允许作为拓扑画布旁边的“业务主机清单/索引列”出现，不能作为 `.topo-node` 业务拓扑节点出现。
- 点击业务主机清单中的 IP，必须定位并选中拓扑中同 IP 的第一个业务组件节点；若没有组件，必须给出明确提示。
- 拓扑画布节点从业务组件层开始展示：入口层、应用层、中间件层、数据库层、观测层；主机层不进入拓扑节点布局。
- 顶部统计必须使用可见业务组件数量和可见连线数量，不能把隐藏主机、Main Agent、Catpaw Sub Agent 计入“节点”。
- Main Agent/Catpaw Sub Agent 只能作为在线状态或详情元信息出现，不能作为业务节点参与连线。
- 回归脚本必须覆盖：主机清单可见、点击主机后 `.topo-node.selected` 唯一选中、`.topo-node` 中无“业务主机”、统计文案为“组件”。

当前基准状态：已纳入 `web/tests/whitebox-ui.spec.js` 的 MCP/Playwright 真实交互回归；后续所有业务拓扑测试以本条为最高优先级验收口径。
## 2026-04-25 23:56 Catpaw Linux、卸载语义与 AI 业务巡检回归

- 修复 198.18.20.11 Linux 探针卸载误显示 Windows `C:\catpaw` 的问题：前端根据探针 OS/hostname/version 推断 Linux/Windows，Linux 只展示 `/usr/local/bin/catpaw`、`/etc/catpaw`、`/var/log/catpaw*` 白名单路径。
- 卸载二次确认已中文化，确认内容展示目标、风险、白名单范围、后端二次校验说明。
- 探针管理新增“删除主机”动作：离线或废弃主机可仅删除平台记录，不强制远程卸载，避免主机不在线时占位无法清理。
- 本机 WSL Linux 已真实构建并安装 Catpaw 到 `/usr/local/bin/catpaw`，执行 `catpaw selftest cpu -q` 通过 2/2，并以 `198.18.20.11 whitebox-linux-wsl` heartbeat 注册到平台。
- 业务巡检新增 `ai_suggestions`，建议不再只是告警列表；结合业务链路完整性、资源指标、进程/端口、Redis/Oracle/JVM/Nginx、告警影响和观测一致性给出下一步诊断建议。
- 智能对话新增业务巡检调度：用户说“帮我巡检一下 clims 业务”时，平台主 Agent 直接调用业务巡检工具链，并返回业务评分、数据源、AI 建议和关键发现；不会再只透传到外部模型。
- 回归：`go test ./...` 通过；`npm run build` 通过；WSL 内 `run_user_journey_regression.py --base-url http://127.0.0.1:8080` 通过 19/19，评分 100。
- 环境说明：Windows 侧 Playwright 本轮因 WSL 后端端口映射/可达性超时未完成，不标记通过；功能链路已用 WSL 本地 API 和用户旅程脚本验证。
## 2026-04-26 00:13 创建并发现无反馈与配置可见性修复

- 问题根因：AI 配置和数据源配置没有被删除，WSL `api/config.yaml` 仍保留用户配置，MySQL/接口也能读取；当时 Windows 前端代理指向 `localhost:8080`，但新后端实际在 WSL 内监听，导致前端 API 500/连接失败，看起来像配置消失。
- 修复：`web/vite.config.js` 新增 `VITE_API_PROXY`，前端可明确代理到 WSL 后端 IP，避免 Windows localhost/WSL localhost 混淆。
- 修复：`Topology.vue` 的“创建并发现”和“重新发现”增加 `catch`，接口失败时明确 toast 展示错误，不再表现为点击没反应。
- 验证：通过 MCP 浏览器真实点击“新增业务端口 → 创建并发现”，创建 `mcp-create-test-20260426` 成功，业务列表增加、画布显示 `组件 1` 和 `198.18.20.11:8081 jvm`。
- 回归：新增 Playwright 用例 `topology create and discover gives visible result`，单测通过。
- 配置确认：`/api/v1/ai-providers` 返回 1 个 provider（key 脱敏），`/api/v1/data-sources` 返回 1 个 Prometheus 数据源。