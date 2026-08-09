# Bug #45 — fail closed on constructs the bytecode compiler cannot lower

**Date:** 2026-08-08
**Tracker:** `bugs.md` #45
**Execution mode:** `plan-then-reviewed-edit`
**Planning input:** `docs/journals/2026-08-07_standalone_blueprint_reconciliation.md`, `docs/standalone_runtime_blueprint.md`

## Problem

`internal/bytecode/bytecode.go`'s `compileNode` head-dispatch switch had no `default` case. Any head symbol it
did not recognize fell through the entire switch and returned an empty instruction slice, so the construct —
and everything nested inside it — was dropped from the artifact with no diagnostic and exit code 0.

```
$ go run howlframe.go -run tests/test_advanced_control.howl
{"reason":"\"match\" is not supported under -run in Phase 1 ..."}   exit 1   # interpreter fails closed
$ go run howlframe.go -compile-bc tests/test_advanced_control.howl -o /tmp/m.bin && go run howlframe.go -run-bc /tmp/m.bin
                                                                    exit 0   # no output; should print zero/one/other
```

Nothing downstream caught it. `checker.checkGoStatement`'s default must stay permissive for the Go backend's
open head set; `runHFIRGate` only blocks on `HFIR_INVALID_REF`/`HFIR_TARGET_INFEASIBLE` and `isFeasible` had rules
only for `wasm`; the VM's opcode `default` is fail-closed but unreachable because no instruction was emitted;
and `tools/difftest` exempts exactly the fixtures where this happens.

## What changed

| File | Change |
| --- | --- |
| `internal/construct/construct.go` | **new** — the authoritative table, the parent-scoped sub-form set, and `Scan` |
| `internal/construct/construct_test.go`, `scan_test.go` | **new** — registry invariants and one case per false-positive trap |
| `internal/hfir/constructs.go` | **new** — `VerifyConstructs` emits `HFIR_TARGET_INFEASIBLE` |
| `internal/hfir/constructs_test.go` | **new** — target scoping, diagnostic shape, sub-form acceptance |
| `internal/bytecode/bytecode.go` | fail-closed `default:` backstop; explicit cases for `type_hints` / `type_param` |
| `internal/bytecode/construct_drift_test.go` | **new** — drift test that parses the real `switch head` |
| `howlframe.go` | `runHFIRGate` merges construct diagnostics into the existing blocking-code filter |
| `howlframe_test.go` | 6 new CLI-level tests, including the corpus partition guard |
| `README.md`, `bugs.md` | accuracy corrections and the Done note |
| `internal/vm/cli_test.go` | pre-existing `gofmt` violation at HEAD, fixed in passing |

