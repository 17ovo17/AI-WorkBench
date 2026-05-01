# AI WorkBench 商业化运维闭环测试基准

> 本文档是后续所有测试、修复、回归和验收的最高优先级基准。任何结论必须标记 PASS / FAIL / BLOCKED / NOT_TESTED，没有证据不得标记 PASS。

## 角色闭环验收

| 角色 | 必测闭环 | 核心断言 |
| --- | --- | --- |
| 运维人员 | 业务巡检 -> AI 建议 -> 发起诊断 -> 报告下载/删除 | 必须综合拓扑、Prometheus、Catpaw、告警、资源和进程 |
| 值班人员 | 告警聚合 -> 认领 -> 转派 -> 静默 -> 恢复/归档/删除 | 重复告警必须降噪，动作有反馈和审计 |
| DBA | Redis/Oracle/MySQL/Postgres 指标 -> 专项诊断 -> 业务影响 | 数据库不混应用层，中间件不混数据库层 |
| 应用负责人 | 创建业务 -> AI 分类 -> 分层拓扑 -> SLO/仿真 | 主机仅作旁侧索引，图内从入口层开始 |
| 安全审计员 | 危险动作确认/取消、脱敏、越权尝试 | L4 永拒，L3 必须二次确认，密钥和凭据不外泄 |

## 告警事件中心

- 按 `alertname + target_ip + severity + business_id + source` 生成指纹并聚合。
- 重复 firing 只增加 `count/first_seen/last_seen`，不得重复创建诊断或通知。
- 状态流：`firing -> acknowledged -> assigned -> diagnosing -> mitigated -> resolved -> archived/deleted`。
- 未知 action 必须失败，删除必须软删除并写审计。
- 详情只展示脱敏标签摘要，禁止直出 token/key/secret/password。

## 业务可观测

- 用户输入业务范围是唯一发现边界。
- 业务主机作旁侧索引，不作为图内节点。
- 拓扑层级：入口层、应用层、中间件层、数据库层、观测层。
- Nginx 属入口层，JVM 属应用层，Redis/Sentinel 属中间件层，Oracle/MySQL/Postgres 属数据库层。
- 业务巡检由 AI 主导，结果写入智能诊断记录。

## DBA / Catpaw / API / 持久化

- DBA 专项：Redis PING/端口/连接数/内存/OPS，Oracle listener/表空间/会话，MySQL/Postgres 慢 SQL/锁等待。
- Catpaw Windows/Linux 对等覆盖：安装、运行、自检、巡检、聊天、诊断、卸载、删除离线主机、重装。
- API 覆盖所有 `/api/v1/*`，包含越权、XSS、SQL、SSRF、路径遍历、并发。
- MySQL 持久化告警、动作日志、通知记录、业务巡检、诊断、Agent、审计、拓扑、AI 配置和数据源。
- Redis 只能作为在线状态、heartbeat、缓存和 TTL，不得作为唯一持久化来源。
- 同步 WSL 时禁止覆盖 `/opt/ai-workbench/api/config.yaml`。

## MCP 与证据

- 每个可见按钮至少覆盖：正常、异常、边界、取消确认、重复点击、刷新恢复。
- 证据目录：`.test-evidence/<batch>/`，必须包含 summary、self-score、coverage、defects、pre/post snapshot 和 teardown。
- 任何“展示但不可点、点了无反馈、乱码、原始 JSON 噪声、敏感信息直出”均按缺陷处理。

## 商业可用评分

- P0/P1 遗留必须为 0。
- 角色闭环评分 >= 90，MCP 主链路 >= 95，安全基线 S0 = 100。


## ???????2026-04-26?

- PASS????????????????? action ???????????????
- PASS?CORS ???????????????????????
- PASS???????? `AIW_ADMIN_TOKEN` ? `security.admin_token`?????????????? `X-Security-Mode` ?????
- PASS?????????????? `/api/v1/oncall/test-send` ???????
- BLOCKED??? Webhook/Email/Flashduty/PagerDuty ????????????????

