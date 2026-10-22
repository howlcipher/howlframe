# Bounded Confirmation Review Report: HowlFrame PR #212 at 8f45ad63

## Verdict: REJECT

Commit `8f45ad63` directly addresses the top-level `try_let` probes and correctly updates the HFIR model adapter's `noValue` logic. However, **nested variants specified in Item 2 are not refused**: when a `try_let` with a value-less body is placed inside a `do`, inside an `if` branch, or inside a `do` within an outer `try_let` body while in a value position, the checker accepts the program (`OK`). On compilation and execution, the program performs the side-effect (e.g. disk write) and then terminates with a runtime `STACK_UNDERFLOW`.

---

## Item-by-Item Results

### Item 1: Refusal of `probe_trylet.howl`, Catch-Body Variant, and Adapter `try` Candidate

All three primary test probes are refused before any artifact is created or any file is written to disk:

1. **`probe_trylet.howl` (Success-body effect):**
   ```lisp
   (cli_app
     (let (x (try_let (r "val") (catch e "err") (write_file "trylet_leaked.txt" "pwned")))
       (print x)))
   ```
   * `check`: Refused (exit code 1):
     `{"reason":"initializer of \"x\" is a try_let whose catch or success body produces no value","line":2,"column":11}`
   * `build`: Refused (exit code 1; no artifact written).
   * `-compile-bc`: Refused (exit code 1; no `.bc.bin` written).
   * `-compile-hfir-bc`: Refused (exit code 1; no `.bc.bin` written).
   * Disk write: `trylet_leaked.txt` was not created.

2. **Catch-Body Variant (`probe_catch.howl`):**
   ```lisp
   (cli_app
     (let (x (try_let (r (to_int "zz")) (catch e (write_file "catch_leaked.txt" "pwned")) "ok"))
       (print x)))
   ```
   * `check`: Refused (exit code 1):
     `{"reason":"initializer of \"x\" is a try_let whose catch or success body produces no value","line":2,"column":11}`
   * `build`: Refused (exit code 1; no artifact written).
   * `-compile-bc`: Refused (exit code 1; no `.bc.bin` written).
   * `-compile-hfir-bc`: Refused (exit code 1; no `.bc.bin` written).
   * Disk write: `catch_leaked.txt` was not created.

3. **HFIR Model Adapter `try` Candidate:**
   * In `internal/hfir/value_roles_test.go`, `TestAdapterRejectsValueLessOperandBeforeExecution` includes `"try whose success body is an effect"`.
   * `DecodeCandidate` returns failure with `candidate.Graph == nil` and diagnostic:
     `HFIR_TRANSPORT_TYPE: node "p" has value role "value" targeting value-less "try"`.
   * Verified via `go test ./internal/hfir -run TestAdapterRejectsValueLessOperandBeforeExecution`: PASS (0 files written to disk).

---

### Item 2: Nested or Near Variants in Value Positions (Defect Confirmed)

We probed the nested variants explicitly designated by the brief: a `try_let` inside a `do`, inside an `if` branch, and inside another `try_let` body.

#### 1. `try_let` inside `do` in value position: **FAILED (Not Refused)**
* **Program:**
  ```lisp
  (cli_app
    (let (x (do (print "a") (try_let (r "val") (catch e "err") (write_file "do_leaked.txt" "pwned"))))
      (print x)))
  ```
* **Execution & Result:**
  * `evidence/bin/pr check test_do.howl`: **`OK: test_do.howl` (Exit code 0 - Accepted)**
  * `evidence/bin/pr -compile-bc -o out/ test_do.howl`: Clean compilation (Exit code 0).
  * `evidence/bin/pr -allow-caps filesystem -run-bc out/test_do.howl.bc.bin`:
    * Printed: `a`
    * Disk write executed: `do_leaked.txt` created containing `"pwned"`.
    * Runtime crash: `{"phase":"runtime","code":"STACK_UNDERFLOW","function":"main","instruction":8,"opcode":"STORE_VAR","message":"stack underflow at STORE_VAR"}`

#### 2. `try_let` inside an `if` branch in value position: **FAILED (Not Refused)**
* **Program:**
  ```lisp
  (cli_app
    (let (x (if true (try_let (r "val") (catch e "err") (write_file "if_leaked.txt" "pwned")) "other"))
      (print x)))
  ```
* **Execution & Result:**
  * `evidence/bin/pr check test_if.howl`: **`OK: test_if.howl` (Exit code 0 - Accepted)**
  * `evidence/bin/pr -compile-bc -o out/ test_if.howl`: Clean compilation (Exit code 0).
  * `evidence/bin/pr -allow-caps filesystem -run-bc out/test_if.howl.bc.bin`:
    * Disk write executed: `if_leaked.txt` created containing `"pwned"`.
    * Runtime crash: `{"phase":"runtime","code":"STACK_UNDERFLOW","function":"main","instruction":10,"opcode":"STORE_VAR","message":"stack underflow at STORE_VAR"}`

