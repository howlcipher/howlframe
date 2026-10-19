# Ticketdesk B6 Round 5 independent delta check, 2026-10-19

Tested commit `f7cc2a8`. Reviewer: independent final reviewer (fresh Codex CLI session;
no implementation role; not the Round 4 session). Reviewer verdict: ACCEPTED; **reclassified NOT ACCEPTED** (see the addendum).

Read AGENTS.md, both Round 4 review sections, and the Round 4 journal. Independently
reviewed `git show HEAD`: the storm instrumentation adds request/process/box diagnostics;
pass criteria, seed, ticket/run counts, kill delays and existing timeouts are unchanged.
Also inspected the B6 product delta: listing guards missing records and deletion unindexes first.
No product or script edits were made. Frozen-path diff against `10d9c2a8` is empty.

Ran `python3 milestone2/ticketdesk/evidence/b6_kill_storm.py` unchanged and alone:
exit 0, 25/25 PASS, 0 failures, both crafted probes PASS. Seed: 1500 HTTP-created tickets,
504724-byte snapshot, default runner ceilings. All 36,912 survivor GETs and 36,912
survivor DELETE checks passed; 14 runs had readable unindexed survivors. Crafted dangling
entries across bucket boundaries were harmless; the opposite unindexed window passed.

Diagnosis: no timeouts and no DIAG blocks in Round 5. No targeted rerun needed.
Round 4's timeout remains historically unclassified; this run does not prove a box or server cause.

After the storm, ran `bash milestone2/scripts/clean_repro.sh`: exit 0,
101/101 acceptance, 9/9 regression, 13/13 DX. Then ran `go test -count=1 ./...`:
exit 0, 76 packages ok, no failing package. Full storm and Go output plus clean-repro
counts are under `milestone2/ticketdesk/evidence/round5_2026-10-19/`.

B6 is cleared and the milestone stop rule is satisfied. Non-blocking m12, m15–m18
remain (port-probe race, earlier-state migration, unindexed readable records, truncated
upload error shape, persistent harmless dangling entries). No new product bug found;
no further B6 work required. Recorded Round 5 atop both decision/review files and retained
Round 4 as superseded history. No push, PR or merge.

## Addendum (Eng Mgr, 2026-10-19, after Motoko's review of #189)

Round 5 is reclassified **NOT ACCEPTED: clean rerun, cause unknown.** A 25/25 rerun doesn't clear an
undiagnosed 1-in-25 timeout: a 4% flake passes 25/25 about 36% of the time. The only lead is that Round 4 run 24 was
itself the 11th run with an unindexed survivor. A candidate root cause is filed as #190: `store.mu` is taken without `defer`
in `store_put`/`store_delete` in `internal/vm/vm.go`, so a panic leaks the lock and every later store request hangs.

Done in #189 after the reclassification:
* `b6_kill_storm.py` (commit `a2d8534`): per-run server logs kept under `--artifacts`, SIGQUIT goroutine dump before the
  kill on a timeout, state file plus `*.tmp-*` copied on failure, per-run p99/max latency, and `--runs`. Pass criteria,
  seed, delays and the 10 s timeout are unchanged. A 2-run smoke passed, and the diag path was exercised against a real server.
* Fixed regression test `b6_run24_unindexed_survivor_then_cleanup_deletes` (10/10 regression locally).
* Round 6 pre-registered in the decision doc: 0 failures in 75 or more runs, script and timeout frozen. **Not run yet.**

Next: fix #190 (unlock with defer and add a panic-in-request regression test), then have a fresh independent session
run Round 6 exactly as registered.
