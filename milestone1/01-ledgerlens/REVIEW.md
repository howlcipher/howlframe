# ledgerlens: independent final review (D24-J)

Reviewer: independent final reviewer (did not write or help write any implementation).
Date: 2026-10-13. Reviewed tree: HEAD 2a95d364. Reviewed against the frozen
`requirements/REQUIREMENTS.md` (not only the acceptance suite).

## DECISION: NOT ACCEPTED

One reproducible violation of an explicit written requirement (BLOCKER LL-1, rules error
ordering, REQUIREMENTS 3.2 / R5). It is small and cheap to fix, but the requirement is
unambiguous and the program does not meet it. Re-review after the fix is sufficient (the rest of
the package is in good shape; see below).

## Freeze check

`git diff 78bcf393 HEAD -- 'milestone1/*/requirements' 'milestone1/*/tests'` is empty (no
output). Requirements and the suite are unchanged since the freeze commit.

## D24 checklist

| ID | Result | Evidence |
|---|---|---|
| A frozen requirements | PASS | `requirements/REQUIREMENTS.md`, `ACCEPTANCE.md`, suite and fixtures all added in 78bcf393 (17:10), implementation first appears in 86ae075c (17:56). Diff of frozen paths since 78bcf393 is empty. |
| B source + standalone artifact | PASS | `src/ledgerlens.howl` (476 lines, read in full: no hard-coded fixtures, no special-casing of test inputs). `howlframe build` produces an `HFBC` bytecode file; it runs with `env -i PATH=/nonexistent howlframe run ledgerlens.hfbc version` (no Go, Python, JS or helper program at run time). Note: `build/` is gitignored, so the artifact is produced by the build step rather than committed (minor, LL-m5). |
| C independent acceptance tests | PASS | 35 tests, authored by the selector in 78bcf393, unmodified. |
| D clean-environment repro | PASS | `bash milestone1/scripts/clean_repro.sh 01-ledgerlens` (git archive HEAD into a temp dir, go build, howlframe build, suite): `35 tests, 35 passed, 0 failed`, wall time 13 s. Caveat: "clean" means fresh directory from the committed tree; it reuses the host Go toolchain and module cache (not a fresh container). |
| E autonomous implementation/repair | PASS (plausible, not independently provable) | `DEVELOPMENT_JOURNAL.md` records two real blockers (stdin EOF, `starts_with` infinite loop) with root causes; both are visible in the diff of 86ae075c (`read_stdin` opcode, `desugarStartsEndsWith`) and each has a Go regression test. Journal and diff are consistent. Commit granularity is coarse (one commit holds source, journal, evidence and runtime changes), so repair iterations are only narrated, not reconstructable from history. |
| F user documentation + workflow | PASS | I followed the README with my own data (Blue Bottle/Starbucks/PAYROLL, rules with `(?i)` and a budget): `build`, `run --allow-caps filesystem:read=$PWD ... summary --rules rules.ini txns.csv` gave the documented tables, exit 1 for the exceeded budget; `--format json --out rep.json` with a write grant wrote the file and left stdout empty; `categorize --rules rules.ini -` from a pipe produced the normalized CSV. README claim "Rules errors found after a syntax error line are not reported (first syntax error wins)" is inaccurate; see LL-1. |
| G regression tests + honest limitations | PASS with notes | The frozen suite is the regression suite; Go regression tests exist for `read_stdin` and `starts_with`. Known limitations are documented (rune-based length, non-ASCII literals, memory-bound output). Two undocumented deviations found (LL-1, LL-m1). |
| H capability/security verification | PASS | Verified myself: read denied outside root (77, even for a nonexistent file, no probe); write denied with read-only grant (77, file not created); read not implied by write grant (77); `version`/usage/stdin without grants works; `--out` into a missing directory 73; `--out` onto a directory 73; no network/process/env use. Runner limits: instruction and deadline limits stop the program with empty stdout. |
| I development history | PASS with note | Commits 78bcf393, 86ae075c (+ later shared-script commits), journal, handoff, evidence file present. Coarse commits (see E). |
| J independent decision | NOT ACCEPTED | See blocker LL-1. |

## Adversarial black-box checks I ran (scratchpad scripts, launcher convention of 1.2)

All run through `tests/acceptance/launcher.example.sh` with the generous ceilings.

