# 2026-10-12: #139 fix and core-library hardening

Base: main `facdd332` (#138 merged). No new language features, no token
claims, no production `-compile-bc`/HFIR flip, #90 untouched.

## 1. #139: governed `(fetch "http://x")` crashed the bytecode compiler

**Root cause:**
- `fetch` takes `(fetch url method [body])`. The AST bytecode compiler's
  `fetch` case (`internal/bytecode/bytecode.go`) compiled `Children[1]` and
  `Children[2]` without checking arity.
- A one-argument `(fetch url)` therefore indexed past the end of the child
  list and panicked with `index out of range [2] with length 2`.
- Nothing upstream stopped it. The governed profile allows `fetch`, since
  network is grantable at run time, so the profile scan accepted it. The
  checker's `fetch` arity check only runs on the JS statement path.
- The interpreter (`InterpErr`) and the HFIR lowering already reject the wrong
  arity cleanly. Only the AST bytecode compiler crashed.
- The panic affected both profiles. It surfaced in the governed build reported
  in #139.

**Fix:** the compiler now checks arity first. On a bad arity it reports the
diagnostic `fetch expects (fetch url method [body])` (same wording as the
interpreter), exits 1, prints no panic and writes no artifact. The well-formed
`(fetch url method)` still builds under governed. Network stays
grant-at-run-time and the capability rules are unchanged.

**Regression test:** `TestIssue139FetchArityFailsClosed`.

**Not changed:** a body argument is still compiled away by AST bytecode. This
is pre-existing; `OpFetch` has no body operand.

## 2. Empty `min` / `max`

**Before:** `(list_get L 0)` on an empty list silently returned an
empty/absent value.

**Now:** it is a runtime error on every path: the interpreter, governed AST
bytecode and HFIR bytecode. It writes `VALUE_ERROR: min of empty list` (or
`max`) to stderr and exits 1. Output printed before the error stays.
- This is the same stderr-plus-`exit 1` shape that `assert` already desugars
  to.
- It is still pure sugar: no new opcode, and `inspect --caps` still reports
  nothing.
- **Caveat:** this is a coded stderr message, not a structured `RuntimeError`
  JSON code. Making it structured would need a new opcode, which is out of
  scope.

**Test:** `TestEmptyMinMaxIsValueError`.

## 3. Go/JS target guard (Lain)

`build --target go|js` now fails closed when the program uses a #138
core-library built-in or `nz`. The error is
`UNSUPPORTED_ON_TARGET: … build for the bytecode target instead`. The exit is
nonzero, there is no "Built" line and no file is written. The built-ins are
not implemented on these targets.

- **Detection:** it runs after desugar and looks for the desugar temporaries
  (`__cl*`, `__nz`). Only real expansions are rejected; a user variable or
  defun named `sum` or `sort` still builds.
- **Why nz is included:** it lowers to a let-expression in value position,
  which both backends emit as a statement. The output did not compile, and
  this was confirmed in #137.

**Test:** `TestGoJSTargetsRejectCoreLibraryAndNz`, which covers 7 forms × 2
targets plus the variable-name control.

**Caveat:** other sugar that produces let-expressions (`all`/`any`,
`ver_gt`/`ver_ge`, the `list_get`/`parse_json` operand binding) is not covered
by this guard. Lain's guard was scoped to #138 and `nz`.

## Docs

- heldout-v1 REPORT:
  - Total-token ratio: HowlFrame uses 1.076× Python's total on cl100k_base
    (2132 vs 1982) and 1.078× on o200k_base.
  - `str_len`/`str_slice` count bytes.
  - `sort` is O(n²) insertion.
  - The Go/JS guard.
- The `desugar_corelib.go` header now documents the same points.

## Verification

- `gofmt -l` is empty.
- `go build/vet/test ./...` and `go test ./experiments/...` pass.
- All `run.sh` scripts pass:
  - v0: 60/60 per language.
  - heldout-v0: 126/126.
  - heldout-v1: 136/136.
  - The results files are unchanged, so no reference changed.
