# Windows 远程 Catpaw 探针最终确认记录 - 2026-04-25 10:55

## 目标

- 目标主机：`192.168.1.7`
- 系统：Windows 10 专业版，主机名 `DESKTOP-7QO0TFK`
- 认证：平台凭证管理，`credential_id=1777083904425519060`，页面/API 返回密码脱敏
- 可用协议：WMI/DCOM + SMB 管理共享；WinRM `5985/5986` 未开放，RDP `3389` 可达
- 平台回调：`http://192.168.1.6:18080` 用户态 HTTP 代理到后端 `localhost:8080`

## 执行结果

| 阶段 | 结果 | 证据 |
| --- | --- | --- |
| 连通性 | `135/445/3389` 可达，`22/5985/5986` 不可达 | PowerShell `Test-NetConnection` |
| 凭证管理 | 已创建并脱敏展示 | `/api/v1/credentials` 返回 `password=******` |
| 远程投放 | `C:\catpaw\catpaw.exe` 与 `conf.d\config.toml` 成功写入 | SMB 管理共享检查 |
| 远程启动 | `catpaw.exe run --configs C:\catpaw\conf.d` 已运行 | 远程进程 `ProcessId=4464` |
| 心跳 | 平台显示 `192.168.1.7` 在线 | `/api/v1/catpaw/agents` |
| 巡检报告 | Windows 巡检摘要已入库且包含 CPU/内存/磁盘/网络 | `/api/v1/diagnose` 最新 `target_ip=192.168.1.7` |
| 卸载 | 卸载后无 `catpaw.exe` 进程，`C:\catpaw` 被删除 | WMI + SMB 检查 |
| 重装 | 卸载后重新安装/启动成功，最终在线且有新报告 | `/api/v1/catpaw/agents` 与 `/api/v1/diagnose` |

## 最新平台状态

- Agent：`192.168.1.7`，`online=true`
- Hostname：`DESKTOP-7QO0TFK`
- Version：`windows-local-compat-1.0`
- Last seen：`2026-04-25T10:53:19+08:00`
- 最新报告 ID：`1777085419382438923`
- 最新报告结论：未发现明显高危风险
- 关键值：CPU `1%`，内存 `41.13%`，磁盘最高使用率 `39.49%`，网络收/发字节 `383000639 / 17525059`

## 修复/改进

- 修复后端 WMI 安装脚本：改为写 `start-catpaw.bat` 并通过批处理启动，避免计划任务直接运行长命令失败。
- 验证 WinRM 未开时可通过 WMI/DCOM + SMB 完成 Windows 探针生命周期闭环。
- 保持 Catpaw AI 通过平台 `/api/v1/agent/llm` 网关，不在探针配置中写明文 API Key。

## 注意事项

- 当前 `192.168.1.6:18080` 是本轮测试用的用户态 HTTP 代理；若机器重启，需要重新启动该代理或改用管理员权限配置 `netsh portproxy`。
- 平台一键 WMI 安装脚本已修复并构建通过，但本轮最终重装采用等价的 WMI + SMB 实操方式完成，以保证目标机最终在线。
