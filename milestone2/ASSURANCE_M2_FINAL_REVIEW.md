# Milestone 2 final independent Assurance review (gates A, C, D, G, H)

* **Reviewer and independence:** Assurance (`bot-assurance-0001`), assigned by Rintaro (EM). I did not build any of the ticketdesk
  code, the runtime/VM changes, the B6 fix or the storm script, and I took no part in any earlier review round. Commits go through the
  shared `howlcipher` account, so git authorship cannot show this. The assignment and this statement are the record; PR #203 and
  Motoko's review `5585112963` hold the conversation.
* **Files this PR touches:** it adds only `milestone2/ASSURANCE_M2_FINAL_REVIEW.md` (this record) and
  `milestone2/ticketdesk/tests/regression/assurance_m10.py`. Commit `0504a55` also added a CI step to `.github/workflows/ci.yml`, but
  GitHub rejected the push because the token lacks `workflow` scope, so `f8b95a7` backs it out. The PR's net diff to `ci.yml` is
  empty (section 5).
* **Date:** 2026-10-20, about 08:09–08:40 EDT (UTC-4), revised about 08:45–09:10 EDT after Motoko's review. Venue: the shared box (Linux, Go 1.27.2, Python 3 stdlib).
* **Scope:** verify only. No product code (ticketdesk app, compiler, VM, runtime) is changed by this PR. Criteria: `HOWLFRAME_PRODUCT_DIRECTIVE.md` section 6 (Milestone 2,
  D29 A, C, D, G, H) and `milestone2/PLAN.md` section 2, with the frozen `ticketdesk/requirements/REQUIREMENTS.md` as the contract.
  No criterion was weakened. Earlier journals and review records were treated as claims, and every verdict below rests on a run made here.
* **B6 is OUT OF SCOPE.** The kill-storm gate is pending William's PC run. `b6_kill_storm.py` was not run, including the Go tests that
  execute it (see the go test command below), and `MILESTONE2_DECISION.md` was not edited. Nothing here accepts or rejects B6. The
  milestone decision still needs B6.

## 0. Owner decisions (recorded 2026-10-20, 11:02 EDT, UTC-4)

William Elias (Owner) decided the open items from this review in chat. The EM (Rintaro) had recommended granting all five waivers
so that the B6 pin `7e9ef32e` stays unchanged, and keeping the m10 tests manual-only. William's replies:

* "Manual-only is fine for the m10 tests in M2" (answering how the new regression tests should run).
* "Do what you suggest and merge" (answering the waive-all-five vs fix recommendation, and the merge).

Recorded decisions:

1. **Waivers granted for M2:** W-m2 (gate C, R9: a bearer secret with one trailing space still authenticates), W-m3 (gates D R8.1 and
   A R3.3: transport-level rejections are plain text), W-m17 (gates D and A: the truncated-upload 400 is not in the uniform error
   shape), W-m12 (gate G, R1.3: a port-probe race can print the raw runtime error object) and W-m4 (gate G, R1.3: semantically
   corrupt state starts, then answers 500). No product fix is made for these in M2, and the **B6 pin `7e9ef32e` is kept**.
2. **m10 tests are manual-only for M2:** `assurance_m10.py` is manual-only until a workflow-scoped change adds the CI step
   (section 5).

With these waivers, gates A, C, D and G count as **PASS under Owner waivers** and H is an unconditional PASS. **Milestone 2 is still
not accepted:** acceptance needs the B6 kill storm to pass on William's PC run. This record does not edit `MILESTONE2_DECISION.md`.

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
| New m10 tests | `python3 tests/regression/assurance_m10.py tests/acceptance/launcher.example.sh` | `5 tests, 0 failed, 1 expected failures` (the strict expected failure reproduces m12), exit 0, 33 s |
| Clean repro, fresh git clone | `git clone --no-local` of the repo, checkout `7e9ef32e`, then `env -i PATH=… GOCACHE=<empty> GOMODCACHE=<empty> GOPROXY=off GOTOOLCHAIN=local bash milestone2/scripts/clean_repro.sh` | exit 0; acceptance 101/101, regression 10/10, DX 13/13; 28.6 s |
| Clean install, non-git copy | `git archive 7e9ef32e \| tar -x` into a fresh directory with no `.git`; under the same `env -i` and empty caches, run the README steps by hand: `go build -o howlframe .` and `howlframe build …`, then the suites | build OK with an empty module cache and `GOPROXY=off` (standard library only); hfbc byte-identical; 101/101, 10/10, 13/13 |
| `clean_repro.sh` from the non-git copy | same tree, `bash milestone2/scripts/clean_repro.sh` | exit 2: `fatal: not a git repository` (m7) |

