# REQUIREMENTS: healthwatch (Milestone 1, application 2 of 3)

> **STATUS: FROZEN, dated 2026-10-13.** Changes require a separately approved requirements revision (see 1.5). Tool name: `healthwatch`. Domain: service operations / HTTP endpoint monitoring with persisted state. Unrelated to the Howl ecosystem.

## 0. Problem statement

Operators of small services need a tiny, auditable probe that, from a declarative JSON file, calls a list of HTTP endpoints, decides for each whether the service is healthy (status code, body text, JSON field assertions, retries), remembers each endpoint's health between runs, and reports **transitions** (newly seen, went down, recovered, removed) with a monitoring-friendly exit status. Because it makes outbound network calls with possibly secret-bearing URLs, it must only talk to hosts it was explicitly granted, must never print URLs, request bodies, response bodies or secrets, must fail before sending anything when it cannot persist its state, and must keep working (and keep reporting) when individual checks are denied, over a limit, or failing.

## 1. Common conventions (apply to every Milestone 1 application)

### 1.1 Deliverable layout and build

The implementation lives under `milestone1/02-healthwatch/src/` (`.howl` source; HFIR is permitted if it qualifies under directive D24-B). The program MUST be built to a standalone HowlFrame artifact:

```
howlframe build milestone1/02-healthwatch/src/healthwatch.howl -o milestone1/02-healthwatch/build/healthwatch.hfbc
```

The artifact MUST execute on the standalone VM only (`howlframe run`, or legacy `-run-bc` with an equivalent `-compile-bc` artifact). It MUST NOT depend on generated Go/Python/JavaScript, on helper scripts, or on any external program at run time unless this document says so.

### 1.2 Invocation convention ("the launcher")

All behavior is specified for the invocation `healthwatch <args...>`, which the acceptance suite performs through a **launcher**: an executable file with this contract:

```
LAUNCHER GRANTS [APP_ARG...]
```

* `GRANTS` is exactly ONE argument: the comma-separated runtime grant list for `--allow-caps` (for example `filesystem:read=/tmp/x,filesystem:write=/tmp/x`). The empty string means "no grants": the launcher MUST then pass no `--allow-caps` at all.
* `APP_ARG...` are passed to the application verbatim, as its `cli_args`, in order, with no re-quoting and no interpretation of flag-looking arguments.
* The environment variable `HF_RUN_FLAGS` (whitespace-separated runner flags such as `--max-instructions 5000`, `--max-fetch-bytes 1024`, `--deadline 1s`, `--max-exec-output-bytes 100`) MUST be inserted by the launcher as runner options before the artifact path. It is the only channel for resource limits.
* The launcher forwards its stdin, stdout, stderr, working directory and process environment unchanged and exits with the application's exit status.

Reference launcher (`tests/acceptance/launcher.example.sh`):

```
howlframe run [--allow-caps "$GRANTS"] $HF_RUN_FLAGS milestone1/02-healthwatch/build/healthwatch.hfbc "$@"
```

Unless a test says otherwise the acceptance suite sets `HF_RUN_FLAGS="--max-instructions 4000000000 --max-memory-bytes 4294967296 --max-call-depth 10000"` (generous but finite ceilings; the runtime defaults of 100000 instructions / 64 MiB / call depth 1000 are far too small for realistic input and are NOT an application requirement). Resource-limit tests substitute smaller values.

Grant syntax (see `docs/cli.md`, `docs/reference/network_authority.md`, `docs/reference/process_authority.md`): `filesystem:read=<abs dir>`, `filesystem:write=<abs dir>`, `network:fetch=<host>:<port>`, `process:exec=<abs program> <pattern...>`, `process:env=<NAME>`, `environment`. The application never receives more than the grants it is launched with and MUST NOT attempt to widen them.

### 1.3 Streams and exit codes

* Results go to **stdout**. Diagnostics go to **stderr**; every diagnostic line MUST start with `healthwatch: `. Because the runtime's `stderr` built-in adds no newline, the application MUST terminate each diagnostic line with `\n`.
* stdout and stderr are UTF-8; text is passed through byte-exact (no re-encoding, no case-folding of non-ASCII).
* Exit codes (all applications):

