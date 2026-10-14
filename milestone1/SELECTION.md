# Milestone 1 application selection

> **FROZEN, dated 2026-10-13.** Written by the independent selector/evaluator under directive section 6 (Milestone 1), section 7.4, section 8, and ledger D24/D27. The implementing agent does not choose, reorder, shrink or reinterpret these applications; changes require a separately approved, dated revision.

## 1. The three applications

| # | Directory | Tool | Domain | Complexity tier |
|---|---|---|---|---|
| 1 | `01-ledgerlens/` | `ledgerlens` | Personal/small-business finance: bank-export CSV analysis (categorization by ordered regex rules, monthly and per-category summaries, budget checks, normalized CSV) | Moderate |
| 2 | `02-healthwatch/` | `healthwatch` | Service operations: declarative HTTP health probes with retries, JSON assertions, persisted state and transition events | Harder |
| 3 | `03-taskflow/` | `taskflow` | Build/automation orchestration: a task runner (Flowfile parser, quoting and variable substitution without a shell, dependency graph, governed process execution, report and bounded history) | Hardest realistic CLI |

Each directory has `requirements/REQUIREMENTS.md` (frozen spec), `requirements/ACCEPTANCE.md` (criteria mapped to requirement ids plus a test inventory) and `tests/acceptance/` (`run_acceptance.py`, shared `harness.py`, `launcher.example.sh`, `fixtures/`). Implementations go in `src/` and `build/` of the same directory (nothing exists there yet).

## 2. Invocation convention (all three)

Black-box tests drive the built artifact only through a **launcher**: `LAUNCHER GRANTS [APP_ARG...]`.

* `GRANTS` is ONE argument, the comma-separated `--allow-caps` list (`""` = no grants, then no `--allow-caps` is passed). Grants are always explicit per call, so denial tests simply pass fewer or no grants.
* Resource limits travel in the environment variable `HF_RUN_FLAGS` (for example `--max-fetch-bytes 20000`, `--deadline 1s`, `--max-exec-output-bytes 1000`, `--max-instructions 300`).
* Reference implementation: `howlframe run [--allow-caps "$GRANTS"] $HF_RUN_FLAGS <app>.hfbc "$@"` (`tests/acceptance/launcher.example.sh`; `HF_BIN`, `APP_ARTIFACT` select the binary and artifact). Build: `howlframe build src/<tool>.howl -o build/<tool>.hfbc`.
* Run a suite: `python3 milestone1/<NN>-<slug>/tests/acceptance/run_acceptance.py <launcher> [-v] [--only ID]`; `--list` prints test ids and the requirement ids each covers without needing a launcher. It prints `PASS|FAIL <test-id> [requirements] title`, then a per-requirement PASS/FAIL table, and exits non-zero on any failure (2 if nothing ran). Python 3 standard library only; HTTP tests use loopback servers on ephemeral ports.
* Default runner ceilings used by all functional tests: `--max-instructions 4000000000 --max-memory-bytes 4294967296 --max-call-depth 10000` (the runtime defaults of 100000 instructions / 64 MiB / depth 1000 cannot run realistic CLI workloads; this is a stated test condition, not an application requirement).

Totals: 35 + 29 + 32 = 96 black-box tests; each requirement id (R1..R18 / R1..R14 / R1..R15) is covered by at least one test.

## 3. Why the domains are substantially different

* **ledgerlens** is a pure data-in/data-out problem: strict text parsing (RFC 4180 subset, BOM, CRLF, line numbers), exact decimal arithmetic in integer cents, regex classification with priority semantics, grouping, sorting, text/JSON/CSV rendering. Its authority surface is the filesystem only.
* **healthwatch** is a networked, stateful, time-dependent problem: structured JSON configuration validation with exact error paths, outbound HTTP with methods/bodies/redirects, retry policy, JSON-path assertions over responses, a persisted state machine producing transition events, and secrets handling (`${ENV:}` in URLs without leaking). Its authority surface is network (per host:port), environment and filesystem.
* **taskflow** is a language-and-systems problem: a small text language with quoting and variable substitution that must be injection-proof, graph algorithms (post-order planning, cycle detection with line attribution), process execution with output capture and failure policies, persistent report and bounded history. Its authority surface is process (exact-argv allow-lists, environment pass-through), environment and filesystem.