Live probes used a server on localhost started with the README command (`--max-request-bytes 1048576 --access-log`, default
ceilings). The bootstrap token was generated at runtime with `python3 -c 'import secrets…'`, and issued secrets were captured into shell
variables with `jq`. No secret was printed or written to a file. Request ids in the excerpts are left out for brevity.

## 3. Per-gate verdicts

**Gates A, C, D and G pass only under the Owner waivers below.** Each one is "PASS conditional on Owner waivers". William
granted all five on 2026-10-20 11:02 EDT (section 0), so A, C, D and G now count as **PASS under Owner waivers**. H is the only
unconditional PASS. All five exclude B6 (see the scope note), and M2 is not accepted until B6 passes.

Each waiver is mapped to its gate through the frozen requirement-to-gate map (`REQUIREMENTS.md`: "A: R3, R4, R5, R6, R13, R17.
C: R9, R10, R11, R12. D: R7, R8, R14, R15, (R16). G: R1, R2."). The waivers themselves are in section 4.1.

| Gate | QA verdict | Security verdict | Overall | Evidence refs |
|---|---|---|---|---|
| **A** Practical HTTP API | PASS except the R3.3 deviations m3 and m17 | PASS | **PASS conditional on Owner waivers W-m3, W-m17 (R3.3)**; B6 excluded | acceptance 101/101 (R3, R4, R5, R6, R13, R17); `m10_default_flag_operation`; `m10_double_slash_routing_and_authz`; curl CRUD probes in A-QA below; 2000-ticket listing; m3 and m17 repros in section 4 |
| **C** Authn/authz | PASS | PASS except the R9 deviation m2; m8 is within the letter (recommendation only) | **PASS conditional on Owner waiver W-m2 (R9)** | acceptance auth section; `ticketdesk.howl:172-182` (`sha256_hex` + `secure_equal`); curl role and ownership matrix; secret scan of the state file and logs |
| **D** Validation/errors | PASS except the R8.1 deviations m3 and m17 | PASS | **PASS conditional on Owner waivers W-m3, W-m17 (R8.1)** | acceptance validation/error section; curl 400/413/415/405/404/500 probes; fault probe; m3 and m17 repros in section 4 |
| **G** Standalone install/run | PASS except the R1.3 deviations m12 and m4; m7 is within the letter | PASS | **PASS conditional on Owner waivers W-m12, W-m4 (R1.3)** | clean_repro from a fresh clone, exit 0; manual non-git install with an empty module cache; startup exit-code matrix; `m10_port_in_use_stderr_shape`; `m10_port_lost_race_stderr_shape` (strict expected failure); m4 repro |
| **H** Developer experience | PASS | PASS | **PASS** (unconditional) | DX 13/13; "before" outputs independently re-derived from `52c0c531`; `TestHTTPAPIGuideRule`; access and fault lines |

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
* m2 breaks R9, so gate C needs waiver W-m2 (section 4.1). m8 is a recommendation only (section 4).

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

Deviations m3 and m17 break R8.1, so gate D needs waivers W-m3 and W-m17 (section 4.1). m4 belongs to gate G (R1.3).

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
* m7 is within the letter. m12 and m4 break R1.3, so gate G needs waivers W-m12 and W-m4 (section 4.1).

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

**Classification rule.** An item that does not meet the letter of the frozen `REQUIREMENTS.md` is **waiver-pending**: it blocks
its gate's PASS until William records the waiver in 4.1. Five items fall here: m2 (R9), m3 and m17 (R8.1 and R3.3), and m12 and m4
(R1.3). I recommend granting all five on impact. That is a recommendation for the Owner to decide, not a claim that the requirement
is met. Items within the letter are documented-accepted (m7, m8, m9), and m1 and m13 are fixed.

| Id | Status at pin | Classification | One-line reasoning and evidence |
|---|---|---|---|
| **m1** listing claim and performance | **FIXED** (re-verified) | n/a | README:54-56 now describes the bucket index. Measured: 2004 tickets, `limit=100` in 3 ms, full walk 0.08 s; after 1500 deletions, `limit=1` in 2 ms. The "thousands" claim is gone. |
| **m13** `%2F` alias | **FIXED** (re-verified) | n/a | `httpreq.go:382` `EscapedPath()`. 6 encoded variants → 404; `review_round1.py` `m13_encoded_slash_is_not_an_alias` PASS. |
| **m2** trailing space after the bearer secret | present: `Authorization: Bearer <member secret> ` → 200 | **waiver-pending W-m2** (R9; gate C) | Go `net/http` strips trailing OWS (RFC 9110 §5.5) before the app sees the value, so the app cannot tell. No security impact, because the full secret is still needed. Documented at README:59. R9 lists trailing whitespace as 401, so this needs an Owner waiver. |
| **m3** transport-level rejections | present: `/v1/tickets/%zz` → `400 text/plain "400 Bad Request"`; 2 MB header → `431 text/plain`; `OPTIONS *` → empty 200. None of the three carries `X-Request-Id` | **waiver-pending W-m3** (R8.1, R3.3; gates D and A) | `net/http` answers before any route or handler runs. Inputs are not valid HTTP or URL syntax and are rejected predictably. Recorded only in REVIEW.md (m3, ADV36/37/89); user docs do not mention it (non-blocking doc gap). |
| **m4** shallow state validation | present: a hand-edited state with `b:00000000 {"ids":"x"}` **starts**, then `GET /v1/tickets` and `POST` → 500 (2 fault lines). `title: 7` is served as is, and `meta.next_ticket: -5` is accepted. Wrong-typed `meta` and a non-object ticket do exit 65 | **waiver-pending W-m4** (R1.3; gate G) | R1.3 requires exit 65 for state that is "not valid ticketdesk state (corrupt, …)". This state is corrupt and still starts, so R1.3 is **not** met for semantically corrupt state. I see no rigorous argument that it counts as valid. Only reachable by editing the file outside the API; the failure is a safe 500 with no leak. |
| **m7** clean_repro from a non-git copy | present: `fatal: not a git repository`, exit 2 | documented-accepted | PLAN G requires "a clean checkout of the committed tree", and a fresh clone passes (exit 0). The README manual steps work from a non-git copy (101/101, 10/10, 13/13). Recorded in REVIEW.md m7. |
| **m8** state file mode | present: state file `0644`, **forced** by `internal/vm/vm.go:1906` `os.Chmod(name, 0o644)` even under `umask 077`. The directory fsync after rename is present (`vm.go:1917-1921`) | documented-accepted (within the letter: REQUIREMENTS has no file-mode clause, and R12 holds) | The file holds ticket data and SHA-256 digests of 256-bit secrets, never a secret. Because 0644 is forced even under `umask 077`, tickets are world-readable to other local users unless the operator restricts the directory. **Recommendation:** write the state file with mode 0600 (drop the forced `os.Chmod(name, 0o644)` at `vm.go:1906`, or make it 0600). If that is not done, document in README "Known limitations" that the data directory should be mode 0700. Recorded in REVIEW.md m8. |
| **m9** grant-coverage tests | present: the coverage gap is shown by mutation. The request-id zero fallback is fixed (`vm.go:4322-4329` panics instead) | documented-accepted (test gap) | Mutating `internal/vm/crypto.go:51` (drop `name != "network" &&`) leaves `go test ./...` at 76 ok / exit 0 and the acceptance suite unaffected. Yet a mutated server with only `network:fetch=…` then prints a raw `CAPABILITY_DENIED` object with rc 1 instead of 77. Behaviour at the pin is correct (rc 77). Recommend a test. |
| **m12** port-probe race | present, **deterministically reproduced** (finding below) | **waiver-pending W-m12** (R1.3; gate G) | Reachable only if another process binds the port in the window between the startup probe (`vm.go:3532`) and `ListenAndServe` (`vm.go:3045-3051`). Exit is still nonzero, nothing listens, and the data file is not modified, but stderr shows the raw `{"phase":"runtime","code":"IO_ERROR",…}` object. Listed as non-blocking carryover in MILESTONE2_DECISION.md. |
| **m17** truncated-upload 400 | present: a half-closed upload gets an immediate 400, and a stalled one gets a 400 after 30.0 s. Body `{"error":"request body was not received completely","request_id":"…"}`, no `X-Request-Id` header. Never executed: list empty | **waiver-pending W-m17** (R8.1, R3.3; gates D and A) | Only clients that break HTTP framing (Content-Length not delivered) see it. The safety property (not executed) holds. Recorded as non-blocking carryover in MILESTONE2_DECISION.md. |
| **#194** | code folded in; **GitHub issue still OPEN** | not done administratively | `b6_kill_storm.py:594-601` polls `/proc/PID/stat` for state `T`; `verify_head` compares byte-exact (`:1108-1110`, no `rstrip`); `milestone2_round7_pin_test.go:213` has the `pin-plus-trailing-space` case. These were verified by reading only, because running those tests executes the storm script (out of scope). The issue can be closed once the Owner agrees. |

### 4.1 Owner waivers requested (decidable from this record)

William records each decision. Until he does, the mapped gates stay conditional (section 3). **Decided 2026-10-20 11:02 EDT:
all five granted as option (c), waive as is, for M2 (section 0).**

**W-m2 → gate C (R9).**
* *Broken text* (`REQUIREMENTS.md:195`): "`401 unauthorized` (+ `WWW-Authenticate`) for: … a secret with leading/trailing whitespace; …"
* *Observed:* `Authorization: Bearer <secret> ` (one trailing space) → 200. Leading or doubled whitespace → 401, as required.
* *Impact:* none for security. The caller must still present the full, exact secret. Go `net/http` strips trailing optional
  whitespace (RFC 9110 §5.5) before the application can read the header. Documented at README:59.
* *Fix options:*
  * (a) Runtime: wrap the listener's `net.Conn` to keep the raw `Authorization` header line, then expose it to `req_header` or reject
    trailing whitespace before dispatch. This is invasive, and it duplicates header parsing.
  * (b) A separately recorded requirements revision of R9 that excludes RFC 9110 trailing OWS (directive line 247: a revision kept
    distinct from the original).
  * (c) Waive as is.

**W-m3 → gates D (R8.1) and A (R3.3).**
* *Broken text:*
  * R8.1 (`REQUIREMENTS.md:166`): "Every non-2xx response, from every route and every failure class, is exactly:" (followed by the
    `{"error":{"code",…,"request_id"}}` shape).
  * R3.3 (`REQUIREMENTS.md:88-91`): "Every response, success or error, from any route including 404 and 405, carries:
    `Content-Type: application/json` … and a JSON body. There are no empty-body responses." and "`X-Request-Id`: exactly 32
    lowercase hex characters, generated by the server per request".
* *Observed:*
  * a malformed percent-escape gives `400 text/plain "400 Bad Request"`;
  * a 2 MB header gives `431 text/plain`;
  * `OPTIONS *` gives `200` with an empty body.
  * None of them has `X-Request-Id`.
* *Impact:* only requests that are not valid HTTP or URL syntax, which `net/http` rejects before any route or handler runs. They are
  rejected predictably, and no data is exposed. Clients lose the uniform error code and request id in these cases. User docs don't
  mention it.
* *Fix options:*
  * (a) Runtime: pre-validate the request line and header size at the connection or listener level and answer with the R8.1 envelope
    plus `X-Request-Id`. For `OPTIONS *`, send a JSON 404/405 from a top-level `http.Handler` before the mux.
  * (b) Requirements revision scoping R3.3 and R8.1 to requests that reach routing.
  * (c) Waive and document it in the guide and README.

**W-m17 → gates D (R8.1) and A (R3.3).**
* *Broken text:* R8.1 (`REQUIREMENTS.md:166,173`): "Every non-2xx response, from every route and every failure class, is exactly:"
  and "`code`, `message` … and `request_id` are always present". R3.3 (`REQUIREMENTS.md:91`): "`X-Request-Id`: exactly 32 lowercase
  hex characters, generated by the server per request".
* *Observed:* when a declared Content-Length is not delivered, the server answers `400`, with body
  `{"error":"request body was not received completely","request_id":"…"}` (`error` is a string; `code` and `message` are missing)
  and no `X-Request-Id` header (`vm.go:2957-2963`, where the header is set only after this early return). The upload is never
  executed.
* *Impact:* only clients that break HTTP framing see it. The safety property holds (not executed), and the id is still in the body.
* *Fix options:*
  * (a) Runtime: in the pre-read error branch, set `X-Request-Id` and write
    `{"error":{"code":"invalid_json","message":"…","request_id":"…"}}`. This needs an agreed code, because R8.2 has no
    "incomplete body" code; `invalid_json` 400 is the closest existing pairing.
  * (b) Drop the connection without a response. Then no non-compliant response exists.
  * (c) Waive.

**W-m12 → gate G (R1.3).**
* *Broken text* (`REQUIREMENTS.md:41,51`): "stderr lines start with `ticketdesk: `, each terminated by `\n`; … no raw runtime error
  object, `"phase":`, `Traceback`, `panic:` or `goroutine ` text appears on stderr" for "The port is already in use | any nonzero
  exit (not 0), within 10 s".
* *Observed:* if the port is taken between the startup probe (`vm.go:3532`) and `ListenAndServe` (`vm.go:3045-3051`), stderr is
  `{"phase":"runtime","code":"IO_ERROR",…,"opcode":"HTTP_SERVER_SERVE",…"bind: address already in use"}` and rc is 1. Reproduced
  deterministically (`m10_port_lost_race_stderr_shape`, strict expected failure).
* *Impact:* a narrow startup race. Exit is still nonzero within 10 s, nothing listens, and the data file is unchanged. Only the stderr
  text is wrong.
* *Fix options:*
  * (a) Runtime: bind once, in the probe or at server start, and serve on that same listener (`server.Serve(ln)`), so no window exists.
  * (b) Map a listen failure in `OpHttpServerServe` to a catchable error the app turns into `ticketdesk: port N is already in use`.
  * (c) Waive.
  * After a fix, `assurance_m10.py` reports XPASS and fails until its XFAIL entry is removed.

**W-m4 → gate G (R1.3).**
* *Broken text* (`REQUIREMENTS.md:48`): "`TICKETDESK_DATA` names an existing path that is not valid ticketdesk state (corrupt, a
  directory, not readable as state). A zero-byte regular file is valid empty state | 65".
