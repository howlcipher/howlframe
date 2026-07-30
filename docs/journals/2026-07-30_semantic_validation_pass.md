# Semantic Validation Pass

**Date:** 2026-07-30
**Status:** In progress — Phase 1 shipped
**Backlog:** #64, Semantic Type Checker Pass

## Current state

The in-flight work adds a dedicated `internal/checker` frontend pass and moves the executable entry point into `zero.go`, with the Go, JavaScript, and Wasm backends consuming the shared AST/IR packages. `zero_test.go` covers output-directory generation, crash-state serialization, and the Wasm backend's valid and invalid paths.

The first checkpoint was structural/backend-capability validation. Phase 1 now adds a typed value lattice in `internal/ast`, an `Analyze` pass in `internal/checker/types.go`, and AST annotations containing source kind plus native size, alignment, and pointer metadata. It infers literals, typed `defun` signatures, `let`, `call`, `if`, `while`, `do`, `set`, `try_let`, `for`, `spawn`, `match`, structs, field access, `parse_json`, `env`, file I/O, typed casts, lists, dictionaries, collection reads, conversions, and common operators. Unknown or dynamic values remain legal.

`IRNode` now carries the inferred `ast.TypeInfo` through `LowerShared`, and the Wasm backend consumes it to select layouts: Zero `int` values are emitted as Wasm `i64`, floats as `f64`, boolean control-flow values remain `i32`, and bytes/lists/dicts/structs are represented as indirect `i32` linear-memory pointers while retaining their inferred size/alignment. Struct layouts now preserve declaration order and explicit field offsets, which are carried into the Wasm descriptor. Typed integer lists are emitted as a static linear-memory data segment with an 8-byte length header and return an `i32` pointer; string lists use a pointer table and NUL-terminated payloads. `list_get` reads either representation with unsigned bounds checking, scaled pointer arithmetic, and typed loads, returning zero out of range. Static dictionaries now emit a count header plus key/value pointer table and inline NUL-terminated string or little-endian integer payloads; `map_get` supports static string keys for homogeneous int or string dictionaries. Other aggregate expression emission remains deferred. Native integer/float conversions are emitted explicitly, while unsupported string conversions are rejected before lowering. Backlog item #64 remains pending until the remaining layout decisions are complete and verified with a Wasm validator.

## Verification

- `CCACHE_DISABLE=1 GOCACHE=/tmp/zero-gocache go test ./...` passes.
- `CCACHE_DISABLE=1 GOCACHE=/tmp/zero-gocache go test ./internal/checker -v` passes the layout/inference and diagnostic regression tests.
- The checker regression suite verifies an inferred integer expression reaches shared IR with 8-byte size and alignment metadata.
- The checker regression suite verifies `try_let`, `for`, `spawn`, and `match` propagation, including a string loop element retaining its 16-byte pointer-bearing layout.
- The checker regression suite verifies a `User` struct's 24-byte/8-byte-aligned layout, typed `user.age` field access, `env` string results, and `read_file` byte results.
- Struct regression coverage verifies declaration-order offsets (`name` at byte 0 and `age` at byte 16), and Wasm layout tests verify those offsets are preserved.
- Representative existing fixtures for advanced control flow, primitives, JSON parsing, direct execution, mutable collections, no-else conditionals, and compound comparisons all pass the checker.
- Representative backend-specific fixtures for JSON parsing, primitive conversions, schema/database declarations, AI casts, environment variables, and context threading all pass the checker.
- The Wasm regression test verifies inferred integer metadata changes module results, arithmetic, comparisons, and branch results from `i32` to `i64`; `wat2wasm` is not installed, so external WAT validation remains a follow-up environment check.
- The Wasm regression test verifies inferred float metadata selects an `f64` module result, `f64` comparison, and `f64.convert_s/i64` conversion.
- Wasm backend layout tests verify aggregate values select indirect `i32` representations without losing their 24-byte/8-byte-aligned metadata.
- The Wasm regression test verifies `(list 1 2)` emits an exported memory, a static data segment, and an `i32` pointer result; non-integer lists are rejected by the checker.
- The Wasm regression test verifies `(list_get (list 10 20) 1)` emits `i64.load`, `i32.wrap_i64`, scaled indexing, and the backing memory/data segment.
- The list memory invariant is length at byte offset 0 and elements at byte offset 8; `list_get` uses an unsigned length comparison and returns `i64.const 0` for out-of-range indices.
- String-list regression coverage verifies pointer-table `i32.load` results and NUL-terminated payload bytes for `(list_get (list "alpha" "beta") 1)`.
- Static dictionary regression coverage verifies string `map_get` returns an `i32` payload pointer, integer `map_get` returns an `i64` constant, and generated data segments contain the encoded value bytes.
- `CCACHE_DISABLE=1 GOCACHE=/tmp/zero-gocache go vet ./...` passes.
- `CCACHE_DISABLE=1 GOCACHE=/tmp/zero-gocache go build -o /tmp/zero-check .` passes.
- The fixture sweep passes valid fixtures; `tests/routes.zero` and `examples/routes.zero` are include-only fragments, `tests/test_achieve.zero` is a top-level function fragment, and `tests/test_confidence.zero` exposes the existing lexer limitation for decimal literals.

## Next step

The Wasm backend now runs a local, string-aware structural WAT gate before returning generated modules. It checks the module/function envelope, balanced parentheses, comments, and quoted strings. This is intentionally not an instruction/type validator; `wat2wasm`/`wasm-tools` remain required for that stronger check when available.

Next, install/use a full WAT validator when available, add Wasm memory access primitives for remaining aggregate expression forms and dynamic dictionary lookup, and verify the generated module end to end. Do not mark #64 done until those decisions are validated beyond structural checks and string fragments.
