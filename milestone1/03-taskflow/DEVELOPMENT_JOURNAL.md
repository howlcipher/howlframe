# taskflow development journal (Milestone 1, application 3)

Implementer: Claude (Sonnet 5.5), autonomous, from the frozen requirements and acceptance
suite written by the independent selector. No human wrote or repaired implementation code.

## 2026-10-13

1. Read `REQUIREMENTS.md` and the whole acceptance suite. The selector had flagged two runtime
   risks (SELECTION.md section 7); both were real blockers for this app.
2. **Blocker 1 (R6/R7, failed child loses its output and status):** `exec` on a non-zero
   exit raised only `exec failed: exit status N`, discarding the child's output (the VM had it in
   `output.buffer`). Narrow fix: `VMError` carries `ExecExitCode`/`ExecOutput`, exposed ONLY to a
   `try_let` catch binding through `CatchString()`; `Error()`, receipts, evidence and server logs
   are unchanged. Test `TestExecNonZeroExitExposesStatusAndOutputToCatch` (also asserts an uncaught
   failure never prints the child output); mutation-checked twice (drop output, drop exit code).
3. **Blocker 2 (R4, argv of unknown length):** `exec` takes a compile-time argument count. A
   fixed-arity dispatch would cap commands at an arbitrary length, so `(exec program list)` was
   defined: exactly one trailing list is the argument vector. Grants are checked against the final
   vector. Test `TestExecListIsArgumentVector` (bytecode and interpreter parity, allow-list denial
   on the spread vector), mutation-checked by disabling the spread.
4. Wrote `src/taskflow.howl`: Flowfile loader that keeps scanning after the first syntax error so
   unknown dependencies and cycles can still win by line number, DFS cycle detection with
   rotation to the first-defined member, a character-level word/quote/`$` lexer shared by
   validation, expansion and dry-run (modes 0/1/2), memoized variable expansion with cycle
   detection, plan/list, governed execution with fail-fast and keep-going, report and history.
5. First full run: 31/32. The failure was mine: usage errors printed a second stderr line without
   the `taskflow: ` prefix (R15). The same latent defect existed in ledgerlens and healthwatch
   (their suites do not check every usage-error line); all three now print one prefixed line.
   `no task given...` and `unknown task: NAME` were restored to their exact spec text.
6. Final: taskflow 32/32, ledgerlens 35/35, healthwatch 29/29.

## Test impact assessment

* Behavior changed: `exec` list argument; catch value after a failed child; fix of `starts_with`
  (earlier, ledgerlens).
* Existing tests re-run: all internal/vm exec, receipt and resource-limit tests, plus the full
  regression gate (see baseline journal for the 14 known host failures, unchanged).
* New coverage: `internal/vm/exec_exit_test.go`.

## Review round 1 (2026-10-13)

Accepted by the independent reviewer. Minor fixes applied: the provisional `--report` is now written
only after the history file is validated and proven writable (an invalid history no longer leaves or
overwrites a report), and a memory/limit stop during a file read exits 70 (was 66).
