# ACCEPTANCE: ticketdesk (Milestone 2)

> **FROZEN, dated 2026-10-13.** Companion of `REQUIREMENTS.md`; changes require a separately recorded revision.

## How acceptance is decided

1. Build the toolchain and the server from a clean checkout (`go build -o <scratch>/howlframe .`; `howlframe build milestone2/ticketdesk/src/ticketdesk.howl -o milestone2/ticketdesk/build/ticketdesk.hfbc`) and capture the commands and output (gate G; `milestone2/scripts/clean_repro.sh` is the one-script form).
2. Provide a launcher satisfying REQUIREMENTS.md R1.1 (reference: `tests/acceptance/launcher.example.sh`; set `HF_BIN` and `APP_ARTIFACT`).
3. Run the independent suite:

```
python3 milestone2/ticketdesk/tests/acceptance/run_acceptance.py /path/to/launcher -v
python3 milestone2/ticketdesk/tests/acceptance/run_acceptance.py --list              # ids and requirements
python3 milestone2/ticketdesk/tests/acceptance/run_acceptance.py LAUNCHER --only T-R14
```

   The suite starts the server per test or test group on free loopback ports with temporary data directories, waits for `GET /v1/health`, stops it with SIGTERM (SIGKILL where a test says so) and removes everything. Python 3 standard library only. Exit 0 means every test and every requirement PASSed.
4. DX gate: `HF_BIN=<new howlframe> python3 milestone2/ticketdesk/dx/check_dx.py -v` must exit 0; `dx/BASELINE.md` is the recorded "before" (the checker is expected to fail on the unmodified compiler). The docs rule `dx/docs_rule.md` must hold.
5. The application is accepted only if (a) the suite exits 0, (b) the DX checker exits 0 and the docs rule test passes, and (c) the independent reviewer confirms from the preserved history that no requirement or test was edited, weakened or removed after this freeze. A self-reported pass is insufficient.

## Reviewer-verified items (not automatable black-box)

| Id | Item | Evidence |
|---|---|---|
| V1 | Token comparison uses the constant-time primitive (`secure_equal`) on digests; no early-exit `=` on secrets (R9) | source inspection |
| V2 | Secrets come from a CSPRNG (`rand_hex`), not from time or counters (R3.3, R11.1) | source inspection |
| V3 | The artifact is a `.hfbc` built from `.howl`; no generated Go/Python/JS server and no helper program at run time (R2.3) | build log, `ldd`-free file listing of the run command |
| V4 | The runner flags `--max-request-bytes`, `--access-log` are documented in `docs/cli.md` | docs diff |

## Acceptance criteria by requirement

