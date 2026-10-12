# 2026-10-12: Token-efficiency benchmark (measure only)

## Findings and change

- Added `experiments/token-efficiency-v0/` with 60 references in each of
  HowlFrame, Python, JavaScript and Go: 240 plain programs. The six existing
  HowlFrame references are preserved byte for byte. All programs read task
  evidence at execution time, preserve selected records/order, ignore note
  instructions and perform the manifest's release/approval workflow.
- `author_solutions.py` records task-template expansion without reading
  oracles. Python/Node/Go programs use stdlib only. Each Go file is standalone
  `package main`, with a build tag to exclude the colliding mains from package
  builds; explicit-file compilation verifies each one.
- `cmd/tokverify` calls the actual hfexp arm-a CLI for HowlFrame and the
  experiment harness materializer, snapshots, unauthorized-diff audit and
  oracle comparison for native languages. A small exported helper in the
  experiment harness exposes its existing materializer; production code is
  unchanged. Runtime/build/timeout errors fail, all outcomes are retained,
  and any failure yields a nonzero verifier exit after full coverage.
- `count_tokens.py` pins tiktoken 0.8.0, counts cached cl100k_base/o200k_base
  encodings and disables the uncached loader. It measures raw UTF-8 source
  and comments/blank-lines-stripped source. All 480 rows retain task, language,
  encoding, raw/stripped token counts, bytes and pass status. Source hashes
  prevent stale verification attribution; duplicate/incomplete coverage fails.
- `run.sh` builds binaries under /tmp, verifies every reference, counts all
  outcomes including failures and propagates verification failures. Replay
  evidence/receipts/binaries remain under /tmp; only measurement records are
  checked in under the experiment's results directory.

## Measured results

All languages pass **60/60**, 240/240 total, with no unauthorized observed
mutations. The final full benchmark exits 0. HowlFrame loses to Python and
JavaScript on every task for both encodings and both count variants; it uses
fewer tokens than standalone Go on every task.

| Encoding | Language | Raw median | Raw mean |
| --- | --- | ---: | ---: |
| cl100k_base | HowlFrame | 160 | 248.83 |
| cl100k_base | Python | 119 | 142.70 |
| cl100k_base | JavaScript | 122 | 163.20 |
| cl100k_base | Go | 250 | 328.00 |
| o200k_base | HowlFrame | 159 | 248.73 |
| o200k_base | Python | 118 | 142.60 |
| o200k_base | JavaScript | 129 | 171.10 |
| o200k_base | Go | 250 | 328.90 |

Median per-task HowlFrame ratios to Python/JS/Go are **1.422/1.368/0.640**
(cl100k) and **1.417/1.270/0.636** (o200k). Ratios of medians are
**1.345/1.311/0.640** and **1.347/1.233/0.636**. Stripping changes only Go:
median 244, means 322.00/322.90; both types of HowlFrame/Go ratios become
0.656/0.652. Exact values and task lists are in results/summary.json.

The 22 conditional references average 413.18 HowlFrame tokens versus 196
Python tokens with either encoding. Source inspection implicates nested lets,
read conversion, explicit map/list access, string splitting and expanded
numeric version comparison. These observations are hypotheses for future
compact syntax work, not an implemented syntax change or BPE attribution.
Source capability declarations add zero tokens here; grants and audits are
external to the measured source files.

## Verification

Toolchain: Go 1.24.4 linux/amd64 (module and CI Go 1.21), Python 3.13.5,
Node v20.19.2, tiktoken 0.8.0 in a local venv. Branch
`okabe/token-efficiency-benchmark`, starting revision `8793d6ed`.
Go commands use `GOCACHE=/tmp/hf-token-go-cache`.

```sh
experiments/token-efficiency-v0/run.sh
go build ./... && go test ./...
go test ./... -timeout=60s
go vet ./...
go test ./experiments/token-efficiency-v0/... -count=1
gofmt -l .
git diff --check
(cd benchmarks/v2/harness && python3 -m unittest test_harness.py)
python3 scripts/test_seo.py
```

- Final full benchmark, repository build/vet/format, coverage test, diff
  whitespace check and SEO validation pass. Coverage also enforces exact
  reuse of the six old HowlFrame references.
- Initial counting failed because replacing `socket.socket` before SSL import
  broke its subclass definition. Replaced that approach with disabling
  tiktoken's uncached `read_file` loader. Counting and the subsequent **full**
  benchmark rerun both pass. A separate empty-cache probe confirms failure
  without network fallback; quoted comment markers survive stripping in all
  four languages.
- Repository validation is **incomplete**. The requested unbounded full-suite
  invocation hung in existing server cleanup and was interrupted. The bounded
  full replay exits 1; no tests were skipped or weakened:
  - `apps/status_api: TestStatusAPI` and `apps/task_api: TestTaskAPI` time out
    after 60 seconds with cleanup stacks blocked on server exit channels.
  - `examples: TestHTTPServerServeHFBC`,
    `harness: TestArmAFetchDeniedRecordingListener`,
    `gogen: TestGoFetchRequiresGrantBeforeRequest`,
    `javascript: TestJSFetchRequiresGrantBeforeRequest`,
    `vm: TestInterpretFetchDeniedBeforeRequest`, and
    `difftest: TestLoweredHFIRABIConformance` fail on TCP socket creation:
    `operation not permitted`.
  - `javascript: TestJSExecRequiresGrantBeforeSpawn` and
    `TestJSExecGrantedPrintsCapturedOutput` fail with `spawnSync touch EPERM`
    and `spawnSync printf EPERM`.
- The v2 Python harness initially failed due to the default read-only Go cache;
  rerunning with the writable cache runs all eight tests but still fails
  `test_all_references` on `task_03_http_json_endpoint howlframe`. Its server
  logs explicitly report `listen tcp :8080: socket: operation not permitted`.
  Deliberate broken-candidate diagnostics elsewhere in that test log are
  expected test inputs, not additional harness test failures.
- Diagnostic command logs were retained under `/tmp/hf-token-*.log`.
  Generated files left by existing suites are cleaned from the workspace;
  no binaries or replay sandboxes are included in the benchmark sources.
- No commit or push: the requested green-before-commit condition is unmet in
  this restricted sandbox. Completion requires the unmodified full repository
  CI-equivalent validation in an environment that permits its existing local
  listeners and process tests, then the requested commit.

## Honesty and deferrals

This measures authored reference source size only. No live LLM/API calls,
model trials, generation/repair costs, H1 decision or PASS/KILL claim. Issue
#90 stays Partial. No language syntax, compiler, checker, VM, HFIR default or
production `-compile-bc` changes. No compatibility claim beyond the recorded
executable corpus evidence.

REPORT.md documents single-author/style bias, template dependence and synthetic
corpus limits, tokenizer choice, Go boilerplate/error handling, capability
declarations, fixed valid input scope and author access to expected outputs.
Native two-tree mutation checks provide no comprehensive host/network/process
audit and no VM capability receipts; successful trusted references cannot
prove isolation or prompt-injection resistance. Future syntax design and
independent model evaluation remain separate work.

## Follow-up: full suite outside the sandbox

Re-ran unsandboxed on the same tree: `go build ./... && go test ./...` exits 0
(all packages ok), and `TOKBENCH_PYTHON=<venv python> run.sh` reports 60/60
for HowlFrame, Python, JS and Go. The socket/spawn failures above were
sandbox-only (EPERM), not regressions.
