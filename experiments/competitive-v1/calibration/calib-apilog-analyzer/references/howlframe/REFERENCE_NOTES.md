# Calibration Reference Implementation Notes

This document details the HowlFrame implementation of `calib-apilog-analyzer` in `reference/`, created for the calibration task defined in `task/prompt.md`.

---

## 1. Test Results & Revision 1 Workload Metrics

### Acceptance Test Suite
* **Test Suite:** `task/acceptance/test_analyzer.py` via `kit/testrunner.py`
* **Command:** `python3 kit/testrunner.py --tests task/acceptance --app "$PWD/reference/run"`
* **Summary:**
  * **Total Tests:** 18
  * **Passed:** 18
  * **Failed:** 0
  * **Skipped:** 0
  * **Execution Time:** ~0.498 seconds

All 18 acceptance tests pass cleanly. Both `reference/build` and `reference/run` invoke `howlframe` directly from the environment `PATH` without path hardcoding or fallbacks.

### Workload Benchmark (250,000 Records, ~46 MB)
* **Command:** `time ./reference/run --input task/workload/requests.jsonl --output /tmp/report.json`
* **Dataset:** 250,000 NDJSON records generated via `cd task/workload && python3 -I -B generate.py` (file size ~46 MB)
* **Execution Time:**
  * `real`: 30.961s (~31s)
  * `user`: 37.333s
  * `sys`: 2.878s
* **Execution Receipt Metrics (`howlframe run --receipt`):**
  * `instructions_used`: 159,684,631 (~160 million instructions; ~638 instructions/record)
  * Throughput: ~5.15 million instructions/second (~8,075 records/second)
* **Output Validation:**
  * `summary.total_records`: 250,000
  * `summary.valid_records`: 247,500
  * `summary.invalid_records`: 2,500 (100% of injected corrupt lines correctly caught)
  * `windows`: 2,500 tumbling windows, accurately bucketed and chronologically ordered
  * `services`: 8 services with complete error rates and latency statistics
  * `top_clients`: 10 clients sorted by request count descending with client ID tie-breaking

---

## 2. Resource Limit Sizing (250k and 10x Workloads)

The default VM resource ceilings (`--max-instructions 100000`, `--max-memory-bytes 67108864` [64 MiB cumulative]) are insufficient for large data processing:
1. **Cumulative Memory Accounting:** HowlFrame VM's `--max-memory-bytes` tracks total cumulative allocations over the process lifetime without decrementing on garbage collection. In a 46 MB file, reading the file byte slice and converting to string consumes ~92 MB immediately, exceeding the default 64 MiB limit before line parsing begins (triggering an uncaught `LIMIT_EXCEEDED` error wrapped in `read_file`'s catch block, resulting in the misleading exit 2).
2. **Instruction Counts:** Processing 250k records takes ~160 million instructions (~638 instructions/record for line splitting, JSON parsing, validation, dictionary updates, and metrics aggregation).

### 10x Scale Projections (2,500,000 Records, ~460 MB)
* **Instruction Demand:** $2.5\text{M records} \times 638 \approx 1,596,846,310$ instructions (~1.6 billion instructions).
* **Cumulative Allocation Demand:**
  * File read & string conversion: ~920 MB
  * String line slicing: ~500 MB
  * Per-record JSON parsing & intermediate dicts: ~2.5 GB
  * Record formatting & state maps: ~5–10 GB
  * Total cumulative allocation: ~15–25 GB.

### Selected Ceilings in `reference/run`
```sh
exec howlframe run --allow-caps filesystem --max-instructions 2000000000 --max-memory-bytes 68719476736 "$DIR/main.hfbc" "$@"
```
* **`--max-instructions 2000000000` (2 billion instructions):** Provides a 25% safety margin over the projected 1.6 billion instructions needed for 2.5M records.
* **`--max-memory-bytes 68719476736` (64 GiB cumulative):** Provides a ~2.5x to 4x safety factor over the projected ~20 GB cumulative allocations for 2.5M records, preventing `LIMIT_EXCEEDED` aborts during large-scale runs.

---

