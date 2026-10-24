# HowlFrame and HowlBoard: current verification

Run 2026-10-23 to 2026-10-24 by Claude Opus 5.5 (`claude-opus-5-5`, Claude Code 2.1.309). The runs used disposable fresh `git clone`s
from GitHub in a session scratch directory. The developer checkouts under `dev/` were only read (`git status`, `git fetch`), never
modified. Logs are in `evidence/logs/`, with the scratch path replaced by `$SCRATCH`.

## 1. Baseline (Phase 1)

| | HowlFrame | HowlBoard |
|---|---|---|
| Default branch HEAD | `056fbf89e7477637e3e96acd93f3d14870dbc3fc` (merge of #221) | `0b9236bc637fee00e4c400a3304230e42f0c7c49` (merge of #21) |
| Same as the historical reference? | **Yes**: no commits since | **Yes**: no commits since |
| Developer checkout | `main`, clean, equal to `origin/main` | `main`, clean, equal to `origin/main` |
| Branch protection (live API) | Required `build`, `acceptance`, `race`; `strict: true`; admins not enforced | Required `Build, Contract Tests, Docs & SEO Verification`; `strict: true`; admins not enforced |
| CI on HEAD | CI run [39990808398](https://github.com/howlcipher/howlframe/actions/runs/39990808398): `build`, `acceptance`, `race`, `latest-go` all **success** (2026-10-23 16:57–16:59Z). The Pages `build`/`deploy` jobs also pass | CI run [39807668367](https://github.com/howlcipher/howlboard/actions/runs/39807668367): **success** (2026-10-22 15:46Z) |
| Minimum Go | `go.mod`: `go 1.25`; CI `build`/`acceptance`/`race` use 1.25; `latest-go` uses stable (Owner decision A) | CI pins Go `1.26.0` and builds HowlFrame at **`a154ddac`** (see finding F1) |

### Merged status of the named PRs (live `gh` API)

| PR | Title (short) | Merged (UTC) | Merge commit |
|---|---|---|---|
| howlframe #209 | Post-audit stabilization | 2026-10-22 00:41 | `bfc4ff8f` |
| howlframe #211 | EvidenceDesk `parse_within` export | 2026-10-22 14:18 | `a154ddac` |
| howlframe #212 | HF-05 checker/HFIR value positions | 2026-10-22 15:32 | `6912b0bb` |
| howlframe #218 | Owner decisions A–E, H step 1, pilot options | 2026-10-22 15:55 | `d83204d3` |
| howlframe #219 | `process:env` execution-steering families (#182 step 1) | 2026-10-22 21:38 | `951c5a85` |
| howlframe #220 | EvidenceDesk runs current HowlBoard | 2026-10-23 16:50 | `2a75a611` |
| howlframe #221 | Env families found reviewing #219 | 2026-10-23 16:55 | `056fbf89` |
| howlboard #20 | Import validates with the EvidenceDesk ledger module | 2026-10-22 14:18 | `24c59173` |
| howlboard #21 | Explicit authorization for Board mutations | 2026-10-22 15:44 | `0b9236bc` |

**Open PRs:** HowlFrame #213 (draft, competitive-v1, head `1116c48b`, 28 commits behind `main`), #206 (draft, M3 spec freeze) and #178
(draft, live-v1 harness). HowlBoard: none.
**Open issues relevant here:** #216 (hostile code that kills its supervisor), #217 (macOS/BSD/Windows descendant containment), #182 (env
allowlist, step 2 deferred), #194 (SIGSTOP self-test flake), #147 (parallel-run flake), #175 (runner-owned adapter deadline before live runs).
**Product directive:** `HOWLFRAME_PRODUCT_DIRECTIVE.md` is byte-identical between the repository and `dev/`. D14–D16 are later competitive
targets, explicitly "not Milestone 1 gates"; D30 A/B/J covers Milestone 3.
**Independent-review records present:** `docs/adoption/evidence/reviews/` (HF-05 Antigravity rounds 2–4 and Codex probe matrices;
HowlBoard #20 Codex rounds 1–2); `docs/remediation/evidence/review/` (#209: Codex R1 REJECT, Antigravity R2 ACCEPT);
`milestone3/evidencedesk/INDEPENDENT_REVIEW.md`; and on #213, `FAIRNESS_REVIEW_R1–R3.md`.

## 2. Commands and exit codes (Phase 2)

Toolchain: Go 1.26.0 (local; see limitation L1), Python 3.14.4, Playwright with Chromium 148.0.7778.96 (existing install; nothing
installed), bubblewrap.

| # | Command (in the fresh clone) | Exit | Time | Result |
|---|---|---|---|---|
| 1 | `gofmt -l .` (must be empty) | 0 | 1 s | clean |
| 2 | `go build ./...` | 0 | 4 s | |
| 3 | `go vet ./...` | 0 | 2 s | |
| 4 | `go test -count=1 ./...` | 0 | 212 s | all packages ok |
| 5 | `go test -race -count=1 ./...` | 0 | 225 s | all packages ok, no races |
| 6 | `HF_BIN=<fresh build> scripts/ci_acceptance.sh` | 0 | 145 s | "all accepted-application suites passed" |
| 7 | `HOWLFRAME_BIN=<fresh> HOWLBOARD_REPO=<fresh hb@0b9236b> bash milestone3/evidencedesk/scripts/test.sh` | 0 | 5 s | "PASS all implementation workflows" |
| 8 | Same verifier against a disposable HowlBoard at `cd2f2fd` | 0 | — | adapter path, PASS |
| 9 | HowlBoard `make test` with the fresh compiler (current HowlFrame `056fbf89`) | 0 | — | `ok howlboard/backend` (23 tests). First attempt exit 2: my error, a relative `HOWLFRAME_BIN` resolved from `backend/` |
| 10 | HowlBoard `make import LEDGER=<first 500 lines of sanitized sample> MAX=500` (the CI step) | 0 | — | `entries=72 missions=16` |
| 11 | HowlBoard `make import` (default sanitized fixture) | 0 | — | `entries=72 missions=16` |
| 12 | HowlBoard `make import` with a malformed line 4 | 2 (make; tool exit 3) | — | `invalid_ledger at line 4: Malformed JSON; nothing was imported` |
| 13 | HowlBoard `make import` with a duplicate `entry_id` | 2 (make; tool exit 3) | — | `invalid_ledger at line 3: Duplicate entry_id; nothing was imported`; mission store byte-identical before and after |
| 14 | `scripts/test_docs.py`, `test_seo.py`, `test_timeline_layout.py` (Chromium) | 0, 0, 0 | — | |
| 15 | Targeted security tests, `-v` (§4) | 0 | — | 0 failures |
| 16 | `restart_default_budget.py` standalone | 0 | 33 s | 3/3. First attempt exit 1: my error, `HF_BIN` unset, so the launcher used the stale `~/.local/bin/howlframe` 0.1.0 (finding F3) |

Neither fresh clone was left dirty by the builds (`git status --porcelain` empty), so the committed `static/app.js` and HowlBoard
generated files reproduce byte-for-byte from source.

### Acceptance detail (`ci_acceptance.sh`)

| Suite | Result |
|---|---|
| M1 LedgerLens / HealthWatch / TaskFlow frozen acceptance | 35/35, 29/29, 32/32 |
| M1 retained regressions (fresh boundaries, HealthWatch supplemental, LedgerLens rules) | pass |
| M2 TicketDesk frozen acceptance | **101/101** |
| M2 developer-experience checks | 13/13 |
| M2 reassessment/adversarial; review round-1 regressions | 11/11; 10 tests, 0 failed |
| M2 Assurance m10 | 5 tests, 0 failed, 1 *recorded* expected failure (allowed by the script, as historically recorded) |
| M2 restart at default ceilings (1,000 tickets, 100 tokens), SIGKILL, invalid state | 3/3 |

## 3. EvidenceDesk (M3) and HowlBoard

**Observed (`evidence/logs/evidencedesk.log`):**
* The 6 independently authored frozen API/projector tests pass (`Ran 6 tests ... OK`).
* Restart persistence, encoded ID, Unicode notes and physical-line diagnostics pass.
* **Real Chromium 148** passes import, header, filter, ordered claims, notes, reload, errors, **XSS** and keyboard.
* **Capability denial:** network, environment, database, `filesystem:read` and `filesystem:write` are each denied. A static read is
  denied without corrupting persisted state. A too-small instruction limit is handled, and process and remote-fetch grants are absent
  throughout.
* **Current HowlBoard consumer:** the Board at `0b9236b` declares its own root grants. It is **built unmodified** (source SHA-256
  `b0a5c896…`) and run under exactly its pinned declared grants, with no host environment beyond `PATH`. It serves both sanitized
  fixtures: review.jsonl (3 records, 2 tasks) and ledger_sample.jsonl (72 records, 16 tasks).
* **Historical adapter limited to historical consumers.** In `verify_workflows.py` the adapter is applied only when the Board declares no
  root `(grants …)`, and a declared form must equal the pinned declaration or the run fails. Run 8 confirms the adapter path is taken
  only for `cd2f2fd` (adapter insertion SHA-256 `7c4a4e00…`).

**The importer genuinely uses the EvidenceDesk validator (source inspected, not just file presence):**
* `tools/ledger_import/ledger_import.howl:29` has `(use "evidence_ledger.howl" as ledger)`, and `:119` calls `ledger/parse_within … 3145728`.
* The Makefile's `build` compiles `tools/ledger_import/ledger_import.howl`, and `make import` runs that artifact.
* The vendored `evidence_ledger.howl` is **byte-identical** to HowlFrame `main`'s `milestone3/evidencedesk/app/ledger.howl`
  (SHA-256 `aacde250…2d55` for both).
* Behavioral proof (runs 12–13): the refusal messages come from the vendored module's own `failure` calls (`evidence_ledger.howl:51`,
  `:87`), and the store is unchanged after a refused import.
* HowlBoard's `ledger_import_validation_test.go` (6 tests, including `TestLedgerImportServedByBoard`) passes with the **current** compiler.

Live private HowlPlane ledgers were not read. Only `data/fixtures/ledger_sample.jsonl` (SHA-256 `439ca83f…`) and EvidenceDesk's committed
`tests/fixtures/review.jsonl` were used. The Board ran with its working directory in a temporary directory, so its relative store
paths could not reach live data. The verifier exercises only `/api/missions*`, and the Factory status route (which can fetch from GitHub) was
not called.

## 4. Security regression confirmation

All bounded, on disposable temporary files, through the project's own tests. Nothing hostile was run against the host.

| Area | Tests run (all PASS) | Log |
|---|---|---|
| Filesystem-scoping races | `TestScopedRead/Write/MkdirNeverEscapesWhileSymlinkChanges`, `TestFileStoreNeverEscapesWhileSymlinkChanges`, `TestFileRootGrantNeverEscapesWhileSymlinkChanges`, `TestRaceHarnessReadsInsideRoot`, `TestAbsoluteInRootSymlinkFailsClosed` (+ file-store scope tests; 31 passes) | `sec_fs.log` |
| Single-file root: symlink and sibling | `TestFileRootGrantReadsAndWritesExactlyThatFile`, `…DoesNotFollowSymlinkOutOfItsDirectory`, `…RefusesRelativeSymlinkToSibling`, `…WriteDoesNotTruncateBeforeVerifying`, `TestRootEscapeErrorIsRecognized`, `TestOpenScopeMissingRootIsIOErrorNotDenial`, `TestScopedMkdirCreatesMissingGrantedRoot` (7) | `sec_scopedfs.log` |
| Process-descendant termination | `TestExecKillsBackgroundAndSessionEscapedDescendantsOnNormalExit`, `TestExecDeadlineKillsEveryDescendant`, `TestExecOutputLimitKillsEveryDescendant` (+6 exec tests) | `sec_exec.log` |
| TicketDesk default-budget restart | `restart_default_budget.py` 3/3 (inside acceptance and standalone) | `acceptance.log`, `sec_ticketdesk_restart_rerun.log` |
| Slow-client isolation | `TestSerializedServerSurvivesNonReadingClient`, ×3 runs (`-count=3`) | `sec_slow_rerun.log` |
| HF-05 value positions | `TestValuePositionVoidRejectedBeforeEffects` (8 subtests), `TestVoidInValuePositionIsRejected` (checker) and HFIR variants (38 passes) | `sec_hf05.log` |
| Env restrictions (#219, #221) | `TestEnvGrantRefusesCodeLoading`, `…DynamicLoader`, `TestIssue166EnvGrantRefusesLoaderVars`, Go/JS `…EnvRequiresGrantBeforeRead`, `…CodeLoadingEnv`, `…ExecEnvironmentAndFlagSlots`, proxy-env tests (13), all driven by the shared `internal/nettest` refused/allowed lists that #221 extended (`DOTNET_STARTUP_HOOKS`, `COMPlus_*`, `CORECLR_*`, `COMSPEC`, `PATHEXT`, `WGETRC` …) | `sec_env.log` |

All of these also run in the required `build` (`go test ./...`) and `race` jobs, so **CI protects the October 21 remediation on every
merge to `main`**. The TicketDesk restart is in the required `acceptance` job.

**Containment versus isolation.** Linux exec has *runaway-process containment*: a `PR_SET_CHILD_SUBREAPER` supervisor kills every
descendant when an exec returns, including background and `setsid` children. That is **not adversarial isolation**: code that kills its own
supervisor can leave detached descendants (#216). macOS/BSD and Windows have only a weaker process-group kill (#217). There is **no OS
sandbox** (namespaces, seccomp, cgroups). Competitive-v1 must therefore use an external jail for every arm, and the protocol already
says so (§4).

## 5. Findings

| # | Finding | Severity for the next experiment |
|---|---|---|
| F1 | HowlBoard CI builds HowlFrame at `a154ddac` (#211), not current `main`. Its required check therefore does not test #212 (HF-05 checker tightening), #219 or #221 | Not blocking. This run shows HowlBoard passes with `056fbf89`. Recommend bumping the pin (HowlBoard PR, separate approval) |
| F2 | The `docs/adoption/HOWLFRAME_FINAL_HANDOFF.md` table gives #213's head as `28febce3`; the actual head is `1116c48b` (the Owner pilot-options record) | Cosmetic |
| F3 | `howlframe` on this host's PATH is the 2026-10-01 release 0.1.0, which lacks `-max-request-bytes`. Any script that defaults to `howlframe` on PATH silently tests an old compiler | Matters for the experiment: the harness must call the pinned build by absolute path |
| F4 | `milestone3/evidencedesk/tests/reviewer_boundaries.py` hard-codes `/tmp/m1-independent-hf` | Historical reviewer script; not part of `test.sh`; not run |

No blocking regression was found.

## 6. Limitations

* **L1.** The local toolchain is Go 1.26.0, not the 1.25 minimum. Go 1.25 is covered by hosted CI on the same commit (all required jobs
  green). No toolchain was downloaded.
* **L2.** Python is 3.14.4 locally; CI uses the version in the workflow.
* **L3.** HowlBoard was tested on Linux only, and Windows/macOS binaries were not executed (as before).
* **L4.** EvidenceDesk browser tests ran headless.
* **L5.** Passing tests show the specified behavior. They do not show the absence of other defects.

## 7. Assessment

| Milestone | Status at `056fbf89` / `0b9236b` |
|---|---|
| M1 (LedgerLens, HealthWatch, TaskFlow) | **Healthy**: frozen suites and retained regressions pass |
| M2 (TicketDesk) | **Healthy**: 101/101 frozen, DX, adversarial, review and restart regressions pass |
| M3 (EvidenceDesk; D30 A/B/J) | **Healthy**. D30 A is demonstrated again in real Chromium. D30 B remains *partially adopted* (validator, not projector), per the 2026-10-22 correction, and current HowlBoard verifiably uses it |

**Suitable as a frozen competitive baseline?** Yes. HowlFrame `056fbf89` is green locally and on the required CI checks, with no open
blocking defect. Protocol §11.4 still requires pinning a merged commit at freeze and no HowlFrame change until the pilot ends. If `main`
moves before freeze, re-run this verification on the new commit.
