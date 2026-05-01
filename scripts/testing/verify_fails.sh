#!/bin/bash
# 验证 4 个 FAIL 项的实际状态

API="http://127.0.0.1:8080"

echo "=== 告警实际入库 ==="
curl -s "$API/api/v1/alerts?limit=20" | python3 -c '
import sys,json
d=json.load(sys.stdin)
items=d if isinstance(d,list) else d.get("items",[])
print(f"告警数: {len(items)}")
for i in items[:5]:
    print(f"  - {i.get(\"title\",\"\")[:30]} severity={i.get(\"severity\")} status={i.get(\"status\")}")
'

echo
echo "=== 告警触发的诊断 ==="
curl -s "$API/api/v1/diagnose?limit=30" | python3 -c '
import sys,json
d=json.load(sys.stdin)
items=d if isinstance(d,list) else d.get("items",[])
alert_diag=[i for i in items if i.get("trigger")=="alert"]
print(f"告警触发的诊断: {len(alert_diag)}")
for i in alert_diag[:3]:
    print(f"  - ID={str(i.get(\"id\",\"\"))[:25]} alert_title={i.get(\"alert_title\",\"\")[:30]} status={i.get(\"status\")}")
'

echo
echo "=== 工作流归档诊断 ==="
curl -s "$API/api/v1/diagnose?limit=30" | python3 -c '
import sys,json
d=json.load(sys.stdin)
items=d if isinstance(d,list) else d.get("items",[])
wf=[i for i in items if str(i.get("id","")).startswith("wf_") or i.get("source")=="workflow"]
print(f"工作流归档诊断: {len(wf)}")
for i in wf[:5]:
    print(f"  - ID={str(i.get(\"id\",\"\"))[:30]} target={i.get(\"target_ip\",\"\")} status={i.get(\"status\")}")
'

echo
echo "=== 磁盘空间满 搜索 ==="
curl -s -X POST "$API/api/v1/knowledge/search" -H "Content-Type: application/json" -d '{"query":"磁盘空间不足","top_k":3}' | python3 -c '
import sys,json
d=json.load(sys.stdin)
print(f"engine={d.get(\"engine\")} total={d.get(\"total\",0)}")
for item in d.get("items",[])[:3]:
    if isinstance(item,dict):
        print(f"  - {item.get(\"title\",item.get(\"doc_id\",\"\"))[:70]}")
'

echo
echo "=== 诊断记录总数 ==="
curl -s "$API/api/v1/diagnose?limit=200" | python3 -c '
import sys,json
d=json.load(sys.stdin)
items=d if isinstance(d,list) else d.get("items",[])
print(f"总数: {len(items)}")
by_trigger={}
for i in items:
    t=i.get("trigger","unknown")
    by_trigger[t]=by_trigger.get(t,0)+1
for k,v in by_trigger.items():
    print(f"  {k}: {v}")
'
