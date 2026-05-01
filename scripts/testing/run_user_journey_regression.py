#!/usr/bin/env python3
"""Ultimate user-journey regression for AI WorkBench.

Safe by design: creates only batch-tagged records, never runs remote destructive
Catpaw operations, and tears down chat/diagnosis/topology records it creates.
"""
from __future__ import annotations

import argparse
import json
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

BUSINESS_HOSTS = ["198.18.20.11", "198.18.20.12", "198.18.20.20", "198.18.22.11", "198.18.22.12", "198.18.22.13"]
BUSINESS_ENDPOINTS = [
    {"ip": "198.18.20.20", "port": 80, "service_name": "nginx", "protocol": "HTTP health"},
    {"ip": "198.18.20.11", "port": 8081, "service_name": "jvm-app", "protocol": "JVM app probe"},
    {"ip": "198.18.20.12", "port": 8081, "service_name": "jvm-app", "protocol": "JVM app probe"},
    {"ip": "198.18.20.20", "port": 6379, "service_name": "redis", "protocol": "Redis PING"},
    {"ip": "198.18.22.11", "port": 1521, "service_name": "oracle", "protocol": "Oracle listener probe"},
    {"ip": "198.18.22.12", "port": 1521, "service_name": "oracle", "protocol": "Oracle listener probe"},
    {"ip": "198.18.22.13", "port": 1521, "service_name": "oracle", "protocol": "Oracle listener probe"},
]

class Client:
    def __init__(self, base: str):
        self.base = base.rstrip("/")

    def req(self, method: str, path: str, body=None, timeout=45):
        data = None
        headers = {"Accept": "application/json"}
        if body is not None:
            data = json.dumps(body, ensure_ascii=False).encode("utf-8")
            headers["Content-Type"] = "application/json; charset=utf-8"
        request = urllib.request.Request(self.base + path, data=data, headers=headers, method=method)
        started = time.time()
        try:
            with urllib.request.urlopen(request, timeout=timeout) as resp:
                return self._response(method, path, resp.status, resp.read(), started, dict(resp.headers))
        except urllib.error.HTTPError as err:
            return self._response(method, path, err.code, err.read(), started, dict(err.headers), ok=False)
        except Exception as err:
            return {"method": method, "path": path, "status": 0, "ok": False, "ms": int((time.time() - started) * 1000), "error": str(err)}

    @staticmethod
    def _response(method, path, status, raw, started, headers, ok=None):
        text = raw.decode("utf-8", "replace")
        try:
            body = json.loads(text) if text else None
        except Exception:
            body = text
        return {"method": method, "path": path, "status": status, "ok": status < 400 if ok is None else ok and status < 400, "ms": int((time.time() - started) * 1000), "body": body, "headers": headers}

def prom_query(prom_url: str, query: str):
    url = prom_url.rstrip("/") + "/api/v1/query?" + urllib.parse.urlencode({"query": query})
    try:
        with urllib.request.urlopen(url, timeout=20) as resp:
            body = json.loads(resp.read().decode("utf-8", "replace"))
            return {"query": query, "status": resp.status, "ok": resp.status < 400 and body.get("status") == "success", "body": body}
    except Exception as err:
        return {"query": query, "status": 0, "ok": False, "error": str(err)}

def record(name, result, expect=None):
    ok = bool(result.get("ok")) if expect is None else result.get("status") in expect
    return {"name": name, "pass": ok, "result": result}

def node_layers(graph):
    return {(n.get("type"), n.get("service_name") or n.get("name")): {"x": n.get("x"), "layer": n.get("layer"), "name": n.get("name")} for n in graph.get("nodes", []) if isinstance(n, dict)}

