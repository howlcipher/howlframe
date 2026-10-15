# Milestone 2 decision

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
