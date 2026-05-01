#!/usr/bin/env python3
"""Generate a white-box inventory from AI WorkBench source code.
This script is read-only: it scans source files and writes inventory artifacts only to --out.
"""
from __future__ import annotations
import argparse, json, re
from pathlib import Path

ROUTE_RE = re.compile(r'v1\.(GET|POST|PUT|DELETE|PATCH)\("([^"]+)",\s*handler\.([A-Za-z0-9_]+)\)')
FRONT_ROUTE_RE = re.compile(r"\{\s*path:\s*'([^']+)'(?:,\s*redirect:\s*'([^']+)')?(?:,\s*component:\s*([A-Za-z0-9_]+))?")
AXIOS_RE = re.compile(r"axios\.(get|post|put|delete|patch)\((`[^`]+`|'[^']+'|\"[^\"]+\")")
FUNC_RE = re.compile(r'^func\s+(?:\([^)]*\)\s*)?([A-Za-z0-9_]+)\s*\(', re.M)
BRANCH_RE = re.compile(r'\b(if|else if|switch|case|for|range)\b')
STATE_RE = re.compile(r'\b(Status[A-Za-z0-9_]+|firing|resolved|pending|running|done|failed|online|offline|connected|disconnected|ai_assisted|ai_fallback|heuristic)\b')


def read(path: Path) -> str:
    return path.read_text(encoding='utf-8', errors='replace') if path.exists() else ''

def rel(root: Path, path: Path) -> str:
    return str(path.relative_to(root)).replace('\\', '/')

def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument('--root', default='.')
    ap.add_argument('--out', required=True)
    args = ap.parse_args()
    root = Path(args.root).resolve()
    out = Path(args.out).resolve(); out.mkdir(parents=True, exist_ok=True)

    backend_routes = []
    main_go = root / 'api/main.go'
    for method, path, handler in ROUTE_RE.findall(read(main_go)):
        backend_routes.append({'method': method, 'path': '/api/v1' + path, 'handler': handler, 'source': rel(root, main_go)})

    frontend_routes = []
    router = root / 'web/src/router/index.js'
    for path, redirect, component in FRONT_ROUTE_RE.findall(read(router)):
        frontend_routes.append({'path': path, 'redirect': redirect or None, 'component': component or None, 'source': rel(root, router)})

    vue_calls = []
    for vf in sorted((root / 'web/src/views').glob('*.vue')):
        text = read(vf)
        for method, target in AXIOS_RE.findall(text):
            vue_calls.append({'view': vf.stem, 'method': method.upper(), 'target': target.strip('`\'"'), 'source': rel(root, vf)})

    handlers = []
    for gf in sorted((root / 'api/internal/handler').glob('*.go')):
        text = read(gf)
        funcs = FUNC_RE.findall(text)
        handlers.append({
            'file': rel(root, gf),
            'functions': funcs,
            'branch_markers': len(BRANCH_RE.findall(text)),
            'states': sorted(set(STATE_RE.findall(text))),
        })

    store = read(root / 'api/internal/store/store.go')
    store_inventory = {
        'file': 'api/internal/store/store.go',
        'functions': FUNC_RE.findall(store),
        'mysql_branches': len(re.findall(r'mysqlOK|db\.Query|db\.Exec|REPLACE INTO|DELETE FROM', store)),
        'redis_branches': len(re.findall(r'redisOK|redisClient|Set\(|Get\(|Ping\(', store)),
        'tables': sorted(set(re.findall(r'(diagnose_records|alerts|credentials|chat_sessions|chat_messages|topology_businesses|audit_events)', store))),
    }

    security = read(root / 'api/internal/security/guard.go')
    security_inventory = {
        'file': 'api/internal/security/guard.go',
        'functions': FUNC_RE.findall(security),
        'branch_markers': len(BRANCH_RE.findall(security)),
        'levels': sorted(set(re.findall(r'"(L[0-4])"', security))),
    }

    inventory = {
        'backend_routes': backend_routes,
        'frontend_routes': frontend_routes,
        'vue_api_calls': vue_calls,
        'handlers': handlers,
        'store': store_inventory,
        'security': security_inventory,
        'coverage_expectations': {
            'api_cases_per_route_min': 2,
            'ui_cases_per_page_min': 3,
            'state_transition_cases_required': True,
            'persistence_roundtrip_required': True,
        }
    }
    (out / 'whitebox-inventory.json').write_text(json.dumps(inventory, ensure_ascii=False, indent=2), encoding='utf-8')

    lines = ['# Whitebox Inventory', '', '## Backend routes', '']
    lines += ['| Method | Path | Handler |', '|---|---|---|']
    for r in backend_routes:
        lines.append(f"| {r['method']} | `{r['path']}` | `{r['handler']}` |")
    lines += ['', '## Frontend routes', '', '| Path | Component | Redirect |', '|---|---|---|']
    for r in frontend_routes:
        lines.append(f"| `{r['path']}` | `{r.get('component') or ''}` | `{r.get('redirect') or ''}` |")
    lines += ['', '## Vue API calls', '', '| View | Method | Target |', '|---|---|---|']
    for c in vue_calls:
        lines.append(f"| `{c['view']}` | {c['method']} | `{c['target']}` |")
    lines += ['', '## Store', '', f"- Tables: {', '.join(store_inventory['tables'])}", f"- MySQL branch markers: {store_inventory['mysql_branches']}", f"- Redis branch markers: {store_inventory['redis_branches']}"]
    (out / 'whitebox-inventory.md').write_text('\n'.join(lines) + '\n', encoding='utf-8')
    print(json.dumps({'ok': True, 'out': str(out), 'routes': len(backend_routes), 'pages': len(frontend_routes), 'vue_calls': len(vue_calls)}, ensure_ascii=False))
    return 0

if __name__ == '__main__':
    raise SystemExit(main())