Design: `internal/construct` (AST → violations) ← `internal/hfir/constructs.go` (violations → diagnostics) ←
`howlframe.go` `runHFIRGate`. `internal/construct` imports only `internal/ast`, mirroring `internal/capability`'s
backend-independence (the precedent from improvement #94). `internal/hfir` does not import `internal/bytecode`.

## The finding that changed the design

`bugs.md` #45's fix sketch prescribed a deny-list over **HFIR node kinds** inside `isFeasible`. That matching
surface turned out to be insufficient, verified against the code:

1. **`catch` is a sub-form of the supported `try_let`.** `bytecode.go:136-149` destructures `node.Children[2]`
   itself, so `catch` never reaches `compileNode` in head position — but `hfir.LowerAST` (`lowering.go:64-66`)
   still emits a node with `Kind == "catch"`. Denying it breaks all 6 `try_let` fixtures.
2. **`route` and `lambda` are sub-forms of the supported `http_server`** (`bytecode.go:252-265`) — same trap.
3. **`tests/optimization_signature.howl` uses `(test "go test ./...")` as `optimize_signature` metadata**, not
   as a test block. That fixture is *not* exempt in `tools/difftest/manifest.json`, so a blanket `test` denial
   would have newly failed difftest.

Plus the caveat already in the tracker: `LowerAST` names a node's `Kind` after the head of every list, so
`(let (match 1) x)` yields `Kind == "match"`.

The AST still carries parent and child position, so `construct.Scan` classifies a list's head only when the
list actually sits in construct position — not when it is a let binding, a parameter list, a dict pair, or a
sub-form its parent destructures. `TestVerifyConstructsMatchesLoweredKindsWouldNot` pins this rationale in
place: it asserts `LowerAST` really does produce a `catch`-kinded node while `VerifyConstructs` accepts the
program, so the reasoning fails loudly if lowering ever changes.

Confirmed with the user before implementing, along with the strict classification of `struct` and `go_import`
as `Unsupported` (deviating from fix-sketch step 4).

## The test that could not do the job

`TestHFIRGateAcceptsAllExistingFixtures` (`howlframe_test.go:919`) was the designated empirical check. It runs
`-validate`, which passes `hfirTargetNone` (`""`), and `Verify` only applies target rules for a non-empty
target — so it never reaches the construct check at all. `TestCompileBcCorpusPartitionMatchesRegistry` was
added for that role: for every tracked fixture it asserts `-compile-bc`'s exit status agrees exactly with
`construct.Scan`, so a false positive and a bypassed gate both fail the build.

## Two false positives the empirical loop caught

Running the corpus before writing any test surfaced both immediately:

1. **`defun` was missing from the table**, so every fixture defining a function was rejected as an unknown
   head. Added.
2. **`type_hints` and `type_param` had no `compileNode` case at all.** They "worked" only because the missing
   `default` silently dropped them — precisely the accident this bug describes. They now have explicit empty
   cases alongside `type_hint`, and `TestCompileTimeOnlyConstructsReachingCompileNodeHaveCases` prevents the
   next annotation from repeating it.

## Verification

```
go build -o <scratch>/howlframecheck howlframe.go     OK
go vet ./...                                OK
go test ./...                               ok (all packages)
gofmt -l .                                  empty
go run tools/difftest/main.go               Passed: 16, Skipped: 26, Failed: 0
```

The difftest result is identical to a run of the same harness from an unmodified `HEAD` worktree.

CLI behaviour:

| Command | Result |
| --- | --- |
| `-compile-bc tests/test_advanced_control.howl` | fails, `HFIR_TARGET_INFEASIBLE` naming `match`, **no artifact** |
| `-compile-bc tests/test_void_defun.howl` | fails, naming `test` and citing `improvements.md #96`, no artifact |
| `-compile-bc` on an invented head | fails, naming the head, no artifact |
| `-compile-bc examples/cli_hello.howl` → `-run-bc` | succeeds, prints `Hello, World!` / `Welcome to HowlFrame` |
| `-compile-bc tests/optimization_signature.howl` | still succeeds (its `(test ...)` is metadata) |

**Benchmark gate (Working Protocol #9).** This touches `test`-block and `type_hint` handling, so the gate
applies. No language-surface change was made, and the byte-identical-output proof was produced instead of
re-running `benchmarks/language_write_cost/`: all **30** fixtures that still compile produce a semantically
identical bytecode program before and after, compared by gob-decoding both artifacts and normalizing the
`Functions` map order.

Raw byte comparison is *not* a valid oracle here. `BCProgram.Functions` is a Go map, so gob encoding order
varies run to run — an unmodified `HEAD` binary produced two different digests for
`tests/test_return_compound.howl` across three consecutive runs. That artifact nondeterminism is pre-existing
and belongs to improvement #98 (artifact validation, hashing, versioning), not to this item.

14 fixtures newly fail closed: `test_advanced_control`, `test_ai_primitives`, `test_feature`, `test_generics`,
`test_let_arity`, `test_middleware`, `test_new_features`, `test_primitives`, `test_schema`, `test_swarm_js`,
`test_trace`, `test_void_defun`, `test.howl`, and `examples/wasm_math`. Every one is already exempt in
`tools/difftest/manifest.json` or skipped by the HFIR fixture test, which is why difftest is unchanged. Each
names a construct with genuine runtime meaning the VM cannot express — that visibility is the deliverable.

`tests/routes.howl` and `tests/test_include.howl` still fail earlier, in `checker.Check`, on bug #43's
module-resolution gap. Unchanged by this work, and excluded from the partition test with that reason recorded.

## Deliberately not done

`match`, `test`, `use`, `export` and modules remain unimplemented in the VM (#95/#96). Bytecode lowering still
comes from the AST, not HFIR (#90). No opcode was added, removed, or renamed, so
`docs/reference/bytecode_reference.md` needed no regeneration. `hfirBlockingCodes`' policy is unchanged and
`HFIR_UNBOUND_REF` remains excluded (#42). The gate was not extended to `-validate`, which is target-independent
by contract. Artifact hashing/versioning is untouched (#98). This item makes the gap visible; it does not close
it.

## Follow-on candidates (not filed)

- Artifact encoding is nondeterministic because `BCProgram.Functions` is a map. Worth folding into #98's scope
  rather than filing separately.
- `-validate` remains target-independent and therefore still accepts programs no target can run. A `-target`
  flag for `-validate` would close that, but it is a CLI surface change, not part of this bug.