The three share no data model, no I/O pattern, and no capability profile beyond filesystem.

## 4. Complexity progression

| Measure | ledgerlens | healthwatch | taskflow |
|---|---|---|---|
| Requirement ids | 18 | 14 | 15 |
| Acceptance tests | 35 | 29 | 32 |
| Commands | 4 | 4 | 5 |
| Persistent artifact formats | none (output files only) | state JSON (schema, lifecycle) | report JSON, history JSON (bounded) |
| Input grammars to implement | CSV (quotes), rules INI | JSON config with strict schema and error paths | Flowfile (directives, quoting, `${}` expansion) |
| Algorithms | stable multi-key sort, exact decimal aggregation | state machine, JSON-path resolution, retry loop | DFS post-order, cycle detection, skip propagation |
| Hard governed effects | read/write files | outbound HTTP per host, env, state preflight | exec with allow-list, env isolation, output limits, deadline |
| Failure-mode matrix | exit 0/1/2/65/66/70/73/77 | adds per-check DENIED/ERROR/FAIL semantics and zero-request guarantees | adds fail-fast vs keep-going, partial execution, nothing-executed-before-denial guarantees |

## 5. Capabilities exercised (and the security behaviors tested)

| Capability / runtime feature | ledgerlens | healthwatch | taskflow |
|---|---|---|---|
| `filesystem:read=<root>` scoping (incl. `..` and symlink escape, missing file vs denial) | yes | yes | yes |
| `filesystem:write=<root>` without read | `--out` | state file | report, history |
| `network:fetch=host:port` (coarse `network` insufficient; redirect-hop re-check; ungranted server never contacted) | no | central | no |
| `environment` | no (must work without) | `${ENV:}` | `${env:}` |
| `process:exec=` exact-argv allow-list, `process:env=` pass-through | no | no | central |
| stdin, stdout/stderr discipline, exit codes | yes | yes | yes |
| JSON parse/encode, persistence | JSON out | JSON in/out, state | JSON report/history |
| Clean denial (exit 77, one diagnostic, no raw runtime error, no side effects) | yes | yes (per-check DENIED, zero requests on state denial) | yes (nothing executed after denial) |
| Resource limits via runner flags | `--max-instructions`, `--max-memory-bytes` (+ app `--max-rows`) | `--max-fetch-bytes`, `--deadline`, `--max-instructions` (+ 500-check cap) | `--max-exec-output-bytes`, `--deadline`, `--max-instructions` (+ 1000-task cap) |

Every application also has an explicit "works with only the minimal grants" test and a positive control next to each limit/denial test, so a stub that always fails cannot pass them.

## 6. Why the selection is not cherry-picked for language ease

* The problems were chosen by what real users need, then checked for feasibility; nothing was simplified to fit the language. Requirements deliberately lean on weak spots found while reading the runtime: byte-exact argv handling, exact money arithmetic, exact JSON equality semantics, deterministic multi-key sorting, strict line-numbered diagnostics, injection-proof substitution, zero-side-effect denials, and failure-path output (see risks below).
* None duplicates `apps/` (cut, wc, head, grep, log_analyzer, kv, todo, ...). Those are single-purpose demos of one built-in; these are multi-command tools with strict specifications, persistence and security contracts. Nothing here is a Howl ecosystem product or depends on Howl projects (directive: at least one unrelated; all three are unrelated).
* Hard requirements were kept where the current runtime appears unable to meet them (section 7). They are not removed or softened, because the milestone's purpose is to expose real gaps.

## 7. Known feasibility risks (verified against `internal/vm/vm.go`, `docs/`, and probes with a locally built binary)

Verified facts:

