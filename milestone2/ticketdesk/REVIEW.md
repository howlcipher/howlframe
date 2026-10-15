# Milestone 2 independent final review: ticketdesk

Reviewer: independent final reviewer (did not write any implementation). Date: 2026-10-14.
Reviewed commit: `3b79b7a3` ("milestone2: per-request memory budget, HTTP API guide ..."). Scope: PLAN.md and
directive section 6 (Milestone 2, D19, D29 A, C, D, G, H). Nothing in `milestone2/ticketdesk/src`,
requirements, tests, dx or any Go source was modified by this review. The adversarial harness lives in the
reviewer scratchpad (not in the repo): `adv.py`, `adv2.py`, `t1.py` to `t10.py`.

## Decision: NOT ACCEPTED

Four blockers were found (B1 to B4, section 5). The frozen 101-test suite and the 13-check DX checker pass,
but only because the suite launches the server with raised resource ceilings
(`--max-instructions 4000000000 --max-memory-bytes 4294967296 --max-call-depth 10000`, `tests/acceptance/harness.py`
line 27). Under the command that README.md and `docs/guides/http_api.md` document, valid requests fail and the
listing is permanently broken after 100 tickets (B1). A busy port prints a raw runtime error object that R1.3
forbids (B2). One authenticated connection can freeze the whole server (B3). R3.1's exact route table is
violated by path normalisation (B4). All four have small, local fixes; see the carryover list in
`milestone2/MILESTONE2_DECISION.md`.

## 1. Freeze integrity (acceptance condition 5c)

```
git diff 10d9c2a8 HEAD --name-status -- milestone2/ticketdesk/requirements milestone2/ticketdesk/tests \
    milestone2/ticketdesk/dx milestone2/PLAN.md          # output: empty
```

Result: no change, not even a new file, to the frozen plan, requirements, acceptance suite (101 tests) or DX
checker and fixtures since `10d9c2a8`. `git status --short` is clean at HEAD. PASS.

## 2. Per-gate verdicts

| Gate | Verdict | One-line reason |
|---|---|---|
| A Practical HTTP API | FAIL | Works under the suite's flags; broken under the documented default command (B1); R3.1 route table violated by path aliasing (B4) |
| C Authentication and authorization | PASS | Token lifecycle, roles, ownership, secrets handling and constant-time compare verified by source and black box |
| D Validation and reliable errors | PASS (with caveats) | Error model, every input class, limits and fault handling hold; transport-level rejections are not JSON (minor); B1 shows safe 500s on valid input under default limits |
| G Standalone install and execution | FAIL | Clean repro passes and needs only Go and Python (plus git/tar); but R1.3 violated for port in use (B2) and the documented run command is functionally broken (B1) |
| H Demonstrably improved developer experience | PASS (with caveat) | Items 1 to 5 verified before/after; item 6 guide is accurate except its "Running standalone" command inherits B1 |

### Gate A: Practical HTTP API (FAIL)

Evidence gathered:

* Clean-repro run (section G) passes all 101 tests, including R3, R5, R6, R13 and R17.
* Black-box checks ADV01 to ADV12, ADV16 to ADV20, ADV29 to ADV32, ADV37 to ADV40, ADV54 to ADV58: check order
  (R3.4), 415/413/400 ordering, ceiling exactness (ceiling bytes accepted, ceiling+1 rejected, bytes not
  characters, chunked bodies counted), `limit` edge values, status filter with limit=1 cursor walk, HEAD and
  other methods (405 + exact `Allow`), request-id uniqueness (150 ids), 60 concurrent PATCH and 60 concurrent
  creates with no lost update, id non-reuse. All PASS.
* FAIL, B1: with the command documented in `README.md` and `docs/guides/http_api.md` (default
  `--max-instructions 100000`), see section 5.
* FAIL, B4: `GET //v1/tickets`, `/v1//tickets`, `/v1/tickets/.`, `/v1/./tickets`, `/v1/x/../tickets` all answer
  `200` (served as `/v1/tickets`) instead of `404 not_found`; `/v1/tokens//x` answers `405`. R3.1 says any
  other path is an unknown route. Cause: `path.Clean` in `cleanRoutePath` (`internal/httpreq/httpreq.go`),
  pre-existing runtime behaviour that the new fallback route does not change.

### Gate C: Authentication and authorization (PASS)

