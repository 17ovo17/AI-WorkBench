#!/bin/bash
# 切换 mock metrics 场景
# 用法：./scenarios.sh [scenario_name]
# 注意：首次使用前请 `chmod +x scenarios.sh`

SCENARIO="${1:-normal}"
curl -s -X POST "http://127.0.0.1:9101/scenario/$SCENARIO"
echo
