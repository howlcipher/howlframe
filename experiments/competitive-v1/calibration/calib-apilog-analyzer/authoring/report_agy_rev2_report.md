# Revision 2 Completion Report

All points from `REVISION_2.md` have been resolved, explicitly specified in [`calibration_task/prompt.md`](calibration_task/prompt.md), and implemented across all three reference codebases.

---

### 1. Contract Decisions & Explicit Rules

1. **Integer Representation Rule (`timestamp`, `status`, `latency_ms`, `bytes_sent`)**:
   * **Rule**: An integer field must be a finite JSON number representing a whole number (integer value). Whole-number representations without fractional parts such as `1` and `1.0` are accepted as integer value `1`.
   * **Invalid Cases**: Numbers with non-zero fractional parts (e.g. `1.5`), non-finite numbers (`NaN`, `Infinity`, `1e400`), numbers exceeding the safe integer limit $2^{53} - 1$ ($9{,}007{,}199{,}254{,}740{,}991$), and non-number types (strings, booleans, null) are rejected as invalid records.
   * **Language Implementations**:
     * **Python**: Validates that values are finite numbers with `math.floor(val) == val` and $\le 2^{53} - 1$, casting validated values to `int`.
     * **Go**: Validates unmarshaled JSON numbers with `math.Floor(f) == f`, checking against `1<<53 - 1` and `math.IsNaN`/`math.IsInf`.
     * **JavaScript**: Uses `typeof val === 'number' && Number.isSafeInteger(val)` (which natively treats `1` and `1.0` as safe integers and rejects `1.5`, booleans, and values $> 2^{53} - 1$).

2. **Directory Input Path**:
   * **Rule**: If `--input <path>` specifies a path that does not exist, is a directory, or cannot be read, the program must exit with code `2` and print an error message to `stderr`.
   * **Language Implementations**:
     * **Python**: Explicitly checks `os.path.isdir(input_path)` to exit `2`.
     * **Go**: Checks `fi.IsDir()` via `os.Stat(inputPath)` before reading to exit `2`.
     * **JavaScript**: Checks `stat.isDirectory()` via `fs.statSync(inputPath)` before opening stream to exit `2`.

---

### 2. Verification & Acceptance Tests

* Added test cases to [`calibration_task/acceptance/test_analyzer.py`](calibration_task/acceptance/test_analyzer.py) covering:
  * Whole-number float representations (`1.0`, `200.0`, etc.) accepted as valid.
  * Fractional floats (`1.5`) and integers $> 2^{53} - 1$ rejected as invalid.
  * Directory paths passed to `--input` exiting with code `2`.
* Re-ran the acceptance suite against all three references via `kit/testrunner.py`:
  * **Python Reference**: **18 passed**, 0 failed, 0 skipped of 18 (1.153s)
  * **Go Reference**: **18 passed**, 0 failed, 0 skipped of 18 (0.224s)
  * **JavaScript Reference**: **18 passed**, 0 failed, 0 skipped of 18 (3.625s)
* Verified that no pre-built binary (`app`) or generated dataset (`requests.jsonl`) is left in the repository.
* Updated [`calibration_task/AUTHOR_NOTES.md`](calibration_task/AUTHOR_NOTES.md) with a Revision 2 section describing the adopted rules.
