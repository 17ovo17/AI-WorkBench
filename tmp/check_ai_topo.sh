#!/bin/bash
set -e
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json;print(json.load(sys.stdin).get('token',''))")

curl -s -X POST http://localhost:8080/api/v1/aiops/topology/generate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"business_id":"biz-ai-workbench","service_name":"AI WorkBench","hosts":["10.10.1.11","10.10.1.21","10.10.1.22"],"endpoints":[]}' \
  --max-time 30 > /tmp/ai_topo.json 2>&1

python3 << 'PYEOF'
import json
try:
    d = json.load(open("/tmp/ai_topo.json"))
except:
    print("PARSE_ERROR:", open("/tmp/ai_topo.json").read()[:500])
    exit(1)
print("keys:", list(d.keys()) if isinstance(d, dict) else type(d))
if isinstance(d, dict):
    print("nodes:", len(d.get("nodes", [])))
    print("links:", len(d.get("links", [])))
    print("edges:", len(d.get("edges", [])))
    for l in (d.get("links", []) or d.get("edges", []))[:3]:
        print("  link:", l)
PYEOF
