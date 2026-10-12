# Held-out token-efficiency tasks, second set (heldout-v1): frozen specs

This file and the others in `tasks/` (`manifest.json`, `build_specs.py`) are
**frozen**. They were committed before any core-library code or reference
solution was written, and before anything was measured.

## Corpus

There are 34 tasks in 7 categories. None of them repeats a task from
token-efficiency-v0 or token-efficiency-heldout-v0.

| Category | Tasks |
|---|---|
| text | v01–v05 |
| table | v06–v10 |
| numbers | v11–v15 |
| lists | v16–v20 |
| strings | v21–v25 |
| records | v26–v30 |
| logic | v31–v34 |

## Running a reference

The rules are the same as heldout-v0:

- Each reference runs with its working directory set to a fresh sandbox
  containing only the task's `inputs`.
- It may read under `in/` and must write exactly the one `output` path.
- `text` outputs are compared exactly, after trailing newlines are removed
  from both sides. `json` outputs are compared as decoded values.
- `expected` values come from the spec-time oracle in `build_specs.py`, which
  is part of this freeze. The oracle is not a counted or verified reference.
- HowlFrame references are built with `build --profile governed` and run with
  `-run-bc` and only `filesystem:read=in`, `filesystem:write=out`.
- A task that a language cannot express with the features on the measured
  commit is recorded as a gap with a reason, not as a solution.
