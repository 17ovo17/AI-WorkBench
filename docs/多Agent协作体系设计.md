# AI WorkBench 多 Agent 协作体系设计

> 版本：v2.0 | 日期：2026-04-29

## 一、设计目标

借鉴"主 Agent 编排 + 子 Agent 执行 + 修正闭环"模式，结合 AIOps 平台特点，实现：

1. 主 Agent 只做编排和决策，不直接写业务代码
2. 专业化子 Agent 各司其职（后端/前端/测试/诊断/文档）
3. "开发 → 测试 → 修正"闭环，子 Agent 写的 bug 由同一个子 Agent 修复
4. Agent ID 追踪，修正时 resume 而不是新建
5. 日志驱动，所有操作记录到 main-log.md
6. 防屎山强约束贯穿所有 Agent，避免代码质量退化

## 二、子 Agent 角色定义

### 2.1 go-backend（Go 后端开发）

职责：编写/修改 Go 后端代码（handler/model/store/security）
规则：
- 只操作 api/ 目录下的 .go 文件
- 每个文件不超过 400 行
- 修改后必须 `go build` 验证编译通过
- 不操作前端代码、不操作配置文件

### 2.2 vue-frontend（Vue 前端开发）

职责：编写/修改 Vue 3 前端代码（views/components/router/utils）
规则：
- 只操作 web/src/ 目录下的文件
- 每个 .vue 文件不超过 300 行
- 使用 Element Plus 组件库
- 不操作后端代码

### 2.3 qa-tester（质量测试）

职责：验证功能正确性
规则：
- 用 MCP 浏览器做 UI 验证（登录 → 操作 → 截图 → 验证）
- 用 curl 做 API 验证
- 输出测试报告：每个检查项标记 PASS/FAIL
- 不修改任何代码，只报告问题

### 2.4 ops-diagnostician（运维诊断专家）

职责：诊断工作流设计、指标适配、知识库管理
规则：
- 编写 Dify 工作流 DSL
- 编写 metrics_mapping.yaml
- 编写诊断 prompt
- 不直接写 Go/Vue 代码

### 2.5 doc-writer（文档维护）

职责：更新项目文档
规则：
- 只操作 docs/ 和 README.md
- 每次代码变更后同步更新相关文档
- 维护开发日志

## 三、主 Agent 编排流程

### 3.1 功能开发流程

```
用户需求 → 主 Agent 分析拆解
    │
    ├── Phase 1: 开发
    │   ├── 启动 go-backend Agent（后台）→ 记录 Agent ID
    │   ├── 启动 vue-frontend Agent（后台）→ 记录 Agent ID
    │   └── 等待完成 → 同步到 WSL → 编译
    │
    ├── Phase 2: 测试
    │   ├── 启动 qa-tester Agent → 输出测试报告
    │   └── 解析报告：全 PASS → Phase 3 / 有 FAIL → 修正
    │
    ├── Phase 2.5: 修正循环（最多 3 轮）
    │   ├── 收集 FAIL 项
    │   ├── resume go-backend Agent（用原 ID）修复后端问题
    │   ├── resume vue-frontend Agent（用原 ID）修复前端问题
    │   ├── 重新编译 → 重新测试
    │   └── 3 轮后仍 FAIL → 标记为人工处理
    │
    └── Phase 3: 收尾
        ├── 启动 doc-writer Agent → 更新文档
        ├── 同步到 WSL
        └── 写入 main-log.md 统计
```

### 3.2 主 Agent 禁止清单

1. 不直接编辑 .go / .vue / .ts 文件 — 全部委托子 Agent
2. 不读测试报告全文 — 只用 Grep 提取 PASS/FAIL 行
3. 不跳过测试 — 开发完必须测试
4. 不跳过 Agent ID 收集 — 修正必须 resume 原 Agent

### 3.3 Agent ID 收集（修正闭环的关键）

修正循环必须 resume 同一个子 Agent，而不是启动新 Agent。这依赖 Agent ID 的准确收集。

#### 获取方式：文件系统探测

子 Agent 完成后，其 agentId 会写入文件系统。用以下命令获取最新的 agent ID：

```bash
find ~/.claude/projects/ -name "agent-*.meta.json" -type f -printf '%T@ %p\n' 2>/dev/null | sort -rn | head -1 | cut -d' ' -f2-
```