| Code | Meaning |
|---|---|
| 0 | Success, nothing to report as a finding |
| 1 | Success of the tool, but the analysis found failures/violations (defined per application) |
| 2 | Usage error (bad command, bad/missing/duplicate option, wrong operand count). Nothing else is done: no input read, no file written |
| 65 | Input data/format/config error (data is unusable) |
| 66 | A required input file does not exist or is not a regular readable file |
| 70 | A resource limit was reached (application-level limit or a runtime `LIMIT_EXCEEDED` the application is able to intercept) |
| 73 | An output file cannot be written (and the failure is not a capability denial) |
| 77 | Capability denied by the runtime (see 1.4) |

### 1.4 Capability denial behavior (uniform)

When the runtime refuses an operation with `CAPABILITY_DENIED` the application MUST:

1. stop the operation that needed it, with no partial side effects (no partially written output file; nothing destructive done);
2. print exactly one diagnostic line of the form `healthwatch: capability denied: <capability-name>[ (<what>)]` on stderr, where `<capability-name>` is one of `filesystem`, `network`, `process`, `environment`, `database`;
3. exit with status 77 (unless this document defines a different, explicitly stated, per-item handling);
4. NEVER print raw runtime error objects (strings containing `"phase":`), host stack traces, or the text of secret values.

Missing files are NOT denials (exit 66, or as specified). Operations that do not need a capability MUST work with an empty grant list.

### 1.5 Freeze notice

**FROZEN 2026-10-13.** This document, together with `ACCEPTANCE.md` and `tests/acceptance/`, was written by the independent selector/evaluator before any implementation. Changes require a separately approved requirements revision that is recorded as such (new revision header, dated, with rationale); implementers MUST NOT edit, dilute or remove requirements or tests to claim success. The freeze is sha-agnostic: it binds to the content as authored, not to a repository revision.

## 2. Command-line interface

```
healthwatch run      [--state FILE] [--format text|json] CONFIG
healthwatch validate CONFIG
healthwatch status   [--format text|json] [--fail-if-down] STATE
healthwatch version
```

* `CONFIG` is a path, or `-` for standard input. `STATE` is a path.
* Options take their value as the NEXT argument; `--opt=value` is NOT supported (usage error). Options may appear before or after the operand. Duplicate options, unknown options, options not valid for the command, a missing option value, `--format` other than `text`/`json` (default `text`), a missing or extra operand, operands for `version` => exit 2 with nothing on stdout and no I/O performed. No command or an unknown command => exit 2. `help`, `--help`, `-h` print a usage text on stdout (first line begins `usage: healthwatch`) and exit 0.
* `version` prints exactly `healthwatch 1.0.0\n`, needs no grants.
* Exit codes are those of 1.3 with these command-specific meanings. `run`: precedence **2 > 65/66/70 > 77 > 1 > 0**, where 77 means "at least one check was DENIED" (the full report is still printed), 1 means "at least one check is FAIL or ERROR (and none DENIED)", 0 means all checks PASS. `status`: 0, or 1 only with `--fail-if-down` when at least one check is `down`. `validate`: 0 or 65/66/77.

## 3. Formats

### 3.1 Configuration file (JSON, UTF-8)

```json
{
  "version": 1,
  "defaults": { "method": "GET", "expect_status": 200, "retries": 0, "retry_delay_ms": 0 },
  "checks": [
    { "name": "api", "url": "http://127.0.0.1:8080/health", "method": "GET",
      "expect_status": [200, 204], "body_contains": "ok",
      "json_assert": [ {"path": "status", "equals": "ok"}, {"path": "items.0.id", "equals": 1}, {"path": "debug", "exists": false} ],
      "retries": 2, "retry_delay_ms": 100 },
    { "name": "echo", "url": "http://127.0.0.1:8080/echo", "method": "POST", "body": "ping ${ENV:HW_TOKEN}" }
  ]
}
```

Rules (strict: any violation is a config error, exit 65):

