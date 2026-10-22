# Independent Code Review Report: HowlFrame PR #212

## Verdict: REJECT

While PR #212 successfully closes the original bug in `try_let` bindings, `dict` values, `return` values, and many built-in argument positions, **a critical defect remains**: `try_let` bodies (`success_body` and `catch` body) used in a value position are not checked for value production when the alternate branch yields a value. Consequently, programs where a value-less form (e.g. `write_file`) resides in a `try_let` body inside a value position pass `check`, `build`, `-compile-bc`, and `-compile-hfir-bc`, execute the file write on disk, and only then terminate with a runtime `STACK_UNDERFLOW`. The exact same vulnerability exists on the HFIR model-adapter path.

---

## Item-by-Item Results

### Item 1: Bug Reproduction on Base & Verification on PR Head

**Target Program (`bug.howl`):**
```lisp
(cli_app (try_let (r (write_file "repro_item1/leaked.txt" "pwned")) (catch e (print "c")) (print "b")))
```

#### Base Binary (`evidence/bin/base` @ `bfc4ff8f`)
* `check`: Accepted with exit code 0 (`OK: repro_item1/bug.howl`).
* `build`: Accepted with exit code 0 (`Built repro_item1/base_out`).
* `-compile-bc`: Accepted with exit code 0 (emitted bytecode).
* `-compile-hfir-bc`: Accepted with exit code 0 (emitted bytecode).
* Execution (`-allow-caps filesystem -run-bc`): **File write executed (`repro_item1/leaked.txt` created with `"pwned"`), then failed:**
  ```json
  {"phase":"runtime","code":"STACK_UNDERFLOW","function":"main","instruction":2,"opcode":"TRY_LET","message":"stack underflow at TRY_LET"}
  ```

#### PR Head Binary (`evidence/bin/pr` @ `692b4dfa`)
* `check`: Refused with exit code 1:
  ```json
  {"reason":"try_let binding of \"r\" is a void expression and produces no value","line":1,"column":22}
  ```
* `build`: Refused with exit code 1 (same error; no artifact created).
* `-compile-bc`: Refused with exit code 1 (same error).
* `-compile-hfir-bc`: Refused with exit code 1 (same error).
* File system: `repro_item1/leaked.txt` was never created.

---

### Item 2: Search for Bypasses in Value Positions (Defect Found)

We systematically probed a wide array of positions with value-less forms (`write_file`, `print`, `store_put`, etc.):

| Position Probed | Result on PR Head | Diagnostic / Behavior |
| :--- | :--- | :--- |
| Built-in argument (`read_file`, `mkdir`, `to_string`, `encode_json`, etc.) | Refused | `... argument N is a void expression and produces no value` |
| User function call argument | Refused | `argument N to "foo" is a void expression and produces no value` |
| Nested `do` ending in effect | Refused | `initializer of "x" is a void expression and produces no value` |
| `if` without `else` in value position | Refused | `... is an if without an else branch, which produces no value ...` |
| `while` in value position | Refused | `... is a void expression and produces no value` |
| `for` in value position | Refused | `... is a void expression and produces no value` |
| `for` iterable expression | Refused | `for iterable is a void expression and produces no value` |
| `match` subject expression | Refused | `match subject is a void expression and produces no value` |
| `match` without default in value position | Refused | `... is a match without a default arm, which produces no value ...` |
| `let` / `set` value | Refused | `... is a void expression and produces no value` |
| `dict` value / `list` element | Refused | `... is a void expression and produces no value` |
| `return` value | Refused | `return value is a void expression and produces no value` |
| `str` argument (`to_string`) | Refused | `to_string argument 1 is a void expression and produces no value` |
| `exec` argument | Refused | `exec argument 1 is a void expression and produces no value` |
| **`try_let` bodies in value position** | **ACCEPTED (DEFECT)** | **Compiles clean, writes file, crashes `STACK_UNDERFLOW`** |

#### Defect Analysis & Minimal Reproduction

