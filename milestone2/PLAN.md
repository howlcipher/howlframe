# Milestone 2 demonstration plan: a practical standalone backend API and better developer UX

**Status:** FROZEN 2026-10-13, before any implementation. Changes need a separately recorded revision.
**Authority:** `HOWLFRAME_PRODUCT_DIRECTIVE.md` section 6, Milestone 2 (D19, D29 **A, C, D, G, H**).
This plan adds no gate beyond those five. Persistent data (B), a separate networking/database gate (E),
a concurrency gate (F), and a Milestone 2 autonomy or independent-review criterion (I, J) are **not**
required. General security, correctness and authority obligations still apply to what is built.

## 1. The demonstration backend: `ticketdesk`

A small but real issue-tracker HTTP API (JSON over HTTP), written in HowlFrame and run on the
standalone VM. It is not related to the Howl ecosystem. Resources and rules (details are fixed by the
independently written `ticketdesk/requirements/REQUIREMENTS.md`, authored before implementation):

* **Tickets:** create, list (filter by status, pagination), read, partial update, delete.
* **Users and tokens:** bearer API tokens with roles `admin`, `member`, `viewer`. A bootstrap admin
  token comes from the environment; admins issue and revoke further tokens. Tokens are stored only as
  SHA-256 hashes and compared in constant time.
* **Authorization:** viewers read; members create and modify only their own tickets; admins do
  everything including token management and deleting any ticket.
* **Validation and errors:** every error is one JSON shape with a stable machine code; malformed JSON,
  wrong types, unknown fields, out-of-range values, oversize bodies and wrong methods are rejected
  with the right status; no internal detail, token or stack trace ever reaches a client.
* **Persistence:** state survives restart (file-backed native store, under governed grants).

## 2. Acceptance mapping (the five selected gates)

| Gate | What is demonstrated | Evidence |
|---|---|---|
| **A** Practical HTTP API | Routing with path parameters and methods, request/response processing, structured JSON payloads, status codes, headers, query filters, pagination, real CRUD workflow | Independent black-box suite `ticketdesk/tests/acceptance` passes against the running server |
| **C** Authentication and authorization | Missing, malformed, unknown and revoked tokens are 401; role and ownership rules yield 403; token secrets never stored or logged in clear | Same suite (auth section) plus a storage inspection test |
| **D** Validation and reliable errors | Uniform error schema; every malformed or invalid input class rejected with a specific code and field list; handler faults become a safe 500 with a correlation id and a useful server-side log line; limits (body size) enforced | Same suite (validation/error section) plus a fault-injection test |
| **G** Standalone install and execution | A clean checkout of the committed tree needs only Go and Python 3 stdlib (for the tests): one script builds the toolchain and the server artifact, starts it with the minimum grants, and runs the suite. No other Howl component, no generated Go/Python server | `milestone2/scripts/clean_repro.sh` output |
| **H** Demonstrably improved developer experience | Measured before/after improvements in real build/debug workflows, listed in section 3 | `milestone2/dx/` fixtures with recorded before/after outputs and automated tests |

## 3. Developer-experience improvements (gate H), chosen from Milestone 1 pain

Each item names a real obstacle met while building the three Milestone 1 applications, a measurable
"before" (recorded from the unmodified toolchain at the freeze commit), and an automated "after" check.

1. **Unbalanced parentheses.** Before: `{"reason":"Expected ')'","line":N,"column":1}` pointing at end of
   file for any unclosed form. After: the diagnostic names the line and column of the *innermost
   unclosed* `(` and its head word (and `unexpected ')'` names its position), so the fix location is
   found without bisecting.
2. **Calling a user function without `(call ...)`.** Before: `construct "f" cannot be compiled for the
   standalone bytecode target: no bytecode lowering exists for it`. After: when the name is a function
   defined in the same program, the diagnostic says to write `(call f ...)`.
3. **Multi-form bodies.** Before: `ARITY_ERROR: let expects 2 args, got 3`. After: the message says a
   `let` takes one body expression and suggests wrapping several in `(do ...)`.
4. **Type-mismatch branches.** Before: `if branches have incompatible types void and int`. After: the
   message names each branch's line and gives the usual remedies (make helpers `void`, or `return`).
5. **Server observability.** Before: no per-request log, and a failing handler logs a raw runtime error
   with no request context while the client sees a bare 500. After: an opt-in runner flag
   (`--access-log`) writes one structured line per request (method, path, status, duration, request
   id), and a handler failure logs the request id, method and path with the error; the same id is
   returned to the client in an `X-Request-Id` header and the safe 500 body.
6. **Documentation.** A tested guide `docs/guides/http_api.md` covering routing, JSON bodies, auth
   patterns, validation, errors, limits, logging and running the server standalone, derived from the
   ticketdesk implementation; every code block in it is compiled by an automated test.

## 4. Language and runtime support that this backend legitimately needs

Added only where a *demonstrated* blocker exists, narrowly, with regression tests (directive 7.3):

* Cryptographic primitives needed by token authentication: `(sha256_hex text)`, `(rand_hex n)` (secure
  random, `n` bytes), `(secure_equal a b)` (constant-time comparison). No capability; no new authority.
* A configurable request-body ceiling for `parse_json` in routes (runner flag), returning a catchable
  limit error so the application can answer 413.
* Whatever else an independent acceptance test shows to be missing, found and recorded in the journal
  with its failing reproduction first.

## 5. Process

1. This plan is committed. 2. An independent agent that does not implement writes
   `ticketdesk/requirements/REQUIREMENTS.md`, the black-box acceptance suite, and the DX fixtures with
   expected after-states; these are committed and frozen. 3. Baseline "before" outputs for section 3 are
   recorded from the unmodified toolchain. 4. Implementation, journals, local tests, pushes to `main`.
5. Clean-environment reproduction. 6. A separate reviewer runs everything and records a decision.
7. **Stop** when the five gates are demonstrated; do not start Milestone 3.

## 6. Out of scope

Persistence beyond a file-backed store, TLS termination, rate limiting, WebSockets, background jobs,
horizontal scaling, an ORM, a web UI, OS-level sandboxing, performance work beyond a functional smoke
test.
