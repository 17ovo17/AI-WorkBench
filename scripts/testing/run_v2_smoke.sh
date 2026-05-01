#!/bin/bash
# AI WorkBench v2 全量穷举烟测
# 输出: .test-evidence/v2-{timestamp}/

set +e

TS=$(date +%Y%m%d-%H%M%S)
EVIDENCE_DIR="/mnt/d/ai-workbench/.test-evidence/v2-$TS"
mkdir -p "$EVIDENCE_DIR"

API="http://127.0.0.1:8080"
MOCK="http://127.0.0.1:9101"
PROM="http://127.0.0.1:9090"
WEB="http://172.20.32.65:3000"

PASS=0
FAIL=0

run_check() {
    local name="$1"
    local cmd="$2"
    local expect="$3"
    local result
    result=$(eval "$cmd" 2>&1)
    if echo "$result" | grep -q "$expect"; then
        PASS=$((PASS+1))
        echo "[PASS] $name"
    else
        FAIL=$((FAIL+1))
        echo "[FAIL] $name (expect '$expect', got: $(echo "$result" | head -1))"
    fi
}

echo "================================================" | tee "$EVIDENCE_DIR/00_summary.log"
echo "AI WorkBench v2 穷举烟测 - $TS" | tee -a "$EVIDENCE_DIR/00_summary.log"
echo "================================================" | tee -a "$EVIDENCE_DIR/00_summary.log"

# ====================
# Phase 1: 基础设施健康检查
# ====================
echo -e "\n[Phase 1] 基础设施健康检查" | tee -a "$EVIDENCE_DIR/00_summary.log"
{
    echo "=== API health ==="
    curl -s "$API/api/v1/health/storage"
    echo
    echo "=== Mock metrics exporter ==="
    curl -sI "$MOCK/metrics" | head -1
    echo "=== Prometheus ==="
    curl -sI "$PROM/-/ready" | head -1
    echo "=== Web ==="
    curl -sI "$WEB" | head -1
} > "$EVIDENCE_DIR/01_setup.txt" 2>&1

run_check "API 健康" "curl -s $API/api/v1/health/storage" '"mysql":true'
run_check "Mock exporter 可达" "curl -s $MOCK/metrics | head -3" "container_memory"
run_check "Prometheus 就绪" "curl -s $PROM/-/ready" "Ready"
run_check "Web 可达" "curl -sI $WEB" "HTTP/1.1 200"

# ====================
# Phase 2: Embedding/Reranker 配置
# ====================
echo -e "\n[Phase 2] Embedding/Reranker 配置" | tee -a "$EVIDENCE_DIR/00_summary.log"
curl -s "$API/api/v1/settings/embedding" > "$EVIDENCE_DIR/02_embedding_config.json"
curl -s "$API/api/v1/settings/reranker" > "$EVIDENCE_DIR/02_reranker_config.json"

run_check "Embedding 配置存在" "cat $EVIDENCE_DIR/02_embedding_config.json" "provider"
run_check "Reranker 配置存在" "cat $EVIDENCE_DIR/02_reranker_config.json" "provider"

# ====================
# Phase 3: 知识库总数
# ====================
echo -e "\n[Phase 3] 知识库基线" | tee -a "$EVIDENCE_DIR/00_summary.log"
{
    echo "cases:     $(curl -s "$API/api/v1/knowledge/cases?limit=1" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("total",0))')"
    echo "runbooks:  $(curl -s "$API/api/v1/knowledge/runbooks?limit=1" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("total",0))')"
    echo "documents: $(curl -s "$API/api/v1/knowledge/documents?limit=1" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("total",0))')"
    echo "workflows: $(curl -s "$API/api/v1/workflows" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("total",0))')"
} | tee "$EVIDENCE_DIR/03_baselines.txt"

CASES=$(curl -s "$API/api/v1/knowledge/cases?limit=1" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("total",0))')
RUNBOOKS=$(curl -s "$API/api/v1/knowledge/runbooks?limit=1" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("total",0))')
WORKFLOWS=$(curl -s "$API/api/v1/workflows" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("total",0))')

[ "$CASES" -ge 30 ] && { PASS=$((PASS+1)); echo "[PASS] 案例库 ≥ 30 ($CASES)"; } || { FAIL=$((FAIL+1)); echo "[FAIL] 案例库 ($CASES)"; }
[ "$RUNBOOKS" -ge 18 ] && { PASS=$((PASS+1)); echo "[PASS] Runbook ≥ 18 ($RUNBOOKS)"; } || { FAIL=$((FAIL+1)); echo "[FAIL] Runbook ($RUNBOOKS)"; }
[ "$WORKFLOWS" -ge 18 ] && { PASS=$((PASS+1)); echo "[PASS] 工作流 ≥ 18 ($WORKFLOWS)"; } || { FAIL=$((FAIL+1)); echo "[FAIL] 工作流 ($WORKFLOWS)"; }

# ====================
# Phase 4: 12 个核心工作流执行（blocking 模式）
# ====================
echo -e "\n[Phase 4] 工作流全量调用（blocking）" | tee -a "$EVIDENCE_DIR/00_summary.log"

