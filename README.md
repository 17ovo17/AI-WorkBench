# AI WorkBench

AI WorkBench 是面向 AI 运维问答、智能诊断、Catpaw 探针巡检、Prometheus/Categraf 监控适配、告警和业务拓扑的工作台，内置原生工作流引擎和知识库混合搜索，实现知识库驱动的智能诊断。

## 核心功能

| 模块 | 说明 |
|------|------|
| 智能对话 | LLM + WebSocket 流式诊断，支持工具调用 |
| 诊断记录 | 历史诊断报告查询，结构化巡检展示 |
| **原生工作流引擎** | DAG 执行引擎 + 18 种节点 + 12 个内置运维工作流 + 并行执行 |
| **知识库** | 诊断案例 CRUD + 文档管理 + BM25/向量混合搜索 + Reranker |
| **运维手册（Runbooks）** | 标准化 SOP + 版本控制 + 模板变量 + 执行历史 + 回滚步骤 |
| **诊断工作流** | 原生引擎驱动的流水线 + SSE 流式 + 缓存 |
| **指标映射** | Prometheus 全量扫描 + AI 自动适配 + 用户确认 |
| 告警中心 | 告警接收、去重、自动诊断、值班分配 |
| 业务拓扑 | D3 力导向图 + 风险分析 + 一键巡检 |
| 探针管理 | Catpaw 探针注册/心跳/远程安装 |
| **工作流管理** | 工作流列表 + DSL 查看 + 手动执行 + 可视化画布 |

## 主要页面

- 运维总览 / 智能对话 / 诊断记录 / 知识库（案例库 + 文档管理 + 搜索测试）/ 运维手册（Runbook + 执行历史）/ 诊断工作流 / 工作流管理（列表 + 可视化画布）/ 告警中心 / 业务拓扑 / 探针管理
- 设置：常用地址 / AI 配置 / 数据源 / 指标映射 / 值班通知

## 运行环境

| 项目 | 值 |
|------|------|
| 操作系统 | Ubuntu 24.04.4 LTS (WSL2) |
| 项目目录 | /opt/ai-workbench（WSL）、D:\ai-workbench（Windows 映射） |
| 前端 | http://172.20.32.65:3000 |
| 后端 | http://172.20.32.65:8080 |
| Prometheus | http://172.20.32.65:9090 |
| 登录 | admin / admin123 |

## 启停命令

```bash
# 启动
bash scripts/start-api.sh    # 后端
bash scripts/start-web.sh    # 前端

# 构建前端
bash scripts/build-web.sh
```

## 配置文件

核心配置：`api/config.yaml`

```yaml
mysql:
    dsn: root:***@tcp(127.0.0.1:3306)/ai_workbench?charset=utf8mb4&parseTime=true&loc=Local
server:
    host: "0.0.0.0"    # 监听地址，可改为 127.0.0.1 仅本机访问
    port: "8080"
    server_ip: "172.20.32.65"

# Embedding 引擎（知识库混合搜索）
embedding:
    provider: "builtin"          # builtin（仅 BM25）或 api（BM25 + 向量混合搜索）
    api:
        url: ""                  # 向量 API 地址（OpenAI 兼容接口）
        key: ""                  # 通过环境变量设置更安全
        model: "text-embedding-3-small"
        dimensions: 1536
        batch_size: 20

# Reranker（可选，二次精排）
reranker:
    enabled: false
    provider: "llm"              # llm 或 api
    top_k: 5

# 工作流执行缓存
workflow:
    cache:
        enabled: true
        ttl_seconds: 300
```

详细配置说明见 `docs/运维手册.md`。

## 原生工作流引擎

内置 DAG 工作流引擎，零外部依赖，所有能力编译进单一 Go 二进制：

- 18 种节点类型（LLM / 知识检索 / HTTP / 代码沙箱 / 条件分支 / 循环 / 并行等）
- 12 个预置运维工作流（诊断 / 告警 / 巡检 / 指标分析 / 日志分析 / 慢查询 / 网络检查 / 安全审计等）
- 并行执行（parallel_group 节点）
- SSE 流式输出
- 执行缓存（Redis + 内存 fallback）
- 自定义工作流（YAML DSL）

详见 `docs/运维手册.md` 第 11 章。

## 知识库混合搜索

内置 Embedding 引擎，支持多种搜索模式：

- BM25 关键词搜索（内置中文分词，零外部依赖）
- 向量语义搜索（需配置 Embedding API）
- BM25 + 向量混合搜索（RRF 融合排序）
- Reranker 二次精排（可选）

支持 MD / TXT / PDF / DOCX / HTML 文档上传、自动解析和分块。详见 `docs/运维手册.md` 第 12 章。

## API 文档

详见 `docs/API文档.md`，48 个新增 API（知识库案例 / 文档管理 / Runbook / 诊断工作流 / 工作流管理 / 反馈 / 归档 / 指标映射 / AI 设置）。

## 测试规范

统一测试基准（唯一权威文档）：

- `docs/testing/AI_WorkBench_统一测试基准.md` — 26 章 + 4 附录，约 540 个用例

基础烟测脚本：

```bash
powershell -ExecutionPolicy Bypass -File ./scripts/testing/run_full_smoke.ps1
```

测试证据写入 `.test-evidence/<batch-id>`，敏感凭证不得写入文档或日志。
