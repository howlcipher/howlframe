# HowlFrame Remediation Independent Review Report (Round 2)

**Review Target Commit:** `3832d83e7a4d8e33648db5c93d9611f4efde8441`  
**Baseline Commit:** `5d2983b2c2311daf9b7494311ad719ff5dc51c09`  
**Reviewer Role:** Independent Adversarial Reviewer  
**Final Overall Verdict:** **ACCEPT**

---

## Executive Summary

In Round 1, the remediation branch was rejected due to three reproduced defects:
1. Single-file capability grant sibling escape via relative symlink (`internal/capability/scopedfs.go`).
2. Detached process escape when child targets and kills its supervisor (`internal/vm/exec_contain_linux.go`), while claiming complete containment in documentation.
3. Early termination (exit code 137) of commands closing stdout/stderr before exiting (`internal/vm/exec_contain_linux.go`).

In Round 2, all three defects were re-attacked with independent adversarial probes and verified resolved:
- Single-file grants now use atomic open/stat pinning (`os.OpenFile` + `os.SameFile`), strictly refusing symlinks and directory escapes.
- Exec containment supervisor now tracks child exit status via a dedicated control/status pipe independently from stdio streams; programs closing stdout/stderr exit with their real exit status and execution duration.
- Process containment documentation in `docs/remediation/HOWLFRAME_SECURITY_REMEDIATION.md` has been amended to honestly and completely describe the residual limit regarding adversarial supervisor termination.
- All gates were fully executed: test suites passed with 0 data races under `-race`, `scripts/ci_acceptance.sh` passed all suites, TicketDesk restarted successfully under default ceilings, HowlBoard upstream repository was verified, and frozen test suites remain 100% byte-identical.

---

## Re-Attack of Round 1 Rejection Defects

### Defect 1: Single-File Filesystem Grant Sibling Bypass
- **Vulnerability in Round 1:** Opening a single-file grant opened an `os.Root` on the parent directory. An adversary could swap the granted file for a relative symlink pointing to an ungranted sibling file, allowing unauthorized read/write access.
- **Fix in Current Commit:** `internal/capability/scopedfs.go` pins single-file grants via `openPinnedFile` (`os.OpenFile` with `O_NOFOLLOW` / `os.Lstat` verification + `os.SameFile` check before truncation or write).
- **Adversarial Probe (`/tmp/probe_gate1.py`):**
  - *Probe 1.1 (Static Relative Symlink):* Created granted file `target.txt` pointing to ungranted sibling `sibling.txt`. Executed HowlFrame bytecode and interpreter reading/writing `target.txt`.
    - **Result:** Refused with `CAPABILITY_DENIED: access outside granted root /tmp/.../target.txt`.
  - *Probe 1.2 (Concurrent Symlink Swapping Race):* Ran 50 concurrent swap attempts while repeatedly writing to the single-file grant.
    - **Result:** 0 escapes to sibling file. In 50/50 runs, sibling content remained untampered (`SAFE`). Swapped symlinks were rejected with `CAPABILITY_DENIED` or `EEXIST`.
  - *Probe 1.3 (Store Open & Sync):* Attempted store open on a single-file grant swapped to sibling.
    - **Result:** Refused with `CAPABILITY_DENIED`. Sibling file unmodified.
- **Verdict:** **FIXED.**

### Defect 2: Exec Supervisor Death / Detached Descendant Containment
- **Vulnerability in Round 1:** Child process killed its parent supervisor via `$PPID`, allowing double-forked `setsid` descendants to escape. Documentation claimed unconditional containment on Linux.
- **Remediation in Current Commit:** `docs/remediation/HOWLFRAME_SECURITY_REMEDIATION.md` (lines 92–107) accurately and explicitly documents this residual boundary:
  > *"A program written to defeat containment can. The supervisor is the program's parent, and a same-user program can find and kill it. If it does, descendants that already detached into their own session are no longer re-parented to the supervisor and can survive (reproduced by the independent review). Containment therefore protects against runaway, forgotten or accidentally detached descendants (issue #181's case), not against hostile code that targets the supervisor."*
