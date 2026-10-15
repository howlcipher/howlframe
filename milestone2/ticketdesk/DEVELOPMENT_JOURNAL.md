# Milestone 2 development journal (ticketdesk + developer experience)

Implementer: Claude (Sonnet 5.5), autonomous. The plan (`../PLAN.md`), the API contract, the 101-test
acceptance suite and the developer-experience (DX) checker were written and frozen by an independent author
before any implementation (commit `10d9c2a8`). The "before" outputs for gate H are in `dx/BASELINE.md`.

## What the independent feasibility notes got right

The author listed 14 risks from reading the runtime; each was a real blocker and was closed with the
smallest runtime change that removes it, a regression test, and (where a test pinned old wording) an
evidence-based test update:

| Blocker | Change | Test |
|---|---|---|
| Token auth needs hashing, randomness, constant-time compare | `sha256_hex`, `rand_hex`, `secure_equal` (pure, no capability) | `internal/vm/crypto_test.go` (interpreter and bytecode, mutation-checked) |
| Fixed 10 MiB body, no flag | `--max-request-bytes` (default 1 MiB), catchable `LIMIT_EXCEEDED` | `request_limit_cli_test.go` |
| No request id, no access log, fault log without context, bare 500 | `X-Request-Id` on every response, `(req_id req)`, `--access-log`, `fault request_id=... method=... path=...`, id in the safe 500 body | `observability_cli_test.go` |
| Unknown routes got Go's plain-text 404 | `(route "*" ...)` fallback | `m2_runtime_cli_test.go` |
| Port could not come from the environment | `(http_server <expression> ...)` | same |
| Body readable only through `parse_json` (no UTF-8 check) | `(req_body req)`, `(utf8_valid s)`, `(req_url_path req)` | same |
| No atomic read-modify-write | `(serialize_requests)` | same (20 concurrent updates, mutation-checked) |
| Store file: non-atomic write, zero-byte file rejected | atomic temp+rename+fsync; empty file is empty state | `runtime_integrity_test.go` |
| Startup could not prove grants | `(granted "network")`; `inherit:filesystem` grant attenuation so handlers get exactly the runner's filesystem scope | `m2_runtime_cli_test.go`, `capability` test |
| Shared never-reclaimed memory budget exhausts a server | memory budget per request | `TestHTTPServerMemoryBudgetIsPerRequest` (mutation-checked) |
| Invalid resource flag exit status | `run` exits 2 (usage) | acceptance T-R14 |
| Percent-decoded paths could split a log line | paths are logged and returned percent-encoded | acceptance T-R16, `m2_runtime_cli_test.go` |

## Server implementation

`src/ticketdesk.howl`. Learned from probes before writing: defun helpers can call `res_json` and open
stores, but a store handle cannot be a function argument expression (bind it first), startup forms cannot
see store handles from other forms (helpers reopen the store), and lists are copied across calls while dicts
are shared. First full run of the frozen suite: 99/101; the two failures were the runner exit status for an
invalid flag and a log path that was percent-decoded (a real spec violation the suite caught). Then 101/101
on two consecutive runs.

## Gate H

1. Parentheses: the parser now names the innermost unclosed `(` (line, column, head word) and a stray `)`.
2. Missing `(call ...)`: the build diagnostic says the name is a function defined in the program.
3. `let` body: `ARITY_ERROR` now says a `let` takes one body expression and to wrap several in `(do ...)`.
4. `if` branches: names both branch lines and the remedies; the old prefix is kept.
   The DX checker went from 2/13 (controls only) to 13/13. Two existing tests pinned the old wording:
   one still matches the unchanged prefix; `TestAnalyzeRejectsIncompatibleBranchAndCallLayouts` is an exact
   match and was updated to the new message (the wording change is the gate H deliverable).
5. Server observability: see the table.
6. `docs/guides/http_api.md`, enforced by `TestHTTPAPIGuideRule`: every code block compiles; breaking a block
   makes the test fail (checked by hand). New flags are documented in `docs/cli.md`.

## Test impact assessment

Behavior changed: HTTP runtime (ids, logging, fallback, body, port, serialization, per-request budget),
store persistence, grants attenuation, six new built-ins, run-flag exit status, four diagnostics.
Existing tests re-run in full; the only failures are the 14 known host-specific exec tests (symlinked
coreutils), unchanged from the Milestone 1 baseline.

## Review round 1 (2026-10-14): NOT ACCEPTED, four blockers, all fixed

The independent reviewer found what the frozen harness hid: it launches the server with raised ceilings
(`--max-instructions 4000000000 ...`) while the documented command uses the defaults.

* **B1** (documented command, default ceilings): a 2857+ character body returned 500 and listings failed after
  ~100 tickets. Causes: my type classifier cost instructions proportional to the value size (`str_slice` over
  the whole encoded value), and listings read and sorted every key. Fixed: anchored regexes (constant cost), a
  byte-length pre-check before counting code points, listings that walk the creation counter (cost grows with
  the page, not the dataset). Regression `tests/regression/review_round1.py` runs with default ceilings.
* **B2** (port in use printed a raw runtime object): new `port_available` built-in (network capability,
  registered, gated, denial-tested); the server probes before listening and exits 1 with a prefixed line.
* **B3** (a stalled upload froze the server under `(serialize_requests)`): the body is read, bounded, before
  the lock is taken, and the server has read-header/read/idle timeouts.
* **B4** (`/v1//tickets`, `/v1/./tickets` answered 200): with a `"*"` fallback the dispatcher matches paths
  exactly as sent instead of cleaning them.
* Also: state file with a wrong-shaped `meta` is rejected (65); the fix exposed a language bug, `(exit n)`
  inside a `try_let` value was swallowed (now uncatchable, test mutation-checked); percent-encoded paths;
  directory fsync after the atomic rename; no constant request id on entropy failure; guide example returns
  413; `docs/cli.md` budget text; README claims corrected.