#### 3. `try_let` inside another `try_let` body: **Partially Refused**
* Direct child:
  ```lisp
  (let (x (try_let (r1 "ok1") (catch e1 "err1") (try_let (r2 "ok2") (catch e2 "err2") (write_file "f.txt" "pwned")))))
  ```
  * `check`: Refused (`initializer of "x" is a try_let whose catch or success body produces no value`).
* Wrapped in `do`:
  ```lisp
  (let (x (try_let (r1 "ok1") (catch e1 "err1") (do (print "hi") (try_let (r2 "ok2") (catch e2 "err2") (write_file "nested_do.txt" "pwned"))))))
  ```
  * `check`: **`OK` (Exit code 0 - Accepted)**
  * Runtime: Wrote `nested_do.txt` and crashed `STACK_UNDERFLOW`.

#### Root Cause Analysis
In `internal/checker/types.go`:
1. `case "try_let"` computes `return join(catchType, successType)`. When one branch produces `string` and the other is `void`, `join(STRING, VOID)` returns `ast.Layout(ast.Any)`.
2. When the `try_let` is enclosed in `(do ...)` or `(if ...)`, the outer expression's inferred type is `ast.Any`, not `ast.Void`.
3. In `requireValue(node *ast.Node, got ast.TypeInfo, use string)`:
   * `got.Kind == ast.Void` evaluates to false.
   * `head` is `"do"` or `"if"`. The switch only checks `case head == "try_let" && a.branchYieldsNoValue(node)`.
   * For `"do"`, `requireValue` does not unwrap the sequence to validate its final expression.
   * For `"if"`, `requireValue` only checks `head == "if" && len(node.Children) == 3` (if without else); it does not validate that both branches produce values when `len == 4`.
   * Similarly, `branchYieldsNoValue(node *ast.Node)` only checks if a branch `body.Children[0].Value == "try_let"`; it does not unwrap `do` bodies.
4. In contrast, `internal/hfir/model_adapter.go` implements this properly: `noValue` and `branchNoValue` recursively unwrap `let`, `sequence`, `if`, and `try`.

---

### Item 3: Acceptance of Valid Code

1. **`try_let` with value bodies in value position:**
   ```lisp
   (cli_app
     (let (x (try_let (r "hello") (catch e "err") r))
       (print x)))
   ```
   * Result: `check` OK; executed and printed `"hello"`.

2. **Catch body that exits (`(catch e (do (stderr "x") (exit 3)))`) in value position:**
   ```lisp
   (cli_app
     (let (raw (try_let (r (to_int "zz")) (catch e (do (stderr "cannot parse") (exit 3))) r))
       (print raw)))
   ```
   * Result: `check` OK; executed, printed `"cannot parse"` to stderr, exited 3.

3. **Mixed bodies in statement position:**
   ```lisp
   (cli_app
     (try_let (r (to_int "123"))
       (catch e (print "missing"))
       (print (to_string r))))
   ```
   * Result: `check` OK; executed and printed `"123"`.

---

### Item 4: Corpus `check` Diff Across `howlframe-pr212`

* Binaries: Base binary (`evidence/bin/base` @ `bfc4ff8f`) vs rebuilt PR binary (`evidence/bin/pr` @ `8f45ad63`).
* Corpus: All 481 `.howl` files across the `howlframe-pr212` repository.
* Results:
  * Total files: 481
  * Diffs: **0 differences found** between base and PR binaries.

---

### Item 5: Test Suite Execution on PR Head (`8f45ad63`)

Command executed on PR head:
`go test ./internal/checker ./internal/hfir .`

Results:
```
ok   github.com/howlcipher/howlframe/internal/checker   0.083s
ok   github.com/howlcipher/howlframe/internal/hfir      1.866s
ok   github.com/howlcipher/howlframe                  175.316s
```
All packages passed cleanly with exit code 0.

---

## Conclusion & Recommendations

Commit `8f45ad63` resolves the direct `try_let` probes and corrects the HFIR adapter. However, the source checker's `requireValue` and `branchYieldsNoValue` checks in `internal/checker/types.go` are non-recursive with respect to `do` tails and `if` branches.

To complete the remediation:
1. In `requireValue(node *ast.Node, got ast.TypeInfo, use string)`:
   * If `head == "do"` and `len(node.Children) > 1`, recursively call `requireValue` on the last child `node.Children[len(node.Children)-1]`.
   * If `head == "if"` and `len(node.Children) == 4`, recursively call `requireValue` on both `node.Children[2]` (then) and `node.Children[3]` (else).
2. In `branchYieldsNoValue(node *ast.Node)`:
   * If a branch body is `do`, inspect its tail expression.
   * If a branch body is `if`, verify that neither branch is value-less.
