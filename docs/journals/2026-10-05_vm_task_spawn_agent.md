# VM TASK and SPAWN_AGENT dogfood

Base tip: 2a69e60febac000c5d05993c0f366054802af349.
TASK now pushes its description string without requiring a capability.
SPAWN_AGENT consumes a string task and completes cooperatively, synchronously,
and in-process. It emits two deterministic quoted lines per agent.
The existing process capability gate runs before stack access and type checking.
A non-string task raises TYPE_ERROR, which try_let can catch.

The swarm_dogfood cli_app prints start, runs Researcher, Writer, and Reviewer,
and prints done. Tests compile it through both -compile-bc and build with -o,
then execute each nonempty .hfbc with -run-bc -allow-caps process.
Exact stdout and empty stderr are checked with decoy go/node/wat2wasm tools.
Without capabilities, execution prints only "swarm start" before failing with
CAPABILITY_DENIED at instruction 3, opcode SPAWN_AGENT; no decoy is invoked.

There is no real agent runtime, concurrency, or message passing.
Agents do not execute HowlFrame code. A CLI probe confirms experimental
-compile-hfir-bc still rejects spawn_agent as "not in the Phase-1 executable subset".
A -run interpreter probe still reports spawn_agent unsupported in Phase 1.
Wasm keeps spawn_agent in WasmInfeasibleKinds. No DOM opcodes were added.
There is no production -compile-bc flip; #90 stays Partial.
The assurance tip-lock 4d74dbcf was not retaken.

Codex (workspace-write sandbox) made the edits; its TestVM run hit the sandbox
socket-listening denial. The parent executor re-ran validation outside the
sandbox with GOCACHE=/tmp/howlframe-task-spawn-gocache: gofmt, go vet, and
git diff --check are clean; ./internal/vm/ passes in full; the root
TestBytecodeRun|TestProd|TestHFIR tests, ./examples/ TestSwarm|TestCliAppsNoFlag,
and ./internal/bytecode ./internal/construct ./internal/hfir ./internal/checker
pass. No tests were weakened beyond replacing the UNSUPPORTED_CONSTRUCT
expectations that this change intentionally retires.
