#!/bin/bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json;print(json.load(sys.stdin).get('token',''))")
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/topology/businesses | python3 -c "
import sys,json
for b in json.load(sys.stdin):
    print(b.get('id','?'), '|', b.get('name','?'))
"
