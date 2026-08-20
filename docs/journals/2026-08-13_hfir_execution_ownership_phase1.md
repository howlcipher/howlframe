# Goal

Establish the first deterministic executable path that lowers verified HFIR directly to standalone bytecode, without rebuilding an AST or falling back to the legacy AST bytecode compiler.

# Starting SHA

`8b529d9a8118dafdb9bdd5122f943ffcdc5782c8` (`origin/main`, merged PR #12). The initial local checkout was on a stale fix branch; `git fetch origin --prune` established this live baseline before the feature worktree was created.

# Architectural hypothesis

HFIR can own a useful deterministic bytecode subset if lowering records semantic operand roles and literal/name values, and a new bytecode lowerer consumes those graph nodes directly. Semantic HFIR must not contain bytecode opcode identities.

# Current execution pipeline

For source builds, `howlframe.go` parses, expands/resolves source constructs, applies AST transforms, checks the AST, then calls `runHFIRGate`. That gate calls `hfir.LowerAST` and `Verifier.Verify`, but `buildSource` subsequently calls `bytecode.CompileToBytecode(root)`: the AST still drives bytecode emission. The VM executes the resulting `BCProgram`.

# Current semantic owners

The AST bytecode compiler owns construct shape interpretation, argument order, literal conversion, variable naming, control-flow jump layout, function layout, and opcode selection. HFIR currently preserves node kind, ordered data references, inferred type, effects, and source provenance, but does not preserve semantic operand roles for structural forms such as `let` or `dict`.

# Phase-1 scope

Select only deterministic `cli_app` policy/application logic whose source form can be represented directly as semantic HFIR: constants, variable references, `do`, `let`, `set`, `if`, binary operations, list and dictionary values, collection reads/mutations, conversions, string split/join, and observable `print`, `stderr`, and `exit`, subject to the direct compiler and tests.

# Explicit non-goals

No new syntax, model-provider integration, AST reconstruction, silent fallback, whole-backend rewrite, networking/database expansion, or broad HTTP redesign. Function definitions/calls, loops, `try_let`, and external effects are outside the initial slice unless evidence shows they fit cleanly without scope expansion.

## Investigation

Fetched the live remote and created isolated feature and review worktrees. Inspected `internal/hfir`, `internal/bytecode`, `internal/vm`, `internal/capability`, `internal/construct`, and the CLI build wiring. The required no-edit baseline test `go test ./...` passed on the feature worktree at the starting SHA.

## Evidence

`buildSource` calls `runHFIRGate` at `howlframe.go:612`, then calls `bytecode.CompileToBytecode(root)` at `howlframe.go:614`. `internal/hfir/lowering.go` assigns list node kind from its head and appends anonymous `DataInputs`; `internal/bytecode/bytecode.go` destructures AST children directly. `internal/vm/vm.go` executes `BCProgram.Main`.

## Decision

Keep public `howlframe build` unchanged for Phase 1 unless the direct lowering evidence establishes a sufficiently broad safe contract. Expose the new path as an explicit internal API and verify it differentially in CI; this prevents invisible AST fallback.

## Remaining semantic ownership

All executable bytecode semantics remain AST-owned at this point. HFIR owns verifier effect inference and provenance only.

## Next step

Complete independent architecture and consumer reviews, then define the exact HFIR node contract and direct bytecode lowerer.

## Investigation

Independent architecture review confirmed that anonymous HFIR edges could not distinguish a binding, dictionary pair, catch clause, parameter list, or ordinary argument. Consumer review found HowlChangeOps needs 23 runtime constructs plus `catch`; HowlBoard needs functions, HTTP routes/lambdas, request parsing, stores, and loops. Differential-test review recommended in-process isolated `BCVM` execution because the public runner exits the process on a structured VM error.

## Evidence

Architecture review executed simple and representative baseline bytecode artifacts successfully and located the production AST compiler call after the HFIR verifier gate. Consumer review ran HowlChangeOps check/build, adapter build/vet/test, integration/adversarial/authority-bypass scripts successfully. HowlBoard `make test` passed with the candidate binary; its browser test was blocked by an absent local Playwright Chromium executable after backend/frontend build completed.

## Decision

Normalize only semantic roles required by the deterministic core rather than recovering AST child positions in the new compiler. Keep `defun`, `call`, `return`, loops, `try_let`, JSON parsing, HTTP, stores, and broad effects outside Phase 1. Retain public AST compilation with no auto-detection or fallback and describe the direct path accurately as internal experimental coverage.

## Implementation

Added `LiteralKind` and graph lookup support. `LowerAST` now emits explicit Phase-1 `program`, `sequence`, binding, branch, binary, list, dictionary-entry, conversion, collection, output, and environment forms with named data edges. Added `hfir.LowerToBytecode(*Graph)`, which accepts only HFIR, selects existing bytecode instructions after the semantic boundary, validates referenced nodes while lowering, detects cyclic data dependencies, preserves provenance in compiler diagnostics, and returns no partial artifact on unsupported input.

## Tests

`internal/hfir/bytecode_test.go` verifies direct graph lowering, structural bytecode validation, stable unsupported diagnostics, missing-reference rejection, and provenance. `internal/vm/hfir_equivalence_test.go` parses and checks source, lowers/verifies HFIR, emits both legacy and direct artifacts, serializes and validates both artifacts, then runs each in fresh VMs. It compares stdout, stderr, exit code, structured VM errors, and capability denial for scalar control, collection mutation/read logic, string transformations, output/exit, and environment capability handling.

Focused validation passed:

```text
go test ./internal/hfir ./internal/vm
```

## Result

There is now one real execution path where semantic HFIR drives bytecode generation and VM-observable behavior without AST reconstruction: checked `.howl` compatibility input to HFIR, HFIR verification, `LowerToBytecode`, HFBC artifact validation, and isolated VM execution. Unsupported nodes fail closed. The public CLI was intentionally not switched because the coherent subset is still narrow.

The skeptical architecture review found and resolved one issue before commit: `map_set` originally labeled both operands as `value`, leaving key/value semantics positional. Collection edges now identify `key`, `value`, `index`, and `item` explicitly, and a graph-shape test rejects regression to generic collection operand roles.

## Remaining semantic ownership

The AST remains the public build representation and owns source preprocessing, checker rules, broad bytecode semantics, function metadata, structured errors, loop frames, HTTP/store/opaque effects, and runtime source mapping. HFIR owns semantic normalization and direct bytecode lowering only for the Phase-1 subset.

## Next step

Run the full CI-equivalent suite, inspect the skeptical implementation review, update evidence if it identifies a flaw, then commit the bounded vertical slice and documentation. The separate HTTP request-body safety issue is recorded for a focused follow-up rather than mixed into this compiler-boundary commit.
