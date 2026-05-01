#!/bin/bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json;print(json.load(sys.stdin).get('token',''))")

echo "=== Unmapped metrics ==="
curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/metrics/mappings?page=1&limit=5&status=unmapped" | python3 << 'PYEOF'
import sys,json
d=json.load(sys.stdin)
print(f"total: {d.get('total',0)}")
for i in d.get("items",[])[:5]:
    print(f"  {i.get('raw_name','')} | ds={i.get('datasource_id','')} | status={i.get('status','')}")
PYEOF

echo ""
echo "=== All datasource IDs ==="
curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/metrics/mappings?page=1&limit=1000" | python3 << 'PYEOF'
import sys,json
d=json.load(sys.stdin)
ds_ids = set()
for i in d.get("items",[]):
    ds_ids.add(i.get("datasource_id",""))
print(f"Unique datasource_ids: {ds_ids}")
PYEOF

echo ""
echo "=== Test adapt with correct datasource_id ==="
# 用第一个找到的 datasource_id 重试
DS_ID=$(curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/metrics/mappings?page=1&limit=1&status=unmapped" | python3 -c "import sys,json;items=json.load(sys.stdin).get('items',[]);print(items[0].get('datasource_id','') if items else '')")
echo "Using datasource_id: $DS_ID"
if [ -n "$DS_ID" ]; then
  curl -s -X POST http://localhost:8080/api/v1/metrics/auto-adapt \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"datasource_id\":\"$DS_ID\",\"max_batches\":1}" \
    --max-time 120
  echo ""
fi
