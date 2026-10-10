# Dogfood common apps: find gaps, fix HowlFrame (2026-10-10)

## Goal

Build common application shapes in HowlFrame, compare against what Python/Go/Node
express easily, fix platform gaps, and repeat until those apps are fully
expressible on standalone bytecode.

## Round 1 — blockers found

| Gap | Symptom vs other languages | Fix |
|---|---|---|
| Density sugar skipped inside `lambda` | `(cond … (else …))` in a route failed compile with `construct "else"`; multi-let inside handlers also broken. Python `elif` / Go `switch` work in handlers without ceremony. | Correct `lambda` params index in `ast.Desugar` (params at child 1, not 2) and treat lambda body as sugar context. |
| `fetch` returns body bytes only | Cannot relay upstream HTTP status; Python `requests` / Go `http.Response` expose status+body. | Add `(http_request url method [body])` → dict `status` + `body`, with optional request body on bytecode (`HTTP_REQUEST`, `IntOperand` body flag). |
| No REST dogfood app | `task_api` is POST-body RPC; path/method surface existed but unused. | New `apps/rest_tasks`. |
| No file write pipeline app | `log_analyzer` only reads. | New `apps/file_transform`. |
| No outbound HTTP app | Nothing used `fetch`/`http_request` in `apps/`. | New `apps/webhook_relay`. |

## Apps shipped

1. **`apps/file_transform`** — CSV → JSONL via `read_file`/`write_file`/`lines`/`encode_json`
2. **`apps/rest_tasks`** — `GET/POST /tasks`, `GET/PATCH/DELETE /tasks/{id}` with `store_keys`
3. **`apps/webhook_relay`** — forward GET/POST to `UPSTREAM_URL`, relay status+body

## Evidence

```bash
go test ./apps/file_transform/ ./apps/rest_tasks/ ./apps/webhook_relay/ -count=1
go test ./internal/ast/ ./internal/bytecode/ ./internal/vm/ ./internal/construct/ -count=1
```

## Remaining limits (not blocking these apps)

- `fetch` still returns bytes only (kept for ABI/conformance); prefer `http_request` when status matters
- `middleware` still Unsupported (auth must be hand-rolled in handlers)
- `schema` / SQL migrations still Unsupported
- `match` still Unsupported (`cond`/`switch` cover the dispatch need for these apps; see round 2 journal)
