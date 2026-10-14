# REQUIREMENTS: ticketdesk (Milestone 2 demonstration backend)

> **STATUS: FROZEN, dated 2026-10-13.** Authority: `milestone2/PLAN.md` section 1 and `HOWLFRAME_PRODUCT_DIRECTIVE.md` section 6 (Milestone 2, D29 A, C, D, G, H). Author: independent requirements/test author (does not implement). Changes require a separately recorded revision; requirements and tests must not be weakened to make an implementation pass.
> Normative words: MUST, MUST NOT, SHOULD (SHOULD items are not acceptance-tested).

## 0. Problem statement

`ticketdesk` is a small issue-tracker HTTP API (JSON over HTTP) written in HowlFrame and run on the standalone VM (`howlframe run`). It stores tickets and API tokens durably, authenticates callers by bearer token, enforces a role and ownership model, validates every input, answers every failure with one error shape and stable codes, and never leaks internals or secrets. It is unrelated to the Howl ecosystem.

## 1. Conventions

* All routes are under the base path `/v1`; there are no unversioned routes.
* All request and response bodies are UTF-8 JSON (`application/json`). "Integer" means a JSON number without fraction or exponent. Timestamps are integer Unix seconds.
* "Code point count" is used for every string length: `"é"` has length 1.
* Header names are case-insensitive. Examples show canonical case.
* The acceptance suite is `tests/acceptance/run_acceptance.py` (see `ACCEPTANCE.md`).

## R1. Launch, configuration, startup errors, shutdown

**R1.1 Launcher contract.** The suite starts the server through a launcher executable:

```
LAUNCHER GRANTS [APP_ARG...]
```

* `GRANTS` is exactly ONE argument: the comma-separated `--allow-caps` list (never empty for ticketdesk; the empty string would mean "no grants" and the launcher must then pass no `--allow-caps`). `APP_ARG...` is empty for ticketdesk.
* `HF_RUN_FLAGS` (whitespace-separated runner flags) is inserted by the launcher before the artifact path. This is the only channel for runner flags: resource ceilings (`--max-instructions N`, `--max-memory-bytes N`, `--max-call-depth N`), `--max-request-bytes N` (R14) and `--access-log` (R16).
* The launcher passes its environment through unchanged and `exec`s the runner, so SIGTERM sent to the launcher process reaches the server.
* Reference launcher (`tests/acceptance/launcher.example.sh`): `howlframe run [--allow-caps "$GRANTS"] $HF_RUN_FLAGS "$APP_ARTIFACT"`.
* The server is built with `howlframe build milestone2/ticketdesk/src/ticketdesk.howl -o milestone2/ticketdesk/build/ticketdesk.hfbc`.

**R1.2 Environment variables.**

| Variable | Required | Meaning |
|---|---|---|
| `TICKETDESK_ADMIN_TOKEN` | yes | Bootstrap admin token secret. 16 to 256 characters, each a printable ASCII character other than space (`0x21`-`0x7e`). |
| `TICKETDESK_DATA` | yes | Path of the state file (absolute). Created if absent. The directory containing it MUST exist. The server MAY create sibling files in that same directory only, and MUST NOT touch anything else on the filesystem. |
| `TICKETDESK_PORT` | no | TCP port to listen on, decimal integer 1 to 65535. Default `8080`. The server listens on all interfaces. |
| `TICKETDESK_ALLOW_DEBUG_FAULT` | no | Exactly `1` enables the fault-injection header (R15). Any other value or absence disables it. |

**R1.3 Startup errors (stderr lines start with `ticketdesk: `, each terminated by `\n`; nothing is listening afterwards; the process exits within 10 s; no raw runtime error object, `"phase":`, `Traceback`, `panic:` or `goroutine ` text appears on stderr).**

| Situation | Exit code |
|---|---|
| `TICKETDESK_ADMIN_TOKEN` missing, empty, shorter than 16 or containing a disallowed character | 2 |
| `TICKETDESK_DATA` missing or empty | 2 |
| `TICKETDESK_PORT` set but not an integer in 1..65535 | 2 |
| `TICKETDESK_DATA` names an existing path that is not valid ticketdesk state (corrupt, a directory, not readable as state). A zero-byte regular file is valid empty state | 65 |
| The directory of `TICKETDESK_DATA` does not exist | 66 |
| A required grant is missing (R2) | 77, stderr contains `capability denied` (case-insensitive) |
| The port is already in use | any nonzero exit (not 0), within 10 s |

