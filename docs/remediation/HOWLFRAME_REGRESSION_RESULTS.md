# Regression results

All results below were produced from tree `3832d83e7a4d8e33648db5c93d9611f4efde8441` (branch
`fix/post-audit-stabilization`). Commits after it change only documents and review evidence (no source, test, test
configuration, CI or policy file), so these results correspond to the final working-tree state of everything that executes.
Raw logs: `docs/remediation/evidence/` (named in each row).

Host: Linux 7.2.4 x86_64, Ubuntu 26.04 userspace, 12 CPUs. Toolchains: go1.26.0 (default) and **go1.25.0 (the `go.mod`
minimum)**; Python 3.14.4; bash. No container runtime, so "clean" means the project's own isolated scripts, not an OS clean room.

## 1. Defect reproduction: new regressions fail on the baseline, pass on the fix

| Finding | Regression test | Baseline `5d2983b` | Fixed tree | Evidence |
|---|---|---|---|---|
| HF-01 filesystem | `internal/vm/filesystem_race_test.go` (read, write, mkdir, file store, file grant) | **FAIL**: 2,063 of 4,000 reads escaped; write, mkdir and store writes landed outside | pass; 15 repeated runs; `-race` clean | `hf01_baseline_red.log` |
| HF-02 exec descendants | `internal/vm/exec_containment_linux_test.go` | **FAIL**: 7 surviving processes across 3 scenarios | pass; 15 repeated runs; `-race` clean; no leftover processes | `hf02_baseline_red.log` |
| HF-03 restart | `milestone2/ticketdesk/tests/regression/restart_default_budget.py` | **FAIL**: restart exits 1, `LIMIT_EXCEEDED` | 3/3 pass; restart with 1,000 tickets + 100 tokens in 0.3 s; default-budget restart verified at 6,000 tickets | `hf03_baseline_red.log`, `hf03_green.log`, `hf03_sweep.log` |
| HF-04 slow reader | `internal/vm/slow_reader_test.go` | **FAIL**: unrelated `/health` blocked | pass (3 consecutive runs); audit's own script: health 200 in 0.0 s (was a timeout past 60 s) | `hf04_baseline_red.log`, `hf04_audit_script_green.log` |
| HF-08 port diagnostic | `assurance_m10.py` (existing, unchanged) | 1 failed (`m10_port_in_use_stderr_shape`) | `5 tests, 0 failed, 1 expected failures` | `m2_after_hf01_04.log`, `final_ci_acceptance.log` |
| Review defect 1 (file grant -> sibling) | `TestFileRootGrantRefusesRelativeSymlinkToSibling`; race test extended | **FAIL** (read returned the sibling; 114 leaks in the race test) | pass | journal |
| Review defect 3 (early output close) | `TestExecProgramThatClosesItsOutputEarlyKeepsRunning` | **FAIL**: exit 137 after 180 ms, want 7 after 500 ms | pass | journal |

## 2. Original frozen evidence reproduced on the final tree (`scripts/ci_acceptance.sh`, exit 0)

| Suite | Result |
|---|---|
| Milestone 1 LedgerLens, frozen | 35 / 35 |
| Milestone 1 HealthWatch, frozen | 29 / 29 |
| Milestone 1 TaskFlow, frozen | 32 / 32 |
| Milestone 1 retained regressions (`fresh_boundaries.py`, HealthWatch `supplemental.py`, `rules_lowest_line.sh`) | pass |
| Milestone 2 TicketDesk frozen acceptance | 101 / 101 |
| Milestone 2 developer-experience checks | 13 / 13 |
| Milestone 2 reassessment / adversarial (includes SIGKILL, token interruption, legacy-state migration) | 11 / 11 |
| Milestone 2 historical review regressions | 10 / 10 (0 failed) |
| Milestone 2 Assurance m10 | 5 tests, 0 failed, **1 expected failure** (m12 lost-race, documented Owner-waived) |
| Milestone 2 default-ceiling restart / invalid state / SIGKILL (new) | 3 / 3 |

Frozen requirement and acceptance files are byte-unchanged:
`git diff 5d2983b2c2311daf9b7494311ad719ff5dc51c09..HEAD -- 'milestone1/*/requirements' 'milestone1/*/tests/acceptance'
'milestone2/ticketdesk/requirements' 'milestone2/ticketdesk/tests/acceptance' 'milestone2/ticketdesk/dx'
'milestone3/evidencedesk/requirements' 'milestone3/evidencedesk/tests/acceptance.py'` is empty. The only changes under
`milestone*/` are the TicketDesk port message (two byte-identical source copies), its README, the new regression script,
and the new Milestone 3 statement plus a pointer notice.
Log: `final_ci_acceptance.log` (also `ci_acceptance_local.log`, an earlier identical-outcome run).

