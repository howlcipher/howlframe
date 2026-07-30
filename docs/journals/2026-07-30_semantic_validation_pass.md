# Semantic Validation Pass

**Date:** 2026-07-30
**Status:** Done — semantic layouts verified through external Wasm validation and runtime instantiation
**Backlog:** #64, Semantic Type Checker Pass

## Current state

The in-flight work adds a dedicated `internal/checker` frontend pass and moves the executable entry point into `zero.go`, with the Go, JavaScript, and Wasm backends consuming the shared AST/IR packages. `zero_test.go` covers output-directory generation, crash-state serialization, and the Wasm backend's valid and invalid paths.

The first checkpoint was structural/backend-capability validation. Phase 1 now adds a typed value lattice in `internal/ast`, an `Analyze` pass in `internal/checker/types.go`, and AST annotations containing source kind plus native size, alignment, and pointer metadata. It infers literals, typed `defun` signatures, `let`, `call`, `if`, `while`, `do`, `set`, `try_let`, `for`, `spawn`, `match`, structs, field access, `parse_json`, `env`, file I/O, typed casts, lists, dictionaries, collection reads, conversions, and common operators. Unknown or dynamic values remain legal.

`IRNode` now carries the inferred `ast.TypeInfo` through `LowerShared`, and the Wasm backend consumes it to select layouts: Zero `int` values are emitted as Wasm `i64`, floats as `f64`, boolean control-flow values remain `i32`, and bytes/lists/dicts/structs are represented as indirect `i32` linear-memory pointers while retaining their inferred size/alignment. Struct layouts preserve declaration order and explicit field offsets. List metadata carries its element layout; dictionary metadata now carries both string-key and homogeneous value layouts. The checker rejects incompatible aggregate elements, keys, mutations, indexes, numeric operations, function arity, and branch results before lowering.

Typed integer lists use an 8-byte length header followed by 8-byte elements; string lists use a pointer table and NUL-terminated payloads. Integer expressions inside list and dictionary literals emit runtime `i64.store` initialization, string expressions inside list and dictionary literals emit `i32.store` pointer-table initialization, and dynamic dictionary key expressions compare interned string pointers. Integer dictionary reads use `i64.load` from the encoded value slot. Aggregate table reservations grow past the original 256-byte floor when needed, emitted memory page counts scale with payload size, and each aggregate literal receives an independent aligned linear-memory region.

The local WAT gate tokenizes and parses the emitted folded subset and validates function results, instruction operands/results, `if`/`block` branches, returns, constants, loads/stores, declared memory, data offsets, and data bounds. `wasm-tools` is now installed locally and validates representative generated WAT after parsing it to Wasm binaries. Node then instantiates those binaries and calls exported `main` for numeric and pointer-returning cases. Backlog item #64 is complete.

## Verification

