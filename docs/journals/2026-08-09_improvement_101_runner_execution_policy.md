# Improvement #101: runner-controlled execution policy and instruction budgets

## Objective

Resolve the next Repo Analyst dogfooding blocker without raising the standalone VM's default instruction ceiling or giving HowlFrame source authority over runtime policy. Establish one small expansion boundary for future runner-owned budgets while implementing only a finite instruction budget today.

## Preflight and tracker decision

The worktree started clean on `main` at `5cffafc`. Recent history contains improvement #97, bugs #45 and #43, improvement #95, bug #46, and the Repo Analyst reference application.

The required search of `improvements.md` and `bugs.md` covered `instruction limit`, `execution limit`, `resource limit`, `VMLimits`, `VM limits`, `budget`, `runtime policy`, `execution policy`, `runner policy`, and `resource policy`. No existing item owns a trusted-runner instruction override. Legacy V2 item #37 describes a source-level token/cost `with_budget` primitive and is not an owner for runtime instruction authority. Bug #46 only records this gap as the next boundary. Improvement #101 was therefore added as the one tracker item for this session.

## Current implementation evidence

`internal/vm/error.go` defines:

```text
VMLimits
  MaxInstructions = 100000
  MaxMemoryBytes  = 67108864
  MaxCallDepth    = 128
```

Only `MaxInstructions` is currently enforced. `RunBytecode` constructs every top-level VM with `DefaultLimits`; tests can instantiate `BCVM` with another `VMLimits` value, but no public runner entry point accepts one.

The dispatch loop increments `executed` once per instruction and rejects when `executed > MaxInstructions`. For a positive value `N`, exactly `N` instructions may execute and the attempted `N+1` instruction fails before its opcode dispatch. A zero or negative VM value executes no instruction. There is no unlimited sentinel. Recursive `run` calls used by functions and embedded `try_let` regions share the parent VM counter. Spawned and HTTP-handler child VMs currently receive a copied limit and a fresh counter; changing aggregate accounting is outside this item.

Capability enforcement is a separate `AllowedCaps` check. It runs for every capability-bearing opcode and is not derived from `VMLimits`. The direct AST interpreter has no equivalent instruction-limit mechanism; this item is restricted to the primary standalone bytecode runtime.

The tracked and documented CLI is `howlframe.go`. `cmd/howlframe/main.go` is ignored by `.gitignore`, absent from `git ls-files`, lacks the checker and capability policy, and is explicitly classified by improvement #85 and `docs/architecture_roadmap.md` as non-product legacy scaffolding. It is not an equivalent runtime entry point and will not be promoted or presented as one in this item.

## Architecture options

### Add a scalar `maxInstructions` parameter

Pros: minimal immediate code.

Cons: extends an already long runner signature and encourages one parameter per future wall-clock, token, cost, or model-call budget. It obscures that the value is trusted runtime policy rather than application input.

Decision: rejected.

### Change `DefaultLimits` or add a mutable global override

Pros: almost no call-site work.

Cons: changes behavior process-wide, creates shared mutable policy, makes concurrent execution unsafe, and risks solving Repo Analyst by weakening every application default.

Decision: rejected.

### Add a small `ExecutionPolicy` and a limits-aware entry point

Pros: preserves the existing default entry point, keeps future resource dimensions inside one runner-owned value, reuses `VMLimits`, and leaves capability selection independent.

Cons: adds one API entry point and requires callers that need an override to construct policy explicitly.

Decision: chosen. `ExecutionPolicy` contains only `Limits VMLimits`; `DefaultExecutionPolicy` returns the current defaults; `RunBytecode` remains the default-safe wrapper; and `RunBytecodeWithPolicy` is the explicit trusted-runner path. The zero policy value remains fail-closed and no unlimited representation is added.

## Security invariant

> Execution policy may constrain an application, but application code may not widen its own execution policy.

The CLI parses the finite ceiling before bytecode execution. The policy is not serialized into source, HFIR, bytecode, or the artifact. Capabilities remain separately selected and enforced.

## Test-first plan

1. Add direct VM tests for default preservation, insufficient, exact, greater, zero, and negative budgets, plus the limits-aware API.
2. Add real CLI tests for omitted, explicit, malformed, zero, and negative values.
3. Extend the capability CLI test to prove a large instruction budget does not grant environment access.
4. Extend Repo Analyst with a generated deterministic workload that exceeds the default, then prove the same artifact succeeds with a finite explicit budget.
5. Implement only the policy struct, entry-point threading, CLI validation, and documentation needed to make those tests pass.

## Implementation

`internal/vm` now exposes a deliberately small runner boundary:

```go
type ExecutionPolicy struct {
    Limits VMLimits
}

func DefaultExecutionPolicy() ExecutionPolicy
func RunBytecodeWithPolicy(..., policy ExecutionPolicy, allowedCaps []capability.Capability, ...)
```

The existing `RunBytecode` entry point remains source-compatible and delegates through `DefaultExecutionPolicy`, so callers that do not opt into policy selection keep the exact existing `VMLimits` defaults. The policy value contains no application-derived data, and its zero value permits no instruction execution. Capabilities remain a separate runner grant rather than being inferred from or widened by the resource policy.

