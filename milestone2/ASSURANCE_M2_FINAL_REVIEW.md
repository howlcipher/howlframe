# Milestone 2 final independent Assurance review (gates A, C, D, G, H)

* **Reviewer:** Assurance (independent; wrote no product code, no B6 fix, no storm script, no earlier round). Assigned by Rintaro (EM).
* **Date:** 2026-10-20, about 08:09–08:40 EDT (UTC-4). Venue: the shared box (Linux, Go 1.27.2, Python 3 stdlib).
* **Scope:** verify only. No product code (ticketdesk app, compiler, VM, runtime) is changed by this PR. It adds this record and
  `milestone2/ticketdesk/tests/regression/assurance_m10.py`. Criteria: `HOWLFRAME_PRODUCT_DIRECTIVE.md` section 6 (Milestone 2,
  D29 A, C, D, G, H) and `milestone2/PLAN.md` section 2, with the frozen `ticketdesk/requirements/REQUIREMENTS.md` as the contract.
  No criterion was weakened. Earlier journals and review records were treated as claims, and every verdict below rests on a run made here.
* **B6 is OUT OF SCOPE.** The kill-storm gate is pending William's PC run. `b6_kill_storm.py` was not run, including the Go tests that
  execute it (see the go test command below), and `MILESTONE2_DECISION.md` was not edited. Nothing here accepts or rejects B6. The
  milestone decision still needs B6.

## 1. Pin and docs-only check

* **Code reviewed:** `7e9ef32eb2eb6c71f7e0fe0280ed3240e1842989`, B6-pinned. Its parent `e8fcea20` is the Round 7 code under test, and
  `git diff --stat 7e9ef32e^ 7e9ef32e` shows only `milestone2/MILESTONE2_DECISION.md | 2 +-`. Scoring used a worktree at the pin
  (`git worktree add /workspace/m2pin 7e9ef32e`).
* **Pin to `origin/main`** (`e41b683`, after `git fetch origin`): `git diff --stat 7e9ef32e origin/main` touches 16 files. All of
  them are docs or evidence: `docs/journals/2026-10-19_ticketdesk_b6_round7.md`, `milestone2/MILESTONE2_DECISION.md`,
  `milestone2/ticketdesk/REVIEW.md` and 13 files under `milestone2/ticketdesk/evidence/round7_2026-10-19/` (`.txt`, `.log`,
  `.err`, `.sha256`). `git diff --name-only 7e9ef32e origin/main -- '*.go' '*.howl' '*.py' '*.sh' go.mod go.sum` prints nothing.
  **No non-doc code-under-test file changed.** As a cross-check, `ticketdesk.hfbc` built from `origin/main` is byte-identical
  (`cmp`) to the one built at the pin, and `assurance_m10.py` gives the same result on both.
* **Freeze integrity:** `git diff 10d9c2a8 7e9ef32e --name-status -- milestone2/ticketdesk/requirements milestone2/ticketdesk/tests/acceptance milestone2/ticketdesk/dx milestone2/PLAN.md` is empty.
* **Build at the pin:** `go build -o /tmp/m2r/howlframe .`, then
  `howlframe build milestone2/ticketdesk/src/ticketdesk.howl -o /tmp/m2r/ticketdesk.hfbc`
  (sha256 `54b90972020ce451d641c40e17fe95c1a0fe198bfd3fb0df823d9d7258fc5abe`).

## 2. Runs at the pin (all reproduced here)

