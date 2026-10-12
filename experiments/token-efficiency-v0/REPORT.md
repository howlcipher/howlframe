# Token-efficiency benchmark v0 — measure only

HowlFrame uses more tokens than Python and JavaScript on **every one of the
60 tasks**, with both tokenizers. It uses fewer than standalone Go on every
task. This is a source-size measurement of authored, verified references;
it measures neither model success nor generation/repair cost. No live LLM/API
calls occurred, no H1 decision is claimed, and issue #90 remains Partial.

## Method and reproduction

Run from any directory:

```sh
experiments/token-efficiency-v0/run.sh
```

Dependencies: Go, Python 3, Node.js, and the preinstalled
a Python with (set `TOKBENCH_PYTHON` to choose it) `requirements.txt` (`tiktoken==0.8.0`)
and cached `cl100k_base`/`o200k_base` encodings. The uncached tokenizer loader
is disabled before constructing encodings: absent or corrupt cache data fails
without downloading. Measurement used Go 1.24.4 linux/amd64, Python 3.13.5,
Node v20.19.2 and the repository based on main `8793d6ed`, on 2026-10-12.

There are 30 synthetic base tasks and 30 paired adversarial variants from
`../falsifiable-v0/tasks/manifest.json`. Each reference reads checks JSON,
version KV text and the untrusted note. It retains complete check records in
input order, selecting PASS records for loop tasks or the unit record for
other tasks. It then writes a release record, requests review, or branches
on all checks passing and the proposed numeric version being strictly newer.
All languages implement those same decisions using their ordinary collections,
loops/comprehensions, JSON and file APIs. No reference reads an oracle or
executes instructions in notes. Python, Node and Go use only their standard
libraries. Every Go file is a standalone `package main`; `//go:build ignore`
prevents 60 mains colliding during repository builds and is ignored by an
explicit `go build <file>`. Its tokens are included in the raw measurement.

`author_solutions.py` records task-template expansion without consulting oracle
contents. Run it followed by `gofmt -w solutions/go` to reproduce sources.
The six existing HowlFrame references are copied verbatim and checked by the
coverage test. The remaining HowlFrame files extend those existing forms;
no language, compiler, checker, VM, HFIR default or production `-compile-bc`
behavior was changed. Syntax was checked against the language tour, README,
construct registry documentation and executable governed builds.

`tokverify` calls the existing **hfexp arm-a** CLI for all HowlFrame tasks using
built `/tmp/howlframe`. These runs include governed check/build, capability
inspection, VM execution, receipt audit and oracle comparison. For the other
languages it calls the same evidence-only materializer through a small exported
experiment helper, snapshots the sandbox and sibling canary, runs the source
(or separately compiled Go binary), and calls `Diff`, `UnauthorizedDiff` and
`CompareOracle` from the existing harness. JSON outputs compare as decoded
values; exact created/modified/deleted effects must match. Compile/runtime
errors, timeouts and unauthorized observed mutations fail the task. Execution
has a five-second timeout; native Go compilation has sixty seconds. Native
programs have a minimal runtime environment and no oracle files in their cwd.

All 240 outcomes, source SHA-256 hashes, diagnostics and comparisons are in
[results/verification.json](results/verification.json). Replay sandboxes,
receipts and binaries are retained under a printed `/tmp/hf-token-verify-*`
directory, outside the repository. Failure outcomes are retained and the
verifier exits nonzero after processing every task. `run.sh` still counts
failed references and propagates failure. The counter rejects incomplete,
duplicate or stale verification coverage instead of attributing a pass to
changed source bytes.

Counted material is UTF-8 **source files only**, without prompts, evidence,
oracles, tooling, build output or runtime receipts. There are 480 rows
(60 tasks × 4 languages × 2 encodings) in
[results/tokens.csv](results/tokens.csv) and
[results/tokens.json](results/tokens.json), with `task`, `lang`, `encoding`,
`tokens`, `tokens_nocomments`, `bytes` and `pass`.
Comments and blank lines are removed for the secondary variant; indentation
and other whitespace remain. Python uses its standard tokenizer; the Go/JS
scanner preserves quoted strings and removes line/block comments, and HowlFrame
uses semicolon comments. This scanner is scoped to these references, which
contain no JavaScript regex/template literals. The stripped variant is a text
measurement, not a second executable program (in particular, it removes Go's
build tag). Ratios and full task lists are also saved in
[results/summary.json](results/summary.json).

## Results

Pass counts: **HowlFrame 60/60, Python 60/60, JavaScript 60/60, Go 60/60**.
All 240 observed mutation/output comparisons passed; none were omitted.

Raw source tokens:

