#!/bin/bash
# 加载 v2 种子数据（12 案例 + 8 Runbook）并触发向量索引同步
# 使用前提：API 服务已启动在 127.0.0.1:8080

set -e

API="${API:-http://127.0.0.1:8080}"
CASES_FILE="${CASES_FILE:-/mnt/d/ai-workbench/api/assets/seed_cases_v2.json}"
RUNBOOKS_FILE="${RUNBOOKS_FILE:-/mnt/d/ai-workbench/api/assets/seed_runbooks_v2.json}"

# 兼容 Windows Git Bash：若 /mnt 路径不存在则回退到 d:/ 路径
if [ ! -f "${CASES_FILE}" ]; then
  CASES_FILE="d:/ai-workbench/api/assets/seed_cases_v2.json"
fi
if [ ! -f "${RUNBOOKS_FILE}" ]; then
  RUNBOOKS_FILE="d:/ai-workbench/api/assets/seed_runbooks_v2.json"
fi

echo "API: ${API}"
echo "Cases: ${CASES_FILE}"
echo "Runbooks: ${RUNBOOKS_FILE}"

if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq 未安装，请先安装 jq"
  exit 1
fi

if [ ! -f "${CASES_FILE}" ]; then
  echo "ERROR: cases 文件不存在: ${CASES_FILE}"
  exit 1
fi
if [ ! -f "${RUNBOOKS_FILE}" ]; then
  echo "ERROR: runbooks 文件不存在: ${RUNBOOKS_FILE}"
  exit 1
fi

echo "=== 加载 12 条新案例 ==="
RESP=$(curl -sS -X POST -H "Content-Type: application/json" \
  -d @"${CASES_FILE}" \
  "${API}/api/v1/knowledge/cases/import")
echo "${RESP}"

echo ""
echo "=== 逐条加载 8 条 Runbook ==="
COUNT=0
FAILED=0
while IFS= read -r rb; do
  RB_ID=$(echo "${rb}" | jq -r '.id')
  HTTP_CODE=$(curl -sS -o /tmp/rb_resp_${RB_ID}.json -w "%{http_code}" \
    -X POST -H "Content-Type: application/json" \
    -d "${rb}" \
    "${API}/api/v1/knowledge/runbooks")
  if [ "${HTTP_CODE}" = "200" ] || [ "${HTTP_CODE}" = "201" ]; then
    echo "  [OK]  ${RB_ID} (${HTTP_CODE})"
    COUNT=$((COUNT + 1))
  else
    echo "  [FAIL] ${RB_ID} (${HTTP_CODE}): $(cat /tmp/rb_resp_${RB_ID}.json)"
    FAILED=$((FAILED + 1))
  fi
done < <(jq -c '.[]' "${RUNBOOKS_FILE}")
echo "已加载 ${COUNT} 条 Runbook，失败 ${FAILED} 条"

echo ""
echo "=== 触发 reindex-all 同步到向量索引 ==="
curl -sS -X POST "${API}/api/v1/knowledge/reindex-all"
echo ""

echo ""
echo "完成"