| Run | Command | Result |
|---|---|---|
| Go suite | `go test -count=1 -skip 'TestRound7StormScriptSigstopSelfTest\|TestRound7StormScriptNoiseSelfTest\|TestRound7ExpectHeadPinRules' ./...` (these three tests exec `b6_kill_storm.py`, so they were skipped) | 76 packages `ok`, no FAIL, exit 0 |
| Frozen acceptance | `HF_BIN=… APP_ARTIFACT=… python3 tests/acceptance/run_acceptance.py tests/acceptance/launcher.example.sh` | `101 tests, 101 passed, 0 failed` |
| Existing regressions | `python3 tests/regression/review_round1.py tests/acceptance/launcher.example.sh` | `10 tests, 0 failed` |
| DX checker | `HF_BIN=… python3 dx/check_dx.py` | `13 checks, 13 passed, 0 failed` |
| Guide rule | `go test -count=1 -run TestHTTPAPIGuideRule -v .` | `--- PASS: TestHTTPAPIGuideRule` |
| New m10 tests | `python3 tests/regression/assurance_m10.py tests/acceptance/launcher.example.sh` | `4 tests, 0 failed, 1 skipped` (the skip is finding m12) |
| Clean repro, fresh git clone | `git clone --no-local` of the repo, checkout `7e9ef32e`, then `env -i PATH=… GOCACHE=<empty> GOMODCACHE=<empty> GOPROXY=off GOTOOLCHAIN=local bash milestone2/scripts/clean_repro.sh` | exit 0; acceptance 101/101, regression 10/10, DX 13/13; 28.6 s |
| Clean install, non-git copy | `git archive 7e9ef32e \| tar -x` into a fresh directory with no `.git`; under the same `env -i` and empty caches, run the README steps by hand: `go build -o howlframe .` and `howlframe build …`, then the suites | build OK with an empty module cache and `GOPROXY=off` (standard library only); hfbc byte-identical; 101/101, 10/10, 13/13 |
| `clean_repro.sh` from the non-git copy | same tree, `bash milestone2/scripts/clean_repro.sh` | exit 2: `fatal: not a git repository` (m7) |

Live probes used a server on localhost started with the README command (`--max-request-bytes 1048576 --access-log`, default
ceilings). The bootstrap token was generated at runtime with `python3 -c 'import secrets…'`, and issued secrets were captured into shell
variables with `jq`. No secret was printed or written to a file. Request ids in the excerpts are left out for brevity.

## 3. Per-gate verdicts

| Gate | QA verdict | Security verdict | Overall | Evidence refs |
|---|---|---|---|---|
| **A** Practical HTTP API | PASS | PASS | **PASS** (B6 excluded, see the scope note) | acceptance 101/101 (R3, R4, R5, R6, R13, R17); `m10_default_flag_operation`; `m10_double_slash_routing_and_authz`; curl CRUD probes in A-QA below; 2000-ticket listing |
| **C** Authn/authz | PASS | PASS, with documented caveats m2 and m8 | **PASS** | acceptance auth section; `ticketdesk.howl:172-182` (`sha256_hex` + `secure_equal`); curl role and ownership matrix; secret scan of the state file and logs |
| **D** Validation/errors | PASS, with documented caveats m3, m4 and m17 | PASS | **PASS** | acceptance validation/error section; curl 400/413/415/405/404/500 probes; fault probe; m4 and m17 repros below |
| **G** Standalone install/run | PASS, with documented caveats m7 and m12 | PASS | **PASS** | clean_repro from a fresh clone, exit 0; manual non-git install with an empty module cache; startup exit-code matrix; `m10_port_in_use_stderr_shape` |
| **H** Developer experience | PASS | PASS | **PASS** | DX 13/13; "before" outputs independently re-derived from `52c0c531`; `TestHTTPAPIGuideRule`; access and fault lines |

### Gate A: Practical HTTP API

**QA.**
* Acceptance 101/101.
* curl probes:
  * `GET /v1/health` → `200 {"status":"ok"}` with `Content-Type: application/json` and a 32-hex `X-Request-Id`.
  * Member `POST /v1/tickets` → `201` with `Location: /v1/tickets/t1` and the full ticket object.
  * `PATCH` own ticket → `200` with the new status.
  * `GET /v1/tickets?status=open&limit=2` → `{"n":["t2","t3"],"next_cursor":"3"}`.
  * `PUT /v1/tickets` → `405 Allow: GET, POST`; `/v2/x` → `404 not_found`.
* `m10_default_flag_operation` runs the whole workflow with no runner flags at all and the default port 8080:
  * 120/4000-code-point maximum ticket;
  * 151-ticket cursor walk;
  * status filter;
  * restart durability.
* Scale under default ceilings: 2000 creates (37.2 s, about 18 ms each, because each mutation rewrites the whole file), then:
  * `limit=1` 1 ms;
  * `limit=100` 3 ms;
  * full walk of 2004 items 0.08 s;
  * after deleting the 1500 oldest, `limit=1` → 200, first `t1501`, 2 ms.

**Security.**
* Routing is exact: `internal/httpreq/httpreq.go:377-382` uses `EscapedPath()` when a `*` fallback exists.
* 19 doubled-slash variants × 4 methods × 5 callers (none, viewer, member, owner, admin) all give `404 not_found` before
  authentication, with no `WWW-Authenticate`. Tickets and tokens are unchanged afterwards (`m10_double_slash_routing_and_authz`).