文件名格式 `agent-abc123.meta.json`，提取裸 ID 即 `abc123`。

#### ID 使用规则

1. 收到子 Agent 返回后，第一时间执行上述命令，将 ID 写入日志，不要先做其他事
2. 日志格式：`- {yymmdd hhmm} 任务完成：{任务描述} (AGENT_ID: {id})`
3. 如果获取不到 ID，禁止跳过、禁止启动新 Agent，暂停并报告错误
4. 修正时使用 `SendMessage(to: "{AGENT_ID}")` 恢复原 Agent
5. 每个批次的所有子 Agent ID 都要记录，方便后续任意一个需要修正时能找到

#### 为什么不能新建 Agent

新建的 Agent 没有原 Agent 的上下文（它写了哪些代码、做了哪些决策、遇到了什么问题）。resume 原 Agent 可以保留完整的对话历史，修 bug 更准确。

### 3.4 防屎山强约束（所有 Agent 共同遵守）

#### 写代码前必须做的事
1. 先读再写：修改任何文件前，必须先 Read 该文件
2. 先搜再建：新建函数/组件前，先 Grep 搜索是否已有类似实现
3. 先想再做：超过 3 个文件的改动，必须先列出改动清单

#### 绝对禁止
- 复制粘贴代码块 → 提取为函数/组件
- God Function（>50 行）→ 拆分
- Magic Number → 命名常量或配置项
- 忽略错误 → 必须处理
- 硬编码 URL/IP/密码 → 配置或环境变量
- 跨层调用 → handler 通过 store 操作数据库
- 循环导入 → 立即重构

#### AIOps 平台特有约束
- 降级优先：外部依赖调用必须有超时和 fallback
- 指标安全：PromQL 标签值要转义防注入
- 诊断幂等：并发诊断不创建重复记录
- 知识库一致性：Dify 同步失败不影响本地数据
- 审计追踪：所有写操作记录 audit_event

### 3.3 日志规范

追加到 `docs/main-log.md`，每行以 `- ` 开头：

```
- {yymmdd hhmm} 任务启动：{任务描述}
- {yymmdd hhmm} 后端开发完成 (AGENT_ID: {id})
- {yymmdd hhmm} 前端开发完成 (AGENT_ID: {id})
- {yymmdd hhmm} 编译通过
- {yymmdd hhmm} 测试结果：PASS {n} / FAIL {m}
- {yymmdd hhmm} 第{round}轮修正完成 (AGENT_ID: {id})
- {yymmdd hhmm} 最终结果：全部 PASS / {n} 项需人工处理
- {yymmdd hhmm} 文档已更新
```

## 四、与 Dify 工作流的结合

Dify 工作流处理的是运行时的诊断流程（用户发起诊断 → 检索案例 → 查指标 → LLM 分析），而 Claude Agent 体系处理的是开发时的代码编写流程。两者的结合点：

1. ops-diagnostician Agent 负责设计 Dify 工作流 DSL
2. go-backend Agent 负责实现 Dify 代理层代码（internal/dify/）
3. vue-frontend Agent 负责实现诊断交互页和知识库页
4. qa-tester Agent 负责验证 Dify 集成后的端到端流程

## 五、Codex 适配版

### 5.1 默认策略

Codex 后续在 AI WorkBench 项目中默认使用子代理优先策略：

1. 所有可派发子代理必须显式使用 `model: "gpt-5.5"`。
2. 主代理负责需求裁决、关键路径、写集互斥、合并、最终验证和安全裁决。
3. 代码实现任务默认派发对应角色；测试任务默认派发 `qa-tester`；文档同步默认派发 `doc-writer`。
4. 单文件、三步以内、无并行收益的任务可以主代理直接执行，但需要说明原因。
5. 同一缺陷优先回派原子代理修复，最多 3 轮；仍失败则主代理接管或标记人工处理。

### 5.2 Claude 到 Codex 的映射

| Claude 机制 | Codex 机制 | 说明 |
| --- | --- | --- |
| `Task` | `spawn_agent` | 创建 `worker` 或 `explorer`，用角色 prompt 模板模拟命名 Agent |
| `SendMessage` | `send_input` | 将 QA 失败项或修正要求回派给原 agent |
| `TaskOutput` | `wait_agent` | 等待一个或多个 agent 完成，避免无意义轮询 |
| `.claude/agents/*.md` | `.codex/agents/*.md` | Codex 派发 prompt 模板，不直接假设存在原生命名 agent |
| Agent ID 日志 | Codex agent id | 为便于后续回派，只记录必要的最小化或脱敏 id |

