# Improvement #87 Phase 2 — Wire ZIR verification into the production compiler pipeline
Date: 2026-08-05

## Task Summary
Improvement #87 ("Build the ZIR verifier and versioned diagnostic contract") was marked Done (2026-08-05), but a repo-wide grep confirmed `zir.LowerAST`/`zir.NewVerifier`/`.Verify()` were called only from `internal/zir/*_test.go` — zero production call sites. `zero.go` drove `checker.Check` directly and every backend ran on the checker-annotated AST with no ZIR involvement at all, contradicting #87's own "Why" ("Verification must be a product boundary, not a backend side effect") and its "pre-emission target/effect rejection" acceptance criterion.

This phase makes ZIR verification a real, enforced boundary for source-based compiler paths: `-validate`, `-compile-bc`, `-compile-wasm`, `-run`, legacy `wasm_app`, `web_app`, and the default Go backend. `-mask-plan`/`-optimization-plan` are explicitly exempt (they consume only `checker.Analysis`, never touch the AST, and write no artifacts). `-run-bc` is explicitly out of scope — it decodes a pre-compiled bytecode artifact and never runs the lexer/parser/checker at all.

## Details

### Prerequisite bug fixes (bugs.md #41)
Before any real program could be gated, two lowering gaps had to be fixed in `internal/zir/lowering.go`:
1. **Integer literals crashed lowering.** The leaf-type check tested for AST type `"NUMBER"`, but the lexer emits `"INT"` (confirmed via `internal/lexer/lexer.go`'s `TokenType` constants, and consistent with every other consumer in the codebase). Any program containing an integer literal failed with `unknown AST node type: INT`. One-token fix.
2. **Zero-argument parameter lists crashed lowering.** `lowerNode` only handled non-empty `List` nodes; a `defun`/`lambda`'s empty `()` parameter list has zero children and fell through to the same unknown-node-type error path. Fixed by adding an explicit `len(Children) == 0` case producing a trivial `"empty_list"`-kind leaf node.

Both gaps were invisible to the package's own prior tests (`graph_test.go`, `verifier_test.go`) because those tests exclusively used hand-built `ast.Node`/`zir.Node` literals rather than real lexer/parser output. New tests in `internal/zir/lowering_test.go` close that gap by running real source through `lexer.NewLexer`/`parser.NewParser` before lowering.

### The false-positive discovery (bugs.md #42)
Running the new `-validate`-based ZIR gate against the full `tests/*.zero` corpus (before any gate-composition decision) surfaced a much larger issue than anticipated: `ZIR_UNBOUND_REF` false-positived on 12 of the corpus's fixtures. Root cause: `internal/checker/types.go`'s `Analysis.infer` sets a bare `SYMBOL`'s `Inferred` type to `ast.Unknown` for two different reasons — a genuinely undefined identifier (which already fails `checker.Check` with its own diagnostic before ZIR ever runs) and a variable legitimately bound via `try_let`/`catch` or bound to a dynamically-typed primitive's result (`llm_generate`, a generic function's type parameter, an HTTP lambda parameter's field access) — the latter is intentional, checker-sanctioned dynamic typing, not an error. The checked AST's `TypeInfo` does not preserve which case produced `Unknown`, so on real programs `ZIR_UNBOUND_REF` currently has no reachable true positive, only false positives.

Given the task's explicit instruction to never mark unsupported coverage as verified, the production gate (`zero.go`'s `zirBlockingCodes`) does **not** treat `ZIR_UNBOUND_REF` as blocking. `internal/zir/verifier.go` itself is unchanged — `Verify()` still computes and returns this diagnostic — the exclusion lives only in the production gate's failure policy. Filed as bugs.md #42 (Pending); a real fix needs the checker to preserve "resolved-but-dynamic" vs. "never resolved" as distinct states, which is a checker-semantics change explicitly out of scope for this phase.

### Gate composition
`zero.go`'s `runZirGate` treats only two diagnostic codes as blocking:
- `ZIR_INVALID_REF` (dangling data/control-edge reference)
- `ZIR_TARGET_INFEASIBLE` (real rules exist only for `target == "wasm"` in `internal/zir/verifier.go`'s `isFeasible`)

Capability-effect inference still runs as a `Verify()` side effect (mutating node `Effects`) but never itself produces a blocking diagnostic in this phase.

A small `zirTarget` type in `zero.go` maps each CLI mode to a verifier target identity (`zirTargetWasm`, `zirTargetBytecode`, `zirTargetInterpreter`, `zirTargetJavaScript`, `zirTargetGo`, `zirTargetNone` for `-validate`). Only `"wasm"` has real feasibility rules today; the others are accepted permissively by `isFeasible` — the identities exist for consistency, documentation, and future extension, not because per-target coverage exists yet for all of them.

Diagnostics are reported via a new `reportZirDiagnostics` helper (in `zero.go`, not `internal/ast` — `internal/zir` already imports `internal/ast`, so the reverse import would cycle). It prints **all** blocking diagnostics as one JSON array, preserving `Verify()`'s deterministic node-order-derived ordering, then exits 1 — unlike `ast.ReportError`'s single-object contract, since there's no existing consumer of ZIR diagnostics to stay wire-compatible with.

### Diagnostic contract extension
`internal/zir/diagnostic.go`'s `Diagnostic` gained two fields: `ContractVersion` (new const `DiagnosticContractVersion = "v1"`, versioning the diagnostic wire format independently of `Graph.Version`, which versions the graph schema — the two happen to both start at `"v1"` but track different things) and `Target` (`omitempty`, so target-independent `-validate` diagnostics don't carry a misleading empty field). This is the smallest compatible extension satisfying #87's "versioned diagnostic contract" framing and the Phase 2 brief's requirement to preserve the selected target.

