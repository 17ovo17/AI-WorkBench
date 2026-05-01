#!/bin/bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=== AI WorkBench 生产部署 ==="

echo "[1/4] 安装 systemd 服务..."
sudo cp "$SCRIPT_DIR/systemd/ai-workbench-api.service" /etc/systemd/system/
sudo cp "$SCRIPT_DIR/systemd/ai-workbench-prometheus.service" /etc/systemd/system/
sudo systemctl daemon-reload

echo "[2/4] 构建前端..."
bash "$SCRIPT_DIR/build-web.sh"

echo "[3/4] 配置 nginx..."
sudo cp "$PROJECT_DIR/docker/nginx.conf" /etc/nginx/sites-available/ai-workbench
sudo ln -sf /etc/nginx/sites-available/ai-workbench /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx

echo "[4/4] 启动服务..."
sudo systemctl enable --now ai-workbench-api
sudo systemctl enable --now ai-workbench-prometheus

echo "=== 部署完成 ==="
echo "前端: http://$(hostname -I | awk '{print $1}'):3000"
echo "API:  http://$(hostname -I | awk '{print $1}'):8080"
