# Issue #181: exec deadline containment (PARKED)

Date: 2026-10-19
Branch: `exec-deadline-containment` (off origin/main 7092bcf)
Status: **Parked under HOWLFRAME_PRODUCT_DIRECTIVE.md** until an accepted
application needs `process:exec`. No PR opened; no code changes yet.

## Why parked

#181 isn't tied to a demonstrated milestone blocker. Under the directive,
security hardening for a capability no accepted application uses waits until
an accepted application needs it.

## Survey done before parking (for whoever resumes)

Exec call sites with a deadline:
- `internal/vm/vm.go` `OpExec` (~L2608-2650): `exec.CommandContext` on a child
  of `vm.ctx`. `cmd.WaitDelay = 100ms` is already set when `vm.ctx` has a
  deadline, but there's no `SysProcAttr`, `Setpgid` or group kill. `cmd.Cancel`
  kills only the direct child, so `setsid` and background grandchildren survive
  the deadline.
- `-deadline` (`resourcePolicy`, howlframe.go) feeds only the bytecode VM
  (`-run-bc`, `run --target bytecode`).

Exec call sites with no deadline today (so no deadline to contain):
- AST interpreter `internal/vm/vm.go` ~L866 (`exec.Command(...).CombinedOutput()`).
- gogen helper `howlFrameExec` (`internal/backend/gogen/gogen.go` ~L623).
- JS backend `spawnSync` (`internal/backend/javascript/javascript.go` ~L397).

Planned approach (from Lain):
1. `SysProcAttr{Setpgid: true}` plus `cmd.Cancel` doing `kill(-pgid, SIGKILL)`.
2. Keep `cmd.WaitDelay`.
3. Contain setsid escapees, which leave the pgid. Candidates: a PID namespace
   (`CLONE_NEWPID`/`CLONE_NEWUSER`; not always available, e.g. in
   unprivileged containers), `prctl(PR_SET_CHILD_SUBREAPER)` plus reaping and
   killing reparented descendants (Linux 3.4+, no privileges; but it's
   process-wide, so it needs care inside the runner), or a cgroup v2
   `cgroup.kill` (needs a delegated cgroup). Fail-closed: if the chosen
   mechanism isn't available, refuse `process:exec` under a deadline rather
   than run uncontained.
4. Regression tests: a setsid grandchild holding stderr is dead after the
   deadline; a setsid grandchild with no output is dead after the deadline;
   the deadline still fires on time. Skip only on non-Linux or where the
   mechanism is unavailable, and say so in the skip message.

Scorecard sections it would move: security, correctness.
