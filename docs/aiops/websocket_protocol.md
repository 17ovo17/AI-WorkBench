# AIOps WebSocket Protocol v2

Endpoint: `/api/v1/aiops/ws/sessions/:id`

## Client Messages
- `ping`: 心跳，服务端返回 `pong`。
- `chat`: `{ type, content, attachments? }`，触发与 HTTP message 相同的诊断编排器。
- `interrupt`: 中断当前会话流，服务端返回 `interrupted`。
- `execute_action`: `{ type, actionType, params }`，只允许 promql、command(copyOnly)、link、topology。
- `subscribe_metrics`: `{ type, metrics: [promql], interval }`，服务端轮询 Prometheus 并推送 `metric_update`。
- `unsubscribe_metrics`: 停止当前连接的指标推送。
- `feedback`: `{ type, messageId, rating, comment }`，服务端返回 `feedback_ack`。

## Server Events
- `connected`: 连接建立，包含 sessionId 与 serverTime。
- `pong`: 心跳响应。
- `reasoning_step`: `{ status: running|completed|failed, step, action, query?, output?, latency_ms? }`。
- `diagnosis`: 最终助手消息，包含 content、reasoningChain、dataSources、suggestedActions、topology。
- `topology_update`: 完整 Topo-Architect JSON 与 highlight。
- `metric_update`: `{ query, metric, value?, raw?, timestamp }`。
- `alert`: 活跃告警或诊断过程告警。
- `action_result`: execute_action 结果。
- `error`: 协议、查询或编排错误。
- `complete`: 当前 chat 结束。
- `interrupted`: 当前 chat 被用户中断。

## Invariants
- WS `chat` 和 HTTP fallback 使用同一个后端编排器。
- PromQL 模板执行前必须推送 running，完成后推送 completed/failed。
- 所有建议操作保持只读；写操作必须拒绝并可审计。
- topology_update 只展示业务层节点，Agent/采集器仅能作为旁侧索引或数据来源出现。
