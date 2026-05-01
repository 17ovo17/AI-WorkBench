#!/bin/bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json;print(json.load(sys.stdin).get('token',''))")

echo "=== Test auto-adapt with correct datasource_id ==="
DS_ID=$(curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/metrics/mappings?page=1&limit=1&status=unmapped" | python3 -c "import sys,json;items=json.load(sys.stdin).get('items',[]);print(items[0].get('datasource_id','') if items else '')")
echo "Using datasource_id: $DS_ID"

if [ -n "$DS_ID" ]; then
  RESULT=$(curl -s -X POST http://localhost:8080/api/v1/metrics/auto-adapt \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"datasource_id\":\"$DS_ID\",\"max_batches\":1}" \
    --max-time 120)
  echo "Result: $RESULT"

  PROCESSED=$(echo "$RESULT" | python3 -c "import sys,json;print(json.load(sys.stdin).get('processed',0))")
  ADAPTED=$(echo "$RESULT" | python3 -c "import sys,json;print(json.load(sys.stdin).get('adapted',0))")
  echo "Processed: $PROCESSED, Adapted: $ADAPTED"

  if [ "$ADAPTED" -gt 0 ]; then
    echo "SUCCESS: AI auto-adapt is working!"
  else
    echo "STILL_FAILING: processed=$PROCESSED but adapted=0"
  fi
fi

echo ""
echo "=== Check models endpoint ==="
curl -s http://localhost:8080/api/v1/models

echo ""
echo "=== Check oncall config ==="
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/oncall/config | head -c 200

echo ""
echo "=== Check oncall groups ==="
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/oncall/groups | head -c 200
