# AI WorkBench + Catpaw 终极用户闭环测试基准

本基准是 AI WorkBench 后续测试、修复、回归的最高优先级标准。测试目标不是“页面能打开”或“接口能返回”，而是验证真实用户能完成完整闭环：配置 → 发现 → 诊断 → 巡检 → 报告 → 告警 → 拓扑 → 清理 → 审计 → 恢复。

## 运行与安全基准

- 前端：`http://localhost:3000`；后端：`http://localhost:8080`；Prometheus：`http://localhost:9090`。
- WSL Linux 项目路径：`/opt/ai-workbench`；Windows 探针目标：`192.168.1.7`。
- 同步 WSL 代码时必须排除 `config.yaml`，避免覆盖 AI Key、数据源和敏感配置。
- 测试数据必须携带 `test_batch_id`，证据保存后只清理本批次数据。
- Windows 真实操作只允许 `192.168.1.7` 的 `C:\catpaw`、`Catpaw` 计划任务、Catpaw 进程、测试配置。
- Linux 真实操作只允许 WSL Catpaw 白名单路径：`/usr/local/bin/catpaw`、`/etc/catpaw`、`/var/log/catpaw*`、测试目录。
- L4 危险命令永不真实执行；L3 必须二次确认；确认状态必须回传聊天、审计和证据。

## 业务与监控基准

- 业务主机固定为 6 台：`198.18.20.11`、`198.18.20.12`、`198.18.20.20`、`198.18.22.11`、`198.18.22.12`、`198.18.22.13`。
- `198.18.20.11`、`198.18.20.12` 是 JVM 应用服务器，业务端口 `8081`。
- `198.18.20.20` 是 Nginx + Redis；`198.18.22.11-13` 是 Oracle。
- Prometheus 必须按 IP 兼容 `instance`、`ident`、`ip`、`host`、`hostname`、`from_hostip`、`target` 标签。
- target 离线不等于无数据；历史或未来时序存在该 IP 数据时，必须用于诊断和拓扑健康。

## 用户角色闭环

| 角色 | 用户目标 | 必测闭环 | 验收重点 |
| --- | --- | --- | --- |
| 新手管理员 | 完成首次配置 | 健康状态 → AI 配置 → 数据源配置 → 保存 → 刷新验证 | 无空白、无问号、无乱码；未配置有明确引导 |
| SRE/运维 | 判断主机压力 | Chat 问 IP → Prometheus 指标 → 诊断 → 报告下载 → 删除 | 有数据时不能回答无数据；显示数据源和时间窗口 |
| 应用负责人 | 建业务拓扑 | 新增业务 → 输入 IP/端口 → AI 发现 → 分层拓扑 → 节点诊断 | 只发现用户范围；链路符合 Nginx → JVM → Redis/Oracle |
| DBA | 排查数据库/中间件 | 选择 Oracle/Redis 节点 → 指标诊断 → Catpaw 工具结果 | 数据库与中间件分层，语义不混淆 |
| 安全审计员 | 验证安全边界 | 危险命令 → 确认/拒绝 → 审计 → 聊天回传 | L4 永拒；敏感信息脱敏；确认不可复用绕过 |
| 值班人员 | 处置告警 | 告警进入 → 筛选 → 诊断 → 关联拓扑 → 恢复/归档/删除 | 状态持久化；删除有确认；测试数据可清理 |
| 测试人员 | 完成批次验证 | 执行批次 → 保存证据 → 自评分 → teardown | 无白名单外副作用；pre/post snapshot 可比对 |

## 页面闭环要求

