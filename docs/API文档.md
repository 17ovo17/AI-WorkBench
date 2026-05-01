# AI WorkBench API 文档

> 本文档列出 Dify 集成相关的 30+ 个新增 API 端点。原有 API（认证、对话、AIOps、告警、拓扑、探针等）见各模块代码注释。

## 通用约定

- Base URL: `http://localhost:8080`
- 所有路径前缀：`/api/v1`
- 写操作（POST/PUT/DELETE）需 `X-Admin-Token` 头或 `Authorization: Bearer <token>`
- 响应：成功返回数据 JSON，失败返回 `{"error": "中文错误描述"}`
- 分页：`?page=1&limit=20`，列表响应含 `total` 字段

## 状态码

| 码 | 含义 |
|----|------|
| 200 | 成功 |
| 400 | 参数错误 |
| 401 | 未认证 |
| 403 | 无权限 |
| 404 | 资源不存在 |
| 500 | 内部错误 |
| 503 | 依赖服务（如 Dify）不可用 |

---

## 一、知识库案例（9 个）

### GET /api/v1/knowledge/cases
分页查询案例。

**Query**：`page` (默认 1) / `limit` (默认 20，最大 200) / `keyword` (FULLTEXT 搜索) / `category`

**响应**：
```json
{
  "items": [
    {
      "id": "seed-001",
      "metric_snapshot": {"cpu_usage_active": 95.3},
      "root_cause_category": "cpu_high",
      "root_cause_description": "...",
      "treatment_steps": "...",
      "keywords": "cpu,top,perf",
      "dify_document_id": "",
      "created_at": "2026-04-29T00:00:00Z",
      "evaluation_avg": 4.5
    }
  ],
  "total": 18,
  "page": 1,
  "limit": 20
}
```

### GET /api/v1/knowledge/cases/:id
单条案例详情。404 表示不存在。

### POST /api/v1/knowledge/cases
创建案例。**Body**：DiagnosisCase JSON（root_cause_category 和 root_cause_description 必填）。

### PUT /api/v1/knowledge/cases/:id
更新案例。

### DELETE /api/v1/knowledge/cases/:id
删除案例。响应：`{"ok": true}`。

### POST /api/v1/knowledge/cases/import
批量导入。**Body**：DiagnosisCase 数组。响应：`{"imported": 18, "total": 18}`。

### GET /api/v1/knowledge/cases/export
导出全量。**Query**：`?category=cpu_high` 可选筛选。响应：JSON 数组（带 Content-Disposition 头）。

### POST /api/v1/knowledge/cases/:id/sync
同步到 Dify。Dify 不可用时返回 503 + `{"error": "Dify service unavailable", "fallback": true}`。

---

## 二、Runbook（6 个）

### GET /api/v1/knowledge/runbooks
分页查询 Runbook。**Query**：`page` / `limit` / `category`。

### GET /api/v1/knowledge/runbooks/:id
单条详情。

### POST /api/v1/knowledge/runbooks
创建 Runbook。**Body**：
```json
{
  "title": "CPU 用户态飙高",
  "category": "cpu_high",
  "trigger_conditions": "{\"metric\":\"cpu_usage_user\",\"operator\":\">\",\"threshold\":80}",
  "steps": "## 1. 确认现状\n...",
  "auto_executable": false
}
```

### PUT /api/v1/knowledge/runbooks/:id
更新 Runbook。

### DELETE /api/v1/knowledge/runbooks/:id
删除 Runbook。

### POST /api/v1/knowledge/runbooks/:id/sync
同步 Runbook 到 Dify 知识库。

---

## 三、诊断工作流（6 个）

### POST /api/v1/diagnosis/start
发起诊断。**Body**：
```json
{
  "hostname": "198.18.20.20",
  "time_range": "1h",
  "user_question": "为什么 CPU 这么高？",
  "response_mode": "streaming"
}
```

