# Native Backend Code Generators (#67)

## Item

Improvement #67: implement LLVM IR or Wasm backend serializers to replace textual Go string concatenations and move Zero toward direct-to-machine-code output.

## Current Evidence

- Selected 2026-07-30 as the highest-scoring open item across `bugs.md` and `improvements.md`: #67 scores 1.5; pending bugs score 1.0.
- No active task journals were present in `docs/journals/` or `documentation/task_journals/`.
- `git worktree list` shows only the main checkout.
- `ps aux | grep -E "claude|agy"` showed no other live matching agent process.
- Listed OpenAI model `gpt-5.6-sol` was verified live with `codex exec -m gpt-5.6-sol`, returning `MODEL_AVAILABLE`.

## Architecture Direction

LLVM IR gives mature optimization and native tool integration, but it introduces a large dependency/toolchain surface for a small compiler. Direct Wasm builds on Zero's shipped `wasm_app` backend, checker metadata, and local WAT validator; it is less "native" than LLVM but lower risk and more aligned with existing code. A custom native emitter maximizes control but has the highest correctness and validation risk.

Decision for this run: extend the existing Wasm/native serializer path around typed SSA or WAT validation instead of adding a new LLVM dependency, unless the implementation inspection shows a smaller established LLVM path already exists.

## Delegations

- Pending: delegate implementation brief to `gpt-5.6-sol`.

## Verification

- Baseline `go test ./...` passed before changes.

## Next Step

Inspect current Wasm and SSA interfaces, then delegate the implementation to `gpt-5.6-sol` with a self-contained brief.
