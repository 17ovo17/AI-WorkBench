# AI WorkBench × Dify 集成设计方案

> 版本：v1.0 | 日期：2026-04-29

## 一、背景与目标

当前 AI WorkBench 的诊断能力完全依赖 LLM 通用知识 + Prometheus 实时指标，缺少历史案例积累和反馈闭环。通过嵌入 Dify 的 RAG + 工作流能力，实现：

1. 诊断知识库 — 积累历史案例，新诊断时自动检索相似案例辅助判断
2. 诊断工作流 — 标准化诊断流程（检索案例 → 查指标 → LLM 分析 → 置信度标注）
3. 反馈闭环 — 运维评价 → 标注 → 归档为案例 → 同步知识库，准确率持续提升
4. 指标适配 — 统一不同 exporter 的指标名，LLM 接收标准化上下文

## 二、技术选型确认

| 项目 | 选择 | 理由 |
|------|------|------|
| 知识库/工作流引擎 | Dify（源码嵌入） | 工作流最成熟，内置标注功能，支持 MySQL，Apache 2.0 |
| 数据库 | 复用现有 MySQL 8.0 | Dify 已原生支持 MySQL（DB_TYPE=mysql） |
| 向量存储 | pgvector 或 Elasticsearch | 按资源选择，pgvector 最轻量 |
| Dify 部署方式 | sidecar 进程（Python API :5001） | 与 Go 后端并行运行，内部 HTTP 通信 |

## 三、Dify 源码关键信息

源码位置：`D:\平台源码\dify-main\dify-main\`

### 3.1 MySQL 支持

Dify 已原生支持 MySQL，配置 `DB_TYPE=mysql` 即可：
- 驱动：`mysql+pymysql`
- 迁移文件：`api/migrations/versions/2025_11_15_2102-09cfdda155d1_mysql_adaptation.py`
- docker-compose 中已有 `db_mysql` 服务（通过 profiles: mysql 激活）

### 3.2 API 认证

Bearer Token 认证，两种 Key：
- App API Key — 用于工作流/对话 API
- Dataset API Key — 用于知识库 API

### 3.3 核心 API 端点

**知识库：**
- `POST /datasets` — 创建知识库
- `POST /datasets/{id}/document/create-by-text` — 上传文本文档
- `POST /datasets/{id}/retrieve` — 检索
- `GET /datasets/{id}/documents` — 列出文档

**工作流：**
- `POST /workflows/run` — 执行工作流（支持 blocking/streaming）
- `GET /workflows/run/{id}` — 查询运行详情
- `POST /workflows/tasks/{id}/stop` — 停止任务

**标注/反馈：**
- `POST /messages/{id}/feedbacks` — 提交反馈（like/dislike）
- `POST /apps/annotations` — 创建标注（question + answer）

**工作流 DSL 格式：** YAML

### 3.4 依赖组件

| 组件 | 用途 | 是否必须 |
|------|------|---------|
| Python 3.11+ | Dify API 运行时 | 是 |
| Redis | 缓存/Celery broker | 是（可复用现有） |
| MySQL 8.0 | 数据存储 | 是（复用现有） |
| 向量数据库 | 知识库向量检索 | 是（pgvector/ES 二选一） |
| Celery worker | 异步任务（文档索引等） | 是 |
| sandbox | 代码执行沙箱 | 可选 |

## 四、架构设计

```
用户浏览器 (:3000)
    │
    └── Vue 3 前端
            │
            └── Go API (:8080)
                    │
                    ├── 现有业务逻辑（对话/诊断/告警/拓扑/探针）
                    │
                    ├── internal/dify/proxy.go ──→ Dify API (:5001)
                    │       │                         │
                    │       ├── 知识库检索              ├── Redis（复用）
                    │       ├── 工作流执行              ├── MySQL（复用）
                    │       ├── 反馈标注               └── pgvector
                    │       └── 案例同步
                    │
                    ├── internal/metrics/normalizer/ ──→ metrics_mapping.yaml
                    │
                    ├── internal/knowledge/ ──→ diagnosis_cases 表
                    │
                    └── Prometheus (:9090)