| Requirement | Acceptance criterion | Acceptance tests |
|---|---|---|
| R1 | Launcher/env contract; each startup error class exits with its documented code and a `ticketdesk: ` stderr line, no raw runtime error; SIGTERM exit within 5 s; port conflicts do not hang. | `T-R1-ready-and-sigterm`, `T-R1-missing-admin-token`, `T-R1-bad-admin-token`, `T-R1-admin-token-length-bounds`, `T-R1-missing-data`, `T-R1-bad-port`, `T-R1-corrupt-data`, `T-R1-data-is-directory`, `T-R1-missing-data-dir`, `T-R1-empty-data-file-ok`, `T-R1-error-precedence`, `T-R1-port-in-use` |
| R2 | Works under exactly the five documented grants; each missing grant (network, database, environment, filesystem:read, filesystem:write) yields exit 77, `capability denied`, nothing listening. | `T-R2-exact-grants-workflow`, `T-R2-missing-network`, `T-R2-missing-database`, `T-R2-missing-environment`, `T-R2-missing-filesystem-read`, `T-R2-missing-filesystem-write`, `T-R2-missing-grant-not-listening`, `T-R13-state-in-granted-directory` |
| R3 | Route table, 404 for unknown routes (all methods), 405 + exact `Allow` (incl. HEAD/OPTIONS), common headers, unique server-generated X-Request-Id, Location on 201, order of checks, media-type negotiation, ignored inputs. | `T-R3-health-public`, `T-R3-unknown-routes`, `T-R3-method-not-allowed`, `T-R3-head-not-allowed`, `T-R3-common-headers`, `T-R3-request-id-unique`, `T-R3-request-id-not-echoed`, `T-R3-location-headers`, `T-R3-check-order`, `T-R3-media-type`, `T-R3-ignored-inputs`, `T-R5-crud-workflow`, `T-R10-role-before-existence`, `T-R11-create-token-shape` |
| R4 | `/v1/health` public and exact; `/v1/me` identity for bootstrap and issued tokens. | `T-R1-ready-and-sigterm`, `T-R3-health-public`, `T-R4-me` |
| R5 | Create/read/patch/delete with exact object shape, defaults, partial update semantics, id uniqueness and non-reuse, unicode round-trip, odd ids are 404. | `T-R2-exact-grants-workflow`, `T-R3-location-headers`, `T-R5-crud-workflow`, `T-R5-create-defaults`, `T-R5-patch-partial`, `T-R5-ids`, `T-R5-unicode-roundtrip`, `T-R5-unknown-id-variants`, `T-R6-order-stable-under-edit`, `T-R7-patch-empty-and-atomic`, `T-R11-tickets-outlive-token`, `T-R13-id-no-reuse-after-restart`, `T-R17-concurrency-smoke` |
| R6 | Pagination (limit default/bounds, opaque cursor, exact-end null cursor, deletion during walk, stable order), status filter before paging, every invalid query value, empty values, all roles see all tickets. | `T-R6-pagination-walk`, `T-R6-default-limit`, `T-R6-status-filter`, `T-R6-invalid-query`, `T-R6-empty-query-values`, `T-R6-delete-during-walk`, `T-R6-order-stable-under-edit`, `T-R6-visible-to-all-roles` |
| R7 | Every validation class: malformed JSON, non-object, wrong types, unknown fields, ranges and boundaries (code points), enums, multiple violations, empty PATCH, atomic failed PATCH, token fields. | `T-R3-media-type`, `T-R5-unicode-roundtrip`, `T-R6-invalid-query`, `T-R7-malformed-json`, `T-R7-non-object-json`, `T-R7-title-rules`, `T-R7-length-code-points`, `T-R7-body-rules`, `T-R7-priority-rules`, `T-R7-status-rules-patch`, `T-R7-unknown-fields`, `T-R7-multiple-violations`, `T-R7-patch-empty-and-atomic`, `T-R7-token-fields`, `T-R10-privilege-escalation`, `T-R17-max-size-roundtrip` |
| R8 | Single error shape on every status; exhaustive code to status table; fixed 401 message; no echo of secrets or internals. | `T-R3-unknown-routes`, `T-R3-method-not-allowed`, `T-R3-common-headers`, `T-R3-check-order`, `T-R3-media-type`, `T-R5-unknown-id-variants`, `T-R6-invalid-query`, `T-R7-malformed-json`, `T-R7-non-object-json`, `T-R7-unknown-fields`, `T-R7-multiple-violations`, `T-R8-code-table`, `T-R8-401-message-identical`, `T-R8-no-secret-in-errors`, `T-R9-garbled-credentials`, `T-R11-revoke-unknown-and-bootstrap`, `T-R14-ceiling-before-json`, `T-R15-fault-shape`, `T-R17-error-storm-survival` |
| R9 | No/garbled/unknown/revoked/case-changed credentials all 401 with `WWW-Authenticate`; Bearer case-insensitive; repeated header; unauthenticated writes do nothing. | `T-R4-me`, `T-R8-401-message-identical`, `T-R9-garbled-credentials`, `T-R9-scheme-case`, `T-R9-repeated-header`, `T-R9-unauth-writes-do-nothing`, `T-R11-revoke-effect`, `T-R13-bootstrap-rotation`, `T-R15-fault-disabled-by-default` |
| R10 | Full role matrix, member ownership, admin edits keep owner, role-before-existence ordering, denied calls do not mutate, no privilege escalation. | `T-R3-check-order`, `T-R6-visible-to-all-roles`, `T-R10-matrix`, `T-R10-ownership`, `T-R10-role-before-existence`, `T-R10-forbidden-no-mutation`, `T-R10-privilege-escalation`, `T-R11-admin-token-powers` |
| R11 | Token creation shape/formats/`no-store`, uniqueness, secret exactly once, list, revoke effect and idempotence, unknown 404, bootstrap 409, issued admin powers, tickets outlive tokens. | `T-R3-location-headers`, `T-R7-token-fields`, `T-R11-create-token-shape`, `T-R11-secret-only-once`, `T-R11-list-tokens`, `T-R11-revoke-effect`, `T-R11-revoke-unknown-and-bootstrap`, `T-R11-admin-token-powers`, `T-R11-tickets-outlive-token` |
| R12 | Raw secrets and Authorization values absent from the state, stdout and stderr; lowercase SHA-256 hex of each issued secret present in the state; no hashes in responses; survives restart and SIGKILL. | `T-R8-no-secret-in-errors`, `T-R11-secret-only-once`, `T-R12-secrets-not-in-state`, `T-R12-secrets-not-on-stderr`, `T-R16-access-log-secrets` |
| R13 | Restart (SIGTERM and SIGKILL) preserves tickets, order, tokens, revocations; no id reuse; bootstrap rotation; files only in the data directory; concurrent updates not lost. | `T-R1-corrupt-data`, `T-R1-empty-data-file-ok`, `T-R12-secrets-not-in-state`, `T-R13-restart-sigterm`, `T-R13-sigkill-durability`, `T-R13-id-no-reuse-after-restart`, `T-R13-bootstrap-rotation`, `T-R13-state-in-granted-directory`, `T-R13-concurrent-no-lost-updates`, `T-R15-fault-no-side-effects-and-recovery` |
| R14 | Default 1 MiB boundary, `--max-request-bytes N` exact boundary on POST/PATCH/token create, 413 before JSON parsing, bytes not characters, invalid flag values exit 2. | `T-R14-default-ceiling`, `T-R14-custom-ceiling`, `T-R14-ceiling-before-json`, `T-R14-invalid-flag-values` |
| R15 | Forced fault yields 500 `internal_error` with fixed message and request id, no internals in body/headers; fault honored on every known path/method; header inert unless env is exactly `1`; no side effects; server recovers. | `T-R15-fault-shape`, `T-R15-fault-stderr-line`, `T-R15-fault-every-route`, `T-R15-fault-no-side-effects-and-recovery`, `T-R15-fault-header-value`, `T-R15-fault-disabled-by-default`, `T-R16-fault-line-without-flag`, `T-R16-access-line-for-fault` |
| R16 | `--access-log` exact line format for all status classes incl. 500, query stripped, one line per request; none without the flag; fault line independent of the flag; no secrets. | `T-R12-secrets-not-on-stderr`, `T-R15-fault-stderr-line`, `T-R16-access-log-lines`, `T-R16-access-log-secrets`, `T-R16-no-access-without-flag`, `T-R16-fault-line-without-flag`, `T-R16-access-line-for-fault` |
| R17 | 20 sequential + 20 parallel requests, unique ids, complete list; largest valid payload; 50 hostile requests; server stays healthy. | `T-R13-concurrent-no-lost-updates`, `T-R17-concurrency-smoke`, `T-R17-max-size-roundtrip`, `T-R17-error-storm-survival` |

