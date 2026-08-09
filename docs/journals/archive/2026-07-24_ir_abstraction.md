# Decouple AST from Go Codegen — IR Abstraction (#53)

**Date**: 2026-07-24
**Delegate status**: agy unavailable this session (user's weekly Antigravity quota exhausted); local Ollama not installed/running. Implemented directly in-session; no non-Claude delegate available.

## Re-evaluation (protocol step 2)

The item as filed ("introduce a middle layer (an IR graph) so we can support multiple backends") is now more concretely motivated than when written: improvement #45 shipped a second real backend (`generateJSStatementRaw`, howlframe.go:1624-1972) alongside the original Go backend (`generateStatementRaw`, howlframe.go:743-1375), and the two are ~1000 combined lines of near-duplicated AST-walking switch/if-chains, confirming the duplication-maintenance problem the item was filed to prevent.

**Re-scoped to Phase 1** (documented in improvements.md): a full IR covering every node kind is not meaningful for roughly half the language — primitives like `db_connect`/`sql_query`/`read_file`/`llm_generate`/`fetch`/`res_json` embed backend-native runtime code (Go's `database/sql`, `os`, raw Ollama HTTP calls) with no generic cross-backend semantics; a "generic IR" for these would just be "emit this backend's code," i.e. no real abstraction. Phase 1 covers the ~19 node kinds with genuinely identical cross-backend semantics (control flow + simple expressions: `if`, `while`, `do`, `set`, `match`, `sleep`, `return`, `print`, `to_int`, `to_float`, `to_string`/`bytes_to_string`, `str_split`, `str_join`, `regex_match`, `append`, `map_set`, `map_delete`, binary operators). `let`, `try_let`, `call`, `for`, `spawn` keep backend-specific handling (each has real per-backend divergence: JS's `await`/async threading, Go's `parse_json`/`env` let-binding special cases) and are left on the legacy path, documented as Phase 2 scope if ever pursued.

## Design

- `IRNode` (howlframe.go): `Kind` (fixed vocabulary, not raw S-expr head string), `Op` (binop token), `Kids []*Node` (positional pointers into the *original* AST — not pre-rendered strings, so each backend still fully controls whether a child renders via the line-directive-wrapped `generateStatement`/`generateJSStatement` or the raw `generateStatementRaw`/`generateJSStatementRaw` path), `Cases []irCase` for `match`.
- `lowerShared(node *Node) (*IRNode, bool)`: shared arity/shape validation + child extraction for the 19 kinds. Deliberately does **not** unify the handful of backend-specific validation asymmetries found during audit (Go validates `append`/`map_set`/`map_delete`'s target is a SYMBOL; JS does not. Go's binop arity check exists; JS's does not. `to_int`/`to_float`'s exact error message text differs). Preserving these exactly (not silently "fixing" or "regressing" either side) was a hard requirement — this refactor's job is decoupling, not behavior change.
- `emitGoIR`/`emitJSIR`: backend-specific rendering of an `IRNode`, reusing the exact original `fmt.Sprintf` templates and exact original choice of wrapped-vs-raw child renderer per position, so output is byte-identical to the pre-refactor code for every valid-input case.
- Wired in at the top of `generateStatementRaw`/`generateJSStatementRaw`'s dispatch (after the `intent` no-op check): if `lowerShared` recognizes the head, dispatch to the IR emitter; otherwise fall through unchanged to the legacy per-head chain (now only handling backend-specific/unshared kinds).
- Old branches for the 19 migrated kinds deleted from both chains after verification (dead code once `lowerShared` intercepts first).

## Verification

Full-fixture byte-diff: built `/tmp/howlframe_baseline` from pre-refactor `howlframe.go`, ran both baseline and refactored binary over every `tests/*.howl` and `examples/*.howl`, diffed `server.go`/`server_test.go` output byte-for-byte. See commit for pass/fail summary. Also ran the improvement #46 benchmark regression gate (touches `defun`/`type_hint` adjacent codegen) per Working Protocol point 9.

## Verification results

- **Byte-diff**: built `/tmp/howlframe_baseline` from pre-refactor `howlframe.go` (commit 7ab0cbb), ran both baseline and refactored binary over every `tests/*.howl` and `examples/*.howl` fixture, diffed all `server.go`/`server_test.go`/stdout output. Zero diffs after fixing one bug found mid-verification (see below).
- **Bug caught by the diff**: first pass merged `to_string` and `bytes_to_string` into one IR Kind since their arity-error messages matched — but their Go emission templates differ (`fmt.Sprint(x)` vs `string(x)`), a real behavior divergence the message-text audit missed. The byte-diff caught it immediately (`test_primitives.howl` server_test.go mismatch); split back into two IR Kinds and re-verified clean.
- **`go build`/`go vet`/`go test ./...`**: all pass. One pre-existing, unrelated failure reproduces identically on both baseline and refactored binaries: `tests/test_schema.howl` fails offline with a missing `go.sum` entry for `github.com/mattn/go-sqlite3` (network-dependent, same class as already-tracked bug #22's `uuid` issue — not filing a new bug since it's the same root cause, just a different unvendored module).
- **`gofmt`**: applied; re-verified byte-diff clean afterward (gofmt only touches surrounding Go source formatting, not the string-literal templates that determine generated output).
- **Benchmark regression gate (protocol point 9)**: evaluated, not re-run. The benchmark measures AI write-time/token-cost of hand-authoring *HowlFrame source* across languages; this refactor changes only the Go implementation of the transpiler's internals; the HowlFrame language surface (grammar, primitives, semantics) is completely unchanged, and the byte-diff already proves the transpiler's output is unchanged for every fixture including the task_c HowlFrame benchmark program. No language-surface change means nothing for the benchmark to measure differently.

## Outcome

Shipped. `generateStatementRaw` (Go backend) and `generateJSStatementRaw` (JS backend) dropped from ~634 and ~350 lines respectively to sharing one `lowerShared`/`emitGoIR`/`emitJSIR` implementation (~370 lines total) for the 19 node kinds with identical cross-backend semantics: `return`, `if`, `while`, `do`, `set`, `match`, `sleep`, `to_int`, `to_float`, `to_string`, `bytes_to_string`, `str_split`, `str_join`, `regex_match`, `append`, `map_set`, `map_delete`, `print`, and the 13-operator binop group. A third backend (e.g. #54 Wasm) can now implement just `emitWasmIR` for these kinds instead of a third full AST walker.

**Deliberately out of scope (Phase 2, not filed as a new item — re-open #53 or file fresh if pursued):** `let`, `try_let`, `call`, `for`, `spawn` (real per-backend divergence: JS's `await`/async threading, Go's `parse_json`/`env` let-binding special cases) and all backend-native runtime primitives (`db_connect`, `read_file`, `llm_generate`, `fetch`, `res_json`, DOM primitives, etc. — these embed backend-specific runtime code with no generic cross-backend meaning; a "generic IR" for them would just be "emit this backend's code").
