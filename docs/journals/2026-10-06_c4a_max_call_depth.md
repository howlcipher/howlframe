# 2026-10-06: Bytecode CALL depth limit (C4a)

## Findings and change

- Review S5 found ordinary CALL recursion could crash the process with a Go
  stack overflow when the runner raised the instruction budget. C4's first
  bullet is implemented; the remaining C4b bullets stay open.
- Default MaxCallDepth is 1000. The existing field already governed SPAWN_AGENT
  nesting, so it remains shared: the spawn default also rises from 128 to 1000.
  Spawn work remains bounded by the shared instruction budget. Call depth and
  spawn depth are separate counters, each compared with MaxCallDepth.
- Main runs at call depth zero. A ceiling N allows exactly N simultaneously
  active CALL frames; the next call raises runtime LIMIT_EXCEEDED at CALL.
  Non-positive ceilings fail closed for CALL; a program without CALL can still
  execute with a positive instruction budget. The spawn guard is unchanged.
- Depth unwinds on normal completion, VmReturn recovery, and propagating panic.
  Spawn children inherit the parent's active call depth, preventing a spawn
  boundary from resetting the ceiling for child CALLs.
- Both legacy -run-bc and howlframe run accept --max-call-depth, default 1000,
  and reject zero and negative values. The bytecode/bc runner applies the policy.

## Verification

- New VM tests cover default policy, unbounded recursion with a 10,000,000
  instruction budget, the exact five/six-frame boundary, 999 active frames
  returning 42, 100 sequential shallow calls in a loop, zero/negative ceilings,
  zero without CALL, spawn inheritance, and restoration on all exit paths.
- CLI tests compile a source artifact through production -compile-bc and run
  it through both runners: default success, depth-three LIMIT_EXCEEDED, and
  zero/negative rejection.
- Required commands: `gofmt -l .`, `go vet ./...`,
  `go test ./internal/vm/ -count=1`, then `go test ./... -count=1`.
- `gofmt -l .` printed nothing; `go vet ./...` passed; `go test ./... -count=1`
  passed (Go 1.24.4, all packages `ok`).
- Targeted: `go test ./internal/vm/ -run 'CallDepth|SpawnAgent' -count=1` and
  `go test . -run 'TestRunBytecodeMax(CallDepth|Instructions)Flag' -count=1`.
  The existing SPAWN_AGENT tests, including the depth-2 "depth guard" case,
  pass unchanged.
- S5 probe: `(cli_app (defun f () (return (call f))) (call f))` compiled with
  `-compile-bc` and run with `-run-bc --max-instructions 2000000000` now exits 1
  in about 50 ms with
  `{"phase":"runtime","code":"LIMIT_EXCEEDED",...,"opcode":"CALL","message":"call depth limit exceeded (max 1000)"}`
  instead of a Go `fatal error: stack overflow`. `run --max-call-depth 5000`
  reports `(max 5000)`.
- Codex's first pass ran inside a workspace-write sandbox where loopback
  listeners and `spawnSync` are denied, so a few fetch/HTTP/JS-exec tests could
  not run there. The full suite above was rerun outside that sandbox and passed.

## Deferrals and limits

C4b allocation accounting, deadlines, and fetch/exec byte caps are excluded.
AST interpreter -run recursion remains unbounded by this limit. No HFBC wire
format, opcode registry, wasm, codegen, or HFIR lowerer change. Production
-compile-bc remains AST bytecode; improvement #90 stays Partial.
