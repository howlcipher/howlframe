# ACCEPTANCE: healthwatch

> **FROZEN, dated 2026-10-13.** Companion of `REQUIREMENTS.md`; changes require a separately approved revision (REQUIREMENTS.md 1.5).

## How acceptance is decided

1. Build the implementation from `src/` into `build/healthwatch.hfbc` with `howlframe build` in a clean environment (no pre-existing artifacts, caches or generated code), and capture the commands and output as evidence (D24-D).
2. Provide a launcher that satisfies REQUIREMENTS.md 1.2 (the reference one is `tests/acceptance/launcher.example.sh`; set `HF_BIN` and `APP_ARTIFACT`).
3. Run the independent suite from a scratch directory outside the source tree:

```
python3 milestone1/02-healthwatch/tests/acceptance/run_acceptance.py /path/to/launcher -v
python3 milestone1/02-healthwatch/tests/acceptance/run_acceptance.py --list     # test ids and the requirements each covers
```

4. The application is accepted only if the suite exits 0 (every test PASS, every requirement PASS) **and** the independent reviewer confirms, from the preserved development history, that the implementation was produced without human-written implementation fixes (D24-E) and that no requirement or test was edited, weakened or removed (REQUIREMENTS.md 1.5). A self-reported pass is insufficient.
5. The suite needs only Python 3 (standard library) and, for the HTTP application, loopback networking. It uses ephemeral ports and temporary directories and removes them afterwards.

## Acceptance criteria by requirement

| Requirement | Acceptance criterion | Acceptance tests |
|---|---|---|
| R1 | Command/option grammar, version/help, usage errors with no I/O and no network. | `H-R1-version-help`, `H-R1-usage-errors` |
| R2 | Every configuration rule has a distinct error path; validation precedes any I/O; defaults merge correctly; validate needs no network/env. | `H-R2-validate-ok`, `H-R2-validate-errors`, `H-R2-run-validates-before-io`, `H-R2-defaults-merge`, `H-R7-state-corrupt` |
| R3 | Sequential execution, result model (PASS/FAIL/ERROR/DENIED, http_status, attempts, reasons), redirects followed, report formats, no state/events without --state. | `H-R2-defaults-merge`, `H-R3-basic-golden`, `H-R8-json-format`, `H-R3-no-state-no-events`, `H-R4-expectations`, `H-R4-methods-and-bodies`, `H-R5-retries`, `H-R12-network-denied`, `H-R12-mixed-and-redirect` |
| R4 | Expectation order and semantics (status list, body_contains, json_assert path/equality/exists/typing), configured method and body observed by the server. | `H-R2-defaults-merge`, `H-R3-basic-golden`, `H-R4-expectations`, `H-R4-methods-and-bodies` |
| R5 | Retry counts, stop on first PASS, delays honored, denied never retried, server-side request counts exact. | `H-R5-retries`, `H-R5-retry-delay` |
| R6 | One failing, refused, oversized or denied check never stops the others. | `H-R6-resilience-and-size-limit`, `H-R12-mixed-and-redirect` |
| R7 | State lifecycle: new/down/recovered/removed events, since/last_run/consecutive_failures bookkeeping, schema validation (exit 65, untouched), preflight (zero requests when state not readable/writable), DENIED leaves state untouched. | `H-R2-run-validates-before-io`, `H-R3-basic-golden`, `H-R3-no-state-no-events`, `H-R7-state-lifecycle`, `H-R7-all-fail-first-run`, `H-R7-state-corrupt`, `H-R7-preflight-denied`, `H-R7-denied-keeps-state`, `H-R11-deadline` |
| R8 | Exact text and JSON report layout and ordering; no URLs, hosts or bodies in output. | `H-R3-basic-golden`, `H-R8-json-format`, `H-R7-state-lifecycle` |
| R9 | status command output, ordering, --fail-if-down, missing/corrupt handling. | `H-R9-status` |
| R10 | ${ENV:NAME} expansion in url/body, undefined => 65, no environment grant => 77 before any request, secrets never leak. | `H-R10-env-interpolation` |
| R11 | >500 checks => 70; --max-fetch-bytes => response_too_large; --deadline and instruction ceilings stop the run, state untouched, no hang. | `H-R6-resilience-and-size-limit`, `H-R11-check-limit`, `H-R11-deadline`, `H-R11-instruction-limit` |
| R12 | Per-host network:fetch grants (coarse network is insufficient), redirect-hop re-check, filesystem and environment denial => 77, scoped reads, minimal-grant success. | `H-R2-validate-ok`, `H-R7-preflight-denied`, `H-R7-denied-keeps-state`, `H-R12-network-denied`, `H-R12-mixed-and-redirect`, `H-R12-fs-denied`, `H-R12-minimal-grants` |
| R13 | Config on stdin with only network grants works. | `H-R3-no-state-no-events`, `H-R12-minimal-grants` |
| R14 | Prefix on every diagnostic, one denial line per denied check, silence on clean runs, no leaks. | `H-R1-usage-errors`, `H-R2-validate-errors`, `H-R3-basic-golden`, `H-R10-env-interpolation`, `H-R12-network-denied`, `H-R12-fs-denied`, `H-R14-hygiene` |

