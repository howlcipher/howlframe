# Test Matrix Execution Harness with Worker Pool and Failure Limits

Implement a test orchestration runner that executes parameterized test cases across a worker pool with bounded concurrency, per-case timeouts, fail-fast abortion, and structured JSON results.

## Invocation

```bash
./run <matrix.json> <results.json> [--concurrency <int>]
```

- `<matrix.json>`: Path to input configuration file specifying test cases, concurrency, and limits.
- `<results.json>`: Path to output JSON reporting individual case execution results and overall summary.
- `--concurrency <int>` (optional): Overrides the worker concurrency limit specified in `matrix.json`.

## Matrix Specification

`<matrix.json>` has the following structure:

```json
{
  "concurrency": 2,
  "fail_fast": false,
  "default_timeout_ms": 5000,
  "cases": [
    {
      "id": "unit-tests",
      "command": ["/usr/bin/coreutils", "echo", "running unit tests"],
      "env": {"TEST_ENV": "development"},
      "timeout_ms": 3000
    },
    {
      "id": "lint-check",
      "command": ["/usr/bin/coreutils", "echo", "linting"],
      "env": {}
    }
  ]
}
```

Fields:
- `concurrency` (integer, optional, default: 1): Maximum number of test processes to run simultaneously. Must be >= 1.
- `fail_fast` (boolean, optional, default: `false`): If `true`, when any test case fails or times out, cancel starting any remaining pending test cases.
- `default_timeout_ms` (integer, optional, default: 5000): Default timeout in milliseconds for test cases that do not specify `timeout_ms`.
- `cases` (array of objects, required): List of test cases to execute:
  - `id` (string, required): Unique identifier for this test case.
  - `command` (array of strings, required): Executable and arguments.
  - `env` (object, optional, default: `{}`): Additional environment variables to supply to the subprocess (merged on top of base environment).
  - `timeout_ms` (integer, optional): Case-specific timeout in milliseconds (falls back to `default_timeout_ms`).

## Execution Semantics

1. **Validation**:
   - `matrix.json` must be valid JSON with a non-empty `cases` array.
   - Each case must contain a non-empty `id` string and non-empty `command` array of strings. Case `id`s must be unique.
   - If validation fails, write error to `stderr` and exit with code **2**.
2. **Concurrency**:
   - The effective concurrency is: if `--concurrency <int>` CLI argument is given, use that value; otherwise use `matrix.concurrency` (default: 1). Effective concurrency must be at least 1.
   - Up to `concurrency` test processes may execute concurrently.
3. **Execution**:
   - Cases are queued in the order defined in `cases`.
   - Each subprocess executes with merged environment (`process.env` + `case.env`).
   - If a test exceeds its timeout, terminate the process (SIGTERM, then SIGKILL if needed) and record `"status": "timed_out"`, `"exit_code": null`.
   - If a test finishes with exit code `0`, record `"status": "passed"`.
   - If a test finishes with non-zero exit code, record `"status": "failed"`.
4. **Fail Fast**:
   - If `fail_fast: true` and any case finishes with `"failed"` or `"timed_out"`:
     - Allow currently running cases to complete normally.
     - Do not start any remaining queued cases.
     - For each queued case that was never started, record `"status": "skipped"`, `"reason": "fail_fast_triggered"`.

## Output Results Specification

`<results.json>` must contain:

```json
{
  "total": 2,
  "passed": 2,
  "failed": 0,
  "skipped": 0,
  "cases": [
    {
      "id": "unit-tests",
      "status": "passed",
      "exit_code": 0,
      "duration_ms": 45,
      "stdout": "running unit tests\n",
      "stderr": ""
    },
    {
      "id": "lint-check",
      "status": "passed",
      "exit_code": 0,
      "duration_ms": 30,
      "stdout": "linting\n",
      "stderr": ""
    }
  ]
}
```

Fields:
- `total`: Total count of test cases.
- `passed`: Count of cases with status `"passed"`.
- `failed`: Count of cases with status `"failed"` or `"timed_out"`.
- `skipped`: Count of cases with status `"skipped"`.
- `cases`: Array of results in the original order of `matrix.cases`.
  - For executed cases: `id`, `status` (`"passed"`, `"failed"`, or `"timed_out"`), `exit_code` (integer or `null`), `duration_ms` (integer >= 0), `stdout` (string), `stderr` (string).
  - For skipped cases: `id`, `status`: `"skipped"`, `reason`: `"fail_fast_triggered"`.

## Exit Codes

- `0`: All test cases passed (`passed == total`).
- `1`: One or more test cases failed, timed out, or were skipped.
- `2`: Invalid CLI arguments, malformed JSON, or missing required fields.