- **Assessment:** The documentation is honest, precise, and technically accurate. It clearly delineates the difference between unintentional runaway orphan processes (the scope of issue #181) and intentional adversary subversion, noting that OS user namespaces / cgroup v2 are not unprivileged primitives available on stock configurations.
- **Verdict:** **DOCUMENTATION HONEST & COMPLETE.**

### Defect 3: Command Closing stdout/stderr Early Killed with Exit 137
- **Vulnerability in Round 1:** Commands executing `exec >/dev/null 2>&1` followed by a sleep were killed prematurely after ~130ms with exit code 137 because stdout/stderr EOF was conflated with process termination.
- **Fix in Current Commit:** Supervisor tracks process exit status over an independent status pipe. Runner waits for process exit notification from supervisor before draining output buffers.
- **Adversarial Probe (`/tmp/probe_gate2.py`):**
  - Ran command: `/bin/dash -c 'exec >/dev/null 2>&1; sleep 0.5; exit 7'` via HowlFrame exec runner.
  - **Result:** Command ran for **0.621s**, exited with **exit code 7** (NOT exit 137).
  - Dedicated unit test `internal/vm/exec_early_close_linux_test.go` also passed under `go test -race`.
- **Verdict:** **FIXED.**

---

## Gate-by-Gate Verification Results

### Gate 1: Filesystem Containment (HF-01)
- **Verdict:** **ACCEPT**
- **Evidence:**
  - Race tests on baseline (`5d2983b`):
    ```sh
    cd /tmp/baseline && GOCACHE=/tmp/gocache go test -v -run 'TestFilesystemScopeRace|TestFileRootGrant' ./internal/vm
    ```
    *Result:* **FAILED** (618 escaped reads, escaped writes, escaped mkdir, escaped store writes, 2180 file root grant escapes).
  - Race tests on HEAD (`3832d83`):
    ```sh
    GOCACHE=/tmp/gocache go test -race -v -run 'TestFilesystemScopeRace|TestFileRootGrant' ./internal/vm
    ```
    *Result:* **PASS** (0 escapes over 4,000 iterations each, no data races).
  - Capability unit tests:
    ```sh
    GOCACHE=/tmp/gocache go test -race -v -count=1 ./internal/capability
    ```
    *Result:* **PASS** (1.096s, 0 data races).
  - Adversarial probes (`/tmp/probe_gate1.py`):
    - Relative symlink at single-file root: Denied (`CAPABILITY_DENIED`).
    - Concurrent symlink race: 0 escapes.
    - Directory grant escaping via internal symlinks: Denied by `os.Root`.
    - Hard links and parent directory replacements remain properly documented as host-level limits.

---

### Gate 2: Process Containment (HF-02, Issue #181)
- **Verdict:** **ACCEPT**
- **Evidence:**
  - Containment tests on baseline (`5d2983b`):
    ```sh
    cd /tmp/baseline && GOCACHE=/tmp/gocache go test -v -run 'TestExecContainmentLinux' ./internal/vm
    ```
    *Result:* **FAILED** (7 surviving processes: background, setsid, and deadline sleepers survived).
  - Containment tests on HEAD (`3832d83`):
    ```sh
    GOCACHE=/tmp/gocache go test -race -v -run 'TestExecContainmentLinux' ./internal/vm
    ```
    *Result:* **PASS** (normal exit, deadline, and output-limit sweeps all cleanly terminated with 0 leftover processes).
  - Adversarial probes (`/tmp/probe_gate2.py`):
    - Probe 2.1 (Early close stdout/stderr): Real exit code 7 preserved, duration 0.621s (Pass).
    - Probe 2.2 (File descriptor leak check): Inspected child `/proc/self/fd`; zero supervisor control or status pipes leaked into child (Pass).
    - Probe 2.3 (Deadline during exec): 3-tier process tree killed completely on deadline; 0 surviving processes (Pass).
    - Probe 2.4 (Output limit exceeded during exec): Child generating excessive output killed along with entire process group; 0 surviving processes (Pass).
    - Probe 2.5 (High concurrency): 20 simultaneous contained execs completed with 0 deadlocks, hangs, or race conditions (Pass).

---

### Gate 3: TicketDesk Restart (HF-03)
- **Verdict:** **ACCEPT**
- **Evidence:**
  - Tested on baseline (`5d2983b`):
    ```sh
    python3 milestone2/ticketdesk/tests/regression/restart_default_budget.py
    ```
    *Result:* **FAILED** (`LIMIT_EXCEEDED: instruction limit exceeded (100000)` during replay of 1,000 tickets).
  - Tested on HEAD (`3832d83`):
    ```sh
    python3 milestone2/ticketdesk/tests/regression/restart_default_budget.py
    ```
    *Result:* **PASS** (3/3 passed in 2.24s):
    - `restart_with_1000_tickets_100_tokens_default_ceilings`: Replayed and restarted in 0.35s under default ceilings.
    - `invalid_state_rejected_without_damage`: Successfully rejected corrupted journal without state degradation.
    - `sigkill_keeps_acknowledged_writes_at_default_ceilings`: All acknowledged tickets/tokens persisted across SIGKILL.
  - Architectural evaluation of `--max-startup-instructions`:
    - Sound and narrowly scoped: `--max-startup-instructions` applies strictly during the initial setup/journal replay phase before the HTTP listener accepts connections (`internal/vm/vm.go`).
    - Once the server enters serving state, request handler execution ceilings revert strictly to `--max-instructions` (100,000 instructions per request). Global instruction ceilings are not weakened.

---

### Gate 4: Slow Reader (HF-04)
- **Verdict:** **ACCEPT**
- **Evidence:**
  - Automated slow reader test:
    ```sh
    GOCACHE=/tmp/gocache go test -race -v -run TestSerializedServerSurvivesNonReadingClient ./internal/vm
    ```
    *Result:* **PASS** (7.04s, 0 data races).
  - Static review of `internal/vm/http_response.go`:
    ```go
    defer buffered.flushTo(w)
    defer serial.Unlock()
    ```
    Because `defer` executes in LIFO order, `serial.Unlock()` executes before `buffered.flushTo(w)`. The process-wide serialization lock is released immediately once the handler finishes computing the response; writing to the slow socket occurs outside the critical section under a 30s deadline configured via `http.NewResponseController`.
  - Intact response: 2 MiB payload delivered completely; unrelated `/health` endpoint responds concurrently in <1ms without being blocked by the stalled client.

---

### Gate 5: Assurance Regression (HF-08)
- **Verdict:** **ACCEPT**
- **Evidence:**
  - File integrity check:
    ```sh
    diff -u /tmp/baseline/milestone2/ticketdesk/tests/regression/assurance_m10.py milestone2/ticketdesk/tests/regression/assurance_m10.py
    ```
    *Result:* Byte-identical (0 diff).
  - Execution check:
    ```sh
    python3 milestone2/ticketdesk/tests/regression/assurance_m10.py
    ```
    *Result:* `5 tests, 0 failed, 1 expected failures`. Strictly matches documented XFAIL (`m10_port_lost_race_stderr_shape`). No test weakened or deleted.

---

### Gate 6: Milestone 3 Evidence (HF-06)
- **Verdict:** **ACCEPT**
- **Evidence:**
  - Cloned upstream repository:
    ```sh
    git clone https://github.com/howlcipher/howlboard /tmp/howlboard
    ```
    *Result:* HEAD commit `f4c7bb544773a717d2bd0640e91ff9f154bdd9b6`.
  - Audited against claims in `milestone3/EVIDENCE_STATEMENT_2026-10-21.md`:
    - Commit `bda6746` added route grants: Confirmed in HowlBoard commit log.
    - Zero references to EvidenceDesk in HowlBoard: Confirmed (`grep -ri "EvidenceDesk" /tmp/howlboard` returned 0 matches).
    - Historical checkout `cd2f2fdb` was 12 commits behind: Confirmed (`git rev-list --count cd2f2fdb..f4c7bb54` = 12).
    - Importer compatibility: Confirmed `EvidenceDesk` retained as an independent downstream consumer.
  - Factual representation: The evidence statement explicitly disclaims Owner approval and makes no false claims of ecosystem adoption.

---

### Gate 7: Frozen Application Tests
- **Verdict:** **ACCEPT**
- **Evidence:**
  - Checked test definitions against baseline:
    ```sh
    git diff 5d2983b2c2311daf9b7494311ad719ff5dc51c09..HEAD -- milestone1 milestone2 milestone3
    ```
    *Result:* 0 diff across all frozen requirements and acceptance tests.
  - Acceptance script run:
    ```sh
    GOCACHE=/tmp/gocache bash scripts/ci_acceptance.sh
    ```
    *Result:* **PASS** across all suites:
    - Milestone 1: LedgerLens (35/35), HealthWatch (29/29), TaskFlow (32/32).
    - Milestone 2: TicketDesk (101/101), DX suite (13/13), Reassessment (11/11), Round 1 Review Suite (10/10), Assurance M10 (5/5 with 1 XFAIL), Restart Default Budget (3/3).
    - Overall exit code: 0.

---

### Gate 8: Continuous Integration (CI)
- **Verdict:** **ACCEPT**
- **Evidence:**
  - Inspected `.github/workflows/ci.yml`:
    - Required job name `build` preserved.
    - Toolchain pinned to Go 1.25.x.
    - `build`, `acceptance`, `race`, and `latest-go` matrix jobs configured and active.
    - No tests skipped or assertions loosened.

---

### Gate 9: Whole-Patch Review
- **Verdict:** **ACCEPT**
- **Evidence:**
  - Codebase verification:
    ```sh
    go build ./... && go vet ./...
    ```
    *Result:* Clean build, 0 vet issues.
  - Full test suite:
    ```sh
    GOCACHE=/tmp/gocache go test -count=1 ./...
    ```
    *Result:* **PASS** across all 76 packages.
  - Race detector on modified packages:
    ```sh
    GOCACHE=/tmp/gocache go test -race -count=1 ./internal/vm ./internal/capability
    ```
    *Result:* **PASS** (0 data races detected).
  - Cross-platform compilation:
    - `GOOS=darwin go build ./...`: **PASS**.
    - `GOOS=windows go build .`: **PASS** (HowlFrame CLI compiles for Windows).
    - `GOOS=windows go build ./...`: Fails on `internal/testutil/fetch_fixture.go:20` (`syscall.Flock`). This is an unchanged, pre-existing test utility file from baseline that does not affect runtime or CLI binaries.
  - Scope: Changes strictly address HF-01, HF-02, HF-03, HF-04, HF-06, and HF-08. No extraneous refactoring or scope creep.

---

## Defects Ranked by Severity

1. **Low (Pre-Existing) — Windows Whole-Module Test Build Failure**  
   - **Location:** `internal/testutil/fetch_fixture.go:20`  
   - **Description:** Direct use of `syscall.Flock` prevents `GOOS=windows go build ./...` from compiling test packages. The file is unchanged from baseline, and the HowlFrame Windows binary itself (`GOOS=windows go build .`) compiles cleanly.

2. **Informational / Documented Boundary — Supervisor Termination Detachment**  
   - **Location:** `internal/vm/exec_contain_linux.go:130`  
   - **Description:** A child process running as the same user that intentionally discovers and signals `SIGKILL` to its parent supervisor can allow detached `setsid` processes to survive. Accurately documented as a residual boundary in `docs/remediation/HOWLFRAME_SECURITY_REMEDIATION.md`.

---

## Verification Limitations

- **Hosted GitHub Actions Environment:** CI workflow configuration was verified statically and validated by running the identical command suite locally; actual execution on GitHub-hosted runners requires repository push permissions.
- **Windows Runtime Execution:** Cross-compilation of the Windows binary was verified on Linux; runtime behavior on Windows was not directly executed.

---

## Final Recommendation

All defects identified in Round 1 have been completely resolved or documented with technical accuracy. No regressions were introduced, all acceptance suites passed, and race detection reports zero data races.

**Final Verdict: ACCEPT.**
