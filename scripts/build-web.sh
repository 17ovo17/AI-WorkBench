#!/bin/bash
set -e
cd "$(dirname "$0")/../web"
echo "=== 构建前端 ==="
npm install --prefer-offline
npm run build
echo "=== 前端构建完成: dist/ ==="
ls -lh dist/index.html
