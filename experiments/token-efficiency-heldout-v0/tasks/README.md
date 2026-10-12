# Held-out token-efficiency tasks (heldout-v0): frozen specs

This file and the others in `tasks/` (`manifest.json`, `build_specs.py`) are
**frozen**. They were committed before any reference solution was written or
anything was measured.

## Corpus

There are 32 tasks in 7 categories. None of them is one of the three
falsifiable-v0 release workflows.

| Category | Tasks |
|---|---|
| text | h01–h05 |
| csv | h06–h10 |
| aggregate | h11–h15 |
| parse | h16–h20 |
| validate | h21–h25 |
| reshape | h26–h30 |
| walk | h31–h32 |

## Running a reference

- Each reference runs with its working directory set to a fresh sandbox
  containing only the task's `inputs`.
- It may read under `in/` and must write exactly the one `output` path.
- `text` outputs are compared exactly, after trailing newlines are removed
  from both sides. `json` outputs are compared as decoded values.
- `expected` values come from the spec-time oracle in `build_specs.py`, which
  is part of this freeze. The oracle is not a counted or verified reference.
- A task that a language cannot express with features on main at freeze time
  is recorded as a gap, not solved by adding features.
