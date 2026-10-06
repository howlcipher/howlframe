# 2026-10-06: Runner-written execution receipt v0 (C5)

## Findings and change

- C5 supplies an optional durable execution record for the bytecode runner.
  Review S14 found that program stdout could imitate a receipt. `--receipt`
  now selects a separate host-written file; printing fake receipt JSON leaves
  stdout unchanged and does not affect the real record.
- Both legacy `-run-bc` and `run --target bytecode/bc` support the option,
  including `.howl` compiled in process. Empty/absent selects the existing
  execution path. Interpreter targets (including legacy `-run`) reject a
  nonempty receipt option. No new opcode exposes the recorder to programs.
- `RunBytecodeWithReceipt` reuses the evidence runner's recovery boundary but
  returns code and receipt without terminating the process. Runtime diagnostics
  remain identical. The CLI persists the receipt before `os.Exit`, including
  capability/limit traps, other structured runtime failures, exit N, and return.
  Parse, compile, and artifact validation errors occur before execution and do
  not produce execution receipts.

## Schema

JSON uses explicit struct field order, two-space indentation, and a trailing
newline. No fields are omitted; empty grant/effects are `[]`, never null.

| Field | v0 meaning |
| --- | --- |
| schema | `howlframe.receipt/v0` |
| artifact_sha256 | Hex SHA-256 of exact artifact file bytes; source uses canonical `bytecode.WriteArtifact` bytes generated before execution |
| compiler_version | Runner's HowlFrame Version; HFBC has format/program version but no recorded compiler identity |
| grant | Sorted, deduplicated runner capability allow-list |
| limits | max_instructions, max_call_depth, max_memory_bytes, max_fetch_body_bytes, max_exec_output_bytes, deadline_ms |
| instructions_used | Existing VM executed count, including synchronous SPAWN_AGENT work even on failures |
| effects | Ordered `{op, capability, target_summary, decision}` for each capability decision, allowed or denied |
| effects_truncated | True if the 10,000-entry effect cap is reached and another entry is attempted |
| exit | `{status, code, error}`; code is the intended program process exit code |
| exit.error | null or `{code, opcode, instruction, message}` |

Zero deadline_ms means no deadline; positive durations are rounded up to whole
milliseconds so a configured submillisecond deadline does not appear absent.
A runtime trap has status error and code 1. Explicit nonzero exit/return has
status error with null error; zero exit/return and normal completion have status
ok. Host error messages can contain secrets, so receipt messages are code-only
(`runtime failure: CODE`) except safe LIMIT_EXCEEDED/CAPABILITY_DENIED messages.
Full existing diagnostics remain on stderr. The artifact checksum in HFBC covers
its payload, not its exact file envelope, so it is not reused for this digest.
Source serialization uses the existing deterministic v2 writer; no wire change.

## Redaction policy

Summaries are computed non-destructively at instruction decision time. Every
summary is bounded to 256 bytes, cutting at a UTF-8 boundary. Operand values are
never dumped by default.

