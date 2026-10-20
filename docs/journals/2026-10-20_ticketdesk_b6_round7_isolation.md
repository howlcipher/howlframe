# Ticketdesk B6 Round 7 isolation rerun, 2026-10-20

This run followed the 2026-10-19 INFRA-VOID (#201). It was launched as the isolation rerun, at a waived venue; whether it counts is William's decision.

## Venue

On 2026-10-19 the cloud agent VM launch failed for lack of usage (4:32–4:35 PM EDT), and William declined to spend
money. At 4:36 PM EDT he answered in the Grok Bot chat with Rintaro: "Retry the rerun on this box in a quiet window
tonight". That knowingly waived the "non-shared machine with no VM snapshot or fork, no active balloon driver" term.
Every other term was kept.

**The isolation condition was NOT met.** This was the same shared, ballooned, fork-prone box as Round 7. The pre-launch
venue evidence shows a virtio guest with `virtio_balloon` bound, 8 CPUs, 16 GB RAM, kernel `6.12.94+`, and six
`virtual machine fork` dmesg records between about 00:08 and 01:04 EDT. Those `dmesg -T` wall times are approximate
after pauses. The declared quiet window ended at 06:00 EDT.

## The run

A fresh Codex CLI session with no implementation role checked the pins: detached HEAD `7e9ef32e`, single parent
`e8fcea20`, clean tree, script sha256 `540c2433…3d38`, `CHECK_HEAD PASS`, artifacts path absent, no storm running. It
then launched the registered storm exactly once, detached with setsid/nohup and with the exit status captured, at
01:07:09 EDT: `--runs 225 --expect-sha256 540c2433…3d38 --expect-head e8fcea2024b42bbfafd6e4410568c76d32e9c37a
--artifacts /tmp/round7iso/artifacts`. The 120 s idle gate passed. No restart.

## Verdict

INFRA-VOID (not ACCEPTED). SUMMARY runs=225 failures=0 orphan_runs=106 survivor_GETs=331903
survivor_DELETEs=331903; crafted probes=2 PASS; noise=INFRA-VOID breaches=16. VERDICT INFRA-VOID (one isolation rerun
allowed, same pins). `EXIT=1` captured, via `AssertionError: noise gates breached: INFRA-VOID`.

All 16 breaches sit in one cluster during run 169's recovery. Runs 1–168 were quiet.

* a stall-type gap, 6.98 s mono / 7.00 s real / 7.00 s boot, at 01:59:42 EDT;
* one freeze-type gap, 0.49 s mono / 19,610.7 s real / 0.46 s boot. The guest clock stopped from about 01:59:49 to
  07:26:40 EDT;
* then one `virtual machine fork` record and 13 `virtio_balloon` "Out of puff" records.

The run resumed at 07:26:40 and ended at 07:44:44 EDT, outside the declared quiet window. Steal (max 0.098% over 60 s),
PSI, throttling and sampler coverage stayed inside their gates. This is consistent with a host-side snapshot or suspend
of the box, but doesn't prove it.

One request overlapped the gaps: run 169 recovery `GET t622`. It took 7.76 s monotonic, a 2.24 s margin to the 10 s
deadline. Most of that time (6.98 s) fell in the stall-type gap, where all clocks were running. On the wall it took
about 19,618 s. The server handled it in 0.25 ms, and it returned the expected 404 and passed. On wall time it would
have been an `infra`-classified deadline failure.

After the storm, `bash milestone2/scripts/clean_repro.sh` passed (101/101, 10/10, 13/13, exit 0). Frozen paths are
unchanged. The Round 7 pin tests pass. Small text evidence is in
`milestone2/ticketdesk/evidence/round7_isolation_2026-10-20/`. Raw artifacts (806 MB) stay in
`/tmp/round7iso/artifacts`.

## Outcome

B6 remains open and Milestone 2 is not accepted. Because the venue term was waived, whether this invocation counts as
the one pre-registered isolation rerun is William's decision. No product, script or frozen files changed.

## Next step (Owner decision, 2026-10-20)

This is William's decision, not a recommendation. At 7:57 AM EDT he wrote in the Grok Bot chat: "Can we note this in
the repo and I'll run this and get findings on my pc."

* **What:** he will run the B6 kill storm himself, once, on his own PC.
* **Why:** the box is a shared VM whose logs show VM-fork and balloon records and freeze-type gaps. Round 7 and this
  rerun passed 450/450 runs, but both were voided by box freezes.
* **Command:** the identical pinned one: detached `7e9ef32e`, script sha `540c2433`, `--expect-head e8fcea20`, EXIT
  captured. An `IDLE_GATE` exit 3 is not an attempt. One attempt only.
* **Gates:** the run must pass the same 120 s idle gate and the same noise gates as this record.
* **Venue evidence before launch:** it is recorded before launch: bare metal vs VM/WSL2, any balloon or dynamic memory,
  sleep disabled, other workloads stopped, and `/dev/kmsg` readable. Whether the run counts as isolated is decided by
  that evidence.
* **Status:** this is a new Owner-authorized venue run, separate from the pre-registered isolation rerun. Its result
  will be recorded and reviewed separately, under the same decision rule, by a fresh reviewer. M2 stays not accepted
  until then.
* **Superseded 2026-10-20 11:24 EDT:** William answered "Waive the B6 venue requirement and accept M2". The PC run
  will not be done; see `docs/journals/2026-10-20_m2_acceptance.md`.
