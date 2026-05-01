#!/bin/bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json;print(json.load(sys.stdin).get('token',''))")

echo "Testing auto-adapt..."
curl -s -X POST http://localhost:8080/api/v1/metrics/auto-adapt \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"datasource_id":"new","max_batches":1}' \
  --max-time 120

echo ""
echo "Done"
