# 2026-10-19: ticketdesk B6 Round 7 protocol (pre-registration, no run)

## Context
Round 6 (#197) was NOT ACCEPTED, with 1/75 timeouts (run 11). Motoko's review of #197 showed that the client's
own 10 s socket timer fired late in runs 11 and 37. In run 61, the client stalled for 34.6 s while the server's
longest handler took 73 ms. `dmesg` showed a VM fork and balloon reclaim during run 62. Runs 38 and 62 lost about
200 s of wall time each. A box pause is the best-supported explanation, but it is not proven for run 11.

## What changed
- **Storm script** (`b6_kill_storm.py`, re-pinned): a heartbeat sidecar, a noise sampler sidecar (/proc/stat, PSI,
  cgroup, diskstats, pgmajfault, /dev/kmsg), a per-request client log, an in-flight watchdog with schedstat
  deltas, a 10 s total deadline, a 120 s idle gate (exit 3, not an attempt), INFRA-VOID noise gates, and mechanical
  CLASSIFY and VERDICT lines. The server runs with `--timing-log` and `GODEBUG=gctrace=1`.
- **#194 folded in:** the SIGSTOP self-test waits for state T, and the pin-line match is byte-exact.
- **Pin rules:** the pin line is now `Round 7 code-under-test SHA`. `TestRound7ExpectHeadPinRules` adds a
  trailing-space attack. `--self-test-noise` runs in CI (`TestRound7StormScriptNoiseSelfTest`).
- **Harness smoke:** a scratch copy with a 5 s gate ran `--runs 2`. It was not a Round 7 run: 2/2 PASS,
  noise=QUIET, about 6 MB of artifacts per run.

## Next
1. Motoko reviews the `--timing-log` PR and this protocol PR.
2. Merge the timing PR, then this PR.
3. Pin the code under test with one squash commit that changes only the pin line.
4. A fresh independent session runs exactly one `--runs 225`, in a quiet window or on a dedicated machine.

The shared box showed further VM-fork records (uptime 11539 s and 11851 s) while this was written. Expect the idle
gate to refuse a noisy box.
