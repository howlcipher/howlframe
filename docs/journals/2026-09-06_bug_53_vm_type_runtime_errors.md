# Bug #53 Follow-up: VM Type/Runtime Error Hardening

## Summary
Merged PR #31 (`devin/vm-unchecked-assertions`) completes the migration of VM opcodes to structured `*VMError` panics for malformed operands.

## Defect
Several bytecode VM opcodes (`OpEnv`, `OpSleep`, `OpCliArgsGet`, `OpForInit`, `OpForNext`, `OpCall`) used raw string panics or bare Go type assertions when operands had the wrong type, or when `OpCall` targeted an undefined function. These surfaced as untyped `VM_INTERNAL` failures instead of the catchable `TYPE_ERROR`/`RUNTIME_ERROR` codes used elsewhere.

## Root Cause
Bug #47 established the checked-assertion/`NewRuntimeError("TYPE_ERROR", ...)` pattern for string/collection instructions, but the remaining opcodes were not migrated. `OpCall` additionally reported undefined functions with a raw `panic("undefined function: ...")`.

## Fix
- `internal/vm/vm.go`:
  - `OpForInit` / `OpForNext`: wrong operand types now raise `TYPE_ERROR`.
  - `OpCall`: undefined function now raises `RUNTIME_ERROR`.
  - `OpSleep`: non-number operand now raises `TYPE_ERROR`.
  - `OpCliArgsGet`: non-number index now raises `TYPE_ERROR`.
  - `OpEnv`: non-string name now raises `TYPE_ERROR` via the existing `popCheckedString` helper.
- `internal/vm/vm_negative_test.go`: added `TestVMTypeAndRuntimeErrors` with table-driven coverage for each opcode.
- `bugs.md`: updated bug #53 status to `Done (2026-09-06)` and corrected the root-cause/fix narrative.

## Verification
- `go test ./internal/vm -run TestVMTypeAndRuntimeErrors` pass.
- `go test ./internal/vm -run TestVMFileAndNetworkTypeAssertions` pass.
- `go test ./...` (32 packages) pass.
- `go vet ./...`, `gofmt -l .`, `go build ./...` clean.
- `slopslint check --classify --enforce` (`go_production` 83/83, `go_tests` 132/132).
- `go run tools/difftest/main.go`: 18 passed, 28 skipped, 0 failed.
- Downstream consumer tests (`go test ./apps/...`) pass.
- PR #31 CI green.

## Merge
- PR: https://github.com/howlcipher/howlframe/pull/31
- Merge commit: `a96c011a7c78ea190ffaacdbcf9ff9863b8bd811`
- Fast-forwarded primary checkout and safe worktree to `origin/main`.

## Review
Same-provider multi-model review: Astra review plus lighter-model adversarial review. Review approved with noted latent follow-ups: `OpSleep` int64 acceptance, `OpForNext` index validation, `OpCall` arity check, and `OpCall` lazy-synthesis capability guard.
