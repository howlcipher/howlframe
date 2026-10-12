# Token-efficiency benchmark v0 — rerun on current shorthand (measure only)

Rerun on main `fd1c6225` (2026-10-12), which includes the #124–#130 density
sugar. The 60 HowlFrame references were re-authored in current idiomatic
syntax; Python, JavaScript and Go references are **byte-identical** to the
`fd1c6225` baseline. Same tasks, same verifier, same `run.sh`/`count_tokens.py`,
same tokenizers (`tiktoken==0.8.0`, `cl100k_base`, `o200k_base`).

Headline: HowlFrame's median fell **160 → 124** (`cl100k_base`) and
**159 → 123** (`o200k_base`). It is now within ~6% of Python and ~2% of
JavaScript on `cl100k_base`, and **beats JavaScript on `o200k_base`**
(median per-task ratio 0.953; fewer tokens on 38/60 tasks). It still uses more
tokens than Python on all 60 tasks, and more than JavaScript on all 22
conditional (version-comparison) tasks under both encodings.

This is a source-size measurement of authored, verified references. It measures
neither model success nor generation/repair cost. No live LLM/API calls were
made, no H1 PASS/KILL decision is claimed, and issue #90 remains Partial. No
language, compiler, checker, VM, HFIR default or production `-compile-bc`
behaviour changed.

## Before vs after