## Test inventory

| Test id | Covers | Description |
|---|---|---|
| `H-R1-version-help` | R1 | version string; help usage text |
| `H-R1-usage-errors` | R1,R14 | usage errors: exit 2, no stdout, no I/O, no network, no state |
| `H-R2-validate-ok` | R2,R12 | validate accepts good configs; needs only file read; stdin works with no grants |
| `H-R2-validate-errors` | R2,R14 | every config rule yields exit 65 with 'config: <path>:' and nothing else on stdout |
| `H-R2-run-validates-before-io` | R2,R7 | run reports config errors before any request or state change |
| `H-R2-defaults-merge` | R2,R3,R4 | defaults are inherited and overridden per check |
| `H-R3-basic-golden` | R3,R4,R7,R8,R14 | golden text report for the basic config with fresh state; exit 1 |
| `H-R8-json-format` | R8,R3 | JSON report shape |
| `H-R3-no-state-no-events` | R3,R7,R13 | without --state: no events, no state files, only network grant needed (config on stdin) |
| `H-R4-expectations` | R4,R3 | status lists, body_contains, json_assert semantics and reasons |
| `H-R4-methods-and-bodies` | R4,R3 | method and body observed by the server are exactly the configured ones |
| `H-R5-retries` | R5,R3 | retry counts, attempts, stop at first PASS, request counts at the server |
| `H-R5-retry-delay` | R5 | retry_delay_ms pauses between attempts |
| `H-R6-resilience-and-size-limit` | R6,R11 | refused / oversized / bad checks never stop the run; fetch-size limit => response_too_large |
| `H-R7-state-lifecycle` | R7,R8 | new/down/recovered/removed events, since & consecutive_failures bookkeeping across runs |
| `H-R7-all-fail-first-run` | R7 | state is written even when every check fails; ERROR results recorded with null http status |
| `H-R7-state-corrupt` | R7,R2 | invalid state files => exit 65 before any request, file left untouched |
| `H-R7-preflight-denied` | R7,R12 | state not readable/writable => exit 77 with ZERO requests sent |
| `H-R7-denied-keeps-state` | R7,R12 | DENIED checks leave existing state untouched, add nothing, emit no events |
| `H-R9-status` | R9 | status command: sorting, formats, --fail-if-down, missing/corrupt |
| `H-R10-env-interpolation` | R10,R14 | ${ENV:} expands in url/body, secrets never leak to stdout/stderr/state |
| `H-R11-check-limit` | R11 | more than 500 checks => exit 70, no requests |
| `H-R11-deadline` | R11,R7 | runner deadline stops a slow run promptly, state untouched |
| `H-R11-instruction-limit` | R11 | tiny instruction ceiling stops the program with non-zero exit |
| `H-R12-network-denied` | R12,R3,R14 | no fetch grant: every check DENIED, full report, exit 77, zero requests, no retries |
| `H-R12-mixed-and-redirect` | R12,R3,R6 | per-host grants; DENIED outranks FAIL in the exit code; redirect hop to ungranted port is DENIED |
| `H-R12-fs-denied` | R12,R14 | config read denied/out-of-scope => 77 and zero requests; missing config => 66 |
| `H-R12-minimal-grants` | R12,R13 | success with exactly: read dir + write dir + fetch grant (no process/database/network-serving) |
| `H-R14-hygiene` | R14 | diagnostics prefixed; no URLs/hosts/bodies in any output; clean runs are silent on stderr |

## Capability and resource evidence (D24-H)

The tests tagged with the capability requirement(s) in the table above exercise the actual runtime grants and denials (including denial with an empty grant list, scoped grants, and the absence of side effects after a denial) and the runner resource limits through `HF_RUN_FLAGS`. Preserve the suite output together with a `--receipt` of at least one denied run per application as additional evidence; the suite itself does not require receipts.
