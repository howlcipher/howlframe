# Held-out-v1 token benchmark (core library): REPORT

This is a measurement only. The specs were frozen in commit `31f85c14` before
any core-library code or reference solution was written. The freeze hashes are
in `docs/journals/2026-10-12_core_library_heldout_v1.md`.

## Headline

**HowlFrame is roughly level with Python on this set, and slightly behind it
per task. It does not win.**

| Comparison (cl100k_base) | Median per-task ratio | W/T/L |
|---|---|---|
| HowlFrame vs Python | 1.026 | 16/0/18 |
| HowlFrame vs JS | 0.922 | 26/2/6 |
| HowlFrame vs Go | 0.339 | 34/0/0 |

- Total tokens across all 34 tasks: HowlFrame uses **1.076×** Python's total
  (2132 vs 1982 on cl100k_base; 1.078× on o200k_base), 0.948× JS's and
  0.394× Go's. The per-task median hides that the losses are larger than the
  wins.
- Median tokens are 52.5 for HowlFrame and 53.0 for Python, so the ratio of
  medians is 0.991.
- HowlFrame is clearly shorter than Go and somewhat shorter than JS.
- All 136 references (34 tasks in each of 4 languages) pass the verifier.
  There are no gaps.

## Method

The method is unchanged from heldout-v0:
- Verifier `cmd/heldoutv1verify`: a fresh sandbox per run, the governed build
  for HowlFrame, an exact output-file check, and a sha256 for every source.
- Tokenizers: tiktoken 0.8.0, cl100k_base and o200k_base.
- `count_tokens.py`, which reuses the v0 stripping rules.
- References are idiomatic, use the standard library only, and have no
  comments.

### cl100k_base (tokens, comments included)

| | howl | python | js | go |
|---|---|---|---|---|
| median | 52.5 | 53.0 | 58.0 | 153.5 |

| HowlFrame vs | median per-task ratio | ratio of medians | W/T/L |
|---|---|---|---|
| python | 1.026 | 0.991 | 16/0/18 |
| js | 0.922 | 0.905 | 26/2/6 |
| go | 0.339 | 0.342 | 34/0/0 |

### o200k_base (tokens, comments included)

| | howl | python | js | go |
|---|---|---|---|---|
| median | 52.5 | 53.0 | 62.0 | 155.5 |

| HowlFrame vs | median per-task ratio | ratio of medians | W/T/L |
|---|---|---|---|
| python | 1.026 | 0.991 | 16/0/18 |
| js | 0.862 | 0.847 | 29/0/5 |
| go | 0.336 | 0.338 | 34/0/0 |

### Per category (cl100k_base, mean tokens)

| category | n | howl | python | js | go | median howl/python |
|---|---|---|---|---|---|---|
| lists | 5 | 37.8 | 47.2 | 52.4 | 135.8 | 0.77 |
| logic | 4 | 139.5 | 91.5 | 104.8 | 196.2 | 1.56 |
| numbers | 5 | 54.4 | 58.2 | 58.8 | 179.0 | 1.02 |
| records | 5 | 64.4 | 60.6 | 63.8 | 189.2 | 1.07 |
| strings | 5 | 49.4 | 43.6 | 51.2 | 112.6 | 1.11 |
| table | 5 | 69.0 | 65.0 | 86.0 | 182.4 | 1.12 |
| text | 5 | 39.8 | 48.6 | 54.0 | 125.6 | 0.83 |

### Per task (cl100k_base tokens)

