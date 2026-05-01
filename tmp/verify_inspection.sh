#!/bin/bash
set -e
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json;print(json.load(sys.stdin).get('token',''))")

SID=$(curl -s -X POST "http://localhost:8080/api/v1/aiops/sessions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"verify-inspection","mode":"inspection"}' | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('sessionId',''))")

echo "SESSION=$SID"

curl -s -X POST "http://localhost:8080/api/v1/aiops/sessions/$SID/messages" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"role":"user","content":"AI WorkBench 巡检"}' \
  --max-time 120 > /tmp/inspection_msg.json

python3 << 'PYEOF'
import json
d = json.load(open("/tmp/inspection_msg.json"))
data = d.get("data", {})
content = data.get("content", "")
print(f"CONTENT_LENGTH: {len(content)}")
print(f"HAS_TABLE: {'| 指标 |' in content or '| 当前值 |' in content}")
print(f"HAS_SCORE: {'健康评分' in content}")
print(f"HAS_DISPOSITION: {'处置建议' in content or 'ssh' in content}")
print(f"HAS_ABNORMAL: {'异常汇总' in content}")
print(f"HAS_HISTORY: {'历史对比' in content}")
print()
print(content[:2000])
PYEOF

# 清理
curl -s -X DELETE "http://localhost:8080/api/v1/chat/sessions/$SID" -H "Authorization: Bearer $TOKEN" > /dev/null 2>&1
