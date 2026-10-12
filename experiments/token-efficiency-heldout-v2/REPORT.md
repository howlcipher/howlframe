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
(10/1/26); vs Go 0.582 / 0.583 (37/0/0).

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
