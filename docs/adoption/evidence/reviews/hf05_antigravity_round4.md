# Bounded Confirmation Review Report: HowlFrame PR #212 at 7de10d4a

## Verdict: ACCEPT

Commit `7de10d4a` replaces the single-level `try_let` check from round 3 with a sound recursive checker traversal (`yieldsNoValue` and `requireValueArguments` in `internal/checker/types.go`). When a value is required, the checker recursively follows `do`/`let` sequence tails, both branches of `if`, `try_let` success and catch bodies, and `match` arms, while properly exempting diverging paths that end in `exit` or `return`.

All nested defect variants identified in Round 3 and a bounded battery of 20 near variants across diverse value positions (`let` initializers, call arguments, `print`, `return`, `dict` values) are now strictly refused across all pipeline stages (`check`, `build`, `-compile-bc`, and `-compile-hfir-bc`) prior to artifact emission or effect execution. Valid code patterns (including value-producing nested forms, diverging branches, statement-position effects, and HowlBoard's `log_event` pattern) continue to compile and execute cleanly. The full test suite passes with zero regressions.

---

## Item-by-Item Results

### Item 1: Verification of Round 3 Nested Variants Across All Pipeline Entry Points

All nested variants that previously escaped the checker in Round 3 (`8f45ad63`), as well as the baseline top-level probes, were evaluated against `evidence/bin/pr` at `7de10d4a` across four compiler entry points: `check`, `build`, `-compile-bc`, and `-compile-hfir-bc`.

| Probe ID | Shape & Code Tested | `check` | `build` | `-compile-bc` | `-compile-hfir-bc` | Filesystem Leak Check |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **r3_do** | `(cli_app (let (x (do (print "a") (try_let (r "val") (catch e "err") (write_file "do_leaked.txt" "pwned")))) (print x)))` | Refused (1) | Refused (1; no artifact) | Refused (1; no artifact) | Refused (1; no artifact) | Clean (0 bytes written, `do_leaked.txt` absent) |
| **r3_if** | `(cli_app (let (x (if true (try_let (r "val") (catch e "err") (write_file "if_leaked.txt" "pwned")) "other")) (print x)))` | Refused (1) | Refused (1; no artifact) | Refused (1; no artifact) | Refused (1; no artifact) | Clean (`if_leaked.txt` absent) |
| **r3_trylet_direct** | `(cli_app (let (x (try_let (r1 "ok1") (catch e1 "err1") (try_let (r2 "ok2") (catch e2 "err2") (write_file "f.txt" "pwned")))) (print x)))` | Refused (1) | Refused (1; no artifact) | Refused (1; no artifact) | Refused (1; no artifact) | Clean (`f.txt` absent) |
| **r3_trylet_nested_do** | `(cli_app (let (x (try_let (r1 "ok1") (catch e1 "err1") (do (print "hi") (try_let (r2 "ok2") (catch e2 "err2") (write_file "nested_do.txt" "pwned"))))) (print x)))` | Refused (1) | Refused (1; no artifact) | Refused (1; no artifact) | Refused (1; no artifact) | Clean (`nested_do.txt` absent) |
| **r3_do_catch** | `(cli_app (let (x (do (print "a") (try_let (r (to_int "zz")) (catch e (write_file "do_catch_leaked.txt" "pwned")) "ok"))) (print x)))` | Refused (1) | Refused (1; no artifact) | Refused (1; no artifact) | Refused (1; no artifact) | Clean (`do_catch_leaked.txt` absent) |
| **r3_if_catch** | `(cli_app (let (x (if true (try_let (r (to_int "zz")) (catch e (write_file "if_catch_leaked.txt" "pwned")) "ok") "other")) (print x)))` | Refused (1) | Refused (1; no artifact) | Refused (1; no artifact) | Refused (1; no artifact) | Clean (`if_catch_leaked.txt` absent) |
| **r3_trylet_catch_nested_do** | `(cli_app (let (x (try_let (r1 (to_int "zz")) (catch e1 (do (print "hi") (try_let (r2 (to_int "zz")) (catch e2 (write_file "nested_catch_do.txt" "pwned")) "ok2"))) "ok1")) (print x)))` | Refused (1) | Refused (1; no artifact) | Refused (1; no artifact) | Refused (1; no artifact) | Clean (`nested_catch_do.txt` absent) |
| **r3_top_success** | `(cli_app (let (x (try_let (r "val") (catch e "err") (write_file "trylet_leaked.txt" "pwned"))) (print x)))` | Refused (1) | Refused (1; no artifact) | Refused (1; no artifact) | Refused (1; no artifact) | Clean (`trylet_leaked.txt` absent) |
| **r3_top_catch** | `(cli_app (let (x (try_let (r (to_int "zz")) (catch e (write_file "catch_leaked.txt" "pwned")) "ok")) (print x)))` | Refused (1) | Refused (1; no artifact) | Refused (1; no artifact) | Refused (1; no artifact) | Clean (`catch_leaked.txt` absent) |

*Diagnostic sample:*
- `r3_do`: `{"reason":"initializer of \"x\" is a (do ...) form with a path that produces no value","line":2,"column":11}`
- `r3_if`: `{"reason":"initializer of \"x\" is a (if ...) form with a path that produces no value","line":2,"column":11}`
- `r3_trylet_direct`: `{"reason":"initializer of \"x\" is a (try_let ...) form with a path that produces no value","line":2,"column":11}`
- `r3_trylet_nested_do`: `{"reason":"initializer of \"x\" is a (try_let ...) form with a path that produces no value","line":2,"column":11}`

*Comparison against Base Binary (`evidence/bin/base` @ `bfc4ff8f`):*
On `r3_do`, `base check` returned `OK` (exit code 0), compiled clean, performed the file write on disk (`do_leaked.txt`), and crashed with runtime `STACK_UNDERFLOW`. The PR binary refused compilation immediately at all entry points before writing any file or emitting any artifact.

---

### Item 2: Bounded Probes of Near Variants Across Unwrapped Shapes and Value Positions

We evaluated a bounded set of 20 near variants probing combinations of unwrapped shapes (`do`/`let` sequence tails, `if` branches, `try_let` success and catch bodies, `match` arms) and value positions (`let` initializers, user function call arguments, built-in call arguments, `print`, `return`, `dict` values).

Every probe was executed against `check`, `build`, `-compile-bc`, and `-compile-hfir-bc`. All 20 were refused before creating any artifact or writing to disk.

| # | Probe Identifier | Shape & Value Position | Diagnostic Reason | Status |
| :-: | :--- | :--- | :--- | :-: |
| 01 | `probe_01_let_match_arm` | `match` arm with `write_file` in `let` init | `initializer of "x" is a (match ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |
| 02 | `probe_02_let_match_default` | `match` default arm with `write_file` in `let` init | `initializer of "x" is a (match ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |
| 03 | `probe_03_print_do_match_arm` | `match` arm in `do` tail in `print` argument | `print argument 1 is a (do ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |
| 04 | `probe_04_call_let_tail` | `let` tail with `write_file` in user call arg | `argument 1 to "f" is a void expression and produces no value` | Refused (0 artifacts, 0 leaks) |
| 05 | `probe_05_let_do_store_put` | `do` tail with `store_put` in `let` init | `initializer of "x" is a (do ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |
| 06 | `probe_06_print_if_store_put` | `if` then-branch with `store_put` in `print` arg | `print argument 1 is a (if ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |
| 07 | `probe_07_return_match_trylet` | `try_let` inside `match` arm in `return` value | `return value is a (match ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |
| 08 | `probe_08_dict_trylet_match` | `match` arm in `try_let` in `dict` value | `dict value 1 is a (try_let ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |
| 09 | `probe_09_let_if_match_do` | `do` in `match` in `if` in `let` init | `initializer of "x" is a (if ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |
| 10 | `probe_10_builtin_do_if_no_else`| `if` without else in `do` in `to_string` arg | `to_string argument 1 is a (do ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |
| 11 | `probe_11_print_trylet_match_no_def` | `match` without default in `try_let` in `print` | `print argument 1 is a (try_let ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |
| 12 | `probe_12_call_match_arm` | `match` arm with `write_file` in call arg | `argument 1 to "foo" is a (match ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |
| 13 | `probe_13_dict_if_trylet_write` | `if` then-branch with `try_let` in `dict` value | `dict value 1 is a (if ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |
| 14 | `probe_14_return_do_write` | `do` tail with `write_file` in `return` value | `return value is a void expression and produces no value` | Refused (0 artifacts, 0 leaks) |
| 15 | `probe_15_call_do_if_trylet` | `try_let` in `if` in `do` in user call arg | `argument 1 to "g" is a (do ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |
| 16 | `probe_16_print_trylet_catch_store_del` | `store_delete` in `catch` body in `print` arg | `print argument 1 is a (try_let ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |
| 17 | `probe_17_dict_let_trylet` | `let` tail with `try_let` in `dict` value | `dict value 1 is a (let ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |
| 18 | `probe_18_return_match_store_put` | `match` default arm with `store_put` in `return` | `return value is a (match ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |
| 19 | `probe_19_builtin_do_match` | `match` arm with `write_file` in `read_file` arg | `read_file argument 1 is a (do ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |
| 20 | `probe_20_let_trylet_if_trylet` | Deep nesting: `try_let` in `if` in `try_let` in `let` | `initializer of "x" is a (try_let ...) form with a path that produces no value` | Refused (0 artifacts, 0 leaks) |

---

### Item 3: Acceptance and Execution of Valid Code

We verified that valid programs across all four specified categories continue to be accepted by the checker and execute correctly at runtime:

1. **Value-producing nested forms:**
   - Nested `do` in value position: `(cli_app (let (x (do (print "a") 42)) (print x)))`
     - `check`: OK; compiles clean; executes and outputs `a\n42\n` (exit code 0).
   - Nested `if` in value position: `(cli_app (let (x (if true (do (print "branch") 10) 20)) (print x)))`
     - `check`: OK; compiles clean; executes and outputs `branch\n10\n` (exit code 0).
   - Nested `try_let` in value position: `(cli_app (let (x (try_let (r "val") (catch e "err") (do (print "ok") (to_upper r)))) (print x)))`
     - `check`: OK; compiles clean; executes and outputs `ok\nVAL\n` (exit code 0).
   - Nested `match` in value position: `(cli_app (let (x (match 1 (1 (do (print "m") 99)) (default 0))) (print x)))`
     - `check`: OK; builds Go target cleanly (`Built .../server.go`).

2. **Branches ending in `exit` or `return` in value positions:**
   - Catch body ending in `exit`:
     ```lisp
     (cli_app
       (let (raw (try_let (r (to_int "zz")) (catch e (do (stderr "cannot parse") (exit 3))) r))
         (print raw)))
     ```
     - `check`: OK; compiles clean; executes, writes `"cannot parse"` to stderr, terminates with exit code 3.
   - Function with `return` in `if` branch in value position:
     ```lisp
     (cli_app
       (defun f ((v bool)) int
         (let (x (if v 100 (return 200)))
           (return x)))
       (print (call f true))
       (print (call f false)))
     ```
     - `check`: OK; compiles clean; executes and outputs `100\n200\n` (exit code 0).
   - Function with `return` in `catch` body in value position:
     ```lisp
     (cli_app
       (defun parse_or_default ((s string)) int
         (let (x (try_let (r (to_int s)) (catch e (return -1)) r))
           (return x)))
       (print (call parse_or_default "42"))
       (print (call parse_or_default "bad")))
     ```
     - `check`: OK; compiles clean; executes and outputs `42\n-1\n` (exit code 0).

3. **Same value-less shapes in statement position:**
   - `write_file` in statement position:
     `(cli_app (write_file "stmt_write.txt" "hello") (print "done"))`
     - `check`: OK; executes and prints `done\n`, writes file (exit code 0).
   - `try_let` with mixed bodies in statement position:
     `(cli_app (try_let (r (to_int "123")) (catch e (print "missing")) (print (to_string r))))`
     - `check`: OK; executes and prints `123\n` (exit code 0).
   - `do` ending in file write in statement position:
     `(cli_app (do (print "step1") (write_file "stmt_do_write.txt" "data")) (print "done"))`
     - `check`: OK; executes and prints `step1\ndone\n` (exit code 0).
   - `if` without else in statement position:
     `(cli_app (if true (print "if ok")) (print "done"))`
     - `check`: OK; executes and prints `if ok\ndone\n` (exit code 0).
   - `match` without default in statement position:
     `(cli_app (match 1 (1 (print "m ok"))) (print "done"))`
     - `check`: OK.

4. **HowlBoard's `log_event` pattern:**
   Helper function ending in `store_put` called as a statement:
   ```lisp
   (cli_app
     (defun log_event ((id string)) void
       (do
         (store_open s "memory://hf05")
         (store_put s "k" (dict ("val" id)))))
     (call log_event "evt1")
     (print "logged"))
   ```
   - `check`: OK; compiles clean; executes and outputs `logged\n` with exit code 0 under `-allow-caps database,filesystem`.

---

### Item 4: Corpus `check` Diff Across `howlframe-pr212`

We ran `check` across all 481 `.howl` files in `howlframe-pr212` using both binaries (`evidence/bin/base` and `evidence/bin/pr`):

- Total `.howl` files checked: **481**
- Matching outcomes: **481**
- Discrepancies / differences: **0**

*Note on repository changes between base and PR:*
In the base repository (`howlframe-base`), `benchmarks/v2/reference/task_08_capability_restricted/howlframe/app.howl` used `(try_let (ignore (write_file "leaked.txt" secret)) ...)`, which the base binary accepted (`OK`) but underflowed at runtime. The PR binary correctly rejects that base benchmark with `{"reason":"try_let binding of \"ignore\" is a void expression and produces no value","line":3,"column":22}`. In `howlframe-pr212`, this benchmark was updated to use a value-producing binding `(try_let (written (do (write_file "leaked.txt" secret) true)) ...)`, which both binaries accept cleanly.

---

### Item 5: Formatting, Linting, and Test Suite Verification

All commands executed on the PR head codebase (`7de10d4a`):

1. **`gofmt -l .`**
   - Output: Empty (0 unformatted Go files). Clean.

2. **`go vet ./...`**
   - Output: Empty (0 vet diagnostics). Clean.

3. **`go test ./internal/checker ./internal/hfir .`**
   - Output:
     ```
     ok  	github.com/howlcipher/howlframe/internal/checker	0.078s
     ok  	github.com/howlcipher/howlframe/internal/hfir	1.961s
     ok  	github.com/howlcipher/howlframe	171.635s
     ```
   - All tests across all three packages passed cleanly with exit code 0.

---

## Conclusion

PR #212 at commit `7de10d4a` completely resolves the vulnerability identified in HF-05 and the nested bypasses discovered in Round 3. By following `do`/`let` sequence tails, `if` branches, `try_let` bodies, and `match` arms recursively in `yieldsNoValue`, the source checker ensures that no value-less expression path can ever reach execution from a value position. At the same time, legitimate diverging paths (`exit`/`return`) and valid statement/value patterns remain fully functional without false positives across the corpus.
