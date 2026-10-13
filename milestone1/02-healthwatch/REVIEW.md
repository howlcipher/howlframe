# healthwatch: independent final review (D24-J)

Reviewer: independent final reviewer (did not write or help write any implementation).
Date: 2026-10-13. Reviewed tree: HEAD 2a95d364, against the frozen `requirements/REQUIREMENTS.md`.

## RE-REVIEW (HEAD a4539b7f): ACCEPTED, confirmed

Fixes verified: read-time resource limit now exits 70 (`resource limit reached while reading`); `status --format json` escapes names (name `a"b\c` round-trips through a JSON parser, text output unchanged). `clean_repro.sh` 02-healthwatch: 29/29. Frozen paths unchanged. No regressions found.

## DECISION (round 1): ACCEPTED

No violation of a written requirement found after about 90 extra black-box probes against real
loopback HTTP servers. Minor defects and platform notes are listed and carried over.

## Freeze check

`git diff 78bcf393 HEAD -- 'milestone1/*/requirements' 'milestone1/*/tests'` is empty.

## D24 checklist

| ID | Result | Evidence |
|---|---|---|
| A frozen requirements | PASS | Requirements, ACCEPTANCE.md, suite, fixtures in 78bcf393; implementation in c16c80ab (later). Frozen paths unchanged. |
| B source + standalone artifact | PASS | `src/healthwatch.howl` (532 lines, read in full): no fixtures, no test special-casing; no external helper. Runs as `HFBC` bytecode on `howlframe run` only (verified with `env -i PATH=/nonexistent`). No process/database capability used. |
| C independent acceptance tests | PASS | 29 tests by the selector, unmodified. |
| D clean-environment repro | PASS | `bash milestone1/scripts/clean_repro.sh 02-healthwatch`: `29 tests, 29 passed, 0 failed`, 24.7 s wall. Same caveat as other apps (fresh directory from `git archive HEAD`, host Go toolchain/cache reused). |
| E autonomous implementation | PASS (plausible) | Journal describes runtime probing (HEAD/body support, catchable error classes, `parse_json Any`, uncatchable deadline) and a first-run 29/29 with no runtime change; consistent with git (no `internal/` change in c16c80ab). Commit is a single coarse commit. |
| F documentation + workflow | PASS | I wrote my own config (status, `body_contains`, `json_assert`, retries, `${ENV:}` POST) and ran `validate`, `run --state`, `run --format json`, `status`, `status --fail-if-down`, stdin config. README grants table is accurate (coarse `network` rejected, `network:fetch=host:port` required, write grant checked before any request). Example config in README is valid. |
| G regression tests + limitations | PASS | Frozen suite is the regression suite. README limitations are honest (1.0/1e2 treated as integers, whole-second timestamps, no request timeout). Additional minor items below. |
| H capability/security | PASS | Verified: no grant and coarse `network` give DENIED (not fetch); redirect to a non-granted port yields DENIED with the redirect target never contacted; DENIED never retried (`retries:3` shows attempts 1); stderr denial line per denied check; env grant missing -> 77 before any request and before state creation; state write grant missing -> 77 with zero requests logged by the server; corrupt state -> 65 with zero requests, file untouched; secret value never in stdout, stderr or state (grep count 0), but observed by the server (correct); oversized response -> ERROR/response_too_large via `--max-fetch-bytes`; 501 checks -> 70 with no state file; deadline/instruction limits stop with empty stdout. |
| I development history | PASS with note | Commit c16c80ab, journal, handoff, evidence. Coarse granularity. |
| J independent decision | ACCEPTED | See reasons above. |

## Adversarial black-box checks (loopback servers on 127.0.0.1:18081/18082 that log method, path and body)