* *Observed:* parseable but semantically corrupt state is accepted at startup. For example, bucket `{"ids":"x"}` starts and then every
  list or create is a 500. Startup validation (`prove_write`, `ticketdesk.howl:49-58`) checks only that `meta` counters are integers.
* *Impact:* only reachable by editing the state file outside the API. Crash windows are covered by the B6 work, which is out of scope
  here. The failure is a safe 500 with a fault line and no leak, but the server runs in a broken state instead of refusing to start.
* *Fix options:*
  * (a) App: deep-validate on load. Check every `t:`, `b:`, `k:`, `h:` and `q:` record's type and fields, that bucket ids are integer
    lists, and that counters are positive. Exit 65 on any violation. This costs startup time proportional to the state size.
  * (b) Requirements revision limiting R1.3's 65 to syntactically unreadable state.
  * (c) Waive.

## 5. m10 regression tests (`milestone2/ticketdesk/tests/regression/assurance_m10.py`)

Conventions follow `review_round1.py`: the frozen launcher, default runner ceilings, a fresh temp data directory per test, stdlib
only, `PASS`/`FAIL` lines and the `N tests, M failed` summary. The bootstrap token is generated at runtime with `secrets` and never
printed. `M10_ONLY=<name>` selects tests. Run with
`HF_BIN=… APP_ARTIFACT=… python3 milestone2/ticketdesk/tests/regression/assurance_m10.py milestone2/ticketdesk/tests/acceptance/launcher.example.sh`.
It takes about 33 s. `clean_repro.sh` is not changed by this PR.

