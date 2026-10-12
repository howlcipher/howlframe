# Held-out token-efficiency benchmark (heldout-v0): measure only

**HowlFrame's v0 token advantage does not generalize.** On a fresh, frozen
corpus of 32 tasks:

- **Python:** HowlFrame uses **more tokens than Python on 29 of the 30 tasks it can
  express** and ties on 1, with both tokenizers. The median per-task ratio is
  **1.309x** (`cl100k_base`) and **1.299x** (`o200k_base`). That compares with
  **0.983x** on the v0 corpus, which the #135 built-ins were shaped for.
- **JavaScript:** roughly at parity, at 1.087x and 1.016x.
- **Go:** HowlFrame still beats standalone Go on every solved task, at about 0.50x.
- **Gaps:** two tasks, both file walking, cannot be expressed in HowlFrame on
  main at all.

This measures source size only: no model success, no generation cost, and no
live LLM/API calls. There is no H1 decision, and #90 stays Partial. No
built-in, syntax, compiler or VM change is part of this PR.

## Corpus and freeze

- **Size:** 32 tasks in 7 categories (text, csv, aggregate, parse, validate,
  reshape, walk). None reuses the three v0 release workflows. See
  [tasks/README.md](tasks/README.md).
- **Freeze:** the specs (`tasks/manifest.json`, `tasks/build_specs.py`,
  `tasks/README.md`) were committed in **`f510cd01`** before any reference was
  written or anything was measured. Their SHA-256 hashes are recorded in the
  journal and enforced by `TestHeldoutSpecsFrozen`.
- **Expected outputs:** these come from a spec-time oracle in `build_specs.py`. It is
  part of the freeze and is never counted or verified as a reference.
- **Not frozen:** the verifier pre-creates an empty `out/` directory, and the
  sandbox contains only the task inputs.

## Method

| Element | Setup |
|---|---|
| Tokenizers | `tiktoken==0.8.0`, `cl100k_base` and `o200k_base`, offline cache only |
| Counting and stripping | Imports v0's `count_tokens.py` (`strip_comments`, offline guard) unchanged |
| Python, JS, Go references | Idiomatic, standard library only, not token-minimized, same style as v0. Go files are standalone `package main` with `//go:build ignore` |
| HowlFrame references | Features on main `24f6a10f` only |

`cmd/heldoutverify` replays every reference in a fresh sandbox:

- **HowlFrame** goes through `howlframe build --profile governed`, then `-run-bc` with
  only `filesystem:read=<sandbox>/in` and `filesystem:write=<sandbox>/out`.
- **Python/Node** run as host processes. **Go** is compiled from the single file and then run.

After each run, the verifier requires all of the following:

- the sandbox's file set is exactly the inputs plus the one output;
- the inputs are unchanged;
- the output matches: exact text after removing trailing newlines, or decoded-JSON equality.

Results are written to `results/verification.json`, which records the SHA-256 of each source.
The counter refuses unverified or changed sources. `TestHeldoutVerificationCoverage` enforces:

- every task × language is either a passing entry with a matching hash or a HowlFrame gap in
  `gaps.json` that has a reason and no solution file;
- there are no duplicates.

Reproduce with `experiments/token-efficiency-heldout-v0/run.sh` (`TOKBENCH_PYTHON` selects the interpreter).

Pass counts: **HowlFrame 30/30 expressible (2 gaps), Python 32/32, JavaScript 32/32, Go 32/32.**

## Results

Medians are over the 30 HowlFrame-solved tasks. The all-32 median is shown in parentheses.

| Encoding | HowlFrame | Python | JavaScript | Go |
| --- | ---: | ---: | ---: | ---: |
| cl100k_base median | 72.5 | 63.0 (62.5) | 69.0 (69.0) | 162.5 (162.5) |
| cl100k_base mean | 85.5 | 60.8 | 71.6 | 162.0 |
| o200k_base median | 72.5 | 63.0 (62.5) | 73.0 (73.0) | 164.5 (164.5) |
| o200k_base mean | 85.6 | 61.1 | 76.3 | 163.6 |

HowlFrame against each language, over the 30 solved tasks. A ratio above 1 means HowlFrame uses more tokens.

