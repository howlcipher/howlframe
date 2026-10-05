# 11: Implementation candidates

**Done in this PR:** C0. Everything else is a plan with enough detail to
start, in priority order. Each is small, keeps artifacts compatible, and
leaves the production compiler, #90, and the tip-lock alone.

## C0. Model-call capability gates (DONE: commit 1 of this PR)

- `OpConfidence`, `OpNeuralCircuit`, `OpEphemeralCircuit` declare
  `capability.Network`. `ForConstruct` returns `network` for the three
  constructs. The interpreter checks `network` before the
  `lazy_synthesize` request.
- Tests: `internal/vm/model_call_capability_test.go` (a recording probe on
  the hard-coded model host counts requests, with a granted-call positive
  control) plus additions to `TestCapabilityGatePerKind` and
  `TestForConstruct_Known`. On the old code the new tests fail.
- bugs.md #57. Journal `docs/journals/2026-10-05_model_call_capability_gates.md`.

## C1. Required-capabilities report (P0.5): S

- **API:** `bytecode.RequiredCapabilities(prog *BCProgram) Report`, where
  `Report{Capabilities []string; Sites []Site}` and
  `Site{Function string; Index int; Opcode string; Capability string; Reason string}`.
- **CLI:** `howlframe inspect --caps app.hfbc` (or `-required-caps`)
  prints deterministic JSON. It never executes anything.
- **Soundness rule:** the report must cover every `requireCapability`
  call site in `internal/vm/vm.go`:
  1. `spec.Capability` for each instruction in `Main` and every function.
     Embedded TRY_LET, SPAWN_AGENT, and route bodies are inline, so a
     linear scan covers them.
  2. `STORE_OPEN` operand URI through `capability.StoreRequirements`
     (`file://` adds `filesystem`).
  3. `CALL` to a function with `LazySynthesize` adds `network`.
  4. File-backed store ops (`STORE_PUT`/`GET`/`DELETE`/`KEYS` on a
     `file://` handle) are covered by rule 2.
- **Test:** for every fixture in `tests/` and `apps/` that compiles, run
  the VM with exactly the reported grant. It must never hit
  `CAPABILITY_DENIED` on paths the fixture exercises. Removing any one
  reported capability must produce a denial for at least one fixture that
  uses it. Also add a source-scan test asserting the number of
  `requireCapability(` sites in `vm.go` matches the rules above, so a new
  site forces an update.

## C2. Governed profile v0 (P0.3): S/M

- `internal/construct`: add `Profile` sets (`default`, `governed`). The
  governed set is the Supported constructs minus the list in 08 P0.3.
- `howlframe check|build --profile governed`: reuse the
  `HFIR_TARGET_INFEASIBLE`-style diagnostic with a new code
  `PROFILE_FORBIDDEN_CONSTRUCT`, naming the construct and location.
- No change to the default profile or to existing programs.
- Tests: each excluded construct is rejected under `governed` and still
  compiles under `default`. Apps that should be governed-clean (for
  example `action_executor`) pass.

## C3. Effect-gate conformance test (P0.2): S

- A Go test that parses `internal/vm/vm.go` with `go/ast`, maps each
  `case bytecode.OpX:` body to the set of selector calls it makes
  (`http.*`, `os.*` except `os.Stdin`/`Stdout`/`Stderr`, `exec.Command`,
  `sql.*`, `net.*`), and fails if that opcode's registry capability is
  empty and the opcode is not in an explicit ambient allow-list (`SLEEP`,
  `TIME_NOW`, `READ_LINE`, `PRINT`, `STDERR`, `EXIT`).
- Would have caught bugs.md #44 and #57.

## C4. Resource limits (P1.1): M/L, needs a decision

- Enforce `MaxCallDepth` on `CALL`. **Decision needed**: 128 may break
  legitimate recursion. The suggestion is to make it runner-configurable
  like `--max-instructions`, with a default of 1,000.
- Allocation accounting on string concat, `append`, `MAKE_LIST`,
  `MAKE_DICT`, `str_split`, `str_join`, `read_file`, and `fetch` bodies
  against `MaxMemoryBytes` (runner-configurable). Fail with structured
  `LIMIT_EXCEEDED`.
- A `--deadline` runner flag. A `context.Context` threaded into `fetch`,
  `exec`, `sleep`, and model calls.
- Caps on `fetch` body bytes and `exec` output bytes.

## C5. Execution receipt v0 (P1.2): M

- `-run-bc --receipt out.json`. The receipt holds `{schema:"howlframe.receipt/v0",
  artifact_sha256, compiler_version, grant, limits, instructions_used,
  effects:[{op, capability, target_summary, decision}], exit}`.
- `target_summary` is redacted by policy. For example `fetch` records the
  host but not the query, and `env` records the variable name but not its
  value.
- Written by the runner, never by program code, so `print` cannot forge it.

## C6. `and`/`or` semantics (P1.4): S/M, needs a decision

- Choose short-circuit (Go and JS already do this, and it matches what
  users expect). Change the VM, interpreter, and AST bytecode compiler
  together. Add a differential case with an effect on the right operand.
  Check the experimental lowerer as well (it must stay in parity, but
  production is unaffected by the experimental path).

## C7. Demo-app output integrity (P0.6): S

- In `apps/release_authority` and `apps/action_executor`, build the
  decision as a `dict` and print it with `encode_json`, which escapes
  strings. Today both apps print it with `str_join` and do not escape.
  The output's key order and whitespace may change, so update the app
  tests to parse JSON instead of matching substrings.
- New test: a proposal whose `target` contains `", "decision": "ALLOW`
  must produce exactly one `decision` key, with the VM's real value.
  This was reproduced in this review: Python reads `ALLOW` when the VM
  decided `DENY`.
- Optional: pre-flight `stage_artifact` capabilities (all grants checked
  before the first effect), or reword the doc to say "ordered, not atomic".
- This was not done in this PR because it changes demo-app output
  formatting. That is a behavior change consumers might match on, so it
  needs William's call.

## Not recommended now

- Wiring the HFIR model adapter to the CLI (P3.1) before 09 shows demand.
- Any change to the HFBC wire format.
- Removing `lazy_synthesize` or the circuit primitives outright. Quarantine
  them by profile so existing fixtures and HowlBoard keep working.
