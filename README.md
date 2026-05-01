<h1 align="center">AI WorkBench</h1>
<p align="center">面向运维工程师的 AI 驱动智能运维平台</p>
<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21-00ADD8?logo=go" />
  <img src="https://img.shields.io/badge/Vue-3.5-4FC08D?logo=vue.js" />
  <img src="https://img.shields.io/badge/MySQL-8-4479A1?logo=mysql" />
  <img src="https://img.shields.io/badge/License-MIT-green" />
</p>

---
##还会持续更新，功能未完全实现，大家可以加群一起讨论一下实现方法，欢迎来抓虫~，加我进交流群，欢迎提Issues
<img width="400" height="400" alt="a10e4079bdca90dbc8645ad8b5a996c1" src="https://github.com/user-attachments/assets/dc3d8887-e974-4caa-83c3-7c43bd3e258f" />
## 什么是 AI WorkBench

AI WorkBench 是一个全栈 AIOps 平台，将 LLM 推理能力与结构化工作流引擎结合，实现从告警接入到智能诊断、知识沉淀的全链路闭环。

**核心理念：LLM 负责理解和推理，代码负责精确计算，工作流负责编排协调。**

## 核心能力

| 能力 | 说明 |
|------|------|
| **智能诊断** | 结构化推理引擎（假设→证据→验证循环），不是让 LLM 猜 |
| **工作流编排** | Go 原生 DAG 引擎，20 种节点类型，9 个内置运维工作流 |
| **混合搜索** | BM25 + 向量搜索 + RRF 融合 + Reranker 四层检索 |
| **时序分析** | CUSUM 突变检测 + 趋势回归 + 异常评分 + 周期性检测 |
| **告警闭环** | 多源接入 → 风暴抑制 → 智能路由 → 自动诊断 → 知识沉淀 |
| **知识沉淀** | 案例库 + Runbook + 文档管理，诊断完成自动归档 |
| **监控集成** | Prometheus 指标扫描 → AI 自动适配 → 标准名映射 |

## 技术栈

```
后端    Go 1.21 + Gin          前端    Vue 3.5 + Vite 5 + Element Plus
数据库  MySQL 8                 缓存    Redis（可选，内存 fallback）
监控    Prometheus              搜索    BM25 + 向量(DashScope/OpenAI兼容)
沙箱    goja (JS)               分词    go-ego/gse
```

## 快速开始

### 环境要求

- Go 1.21+
- Node.js 18+
- MySQL 8（必须，自动建表）
- Redis（可选）
- Prometheus（可选）

### 1. 克隆项目

```bash
git clone https://github.com/17ovo17/AI-WorkBench.git
cd AI-WorkBench
```

### 2. 配置后端

```bash
cd api
cp config.yaml.example config.yaml
# 编辑 config.yaml，填入 MySQL 连接信息
# AI 模型、Embedding、数据源等在前端界面配置
```

### 3. 启动后端

```bash
go mod tidy
go run main.go
# 服务监听 http://localhost:8080
```

### 4. 启动前端

```bash
cd ../web
npm install
npm run dev
# 前端监听 http://localhost:3000
```

### 5. 访问平台

打开浏览器访问 `http://localhost:3000`，进入「AI 模型」页面配置你的 LLM API Key。

## 项目结构

```
AI-WorkBench/
├── api/                          # Go 后端
│   ├── main.go                   # 入口（143 个 API 端点）
│   ├── config.yaml.example       # 配置模板
│   └── internal/
│       ├── handler/              # HTTP 处理层
│       ├── store/                # 数据持久层（MySQL + 内存 fallback）
│       ├── model/                # 数据模型
│       ├── workflow/             # 工作流引擎
│       │   ├── engine/           # DAG 核心 + 20 种节点
│       │   │   └── builtin/      # 内置工作流 YAML
│       │   └── node/             # 节点实现
│       ├── reasoning/            # 结构化推理引擎
│       ├── timeseries/           # 时序分析引擎
│       ├── embedding/            # 混合搜索引擎
│       ├── knowledge/            # 知识库管理
│       ├── correlator/           # 跨数据源关联
│       ├── eventbus/             # 事件总线
│       ├── middleware/           # 熔断器 + 限流
│       ├── scheduler/            # 定时调度
│       └── security/             # 安全机制
├── web/                          # Vue 3 前端
│   └── src/
│       ├── views/                # 10 个页面
│       └── components/           # 可复用组件
├── docker/                       # Docker + Prometheus 配置
├── scripts/                      # 启停脚本
└── docs/                         # 项目文档
```