### Capability metadata duplication
`internal/zir/verifier.go`'s `getCapability` duplicates `internal/bytecode/opcode.go`'s `OpInfo` capability table. Preserved as-is (not refactored) — merging them would mean either a backwards layering dependency (`zir` importing `bytecode`) or refactoring `bytecode.go`'s opcode switch, both judged out of scope ("do not turn this integration task into a full effect-system rewrite"). Filed as improvements.md #94 (Pending), depends on #87.

### Newly discovered, unrelated bug (bugs.md #43)
The fixture sweep also surfaced `tests/test_include.zero` failing `checker.Check` (before any ZIR code runs) with an unresolved module-imported function reference. Unrelated to ZIR — a pre-existing module-system (#93) gap — documented but not fixed this phase, and excluded from the fixture-sweep test alongside the already-known `tests/routes.zero` (an include-only fragment with no standalone root).

## Files changed
- `internal/zir/lowering.go` — INT/NUMBER fix, empty-list handling.
- `internal/zir/lowering_test.go` — new file; real lexer/parser-backed regression tests (moved `TestASTLowering` here from `graph_test.go` and fixed its fixture).
- `internal/zir/graph_test.go` — `TestASTLowering` moved out.
- `internal/zir/diagnostic.go` — `ContractVersion`/`Target` fields, `DiagnosticContractVersion` const.
- `internal/zir/verifier.go` — thread new diagnostic fields; documentation comments on the unbound-reference false-positive class, capability-metadata duplication, and `isFeasible`'s wasm-only coverage.
- `internal/zir/verifier_test.go` — extended existing subtests to assert the new fields; added `TestVerifierIsIdempotent`, `TestVerifierDiagnosticOrderIsDeterministic`.
- `zero.go` — `zir` import, `zirTarget` type + constants, `runZirGate`, `zirBlockingCodes`, `reportZirDiagnostics`, call sites for every in-scope mode.
- `zero_test.go` — `TestZirGateAcceptsAllExistingFixtures`, `TestZirGateRejectsTargetInfeasibleWasmConstructBeforeArtifact`, `TestZirGateDiagnosticOrderingIsDeterministic`, `TestValidateModeRunsZirGateAndHasNoSideEffects`, `TestZirGateForCompileBcDoesNotRegressValidPrograms`.
- `bugs.md` — #41 (Done), #42 (Pending), #43 (Pending).
- `improvements.md` — #87 phased status note, new #94 (Pending).

## Verification
- `go test ./...` — all packages green (including the full `TestZirGateAcceptsAllExistingFixtures` sweep over `tests/*.zero`, and the full pre-existing suite unmodified).
- `go vet ./...` — clean.
- `gofmt -l .` — clean.
- `git status --porcelain` — only the intended files touched; no stray generated artifacts.
- All manual builds during verification used `go build -o <scratch-path> .`, never bare `go build .` in the repo root.

## Compatibility
No existing valid fixture's observable behavior changed. The two fixtures the sweep found already broken (`tests/routes.zero`, `tests/test_include.zero`) fail at `checker.Check`, before any ZIR code runs — unrelated to this phase.

## Remaining risks / follow-ups
- bugs.md #42 (`ZIR_UNBOUND_REF` false positives) needs a checker-semantics fix before that diagnostic can ever be enforced.
- Real control-flow/cycle verification is not implemented (`ControlEdges` is declared but never populated by `LowerAST`).
- Non-wasm target feasibility (`isFeasible`) has no real rules yet — `-compile-bc`'s ZIR gate currently cannot reject any real program (documented in `TestZirGateForCompileBcDoesNotRegressValidPrograms`).
- improvements.md #94 (capability metadata centralization) is filed but not scheduled.

## Does this unblock #88?
Partially. #88 (provider-neutral ZIR model-adapter protocol) can now target a real production boundary (`runZirGate`) for the covered diagnostic subset (`ZIR_INVALID_REF`, `ZIR_TARGET_INFEASIBLE`), but should not assume broader coverage (unbound-reference enforcement, control-flow verification, non-wasm feasibility) until #42 and the documented follow-ups land.