Argument errors take precedence in this order: environment problems (2), then data problems (66, 65), then grants (77). A server that fails to start MUST NOT have modified an existing data file.

**R1.4 Readiness and shutdown.** After a successful start `GET /v1/health` answers `200` (R4). On SIGTERM the server stops accepting and the process exits within 5 s (any exit status). Every mutation is durable (R13) before its response is sent, so SIGKILL at any moment after a response loses nothing that response confirmed.

## R2. Grants and standalone execution (gate G)

**R2.1** The server MUST run with exactly these grants and no others:

```
network,database,environment,filesystem:read=<DIR>,filesystem:write=<DIR>
```

where `<DIR>` is the real path of the directory containing `TICKETDESK_DATA`. No `process`, no `network:fetch=...`, no unscoped `filesystem`.

**R2.2** The server MUST verify at startup, before listening, that it holds each of these five grants (reading the environment, opening/reading/writing the state under `<DIR>`, and binding the port). If any one is missing it fails per R1.3 (exit 77, `ticketdesk: capability denied: ...`), and never lets a request fail later with a generic 500 because a grant was missing.

**R2.3** The only runtime artifacts are the `.hfbc` file built from `.howl` source and the `howlframe` binary. No generated Go/Python/JavaScript server, no other Howl component, no network access other than serving.

## R3. HTTP conventions

**R3.1 Route table.** Exactly these routes exist; the `Allow` value is the exact string returned on 405 (methods sorted alphabetically, separated by `, `).

| Path | Methods | `Allow` |
|---|---|---|
| `/v1/health` | GET | `GET` |
| `/v1/me` | GET | `GET` |
| `/v1/tickets` | GET, POST | `GET, POST` |
| `/v1/tickets/{id}` | DELETE, GET, PATCH | `DELETE, GET, PATCH` |
| `/v1/tokens` | GET, POST | `GET, POST` |
| `/v1/tokens/{id}` | DELETE | `DELETE` |

`{id}` is exactly one non-empty path segment. A trailing slash (`/v1/tickets/`), an extra segment, or any other path (`/`, `/v1`, `/v2/tickets`, `/health`) is an unknown route.

**R3.2 Unknown route.** `404` with the error shape and code `not_found`, for every method, before any authentication. **Known path, unsupported method** (including HEAD and OPTIONS): `405`, code `method_not_allowed`, header `Allow` as in R3.1, before any authentication.

**R3.3 Common response headers.** Every response, success or error, from any route including 404 and 405, carries:

* `Content-Type: application/json` (a `; charset=utf-8` parameter is allowed) and a JSON body. There are no empty-body responses.
* `X-Request-Id`: exactly 32 lowercase hex characters, generated by the server per request, unique across all requests in the server's lifetime (use a CSPRNG). A client-supplied `X-Request-Id` request header MUST be ignored (never echoed).
* Responses that contain a token secret (R11.2) also carry `Cache-Control: no-store`.
* `201` responses carry `Location: /v1/tickets/<id>` or `/v1/tokens/<id>`.
* `401` responses carry `WWW-Authenticate: Bearer realm="ticketdesk"`.

**R3.4 Order of checks** for every route except `GET /v1/health` (a failure at an earlier step wins):

1. route (404) 2. method (405) 3. authentication (401, R9) 4. role authorization (403, R10) 5. resource existence for `{id}` routes (404) 6. ownership (403, R10) 7. for `POST`/`PATCH`: `Content-Type` (415) 8. body size ceiling (413, R14) 9. JSON syntax (400 `invalid_json`) 10. field validation (400 `validation_failed`); for `GET /v1/tickets` the query parameters are validated at this step (it has no `{id}` and no body).

Unknown query parameters are ignored everywhere. Request bodies on GET/DELETE are ignored.

**R3.5 Content type.** `POST` and `PATCH` bodies MUST declare media type `application/json` (case-insensitive, optional parameters such as `; charset=utf-8`). A missing `Content-Type` or any other media type (`text/plain`, `application/x-www-form-urlencoded`, `application/jsonx`) yields `415 unsupported_media_type`.

## R4. Health and identity

* `GET /v1/health`, no authentication, any/no headers: `200 {"status":"ok"}`.
* `GET /v1/me` (any valid token): `200 {"id":"<token id>","name":"<name>","role":"admin|member|viewer"}`. For the bootstrap token: `id` `"bootstrap"`, `name` `"bootstrap"`, `role` `"admin"`.

## R5. Tickets

**R5.1 Ticket object** (all fields always present, no others):

