#!/usr/bin/env python3
"""Safe white-box regression runner for AI WorkBench.
It creates only test_batch_id-tagged data and avoids destructive remote operations.
"""
from __future__ import annotations
import argparse, json, time, urllib.error, urllib.parse, urllib.request
from pathlib import Path

SECURITY_PAYLOADS = [
    "<script>alert(1)</script>",
    "admin' OR 1=1--",
    "$(rm -rf /)",
    "../../../../etc/passwd",
    "http://169.254.169.254/latest/meta-data/",
    "??" * 32,
]

class Client:
    def __init__(self, base: str):
        self.base = base.rstrip('/')
    def req(self, method: str, path: str, body=None, timeout=30):
        data = None
        headers = {'Accept': 'application/json'}
        if body is not None:
            data = json.dumps(body, ensure_ascii=False).encode('utf-8')
            headers['Content-Type'] = 'application/json; charset=utf-8'
        request = urllib.request.Request(self.base + path, data=data, headers=headers, method=method)
        started = time.time()
        try:
            with urllib.request.urlopen(request, timeout=timeout) as resp:
                raw = resp.read()
                text = raw.decode('utf-8', 'replace')
                parsed = None
                try: parsed = json.loads(text)
                except Exception: parsed = text
                return {'method': method, 'path': path, 'status': resp.status, 'ok': resp.status < 400, 'ms': int((time.time()-started)*1000), 'body': parsed, 'headers': dict(resp.headers)}
        except urllib.error.HTTPError as e:
            raw = e.read()
            text = raw.decode('utf-8', 'replace')
            parsed = None
            try: parsed = json.loads(text)
            except Exception: parsed = text
            return {'method': method, 'path': path, 'status': e.code, 'ok': False, 'ms': int((time.time()-started)*1000), 'body': parsed, 'headers': dict(e.headers)}
        except Exception as e:
            return {'method': method, 'path': path, 'status': 0, 'ok': False, 'ms': int((time.time()-started)*1000), 'error': str(e)}

def prom_query(prom_url: str, query: str, timeout=20):
    url = prom_url.rstrip('/') + '/api/v1/query?' + urllib.parse.urlencode({'query': query})
    try:
        with urllib.request.urlopen(url, timeout=timeout) as resp:
            return {'query': query, 'status': resp.status, 'body': json.loads(resp.read().decode('utf-8', 'replace'))}
    except Exception as e:
        return {'query': query, 'status': 0, 'error': str(e)}

def assert_case(result, expect_status=None, allow=None):
    if expect_status is not None:
        result['pass'] = result.get('status') == expect_status
    elif allow is not None:
        result['pass'] = result.get('status') in allow
    else:
        result['pass'] = bool(result.get('ok'))
    return result

