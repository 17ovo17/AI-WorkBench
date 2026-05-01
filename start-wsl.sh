#!/usr/bin/env bash
set -euo pipefail
APP_ROOT=${AI_WORKBENCH_HOME:-/opt/ai-workbench}
cd "$APP_ROOT"
mkdir -p logs prometheus-data docker/targets
service mysql start >/dev/null 2>&1 || true
service redis-server start >/dev/null 2>&1 || true
pkill -x api-linux 2>/dev/null || true
pkill -f "vite.*--port 3000" 2>/dev/null || true
pkill -f "npm run dev.*--port 3000" 2>/dev/null || true
pkill -f "prometheus --config.file=$APP_ROOT" 2>/dev/null || true
sleep 1
cd "$APP_ROOT/api"
[ -x ./api-linux ] || go build -o api-linux .
setsid ./api-linux > "$APP_ROOT/logs/api-wsl.log" 2>&1 < /dev/null &
setsid prometheus --config.file="$APP_ROOT/docker/prometheus.yml" --storage.tsdb.path="$APP_ROOT/prometheus-data" --web.listen-address=0.0.0.0:9090 --web.enable-lifecycle > "$APP_ROOT/logs/prometheus-wsl.log" 2>&1 < /dev/null &
cd "$APP_ROOT/web"
[ -d node_modules ] || npm ci
setsid env VITE_API_PROXY=http://localhost:8080 npm run dev -- --host 0.0.0.0 --port 3000 > "$APP_ROOT/logs/web-wsl.log" 2>&1 < /dev/null &
sleep 3
WSL_IP=$(hostname -I | awk '{print $1}')
echo "AI WorkBench WSL started"
echo "Frontend:   http://$WSL_IP:3000"
echo "Backend:    http://$WSL_IP:8080"
echo "Prometheus: http://$WSL_IP:9090"