* The encoded aliases `/v1%2Ftickets`, `%2f`, `%252F`, `/%76%31/tickets`, `/v1/tickets%2F` and `/v1/tickets/t1601%2F` → 404 (m13).

### Gate C: Authentication and authorization

**QA.** Every probe below returned the required status:

| Probe | Result |
|---|---|
| Viewer create | 403 |
| Member B PATCH of member A's ticket | `403 forbidden` |
| Member DELETE of own ticket | 403 |
| Member `GET /v1/tokens` | 403 |
| No token | `401` with `WWW-Authenticate: Bearer realm="ticketdesk"` |
| Viewer revoked, then the next request | 200, then 401 |
| Two spaces after `Bearer` | 401 |
| Token create | `201` with `Cache-Control: no-store` and `Location: /v1/tokens/tok_…` |

**Security.**
* `authenticate` (`ticketdesk.howl:172-182`) hashes the presented secret and compares digests with `secure_equal`.
* The state file has no bootstrap secret and no issued secret, but does contain the SHA-256 of every issued one (scanned with
  `rg -F` against values held only in shell variables). The server's stdout and stderr (with `--access-log`) contain neither.
* Removing the `network` grant gives exit 77. A `network:fetch=127.0.0.1:9` grant in place of `network` gives
  `ticketdesk: capability denied: network`, rc 77.
* Caveats m2 and m8 (section 4).

### Gate D: Validation and reliable errors

**QA.** All of these give the R8.1 shape:

| Input | Response |
|---|---|
| One create with 4 bad fields | one `validation_failed` listing all 4 field codes |
| `{"title":` | `invalid_json` |
| `text/plain` | `unsupported_media_type` |
| 1048600-byte body | `payload_too_large` |
| `m10_default_flag_operation`: body of exactly 1048576 bytes | 400 `invalid_json` (parsed, not 413) |
| `m10_default_flag_operation`: body of 1048577 bytes | 413 |

Fault probe with `TICKETDESK_ALLOW_DEBUG_FAULT=1` and `X-Debug-Fault: 1`:
* `500 {"error":{"code":"internal_error","message":"An internal error occurred.","request_id":<same as header>}}`;
* stderr `fault request_id=<id> method=GET path=/v1/tickets error=…`;
* health is still 200 afterwards.

Caveats m3, m4 and m17 (section 4).

**Security.** The client sees no runtime detail, and secrets are absent from logs (gate C).

### Gate G: Standalone installation and execution

**QA.**
* Clean runs: see section 2. They need only Go, Python 3 and (for the script) git/tar/bash/mktemp. The root binary builds with
  `GOMODCACHE=<empty> GOPROXY=off`.
* Startup matrix at the pin:

| Startup case | Exit | stderr |
|---|---|---|
| Drop network / database / environment / `filesystem:read=` / `filesystem:write=` | 77 each | `ticketdesk: capability denied: <name>` |
| Missing or short admin token | 2 | prefixed line |
| `TICKETDESK_PORT=70000` | 2 | prefixed line |
| Relative `TICKETDESK_DATA` | 2 | prefixed line |
| Missing directory | 66 | prefixed line |
| Corrupt state | 65 | prefixed line |

* Port in use (0.0.0.0, 127.0.0.1 and `[::]` v6only): nonzero exit within 10 s, one `ticketdesk: port N is already in use` line,
  and the existing data file byte-identical (`m10_port_in_use_stderr_shape`).
* Caveats m7 and m12.

**Security.**
* The only artifacts are `howlframe` and `ticketdesk.hfbc`. There is no generated server.
* The grants are exactly the five in R2.1. An extra `process` grant is harmless.

### Gate H: Developer experience

**QA.**
* `dx/check_dx.py` 13/13.
* I rebuilt the freeze-commit toolchain (`git archive 52c0c531`, `go build`) and re-ran every fixture with old output:
  * 11 of 11 command outputs match the recorded `old_output`;
  * all 11 differ at the pin.
* Pin diagnostics checked by hand:
  * innermost unclosed `(let` named with line and column;
  * `(call triple ...)` hint;
  * `let` → `(do ...)` hint;
  * if-branch lines plus remedies.
