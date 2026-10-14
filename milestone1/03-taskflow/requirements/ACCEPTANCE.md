# ACCEPTANCE: taskflow

> **FROZEN, dated 2026-10-13.** Companion of `REQUIREMENTS.md`; changes require a separately approved revision (REQUIREMENTS.md 1.5).

## How acceptance is decided

1. Build the implementation from `src/` into `build/taskflow.hfbc` with `howlframe build` in a clean environment (no pre-existing artifacts, caches or generated code), and capture the commands and output as evidence (D24-D).
2. Provide a launcher that satisfies REQUIREMENTS.md 1.2 (the reference one is `tests/acceptance/launcher.example.sh`; set `HF_BIN` and `APP_ARTIFACT`).
3. Run the independent suite from a scratch directory outside the source tree:

```
python3 milestone1/03-taskflow/tests/acceptance/run_acceptance.py /path/to/launcher -v
python3 milestone1/03-taskflow/tests/acceptance/run_acceptance.py --list     # test ids and the requirements each covers
```

4. The application is accepted only if the suite exits 0 (every test PASS, every requirement PASS) **and** the independent reviewer confirms, from the preserved development history, that the implementation was produced without human-written implementation fixes (D24-E) and that no requirement or test was edited, weakened or removed (REQUIREMENTS.md 1.5). A self-reported pass is insufficient.
5. The suite needs only Python 3 (standard library) and, for the HTTP application, loopback networking. It uses ephemeral ports and temporary directories and removes them afterwards.

## Acceptance criteria by requirement

| Requirement | Acceptance criterion | Acceptance tests |
|---|---|---|
| R1 | Commands, option grammar (either side of operands), version/help, default and unknown task handling, -f -, usage errors with no I/O and nothing executed. | `T-R1-version-help`, `T-R1-usage-errors`, `T-R1-default-task-and-stdin` |
| R2 | Flowfile grammar, all load errors with FILE:LINE (lowest line first), cycles, unknown dependencies, CRLF/tab/comments, 1000-task limit. | `T-R2-parse-errors`, `T-R2-cycles`, `T-R2-filename-in-diagnostic`, `T-R2-valid-formats` |
| R3 | set/-D/${env:}/$$ semantics, recursive values, undefined variable and cycle errors found by run only, no re-expansion of inserted values. | `T-R3-variables-dry`, `T-R3-variable-errors`, `T-R4-argv-fidelity`, `T-R12-env-expansion-and-denial` |
| R4 | Quote handling and byte-exact argv delivery; no shell interpretation of any metacharacter; absolute-program rule; exit 65 before anything runs. | `T-R4-argv-fidelity`, `T-R4-no-shell`, `T-R4-program-checks`, `T-R11-env-isolation` |
| R5 | Deterministic post-order planning, de-duplication, list/plan output, 300-task chain. | `T-R1-default-task-and-stdin`, `T-R2-parse-errors`, `T-R2-cycles`, `T-R2-valid-formats`, `T-R5-plan-order`, `T-R5-scale-chain`, `T-R6-basic-run-golden` |
| R6 | Per-task output framing, line prefixing, combined stdout/stderr order, failure labels, SUMMARY line, exit codes. | `T-R4-program-checks`, `T-R6-basic-run-golden`, `T-R6-output-shapes`, `T-R7-fail-fast`, `T-R7-keep-going`, `T-R6-exit-and-unknown-failure-kinds`, `T-R11-process-denied`, `T-R15-determinism-and-hygiene` |
| R7 | Fail-fast vs --keep-going semantics, skip reasons, remaining commands of a failed task not run, child output shown on failure. | `T-R4-program-checks`, `T-R7-fail-fast`, `T-R7-keep-going`, `T-R6-exit-and-unknown-failure-kinds`, `T-R9-report`, `T-R11-process-denied`, `T-R11-exec-allow-list`, `T-R14-exec-output-limit` |
| R8 | --dry-run: exact would-run rendering and quoting, no execution, no env read, no report/history, no process grant needed. | `T-R3-variables-dry`, `T-R8-dry-run-semantics` |
| R9 | Report JSON content for ok/failed/skipped tasks, written even on failure, provisional write proves writability first. | `T-R9-report`, `T-R13-fs-denied` |
| R10 | History append, 50-record trim, history command, --last, missing/corrupt handling, validated before execution. | `T-R10-history`, `T-R13-fs-denied` |
| R11 | Exec only through granted process:exec entries (exact argv honored), FAILED (denied) with exactly one stderr line per denied task, children receive only process:env grants. | `T-R8-dry-run-semantics`, `T-R4-argv-fidelity`, `T-R11-process-denied`, `T-R11-exec-allow-list`, `T-R11-env-isolation`, `T-R13-minimal-grants` |
| R12 | ${env:} needs the environment grant, denial is 77 before anything runs or is written, plan/list/dry-run never read the environment. | `T-R8-dry-run-semantics`, `T-R12-env-expansion-and-denial` |
| R13 | Flowfile/history reads and report/history writes governed by scoped filesystem grants; denial before execution with no side effects; minimal-grant success. | `T-R2-filename-in-diagnostic`, `T-R13-fs-denied`, `T-R13-minimal-grants` |
| R14 | Task limit 70; exec-output limit => FAILED (output limit exceeded); deadline and instruction ceilings stop the run without hanging. | `T-R2-parse-errors`, `T-R14-exec-output-limit`, `T-R14-deadline`, `T-R14-instruction-limit` |
| R15 | Deterministic stdout, silent stderr on success, no secret leakage into stdout/stderr/report/history. | `T-R1-usage-errors`, `T-R5-scale-chain`, `T-R4-no-shell`, `T-R12-env-expansion-and-denial`, `T-R15-determinism-and-hygiene` |