## 2026-04-26 增补基准：业务级统一诊断与记录治理
- **业务统一诊断**：业务拓扑发起巡检时，必须以单个业务为诊断对象，不得把同一业务下的多台主机拆成多条割裂诊断。
- **巡检记录入口**：业务列表左侧必须展示当前业务的巡检记录，支持刷新、打开详情、按当前业务清理。
- **诊断记录治理**：智能诊断中心必须提供单条删除、清理业务巡检、清理测试记录、来源筛选、关键字搜索。
- **按业务清理安全线**：`DELETE /api/v1/diagnose?scope=business_inspection&business_id=<id>` 只允许删除当前业务巡检记录，不得影响其他业务或手动诊断记录。
- **展示降噪**：业务巡检结果默认展示摘要、Top 风险、AI 建议和折叠原始证据，禁止在主视图直接堆叠原始 JSON。
- **验收状态**：代码层已实现；Go 单测、前端构建、MCP 点击清理确认作为最终发布前证据。


## Topo-Architect 拓扑 Agent 验收项（2026-04-26 03:25:29）
- [ ] POST /api/v1/topology/ai/generate 输出 nodes/links/risks/summary 标准 JSON。
- [ ] clims 拓扑中 nginx=入口层、JVM=应用层、Redis=缓存层、Oracle=数据层。
- [ ] 业务主机只显示在旁侧索引，不进入拓扑 nodes。
- [ ] Main Agent/Catpaw Agent 不作为业务节点。
- [ ] 巡检报告包含 Topo-Architect 结构摘要、拓扑结构风险和关键路径。


## AIOps-Diagnostician 智能问诊验收项（2026-04-26 03:47:00）
- [x] /workbench 重构为三栏 AIOps 问诊工作台。
- [x] 新增 /api/v1/aiops 七模块接口：sessions/messages/actions/inspections/data/topology/ws。
- [x] PromQL 模板库内置，cpu_high 等诊断链可回放。
- [x] suggestedActions 固定只读：PromQL 查询、命令复制、拓扑跳转。
- [x] 响应包含 reasoningChain、dataSources、suggestedActions、topology.highlight。

## AIOps v2 全链路联动验收基准（2026-04-26 04:20:00）

| 模块 | 验收项 | 准出标准 | 优先级 | 状态 |
|---|---|---|---|---|
| WS 会话协议 | /api/v1/aiops/ws/sessions/:id connected/ping/chat/complete | 连接后返回 connected，ping 返回 pong，chat 依次输出推理步骤、诊断和完成事件 | P0 | PASS |
| 推理链流式 | PromQL 模板执行前后推送 unning/completed/failed | 每个模板步骤包含 step/action/status/latency_ms，HTTP fallback 语义一致 | P0 | PASS |
| 只读动作 | execute_action 仅允许 promql、command(copyOnly)、link、topology | restart/delete/write/remote_exec 返回 403，不执行远程写命令 | P0 | PASS |
| 指标联动 | subscribe_metrics 推送 metric_update | PromQL 查询可解析 metric/value，取消订阅后停止当前连接推送 | P1 | PASS |
| 拓扑联动 | 多 IP、业务名、Redis/DB/cache 问题触发 	opology_update | 返回业务 topology JSON 与 highlight，不混入 Agent 节点 | P1 | PASS |
| Workbench v2 | 三栏工作台 + 可折叠拓扑面板 | WS 可用时流式展示；WS 不可用时 HTTP fallback 可完成最终问诊 | P1 | PASS |
| 文档归档 | Agent/WS/topology 参考资产归档 | docs/aiops/ 保存运行时规范、协议和拓扑参考说明 | P2 | PASS |

## AIOps v2 角色化问诊和值班交接验收（2026-04-26 角色化增强）

| 模块 | 验收项 | 准出标准 | 优先级 | 状态 |
|---|---|---|---|---|
| 普通用户视图 | 简明结论卡 | 显示问题、影响、级别、下一步、是否升级，不暴露原始 JSON 噪声 | P0 | PASS |
| 运维视图 | 完整证据链 | 展示 PromQL、Catpaw、reasoningChain、只读验证动作 | P0 | PASS |
| 值班视图 | 交接摘要 | 可复制现象、影响、证据、待确认项、建议动作和升级条件 | P0 | PASS |
| 管理视图 | 非技术影响说明 | 聚焦业务影响、健康与风险趋势，不要求理解 PromQL | P1 | PASS |
| 后端响应 | audience/summaryCard/handoffNote | HTTP 与 WS diagnosis 消息均包含角色化字段 | P0 | PASS |
| 动作安全 | 摘要/交接复制 | 复制动作走 command(copyOnly)，平台不执行远程命令 | P0 | PASS |
