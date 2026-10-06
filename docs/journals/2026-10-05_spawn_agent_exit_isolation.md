# SPAWN_AGENT body EXIT isolation — 2026-10-05

Starting point: `6502d615ef40025d3e4bdc9240a4cf54927fc6af` (main tip), branch `okabe/isolate-spawn-agent-vmexit`.

Before the change, the new `body exit isolated` and `nested exit isolated` subtests of `TestVMSpawnAgentNesting` failed. Each body loads int64(7) and executes OpExit. A bare body EXIT escaped the child recovery boundary: the parent print never ran and there was no child diagnostic. An inner EXIT likewise escaped both spawn boundaries and prevented the outer print and completion.

The SPAWN_AGENT child recovery switch now handles VmExit with one ErrOut line using its code field:
`[Swarm VM] Agent %q failed task: %q: EXIT: exit %d from spawn agent body\n`.
It sets the existing failed flag and skips the child's completion line. The parent continues with exit zero and no runtime failure. A nested exit is isolated at the inner boundary, allowing the outer body to print and complete. Child instruction accounting (`vm.executed = childVM.executed`) and parent stack truncation remain before the recovery switch and are unchanged.

RETURN's diagnostic and isolation remain unchanged; CALL still handles normal function returns within a spawn body. LIMIT_EXCEEDED still re-panics for instruction budgets and nesting depth. The parent process capability gate and non-string task TYPE_ERROR still fail before child execution. Ordinary child VM errors retain their existing isolation.

The new tests assert exact stdout and stderr, including exactly one EXIT diagnostic with code 7 and no completion for the failed child. Both require exit zero and no RuntimeFailure. The focused suite also passed its existing RETURN, ordinary child failure, sibling continuation, shared budget, depth guard, process gate, task type, and CALL return coverage.

Validation used `GOCACHE=/tmp/howlframe-vmexit-gocache` from the repository root:
- `gofmt -w internal/vm/vm.go internal/vm/spawn_agent_test.go`
- `gofmt -l internal/vm/vm.go internal/vm/spawn_agent_test.go` (empty)
- `git diff --check`
- `go vet ./internal/vm/ ./internal/bytecode/ ./examples/ .`
- `go test ./internal/vm/ -count=1 -run 'TestVMTaskAndSpawnAgent|SpawnAgent'` (passed)
- `go build ./...`
- `go test ./... -count=1` (passed; full suite green outside sandbox)

All requested validation passed after the fix. The initial regression-only run failed as expected before the recovery change. No tests were weakened; no generated artifacts were added to the repository root.

Production `-compile-bc` remains AST bytecode. No HFIR flip, new opcodes, opcode metadata changes, DOM changes, or tip-lock `4d74dbcf` retake. #90 remains Partial. No C1/C2/C4 or harness changes.
