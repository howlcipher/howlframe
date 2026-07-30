# Semantic Validation Pass

**Date:** 2026-07-30
**Status:** In progress
**Backlog:** #64, Semantic Type Checker Pass

## Current state

The in-flight work adds a dedicated `internal/checker` frontend pass and moves the executable entry point into `zero.go`, with the Go, JavaScript, and Wasm backends consuming the shared AST/IR packages. `zero_test.go` covers output-directory generation, crash-state serialization, and the Wasm backend's valid and invalid paths.

The current checker is structural/backend-capability validation. It does not yet infer semantic types or attach size, alignment, or pointer metadata to IR nodes, so backlog item #64 remains pending.

## Verification

- `CCACHE_DISABLE=1 GOCACHE=/tmp/zero-gocache go test ./...` passes.
- `CCACHE_DISABLE=1 GOCACHE=/tmp/zero-gocache go build -o /tmp/zero-check .` passes.
- The fixture sweep passes valid fixtures; `tests/routes.zero` and `examples/routes.zero` are include-only fragments, `tests/test_achieve.zero` is a top-level function fragment, and `tests/test_confidence.zero` exposes the existing lexer limitation for decimal literals.

## Next step

Complete semantic inference: define the Zero type lattice and inferred metadata, add type propagation/checks for literals, bindings, calls, control-flow joins, collections, and backend-specific primitives, then consume the annotations in native-oriented lowering tests. Do not mark #64 done until those annotations and tests exist.
