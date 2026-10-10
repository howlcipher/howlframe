# HowlFrame Applications

`examples/` demonstrates individual HowlFrame concepts.
`apps/` contains software intentionally built with HowlFrame to expose real application-development strengths and limitations.

## Current Applications
* **[Status API](./status_api)**: Proves network capabilities, HTTP serving under bytecode execution, and environment interaction. (Backend: Bytecode)
* **[Log Analyzer](./log_analyzer)**: Proves file reading, deterministic string manipulation, regex matching, and graceful capability denial handling. (Backend: Bytecode)
* **[KV CLI](./kv_cli)**: Proves native in-memory store functionality, sequential state manipulation, and capability-gated database actions. (Backend: Bytecode)
* **[Todo CLI](./todo_cli)**: Proves stateful task management, CLI CRUD logic, and persistent storage interaction. (Backend: Bytecode)
* **[Task API](./task_api)**: Proves HTTP CRUD, native-store state shared correctly across independent requests, and capability enforcement inside HTTP handlers. (Backend: Bytecode)
* **[REST Tasks](./rest_tasks)**: Real REST methods + path params (`GET/POST /tasks`, `GET/PATCH/DELETE /tasks/{id}`) with `store_keys`. (Backend: Bytecode)
* **[File Transform](./file_transform)**: CSV→JSONL pipeline proving `read_file`/`write_file` end-to-end. (Backend: Bytecode)
* **[Webhook Relay](./webhook_relay)**: Outbound `http_request` with status+body relay and optional POST body. (Backend: Bytecode)
* **[Notes CLI](./notes_cli)**: Persistent notes CRUD with runtime `store_open` URI + `sget`/`sput`/`switch` density. (Backend: Bytecode)
* **[Auth API](./auth_api)**: Bearer-gated HTTP routes and path/query echo without middleware. (Backend: Bytecode)
* **[Grep CLI](./grep_cli)**: Regex line matcher with `lines`/`regex_match` and capability denial. (Backend: Bytecode)
* **[Release Authority](./release_authority)**: Demonstrates an untrusted AI proposing deployment actions while standalone HowlFrame independently enforces evidence, approval, capability, and state-transition policy. (Backend: Bytecode)
* **[Action Executor](./action_executor)**: Bounded Action Executor — demonstrates a finite trusted action catalog where untrusted AI/user proposals can request operations but cannot select arbitrary code, paths, capabilities, or execution primitives. (Backend: Bytecode)