* V1 (constant-time compare): `src/ticketdesk.howl` `authenticate` hashes the presented secret with
  `sha256_hex`, compares to the bootstrap digest with `secure_equal`, looks the digest up as a store key
  (`h:<digest>`) and re-compares the stored hash with `secure_equal`. `OpSecureEqual` in `internal/vm/vm.go`
  uses `subtle.ConstantTimeCompare`. No `=` on secrets. PASS.
* V2 (CSPRNG): token ids and secrets use `rand_hex` (`crypto/rand`, `internal/vm/crypto.go`); request ids use
  the same primitive (128 bits). PASS. Minor: `newRequestID` returns all zeros if `rand.Read` fails (m9).
* Secrets never stored or logged: ADV46/47/48 (state file contains no issued or bootstrap secret, does contain
  the SHA-256 of an issued one, list response has no secret or hash), ADV59/64 (no Authorization value or query
  secret on stderr, with `--access-log` and with a fault). PASS.
* Lifecycle: ADV41 (admin token revokes itself, next request 401, re-revoke 200), ADV42 (30 concurrent token
  creations, 30 unique ids and secrets), ADV43 (hammering a token from 4 threads while revoking it: zero `200`
  responses after the revoke response returned), ADV44 (unknown 404, `bootstrap` 409, wrong-case id 404). PASS.
* Authorization: ADV01/02/03 (role before existence, ownership before media type, member PATCH of a missing
  ticket 404), ADV45 (a member cannot mint an admin token, cannot patch `owner` or `id`). PASS.
* Header edge cases ADV13/14/15: tab or double space after `Bearer`, upper-case secret, truncated or extended
  secret, `Basic`, empty, non-ASCII byte: all 401; first of two Authorization headers wins. Note: a trailing
  space after the secret is stripped by `net/http` (RFC 9110 OWS) before the app sees it, so it authenticates;
  not attributable to ticketdesk (m2).

### Gate D: Validation and reliable errors (PASS with caveats)

* ADV06 to ADV12, ADV17 to ADV28: oversize and boundary bodies, two Content-Type headers, NUL and exotic Unicode
  round trip, astral code points counted as one (120 ok, 121 too_long), lone-surrogate escape (stored as U+FFFD,
  no 500), invalid UTF-8 bytes (`invalid_json`), BOM prefix (`invalid_json`), duplicate keys (last wins, 201),
  100000-deep and 200000-deep nested arrays and 50000-deep nested objects (all `400 invalid_json`, server
  healthy, stderr free of `goroutine`/`panic`). PASS.
* ADV39/59/60/61: fault header inert unless `TICKETDESK_ALLOW_DEBUG_FAULT=1`; fault returns the fixed safe 500
  with request id; stderr has the `fault request_id=... method=... path=... error=...` line and, with
  `--access-log`, an access line with status 500; unknown route with the header stays 404. PASS.
* ADV62/63: percent-encoded CRLF and a raw control byte in the path cannot split log lines (path logged
  percent-encoded via `EscapedPath`). PASS.
* Caveats (minor, m3, m4): malformed percent-escape in the path (`400 Bad Request`), a 2 MB header (`431`) and
  `OPTIONS *` are answered by `net/http` as plain text or an empty body, outside R8.1's shape; the application
  cannot intercept them and no document says so. A syntactically valid but semantically corrupt state file
  (`{"meta":{"next_ticket":"x"}}`) is accepted at startup and every `POST /v1/tickets` then returns the safe 500.

### Gate G: Standalone installation and execution (FAIL)

* `bash milestone2/scripts/clean_repro.sh` run by the reviewer: builds from `git archive HEAD` into a fresh temp
  dir, `go build .`, `howlframe build`, runs the acceptance suite: `101 tests, 101 passed, 0 failed`; DX checker
  `13 checks, 13 passed, 0 failed`; wall time 15.9 s.
* Requirements of the script: bash, git (for `git archive`), tar, mktemp, go, python3. The plan says Go and
  Python 3; the extra tools are ubiquitous but the script cannot run from a tarball or a non-git copy (m7).
  `GOMODCACHE=<empty> GOPROXY=off go build .` succeeds, so the root binary needs the Go standard library only
  (the `jsonschema` module in `go.mod` is not on the build path of `.`).