* `TestHTTPAPIGuideRule` passes.
* With `--access-log`, the server writes one `access method=… path=… status=… duration_ms=… request_id=…` line per request. A fault
  always writes a `fault` line, with or without the flag.

**Security.** Log lines carry no secrets (gate C scan).

## 4. Caveat classifications

Every id was grepped in `milestone2/` (sources: `ticketdesk/REVIEW.md` Round 1 §5 minor list, R2.4, R3.5, `MILESTONE2_DECISION.md`
carryover lines, `ticketdesk/README.md` known limitations).

**Requirement-letter deviations:** m2 (R9), m3 and m17 (R8.1, R3.3) and m12 (R1.3) do not meet the letter of the frozen
REQUIREMENTS.md. I classify them as documented-accepted on impact. That acceptance is a waiver the Owner should make explicitly. It is
not a claim that the requirement is met.

| Id | Status at pin | Classification | One-line reasoning and evidence |
|---|---|---|---|
| **m1** listing claim and performance | **FIXED** (re-verified) | n/a | README:54-56 now describes the bucket index. Measured: 2004 tickets, `limit=100` in 3 ms, full walk 0.08 s; after 1500 deletions, `limit=1` in 2 ms. The "thousands" claim is gone. |
| **m13** `%2F` alias | **FIXED** (re-verified) | n/a | `httpreq.go:382` `EscapedPath()`. 6 encoded variants → 404; `review_round1.py` `m13_encoded_slash_is_not_an_alias` PASS. |
| **m2** trailing space after the bearer secret | present: `Authorization: Bearer <member secret> ` → 200 | documented-accepted (R9 deviation) | Go `net/http` strips trailing OWS (RFC 9110 §5.5) before the app sees the value, so the app cannot tell. No security impact, because the full secret is still needed. Documented at README:59. R9 lists trailing whitespace as 401, so this needs an Owner waiver. |
| **m3** transport-level rejections | present: `/v1/tickets/%zz` → `400 text/plain "400 Bad Request"`; 2 MB header → `431 text/plain`; `OPTIONS *` → empty 200 | documented-accepted (R8.1 deviation) | `net/http` answers before any route or handler runs. Inputs are not valid HTTP or URL syntax and are rejected predictably. Recorded only in REVIEW.md (m3, ADV36/37/89); user docs do not mention it (non-blocking doc gap). |
| **m4** shallow state validation | present: hand-edited state `b:00000000 {"ids":"x"}` starts, then `GET /v1/tickets` and `POST` → 500 (2 fault lines); `title: 7` is served as is; `meta.next_ticket: -5` is accepted. Wrong-typed meta and a non-object ticket → 65 | documented-accepted | Reachable only by tampering with the state file outside the API. Failures are the safe 500 with no leak, and R1.3 rejection of corrupt or unreadable state holds. Recorded in REVIEW.md m4. |
| **m7** clean_repro from a non-git copy | present: `fatal: not a git repository`, exit 2 | documented-accepted | PLAN G requires "a clean checkout of the committed tree", and a fresh clone passes (exit 0). The README manual steps work from a non-git copy (101/101, 10/10, 13/13). Recorded in REVIEW.md m7. |
| **m8** state file mode | present: state file `0644`, **forced** by `internal/vm/vm.go:1906` `os.Chmod(name, 0o644)` even under `umask 077`. The directory fsync after rename is present (`vm.go:1917-1921`) | documented-accepted (security hardening item) | The file holds ticket data and SHA-256 digests of 256-bit secrets, never a secret (R12 holds). An operator-chosen data directory with mode 0700 blocks other local users. Recorded in REVIEW.md m8; not in README (recommend documenting it or using 0600). |
| **m9** grant-coverage tests | present: the coverage gap is shown by mutation. The request-id zero fallback is fixed (`vm.go:4322-4329` panics instead) | documented-accepted (test gap) | Mutating `internal/vm/crypto.go:51` (drop `name != "network" &&`) leaves `go test ./...` at 76 ok / exit 0 and the acceptance suite unaffected. Yet a mutated server with only `network:fetch=…` then prints a raw `CAPABILITY_DENIED` object with rc 1 instead of 77. Behaviour at the pin is correct (rc 77). Recommend a test. |
| **m12** port-probe race | present, **deterministically reproduced** (finding below) | documented-accepted (R1.3 deviation) | Reachable only if another process binds the port in the window between the startup probe (`vm.go:3532`) and `ListenAndServe` (`vm.go:3045-3051`). Exit is still nonzero, nothing listens, and the data file is not modified, but stderr shows the raw `{"phase":"runtime","code":"IO_ERROR",…}` object. Listed as non-blocking carryover in MILESTONE2_DECISION.md. |
| **m17** truncated-upload 400 | present: a half-closed upload gets an immediate 400, and a stalled one gets a 400 after 30.0 s. Body `{"error":"request body was not received completely","request_id":"…"}`, no `X-Request-Id` header. Never executed: list empty | documented-accepted (R8.1/R3.3 deviation) | Only clients that break HTTP framing (Content-Length not delivered) see it. The safety property (not executed) holds. Recorded as non-blocking carryover in MILESTONE2_DECISION.md. |
| **#194** | code folded in; **GitHub issue still OPEN** | not done administratively | `b6_kill_storm.py:594-601` polls `/proc/PID/stat` for state `T`; `verify_head` compares byte-exact (`:1108-1110`, no `rstrip`); `milestone2_round7_pin_test.go:213` has the `pin-plus-trailing-space` case. These were verified by reading only, because running those tests executes the storm script (out of scope). The issue can be closed once the Owner agrees. |

