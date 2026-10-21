# Milestone 2 decision

## Fresh rebuild acceptance, 2026-10-21

The requirements-first replacement is accepted for D29 A, C, D, G, H at
implementation commit `b028e813`. The canonical source matches the new reviewed
candidate; earlier Round 1–4 records below remain historical evidence, not a
substitute for this fresh acceptance.

* [Final independent review](rebuild/ticketdesk/REVIEW.md): independent source
  review, frozen 101/101, DX 13/13, retained adversarial 11/11 and actual migration.
* [Committed clean reproduction](../docs/journals/evidence/fresh_milestones/m2_clean.txt)
  and [script](rebuild/scripts/clean_repro.sh): fresh archive, empty inherited
  environment, HOME/temp/Go caches, all checks exit 0 using installed host tools.
* [Implementation journal](rebuild/ticketdesk/DEVELOPMENT_JOURNAL.md),
  [user guide and migration](ticketdesk/README.md),
  [cross-milestone verification](../docs/journals/2026-10-21_fresh_milestones.md).

Atomic ticket buckets and a complete atomic token collection replace the former
split indexes. Legacy indexes are recovered through explicit offline conversion;
old state is never silently treated as empty. Historical carryover m15/m16/m17 is
therefore superseded by the new design and framing repair. Remaining non-blocking
limits: the probe/close/bind port race, linear token-hash scans, whole-file writes,
finite startup/request budgets, transport-level net/http errors and documented
file permissions. Scope is functional standalone serving, not throughput or OS
sandboxing. Stop the milestone; no carryover implementation is required.

---

## Closure evidence index (2026-10-20)

The Round 4 acceptance below remains the independent decision for D29 A, C, D, G, H.
This closure records verification of implementation commit `7092bcff`; it does not reopen the milestone.

* Frozen scope: [plan](PLAN.md), [requirements](ticketdesk/requirements/REQUIREMENTS.md),
  [acceptance contract](ticketdesk/requirements/ACCEPTANCE.md), [acceptance runner](ticketdesk/tests/acceptance/run_acceptance.py).
* Executable evidence: [regressions](ticketdesk/tests/regression/review_round1.py),
  [DX checker](ticketdesk/dx/check_dx.py), [DX baseline](ticketdesk/dx/BASELINE.md),
  [reproduction script](scripts/clean_repro.sh).