## Criteria grouped by selected gate (PLAN.md section 2)


### Gate A: Practical HTTP API

Requirements: R3, R4, R5, R6, R13, R17. Tests: `T-R1-corrupt-data`, `T-R1-empty-data-file-ok`, `T-R1-ready-and-sigterm`, `T-R10-role-before-existence`, `T-R11-create-token-shape`, `T-R11-tickets-outlive-token`, `T-R12-secrets-not-in-state`, `T-R13-bootstrap-rotation`, `T-R13-concurrent-no-lost-updates`, `T-R13-id-no-reuse-after-restart`, `T-R13-restart-sigterm`, `T-R13-sigkill-durability`, `T-R13-state-in-granted-directory`, `T-R15-fault-no-side-effects-and-recovery`, `T-R17-concurrency-smoke`, `T-R17-error-storm-survival`, `T-R17-max-size-roundtrip`, `T-R2-exact-grants-workflow`, `T-R3-check-order`, `T-R3-common-headers`, `T-R3-head-not-allowed`, `T-R3-health-public`, `T-R3-ignored-inputs`, `T-R3-location-headers`, `T-R3-media-type`, `T-R3-method-not-allowed`, `T-R3-request-id-not-echoed`, `T-R3-request-id-unique`, `T-R3-unknown-routes`, `T-R4-me`, `T-R5-create-defaults`, `T-R5-crud-workflow`, `T-R5-ids`, `T-R5-patch-partial`, `T-R5-unicode-roundtrip`, `T-R5-unknown-id-variants`, `T-R6-default-limit`, `T-R6-delete-during-walk`, `T-R6-empty-query-values`, `T-R6-invalid-query`, `T-R6-order-stable-under-edit`, `T-R6-pagination-walk`, `T-R6-status-filter`, `T-R6-visible-to-all-roles`, `T-R7-patch-empty-and-atomic`.