## Test inventory

| Test id | Covers | Description |
|---|---|---|
| `T-R1-version-help` | R1 | version string; help usage text; needs no grants |
| `T-R1-usage-errors` | R1,R15 | usage errors: exit 2, no stdout, no I/O, nothing executed |
| `T-R1-default-task-and-stdin` | R1,R5 | default task used when none given; -f - reads stdin; -f file works with read grant |
| `T-R2-parse-errors` | R2,R5,R14 | every grammar violation: exit 65 with '-:LINE:' prefix, nothing on stdout |
| `T-R2-cycles` | R2,R5 | dependency cycles are load errors naming the cycle, even if unreferenced |
| `T-R2-filename-in-diagnostic` | R2,R13 | diagnostic names the -f argument exactly; missing file => 66 |
| `T-R2-valid-formats` | R2,R5 | CRLF, tabs, comments, blank lines, zero-run tasks, desc; list golden |
| `T-R5-plan-order` | R5 | post-order, declaration-order deps, no repeats, multiple/duplicate targets |
| `T-R5-scale-chain` | R5,R15 | 300 chained tasks planned correctly; 1000 tasks fine, 1001 => exit 70 |
| `T-R3-variables-dry` | R3,R8 | set, -D override, -D-only, recursion, $$, quoting contexts (dry-run shows expanded argv) |
| `T-R3-variable-errors` | R3 | undefined variable / cycles found by run (exit 65, FILE:LINE), not by plan/list |
| `T-R8-dry-run-semantics` | R8,R12,R11 | dry-run: nothing executed, no process/env grants needed, ${env:} stays verbatim, relative program rejected |
| `T-R4-argv-fidelity` | R4,R3,R11 | child argv equals the expanded words exactly (quotes, blanks, empty args, UTF-8, no re-splitting) |
| `T-R4-no-shell` | R4,R15 | shell metacharacters in arguments have no effect |
| `T-R4-program-checks` | R4,R6,R7 | relative program => 65 before anything runs; missing absolute program => FAILED (error) |
| `T-R6-basic-run-golden` | R6,R5 | golden output of a successful pipeline, exit 0 |
| `T-R6-output-shapes` | R6 | output without trailing newline, empty lines, interleaved stderr, no output, exit codes |
| `T-R7-fail-fast` | R7,R6 | fail-fast: first failure aborts, later tasks skipped (aborted), later commands of a failed task not run |
| `T-R7-keep-going` | R7,R6 | --keep-going: independent branches run; dependents skipped naming the first non-ok dependency |
| `T-R6-exit-and-unknown-failure-kinds` | R6,R7 | exit status 0 only when every task ok; summary counts |
| `T-R9-report` | R9,R7 | report JSON for ok and failed runs, written even on failure; plan order |
| `T-R10-history` | R10 | history append, formats, --last, trimming to 50, corrupt/missing handling |
| `T-R11-process-denied` | R11,R6,R7 | no exec grant: FAILED (denied), one stderr line per denied task, exit 77, nothing executed |
| `T-R11-exec-allow-list` | R11,R7 | per-program / per-argv grants honored; denial does not stop independent tasks with --keep-going; 77 outranks 1 |
| `T-R11-env-isolation` | R11,R4 | children get only process:env= variables; nothing else is inherited |
| `T-R12-env-expansion-and-denial` | R12,R3,R15 | ${env:} needs 'environment'; denial before anything runs; no secret leakage |
| `T-R13-fs-denied` | R13,R9,R10 | Flowfile/report/history access is governed; denials leave no side effects |
| `T-R13-minimal-grants` | R13,R11 | works with exactly: read dir + write dir + exec grant (no network/database/coarse process) |
| `T-R14-exec-output-limit` | R14,R7 | runner exec-output limit => FAILED (output limit exceeded), run continues per policy, oversize output not printed |
| `T-R14-deadline` | R14 | runner deadline stops a run with a running child promptly |
| `T-R14-instruction-limit` | R14 | tiny instruction ceiling stops the program non-zero without success output |
| `T-R15-determinism-and-hygiene` | R15,R6 | identical stdout across runs; stderr silent on success; no secret in report/history/stderr |

## Capability and resource evidence (D24-H)

The tests tagged with the capability requirement(s) in the table above exercise the actual runtime grants and denials (including denial with an empty grant list, scoped grants, and the absence of side effects after a denial) and the runner resource limits through `HF_RUN_FLAGS`. Preserve the suite output together with a `--receipt` of at least one denied run per application as additional evidence; the suite itself does not require receipts.
