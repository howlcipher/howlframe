# Milestone 2 decision

# MILESTONE 2 ACCEPTED BY THE OWNER (2026-10-20, 11:24 EDT), with a B6 venue waiver

## Decision: M2 ACCEPTED by Owner decision; B6 gate waived for M2 (no B6 ACCEPTED verdict)

This is William's (Owner) decision, recorded as given. It is not a mechanical verdict of the B6 script, and it does
not change the INFRA-VOID verdicts recorded below.

* **Context.** William could not run the B6 kill storm on his PC because he is not at home, and asked in the Grok Bot
  chat with Rintaro: "Can we skip this or work around?". At 11:24 AM EDT on 2026-10-20 he answered: "Waive the B6 venue
  requirement and accept M2".

### Waiver W-B6-venue

* **What is waived.** The B6 kill-storm isolation/venue requirement: an isolated, non-shared venue with no VM snapshot
  or fork and no active balloon driver (the Round 7 isolation rerun terms), or the Owner-authorized PC run in
  "Next step (Owner decision, 2026-10-20)" below. The waiver **waives the B6 gate for M2** on the evidence below,
  without an isolated-venue run.
* **Scope.** Milestone 2 only, for code pin `7e9ef32eb2eb6c71f7e0fe0280ed3240e1842989` (Round 7 code `e8fcea20`,
  script sha256 `540c243325c7b9d5439f0c592e04354ec9937ce98ec8beab760874671bd83d38`). It does not carry over to later
  milestones or to other code.
* **B6 has no ACCEPTED verdict.** Both B6 runs at the pin are INFRA-VOID (not ACCEPTED), and those verdicts stand. The
  B6 gate is waived for M2; it was not passed.
