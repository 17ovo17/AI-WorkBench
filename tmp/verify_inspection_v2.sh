#!/bin/bash
set -e
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json;print(json.load(sys.stdin).get('token',''))")

SID=$(curl -s -X POST "http://localhost:8080/api/v1/aiops/sessions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"verify-inspection-v2","mode":"inspection"}' | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('sessionId',''))")

echo "SESSION=$SID"

curl -s -X POST "http://localhost:8080/api/v1/aiops/sessions/$SID/messages" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"role":"user","content":"请对 AI WorkBench 运维诊断业务链路 做一次全面业务巡检"}' \
  --max-time 120 > /tmp/inspection_v2.json

python3 << 'PYEOF'
import json
d = json.load(open("/tmp/inspection_v2.json"))
data = d.get("data", {})
content = data.get("content", "")
print(f"CONTENT_LENGTH: {len(content)}")
checks = {
    "HAS_SCORE": "健康评分" in content,
    "HAS_TABLE": "| 指标 |" in content or "| 当前值 |" in content,
    "HAS_ABNORMAL": "异常汇总" in content,
    "HAS_DISPOSITION": "处置建议" in content or "ssh" in content,
    "HAS_HISTORY": "历史对比" in content,
    "HAS_HOST_DETAIL": "主机巡检明细" in content,
}
for k, v in checks.items():
    print(f"{k}: {v}")
passed = sum(1 for v in checks.values() if v)
print(f"\nPASS: {passed}/6")
print()
print(content[:2500])
PYEOF

curl -s -X DELETE "http://localhost:8080/api/v1/chat/sessions/$SID" -H "Authorization: Bearer $TOKEN" > /dev/null 2>&1
