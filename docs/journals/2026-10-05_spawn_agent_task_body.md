# SPAWN_AGENT task bodies — 2026-10-05

Starting point: `a286c6489b6c280b3b794c20275075d10282c667`, branch `rintaro/spawn-agent-task-body`.

`(spawn_agent "Name" (task "description" stmt...))` now compiles the opaque description to TASK and attaches the compiled statements after SPAWN_AGENT. Its existing opcode carries the body instruction count after the agent name. Standalone TASK still emits only the description. A spawn_agent task operand that is not a `(task ...)` list falls back to compiling the expression onto the stack so SPAWN_AGENT can still fail closed on a non-string type. The JS checker accepts and checks optional statements; the Go checker already traverses them.

SPAWN_AGENT retains its registry `process` gate and string type check. It captures the environment using SPAWN's nearest-binding rule and runs a child BCVM synchronously with the same program, stores, limits, capabilities, and output streams. Errors propagate; completion prints only after success. The parent skips the attached instructions. No goroutine or recovery handler is introduced. Description-only fixtures remain unchanged and exercise the two-line stub.

Consumer evidence: `TestSwarmDogfoodHFBC` compiles the updated example through both production `-compile-bc` and `build`, serializes artifacts outside the repository, and executes them with decoy toolchains. Researcher, Writer, and Reviewer body prints appear once, between their respective spawn/completion lines. Without `process`, both artifacts stop at instruction 3 with only `swarm start` on stdout. `TestCliAppsNoFlagBytecode` retains that exact denial. VM tests also verify captured variables, propagated ENV capability denial without completion output, and existing non-string TYPE_ERROR behavior.

Validation passed:
- gofmt check and `git diff --check`
- `go test ./internal/vm/ -count=1 -run TestVMTaskAndSpawnAgent`
- `go test ./internal/bytecode/ ./internal/construct/ ./internal/checker/ -count=1`
- `go test ./examples/ -count=1 -run 'TestSwarmDogfoodHFBC|TestCliAppsNoFlag'`
- `go build ./...`, `go vet ./...`, `python3 scripts/test_seo.py`

Full `go test ./...` was interrupted after it stopped making progress, then rerun as `go test ./... -timeout 60s` (log: `/tmp/spawn-agent-task-body-go-test.log`). The root package, CLI app packages, and affected bytecode/checker/construct packages passed. HTTP fixtures in examples, Go/JS backends, VM, and difftest fail on sandbox socket denial. JS subprocess fixtures fail with `spawnSync ... EPERM`. `apps/status_api` and `apps/task_api` time out after 60 seconds. Full CI remains incomplete pending an outside-sandbox rerun.

The CI benchmark harness (`python3 -m unittest test_harness.py` in `benchmarks/v2/harness`) fails its HTTP reference because sandbox socket creation and connection are denied (`operation not permitted`). It requires a rerun outside the sandbox; tests were not weakened.

Production `-compile-bc` remains AST bytecode. No new opcode or DOM behavior, no Go/JS codegen stub changes, no tip-lock retake. #90 remains Partial.
