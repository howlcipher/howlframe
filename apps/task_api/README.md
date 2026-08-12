# Task API

A stateful HTTP task-management service written in HowlFrame, running standalone
on the bytecode VM. Demonstrates HTTP routing, request/response handling,
CRUD logic, and native-store state shared correctly across independent HTTP
requests within one running server process.

## Usage

```bash
go run ../../howlframe.go -compile-bc task_api.howl

go run ../../howlframe.go -run-bc -allow-caps network,database task_api.howl.bc.bin
```

The server listens on port 8081.

## API

The standalone bytecode HTTP runtime has no dynamic path parameters, no
query-string access, and no way for application code to read the request
method — see `DEVELOPMENT_NOTES.md` for the full investigation. Every action
is therefore a `POST` endpoint that takes its input, if any, as a JSON body:

* `POST /tasks/create` — body `{"title": "..."}` → `201` created task, or
  `400 {"error":"title_required"}` for a missing/empty title.
* `POST /tasks/list` — body ignored → `200 {"tasks": [...]}` in ascending id
  order.
* `POST /tasks/get` — body `{"id": "..."}` → `200` task, `400` for a
  missing/empty id, `404 {"error":"not_found"}` for an unknown or malformed
  id.
* `POST /tasks/complete` — body `{"id": "..."}` → `200` updated task with
  `status: "done"`. Completing an already-done task is idempotent (same
  `200` response, no error).
* `POST /tasks/delete` — body `{"id": "..."}` → `200 {"deleted": "..."}`.
  A subsequent `get`/`complete`/`delete` on that id returns `404`.

A task is `{"id": "1", "title": "...", "status": "open"|"done"}`. Malformed
JSON on any endpoint returns `400 {"error":"invalid_json"}`. Any HTTP method
works identically on a given path — method is not read or enforced anywhere
in the standalone bytecode runtime.

## State and capabilities

State lives in the native `memory://session` store, shared safely across
every HTTP request handled by the running server process (verified — see
`DEVELOPMENT_NOTES.md`). It does not survive a process restart.

Requires exactly `network,database`. `network` is needed to start the
listener and to send any response; `database` is needed for every store
operation. Neither capability substitutes for the other, and no other
capability grants any authority here.
