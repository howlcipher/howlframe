**Overall verdict: REJECT.** Three reproduced defects prevent acceptance. The [full report](/tmp/hf-review/REPORT.md) contains exact commands, probe sources, results, and verification limits. No tracked files were changed.

Defects, ranked:

1. **High — single-file grant bypass**, `internal/capability/scopedfs.go:155`. Confinement expands to the file’s parent directory. After opening a scope on the granted file, replacing it with a relative symlink allowed overwriting an ungranted sibling with `CORRUPTED`. A compiled bytecode probe also read sibling content using only the single-file grant. This contradicts the documented guarantee.

2. **High — supervisor death permits descendant escape**, `internal/vm/exec_contain_linux.go:129`. An exec child started a redirected `setsid` descendant and killed its supervisor through `$PPID`. Exec returned `signal: killed`; the detached descendant remained alive. Probe cleanup killed it. The process-group fallback does not contain this case.

3. **Medium — closing output prematurely kills ordinary commands**, `internal/vm/exec_contain_linux.go:111` and `:183`. Running:
   ```sh
   /bin/sh -c 'exec >/dev/null 2>&1; sleep 0.5; exit 7'
   ```
   through `runContained(context.Background(), ...)` returned exit **137 after 124–137 ms**, instead of exit **7 after approximately 500 ms**. Output EOF is incorrectly treated as cancellation.

4. **Low, pre-existing — Windows whole-module build failure**, `internal/testutil/fetch_fixture.go:20`. Unconditional `syscall.Flock` prevents `GOOS=windows go build ./...`. The file is unchanged from baseline; the Windows CLI itself builds.

| Gate | Verdict | Evidence |
|---|---|---|
| **1. HF-01 filesystem** | **REJECT** | Supplied race tests pass on HEAD under `-race`. All five security tests fail on baseline: 622 escaped reads, 975 escaped single-file reads, plus outside writes, mkdir and store writes. New sibling probes expose the missed bypass. Interpreter and store paths use the wrapper. Hard links and replaceable root parents remain documented exclusions. |
| **2. HF-02 exec** | **REJECT** | Three supplied descendant tests pass under `-race`; ordinary exec tests and TaskFlow pass. Adversarial probes reproduce defects 2–3. Control/status descriptors are marked close-on-exec, but complete containment is disproved. |
| **3. HF-03 restart** | **ACCEPT-WITH-CONCERNS** | Restart script executed but failed creating a socket: `PermissionError: Operation not permitted`. Startup-budget scoping tests pass. Design is finite and server-specific, with request budgets preserved. Actual 1,000-ticket/100-token restart, authorization and SIGKILL behavior remain unverified. |
| **4. HF-04 slow reader** | **ACCEPT-WITH-CONCERNS** | Test cannot bind loopback. Static review confirms unlock occurs before buffered flush and writes receive deadlines. Response integrity, concurrent behavior and resource release could not be reproduced. |
| **5. HF-08 Assurance** | **ACCEPT-WITH-CONCERNS** | File byte-unchanged. Execution: **5 tests, 5 failed, 0 expected failures**, all reporting EPERM. Existing strict XFAIL remains intact; no weakening found. |
| **6. HF-06 evidence** | **ACCEPT-WITH-CONCERNS** | Fresh GitHub access failed. Offline clone at exact `f4c7bb5` independently confirms hashes, 12-commit gap, grant change and retained importer. Statement disclaims adoption and Owner approval. Live PR status and HTTP consumption remain unverified. |
| **7. Frozen applications** | **ACCEPT-WITH-CONCERNS** | `scripts/ci_acceptance.sh`: LedgerLens **35/35**, HealthWatch **5/29** under socket restrictions, then stopped. TaskFlow independently **32/32**. Frozen requirements and acceptance tests are byte-unchanged. M2 results remain unverified. |
| **8. CI** | **ACCEPT** | Required job name `build` preserved. Go 1.25 build/vet/test, application acceptance, full-module race detection and stable-toolchain checks are present. No newly weakened assertions or green-only skips found. Hosted execution was unavailable. |
| **9. Whole patch** | **REJECT** | Scope largely follows findings, but supervisor lifecycle complexity introduces regressions. Single-file and unconditional Linux process guarantees are inaccurate. Darwin whole-module build and both platform CLI builds pass. |

“ACCEPT-WITH-CONCERNS” on blocked gates records missing execution evidence, **not a reproduced pass**.

Additional verification:

- `go build ./...` and `go vet ./...`: **pass**.
- `go test -count=1 ./...`: observed environment failures; interrupted while app tests remained stuck.
- `go test -count=1 -timeout=120s ./...`: **fails**, including socket/Node-spawn EPERM and two application timeouts.
- `go test -race -count=1 ./internal/vm ./internal/capability`: **fails because socket creation is forbidden**.
- Targeted filesystem, containment and startup tests under `-race`: **pass**, with no race reports.

The sandbox limitations prevent full acceptance evidence; they do not account for the reproduced grant bypass, escaped descendant, or premature process termination.