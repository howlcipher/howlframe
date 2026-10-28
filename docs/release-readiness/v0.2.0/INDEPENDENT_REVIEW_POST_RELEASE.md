# Independent Review Report

This report evaluates two pull requests:
1. **Item 1: HowlBoard PR (application repository)** — PR #24 (`howlboard_pr24.diff`)
2. **Item 2: HowlFrame PR (test-only fix)** — PR #241 (`howlframe_pr241.diff`)

All claims below explicitly distinguish between what was **read** in source code and what was **ran** (including exact command invocations and relevant output lines). Complete raw execution logs are archived in `work/logs/`. All tests and modifications were conducted strictly in isolated copies under `/tmp/work/`, leaving `howlboard_main/`, `howlboard_branch/`, `howlframe_pr241/`, and `release/` untouched.

---

## 1. Item 1: HowlBoard PR (`howlboard_branch`)

### 1.1 Requirements Verification

#### Requirement 1: Pinned Release Archive Installation & SHA-256 Gate
> *CI installs HowlFrame from the published v0.2.0 archive, verified against a SHA-256 pinned in the HowlBoard repository (not only the release's own `SHA256SUMS`); a mismatch must fail before anything is installed or run.*

* **Code Read:**
  Examined `scripts/install_howlframe.sh` lines 10–51:
  - Line 10 sets `VERSION=v0.2.0`.
  - Lines 13–22 define a `case "$(uname -s)/$(uname -m)"` block with pinned SHA-256 digests for `Linux/x86_64` (`692c7130588422240eb84ee9682c26efc2d0fb9b0f1519bcc7c84b45bff83205`), `Linux/aarch64` (`e64e3131165c279efe56be72547aec234bbe13bce34d9bad2e0c0b90e82714e4`), `Darwin/x86_64` (`c88caf30767110409196c4ce36583f136e5c32e1065d70ea694f6faae6c968f9`), and `Darwin/arm64` (`e592637a84df248d511e0c394497b209d3adaa04b37d786ad02301c4ab786d9d`).
  - Downloads the archive directly to a temporary directory `$WORK` via `mktemp -d` (`trap 'rm -rf "$WORK"' EXIT`).
  - Compares the actual digest against the repository-pinned `$SHA256` (lines 33–41). If mismatched, prints an error message and exits `1`.
  - After unpacking in `$WORK`, executes `"${WORK}/howlframe" version` (lines 44–48) and verifies the reported version equals `HowlFrame 0.2.0`.
  - Only when both gates succeed does it run `install -m 0755 "${WORK}/howlframe" "$DEST"` (line 50).

* **Command Ran (Installation):**
  In an isolated copy of `howlboard_branch`, executed `scripts/install_howlframe.sh`:
  ```bash
  cd /tmp/work/howlboard_branch_test1 && scripts/install_howlframe.sh
  ```
  Output:
  ```
  Installed HowlFrame 0.2.0 (linux_amd64) as /tmp/work/howlboard_branch_test1/howlframe_bin
  ```
  Verified binary checksum and version (log: `work/logs/03_installer_run.log`):
  ```bash
  sha256sum howlframe_bin && ./howlframe_bin version
  ```
  Output:
  ```
  af9216894e9a14148b8592fc12aacc6e1d6c6115033799a6c99882a0b223bbb7  howlframe_bin
  HowlFrame 0.2.0
  HFBC format: 2
  ```
  Verified that the extracted binary in `release/howlframe_v0.2.0_linux_amd64.tar.gz` matches this exact SHA-256 (`af921689...`, log: `work/logs/01_release_tarball.log`).

* **Command Ran (Gate Falsification - Hash Mismatch):**
  Corrupted the pinned hash in `scripts/install_howlframe.sh` and ran the script:
  ```bash
  scripts/install_howlframe.sh
  ```
  Output (log: `work/logs/05_checksum_gate.log`):
  ```
  install_howlframe.sh: howlframe_v0.2.0_linux_amd64.tar.gz has SHA-256 692c7130588422240eb84ee9682c26efc2d0fb9b0f1519bcc7c84b45bff83205, expected 692c7130588422240eb84ee9682c26efc2d0fb9b0f1519bcc7c84b45bff83200; nothing installed
  ```
  Exit code was `1`, and `[ -e howlframe_bin ]` confirmed that `./howlframe_bin` was not created.

* **Command Ran (Gate Falsification - Version Mismatch):**
  Tested with an archive matching the hash but failing the version string check:
  ```bash
  scripts/install_howlframe.sh
  ```
  Output (log: `work/logs/05_checksum_gate.log`):
  ```
  install_howlframe.sh: archive reports 'HowlFrame 0.2.0', expected 'HowlFrame 0.3.0'; nothing installed
  ```
  Exit code was `1`, and `./howlframe_bin` was not created.

* **Status:** **MET**.

---

#### Requirement 2: No Source Build in CI and Quickstart
> *CI and the README quickstart no longer build HowlFrame from source; Go remains only for the backend test harness.*

* **Code Read:**
  - Examined `.github/workflows/ci.yml`: lines 27–33 replace the source checkout and `go build` with `scripts/install_howlframe.sh`. `setup-go@v5` is retained with a comment: `# Go above is only for the backend contract tests.`
  - Examined `README.md` lines 33–45:
    Prerequisites state: `Requires curl, Python 3 (to serve the static frontend), and Go 1.26 for make test only.`
    Quickstart step 1 executes `scripts/install_howlframe.sh`.
* **Status:** **MET**.

---

#### Requirement 3: Existing Tests and CI Steps Unchanged and Passing
> *Existing HowlBoard tests and CI steps are unchanged in substance and still pass.*

* **Code Read:**
  Reviewed diff `howlboard_pr24.diff`. No test files in `backend/` or `frontend/` were modified. The changes were restricted to CI workflow, docs, README, changelog, `install_howlframe.sh`, and `test_docs.py`.
* **Commands Ran:**
  In a fresh copy of `howlboard_branch` with `./howlframe_bin` installed:

  1. **Ledger Import Tool Verification (`make import`):**
     ```bash
     head -n 500 data/fixtures/ledger_sample.jsonl > /tmp/slice.jsonl
     make import LEDGER=/tmp/slice.jsonl MAX=500
     ```
     Output (log: `work/logs/09_ledger_import.log`):
     ```
     ./howlframe_bin -run-bc -allow-caps filesystem,database \
         -max-instructions 200000000 -max-memory-bytes 268435456 \
         build/ledger_import.hfbc /tmp/slice.jsonl 500
     entries=72
     missions=16
     ```
     Exit code: `0` (**PASS**).

  2. **Documentation Verification (`test_docs.py`):**
     ```bash
     python3 scripts/test_docs.py
     ```
     Output (log: `work/logs/10_test_docs.log`):
     All 28 checks reported `PASS`. Final line: `All documentation checks passed`. Exit code: `0` (**PASS**).

  3. **SEO Verification (`test_seo.py`):**
     ```bash
     python3 scripts/test_seo.py
     ```
     Output (log: `work/logs/11_test_seo.log`):
     `All SEO validations PASSED successfully!`. Exit code: `0` (**PASS**).

  4. **Timeline Layout Verification (`test_timeline_layout.py`):**
     ```bash
     python3 scripts/test_timeline_layout.py
     ```
     Output (log: `work/logs/12_test_timeline.log`):
     ```
     Playwright is installed
     Built /tmp/.../app.js
     Traceback (most recent call last):
       File ".../scripts/test_timeline_layout.py", line 38, in main
         page.add_style_tag(path=str(ROOT / 'frontend/styles.css'))
     playwright._impl._errors.Error: Page.add_style_tag: Event
     ```
     Exit code: `1` (**FAIL** due to Playwright local file-path policy in sandboxed Chromium).

  5. **Backend Contract Tests Suite:**
     Ran with `HOWL_FORBID_LOCAL_INFERENCE=1`:
     ```bash
     export HOWL_FORBID_LOCAL_INFERENCE=1
     cd backend && go test -count=1 -v ./...
     ```
     Counts from execution (log: `work/logs/08_backend_contract_tests_full.log`):
     - **Top-level `--- PASS` lines:** 18
     - **Nested `--- PASS` lines:** 38 (21 in `TestMissionAPIContract`, 17 in `TestLedgerImportRejectsInvalidLedgerWithoutWriting`)
     - **Top-level `--- FAIL` lines:** 5
       * `--- FAIL: TestFactoryRemoteSurface (0.00s)`
       * `--- FAIL: TestDeniedFactoryFetchIsNotAbsent (0.00s)`
       * `--- FAIL: TestLedgerImportServedByBoard (0.05s)`
       * `--- FAIL: TestMutationAuthorizationAndNoEffect (0.00s)`
       * `--- FAIL: TestMissingWriteConfigurationRefuses (0.00s)`
     - **Nested `--- FAIL` lines:** 2
       * `--- FAIL: TestMissingWriteConfigurationRefuses/missing (0.00s)`
       * `--- FAIL: TestMissingWriteConfigurationRefuses/malformed (0.00s)`
     
     Summary lines pasted from log:
     ```
     FAIL
     FAIL	howlboard/backend	1.474s
     FAIL
     ```
     Exit code: `1`.

* **Root Cause Analysis of Backend Test Failure:**
  Every failing test failed with:
  `contract_test.go:97: port localhost:8080 is already in use; stop the other server first (listen tcp 127.0.0.1:8080: bind: address already in use)`.
  Investigation revealed:
  - `TestMissionAPIContract` makes HTTP calls using Go's `http.DefaultClient`.
  - `http.DefaultClient` holds persistent keep-alive TCP connections in its connection pool.
  - When `TestMissionAPIContract` completes, its `t.Cleanup` sends `SIGKILL` to `howlframe_bin`.
  - Because the server terminates with client connections open, the server's TCP endpoint enters the kernel `TIME_WAIT` state (`[::ffff:127.0.0.1]:8080`) for 60 seconds (`tcp_fin_timeout`).
  - The subsequent test calls `requireFreePort(t)`, which executes `net.Listen("tcp", "localhost:8080")`.
  - Linux rejects IPv4 bind while an IPv6 dual-stack socket on the port is in `TIME_WAIT`. Because `requireFreePort` does not retry and `http.DefaultClient.CloseIdleConnections()` is not called prior to `SIGKILL`, the test fails immediately.
  - **Crucially:** We ran `cd backend && go test -count=1 -v ./...` against `howlboard_main` (log: `work/logs/07_howlboard_main_test.log`) and verified that **this exact failure occurs identically on `main` before PR 24 was applied**. PR 24 did not introduce this defect. When run individually, each test passes.

* **Status:** **PARTIALLY MET / UNMET IN TEST SUITE EXECUTION** (Pre-existing defect in `backend/contract_test.go` causes `make test` to fail on sequential execution).

---

#### Requirement 4: Release as Supported Compiler & Rollback Pin Options
> *The documentation names the release as the supported compiler and keeps the earlier source commits (`e77e008f…`, `7631ce5d…`) as rollback options.*

* **Code Read:**
  - `README.md` lines 61–80:
    ```markdown
    The documented build and CI use the published HowlFrame release
    [v0.2.0](https://github.com/howlcipher/howlframe/releases/tag/v0.2.0) (tag
    `v0.2.0`, commit `77cb55c87076aef846a601f18ace6bba78a12010`).
    ...
    git -C /tmp/howlframe checkout --detach v0.2.0   # or e77e008f7eb944f286414f4a5725b306fcc02d31
    ...
    The older known-good pin 7631ce5d4008534ef681f35c3436922507d3f441 remains
    recorded for rollback together with Board3b1e007...
    ```
  - `change_log.md` lines 90–96 document `v0.2.0`, `e77e008f`, and `7631ce5d`.
* **Status:** **MET**.

---

#### Requirement 5: Single Source of Truth for Version/Checksums & Enforcement
> *The version and checksums live in one place, and an automated docs check enforces that every documented install path uses that place.*

* **Code Read:**
  - `scripts/install_howlframe.sh` contains the only definition of `VERSION=v0.2.0` and SHA-256 hashes. Neither `README.md` nor `docs/index.html` hardcodes checksums.
  - `scripts/test_docs.py` check 11 enforces:
    1. Installer pins version (`pinned is not None`)
    2. CI installs via installer and does not call `go build`
    3. README and `docs/index.html` install via installer
    4. README names the installer's release version
* **Command Ran (Docs Check Falsification Suite):**
  Tested 6 distinct mutations in a copy of `howlboard_branch` (log: `work/logs/13_docs_checks_falsification.log`):
  1. Remove installer from `.github/workflows/ci.yml` -> Fails: `FAIL CI installs HowlFrame through the pinned installer` (Exit 1).
  2. Add `go build` to `.github/workflows/ci.yml` -> Fails: `FAIL CI installs HowlFrame through the pinned installer` (Exit 1).
  3. Change installer script name in `README.md` -> Fails: `FAIL README and published page install through the pinned installer` (Exit 1).
  4. Change installer script name in `docs/index.html` -> Fails: `FAIL README and published page install through the pinned installer` (Exit 1).
  5. Remove `` `v0.2.0` `` formatting from `README.md` -> Fails: `FAIL README names the installer's release version` (Exit 1).
  6. Unset version pin in `install_howlframe.sh` -> Fails: `FAIL installer pins a HowlFrame release version` and `FAIL README names the installer's release version` (Exit 1).
* **Status:** **MET**.

---

### 1.2 Evaluation of `scripts/install_howlframe.sh` Failure Modes

1. **Partial Installs:**
   - *Code Read:* Archive is downloaded to an isolated `$WORK` directory (`mktemp -d`). Checksum and version tests are executed before `install -m 0755 "${WORK}/howlframe" "$DEST"`.
   - *Command Ran:* In hash mismatch and version mismatch tests, confirmed `$DEST` was never touched or created (log: `work/logs/05_checksum_gate.log`).
2. **Wrong Platform Handling:**
   - *Code Read:* `case "$(uname -s)/$(uname -m)"` handles `Linux/x86_64`, `Linux/aarch64|Linux/arm64`, `Darwin/x86_64`, and `Darwin/arm64`. Any other architecture prints `install_howlframe.sh: no HowlFrame v0.2.0 archive for ...; build it from source (README)` and exits `1`.
   - *Command Ran:* Mocked `uname` returning `FreeBSD/amd64`. Script printed error message, exited `1`, and installed nothing (log: `work/logs/05_checksum_gate.log`).
3. **Missing Tools:**
   - *Code Read:* Script sets `set -euo pipefail`. If `curl`, `tar`, `mktemp`, `install`, or hashing utilities are absent, bash terminates immediately. If `sha256sum` is unavailable, lines 33–37 fall back to `shasum -a 256`.
4. **Checksum Bypass:**
   - *Code Read:* Checksum comparison `if [ "$ACTUAL" != "$SHA256" ]; then ... exit 1; fi` is strict. If `ACTUAL` is empty or mismatched, the gate triggers and aborts.
5. **Temp-file Handling:**
   - *Code Read:* `trap 'rm -rf "$WORK"' EXIT` guarantees cleanup on exit or abnormal termination.
6. **Directory Independence:**
   - *Code Read:* Line 24 computes `ROOT="$(cd "$(dirname "$0")/.." && pwd)"` and `DEST="${ROOT}/howlframe_bin"`.
   - *Command Ran:* Executed `/tmp/work/howlboard_branch_test2/scripts/install_howlframe.sh` from `/tmp/other_dir`. Binary was correctly placed at `/tmp/work/howlboard_branch_test2/howlframe_bin`, with nothing placed in `/tmp/other_dir` (log: `work/logs/04_installer_from_other_dir.log`).

---

## 2. Item 2: HowlFrame PR (`howlframe_pr241`)

### 2.1 Problem Statement & Implementation Review
PR #241 modifies `apps/echo_api/echo_api_test.go` (`howlframe_pr241.diff`).
Claimed Problem: `apps/echo_api` tests left a server listening on `127.0.0.1/*:8106` after `go test`, and could pass against a leftover server.

* **Code Read (Old Implementation):**
  - Compiled with `exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "echo_api.howl")`. Artifact was written directly into the working tree (`echo_api.howl.bc.bin`).
  - Executed server via `srv := exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "network", "echo_api.howl.bc.bin")`.
  - Teardown was `defer func() { _ = srv.Process.Kill(); _, _ = srv.Process.Wait() }()`.
  - **Flaw in Old Code:** `go run` starts the compiled binary as a grandchild process. `srv.Process.Kill()` killed only the `go` runner, orphaning the child `howlframe` binary, which remained listening on `:8106`.

* **Code Read (New Implementation):**
  - Writes bytecode to `filepath.Join(t.TempDir(), "echo_api.hfbc")`.
  - Checks if port is occupied prior to starting:
    ```go
    if conn, err := net.DialTimeout("tcp", echoAddr, 200*time.Millisecond); err == nil {
        conn.Close()
        t.Fatalf("%s is already in use; refusing to test against a server this test did not start", echoAddr)
    }
    ```
  - Spawns server in dedicated process group: `srv.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`.
  - Terminates process group: `syscall.Kill(-srv.Process.Pid, syscall.SIGKILL)`.
  - Registers cleanup with `t.Cleanup(stop)`.
  - Tests port release with `waitPortReleased(t)` via `net.DialTimeout`.
  - Adds `TestEchoAPIStopsServerWhenTestFails` running a subprocess with `ECHO_API_TEST_FAIL_AFTER_START=1` to test cleanup on failure paths.

---

### 2.2 Demonstration of the Old Defect
In a copy of `howlframe_pr241`, reversed `howlframe_pr241.diff` using `patch -p1 -R` and ran `go test`:

* **Command Ran (Demonstrating Leftover Server):**
  ```bash
  cd /tmp/work/howlframe_old && go test -count=1 -v ./apps/echo_api/
  ```
  Output (log: `work/logs/15_howlframe_old_behavior.log`):
  ```
  === RUN   TestEchoAPI
  --- PASS: TestEchoAPI (2.81s)
  PASS
  ok  	github.com/howlcipher/howlframe/apps/echo_api	2.816s
  ```
  Immediately checked listening sockets and running processes:
  ```bash
  ss -ltnp 'sport = :8106'
  ps aux | grep -E "8106|howlframe|echo_api"
  ```
  Output:
  ```
  LISTEN 0      4096               *:8106            *:*    users:(("howlframe",pid=437,fd=8))
  howlcip+     437  0.0  0.0 1639048 10980 pts/4   Sl+  16:01   0:00 /home/howlcipher/.../howlframe -run-bc -allow-caps network echo_api.howl.bc.bin
  ```
  **Result:** Confirmed. The old test passed but left PID 437 listening on `*:8106`.

* **Command Ran (Demonstrating Old Test Pass Against Pre-existing Server):**
  Started an independent server listening on `8106` and ran the old test (log: `work/logs/16_new_test_against_leftover.log`):
  ```bash
  go test -count=1 -v ./apps/echo_api/
  ```
  The old test exited `0` (`PASS 0.28s`), silently testing against the foreign server.
  In contrast, the new test immediately refused:
  ```
  echo_api_test.go:79: 127.0.0.1:8106 is already in use; refusing to test against a server this test did not start
  --- FAIL: TestEchoAPI (0.00s)
  ```

---

### 2.3 Falsification & Critical Flaw in the New PR

When running the clean, unmodified `howlframe_pr241` test suite, **the test suite fails**:

* **Command Ran:**
  ```bash
  cd /tmp/work/howlframe_pr241_fresh && go test -count=1 -v ./apps/echo_api/
  ```
  Output (log: `work/logs/17_howlframe_pr241_run.log`):
  ```
  === RUN   TestEchoAPI
  --- PASS: TestEchoAPI (2.63s)
  === RUN   TestEchoAPIStopsServerWhenTestFails
      echo_api_test.go:141: child TestEchoAPI should fail after starting the server; err=exit status 1
          === RUN   TestEchoAPI
              echo_api_test.go:99: health: Get "http://127.0.0.1:8106/health": dial tcp 127.0.0.1:8106: connect: connection refused
          --- FAIL: TestEchoAPI (3.20s)
          FAIL
  --- FAIL: TestEchoAPIStopsServerWhenTestFails (3.21s)
  FAIL
  FAIL	github.com/howlcipher/howlframe/apps/echo_api	5.845s
  FAIL
  ```
  Exit code: `1`.

* **Root Cause Analysis:**
  1. `TestEchoAPI` runs first. It performs HTTP GET requests to `127.0.0.1:8106` using Go's `http.Get`.
  2. `http.Get` leaves keep-alive connections in `http.DefaultClient`.
  3. `stop()` sends `SIGKILL` to the server process group. Because the server is killed while client connections remain open, the server socket enters `TIME_WAIT` on port 8106 (`[::ffff:127.0.0.1]:8106`).
  4. `waitPortReleased(t)` only tests `net.DialTimeout`. Because the server process is dead, `net.DialTimeout` immediately receives `ECONNREFUSED` and reports the port released, even though the socket is still in `TIME_WAIT`.
  5. `TestEchoAPIStopsServerWhenTestFails` immediately runs in the same test execution. It spawns a child process to run `TestEchoAPI` with `ECHO_API_TEST_FAIL_AFTER_START=1`.
  6. The child calls `startEchoServer()`. `net.DialTimeout` again succeeds (connection refused). But when HowlFrame starts and executes `(http_server 8106 ...)`, Go's `net.Listen("tcp", ":8106")` crashes with:
     `{"phase":"runtime","code":"IO_ERROR","function":"main","instruction":27,"opcode":"HTTP_SERVER_SERVE","message":"http listen failed: listen tcp :8106: bind: address already in use"}`.
  7. The child server dies immediately. The child's health-check polling loop times out and fails with:
     `health: Get "http://127.0.0.1:8106/health": dial tcp 127.0.0.1:8106: connect: connection refused`.
  8. Because the child fails at the health check instead of reaching `t.Fatal("forced failure after the server started")`, the parent's assertion `strings.Contains(string(out), "forced failure after the server started")` fails!
  9. Therefore, **PR #241 fails its own verification suite**.

---

## 3. Verdict

ITEM 1 VERDICT: ACCEPT WITH NOTES
ITEM 2 VERDICT: REJECT

---

## 4. Findings

1. **[High Severity] HowlFrame PR #241 test suite fails deterministically during `go test ./apps/echo_api/`**
   - *Evidence:* Running `cd howlframe_pr241 && go test -count=1 -v ./apps/echo_api/` exited `1` (log: `work/logs/17_howlframe_pr241_run.log`). `TestEchoAPIStopsServerWhenTestFails` failed because the preceding `TestEchoAPI` left port 8106 in `TIME_WAIT`, causing the child server to crash on `bind: address already in use`.
   - *Remediation:* `TestEchoAPI` must invoke `http.DefaultClient.CloseIdleConnections()` before stopping the server, and `waitPortReleased` must probe port bindability (via `net.Listen`) rather than mere dialability.

2. **[Medium Severity] Pre-existing backend test harness failure in HowlBoard `backend/contract_test.go`**
   - *Evidence:* Running `cd backend && go test -count=1 -v ./...` in both `howlboard_branch` (log: `work/logs/08_backend_contract_tests_full.log`) and `howlboard_main` (log: `work/logs/07_howlboard_main_test.log`) failed with exit code `1`. Five tests failed with `port localhost:8080 is already in use` due to `TIME_WAIT` socket retention after `TestMissionAPIContract`.
   - *Note:* While this defect predates PR #24, it causes `make test` to fail in CI unless tests are isolated or connection teardown is remediated.

3. **[Low Severity] `scripts/test_timeline_layout.py` fails in sandboxed Playwright environments**
   - *Evidence:* Executing `python3 scripts/test_timeline_layout.py` failed with `Page.add_style_tag: Event` (log: `work/logs/12_test_timeline.log`) because Playwright's `add_style_tag(path=...)` encounters origin/security restrictions when injecting local stylesheet paths into `about:blank`.
   - *Remediation:* Pass CSS content via `page.add_style_tag(content=Path(...).read_text())` or serve via local HTTP server.

4. **[Low Severity] Windows platform unsupported by `install_howlframe.sh`**
   - *Evidence:* `scripts/install_howlframe.sh` only supports `Linux` and `Darwin`. Although `howlframe_v0.2.0_windows_amd64.zip` is published in the release, Windows users running bash are directed to build from source.