## 3. Go suites

| Command | Toolchain | Result |
|---|---|---|
| `gofmt -l .`; `go build ./...`; `go vet ./...` | go1.25.0 | all exit 0 |
| `go test -count=1 ./...` (`HOWL_FORBID_LOCAL_INFERENCE=1`) | go1.25.0 | exit 0, **76 packages ok** |
| `go test -race -count=1 ./...` | go1.26.0 | exit 0, **76 packages ok, 0 data races** |
| Cross-compile `howlframe.go` | go1.26.0 | ok for windows/amd64, darwin/arm64, darwin/amd64, freebsd/amd64, linux/arm64 |

Logs: `final_go125_full.log`, `final_go_race_full.log`. Issue #192 (race failures in `TestIssue165RouteNetworkScope`,
`TestAccessLogIsOptIn`, `TestRequestIDAccessLogAndFaultLine`) is fixed in the tests without changing any assertion
(a mutex-guarded buffer replaces a `bytes.Buffer` that was read while `os/exec` was writing it); they pass under `-race`.
Known and unchanged: `GOOS=windows go build ./...` fails in `internal/testutil/fetch_fixture.go` (`syscall.Flock`), pre-existing
and test-only; the CLI itself builds for Windows.

## 4. Milestone 3

* Consumer compatibility (`hf06_board_current_vs_historical.log`, remediated build): current upstream HowlBoard `f4c7bb5`
  unmodified, with its declared grants: **3/3 and 72/72 timeline events**; under the narrow scoped grants: does not start;
  historical checkout `cd2f2fdb` unmodified: fails with 500 `CAPABILITY_DENIED` exactly as the historical record says.
* This check found a regression in this remediation (single-file grants); fixed and tested.
* **Full EvidenceDesk workflow on the final tree** (`final_m3_workflows.log`, exit 0): `HOWLFRAME_BIN=<final build>
  HOWLBOARD_REPO=<archive of cd2f2fdb> bash milestone3/evidencedesk/scripts/test.sh`. 6 / 6 frozen API and projector tests; live
  loopback listener; exact restart persistence with encoded IDs and Unicode notes; **real Chromium 148** import, filter, ordered
  claims, notes, reload, errors, XSS and keyboard; the adapted Board consumer (the historical procedure, on the historical
  checkout) reading the 3-record and 72-record ledgers; all five individual grant denials; denied static read without persisted
  corruption; too-small instruction limit enforced. This was run after the review-round fixes, which touched the single-file
  grant path the projector uses. It was *not* run in CI (needs Playwright and a Board checkout; documented manual step).

## 5. Independent review

| Round | Reviewer (provider) | Commit | Verdict |
|---|---|---|---|
| 1 | Codex (OpenAI), sandboxed without sockets | `52b9b714` | **REJECT**: 3 reproduced defects (high: file grant reached a sibling; high: killing the supervisor frees detached descendants; medium: early output close killed a command). Gates needing sockets were not executable |
| 2 | Antigravity `agy` (Google), sandboxed, network allowed | `3832d83e` | **ACCEPT**, all nine gates; defects 1 and 3 re-attacked and fixed; defect 2 judged honestly documented |

Notes on weight: the implementer did not review its own work; neither reviewer is the implementing provider. Round 2's prose misnames some
code (it cites `openPinnedFile`, `O_NOFOLLOW` and test names that do not exist; the real mechanism is `Scope.openTarget`
with `Lstat` plus `os.SameFile`), so I did **not** accept the report on its word: I ran its two probe scripts
(`evidence/review/probe_gate{1,2}.py`) against my own build of the final commit and they pass (single-file symlink refused
through bytecode and interpreter, 50-run swap race with 0 escapes, no fd leaks into children, deadline and output-limit sweeps, 20
concurrent execs, exit status 7 for the early-close command). Reports and prompts: `evidence/review/`.
Round 1 used the reviewer's own (non-socket) sandbox; the Codex limit was reached before a socket-enabled rerun, so Round 2 was run on `agy`.

## 6. CI

The new workflow jobs (`acceptance`, `race`, `latest-go`; `build` on the minimum toolchain) were validated locally by running
the same commands. Hosted results: see the pull request checks (link recorded in `HOWLFRAME_FINAL_HANDOFF.md`).
