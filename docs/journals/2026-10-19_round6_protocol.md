# Round 6 protocol amendments (pre-run), 2026-10-19

These are Motoko's pre-Round-6 amendments, made after #189 merged (`c60e38e0`). Round 6 has **not** been run.

**Storm script** (`milestone2/ticketdesk/evidence/b6_kill_storm.py`). Pass criteria, seed, delays and the 10 s timeout
are unchanged.
* `stop()` falls back to SIGKILL if SIGTERM doesn't end the process within 5 s.
* `diag` records `/proc/PID/wchan`, `status` and each `task/*/stat` (plus the task's wchan) **before** the health probe
  and SIGQUIT. If SIGQUIT doesn't end the process within 5 s, diag logs it and SIGKILLs.
* After every snapshot restore, leftover `state.json.tmp-*` files are moved aside into the artifacts directory, with
  the run number and a timestamp in the name.
* Any failed run (not only timeouts) snapshots `state.json` and any `*.tmp-*` into `runNN_failure/`. If the server is
  still alive, the full /proc and SIGQUIT diag also runs.
* The script prints `SCRIPT_SHA256`, writes `script.sha256` to the artifacts directory and puts the sha in its SUMMARY
  line. `--expect-sha256` aborts on a mismatch.
* The script prints `CODE_SHA` (git HEAD). `--expect-head` aborts on a mismatch.
* `--self-test-sigstop` exercises the diagnostics on a SIGSTOPped process.

**Tests.**
* `milestone2_round6_pin_test.go`:
  * `TestRound6StormScriptPinned` (script sha256 must equal the pin in `MILESTONE2_DECISION.md`). Mutation-checked:
    appending one line to the script fails it.
  * `TestRound6StormScriptSigstopSelfTest`.
* Real-server check: I SIGSTOPped a live ticketdesk server.
  * The request timed out after 10 s.
  * diag printed state=T, `wchan=do_signal_stop` for all 6 tasks, and Threads=6.
  * The health probe timed out, then "sigquit: no exit in 5s; falling back to SIGKILL" (exit -9).
  * `state.json` and `state.json.tmp-999` were saved.
  * Separately, `stop()` on a stopped server returned in 5.0 s via SIGKILL.

**Protocol.** `MILESTONE2_DECISION.md` Round 6 is re-pinned:
* the new script sha256;
* exactly one `--runs 75` invocation;
* no SUMMARY line and exit 0 (harness crash, infra error, mismatch, interrupt) = NOT ACCEPTED, with no restart;
* the code-under-test SHA is pinned by a commit before the run, and must be a main commit that includes #191.

#190 is described as a *candidate* cause of run 24 everywhere.

**Next.** Merge #191 → pin the code-under-test SHA (a doc-only commit) → a fresh independent session runs Round 6 exactly as
registered.