* Top level object keys: `version` (required, the integer `1`), `defaults` (optional object), `checks` (required array of 1..500 objects; more than 500 => exit 70 instead, see R11). Unknown keys anywhere are errors.
* `defaults` may contain only `method`, `expect_status`, `retries`, `retry_delay_ms`; a check's own key overrides its default. Built-in defaults: `method` `"GET"`, `expect_status` `200`, `retries` `0`, `retry_delay_ms` `0`.
* Check keys: `name` (required; `[A-Za-z0-9._-]{1,64}`; unique across checks), `url` (required string, scheme `http` or `https`, non-empty host), `method` (one of `GET HEAD POST PUT PATCH DELETE`), `body` (string; only allowed with `POST`, `PUT`, `PATCH`), `expect_status` (integer 100..599 or a non-empty array of such integers), `body_contains` (non-empty string; not allowed with `HEAD`), `json_assert` (non-empty array, not allowed with `HEAD`), `retries` (integer 0..5), `retry_delay_ms` (integer 0..5000).
* Each `json_assert` item has a non-empty string `path` and EXACTLY ONE of `equals` (a JSON string, number, boolean or null) or `exists` (a boolean).
* `${ENV:NAME}` (NAME matches `[A-Z_][A-Z0-9_]*`) may appear in `url` and `body` strings only; any other `${` sequence there, or `${ENV:` in any other string, is a config error. A reference is replaced by the value of that environment variable at `run` time (never by `validate`). An unset or empty variable is a config-time error at `run`: exit 65, `healthwatch: config: <path>: undefined environment variable NAME`.
* Config error message format: `healthwatch: config: <path>: <message>` on stderr, where `<path>` is `$` for the document itself (e.g. invalid JSON, not an object) or a JSON path such as `version`, `defaults.method`, `checks[2].url`, `checks[0].json_assert[1].path` (array indexes are 0-based). Exactly ONE error is reported (messages after the first are not printed). When several exist, the reported one is the first in this fixed order, independent of the key order in the file: (1) document shape / JSON syntax (`$`); (2) `version`; (3) `defaults` (keys `method`, `expect_status`, `retries`, `retry_delay_ms`, then unknown keys in byte order); (4) `checks` itself (must be a non-empty array; more than 500 elements => exit 70 at this point); (5) each check in ascending index, its keys in the order `name`, `url`, `method`, `body`, `expect_status`, `body_contains`, `json_assert` (the array, then its items in order, each item's `path` before `equals`/`exists`), `retries`, `retry_delay_ms`, then unknown keys in byte order; a duplicate name is reported at the later check's `name`; (6) unknown top-level keys in byte order. The path of an unknown key is the path of that key (e.g. `checks[0].foo`). Errors inside `expect_status` (wrong type, out-of-range or non-integer element, empty array) are reported at `checks[i].expect_status`/`defaults.expect_status` without an element index; errors in a `json_assert` item other than a bad `path` (neither/both of `equals`/`exists`, wrong `exists` type, non-scalar `equals`, non-object item) are reported at the item path `checks[i].json_assert[j]`; a bad `path` at `checks[i].json_assert[j].path`. `<message>` is free text.

### 3.2 State file (JSON, written by `run --state`, read by `run --state` and `status`)

```json
{"version": 1, "checks": {"api": {"health": "up", "since": 1760000000, "last_run": 1760000300,
                                  "last_status": "PASS", "last_http_status": 200, "consecutive_failures": 0}}}
```

`health` is `up` or `down`; `since`, `last_run` are Unix timestamps in seconds (integers); `last_status` is `PASS`, `FAIL` or `ERROR`; `last_http_status` is an integer or `null`; `consecutive_failures` is a non-negative integer. A state file that is not valid JSON or violates this schema => exit 65 (`healthwatch: state: <message>`), file untouched.

## 4. Functional requirements

**R1 (CLI shell).** Section 2: dispatch, option parsing, `version`, `help`, usage errors (nothing on stdout, no I/O, no network, no state change), exit codes, diagnostic prefix.

**R2 (config validation).** Section 3.1 enforced completely by `validate` and by `run` before any network I/O. `validate` prints `OK: N check(s)\n` and exits 0, or the config error (exit 65). Every rule of 3.1 has a distinct error path. Defaults merging: a check inherits `defaults` keys it does not set.

**R3 (check execution and result model).** `run` executes the checks sequentially in config order. One HTTP call per attempt with the check's method (and `body` if present) to the URL (after `${ENV:}` expansion); redirects are followed by the runtime. Each check yields a result `{name, status, http_status, attempts, reason}`:

| status | meaning |
|---|---|
| `PASS` | response received and all expectations met; reason `ok` |
| `FAIL` | response received, an expectation failed; reason one of `status_mismatch`, `body_missing`, `json_invalid`, `json_mismatch` |
| `ERROR` | no usable response; reason `request_failed` (connection refused, DNS failure, protocol error, any failure that is not a capability denial or limit) or `response_too_large` (the runtime's fetch-size limit was hit) |
| `DENIED` | the runtime refused the request with `CAPABILITY_DENIED` (including a redirect hop to a non-granted host); reason `capability_denied` |

`http_status` is the integer status of the last response, or `null` if none. `attempts` is the number of HTTP calls issued for the check (a denied call counts; a check is never attempted more than `1 + retries` times).

**R4 (expectations).** Evaluated in this order, stopping at the first failure: (1) the response status must equal `expect_status` (or be in the array) else `status_mismatch`; (2) if `body_contains` is set, the response body (as text) must contain it as a case-sensitive substring else `body_missing`; (3) if `json_assert` is set, the body must parse as JSON else `json_invalid`; then each assertion in order must hold else `json_mismatch`. JSON path: split on `.`; a segment applies to an object as an exact key, or to an array when it is a non-negative integer without leading zeros as an index; anything else does not resolve. `exists: true` holds when the path resolves (a resolved `null` counts), `exists: false` when it does not. `equals` holds when the path resolves and the value is JSON-equal: same JSON type, strings exact, numbers by numeric value (`1` equals `1.0`), booleans, null. `HEAD` checks only assert status. The request method and body the server observes MUST be exactly the configured ones.

**R5 (retries).** After an attempt that ends `FAIL` or `ERROR`, up to `retries` further attempts are made, each preceded by a pause of `retry_delay_ms` milliseconds; the first `PASS` stops retrying; the check result is that of the last attempt. `DENIED` is never retried. A check with `retries: 0` issues exactly one request.

**R6 (resilience).** A failure of one check (connection refused, oversized response, denial, bad JSON) never stops later checks from running and being reported. An oversized response (runner `--max-fetch-bytes`) yields `ERROR`/`response_too_large`.

**R7 (state and events).** With `--state FILE`: before ANY request the program validates the state file (missing file => empty state) and **proves it can persist** by (re)writing it with its current content (an initial `{"version":1,"checks":{}}` if absent); if reading or writing is denied the program exits 77 having sent zero requests. After all checks ran, for each non-`DENIED` result compute `health` = `up` iff `PASS`, and update the state: no previous entry => event `new`, `since` = now; previous `up` and now down => event `down`, `since` = now, `consecutive_failures` = 1; previous `down` and now up => event `recovered`, `since` = now, `consecutive_failures` = 0; unchanged health => no event, `since` kept, `consecutive_failures` = previous + 1 if down else 0. `last_run` = now, `last_status`, `last_http_status` from the result. Entries whose names are no longer in the config are deleted and reported as event `removed`. `DENIED` results change nothing and produce no events (a denied name that is new is not added; an existing entry is kept as is, not removed). The state file is written once, after all checks ran, and is valid JSON of section 3.2. Without `--state` there is no state I/O and no events.

**R8 (output).** `--format text` (default), one line per record, fields separated by a single TAB, in this order: all result lines in config order `STATUS<TAB>name<TAB>http_status-or--<TAB>attempts<TAB>reason`, then event lines `EVENT<TAB>kind<TAB>name` (config order for new/down/recovered, then `removed` sorted by name), then exactly one `SUMMARY<TAB>pass=N<TAB>fail=N<TAB>error=N<TAB>denied=N` line. `--format json`: one JSON document `{"results":[{"name","status","http_status","attempts","reason"}...],"events":[{"kind","name"}...],"summary":{"pass":N,"fail":N,"error":N,"denied":N}}` followed by a newline. Output is produced after the state write; ordering across streams is irrelevant. Nothing else is written to stdout. Output never contains URLs, hosts, request bodies, response bodies or environment values.

**R9 (status command).** `status STATE` reads the state file (read grant only) and prints, sorted by name (byte order), text lines `health<TAB>name<TAB>since<TAB>last_run<TAB>last_status<TAB>last_http_status-or--<TAB>consecutive_failures`; or JSON `{"checks":[{"name","health","since","last_run","last_status","last_http_status","consecutive_failures"}...]}`. An empty state prints nothing (text) / `{"checks":[]}` (json). Missing file => exit 66; invalid state => 65. `--fail-if-down` => exit 1 when any entry is `down`.

**R10 (environment interpolation).** Section 3.1 `${ENV:NAME}` semantics. Reading the environment requires the `environment` grant and is attempted only if the config contains at least one reference; if denied, exit 77 before any request, and (for `run --state`) before the state preflight. Secrets expanded into URL/body MUST NOT appear anywhere in stdout, stderr or the state file.

**R11 (resource limits).** (a) More than 500 checks => exit 70 `healthwatch: check limit exceeded (max 500)`, no request, no state change. (b) Runner `--max-fetch-bytes N`: R6. (c) Runner `--deadline D`: when it expires the program stops (non-zero exit, no hang past ~D + 5 s); the state file is left as it was (R7 writes only at the end). (d) Runner `--max-instructions`/`--max-memory-bytes` exhaustion stops the program with non-zero exit and no `OK`/report text on stdout; it never hangs.

**R12 (capabilities).** Required grants and expected denials:

| Operation | Required grant | Denied behavior |
|---|---|---|
| read CONFIG / STATE file | `filesystem:read=<dir>` | exit 77, section 1.4, no request sent |
| write STATE | `filesystem:write=<dir>` | exit 77 before any request (R7) |
| HTTP request to `host:port` | `network:fetch=host:port` (the coarse `network` grant does NOT authorize fetching) | that check is `DENIED`; run continues; exit 77 at the end |
| `${ENV:}` expansion | `environment` | exit 77 before any request |
| CONFIG `-`, `version`, `help`, usage errors | none | work with empty grants |

A redirect from a granted host to a non-granted host or port yields `DENIED` for that check (the runtime re-checks each hop). The program MUST NOT require `process` or `database`. It MUST NOT retry a denied request, probe other hosts, or fall back to any other mechanism.

**R13 (config from stdin; network-only run).** With CONFIG `-` and no `--state`, a run needs no grant except `network:fetch=` entries (and `environment` if references exist).

**R14 (diagnostic hygiene).** Every stderr line is prefixed `healthwatch: `; failures never print raw runtime error objects, stack traces, URLs or secrets; a run whose checks all pass writes nothing to stderr. A DENIED check additionally writes ONE stderr line per denied check: `healthwatch: capability denied: network (<check name>)`.

## 5. Edge cases (binding)

Empty `checks`; duplicate names; names with spaces/`/`; `expect_status` 99/600/`"200"`/`[]`/`[200,"x"]`/`true`; `retries` 6, `-1`, `1.5`; `body` with `GET`; `body_contains` with `HEAD`; `json_assert` item with both/neither of `equals`/`exists`; empty `path`; `url` with `ftp://`, no scheme, empty host; unknown key at any level; `version` 2 / `"1"`; invalid JSON; top-level array; `${ENV:lower}`, `${FOO}`, unset env var; 501 checks; JSON paths into arrays (`items.0.id`, `items.00`, `items.-1`, `items.5`), null values, numeric equality `1` vs `1.0` vs `"1"`; non-JSON body with `json_assert`; non-2xx `expect_status`; redirect followed to the final 200; redirect to a non-granted port; connection refused; oversized body; retries that succeed on the last allowed attempt; state with removed check; new check added later; denied check with an existing state entry; state written when all checks failed; state corrupt.

## 6. Out of scope

Request headers, authentication schemes, TLS options/client certificates, proxies, request timeouts per check (only the runner `--deadline`), latency measurement, concurrency/parallel checks, scheduling/daemon mode, alert delivery (email/webhook), metrics export, HTML/Prometheus output, config includes, wildcard hosts, cookie jars, following redirects manually, comparing response bodies against files.
