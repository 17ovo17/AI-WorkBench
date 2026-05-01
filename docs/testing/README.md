# AI WorkBench 测试入口

## 统一测试基准（权威文档）

**`AI_WorkBench_统一测试基准.md`** — 整合全部 17 份测试文档，消除重叠，补充缺失领域，形成唯一权威测试基准。

- 26 章 + 4 附录，约 540 个用例
- 覆盖：功能测试（10 章）、安全/UI/白盒/持久化/角色闭环（5 章）、新增领域（11 章）
- 每个用例含：编号、描述、前置条件、操作步骤、预期结果、优先级（P0/P1/P2/P3）
- `1.1 Codex 子代理默认测试基准` 是 `qa-tester` 的默认执行口径，规定 PASS/FAIL/BLOCKED/NOT_RUN/RISK、变更选测路由、证据目录、阻断条件和报告格式。

## Codex 子代理默认口径

- 测试任务默认派发 `qa-tester`，并显式使用 `model: "gpt-5.5"`。
- `qa-tester` 必须先引用 `AI_WorkBench_统一测试基准.md` 的用例编号；新增功能暂无编号时使用 `QA-{层}-{模块}-{序号}` 临时编号；历史 `COD-QA-*` 可识别但不再新增。
- 任何未真实执行的项目只能标记 `NOT_RUN` 或 `BLOCKED`，历史 `NOT_TESTED` 视为 `NOT_RUN`，不能写成 PASS。
- 单条测试记录必须包含 case_id、layer、scenario、priority、status、steps、input、expected、actual、evidence、owner、regression。
- API 证据不能替代 UI/MCP 真实交互；开发代理自测不能替代 QA 回归。
- P0/P1 缺陷、敏感信息泄露、S0 安全预检失败、证据缺失、测试数据未清理均阻断通过。

## 强制前置

任何涉及远程执行、探针安装/卸载、删除、数据库清理、Redis 清理、压测、危险命令的测试，必须先执行安全预检：

```bash
powershell -ExecutionPolicy Bypass -File ./scripts/testing/run_s0_safety_precheck.ps1
```

## 脚本

- `../../scripts/testing/run_s0_safety_precheck.ps1`：S0 安全预检
- `../../scripts/testing/check_sensitive_api_guards.ps1`：敏感 API 守卫回归
- `../../scripts/testing/run_full_smoke.ps1`：基础健康烟测
- `../../scripts/testing/check_storage_health.ps1`：MySQL/Redis 健康检查
- `../../scripts/testing/check_prometheus_categraf.ps1`：Categraf 指标检查
- `../../scripts/testing/check_api_security_inputs.ps1`：API 安全输入检查
- `../../scripts/testing/generate_fake_categraf_future.py`：测试数据生成

## 历史文档（已归档）

以下文档已整合到统一测试基准中，保留作为历史参考：

- `AI_WorkBench_全量穷举测试方案.md` — 原始总方案
- `AI_WorkBench_测试覆盖Checklist.md` — 116 项检查清单
- `AI_WorkBench_安全测试沙箱与自评分规范.md` — 安全白名单与自评分
- `AI_WorkBench_MCP浏览器真实交互基准.md` — MCP 验收原则
- `AI_WorkBench_可点击元素覆盖矩阵.md` — 控件级交互矩阵
- `AI_WorkBench_白盒逻辑全量分析.md` — 代码路径分析
- `AI_WorkBench_逻辑覆盖矩阵.md` — 页面/API 覆盖矩阵
- `AI_WorkBench_全量测试执行基准.md` — 角色闭环基准
- `AI_WorkBench_商业化运维闭环测试基准.md` — 商业化验收基准
- `AI_WorkBench_终极用户闭环测试基准.md` — 用户闭环基准
- `AI_WorkBench_Catpaw_Windows_Linux_闭环矩阵.md` — 探针对等矩阵
- `AI_WorkBench_终极测试完成度报告.md` — 完成度报告
- `AI_WorkBench_测试执行记录.md` — 执行记录模板
- `AI_WorkBench_测试执行记录_20260425-0358.md` — 2026-04-25 执行记录
- `Windows_Catpaw_远程安装最终确认_20260425.md` — Windows 探针确认
- `Prometheus_Categraf_测试数据.md` — 测试数据说明