| task | howl | python | js | go | howl/python |
|---|---|---|---|---|---|
| v01 | 43 | 49 | 54 | 127 | 0.88 |
| v02 | 44 | 53 | 58 | 136 | 0.83 |
| v03 | 40 | 47 | 56 | 126 | 0.85 |
| v04 | 29 | 41 | 45 | 103 | 0.71 |
| v05 | 43 | 53 | 57 | 136 | 0.81 |
| v06 | 78 | 68 | 79 | 162 | 1.15 |
| v07 | 75 | 67 | 81 | 191 | 1.12 |
| v08 | 77 | 63 | 83 | 181 | 1.22 |
| v09 | 34 | 53 | 104 | 182 | 0.64 |
| v10 | 81 | 74 | 83 | 196 | 1.09 |
| v11 | 52 | 51 | 53 | 153 | 1.02 |
| v12 | 56 | 68 | 61 | 201 | 0.82 |
| v13 | 63 | 61 | 66 | 206 | 1.03 |
| v14 | 54 | 51 | 54 | 154 | 1.06 |
| v15 | 47 | 60 | 60 | 181 | 0.78 |
| v16 | 48 | 44 | 47 | 132 | 1.09 |
| v17 | 31 | 53 | 41 | 121 | 0.58 |
| v18 | 34 | 44 | 49 | 141 | 0.77 |
| v19 | 31 | 49 | 75 | 151 | 0.63 |
| v20 | 45 | 46 | 50 | 134 | 0.98 |
| v21 | 42 | 39 | 43 | 94 | 1.08 |
| v22 | 50 | 45 | 53 | 122 | 1.11 |
| v23 | 41 | 46 | 46 | 94 | 0.89 |
| v24 | 58 | 48 | 53 | 133 | 1.21 |
| v25 | 56 | 40 | 61 | 120 | 1.40 |
| v26 | 78 | 73 | 81 | 190 | 1.07 |
| v27 | 53 | 54 | 58 | 178 | 0.98 |
| v28 | 74 | 59 | 56 | 212 | 1.25 |
| v29 | 58 | 54 | 58 | 175 | 1.07 |
| v30 | 59 | 63 | 66 | 191 | 0.94 |
| v31 | 161 | 110 | 103 | 202 | 1.46 |
| v32 | 203 | 123 | 154 | 243 | 1.65 |
| v33 | 42 | 51 | 58 | 141 | 0.82 |
| v34 | 152 | 82 | 104 | 199 | 1.85 |

## Hot spots (where HowlFrame loses)

- **logic** (v31 FizzBuzz, v32 brackets, v34 chain): 1.46–1.85× Python.
  - There is no `%`, so FizzBuzz uses counters.
  - There is no `break` and no stack pop. The stack is popped with
    `list_slice`, and a result string acts as an early-exit guard.
  - There is no `elif`.
- **Field extraction after a reduction** (v08, v25, v28, v29): 1.07–1.40×.
  - There is no `max_by`/`min_by` and no `key=`, so the solution builds a key
    list, finds the extreme, and then calls `index_of`.
  - There is no set or `dict.fromkeys`, so first-seen order uses an `index_of`
    scan.
- **Tabular parsing** (v06–v10): 1.09–1.22×. Every row needs
  `(str_split l "\t")` and `(to_int (list_get r i))`, with no destructuring.
- **Negative literals:** `-1` is not lexed as an integer, so the solutions
  write `(- 0 1)` or `(- (list_len q) 3)`.

## Where HowlFrame wins

- Whole-collection transforms that map onto one built-in:
  - `sort`: v04 0.71×, v09 0.64×.
  - `count_by`: v19 0.63×.
  - `index_of`: v17 0.58×.
  - `map_list` and `filter` in the text category: about 0.8×.

## Notes and threats to validity

- **Same author.** The tasks were written by the same agent that then wrote
  the library and the solutions. The specs were frozen first, but the library
  was designed knowing the tasks. Treat this as weaker evidence than a truly
  blind set.
- **v09 sort.** The HowlFrame solution sorts whole lines, while the others
  sort by the sku key. The results are identical because sku is the first
  field and is unique. With duplicate skus, ties would be broken differently.
- **v18 distinct.** It uses `map_keys` of `count_by`, which is documented to
  return keys in byte order.
- **Library semantics.** `str_len` and `str_slice` count bytes, not
  characters (`str_split s ""` splits bytes). `sort` is a stable insertion
  sort, O(n²). Empty `min`/`max` is a runtime `VALUE_ERROR` (since the #139
  hardening slice).
- **Go/JS targets.** The core-library sugar does not compile on the Go/JS
  backends, because their let-in-expression lowering is a known pre-existing
  gap. `build --target go|js` now rejects it with `UNSUPPORTED_ON_TARGET`. All
  HowlFrame references run on governed `-run-bc`.