call_workflow() {
    local wf="$1"
    local inputs="$2"
    local out="$EVIDENCE_DIR/04_wf_${wf}.json"
    timeout 90 curl -s -X POST "$API/api/v1/workflows/builtin:${wf}/run" \
        -H "Content-Type: application/json" \
        -d "{\"inputs\":${inputs}}" > "$out" 2>&1
    local status=$(python3 -c "import sys,json; d=json.load(open('$out')); print(d.get('status','FAIL'))" 2>/dev/null)
    if [ "$status" = "succeeded" ]; then
        PASS=$((PASS+1))
        echo "[PASS] 工作流 $wf"
    else
        FAIL=$((FAIL+1))
        echo "[FAIL] 工作流 $wf (status=$status)"
    fi
}

# 切换到 cpu_high 场景
curl -s -X POST "$MOCK/scenario/cpu_high" > /dev/null

call_workflow "diagnosis" '{"hostname":"172.20.32.65","time_range":"1h","user_question":"CPU使用率高"}'
call_workflow "metrics_analysis" '{"hostname":"172.20.32.65","time_range":"6h","focus_area":"CPU"}'
call_workflow "capacity_forecast" '{"hostname":"172.20.32.65","forecast_days":"7"}'
call_workflow "log_analysis" '{"hostname":"172.20.32.65","time_range":"1h","log_source":"application"}'
call_workflow "container_diagnosis" '{"pod_name":"test-pod","namespace":"default","time_range":"30m"}'
call_workflow "jvm_diagnosis" '{"hostname":"172.20.32.65","app_name":"order-service","time_range":"1h"}'
call_workflow "ssl_audit" '{"domains":"example.com,api.example.com"}'
call_workflow "dependency_health" '{"business_id":"biz-001"}'
call_workflow "security_audit" '{"time_range":"24h","min_risk":"medium"}'
call_workflow "incident_timeline" '{"hostname":"172.20.32.65","time_range":"24h"}'
call_workflow "slow_query_diagnosis" '{"hostname":"172.20.32.65","threshold_ms":1000,"limit":20}'
call_workflow "network_check" '{"target_ips":"172.20.32.65,172.20.32.66"}'

# ====================
# Phase 5: 告警闭环（5 个场景）
# ====================
echo -e "\n[Phase 5] 告警 → 自动诊断闭环" | tee -a "$EVIDENCE_DIR/00_summary.log"

send_alert() {
    local title="$1"
    local severity="$2"
    local ip="$3"
    local payload="{\"title\":\"$title\",\"severity\":\"$severity\",\"status\":\"firing\",\"labels\":{\"from_hostip\":\"$ip\",\"instance\":\"$ip:9100\"},\"annotations\":{\"description\":\"测试告警 $title\"}}"
    curl -s -X POST "$API/api/v1/alert/catpaw" -H "Content-Type: application/json" -d "$payload" > "$EVIDENCE_DIR/05_alert_${title//[^a-zA-Z0-9]/_}.json" 2>&1
}

send_alert "CPU使用率超90%" "critical" "172.20.32.65"
send_alert "内存OOM Kill触发" "critical" "172.20.32.66"
send_alert "磁盘使用率超95%" "warning" "172.20.32.65"
send_alert "MySQL慢查询飙升" "warning" "172.20.32.66"
send_alert "网络丢包率超5%" "info" "172.20.32.65"

sleep 3
ALERT_COUNT=$(curl -s "$API/api/v1/alerts?limit=10" | python3 -c 'import sys,json; print(len(json.load(sys.stdin).get("items",[])))' 2>/dev/null)
[ "$ALERT_COUNT" -ge 5 ] && { PASS=$((PASS+1)); echo "[PASS] 告警入库 ($ALERT_COUNT 条)"; } || { FAIL=$((FAIL+1)); echo "[FAIL] 告警入库 ($ALERT_COUNT)"; }

# 等待自动诊断完成
sleep 30
DIAG_COUNT=$(curl -s "$API/api/v1/diagnose?limit=10" | python3 -c 'import sys,json; d=json.load(sys.stdin); items=d.get("items",d if isinstance(d,list) else []); print(len([i for i in items if i.get("trigger")=="alert"]))' 2>/dev/null)
[ "$DIAG_COUNT" -ge 1 ] && { PASS=$((PASS+1)); echo "[PASS] 告警触发诊断 ($DIAG_COUNT)"; } || { FAIL=$((FAIL+1)); echo "[FAIL] 告警触发诊断 ($DIAG_COUNT)"; }

# ====================
# Phase 6: 知识库语义搜索
# ====================
echo -e "\n[Phase 6] 知识库语义搜索" | tee -a "$EVIDENCE_DIR/00_summary.log"

search_test() {
    local q="$1"
    local out="$EVIDENCE_DIR/06_search_${q//[^a-zA-Z0-9]/_}.json"
    curl -s -X POST "$API/api/v1/knowledge/search" -H "Content-Type: application/json" -d "{\"query\":\"$q\",\"top_k\":3}" > "$out"
    local total=$(python3 -c "import sys,json; print(json.load(open('$out')).get('total',0))" 2>/dev/null)
    [ "$total" -ge 1 ] && { PASS=$((PASS+1)); echo "[PASS] 搜索 '$q' ($total 条)"; } || { FAIL=$((FAIL+1)); echo "[FAIL] 搜索 '$q'"; }
}

