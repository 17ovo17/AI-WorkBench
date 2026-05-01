#!/usr/bin/env bash
set -euo pipefail
APP_ROOT=${AI_WORKBENCH_HOME:-/opt/ai-workbench}
pkill -x api-linux 2>/dev/null || true
pkill -f "vite.*--port 3000" 2>/dev/null || true
pkill -f "npm run dev.*--port 3000" 2>/dev/null || true
pkill -f "prometheus --config.file=$APP_ROOT" 2>/dev/null || true
echo "AI WorkBench WSL app processes stopped (MySQL/Redis kept running)."
