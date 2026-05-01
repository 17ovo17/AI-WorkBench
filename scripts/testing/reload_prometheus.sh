#!/bin/bash
# 同步 prometheus.yml 到 /opt 并 reload
# 注意：首次使用前请 `chmod +x reload_prometheus.sh`

sudo cp /mnt/d/ai-workbench/docker/prometheus.yml /opt/ai-workbench/docker/prometheus.yml 2>/dev/null
curl -s -X POST http://127.0.0.1:9090/-/reload
echo "Prometheus reloaded"