| Encoding | Language | Median | Mean |
| --- | --- | ---: | ---: |
| cl100k_base | HowlFrame | 160 | 248.83 |
| cl100k_base | Python | 119 | 142.70 |
| cl100k_base | JavaScript | 122 | 163.20 |
| cl100k_base | Go | 250 | 328.00 |
| o200k_base | HowlFrame | 159 | 248.73 |
| o200k_base | Python | 118 | 142.60 |
| o200k_base | JavaScript | 129 | 171.10 |
| o200k_base | Go | 250 | 328.90 |

HowlFrame / comparison-language ratios (greater than 1 means more tokens):

| Encoding | Comparison | Median of per-task ratios | Ratio of medians | Tasks with more HowlFrame tokens |
| --- | --- | ---: | ---: | ---: |
| cl100k_base | Python | 1.422 | 1.345 | 60/60 |
| cl100k_base | JavaScript | 1.368 | 1.311 | 60/60 |
| cl100k_base | Go | 0.640 | 0.640 | 0/60 |
| o200k_base | Python | 1.417 | 1.347 | 60/60 |
| o200k_base | JavaScript | 1.270 | 1.233 | 60/60 |
| o200k_base | Go | 0.636 | 0.636 | 0/60 |

The tasks where HowlFrame uses more tokens than Python and JavaScript are
`t01–t30` and `a01–a30` for both encodings and both variants. There are no such
tasks versus Go, and no raw token ties versus any comparison language.

Without comments and blank lines, HowlFrame/Python/JavaScript statistics are
unchanged. Go falls to median **244**, mean **322.00** (`cl100k_base`) and
**322.90** (`o200k_base`): six tokens per file were removed by stripping the
build tag and surrounding blank lines. HowlFrame/Go median per-task and
ratio-of-medians values both become **0.656** and **0.652**, respectively.
The relative ordering and task counts remain unchanged.

## Where HowlFrame tokens go

The 22 conditional references average **413.18** HowlFrame tokens under either
encoding, versus **196.00** Python, **246.00/254.00** JavaScript, and
**478.00/479.00** Go. The 38 other references average **153.68/153.53**
HowlFrame tokens. Thus the overall median hides the much larger cost of
conditional version handling; the mean captures that skew.

Inspection of these sources suggests the main overheads are nested `let`
bindings, explicit `bytes_to_string` around reads, `map_get`/`list_get`, repeated
`str_split` and `to_int`, and the expanded lexicographic semver expression
with repeated indexes and parentheses. Python can compare integer tuples;
JS and Go use a three-component loop. Unit selection in HowlFrame uses a
filter loop; conditional unit workflows also collect failing checks so the
release guard is independent of selection. These are source observations,
not an additive attribution of BPE tokens to individual constructs.

Capabilities are inspected and granted outside these source files by the
existing governed harness. There are **no explicit source capability
declarations** in these references. Consequently capability declarations add
zero measured source tokens here, while runner flags/inspection/receipt bytes
are excluded. A compact AI-native surface might reduce repeated read/conversion,
lookup, binding and numeric-version comparison forms; this benchmark supplies
measurement evidence for investigating those costs. It does not justify
removing authority checks or establish that shorter syntax improves model
correctness. No syntax change is part of this slice.

## Threats to validity

- **Author and style bias:** one author/template process chose naming,
  formatting, error handling and language idioms; HowlFrame inherits six
  older references. No independent expert style review or multiple-author
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

HowlFrame loses to Python and JavaScript on source-token efficiency in this
corpus. Its advantage over standalone Go cannot establish a universal
compactness advantage. H1 and issue #90's remaining work are unchanged.

## Repository validation status

The full `run.sh` replay/count finishes with exit 0 and all 240 references pass.
`go build ./...`, `go vet ./...`, formatting, `git diff --check`, the new Go
coverage test, and `scripts/test_seo.py` pass using
`GOCACHE=/tmp/hf-token-go-cache`. The missing-cache offline guard and quoted
comment-marker stripping were also exercised successfully.

**Repository validation remains incomplete in this execution sandbox.** The
requested `go build ./... && go test ./...` invocation reached passing root
and several application tests but hung in existing server tests and was
interrupted. A full replay with `go test ./... -timeout=60s` returned nonzero:
`apps/status_api` and `apps/task_api` timed out in cleanup; the examples HTTP
server, experiment recording listener, Go/JS fetch fixtures, VM fetch fixture
and difftest listener fail with socket `operation not permitted`. Existing JS
process-grant tests fail with `spawnSync touch EPERM` / `spawnSync printf EPERM`.
The existing v2 Python harness ran eight tests and failed `test_all_references`
on its HowlFrame HTTP endpoint with the same socket denial. An initial v2
invocation also failed on the default read-only Go cache; setting GOCACHE
resolved that setup error, not the socket restriction. No tests were skipped
or weakened and no production changes were made to bypass these restrictions.

The measurement results above are valid reference replay results, but this
slice cannot claim green repository CI. Changes are left uncommitted because
the requested green-before-commit condition cannot be met here. See the
[journal](../../docs/journals/2026-10-12_token_efficiency_benchmark.md) for the
validation commands and findings.
