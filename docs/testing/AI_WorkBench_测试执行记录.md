# AI WorkBench 测试执行记录

## 本轮信息

- 执行日期：YYYY-MM-DD HH:mm:ss
- 执行人：
- 代码版本/分支：
- 环境：Windows + WSL `Ubuntu-24.04` + 本机 MySQL/Redis/Prometheus
- 前端地址：`http://localhost:3000`
- 后端地址：`http://localhost:8080`
- Prometheus 地址：`http://localhost:9090`
- 证据目录：`.test-evidence/<timestamp>`

## 环境基线

| 检查项 | 结果 | 证据 | 备注 |
| --- | --- | --- | --- |
| 前端可访问 | 未测 |  |  |
| 后端可访问 | 未测 |  |  |
| MySQL 健康 | 未测 |  |  |
| Redis 健康 | 未测 |  |  |
| Prometheus 健康 | 未测 |  |  |
| 6 台业务主机指标 | 未测 |  |  |
| Windows 探针连通 | 未测 |  |  |
| Linux/WSL 巡检 | 未测 |  |  |

## 功能测试记录

按 AGENTS.md 要求，每个功能点使用以下格式记录。

### 测试用例模板

- **测试模块**：
- **测试点**：
- **测试步骤**：1.  2.  3.
- **输入数据**：
- **预期结果**：
- **实际结果**：
- **潜在风险**：
- **优先级**：P0/P1/P2/P3
- **状态**：通过/失败/阻塞/待补齐
- **证据**：
- **修复记录**：
- **回归结果**：

## 缺陷清单

| ID | 优先级 | 模块 | 标题 | 复现步骤 | 修复文件 | 回归结果 |
| --- | --- | --- | --- | --- | --- | --- |
|  |  |  |  |  |  |  |

## 缺失功能补齐清单

| 功能点 | 所属模块 | 最小闭环要求 | 实现状态 | 回归状态 |
| --- | --- | --- | --- | --- |
|  |  |  |  |  |

## 结论

- P0：0
- P1：0
- P2：0
- P3：0
- 是否满足本轮准出：否
- 遗留风险：

## 2026-04-26 03:20:00 业务诊断记录治理回归记录
- **测试模块**：业务拓扑 / 智能诊断中心
- **测试点**：业务级统一巡检记录、按业务清理、诊断中心来源筛选、测试记录清理
- **测试步骤**：1. 在业务拓扑选择业务并巡检；2. 检查业务左侧巡检记录；3. 打开智能诊断中心按业务巡检筛选；4. 执行清理业务巡检/测试记录二次确认；5. 刷新验证删除范围。
- **输入数据**：`source=business_inspection`、`business_id=<当前业务ID>`、`scope=business_inspection`、`scope=test`
- **预期结果**：业务巡检写入一条业务级统一诊断；当前业务只清理自身巡检记录；诊断中心不再被历史测试数据淹没。
- **潜在风险**：误传空 `business_id` 会清理全部业务巡检，因此 UI 当前业务清理必须始终带 `business_id`。
- **优先级**：P1
- **当前定调**：已完成代码修复，待执行最终构建、API 与 MCP 真实点击证据。


## Topo-Architect 重构执行记录（2026-04-26 03:25:29）
| 项目 | 状态 | 说明 |
|---|---|---|
| Agent 文档 | PASS | 新增 Topo-Architect 与 SysDiag-Inspector 联动规范。 |
| 后端 DTO | PASS | 新增 AI topology 标准 JSON 数据结构。 |
| 生成接口 | PASS | 新增 /api/v1/topology/ai/generate，AI 不可用时 heuristic_fallback。 |
| D3 画布 | PASS | 新增 BusinessTopologyCanvas.vue，支持布局切换、风险视图、详情面板、健康状态。 |
| 前端真实交互 | NOT_TESTED | 待通过浏览器 MCP 验证 /topology。 |
| 后端测试 | NOT_TESTED | 待执行 go test ./...。 |
| 前端构建 | PASS | 已执行 web npm run build 通过；仅有 Vite chunk size 警告。 |



## AIOps-Diagnostician 重构执行记录（2026-04-26 03:47:00）
| 项目 | 状态 | 说明 |
|---|---|---|
| 后端模型 | PASS | 新增 AIOpsSession/AIOpsMessage/ReasoningStep/DataSourceUsage/SuggestedAction/TopologyHighlight/AIOpsInspection。 |
| 后端接口 | PASS | 新增 /api/v1/aiops 七模块路由，保留 /chat 兼容。 |
| PromQL 模板库 | PASS | 已内置用户提供的 promql_library.json，白盒验证 cpu_high 顺序。 |
| 操作安全 | PASS | 白盒验证 restart 类写操作 403，command 仅 copyOnly。 |
| 前端 Workbench | PASS | 已替换为 Vue 三栏 AIOps 工作台。 |
| 后端测试 | PASS | api go test ./... 已通过。 |
| 前端构建 | PASS | web npm run build 已通过，仅 Vite chunk size 警告。 |
| MCP 真实交互 | NOT_TESTED | 本轮未启动浏览器服务验证。 |

## AIOps v2 全链路联动执行记录（2026-04-26 04:20:00）

| 项目 | 状态 | 说明 |
|---|---|---|
| 后端 WS v2 | PASS | 已支持 connected/pong/reasoning_step/diagnosis/topology_update/metric_update/alert/error/complete/interrupted。 |
| WS 客户端消息 | PASS | 已支持 chat/ping/interrupt/execute_action/subscribe_metrics/unsubscribe_metrics/feedback。 |
| HTTP/WS 编排一致 | PASS | WS chat 与 HTTP messages 共用 unAIOpsDiagnosis。 |
| 操作安全 | PASS | 白盒覆盖 topology 只读动作与 restart/delete/write/remote_exec 拒绝。 |
| 指标解析 | PASS | 白盒覆盖 PromQL metric 名称提取与数值提取。 |
| 拓扑触发 | PASS | 白盒覆盖多 IP、Redis 影响面、clims 拓扑触发。 |
| Workbench 构建 | PASS | 
pm run build 通过；仅 Vite chunk size 警告。 |
| 后端回归 | PASS | go test ./... 通过。 |
| MCP 真实交互 | NOT_TESTED | 本轮未启动本地 Web 服务做浏览器点击验证。 |
| 乱码扫描 | PASS | 新增/修改的 AIOps v2 文件未发现 乱码替换符 字符。 |

## AIOps v2 角色化增强执行记录（2026-04-26 角色化增强）

| 项目 | 状态 | 说明 |
|---|---|---|
| 后端消息字段 | PASS | AIOpsMessage 新增 audience、summaryCard、handoffNote。 |
| 受众识别 | PASS | 支持 user/ops/oncall/manager 显式传入与关键词推断。 |
| 交接摘要 | PASS | 每次诊断生成可复制值班 handoffNote 和 copy-handoff 动作。 |
| 前端视图 | PASS | Workbench 增加简明/运维/值班/管理视图切换。 |
| 细节降噪 | PASS | 非运维视图隐藏推理链和数据源噪声，保留结论卡。 |
| 回归状态 | NOT_TESTED | 待执行 go test、npm build、乱码扫描。 |
