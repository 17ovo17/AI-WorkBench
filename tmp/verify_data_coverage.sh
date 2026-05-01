#!/bin/bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json;print(json.load(sys.stdin).get('token',''))")

echo "=== Prometheus data check ==="
curl -s 'http://localhost:9090/api/v1/query?query=cpu_usage_active' | python3 << 'PYEOF'
import sys,json
d=json.load(sys.stdin)
results=d.get("data",{}).get("result",[])
print(f"cpu_usage_active count: {len(results)}")
for r in results:
    m=r.get("metric",{})
    v=r.get("value",["",""])[1]
    print(f"  ident={m.get('ident','')} value={v}%")
PYEOF

echo ""
echo "=== Quick inspection test ==="
SID=$(curl -s -X POST "http://localhost:8080/api/v1/aiops/sessions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"verify-data","mode":"inspection"}' | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('sessionId',''))")

RESP=$(curl -s -X POST "http://localhost:8080/api/v1/aiops/sessions/$SID/messages" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"role":"user","content":"AI WorkBench 运维诊断业务链路 巡检"}' \
  --max-time 120)

CONTENT=$(echo "$RESP" | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('content','')[:3000])")
echo "$CONTENT"

echo ""
echo "=== Check data coverage ==="
echo "$CONTENT" | python3 << 'PYEOF'
import sys
content = sys.stdin.read()
hosts_with_data = 0
hosts_no_data = 0
for line in content.split("\n"):
    if "无数据" in line:
        hosts_no_data += 1
    if "%" in line and ("CPU" in line or "内存" in line or "磁盘" in line):
        hosts_with_data += 1
print(f"Lines with real data: {hosts_with_data}")
print(f"Lines with '无数据': {hosts_no_data}")
PYEOF

# 清理
curl -s -X DELETE "http://localhost:8080/api/v1/chat/sessions/$SID" -H "Authorization: Bearer $TOKEN" > /dev/null 2>&1
