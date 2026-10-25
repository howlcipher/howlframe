# DAG Pipeline Subprocess Runner with Deadlines

Implement a command-line pipeline runner that executes a directed acyclic graph (DAG) of subprocess tasks with per-task deadlines, retry limits, and status reporting.

## Invocation

```bash
./run <pipeline.json> <report.json>
```

- `<pipeline.json>`: Path to input JSON file specifying pipeline tasks.
- `<report.json>`: Path to output JSON file where execution report will be written.

## Input Specification

The input file `<pipeline.json>` has the following structure:

```json
{
  "tasks": [
    {
      "id": "build",
      "command": ["/usr/bin/coreutils", "echo", "building"],
      "timeout_ms": 2000,
      "max_retries": 1,
      "depends_on": []
    },
    {
      "id": "test",
      "command": ["/usr/bin/coreutils", "echo", "testing"],
      "timeout_ms": 3000,
      "max_retries": 0,
      "depends_on": ["build"]
    }
  ]
}
```

Fields for each task:
- `id` (string, required): Unique task identifier.
- `command` (array of strings, required): Command and arguments to execute as a subprocess. Must contain at least one element.
- `timeout_ms` (integer, optional, default: 5000): Maximum execution time in milliseconds for each attempt.
- `max_retries` (integer, optional, default: 0): Number of retries permitted after an initial failure or timeout.
- `depends_on` (array of strings, optional, default: `[]`): List of task `id`s that must complete successfully before this task runs.

## Validation Rules

Before executing any task, the program must validate:
1. Valid JSON format and top-level object containing a `"tasks"` array.
2. Every task has a non-empty `id` string and a non-empty `command` array of strings. Task `id`s must be unique.
3. Every `id` in `depends_on` references an existing task defined in the pipeline.
4. The dependency graph must be acyclic (no circular dependencies).

If validation fails:
- Print an error description to `stderr`.
- Do not create or write `<report.json>`.
- Exit with status code **2**.

## Execution Semantics

1. Tasks run in dependency order. A task is ready when all tasks listed in `depends_on` have completed with `"status": "success"`.
2. When multiple tasks are ready simultaneously, execute them in ascending lexicographical (ASCII / code-point) order of their `id`.
3. For each task:
   - Run the subprocess with the specified `command`.
   - If the task exceeds `timeout_ms`, terminate the process and record the attempt as timed out.
   - If the task exits with a non-zero exit code or times out, and remaining retries > 0, retry the execution.
   - Total attempts made = 1 initial attempt + retries performed.
   - If the task finishes with exit code `0` within `timeout_ms`, its status is `"success"`.
   - If the task exhausts all attempts (including `max_retries`) and still fails or times out, its status is `"failed"`.
4. Downstream tasks:
   - If any task in `depends_on` fails or is skipped, the dependent task must not be executed. It must be marked with `"status": "skipped"` and `"reason": "dependency_failed"`.

## Output Report Specification

`<report.json>` must be formatted JSON containing:

```json
{
  "success": true,
  "total_tasks": 2,
  "completed_tasks": 2,
  "failed_tasks": 0,
  "skipped_tasks": 0,
  "tasks": {
    "build": {
      "status": "success",
      "attempts": 1,
      "exit_code": 0,
      "stdout": "building\n",
      "stderr": ""
    },
    "test": {
      "status": "success",
      "attempts": 1,
      "exit_code": 0,
      "stdout": "testing\n",
      "stderr": ""
    }
  }
}
```

Task report properties:
- For `"status": "success"` or `"failed"`:
  - `status`: `"success"` or `"failed"`.
  - `attempts`: integer count of attempts made.
  - `exit_code`: integer exit code of the final attempt (or `null` if the final attempt timed out).
  - `timed_out`: boolean, `true` if the final attempt timed out, `false` otherwise.
  - `stdout`: captured standard output string from the final attempt.
  - `stderr`: captured standard error string from the final attempt.
- For `"status": "skipped"`:
  - `status`: `"skipped"`.
  - `reason`: `"dependency_failed"`.

Summary properties:
- `success`: `true` if every task in the pipeline has `"status": "success"`, else `false`.
- `total_tasks`: Total number of tasks in the pipeline.
- `completed_tasks`: Count of tasks with status `"success"`.
- `failed_tasks`: Count of tasks with status `"failed"`.
- `skipped_tasks`: Count of tasks with status `"skipped"`.
- `tasks`: Object keyed by task `id` in ascending code point order.

## Exit Codes

- `0`: Pipeline completed and all tasks succeeded (`"success": true`).
- `1`: Pipeline executed, but one or more tasks failed or were skipped (`"success": false`).
- `2`: Invalid command-line arguments, malformed input JSON, invalid task structure, unknown dependency, or circular dependency detected.
