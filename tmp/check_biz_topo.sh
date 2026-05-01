#!/bin/bash
set -e
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json;print(json.load(sys.stdin).get('token',''))")
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/topology/businesses > /tmp/biz_topo.json
python3 << 'PYEOF'
import json
d = json.load(open("/tmp/biz_topo.json"))
for biz in d:
    g = biz.get("graph", {})
    nodes = g.get("nodes", [])
    edges = g.get("edges", [])
    print(f"BIZ: {biz.get('name','?')} nodes={len(nodes)} edges={len(edges)}")
    nids = set(n["id"] for n in nodes)
    for n in nodes[:4]:
        print(f"  NODE {n['id']} type={n.get('type','')} layer={n.get('layer','')}")
    for e in edges[:8]:
        s = e.get("source_id", "")
        t = e.get("target_id", "")
        print(f"  EDGE {s}({'OK' if s in nids else 'MISS'}) -> {t}({'OK' if t in nids else 'MISS'}) {e.get('protocol','')}")
PYEOF
