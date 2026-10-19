# Ticketdesk B6 Round 7, 2026-10-19

A fresh Codex CLI session with no implementation role checked the pins (exact HEAD `7e9ef32e`, single parent
`e8fcea20`, script sha256 `540c2433…3d38`, `CHECK_HEAD PASS`, artifacts path absent) and launched the registered
storm exactly once, detached with setsid/nohup, at 12:55:46 EDT: `--runs 225 --expect-sha256 540c2433…3d38
--expect-head e8fcea2024b42bbfafd6e4410568c76d32e9c37a --artifacts /tmp/round7/artifacts`. The 120 s idle gate passed.
The Eng Mgr session only polled the output sparingly while it ran; no builds, tests or storms ran alongside it.
No restart.

Verdict: INFRA-VOID (not ACCEPTED). SUMMARY runs=225 failures=0 orphan_runs=94 survivor_GETs=332155
survivor_DELETEs=332155; crafted probes=2 PASS; noise=INFRA-VOID breaches=16. VERDICT INFRA-VOID (one isolation rerun
allowed, same pins). Exit 1 via `AssertionError: noise gates breached: INFRA-VOID`.

The breaches come from two VM-fork pauses of the shared box. About 13:51 EDT (run 168 recovery) there were heartbeat
gaps of 2.8 s and 7.6 s monotonic / 48.7 s realtime, a `virtual machine fork` record and ten `virtio_balloon`
records. About 14:05 EDT (runs 208–209) there were gaps of 13.5 s and 2.7 s monotonic / 74.1 s realtime and a second
`virtual machine fork` record. Steal, PSI, throttling and sampler coverage stayed inside their gates.

Two requests overlapped the pauses and still passed: run 168 DELETE t997 (8.30 s monotonic, ~49.5 s wall; lock wait
0.002 ms, store write 187 ms) and run 209 DELETE t487 (3.05 s monotonic, ~74 s wall). This supports the box-pause
reading of the Round 6 timeout but proves nothing about it. Observation for the rerun protocol: the 10 s total deadline
is measured on CLOCK_MONOTONIC, which mostly stops while the VM is frozen. A frozen-VM stall therefore shows as
realtime ≫ monotonic in the heartbeat log, not as a deadline failure.

After the storm, `bash milestone2/scripts/clean_repro.sh` passed (101/101, 10/10, 13/13, exit 0). Frozen paths
unchanged. Small text evidence is in `milestone2/ticketdesk/evidence/round7_2026-10-19/`. Raw artifacts (805 MB)
stay in `/tmp/round7/artifacts`.

Next steps: B6 remains open. The pre-registered rule allows exactly one isolation rerun with the same pins on an
isolated machine that is not snapshotted or ballooned. Running it again on this shared box would likely hit the same
VM forks. No product, script or frozen files changed.