- `response_mode = "blocking"`：返回完整 JSON
- `response_mode = "streaming"`：返回 SSE 流（Content-Type: text/event-stream）
- Dify 不可用时返回 503 + `{"fallback": true}`

### POST /api/v1/diagnosis/feedback
提交反馈。**Body**：
```json
{
  "diagnosis_id": "diag-001",
  "rating": "accurate",
  "comment": "很准确",
  "dify_message_id": "msg-xxx"
}
```

`rating ∈ {accurate, partial, inaccurate}`。accurate→Dify like，inaccurate→Dify dislike，partial 跳过。

**响应**：`{"ok": true, "id": "...", "sync_status": "ok|failed|skipped|dify_unavailable"}`

### GET /api/v1/diagnosis/feedback
查询某诊断的反馈。**Query**：`diagnosis_id` 必填。

### GET /api/v1/diagnosis/feedback/all
分页查询所有反馈。**Query**：`page` / `limit`。

### POST /api/v1/diagnosis/archive
归档诊断为案例。**Body**：
```json
{
  "diagnosis_id": "diag-001",
  "root_cause_category": "cpu_high",
  "root_cause_description": "...",
  "treatment_steps": "...",
  "keywords": "cpu,top",
  "sync_to_dify": true
}
```

**响应**：`{"ok": true, "case": {...}, "sync_status": "ok|failed|skipped|dify_unavailable", "sync_error": "可选"}`

### GET /api/v1/diagnose/compare
诊断对比。**Query**：`ip` 必填，`limit` (默认 2)。

**响应**（≥2 条诊断）：
```json
{
  "ip": "198.18.20.20",
  "current": {...},
  "previous": {...},
  "diff": {
    "time_gap": "2h30m",
    "current_at": "2026-04-29T10:00:00Z",
    "previous_at": "2026-04-29T07:30:00Z",
    "alert_changed": true
  },
  "comparable": true
}
```

---

## 四、指标映射（5 个）

### POST /api/v1/metrics/scan
扫描 Prometheus 指标。**Body**：`{"datasource_id": "1776951891799"}`

调用 Prometheus `/api/v1/label/__name__/values`，新增的指标 status=unmapped。**响应**：`{"added": 546}`。

### POST /api/v1/metrics/auto-adapt
AI 自动适配。**Body**：`{"datasource_id": "...", "max_batches": 5}`

每批 30 条发给 LLM，返回标准名 / exporter / 描述 / PromQL，写入数据库（status=auto）。**响应**：`{"processed": 150, "adapted": 142}`。

### GET /api/v1/metrics/mappings
查询映射列表。**Query**：`datasource_id` (必填) / `status` (auto/confirmed/custom/unmapped) / `page` / `limit`。

### PUT /api/v1/metrics/mappings/:id
单条编辑映射。**Body**：MetricsMapping JSON。

### POST /api/v1/metrics/mappings/confirm
批量确认 auto 状态的映射。**Body**：`{"datasource_id": "..."}`。**响应**：`{"confirmed": 120}`。

---

## 五、AI 设置（5 个）

### GET /api/v1/settings/ai
返回所有设置（敏感字段已掩码）。

**响应**：
```json
{
  "dify.base_url": "http://localhost:5001/v1",
  "dify.app_api_key": "app-se...",
  "dify.dataset_id": "abc-123-..."
}
```

### PUT /api/v1/settings/ai
批量更新。**Body**：`map[string]string`。掩码值（含 "..."）会被自动跳过避免回写。**响应**：`{"ok": true, "updated": 3}`。

### GET /api/v1/settings/ai/:key
单条查询。**响应**：`{"key": "...", "value": "..."}`（敏感字段已掩码）。

### PUT /api/v1/settings/ai/:key
单条更新。**Body**：`{"value": "..."}`。

### DELETE /api/v1/settings/ai/:key
删除单条设置。

---

## 敏感字段掩码规则

key 包含以下任一关键字时（不区分大小写），value 长度 > 6 时显示前 6 位 + "..."：
- `api_key`
- `password`
- `secret`
- `token`