def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument('--base-url', default='http://localhost:8080')
    ap.add_argument('--frontend-url', default='http://localhost:3000')
    ap.add_argument('--prom-url', default='http://localhost:9090')
    ap.add_argument('--batch-id', default='aiw-whitebox-' + time.strftime('%Y%m%d-%H%M%S'))
    ap.add_argument('--out', default=None)
    args = ap.parse_args()
    out = Path(args.out or ('.test-evidence/' + args.batch_id)); out.mkdir(parents=True, exist_ok=True)
    c = Client(args.base_url)
    cases = []

    # health and config
    cases.append(assert_case(c.req('GET', '/api/v1/health/storage')))
    cases.append(assert_case(c.req('GET', '/api/v1/health/datasources')))
    cases.append(assert_case(c.req('GET', '/api/v1/health/ai-providers'), allow={200}))
    cases.append(assert_case(c.req('GET', '/api/v1/models'), allow={200}))

    # chat sessions persistence roundtrip
    session = c.req('POST', '/api/v1/chat/sessions', {'title': 'whitebox ' + args.batch_id, 'model': 'gpt-5.4', 'target_ip': '198.18.20.11'})
    cases.append(assert_case(session, allow={200}))
    sid = None
    if isinstance(session.get('body'), dict): sid = session['body'].get('id')
    if sid:
        cases.append(assert_case(c.req('GET', f'/api/v1/chat/sessions/{sid}'), allow={200}))
        cases.append(assert_case(c.req('PUT', f'/api/v1/chat/sessions/{sid}', {'title': 'whitebox renamed ' + args.batch_id}), allow={200}))
        cases.append(assert_case(c.req('PUT', f'/api/v1/chat/sessions/{sid}', {'title': ''}), expect_status=400))

    # invalid JSON / bad input branches
    cases.append(assert_case(c.req('GET', '/api/v1/chat/sessions/not-found'), expect_status=404))
    cases.append(assert_case(c.req('POST', '/api/v1/catpaw/heartbeat', {'ip': '999.1.1.1'}), expect_status=400))
    cases.append(assert_case(c.req('POST', '/api/v1/catpaw/heartbeat', {'ip': '198.18.20.11', 'hostname': 'whitebox', 'version': args.batch_id}), allow={200}))
    cases.append(assert_case(c.req('POST', '/api/v1/catpaw/report', {'ip': '198.18.20.11', 'title': 'whitebox ' + args.batch_id, 'report': '# report\n' + SECURITY_PAYLOADS[0]}), allow={200}))

    # alerts state branch
    alert = {'title': 'whitebox alert ' + args.batch_id, 'target_ip': '198.18.20.11', 'severity': 'warning', 'labels': {'test_batch_id': args.batch_id, 'payload': SECURITY_PAYLOADS[1]}, 'starts_at': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())}
    cases.append(assert_case(c.req('POST', '/api/v1/alert/catpaw', alert), allow={200}))
    alerts = c.req('GET', '/api/v1/alerts'); cases.append(assert_case(alerts, allow={200}))
    alert_id = None
    if isinstance(alerts.get('body'), list):
        for item in alerts['body']:
            if isinstance(item, dict) and item.get('title') == alert['title']:
                alert_id = item.get('id'); break
    if alert_id:
        cases.append(assert_case(c.req('PUT', f'/api/v1/alerts/{alert_id}/resolve'), allow={200}))
    cases.append(assert_case(c.req('PUT', '/api/v1/alerts/not-found/resolve'), allow={200,404}))

    # topology scoped discovery and persistence
    topo_body = {
        'hosts': ['198.18.20.11', '198.18.20.20', '198.18.22.11'],
        'endpoints': [
            {'ip': '198.18.20.20', 'port': 80, 'service_name': 'nginx', 'protocol': 'HTTP'},
            {'ip': '198.18.20.11', 'port': 8081, 'service_name': 'jvm-app', 'protocol': 'HTTP'},
            {'ip': '198.18.20.20', 'port': 6379, 'service_name': 'redis', 'protocol': 'TCP'},
            {'ip': '198.18.22.11', 'port': 1521, 'service_name': 'oracle', 'protocol': 'TCP'},
        ],
        'include_platform': False,
        'use_ai': True,
    }
    discovered = c.req('POST', '/api/v1/topology/discover', topo_body, timeout=90); cases.append(assert_case(discovered, allow={200}))
    graph = discovered.get('body') if isinstance(discovered.get('body'), dict) else {'nodes': [], 'edges': []}
    cases.append(assert_case(c.req('POST', '/api/v1/topology/businesses', {'name': 'whitebox topology ' + args.batch_id, 'hosts': topo_body['hosts'], 'endpoints': topo_body['endpoints'], 'graph': graph}), allow={200}))
    cases.append(assert_case(c.req('POST', '/api/v1/topology/discover', {'hosts': [], 'endpoints': [], 'include_platform': False}), expect_status=400))
    cases.append(assert_case(c.req('POST', '/api/v1/topology', {'nodes': [{'id': '', 'name': ''}], 'edges': []}), expect_status=400))

    # remote safety dry-run / blocked cases, no real destructive execution expected.
    cases.append(assert_case(c.req('POST', '/api/v1/remote/exec', {'ip': '8.8.8.8', 'command': 'whoami', 'test_batch_id': args.batch_id}), allow={400,403}))
    cases.append(assert_case(c.req('POST', '/api/v1/remote/exec', {'ip': '192.168.1.7', 'command': 'rm -rf /', 'test_batch_id': args.batch_id}), allow={400,403,409}))

    # datasource / provider health should never leak secrets.
    ai = c.req('GET', '/api/v1/ai-providers'); cases.append(assert_case(ai, allow={200}))
    if 'api_key' in json.dumps(ai.get('body'), ensure_ascii=False).lower() and 'sk-' in json.dumps(ai.get('body'), ensure_ascii=False):
        cases[-1]['pass'] = False; cases[-1]['defect'] = 'possible plaintext API key leak'
    cases.append(assert_case(c.req('GET', '/api/v1/data-sources'), allow={200}))
    cases.append(assert_case(c.req('GET', '/api/v1/audit/events'), allow={200}))

    # Prometheus label/IP coverage
    prom = []
    for ip in ['198.18.20.11','198.18.20.12','198.18.20.20','198.18.22.11','198.18.22.12','198.18.22.13']:
        prom.append(prom_query(args.prom_url, f'count({{instance=~".*{ip}.*"}}) or count({{ident="{ip}"}}) or count({{ip="{ip}"}}) or count({{host="{ip}"}}) or count({{hostname="{ip}"}}) or count({{from_hostip="{ip}"}}) or count({{target=~".*{ip}.*"}})'))

    # Cleanup own session only; do not remote-delete or global-clean.
    cleanup = []
    if sid:
        cleanup.append(c.req('DELETE', f'/api/v1/chat/sessions/{sid}'))

    passed = sum(1 for x in cases if x.get('pass'))
    failed = [x for x in cases if not x.get('pass')]
    summary = {'batch_id': args.batch_id, 'base_url': args.base_url, 'frontend_url': args.frontend_url, 'cases': len(cases), 'passed': passed, 'failed': len(failed), 'status': 'pass' if not failed else 'fail'}
    (out / 'api-regression.json').write_text(json.dumps({'summary': summary, 'cases': cases}, ensure_ascii=False, indent=2), encoding='utf-8')
    (out / 'prometheus-regression.json').write_text(json.dumps(prom, ensure_ascii=False, indent=2), encoding='utf-8')
    (out / 'persistence-regression.json').write_text(json.dumps({'session_id': sid, 'cleanup': cleanup}, ensure_ascii=False, indent=2), encoding='utf-8')
    defects = ['# Defects', '']
    if failed:
        for f in failed:
            defects.append(f"- FAIL `{f.get('method')} {f.get('path')}` status={f.get('status')} body={str(f.get('body') or f.get('error'))[:300]}")
    else:
        defects.append('- No P0/P1 defects found by API whitebox baseline.')
    (out / 'defects.md').write_text('\n'.join(defects) + '\n', encoding='utf-8')
    score = 100 if not failed else max(0, int(passed * 100 / len(cases)))
    (out / 'self-score.json').write_text(json.dumps({'score': score, 'passed': passed, 'failed': len(failed), 'p0': 0, 'p1': len(failed)}, ensure_ascii=False, indent=2), encoding='utf-8')
    (out / 'teardown-result.json').write_text(json.dumps({'cleanup': cleanup, 'scope': 'only test chat session deleted; no remote destructive operations'}, ensure_ascii=False, indent=2), encoding='utf-8')
    print(json.dumps(summary, ensure_ascii=False))
    return 0 if not failed else 1

if __name__ == '__main__':
    raise SystemExit(main())