In `internal/checker/types.go` (lines 515–528):
```go
case "try_let":
    ...
    catchType := a.inferChild(node.Children[2], 2, catchEnv)
    successEnv := cloneEnv(env)
    successEnv[binding.Children[0].Value] = value
    successType := a.infer(node.Children[3], successEnv)
    return join(catchType, successType)
```
When `try_let` is evaluated in a value position:
1. If one branch produces a value (e.g. `catchType` is `STRING` from `"err"`) and the other branch is void (e.g. `successType` is `VOID` from `(write_file ...)`), `join(STRING, VOID)` returns `ast.Layout(ast.Any)` because `left.Kind != right.Kind` and both are known.
2. Because `Any != Void`, `requireValue` on the enclosing form (e.g. `let`, `print`, `set`, `call`) sees a non-void type and accepts it.
3. Neither `catch` body nor `success` body is checked with `requireValue`.

**Reproduction Program (`probe_trylet.howl`):**
```lisp
(cli_app
  (let (x (try_let (r "val") (catch e "err") (write_file "test_scratch/trylet_leaked.txt" "pwned")))
    (print x)))
```

**Commands & Output:**
```bash
$ evidence/bin/pr check probe_trylet.howl
OK: probe_trylet.howl

$ evidence/bin/pr -compile-bc -o out/ probe_trylet.howl
# Exits 0, produces out/probe_trylet.howl.bc.bin

$ evidence/bin/pr -allow-caps filesystem -run-bc out/probe_trylet.howl.bc.bin
{"phase":"runtime","code":"STACK_UNDERFLOW","function":"main","instruction":6,"opcode":"STORE_VAR","message":"stack underflow at STORE_VAR"}

$ cat test_scratch/trylet_leaked.txt
pwned
```

The error path in `catch` body has the identical defect when the binding fails at runtime:
```lisp
(cli_app
  (let (x (try_let (r (dec "not_a_num")) (catch e (write_file "test_scratch/catch_leaked.txt" "pwned")) "ok"))
    (print x)))
```

#### Defect on HFIR Model Adapter Path
In `internal/hfir/model_adapter.go`:
- `statementRole` classifies `try.success_body` and `try.catch` as statement roles (`return true`), so their targets are bypassed during value role validation.
- `noValue` does not evaluate `try` nodes (`try` is not in `valueLessKinds`).
- Therefore, an HFIR candidate with a `try` node wired into a value role (e.g. `print.value`) whose `success_body` is `write_file` is accepted by `DecodeCandidate` (0 diagnostics) and `CompileCandidate` (0 diagnostics). At runtime, `vm.RunBytecodeWithEvidence` executes `write_file`, creates the file on disk, and terminates with `&{Code:STACK_UNDERFLOW Instruction:6 Opcode:PRINT NodeID:p Message:stack underflow at PRINT}`.

---

### Item 3: Checking Valid Programs & Corpus Diff

