# SPAWN_AGENT body RETURN isolation — 2026-10-05

Starting point: `fca83079b19940ed7242dc48eb36d001edef50bf` (post-#101), branch `rintaro/spawn-agent-return-isolation`.

Before the change, the new `body return isolated` and `nested return isolated` subtests of `TestVMSpawnAgentNesting` failed. A bare body RETURN with value 7 escaped the child recovery boundary: the parent print never ran and there was no child diagnostic. An inner RETURN likewise escaped both spawn boundaries and prevented the outer print and completion. The previous test explicitly expected exit code 7.

The SPAWN_AGENT child recovery switch now handles VmReturn with one ErrOut line:
`[Swarm VM] Agent "Name" failed task: "work": RETURN: return from spawn agent body`.
It sets the existing failed flag and skips the child's completion line. The parent continues with exit zero and no runtime failure. A nested return is isolated at the inner boundary, allowing the outer body to print and complete. CALL still handles normal function returns within a spawn body. Child instruction accounting (`vm.executed = childVM.executed`) and parent stack truncation are unchanged.

VmExit and LIMIT_EXCEEDED still re-panic. The parent process capability gate and non-string task TYPE_ERROR still fail before child execution; existing try_let handling is unchanged. Ordinary child VM errors retain their existing isolation. Tests assert exact stdout and stderr, including exactly one RETURN diagnostic and no completion for the failed child, and check exit zero for all successful table cases.

Validation used `GOCACHE=/tmp/howlframe-return-gocache` from the repository root:
- `gofmt -w internal/vm/vm.go internal/vm/spawn_agent_test.go`
- `gofmt -l internal/vm/vm.go internal/vm/spawn_agent_test.go` (empty)
- `git diff --check`
- `go vet ./internal/vm/ ./internal/bytecode/ ./examples/ .`
- `go test ./internal/vm/ -count=1 -run 'TestVMTaskAndSpawnAgent|SpawnAgent'`
- `go test ./examples/ -count=1 -run 'TestSwarm|TestCliAppsNoFlag'`
- `go test . -count=1 -run 'TestBytecodeRunSwarm|TestBytecodeRunSingleSpawn|TestProd|TestHFIR'`
- `go build ./...`

All requested focused validation passed. Existing serialized swarm consumer tests passed without new examples or changes to production compilation.

Broader checks: `go vet ./...` and `python3 scripts/test_seo.py` passed. `go test ./... -count=1` passed the root and six app packages, then produced no further output for several minutes and was interrupted; the full suite is not claimed green. The benchmark harness was run in a temporary source copy to keep its generated root binary out of this repository. Its initial invocation failed because the default Go cache is read-only; the copy also required `GOFLAGS=-buildvcs=false` because it has no Git metadata. With the writable cache and VCS stamping disabled, seven of eight harness tests passed; `test_all_references` failed on the HTTP reference because this sandbox forbids socket creation (`socket: operation not permitted`). No tests were weakened. These broader checks remain incomplete in the sandbox; the requested focused suites provide the validation for this slice.

Production `-compile-bc` remains AST bytecode. No new opcodes, no DOM changes, no tip-lock `4d74dbcf` retake. #90 remains Partial. No push or PR is part of this slice.
