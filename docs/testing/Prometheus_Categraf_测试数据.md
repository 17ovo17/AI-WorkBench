# Prometheus Categraf 测试数据刷新说明

`scripts/testing/generate_fake_categraf_future.py` 用于生成未来 24h、15s 采集频率的 Categraf 风格 OpenMetrics 数据，供 AI 问诊、业务拓扑和诊断报告回归测试使用。

## 覆盖主机

- `198.18.20.11`、`198.18.20.12`：应用服务器，包含基础资源、JVM、HTTP `8081`、进程指标。
- `198.18.20.20`：Nginx + Redis，包含基础资源、Nginx、Redis、进程指标。
- `198.18.22.11`、`198.18.22.12`、`198.18.22.13`：Oracle 数据库，包含基础资源、Oracle、进程指标。

## 数据要求

- 标签必须至少包含 `job="categraf"`、`instance="<ip>"`、`ident`、`hostname`、`role`。
- 基础资源必须覆盖 CPU、内存、磁盘、磁盘 IO、网络带宽、丢包、错误包、TCP、负载、进程。
- 业务指标必须覆盖 JVM、Nginx、Redis、Oracle 的默认测试项。
- target 离线只代表健康异常，不代表没有可用于测试的历史/未来时序。

## WSL 刷新步骤

```bash
cd /opt/ai-workbench
python3 scripts/testing/generate_fake_categraf_future.py
./stop-wsl.sh || true
./start-wsl.sh
```

## 验证命令

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\testing\check_prometheus_categraf.ps1
```

关键 PromQL：

```promql
count(count by(instance) (cpu_usage_active{job="categraf"}))
count_over_time(cpu_usage_active{instance="198.18.20.11"}[5m])
count_over_time(net_bits_recv{instance="198.18.20.20"}[5m])
count_over_time(oracle_up{instance="198.18.22.11"}[5m])
```
