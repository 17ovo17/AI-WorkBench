#!/bin/bash
echo "=== Prometheus targets ==="
curl -s 'http://localhost:9090/api/v1/targets' | python3 -c "
import sys,json
d=json.load(sys.stdin)
for t in d.get('data',{}).get('activeTargets',[]):
    labels=t.get('labels',{})
    print(f\"  {labels.get('job','')} {labels.get('instance','')} ident={labels.get('ident','')} health={t.get('health','')}\")
"

echo ""
echo "=== cpu_usage_active labels ==="
curl -s 'http://localhost:9090/api/v1/query?query=cpu_usage_active' | python3 -c "
import sys,json
d=json.load(sys.stdin)
for r in d.get('data',{}).get('result',[]):
    m=r.get('metric',{})
    v=r.get('value',['',''])[1]
    print(f\"  ident={m.get('ident','')} instance={m.get('instance','')} job={m.get('job','')} value={v}\")
"