* No generated Go/Python/JS server: the run command is `howlframe run ... ticketdesk.hfbc`; the only artifacts
  are the `.hfbc` built from `ticketdesk.howl` and the `howlframe` binary. PASS (V3).
* Grants: exactly the five documented grants (`network,database,environment,filesystem:read=<DIR>,filesystem:write=<DIR>`)
  run the server. ADV75: removing each of the five in turn gives exit 77 and `ticketdesk: capability denied: <name>`
  (network, database, environment, filesystem for both read and write); ADV73/74: no filesystem grants, or
  grants for the wrong directory, give 77. ADV84d: an extra `process` grant is harmless; ADV76: a coarse
  `filesystem` grant runs. Nothing listens before the check (suite test, plus ADV75). PASS.
* Startup precedence (R1.3): ADV81/85/86 (environment 2 before everything), ADV83 (corrupt data with
  `network` missing: 65), ADV84 (missing data dir with `network` missing: 66), ADV79 (ports `08080` and
  empty accepted, `+80`, ` 80`, `0x50`, `65536`, `1e3` exit 2). PASS. Note: with no `environment` or no
  filesystem grant the server cannot know the lower-precedence errors and reports 77 first (acceptable).
* FAIL, B2: busy port (ADV87). Exit status 1, stderr is one raw line
  `{"phase":"runtime","code":"IO_ERROR",...,"message":"http listen failed: listen tcp :57047: bind: address already in use"}`,
  with no `ticketdesk: ` prefix. R1.3 requires that stderr lines start with `ticketdesk: ` and contain no raw
  runtime error object or `"phase":` text for every row of its table, including this one. The frozen
  `T-R1-port-in-use` test checks only exit code and non-hang, so the suite cannot see it.
* FAIL, B1: the documented run command does not serve valid data (section 5).

### Gate H: Developer experience (PASS with caveat)

* `HF_BIN=<fresh build> python3 milestone2/ticketdesk/dx/check_dx.py -v`: 13/13 PASS. `go test -run
  'DeveloperExperience|HTTPAPIGuideRule' .`: ok.
* Before (`dx/BASELINE.md`, unmodified toolchain) versus after (run by hand on each fixture):
  * unclosed paren: before `Expected ')'` at 37:1; after names the `(if` at 33:5 as unclosed. Better.
  * stray paren: before `Unexpected tokens after EOF`; after explains the form was already complete and where
    to remove the extra `)` (two fixtures). Better. The `extra_close` message gives line 6 column 17 and says
    "more code follows at line 7"; correct and actionable.
  * `(triple ...)` without `call`: before an opaque "no bytecode lowering exists"; after "it is a function
    defined in this program, so it must be invoked as (call triple ...)". Better.
  * `let` with three args: after suggests `(do ...)`. Better.
  * `if` branch mismatch: after names both branch lines and the usual remedies. Better.
* Item 5 observability, run by hand with `--access-log` (ADV59, ADV62 to ADV65, ADV40): one access line per
  request, matching format, request id equals response header, fault line independent of the flag. PASS.
* Item 6, `docs/guides/http_api.md`: all `lisp check` blocks compile; I ran blocks 1 and 2 as servers. The
  fallback route, `req_path`, 405 with `Allow`, `req_url_path` and the claim that `parse_json` over `req.body`
  raises a catchable `LIMIT_EXCEEDED` (message `request body exceeds maximum limit of N bytes`) are all true.
  Criticisms: (a) the "Running standalone" command inherits B1; (b) the second example's `echo-checked` route
  reads `(req_body req)` in a plain `let`, so an oversize body becomes an uncaught fault and a 500, while the
  surrounding prose recommends turning it into 413 and shows no code for that (m6); (c) the `check_title`
  fragment is trivially small; (d) the guide says an exact path is exact, which B4 contradicts.
* `docs/cli.md` documents `--max-request-bytes` and `--access-log` (V4 PASS) but its `--max-memory-bytes` line still
  says the charge is cumulative and never reclaimed, which is no longer true for HTTP routes (m5).

## 3. Runtime changes (`git log 10d9c2a8..HEAD`, `internal/`, `howlframe.go`)

