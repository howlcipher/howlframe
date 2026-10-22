# HowlFrame Remediation Journal

Branch `fix/post-audit-stabilization`, base `5d2983b2c2311daf9b7494311ad719ff5dc51c09`
(== origin/main at start; 0 commits landed since the audit; CI on base: success).
Toolchain: go1.26.0 local, Python 3.14.4. Audit inputs: /run/media/system/tallgeese/dev/howlframe-independent-audit-20261021/ (unchanged).

## 2026-10-21 Phase 0
- Verified baseline == audited commit; worktree created at dev/worktrees/howlframe-post-audit-remediation.
- Design decision HF-01: use `os.Root` (Go 1.25: needs Root.Rename/Chmod/MkdirAll). Rejected: hand-written
  openat walkers per platform (no Windows/macOS parity, release builds ship both), and staying on Go 1.21
  (no race-free primitive). `go 1.25` in go.mod; `GOTOOLCHAIN=auto` fetches it for 1.21+ users.
- Design decision HF-02: Linux per-exec supervisor acting as a subreaper (cgroup delegation and user namespaces
  are not reliably available; probed on this host: cgroup not writable). Other OSes: process group only, documented.

## HF-01 filesystem containment
- Red first: `internal/vm/filesystem_race_test.go` (read, write, mkdir, file store while a symlink inside the root flips
  between an in-root dir and an outside dir). Baseline: all four FAIL (read escaped 2063/4000, write/mkdir/store landed
  outside). Log: evidence/hf01_baseline_red.log.
- Fix: `internal/capability/scopedfs.go` (`OpenScope`, `Scope`, `FileSystem`), `matchRoot` split from `allows` (decision logic
  unchanged), VM/interpreter read/write/mkdir and file-backed store open/persist routed through the root handle.
- Behavior preserved on purpose: missing granted root => ordinary I/O error (M2 R1.3 exit 66); mkdir under a missing granted
  root still creates it; coarse `filesystem` alias stays unrestricted.
- Behavior change (documented, fail closed): an absolute symlink inside the root is refused even if it points back in;
  relative in-root links work. Test: TestAbsoluteInRootSymlinkFailsClosed.
- Side effect of go 1.25: new vet check flagged a real latent bug (`"undefined var: "+bodyVar` as a format string); fixed.
- Test setup change (not assertion): store_timing tests construct a bare BCVM; they now carry a coarse filesystem grant
  because persistence is authorized through the VM's grants.
- Green: internal/vm and internal/capability pass; race tests 15x stable and clean under -race.

## HF-02 / #181 exec descendant containment
- Red first: `internal/vm/exec_containment_linux_test.go` (normal exit, deadline, output-limit; each with foreground,
  background and session-escaped (setsid) sleepers found by unique /proc cmdline markers, killed in t.Cleanup).
  Baseline: all three FAIL, 7 survivors. Log: evidence/hf02_baseline_red.log.
- Fix (Linux): `internal/vm/exec_contain_linux.go`. Each exec runs the program under a supervisor (re-exec of the runner
  binary, `--howlframe-exec-supervisor-v1`, handled in package init) that is a PR_SET_CHILD_SUBREAPER. Runner owns the
  output pipe and waits for its EOF (so the old "wait for processes still holding the output" behavior is unchanged), then closes a
  control pipe; the supervisor kills and reaps every remaining descendant (loops, because killing a parent re-parents
  its children to the subreaper). Deadline, output limit, or runner death close the control pipe at once.
- First attempt killed leftovers immediately on direct-child exit; that would have changed the pinned default behavior
  (`TestExecWithoutDeadlineWaitsForInheritedOutput`), so it was restructured as above. A second bug (control pipe closed
  before the supervisor observed a normal exit => bogus status 137) was fixed with a 100 ms grace; found by the existing
  exec tests, not by new ones.
- Other Unix: process group kill only. Other OSes: direct child only. Both documented as NOT covering session-escapers.
- Green: TestExec* x10 stable, no leftover processes; -race clean; TaskFlow frozen 32/32; the audit's tf_probe (which FAILED
  exec_deadline_kills_descendants) now 7/7.

## HF-03 TicketDesk restart cliff
- Cause (confirmed): startup code of an HTTP server shares ONE 100,000-instruction budget for the whole run (handlers each
  get a fresh one). TicketDesk's startup validates every stored record (~330 instructions/ticket), so ~300 tickets exhaust it.
  Making validation cheaper cannot reach 1,000 tickets inside 100,000 instructions (<100/ticket) and weakening validation is
  excluded, so the narrow fix is a separate startup budget.
- Red first: `milestone2/ticketdesk/tests/regression/restart_default_budget.py` (documented command, default ceilings, 1,000
  tickets + 100 tokens, restart, data/token/authz verification, invalid-state rejection without damage, SIGKILL storm).
  Baseline: restart fails with LIMIT_EXCEEDED (log evidence/hf03_baseline_red.log).