The VM dispatch loop now states and enforces the positive-budget contract directly: `MaxInstructions` is the maximum number of instructions that may dispatch. A budget of `N` permits exactly `N` instructions; if another instruction is present, the attempted `N+1` instruction raises structured `LIMIT_EXCEEDED` before dispatch. Checking before increment also prevents a maximum machine integer from wrapping the counter into effectively unlimited execution. At the direct VM API boundary, zero and negative limits permit no instruction. The CLI rejects those values before reading or executing bytecode.

The tracked `howlframe.go` CLI adds `--max-instructions` only as runner input to `-run-bc`. Omission initializes it from the current 100,000-instruction default. Positive machine-representable integers become the one overridden field in a copy of `DefaultExecutionPolicy`; zero and negative values return a diagnostic; malformed and overflowed values are rejected by flag parsing. No sentinel for unlimited execution exists.

No source form, checker rule, HFIR node, bytecode opcode, serialized artifact field, or capability mapping changed. The architectural blueprint was reviewed but not edited because its bounded-execution and deterministic-authority design already describes this boundary and is intentionally not an implementation-status document.

## Test evidence

Tests were written before the production API. The initial focused VM test failed to build because `ExecutionPolicy`, `DefaultExecutionPolicy`, and `RunBytecodeWithPolicy` did not yet exist. After the implementation:

- direct VM tests prove insufficient, exact, and greater budgets, structured `LIMIT_EXCEEDED`, the unchanged 100,000 default, and fail-closed zero/negative API values;
- real CLI subprocess tests prove the omitted default, explicit smaller/exact/greater values, zero, negative, malformed, and integer-overflow rejection, and that the application produces no output before a rejected policy fails;
- the capability CLI test gives a program 1,000,000 instructions without its required environment capability and still receives `CAPABILITY_DENIED`;
- the Repo Analyst integration test compiles one artifact, deletes its HowlFrame source copy, and uses a generated 20,000-byte deterministic repository. The default run receives structured `LIMIT_EXCEEDED`; the same artifact under an explicit 1,000,000-instruction runner policy produces the exact report and exits zero.

## Manual dogfooding proof

A scratch compiler built from this worktree compiled the unchanged five-module Repo Analyst application once. At the time of the final run, `examples/repo_analyst/` contained seven files totaling 22,665 bytes.

Without an override, the artifact exited 1 with:

```json
{"phase":"runtime","code":"LIMIT_EXCEEDED","function":"main","instruction":6,"opcode":"LOAD_CONST","message":"instruction limit exceeded"}
```

The same artifact, path, and capabilities under `--max-instructions 1000000` exited 0 and reported seven total files, five HowlFrame files, one Go file, seven text files, four `TODO` markers, three `FIXME` markers, and the deterministic largest file. The source modules were not needed at execution time.

The same 1,000,000-instruction policy with no capability grant exited 3 at `EXEC` with structured `CAPABILITY_DENIED` for `process`, confirming that resource authorization does not imply external authority.

As a larger non-CI dogfooding check, the same artifact analyzed the full HowlFrame repository under an explicit 20,000,000-instruction policy in 1.47 seconds and exited 0, reporting 211 total files and 190 text files. This result is evidence only; committed tests do not depend on incidental repository contents.

## Deferred findings

- `MaxMemoryBytes` and `MaxCallDepth` are fields in `VMLimits` but are not currently enforced. This item does not claim otherwise and does not add those policy dimensions.
- Recursive function and embedded-region execution share one `BCVM` counter. Existing spawned-task and HTTP-handler child VMs copy the configured limit but start fresh per-VM counters; request-wide concurrent accounting remains separate future trust-boundary work.
- The direct AST interpreter has no equivalent instruction policy. This session is limited to the primary standalone bytecode runtime.
- `cmd/howlframe/main.go` remains ignored legacy scaffolding, not a tracked or equivalent product entry point. Promoting it would require the broader entry-point unification work already excluded by improvement #85's recorded boundary.

## Verification record

- Focused VM policy tests: passed.
- Focused real-CLI policy and capability-independence tests: passed.
- Focused Repo Analyst integration: passed.
- `go test ./internal/vm`: passed.
- `go test ./examples/repo_analyst`: passed.
- `go test ./...`: passed across all packages.
- `go vet ./...`: passed.
- `go build -o /tmp/howlframe-execution-policy-check howlframe.go`: passed.
- `go run tools/difftest/main.go`: 17 passed, 27 intentionally skipped by the manifest, 0 failed.
- `gofmt` and `git diff --check`: passed after the implementation edits; repeated after final documentation edits.
- Benchmark gate: not applicable. This change does not alter a benchmark-named source-writing surface, grammar, checker behavior, lowering, or emitted program; it only selects an existing runtime instruction ceiling at the trusted runner boundary.

## Outcome

Improvement #101 is Done. The default remains bounded, legitimate larger workloads require an explicit finite runner authorization, invalid policy fails closed, and application code gains no mechanism to raise its own budget or capabilities. This is not ordinary-language parity work: it strengthens the deterministic authority surrounding programs that may later contain adaptive behavior, without implementing any probabilistic or self-modifying feature today.
