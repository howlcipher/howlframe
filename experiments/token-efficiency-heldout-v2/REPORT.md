# Held-out-v2 token benchmark: baseline on main `576c87c1`

This is a measurement only, taken **before any control-flow change**. It is the
"before" number that the next control-flow slice must beat on this set. The
specs (`tasks/`) were written by Hange (R&D seat) and frozen at `b3d67091`; they
are untouched here (`TestHeldoutV2SpecsFrozen`). No language feature, opcode,
capability or effect was added. Production `-compile-bc`/HFIR defaults are
unchanged, #90 stays Partial, and no live LLM was used.

## Headline

**HowlFrame loses to Python on held-out v2, clearly. It wins 2 of 37 tasks.**

| Comparison (cl100k_base) | Median per-task ratio | Ratio of medians | Total-token ratio | W/T/L |
|---|---|---|---|---|
| HowlFrame vs Python | 1.306 | 1.295 | 1.324 (4084 / 3084) | 2/0/35 |
| HowlFrame vs JS | 1.136 | 1.110 | 1.167 (4084 / 3500) | 7/2/28 |
| HowlFrame vs Go | 0.590 | 0.558 | 0.586 (4084 / 6972) | 37/0/0 |

o200k_base: vs Python 1.306 median / 1.321 total (2/0/35); vs JS 1.083 / 1.112
(10/1/26); vs Go 0.582 / 0.583 (37/0/0). The tokenizer is *not* neutral for
JS: on o200k the HowlFrame-vs-JS median drops from 1.136 to 1.083 and wins rise
from 7 to 10 (the #146 PR body called o200k "nearly identical"; that was wrong
for JS). Python and Go are near-identical across the two encodings.

All 148 references (37 tasks x 4 languages) pass the verifier. `gaps.json` is
empty: every task is expressible in current HowlFrame, but many only through
workarounds (below), which is where the tokens go.

## Per category (cl100k_base, HowlFrame / Python)

| Category | n | Median per-task | Total-token |
|---|---|---|---|
| text | 4 | 1.254 | 1.314 |
| table | 4 | 1.279 | 1.378 |
| numbers | 4 | 1.433 | 1.470 |
| lists | 4 | 1.423 | 1.391 |
| strings | 4 | 1.344 | 1.321 |
| records | 4 | 1.190 | 1.242 |
| **logic** | 13 | **1.244** | **1.290** |

## Per task (cl100k_base tokens)

| Task | Cat | howl | py | js | go | howl/py |
|---|---|---|---|---|---|---|
| w01 | text | 111 | 95 | 106 | 168 | 1.17 |
| w02 | text | 79 | 59 | 61 | 133 | 1.34 |
| w03 | text | 104 | 57 | 64 | 143 | 1.82 |
| w04 | text | 66 | 63 | 80 | 163 | 1.05 |
| w05 | table | 98 | 76 | 88 | 177 | 1.29 |
| w06 | table | 97 | 79 | 101 | 179 | 1.23 |
| w07 | table | 104 | 82 | 91 | 202 | 1.27 |
| w08 | table | 135 | 78 | 86 | 181 | 1.73 |
| w09 | numbers | 56 | 50 | 61 | 150 | 1.12 |
| w10 | numbers | 117 | 80 | 108 | 166 | 1.46 |
| w11 | numbers | 139 | 79 | 127 | 220 | 1.76 |
| w12 | numbers | 101 | 72 | 97 | 206 | 1.40 |
| w13 | lists | 79 | 62 | 78 | 171 | 1.27 |
| w14 | lists | 123 | 86 | 105 | 221 | 1.43 |
| w15 | lists | 94 | 66 | 74 | 205 | 1.42 |
| w16 | lists | 81 | 57 | 59 | 164 | 1.42 |
| w17 | strings | 92 | 57 | 62 | 156 | 1.61 |
| w18 | strings | 64 | 63 | 64 | 165 | 1.02 |
| w19 | strings | 91 | 69 | 73 | 129 | 1.32 |
| w20 | strings | 78 | 57 | 61 | 125 | 1.37 |
| w21 | records | 99 | 65 | 69 | 174 | 1.52 |
| w22 | records | 99 | 88 | 104 | 226 | 1.12 |
| w23 | records | 90 | 78 | 83 | 198 | 1.15 |
| w24 | records | 92 | 75 | 81 | 190 | 1.23 |
| w25 | logic | 128 | 77 | 102 | 188 | 1.66 |
| w26 | logic | 94 | 72 | 76 | 157 | 1.31 |
| w27 | logic | 91 | 95 | 98 | 184 | **0.96** |
| w28 | logic | 146 | 133 | 155 | 229 | 1.10 |
| w29 | logic | 108 | 90 | 118 | 214 | 1.20 |
| w30 | logic | 163 | 140 | 147 | 254 | 1.16 |
| w31 | logic | 104 | 49 | 54 | 151 | 2.12 |
| w32 | logic | 142 | 124 | 142 | 232 | 1.15 |
| w33 | logic | 143 | 95 | 113 | 252 | 1.51 |
| w34 | logic | 227 | 145 | 123 | 250 | 1.57 |
| w35 | logic | 140 | 100 | 123 | 230 | 1.40 |
| w36 | logic | 110 | 111 | 98 | 179 | **0.99** |
| w37 | logic | 199 | 160 | 168 | 240 | 1.24 |