| Change | Narrow? | Tested? | Capability enforcement | Verdict |
|---|---|---|---|---|
| `sha256_hex`, `rand_hex`, `secure_equal` | yes, no capability | unit tests earlier | n/a | fine |
| `inherit:filesystem` in `Attenuate` | yes, one constant | `TestAttenuateInheritFilesystemOnlyNarrows` (subset, empty parent). A mutation that also copied non-filesystem grants is caught; one that matched `strings.HasPrefix` too loosely is only partly caught | verified by hand: runner with `filesystem:read/write=<A>` gave the handler read/write on A, `CAPABILITY_DENIED` on sibling dir B, on `/etc`, and on `env`; runner with no filesystem grant gave the handler none (`granted "filesystem"` false) | no escalation found |
| `granted` | yes | `TestHTTPServerM2Features` (network true, process false); negative path for scoped `network:fetch=` not tested | reports route/VM `AllowedCaps` only; leaks only the boolean about the caller's own grants; a handler sees its attenuated set, not the runner's | no leak found |
| `req_body` | yes, bounded by `MaxRequestBodyBytes`, reads `ceiling+1`, charges allocation | `TestHTTPServerM2Features` (oversize raises), suite R14 | none needed | fine |
| `req_url_path`, `utf8_valid` | yes | feature test | none | fine |
| Fallback route `"*"` | yes, one `fallback` field; reached only after literals and patterns | feature test | confers no authority: runs with the same route grants as any handler; auth stays in the app | fine; see B4 for the normalisation it does not cause |
| `serialize_requests` | one mutex per server | feature test (20 concurrent RMW) | none | introduces B3 (lock held across body read, no read timeout) |
| Per-request memory budget (`&allocationBudget{}` per request) | narrow edit, but applies to every `http_server` program | `TestHTTPServerMemoryBudgetIsPerRequest` | weakens the process-wide cumulative bound: N concurrent non-serialized requests may each charge a full budget; documentation not updated (m5). With `--max-instructions 4000000000 --max-memory-bytes 300000`, 150 creates of 3000-byte bodies all succeeded | acceptable, document |
| Atomic store write (`writeFileAtomic`) | yes | `TestFileStoreEmptyFileAndAtomicWrite`, relative-path test | sibling temp file in the same directory, covered by the same filesystem grant; killed mid-write storm left no temp file (ADV52) | fine; no directory fsync (m8) |
| Default `MaxRequestBodyBytes` 10 MiB to 1 MiB | one constant | `runtime_integrity` updated | none | behaviour change for all servers, documented in cli.md |
| `run` flag errors exit 2, percent-encoded paths in logs | yes | cli tests | none | fine |

## 4. Adversarial black-box checks (ADV01 to ADV100, about 110 cases including sub-cases)

Result: every check below passed unless marked. Harness: Python raw sockets against a freshly built server,
per-check temp directories.