## 5. m10 regression tests (`milestone2/ticketdesk/tests/regression/assurance_m10.py`)

Conventions follow `review_round1.py`: the frozen launcher, default runner ceilings, a fresh temp data directory per test, stdlib
only, `PASS`/`FAIL` lines and the `N tests, M failed` summary. The bootstrap token is generated at runtime with `secrets` and never
printed. `M10_ONLY=<name>` selects tests. `M10_RUN_SKIPPED=1` runs the skipped finding test. Run with
`HF_BIN=… APP_ARTIFACT=… python3 milestone2/ticketdesk/tests/regression/assurance_m10.py milestone2/ticketdesk/tests/acceptance/launcher.example.sh`.
It takes about 65 s. `clean_repro.sh` is not changed by this PR.

| Test | What it asserts | At pin |
|---|---|---|
| `m10_port_in_use_stderr_shape` | Busy listener on 0.0.0.0, 127.0.0.1 or `[::]` (v6only) → nonzero exit within 10 s. Every stderr line starts with `ticketdesk: ` and ends with `\n`, and names the port. None of `"phase"`, `Traceback`, `panic:`, `goroutine `, `RUNTIME_ERROR`, `IO_ERROR`, `{` appears. A server-written data file is byte-identical afterwards. | PASS |
| `m10_port_lost_race_stderr_shape` | The same stderr rules when the port is taken after the probe. The child's stdout is a full 4 KiB pipe, so it blocks on `Listening on N` (`vm.go:3039`), which is printed after the probe and right before the bind. The test takes the port, then drains the pipe. | **FAIL → SKIPPED** (finding m12) |
| `m10_default_flag_operation` | No `HF_RUN_FLAGS` and no `TICKETDESK_PORT` (8080 when free). Covers: health with an inert fault header; member and viewer tokens; a 120/4000-code-point `é` ticket; 150 more; a 151-item cursor walk; PATCH; status filter; 1048576 bytes → 400 `invalid_json`; 1048577 → 413; admin DELETE; no `access ` lines; no secret on stderr; state survives a restart. | PASS |
| `m10_double_slash_routing_and_authz` | Canonical controls first (viewer DELETE 403, foreign PATCH 403, member DELETE 403, no token 401, member token list 403). Then 19 doubled-slash paths (for example `//v1/tickets`, `/v1/tickets//t1`, `/v1/tickets/t1//`, `/v1/tokens//<id>`, `//v1/health`) × GET/POST/PATCH/DELETE × 5 callers, sent byte-exact over raw sockets. Every one must be `404 not_found` with an `X-Request-Id` and no `WWW-Authenticate`. Tickets and tokens must be identical afterwards, and the viewer token must still be valid. | PASS |
| `m10_slow_body_clients_bounded` | Six authenticated admin slow-body POSTs (1 byte/s, never complete), two slow-header clients and one slow but complete upload (about 3.5 s). For 14 s, health, an authenticated list and creates must each answer in under 2 s. Slow-header connections must close within 16 s and slow-body ones within 40 s. The slow complete upload must get 201, no cut-off upload may be executed, and the server must stay healthy. | PASS (`worst normal request 0.01 s; slow-header closed after [10.0, 10.0] s; slow-body closed after [30.0 ×6] s`) |

