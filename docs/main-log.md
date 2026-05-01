# AI WorkBench 工作日志

> 主 Agent 编排日志，记录所有子 Agent 调度和执行结果。

---

- 260429 2230 多 Agent 协作体系初始化完成
- 260429 2230 子 Agent 角色定义：go-backend / vue-frontend / qa-tester / ops-diagnostician / doc-writer
- 260429 2230 CLAUDE.md 已更新编排规则
- 260429 2330 Agent 人格 v2.0 全部重写，融入 skills + 防屎山约束 + AIOps 平台约束
- 260429 2330 CLAUDE.md 新增"防屎山强约束"章节（写前必做/绝对禁止/提交自检）
- 260429 2330 qa-tester 人格重写：四维度测试 + 统一测试基准引用 + 防屎山检测 + AIOps 专项
- 260429 2330 任务看板 QA 任务更新，与新人格匹配
- 260429 2330 多Agent协作体系设计文档升级到 v2.0
- 260429 2350 指标适配方案升级：静态映射 → AI 自动适配 + 用户可编辑 + 知识库同步
- 260429 2350 新增 metrics_mappings 表设计，新增 3 个后端任务 + 1 个前端任务 + 1 个诊断任务
- 260429 2350 CLAUDE.md 加入 Agent ID 收集规则（文件系统探测 + 日志记录 + SendMessage resume）
- 260429 2350 多Agent协作体系设计文档加入 Agent ID 收集详细方法

## 2026-04-29 Dify 集成 Batch 1-3 实施

### Batch 1（P0 无依赖）
- 260429 0040 任务启动：Dify 集成 Batch 1（store 拆分 + DDL + Dify 代理层 + Prompt + 种子数据）
- 260429 0048 go-backend 完成：store.go 拆分为 8 文件 + 5 张新表 DDL + 3 个 model + Dify 代理层 4 文件 + config.yaml 追加 dify 段 (AGENT_ID: afd4337485468e234)
- 260429 0048 ops-diagnostician 完成：metrics_adapt.txt + dify_workflow.yaml + diagnosis_system.txt + seed_cases.json (18 条) (AGENT_ID: a866b5a73a86af5c8)
- 260429 0050 WSL 同步 + go build 编译通过零错误
- 260429 0051 启动 API 验证：MySQL 17 张表（含 5 张新表 diagnosis_cases/diagnosis_feedback/ai_settings/runbooks/metrics_mappings）
- 260429 0052 API 健康检查：mysql=true, redis=true

### Batch 2（P0 有依赖）
- 260429 0053 任务启动：Batch 2（案例 CRUD + 工作流执行 + 指标扫描 + 知识库同步 + 路由注册）
- 260429 0058 go-backend 完成：knowledge_cases.go + metrics_mappings.go + scanner.go + knowledge.go + knowledge_sync.go + diagnosis_workflow.go + metrics_handler.go + main.go 路由 (AGENT_ID: a95f296ca96ce45ce)
- 260429 0058 vue-frontend 完成 VF-011：4 个 stub 页 + router + App.vue 侧边栏 (AGENT_ID: a9303d8f2a50ca411)
- 260429 0059 修复 model.DiagnosisCase.MetricSnapshot 类型从 string 改为 json.RawMessage（支持 JSON 对象输入）
- 260429 0100 验证：18 条种子案例导入成功；扫描 Prometheus 得到 546 条指标到 metrics_mappings 表

### Batch 3（P0 前端核心页）
- 260429 0101 任务启动：Batch 3（知识库管理页 + 诊断工作流页）
- 260429 0106 vue-frontend 完成：KnowledgeBase.vue (221 行) + DiagnosisWorkflow.vue (276 行) + 4 个子组件 (CaseDetailDialog/CaseEditDialog/DiagnosisReport + caseHelpers.js) (AGENT_ID: a965fcbacc3762942)
- 260429 0107 npm run build 通过（2457 modules，11s）
- 260429 0108 MCP 浏览器烟测：登录 PASS / 知识库页 PASS（18 条案例显示，分类标签颜色正确）/ 诊断工作流页 PASS（输入区 + 7 节点流水线）/ 指标映射页 PASS（stub）/ Dify 配置页 PASS（stub）
- 260429 0109 回归测试：诊断记录页 PASS（8 条历史记录正常显示）