1. Result matrix: PASS/FAIL(status_mismatch, body_missing, json_invalid, json_mismatch)/ERROR(request_failed for connection refused and DNS failure)/DENIED, with attempts and `-` for no response. PASS.
2. Expectation order and JSON paths: `items.1.id` equals 2 and `2.0` (numeric equality), `items.0.id` equals `1.0`, `n` exists with null, `n` equals null, `zz` exists false, `f` equals 1.5, `s` equals `1` (string "1" vs number) -> json_mismatch, `items.00.id` unresolved, `items.-1`/`items.5` not exists, `0` key on an object, array root index `2`, equals true vs 1 -> mismatch, null vs 0 -> mismatch, non-JSON body with json_assert -> json_invalid, `body_contains` case sensitivity. PASS.
3. HEAD checks status only; observed methods and bodies at the server: POST body `he llo`, PUT body `x`, PATCH/DELETE no body, GET none; defaults merging (`method`, `expect_status`, `retries`, `retry_delay_ms` inherit and override) and `expect_status` array (204, 500 accepted). PASS.
4. Retries: server fails twice then succeeds: `retries:2` -> PASS after 3 attempts; later check proves exactly 4 requests total; `retry_delay_ms:400` with 2 retries took 0.84 s. PASS.
5. Redirect followed to final 200 (same host); redirect to ungranted port -> DENIED and other checks still run; exit 77 at the end with the full report printed. PASS.
6. State machine over four runs with a toggling endpoint: `new`, `down` (since reset, consecutive_failures 1), unchanged down (consecutive 2, since kept, no event), `recovered` (consecutive 0, since reset), `removed` sorted after other events, new check added later; DENIED existing entry kept as is, DENIED new name not added, denied retries stay 1; state written once and valid JSON; `last_http_status` null for ERROR. PASS.
7. State preflight: missing file created as `{"version":1,"checks":{}}`; invalid schema (missing key, `since` 1.5, `consecutive_failures` -1, extra key) -> 65 `healthwatch: state: ...`, file bytes unchanged, zero requests; non-JSON state -> 65; state dir missing -> 73 before requests; read grant only -> 77 before requests, no file created. PASS.
8. `status`: sorted by name, text and json, empty state (nothing / `{"checks":[]}`), missing file 66, invalid 65, denied 77, `--fail-if-down` exit 1 only when a down entry exists. PASS.
9. Config error order and paths (about 35 documents): top-level array/null/string/invalid JSON (`$`), `version` `"1"`/2/missing, missing/empty/non-array `checks`, name with space, `ftp://`, `http:///x`, `body` with GET, `expect_status` `[]`/`"200"`/99/`[200,"x"]`/true, `retries` 6 and 1.5, `retry_delay_ms` 5001, both/neither of `equals`/`exists`, empty path, non-object item, `exists:"yes"`, `equals:[1]`, `HEAD` + `body_contains`, `${FOO}` and `${ENV:lower}` in url, `${ENV:A}` in `body_contains`, duplicate name reported at the later check, unknown keys (check-level beats top-level; `defaults.method` before unknown defaults key, independent of key order in the file), `checks:["x"]`, `defaults:[]`. All matched the spec paths and order. PASS.
10. 500 checks validate (`OK: 500 check(s)`), 501 -> 70 even when check 4 is also invalid (spec step 4 precedes step 5); with `--state` no file created. PASS.
11. Usage matrix: missing/extra operand, missing value, `--format yaml`, `--state=x`, `--state` on validate/status, `--format` on validate/version, `--fail-if-down` on run, duplicates, unknown command, `help x`, `version x`: exit 2, one prefixed line. `-h` prints `usage: healthwatch ...`. PASS.
12. Env: value `a}b${x` expanded twice in url and body arrives byte-exact at the server; unset and empty variable -> `config: checks[0].url: undefined environment variable HW_TOKEN` (65); `validate` does not read the environment; stdin config with and without grants. PASS.
13. Resource limits: `--max-fetch-bytes 1000`, `--deadline 1ms` (state untouched), `--max-instructions 3000` on run and on validate (no `OK` on stdout). PASS.
14. stderr hygiene on success: empty; on failure only prefixed lines, except runner limit stops (see HW-m3).

## Defects (none are blockers)

| Id | Severity | Description |
|---|---|---|
| HW-m1 | minor | Integer detection goes through JSON re-encoding, so `"version": 1.0` and `"retries": 1e0` are accepted as integers. Documented in the README; spec says "the integer 1" so a strict reading would reject. |
| HW-m2 | minor | `ensure_env` maps every error from `env` to `capability denied: environment` / 77, even non-denial errors. `read_file` memory-limit errors map to 66 (see LL-m1). |
| HW-m3 | minor (platform) | Runner-level limit stops print the runtime's raw JSON error on stderr (no `healthwatch: ` prefix); not interceptable from the app. |
| HW-m4 | minor | `status` prints state check names unescaped; a hand-edited state with a name containing `"` yields invalid JSON in `--format json` (the tool itself only writes validated names). |
| HW-m5 | minor | Commit history coarse; build artifact not committed (gitignored). |

## Summary

All fourteen requirements behave as written, including the fixed single-error config precedence,
state transitions, preflights that send zero requests on denial, redirect re-checks and secret
hygiene. ACCEPTED.
