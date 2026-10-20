# Ticketdesk B6 Round 7 isolation rerun, 2026-10-20

This was the one isolation rerun allowed by the Round 7 rule after the 2026-10-19 INFRA-VOID (#201).

Venue: William (Owner) chose this shared box in an overnight quiet window over a non-shared machine, because he chose
not to spend money on extra compute. He knowingly waived the "non-shared machine with no VM snapshot or fork, no
active balloon driver" term. Every other term was kept. The waiver and the pre-launch venue evidence were written
before launch. The evidence shows a virtio guest with `virtio_balloon` bound, 8 CPUs, 16 GB RAM, kernel `6.12.94+`, and
six `virtual machine fork` dmesg records between 00:08 and 01:04 EDT that night.

A fresh Codex CLI session with no implementation role checked the pins: detached HEAD `7e9ef32e`, single parent
`e8fcea20`, clean tree, script sha256 `540c2433…3d38`, `CHECK_HEAD PASS`, artifacts path absent, no storm running. It
then launched the registered storm exactly once, detached with setsid/nohup and with the exit status captured, at
01:07:09 EDT: `--runs 225 --expect-sha256 540c2433…3d38 --expect-head e8fcea2024b42bbfafd6e4410568c76d32e9c37a
--artifacts /tmp/round7iso/artifacts`. The 120 s idle gate passed. The Eng Mgr session only waited on the output. No
restart.

Verdict: INFRA-VOID (not ACCEPTED). SUMMARY runs=225 failures=0 orphan_runs=106 survivor_GETs=331903
survivor_DELETEs=331903; crafted probes=2 PASS; noise=INFRA-VOID breaches=16. VERDICT INFRA-VOID (one isolation rerun
allowed, same pins). `EXIT=1` captured, via `AssertionError: noise gates breached: INFRA-VOID`.

All 16 breaches sit in one cluster during run 169's recovery. Runs 1–168 were quiet.

* a stall-type gap, 6.98 s mono / 7.00 s real / 7.00 s boot, at 01:59:42 EDT;
* a freeze-type gap, 0.49 s mono / 19,610.7 s real / 0.46 s boot. The guest clock stopped from about 01:59:49 to
  07:26:40 EDT;
* then one `virtual machine fork` record and 13 `virtio_balloon` "Out of puff" records.

Steal (max 0.098% over 60 s), PSI, throttling and sampler coverage stayed inside their gates. This is consistent with
a host-side snapshot or suspend of the box, but doesn't prove it.

One request overlapped the freeze: run 169 recovery `GET t622`. It took 7.76 s monotonic and about 19,618 s on the
wall. The server handled it in 0.25 ms (lock wait 0.002 ms), and it returned the expected 404 and passed. Heartbeat
gaps cover 96% of its monotonic time. On wall time it would have been an `infra`-classified deadline failure, giving
NOT_ACCEPTED_INFRA. The outcome is the same either way.

After the storm, `bash milestone2/scripts/clean_repro.sh` passed (101/101, 10/10, 13/13, exit 0). Frozen paths are
unchanged (`git diff 10d9c2a8 HEAD` on the registered paths is empty). Small text evidence is in
`milestone2/ticketdesk/evidence/round7_isolation_2026-10-20/`. Raw artifacts (806 MB) stay in
`/tmp/round7iso/artifacts`.

Outcome: this second INFRA-VOID spends the only pre-registered isolation rerun. B6 remains open, Milestone 2 is not
accepted, and there is no further registered run. The next step is William's decision. No product, script or frozen
files changed.