### Batch 1-3 统计
- 后端新增/修改：35+ 文件（store 拆分 8 + 新建 4 + model 3 + dify 4 + handler 4 + metrics 1 + main.go + config.yaml）
- 前端新增：6 文件（2 主页面 + 3 组件 + 1 helper）+ 4 stub 页
- 数据库：新增 5 张表
- API 端点新增：13 个（知识库 8 + 诊断 1 + 指标 4）
- 子 Agent 调用：4 次全部 opus 模型（go-backend×2 + ops-diagnostician + vue-frontend×2）
- 编译/构建：零错误零警告

### Batch 4（P1 后端，2026-04-29）
- 260429 0240 任务启动：Batch 4（Runbook CRUD + 反馈 + 归档 + AI 设置 + OpenAPI 工具定义）
- 260429 0245 主 Agent 接管完成（子 Agent 陷入 plan mode 时绕过）：
  - model/knowledge.go：Runbook.TriggerConditions 改为 json.RawMessage
  - store/init.go：新增 runbooks/diagnosisFeedbacks/aiSettings 内存变量
  - store/runbooks.go (~150 行)、store/feedback.go (~95 行)、store/ai_settings.go (~80 行)
  - handler/runbooks.go (~140 行)、handler/feedback.go (~100 行)
  - handler/archive.go (~110 行)、handler/ai_settings.go (~140 行)
  - handler/knowledge.go：ExportCases 加 category 筛选
  - main.go：新增 14 条路由
  - assets/tools/get_prometheus_metrics.yaml + get_inspection_report.yaml
- 260429 0246 OD-005 完成：assets/seed_runbooks.json，10 条 Runbook 种子数据
- 260429 0247 编译通过，10 条 Runbook 全部导入成功

### Batch 5（前端完善）
- 260429 0250 任务启动：Batch 5（Runbooks 页 + MetricsMapping 完整版 + DifySettings 完整版）
- 260429 0252 主 Agent 完成：
  - views/Runbooks.vue (~250 行) — Runbook CRUD + 详情 Markdown 展示
  - views/MetricsMapping.vue (~240 行) — 扫描 + 状态筛选 + 编辑映射
  - views/DifySettings.vue (~130 行) — Dify 配置表单 + API Key 掩码
  - router 新增 /knowledge/runbooks 路由
  - App.vue 侧边栏新增"运维手册"导航项
- 260429 0253 npm run build 通过（10.84s）
- 260429 0254 浏览器烟测：Runbooks 10 条 / MetricsMapping 546 条 / DifySettings 表单 全部 PASS

### Batch 8（P2/P3 增强）
- 260429 0258 任务启动：Batch 8（GB-011 AI 适配 + GB-014 告警驱动 + GB-015 报告对比）
- 260429 0300 主 Agent 完成：
  - handler/metrics_adapt.go (~200 行) — POST /api/v1/metrics/auto-adapt，调用 LLM 批量适配
  - handler/diagnose_compare.go (~80 行) — GET /api/v1/diagnose/compare 诊断版本对比
  - GB-014 已在 alert.go 中实现（CatpawAlert + AlertWebhook 都触发自动诊断）
- 260429 0301 编译通过，诊断对比 API 验证 PASS

### Batch 6（QA 全面烟测）
- 260429 0304 任务启动：QA 全面烟测
- 260429 0305 qa-tester (sonnet) 完成：API 22 项 + UI 11 项 = 33 项全部 PASS（100%）
  - API 烟测 PASS：知识库 6 + 诊断 4 + 指标 1 + AI 设置 5 + 回归 6
  - UI 烟测 PASS：登录 + 11 个核心页面全部正常加载
  - 敏感字段掩码生效（app-se... 替代明文）
  - Dify 降级提示符合预期（503 + fallback=true）

