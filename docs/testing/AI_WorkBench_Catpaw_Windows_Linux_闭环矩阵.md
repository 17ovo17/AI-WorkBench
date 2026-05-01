# Catpaw Windows/Linux 闭环矩阵

Catpaw 是平台主 Agent 调度的子 Agent。Windows 与 Linux 必须对等测试，差异只能来自 OS 能力，不允许只测一种系统后推断另一种通过。

| 能力 | Windows `192.168.1.7` | Linux WSL Ubuntu | 验收 |
| --- | --- | --- | --- |
| 安装 | WinRM/WMI 安装到 `C:\catpaw` | SSH/本地安装到白名单路径 | 只影响白名单，重复安装幂等 |
| 运行 | 计划任务/进程 heartbeat | run 模式 heartbeat | UI 在线，Redis 状态有 TTL |
| 巡检 | CPU、内存、磁盘、网络、服务、进程、事件日志 | CPU、内存、磁盘、IO、网络、TCP、进程、日志 | 报告可读、无乱码、可折叠原始数据 |
| selftest | Windows 插件可用性 | Linux 插件可用性 | PASS/SKIP/BLOCKED 明确，不假通过 |
| diagnose | list/show/report | list/show/report | 报告入库，可下载，可删除 |
| chat | 探针对话连接/发送/断开 | 探针对话连接/发送/断开 | 工具反馈被 AI 捕捉，危险命令确认 |
| 事件 | heartbeat/alert/report | heartbeat/alert/report | 进入告警、诊断、聊天上下文 |
| 安全 | PowerShell 危险命令拦截 | shell 危险命令拦截 | L4 永拒，L3 确认，审计完整 |
| 卸载 | 删除 `C:\catpaw`、任务、进程 | 删除白名单测试资源 | 无残留、无误删、可重装 |
| 重装 | 卸载后恢复在线 | 卸载后恢复在线 | 状态机完整 |

## Linux 必测工具类别

- 系统：CPU Top、内存分布、OOM、cgroup、PSI、进程线程、wchan、fd、env 脱敏。
- 网络：ping、traceroute、DNS、ARP、TCP 状态、socket RTT/cwnd、重传、listen 队列、softnet、路由、防火墙。
- 存储：磁盘 I/O、块设备拓扑、LVM、挂载信息。
- 内核安全：dmesg、中断、conntrack、NUMA、thermal、sysctl、SELinux/AppArmor、coredump。
- 日志服务：tail、grep、journald、systemd 状态、失败服务、timer、Docker ps/inspect。
- 中间件：Redis、Redis Sentinel、HTTP、证书、NTP、DNS。

## Windows 必测专项

- PowerShell 输出必须 UTF-8 可读。
- Windows Event Log 中 `/Date(...)\/` 必须转 ISO 时间。
- Security-SPP、Defrag 等常见事件必须解释为可读中文或英文，不允许 `����`。
- 自动启动但未运行服务必须解释风险，不等于探针失败。

## 2026-04-25 23:56 Catpaw Linux、卸载语义与 AI 业务巡检回归

- 修复 198.18.20.11 Linux 探针卸载误显示 Windows `C:\catpaw` 的问题：前端根据探针 OS/hostname/version 推断 Linux/Windows，Linux 只展示 `/usr/local/bin/catpaw`、`/etc/catpaw`、`/var/log/catpaw*` 白名单路径。
- 卸载二次确认已中文化，确认内容展示目标、风险、白名单范围、后端二次校验说明。
- 探针管理新增“删除主机”动作：离线或废弃主机可仅删除平台记录，不强制远程卸载，避免主机不在线时占位无法清理。
- 本机 WSL Linux 已真实构建并安装 Catpaw 到 `/usr/local/bin/catpaw`，执行 `catpaw selftest cpu -q` 通过 2/2，并以 `198.18.20.11 whitebox-linux-wsl` heartbeat 注册到平台。
- 业务巡检新增 `ai_suggestions`，建议不再只是告警列表；结合业务链路完整性、资源指标、进程/端口、Redis/Oracle/JVM/Nginx、告警影响和观测一致性给出下一步诊断建议。
- 智能对话新增业务巡检调度：用户说“帮我巡检一下 clims 业务”时，平台主 Agent 直接调用业务巡检工具链，并返回业务评分、数据源、AI 建议和关键发现；不会再只透传到外部模型。
- 回归：`go test ./...` 通过；`npm run build` 通过；WSL 内 `run_user_journey_regression.py --base-url http://127.0.0.1:8080` 通过 19/19，评分 100。
- 环境说明：Windows 侧 Playwright 本轮因 WSL 后端端口映射/可达性超时未完成，不标记通过；功能链路已用 WSL 本地 API 和用户旅程脚本验证。