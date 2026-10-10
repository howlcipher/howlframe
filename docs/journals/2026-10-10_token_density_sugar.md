# Token-density sugar for AI-first adoption (2026-10-10)

## Goal

Make HowlFrame win on `cl100k_base` source tokens against Python on the write-cost benchmark, especially Task C (function + check), which previously lost 110 vs 37.

## What shipped

`internal/ast/desugar.go` expands density sugar before checker/lowering:

1. **Root `(do …)` → `(cli_app …)`** — one-token root instead of two-token `cli_app`.
2. **Expression-bodied `defun`** — last value form becomes `(return …)` (skips `void` and statement-only bodies).
3. **`(assert expr)`** — expands to `if` / `stderr` / `exit 1` (CompileTimeOnly registry entry).
4. **Auto-`call`** — `(fname args…)` rewrites to `(call fname …)` when `fname` is a `defun` name.

Wired through `howlframe.go` preprocess, `CompileToBytecode`, Go/JS emitters, and `vm.Interpret`. Corpus partition test desugars before `construct.Scan` so it matches the real gate.

## Evidence

Task C fixture (`benchmarks/language_write_cost/task_c_function_test/howlframe/add.howl`):

```lisp
(do
  (defun add ((a int) (b int)) int (+ a b))
  (assert (= (add 2 3) 5)))
```

| Language | Tokens (`cl100k_base`) |
|---|---|
| HowlFrame | **34** |
| Python (main+test) | 37 |

Totals across A+B+C (recounted HowlFrame fixtures): HowlFrame **202** vs Python **262**.

Verified: `tests/test_token_density.howl` via `-run` and `-run-bc` prints `42`; `go test ./...` green; coverage matrix regenerated.

## Next

Task B still trails Python (80 vs 61). Candidate: denser line-iteration / file-read idioms without `bytes_to_string` + `str_split` nesting.
