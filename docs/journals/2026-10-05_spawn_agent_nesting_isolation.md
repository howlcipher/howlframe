# SPAWN_AGENT nesting and child error isolation — 2026-10-05

Starting point: `e2d286e2d004790a3ff4600d7bd5946dd8e1a7f0`, branch `rintaro/spawn-agent-nesting-isolation`.

Before the VM change, `go test ./internal/vm/ -count=1 -run TestVMSpawnAgentNesting` failed four subtests: an inner ENV capability denial aborted the outer agent; a failed child prevented its sibling and parent print; three nested bodies succeeded despite a six-instruction budget; and three levels succeeded with MaxCallDepth set to two. Three-level output ordering, binding/stack separation, and escaping RETURN already passed. The existing bytecode compiler's attached body lengths work relative to each child slice: no compiler or checker change was needed.

SPAWN_AGENT remains synchronous and deterministic, with depth-first output. A child starts at its parent's executed count and copies that count back in a defer, including failed work and nested work. Spawn depth starts at zero; each child adds one. A spawn exceeding MaxCallDepth fails before its spawning line. Two levels are allowed at a limit of two, and the third is rejected. Description-only spawns use the same depth gate.

The child recovery boundary emits exactly one diagnostic to ErrOut for an ordinary VM error:
`[Swarm VM] Agent "Name" failed task: "description": CODE: message`.
Unexpected recovered panics use VM_INTERNAL and their formatted message. Failed children have no completion line. The parent retains its operand stack (apart from consuming the task operand), skips the entire attached body once, and continues. Its environment and call frames are untouched; nearest bindings are captured, and the existing value-copy helper copies maps and lists so child map writes also stay isolated. Shared stores retain their existing semantics.

LIMIT_EXCEEDED is re-panicked through every child boundary: budget and depth errors cannot become successful agent failures. VmReturn is also re-panicked, preserving the executable starting-point behavior: it reaches the enclosing function or top-level runner, where an int64 value becomes the exit code. A test preserves return 7 and absence of completion output. VmExit likewise retains its existing process-control behavior. The SPAWN_AGENT process gate and task string check still happen before child execution and remain fatal at that instruction. Existing try_let handling of these fatal-to-spawn cases is unchanged; its parent type-error catch test remains green. A capability denial inside the child is isolated instead.

Consumer evidence: `TestSwarmNestedDogfoodHFBC` compiles the new cli_app through production `-compile-bc` and `build`, writes nonempty serialized artifacts in temporary directories, and runs with only process granted and decoy Go/Node/wat2wasm toolchains. Exact stdout shows Outer/Inner depth-first execution, the failed Denied sibling, and the final parent print; exact stderr contains only the isolated environment denial; exit is zero. Both artifacts still fail at parent SPAWN_AGENT instruction 3 without process. `TestCliAppsNoFlagBytecode` includes the new example and verifies its no-flag artifact and exact default-capability denial. The existing swarm example and its tests are unchanged.

VM coverage includes three-level nesting, inner-only failure with outer completion, failed child followed by a successful sibling and parent print, isolated TYPE_ERROR, variable and stack separation, map writes before failure, exact shared instruction budget, failed-work accounting, fatal shared budget exhaustion, fatal depth exhaustion, and escaping RETURN.

Validation passed with `GOCACHE=/tmp/howlframe-spawn-nest-gocache GOFLAGS=-mod=mod`:
- `gofmt -l .` (empty) and `git diff --check`
- `go vet ./internal/vm/ ./internal/bytecode/ ./examples/ .`
- `go build ./...`
- `go test ./internal/vm/ -count=1 -run 'TestVMTaskAndSpawnAgent|SpawnAgent'`
- `go test ./internal/bytecode/ ./internal/construct/ ./internal/checker/ -count=1`
- `go test ./examples/ -count=1 -run 'TestSwarm|TestCliAppsNoFlag'`
- `go test . -count=1 -run 'TestBytecodeRunSwarm|TestBytecodeRunSingleSpawn|TestProd|TestHFIR'`

All requested validation is green. Before push, the CI-equivalent run outside the sandbox also passed: `go vet ./...`, `go test ./... -count=1`, `python3 scripts/test_seo.py`, and `python3 -m unittest test_harness.py` in `benchmarks/v2/harness`.

Production -compile-bc remains AST bytecode. No new opcode or DOM behavior, no tip-lock retake. #90 remains Partial.