- `CCACHE_DISABLE=1 GOCACHE=/tmp/zero-gocache go test ./...` passes.
- `CCACHE_DISABLE=1 GOCACHE=/tmp/zero-gocache go test ./internal/checker -v` passes the layout/inference and diagnostic regression tests.
- The checker regression suite verifies an inferred integer expression reaches shared IR with 8-byte size and alignment metadata.
- The checker regression suite verifies `try_let`, `for`, `spawn`, and `match` propagation, including a string loop element retaining its 16-byte pointer-bearing layout.
- The checker regression suite verifies a `User` struct's 24-byte/8-byte-aligned layout, typed `user.age` field access, `env` string results, and `read_file` byte results.
- Struct regression coverage verifies declaration-order offsets (`name` at byte 0 and `age` at byte 16), and Wasm layout tests verify those offsets are preserved.
- Representative existing fixtures for advanced control flow, primitives, JSON parsing, direct execution, mutable collections, no-else conditionals, and compound comparisons all pass the checker.
- Representative backend-specific fixtures for JSON parsing, primitive conversions, schema/database declarations, AI casts, environment variables, and context threading all pass the checker.
- The Wasm regression test verifies inferred integer metadata changes module results, arithmetic, comparisons, and branch results from `i32` to `i64`.
- The Wasm regression test verifies inferred float metadata selects an `f64` module result, `f64` comparison, and `f64.convert_i64_s` conversion.
- Wasm backend layout tests verify aggregate values select indirect `i32` representations without losing their 24-byte/8-byte-aligned metadata.
- The Wasm regression test verifies `(list 1 2)` emits an exported memory, a static data segment, and an `i32` pointer result; non-integer lists are rejected by the checker.
- The Wasm regression test verifies `(list_get (list 10 20) 1)` emits `i64.load`, `i32.wrap_i64`, scaled indexing, and the backing memory/data segment.
- The list memory invariant is length at byte offset 0 and elements at byte offset 8; `list_get` uses an unsigned length comparison and returns `i64.const 0` for out-of-range indices.
- String-list regression coverage verifies pointer-table `i32.load` results and NUL-terminated payload bytes for `(list_get (list "alpha" "beta") 1)`.
- Static dictionary regression coverage verifies string `map_get` returns an `i32` payload pointer, integer `map_get` reads an `i64` memory slot, and generated data segments contain the encoded value bytes.
- Checker regressions verify heterogeneous lists/dictionaries, non-string dictionary keys, invalid aggregate access/mutation, function arity, mixed numeric operations, and incompatible branch layouts produce source-located diagnostics.
- CLI regressions verify semantic layout failures exit before `app.wat` is written.
- WAT validator regressions reject function-result mismatches, invalid operands, incompatible branches, loads without memory, unknown instructions, invalid constants, and out-of-bounds data segments.
- Dynamic aggregate regressions verify integer list/dictionary expressions emit `i64.store`, integer dictionary reads emit `i64.load`, large tables do not overlap their payloads, and large payloads request multiple memory pages.
- Multi-aggregate regressions verify independent list literals emit distinct data segments and branch-local `list_get` operations read length and element slots from their own aligned base offsets.
- Dynamic string regressions verify string list and dictionary expressions emit `i32.store`, dynamic dictionary keys emit `i32.eq`, and interned string payloads are returned through pointer-table loads.
- Integer dictionary regression coverage now verifies reads come from the encoded linear-memory slot rather than returning a compile-time constant.
- `CCACHE_DISABLE=1 GOCACHE=/tmp/zero-gocache go vet ./...` passes.
- `CCACHE_DISABLE=1 GOCACHE=/tmp/zero-gocache go build -o /tmp/zero-check .` passes.
- `cargo install wasm-tools --locked` installed `wasm-tools v1.254.0`; `wasm-tools parse` and `wasm-tools validate` pass for representative generated modules covering float conversion, multiple aggregates, dynamic string lists, dynamic integer dictionary keys, dynamic string dictionary values, and static integer dictionaries.
- Node v26.5.0 instantiates the compiled Wasm binaries and calls `main`; numeric cases return the expected values and pointer-returning string cases resolve to the expected NUL-terminated memory payloads.
- The language write-cost benchmark was not re-run because these changes add semantic rejection and backend-internal layout/validation without changing the Zero source needed by its `defun`/`type_hint`, `read_file`, `str_split`, or `test` tasks.
- The fixture sweep passes ordinary valid fixtures. Expected standalone transpile failures remain `tests/routes.zero` (include-only), `tests/test_achieve.zero` (top-level fragment), and `tests/test_confidence.zero` (existing decimal-literal lexer limitation). Two pre-existing generated-Go build gaps also remain: `tests/test_lazy_synthesize.zero` references a runtime-synthesized function and `tests/test_schema.zero` needs the deliberately untracked SQLite `go.sum` entry.

## Next step

#64 is Done. Future native backend work should proceed through #65 Linear SSA-based IR and #67 Native Backend Code Generators rather than reopening this semantic layout checkpoint.
