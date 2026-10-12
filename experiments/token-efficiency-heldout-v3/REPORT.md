# Held-out v3: records/JSON slice, measured blind

Specs in `tasks/` are frozen (authored by bot-assurance-0001, base `d887b96b`,
pinned by `spec_freeze_test.go`) and were not edited. This directory adds the
harness, references and results only.

## Protocol

1. **Phase 1, blind.** The records/JSON forms were designed and committed
   (`e18ad5d`) before anything in this directory was opened, guided only by
   heldout-v2 evidence (records flat at 1.19x Python; cost in s-expression
   structure and JSON I/O).
2. **Phase 2.** After that commit the specs were read and the references were
   written: Python, JS and Go once (`solutions/`), HowlFrame twice:
   - `solutions/howl/` baseline: only features on main `a6bb53b6`. Verified
     35/35 with a binary built from `a6bb53b6` and with the slice binary.
   - `postrec/solutions/howl/` the same programs rewritten to use the slice
     where it applies (`run_postrec.sh`). They do not build on `a6bb53b6`.
3. `run.sh` / `run_postrec.sh` replay every reference in a fresh sandbox
   (`cmd/heldoutv3verify`, same rules as v2: governed build, `-run-bc`,
   `filesystem:read=in`, `filesystem:write=out`), then count tokens with
   tiktoken 0.8.0 (cl100k_base, o200k_base; `count_tokens.py` reuses the v0
   comment stripping). `TOKBENCH_PYTHON=/workspace/tokvenv/bin/python`.
4. Results are pinned by `results_freeze_test.go` and `postrec_test.go`;
   `verification_test.go` checks coverage and source hashes. No gaps.

## Headline (cl100k_base; o200k_base in `postrec/results/compare.md`)

| HowlFrame vs | Median per-task ratio before → after | Total tokens before → after | W/T/L before → after |
|---|---|---|---|
| Python | 1.282 → **1.186** | 3783 → 3547 / 2840 (1.332 → **1.249**) | 3/0/32 → **8/0/27** |
| JS | 1.103 → **1.024** | 3783 → 3547 / 3304 (1.145 → **1.074**) | 13/0/22 → **16/0/19** |
| Go | 0.460 → **0.434** | 3783 → 3547 / 8095 (0.467 → **0.438**) | 35/0/0 → **35/0/0** |

Median tokens per task: HowlFrame 105 → 99, Python 77, JS 94, Go 221.

| Category | n | Median howl/py before → after | Total howl/py before → after |
|---|---|---|---|
| records | 17 | 1.195 → 1.113 | 1.244 → 1.118 |
| text | 3 | 1.282 → 1.282 | 1.377 → 1.377 |
| table | 3 | 1.411 → 1.400 | 1.431 → 1.427 |
| numbers | 3 | 1.260 → 1.260 | 1.411 → 1.370 |
| lists | 3 | 1.247 → 1.137 | 1.184 → 1.047 |
| strings | 3 | 1.458 → 1.458 | 1.436 → 1.436 |
| logic | 3 | 1.766 → 1.766 | 1.684 → 1.654 |

## Reading it honestly

- HowlFrame still **loses to Python on 27 of 35** tasks and on total tokens
  (1.25x). The slice moved records from 1.24x to 1.12x of Python (total), not
  to parity. Against JS it is now roughly even; against Go it wins every task.
- What paid: `read_json`/`write_json` (every JSON task), `get` (1 token
  shorter per read, multi-step paths collapse), `rec` for literal records.
- What did not: `put`/`get` take a static path, so the three tasks with
  runtime dotted paths (x01, x04, x17) still loop by hand and stay at
  1.1-1.5x. `merge` is shallow, so deep merge (x02) is still a recursive
  defun. `pluck`/`select`/`group_by`/`merge` were not used by any v3
  reference: the blind guess at those shapes did not match this corpus.
- Remaining structural costs seen in the baseline: no type predicate (object
  vs array is tested with `(starts_with (encode_json v) "{")`), recursion
  needs a typed `defun`, `(call f …)` and explicit `return`, `append` is a
  statement (no value), and list equality compares references.
- Non-record categories were untouched by design; their ratios moved only
  where a JSON read/write appears.

## Findings for follow-up (not fixed here)

- HFIR bytecode: in `(print (encode_json (parse_json Any s)) (let (t (parse_json Any s)) …))`
  the first argument prints the raw text `s` instead of the re-encoded value
  (reproduces on `a6bb53b6`; AST bytecode and the interpreter are correct).
  The parity test in `records_cli_test.go` avoids that shape.
- `encode_json` converts int64 to float64 (`normalizeForJSON`), so integers
  above 2^53 lose precision on output. No v3 task hits it.

## Notes on the frozen specs (from review of #152; specs not edited)

- `tasks/README.md` says the input file names are new; five reuse earlier
  names with new contents (`config.json`, `people.json`, `prices.json`,
  `nums.txt`, `words.txt`). Contents and operations are new.
- x01 says "compact JSON" while the oracle uses Python's default separators;
  no x01 output contains a separator, so it does not affect scoring. Tighten
  the wording in v4.