1. **Specific Valid Patterns Tested:**
   - Effect-only statements (`write_file`, `mkdir`, `print` in sequence): **Accepted**.
   - `if` statements with or without `else` used as statements: **Accepted**.
   - Void helper functions (`(defun foo () void ...)`): **Accepted**.
   - Store writes as the last form of a helper (`HowlBoard`'s `log_event` pattern): **Accepted**.
   - Safe effect-value binding via `do` (`(let (x (do (write_file ...) true)))`): **Accepted**.

2. **Full Corpus Check Across All 481 `.howl` Files in PR:**
   - Ran `evidence/bin/base check <file>` vs `evidence/bin/pr check <file>` across all 481 `.howl` files in `howlframe-pr212`.
   - **0 differences found**.
   - Checking base repository's `benchmarks/v2/reference/task_08_capability_restricted/howlframe/app.howl`:
     - Base: Accepted (`OK`)
     - PR: Refused with `try_let binding of "ignore" is a void expression and produces no value`
     - PR's updated version in `howlframe-pr212` uses `(try_let (written (do (write_file ...) true)) ...)` which is accepted by both.

---

### Item 4: `valueLessKinds` vs `opcode.go` Table & Model Adapter Soundness

1. **Agreement with Opcode Table:**
   `valueLessKinds` in `internal/hfir/model_adapter.go` lists:
   `write_file`, `mkdir`, `print`, `stderr`, `exit`, `set`, `map_set`, `map_delete`, `append`, `sleep`, `for`, `while`, `defun`, `return`.
   - Each straight-line kind corresponds to an opcode with `Pushes: 0` in `internal/bytecode/opcode.go`.
   - Non-straight-line kinds (`for`, `while`, `defun`, `return`) are statements that push 0 values to the stack.
   - Other `Pushes: 0` opcodes in `opcode.go` (`STORE_PUT`, `STORE_DELETE`, `DB_CONNECT`, `HTTP_*`, `SPAWN_AGENT`) are deliberately excluded because their forms are outside the model adapter's executable subset (`nodeRoleAlternatives`).
   - `TestValueLessKindsMatchOpcodeTable` in `internal/hfir/value_roles_test.go` verifies this alignment.
   - Note: `OpTryLet` has `Pushes: 0`, and `try` is in `nodeRoleAlternatives`; as identified in Item 2, `try` is missing from `noValue` / value role handling.

2. **Soundness of `validateValueRoles`:**
   - Every role not listed in `statementRole` corresponds to an instruction that pops one or more operands from the VM stack.
   - Supplying a node that pushes nothing (`Pushes: 0`) is guaranteed to cause a runtime `STACK_UNDERFLOW`.
   - Hence, `validateValueRoles` does not refuse any valid adapter candidate.

---

### Item 5: Inspection of Tests and Benchmark References

We inspected the diff of every modified test file and reference benchmark:
- `benchmarks/v2/reference/task_08_capability_restricted/howlframe/app.howl`:
  Updated from `(try_let (ignore (write_file ...)) ...)` to `(try_let (written (do (write_file ...) true)) ...)`. Token count updated 46 -> 50. The capability restriction assertion is preserved; the test was not weakened (the original actually underflowed when capability was granted).
- `internal/hfir/adapter_blockers_test.go`:
  `caught_sleep` fixture updated from `(sleep ...)` to `(do (sleep ...) 1)` so the binding produces a value while preserving the deadline escape assertion.
- `internal/checker/numeric_contract_test.go`:
  Added 13 invalid probes and 12 valid program assertions. No checks removed.
- `hf05_value_position_cli_test.go`:
  New test suite asserting rejection across `check`, `build`, `-compile-bc`, and `-compile-hfir-bc`.
- `internal/hfir/value_roles_test.go`:
  New test suite validating adapter rejection of value-less roles and correspondence with `opcode.go`.

**Conclusion:** No test or benchmark reference was weakened.

---

### Item 6: Code Formatting, Linting, and Test Suite Execution

All commands executed on PR head (`692b4dfa`):

1. `gofmt -l .`
   - Command: `gofmt -l .`
   - Output: Empty (0 files unformatted). Clean.

2. `go vet ./...`
   - Command: `go vet ./...`
   - Output: Empty. Clean.

3. `go test ./internal/checker ./internal/hfir .`
   - Command: `go test ./internal/checker ./internal/hfir .`
   - Output:
     ```
     ok   github.com/howlcipher/howlframe/internal/checker  0.069s
     ok   github.com/howlcipher/howlframe/internal/hfir     2.194s
     ok   github.com/howlcipher/howlframe                  176.132s
     ```
   - All tests in all three packages pass.

---

## Recommended Remediation

To complete HF-05:
1. In `internal/checker/types.go` under `case "try_let"`:
   When `try_let` is evaluated in a value context (or if either `catchType` or `successType` is void while the other is not), enforce that both bodies produce a value, e.g.:
   - Check `compatible(catchType, successType)` (or verify that neither `catchType.Kind` nor `successType.Kind` is `ast.Void` when used as a value).
   - Alternatively, in `requireValue`, if `head == "try_let"`, verify both `node.Children[2]` (catch body) and `node.Children[3]` (success body) produce values.
2. In `internal/hfir/model_adapter.go`:
   - In `noValue(id NodeID)`, handle `case "try":`: return `noValue(node.Inputs[1].NodeID) || noValue(catchNode.Inputs[0].NodeID)`.
   - Ensure `statementRole` does not treat `try.success_body` and `try.catch` as statement roles when `try` itself is in a value role.