- Fix: `VMLimits.MaxStartupInstructions`, `BCVM.startup`, `instructionLimit()`; `run --max-startup-instructions`
  (default 10,000,000 only for HTTP-server programs and only when --max-instructions is not given explicitly). Non-server
  programs and request handlers are unaffected (unit-tested). Interop descriptor updated.
- Result: restart with 1,000 tickets/100 tokens 0.3 s; default-budget restart verified at 6,000 tickets / 2.2 MB
  (evidence/hf03_sweep.log). 30,000 is an extrapolation and labelled as such in the README.
- Side finding kept in the follow-up register (not fixed): the failure surfaced as a raw runtime JSON object on stderr (HF-12).

## HF-04 slow-reader stall
- Cause (confirmed in code and by test): with `(serialize_requests)` the handler lock was held until the response was
  written to the client, and the server had no write timeout.
- Red first: `internal/vm/slow_reader_test.go` (in-process server on a free loopback port; client pipelines twelve 2 MiB
  responses and never reads; `/health` must answer within 1.5 s; stalled connection must be released after the write
  timeout without delivering everything; a normal 2 MiB response is still intact). Baseline: health blocked.
- Fix: in serialized mode the handler writes to an in-memory `bufferedResponse`; it is flushed to the client AFTER the lock is
  released, under a per-flush write deadline (`responseWriteTimeout`, 30 s). Without the lock, writes get the same deadline.
  Helpers live in `internal/vm/http_response.go` (see below).
- Test-authoring mistakes caught on the way: (1) my first drain step ran before the deadline expired so the server correctly
  delivered everything; the test now outlasts the timeout and asserts the response was cut short. (2) a 4 KB client receive
  window made draining take minutes; widened before the drain.
- `TestEffectGateSourceConformance` flagged `http.NewResponseController` reachable from the `res` opcodes. Not new
  authority, but the scanner cannot tell; the helper moved to another file (the test documents that other files are out of
  scope). The conformance test itself was NOT changed.
- Green: slow-reader test 3x; the audit's own td_slowreader.py now shows health 200 in 0.0 s with a stalled client (it timed
  out at 60 s in the audit); vm/capability pass under -race; M2 frozen 101/101, DX 13/13, reassessment 11/11, review_round1 10/10.

## Review round 1 (Codex, independent provider) and repairs
- Fixes up to this point were committed; full runs were green (Go 1.25.0 minimum: 76 packages; race: 76 packages, 0 races) after
  one self-found regression: go-target scratch modules declared `go 1.21` (fixed; found by the first full race run, not
  by the suites I had been running).
- Self-found regression, earlier: single-file grants broke the projector (found by re-running the HowlBoard consumer check).
- Reviewer verdict: REJECT, three reproduced defects (the reviewer's sandbox forbade sockets, so gates 3-5 and 7-M2 were not executed):
  1. HIGH single-file grant reached a sibling through a relative symlink. I reproduced it, wrote red tests, and fixed it
     with pinned opens (Lstat, SameFile check, exclusive create, deferred truncate). Race test extended; verified it fails
     without the fix (114 leaks) and passes with it.
  2. HIGH a program can kill its own supervisor and leave detached descendants. Reproduced in concept; NOT fixed. A sound
     mitigation needs the runner to be a second-level subreaper plus a registry of its children; judged out of proportion,
     so the guarantee was narrowed in the docs instead (protects against runaway descendants, not hostile code).
  3. MEDIUM a program that closes its own output and keeps running was killed (exit 137 after ~200 ms instead of 7 after
     500 ms): output EOF was treated as cancellation. Reproduced; fixed by an explicit exit report on the status pipe.
  4. LOW (pre-existing) `GOOS=windows go build ./...` fails in internal/testutil (syscall.Flock). Unchanged from baseline,
     not in scope; the CLI itself builds for Windows.

## Review round 2, hosted CI, and one more repair
- Codex hit its usage limit before a socket-enabled rerun; round 2 was run on Antigravity (`agy`) against `3832d83e`: ACCEPT. Its
  report misnames some code (`openPinnedFile`, `O_NOFOLLOW`, test names that do not exist), so I ran its two probe scripts
  against my own build of the final commit rather than trusting the prose: all pass.
- Closed a gap: ran the full EvidenceDesk workflow (real Chromium, both Board fixtures) on the final tree: pass.
- PR #209 opened. First hosted run (`7823c901`): `build` FAILED, other three jobs passed. Cause: my own new slow-reader test, in
  its drain step (tiny client receive buffer made draining crawl on the 4-core runner); the server behavior it tests was fine.
  Reproduced the conditions by pinning the full suite to 4 cores, redesigned the test (default buffers, 80 MB pipelined, so
  blocking is certain and draining fast), 76/76 packages pass pinned to 4 cores. Second hosted run (`e472a928`): all four jobs pass.
  Lesson recorded: a green run on a 12-core host did not prove a test was load-independent.
