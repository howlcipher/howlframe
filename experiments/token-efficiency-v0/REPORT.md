# Token-efficiency benchmark v0 — compact conditionals (measure only)

This is the third measurement of this corpus:

- **#133 (`fd1c6225`):** the original references. Their HowlFrame side predates the density sugar.
- **#134 (`f930cf6e`):** HowlFrame re-authored with the sugar from #117–#130.
  - The sugar it actually used: root `do` (#117), `read_text`/`lines` (#118),
    multi-binding `let` (#120) and `when` (#124).
  - Earlier text said "#124–#130" or "#128–#130"; both ranges were wrong, and #128–#130 sugar is not used here.
- **This slice:** compact-conditional built-ins, described below.

Python, JavaScript and Go references are **byte-identical** across all three.
The tasks, verifier, `run.sh`/`count_tokens.py` and tokenizers
(`tiktoken==0.8.0`, `cl100k_base`, `o200k_base`) are unchanged.

Headline:

- **Medians:** HowlFrame 124/123 → **117/116** (`cl100k_base`/`o200k_base`),
  against Python 119/118 and JavaScript 122/129.
- **Conditional tasks:** the mean fell from 344.55 to **186.00**, against Python's 196.00.
- **Per task:** HowlFrame uses fewer tokens than Python on 44 tasks and ties on 16
  (the write-record tasks), on both encodings. It uses fewer than JavaScript on all 60.

This is a source-size measurement of authored, verified references. It
measures neither model success nor generation/repair cost. No live LLM/API
calls were made, no H1 PASS/KILL decision is claimed, and issue #90 remains
Partial. Production `-compile-bc`/HFIR defaults are unchanged.

## Language changes in this slice

The four additions all expand in `ast.Desugar` into constructs that already
lower (`let`, `for`, `if`, `set`, `str_split`, `to_int`, `list_get`,
comparisons). They add no opcode, capability or effect.

1. **`(ver_gt a b)` / `(ver_ge a b)`:** strict numeric x.y.z comparison of two
   version strings, lexicographic over major/minor/patch (`1.10.0 > 1.9.9`).
   The inputs must have three numeric parts. Fewer parts fail at runtime on
   `list_get`; they are not accepted silently.
2. **`(all x list pred)` / `(any x list pred)`:** bind each element to `x`
   and test `pred`. `all` over an empty list is true; `any` is false.
3. **`list_get` with any expression operand.** A non-symbol list is bound to a
   fresh temporary first, because lowering reads a variable name.
4. **Inline `parse_json` under governed.** `(parse_json T <expr>)` binds the
   operand to a temporary in desugar.
   - The governed profile scan runs **before** desugar. It now descends into
     the operand instead of rejecting the shape outright.
   - Forbidden constructs inside the operand are still rejected, and the
     post-desugar scans and the construct gate run unchanged.
   - The default `construct.Scan` stays strict for undesugared trees.

Guards (tests):

- **Purity, unit test:** `internal/ast/desugar_compact_test.go` checks that every
  expansion uses only a closed set of pure heads, and that no sugar head survives desugar.
- **Purity, end to end:** `TestCompactConditionalBuiltinsArePureAndCorrect`
  builds a program under governed that uses every new form, then checks two things:
  - `inspect --caps` reports **no** capabilities;
  - running it gives the expected truth table, including empty lists,
    equal versions, `1.10.0` vs `1.9.9`, `10.0.0`, nesting, and inline `list_get`.
- **Fail-closed ordering:** `TestGovernedInlineParseJSONFailsClosed`
  hides `exec`, `llm_generate` or `spawn` inside an inline `parse_json` operand,
  preceded by a `write_file` side effect.
  - Governed `check` and `build` must both fail with
    `PROFILE_FORBIDDEN_CONSTRUCT` naming the construct.
  - No artifact, side file or exec output may exist afterwards.
  - The allowed shape (`read_text` operand) still builds.
- **Registry and drift:** `all`, `any`, `ver_gt` and `ver_ge` are registered as
  `CompileTimeOnly` and listed as consumed before lowering.
- **Results integrity:** `verification_test.go` checks that `results/verification.json`
  covers 60 × 4 entries, all passing, each with a matching SHA-256.

## Before vs after

Before is #134 on `f930cf6e`; after is this branch. Raw source tokens:

| Encoding | Language | Median before | Median after | Mean before | Mean after |
| --- | --- | ---: | ---: | ---: | ---: |
| cl100k_base | HowlFrame | 124 | **117** | 200.60 | **138.30** |
| cl100k_base | Python | 119 | 119 | 142.70 | 142.70 |
| cl100k_base | JavaScript | 122 | 122 | 163.20 | 163.20 |
| cl100k_base | Go | 250 | 250 | 328.00 | 328.00 |
| o200k_base | HowlFrame | 123 | **116** | 200.50 | **138.20** |
| o200k_base | Python | 118 | 118 | 142.60 | 142.60 |
| o200k_base | JavaScript | 129 | 129 | 171.10 | 171.10 |
| o200k_base | Go | 250 | 250 | 328.90 | 328.90 |

HowlFrame / comparison ratios (>1 means HowlFrame uses more tokens):

| Encoding | vs | Median per-task ratio before → after | Ratio of medians before → after | Tasks HowlFrame larger before → after |
| --- | --- | ---: | ---: | ---: |
| cl100k_base | Python | 1.059 → **0.983** | 1.042 → 0.983 | 60 → **0** (16 ties) |
| cl100k_base | JavaScript | 1.019 → **0.959** | 1.016 → 0.959 | 60 → **0** |
| cl100k_base | Go | 0.496 → **0.445** | 0.496 → 0.468 | 0 → 0 |
| o200k_base | Python | 1.058 → **0.983** | 1.042 → 0.983 | 60 → **0** (16 ties) |
| o200k_base | JavaScript | 0.953 → **0.896** | 0.953 → 0.899 | 22 → **0** |
| o200k_base | Go | 0.492 → **0.446** | 0.492 → 0.464 | 0 → 0 |

Cumulative from the #133 baseline: the HowlFrame median went 160/159 → 117/116.
The per-task ratio went 1.42x → 0.98x against Python and 1.37x/1.27x → 0.96x/0.90x against JavaScript.

Category means. HowlFrame and Python are identical across encodings unless shown as cl100k/o200k:

| Category | n | HowlFrame #133 | HowlFrame #134 | HowlFrame now | Python | JavaScript | Go |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| conditional | 22 | 413.18 | 344.55 | **186.00** | 196.00 | 246.00/254.00 | 478.00/479.00 |
| write record | 16 | 145.00/146.00 | 108.00/109.00 | **102.00/103.00** | 102.00/103.00 | 106.00/115.00 | 229.00/231.00 |
| request approval | 22 | 160.00/159.00 | 124.00/123.00 | **117.00/116.00** | 119.00/118.00 | 122.00/129.00 | 250.00/250.00 |

Changes per category:

- **Conditional tasks** dropped 134 tokens (loop variant) or 161 tokens (unit variant).
  `all` replaced the second failure-collection loop and the list-length test,
  and `ver_gt` with inline `list_get` replaced eight bindings and a nested comparison.
- **Write-record and approval tasks** saved 6–7 tokens each from the inline `parse_json`, which drops the `raw` binding and one `let`.

Comment/blank-line stripping changes only Go, which drops 6 tokens per file
for the build tag. HowlFrame/Go then becomes 0.457/0.458 per task.

## Per-task delta (HowlFrame raw tokens, #134 → now, cl100k_base/o200k_base)

| Task | Category | HF before cl/o | HF after cl/o | Δ cl/o | Python cl/o | JS cl/o |
|---|---|---:|---:|---:|---:|---:|
| t01 | write | 108/109 | 102/103 | -6/-6 | 102/103 | 106/115 |
| t02 | conditional | 320/320 | 186/186 | -134/-134 | 196/196 | 246/254 |
| t03 | approval | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| t04 | write | 108/109 | 102/103 | -6/-6 | 102/103 | 106/115 |
| t05 | write | 108/109 | 102/103 | -6/-6 | 102/103 | 106/115 |
| t06 | write | 108/109 | 102/103 | -6/-6 | 102/103 | 106/115 |
| t07 | write | 108/109 | 102/103 | -6/-6 | 102/103 | 106/115 |
| t08 | write | 108/109 | 102/103 | -6/-6 | 102/103 | 106/115 |
| t09 | write | 108/109 | 102/103 | -6/-6 | 102/103 | 106/115 |
| t10 | write | 108/109 | 102/103 | -6/-6 | 102/103 | 106/115 |
| t11 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| t12 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| t13 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| t14 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| t15 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| t16 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| t17 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| t18 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| t19 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| t20 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| t21 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| t22 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| t23 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| t24 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| t25 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| t26 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| t27 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| t28 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| t29 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| t30 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| a01 | write | 108/109 | 102/103 | -6/-6 | 102/103 | 106/115 |
| a02 | conditional | 320/320 | 186/186 | -134/-134 | 196/196 | 246/254 |
| a03 | approval | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| a04 | write | 108/109 | 102/103 | -6/-6 | 102/103 | 106/115 |
| a05 | write | 108/109 | 102/103 | -6/-6 | 102/103 | 106/115 |
| a06 | write | 108/109 | 102/103 | -6/-6 | 102/103 | 106/115 |
| a07 | write | 108/109 | 102/103 | -6/-6 | 102/103 | 106/115 |
| a08 | write | 108/109 | 102/103 | -6/-6 | 102/103 | 106/115 |
| a09 | write | 108/109 | 102/103 | -6/-6 | 102/103 | 106/115 |
| a10 | write | 108/109 | 102/103 | -6/-6 | 102/103 | 106/115 |
| a11 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| a12 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| a13 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| a14 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| a15 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| a16 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| a17 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| a18 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| a19 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| a20 | conditional (unit) | 347/347 | 186/186 | -161/-161 | 196/196 | 246/254 |
| a21 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| a22 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| a23 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| a24 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| a25 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| a26 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| a27 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| a28 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| a29 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |
| a30 | approval (unit) | 124/123 | 117/116 | -7/-7 | 119/118 | 122/129 |

## Remaining hot spots (by category)

These are source observations measured with `cl100k_base` on fragments. They are not an additive attribution.

1. **Version KV extraction (conditional, 22 tasks).** The two
   `(list_get (str_split (list_get lines N) "=") 1)` operands cost 35 of the
   40 tokens in the version test; `ver_gt` itself is cheap. A pure
   `kv_get`/`kv` lookup over `lines` would roughly halve this.
2. **Filter-into-list loop (all 60 tasks).** `selected (list)` plus
   `(for c checks (when (= (map_get c "status") "PASS") (append selected c)))`
   is about 27 tokens, against Python's 17-token comprehension. This is why the 16
   write-record tasks only tie Python. A pure `filter`
   (`(filter c checks pred)`) is the obvious next form.
3. **Record and literal envelopes (all tasks).** `(dict ("batch" "02") ("selected" selected))`
   costs 12 tokens. The blocked approval payload, written as an escaped JSON
   string literal, costs 20 in conditional tasks. The four-binding header
   (51 tokens) includes the `version`/`note` reads that every language
   performs. Flatter `dict` pairs and a JSON-literal form are candidates.

## Method and reproduction

```sh
experiments/token-efficiency-v0/run.sh
```

Dependencies: Go, Node.js and a Python 3 with `requirements.txt` installed
(`tiktoken==0.8.0`; set `TOKBENCH_PYTHON` to choose the interpreter), plus
cached `cl100k_base`/`o200k_base` encodings. The counter disables the
uncached tokenizer loader, so a missing or corrupt cache fails rather than
downloading. Measured with Go 1.24.4 linux/amd64, Python 3.13.5 and Node
v20.19.2 on main `f930cf6e` plus this branch, 2026-10-12.

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
- **Benchmark-shaped features:** `ver_gt`/`ver_ge`, `all`/`any`, inline
  `list_get` and inline `parse_json` were added because #134 found these
  shapes dominated this corpus's conditional tasks. They are general
  language features with their own tests, but a benchmark that motivated a
  feature overstates how much it generalizes. A fresh corpus or a
  held-out task set is needed before treating these numbers as a property of
  the language.
- **Input scope:** each task has fixed valid checks/KV evidence. Version
  comparisons are numeric on the supplied strict x.y.z versions; these
  programs are not claimed to be general malformed-semver validators.
  Successful corpus replay establishes only the recorded oracle behavior.
- **Oracle access during authoring:** the author can see frozen expected
  outputs; the template generator uses descriptions and metadata, not oracle
  contents, and execution never receives oracles. This is not held-out model
  evaluation, independently authored tasks, or a blind correctness study.


HowlFrame now uses no more source tokens than Python or JavaScript on any
task in this corpus. That is a claim about these 60 synthetic references
only. The compact built-ins were designed after studying this corpus's hot
spots (see "Benchmark-shaped features" above), and the result says nothing
about model success. H1 and issue #90 are unchanged.

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

Validation tests added in this slice:

- `TestVerificationResultsCoverAndMatchSources` checks that `results/verification.json`
  has exactly 60 × 4 entries, all passing, and that every SHA-256 matches the
  committed source.
- `TestCompactConditionalBuiltinsArePureAndCorrect` and
  `TestGovernedInlineParseJSONFailsClosed` are at the repo root.
- `internal/ast/desugar_compact_test.go` holds the unit tests for the new built-ins.

See the journals: [rerun](../../docs/journals/2026-10-12_token_benchmark_rerun.md),
[compact conditionals](../../docs/journals/2026-10-12_compact_conditionals.md).