## 工作流引擎

Go 原生实现的 DAG 工作流引擎，零外部依赖。

### 20 种节点类型

| 类型 | 作用 |
|------|------|
| `start` / `end` | 工作流入口和出口 |
| `llm` | 调用大模型推理 |
| `knowledge_retrieval` | 知识库混合搜索 |
| `http_request` | HTTP 请求 |
| `code` | JavaScript 沙箱执行 |
| `condition` | 条件分支（支持 in/starts_with/regex 等） |
| `loop` / `iteration` | 循环和迭代 |
| `sub_workflow` | 子工作流编排 |
| `human_input` | 暂停等待用户输入 |
| `agent` | LLM + 工具自主循环 |
| `tool` | 工具调用 |
| `template_transform` | 模板渲染 |
| `variable_aggregator` / `variable_assigner` | 变量操作 |
| `parameter_extractor` / `question_classifier` | LLM 提取/分类 |
| `list_filter` / `document_extractor` | 数据处理 |

### 9 个内置工作流

| 工作流 | 用途 |
|--------|------|
| `smart_diagnosis` | 智能诊断（手动/告警触发） |
| `domain_diagnosis` | 专项诊断（容器/JVM/数据库/日志） |
| `health_inspection` | 健康巡检（业务/依赖/存储/中间件） |
| `metrics_insight` | 指标分析（异常/预测/SLO/流量） |
| `security_compliance` | 安全合规（审计/SSL/配置漂移） |
| `incident_review` | 故障回顾（时间线/复盘） |
| `network_check` | 网络连通性检测 |
| `runbook_execute` | Runbook 执行/变更回滚 |
| `knowledge_enrich` | 知识库自动沉淀 |

### 智能路由

用户输入自然语言，系统自动选择最匹配的工作流：

```
"JVM Full GC 频繁"  → domain_diagnosis (domain=jvm)
"做一次全面巡检"     → health_inspection (scope=full)
"SSL 证书快过期了"   → security_compliance (audit_type=ssl)
```

## 安全机制

| 机制 | 说明 |
|------|------|
| JS 沙箱 | goja 运行时冻结全局对象 + 删除 eval/Function + Unicode 归一化 |
| 命令安全 | L0-L4 四级分级，工作流模式 L2 降级 L1，L3/L4 拒绝 |
| 审计日志 | HMAC-SHA256 签名防篡改 |
| 告警风暴 | 60 秒内同 IP >20 条自动抑制 |
| LLM 并发 | 信号量限制 5 并发 + 熔断器 |
| API 限流 | 令牌桶算法 100 req/s |

## API 概览

143 个 REST API 端点，按功能分组：

- **对话与 AI** — 通用对话、Agent 对话
- **AIOps 智能运维** — 会话、巡检、WebSocket 实时推理
- **诊断** — 启动诊断、记录管理、反馈、归档
- **告警** — Catpaw/Alertmanager/夜莺 webhook 接入
- **知识库** — 案例 CRUD、文档上传、Runbook 管理、语义搜索
- **工作流** — CRUD、执行、SSE 流式、定时调度
- **拓扑** — 业务拓扑管理、AI 生成、巡检
- **监控** — Prometheus 集成、指标扫描、AI 适配
- **设置** — AI 模型、Embedding、Reranker、数据源、通知渠道

完整 API 文档见 `docs/API文档.md`。

## 前端页面

| 页面 | 功能 |
|------|------|
| 运维总览 | 仪表盘（告警/诊断/工作流/知识库统计） |
| 智能诊断 | AIOps 多轮对话 + 实时指标面板 |
| 知识中心 | 案例库 + Runbook + 文档管理 + 语义搜索 |
| 工作流 | 诊断工作流 + 工作流管理（参数化执行） |
| 告警中心 | 告警列表 + 操作（确认/静默/解决） |
| 业务拓扑 | 拓扑可视化 + AI 生成 + 业务巡检 |
| AI 模型 | LLM Provider 配置 |
| 系统配置 | 数据源 + 指标映射 + Embedding + 通知 |

## 许可证

MIT License



