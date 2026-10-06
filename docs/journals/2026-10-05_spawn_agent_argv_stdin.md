# SPAWN_AGENT argv and stdin inheritance — 2026-10-05

Starting point: `649745dcd4aa9f28314131578458e960dee5f550`, branch `rintaro/spawn-agent-argv-stdin` (base main).

Before the VM change, `go test ./internal/vm/ -count=1 -run TestVMSpawnAgentInheritsArgvAndStdin` failed all five original subtests:
- `child argv` and `nested argv` printed `[]` and an empty indexed argument instead of `[alpha --beta]` and `alpha`.
- `child reads next line`, `shared stream interleaving`, and `child EOF` emitted `VM_INTERNAL: runtime error: invalid memory address or nil pointer dereference` in a failed-task stderr line and omitted child completion. In the interleaving case the parent resumed at line2 rather than line4.

The child BCVM now receives its parent's `args`, `In`, and the same `lineReader` pointer. Agents execute synchronously and depth-first, so sharing is deterministic. A second bufio.Reader over In would miss bytes already buffered by the first reader and could steal bytes needed by later parent reads. The tests exercise parent line1, child line2, grandchild line3, then parent line4 using one strings.Reader. Child EOF prints an empty string and completes with exit zero and empty stderr. Two later cases cover a parent whose first read follows a child read (`parent reads after child`) and a parent that reads EOF (`""`) after the child consumed the only line (`parent EOF after child drains`). As a discrimination check, replacing the shared pointer with `bufio.NewReader(vm.In)` fails `child reads next line`, `shared stream interleaving`, and `parent reads after child`; reverting vm.go entirely fails all seven subtests. Trace and mapLedger remain unchanged: neither is needed for argv or stdin inheritance, and changing evidence collection would extend this fix's scope.

Instruction construction follows the production AST compiler: CLI_ARGS pushes a list; CLI_ARGS_GET pops a compiled index; READ_LINE pushes a string. The authoritative opcode registry grants no capability for these instructions. Only SPAWN_AGENT requires process.

Consumer evidence: `TestSwarmArgvDogfoodHFBC` builds the CLI, installs failing decoy Go/Node/wat2wasm toolchains, and compiles the new cli_app through both production `-compile-bc` and `build`. Nonempty HFBC artifacts live in temporary directories. Runs use `-run-bc -allow-caps process <artifact> alpha --beta` and cmd.Stdin containing child and grandchild lines. Exact stdout verifies both levels see indexed argv `alpha` and list `[alpha --beta]`, consume their respective input lines, complete depth-first, and return to the parent's final print. Exact stderr is empty and exit is zero; no decoy toolchain runs. howlframe.go's argvOwningModes includes run-bc and forwards flag.Args()[1:] unchanged, including the trailing flag-like argument. Without process, both artifacts fail at SPAWN_AGENT instruction 3. The new entry in `TestCliAppsNoFlagBytecode` verifies the nonempty default artifact and exact default-capability denial; existing expectations are unchanged.

The initial example validation caught a missing closing parenthesis in the new source (Expected ')' at line12); the source was corrected before rerunning validation.

Validation uses `GOCACHE=/tmp/howlframe-spawn-argv-gocache GOFLAGS=-mod=mod`:
- `gofmt -l .` (empty)
- `git diff --check`
- `go vet ./...`
- `go build ./...`
- `go test ./internal/vm/ -count=1 -run 'SpawnAgent'`
- `go test ./examples/ -count=1 -run 'TestSwarm|TestCliAppsNoFlag'`
- `go test . -count=1 -run 'TestBytecodeRunSwarm|TestBytecodeRunSingleSpawn|TestProd|TestHFIR'`
- `go test ./... -count=1`

All commands above passed on 2026-10-05 (go1.24.4 linux/amd64). `gofmt -l .` printed nothing. `go test ./... -count=1` reported every package with tests as ok.

Committed on `rintaro/spawn-agent-argv-stdin` and opened as a draft PR against main; not undrafted or merged.

Production -compile-bc remains AST bytecode. No new opcode or DOM behavior, no tip-lock retake. #90 remains Partial.