写入时如检测到掩码值（结尾含 "..."），自动跳过避免破坏现有配置。

---

## 审计追踪

所有写操作（创建/更新/删除/同步/扫描）都会记录到 `audit_events` 表，可通过 GET /api/v1/audit/events 查询。

---

## 降级行为

| 场景 | 行为 |
|------|------|
| Dify 不可用 | 503 + `{"fallback": true}` |
| Prometheus 不可用 | scanner 返回错误，不写入指标 |
| MySQL 不可用 | 自动降级到内存存储 |
| Redis 不可用 | 探针在线状态降级 |

---

## 完整路由表

```
GET    /api/v1/knowledge/cases
GET    /api/v1/knowledge/cases/export
POST   /api/v1/knowledge/cases
POST   /api/v1/knowledge/cases/import
GET    /api/v1/knowledge/cases/:id
PUT    /api/v1/knowledge/cases/:id
DELETE /api/v1/knowledge/cases/:id
POST   /api/v1/knowledge/cases/:id/sync

GET    /api/v1/knowledge/runbooks
GET    /api/v1/knowledge/runbooks/:id
POST   /api/v1/knowledge/runbooks
PUT    /api/v1/knowledge/runbooks/:id
DELETE /api/v1/knowledge/runbooks/:id
POST   /api/v1/knowledge/runbooks/:id/sync

POST   /api/v1/diagnosis/start
POST   /api/v1/diagnosis/feedback
GET    /api/v1/diagnosis/feedback
GET    /api/v1/diagnosis/feedback/all
POST   /api/v1/diagnosis/archive
GET    /api/v1/diagnose/compare

POST   /api/v1/metrics/scan
POST   /api/v1/metrics/auto-adapt
GET    /api/v1/metrics/mappings
PUT    /api/v1/metrics/mappings/:id
POST   /api/v1/metrics/mappings/confirm

GET    /api/v1/settings/ai
PUT    /api/v1/settings/ai
GET    /api/v1/settings/ai/:key
PUT    /api/v1/settings/ai/:key
DELETE /api/v1/settings/ai/:key
```

总计 31 个新增端点。

---

## 六、工作流管理（8 个）

### GET /api/v1/workflows
分页查询工作流列表。

**Query**：`page` (默认 1) / `limit` (默认 20) / `category` (可选筛选)

**响应**：
```json
{
  "items": [
    {
      "id": "wf-001",
      "name": "diagnosis",
      "display_name": "主诊断工作流",
      "category": "diagnosis",
      "description": "知识检索→指标→巡检→LLM→置信度路由",
      "builtin": true,
      "enabled": true,
      "created_at": "2026-04-29T00:00:00Z"
    }
  ],
  "total": 12,
  "page": 1,
  "limit": 20
}
```

### GET /api/v1/workflows/:id
单个工作流详情（含 DSL YAML）。

**响应**：
```json
{
  "id": "wf-001",
  "name": "diagnosis",
  "dsl_yaml": "nodes:\n  - id: start\n    type: start\n...",
  "builtin": true,
  "enabled": true
}
```

### POST /api/v1/workflows
创建自定义工作流。**Body**：
```json
{
  "name": "custom_check",
  "display_name": "自定义检查",
  "category": "custom",
  "description": "...",
  "dsl_yaml": "nodes:\n  - id: start\n    type: start\n..."
}
```

### PUT /api/v1/workflows/:id
更新工作流。内置工作流仅允许修改 enabled 字段。

### DELETE /api/v1/workflows/:id
删除自定义工作流。内置工作流不可删除，返回 403。

### POST /api/v1/workflows/:id/execute
执行工作流（阻塞模式）。**Body**：
```json
{
  "inputs": {
    "hostname": "198.18.20.20",
    "time_range": "1h"
  }
}
```

