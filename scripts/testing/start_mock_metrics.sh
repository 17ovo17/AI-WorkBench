#!/bin/bash
# 后台启动 mock metrics exporter
# 注意：本脚本需要在 WSL/Linux 下执行；首次使用前请 `chmod +x start_mock_metrics.sh`

set -u

cd /mnt/d/ai-workbench/scripts/testing

# 检查端口是否已被占用
if lsof -i :9101 > /dev/null 2>&1; then
    echo "Port 9101 already in use, killing..."
    lsof -ti :9101 | xargs kill -9 2>/dev/null
    sleep 1
fi

# 后台启动
nohup go run mock_metrics_exporter.go > /tmp/mock_metrics.log 2>&1 &
sleep 2

# 验证
if curl -sf http://127.0.0.1:9101/metrics > /dev/null; then
    echo "Mock metrics exporter started on :9101"
    echo "Available scenarios: normal, cpu_high, memory_leak, disk_full, oom, slow_sql, network_drop, connection_pool, io_saturation, redis_memory, nginx_502, jvm_gc"
else
    echo "Failed to start mock metrics exporter"
    cat /tmp/mock_metrics.log
    exit 1
fi
