# HowlFrame post-audit stabilization: remediation plan

Mission source: "HOWLFRAME — POST-AUDIT STABILIZATION MISSION". Audit inputs (read, unchanged):
`/run/media/system/tallgeese/dev/howlframe-independent-audit-20261021/` (six reports, SHA-256 recorded in the journal).
This file is the plan as it stood after Phase 0, with outcomes added so it can be read against the result.

## 1. Verified baseline (Phase 0)

| Item | Value |
|---|---|
| Default branch HEAD (fetched) | `5d2983b2c2311daf9b7494311ad719ff5dc51c09`, 2026-10-20T23:08:50-04:00 |
| Commits on main since the audited baseline | **0** (so every audit finding applied to current main) |
| Working tree | clean; work done in a new worktree, branch `fix/post-audit-stabilization` |
| CI on baseline | run `39536960740` succeeded (the pipeline did not run any accepted application suite) |
| Toolchain | go1.26.0, Python 3.14.4 (CI used Go 1.21; `go.mod` said 1.21) |
| Other worktrees | several unrelated agent worktrees exist; none touched |
| Open upstream PRs of note | #206 (M3 spec freeze, draft), #178 (live-v1 harness); neither touched |

Each finding was reproduced or re-derived before design: the audit's own reproductions were rerun where safe, and each
fix was preceded by a failing regression test (red-green; baseline logs in `evidence/`).

## 2. Workstreams, decisions, tests, exit criteria

| ID | Defect (cause confirmed) | Design chosen (alternatives rejected) | Files | Regression tests | Exit criterion | Outcome |
|---|---|---|---|---|---|---|
| HF-01 | Scoped grants checked on a symlink-resolved path, then opened by name: a swapped path component redirects the I/O | `os.Root` opened on the granted directory; existing decision logic untouched (`matchRoot`). Single-file grants open the parent directory. *Rejected:* hand-written `openat` walkers per platform (no Windows/macOS parity, release ships both); staying on Go 1.21 (no race-free primitive). Raises `go.mod` to 1.25 after weighing it (see security doc) | `internal/capability/{scopedfs,grants}.go`, `internal/vm/vm.go`, `go.mod` | `filesystem_race_test.go`, `scopedfs_test.go` | No escape in bounded concurrent tests; existing capability tests pass | Met |
| HF-02 / #181 | `exec.CommandContext` kills only the direct child; background and `setsid` descendants survive | Linux: per-exec supervisor that is a child subreaper, sweeping the whole tree on end, deadline, output-limit or runner death. Default wait-for-output behavior kept. Other Unix: process group. Others: direct child, documented. *Rejected:* process group alone (misses `setsid`), cgroups (not delegated here), PID namespaces (policy-dependent), killing leftovers immediately (changes pinned default behavior) | `internal/vm/exec_contain_*.go`, `vm.go` | `exec_containment_linux_test.go` | No survivors on normal exit, deadline, output-limit; exec tests and TaskFlow 32/32 pass | Met on Linux; documented limits elsewhere |
| HF-03 | Server startup shares one 100,000-instruction budget with the whole run; TicketDesk validates every record at start (~330 instr/ticket) | Separate startup budget for HTTP-server programs only (`run --max-startup-instructions`, default 10,000,000 unless `--max-instructions` is explicit). *Rejected:* cheaper validation alone (cannot reach 1,000 tickets in 100,000 instructions; weakening validation is excluded), raising every limit globally, a new language form | `internal/vm/{error,vm}.go`, `howlframe.go`, docs | `restart_default_budget.py`, `startup_budget_test.go`, `startup_instructions_cli_test.go` | 1,000 tickets + 100 tokens restart under the documented command; SIGKILL, authz, invalid-state and migration behavior preserved | Met |
| HF-04 | Serialization lock held through response write; no write timeout | Buffer the response under the lock, flush after release under a write deadline; same deadline for unserialized handlers. *Rejected:* a server-wide `WriteTimeout` (would also cap time spent queued behind the lock) | `internal/vm/{vm,http_response}.go` | `slow_reader_test.go` | A non-reading client cannot stall unrelated requests; large valid responses intact; no race | Met |
| HF-08 | Replacement dropped the port number from the port-in-use diagnostic (original printed it) | Restore the original message in the app source (both byte-identical copies). The m12 lost-race XFAIL is a documented waiver with a different cause; left as a recorded expected failure | `milestone2/{,rebuild/}ticketdesk/src/ticketdesk.howl` | `assurance_m10.py` (existing) | m10 passes except the documented XFAIL | Met (m12 deferred) |
| HF-06 | M3 compatibility evidence built on a stale HowlBoard checkout; addendum approved by the implementing side | Rerun against current upstream; add a dated correction beside, not over, the historical records; list Owner decisions without deciding | `milestone3/EVIDENCE_STATEMENT_2026-10-21.md` | `evidence/hf06_*.log` | Accurate current comparison; history preserved; no fabricated approval | Met |
| HF-09 | CI ran none of the accepted application suites; no race job; toolchain mismatch (1.21 vs 1.26) | `scripts/ci_acceptance.sh` plus CI jobs `acceptance`, `race`, `latest-go`; required `build` job keeps its name, moves to the `go.mod` minimum. #192 fixed in the three tests. M3 browser checks stay manual | `.github/workflows/*`, `scripts/ci_acceptance.sh`, 3 test files | CI itself | Suites run on PRs and main | Met locally; first CI run recorded in the regression results |

## 3. Dependencies and order
HF-01 (go.mod bump) first because every later job inherits the toolchain. HF-02 and HF-03/04 are independent. HF-08 after
HF-03 (shares the TicketDesk source). HF-06 reuses the HF-01 build and exposed a regression in it (single-file grants;
fixed with its own tests). HF-09 last so CI encodes the final set of suites.

## 4. Out of scope (recorded, not fixed)
HF-05 (checker stack effects), HF-11 / #182 (env allowlist), HF-12 (raw limit diagnostics), HF-13 (file modes), HF-14
(live-memory bound), HF-20 (loopback default), #147, the m12 lost-race XFAIL, and the remaining audit roadmap items.
No proven dependency required touching any of them. See the follow-up register in `HOWLFRAME_FINAL_HANDOFF.md`.