Raw source tokens (before = merged #133 baseline on `fd1c6225` sources; after = this rerun):

| Encoding | Language | Median before | Median after | Mean before | Mean after |
| --- | --- | ---: | ---: | ---: | ---: |
| cl100k_base | HowlFrame | 160 | **124** | 248.83 | **200.60** |
| cl100k_base | Python | 119 | 119 | 142.70 | 142.70 |
| cl100k_base | JavaScript | 122 | 122 | 163.20 | 163.20 |
| cl100k_base | Go | 250 | 250 | 328.00 | 328.00 |
| o200k_base | HowlFrame | 159 | **123** | 248.73 | **200.50** |
| o200k_base | Python | 118 | 118 | 142.60 | 142.60 |
| o200k_base | JavaScript | 129 | 129 | 171.10 | 171.10 |
| o200k_base | Go | 250 | 250 | 328.90 | 328.90 |

HowlFrame / comparison ratios (>1 means HowlFrame uses more tokens):

| Encoding | vs | Median per-task ratio before → after | Ratio of medians before → after | Tasks HowlFrame larger before → after |
| --- | --- | ---: | ---: | ---: |
| cl100k_base | Python | 1.422 → **1.059** | 1.345 → 1.042 | 60 → 60 |
| cl100k_base | JavaScript | 1.368 → **1.019** | 1.311 → 1.016 | 60 → 60 |
| cl100k_base | Go | 0.640 → **0.496** | 0.640 → 0.496 | 0 → 0 |
| o200k_base | Python | 1.417 → **1.058** | 1.347 → 1.042 | 60 → 60 |
| o200k_base | JavaScript | 1.270 → **0.953** | 1.233 → 0.953 | 60 → **22** |
| o200k_base | Go | 0.636 → **0.492** | 0.636 → 0.492 | 0 → 0 |

The 22 tasks still larger than JavaScript on `o200k_base` are exactly the
conditional tasks (`t02`, `t11`–`t20`, `a02`, `a11`–`a20`). No raw ties occur.

Category means (identical across encodings for HowlFrame and Python except where shown cl100k/o200k):

| Category | n | HowlFrame before | HowlFrame after | Python | JavaScript | Go |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| conditional (release iff all PASS and newer version) | 22 | 413.18 | **344.55** | 196.00 | 246.00/254.00 | 478.00/479.00 |
| write record (non-conditional) | 16 | 145.00/146.00 | **108.00/109.00** | 102.00/103.00 | 106.00/115.00 | 229.00/231.00 |
| request approval | 22 | 160.00/159.00 | **124.00/123.00** | 119.00/118.00 | 122.00/129.00 | 250.00/250.00 |

Comment/blank-line stripping changes no HowlFrame, Python or JavaScript
count; Go drops 6 tokens per file (build tag), giving HowlFrame/Go
0.508 (`cl100k_base`) and 0.504 (`o200k_base`).

## What changed in the HowlFrame references

All 60 files are produced by `howl_src` in `author_solutions.py` and use:

- `read_text` instead of `(bytes_to_string (read_file …))`;
- `lines` for the version KV file in conditional tasks;
- root `(do …)` instead of `cli_app`;
- multi-binding `let` instead of one nested `let` per binding;
- `when` for single-branch filters;
- `int` bindings (`c0…p2`) so the strict x.y.z comparison no longer repeats
  `(to_int (list_get …))` for every comparison.

Three constraints surfaced while authoring (both documented, neither changed):

- Under the governed profile used by `hfexp arm-a`,
  `(parse_json Checks (read_text …))` is rejected with
  `PROFILE_FORBIDDEN_CONSTRUCT` (the profile scan runs before desugar), so the
  JSON text still needs its own `raw` binding.
- `list_get` requires a symbol list operand, so the `=`-split and `.`-split
  intermediates (`cp`, `pp`, `cur`, `pro`) must be bound.
- A multi-form `let` body still needs an explicit `(do …)`.


## Per-task delta (HowlFrame raw tokens, cl100k_base/o200k_base)

| Task | Category | HF before cl/o | HF after cl/o | Δ cl/o | Python cl/o | JS cl/o |
|---|---|---:|---:|---:|---:|---:|
| t01 | write | 145/146 | 108/109 | -37/-37 | 102/103 | 106/115 |
| t02 | conditional | 385/385 | 320/320 | -65/-65 | 196/196 | 246/254 |
| t03 | approval | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| t04 | write | 145/146 | 108/109 | -37/-37 | 102/103 | 106/115 |
| t05 | write | 145/146 | 108/109 | -37/-37 | 102/103 | 106/115 |
| t06 | write | 145/146 | 108/109 | -37/-37 | 102/103 | 106/115 |
| t07 | write | 145/146 | 108/109 | -37/-37 | 102/103 | 106/115 |
| t08 | write | 145/146 | 108/109 | -37/-37 | 102/103 | 106/115 |
| t09 | write | 145/146 | 108/109 | -37/-37 | 102/103 | 106/115 |
| t10 | write | 145/146 | 108/109 | -37/-37 | 102/103 | 106/115 |
| t11 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| t12 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| t13 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| t14 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| t15 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| t16 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| t17 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| t18 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| t19 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| t20 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| t21 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| t22 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| t23 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| t24 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| t25 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| t26 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| t27 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| t28 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| t29 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| t30 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| a01 | write | 145/146 | 108/109 | -37/-37 | 102/103 | 106/115 |
| a02 | conditional | 385/385 | 320/320 | -65/-65 | 196/196 | 246/254 |
| a03 | approval | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| a04 | write | 145/146 | 108/109 | -37/-37 | 102/103 | 106/115 |
| a05 | write | 145/146 | 108/109 | -37/-37 | 102/103 | 106/115 |
| a06 | write | 145/146 | 108/109 | -37/-37 | 102/103 | 106/115 |
| a07 | write | 145/146 | 108/109 | -37/-37 | 102/103 | 106/115 |
| a08 | write | 145/146 | 108/109 | -37/-37 | 102/103 | 106/115 |
| a09 | write | 145/146 | 108/109 | -37/-37 | 102/103 | 106/115 |
| a10 | write | 145/146 | 108/109 | -37/-37 | 102/103 | 106/115 |
| a11 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| a12 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| a13 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| a14 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| a15 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| a16 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| a17 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| a18 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| a19 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| a20 | conditional (unit) | 416/416 | 347/347 | -69/-69 | 196/196 | 246/254 |
| a21 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| a22 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| a23 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| a24 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| a25 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| a26 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| a27 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| a28 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| a29 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |
| a30 | approval (unit) | 160/159 | 124/123 | -36/-36 | 119/118 | 122/129 |

## Remaining hot spots (by category)

Source observations from the re-authored files, not an additive BPE attribution:

1. **Conditional semver comparison (22 tasks, ≈150 tokens over Python).** The
   KV split, four intermediate splits, six `to_int`/`list_get` bindings and the
   nested lexicographic `or`/`and` take about 189 `cl100k_base` tokens in `t02`
   alone. Python does it with `dict(...)`, `tuple(map(int, …))` and a tuple
   `>`. A version-compare or KV-lookup builtin would hit the largest remaining gap.
2. **Unit-conditional failure collection (20 of 22 conditional tasks, +27).**
   The `failed` list and second `for`/`when` loop (347 vs 320 for the loop
   variant) stand in for Python's `all(...)`. An `all`/`every` or
   `filter`/`count` form would remove it.
3. **Binding and envelope overhead on every task (≈6–7 tokens over Python on
   write/approval tasks).** The governed-profile `raw` binding for
   `parse_json`, the explicit `(do …)` in `let` bodies, the
   `(dict ("k" v) …)` pair syntax and the trailing run of closing parentheses.
   Allowing typed `parse_json` over an inline `read_text` under governed, and
   implicit-`do` `let` bodies, are the cheap wins.

## Method and reproduction

```sh
experiments/token-efficiency-v0/run.sh
```

Dependencies: Go, Node.js and a Python 3 with `requirements.txt` installed
(`tiktoken==0.8.0`; set `TOKBENCH_PYTHON` to choose the interpreter), plus
cached `cl100k_base`/`o200k_base` encodings. The counter disables the
uncached tokenizer loader, so a missing or corrupt cache fails rather than
downloading. Measured with Go 1.24.4 linux/amd64, Python 3.13.5 and Node
v20.19.2 on main `fd1c6225`, 2026-10-12.

`author_solutions.py` regenerates all four languages from task metadata
without reading oracle contents (run it, then `gofmt -w solutions/go`). The
corpus, `tokverify`, materializer and counting rules are unchanged from #133:
HowlFrame goes through `hfexp arm-a` (governed check/build, capability
inspection, VM execution, receipt audit, oracle comparison). Python/JS/Go run
on the same materialized evidence with snapshot diffs and `CompareOracle`.
480 rows are in [results/tokens.csv](results/tokens.csv) /
[results/tokens.json](results/tokens.json); per-task outcomes, SHA-256 hashes
and comparisons are in [results/verification.json](results/verification.json);
ratios and task lists are in [results/summary.json](results/summary.json).

Pass counts: **HowlFrame 60/60, Python 60/60, JavaScript 60/60, Go 60/60.**

## Documented limitations

- **JS/Go comment stripper:** the secondary "no comments" scanner handles
  quoted strings and `//`/`/* */` comments only. It does **not** understand
  JavaScript regex literals or template-literal `${…}` nesting. The current
  references contain neither, so counts are unaffected. Adding such sources
  would need a real lexer.
- **No verifier host isolation for Python/JS/Go:** native references run as
  ordinary host processes with a minimal environment and a timeout. The
  verifier compares the sandbox and a sibling canary tree after execution,
  but it does not confine or observe other host reads/writes, network or
  process spawns. These are trusted references, not an isolation or
  injection-resistance experiment. Only HowlFrame runs under VM capability
  enforcement and receipts.

## Threats to validity

- **Author and style bias:** one author/template process chose naming,
  formatting, error handling and language idioms; the six HowlFrame
  references formerly copied from `reference/arm_a` were re-authored in
  current shorthand for this rerun (the arm_a files themselves are unchanged). No independent expert style review or multiple-author
  distribution was measured. These are plain references, not minimum-token
  solutions; other reasonable implementations could alter the ranking.
- **Synthetic corpus and dependence:** the 60 tasks repeat three workflows,
  30 of them paired note injections. The pairs have identical source and
  token counts; 60 is not 60 independent algorithms. No broad language or
  real-world corpus claim follows from this measurement.
- **Tokenizer choice:** two fixed BPE encodings are source-token proxies;
  neither measures a specific live model's generation cost, reasoning,
  context reuse, prompt tokens, retries, latency or price.
- **Go boilerplate:** standalone imports, main, JSON error handling and file
  helpers inflate Go relative to interpreted scripts. Build-tag overhead is
  included and separately stripped. Shared application scaffolding could
  amortize some of this cost; it is deliberately excluded here.
- **Capability declarations and enforcement:** source counts omit external
  grants and policies. Native programs lack VM capability receipts; their
  verifier checks two trees after execution, not all host reads/writes,
  network or processes. These are trusted references, not an isolation or
  injection-resistance experiment. The broad filesystem grant and audit
  limitations of the original protocol still apply.
- **Input scope:** each task has fixed valid checks/KV evidence. Version
  comparisons are numeric on the supplied strict x.y.z versions; these
  programs are not claimed to be general malformed-semver validators.
  Successful corpus replay establishes only the recorded oracle behavior.
- **Oracle access during authoring:** the author can see frozen expected
  outputs; the template generator uses descriptions and metadata, not oracle
  contents, and execution never receives oracles. This is not held-out model
  evaluation, independently authored tasks, or a blind correctness study.


HowlFrame still uses more source tokens than Python in this corpus, now by
about 6% at the median (per-task ratio 1.059) rather than 42%. Its JavaScript parity depends on the
encoding and does not hold for the conditional tasks. H1 and issue #90 are
unchanged.

## Repository validation status

Green locally on the box (not in a restricted sandbox), with
`GOCACHE=/tmp/hf-token-go-cache`:

- `go build ./... && go vet ./... && go test ./... -timeout=300s` — exit 0
  (all packages, including the socket/HTTP and process-grant tests that
  failed only inside the #133 authoring sandbox);
- `go test -count=1 ./experiments/...` (falsifiable-v0 harness, hfexp,
  token-efficiency coverage test) — pass;
- `experiments/token-efficiency-v0/run.sh` — exit 0, 240/240 references pass;
- `gofmt -l experiments` clean. `git diff --check` flags only the CRLF line
  endings Python's `csv` writer has always emitted in `results/tokens.csv`.

This replaces the stale #133 section, which reported sandbox-only socket
failures and "changes left uncommitted".

See the [journal](../../docs/journals/2026-10-12_token_benchmark_rerun.md).