* **Strict expected failure.** `m10_port_lost_race_stderr_shape` is in `XFAIL`. It always runs, and it is an expected failure because
  it reproduces m12 deterministically, not because it is flaky. It is reported as follows:
  * `XFAIL` (run still passes) only if the failure message contains all of `"phase":"runtime"`, `"code":"IO_ERROR"`,
    `"opcode":"HTTP_SERVER_SERVE"` and `address already in use`;
  * `XPASS`, which **fails the run**, if the test passes (m12 looks fixed: remove the entry);
  * `FAIL` if it fails any other way.
* **CI: not wired yet; needs someone with `workflow` scope.**
  * `.github/workflows/ci.yml` is **not frozen**:
    * The frozen paths are `milestone2/PLAN.md`, `ticketdesk/requirements`, `ticketdesk/tests/acceptance` and `ticketdesk/dx`
      (freeze diff vs `10d9c2a8`).
    * The hash-pinned files are the storm script and the experiment specs (`*spec_freeze_test.go`).
    * No test or document restricts edits to `ci.yml`: `git grep ci.yml` outside markdown finds nothing.
  * I added the step below, but the push was refused: `refusing to allow an OAuth App to create or update workflow
    .github/workflows/ci.yml without workflow scope`. So it is backed out. **Owner decision (section 0): `assurance_m10.py` is
    manual-only for M2, manual-only until a workflow-scoped change adds the CI step.**
  * The step goes right after the existing "Build howlframe CLI" step. To add it, William's OK or a credential with `workflow` scope
    is needed:

    ```yaml
        - name: Ticketdesk m10 assurance regressions (default runner ceilings)
          run: |
            ./howlframe build milestone2/ticketdesk/src/ticketdesk.howl -o "$RUNNER_TEMP/ticketdesk.hfbc"
            HF_BIN="$PWD/howlframe" APP_ARTIFACT="$RUNNER_TEMP/ticketdesk.hfbc" python3 milestone2/ticketdesk/tests/regression/assurance_m10.py milestone2/ticketdesk/tests/acceptance/launcher.example.sh
    ```

  * I ran that exact step locally against a copy of the branch, extracted from the YAML and run with `bash -euo pipefail`:
    `5 tests, 0 failed, 1 expected failures`, exit 0.
  * Manual command until then: `go build -o howlframe howlframe.go && ./howlframe build milestone2/ticketdesk/src/ticketdesk.howl -o /tmp/ticketdesk.hfbc && HF_BIN=$PWD/howlframe APP_ARTIFACT=/tmp/ticketdesk.hfbc python3 milestone2/ticketdesk/tests/regression/assurance_m10.py milestone2/ticketdesk/tests/acceptance/launcher.example.sh`.
  * CI uses `go-version: '1.21'`, while the box runs used Go 1.27.2.