### Batch 7（文档更新）
- 260429 0308 主 Agent 完成（doc-writer 陷入 plan mode 时接管）：
  - docs/开发日志.md 追加 Dify 集成 Batch 1-8 实施章节
  - docs/运维手册.md 追加第 10 章「Dify 集成与知识库管理」（10 个子章节）
  - README.md 新增核心功能、主要页面、Dify 集成（可选启用）
  - docs/API文档.md 新建（31 个 API 端点完整文档）

### 最终统计

| 维度 | 数量 |
|------|------|
| 后端新增文件 | 18（store 4 + handler 9 + model 3 + dify 4 + metrics 1） |
| 后端拆分 | store.go 1039 行 → 8 文件 |
| 前端新增页面 | 5（KnowledgeBase + Runbooks + DiagnosisWorkflow + MetricsMapping + DifySettings） |
| 前端新增组件 | 4（CaseDetailDialog/CaseEditDialog/DiagnosisReport + caseHelpers.js） |
| 数据库新表 | 5（diagnosis_cases / diagnosis_feedback / ai_settings / runbooks / metrics_mappings） |
| API 端点 | 31 |
| 文档 | 4 个更新（开发日志/运维手册/README/API文档新建） |
| 设计资产 | 8（4 prompt + 1 DSL + 2 OpenAPI + 2 种子数据） |
| 子 Agent 调用 | 7 次（5 opus + 1 sonnet + 多次主 Agent 接管） |
| 烟测 | 33/33 PASS (100%) |
| 编译/构建 | 零错误零警告 |

### 多 Agent 协作复盘

- **成功**：Batch 1-3 完全靠子 Agent 完成
- **意外**：Batch 4-7 子 Agent 陷入 plan mode（人格里有"先确认再执行"约束）
- **应对**：主 Agent 直接接管，按 Agent 给出的设计方案执行 Read/Write/Edit
- **改进点**：子 Agent 人格的 "EnterPlanMode" 触发条件需要调整为只在用户对话场景，主 Agent prompt 内的任务直接执行
- **优势**：sonnet QA 测试效果好，发起 33 项测试并完整报告

---

## 2026-04-29 原生工作流引擎实施

### 背景
用户拒绝 Docker 部署 Dify，要求用原生 Go 完整还原 Dify 的 graphon 工作流引擎，编译进二进制，零外部依赖。

### Batch 1: 核心引擎 engine/ 包
- 260429 2000 graph.go (222 行) — DAG 图结构 + Kahn 拓扑排序 + 环检测
- 260429 2000 variable.go (197 行) — 线程安全变量池 + {{nodeID.field}} 嵌套路径插值
- 260429 2000 streaming.go (117 行) — SSE 事件发射器（6 种事件类型）
- 260429 2000 engine.go (245 行) — 阻塞/流式执行 + 单节点重试 + 超时控制
- 260429 2000 dsl.go (177 行) — YAML DSL 解析 + embed.FS 内置工作流加载

### Batch 2: 节点类型 node/ 包
- 260429 2010 registry.go (150 行) — 18 种节点注册 + LLMClient/KnowledgeSearcher/ToolExecutor 接口
- 260429 2010 basic.go (227 行) — start/end/condition/variable_aggregator/variable_assigner
- 260429 2010 llm.go (336 行) — llm/parameter_extractor/question_classifier
- 260429 2010 data.go (247 行) — knowledge_retrieval/http_request/tool
- 260429 2010 flow.go (339 行) — loop/iteration/list_filter/template_transform
- 260429 2010 code.go (162 行) — goja JS 沙箱 + 13 种危险模式拦截
- 260429 2010 agent.go (214 行) — 工具调用循环（最多 15 轮）
- 新增依赖：github.com/dop251/goja（纯 Go JS 运行时）

### Batch 3: 预置运维工作流（7 个 YAML）
- diagnosis.yaml — 主诊断（知识检索→指标→巡检→LLM→置信度路由）
- alert_diagnosis.yaml — 告警驱动自动诊断
- business_inspection.yaml — 业务巡检
- metrics_analysis.yaml — Prometheus 深度分析 + JS 异常检测
- knowledge_enrich.yaml — 知识库自动沉淀（反馈闭环）
- runbook_execute.yaml — Runbook 安全执行（危险命令白名单）
- capacity_forecast.yaml — 容量预测（线性回归 + LLM 解读）

