# taskflow: independent final review (D24-J)

Reviewer: independent final reviewer (did not write or help write any implementation).
Date: 2026-10-13. Reviewed tree: HEAD 2a95d364, against the frozen `requirements/REQUIREMENTS.md`.

## DECISION: ACCEPTED

No violation of a written requirement found after about 120 extra black-box probes, including
argv fidelity with hostile `-D` values, exact error lines, failure policies, denial behavior and
report/history files. Minor defects (notably a provisional-report side effect, TF-m1) are
carried over.

## Freeze check

`git diff 78bcf393 HEAD -- 'milestone1/*/requirements' 'milestone1/*/tests'` is empty.

## D24 checklist

| ID | Result | Evidence |
|---|---|---|
| A frozen requirements | PASS | 78bcf393 (17:10) precedes the implementation commit 2a95d364 (18:17); frozen paths unchanged. |
| B source + standalone artifact | PASS | `src/taskflow.howl` (718 lines, read in full): lexer, loader, cycle detection, planner, execution; no hard-coded fixtures. Runs as `HFBC` on the standalone VM. Child processes are the declared product behavior, launched only through governed `exec`; no shell. |
| C independent acceptance tests | PASS | 32 tests by the selector, unmodified. |
| D clean-environment repro | PASS | `bash milestone1/scripts/clean_repro.sh 03-taskflow`: `32 tests, 32 passed, 0 failed`, 18 s wall. Same caveat (fresh directory from the committed tree, host toolchain reused). |
| E autonomous implementation/repair | PASS (plausible) | Journal names two blockers (child exit/output lost; fixed-arity `exec`) matching the runtime diff in 2a95d364 (`ExecExitCode`/`ExecOutput`, `execArgv`) with mutation-checked tests, plus the usage-error prefix defect it found and fixed in all three apps. Coherent with the diff; one coarse commit. |
| F documentation + workflow | PASS | I wrote my own Flowfile and followed the README: `list`, `plan`, `run`, `--dry-run`, `--keep-going`, `--report`, `--history`, `history --last`, `-f -`, grants as documented (`process:exec=/usr/bin/python3.14 **`, canonical path, symlink refusal noted). Behaviors match the README. |
| G regression tests + limitations | PASS | Frozen suite plus Go tests `internal/vm/exec_exit_test.go`. Limitations documented honestly (recursion depth vs `--max-call-depth`, UTF-8 replacement in failed-child output, denied command not counted in `commands`, `-D` literal). |
| H capability/security | PASS | Verified: unlisted program -> `FAILED (denied)`, exactly one stderr line `taskflow: capability denied: process (<task>)` with no program or arguments echoed, exit 77 outranking 1, never retried; coarse `process`, `network,environment,database` grants do not authorize exec; argv pattern grants (`-c *`) deny a different argv; `${env:}` without `environment` -> 77 `capability denied: environment` before anything ran and no report or history created; dry-run reads no environment and needs no process grant; filesystem read denied -> 77, write denied (report, history) -> 77 with no command run and no file created in the denied-first-preflight case; child environment holds only `process:env=` names (the extra `LC_CTYPE` seen is set by Python itself, not inherited); `--max-exec-output-bytes` -> `FAILED (output limit exceeded)` with none of the output printed; deadline kills a 30 s child promptly (2.04 s); 1001 tasks -> 70. |
| I development history | PASS with note | 2a95d364, journal, handoff, evidence; coarse granularity. |
| J independent decision | ACCEPTED | See above. |

## Adversarial black-box checks (python3.14 test children; grants per call)