* **Timing.** `m10_slow_body_clients_bounded` is timing-based: normal requests under 3 s, slow-header cut within 16 s, slow-body
  within 40 s, against server timeouts of 10 s and 30 s. It can flake on a paused VM. The normal-request bound was raised from 2 s
  to 3 s for CI headroom. Measured worst case: 0.01–0.04 s. Mutation M4b still fails it.

| Test | What it asserts | At pin |
|---|---|---|
| `m10_port_in_use_stderr_shape` | Busy listener on 0.0.0.0, 127.0.0.1 or `[::]` (v6only) → nonzero exit within 10 s. Every stderr line starts with `ticketdesk: ` and ends with `\n`, and names the port. None of `"phase"`, `Traceback`, `panic:`, `goroutine `, `RUNTIME_ERROR`, `IO_ERROR`, `{` appears. A server-written data file is byte-identical afterwards. | PASS |
| `m10_port_lost_race_stderr_shape` | The same stderr rules when the port is taken after the probe. The child's stdout is a full 4 KiB pipe, so it blocks on `Listening on N` (`vm.go:3039`), which is printed after the probe and right before the bind. The test takes the port, then drains the pipe. | **strict XFAIL** (reproduces m12; W-m12) |
| `m10_default_flag_operation` | No `HF_RUN_FLAGS` and no `TICKETDESK_PORT` (8080 when free). Covers: health with an inert fault header; member and viewer tokens; a 120/4000-code-point `é` ticket; 150 more; a 151-item cursor walk; PATCH; status filter; 1048576 bytes → 400 `invalid_json`; 1048577 → 413; admin DELETE; no `access ` lines; no secret on stderr; state survives a restart. | PASS |
| `m10_double_slash_routing_and_authz` | Canonical controls first (viewer DELETE 403, foreign PATCH 403, member DELETE 403, no token 401, member token list 403). Then 19 doubled-slash paths (for example `//v1/tickets`, `/v1/tickets//t1`, `/v1/tickets/t1//`, `/v1/tokens//<id>`, `//v1/health`) × GET/POST/PATCH/DELETE × 5 callers, sent byte-exact over raw sockets. Every one must be `404 not_found` with an `X-Request-Id` and no `WWW-Authenticate`. Tickets and tokens must be identical afterwards, and the viewer token must still be valid. | PASS |
| `m10_slow_body_clients_bounded` | Six authenticated admin slow-body POSTs (1 byte/s, never complete), two slow-header clients and one slow but complete upload (about 3.5 s). For 14 s, health, an authenticated list and creates must each answer in under 3 s. Slow-header connections must close within 16 s and slow-body ones within 40 s. The slow complete upload must get 201, no cut-off upload may be executed, and the server must stay healthy. | PASS (`worst normal request 0.01 s; slow-header closed after [10.0, 10.0] s; slow-body closed after [30.0 ×6] s`) |

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
| (race) | none (the pin itself) | n/a | `m10_port_lost_race_stderr_shape` | `XFAIL … expected failure, m12 … '{"phase":"runtime","code":"IO_ERROR","function":"main","instruction":30,"opcode":"HTTP_SERVER_SERVE","message":"http listen failed: listen tcp :45881: bind: address already in use"}'`; the run exits 0 |
| M12fix | `internal/vm/vm.go:3051` | replace the `IO_ERROR` panic with `fmt.Fprintf(vm.ErrOut, "ticketdesk: port %s is already in use\n", port); os.Exit(1)` (simulates an m12 fix) | `m10_port_lost_race_stderr_shape` (strictness) | `XPASS m10_port_lost_race_stderr_shape - FAIL: expected failure did not happen; m12 … looks fixed: remove its XFAIL entry`, then `1 tests, 1 failed`, exit 1 |
| (m9) | `internal/vm/crypto.go:51` | drop `name != "network" &&` | `go test ./...` (shows the gap, not an m10 test) | 76 ok, exit 0: no test catches it |