## Workarounds forced by current HowlFrame (observations, not fixes)

Recorded so the next slice can be judged against them; none was implemented.

- No modulo / integer division: `(- n (* d (to_int (/ n d))))` everywhere
  (w10, w15, w25-w27, w31, w32). `/` returns a float.
- No early exit (`break`/`return` from a loop): result flags plus guarded
  loops (w28, w29, w35).
- No range/index iteration: manual `while` counters (w12-w16, w34, w35).
- No negative integer literal: `(- 0 1)` (w34).
- No char code / translate: alphabet `index_of` table (w19). No base
  formatting (w31). No right-trim, pad or repeat (w03, w08, w20).
- No sort by key: distinct-scores loop for w33. `map_keys` returns sorted
  string keys, so numeric buckets needed a counted loop (w11).
- Governed profile bans unknown heads such as `count`: any head that is not a
  registry construct (including a helper or defun called by bare name) is
  `PROFILE_FORBIDDEN_CONSTRUCT` under `--profile governed`, so references
  cannot factor small helpers; they inline the logic instead (Motoko, #146).
- `let` binding limits: a `let` takes exactly one body form (multi-statement
  bodies need `(do ...)`), and bindings must be flat `(name value ...)` pairs
  (no destructuring). Motoko also reported `let` as non-sequential while
  golfing; on main a later binding does see an earlier one
  (`(let (a 2 b (* a 3)) ...)` gives 6, via nested lets), so the practical
  cost is the extra `do` and the lack of destructuring.
- Multi-line indentation is counted like every prior set (v1 style).

## Method

Unchanged from heldout-v1: `cmd/heldoutv2verify` (fresh sandbox, governed
build, `-run-bc` with only `filesystem:read=in`/`filesystem:write=out`, exact
output-file check, sha256 per source), tiktoken 0.8.0 (`requirements.txt`),
`count_tokens.py` reusing the v0 stripping rules. References are idiomatic,
standard library only, no comments. Reproduce with `./run.sh`; outputs are
byte-identical across reruns.

## Freeze

`results_freeze_test.go` pins sha256 of `gaps.json` and every file in
`results/`. `verification_test.go` checks every task x language is verified
with a current hash.

## After the control-flow slice (`postcf/`)

A **separate** results set. The frozen specs (`tasks/`), the frozen baseline
HowlFrame sources (`solutions/howl/`) and baseline results (`results/`,
`gaps.json`) are untouched and keep their pins (`TestHeldoutV2SpecsFrozen`,
`TestHeldoutV2BaselineResultsFrozen`, `TestHeldoutV2VerificationCoverage`).
The Python/JS/Go references are the same baseline files. Only the HowlFrame
sources were rewritten, into `postcf/solutions/howl/`, and verified and counted
with the same verifier (governed build, `-run-bc`, fs read=in/write=out only)
and the same tokenizer rules. Reproduce with `./run_postcf.sh`
(`TOKBENCH_PYTHON` as for `run.sh`); `postcf/results/` is pinned by
`TestHeldoutV2PostCFResultsFrozen` and checked by
`TestHeldoutV2PostCFVerificationCoverage`. 148/148 pass, no gaps.

**HowlFrame still loses to Python on held-out v2**: 1.154 median per task and
1.167 total on cl100k (5 wins, 32 losses), down from 1.306 / 1.324 (2/35). It
is now roughly level with JS on cl100k (1.028 median, 16/1/20) and ahead of JS
on o200k (0.968 median, 20/0/17). Go stays far behind HowlFrame.

### What the slice added (pure sugar; no opcode, capability or effect)

`(% a b)` / `(// a b)` floored like Python; n-ary `if` (elif chains);
`(break)` in for/while bodies; `(range [a] b)`; negative literals (`-1`,
`-2.5`); `max_by` / `min_by` / `sort_by`; `pad_left` / `pad_right` /
`str_repeat` / `rtrim` / `ltrim`; `(to_base n b)`; `let` destructuring
`((a b) lst)`. `map_keys` is **unchanged**: map keys are strings on every
backend (Go/JS included), so sorting numeric-looking keys numerically would
change existing output on every target; `sort_by k (map_keys d) (to_int k)`
covers the w11 case instead.

### How the sources were rewritten

- 25 of 37 files changed; the other 12 (w02, w04-w07, w17, w18, w21-w24, w30)
  are byte-identical copies of the baseline, because nothing in the slice
  applies to them.
- Rewrites use the new forms where the baseline used a workaround, keeping the
  baseline layout. Two also collapsed structure with pre-existing features:
  w03 (`filter` over `map_list` instead of for/append) and w25 (`filter`/`any`
  over `range` instead of nested while). Motoko's #146 golf showed packing and
  `filter` alone take w03 from 104 to 83 tokens, so about 21 of w03's 52-token
  gain is not attributable to the slice. w25's gain needs `range` and `%`.
- Each changed task also runs the same on the interpreter and on HFIR bytecode,
  except the tasks that use `parse_json` (not supported under `-run`, as on
  main) and w22 (byte-identical to the baseline; it fails with
  CAPABILITY_DENIED under HFIR bytecode on main too, a pre-existing issue).

### Headline (cl100k_base)

| Comparison | Median per-task ratio before → after | Total-token ratio before → after | W/T/L before → after |
|---|---|---|---|
| HowlFrame vs python | 1.306 → **1.154** | 1.324 → **1.167** (4084 → 3599 / 3084) | 2/0/35 → **5/0/32** |
| HowlFrame vs js | 1.136 → **1.028** | 1.167 → **1.028** (4084 → 3599 / 3500) | 7/2/28 → **16/1/20** |
| HowlFrame vs go | 0.590 → **0.519** | 0.586 → **0.516** (4084 → 3599 / 6972) | 37/0/0 → **37/0/0** |

### Per category (cl100k_base, HowlFrame / Python)

| Category | n | Median per-task before → after | Total-token before → after |
|---|---|---|---|
| text | 4 | 1.254 → 1.097 | 1.314 → 1.117 |
| table | 4 | 1.279 → 1.248 | 1.378 → 1.248 |
| numbers | 4 | 1.433 → 1.090 | 1.470 → 1.153 |
| lists | 4 | 1.423 → 1.186 | 1.391 → 1.210 |
| strings | 4 | 1.344 → 1.138 | 1.321 → 1.220 |
| records | 4 | 1.190 → 1.190 | 1.242 → 1.242 |
| logic | 13 | 1.244 → 1.150 | 1.290 → 1.127 |

### Headline (o200k_base)

| Comparison | Median per-task ratio before → after | Total-token ratio before → after | W/T/L before → after |
|---|---|---|---|
| HowlFrame vs python | 1.306 → **1.164** | 1.321 → **1.165** (4095 → 3613 / 3100) | 2/0/35 → **5/0/32** |
| HowlFrame vs js | 1.083 → **0.968** | 1.112 → **0.981** (4095 → 3613 / 3683) | 10/1/26 → **20/0/17** |
| HowlFrame vs go | 0.582 → **0.520** | 0.583 → **0.514** (4095 → 3613 / 7030) | 37/0/0 → **37/0/0** |

### Per category (o200k_base, HowlFrame / Python)

| Category | n | Median per-task before → after | Total-token before → after |
|---|---|---|---|
| text | 4 | 1.254 → 1.097 | 1.314 → 1.117 |
| table | 4 | 1.247 → 1.228 | 1.348 → 1.224 |
| numbers | 4 | 1.433 → 1.096 | 1.470 → 1.160 |
| lists | 4 | 1.403 → 1.187 | 1.381 → 1.201 |
| strings | 4 | 1.351 → 1.145 | 1.325 → 1.224 |
| records | 4 | 1.197 → 1.197 | 1.248 → 1.248 |
| logic | 13 | 1.242 → 1.149 | 1.290 → 1.127 |

### Per task (HowlFrame tokens before → after; cl100k_base and o200k_base)

| Task | Cat | cl100k howl | py | js | go | howl/py | o200k howl | py | js | go | howl/py |
|---|---|---|---|---|---|---|---|---|---|---|---|
| w01 | text | 111 → 109 | 95 | 106 | 168 | 1.17 → 1.15 | 111 → 109 | 95 | 110 | 170 | 1.17 → 1.15 |
| w02 | text | 79 (=) | 59 | 61 | 133 | 1.34 → 1.34 | 79 (=) | 59 | 65 | 134 | 1.34 → 1.34 |
| w03 | text | 104 → 52 | 57 | 64 | 143 | 1.82 → **0.91** | 104 → 52 | 57 | 68 | 144 | 1.82 → **0.91** |
| w04 | text | 66 (=) | 63 | 80 | 163 | 1.05 → 1.05 | 66 (=) | 63 | 86 | 165 | 1.05 → 1.05 |
| w05 | table | 98 (=) | 76 | 88 | 177 | 1.29 → 1.29 | 98 (=) | 78 | 92 | 178 | 1.26 → 1.26 |
| w06 | table | 97 (=) | 79 | 101 | 179 | 1.23 → 1.23 | 97 (=) | 82 | 105 | 180 | 1.18 → 1.18 |
| w07 | table | 104 (=) | 82 | 91 | 202 | 1.27 → 1.27 | 104 (=) | 84 | 96 | 203 | 1.24 → 1.24 |
| w08 | table | 135 → 94 | 78 | 86 | 181 | 1.73 → 1.21 | 135 → 95 | 78 | 90 | 182 | 1.73 → 1.22 |
| w09 | numbers | 56 → 54 | 50 | 61 | 150 | 1.12 → 1.08 | 56 → 54 | 50 | 65 | 151 | 1.12 → 1.08 |
| w10 | numbers | 117 → 88 | 80 | 108 | 166 | 1.46 → 1.10 | 117 → 89 | 80 | 112 | 167 | 1.46 → 1.11 |
| w11 | numbers | 139 → 106 | 79 | 127 | 220 | 1.76 → 1.34 | 139 → 107 | 79 | 131 | 222 | 1.76 → 1.35 |
| w12 | numbers | 101 → 76 | 72 | 97 | 206 | 1.40 → 1.06 | 101 → 76 | 72 | 101 | 208 | 1.40 → 1.06 |
| w13 | lists | 79 → 70 | 62 | 78 | 171 | 1.27 → 1.13 | 78 → 69 | 61 | 81 | 171 | 1.28 → 1.13 |
| w14 | lists | 123 → 115 | 86 | 105 | 221 | 1.43 → 1.34 | 124 → 116 | 88 | 112 | 223 | 1.41 → 1.32 |
| w15 | lists | 94 → 82 | 66 | 74 | 205 | 1.42 → 1.24 | 94 → 82 | 66 | 80 | 207 | 1.42 → 1.24 |
| w16 | lists | 81 → 61 | 57 | 59 | 164 | 1.42 → 1.07 | 81 → 61 | 58 | 63 | 165 | 1.40 → 1.05 |
| w17 | strings | 92 (=) | 57 | 62 | 156 | 1.61 → 1.61 | 92 (=) | 57 | 66 | 158 | 1.61 → 1.61 |
| w18 | strings | 64 (=) | 63 | 64 | 165 | 1.02 → 1.02 | 64 (=) | 63 | 70 | 165 | 1.02 → 1.02 |
| w19 | strings | 91 → 82 | 69 | 73 | 129 | 1.32 → 1.19 | 92 → 83 | 69 | 81 | 131 | 1.33 → 1.20 |
| w20 | strings | 78 → 62 | 57 | 61 | 125 | 1.37 → 1.09 | 78 → 62 | 57 | 65 | 126 | 1.37 → 1.09 |
| w21 | records | 99 (=) | 65 | 69 | 174 | 1.52 → 1.52 | 99 (=) | 65 | 73 | 175 | 1.52 → 1.52 |
| w22 | records | 99 (=) | 88 | 104 | 226 | 1.12 → 1.12 | 100 (=) | 88 | 110 | 228 | 1.14 → 1.14 |
| w23 | records | 90 (=) | 78 | 83 | 198 | 1.15 → 1.15 | 91 (=) | 78 | 89 | 198 | 1.17 → 1.17 |
| w24 | records | 92 (=) | 75 | 81 | 190 | 1.23 → 1.23 | 92 (=) | 75 | 85 | 190 | 1.23 → 1.23 |
| w25 | logic | 128 → 66 | 77 | 102 | 188 | 1.66 → **0.86** | 128 → 67 | 77 | 106 | 190 | 1.66 → **0.87** |
| w26 | logic | 94 → 87 | 72 | 76 | 157 | 1.31 → 1.21 | 94 → 87 | 72 | 80 | 160 | 1.31 → 1.21 |
| w27 | logic | 91 → 79 | 95 | 98 | 184 | 0.96 → **0.83** | 92 → 80 | 95 | 102 | 185 | 0.97 → **0.84** |
| w28 | logic | 146 → 137 | 133 | 155 | 229 | 1.10 → 1.03 | 147 → 138 | 134 | 162 | 231 | 1.10 → 1.03 |
| w29 | logic | 108 → 105 | 90 | 118 | 214 | 1.20 → 1.17 | 108 → 105 | 90 | 124 | 217 | 1.20 → 1.17 |
| w30 | logic | 163 (=) | 140 | 147 | 254 | 1.16 → 1.16 | 163 (=) | 140 | 151 | 255 | 1.16 → 1.16 |
| w31 | logic | 104 → 41 | 49 | 54 | 151 | 2.12 → **0.84** | 104 → 41 | 49 | 59 | 152 | 2.12 → **0.84** |
| w32 | logic | 142 → 128 | 124 | 142 | 232 | 1.15 → 1.03 | 143 → 129 | 126 | 147 | 237 | 1.13 → 1.02 |
| w33 | logic | 143 → 142 | 95 | 113 | 252 | 1.51 → 1.49 | 145 → 143 | 96 | 120 | 255 | 1.51 → 1.49 |
| w34 | logic | 227 → 208 | 145 | 123 | 250 | 1.57 → 1.43 | 227 → 208 | 145 | 130 | 251 | 1.57 → 1.43 |
| w35 | logic | 140 → 122 | 100 | 123 | 230 | 1.40 → 1.22 | 141 → 123 | 101 | 130 | 234 | 1.40 → 1.22 |
| w36 | logic | 110 → 106 | 111 | 98 | 179 | 0.99 → **0.95** | 111 → 107 | 112 | 102 | 180 | 0.99 → **0.96** |
| w37 | logic | 199 → 184 | 160 | 168 | 240 | 1.24 → 1.15 | 200 → 185 | 161 | 174 | 242 | 1.24 → 1.15 |

### Where HowlFrame still loses

- records (w21-w24) and w05-w07, w17, w30 have no slice-relevant workaround;
  the cost there is s-expression structure (`(map_set d k (+ (nz (map_get d k)
  0) v))` against `d[k] = d.get(k, 0) + v`) and the explicit
  `parse_json Any (read_text ...)` / `write_file` / `encode_json` I/O.
- w33 (sort by two keys) barely moves: two stable `sort_by` passes plus a
  dense-rank loop cost as much as the old distinct-score scan.
- w34 (grid neighbours) is still 1.43x Python: indexing a list of lists costs
  `(list_get (list_get g y) x)` per access.