search_test "CPU 使用率高"
search_test "内存 OOM"
search_test "磁盘空间满"
search_test "Redis 内存"
search_test "JVM Full GC"
search_test "K8s Pod"
search_test "SSL 证书"
search_test "MySQL 慢查询"

# ====================
# Phase 7: 指标 AI 适配
# ====================
echo -e "\n[Phase 7] 指标扫描 + AI 适配" | tee -a "$EVIDENCE_DIR/00_summary.log"
DS=$(curl -s "$API/api/v1/data-sources" | python3 -c 'import sys,json; d=json.load(sys.stdin); print((d if isinstance(d,list) else d.get("items",[]))[0].get("id",""))' 2>/dev/null)
if [ -n "$DS" ]; then
    curl -s -X POST "$API/api/v1/metrics/scan" -H "Content-Type: application/json" -d "{\"datasource_id\":\"$DS\"}" > "$EVIDENCE_DIR/07_metrics_scan.json"
    METRICS_COUNT=$(curl -s "$API/api/v1/metrics/mappings?datasource_id=$DS&limit=1" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("total",0))' 2>/dev/null)
    [ "$METRICS_COUNT" -ge 1 ] && { PASS=$((PASS+1)); echo "[PASS] 指标映射 ($METRICS_COUNT)"; } || { FAIL=$((FAIL+1)); echo "[FAIL] 指标映射 ($METRICS_COUNT)"; }
fi

# ====================
# Phase 8: Runbook CRUD + 执行历史
# ====================
echo -e "\n[Phase 8] Runbook CRUD" | tee -a "$EVIDENCE_DIR/00_summary.log"
curl -s "$API/api/v1/knowledge/runbooks?limit=20" > "$EVIDENCE_DIR/08_runbooks_list.json"
RB_LIST=$(python3 -c "import sys,json; print(json.load(open('$EVIDENCE_DIR/08_runbooks_list.json')).get('total',0))" 2>/dev/null)
[ "$RB_LIST" -ge 18 ] && { PASS=$((PASS+1)); echo "[PASS] Runbook 列表 ($RB_LIST)"; } || { FAIL=$((FAIL+1)); echo "[FAIL] Runbook 列表 ($RB_LIST)"; }

# ====================
# Phase 9: 工作流执行结果归档为诊断记录
# ====================
echo -e "\n[Phase 9] 工作流→诊断记录归档" | tee -a "$EVIDENCE_DIR/00_summary.log"
WF_DIAG=$(curl -s "$API/api/v1/diagnose?limit=20" | python3 -c 'import sys,json; d=json.load(sys.stdin); items=d.get("items",d if isinstance(d,list) else []); print(len([i for i in items if i.get("source")=="workflow" or "wf_" in str(i.get("id",""))]))' 2>/dev/null)
[ "$WF_DIAG" -ge 1 ] && { PASS=$((PASS+1)); echo "[PASS] 工作流归档诊断记录 ($WF_DIAG)"; } || { FAIL=$((FAIL+1)); echo "[FAIL] 工作流归档诊断记录 ($WF_DIAG)"; }

# ====================
# Phase 10: 缓存命中
# ====================
echo -e "\n[Phase 10] 工作流执行缓存" | tee -a "$EVIDENCE_DIR/00_summary.log"
T1=$(date +%s%N)
curl -s -X POST "$API/api/v1/diagnosis/start" -H "Content-Type: application/json" -d '{"hostname":"172.20.32.65","time_range":"1h","user_question":"测试缓存","response_mode":"blocking"}' > /dev/null
T2=$(date +%s%N)
curl -s -X POST "$API/api/v1/diagnosis/start" -H "Content-Type: application/json" -d '{"hostname":"172.20.32.65","time_range":"1h","user_question":"测试缓存","response_mode":"blocking"}' > /dev/null
T3=$(date +%s%N)
FIRST=$(( (T2-T1)/1000000 ))
SECOND=$(( (T3-T2)/1000000 ))
echo "首次: ${FIRST}ms / 命中缓存: ${SECOND}ms" | tee -a "$EVIDENCE_DIR/10_cache.txt"
[ "$SECOND" -lt "$FIRST" ] && { PASS=$((PASS+1)); echo "[PASS] 缓存命中（$SECOND < $FIRST）"; } || { FAIL=$((FAIL+1)); echo "[FAIL] 缓存未命中"; }

# ====================
# 总结
# ====================
TOTAL=$((PASS+FAIL))
echo -e "\n================================================" | tee -a "$EVIDENCE_DIR/00_summary.log"
echo "总计: $TOTAL  PASS: $PASS  FAIL: $FAIL" | tee -a "$EVIDENCE_DIR/00_summary.log"
echo "证据目录: $EVIDENCE_DIR" | tee -a "$EVIDENCE_DIR/00_summary.log"
echo "================================================" | tee -a "$EVIDENCE_DIR/00_summary.log"