## 3. Algorithm Optimization: Zero-Padded Keys for $O(K)$ Window Sorting

In HowlFrame, `sort_by` is desugared to an AST insertion sort that constructs intermediate lists with repeated `append` operations ($O(N^2)$ complexity). For small lists (e.g. 50 top clients), insertion sort takes negligible time. However, for 2,500 tumbling windows, an $O(N^2)$ insertion sort produces over 3.1 million list operations and gigabytes of intermediate allocations, stalling execution.

### Solution
In HowlFrame, `(map_keys dict)` returns map keys in native ascending lexicographical order. By formatting window keys with a fixed 16-character zero-padded string:
```lisp
(let (win_key (pad_left (to_string win_start) 16 "0"))
  (map_set windows_map win_key new_w)
)
```
Lexicographical ordering is identical to numeric chronological ordering. Iterating directly over `(map_keys windows_map)` yields pre-sorted windows in $O(K)$, completely eliminating `sort_by` for window generation and allowing the 250k workload to finish in ~31 seconds.

---

## 4. Option Validation & The `to_float "nan"` Discrepancy

### Sealed Test Failure Analysis
During initial evaluation, `--error-rate-threshold nan` exited 0 instead of 1.
* `docs/docs/reference/NUMERIC_CONTRACT.md:26` explicitly states:
  > `to_float` of `"NaN"`, `"Inf"`, `"Infinity"`, etc. fails with `CONVERSION_ERROR`.
* However, in the compiler and VM, `to_float "nan"`, `to_float "NaN"`, `to_float "inf"`, and `to_float "Infinity"` return floating-point `NaN` and `+Inf` without raising an error.
* Under IEEE 754 comparison semantics implemented by the VM, `NaN < 0.0` is `false` and `NaN > 1.0` is `false`. Thus, range checks evaluating `(or (< rate 0.0) (> rate 1.0))` evaluated to `false`, allowing `NaN` to be silently accepted.

### Fix
In `reference/main.howl`, the option parser checks for IEEE 754 `NaN` using identity comparison `(!= parsed_rate parsed_rate)` (which evaluates to `true` exclusively for `NaN`):
```lisp
(if (or (!= parsed_rate parsed_rate) (or (< parsed_rate 0.0) (> parsed_rate 1.0)))
  (do (stderr "Error rate out of bounds\n") (exit 1))
  ...
)
```
Numeric integer options (`--sla-latency-ms` and `--window-seconds`) parse values with `to_int`, which correctly throws a conversion error on `"nan"`, `"inf"`, and fractional floats, cleanly exiting with code 1 and printing an error to `stderr`.

---

## 5. Approximate Time Spent

* **Part 1 Documentation Audit & Snippet Verification:** ~45 minutes
* **Part 2 Reference Implementation Architecture & Coding:** ~30 minutes
* **Acceptance Testing, Refinement & Documentation:** ~20 minutes
* **Total Time:** ~1.5 hours

---

## 6. Documentation Inaccuracies and Insufficiencies Encountered

While developing the reference implementation using only `docs/`, several areas where the documentation was either wrong, incomplete, or misleading were encountered:

1. **Default Type Assumptions in `defun` Signatures:**
   * *Issue:* The documentation (`SKILL.md:50`, `README.md:369`) describes `defun` without explaining how untyped parameters and return types are resolved.
   * *Reality:* Any untyped parameter or untyped return value in a `defun` defaults strictly to type `string`. When defining helper functions for numeric operations (such as `round_2`, `validate_whole_number`), omitting type hints causes compile errors such as `* requires numeric operands, got string and float64` and `= compares incompatible types string and int`.
   * *Resolution:* Explicit type hints were required: `(type_hint v "any"|"int"|"float")` and `(type_hint return "int"|"float"|"void")`.

2. **`read_file` Returns `[]byte`, Not `string`:**
   * *Issue:* `SKILL.md` suggests `read_file` returns text.
   * *Reality:* `read_file` returns raw byte slice `[]byte`, unlike `read_stdin` which returns `string`. Assigning `read_file` output to a string variable or attempting to split it with `str_split` fails at compile time with `cannot assign []byte to ... of type string`.
   * *Resolution:* Must explicitly convert the file payload using `(bytes_to_string (read_file path))`.