```json
{"id":"t1","title":"Printer on fire","body":"","priority":"normal","status":"open",
 "owner":"tok_0123456789abcdef","created_at":1760000000,"updated_at":1760000000}
```

* `id`: string matching `^[A-Za-z0-9_-]{1,64}$`, unique, never reused after deletion.
* `owner`: id of the token that created it; immutable (an admin editing a ticket does not change it). `"bootstrap"` for the bootstrap token.
* `created_at` and `updated_at` are positive integers; `updated_at >= created_at` and `updated_at` changes (non-decreasing) on every successful PATCH.

**R5.2 Create** `POST /v1/tickets` (roles admin, member). Body fields: `title` (required), `body` (optional, default `""`), `priority` (optional, default `"normal"`). Result `201` + ticket with `status` `"open"`, `owner` = caller's token id. `status` is NOT accepted on create (unknown field).

**R5.3 Read** `GET /v1/tickets/{id}` (all roles): `200` + ticket, or `404 not_found`.

**R5.4 Partial update** `PATCH /v1/tickets/{id}`: body fields `title`, `body`, `priority`, `status`, any non-empty subset; omitted fields are unchanged; the empty object `{}` is `400 validation_failed` with field `$` code `required`. Result `200` + the full updated ticket.

**R5.5 Delete** `DELETE /v1/tickets/{id}` (admin only): `200 {"id":"<id>","deleted":true}`; afterwards GET/PATCH/DELETE of that id are `404`.

**R5.6 List** `GET /v1/tickets` (all roles), see R6.

## R6. Listing, pagination, filter

* Query parameters: `limit` (integer 1..100, default 20), `cursor` (opaque), `status` (`open|in_progress|done`).
* Response `200 {"items":[<ticket>...],"next_cursor":<string|null>}`.
* Order: ascending creation order (the order of creation, stable across restarts and across edits).
* `status` filters before paging. `items` has at most `limit` elements. `next_cursor` is `null` exactly when no further matching ticket exists after the last returned one (so a page that ends exactly at the end of the data has `next_cursor: null`); otherwise it is a string matching `^[A-Za-z0-9_-]{1,64}$`.
* Passing a returned `next_cursor` (with the same `status` filter) returns the page that follows; walking all pages returns every matching ticket exactly once. Tickets deleted between page requests do not break the walk (the cursor is a position, not an offset).
* An empty query value (`?limit=`) is the same as absent. Invalid: `limit` not an integer, `0`, negative, `>100`, `1.5`, `abc`; `status` outside the enum; `cursor` containing characters outside `[A-Za-z0-9_-]` or longer than 64 characters. Each yields `400 validation_failed` with a `fields` entry for the parameter (`limit`, `status` or `cursor`) and code `invalid_value`. Syntactically valid cursors the server did not issue are unspecified.

## R7. Input validation

**R7.1 Request body framing** (POST/PATCH, after R3.5 and R14):

| Body | Result |
|---|---|
| zero bytes, whitespace only, not JSON, truncated JSON, trailing garbage after the value (`{} x`), invalid UTF-8, bare `NaN` | `400 invalid_json` |
| valid JSON that is not an object (`[]`, `"x"`, `1`, `null`, `true`) | `400 validation_failed`, fields `[{"field":"$","code":"invalid_type",...}]` |

**R7.2 Ticket fields** (create and patch). The server reports EVERY violated field in one response, one `fields` entry per field (first violation of that field), in unspecified order.

| Field | Rule | Violation -> field code |
|---|---|---|
| `title` | string, 1..120 code points | missing on create -> `required`; non-string (number, bool, null, array, object) -> `invalid_type`; `""` -> `too_short`; >120 -> `too_long` |
| `body` | string, 0..4000 code points | non-string or null -> `invalid_type`; >4000 -> `too_long` |
| `priority` | one of `low`, `normal`, `high` | non-string -> `invalid_type`; other string (including `HIGH`, `""`) -> `invalid_value` |
| `status` (patch only) | one of `open`, `in_progress`, `done` | non-string -> `invalid_type`; other string -> `invalid_value` |
| any other key (on create `status`, `id`, `owner`, `created_at`, `updated_at` are "other") | not allowed | `unknown_field`, `field` = the key |

Types are strict: no coercion (`"title": 5`, `"priority": 1`, `"body": null` are errors). Strings are accepted as given (no trimming, no case change); title `" "` is valid. Boundary lengths 1, 120, 0 (body), 4000 are valid.