**响应**：
```json
{
  "run_id": "run-001",
  "status": "completed",
  "outputs": {"report": "..."},
  "duration_ms": 3200,
  "node_results": [
    {"node_id": "start", "status": "completed", "duration_ms": 1},
    {"node_id": "knowledge_retrieval", "status": "completed", "duration_ms": 450}
  ]
}
```

### POST /api/v1/workflows/:id/stream
执行工作流（流式模式）。Body 同 execute。

**响应**：`Content-Type: text/event-stream`

```
event: workflow_started
data: {"run_id": "run-001", "workflow_id": "wf-001"}

event: node_started
data: {"node_id": "knowledge_retrieval", "node_type": "knowledge_retrieval"}

event: node_completed
data: {"node_id": "knowledge_retrieval", "outputs": {...}, "duration_ms": 450}

event: text_chunk
data: {"node_id": "llm_diagnosis", "text": "根据分析..."}

event: workflow_completed
data: {"run_id": "run-001", "outputs": {...}, "duration_ms": 3200}
```

SSE 事件类型：`workflow_started` / `node_started` / `node_completed` / `text_chunk` / `workflow_completed` / `workflow_failed`

### GET /api/v1/workflows/:id/runs
查询工作流执行历史。**Query**：`page` / `limit`

**响应**：
```json
{
  "items": [
    {
      "id": "run-001",
      "workflow_id": "wf-001",
      "status": "completed",
      "inputs": {"hostname": "198.18.20.20"},
      "duration_ms": 3200,
      "created_at": "2026-04-29T20:00:00Z"
    }
  ],
  "total": 5
}
```

---

## 七、知识库文档管理（7 个）

### POST /api/v1/knowledge/documents/upload
上传文档。**Content-Type**: `multipart/form-data`

**Form Fields**：
- `file` — 文档文件（支持 .md / .txt / .pdf / .docx / .html）
- `category` — 分类（可选）

**响应**：
```json
{
  "id": "doc-001",
  "filename": "troubleshooting-guide.md",
  "format": "markdown",
  "size_bytes": 15360,
  "chunk_count": 12,
  "embedding_status": "pending",
  "created_at": "2026-04-29T20:00:00Z"
}
```

### GET /api/v1/knowledge/documents
分页查询文档列表。**Query**：`page` / `limit` / `format` (可选筛选)

**响应**：
```json
{
  "items": [
    {
      "id": "doc-001",
      "filename": "troubleshooting-guide.md",
      "format": "markdown",
      "chunk_count": 12,
      "embedding_status": "completed",
      "created_at": "2026-04-29T20:00:00Z"
    }
  ],
  "total": 5
}
```

### GET /api/v1/knowledge/documents/:id
单个文档详情（含分块信息）。

### DELETE /api/v1/knowledge/documents/:id
删除文档（同时清理分块和索引）。

### POST /api/v1/knowledge/documents/search
混合搜索。**Body**：
```json
{
  "query": "CPU 使用率过高怎么排查",
  "top_k": 10,
  "search_mode": "hybrid",
  "rerank": true
}
```

`search_mode` 可选：`bm25`（仅关键词）/ `vector`（仅向量）/ `hybrid`（混合，默认）

**响应**：
```json
{
  "results": [
    {
      "document_id": "doc-001",
      "chunk_id": "chunk-003",
      "content": "CPU 使用率过高的常见原因包括...",
      "score": 0.85,
      "source": "troubleshooting-guide.md",
      "metadata": {"section": "CPU 诊断"}
    }
  ],
  "search_mode": "hybrid",
  "reranked": true,
  "total": 8
}
```

### POST /api/v1/knowledge/documents/:id/reindex
重建单个文档的索引。文档内容变更后调用。

**响应**：`{"ok": true, "chunk_count": 15}`

### POST /api/v1/knowledge/documents/preview-chunks
分块预览（不入库）。**Body**：
```json
{
  "content": "# 标题\n\n段落一...\n\n## 子标题\n\n段落二...",
  "chunk_size": 500,
  "overlap": 50
}
```

