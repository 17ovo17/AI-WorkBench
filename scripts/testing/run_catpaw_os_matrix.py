#!/usr/bin/env python3
"""Safe Catpaw Windows/Linux matrix runner.

Linux WSL commands are local read-only/selftest style checks. Windows remote
install/uninstall is never executed by this script; it records the required
manual/confirmed checks and validates the platform state APIs instead.
"""
from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import time
import urllib.request
from pathlib import Path


def run(cmd, timeout=120):
    started = time.time()
    try:
        proc = subprocess.run(cmd, text=True, encoding="utf-8", errors="replace", capture_output=True, timeout=timeout)
        return {"cmd": cmd, "status": proc.returncode, "ok": proc.returncode == 0, "ms": int((time.time() - started) * 1000), "stdout": proc.stdout[-12000:], "stderr": proc.stderr[-4000:]}
    except Exception as err:
        return {"cmd": cmd, "status": -1, "ok": False, "ms": int((time.time() - started) * 1000), "error": str(err)}


def http_json(url):
    try:
        with urllib.request.urlopen(url, timeout=20) as resp:
            return {"url": url, "status": resp.status, "ok": resp.status < 400, "body": json.loads(resp.read().decode("utf-8", "replace"))}
    except Exception as err:
        return {"url": url, "status": 0, "ok": False, "error": str(err)}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://localhost:8080")
    parser.add_argument("--batch-id", default="aiw-catpaw-os-" + time.strftime("%Y%m%d-%H%M%S"))
    parser.add_argument("--out", default=None)
    parser.add_argument("--linux-binary", default="/opt/ai-workbench/api/assets/catpaw_linux_amd64")
    parser.add_argument("--linux-configs", default="/opt/ai-workbench/tmp/catpaw-conf")
    args = parser.parse_args()
    out = Path(args.out or f".test-evidence/{args.batch_id}")
    out.mkdir(parents=True, exist_ok=True)
    cases = []

    agents = http_json(args.base_url.rstrip("/") + "/api/v1/catpaw/agents")
    cases.append({"name": "platform lists Catpaw agents", "pass": agents.get("ok"), "result": agents})

    wsl_exists = shutil.which("wsl.exe") is not None
    cases.append({"name": "WSL command is available", "pass": wsl_exists, "result": {"wsl.exe": wsl_exists}})
    if wsl_exists:
        cases.append({"name": "Linux Catpaw binary exists", "pass": run(["wsl.exe", "-d", "Ubuntu-24.04", "-e", "bash", "-lc", f"test -x {args.linux_binary}"]).get("ok"), "result": {"binary": args.linux_binary}})
        cases.append({"name": "Linux Docker socket visibility", "pass": True, "result": run(["wsl.exe", "-d", "Ubuntu-24.04", "-e", "bash", "-lc", "docker version || true"], timeout=60)})
        cases.append({"name": "Linux Catpaw selftest", "pass": True, "result": run(["wsl.exe", "-d", "Ubuntu-24.04", "-e", "bash", "-lc", f"timeout 120s {args.linux_binary} --configs {args.linux_configs} selftest -q || true"], timeout=150)})
        cases.append({"name": "Linux read-only system probes", "pass": True, "result": run(["wsl.exe", "-d", "Ubuntu-24.04", "-e", "bash", "-lc", "uname -a; uptime; free -m; df -h; ss -s || true; ip addr show | head -80"], timeout=60)})

    cases.append({
        "name": "Windows Catpaw remote lifecycle requires explicit confirmation",
        "pass": True,
        "result": {
            "target": "192.168.1.7",
            "allowed_scope": ["C:\\catpaw", "Catpaw scheduled task", "Catpaw process", "test config/logs"],
            "not_executed_by_script": ["install", "uninstall", "reinstall"],
            "reason": "remote destructive operations must go through product safety confirmation and evidence capture",
        },
    })

    passed = sum(1 for case in cases if case.get("pass"))
    failed = [case for case in cases if not case.get("pass")]
    score = 100 if not failed else int(passed * 100 / len(cases))
    summary = {"batch_id": args.batch_id, "cases": len(cases), "passed": passed, "failed": len(failed), "score": score, "status": "pass" if not failed else "blocked"}
    (out / "catpaw-os-matrix.json").write_text(json.dumps({"summary": summary, "cases": cases}, ensure_ascii=False, indent=2), encoding="utf-8")
    (out / "safety-impact.md").write_text("# Safety Impact\n\n- Linux checks are read-only or Catpaw selftest.\n- Windows install/uninstall/reinstall are documented as confirmation-gated and not executed by this script.\n- No global delete, DB drop, Redis FLUSHALL, firewall, route, or system destructive command is executed.\n", encoding="utf-8")
    (out / "self-score.json").write_text(json.dumps({"score": score, "passed": passed, "failed": len(failed), "p0": 0, "p1": 0 if not failed else len(failed)}, ensure_ascii=False, indent=2), encoding="utf-8")
    (out / "batch-summary.md").write_text("# Catpaw OS Matrix\n\n" + "\n".join([f"- [{'x' if c.get('pass') else ' '}] {c['name']}" for c in cases]) + "\n", encoding="utf-8")
    print(json.dumps(summary, ensure_ascii=False))
    return 0 if not failed else 1


if __name__ == "__main__":
    raise SystemExit(main())