### 5.3 Codex 角色模板

Codex 项目级规则入口：

- `AGENTS.md`：项目级多代理执行约定。
- `.codex/agents/go-backend.md`：Go 后端实现模板。
- `.codex/agents/vue-frontend.md`：Vue 前端实现模板。
- `.codex/agents/qa-tester.md`：只读 QA 测试模板。
- `.codex/agents/ops-diagnostician.md`：诊断、指标、Runbook、工作流设计模板。
- `.codex/agents/doc-writer.md`：文档同步模板。

### 5.4 强化门禁

- **关键路径识别**：主代理先判断当前下一步是否依赖某项结果；阻塞项本地做，旁路项派发。
- **写集互斥**：同一批并行 agent 不得写同一文件或同一不可分割模块。
- **QA 权威优先**：开发代理自测不能替代 QA；QA 必须按 `AI_WorkBench_统一测试基准.md` 的 `1.1 Codex 子代理默认测试基准` 输出 PASS/FAIL/BLOCKED/NOT_RUN/RISK，FAIL 必须归属到责任角色。
- **敏感配置屏蔽**：prompt、日志、文档只使用 `<API_KEY>`、`<TOKEN>`、`<DB_DSN>` 等占位符。
- **Agent ID 最小化**：日志只记录必要的短 id、脱敏 id 或主代理本地可追溯信息，不扩散完整会话类标识。
- **失败归属回派**：修复优先发回原 agent，上下文保留，最多 3 轮。
- **Windows/WSL 分离**：`D:\ai-workbench` 是源码和文档主入口，`/opt/ai-workbench` 是运行和回归入口；同步方向必须明确。

### 5.5 开发防屎山门禁

Codex 子代理不仅交付功能，还要阻断代码劣化。以下规则为默认门禁：

- **执行入口唯一**：Codex 项目执行以 `AGENTS.md` 和 `.codex/agents/*.md` 为准；本文档只描述体系原则和历史迁移关系，不作为具体工具命令来源。
- **历史机制隔离**：涉及 `.claude/agents/`、`CLAUDE.md`、`SendMessage`、`~/.claude/projects/` 的内容仅作为 Claude 经验迁移说明，Codex 不依赖这些状态文件执行任务。
- **架构归属判断**：修改前声明目标模块、所属层级、不可跨越边界和是否新增公共抽象。
- **契约优先**：API、数据库、配置、工作流 DSL、前端状态结构变更前，必须说明输入、输出、错误、权限、兼容和降级行为。
- **边界优先**：后端不得越过 handler/service/store/model/security 分层；前端不得绕过 page/component/api/composable/router 分层；文档不得替代真实验证。
- **先搜再写**：新建函数、组件、脚本、配置、文档前必须搜索现有实现，避免重复造轮子。
- **复杂度预算**：超过角色模板中的文件、函数、组件复杂度上限时必须拆分，不接受“以后再整理”。
- **公共抽象准入**：公共函数、组件、包或工具需要至少 2 个真实调用点，或属于项目既有架构标准接口。
- **依赖准入**：新增 Go module、npm 包、UI 框架、存储组件或外部服务必须说明必要性、替代方案、维护成本和验证命令。
- **API_CONTRACT_CHANGE**：接口路径、方法、请求、响应、错误码、认证变化必须打标并联动前端、QA、文档。
- **DATA_CHANGE**：数据库表、索引、字段语义、状态值、唯一约束变化必须打标并说明迁移、回滚、兼容和幂等。
- **兼容回滚**：API、数据库、配置、调度、远程执行、AI/模型配置变更必须说明兼容、迁移、回滚和数据保护策略。
- **测试先行**：修复类任务先建立复现或测试点；开发代理自测不能替代 QA 或主代理回归。
- **技术债检查**：实现类子代理输出必须包含重复逻辑、复杂度、边界、依赖、兼容、测试、回滚和遗留风险。
- **QA 可阻断**：`qa-tester` 可以因巨型文件、重复大段逻辑、越层调用、硬编码环境、吞 error、绕过认证、无降级或无回滚判定 FAIL。
- **停止条件**：需要真实密钥/生产数据、测试环境不可替代、修复越界、修复方向冲突或需求与架构冲突时停止自动修复。

