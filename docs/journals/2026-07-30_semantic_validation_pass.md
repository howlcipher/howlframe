# Semantic Validation Pass

**Date:** 2026-07-30
**Status:** In progress — Phase 1 shipped
**Backlog:** #64, Semantic Type Checker Pass

## Current state

The in-flight work adds a dedicated `internal/checker` frontend pass and moves the executable entry point into `zero.go`, with the Go, JavaScript, and Wasm backends consuming the shared AST/IR packages. `zero_test.go` covers output-directory generation, crash-state serialization, and the Wasm backend's valid and invalid paths.

The first checkpoint was structural/backend-capability validation. Phase 1 now adds a typed value lattice in `internal/ast`, an `Analyze` pass in `internal/checker/types.go`, and AST annotations containing source kind plus native size, alignment, and pointer metadata. It infers literals, typed `defun` signatures, `let`, `call`, `if`, `while`, `do`, `set`, `try_let`, `for`, `spawn`, `match`, structs, field access, `parse_json`, `env`, file I/O, typed casts, lists, dictionaries, collection reads, conversions, and common operators. Unknown or dynamic values remain legal.

`IRNode` now carries the inferred `ast.TypeInfo` through `LowerShared`, so a native-oriented backend can consume layout metadata without returning to the AST. Backend-specific runtime types are still intentionally conservative. Backlog item #64 remains pending until the remaining control-flow/function type propagation is complete and a native backend makes code-generation decisions from the metadata.

## Verification

- `CCACHE_DISABLE=1 GOCACHE=/tmp/zero-gocache go test ./...` passes.
- `CCACHE_DISABLE=1 GOCACHE=/tmp/zero-gocache go test ./internal/checker -v` passes the layout/inference and diagnostic regression tests.
- The checker regression suite verifies an inferred integer expression reaches shared IR with 8-byte size and alignment metadata.
- The checker regression suite verifies `try_let`, `for`, `spawn`, and `match` propagation, including a string loop element retaining its 16-byte pointer-bearing layout.
- The checker regression suite verifies a `User` struct's 24-byte/8-byte-aligned layout, typed `user.age` field access, `env` string results, and `read_file` byte results.
- Representative existing fixtures for advanced control flow, primitives, JSON parsing, direct execution, mutable collections, no-else conditionals, and compound comparisons all pass the checker.
- Representative backend-specific fixtures for JSON parsing, primitive conversions, schema/database declarations, AI casts, environment variables, and context threading all pass the checker.
- `CCACHE_DISABLE=1 GOCACHE=/tmp/zero-gocache go vet ./...` passes.
- `CCACHE_DISABLE=1 GOCACHE=/tmp/zero-gocache go build -o /tmp/zero-check .` passes.
- The fixture sweep passes valid fixtures; `tests/routes.zero` and `examples/routes.zero` are include-only fragments, `tests/test_achieve.zero` is a top-level function fragment, and `tests/test_confidence.zero` exposes the existing lexer limitation for decimal literals.

## Next step

Next, add a backend that uses `IRNode.Type` to select native layouts and verify those decisions end to end. Do not mark #64 done until a native-oriented backend consumes the metadata rather than only preserving it.