1. **`exec` loses the child's output and exit code on failure.** `OpExec` returns combined stdout+stderr only on success; a non-zero exit becomes `IO_ERROR "exec failed: exit status N"` with no output. taskflow R6/R7 require showing a failing command's output and `FAILED (exit N)` (N is recoverable by parsing the message; the output is not). Likely needs a runtime enhancement (permitted: directive D22). stdout and stderr are merged into one stream (the spec accepts this).
2. **`read_line` cannot distinguish EOF from an empty line** (both return `""`) and strips `\r`. Reading all of stdin faithfully (blank lines, physical line numbers, stdin configs/Flowfiles) is therefore not reliably possible with existing built-ins. Affects ledgerlens R13/R2 (stdin), healthwatch R13, taskflow `-f -` (many tests feed multi-line stdin with blank lines). Expect a runtime addition (for example a read-all-stdin form or a nil-at-EOF `read_line`).
3. **`exec` requires canonical absolute program paths**; on usr-merged systems `/usr/bin/sh`, `/usr/bin/false` and others are symlinks and are refused. The taskflow suite therefore uses `os.path.realpath(sys.executable)`; applications must not assume `/usr/bin/...` exists as canonical.
4. **`stderr` appends no newline**; applications must add `\n` themselves (stated in the spec).
5. **`parse_json` needs a type hint** (`Any`) and untyped numbers (int vs float) may differ; healthwatch R4 requires numeric-value equality (`1` equals `1.0`) and JSON-type-strict equality.
6. **`regex_match` returns only a boolean** (no captures); `INVALID_REGEX` is raised for bad patterns and is catchable. Tokenizers and parsers must be hand written.
7. **Denials and limits are catchable** with `try_let`: `CAPABILITY_DENIED`, `LIMIT_EXCEEDED` (for example `http_request body exceeds maximum limit`), `INVALID_REGEX`, `IO_ERROR`, `PARSE_JSON` errors carry a JSON object text with a `code`. Applications must map them to the clean diagnostics required; the deadline and instruction/memory ceilings are not application-catchable in general (specified accordingly).
8. `(env NAME)` returns `""` for an unset variable (set/unset indistinguishable); `sleep` takes milliseconds; `time_now` has one-second resolution (used for state timestamps only).
9. The language offers no directory listing, stat, rename, delete, append or atomic write; all persistence is whole-file `write_file`. The specs avoid needing those (state is rewritten whole, "preflight" write proves writability before side effects).

Suspected or unverified (the requirement is kept; the evaluator expects friction):

10. `http_request` has no header, timeout or latency parameters (specs do not need them); support and behavior for `HEAD`, `PUT`, `PATCH`, `DELETE`, request bodies with non-ASCII text, redirects for non-GET methods, and 204/HEAD responses are unverified and exercised by healthwatch R3/R4.
11. `str_len`/`str_slice` semantics (documented as bytes, implemented via splitting on `""`, possibly characters): ledgerlens' 1000-byte description limit and byte-exact UTF-8 pass-through may expose differences.
12. Performance: `sort` is an insertion sort (O(n^2)); interpreted VM instruction counts for 5,000 CSV rows and 1,000 tasks are untested; default call depth is 1000 (the suite raises it to 10000; deep recursion for a 300-task chain is allowed but should be iterative where possible).
13. Concurrent redirect/DNS guard behavior on `localhost` vs `127.0.0.1`: the suites use IP-literal grants for 127.0.0.1 only.
14. The runtime is "capability-governed, not a sandbox" (docs/reference/network_authority.md). The suites test the runtime's logical checks and the applications' use of them; they do not claim host isolation (directive section 8).

## 8. Independence and integrity notes

* The suites encode only the written requirements; they know nothing about any implementation. Syntax was checked with `python3 -m py_compile` and `bash -n`. To catch internal contradictions (impossible expectations, wrong golden values), each suite was also run once against a private, throwaway reference model written from the specification text alone and kept outside the repository; those models are not deliverables, are not placed in `milestone1/`, and are not available to implementers. Passing the suites still requires a real HowlFrame implementation.
* Fixtures contain no secrets; test secrets are fabricated strings that the suites assert never appear in outputs.
* Nothing outside `milestone1/` was modified and no git state was changed.