| Ids | What | Outcome |
|---|---|---|
| ADV01 to ADV05 | check order R3.4: viewer PATCH missing + bad media; member PATCH foreign + bad media; member PATCH missing + bad JSON; bad media vs oversize; unauth vs oversize | PASS |
| ADV06 to ADV10 | body exactly at ceiling (invalid and valid JSON), ceiling+1, multibyte counted in bytes | PASS |
| ADV11, ADV12 | chunked transfer, oversize and small | PASS |
| ADV13 to ADV15 | Authorization edge cases, repeated header, non-ASCII | PASS (trailing space note, m2) |
| ADV16, ADV17 | 11 Content-Type variants, repeated Content-Type | PASS |
| ADV18 to ADV23 | NUL, U+2028, BOM char, emoji, lone surrogate, invalid UTF-8, BOM prefix, duplicate keys | PASS |
| ADV24 to ADV28 | 100k and 200k deep nesting, deep objects, stderr clean | PASS |
| ADV29, ADV30 | 26 `limit`/`status`/`cursor` edge values; repeated `limit` (first wins, consistent) | PASS |
| ADV31, ADV32 | HEAD, OPTIONS, PUT, TRACE, lower-case `get`, PROPFIND on 4 routes: 405, exact Allow, JSON body | PASS |
| ADV33 | trailing slash, `//`, dot segments, case, nested | FAIL for 5 of 18 inputs (B4); trailing slash and case variants correctly 404 |
| ADV34, ADV35 | odd ids; `t%31` decodes to `t1` (HTTP-equivalent, informational) | PASS (note) |
| ADV36 | malformed percent escape: plain text 400 | NOTE (m3) |
| ADV37, ADV38 | HTTP/1.0; 2 MB header | PASS (431 plain text, note) |
| ADV39, ADV40 | fault header inert; 150 request ids | PASS |
| ADV41 to ADV45 | token lifecycle, 30 concurrent creates, revoke race hammer, unknown/bootstrap revoke, privilege escalation | PASS |
| ADV46 to ADV49 | secrets and hashes in state and token list; directory contents | PASS |
| ADV50 to ADV52 | SIGKILL after each response (41 confirmed tickets all present, order kept); SIGKILL during 4-thread write storm then restart | PASS |
| ADV53 | 11 state-file shapes: truncated, null, `[]`, `{}`, number, whitespace-only, BOM, string, NULs, wrong-typed meta, truncated records | PASS for 65 on corrupt, zero-byte, null and `{}` start; `[]` and BOM give 65 (note); wrong-typed `meta` starts and then 500s (m4) |
| ADV54 to ADV58 | 60 concurrent PATCH, 60 concurrent creates, id reuse, limit=1 walk with filter, unissued cursors (return first page, unspecified) | PASS |
| ADV59 to ADV65 | fault safety, header variants, CRLF and control byte in path, secret in query and path | PASS; secret sent in the URL path is logged (client error, informational) |
| ADV66 | authenticated member sends headers and half a body, then a health probe from another connection | FAIL (B3): probe blocked more than 6 s |
| ADV67, ADV68 | 200 idle connections; half-sent headers | PASS |
| ADV69 | 300 creates, list latency, state size | note: create 7.8 ms avg, state 214 KB |
| ADV70 to ADV72 | precedence with no `environment` grant | invalid tests (server cannot read env without the grant); redone as ADV81 to ADV86 |
| ADV73 to ADV77 | no filesystem grants, wrong-directory grants, drop each of five grants, coarse `filesystem`, parent-directory grant for a subdirectory data file | PASS |
| ADV78 to ADV80 | admin token with quote, backslash, `$`; port syntaxes; 256-char token and its 255-char prefix | PASS |
| ADV81 to ADV86 | precedence with the right grants: 2 before 66/65 before 77 | PASS |
| ADV84b | data path is a directory with parent not granted | invalid test (parent not granted gives 77); not counted |
| ADV87 | port in use | FAIL (B2) |
| ADV89 to ADV92 | `OPTIONS *` (empty 200, note), absolute-form target, GET with body, DELETE with garbage body | PASS |
| ADV93, ADV94 | stalled member POST probed at 1 s, 10 s and 25 s; recovery on close | FAIL (B3): every probe timed out; server recovers once the connection closes |
| ADV95, ADV96 | same stall by a viewer (rejected before body read) and by an unauthenticated client | PASS (not blocking); only member and admin roles reach the body read |
| ADV97 to ADV100 | memory and instruction ceilings | at default instruction ceiling a 3000-byte body gives 500 (B1); with a raised ceiling 150 requests under `--max-memory-bytes 300000` all 201; tiny `--max-memory-bytes 20000` gives a safe 500 and the server lives; handler instruction exhaustion is a safe 500 |

Additional reviewer checks outside the numbered list: `t4.py` to `t8.py` (B1 thresholds and list scaling),
`fs.howl` (attenuation), guide blocks `g0`, `g1`, `g5`.

## 5. Defects

### Blockers