3. **`write_file` is a Void Statement:**
   * *Issue:* `SKILL.md:230` advises that "All input parsing (parse_json), file operations (read_file), and network calls (fetch) use try_let or error checking."
   * *Reality:* Because `write_file` produces no value (statement type `void`), attempting to bind its result via `(try_let (res (write_file path data)) ...)` fails at compile time with `try_let binding of "res" is a void expression and produces no value`.
   * *Resolution:* Catching write failures requires wrapping the write in a non-void sequence: `(try_let (ignored (do (write_file path data) "ok")) ...)`.

4. **`try_let` Arity Requirements:**
   * *Issue:* `SKILL.md:51` documents error handling as a two-operand form: `(try_let (var expr) (catch err ...))`.
   * *Reality:* `try_let` strictly requires three arguments: the binding pair, the catch block, and the success then-body expression `(try_let (var expr) (catch err ...) then_body)`. Providing two arguments fails at compile time with `try_let expects 3 arguments, got 2`.

5. **`stderr` Takes Exactly One Operand:**
   * *Issue:* `print` accepts a variable number of arguments (`PRINT int64 var 0`).
   * *Reality:* `stderr` accepts strictly one argument (`STDERR 1 0`). Passing multiple arguments like `(stderr "Error: " path "\n")` fails at compile time with `ARITY_ERROR: stderr expects 1 arg, got 3`.
   * *Resolution:* Values must be joined using `(str ...)` (which desugars to `str_join`) or passed across multiple `stderr` invocations.

6. **`parse_json` Operand Constraints in `cli_app`:**
   * *Issue:* Outside HTTP routes, `parse_json` requires its target argument to be a declared variable symbol (e.g. `(parse_json Any var_name)`).
   * *Reality:* Passing an expression or string literal directly (e.g. `(parse_json Any (read_stdin))` or `(parse_json Any "{}")`) fails with `parse_json body must be a variable name, not an expression or literal; bind it with let first`.

7. **Lack of Standard Numeric Rounding Built-in:**
   * *Issue:* The task requires rounding metrics to 2 and 4 decimal places. HowlFrame provides no built-in `round` or formatting function.
   * *Resolution:* Floating-point rounding was implemented via integer promotion and division: `(/ (to_float (to_int (+ (* x factor) 0.5))) factor)`.

8. **Lack of `nil` Literal Identifier:**
   * *Issue:* The task contract requires `earliest_timestamp` and `latest_timestamp` to be `null` when there are zero valid records. `SKILL.md:131` references `(is_nil rec)`, but writing `nil` directly in source fails with `undefined reference to "nil"`.
   * *Resolution:* A `null` value was produced by referencing an unassigned map key from an empty dictionary: `(map_get (dict) "nil")`, which evaluates to nil and serializes to JSON as `null`.

---

## 7. Compiler Bugs Discovered

1. **Module Import / Function Call Bytecode Name Mismatch:**
   * *Description:* When modularizing code using `(module ...)` and `(use "mod.howl" as alias)`, invoking an exported function via `(call alias.func args...)` compiles and builds successfully. However, the bytecode writer registers the function in the function table as `alias_func` (converting the period to an underscore), while emitting `CALL alias.func` at the call site.
   * *Consequence:* The bytecode VM immediately halts upon loading the `.hfbc` artifact with `Cannot parse bytecode: structurally invalid bytecode program: main: instruction 1: references missing function "alias.func"`. On the interpreter, it reports `"alias.func" is not a defined function`.
   * *Workaround:* Code must either avoid `use` modules for bytecode targets or call the mangled function name directly `(call alias_func ...)`. For the reference implementation, all logic was encapsulated in `main.howl`.

2. **Default Toolchain Target Discrepancy:**
   * *Description:* `README.md:173` states that running `howlframe <source.howl>` defaults to generating Go code (`server.go`).
   * *Consequence:* Invoking the compiler without target flags compiles directly to bytecode and attempts standalone VM execution. It does not invoke Go or write `server.go`.