| Encoding | vs | Median per-task ratio | Ratio of medians | HowlFrame wins / ties / losses | Gaps |
| --- | --- | ---: | ---: | ---: | ---: |
| cl100k_base | Python | **1.309** | 1.151 | **0 / 1 / 29** | 2 |
| cl100k_base | JavaScript | 1.087 | 1.051 | 6 / 1 / 23 | 2 |
| cl100k_base | Go | 0.499 | 0.446 | 30 / 0 / 0 | 2 |
| o200k_base | Python | **1.299** | 1.151 | **0 / 1 / 29** | 2 |
| o200k_base | JavaScript | 1.016 | 0.993 | 13 / 0 / 17 | 2 |
| o200k_base | Go | 0.493 | 0.441 | 30 / 0 / 0 | 2 |

If the two gaps are counted as losses, the result against Python is 0 / 1 / 31 out of 32.
Comment and blank-line stripping changes only the Go numbers, which go to 0.519/0.513.

**Compared with v0:**

| Corpus | HowlFrame / Python, median per-task ratio | HowlFrame wins against Python |
|---|---|---|
| v0 (#135) | 0.983x on both encodings | 44 wins, 16 ties, 0 losses |
| Held-out | 1.309x / 1.299x | 0 wins, 1 tie, 29 losses |

The v0 result was produced by built-ins tuned on that corpus. It does not
carry over to unseen workflows.

### By category (`cl100k_base`; `o200k_base` is within 0.1 except where shown)

| Category | n | HowlFrame solved | HowlFrame mean | Python mean | JavaScript mean | Go mean | Median HF/Py ratio |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| csv | 5 | 5 | 135.0 | 73.0 | 89.0 | 185.6 | **1.843** (1.817) |
| aggregate | 5 | 5 | 81.8 | 53.6 | 71.8 | 152.4 | 1.476 |
| parse | 5 | 5 | 81.2 | 57.8 | 72.0 | 152.8 | 1.397 |
| text | 5 | 5 | 58.4 | 47.0 | 50.8 | 119.2 | 1.227 (1.200) |
| validate | 5 | 5 | 84.0 | 71.8 | 80.4 | 187.8 | 1.111 |
| reshape | 5 | 5 | 72.4 | 61.8 | 65.6 | 174.2 | 1.079 |
| walk | 2 | **0** | gap | n/a | n/a | n/a | n/a |

### Per task (raw tokens, cl100k_base/o200k_base)

| Task | Category | HowlFrame | Python | JS | Go | HF/Py |
|---|---|---:|---:|---:|---:|---:|
| h01 | text | 54/54 | 44/44 | 49/55 | 112/114 | 1.23 |
| h02 | text | 70/70 | 40/40 | 43/47 | 138/139 | 1.75 |
| h03 | text | 66/66 | 62/62 | 61/65 | 125/126 | 1.06 |
| h04 | text | 66/66 | 53/55 | 61/65 | 133/138 | 1.25 |
| h05 | text | 36/36 | 36/36 | 40/45 | 88/89 | 1.00 |
| h06 | csv | 122/122 | 71/72 | 74/79 | 165/166 | 1.72 |
| h07 | csv | 144/144 | 82/84 | 89/96 | 206/211 | 1.76 |
| h08 | csv | 127/127 | 64/64 | 81/87 | 186/187 | 1.98 |
| h09 | csv | 129/129 | 70/71 | 96/100 | 185/186 | 1.84 |
| h10 | csv | 153/153 | 78/78 | 105/110 | 186/187 | 1.96 |
| h11 | aggregate | 137/137 | 71/71 | 96/101 | 224/225 | 1.93 |
| h12 | aggregate | 92/92 | 67/67 | 91/97 | 154/156 | 1.37 |
| h13 | aggregate | 62/62 | 42/42 | 60/64 | 133/134 | 1.48 |
| h14 | aggregate | 56/56 | 50/50 | 53/57 | 124/125 | 1.12 |
| h15 | aggregate | 62/62 | 38/38 | 59/63 | 127/128 | 1.63 |
| h16 | parse | 109/109 | 59/59 | 84/89 | 152/153 | 1.85 |
| h17 | parse | 88/88 | 63/63 | 86/90 | 145/146 | 1.40 |
| h18 | parse | 62/62 | 54/54 | 57/61 | 126/127 | 1.15 |
| h19 | parse | 88/88 | 63/63 | 81/86 | 181/182 | 1.40 |
| h20 | parse | 59/60 | 50/51 | 52/57 | 160/163 | 1.18 |
| h21 | validate | 64/65 | 61/61 | 68/72 | 173/174 | 1.05 |
| h22 | validate | 99/100 | 72/73 | 73/78 | 197/201 | 1.38 |
| h23 | validate | 80/80 | 72/72 | 83/87 | 154/156 | 1.11 |
| h24 | validate | 73/73 | 69/69 | 74/80 | 218/220 | 1.06 |
| h25 | validate | 104/104 | 85/85 | 104/109 | 197/198 | 1.22 |
| h26 | reshape | 63/63 | 59/59 | 64/68 | 175/176 | 1.07 |
| h27 | reshape | 68/68 | 63/63 | 61/65 | 181/182 | 1.08 |
| h28 | reshape | 97/97 | 64/64 | 70/74 | 183/184 | 1.52 |
| h29 | reshape | 72/72 | 65/66 | 65/69 | 177/178 | 1.11 |
| h30 | reshape | 62/62 | 58/58 | 68/72 | 155/156 | 1.07 |
| h31 | walk | gap/gap | 41/41 | 54/58 | 141/142 | gap |
| h32 | walk | gap/gap | 57/57 | 109/114 | 196/198 | gap |

## Hot spots

These are `cl100k_base` counts of source fragments. They are not an additive attribution.

1. **No index or slice primitives (csv: 1.84x).** Finding a column takes
   about 19 tokens for the header binding plus a 28-token loop of
   `for`/`when`/`set`/`inc`. Python does it in 15 tokens with `header.index("email")`.
   Skipping the header needs a second counter and `(when (> i 0) …)`, 14 tokens
   against 4 for `lines[1:]`.
2. **Counting into a map (aggregate, parse, csv).** Each count is
   `(map_set counts k (+ (if (map_has counts k) (map_get counts k) 0) 1))`,
   about 25 tokens, against Python's `Counter(` at 4. A missing `map_get` returns
   `""`, not nil, so `nz` cannot provide a default here.
3. **Imperative accumulation instead of map/filter/reduce (every category).**
   - Each transform pays `out (list)` + `(for x xs (append out …))` + `(str_join out "\n")`,
     about 21 tokens. Python's generator expression inside `"\n".join(…)` costs 12.
   - Reversal is a 22-token `while`/`dec` loop against `reversed(lines)` at 4.
   - min/max/sum are manual loops.
   - There is no `map`, `filter`, `sum`, `min`, `max`, `reverse` or `index_of`.

## Gaps and findings (recorded, not fixed)

- **h31 and h32 (walk):** HowlFrame has no directory listing or walking construct,
  so neither task can be expressed. h32 would also need string ordering:
  `<` rejects strings with `TYPE_ERROR`, and there is no sort.
- **Sugar inside some call arguments:**
  - Inside `write_file` or `to_lower` arguments, `str_replace`/`read_text` is not
    expanded. The governed build fails with "no lowering for construct".
  - Their parents are missing from `isSugarExprContext`.
  - Workaround used in h05 and h12: bind the value first.
- **`or` evaluates both operands:** h07's `(or (= i 0) (>= (to_int …) 30))` raised a
  `CONVERSION_ERROR` on the header row. Rewritten with `if`.
- **Value semantics:** `append` on a list taken out of a map does not update the
  map, so h28 has to write the list back with `map_set`.

## Threats to validity

- **One author:** the same author wrote the specs, the oracle and all 122 references.
  The specs were frozen first, but that author has seen HowlFrame's
  weaknesses, and the task choice may lean towards Python idioms (CSV,
  `Counter`, slicing).
- **Fixtures:** small, valid inputs only. CSV has no quoting; text has no Unicode edge cases.
- **Proxy only:** these are tokenizer proxies, not model generation cost or correctness.
- **Isolation:** Python, JS and Go run as host processes, with the same lack of host
  isolation as v0. The verifier only checks the sandbox file set afterwards.
  HowlFrame runs under scoped VM filesystem grants.
- **Small samples:** each category has n = 5, so category means are noisy.

## Repository validation status

All of these were run locally on the box, outside a sandbox, with `GOCACHE=/tmp/hf-token-go-cache`:

- `go build ./... && go vet ./... && go test -count=1 ./...`: exit 0.
- `go test -count=1 ./experiments/...`: passes. This includes `TestHeldoutVerificationCoverage` and `TestHeldoutSpecsFrozen`.
- `experiments/token-efficiency-heldout-v0/run.sh`: exit 0, with 122 of 122 verified references passing.
- `experiments/token-efficiency-v0/run.sh`: exit 0. v0 is unchanged.

See the [journal](../../docs/journals/2026-10-12_heldout_token_benchmark.md).