* **Evidence relied on.** Both runs used the identical pinned invocation (`--runs 225`, `--expect-head e8fcea20`) on
  the shared box:
  * Round 7 (record #201, `e41b6830`): 225/225 PASS, 0 failures, both crafted probes PASS; INFRA-VOID on 16 noise
    breaches (cause not proven), clustered around two VM-fork records (stall- and freeze-type heartbeat gaps, VM-fork
    and balloon kmsg records).
    * Record: "ROUND 7 (run 2026-10-19): INFRA-VOID" in `milestone2/MILESTONE2_DECISION.md`; Round 7 in
      `milestone2/ticketdesk/REVIEW.md`; `docs/journals/2026-10-19_ticketdesk_b6_round7.md`.
    * Run evidence: `milestone2/ticketdesk/evidence/round7_2026-10-19/`.
  * Round 7 isolation rerun (record #202, `883f26de`): 225/225 PASS, 0 failures, both crafted probes PASS; INFRA-VOID
    on 16 noise breaches (cause not proven) around one freeze-type gap (a VM-fork record and balloon kmsg records).
    * Record: "ROUND 7 ISOLATION RERUN (run 2026-10-20)" in `milestone2/MILESTONE2_DECISION.md`; Round 7 isolation
      rerun in `milestone2/ticketdesk/REVIEW.md`; `docs/journals/2026-10-20_ticketdesk_b6_round7_isolation.md`.
    * Run evidence: `milestone2/ticketdesk/evidence/round7_isolation_2026-10-20/`.
  * 450/450 runs passed in total. Neither run printed a `CLASSIFY` line, so no B6 behavioural failure was observed.
    Both were INFRA-VOID only because of noise breaches; cause not proven.
* **Residual risk.**
  * Isolation was **never demonstrated**: both runs were on the shared, ballooned, fork-prone box.
  * Wall-time behaviour under the noise gaps is **unverified**. Three requests overlapped the gaps (Round 7 t997 and
    t487, rerun t622). All three passed under the pinned monotonic 10 s deadline, but on wall time all three would
    have exceeded 10 s; the pinned `classify()` would have marked them `infra`, and the verdicts would have been
    NOT ACCEPTED (infra), not INFRA-VOID.
* **Reopen triggers.** The waiver lapses, and B6 reopens as a blocker, if any of these happens:
  * a B6-type product failure (a kill-storm or crash-recovery durability or deadline failure classified as product)
    is observed at or after pin `7e9ef32e`;
  * an isolated-venue run of the pinned storm is ever done and does not pass;
  * the code under test, the storm script or its pin changes in a way that the 450/450 evidence no longer covers.
* **Superseded and moot items.** The Owner-authorized PC run below will not be done for M2. Whether #202's waived-venue
  invocation counts as the one pre-registered isolation rerun is now moot.

### Other gates and result

* Lain's final independent assurance review (#203, `dd8f2728`, `milestone2/ASSURANCE_M2_FINAL_REVIEW.md`) passes gates
  A, C, D and G under Owner waivers W-m2, W-m3, W-m4, W-m12 and W-m17 (granted 2026-10-20 11:02 EDT), passes H
  unconditionally, and records the m10 regression tests (`assurance_m10.py`) as manual-only until a workflow-scoped
  change adds the CI step. That review excluded B6; the B6 gate is waived for M2 by W-B6-venue above. Its PC-run note
  is superseded by this section.
* **Result: Milestone 2 (ticketdesk, D29 A, C, D, G, H) is ACCEPTED by the Owner** under waivers W-m2, W-m3, W-m4,
  W-m12, W-m17 and W-B6-venue. Per the directive's stop rule, the M2 loop stops here and M3 planning follows.
* No product code, script, pin or frozen files are changed by this record.

---

# ROUND 7 ISOLATION RERUN (run 2026-10-20): INFRA-VOID; M2 not accepted

Run by: a fresh Codex CLI session with no implementation role (not the Round 4–7 sessions), which checked the pins and
launched the single invocation (`evidence/round7_isolation_2026-10-20/codex_launch_prompt.md`). The verdict is the
script's own mechanical `VERDICT` line. The Eng Mgr session compiled this record from that output and ran clean_repro
and the freeze check after the run ended. It applied no classification of its own.

## Decision: INFRA-VOID (not ACCEPTED)

The single invocation completed all 225 trials with **0 failures** (225 PASS) and both crafted probes PASS. The noise
gates then reported **16 breaches**, so the script printed `noise=INFRA-VOID` and
`VERDICT INFRA-VOID (one isolation rerun allowed, same pins)`, and exited 1 (`AssertionError: noise gates breached:
INFRA-VOID`). The exit status was captured this time: `EXIT=1` is the last line of `kill_storm.txt`. No product
failure occurred and no `CLASSIFY` line was printed. Full clean reproduction passed (101/101 acceptance, 10/10
regression, 13/13 DX) and frozen paths are unchanged.

* **The isolation condition was NOT met.** This ran on the same shared, ballooned, fork-prone box as Round 7 (see
  "Venue" below). It therefore cannot separate product from infrastructure causes any better than Round 7 could.
* **Whether this counts as the one isolation rerun is William's decision.** The script's `VERDICT` text is fixed and
  generic. The rerun terms say "One attempt only … Every other exit is the attempt. No further reruns.", but the venue
  term was waived. Whether this waived-venue invocation used up the pre-registered rerun is therefore for William
  (Owner) to decide; this record does not state it as fact.
* **Milestone 2 is not accepted.** B6 remains open and the milestone stop rule is not satisfied.
* No restart was performed. No product code, script, or frozen files were edited.

```
SUMMARY runs=225 failures=0 orphan_runs=106 survivor_GETs=331903 survivor_DELETEs=331903; crafted probes=2 PASS; noise=INFRA-VOID breaches=16; SCRIPT_SHA256=540c243325c7b9d5439f0c592e04354ec9937ce98ec8beab760874671bd83d38; CODE_SHA=7e9ef32eb2eb6c71f7e0fe0280ed3240e1842989; CODE_PINNED=e8fcea2024b42bbfafd6e4410568c76d32e9c37a
VERDICT INFRA-VOID (one isolation rerun allowed, same pins)
EXIT=1
```

| Check / exact command | Result |
|---|---|
| Run checkout | `/workspace/hf-r7iso`, detached at `7e9ef32eb2eb6c71f7e0fe0280ed3240e1842989`, single parent `e8fcea2024b42bbfafd6e4410568c76d32e9c37a`, clean tree before and after |
| Pre-launch checks (Codex session) | exact HEAD, single expected parent, clean tree, script SHA-256, `CHECK_HEAD PASS`, artifacts path absent, no storm running |
| `sha256sum milestone2/ticketdesk/evidence/b6_kill_storm.py` | `540c243325c7b9d5439f0c592e04354ec9937ce98ec8beab760874671bd83d38` (before and after) |
| `cd /workspace/hf-r7iso && setsid nohup sh -c 'python3 milestone2/ticketdesk/evidence/b6_kill_storm.py --runs 225 --expect-sha256 540c2433…3d38 --expect-head e8fcea2024b42bbfafd6e4410568c76d32e9c37a --artifacts /tmp/round7iso/artifacts ; echo "EXIT=$?"' > /tmp/round7iso/kill_storm.txt 2>&1 < /dev/null &` | exactly once, at 01:07:09 EDT (launcher pid 1696433); `IDLE_GATE PASS`; `BUILD PASS`; 225 PASS, 0 failures; 16 NOISE breaches; `VERDICT INFRA-VOID`; output ended 07:44:44 EDT (`kill_storm.txt` mtime) |
| Exit status | `EXIT=1`, captured by the launcher shell. The run ended in `AssertionError: noise gates breached: INFRA-VOID` (script line 1348), with no other traceback |
| `bash milestone2/scripts/clean_repro.sh` (after the run) | exit 0; acceptance 101/101, regression 10/10, DX 13/13 |
| Freeze integrity: `git diff 10d9c2a8 HEAD --name-status -- milestone2/ticketdesk/requirements milestone2/ticketdesk/tests/acceptance milestone2/ticketdesk/dx milestone2/PLAN.md` | empty; exit 0: PASS (in the run checkout and on this branch's base `e41b6830`) |
| `find /tmp/round7iso/artifacts -maxdepth 1 -type d -name 'run*_failure'` | none (no failures) |

### Venue (William's waiver; isolation condition NOT met)

The isolation rerun terms ask for "a non-shared machine with no VM snapshot or fork, no active balloon driver".

* **Source of the waiver.** On 2026-10-19 the cloud agent VM launch failed for lack of usage (4:32–4:35 PM EDT), and
  William declined to spend money on extra compute. At 4:36 PM EDT he answered in the Grok Bot chat with Rintaro:
  "Retry the rerun on this box in a quiet window tonight". That knowingly waived the non-shared-machine term.
* **Terms kept.** Every other term was kept: the pins, the identical command, a fresh artifacts directory, the explicit
  exit capture, an independent non-Claude launcher, and one attempt only. The waiver changed no gate.
* **The isolation condition was not met.** The waiver note and the pre-launch venue evidence were written before launch
  (`venue_waiver.md`, `venue_evidence.txt`). They show the same shared virtio guest as Round 7, with the
  `virtio_balloon` driver bound (built in, so `lsmod` shows nothing), 8 CPUs, 16 GB RAM, kernel `6.12.94+`, and six
  `virtual machine fork` dmesg records between about 00:08 and 01:04 EDT that night. The `dmesg -T` wall times are
  approximate after VM pauses.
* **Quiet window.** The declared quiet window ran from launch until the run ended or 06:00 EDT at the latest. The run
  was frozen from about 01:59:49 EDT, resumed at about 07:26:40 EDT and ended at 07:44:44 EDT, so its last
  ~18 minutes fell **outside** the declared window. The operating session only waited on the output. No artifact shows
  that other agents on the box were idle.

### Noise breaches (one freeze-type gap; cause not proven)

The 16 breaches all fall in one cluster at about 01:59 EDT, during run 169's recovery phase (runs 1–168 had no breach):

* **6.978 s mono / 6.997 s real / 7.001 s boot gap** at 01:59:42.5–01:59:49.5 EDT (mono 29025.881): **stall-type**
  (the clocks agree).
* **0.487 s mono / 19610.689 s real / 0.464 s boot gap** starting 01:59:49.5 EDT (mono 29032.860): **freeze-type**. The
  guest clock stopped from about 01:59:49 to 07:26:40 EDT, roughly 5 h 27 min.
* Then `random: crng reseeded due to virtual machine fork` (mono 29033.57) and 13
  `virtio_balloon virtio0: Out of puff! Can't get 1 pages` kmsg records (mono 29038.2–29046.2).
* Breach count: 2 heartbeat gaps over 2 s (the script sizes a gap as max(mono, real, boot)) plus 14 kmsg matches,
  16 in total. No steal breach (max 60 s steal 0.098%). PSI io `full avg10` peaked at 4.21 (gate 10). cgroup
  `nr_throttled` went 0 → 0, there were no OOM events, and sampler coverage was 4238 samples over 4244 s monotonic.
  Both sidecars stayed alive with empty stderr.

A gap of that kind, with realtime jumping about 5.4 h while monotonic moved 0.49 s, followed by a VM-fork record and
balloon reclaim, is consistent with the box being snapshotted or suspended and later resumed. It does not prove it.

### The request that overlapped the gaps (diagnostic, not a criterion)

* **Run 169 recovery, `GET /v1/tickets/t622`** (expected 404 after its confirmed DELETE; got 404):
  * Client: 7.764 s monotonic (tsv seq 4850; the run line reports `max=7754.2ms`). On wall time it ran from
    01:59:42.53 EDT until the next request at 07:26:40.53 EDT, about 19,618 s.
  * **Margin to the 10 s deadline: 2.24 s.** Most of the 7.764 s (6.98 s) fell inside the stall-type gap, where all
    clocks kept running. Only 0.49 s fell inside the freeze-type gap.
  * Server timing log: lock wait 0.002 ms, handler 0.251 ms. The server answered at once; the stall was outside the
    handler.
  * Watchdog (`run169_recovery_watchdog.log`): one sample, at 7.655 s in flight, over a 7.924 s window. Server
    process state `S`; 11 of 12 threads sleeping (mostly `futex_wait_queue`), one runnable.
  * The deadline is monotonic by the pinned script (`Server.expect`, lines 286/295/308), as recorded for Round 7. Heartbeat
    gaps cover 96% of the request's monotonic elapsed time (7.46 of 7.76 s), and nearly all of it on wall time.
  * Counterfactual on wall time: it would have been a deadline failure, and the pinned `classify()` would have marked it
    `infra` (gap cover over 80% plus an overlapping fork record). The verdict would then have been `NOT_ACCEPTED_INFRA`.
    B6 stays open either way.

### Next step (Owner decision, 2026-10-20)

This is William's decision, recorded as given. It is not a recommendation of this record. At 7:57 AM EDT on 2026-10-20
he wrote in the Grok Bot chat: "Can we note this in the repo and I'll run this and get findings on my pc."

* **What William will do:** run the B6 kill storm himself, once, on his own PC.
* **Why:** this box is a shared VM whose logs show VM-fork and balloon records and freeze-type gaps. Round 7 and this
  rerun passed 450/450 runs between them, with 0 failures, but both were voided by box freezes.
* **The command is identical and pinned:**
  * a detached checkout of `7e9ef32eb2eb6c71f7e0fe0280ed3240e1842989`;
  * script sha256 `540c243325c7b9d5439f0c592e04354ec9937ce98ec8beab760874671bd83d38`;
  * `--runs 225` and `--expect-head e8fcea2024b42bbfafd6e4410568c76d32e9c37a`;
  * a fresh artifacts directory, with the exit status captured (`sh -c '…; echo "EXIT=$?"'`).
  * An `IDLE_GATE FAIL` (exit 3) is not an attempt. One attempt only.
* **Gates:** the run must pass the same 120 s idle gate and the same noise gates as this record.
* **Venue evidence before launch:** it is recorded before launch: bare metal vs VM/WSL2, any balloon or dynamic memory,
  sleep disabled, other workloads stopped, and `/dev/kmsg` readable. Whether the run counts as isolated is decided by
  that evidence.
* **Status:** this is a new Owner-authorized venue run, separate from the pre-registered isolation rerun. Its result
  will be recorded and reviewed separately, under the same Round 7 decision rule, by a fresh independent reviewer.
* **Milestone 2 stays not accepted until then.**
* **Superseded 2026-10-20 11:24 EDT:** William waived the B6 venue requirement and accepted M2 (see the top of this
  file). This PC run will not be done for M2.

Evidence: `ticketdesk/evidence/round7_isolation_2026-10-20/` (storm output with the `EXIT=1` line, launcher record,
Codex launch prompt, venue evidence and waiver, `script.sha256`, artifacts listing, heartbeat and kmsg logs, sidecar
stderr, noise summary, the run 169 watchdog log, freeze-overlap excerpts, clean_repro output, freeze-integrity output
and pin-test output, original plus a labelled re-run). The full 806 MB of artifacts stay in `/tmp/round7iso/artifacts`.

---

# ROUND 7 (run 2026-10-19): INFRA-VOID

Run by: a fresh Codex CLI session with no implementation role (not the Round 4–6 sessions), which checked the pins
and launched the single invocation. The verdict is the script's own mechanical `VERDICT` line. The Eng Mgr session
compiled this record from that output and ran clean_repro and the freeze check afterwards. It applied no
classification of its own.

## Decision: INFRA-VOID (not ACCEPTED)

The single registered invocation completed all 225 trials with **0 failures** (225 PASS), both crafted probes PASS,
then the noise gates reported **16 breaches**, so the script printed `noise=INFRA-VOID` and
`VERDICT INFRA-VOID (one isolation rerun allowed, same pins)` and exited 1 (`AssertionError: noise gates breached:
INFRA-VOID`). Under the pre-registered rule "0 failures with a noise breach → INFRA-VOID, not ACCEPTED", Round 7 is
**not ACCEPTED**. No product failure occurred and no `CLASSIFY` line was printed. Full clean reproduction passed
(101/101 acceptance, 10/10 regression, 13/13 DX) and frozen paths are unchanged. B6 remains open; the milestone stop
rule is not satisfied. Per the rule, exactly **one isolation rerun** is allowed with the same pins, on the terms in
"Isolation rerun terms" below. No restart was performed.

```
SUMMARY runs=225 failures=0 orphan_runs=94 survivor_GETs=332155 survivor_DELETEs=332155; crafted probes=2 PASS; noise=INFRA-VOID breaches=16; SCRIPT_SHA256=540c243325c7b9d5439f0c592e04354ec9937ce98ec8beab760874671bd83d38; CODE_SHA=7e9ef32eb2eb6c71f7e0fe0280ed3240e1842989; CODE_PINNED=e8fcea2024b42bbfafd6e4410568c76d32e9c37a
VERDICT INFRA-VOID (one isolation rerun allowed, same pins)
```

| Gate | Round 1 | Round 2 | Round 3 | Round 4 | Round 5 | Round 6 | Round 7 |
|---|---|---|---|---|---|---|---|
| A Practical HTTP API | FAIL | FAIL | FAIL (B6) | INCOMPLETE (timeout) | NOT ACCEPTED (cause unknown) | NOT ACCEPTED (1/75 timeout) | INFRA-VOID (0/225 failures, 16 noise breaches) |
| C Authentication and authorization | PASS | PASS | PASS | PASS | PASS | PASS (full suite) | PASS (full suite) |
| D Validation and reliable errors | PASS with caveats | PASS with caveats | PASS with caveats | PASS with caveats | PASS with caveats | PASS with caveats | PASS with caveats |
| G Standalone installation and execution | FAIL | PASS | PASS | PASS | PASS | PASS | PASS |
| H Developer experience | PASS with caveat | PASS with caveat | PASS | PASS | PASS | PASS | PASS |

| Check / exact command | Result |
|---|---|
| `git rev-parse HEAD` | `7e9ef32eb2eb6c71f7e0fe0280ed3240e1842989` (#200 squash-merge; fills only the pin line on `e8fcea20`) |
| Pre-launch checks (Codex session) | exact HEAD, single expected parent, script SHA-256, `CHECK_HEAD PASS`, artifacts path absent |
| `sha256sum milestone2/ticketdesk/evidence/b6_kill_storm.py` | `540c243325c7b9d5439f0c592e04354ec9937ce98ec8beab760874671bd83d38` (before and after) |
| `python3 milestone2/ticketdesk/evidence/b6_kill_storm.py --runs 225 --expect-sha256 540c2433…3d38 --expect-head e8fcea2024b42bbfafd6e4410568c76d32e9c37a --artifacts /tmp/round7/artifacts` | exactly once, detached with setsid/nohup at 12:55:46 EDT (pid 1646137 launcher); `IDLE_GATE PASS`; 225 PASS, 0 failures; 16 NOISE breaches; `VERDICT INFRA-VOID`; EXIT=1 (inferred; see next row) |
| Exit status | 1. The detached launcher did not record the status; the run ended in an uncaught `AssertionError: noise gates breached: INFRA-VOID` (Python exit 1), with no other traceback |
| `bash milestone2/scripts/clean_repro.sh` | exit 0; acceptance 101/101, regression 10/10, DX 13/13 |
| Freeze integrity: `git diff 10d9c2a8 HEAD --name-status -- milestone2/ticketdesk/requirements milestone2/ticketdesk/tests/acceptance milestone2/ticketdesk/dx milestone2/PLAN.md` | empty; exit 0: PASS |
| `find /tmp/round7/artifacts -maxdepth 1 -type d -name 'run*_failure'` | none (no failures) |

### Noise breaches (clustered around two VM-fork events; cause not proven)

The 16 breaches cluster around two VM-fork events. That is consistent with host-side VM pauses but does not prove
them. Each heartbeat gap is typed by its clocks: **freeze-type** (realtime ≫ monotonic: the guest clock stopped) or
**stall-type** (monotonic = realtime = boottime: the clocks kept running while both sidecars were starved, with
steal around 0).

* **Cluster 1, about 13:51 EDT (run 168 recovery):**
  * 2.796 s gap at 13:51:10 (mono 21510.429): **stall-type** (mono = real = boot = 2.796 s), about 13 s before the
    fork record.
  * 7.581 s mono / **48.749 s real** / 7.603 s boot gap at 13:51:23–13:52:12 (mono 21523.064): **freeze-type**.
  * Then `random: crng reseeded due to virtual machine fork` (mono 21531.1) and 10
    `virtio_balloon virtio0: Out of puff! Can't get 1 pages` kmsg records.
* **Cluster 2, about 14:05 EDT (runs 208–209):**
  * 13.475 s gap at 14:05:11–14:05:25 (mono 22310.323, between the run 208 recovery and run 209 storm servers, no
    request in flight): **stall-type** (mono = real = boot), about 23 s before the fork record.
  * 2.722 s mono / **74.086 s real** gap at 14:05:35–14:06:49 (mono 22333.687, run 209 recovery): **freeze-type**.
  * Then a second `virtual machine fork` kmsg record (mono 22336.6).
* Breach count: 4 heartbeat gaps over 2 s (the script sizes a gap as max(mono, real, boot)) plus 12 kmsg matches,
  16 in total. No steal breach (max 60 s steal 0.091%). PSI io `full avg10` peaked at 9.00 (gate 10). cgroup
  `nr_throttled` went 0 → 0, there were no OOM events, sampler coverage was 4454 samples over 4479 s, and both sidecars
  stayed alive.

### Requests that overlapped the freezes (diagnostic, not a criterion)

* **Which clock the deadline uses.** The pre-registered text says "total client elapsed time" without naming a
  clock. The frozen, pinned script (sha256 `540c2433…3d38`) defines it as monotonic in `Server.expect`:
  `start = time.monotonic()` (line 286) and `elapsed = time.monotonic() - start` (lines 295 and 308), checked against
  the deadline (lines 299 and 309; `TOTAL_DEADLINE = 10.0`, line 79). The clock came from the pre-registered script.
  It was not chosen after the run.
* **Run 168, `DELETE /v1/tickets/t997`:**
  * Client: 8.300 s monotonic, about 49.5 s wall (13:51:23.40 to the next request at 13:52:12.88).
  * Monotonic advanced only partly during the freeze: 7.58 s of the 48.75 s real gap (16%).
  * Server timing log:
    * lock acquired after 0.002 ms (`locked_mono_ms=17337.492`) and held for the whole handler (`end_mono_ms=25615.697`,
      handler 8278 ms);
    * `store_timing` ran from about mono 25314.5 to 25501.1 (186.6 ms; fsync 81 ms).
  * So the ~7.98 s stall came **after lock acquisition and before the store write, with the lock held**. It was not
    lock contention and not the store write.
* **Run 209, `DELETE /v1/tickets/t487`:**
  * Client: 3.055 s monotonic, about 74.4 s wall (14:05:35.13 to the next request at 14:06:49.56).
  * Monotonic advanced 2.72 s of the 74.09 s real gap (4%).
  * Server: lock acquired after 0.002 ms and held through the handler (3051 ms). `store_timing` shows
    `marshal_ms=2901.6` of a 2960 ms store write, so this stall fell **inside the store write's marshal phase, with
    the lock held**.
* **Counterfactual on wall time.** Both requests would have broken the 10 s deadline. The pinned `classify()` would have
  marked both `infra`:
  * t997: the freeze gap alone covers 7.53 s of 8.30 s (91%), or 7.79 s (94%) counting the adjacent 0.259 s gap, as
    `classify()` sums them.
  * t487: gaps cover 2.72 s of 3.06 s (89%).
  * Both are over the 80% threshold. The verdict would then have been `NOT_ACCEPTED_INFRA`, with the **same** one
    isolation rerun. Either way B6 stays open and the next step is unchanged.

### Isolation rerun terms (the one rerun the rule allows)

* **Venue:** a non-shared machine with no VM snapshot or fork, no active balloon driver, about 1.5 GB of free disk,
  and a declared quiet window (no other builds, tests, storms or browser work).
* **Checkout:** a detached checkout of exactly `7e9ef32eb2eb6c71f7e0fe0280ed3240e1842989`, not main's tip. Once this
  record merges, main is past the pin and `--expect-head` would reject it.
* **Command:** identical, with a fresh artifacts directory:
  `python3 milestone2/ticketdesk/evidence/b6_kill_storm.py --runs 225 --expect-sha256 540c243325c7b9d5439f0c592e04354ec9937ce98ec8beab760874671bd83d38 --expect-head e8fcea2024b42bbfafd6e4410568c76d32e9c37a --artifacts <fresh dir>`
* **Exit status captured explicitly:**
  `setsid nohup sh -c 'python3 … ; echo "EXIT=$?"' > <dir>/kill_storm.txt 2>&1 < /dev/null &`
* **Reviewer:** a fresh independent session with no implementation role.
* **One attempt only:**
  * An `IDLE_GATE FAIL` (exit 3) is not an attempt and may wait for a quiet box.
  * Every other exit is the attempt.
  * No further reruns.

**Venue note for this run:** the shared box was allowed only with a declared quiet window. The Eng Mgr session only
polled the output. No artifact shows that other agents on the box were idle, and that cannot be checked after the fact.

Evidence: `ticketdesk/evidence/round7_2026-10-19/` (storm output, `script.sha256`, artifacts listing, heartbeat and
kmsg logs, sidecar stderr, noise summary, clean_repro output, launcher record, watchdog logs and request/server
excerpts for the two pause-overlapping requests). The full 805 MB of artifacts stay in `/tmp/round7/artifacts`.
No product code, script, or frozen files were edited.

---

# ROUND 7 pre-registration (terms of the run above; unchanged)

Registered 2026-10-19, before any Round 7 run. It follows the Round 7 proposal in Motoko's
[#197 review §3](https://github.com/howlcipher/howlframe/pull/197#pullrequestreview-5573915479). These terms are fixed.
Changing any of them voids Round 7.

## Pre-registered terms

* **Why the protocol changed:** in Round 6, the client's 10 s socket timer fired late in runs 11 and 37. Run 61 had a
  34.6 s client stall while the server's longest handler took 73 ms. `dmesg` shows a VM fork plus `virtio_balloon`
  reclaim during run 62. Runs 38 and 62 took 262.7 s and 218.8 s of recovery wall time against an 18.1 s median. Box
  pauses are therefore a live confound, and Round 7 must be able to separate them from product stalls.
* **Script frozen:** `milestone2/ticketdesk/evidence/b6_kill_storm.py`.
  * **Script sha256:** `540c243325c7b9d5439f0c592e04354ec9937ce98ec8beab760874671bd83d38`
  * CI enforces this pin (`TestRound7StormScriptPinned`). The run must pass `--expect-sha256` with this value. The script
    prints `SCRIPT_SHA256` at start and in SUMMARY, and writes `script.sha256` into the artifacts directory.
  * Unchanged from Round 6: seed 20261019, 1500 API-seeded tickets, kill delay U(0.005, 0.8) s, default runner
    ceilings, the 10 s per-socket-operation timeout, the crafted probes, and every Round 6 diagnostic.
  * The two #194 follow-ups are folded in. The SIGSTOP self-test polls `/proc/PID/stat` for state `T`, and the pin
    line comparison is byte-exact (no `rstrip()`, so a trailing space is rejected).
* **New instrumentation, diagnostic only:**
  * **Heartbeat sidecar.** A separate process, pinned to the last CPU where possible, sleeps 50 ms in a loop. It logs
    every gap over 250 ms with CLOCK_MONOTONIC, CLOCK_REALTIME and CLOCK_BOOTTIME deltas, plus an alive line every 10 s
    (`heartbeat.log`).
  * **Noise sampler sidecar**, once a second (`noise.jsonl`):
    * the `/proc/stat` cpu line (steal, iowait)
    * `/proc/pressure/{cpu,io,memory}`
    * cgroup `cpu.stat`, `memory.pressure` and `memory.events`
    * `/proc/diskstats` for the state file's device
    * `pgmajfault`
    * new `/dev/kmsg` records (`kmsg.log`)
  * **Per-request client log** for every request (`runNN_<phase>_requests.tsv`): seq, method, path, monotonic start
    and end, realtime start, status and elapsed.
  * **In-flight watchdog.** Once a request has been in flight for 2 s, every 500 ms until it completes, it samples
    `/proc/PID/task/*/{stat,wchan,schedstat,stack}`, `/proc/PID/status` and diskstats. It reports per-thread on-CPU
    and run-queue-wait deltas against the last sample taken before the request started (`runNN_<phase>_watchdog.log`).
    It never kills anything.
  * **Server timing.** The server runs with `--timing-log` (per-request handler start, end and lock wait, plus a
    store-write phase line for writes of 100 ms or more) and `GODEBUG=gctrace=1`. That output goes to the per-run
    server log. This needs the `--timing-log` PR to be merged into the code under test.
  * **On any failure:** the Round 6 diagnostics, plus schedstat and diskstats deltas since the request started, the
    watchdog samples, a heartbeat excerpt (±5 s), and a copy of the full server log in `runNN_failure/`.
* **Total-deadline rule (a criterion change):** a request whose total client elapsed time is over 10 s is a failure,
  even if a reply arrives. A socket timeout is a deadline failure too. This closes the poll-rescan race that let runs
  37 and 61 pass.
* **Pre-run idle gate:** 120 s of sampling before the build. All of these must hold:
  * steal under 1%;
  * PSI cpu `some avg60` under 5;
  * PSI io `full avg60` under 2;
  * PSI memory `full avg60` = 0;
  * no heartbeat gap over 250 ms;
  * no `virtual machine fork` or `virtio_balloon` kmsg record;
  * `/dev/kmsg` readable and both sidecars running.

  If any check fails, the script prints `IDLE_GATE FAIL <reasons>` and exits 3 before building. **That invocation is
  not an attempt.** It may be repeated once the box is quiet. Every other exit is an attempt.
* **Noise gates (INFRA-VOID).** These are checked from the start of run 01 to the end of the last run. The invocation
  always runs to completion. It is INFRA-VOID on any of:
  * a heartbeat gap over 2 s;
  * steal over 2% in any 60 s window;
  * PSI io `full avg10` over 10;
  * any increase in cgroup `nr_throttled`;
  * any `virtual machine fork` or `virtio_balloon` kmsg record;
  * no heartbeat record for 30 s, or a sidecar exiting;
  * noise samples covering less than half the window.
* **Venue:** a dedicated, otherwise idle machine that is not snapshotted or ballooned is preferred. On the shared box,
  a declared quiet window is required: no other agent may run builds, tests, storms or browser work. The gates
  decide whether the invocation counts. The artifacts need about 1.5 GB of disk.
* **Code under test:** pinned before the run, by one commit that changes only the line below.
  * It must be a commit on main that includes the `--timing-log` PR and this frozen script.
  * The run must pass `--expect-head <that sha>`. The rules are the same as Round 6:
    * HEAD **is** the pin; or
    * HEAD is **one commit directly on the pin** (squash or direct, not a merge) whose
      `git diff --no-renames --numstat` is exactly `1 1 milestone2/MILESTONE2_DECISION.md`. That one change must turn
      the placeholder line below into exactly `` * **Round 7 code-under-test SHA:** `<pinned sha>` `` (byte-exact).
  * CI covers these rules in `TestRound7ExpectHeadPinRules`.
  * **Round 7 code-under-test SHA:** `e8fcea2024b42bbfafd6e4410568c76d32e9c37a`
* **Command, exactly one invocation** (detached with setsid/nohup):
  `python3 milestone2/ticketdesk/evidence/b6_kill_storm.py --runs 225 --expect-sha256 <script sha above> --expect-head <pinned sha> --artifacts /tmp/round7/artifacts`
  * Exactly `--runs 225`, about 75–90 min plus the 120 s gate. At 0/225, the 95% upper bound on the per-run failure
    rate is under 1.4%, which excludes the 1/75 seen in Round 6.
  * Then run `bash milestone2/scripts/clean_repro.sh` and the freeze-integrity diff.
* **Decision rule.** The script applies these rules mechanically: one `CLASSIFY` line per failure, then
  `VERDICT <...>` after SUMMARY. The reviewer applies no other classification.
  * **0 failures, no noise breach, both crafted probes PASS, clean_repro fully passes, frozen paths unchanged →
    ACCEPTED.** The invocation prints `SUMMARY runs=225 failures=0 ... noise=QUIET`, then `VERDICT ACCEPTED`, and
    exits 0.
  * **A failure with no stalled-request window** (a check failure on a reply, such as a 500 or confirmed deletion
    lost) → **NOT ACCEPTED (product).**
  * **A failure where heartbeat gaps cover at least 80% of the request's elapsed time, or a fork/balloon kmsg record
    or a steal breach overlaps the request → NOT ACCEPTED (infra).** Exactly one rerun is allowed, on an isolated
    machine, with the same pins.
  * **A failure where no heartbeat gap of 1 s or more overlaps the request and the noise gates hold → NOT ACCEPTED
    (product).**
  * **Anything else is ambiguous, and ambiguity counts as product → NOT ACCEPTED (product).** If any failure is
    product, the verdict is product.
  * **0 failures with a noise breach → INFRA-VOID, not ACCEPTED.** The same one-isolation-rerun rule applies.
* **No restart:** apart from the idle-gate exit above, an invocation that doesn't end with SUMMARY, VERDICT and exit 0
  is **NOT ACCEPTED**. That includes a harness crash, an infrastructure error, a sha or head mismatch, or an operator
  interrupt. There are no other reruns, restarts or replaced runs.
* **Reviewer:** a fresh independent session that wrote neither the B6 fix, #191, the timing PR, this script, nor
  Rounds 4–6.
* **Evidence:** under `evidence/round7_<date>/`:
  * the full storm output;
  * `script.sha256` and the artifacts listing;
  * the `NOISE`, `CLASSIFY` and `VERDICT` lines;
  * the heartbeat log;
  * a noise summary (max steal over 60 s, max PSI, gaps, kmsg matches);
  * for any failure, its `runNN_failure/` file list, watchdog excerpt, matching timing/access/gctrace lines and
    goroutine dump.

---

# ROUND 6 (superseded by the Round 7 pre-registration; kept as history), 2026-10-19

Reviewer: independent final reviewer (fresh Codex CLI session; no implementation role; not the Round 4/5 sessions)

## Decision: NOT ACCEPTED

The single registered invocation completed 75 trials with 1 failure (74 PASS), both crafted probes PASS,
and exit 1. Run 11 timed out deleting t400 during survivor cleanup. Full clean reproduction passed:
101/101 acceptance, 10/10 regression, 13/13 DX. Frozen paths are unchanged. The zero-failure threshold
was not met; B6 remains open and the milestone stop rule is not satisfied. No restart was performed.
Carryover m12, m15–m18 and issue #194 remain non-blocking.

| Gate | Round 1 | Round 2 | Round 3 | Round 4 | Round 5 | Round 6 |
|---|---|---|---|---|---|---|
| A Practical HTTP API | FAIL | FAIL | FAIL (B6) | INCOMPLETE (timeout) | NOT ACCEPTED (cause unknown) | NOT ACCEPTED (1/75 timeout) |
| C Authentication and authorization | PASS | PASS | PASS | PASS | PASS | PASS (full suite) |
| D Validation and reliable errors | PASS with caveats | PASS with caveats | PASS with caveats | PASS with caveats | PASS with caveats | PASS with caveats |
| G Standalone installation and execution | FAIL | PASS | PASS | PASS | PASS | PASS |
| H Developer experience | PASS with caveat | PASS with caveat | PASS | PASS | PASS | PASS |

| Check / exact command | Result |
|---|---|
| `git rev-parse HEAD` | `1561b9a2c7e6d798b7acf142e65e363352c557d2` |
| `git status --short` (before run) | empty |
| `sha256sum milestone2/ticketdesk/evidence/b6_kill_storm.py` | `e1115c07dd1d37d40b21dc5b4df22f6dd3951f8beadbb5c0dd269fa1be460e2f` |
| `git diff 10d9c2a8 HEAD --name-status -- milestone2/ticketdesk/requirements milestone2/ticketdesk/tests/acceptance milestone2/ticketdesk/dx milestone2/PLAN.md` | empty before and after storm; exit 0 |
| `python3 milestone2/ticketdesk/evidence/b6_kill_storm.py --runs 75 --expect-sha256 e1115c07dd1d37d40b21dc5b4df22f6dd3951f8beadbb5c0dd269fa1be460e2f --expect-head 92c150d1736794c40597920c4b3b7df8e9e77d3b --artifacts /tmp/round6/artifacts` | exactly once, detached with setsid/nohup; runs=75 failures=1 orphan_runs=37 survivor_GETs=110762 survivor_DELETEs=109292; crafted probes=2 PASS; EXIT=1 |
| SCRIPT_SHA256 | `e1115c07dd1d37d40b21dc5b4df22f6dd3951f8beadbb5c0dd269fa1be460e2f` |
| CODE_SHA | `1561b9a2c7e6d798b7acf142e65e363352c557d2` |
| CODE_PINNED | `92c150d1736794c40597920c4b3b7df8e9e77d3b` |
| `bash milestone2/scripts/clean_repro.sh` | exit 0; acceptance 101/101, regression 10/10, DX 13/13 |
| `go test -count=1 ./...` (after evidence records) | exit 0; 76 packages ok, no failing package |
| `grep -c '^DIAG' /tmp/round6/kill_storm.txt` | 63 lines; one reason: run=11 TimeoutError |
| `find /tmp/round6/artifacts -maxdepth 1 -type d -name 'run*_failure'` | only run11_failure/ |

Timeout diagnostics: DIAG fired for run 11. Provisional classification: box stall; the request eventually
logged 200 after 68.768 s, health then answered in 12 ms, and the dump shows no remaining request or
store-lock waiter. Exact cause remains unproven; a transient server delay cannot be excluded.
See `ticketdesk/evidence/round6_2026-10-19/diagnosis.txt` for process states, load, file list and dump excerpts.
Evidence: `ticketdesk/evidence/round6_2026-10-19/`; full artifacts retained in `/tmp/round6/artifacts`.
No product code, script, or frozen files were edited.

## Pre-registered terms (unchanged)

Registered 2026-10-19, before any Round 6 run. Re-pinned the same day (round6-protocol PR) with Motoko's pre-Round-6
amendments; this re-pin supersedes the earlier `a2d8534` pin. These terms are fixed. Changing any of them voids Round 6.

* **Script frozen:** `milestone2/ticketdesk/evidence/b6_kill_storm.py`.
  * **Script sha256:** `e1115c07dd1d37d40b21dc5b4df22f6dd3951f8beadbb5c0dd269fa1be460e2f`
  * CI enforces this pin (`TestRound6StormScriptPinned`). The script prints `SCRIPT_SHA256` at start and in its
    SUMMARY line, and writes `script.sha256` into the artifacts directory. The run must pass
    `--expect-sha256 e1115c07dd1d37d40b21dc5b4df22f6dd3951f8beadbb5c0dd269fa1be460e2f`.
  * Unchanged from earlier rounds: seed 20261019, 1500 API-seeded tickets, kill delay U(0.005, 0.8) s, default
    runner ceilings, and the per-request timeout `REQUEST_TIMEOUT = 10` s.
  * Diagnostics in this version (none of them change pass criteria):
    * per-run server logs and per-run p99/max latency;
    * `/proc/PID/wchan`, `status` and `task/*/stat` recorded before SIGQUIT;
    * SIGQUIT goroutine dump, and `stop()` falls back to SIGKILL;
    * `state.json.tmp-*` cleared, timestamped aside, after every snapshot restore;
    * a `runNN_failure/` state snapshot on every failure, including 'confirmed deletion lost' after the storm server
      has stopped;
    * a server that fails to start is killed before the error propagates;
    * a SIGSTOP self-test (`--self-test-sigstop`, run in CI).
* **Code under test:** pinned before the run, by one commit that changes only the line below.
  * It must be a commit on main that includes #191 (the fix for #190, merged as `5778134e`) and this frozen script.
    That means the main commit that merges this protocol PR, or a later one.
  * The run must pass `--expect-head <that sha>`. The script accepts only two cases:
    * HEAD **is** the pinned commit; or
    * HEAD is **one commit directly on the pin** (its single parent is the pin) whose
      `git diff --no-renames --numstat` is exactly `1 1 milestone2/MILESTONE2_DECISION.md`. That one change must
      turn the placeholder line below into exactly `` * **Code-under-test SHA:** `<pinned sha>` `` (an exact
      line match, with nothing appended).
    * The pin commit must be a **squash or direct commit on the pin, not a merge**. A merge commit has two parents and
      is rejected.

    Anything else aborts the run, including renames, any other `.md` change and extra commits. SUMMARY carries
    `CODE_PINNED=<sha>`. CI covers these rules in `TestRound6ExpectHeadPinRules`.
  * **Code-under-test SHA:** `92c150d1736794c40597920c4b3b7df8e9e77d3b`
* **Command, exactly one invocation:**
  `python3 milestone2/ticketdesk/evidence/b6_kill_storm.py --runs 75 --expect-sha256 e1115c07dd1d37d40b21dc5b4df22f6dd3951f8beadbb5c0dd269fa1be460e2f --expect-head <pinned sha>`
  * Exactly `--runs 75`, on an otherwise idle box.
  * Then `bash milestone2/scripts/clean_repro.sh` and the freeze-integrity diff.
* **Threshold:** ACCEPTED only if all of these hold:
  * the single invocation prints `SUMMARY runs=75 failures=0 ...` and exits 0;
  * both crafted probes PASS;
  * clean_repro passes fully;
  * frozen paths are unchanged.

  Otherwise NOT ACCEPTED. Any timeout, 500, worker hang or check failure is a failure.
* **No restart:** an invocation that doesn't end with a SUMMARY line and exit 0 is **NOT ACCEPTED**. That
  includes a harness crash, an infrastructure error (build, port, disk, OOM, box restart), a sha or head mismatch,
  or an operator interrupt. There are no reruns, no restarts, and no discarding or replacing runs.
* **Flake bound:** with 0/75 required, a 4% per-run flake would still pass only about 5% of the time (0.96^75 ≈ 0.047).
* **Reviewer:** a fresh independent session that wrote neither the B6 fix, #191, nor Rounds 4–5.
* **Evidence:** under `evidence/round6_<date>/`:
  * the full storm output, including the `SCRIPT_SHA256` and `CODE_SHA` lines;
  * `script.sha256`;
  * the artifacts directory listing;
  * any `runNN_failure/` contents, plus the matching goroutine dump and `/proc` diagnostics.

---

# ROUND 5 (superseded by the Round 6 pre-registration; kept as history), 2026-10-19, commit `f7cc2a8`

Reviewer: independent final reviewer (fresh Codex CLI session; no implementation role; not the Round 4 session)

## Decision: NOT ACCEPTED (clean rerun, cause unknown)

Reclassified 2026-10-19 by the Eng Mgr after Motoko's review of #189. The reviewer's raw result is unchanged
(the reviewer originally recorded ACCEPTED): the unchanged storm exited 0 with 25/25 PASS, both crafted probes
PASS, and clean reproduction passed. But Round 4's run-24 timeout was never diagnosed, and a clean 25-run rerun
cannot rule it out: a 4% per-run flake passes 25/25 about 36% of the time (0.96^25 ≈ 0.36). B6 stays open
until Round 6 meets its pre-registered threshold. Candidate root cause filed as #190: `store.mu` is not released
when `store_put`/`store_delete` panics, so one panic would hang every later store request (a candidate
explanation for run 24's DELETE timeout; not proven). New fixed regression test `b6_run24_unindexed_survivor_then_cleanup_deletes` (unindexed survivors,
then cleanup DELETEs) is in `tests/regression/review_round1.py` (10/10 regression locally).

| Gate | Round 1 | Round 2 | Round 3 | Round 4 | Round 5 |
|---|---|---|---|---|---|
| A Practical HTTP API | FAIL | FAIL | FAIL (B6) | INCOMPLETE (storm timeout) | NOT ACCEPTED (clean 25/25 rerun; run-24 cause unknown) |
| C Authentication and authorization | PASS | PASS | PASS | PASS | PASS (full suite) |
| D Validation and reliable errors | PASS with caveats | PASS with caveats | PASS with caveats | PASS with caveats | PASS with caveats |
| G Standalone installation and execution | FAIL | PASS | PASS | PASS | PASS |
| H Developer experience | PASS with caveat | PASS with caveat | PASS | PASS | PASS |

| Check / exact command | Result |
|---|---|
| `git diff 10d9c2a8 HEAD --name-status -- milestone2/ticketdesk/requirements milestone2/ticketdesk/tests/acceptance milestone2/ticketdesk/dx milestone2/PLAN.md` | empty; exit 0; PASS |
| `git show HEAD` | diagnostics-only instrumentation reviewed; assertions, seed 20261019, 25 runs, 1500 tickets, delays and existing timeouts unchanged; diagnostic health probe has a separate 3 s timeout |
| `python3 milestone2/ticketdesk/evidence/b6_kill_storm.py` | exit 0; 25/25 PASS; 0 failures; 14 runs with unindexed survivors; 36,912 survivor GETs and 36,912 survivor DELETEs |
| Crafted opposite window (same storm command) | PASS; unindexed t2 omitted from walks, GET 200 / DELETE 200 / GET 404 |
| Crafted dangling state (same storm command) | PASS; 5 ids across bucket boundaries skipped; walks 200; GET/DELETE 404; subsequent create/delete succeeds; stale entries persist |
| `bash milestone2/scripts/clean_repro.sh` | exit 0; acceptance 101/101, regression 9/9, DX 13/13 |
| `go test -count=1 ./...` | exit 0; 76 packages ok, no failing package |

Evidence: `milestone2/ticketdesk/evidence/round5_2026-10-19/`. Storm output is complete;
clean_repro.txt contains the suite output and exact counts. The storm built committed HEAD
from a fresh archive and used default runner ceilings. No other heavy work was run concurrently.
All recovery walks (limits 1 and 100, status=done limit 7), durable confirmed-deletion checks,
survivor GETs, survivor DELETE/GET-404/repeat-DELETE-404 checks and final empty walks passed.
The script itself remains unchanged by this review.

## Timeout diagnosis

No timeouts in Round 5; no DIAG blocks or failures. No targeted rerun was needed.
Round 4's timeout remains historically unclassified; this passing run does not establish its cause.

## Carryover

Non-blocking m12 (port-probe race), m15 (no earlier-state migration), m16 (readable unindexed
records after interrupted unconfirmed mutations), m17 (truncated-upload 400 error shape), and
m18 (harmless dangling entries persist and DELETE returns 404) remain. No new product bug found.
Superseded by the reclassification: B6 stays open pending Round 6; #190 is the candidate cause for run 24.

---

# ROUND 4 (superseded by Round 5; kept as history), 2026-10-19, commit `7092bcff`

Reviewer: independent final reviewer (Codex CLI session; no implementation role).

## Decision: NOT ACCEPTED

B6's original permanent-listing failure was not reproduced: the independent storm found no 500 or inconsistent
confirmed mutation, and both crafted crash windows behaved harmlessly. However, the required storm exited 1:
24 of 25 runs completed all checks; run 24 timed out during survivor deletion after its recovery walks and
all survivor GETs had passed. This is incomplete validation, not a demonstrated recurrence of B6. No product
code was changed. Acceptance remains pending a fully passing storm and classification of this timeout.

| Gate | Round 1 | Round 2 | Round 3 | Round 4 |
|---|---|---|---|---|
| A Practical HTTP API | FAIL | FAIL | FAIL (B6) | INCOMPLETE (storm timeout; original B6 probes pass) |
| C Authentication and authorization | PASS | PASS | PASS | PASS (suite; capability delta reviewed) |
| D Validation and reliable errors | PASS with caveats | PASS with caveats | PASS with caveats | PASS with caveats |
| G Standalone installation and execution | FAIL | PASS | PASS | PASS |
| H Developer experience | PASS with caveat | PASS with caveat | PASS | PASS |

| Check / exact command | Result |
|---|---|
| `git diff 10d9c2a8 HEAD --name-status -- milestone2/ticketdesk/requirements milestone2/ticketdesk/tests/acceptance milestone2/ticketdesk/dx milestone2/PLAN.md` | empty; PASS |
| `python3 milestone2/ticketdesk/evidence/b6_kill_storm.py` | exit 1; 25 SIGKILL trials, 24 complete PASS, 1 timeout; no observed 500 or inconsistent durable state; 11 runs with unindexed survivors |
| Storm recovery checks | full limit=1 and limit=100 cursor walks, status=done limit=7 walk, GET of every durable survivor; 36,908 survivor GETs passed across all 25 runs; 35,451 survivors fully DELETE/GET-404/repeat-DELETE-404 checked in the 24 completed runs |
| Crafted opposite window | PASS; unindexed t2 omitted from all walks, GET 200, DELETE 200, subsequent GET 404 |
| Crafted dangling state (5 ids, including bucket boundaries) | PASS; all walks 200 and exact; dangling GET/DELETE 404; later create/delete and repeat walks succeed; stale index entries persist |
| `bash milestone2/scripts/clean_repro.sh` | exit 0; acceptance 101/101, regression 9/9, DX 13/13 |
| `go test -count=1 ./...` | exit 0; 76 packages ok, no failing package |
| `git show 7092bcff -- internal/vm/vm.go` | no capability weakening: only request-ID/timer setup moved earlier and request_id added to early 400 body; route attenuation and child AllowedCaps unchanged |

Evidence: `milestone2/ticketdesk/evidence/round4_2026-10-19/` (raw storm output, clean-repro tail,
Go output); independent stdlib script at `milestone2/ticketdesk/evidence/b6_kill_storm.py`.
The script builds a fresh `git archive HEAD` as clean_repro does, seeds 1500 tickets through HTTP
(504,724-byte state, 300 status changes), restores that snapshot per trial, runs the documented server command
with default ceilings, and SIGKILLs an active DELETE loop after a seeded random 0.005–0.8-second delay.
Confirmed DELETE responses were checked against durable state after restart. The script's initial setup used
an unsupported status `closed`; that probe error was corrected to the contract's `done` before these 25 trials.

## Carryover

1. Complete the B6 delta validation: investigate the run-24 survivor-deletion timeout and obtain a fully passing
   independent kill storm. The script currently reports only `timed out`, so the exact DELETE id and whether
   the cause was the server or execution environment are unknown. Do not infer a new product blocker without evidence.
   Only lead: run 24 was itself the 11th run with an unindexed survivor (SUMMARY `orphan_runs=11`; 10 passing runs show `unindexed>=1`, and the counter is incremented before the recovery checks), so the hang followed cleanup DELETEs over a state containing an unindexed ticket.
2. Non-blocking minors remain: m12 port-probe race; m15 no migration of earlier state; m16 unindexed ticket
   after an unconfirmed create or delete interrupted between writes; m17 truncated-upload 400 still lacks the
   uniform error shape (now has a request_id in its body).
3. New non-blocking m18: existing dangling ids are tolerated, not scrubbed; DELETE returns 404 and cannot remove
   the stale index entry. This did not block any operation. R3.7's repair concern is satisfied for readable
   unindexed records by DELETE; stale missing-record entries themselves still need offline cleanup if desired.

---

# ROUND 3 (superseded by Round 4; kept as history), 2026-10-14, commit `4900538a`

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
