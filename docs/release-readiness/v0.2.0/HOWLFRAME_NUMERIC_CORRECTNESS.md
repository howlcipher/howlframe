# HowlFrame numeric correctness (v0.2.0 release candidate)

Base: `main` at `def34540` (PRs #227 to #232 present). Fix: branch `fix/numeric-nonfinite`, commit `2cb71aa8`.
Contract: `docs/reference/NUMERIC_CONTRACT.md`.

## Defects found

PR #227 fixed `to_float` of NaN and infinity text (#225) and recorded that arithmetic could still produce non-finite
values. Reproducing that, and checking every other route by which a non-finite value could reach a program, found
four defects. Each was reproduced on the base before it was changed.

| ID | Defect | Paths affected on the base | Contract |
|---|---|---|---|
| N1 | Float `+`, `-`, `*` that overflow return `+Inf`/`-Inf`; `Inf - Inf` returns NaN | Interpreter, AST bytecode, HFIR bytecode, generated JavaScript, generated Go | 2.3, 7 |
| N2 | An integer literal outside int64 passes `check`; `build` saturates it to an int64 bound | `check`, `build`, `-compile-bc` (interpreter and HFIR lowering refuse it) | 2.1, 12 |
| N3 | A float literal outside float64 passes `check`; `build` writes `+Inf` into the artifact | `check`, `build`, `-compile-bc` | 2.3, 7, 12 |
| N4 | `confidence` pushes whatever `ParseFloat` returns for the model's answer: `"NaN"`, `"Inf"`, `"1e400"` reach the program | Interpreter, VM, generated Go | 2.3 |

### Original reproductions (base binary built from `def34540`)

```text
(cli_app (let (x (to_float "1e308")) (print (* x 10.0))))
  -run: +Inf   run (AST bytecode): +Inf   HFIR bytecode: +Inf   JS: Infinity   Go: +Inf      all exit 0

(cli_app (let (big (to_float "1e308") inf (* big 10.0) score (- inf inf))
  (if (or (< score 0.0) (> score 100.0)) (print "REJECT") (print "ACCEPT"))))
  every path: ACCEPT                       <- a NaN score passes the range check

(cli_app (let (big (to_float "1e308") limit (* big 10.0))
  (if (> 5000.0 limit) (print "REJECT") (print "ACCEPT"))))
  every path: ACCEPT                       <- an infinite limit admits any amount

(cli_app (print 99999999999999999999))
  check: OK   -run: invalid integer literal   build + run: 9223372036854775807

(cli_app (print 1<309 zeros>.0))
  check: OK   -run: invalid float literal     build + run: +Inf
```

`+`, `-`, `*` with additions/products of `1.7976931348623157e308`, `-1.79e308 - 1.79e308` and
`9223372036854775807 * 1e300` (mixed int/float) behaved the same way. Integer overflow, division by zero and
non-finite division results were already errors.

### Routes checked and found correct on the base

* `to_float` text (fixed by #227), `/` (already refused non-finite results), `to_int` of non-finite floats.
* `parse_json`: `1e400` raises a catchable `RUNTIME_ERROR`; integers beyond int64 decode as float64 (section 9);
  `1e-400` underflows to `0`, as IEEE 754 does.
* Non-ASCII digits: the lexer rejects them (`Unexpected character`).
* Exponent literals (`1e10`): not lexed; they fail the check as `undefined reference to "e10"`. This contradicted
  section 3 of the contract, which was corrected (the contract, not the language).
* `%`, `//`, `sum`, `min`, `max` and the other library forms desugar to the same operators, so they inherit the fix.

## Fix

| Area | Change |
|---|---|
| `internal/vm/vm.go` | One helper, `floatArithmetic`, used by the interpreter and the VM. A non-finite result raises `RUNTIME_ERROR` `floating-point overflow: <a> <op> <b> is not finite` (the VM) or the same reason with line and column (the interpreter). `confidence` raises `CONVERSION_ERROR` for a NaN or infinite answer. |
| `internal/checker/types.go` | A literal outside int64 or finite float64 is a check diagnostic, so every entry point (`check`, `-validate`, `run`, `-run`, `build` for every target, `-compile-bc`) refuses it with the same message and position. |
| `internal/bytecode/bytecode.go` | The compiler no longer discards the literal parse error (for direct `CompileToBytecode` callers). |
| `internal/bytecode/artifact.go` | `ValidateProgram`, used both when writing and when loading an artifact, refuses a NaN or infinite `LOAD_CONST`. An artifact written by the base compiler with `+Inf` is now refused at load. |
| `internal/backend/javascript/javascript.go` | `howlFrameArith` and a new `howlFrameFloatArith` throw `RUNTIME_ERROR: floating-point overflow` for non-finite results; string `+` is unchanged. |
| `internal/ast/desugar_controlflow.go` | The `%`/`//` expansion relied on overflow: it doubled `Q` until it passed `R` (reaching `+Inf` when `R` was near the float64 maximum) and tested `|A| > |B|*2^63`. It now doubles while `Q <= R - Q` (exact: `R - Q` cannot round below `Q` while `2Q <= R`, and is exact by Sterbenz once `Q > R/2`) and tests `|A|/2^63 > |B|` (a subnormal rounding miss is caught by the existing post-loop check). |

The fix does not change finite arithmetic, int64 overflow detection, exact mixed comparisons or division-by-zero
handling.

## Evidence

| Check | Result |
|---|---|
| Reproductions above, rerun on the fix | Every overflow raises `floating-point overflow`; neither policy program prints `ACCEPT`; out-of-range literals fail `check` with line and column; finite results unchanged |
| `%` and `//`: all 76 #151 cases | Identical to the base on the interpreter (7 of them failed before the template rewrite) |
| `%` and `//`: randomized differential, 16 programs, 960 operations across 1e-320..1e308 | Identical output to the base binary (499 result lines; programs end at the first `//` result outside int64, identically) |
| Staged release binaries (linux/amd64 native, linux/arm64 under qemu) | Same results as the source build |

New tests (each fails on the base):

* `internal/vm/numeric_nonfinite_test.go`: `TestFloatArithmeticRefusesNonFiniteResults`,
  `TestFloatArithmeticKeepsFiniteResults` (interpreter and VM operators, including the float64 maximum, subnormals,
  `-0`, mixed int/float), `TestNaNOrderingIsFalseAtTheOperators`, `TestConfidenceRefusesNonFiniteModelResponse` (a
  local stub on the model port; no model calls).
* `internal/bytecode/artifact_nonfinite_test.go`: `TestArtifactRefusesNonFiniteConstants`.
* `numeric_nonfinite_cli_test.go`: overflow on every path, the two policy programs, finite results unchanged, literal
  range on eight entry points with boundary literals accepted.
* `internal/backend/javascript/numeric_safety_test.go`: `TestJSFloatOverflowFailsClosed` (generated code run by Node).

Changed tests, with the reason:

* `TestIssue158NaNOrderingOnEveryPath` built its NaN from `Inf - Inf`, which is no longer possible. It now requires
  that construction to fail on every path, keeps the mixed int/float ordering assertions, and the NaN ordering
  guarantee itself moved to `TestNaNOrderingIsFalseAtTheOperators`. The sort/min/max NaN placement it also asserted
  is unreachable from source now (follow-up #164 no longer applies to source programs).
* `TestIssue151ErrorsOnEveryPath`: six cases built `Inf`/NaN operands by overflow; they now expect the earlier
  `floating-point overflow` error (no non-finite value reaches `%` or `//`). The position case used one of those
  operands; it now uses a reachable expansion error and still requires `at line 2, column 10`.

Gates on `2cb71aa8` with Go 1.25.14 (the CI toolchain): `gofmt` clean, `go build ./...`, `go vet ./...`,
`go test ./...` (76 packages ok), `go test -race ./...` (76 ok), `scripts/ci_acceptance.sh` (all suites passed;
318 PASS, 0 FAIL, 1 recorded XFAIL waiver W-m12).

## Execution-path coverage after the fix

| Path | Arithmetic (2.3, 7) | Literal range | `confidence` | Notes |
|---|---|---|---|---|
| Interpreter (`-run`) | Conforms | Conforms (checker) | Conforms | |
| AST bytecode VM (`build`, `run`) | Conforms | Conforms | Conforms | Production path |
| HFIR bytecode (`-compile-hfir-bc`) | Conforms (same VM operator) | Conforms | Conforms | Experimental |
| Generated JavaScript | Conforms | Conforms (checker) | Not changed | Integers beyond 2^53 already fail closed (section 13) |
| Generated Go | **Does not conform** | Conforms (checker) | **Does not conform** | Documented in section 13 |

## Remaining numeric limitations

* **Generated Go** emits Go operators on Go types: int64 overflow of runtime values wraps, float overflow yields
  `+Inf`, mixed int/float follows Go typing, constants fold exactly (`(+ 0.1 0.2)` prints `0.3`), and `confidence` can
  return a non-finite value. Conforming would need type-directed checked arithmetic emission throughout the Go
  backend, which the product directive (section 3.1: backend parity is not a gate) does not justify now. Run numeric
  policy on the standalone VM.
* **VM error position.** Numeric errors raised by the operator report `"instruction":0` on the VM (as integer
  overflow already did); the interpreter reports line and column. The message names the operation and operands.
* **`confidence`** still turns a non-numeric answer into `0` (pre-existing; not a non-finite value).
* **`to_float`** accepts Go forms the contract does not mention (`"0x1p4"`, `"1_000.5"`) on the VM, interpreter and
  Go (pre-existing, recorded in #227).
* **Instruction cost.** The `%`/`//` float path executes one more operation per doubling step; results are unchanged,
  and a program near a tight `--max-instructions` or `--max-memory-bytes` limit can reach it a few instructions sooner.

## Independent review

Antigravity with `gemini-3.8-flash-high` (a different provider from the implementer, subscription login, no paid API
calls) reviewed both changes from an isolated copy of the RC tree (`bbd4c8f5`) and the staged binaries, running its own
programs. On the numeric contract (its item 1) and scope (item 8) it found no defect: overflow, division,
conversions, literals, JSON, artifact constants and the `%`/`//` rewrite (including `(% 1.5e308 1.0e308)` =
`5e+307`) behaved as documented on the interpreter, AST bytecode, HFIR bytecode and generated JavaScript; the
generated-Go deviation in section 13 was reproduced as written; and no test assertion was weakened. Its overall verdict
and the two documentation findings it raised elsewhere are recorded in `HOWLFRAME_RELEASE_CANDIDATE.md`.
