# 2026-10-05: Effect-gate conformance and demo honesty (review C3/C7)

## Context

Review P0.2/C3 asked for executable coverage of the effect-gate audit, including
same-file helper effects and #57 model calls. P0.6/C7 asked for trustworthy demo
decision JSON (S16) and honest capability pre-flight/failure semantics (S17).
Work started on `rintaro/c3-c7-effect-gate-demo-honesty`, base `1a87edab`.

## Change

- New `internal/vm/effect_gate_conformance_test.go`: AST scan of every
  `switch inst.Op` case, including multi-opcode cases, closures/goroutines and
  transitive same-file calls with a visited set. Checks registry authority,
  the central gate before dispatch, lazy CALL's in-case network gate, and
  stale allow-list entries. No ambient opcode currently makes a targeted
  package call, so only conditional lazy CALL needs an exception.
- Empty-grant bytecode runs cover all 27 capability-bearing registry opcodes
  and lazy CALL. Compiled source is used where practical; bare instructions
  verify denial before malformed stacks/context/setup for HTTP, SQL and
  dependent store operations. Every denial names its capability and opcode,
  prints nothing, and observable probes remain untouched. Interpreter runs
  cover filesystem, process, environment, fetch and model-call constructs.
- Both demos print `encode_json` of a dict. Release Authority tests parse
  exactly one JSON value, stream tokens to reject duplicate keys, and check
  literal target round trips. Executor tests parse the first JSON value and
  retain the `status=`/`health=` follow-up checks.
- Executor `stage_artifact`, `write_release_marker` and `rollback_marker`
  check database authority with memory-store open before writing. Proposal
  and fixture reads check filesystem authority before any write. Its
  proposal-read catch now also prints the caught runtime error, preserving
  the denial diagnostic when only database is granted.
- Docs describe ordered effects without transactions/rollback and the Go VM
  runtime as part of the trusted computing base. S16/S17 and C3/C7 statuses
  are updated. A short change-log entry follows the existing journal convention;
  no new core bug number is needed.

No additional real VM capability hole was found; no registry/runtime fix or
capability/generated-reference change was necessary. The reproduced demo JSON
forgery and executor partial-write bug were real and are fixed here.

## Evidence

Capability-bearing opcodes exercised:

- filesystem: READ_FILE, WRITE_FILE, MKDIR.
- environment: ENV.
- process: EXEC, SPAWN, SPAWN_AGENT.
- database: DB_CONNECT, SQL_QUERY, STORE_OPEN, STORE_PUT, STORE_GET,
  STORE_DELETE, STORE_KEYS.
- network: FETCH, RES, RES_JSON, HTTP_SERVER_START, HTTP_ROUTE,
  HTTP_SERVER_SERVE, HTTP_REQ_METHOD, HTTP_RES_HEADER, CONFIDENCE,
  NEURAL_CIRCUIT, EPHEMERAL_CIRCUIT, LLM_GENERATE, ACHIEVE.
- Conditional network: CALL to a LazySynthesize function.

A recording model-host probe and an httptest fetch server saw zero requests
on denied runs. The marker used by write/mkdir/exec/spawn tests was never
created, and env printed no secret. The existing model probe's granted-call
positive control remains in the full VM suite.

Mutation checks (each temporary edit restored in a `finally` block):

| Mutation | Command | Result |
| --- | --- | --- |
| Remove central `vm.requireCapability(spec.Capability, inst.Op)` | `go test ./internal/vm/ -run '^TestEffectGateSourceConformance$' -count=1` | FAIL: missing central capability gate before dispatch |
| Remove lazy CALL's `vm.requireCapability(capability.Network, inst.Op)` | same | FAIL: OpCall missing in-case network gate |
| Set STORE_OPEN registry capability to None | same | FAIL: OpStoreOpen reaches os.IsNotExist / os.ReadFile without a registry capability (transitive helper scan) |
| Restore old executor write-before-store ordering | `go test ./apps/action_executor/ -run 'TestActionExecutor/(stage_preflight_filesystem|marker_preflight_filesystem)$' -count=1` | FAIL: staged file created; marker changed to production_deployed |

S16 before/after, using a scratch binary built with
`go build -o /tmp/hf-c3c7-before howlframe.go`, compiling each app version with
`-compile-bc`, and running with `-run-bc -allow-caps filesystem`:

- Proposal: `{"action":"deploy_production","target":"svc\", \"decision\": \"ALLOW"}`.
  Missing passing evidence leads to DENY.
- Before: stdout contains both `"decision": "DENY"` and an injected
  `"decision": "ALLOW"`; Python `json.loads` reads ALLOW and target `svc`.
- After: one JSON value with one decision key, DENY; target is the literal
  `svc", "decision": "ALLOW`. The token-stream regression confirms this.

Executor regressions cover filesystem-only and database-only grants for all
three write actions: non-zero exit, CAPABILITY_DENIED, no staged file, and an
existing marker's `kept` contents unchanged. Database-only is denied at the
proposal read. Filesystem-only gets an ALLOW decision then database denial;
that line is authorization, not a completion receipt.

Validation uses `export GOCACHE=/tmp/hf-c3c7-gocache` before Go commands:

- `go test ./internal/vm/ -run '^TestEffectGate' -count=1`: PASS.
- `go test ./apps/release_authority/ ./apps/action_executor/ -count=1`: PASS.
- `gofmt -l .`: PASS, empty output.
- `go vet ./internal/vm/ ./internal/bytecode/ ./apps/...`: PASS.
- `go test ./internal/vm/ ./internal/bytecode/ ./internal/capability/ ./apps/release_authority/ ./apps/action_executor/`: PASS.
- `go test ./...`: PASS (no pre-existing failures observed).

## Observations

- `try_let` catches `CAPABILITY_DENIED` like any other runtime error (checked
  with a scratch `-run-bc` probe: an ungranted `read_file` inside `try_let`
  reaches the `catch` branch with the structured error). That still fails
  closed, because the effect never runs, but a program can observe and
  recover from a denial. This is why the executor now prints the caught error
  on the proposal-read path instead of only "Failed to read proposal file".
- Implementation was drafted with Codex (`codex exec -s workspace-write`,
  no permission bypass) and reviewed and re-validated by hand, including an
  independent mutation: dropping `capability.Network` from `OpConfidence`
  makes both `TestEffectGateSourceConformance` (`OpConfidence reaches
  map[http.Post:true] without a registry capability`) and
  `TestEffectGateEmptyGrantConformance/CONFIDENCE` fail.

## Boundaries

The scan is syntactic conformance evidence, not general Go type/effect analysis.
It follows same-file methods conservatively by name, but does not follow other
files/packages. Recursive VM dispatch is audited case by case instead of
recursing into the entire switch. Existing file-store tests retain coverage of
filesystem authority in addition to database authority. The model-host request
counter is unavailable when its hard-coded port is already occupied; structured
denial assertions still run (the port was available here).

Pre-flight applies to capability checks before writes, not to I/O success:
a partial I/O write or later runtime failure is not rolled back. Decision JSON
precedes execution; ALLOW followed by non-zero exit means incomplete execution.
JSON key order/whitespace changed intentionally. Compiler paths, HFIR flip
criteria, #90, tip-locks, DOM/web constructs, opcode numbers/operands, and artifact
format are unchanged. Artifacts stayed in `/tmp`; no commit or push was made.