### Gate C: Authentication and authorization

Requirements: R9, R10, R11, R12. Tests: `T-R10-forbidden-no-mutation`, `T-R10-matrix`, `T-R10-ownership`, `T-R10-privilege-escalation`, `T-R10-role-before-existence`, `T-R11-admin-token-powers`, `T-R11-create-token-shape`, `T-R11-list-tokens`, `T-R11-revoke-effect`, `T-R11-revoke-unknown-and-bootstrap`, `T-R11-secret-only-once`, `T-R11-tickets-outlive-token`, `T-R12-secrets-not-in-state`, `T-R12-secrets-not-on-stderr`, `T-R13-bootstrap-rotation`, `T-R15-fault-disabled-by-default`, `T-R16-access-log-secrets`, `T-R3-check-order`, `T-R3-location-headers`, `T-R4-me`, `T-R6-visible-to-all-roles`, `T-R7-token-fields`, `T-R8-401-message-identical`, `T-R8-no-secret-in-errors`, `T-R9-garbled-credentials`, `T-R9-repeated-header`, `T-R9-scheme-case`, `T-R9-unauth-writes-do-nothing`.

### Gate D: Validation and reliable error behavior

Requirements: R7, R8, R14, R15, R16. Tests: `T-R10-privilege-escalation`, `T-R11-revoke-unknown-and-bootstrap`, `T-R12-secrets-not-on-stderr`, `T-R14-ceiling-before-json`, `T-R14-custom-ceiling`, `T-R14-default-ceiling`, `T-R14-invalid-flag-values`, `T-R15-fault-disabled-by-default`, `T-R15-fault-every-route`, `T-R15-fault-header-value`, `T-R15-fault-no-side-effects-and-recovery`, `T-R15-fault-shape`, `T-R15-fault-stderr-line`, `T-R16-access-line-for-fault`, `T-R16-access-log-lines`, `T-R16-access-log-secrets`, `T-R16-fault-line-without-flag`, `T-R16-no-access-without-flag`, `T-R17-error-storm-survival`, `T-R17-max-size-roundtrip`, `T-R3-check-order`, `T-R3-common-headers`, `T-R3-media-type`, `T-R3-method-not-allowed`, `T-R3-unknown-routes`, `T-R5-unicode-roundtrip`, `T-R5-unknown-id-variants`, `T-R6-invalid-query`, `T-R7-body-rules`, `T-R7-length-code-points`, `T-R7-malformed-json`, `T-R7-multiple-violations`, `T-R7-non-object-json`, `T-R7-patch-empty-and-atomic`, `T-R7-priority-rules`, `T-R7-status-rules-patch`, `T-R7-title-rules`, `T-R7-token-fields`, `T-R7-unknown-fields`, `T-R8-401-message-identical`, `T-R8-code-table`, `T-R8-no-secret-in-errors`, `T-R9-garbled-credentials`.

### Gate G: Standalone installation/execution

* **Clean reproduction** (`milestone2/scripts/clean_repro.sh`): builds toolchain and artifact, starts the server with the minimum grants of R2.1 and runs the full suite; the transcript is the gate G evidence. Requires only Go and Python 3.