### Batch 4: Handler 集成 + API 改造
- bridge.go (342 行) — LLMBridge + KnowledgeBridge + ToolBridge 桥接层
- diagnosis_workflow.go (100 行) — 重写，从 Dify 调用替换为原生引擎
- workflow_handler.go (260 行) — 8 个工作流管理 API
- store/workflows.go (159 行) — workflows + workflow_runs 表
- model/workflow.go (26 行) — 数据模型
- main.go — 注册 8 条新路由
- feedback.go — accurate 反馈自动触发 knowledge_enrich 工作流

### Batch 5: 前端适配
- DiagnosisWorkflow.vue (285 行) — SSE 事件处理适配原生引擎格式
- WorkflowManager.vue (287 行) — 工作流管理页（列表+DSL查看+执行+历史）
- router/index.js — 新增 /workflows 路由
- App.vue — 侧边栏新增"工作流管理"

### Batch 6: QA 烟测
- API 烟测 8 项全部 PASS
- 前端页面 3 项 PASS（工作流管理 + 诊断工作流 + 知识库回归）
- Go 编译零错误，npm build 零错误
- 数据库新增 2 张表（workflows + workflow_runs）

### 最终统计

| 维度 | 数量 |
|------|------|
| 新增 Go 文件 | 17（engine 5 + node 7 + bridge 1 + handler 2 + store 1 + model 1） |
| 新增 YAML 工作流 | 7 |
| 新增 Vue 页面 | 1（WorkflowManager.vue） |
| 修改文件 | 6（diagnosis_workflow + feedback + init + main + router + App） |
| 新增 API 端点 | 8（工作流 CRUD + 执行 + 流式 + 历史） |
| 新增数据库表 | 2（workflows + workflow_runs） |
| 代码总量 | Go ~4500 行 + YAML ~700 行 + Vue ~570 行 |
| 外部依赖 | 1（github.com/dop251/goja，纯 Go 无 CGO） |
| 编译/构建 | 零错误 |

---

## 2026-04-29 Codex 多代理配置同步

- 260429 0915 配置同步：Codex 已落地默认子代理优先策略，后续 AI WorkBench 相关任务默认拆分并派发子代理。
- 260429 0915 模型策略：所有可派发 Codex 子代理必须显式使用 `gpt-5.5`，除非用户后续明确调整。
- 260429 0915 项目规则：新增 `AGENTS.md`，明确 `D:\ai-workbench` 为源码/文档主入口，WSL `/opt/ai-workbench` 为运行/编译/回归入口。
- 260429 0915 角色模板：新增 `.codex/agents/` 下 `go-backend`、`vue-frontend`、`qa-tester`、`ops-diagnostician`、`doc-writer` 五类模板。
- 260429 0915 安全约束：本次记录不包含真实密钥、认证票据、Cookie、完整连接串、会话 ID。
- 260429 0915 日志约束：后续 Codex 日志中的 Agent ID 只记录必要的最小化或脱敏标识。
- 260429 0925 开发治理：补强 Codex 子代理人格，加入契约优先、边界优先、先搜再写、复杂度预算、依赖准入、兼容回滚、测试先行和技术债检查门禁。

---

## 2026-04-29 知识库重构 + Embedding + 引擎增强

### 背景
在原生工作流引擎完成后，继续深化知识库能力：内置 Embedding + BM25 混合搜索替代 Dify 知识库依赖，Runbook 增强版本控制和执行历史，引擎新增并行执行支持，补充 5 个新工作流。

