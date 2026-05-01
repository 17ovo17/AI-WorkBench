# AI WorkBench 安全测试沙箱与自评分规范

本文档是全量穷举测试的强制前置规范。任何涉及远程执行、探针安装/卸载、删除、数据库清理、Redis 清理、压测、危险命令、Catpaw 工具调度的测试，必须先通过 S0 安全预检。

## 1. 白名单边界

- Windows 远程探针目标：`192.168.1.7`，仅允许影响 `C:\catpaw`、`C:\catpaw-test-sandbox`、计划任务 `Catpaw`、Catpaw 进程与测试配置。
- Linux 目标：WSL Ubuntu，仅允许影响 `/usr/local/bin/catpaw`、`/etc/catpaw`、`/var/log/catpaw*`、`/tmp/aiw-danger-sandbox` 与测试用服务。
- MySQL：只允许操作 `ai_workbench` 内带 `test_batch_id` 的测试数据，或临时库 `aiw_test_<batch>`。
- Redis：只允许测试 DB 或 `aiw:test:<batch>:` 前缀 key。
- 诊断、聊天、告警、拓扑、凭证等测试数据必须带测试批次 ID，清理时只删除本批次创建的数据。

## 2. 永久禁止真实执行

- `rm -rf /`、`rm -rf /*`、`del C:\`、`rd /s /q C:\`、`format`、`mkfs`。
- `DROP DATABASE ai_workbench`、`FLUSHALL`。
- 关闭防火墙、清空系统目录、删除用户目录、破坏路由或网络配置。
- L4 禁止命令即使用户确认也不得执行，只能验证拦截结果。

## 3. 分级确认

- L0：只读或无副作用命令，可执行。
- L1/L2：可能产生轻微副作用或超长输出，必须有明确提示。
- L3：安装、卸载、停止进程、递归删除、远程执行高风险命令，必须二次确认并回传 `safety_confirm`。
- L4：灾难性破坏命令，服务端直接拒绝。

## 4. 执行命令

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\testing\run_s0_safety_precheck.ps1
```

敏感 API 守卫回归：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\testing\check_sensitive_api_guards.ps1 -Backend http://localhost:8080
```

## 5. 证据要求

每批测试必须写入 `.test-evidence/<timestamp>/<batch-name>/`，至少包含：

- `batch-summary.md`
- `self-score.json`
- `safety-impact.md`
- `coverage-matrix.csv`
- `defects.md`
- `fixed-regression.md`
- `created-ids.json`
- `pre-snapshot.json`
- `post-snapshot.json`
- `teardown-result.json`

MCP 浏览器测试还必须保存页面快照或截图；API/Prometheus/DB/Redis/Catpaw 测试必须保存请求、响应、PromQL、日志或命令输出。

## 6. 自评分规则

- 100：安全预检、白名单、证据、teardown 全部通过。
- 90-99：存在非阻断缺口，但无 P0/P1，无白名单外副作用。
- 80-89：需补测，不允许进入最终通过结论。
- 低于 80：该批重做。
- 任一白名单外改动、误删、明文凭证泄露、危险命令真实执行：直接 P0，批次 0 分。

## 7. Catpaw 全量覆盖补充

Catpaw 测试必须覆盖插件、命令、通知、AI 工具、平台联动五个维度：

- 命令：`run`、`inspect`、`chat`、`diagnose list/show`、`selftest`。
- 通知：console、webapi、Flashduty、PagerDuty，多通道失败互不阻塞。
- 插件：基础资源、网络、日志、systemd、Docker、Redis、Redis Sentinel、etcd、证书、HTTP、文件、exec/scriptfilter、安全基线。
- AI 工具：系统/进程、网络、存储、内核安全、日志、服务、Redis/Sentinel 专用诊断工具。
- 平台联动：智能对话、智能诊断、探针对话、探针管理都必须受统一安全边界保护。

## 8. MCP 浏览器强制项

- 所有页面都必须用 Playwright MCP 真实点击，不允许只做接口烟测。
- 每个高危按钮至少覆盖：正常确认、取消确认、异常输入/边界输入。
- 乱码、暗色不可读、下载文件乱码、`�` 或 mojibake 一律记录缺陷。
- 每次智能诊断测试保存证据后，必须删除本批次创建的诊断记录。