* Records: [development journal](ticketdesk/DEVELOPMENT_JOURNAL.md#closure-verification-2026-10-20),
  [independent review](ticketdesk/REVIEW.md), [handoff](ticketdesk/HANDOFF.md).

Historical adversarial results below are preserved reviewer reports. Their temporary scripts are unavailable;
closure does not claim a fresh rerun of those probes. Historical evaluator identities are incomplete;
no cross-provider independence is claimed. Current verification is a closure check, not a new independent acceptance.
Reproduction prerequisites and isolation limits are recorded in the journal.

# ROUND 4 (FINAL), 2026-10-14, commit `7092bcff`

## Decision: ACCEPTED

Milestone 2 (D29 A, C, D, G, H) is accepted. Blockers B1 to B6 raised in rounds 1 to 3 are fixed and independently re-verified at the default
runner ceilings and the documented command. No new blocker in round 4. The milestone loop stops here; Milestone 3 may start only on a new instruction.

| Gate | Round 1 | Round 2 | Round 3 | Final (Round 4) |
|---|---|---|---|---|
| A Practical HTTP API | FAIL | FAIL | FAIL | PASS |
| C Authentication and authorization | PASS | PASS | PASS | PASS |
| D Validation and reliable errors | PASS with caveats | PASS with caveats | PASS with caveats | PASS |
| G Standalone installation and execution | FAIL | PASS | PASS | PASS |
| H Developer experience | PASS with caveat | PASS with caveat | PASS | PASS |

| Check | Result |
|---|---|
| Freeze integrity (frozen requirements, acceptance suite, DX checker, plan) | unchanged since `10d9c2a8` |
| `bash milestone2/scripts/clean_repro.sh` | 101/101 acceptance, 9/9 regression, 13/13 DX |
| `go test -count=1 ./...` | no failures, exit 0 |
| B6 kill storm (30 SIGKILLs during DELETE loops on a 640 KB state) | list always 200; no resurrected or lost tickets; 12 benign unindexed-readable tickets, all deletable |
| Dangling index id crafted into state | list 200, filtered page 200 |
| ADV01 to ADV100 reviewer harness | 76 pass; only the 4 documented invalid tests fail |
| Percent-encoding, HEAD/OPTIONS, truncated and stalled uploads | pass |

## Carryover (non-blocking, for a later milestone)

1. m12: make a lost race in the port probe print a `ticketdesk:` line.
2. m15: a migration (or startup rebuild) of the bucket index for state written by earlier builds.
3. m16: optionally rebuild the index at startup to repair unindexed tickets after a crash.
4. m17: make the truncated-upload 400 use the uniform error object and `X-Request-Id`.
5. Documentation and tooling: note that `clean_repro.sh` needs git and tar; document plain-text transport-level rejections; consider state file mode 0600 and
   an incremental store write (create latency grows with state size).
6. Keep `tests/regression/review_round1.py` in the default test run alongside the frozen suite.

---

# ROUND 3 (superseded by Round 4; kept as history)

# ROUND 3, 2026-10-14, commit `4900538a`

## Decision: NOT ACCEPTED

B1 to B5, m11, m13 and m14 are fixed and verified by independent probes at default runner ceilings. One blocker remains: B6.
The bucketed listing index is updated in a separate store write after the ticket write. A crash between the ticket delete and the
index update leaves a dangling id: listing then returns 500 permanently and the id cannot be deleted. Reproduced by a real SIGKILL storm
(3 of 25 kills during a DELETE loop). A crash between a ticket create and the index update only leaves an unindexed ticket, which is acceptable.

| Gate | Round 1 | Round 2 | Round 3 |
|---|---|---|---|
| A Practical HTTP API | FAIL | FAIL | FAIL (B6) |
| C Authentication and authorization | PASS | PASS | PASS |
| D Validation and reliable errors | PASS with caveats | PASS with caveats | PASS with caveats |
| G Standalone installation and execution | FAIL | PASS | PASS |
| H Developer experience | PASS with caveat | PASS with caveat | PASS |

| Check | Result |
|---|---|
| Freeze integrity | frozen paths unchanged: PASS |
| `bash milestone2/scripts/clean_repro.sh` | 101/101 acceptance, 8/8 regression, 13/13 DX |
| `go test -count=1 ./...` | no failures, exit 0 |
| B5 probes (3000 tickets, 2900 front deletions, middle deletions, recreate, cursor and status walks, restart, bucket boundaries, concurrency) | all pass |
| B6 crash window | FAIL (state with dangling id: list 500, DELETE 404; 3 of 25 SIGKILL runs) |
| m11, m13 | fixed; `%2F`, `%252F`, encoded dots, `%20`, unicode ids, HEAD/OPTIONS all behave |

## Carryover (required before acceptance)

1. B6: make a crash between the two writes harmless. Either remove the id from its bucket before deleting the ticket, or make the listing skip ids
   without a ticket record (and let DELETE clean the index), or keep ticket and index in one record. Add a regression test with a state containing a
   dangling bucket id (list must return 200).
2. Request a delta check of B6 only; if it passes with the full suite, record ACCEPTED and stop.
3. Non-blocking: m12 port-probe race; m15 no migration for state files written by earlier builds (they list empty); m16 unindexed ticket after a crash
   in create; m17 the m11 400 body is not the uniform error shape.

---

# ROUND 2 (superseded by Round 3; kept as history)

# ROUND 2, 2026-10-14, commit `ec9182dd`

## Decision: NOT ACCEPTED

Round 1 blockers B1 to B4 are fixed and verified by independent probes under the default runner ceilings. One new
blocker (B5) remains: listing returns a permanent 500 after about 450 consecutive deleted ticket ids under the documented
command. Details: `milestone2/ticketdesk/REVIEW.md` Round 2.

| Gate | Round 1 | Round 2 |
|---|---|---|
| A Practical HTTP API | FAIL | FAIL (B5) |
| C Authentication and authorization | PASS | PASS |
| D Validation and reliable errors | PASS with caveats | PASS with caveats |
| G Standalone installation and execution | FAIL | PASS |
| H Developer experience | PASS with caveat | PASS with caveat |

| Check | Result |
|---|---|
| Freeze integrity (`git diff 10d9c2a8 HEAD` on frozen paths) | empty: PASS |
| `bash milestone2/scripts/clean_repro.sh` | 101/101 acceptance, 5/5 regression, 13/13 DX: PASS |
| `go test -count=1 ./...` | 76 packages ok, exit 0 |
| B1 (default ceilings) | fixed: max body 4000 ok, list fast with 2000 tickets |
| B2 (busy port) | fixed: `ticketdesk: port N is already in use` |
| B3 (stalled body) | fixed: probes answered at 1/10/25 s |
| B4 (path aliasing) | fixed for `//`, `/./`, `/../`; encoded `%2F` residual (minor) |
| New: B5 listing after deletions | FAIL: 500 once about 450 consecutive ids deleted |
| New minors | m11 truncated upload executed, m12 port probe race, m13 `%2F` alias, m14 server timeouts undocumented in cli.md |

## Carryover (required before acceptance)

1. B5: make listing cost independent of deleted-id gaps (live index or skip pointers); add a default-flags regression test
   that creates 600 tickets, deletes the first 550, and lists with `limit=1` and `limit=100`; correct the README sentence.
2. Recommended: reject (400 or close) when the pre-read body hits a read error or is shorter than Content-Length (m11).
3. Recommended: compare routes with the escaped path so `%2F` is not an alias (m13); document the server timeouts in
   `docs/cli.md` (m14); make the port-probe loss of race print a `ticketdesk:` line (m12).
4. Request a delta check of B5 only. If it passes with the full suite, record ACCEPTED and stop the milestone.

---

# ROUND 1 (superseded by Round 2; kept as history)

Date: 2026-10-14. Reviewer: independent final reviewer (no implementation role).
Reviewed commit: `3b79b7a3`. Detailed evidence: `milestone2/ticketdesk/REVIEW.md`.

## Decision: NOT ACCEPTED

Milestone 2 (D29 A, C, D, G, H) is not accepted. The frozen suite passes, but the reviewer found four
blockers: a documented run configuration that returns 500 for valid input, a requirement violation on startup
failure output, a single-connection denial of service, and a route-table violation. The stop rule is not
triggered: the milestone stays open for the carryover work below and a delta review. Milestone 3 must not start.

## Evidence

| Check | Command or method | Result |
|---|---|---|
| Freeze integrity | `git diff 10d9c2a8 HEAD --name-status -- milestone2/ticketdesk/requirements milestone2/ticketdesk/tests milestone2/ticketdesk/dx milestone2/PLAN.md` | empty: PASS |
| Clean reproduction | `bash milestone2/scripts/clean_repro.sh` | 101/101 acceptance, 13/13 DX, 15.9 s: PASS |
| Acceptance, frozen suite | run by `clean_repro.sh` (suite launches with raised ceilings) | 101 passed |
| DX checker | `HF_BIN=<fresh build> python3 milestone2/ticketdesk/dx/check_dx.py -v` | 13 passed |
| DX and guide Go tests | `go test -run 'DeveloperExperience\|HTTPAPIGuideRule' .` | ok |
| Full regression | `go test -count=1 ./...` | see "Final regression" below |
| Adversarial checks | about 110 reviewer-written black-box cases (ADV01 to ADV100) | 4 real failures: B1, B2, B3, B4 |
| Constant-time compare, CSPRNG (V1, V2) | source inspection | PASS |
| No generated server (V3) | run command is `howlframe run <.hfbc>` only | PASS |
| CLI docs (V4) | `docs/cli.md` | flags documented; memory line stale (minor) |
| Runtime changes | diff review plus hands-on attenuation test | no escalation found; per-request budget needs documenting |

## Gate outcomes

| Gate | Verdict |
|---|---|
| A Practical HTTP API | FAIL (B1, B4) |
| C Authentication and authorization | PASS |
| D Validation and reliable errors | PASS with caveats |
| G Standalone installation and execution | FAIL (B1, B2) |
| H Demonstrably improved developer experience | PASS with caveat (guide run command inherits B1) |

## Blockers

* B1: with the documented command (default `--max-instructions 100000`) a valid ticket body of 2857 to 4000
  characters returns 500 and, once 100 tickets exist, every list call returns 500. The suite masks this with
  `--max-instructions 4000000000`.
* B2: busy port prints a raw `{"phase":"runtime",...}` object and exits 1, violating R1.3.
* B3: one authenticated member/admin connection with a stalled body freezes every request (global serialize
  lock held across the body read, no server read timeout).
* B4: `//v1/tickets`, `/v1//tickets`, `/v1/tickets/.` and dot-segment variants are served instead of 404
  (R3.1), because routing uses `path.Clean`.

## Carryover work (required before re-review)

1. Fix B1: reduce per-request instruction cost or raise the default for HTTP request VMs; at minimum document the
   required `--max-instructions`/`--max-memory-bytes`/`--max-call-depth` in README, the guide and the launcher
   example. Add a default-flags smoke test: 4000-character body, at least 150 tickets, list with `limit=1`.
2. Fix B2: report bind failure as `ticketdesk: ...` with no raw object; add a new test (the frozen one cannot be
   edited) that asserts the stderr shape for a busy port.
3. Fix B3: add read/header/write timeouts to the HTTP server and/or read the body before taking the serialize
   lock; add a slow-body regression test.
4. Fix B4: route on the raw escaped path (or reject paths that change under cleaning); add tests for `//`, `/./`
   and `/../` variants.
5. Minor: update `docs/cli.md` memory line (per-request budget, no process-wide bound), correct the README
   "thousands of tickets" claim (measured 3.5 s per list at 1000 tickets), make the guide's body example catch
   `LIMIT_EXCEEDED` and answer 413, validate state shape at startup, consider directory fsync, strengthen tests
   for `inherit:filesystem` (coarse grant) and `granted` (`network:fetch=` exclusion), document transport-level
   rejections that bypass the error shape, note git/tar in `clean_repro.sh`.
6. Request a delta review. Do not edit the frozen requirements, tests or DX checker; add new tests alongside.

## Final regression

`go test -count=1 ./...` run after the two review files were written (working tree = HEAD plus these two
files only): 76 packages `ok`, no failures, exit 0. The review files are the only changes to the tree, so this
result corresponds to the final working-tree state.