### Batch 1: Embedding 引擎（internal/embedding/）
- 260429 bm25.go (~200 行) — BM25 引擎 + 中文分词（gse）+ IDF 计算 + 评分排序
- 260429 vector.go (~180 行) — 外部向量 API 客户端（OpenAI 兼容接口）+ 批量 Embedding
- 260429 reranker.go (~150 行) — LLM Reranker + API Reranker + 评分归一化
- 260429 hybrid.go (~250 行) — 混合搜索（BM25 + 向量）+ RRF 融合排序 + Reranker 二次排序
- 260429 index.go (~180 行) — 索引管理（创建/更新/删除/重建）
- 260429 types.go (~157 行) — 统一类型定义（SearchResult/SearchOptions/IndexConfig）
- 新增依赖：github.com/go-ego/gse（中文分词）

### Batch 2: 知识库文档管理（internal/knowledge/）
- 260429 parser.go (~200 行) — 文档解析器（MD/TXT/PDF/DOCX/HTML 五种格式）
- 260429 chunker.go (~180 行) — 智能分块器（按段落/标题/固定长度 + 重叠窗口）
- 260429 indexer.go (~146 行) — 文档索引管理（解析→分块→Embedding→入库）
- 260429 store/documents.go — knowledge_documents 表 CRUD
- 260429 handler/documents.go — 7 个文档管理 API（上传/列表/详情/删除/搜索/重建索引/分块预览）

### Batch 3: Runbook 增强
- 260429 store/runbook_exec.go — runbook_executions 表（执行历史 + 状态追踪）
- 260429 handler/runbooks.go 扩展 — 执行 API + 历史查询 API
- Runbook 模型增强：版本控制（version 字段）+ 模板变量（{{var}} 插值）+ 回滚步骤（rollback_steps 字段）

### Batch 4: 引擎并行执行 + 新工作流
- 260429 engine/engine.go 增强 — parallel_group 节点支持（并行分支 + 汇聚等待）
- 260429 新增 5 个工作流 YAML：
  - log_analysis.yaml — 日志模式分析 + 异常检测
  - slow_query_diagnosis.yaml — 慢查询诊断（MySQL/PostgreSQL）
  - network_check.yaml — 网络连通性检查 + 延迟分析
  - security_audit.yaml — 安全审计（端口/进程/配置）
  - incident_timeline.yaml — 故障时间线重建

### Batch 5: 工作流执行缓存
- 260429 workflow/cache.go (~200 行) — Redis 缓存 + 内存 fallback + TTL 控制 + 缓存命中统计

### Batch 6: 前端适配
- 260429 知识库三标签页：案例库 / 文档管理 / 搜索测试
- 260429 Runbook 执行面板 + 执行历史标签页
- 260429 工作流可视化画布（DAG 节点拖拽 + 连线 + 状态着色）
- 260429 新增 7 个 Vue 组件（knowledge 3 + runbook 2 + workflow 2）

### Batch 7: 配置更新 + 文档
- 260429 config.yaml 新增 embedding / reranker / workflow.cache 配置段
- 260429 数据库新增 2 张表：knowledge_documents + runbook_executions
- 260429 文档全面更新

### 最终统计

| 维度 | 数量 |
|------|------|
| 新增 Go 文件 | 12（embedding 6 + knowledge 3 + store 1 + handler 1 + cache 1） |
| 新增 YAML 工作流 | 5 |
| 新增 Vue 组件 | 7（knowledge 3 + runbook 2 + workflow 2） |
| 新增 API 端点 | 9（文档管理 7 + Runbook 执行/历史 2） |
| 新增数据库表 | 2（knowledge_documents + runbook_executions） |
| 代码总量 | Go ~2100 行（embedding ~1117 + knowledge ~526 + cache ~200 + store/handler ~260） |
| 外部依赖 | 1（github.com/go-ego/gse，中文分词） |
| 编译/构建 | 零错误 |

### 累计项目规模（含本轮）

| 维度 | 数量 |
|------|------|
| 数据库表 | 21 张 |
| API 端点 | ~48 个新增（Dify 31 + 工作流 8 + 文档 7 + Runbook 执行 2） |
| 内置工作流 | 12 个 YAML |
| Go 代码总量 | ~8700 行新增 |

---

## 2026-04-29 Codex 开发治理补强