- MCP/Playwright 浏览器真实交互是 UI 验收主线，不允许只用 API 或静态脚本替代。
- 每个页面必须保存 MCP 页面快照、截图、控制台错误、网络请求摘要；每个关键按钮必须真实点击至少一次。
- MCP 必须覆盖正常、异常、边界、取消确认、确认执行、刷新持久化、浏览器前进/后退。
- 若 API 通过但 MCP 用户操作失败，按用户体验缺陷判定，不允许标记通过。
- Workbench：新建会话、历史会话、发送消息、模型切换、提示词、多轮上下文、Prometheus 查询、Catpaw 调度、危险命令确认。
- Diagnose：IP 输入、诊断发起、Prometheus 优先、Catpaw fallback、报告详情、下载、删除、刷新恢复。
- Alerts：筛选、详情、AI 诊断、标记恢复、归档、删除、Webhook/Catpaw 事件进入。
- Topology：新增业务、编辑业务、添加 IP/端口/服务、AI 发现、树形分层、节点详情、连线健康、删除业务/节点/连线。
- Catpaw：凭证新增/编辑/删除、Linux/Windows 安装、模式切换、插件配置、巡检、聊天、卸载、重装、状态刷新。
- Settings AI：Provider 新增/删除/设默认、模型新增/删除、保存、健康刷新、Key 脱敏。
- Settings Datasource：Prometheus/MySQL/Redis 配置加载、保存、健康检查、错误原因、恢复验证。

## Windows/Linux Catpaw 对等闭环

- Windows：安装、重复安装、run、inspect、selftest、diagnose、chat、事件上报、卸载、残留校验、重装。
- Linux：WSL 安装、run、inspect、selftest、diagnose list/show、chat、事件上报、卸载、残留校验、重装。
- 两类系统都必须验证 AI 调度：Workbench、Diagnose、Agent Chat、告警触发诊断均能捕捉真实工具反馈。
- 报告必须无乱码、原始数据可折叠、敏感信息脱敏、可下载、可删除。

## 拓扑验收标准

- 画布只展示业务主机和业务端口；Main Agent/Catpaw Sub Agent 不作为拓扑节点出现。
- Agent 在线状态只作为节点详情或发现摘要中的元数据展示。
- 层级固定为：业务主机 → 入口层 → 应用层 → 中间件层 → 数据库层 → 观测层。
- Redis/Sentinel/MQ 属于中间件层；Oracle/MySQL/Postgres 属于数据库层；`9091` 属于观测层。
- 协议按服务语义展示：HTTP health、JVM app probe、Redis PING、Oracle listener probe、MySQL probe、Postgres probe、Prometheus scrape、TCP connect fallback。
- 只发现用户录入业务范围内的 IP/端口，不混入无关 target。

## 证据与验收

- 每批目录：`.test-evidence/<batch>/`。
- 每批必须包含：`batch-summary.md`、`self-score.json`、`safety-impact.md`、`coverage-matrix.csv`、`user-journey-summary.md`、`clickable-controls.json`、`defects.md`、`fixed-regression.md`、`created-ids.json`、`pre-snapshot.json`、`post-snapshot.json`、`teardown-result.json`。
- MCP 批次还必须包含：`mcp-browser-summary.md`、`mcp-page-snapshots/`、`mcp-screenshots/`、`mcp-console.json`、`mcp-network.json`、`mcp-click-coverage.json`。
- 自评分低于 90 必须补测；低于 80 必须重做；误删、越权、明文泄露直接 P0 和 0 分。
- 最终无 P0/P1 遗留；P2/P3 必须有复现证据、风险说明和修复建议。

## 业务主机索引列与拓扑节点边界（最终基准追加）

- 业务主机只允许作为拓扑画布旁边的“业务主机清单/索引列”出现，不能作为 `.topo-node` 业务拓扑节点出现。
- 点击业务主机清单中的 IP，必须定位并选中拓扑中同 IP 的第一个业务组件节点；若没有组件，必须给出明确提示。
- 拓扑画布节点从业务组件层开始展示：入口层、应用层、中间件层、数据库层、观测层；主机层不进入拓扑节点布局。
- 顶部统计必须使用可见业务组件数量和可见连线数量，不能把隐藏主机、Main Agent、Catpaw Sub Agent 计入“节点”。
- Main Agent/Catpaw Sub Agent 只能作为在线状态或详情元信息出现，不能作为业务节点参与连线。
- 回归脚本必须覆盖：主机清单可见、点击主机后 `.topo-node.selected` 唯一选中、`.topo-node` 中无“业务主机”、统计文案为“组件”。

当前基准状态：已纳入 `web/tests/whitebox-ui.spec.js` 的 MCP/Playwright 真实交互回归；后续所有业务拓扑测试以本条为最高优先级验收口径。