### Mutation proofs

Each mutation was applied to the pin worktree, then the binary and artifact were rebuilt into `/tmp/m2r/mut`, the one test was run
with `M10_ONLY`, and the file was reverted with `git checkout -- <file>`. After every mutation the script printed
`reverted; tracked changes left: ''`. The m9 mutation ran in a separate `git archive` copy. None of this is in the PR.

| # | File | Change | Test | Failing excerpt |
|---|---|---|---|---|
| M1 | `milestone2/ticketdesk/src/ticketdesk.howl:105` | delete the `(if (not (port_available port)) (call die 1 …))` line | `m10_port_in_use_stderr_shape` | `FAIL … every stderr line must start with 'ticketdesk: ': '{"phase":"runtime","code":"IO_ERROR",…"http listen failed: listen tcp :33011: bind: address already in use"}'` |
| M2a | `howlframe.go:926` | `run` default `--max-instructions` changed to `vm.DefaultLimits.MaxInstructions/5` | `m10_default_flag_operation` | `FAIL m10_default_flag_operation - list page -> 500` |
| M2b | `internal/vm/error.go:161` | `MaxRequestBodyBytes: 1024*1024 - 1` | `m10_default_flag_operation` | `FAIL … body at default ceiling: 413 {'error': {'code': 'payload_too_large', …}}` |
| M3 | `internal/httpreq/httpreq.go:382` | `cleaned = cleanRoutePath(r.URL.EscapedPath())` (put back path cleaning) | `m10_double_slash_routing_and_authz` | `FAIL … 284 alias requests not answered 404 not_found before auth, e.g. ['GET //v1/tickets as none -> 401 unauthorized', 'GET //v1/tickets as viewer -> 200 None', …]` |
| M4a | `internal/vm/vm.go:3045-3046` | remove `ReadHeaderTimeout` and `ReadTimeout` | `m10_slow_body_clients_bounded` | `FAIL … slow-header client held its connection 55.1 s (ReadHeaderTimeout is 10 s)` |
| M4b | `internal/vm/vm.go:2950-2967` | take `serial.Lock()` before the bounded body pre-read (the B3 regression) | `m10_slow_body_clients_bounded` | `FAIL … GET /v1/health while slow clients were connected: no answer within 5 s (timed out)` |
| (race) | none (the pin itself) | n/a | `m10_port_lost_race_stderr_shape` with `M10_RUN_SKIPPED=1` | `FAIL … every stderr line must start with 'ticketdesk: ': '{"phase":"runtime","code":"IO_ERROR","function":"main","instruction":30,"opcode":"HTTP_SERVER_SERVE","message":"http listen failed: listen tcp :50517: bind: address already in use"}'` |
| (m9) | `internal/vm/crypto.go:51` | drop `name != "network" &&` | `go test ./...` (shows the gap, not an m10 test) | 76 ok, exit 0: no test catches it |

## 6. Confirmed blockers

**None for gates A, C, D, G and H at `7e9ef32e`.** B6 is out of scope and unassessed here.

Non-blocking finding with a minimal repro, **m12** (classified documented-accepted, R1.3 letter deviation):

```
# server stdout = a pre-filled 4 KiB pipe (fcntl F_SETPIPE_SZ 4096); start the documented server on free port P
# after 1.5 s (probe done, child blocked writing "Listening on P"): bind+listen 0.0.0.0:P; drain the pipe
# result: rc=1, stderr = {"phase":"runtime","code":"IO_ERROR",...,"opcode":"HTTP_SERVER_SERVE","message":"http listen failed: listen tcp :P: bind: address already in use"}
M10_RUN_SKIPPED=1 M10_ONLY=m10_port_lost_race_stderr_shape python3 milestone2/ticketdesk/tests/regression/assurance_m10.py <launcher>
```

Fix direction (not done here; product code is out of scope): bind once and serve on that listener, or map a listen failure to a
prefixed line. Then remove the test's `SKIPPED` entry.

## 7. Not verified here

* B6 and anything that runs `b6_kill_storm.py`: `TestRound7StormScriptSigstopSelfTest`, `TestRound7StormScriptNoiseSelfTest` and
  `TestRound7ExpectHeadPinRules`. The #194 items were checked by reading code only.
* Performance and power-loss durability beyond the probes above (PLAN section 6).
* Other operating systems. Every run was on the shared Linux box.
