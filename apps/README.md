# HowlFrame Applications

`examples/` demonstrates individual HowlFrame concepts.
`apps/` contains software intentionally built with HowlFrame to expose real application-development strengths and limitations.

## Current Applications
* **[Status API](./status_api)**: Proves network capabilities, HTTP serving under bytecode execution, and environment interaction. (Backend: Bytecode)
* **[Log Analyzer](./log_analyzer)**: Proves file reading, deterministic string manipulation, regex matching, and graceful capability denial handling. (Backend: Bytecode)
* **[KV CLI](./kv_cli)**: Proves native in-memory store functionality, sequential state manipulation, and capability-gated database actions. (Backend: Bytecode)
* **[Todo CLI](./todo_cli)**: Proves stateful task management, CLI CRUD logic, and persistent storage interaction. (Backend: Bytecode)
* **[Task API](./task_api)**: Proves HTTP CRUD, native-store state shared correctly across independent requests, and capability enforcement inside HTTP handlers. (Backend: Bytecode)
* **[Release Authority](./release_authority)**: Demonstrates an untrusted AI proposing deployment actions while standalone HowlFrame independently enforces evidence, approval, capability, and state-transition policy. (Backend: Bytecode)