## 6. Confirmed blockers

**No confirmed product blocker for gates A, C, D, G and H at `7e9ef32e`.** B6 is out of scope and unassessed here.

No product defect blocks these gates on impact. The gates are still **conditional**: A, C, D and G need the five Owner waivers in
section 4.1 before they count as passed. Minimal repro of the finding **m12** (waiver-pending W-m12, R1.3):

```
# server stdout = a pre-filled 4 KiB pipe (fcntl F_SETPIPE_SZ 4096); start the documented server on free port P
# after 1.5 s (probe done, child blocked writing "Listening on P"): bind+listen 0.0.0.0:P; drain the pipe
# result: rc=1, stderr = {"phase":"runtime","code":"IO_ERROR",...,"opcode":"HTTP_SERVER_SERVE","message":"http listen failed: listen tcp :P: bind: address already in use"}
M10_ONLY=m10_port_lost_race_stderr_shape python3 milestone2/ticketdesk/tests/regression/assurance_m10.py <launcher>   # prints XFAIL with this signature
```

Fix direction (not done here; product code is out of scope): bind once and serve on that listener, or map a listen failure to a
prefixed line. Then remove the test's `XFAIL` entry: until it is removed, a manual run of `assurance_m10.py` reports `XPASS`,
counts it as a failure and exits 1 (CI would do the same once the step is added). m12 is waived for M2 (W-m12, section 0).

## 7. Not verified here

* B6 and anything that runs `b6_kill_storm.py`: `TestRound7StormScriptSigstopSelfTest`, `TestRound7StormScriptNoiseSelfTest` and
  `TestRound7ExpectHeadPinRules`. The #194 items were checked by reading code only.
* Performance and power-loss durability beyond the probes above (PLAN section 6).
* Other operating systems. Every run was on the shared Linux box.
