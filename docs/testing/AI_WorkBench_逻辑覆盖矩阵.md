# AI WorkBench ??????

## 1. ????? API ??

| ?? | ??/?? | API | ?? Handler | ???? | ???/???? | ???? |
|---|---|---|---|---|---|---|
| `/workbench` | ????????????? | GET/POST `/chat/sessions`, POST `/chat`, GET `/models` | Chat/session handlers | created/active/persisted/deleted | MySQL chat tables, AI, Prometheus context | ???????XSS?AI key missing?stream/non-stream |
| `/diagnose` | IP ?????????? | POST/GET/DELETE `/diagnose` | Start/List/DeleteDiagnose | pending/running/done/failed/deleted | MySQL diagnose, Prometheus, Catpaw fallback, AI | ?? IP?Prom no data?Catpaw offline?AI fail????? |
| `/alerts` | ?????????AI?? | GET `/alerts`, PUT `/alerts/:id/resolve`, POST `/diagnose` | Alert handlers | firing/resolved/diagnosing | MySQL alerts, diagnose | XSS label???????? ID??? title |
| `/topology` | ?????????????????? | `/topology/*` | topology handlers | business_created/discovered/saved/deleted | MySQL topology, Prometheus targets, Catpaw status, AI planner | ??????????????? IP?AI fallback |
| `/catpaw` | ???????????????? | credentials, remote, catpaw endpoints | credential/remote/catpaw handlers | installing/online/reporting/uninstalling | MySQL credentials/audit, Redis heartbeat, WinRM/SSH | ????????????????????? |
| `/settings/ai` | ?? provider/model??????? | `/ai-providers`, `/health/ai-providers` | settings/health | default provider changed | config/viper, AI provider | `******` ???`${}` ????401 ? alive |
| `/settings/datasource` | ???????? | `/data-sources`, `/health/datasources` | settings/health | datasource changed | config/viper, Prom/MySQL/Redis | SSRF URL???????????????? |

## 2. API ????

| API | ???? | ??/???? | ???? | ????? |
|---|---|---|---|---|
| `POST /chat` | AI ????? assistant message | bind fail, missing model, missing key, upstream fail | prompt injection, XSS markdown | session/messages reload |
| `POST /diagnose` | Prometheus data -> AI report | invalid IP, no Prom data -> Catpaw fallback, AI fail | SSRF-like IP/host rejected | diagnose record + delete |
| `POST /catpaw/heartbeat` | valid IP updates online state | invalid/multicast/link-local IP 400 | spoofed bad IP | agent list/Redis fallback |
| `POST /catpaw/report` | summary/raw stored | bad JSON, invalid IP, empty report | mojibake/XSS raw report safe | summary_report/raw_report |
| `POST /remote/exec` | allowed dry-run/safe command | L3 confirm required, L4 reject, host reject | encoded/base64/newline obfuscation | audit events |
| `POST /topology/discover` | scoped graph generated | empty scope, invalid IP/port, AI fail | unscoped target ignored | business save/reload |
| `POST /ai-providers` | save provider | malformed body, masked key preserve | no plaintext response | health true/false |
| `POST /data-sources` | save datasource | bad URL, service down | SSRF/internal URL | health reason |
| `POST /credentials` | save masked credential | missing name/user, masked preserve | secret not leaked | list/delete reload |

## 3. Coverage Gates

| Gate | Requirement |
|---|---|
| Route coverage | every `api/main.go` route has at least one normal and one invalid request case |
| UI coverage | every route has MCP/Playwright load + primary action + invalid input case |
| State coverage | every transition in the state table has at least one test or documented environment blocker |
| Persistence coverage | every non-monitoring entity can be created, listed, reloaded, deleted, and verified absent |
| Safety coverage | destructive operations require test_batch_id or explicit safety confirmation; L4 never executes |
| Encoding coverage | all reports/UI responses must be UTF-8 readable and free of `????`/replacement-char mojibake |
