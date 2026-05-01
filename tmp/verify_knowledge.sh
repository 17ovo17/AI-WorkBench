#!/bin/bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json;print(json.load(sys.stdin).get('token',''))")

echo "=== Search: CPU ==="
curl -s -X POST http://localhost:8080/api/v1/knowledge/search \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query":"CPU","top_k":3}' | python3 << 'PYEOF'
import sys,json
d=json.load(sys.stdin)
items=d.get("items",d.get("results",[]))
print(f"Results: {len(items)}, engine: {d.get('engine','?')}")
for i,item in enumerate(items[:3]):
    print(f"  [{i+1}] score={item.get('score',0):.3f} title={item.get('title','')[:60]} chunk={item.get('chunk_index','')} ctx={len(item.get('context_chunks',[]))}")
PYEOF

echo ""
echo "=== Search: Prometheus ==="
curl -s -X POST http://localhost:8080/api/v1/knowledge/search \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query":"Prometheus 指标缺失","top_k":3}' | python3 << 'PYEOF'
import sys,json
d=json.load(sys.stdin)
items=d.get("items",d.get("results",[]))
print(f"Results: {len(items)}")
for i,item in enumerate(items[:3]):
    print(f"  [{i+1}] score={item.get('score',0):.3f} title={item.get('title','')[:60]}")
PYEOF

echo ""
echo "=== Search stats ==="
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/knowledge/search/stats | python3 << 'PYEOF'
import sys,json
d=json.load(sys.stdin)
print(f"search_count={d.get('search_count',0)} hit_rate={d.get('hit_rate',0)} avg_score={d.get('average_score',0):.3f} badcase={d.get('badcase_count',0)}")
top=d.get("top_queries",[])
if top:
    print(f"Top queries: {', '.join(q.get('query','') for q in top[:5])}")
PYEOF
