# Token-density sugar for AI-first adoption (2026-10-10)

## Goal

Make HowlFrame win on `cl100k_base` source tokens against Python on the write-cost benchmark, especially Task C (function + check), which previously lost 110 vs 37.

## What shipped

`internal/ast/desugar.go` expands density sugar before checker/lowering:

1. **Root `(do …)` → `(cli_app …)`** — one-token root instead of two-token `cli_app`.
2. **Expression-bodied `defun`** — last value form becomes `(return …)` (skips `void` and statement-only bodies).
3. **`(assert expr)`** — expands to `if` / `stderr` / `exit 1` (CompileTimeOnly registry entry).
4. **Auto-`call`** — `(fname args…)` rewrites to `(call fname …)` when `fname` is a `defun` name.
5. **`(lines expr)`** — expands to `(str_split (bytes_to_string expr) "\n")`.
6. **`(read_text path)`** — expands to `(bytes_to_string (read_file path))`.

Wired through `howlframe.go` preprocess, `CompileToBytecode`, Go/JS emitters, and `vm.Interpret`. Corpus partition test desugars before `construct.Scan` so it matches the real gate.

## Evidence

Task C fixture:

```lisp
(do
  (defun add ((a int) (b int)) int (+ a b))
  (assert (= (add 2 3) 5)))
```

Task B fixture:

```lisp
(do
  (try_let (content (read_file "names.txt"))
    (catch err (print "Error: could not read names.txt"))
    (for line (lines content)
      (if (!= line "") (print "Hello," line) (print)))))
```

| Task | HowlFrame | Python |
|---|---|---|
| A HTTP/JSON | **75** | 168 |
| B CLI file | **56** | 61 |
| C fn+check | **34** | 37 |
| **Total** | **165** | **266** |

Verified:

- `python3 benchmarks/language_write_cost/recount_and_verify.py` (compile/run + token gates)
- `python3 benchmarks/v2/harness/prove_howlframe_refs.py` (densified v2 refs under Track A)
- `tests/test_token_density.howl`, `tests/test_lines_density.howl`
- CI steps added in `.github/workflows/ci.yml`

## Next

Optional further cuts: truthy empty-string `if`, shorter missing-file idioms, HFIR/JSON as the model emission surface.
