#!/bin/bash
echo "=== up metrics ==="
curl -s 'http://localhost:9090/api/v1/query?query=up' | python3 << 'PYEOF'
import sys,json
d=json.load(sys.stdin)
for r in d.get("data",{}).get("result",[]):
    m=r.get("metric",{})
    v=r.get("value",["",""])[1]
    print(f"  job={m.get('job','')} instance={m.get('instance','')} ident={m.get('ident','')} target_ip={m.get('target_ip','')} up={v}")
PYEOF

echo ""
echo "=== all metric names for ident=10.10.1.21 ==="
curl -s 'http://localhost:9090/api/v1/query?query={ident="10.10.1.21"}' | python3 << 'PYEOF'
import sys,json
d=json.load(sys.stdin)
names=set()
for r in d.get("data",{}).get("result",[]):
    names.add(r.get("metric",{}).get("__name__",""))
print(f"  metrics count: {len(names)}")
for n in sorted(names)[:20]:
    print(f"  {n}")
PYEOF

echo ""
echo "=== all metric names for target_ip=10.10.1.21 ==="
curl -s 'http://localhost:9090/api/v1/query?query={target_ip="10.10.1.21"}' | python3 << 'PYEOF'
import sys,json
d=json.load(sys.stdin)
names=set()
for r in d.get("data",{}).get("result",[]):
    names.add(r.get("metric",{}).get("__name__",""))
print(f"  metrics count: {len(names)}")
for n in sorted(names)[:20]:
    print(f"  {n}")
PYEOF
