# AI WorkBench 全量测试执行基准

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


## 固定执行批次

| 批次 | 名称 | 必测内容 | 证据 |
| --- | --- | --- | --- |
| S0 | 安全预检 | 白名单、危险命令守卫、配置脱敏 | safety-impact.md |
| 1 | MCP 可点击元素 | 按钮、菜单、Tab、表单、弹窗、下载、删除 | clickable-controls.json |
| 2 | 角色用户旅程 | 运维、值班、DBA、应用负责人、安全审计 | user-journey-summary.md |
| 3 | Windows Catpaw | 安装、巡检、聊天、诊断、卸载、删除、重装 | catpaw-windows.json |
| 4 | Linux Catpaw | WSL 安装、巡检、聊天、诊断、卸载、删除、重装 | catpaw-linux.json |
| 8 | API 安全 | OWASP API Top 10、越权、注入、SSRF、并发 | api-security-regression.json |
| 9 | 持久化与重启 | MySQL/Redis、后端重启、刷新恢复、清理校验 | persistence-regression.json |
