#!/bin/bash
# Quick verify counts after seed load

API="http://127.0.0.1:8080"

echo "=== 总数验证 ==="
echo "cases:     $(curl -s "$API/api/v1/knowledge/cases?limit=1" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("total",0))')"
echo "runbooks:  $(curl -s "$API/api/v1/knowledge/runbooks?limit=1" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("total",0))')"
echo "documents: $(curl -s "$API/api/v1/knowledge/documents?limit=1" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("total",0))')"
echo "workflows: $(curl -s "$API/api/v1/workflows" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("total",0))')"

echo
echo "=== 工作流列表 ==="
curl -s "$API/api/v1/workflows" | python3 -c '
import sys,json
d=json.load(sys.stdin)
for i in d.get("items",[]):
    print("  " + i["name"])
'

echo
echo "=== 测试新案例语义搜索 ==="
for q in "K8s Pod OOM 内存超 limit" "JVM Full GC 老年代" "Redis 内存 LRU 驱逐" "SSL 证书过期"; do
  echo "查询: $q"
  curl -s -X POST "$API/api/v1/knowledge/search" -H "Content-Type: application/json" -d "{\"query\":\"$q\",\"top_k\":2}" | python3 -c '
import sys,json
d=json.load(sys.stdin)
for item in d.get("items",[])[:2]:
    if isinstance(item,dict):
        title=item.get("title",item.get("doc_id",""))
        score=item.get("score",0)
        print(f"  [{score:.3f}] {title[:70]}")
'
  echo
done