**R7.3 Token fields** (`POST /v1/tokens`): `name` (required string 1..64 code points), `role` (required, one of `admin|member|viewer`); same field codes; unknown keys rejected.

## R8. Error model

**R8.1 Shape.** Every non-2xx response, from every route and every failure class, is exactly:

```json
{"error":{"code":"validation_failed","message":"Request validation failed.","request_id":"<same as X-Request-Id>",
          "fields":[{"field":"title","code":"too_long","message":"title must be at most 120 characters"}]}}
```

`code`, `message` (non-empty human-readable string) and `request_id` are always present; `fields` is present and non-empty only for `validation_failed`. No other keys anywhere in `error` or in a fields entry. `request_id` equals the `X-Request-Id` header.

**R8.2 Exhaustive code table.** These are all the error codes; a response uses exactly the pairing below.

| code | HTTP status | Situation |
|---|---|---|
| `invalid_json` | 400 | R7.1 syntax-level body failures |
| `validation_failed` | 400 | R7.1 non-object, R7.2, R7.3, R6 query validation, empty PATCH |
| `unauthorized` | 401 | R9 |
| `forbidden` | 403 | R10 |
| `not_found` | 404 | unknown route (R3.2), unknown ticket id, unknown token id |
| `method_not_allowed` | 405 | R3.2 |
| `conflict` | 409 | `DELETE /v1/tokens/bootstrap` (the bootstrap token cannot be revoked) |
| `payload_too_large` | 413 | R14 |
| `unsupported_media_type` | 415 | R3.5 |
| `internal_error` | 500 | R15 |

**R8.3 Messages.** `message` for `unauthorized` is the single fixed text `Invalid or missing credentials.` in every 401 case (no oracle for unknown vs revoked vs malformed). For `internal_error` it is the fixed text `An internal error occurred.`. Messages never contain a token, an `Authorization` value, a file path, a stack trace, runtime error text, opcode or capability names.

## R9. Authentication

* Protected routes require `Authorization: Bearer <secret>`. The scheme name is case-insensitive (`bearer`, `BEARER`), followed by exactly one space and the secret.
* `401 unauthorized` (+ `WWW-Authenticate`) for: no `Authorization` header; empty value; scheme other than Bearer (`Basic ...`, `Token ...`); `Bearer` with no secret; extra tokens (`Bearer a b`); a secret that matches no active token; a revoked token; the right secret with changed case; a secret with leading/trailing whitespace; a header repeated more than once is treated by its first value.
* The bootstrap token is the value of `TICKETDESK_ADMIN_TOKEN`, id `bootstrap`, role admin, always active. It is never written to the state (R12).
* Token secrets are compared by their SHA-256 digest with a constant-time comparison (reviewer-verified, ACCEPTANCE.md).

## R10. Authorization

| Operation | admin | member | viewer |
|---|---|---|---|
| `GET /v1/health` | public | public | public |
| `GET /v1/me` | yes | yes | yes |
| `GET /v1/tickets`, `GET /v1/tickets/{id}` | yes | yes | yes |
| `POST /v1/tickets` | yes | yes (owner = self) | **403** |
| `PATCH /v1/tickets/{id}` | any ticket | only tickets whose `owner` is the caller's token id, else **403** | **403** |
| `DELETE /v1/tickets/{id}` | yes | **403** (even for own tickets) | **403** |
| `GET/POST /v1/tokens`, `DELETE /v1/tokens/{id}` | yes | **403** | **403** |

`403 forbidden` is decided by the order in R3.4: a viewer PATCHing a nonexistent id gets 403 (role before existence); a member PATCHing a nonexistent id gets 404; a member PATCHing someone else's existing ticket gets 403. Tickets of a revoked token stay owned by it; nobody else owns them except admins editing.

## R11. Token lifecycle

**R11.1 Create** `POST /v1/tokens` (admin): body `{"name":"ci","role":"member"}`. `201`:

```json
{"id":"tok_0123456789abcdef","name":"ci","role":"member","created_at":1760000000,"revoked":false,
 "token":"td_<64 lowercase hex>"}
```

`id` matches `^tok_[0-9a-f]{16}$`; `token` matches `^td_[0-9a-f]{64}$` (32 CSPRNG bytes). Two creations never produce the same id or token. The secret is returned **exactly once**: no other response of any route, now or later, contains it or any hash of it.

**R11.2 List** `GET /v1/tokens` (admin): `200 {"items":[{"id","name","role","created_at","revoked"}...]}` in creation order, all issued tokens including revoked ones, never the bootstrap token, never keys named `token` or anything containing a secret or hash.