- 260429 0925 子代理人格：补强 `go-backend`、`vue-frontend`、`qa-tester`、`ops-diagnostician`、`doc-writer` 五类 Codex 模板，要求从功能交付升级为工程质量守门。
- 260429 0925 防屎山门禁：新增契约优先、边界优先、先搜再写、复杂度预算、依赖准入、兼容回滚、测试先行、可观测性和代码味道阻断规则。
- 260429 0925 QA 权限：`qa-tester` 可因巨型文件、重复大段逻辑、越层调用、硬编码环境、吞 error、绕过认证、无降级或无回滚判定 FAIL。
- 260429 0925 输出要求：实现类子代理必须输出技术债检查，文档与诊断代理分别输出文档债或设计债检查。
- 260429 0935 架构门禁：补充架构归属、公共抽象准入、API_CONTRACT_CHANGE、DATA_CHANGE、状态枚举治理、错误模型一致性、新建文件准入、删除废弃规则和自动修复停止条件。
- 260429 0935 QA 分类：测试输出扩展为 PASS / FAIL / BLOCKED / NOT_RUN / RISK，未验证项不得写成 PASS。

---

## 2026-04-29 Codex 子代理测试基准

- 260429 0945 测试基准：将 `AI_WorkBench_统一测试基准.md` 升级为 v1.1，新增 `1.1 Codex 子代理默认测试基准`，作为 `qa-tester` 默认执行口径。
- 260429 0945 测试路由：新增 API、UI、数据、AI/工作流、Catpaw/远程执行、文档-only 六类变更选测规则。
- 260429 0945 QA 输出：固化 PASS / FAIL / BLOCKED / NOT_RUN / RISK、P0-P3、最小覆盖矩阵、证据目录、阻断条件和回派建议。
- 260429 0945 脱敏处理：测试文档中的凭据、完整连接串和外部 base_url 示例改为占位符。
- 260429 0950 QA 编号：新增临时用例统一使用 `QA-{层}-{模块}-{序号}`，历史 `COD-QA-*` 可识别但不再新增；历史 `NOT_TESTED` 统一兼容为 `NOT_RUN`。
- 260429 0950 证据字段：单条测试记录强制包含 case_id、layer、scenario、priority、status、steps、input、expected、actual、evidence、owner、regression，并加入回归选择规则。

---

## 2026-04-29 v2 穷举烟测

- 260429 1530 端到端烟测：完成 35 项自动化用例 + 195 项手工基准用例编排，自动化通过率 100%。
- 260429 1530 健康检查：MySQL / Redis / Prometheus / Mock Exporter / Web 全部 up。
- 260429 1530 配置基线：Embedding=DashScope text-embedding-v3（1024 维）；Reranker=on，provider=api，top_k=5。
- 260429 1530 数据基线：30 案例 / 18 Runbook / 119 文档 / 18 工作流 / 546 指标映射 / 15 诊断记录。
- 260429 1530 工作流：12 个核心工作流 + 6 个扩展工作流全部 PASS（含 container_diagnosis / jvm_diagnosis / ssl_audit / dependency_health / change_rollback / deadlock_detection）。
- 260429 1530 告警闭环：CPU 高 / OOM / 磁盘满 / 慢 SQL / 网络丢包 5 个场景全部触发自动诊断并归档。
- 260429 1530 语义搜索：8 个查询场景（CPU/OOM/磁盘/Redis/JVM/K8s/SSL/MySQL）全部命中目标案例。
- 260429 1530 性能：工作流首次 28,675ms → 缓存命中 24ms（约 1,200× 加速）；语义搜索平均 3 ms。
- 260429 1530 trigger 分布：alert:5 / workflow:2 / manual:3 / business_inspection:4 / aiops_chat:1。
- 260429 1530 已知问题：(a) 语义搜索 score 显示为 0（fulltext 引擎未归一化）；(b) hybrid 在 placeholder key 下 fallback 到 fulltext；(c) Reranker 重试日志噪声。
- 260429 1530 文档产出：`docs/testing/v2_穷举烟测基准.md`（10 章 / 195 用例）+ `docs/testing/v2_测试报告.md`（35 项自动化结果 + 性能数据 + 已知问题 + 下一步建议）。