Requirements: R1, R2. Tests: `T-R1-admin-token-length-bounds`, `T-R1-bad-admin-token`, `T-R1-bad-port`, `T-R1-corrupt-data`, `T-R1-data-is-directory`, `T-R1-empty-data-file-ok`, `T-R1-error-precedence`, `T-R1-missing-admin-token`, `T-R1-missing-data`, `T-R1-missing-data-dir`, `T-R1-port-in-use`, `T-R1-ready-and-sigterm`, `T-R13-state-in-granted-directory`, `T-R2-exact-grants-workflow`, `T-R2-missing-database`, `T-R2-missing-environment`, `T-R2-missing-filesystem-read`, `T-R2-missing-filesystem-write`, `T-R2-missing-grant-not-listening`, `T-R2-missing-network`.

### Gate H: Developer experience

* **Items 1-4 (diagnostics):** `dx/check_dx.py` passes all 13 checks (`ok_program` controls plus every improved diagnostic), evidence `dx/BASELINE.md` (before) and the checker output (after).
* **Item 5 (server observability):** requirement R16 and the tests below.
* **Item 6 (documentation):** `dx/docs_rule.md` rules 1-7.

Requirements: R16. Tests: `T-R12-secrets-not-on-stderr`, `T-R15-fault-stderr-line`, `T-R16-access-line-for-fault`, `T-R16-access-log-lines`, `T-R16-access-log-secrets`, `T-R16-fault-line-without-flag`, `T-R16-no-access-without-flag`.

## Test inventory