**B1. The documented standalone command serves valid requests incorrectly (gates A, G, H).**
`README.md` and `docs/guides/http_api.md` give `howlframe run --allow-caps ... --max-request-bytes 1048576 --access-log
ticketdesk.hfbc`, i.e. the default `--max-instructions 100000`. Reproduced by curl against that configuration:
`POST /v1/tickets` with a body of 2857 characters or more (valid up to 4000, R7.2) returns `500 internal_error`
(fault `LIMIT_EXCEEDED ... instruction limit exceeded`); after 100 tickets exist, every `GET /v1/tickets` returns 500,
even `limit=1`, permanently. The suite hides this because its harness always passes
`--max-instructions 4000000000 --max-memory-bytes 4294967296 --max-call-depth 10000`; R1.1 permits that channel but
neither the README nor the guide tell a user to use it. README text mentions only memory ("the default (64 MiB)
applies per request"). Fix options: lower the per-request instruction cost, make http_server request VMs use a
per-request instruction budget that scales, or document the required ceilings in README, the guide and the
launcher example, and add a default-flags smoke test (4000-char body, 150 tickets listed).

**B2. Port in use violates R1.3 (gate G).** See gate G. Exit status 1 with a raw `{"phase":"runtime",...}` object and no
`ticketdesk: ` line. Fix: have the application or runner report bind failure as a `ticketdesk: cannot listen on port N`
line, no raw object; add the stderr assertions to a port test (the frozen test cannot be edited, so add a new one).

**B3. One authenticated connection stalls the whole server (security: availability; gates A, D).** `(serialize_requests)`
takes a global mutex before the handler runs and the handler reads the body inside it. `http.ListenAndServe` has no read
timeout. A member or admin token holder who sends headers and part of a body keeps the mutex indefinitely (probes at 1 s, 10 s and
25 s all timed out); every other client, including `GET /v1/health`, hangs until that socket closes. Viewers and
unauthenticated clients cannot trigger it because they are rejected before the body read. R17 requires no hang beyond 10 s
per request. Fix: set `ReadHeaderTimeout`/`ReadTimeout`/`WriteTimeout` on the server, and/or buffer the body (with a
deadline) before taking the serialization lock.

**B4. Path normalisation breaks R3.1's exact route table (gate A).** See gate A. `//v1/tickets`, `/v1//tickets`,
`/v1/tickets/.`, `/v1/./tickets`, `/v1/x/../tickets` are served as `/v1/tickets` (200 with valid credentials) and
`/v1/tokens//x` as `/v1/tokens/x` (405). No authorization bypass was found (authentication happens in the handler on
the cleaned route), but R3.1 says any other path is an unknown route, and the guide says a route without braces is
exact. Fix: match on the raw escaped path, or reject paths that change under `path.Clean`.

### Minor

* m1. README says listing is "suitable for thousands of tickets". Measured with raised ceilings: 500 tickets 0.5 s,
  1000 tickets 3.5 s, 2000 tickets more than 8 s per list call (quadratic sort); combined with serialization any
  member who creates tickets degrades everyone. Out of scope per PLAN section 6 as performance work, but the claim is wrong.
* m2. A trailing space after a bearer secret authenticates because `net/http` strips optional whitespace; informational.
* m3. Transport-level rejections (malformed percent-encoding, header too large, `OPTIONS *`) bypass the error shape; undocumented.
* m4. State validation is shallow: parseable but semantically corrupt state is accepted at startup and makes writes fail with
  the safe 500 forever; R1.3 only demands rejection of unreadable/corrupt state, so not a blocker.
* m5. `docs/cli.md` still describes `--max-memory-bytes` as cumulative and never reclaimed; the budget is now per HTTP request
  and there is no process-wide bound. Document the trade-off.
* m6. Guide example `echo-checked` leaves `req_body` uncaught (oversize becomes 500) beside prose recommending 413.
* m7. `clean_repro.sh` needs git, tar, bash and mktemp in addition to Go and Python 3 and cannot run from a non-git copy.
* m8. State file mode 0644 (tickets world-readable); no directory fsync after the atomic rename (SIGKILL-safe, not power-loss-safe).
* m9. `newRequestID` falls back to 32 zeros if the CSPRNG fails (not unique); `inherit:filesystem` and `granted` have thin
  mutation coverage (no test of coarse `filesystem` or of the `network:fetch=` exclusion).
* m10. Suite blind spots to close in a successor suite: stderr shape for port-in-use, default-flag operation, double slashes,
  slow-body clients.

## 6. Full regression gate

`go test ./...` (reviewer run before this file was written): all packages ok, final line `EXIT 0` (76 packages with
tests; several results came from the Go test cache, so a `-count=1` re-run was done after the review files were written; its result (76 packages ok, exit 0) is recorded in
`milestone2/MILESTONE2_DECISION.md`).

## 7. Recorded decision

**NOT ACCEPTED.** Reasons: B1 (documented configuration returns 500 for valid input and breaks listing), B2 (R1.3
violated), B3 (single-connection denial of service), B4 (R3.1 violated). Gates C and H (diagnostics, observability) are
demonstrated; gates A and G are not demonstrated for the documented standalone configuration; gate D passes with
caveats. After the fixes in the carryover list, rerun `clean_repro.sh`, the new default-flag and busy-port tests, and
request a fresh independent review of the delta only.
