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

The 16 breaches cluster around two VM-fork events. That is consistent with host-side VM pauses but does not prove
them.

* **About 13:51 EDT (run 168 recovery):**
  * a stall-type gap, 2.8 s with equal mono/real/boot clocks;
  * a freeze-type gap, 7.6 s mono / 48.7 s real;
  * a `virtual machine fork` record and ten `virtio_balloon` records.
* **About 14:05 EDT (runs 208–209):**
  * a stall-type gap, 13.5 s with equal clocks and no request in flight;
  * a freeze-type gap, 2.7 s mono / 74.1 s real;
  * a second `virtual machine fork` record.

The stall-type gaps came 13 s and 23 s before the fork records, with steal around 0. Steal, PSI, throttling and
sampler coverage stayed inside their gates.

Two requests overlapped the freezes and still passed. The 10 s deadline is monotonic by the pinned script
(`Server.expect`, lines 286/295/308, checked at 299/309), not a choice made after the run. Monotonic advanced only
partly during the freezes: 16% (7.58 of 48.75 s) and 4% (2.72 of 74.09 s).

* **Run 168 DELETE t997:** 8.30 s mono, ~49.5 s wall. The lock was acquired in 0.002 ms and held through the ~8 s
  stall, which fell before the 187 ms store write.
* **Run 209 DELETE t487:** 3.05 s mono, ~74 s wall. The lock was held, and the stall fell inside the store write's
  marshal phase (2902 ms).

Counterfactual: on wall time both would have been deadline failures. The pinned classify() rates both `infra` (gap
cover 91%, or 94% counting an adjacent 0.26 s gap, and 89%). The verdict would then be NOT_ACCEPTED_INFRA, with the
same one rerun. This adds weight to the box-pause reading of the Round 6 timeout but doesn't prove it.

After the storm, `bash milestone2/scripts/clean_repro.sh` passed (101/101, 10/10, 13/13, exit 0). Frozen paths
unchanged. Small text evidence is in `milestone2/ticketdesk/evidence/round7_2026-10-19/`. Raw artifacts (805 MB)
stay in `/tmp/round7/artifacts`.

Next steps: B6 remains open. The pre-registered rule allows exactly one isolation rerun:
* a non-shared machine with no VM snapshot or fork and no active balloon, ~1.5 GB of free disk, and a declared quiet
  window;
* a detached checkout of exactly 7e9ef32eb2eb6c71f7e0fe0280ed3240e1842989 (main's tip after this record would fail
  --expect-head);
* the identical command (--runs 225 --expect-sha256 540c2433…3d38 --expect-head e8fcea20…) with a fresh artifacts
  directory;
* the exit status captured with `setsid nohup sh -c 'python3 … ; echo "EXIT=$?"' > …/kill_storm.txt 2>&1 < /dev/null &`;
* a fresh independent reviewer session;
* one attempt only. An IDLE_GATE FAIL (exit 3) is not an attempt; every other exit is.

The shared box was allowed only with a declared quiet window. Whether other agents were idle can't be checked from
the artifacts. No product, script or frozen files changed.