| Test id | Covers | Description |
|---|---|---|
| `T-R1-ready-and-sigterm` | R1,R4 | starts, answers /v1/health, exits within 5s of SIGTERM |
| `T-R1-missing-admin-token` | R1 | no TICKETDESK_ADMIN_TOKEN: exit 2 |
| `T-R1-bad-admin-token` | R1 | empty / <16 chars / space / non-ASCII admin token: exit 2 |
| `T-R1-admin-token-length-bounds` | R1 | 16-char and 256-char admin tokens are accepted |
| `T-R1-missing-data` | R1 | no TICKETDESK_DATA: exit 2 |
| `T-R1-bad-port` | R1 | TICKETDESK_PORT abc / 0 / 70000 / -1: exit 2 |
| `T-R1-corrupt-data` | R1,R13 | garbage in the data file: exit 65, file untouched |
| `T-R1-data-is-directory` | R1 | TICKETDESK_DATA is a directory: exit 65 |
| `T-R1-missing-data-dir` | R1 | directory of TICKETDESK_DATA missing: exit 66 |
| `T-R1-empty-data-file-ok` | R1,R13 | zero-byte data file is valid empty state |
| `T-R1-error-precedence` | R1 | env problems (2) win over data (66) problems |
| `T-R1-port-in-use` | R1 | busy port: nonzero exit within 15s, no hang |
| `T-R2-exact-grants-workflow` | R2,R5 | full workflow under exactly the documented five grants |
| `T-R2-missing-network` | R2 | grant network missing: clean startup failure, exit 77 |
| `T-R2-missing-database` | R2 | grant database missing: clean startup failure, exit 77 |
| `T-R2-missing-environment` | R2 | grant environment missing: clean startup failure, exit 77 |
| `T-R2-missing-filesystem-read` | R2 | grant filesystem:read missing: clean startup failure, exit 77 |
| `T-R2-missing-filesystem-write` | R2 | grant filesystem:write missing: clean startup failure, exit 77 |
| `T-R2-missing-grant-not-listening` | R2 | with a missing grant nothing ever listens |
| `T-R3-health-public` | R3,R4 | GET /v1/health needs no token, ignores garbage Authorization |
| `T-R3-unknown-routes` | R3,R8 | unknown paths: 404 not_found for every method, with or without auth |
| `T-R3-method-not-allowed` | R3,R8 | known path, wrong method: 405 + exact Allow, before authentication |
| `T-R3-head-not-allowed` | R3 | HEAD on every route: 405 with Allow |
| `T-R3-common-headers` | R3,R8 | Content-Type + X-Request-Id on every response class |
| `T-R3-request-id-unique` | R3 | X-Request-Id differs for each of 60 requests; error request_id matches it |
| `T-R3-request-id-not-echoed` | R3 | client-supplied X-Request-Id is ignored |
| `T-R3-location-headers` | R3,R5,R11 | 201 responses carry Location |
| `T-R3-check-order` | R3,R8,R10 | order: route, method, auth, role, existence, ownership, media type, size, JSON, fields |
| `T-R3-media-type` | R3,R7,R8 | Content-Type negotiation: accepted and rejected media types |
| `T-R3-ignored-inputs` | R3 | unknown query params and GET/DELETE bodies are ignored |
| `T-R4-me` | R4,R9 | /v1/me for bootstrap, member, viewer, admin tokens |
| `T-R5-crud-workflow` | R5,R3 | create, read, list, patch, delete, read-404 |
| `T-R5-create-defaults` | R5 | defaults: body '', priority normal, status open; exact key set |
| `T-R5-patch-partial` | R5 | PATCH changes only the given fields, keeps id/owner/created_at, updated_at non-decreasing |
| `T-R5-ids` | R5 | ids unique, well-formed and never reused after delete |
| `T-R5-unicode-roundtrip` | R5,R7 | unicode, quotes, newlines and escapes round-trip exactly |
| `T-R5-unknown-id-variants` | R5,R8 | odd ids are plain 404s |
| `T-R6-pagination-walk` | R6 | limit/cursor walk returns everything once, in creation order; exact end has null cursor |
| `T-R6-default-limit` | R6 | default limit 20, max 100 accepted |
| `T-R6-status-filter` | R6 | status filter applies before paging |
| `T-R6-invalid-query` | R6,R7,R8 | invalid limit/status/cursor: 400 validation_failed invalid_value |
| `T-R6-empty-query-values` | R6 | empty ?limit= ?status= ?cursor= behave as absent |
| `T-R6-delete-during-walk` | R6 | deleting tickets between pages does not break the walk |
| `T-R6-order-stable-under-edit` | R6,R5 | editing an old ticket does not move it in the list |
| `T-R6-visible-to-all-roles` | R6,R10 | every role sees all tickets |
| `T-R7-malformed-json` | R7,R8 | syntactically broken bodies: 400 invalid_json (POST and PATCH) |
| `T-R7-non-object-json` | R7,R8 | valid JSON that is not an object: validation_failed on $ |
| `T-R7-title-rules` | R7 | title required, string, 1..120 |
| `T-R7-length-code-points` | R7 | lengths count code points, not bytes |
| `T-R7-body-rules` | R7 | body optional string 0..4000 |
| `T-R7-priority-rules` | R7 | priority enum low|normal|high, strict type |
| `T-R7-status-rules-patch` | R7 | status enum on PATCH, strict type |
| `T-R7-unknown-fields` | R7,R8 | unknown fields are rejected with unknown_field per key |
| `T-R7-multiple-violations` | R7,R8 | all violated fields reported together |
| `T-R7-patch-empty-and-atomic` | R7,R5 | {} is rejected; invalid patch changes nothing |
| `T-R7-token-fields` | R7,R11 | token name/role validation |
| `T-R8-code-table` | R8 | each documented error code appears with exactly its documented status |
| `T-R8-401-message-identical` | R8,R9 | every 401 has the same fixed message |
| `T-R8-no-secret-in-errors` | R8,R12 | error bodies never echo tokens or Authorization values |
| `T-R9-garbled-credentials` | R9,R8 | no/garbled/unknown credentials: 401 + WWW-Authenticate |
| `T-R9-scheme-case` | R9 | Bearer scheme is case-insensitive |
| `T-R9-repeated-header` | R9 | with repeated Authorization headers the first value counts |
| `T-R9-unauth-writes-do-nothing` | R9 | unauthenticated mutations do not mutate |
| `T-R10-matrix` | R10 | role x operation matrix (R10 table) |
| `T-R10-ownership` | R10 | member edits only own tickets; admin edits any without changing owner |
| `T-R10-role-before-existence` | R10,R3 | 403/404 ordering for missing tickets |
| `T-R10-forbidden-no-mutation` | R10 | denied operations change nothing |
| `T-R10-privilege-escalation` | R10,R7 | a member cannot mint a stronger token or change roles |
| `T-R11-create-token-shape` | R11,R3 | 201 shape, formats, Cache-Control: no-store, uniqueness |
| `T-R11-secret-only-once` | R11,R12 | the secret appears in no later response |
| `T-R11-list-tokens` | R11 | list: creation order, revoked included, no bootstrap, no secrets |
| `T-R11-revoke-effect` | R11,R9 | revoked token gets 401 on the very next request; revoke is idempotent |
| `T-R11-revoke-unknown-and-bootstrap` | R11,R8 | unknown id 404; bootstrap 409 |
| `T-R11-admin-token-powers` | R11,R10 | an issued admin token manages tokens, may revoke itself |
| `T-R11-tickets-outlive-token` | R11,R5 | tickets stay with their owner id after revocation |
| `T-R12-secrets-not-in-state` | R12,R13 | no token secret or Authorization value in any state file; SHA-256 hex is there |
| `T-R12-secrets-not-on-stderr` | R12,R16 | no secret or Authorization value on stderr/stdout after a mixed workload |
| `T-R13-restart-sigterm` | R13 | tickets, order and tokens identical after SIGTERM restart |
| `T-R13-sigkill-durability` | R13 | confirmed mutations survive SIGKILL immediately after the response |
| `T-R13-id-no-reuse-after-restart` | R13,R5 | ids of deleted tickets are not reused after restart |
| `T-R13-bootstrap-rotation` | R13,R9 | restart with a new bootstrap token: new accepted, old rejected, issued tokens kept |
| `T-R13-state-in-granted-directory` | R13,R2 | state lives at TICKETDESK_DATA's directory only |
| `T-R13-concurrent-no-lost-updates` | R13,R17 | parallel PATCHes and creates lose nothing |
| `T-R14-default-ceiling` | R14 | default 1048576: N bytes processed, N+1 rejected with 413 |
| `T-R14-custom-ceiling` | R14 | --max-request-bytes 2048: exact boundary on POST/PATCH/tokens |
| `T-R14-ceiling-before-json` | R14,R8 | oversize non-JSON is 413, not invalid_json; multibyte counted in bytes |
| `T-R14-invalid-flag-values` | R14 | --max-request-bytes 0 / -5 / abc: runner exits 2 with a diagnostic |
| `T-R15-fault-shape` | R15,R8 | forced fault: safe 500, request id, no internals in body or headers |
| `T-R15-fault-stderr-line` | R15,R16 | stderr has 'fault request_id=... method=... path=... error=...' with the same id |
| `T-R15-fault-every-route` | R15 | fault honored on every known path/method, with and without auth |
| `T-R15-fault-no-side-effects-and-recovery` | R15,R13 | after faults the server serves normally and state is unchanged |
| `T-R15-fault-header-value` | R15 | only the value 1 triggers; other values and unknown routes are unaffected |
| `T-R15-fault-disabled-by-default` | R15,R9 | without TICKETDESK_ALLOW_DEBUG_FAULT=1 the header is inert |
| `T-R16-access-log-lines` | R16 | --access-log: one exact line per request incl. 401/404/405/413, path without query |
| `T-R16-access-log-secrets` | R16,R12 | access lines never contain tokens or Authorization |
| `T-R16-no-access-without-flag` | R16 | without --access-log no 'access ' lines |
| `T-R16-fault-line-without-flag` | R16,R15 | fault line is written even without --access-log |
| `T-R16-access-line-for-fault` | R16,R15 | a faulting request also gets an access line with status=500 |
| `T-R17-concurrency-smoke` | R17,R5 | 20 sequential then 20 parallel creates/reads: all correct, unique ids, healthy |
| `T-R17-max-size-roundtrip` | R17,R7 | largest valid ticket (120-char title, 4000-char body) round-trips |
| `T-R17-error-storm-survival` | R17,R8 | 50 hostile requests in a row; server stays healthy and consistent |

## Known limits of the black-box suite (stated honestly)

* Constant-time comparison and CSPRNG quality are not observable from outside (V1, V2).
* The suite cannot see the HFIR/bytecode structure; gate G also needs the clean-reproduction transcript.
* Timing assertions are coarse (exit within 5 s of SIGTERM, 10-15 s startup failure) to avoid flakiness.
* `T-R2-missing-*` assumes the implementation verifies its five grants before listening (R2.2).