```

## 五、新增数据库表

### 5.1 diagnosis_cases（诊断案例）

```sql
CREATE TABLE diagnosis_cases (
    id VARCHAR(64) PRIMARY KEY,
    metric_snapshot JSON NOT NULL COMMENT '指标异常快照',
    root_cause_category VARCHAR(64) NOT NULL COMMENT '根因分类',
    root_cause_description TEXT NOT NULL COMMENT '根因描述',
    treatment_steps TEXT NOT NULL COMMENT '处置方案',
    keywords VARCHAR(500) COMMENT '关键词标签，逗号分隔',
    source_diagnosis_id VARCHAR(64) COMMENT '来源诊断记录ID',
    dify_document_id VARCHAR(64) COMMENT 'Dify知识库文档ID',
    created_at DATETIME NOT NULL,
    created_by VARCHAR(128),
    evaluation_avg DECIMAL(3,1) DEFAULT 0 COMMENT '平均评价分',
    INDEX idx_case_category (root_cause_category),
    FULLTEXT INDEX idx_case_keywords (keywords, root_cause_description)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 5.2 diagnosis_feedback（诊断反馈）

```sql
CREATE TABLE diagnosis_feedback (
    id VARCHAR(64) PRIMARY KEY,
    diagnosis_id VARCHAR(64) NOT NULL COMMENT '关联诊断记录ID',
    user VARCHAR(128) NOT NULL,
    rating ENUM('accurate', 'partial', 'inaccurate') NOT NULL,
    comment TEXT,
    dify_message_id VARCHAR(64) COMMENT 'Dify消息ID，用于标注回写',
    created_at DATETIME NOT NULL,
    INDEX idx_feedback_diagnosis (diagnosis_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 5.3 ai_settings（AI 配置）

```sql
CREATE TABLE ai_settings (
    id VARCHAR(64) PRIMARY KEY,
    setting_key VARCHAR(128) UNIQUE NOT NULL,
    setting_value TEXT,
    updated_at DATETIME NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 5.4 runbooks（运维手册/SOP）

```sql
CREATE TABLE runbooks (
    id VARCHAR(64) PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    category VARCHAR(64) NOT NULL COMMENT '分类：cpu_high/memory_leak/disk_full等',
    trigger_conditions JSON COMMENT '触发条件（指标阈值组合）',
    steps TEXT NOT NULL COMMENT '处置步骤（Markdown）',
    auto_executable TINYINT(1) DEFAULT 0 COMMENT '是否可自动执行',
    dify_document_id VARCHAR(64),
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    INDEX idx_runbook_category (category)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

## 六、新增后端模块

### 6.1 internal/dify/proxy.go — Dify API 代理

封装对 Dify 内部 API 的调用：
- `RunWorkflow(inputs)` — 执行诊断工作流
- `SearchKnowledge(query)` — 检索知识库
- `SyncCase(case)` — 同步案例到知识库
- `SubmitFeedback(messageID, rating)` — 提交反馈标注
- `HealthCheck()` — Dify 可用性检查

### 6.2 internal/metrics/normalizer/ — 指标适配

- `normalizer.go` — 从 metrics_mapping.yaml 读取映射规则
- `discovery.go` — 启动时扫描 Prometheus 自动发现未适配指标
- `metrics_mapping.yaml` — 映射规则配置

### 6.3 internal/knowledge/ — 知识库管理

- `cases.go` — 案例 CRUD
- `runbooks.go` — Runbook CRUD
- `sync.go` — 案例/Runbook 同步到 Dify 知识库

### 6.4 新增 API 路由

```
POST   /api/v1/diagnosis/start          → 调用 Dify 工作流诊断
POST   /api/v1/diagnosis/feedback        → 提交评价
POST   /api/v1/diagnosis/archive         → 归档为案例
GET    /api/v1/knowledge/cases           → 分页获取案例
POST   /api/v1/knowledge/cases           → 创建案例
PUT    /api/v1/knowledge/cases/:id       → 更新案例
DELETE /api/v1/knowledge/cases/:id       → 删除案例
POST   /api/v1/knowledge/cases/sync      → 同步到 Dify 知识库
POST   /api/v1/knowledge/cases/import    → 批量导入
GET    /api/v1/knowledge/cases/export    → 批量导出
GET    /api/v1/knowledge/runbooks        → Runbook 列表
GET    /api/v1/settings/ai               → 获取 AI 配置
PUT    /api/v1/settings/ai               → 更新 AI 配置
GET    /api/v1/metrics/mapping           → 获取指标映射规则
GET    /api/v1/metrics/unmapped          → 获取未适配指标
```

## 七、前端新增页面

### 7.1 AI 基础设置页

配置项：Dify API 地址、API Key、Embedding 模型、推理模型、向量库参数。

### 7.2 诊断交互页

- 输入主机名/业务名发起诊断
- 流式卡片展示工作流步骤（检索案例 → 查指标 → 分析中 → 完成）
- 结构化诊断报告（根因、证据、处置建议、置信度）
- 反馈按钮（准确/部分准确/不准确）+ 归档按钮

### 7.3 知识库管理页

- 案例列表（分页、搜索、按根因分类筛选）
- 案例详情（指标快照、根因、处置步骤、评价历史）
- "同步到 Dify"按钮
- 批量导入/导出
- Runbook 管理 tab

## 八、诊断工作流 DSL

```yaml
# 工作流节点：
# 1. start — 接收输入（hostname, time_range, user_question）
# 2. knowledge_retrieval — 从知识库检索 Top3 相似案例
# 3. http_request_metrics — 调用 Go API 获取 Prometheus 实时指标
# 4. http_request_inspection — 调用 Go API 获取最近巡检报告
# 5. llm_diagnosis — LLM 综合分析，输出结构化诊断报告
# 6. condition — 判断置信度，LOW/MEDIUM 时追加提示
# 7. end — 返回最终结果
```

## 九、自定义工具定义（OpenAPI 3.0）

### get_prometheus_metrics

```yaml
openapi: 3.0.0
info:
  title: AI WorkBench Prometheus API
  version: 1.0.0
paths:
  /api/v1/prometheus/query:
    post:
      operationId: get_prometheus_metrics
      summary: 查询 Prometheus 实时指标
      parameters:
        - name: ip
          in: query
          required: true
          schema:
            type: string
        - name: time_range
          in: query
          schema:
            type: string
            default: "1h"
```

### get_inspection_report

```yaml
paths:
  /api/v1/catpaw/report:
    get:
      operationId: get_inspection_report
      summary: 获取主机最近巡检报告
      parameters:
        - name: ip
          in: query
          required: true
          schema:
            type: string
```

## 十、指标智能适配方案

### 10.1 整体流程

```
添加数据源 → 扫描 Prometheus 全量指标名 → AI 自动适配（批量匹配标准名）
→ 用户在前端审核/修改映射 → 保存到 metrics_mappings 表 → 同步到 Dify 知识库
```

### 10.2 自动扫描

添加 Prometheus 数据源时，后端自动调用 `/api/v1/label/__name__/values` 获取全量指标名列表，存入 `metrics_raw` 表。

### 10.3 AI 自动适配

将扫描到的指标名列表（可能几百到几千条）分批发给 LLM，让 AI 识别每个指标属于什么类别并映射到标准名：

```
Prompt: 以下是 Prometheus 中扫描到的指标名列表，请为每个指标识别：
1. 来源 exporter（node_exporter / categraf / mysqld_exporter / redis_exporter / 未知）
2. 标准名（格式：{domain}.{resource}.{metric}，如 host.cpu.usage）
3. 描述（中文，一句话说明这个指标是什么）
4. 建议的 PromQL 转换公式

指标列表：
- cpu_usage_active
- node_cpu_seconds_total
- mysql_global_status_threads_connected
- redis_connected_clients
...

请以 JSON 数组格式返回。
```

### 10.4 用户审核与编辑

前端提供指标映射管理页面：
- 表格展示所有指标（原始名 / AI 适配的标准名 / 来源 / 描述 / 状态）
- 状态：`auto`（AI 自动适配）/ `confirmed`（用户确认）/ `custom`（用户自定义）/ `unmapped`（未适配）
- 用户可以修改标准名、描述、PromQL 转换公式
- 批量确认：一键确认所有 AI 适配结果
- 重新扫描：数据源变更后重新扫描，增量适配新指标

### 10.5 数据库表

```sql
CREATE TABLE metrics_mappings (
    id VARCHAR(64) PRIMARY KEY,
    datasource_id VARCHAR(64) NOT NULL COMMENT '关联数据源',
    raw_name VARCHAR(255) NOT NULL COMMENT '原始指标名',
    standard_name VARCHAR(255) COMMENT '标准名（domain.resource.metric）',
    exporter VARCHAR(64) COMMENT '来源 exporter',
    description VARCHAR(500) COMMENT '中文描述',
    transform VARCHAR(500) COMMENT 'PromQL 转换公式',
    status ENUM('auto', 'confirmed', 'custom', 'unmapped') DEFAULT 'unmapped',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    UNIQUE KEY uniq_ds_raw (datasource_id, raw_name),
    INDEX idx_mapping_standard (standard_name),
    INDEX idx_mapping_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 10.6 新增 API

```
POST   /api/v1/metrics/scan              → 扫描数据源全量指标名
POST   /api/v1/metrics/auto-adapt        → AI 自动适配（批量）
GET    /api/v1/metrics/mappings          → 获取映射列表（分页/筛选）
PUT    /api/v1/metrics/mappings/:id      → 用户编辑单条映射
POST   /api/v1/metrics/mappings/confirm  → 批量确认 AI 适配结果
POST   /api/v1/metrics/mappings/sync     → 同步到 Dify 知识库
```

### 10.7 知识库同步

适配完成的指标映射同步到 Dify 知识库，格式：

```
指标名：cpu_usage_active
标准名：host.cpu.usage
来源：categraf
描述：主机 CPU 使用率（百分比）
PromQL：cpu_usage_active{ident="{ip}"}
正常范围：0-70% 健康，70-90% 警告，>90% 危险
```

诊断时 LLM 可以检索到这些指标知识，理解每个指标的含义和正常范围。

### 10.8 静态映射兜底

`metrics_mapping.yaml` 保留作为兜底配置，当数据库中没有映射记录时使用：

```yaml
mappings:
  - source: node_cpu_seconds_total
    standard: host.cpu.usage
    transform: "rate({}[5m]) * 100"
    exporters: [node_exporter]

  - source: cpu_usage_active
    standard: host.cpu.usage
    transform: "{}"
    exporters: [categraf]

  - source: mysql_global_status_threads_connected
    standard: mysql.threads.connected
    transform: "{}"
    exporters: [mysqld_exporter]

  - source: redis_connected_clients
    standard: redis.clients.connected
    transform: "{}"
    exporters: [redis_exporter]
```
    standard: redis.clients.connected
    transform: "{}"
    exporters: [redis_exporter]
```

## 十一、运维角度补充优化点

1. **指标动态发现** — 启动时扫描 Prometheus label values，自动匹配已知模式，未匹配的标记"未适配"
2. **时间维度对比** — 工具支持 range query，LLM 可做"和昨天/上周比"的趋势分析
3. **Runbook 联动** — 诊断时同时检索案例和 Runbook，匹配到 SOP 时直接推送标准步骤
4. **多主机关联诊断** — 支持传入业务名，自动拉取所有主机指标做关联分析
5. **告警驱动自动诊断** — webhook 入口，告警触发后自动调用 Dify 工作流
6. **诊断报告版本对比** — 同一主机多次诊断的 diff 展示
7. **降级策略** — Dify 不可用时降级到现有的纯 LLM 诊断，不影响基础功能

## 十二、实施计划

| 阶段 | 内容 | 预估 |
|------|------|------|
| P1 | DDL + 指标适配层 + Dify 代理层骨架 | 1 天 |
| P2 | 知识库 CRUD + 案例同步 + 前端知识库页 | 1 天 |
| P3 | 诊断工作流 DSL + 工具注册 + 前端诊断页 | 1-2 天 |
| P4 | 反馈闭环 + 归档 + 前端反馈组件 | 0.5 天 |
| P5 | AI 设置页 + Runbook + 降级策略 | 0.5 天 |
| P6 | 联调测试 + 文档 | 1 天 |