def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://localhost:8080")
    parser.add_argument("--prom-url", default="http://localhost:9090")
    parser.add_argument("--batch-id", default="aiw-user-journey-" + time.strftime("%Y%m%d-%H%M%S"))
    parser.add_argument("--out", default=None)
    args = parser.parse_args()
    out = Path(args.out or f".test-evidence/{args.batch_id}")
    out.mkdir(parents=True, exist_ok=True)
    client = Client(args.base_url)
    created = {"chat_sessions": [], "topology_businesses": [], "diagnoses": [], "alerts": []}
    cases = []

    cases.append(record("storage health is readable", client.req("GET", "/api/v1/health/storage")))
    cases.append(record("datasource health is readable", client.req("GET", "/api/v1/health/datasources")))
    cases.append(record("AI provider health endpoint is readable", client.req("GET", "/api/v1/health/ai-providers"), {200}))

    session = client.req("POST", "/api/v1/chat/sessions", {"title": f"终极闭环 {args.batch_id}", "model": "default", "target_ip": "198.18.20.11", "test_batch_id": args.batch_id})
    cases.append(record("new user creates a batch chat session", session))
    if isinstance(session.get("body"), dict) and session["body"].get("id"):
        created["chat_sessions"].append(session["body"]["id"])

    topo = client.req("POST", "/api/v1/topology/discover", {"hosts": BUSINESS_HOSTS, "endpoints": BUSINESS_ENDPOINTS, "include_platform": False, "use_ai": True}, timeout=80)
    cases.append(record("application owner runs AI-assisted scoped topology discovery", topo))
    graph = topo.get("body") if isinstance(topo.get("body"), dict) else {"nodes": [], "edges": []}
    nodes = graph.get("nodes", [])
    node_types = {n.get("type") for n in nodes if isinstance(n, dict)}
    cases.append({"name": "topology excludes platform and Catpaw agent nodes", "pass": "ai_agent" not in node_types and "catpaw_agent" not in node_types, "result": {"node_types": sorted(str(x) for x in node_types)}})
    service_positions = {n.get("service_name"): n.get("x") for n in nodes if isinstance(n, dict) and n.get("service_name") in {"nginx", "jvm-app", "redis", "oracle"}}
    cases.append({"name": "topology separates entry app middleware database columns", "pass": service_positions.get("nginx", 0) < service_positions.get("jvm-app", 0) < service_positions.get("redis", 0) < service_positions.get("oracle", 0), "result": service_positions})
    host_positions = [n.get("x") for n in nodes if isinstance(n, dict) and n.get("type") == "host"]
    cases.append({"name": "business hosts stay in host column", "pass": bool(host_positions) and all(x == 70 for x in host_positions), "result": host_positions})
    edge_protocols = {e.get("protocol") for e in graph.get("edges", []) if isinstance(e, dict)}
    cases.append({"name": "topology uses semantic service protocols", "pass": {"HTTP health", "JVM app probe", "Redis PING", "Oracle listener probe"}.issubset(edge_protocols), "result": sorted(str(x) for x in edge_protocols)})

    business_payload = {"id": args.batch_id, "name": f"终极闭环业务 {args.batch_id}", "hosts": BUSINESS_HOSTS, "endpoints": BUSINESS_ENDPOINTS, "attributes": {"test_batch_id": args.batch_id, "owner": "测试负责人", "purpose": "核心业务链路", "slo": "99.9%", "level": "P1"}, "graph": graph}
    business = client.req("POST", "/api/v1/topology/businesses", business_payload)
    cases.append(record("application owner saves business topology", business))
    if isinstance(business.get("body"), dict) and business["body"].get("id"):
        business_id = business["body"]["id"]
        created["topology_businesses"].append(business_id)
        inspection = client.req("GET", f"/api/v1/topology/businesses/{business_id}/inspect", timeout=90)
        cases.append(record("AI business inspection returns health model", inspection))
        body = inspection.get("body") if isinstance(inspection.get("body"), dict) else {}
        processes = body.get("processes", []) if isinstance(body, dict) else []
        metrics = body.get("metrics", []) if isinstance(body, dict) else []
        summary = body.get("summary", "") if isinstance(body, dict) else ""
        cases.append({"name": "business inspection is AI-led when provider is available", "pass": ("external-ai" in str(body.get("planner", "")) and not body.get("ai_error")) or ("ai_provider_unavailable" in body.get("data_sources", []) and bool(body.get("ai_error"))), "result": {"planner": body.get("planner"), "ai_error": body.get("ai_error")}})
        cases.append({"name": "business inspection explicitly includes Redis middleware", "pass": any(p.get("name") == "redis" and p.get("layer") == "middleware" for p in processes) and any("redis" in (m.get("name", "") + m.get("query", "")).lower() for m in metrics) , "result": {"summary": summary, "processes": processes, "redis_metrics": [m for m in metrics if "redis" in (m.get("name", "") + m.get("query", "")).lower()]}})
        cases.append({"name": "business process classification separates app middleware database", "pass": {"app", "middleware", "database"}.issubset({p.get("layer") for p in processes if isinstance(p, dict)}), "result": processes})

    for ip in ["198.18.20.11", "198.18.20.20", "198.18.22.11"]:
        diag = client.req("POST", "/api/v1/diagnose", {"ip": ip, "question": f"{args.batch_id} 用户视角压力诊断", "test_batch_id": args.batch_id}, timeout=70)
        cases.append(record(f"SRE diagnoses {ip}", diag, {200, 202}))
        if isinstance(diag.get("body"), dict) and diag["body"].get("id"):
            created["diagnoses"].append(diag["body"]["id"])

    alert = client.req("POST", "/api/v1/alert/webhook", {"alerts": [{"status": "firing", "labels": {"alertname": f"{args.batch_id} test alert", "severity": "warning", "instance": "198.18.20.11", "test_batch_id": args.batch_id}, "annotations": {"summary": "user journey alert"}}]})
    cases.append(record("operator receives test alert", alert, {200, 202}))
    if isinstance(alert.get("body"), dict) and alert["body"].get("id"):
        created["alerts"].append(alert["body"]["id"])

    prom = [prom_query(args.prom_url, f'count({{instance=~".*{ip}.*"}}) or count({{ident="{ip}"}}) or count({{ip="{ip}"}}) or count({{host="{ip}"}}) or count({{hostname="{ip}"}}) or count({{from_hostip="{ip}"}}) or count({{target=~".*{ip}.*"}})') for ip in BUSINESS_HOSTS]
    cases.append({"name": "Prometheus IP label queries completed", "pass": any(item.get("ok") for item in prom), "result": prom})

    cleanup = []
    for diag_id in created["diagnoses"]:
        cleanup.append(client.req("DELETE", f"/api/v1/diagnose/{diag_id}"))
    for session_id in created["chat_sessions"]:
        cleanup.append(client.req("DELETE", f"/api/v1/chat/sessions/{session_id}"))
    for business_id in created["topology_businesses"]:
        cleanup.append(client.req("DELETE", f"/api/v1/topology/businesses/{business_id}"))

    passed = sum(1 for case in cases if case.get("pass"))
    failed = [case for case in cases if not case.get("pass")]
    score = 100 if not failed else int(passed * 100 / len(cases))
    summary = {"batch_id": args.batch_id, "cases": len(cases), "passed": passed, "failed": len(failed), "score": score, "status": "pass" if not failed else "fail"}
    (out / "user-journey-regression.json").write_text(json.dumps({"summary": summary, "cases": cases}, ensure_ascii=False, indent=2), encoding="utf-8")
    (out / "created-ids.json").write_text(json.dumps(created, ensure_ascii=False, indent=2), encoding="utf-8")
    (out / "teardown-result.json").write_text(json.dumps({"cleanup": cleanup, "scope": "batch-tagged API records only; no remote destructive operations"}, ensure_ascii=False, indent=2), encoding="utf-8")
    (out / "user-journey-summary.md").write_text("# 用户闭环回归\n\n" + "\n".join([f"- [{'x' if c.get('pass') else ' '}] {c['name']}" for c in cases]) + "\n", encoding="utf-8")
    (out / "self-score.json").write_text(json.dumps({"score": score, "passed": passed, "failed": len(failed), "p0": 0, "p1": len(failed)}, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(summary, ensure_ascii=False))
    return 0 if not failed else 1

if __name__ == "__main__":
    raise SystemExit(main())