**响应**：
```json
{
  "chunks": [
    {"index": 0, "content": "# 标题\n\n段落一...", "char_count": 120},
    {"index": 1, "content": "## 子标题\n\n段落二...", "char_count": 95}
  ],
  "total_chunks": 2
}
```

---

## 八、Runbook 执行与历史（2 个）

### POST /api/v1/knowledge/runbooks/:id/execute
执行 Runbook。**Body**：
```json
{
  "parameters": {
    "target_host": "192.168.1.100",
    "top_n": "10"
  }
}
```

模板变量 `{{target_host}}` 和 `{{top_n}}` 会被替换为实际值。

**响应**：
```json
{
  "execution_id": "exec-001",
  "runbook_id": "rb-001",
  "status": "running",
  "started_at": "2026-04-29T20:00:00Z"
}
```

执行状态：`running` / `success` / `failed` / `rolled_back`

### GET /api/v1/knowledge/runbooks/:id/executions
查询 Runbook 执行历史。**Query**：`page` / `limit`

**响应**：
```json
{
  "items": [
    {
      "id": "exec-001",
      "runbook_id": "rb-001",
      "status": "success",
      "parameters": {"target_host": "192.168.1.100"},
      "output": "步骤 1 完成...\n步骤 2 完成...",
      "duration_ms": 5200,
      "created_at": "2026-04-29T20:00:00Z"
    }
  ],
  "total": 3
}
```

---

## 完整路由表（更新）

```
# 知识库案例（9 个）
GET    /api/v1/knowledge/cases
GET    /api/v1/knowledge/cases/export
POST   /api/v1/knowledge/cases
POST   /api/v1/knowledge/cases/import
GET    /api/v1/knowledge/cases/:id
PUT    /api/v1/knowledge/cases/:id
DELETE /api/v1/knowledge/cases/:id
POST   /api/v1/knowledge/cases/:id/sync

# Runbook（8 个，含执行和历史）
GET    /api/v1/knowledge/runbooks
GET    /api/v1/knowledge/runbooks/:id
POST   /api/v1/knowledge/runbooks
PUT    /api/v1/knowledge/runbooks/:id
DELETE /api/v1/knowledge/runbooks/:id
POST   /api/v1/knowledge/runbooks/:id/sync
POST   /api/v1/knowledge/runbooks/:id/execute
GET    /api/v1/knowledge/runbooks/:id/executions

# 诊断工作流（6 个）
POST   /api/v1/diagnosis/start
POST   /api/v1/diagnosis/feedback
GET    /api/v1/diagnosis/feedback
GET    /api/v1/diagnosis/feedback/all
POST   /api/v1/diagnosis/archive
GET    /api/v1/diagnose/compare

# 指标映射（5 个）
POST   /api/v1/metrics/scan
POST   /api/v1/metrics/auto-adapt
GET    /api/v1/metrics/mappings
PUT    /api/v1/metrics/mappings/:id
POST   /api/v1/metrics/mappings/confirm

# AI 设置（5 个）
GET    /api/v1/settings/ai
PUT    /api/v1/settings/ai
GET    /api/v1/settings/ai/:key
PUT    /api/v1/settings/ai/:key
DELETE /api/v1/settings/ai/:key

# 工作流管理（8 个）
GET    /api/v1/workflows
GET    /api/v1/workflows/:id
POST   /api/v1/workflows
PUT    /api/v1/workflows/:id
DELETE /api/v1/workflows/:id
POST   /api/v1/workflows/:id/execute
POST   /api/v1/workflows/:id/stream
GET    /api/v1/workflows/:id/runs

# 知识库文档管理（7 个）
POST   /api/v1/knowledge/documents/upload
GET    /api/v1/knowledge/documents
GET    /api/v1/knowledge/documents/:id
DELETE /api/v1/knowledge/documents/:id
POST   /api/v1/knowledge/documents/search
POST   /api/v1/knowledge/documents/:id/reindex
POST   /api/v1/knowledge/documents/preview-chunks
```

总计 48 个新增端点。