| Operation | Summary | Excluded |
| --- | --- | --- |
| FETCH | scheme://host[:port]; `<invalid-url>` when unparseable or missing scheme/host | userinfo, path, query, fragment |
| ENV | Variable name | Value |
| READ_FILE, WRITE_FILE, MKDIR | Path as given, without cleaning | Contents |
| EXEC | Basename of program operand | Arguments |
| SPAWN, SPAWN_AGENT | Empty (VM tasks have no external argv[0]) | Task/body text |
| DB_CONNECT | Driver | DSN |
| SQL_QUERY | DB handle name | Query |
| STORE_OPEN | Store URI as given (memory://name or file://path) | Keys/values |
| Other STORE_* | Store handle name | Keys/values |
| HTTP_SERVER_START / HTTP_ROUTE / HTTP_SERVER_SERVE | Listen address / route pattern / saved listen address | Requests and bodies |
| RES, RES_JSON, HTTP_RES_HEADER, HTTP_REQ_METHOD | Empty | Bodies/headers |
| Model operations and lazy CALL | Empty | Prompts, inputs, model output |
| Default | Empty | All operands |

Filesystem paths, store names, routes, and env names can themselves be sensitive;
the receipt is a resource-level record, not a blanket removal of personal data.

## Decisions and limits

- Central registry checks and explicit capability/store checks use the same
  recorder. Each executed instruction has a separate decision context; repeated
  checks of the same capability deduplicate, while database plus filesystem
  store decisions remain separate. Denials record before the runtime panic.
- PRINT, STDERR, SLEEP, TIME_NOW, READ_LINE, EXIT and other capability-free
  operations are ambient host channels and omitted in v0.
- Recorder pointer is shared by SPAWN goroutines, HTTP route children, and
  SPAWN_AGENT. A mutex serializes effect append and finalization. Detached
  SPAWN effects after finalization are dropped; those children retain historical
  separate instruction accounting. HTTP child instruction counts likewise retain
  existing behavior. Concurrent effects have scheduler-dependent order; byte
  determinism applies to deterministic runs without concurrency/timing.
- First 10,000 decisions are retained and truncation is explicit. Summary
  computation, decision allocation, and synchronization are absent when the
  recorder pointer is nil. Child errors keep existing isolation semantics: a denied child effect need not
  fail the parent run. Existing evidence sealing remains unchanged; the
  receipt is an independent host reporting object.
- Persistence uses a same-directory temporary file, 0600 permissions, close,
  then rename. Parent directories must exist. This is atomic replacement on
  supported filesystems, not an fsync durability guarantee. Temp files are
  removed on failures. If writing fails, stderr gets a host diagnostic: exit 1
  if execution succeeded, otherwise preserve the program's failure code.
  `exit.code` describes execution rather than a subsequent persistence failure.

## Verification

- VM tests cover empty/pure programs, byte determinism, allowed/denied URL
  redaction (real httptest and in-memory transport), environment/exec secrets,
  limits with instruction usage, spawn-agent effects, per-capability store
  decisions, exit/return and structured failures, cap/dedup/finalization, and
  denied summary policy including invalid URLs and length bounds, every registry
  gate and lazy CALL, listener-free HTTP children, and concurrent recording.
- CLI tests build the real binary and production artifacts, test both runners
  and source compilation, verify the exact/canonical hash, fake stdout receipt,
  capability/limit/runtime failures, exit/return codes, identical streams with
  and without the option, interpreter rejection, no output file when absent or
  empty, 0600 permissions, and write-failure exit behavior.
- Commands use `GOCACHE=/tmp/howlframe-c5-go-cache` because the default cache
  under `/home/box/.cache` is read-only in this sandbox. Go is 1.24.4.
- Required: `gofmt -l .`, `go vet ./...`,
  `go test ./internal/vm/ -count=1`, `go test ./... -count=1`.
- Supplemental hermetic receipt tests:
  `go test ./internal/vm/ -run 'TestReceipt(Empty|FetchRedactionInMemory|Environment|Denied|SpawnAgent|Store|Exit|Cap|Concurrent|Registry|HTTPChild|Deadline)' -count=1`.
- C4 compatibility and CLI receipts:
  `go test . -run 'TestReceipt|TestRunBytecodeResourceFlags|TestRunBytecodeMaxCallDepthFlag|TestRunBytecodeMaxInstructionsFlag' -count=1`.

- `gofmt -l .` printed nothing; `git diff --check` and `go vet ./...`
  passed. Hermetic receipt tests passed, also with `-race`. C4 CLI limits
  and new receipt CLI tests passed. C4 VM compatibility checks passed:
  `go test ./internal/vm -run 'ResourceDefaults|Memory|AllocationOpcode|FetchLimitsInMemory|Exec(OutputCap|LargeOutputAndExit|WithoutDeadline)|Deadline(SleepAndLoop|Model|HTTPResponseBodies)|CallDepth|SpawnAgent|Instruction|DefaultExecutionPolicy|EffectGateSource' -count=1`.
- Full VM validation stops at `TestInterpretFetchDeniedBeforeRequest`:
  httptest listen on `[::1]:0` returns `socket: operation not permitted`.
  The new real-listener `TestReceiptFetchRedaction` independently fails for
  the same reason; its in-memory companion passes without weakening it.
- Full `go test ./... -count=1` was attempted and interrupted after the
  existing app cleanup hangs were confirmed. A subsequent full run used
  `-timeout=60s` to bound existing cleanup hangs; no tests were skipped or
  weakened in source. The root package (including C4/C5 CLI tests) passes.
  Sandbox blockers observed in the full suite:

| Package / test | Sandbox evidence |
| --- | --- |
| apps/status_api TestStatusAPI | Listener on :8090 denied; cleanup waits on an already-consumed process-exit channel and times out |
| apps/task_api TestTaskAPI | Listener on :8091 denied; same cleanup-channel timeout |
| examples TestHTTPServerServeHFBC | listen tcp :8080: socket: operation not permitted |
| internal/backend/gogen TestGoFetchRequiresGrantBeforeRequest | httptest loopback listen denied |
| internal/backend/javascript TestJSExecRequiresGrantBeforeSpawn | spawnSync touch EPERM |
| internal/backend/javascript TestJSExecGrantedPrintsCapturedOutput | spawnSync printf EPERM |
| internal/backend/javascript TestJSFetchRequiresGrantBeforeRequest | httptest loopback listen denied |
| internal/vm TestInterpretFetchDeniedBeforeRequest | httptest loopback listen denied |
| tools/difftest TestLoweredHFIRABIConformance | listen tcp 127.0.0.1:47653: socket: operation not permitted |

The two app artifact runners were also executed directly with their grants:
status_api and task_api each report structured IO_ERROR at HTTP_SERVER_SERVE
with the denied listener syscall. Full validation remains incomplete until
rerun in an environment permitting these syscalls; this journal does not claim
an unrestricted full-suite pass.

## Deferrals

No signing/attestation of receipts in v0. Receipts are not bound into sealed
ExecutionEvidence. AST interpreter receipts and a consumer experiment/harness
are deferred. No HFBC change, opcode registry change, DOM work, C6 work, or
production `-compile-bc`/HFIR flip. Improvement #90 remains Partial.