1. argv fidelity: one run proving `"${A}"` and bare `${A}` stay single arguments (`x y`), `a"b c"'d'` -> `ab cd`, `""` and `''` empty arguments, an expansion to empty stays an argument, `\\ \q \"` inside double quotes, `;`, `|`, `$(id)`, backticks, `*`, `>o` literal, `"$$HOME"` -> `$HOME`, value containing `'` and `"` appended. PASS.
2. `-D` value with newline, `;|$(id)`, backticks, `*`, quotes, UTF-8 `é`, trailing `=`, arrives byte-exact; repeated `-D A=` last wins; `-D C=c=d`; `-D` empty value. PASS. `-D` value containing `${B}` is inserted literally (not expanded); see TF-m3.
3. Variable errors found by `run` only: `undefined variable C` at the run line; undefined inside a `set` value reported at the `set` line; cycle `A -> B -> A` at the first set line; `plan`/`list` never expand. PASS.
4. Load errors with `FILE:LINE`: `$x`, `${a b}`, `${}`, `${env:}`, `${env:1}`, `${A`, trailing `$`, unterminated `"` and `'`, `run` with no program, unknown directive (indented and column 0), duplicate `desc`, indented before task, duplicate task/set, `task a b`, `task t after` (no deps), reported lowest line first. PASS.
5. Cycles: self (`t -> t`), 2-node, 3-node (`a -> b -> c -> a` at line 1); rotation to the first-in-file member (`b -> c -> a -> b`); cycle in a task nobody requests still errors; cycle beats later unknown dependency by line. PASS.
6. Planning: diamond planned once, duplicate targets (`plan default default` -> one line), DFS post-order (`d,a,e,b,c,default`), `list` byte-order sort (`B`,`Z` before `a-b.c_d`), `-` for no deps/desc, tab-indented body, CRLF Flowfile, `# comment` inside a body, `#` mid-line is ordinary text, no `default` task -> 2 with exact text, unknown task -> 2 with exact text, 300-task chain in 0.5 s, 900-task chain even at the default call depth. PASS.
7. Execution output: no trailing newline gets terminated, blank lines become `| `, zero-run task prints only markers, exit 3/255 -> `FAILED (exit N)` with prior output shown, signal death -> `FAILED (error)`, successful binary output byte-exact, commands after a failure in the same task not run (touch file never created). PASS.
8. Failure policy: fail-fast gives `skipped (aborted)`; `--keep-going` gives `skipped (dependency c failed)` naming the first non-ok dependency; middle denied task with keep-going; report JSON per task (`exit`, `reason`, `commands`, targets) and history record. PASS.
9. Report/history: failed run still writes both; history trimmed to 50 from a 60-entry file; `history --last 3`, options after operand, `--last 0`/`x`/duplicate -> 2, missing file 66, invalid record -> 65; `-f -` for run/list; report in missing dir 73 before running; report denied -> 77 nothing executed; history denied -> 77. PASS except TF-m1.
10. Dry-run: single-quote escaping (`it'\''s`, `''`, `'$'`, tab), bare charset `x=y,z@%+:/-.`, `${env:...}` stays verbatim without environment grant, relative program still 65. PASS.
11. Usage matrix (about 30 cases): unknown options, `-fx`, `--file=x`, `-Dx`, `-D 1x=2`, missing values, duplicates, `--last` on run, `-D` on list, `--report` on plan, `history` operand counts, `version -f x`, `help x`, empty `-f` value: exit 2, exactly one `taskflow: ` line. PASS.
12. Resource limits: `--max-exec-output-bytes 1000` (200 B ok, 5000 B fails, run continues with keep-going), `--deadline 2s` against a 30 s child, `--max-instructions 3000`, `--max-call-depth 300` on a 900 chain: non-zero, no success text; stderr is the runtime's raw JSON (platform, TF-m2).
13. Determinism: identical stdout across repeated runs; stderr empty on successful runs.

## Defects (none are blockers)

| Id | Severity | Description |
|---|---|---|
| TF-m1 | minor | Preflight order: the provisional `--report` document is written before the history file is validated and proven writable. With an invalid history (65) or a denied history write (77) the report file has already been created or overwritten with `{"version":1,"status":"running",...}` (I replaced a pre-existing report that way). No command runs, so R10 "nothing executed" holds, but R13 "no file created or modified" is not met in this mixed case. The spec's own R9+R10 preflights make a fully clean outcome impossible when the history file is absent; for an existing history, validating it first would avoid the clobbering. |
| TF-m2 | minor (platform) | Runner-level stops print the runtime's raw JSON error object on stderr, without the `taskflow: ` prefix. Needs runtime/spec clarification; not fixable in the app (`wall-clock deadline` during `exec` is converted to `taskflow: deadline exceeded`, exit 70, but other stop points are not interceptable). |
| TF-m3 | minor (interpretation) | `-D` values are inserted literally; spec 3.2 says variable values may contain `${...}` (expanded at use). Safe and documented; the acceptance suite does not decide it. |
| TF-m4 | minor | The `commands` count excludes a denied command (documented); `get_env` and `read_path` map every non-denial failure to 77/66. Backward-looking: a failing child's combined output order is runtime-provided (stderr text appeared before stdout text for a failed child). |
| TF-m5 | minor | Commit history coarse; artifact not committed. |

## Runtime changes used by this app (commit 2a95d364)

* `(exec program list)`: a single trailing list argument is the whole argument vector. Implemented by `execArgv` before `requireExec`/`AuthorizeExec`, in both interpreter and bytecode paths, so the allow-list is checked against the spread vector (test denies a 4-element vector against a 2-element pattern; interpreter/bytecode parity asserted). Narrow; behavior change only when exactly one trailing argument is a list value (previously stringified). Generated Go/JS do not implement it (documented), so no capability difference there.
* Catch-visible `exit_code`/`output`: carried in `VMError` fields tagged `json:"-"`, exposed only through `CatchString()` used only in the `try_let` handler of the bytecode VM; `Error()`, receipts and evidence are unchanged (test asserts an uncaught failure and stderr never contain the child output, signal death has no `exit_code`, success path unchanged). Output is charged to the memory budget. The interpreter (`-run`) path does not provide `exit_code` to catch (not needed for the standalone VM). Capability enforcement is unchanged: the denial check runs before spawn. Tests are mutation-plausible (they assert the exact JSON fields and the absence of output in the uncaught path).
* Test run: `go test ./internal/vm ./internal/ast ./internal/bytecode ./internal/construct ./internal/checker`: ast, bytecode, construct, checker pass; `internal/vm` fails only the 6 pre-existing exec/symlink tests (`TestInterpretExecGrantedRuns`, `TestHFIRBytecodeExecFixture`, `TestHFIRBytecodeExecOperandOrder`, `TestHFIRBytecodeNestedExecFixture`, `TestHFIRBytecodeNestedMultiEffectFixture`, `TestReceiptEnvironmentAndExec`). Full `go test ./...`: failures only in `internal/backend/gogen` (4), `internal/backend/javascript` (3), `internal/vm` (6) and `tools/difftest` (`TestLoweredHFIRABIConformance`, 4 exec-related subtests): 14 top-level failures, all exec/symlink related, none new.

## Summary

The hardest app holds up under hostile inputs: no shell, no re-splitting, exact error and output formats, correct failure policies and capability denials. ACCEPTED with the minor carryover items above.
