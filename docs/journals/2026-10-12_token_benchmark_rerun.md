# Token benchmark rerun on current shorthand (2026-10-12)

## Goal

Rerun `experiments/token-efficiency-v0` on main `fd1c6225` (includes the
#124–#130 density sugar). Re-author HowlFrame references in idiomatic current
syntax, leave Python/JS/Go untouched, and fix the #133 REPORT.md issues.

## What changed

- `author_solutions.py`: new `howl_src` generator. It covers all 60 HowlFrame files and uses root `(do …)`,
  multi-binding `let`, `read_text`, `lines`, `when` and integer-bound semver
  parts. It no longer copies `falsifiable-v0/reference/arm_a`. Those files are unchanged.
- `solutions_test.go`: the verbatim-copy check for six references is replaced
  by a guard against the expanded `bytes_to_string` form.
- `results/*`: regenerated. Python/JS/Go source hashes and counts are unchanged.
- `REPORT.md`: before/after, per-task delta, category means, hot spots.
  Changes from #133:
  - Fixed the garbled dependencies sentence and rewrote the stale validation section.
  - Documented two limitations: the JS/Go stripper does not handle regex or
    template literals, and the native verifier does not isolate references from the host.

## Results

| | cl100k_base | o200k_base |
|---|---:|---:|
| HowlFrame median before → after | 160 → 124 | 159 → 123 |
| Python / JS / Go median | 119 / 122 / 250 | 118 / 129 / 250 |
| HF/Python median per-task ratio | 1.422 → 1.059 | 1.417 → 1.058 |
| HF/JS | 1.368 → 1.019 | 1.270 → 0.953 |
| HF/Go | 0.640 → 0.496 | 0.636 → 0.492 |

The conditional-task mean fell from 413.18 to 344.55. Python's is 196.00.
On `o200k_base`, HowlFrame now uses fewer tokens than JS on 38 of 60 tasks, every one of them non-conditional.

## Findings while authoring

- The governed profile rejects `(parse_json Checks (read_text …))` with
  PROFILE_FORBIDDEN_CONSTRUCT, because the profile scan runs before desugar.
  The JSON text therefore needs a separate `raw` binding.
- `list_get` requires a symbol operand, so the split intermediates must be bound.
- A multi-form `let` body still needs `(do …)`.

## Next (hot spots)

1. Strict x.y.z comparison and KV lookup cost about 189 tokens per conditional file.
2. The unit-conditional `failed` loop stands in for `all`/`every` and adds 27 tokens.
3. Envelope overhead: the governed `parse_json` binding, the `do` in `let`, and the `dict` pair syntax.

Measure only. There were no live LLM runs and no H1 decision; #90 stays Partial, and nothing changed for HFIR or `-compile-bc`.