**R11.3 Revoke** `DELETE /v1/tokens/{id}` (admin): `200` + the token object (no secret) with `"revoked":true`. Revocation takes effect for the very next request (that token then gets 401). Revoking again is idempotent (`200`). Unknown id: `404`. `bootstrap`: `409 conflict`. A token may revoke itself or any other admin token, including the last non-bootstrap admin (the bootstrap token always remains).

## R12. Secret handling

* The state (every file the server writes in `<DIR>`) MUST NOT contain, as a substring, the bootstrap token, any issued token secret, or the `Authorization` header value of any request. It MUST contain, for each issued token, the lowercase hex SHA-256 of the exact secret string (UTF-8 bytes) - proof that a hash, not the secret, is persisted. Salting is not used (secrets carry 256 bits of entropy).
* stderr and stdout of the server MUST NOT contain any token secret or `Authorization` value, including when `--access-log` is on and when a fault occurs.
* Responses never contain a stored hash.

## R13. Persistence

State (tickets, id counters, tokens) lives under `TICKETDESK_DATA` and survives restart: after stopping the server (SIGTERM or SIGKILL) and starting it again with the same `TICKETDESK_DATA`, all tickets (all fields exactly), issued tokens (still valid, revoked ones still revoked) and the list order are identical; new ticket ids do not collide with or reuse old ones, including those of deleted tickets. Restart with a different `TICKETDESK_ADMIN_TOKEN` accepts the new bootstrap value and rejects the old one, while issued tokens remain valid. Concurrent requests never lose or duplicate an update.

## R14. Request body ceiling

Runner flag `--max-request-bytes N` (positive integer, default `1048576`) is provided by the implementer in `howlframe run`. A POST/PATCH body of at most N bytes is processed normally; a body of N+1 or more bytes yields `413 payload_too_large` without being parsed (even if it is not JSON). The server stays healthy afterwards. Invalid values (`0`, negative, non-numeric) make the runner exit 2 with a diagnostic on stderr.

## R15. Fault handling

* Any unexpected handler fault yields `500 internal_error` with the fixed message (R8.3), the `request_id`, and no other detail. The client sees nothing of the runtime error (no `RUNTIME_ERROR`, `HANDLER_PANIC`, opcode, capability, `ip=`, `.howl`, `panic`, `goroutine`, `Traceback`, error text of the fault).
* **Deliberate fault trigger (testing aid).** When the process environment has `TICKETDESK_ALLOW_DEBUG_FAULT=1` and a request to any *known path* (any method, any role, with or without a token, even before authentication) has the header `X-Debug-Fault: 1`, the handler MUST raise a genuine runtime fault (not hand-craft a 500). The fault is evaluated first, before the checks of R3.4. When the variable is not set to `1` the header MUST be ignored entirely.
* After a fault the server keeps serving; state is unchanged.
* On stderr, always (no flag needed), a fault writes one line, terminated by `\n`:

  ```
  fault request_id=<id> method=<M> path=<P> error=<runtime error text>
  ```

  `<id>` equals the response's `X-Request-Id`, `<P>` is the request path without query string, `<runtime error text>` is non-empty, on one line, and contains no secret.

## R16. Observability (DX item 5)

* With runner flag `--access-log` the server writes exactly one stderr line per request, after the response is produced, for every response including 404/405/413/500:

  ```
  access method=GET path=/v1/tickets status=200 duration_ms=3 request_id=<id>
  ```

  matching `^access method=[A-Z]+ path=\S+ status=[0-9]{3} duration_ms=[0-9]+ request_id=[0-9a-f]{32}$`: `path` is the raw request path without the query string, `status` the status actually sent, `request_id` the `X-Request-Id` of that response, `duration_ms` a non-negative integer.
* Without `--access-log` no line starting with `access ` is ever written.
* The `fault` line (R15) is independent of `--access-log`.
* Other stderr lines are allowed but never contain secrets (R12).

## R17. Robustness smoke

20 sequential followed by 20 parallel (at least 8 concurrent connections) ticket creations and reads all succeed with correct statuses, ids are unique, the list is complete, and the server is still healthy; no crash, no hang beyond 10 s per request.

## Requirement-to-gate map

A: R3, R4, R5, R6, R13, R17. C: R9, R10, R11, R12. D: R7, R8, R14, R15, (R16). G: R1, R2. H: R16 (item 5) and `dx/`.
