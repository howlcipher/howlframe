# healthwatch development journal (Milestone 1, application 2)

Implementer: Claude (Sonnet 5.5), autonomous, from the frozen requirements and acceptance
suite written by the independent selector. No human wrote or repaired implementation code.

## 2026-10-13

1. Read `REQUIREMENTS.md` and the whole acceptance suite first; probed the runtime before coding:
   * `http_request` accepts every method including `HEAD` and a request body; errors are
     catchable (`CAPABILITY_DENIED` for ungranted hosts and redirect hops, `IO_ERROR` for refused
     connections, `LIMIT_EXCEEDED` for oversize bodies). A runner deadline is NOT catchable, which
     is the behavior R11(c) needs.
   * `parse_json Any` accepts scalars; there is no type predicate, so JSON values are classified
     by the first character of `encode_json` (object, array, string, integer, number, bool, null).
   * `time_now` is Unix seconds; `sleep` takes milliseconds; `env` needs the `environment` grant
     and returns "" for unset.
   * There is no `nil` literal; a nil is obtained from a missing `map_get`.
2. Applied the two lessons from ledgerlens: explicit `(call f ...)` for user functions, and
   `void` for diagnostic helpers (an `if` mixing `int` and `void` branches fails type checking).
3. Wrote `src/healthwatch.howl` (config validator with the fixed single-error precedence, check
   engine, state machine, text/JSON reports, status command), fixed parenthesis balance and
   `let` single-body rules, then ran the suite.
4. First full run: 29/29 acceptance tests, R1-R14 all PASS (~22 s). No language or runtime
   change was needed for this app (`read_stdin` from ledgerlens covers `-`, and `starts_with`
   was fixed earlier).

## Test impact assessment

* Behavior changed: none in the language or runtime for this app.
* Regression protection: the frozen suite (29 tests) plus `scripts/clean_repro.sh`.
* Known unrelated baseline failures: exec tests on this host (see baseline journal).