1. Rules error precedence (3.2 "the first error found (lowest line number) is reported"): unknown budget category at line 4, malformed line at line 6. **FAIL**: reports line 6 (LL-1).
2. Budget value with 13 integer digits (`1234567890123.00`): rejected `invalid budget amount`; spec regex `[0-9]+(\.[0-9]{1,2})?` has no digit cap. Deviation, minor (LL-m2).
3. Dates: 1900-02-29 invalid, 2000-02-29 valid, 2100-02-29 invalid, 2999-12-31 valid, 3000-01-01/1899-12-31/2026-04-31/26-01-01/2026-1-5/full-width digits all invalid. PASS.
4. Amounts: `.5`, `5.`, `1e3`, `1,000.00` (field count), `$5`, `1.234`, `1000000000000` rejected; ` 5.00 `, `+0`, `-0`, `-0.00`, 12-digit values accepted and canonicalized (`0.00`, exact cents). PASS.
5. BOM + CRLF + header `AMOUNT , Foo,Date,Description` (case/order/space/extra col) + blank physical lines + `"a,""b"" c"`. PASS (exact bytes in categorize output).
6. Line numbering with blank lines: invalid date on physical line 5 reported as `line 5`; strict mode left no `--out` file; `--lenient` skipped 1 row with the stderr summary line. PASS.
7. Description byte limit: 250 four-byte emoji accepted, 251 rejected; 1000 ASCII accepted, 1001 rejected; 500 two-byte chars accepted, 501 rejected; invalid UTF-8 bytes in a description pass through. PASS.
8. Usage matrix (exit 2, one `ledgerlens: ` line, nothing on stdout): `--month 2026-1`, `--month 2026-13`, `--month 1899-12`, two operands, `categorize` without `--rules`, `--format=json`, `--format xml`, `version extra`, no command, missing option value, duplicate `--lenient`/`--rules`, `--month` on categorize, `--rules` on check-rules, `--max-rows 0` and `10000001`, `help extra`, unknown command, `Version`. PASS. `help`/`-h`/`--help` print `usage: ledgerlens ...` on stdout, exit 0. PASS.
9. Stdin: CRLF stdin file, empty stdin and blank-only stdin (`empty input`, 65). PASS.
10. Budget semantics: spent equal to limit is OK, refund does not reduce spent, `Uncategorized` budget, budget rows for months with no spending, months with only income. PASS. `--month` with no match: zero TOTAL, no month rows, empty budget table (text) and `[]` (json). PASS.
11. Sorting: category rows ordered by month, expenses desc, byte-order names (`B` before `b`), rule priority across duplicate names (first line wins). PASS.
12. Header errors: duplicate `date`, missing `amount`, unterminated quote in header, header-only file (valid, zero rows). PASS. Row errors: junk after closing quote, quote in unquoted field, spaces-only line (field count). PASS.
13. Capability paths listed under H, plus path outside root vs missing file inside root (77 vs 66), directory as INPUT (66). PASS.
14. `--out` semantics: existing file untouched on strict error (`old` kept), exit code 1 preserved with `--out` for an exceeded budget, output file content equals stdout bytes. PASS.
15. Resource limits: `--max-instructions 5000` and `--deadline 1ms` on a 3000-row file: empty stdout, non-zero exit, but stderr is the runner's raw JSON (`{"phase":"runtime",...}`) without the `ledgerlens: ` prefix. This is emitted by the runner before the app can intercept it (not application-fixable); recorded as platform note (LL-m3). `--max-memory-bytes 100000` triggers LL-m1 below.
16. Throughput: 3000-row input processed in about 1 s.
17. Rules edge cases: `=` inside regex, `(?i)`, CRLF rules, indented names, `Uncategorized` reserved, empty/invalid regex, empty/zero/`1.` budgets, duplicate budget, 41-char name, `[ categories ]` with inner spaces (rejected), entry outside section, empty rules file (`OK: 0 rule(s), 0 budget(s)`), missing rules file 66, denied 77. PASS except item 1.

## Defects

| Id | Severity | Description |
|---|---|---|
| LL-1 | **BLOCKER** | `check-rules` and `--rules` do not report the lowest-line error. The "budget for unknown category" check runs after the whole file is parsed, while all other errors `die` immediately. Reproduction: rules `[categories]/Rent = Rent/[budgets]/Foo = 10/[categories]/bad line` gives `ledgerlens: rules: line 6: expected NAME = VALUE`, spec requires line 4 (`budget for unknown category`). Violates 3.2 ("`check-rules` validates the whole file; the first error found (lowest line number) is reported") and R5. The README limitation text describes a different (and incorrect) behavior. |
| LL-m1 | minor | `read_file` hitting `--max-memory-bytes` raises a catchable `LIMIT_EXCEEDED`; the app maps every non-denial error to `ledgerlens: cannot read FILE`, exit 66, instead of the 70 listed in 1.3 for an interceptable runtime limit. R15(b) (non-zero, no report) is still met. The same mapping exists in healthwatch and taskflow. |
| LL-m2 | minor | Budget values limited to 12 integer digits and 18-digit safe range; spec has no cap for budgets. |
| LL-m3 | minor (platform) | Runner-level limit stops (instructions, deadline) print the runtime's raw JSON error on stderr, which conflicts with R18 wording but cannot be intercepted from the program. Needs a runtime/spec clarification. |
| LL-m4 | minor | README limitation list does not mention LL-1/LL-m1/LL-m2. |
| LL-m5 | minor | The built artifact is not committed (`build/` is gitignored); evidence of the artifact is the clean-repro build. |

## Runtime changes used by this app (commit 86ae075c)

* `read_stdin`: narrow (new opcode READ_STDIN appended last, interpreter + bytecode + checker + registries). Needs no capability, consistent with `read_line`. Bytecode path charges memory after the read (`chargeAlloc`), so an enormous stdin is fully read into host memory before the limit trips; the interpreter path does not charge. Tests: `internal/vm/read_stdin_test.go` covers byte-exactness, empty stdin, interaction with `read_line`, second call, NUL bytes, both engines, and memory charge; the memory-charge test would fail if the charge were removed (mutation-plausible). Not lowered to HFIR (documented fence).
* `starts_with` fix: guarded by `(or (= prefix "") (and (!= s "") test))`; empty prefix still true. Test `TestStartsEndsWithEmptySubject` covers the matrix (empty subject/prefix, mismatches, `ends_with`) on both engines. Narrow and correct.

## Summary

Strong, well-tested implementation; 35/35 acceptance and clean-repro pass, and about 60 extra probes agree with the spec except LL-1 (blocker) and the minor items above. NOT ACCEPTED until LL-1 is fixed and re-reviewed (fix: validate budget-name existence in line order, e.g. record unknown-category as a pending error and compare its line with later errors, or run it as a second pass but emit the lowest line overall).
