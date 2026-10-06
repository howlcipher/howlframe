# 2026-10-06: Bytecode resource limits (C4b)

## Findings and change

- Review S4 found MaxMemoryBytes was declared but unenforced; S6 found
  instruction budgets did not bound blocking work; S7 found unbounded fetch
  bodies and exec output. C4b implements the three remaining C4 bullets for
  the bytecode runner. C4a CALL and SPAWN_AGENT depth behavior is preserved.
- MaxMemoryBytes stays 64 MiB. Allocation charges are cumulative, with no
  reclamation on overwrite or GC. A synchronized shared counter holds
  memoryBytes across synchronous SPAWN_AGENT, legacy asynchronous SPAWN, and
  HTTP handler child VMs. Counter addition and storage multiplication check
  overflow before charging. Non-positive memory ceilings deny positive charges.
- Charges approximate storage rather than live heap/RSS: concat/join strings
  use byte length; list/append/split slots use eight bytes each; list strings
  and new append strings add payload length; split adds part lengths; dict
  entries use sixteen bytes plus key lengths. Source bytes and converted
  any-slice storage are charged separately (nine bytes per input byte) for
  read_file, fetch, and exec. encode_json, string conversions, HTML escaping,
  model response strings, map_keys storage, and new map_set entries also charge.
  MAKE_LIST and MAKE_DICT temporary storage uses linear construction.
- Fetch reads through LimitReader at ceiling plus one (overflow safe), then
  rejects an oversized body. Exec uses CommandContext with a shared bounded
  stdout/stderr writer, cancels the process on overflow, and bounds pipe wait
  after cancellation or process exit when a deadline is configured. Without
  a deadline, under-cap inherited pipe behavior remains unchanged. The private
  bytes.Buffer cannot expose ReadFrom to bypass
  checked writes. Under-cap nonzero process exits remain IO_ERROR.
- Deadline is optional wall-clock ExecutionPolicy, separate from VMLimits.
  Absent means no deadline; explicit zero/negative/invalid CLI durations are
  rejected. The runner shares its context with children. Fetch and model HTTP
  requests (including ephemeral create/delete and lazy synthesis), exec, and
  sleep respect cancellation. The instruction loop checks before each opcode.
  Deadline errors become structured LIMIT_EXCEEDED rather than IO_ERROR.
- Positive byte ceilings are required on both CLI bytecode runners; direct VM
  non-positive fetch/exec ceilings fail closed. Numeric addition is unchanged.

## Runner options

| Flag | Default | Meaning |
| --- | --- | --- |
| --max-memory-bytes | 67108864 (64 MiB) | Cumulative approximate allocation charges |
| --max-fetch-bytes | 10485760 (10 MiB) | Fetch response body bytes |
| --max-exec-output-bytes | 10485760 (10 MiB) | Combined exec stdout/stderr bytes |
| --deadline | absent (none) | Positive duration, e.g. 50ms or 5s |

Byte ceilings are independent of allocation accounting: converted byte bodies
also consume the shared memory budget, which can bind before the I/O ceiling.

Both legacy -run-bc and run --target bytecode/bc apply these flags. Existing
--max-instructions and --max-call-depth retain their defaults and behavior.

## Verification

- VM tests cover defaults, concat/doubling, numeric addition, exact memory
  boundaries and overflow, each requested allocation opcode, temporary file
  input, spawn sharing, exact/under/over/fail-closed fetch and exec caps,
  large combined exec output, nonzero exit behavior, inherited output with
  no deadline, sleep/compute/exec
  deadlines with ample instructions, fetch cancellation, and stalled HTTP
  response bodies for fetch and model operations.
- Real httptest server tests remain in source. In-memory HTTP transport tests
  additionally exercise fetch caps and cancellation in listener-denied sandboxes.
- CLI tests compile artifacts through production -compile-bc and exercise both
  runners: memory limit failures, positive-ceiling validation, deadline expiry,
  invalid/zero duration rejection, and default trivial success. Existing C4a
  call-depth and instruction-limit tests pass unchanged.
- Required commands: `gofmt -l .`, `go vet ./...`,
  `go test ./internal/vm/ -count=1`, then `go test ./... -count=1`.
  Go version: 1.24.4.
- `gofmt -l .` printed nothing; `go vet ./...` passed.
- Targeted (also with `-race` during Codex sandbox work):
  `go test ./internal/vm/ -run 'ResourceDefaults|Memory|AllocationOpcode|FetchLimitsInMemory|Exec(OutputCap|LargeOutputAndExit|WithoutDeadline)|Deadline(SleepAndLoop|Model|HTTPResponseBodies)|CallDepth|SpawnAgent|Instruction|DefaultExecutionPolicy|EffectGateSource' -count=1`
  and
  `go test . -run 'TestRunBytecodeResourceFlags|TestRunBytecodeMaxCallDepthFlag|TestRunBytecodeMaxInstructionsFlag' -count=1`.
- Codex's first pass ran inside a workspace-write sandbox where loopback
  listeners and some JS `spawnSync` calls are denied, so a few fetch/HTTP/JS-exec
  and app server tests could not run there (same class of blockers as C4a).
  The full suite was rerun outside that sandbox:
  `go test ./internal/vm/ -count=1` and `go test ./... -count=1` both passed
  (all packages `ok`, including apps/status_api, apps/task_api, examples,
  gogen, javascript, vm, and tools/difftest).

## Deferrals and limits

AST interpreter memory/deadline limits remain unimplemented. Print output is
uncapped (S7 partial). read_line can still block beyond the deadline; HTTP
serving, SQL, and filesystem blocking operations are not context-cancelled
(S6 partial). Allocation charges cover selected construction sites, not every
Go runtime allocation; some charges occur after constructing the result,
including source reads and JSON decoding, so this is not an exact process
memory bound (S4 partial). Model response byte sizes have no dedicated cap.
Legacy SPAWN still has its historical asynchronous error isolation and separate
instruction accounting; its allocation counter and context now share the parent.
No C5 receipt/harness, opcode registry or HFBC change, production compiler flip,
DOM work, or historical journal rewrite. Improvement #90 remains Partial.
