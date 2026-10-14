# ledgerlens development journal (Milestone 1, application 1)

Implementer: Claude (Sonnet 5.5), autonomous, from the frozen requirements and acceptance
suite written by the independent selector (see `../SELECTION.md`). No human wrote or
repaired implementation code.

## 2026-10-13

1. Read `requirements/REQUIREMENTS.md`, `ACCEPTANCE.md`, and `tests/acceptance/run_acceptance.py`.
2. Probed the language before writing code (each probe is a throwaway program):
   * `print` joins arguments with a space; `str` concatenates.
   * `read_file` returns bytes (`bytes_to_string`); `write_file` pushes no value.
   * Runtime errors arrive as JSON text; `CAPABILITY_DENIED`, `IO_ERROR`, `INVALID_REGEX` are catchable.
   * Defun params need type annotations; user functions must be called with `(call f ...)`
     when nested; lists are copied across calls, dicts are shared.
   * `read_line` cannot tell EOF from an empty line.
3. **Blocker 1 (stdin, R13):** `read_line` returns "" for both EOF and a blank line and strips
   `\r`, so physical line numbers and end of input cannot be recovered. Added the narrow
   built-in `read_stdin` (whole remaining stdin, byte-exact, memory-charged), mirroring
   `read_line` at every wiring site. Regression test `internal/vm/read_stdin_test.go`
   (interpreter and bytecode, mutation-checked by dropping the memory charge).
4. Wrote `src/ledgerlens.howl`, fixed parenthesis balance and the type checker's
   `void`/`int` branch rules, then built and ran the suite.
5. **Blocker 2 (hang):** every `summary --rules` run looped forever. Root cause:
   `(starts_with "" " ")` returned true (desugars to `(list_get (str_split s p) 0) = ""`, and
   splitting "" yields [""]), so `trim_sp` never terminated on an empty field. Fixed in
   `internal/ast/desugar.go` (non-empty subject required). Regression test
   `TestStartsEndsWithEmptySubject`, mutation-checked by reverting the fix.
6. First full run after both fixes: 35/35 acceptance tests, R1-R18 all PASS (~11 s).

## Test impact assessment

* Behavior changed: new built-in `read_stdin`; `starts_with` on an empty subject.
* Existing tests re-run: construct drift, bytecode, ast, checker, vm CLI semantics,
  `TestProdFlipCriteriaLock` and `TestModelAdapterCoversArityTable` (both needed the new
  built-in registered as a fenced, not-yet-HFIR-lowered form).
* Missing coverage added: byte-exact stdin, memory charge, empty-subject prefix.
* Known baseline failures unrelated to this change: exec tests on this host (symlinked
  `/usr/bin/{printf,echo,touch}`), see `docs/journals/2026-10-13_milestone1_baseline.md`.

## Review round 1 (2026-10-13)

The independent reviewer (`REVIEW.md`) found blocker LL-1: `check-rules` must report the lowest-line
error, but the unknown-budget-category check (which needs the whole file) was deferred while syntax
errors exited at once, so an early unknown budget lost to a later syntax error. Fix: `parse_rules`
records the lowest-line problem and reports it after the whole file was seen. Regression:
`tests/regression/rules_lowest_line.sh`. Also fixed from the review's minor list: a memory/limit
stop during a file read now exits 70 (was 66); README limitation text corrected.