### 5.6 防屎山验收清单

```text
【架构门禁】
- 是否声明目标模块、层级和边界：是/否
- 是否新增文件/目录：是/否；若是，是否说明复用检查结果
- 是否新增公共抽象：是/否；若是，是否列出调用点
- 是否新增第三方依赖：是/否；若是，是否完成准入说明
- 是否存在跨层调用：是/否

【契约门禁】
- 是否 API_CONTRACT_CHANGE：是/否
- 是否 DATA_CHANGE：是/否
- 是否新增/修改状态值、错误码、枚举：是/否
- 是否需要文档同步：是/否

【质量门禁】
- 是否处理错误路径：是/否
- 是否处理权限路径：是/否/不涉及
- 是否处理降级路径：是/否/不涉及
- 是否处理幂等/重复提交：是/否/不涉及
- 是否记录审计或说明不需要：是/否

【验证门禁】
- 构建/编译是否通过：PASS/FAIL/BLOCKED
- 正常路径是否验证：PASS/FAIL/BLOCKED
- 异常路径是否验证：PASS/FAIL/BLOCKED
- 边界或权限是否验证：PASS/FAIL/BLOCKED
- 未覆盖项是否说明：是/否
```

### 5.7 Codex 子代理测试基准

Codex 测试任务默认派发 `qa-tester`，并使用 `D:\ai-workbench\docs\testing\AI_WorkBench_统一测试基准.md` 作为唯一权威测试基准。

- **基准入口**：统一测试基准 `1.1 Codex 子代理默认测试基准`。
- **用例引用**：优先引用既有编号；新增功能暂无编号时使用 `QA-{层}-{模块}-{序号}` 临时编号；历史 `COD-QA-*` 可识别但不再新增。
- **结果状态**：PASS、FAIL、BLOCKED、NOT_RUN、RISK；历史 NOT_TESTED 视为 NOT_RUN；未执行或证据不足不得 PASS。
- **变更选测**：按 API、UI、数据、AI/工作流、Catpaw/远程执行、文档-only 路由选择测试范围。
- **最小矩阵**：P0-P3、正常/异常/边界/权限/安全/降级/回归、API/UI/数据/任务/文档、证据和责任归属。
- **单条证据**：case_id、layer、scenario、priority、status、steps、input、expected、actual、evidence、owner、regression。
- **回归选择**：改 API 回归调用方和契约；改 UI 回归页面、控件、导航和响应式；改数据回归迁移、旧数据和回滚；改权限回归 401/403 与越权；改任务回归重试、并发和审计；改文档回归命令、路径、端口和接口一致性。
- **阻断条件**：P0/P1 遗留、敏感信息泄露、S0 失败、P0/P1 证据缺失、API/DATA 变更未验证、teardown 缺失。
- **证据目录**：默认 `.test-evidence/<yyyymmdd-hhmm>-codex-<task-slug>/`，只读环境不能写证据时必须在报告中标记 NOT_RUN/BLOCKED 并说明替代证据。

### 5.8 Codex 日志格式

追加到 `docs/main-log.md` 时优先使用以下格式：

```markdown
- {yymmdd hhmm} Codex 多代理：任务启动：{任务描述}
- {yymmdd hhmm} Codex 子代理完成：{角色}/{任务} (AGENT_ID: {masked-or-short-id}, MODEL: gpt-5.5)
- {yymmdd hhmm} Codex QA：PASS {n} / FAIL {m} / BLOCKED {k} / NOT_RUN {r} / RISK {s}
- {yymmdd hhmm} Codex 修正：第{round}轮完成，责任角色：{role}
- {yymmdd hhmm} Codex 收尾：文档/同步/验证完成
```

日志不得写入真实密钥、认证票据、Cookie、完整连接串、会话 ID；Agent ID 采用必要最小化或脱敏记录。

## 六、实施步骤

### Step 1: 创建 Agent 规则文件

在项目根目录创建 `.claude/agents/` 目录，定义各子 Agent 的 system prompt。

### Step 2: 更新 CLAUDE.md

在全局 CLAUDE.md 中加入多 Agent 编排规则。

### Step 3: 创建 main-log.md

在 docs/ 下创建工作日志文件。

### Step 4: 验证

用一个小任务验证整个流程（如"给诊断页面加一个刷新按钮"）。
