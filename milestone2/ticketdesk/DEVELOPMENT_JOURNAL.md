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

## Review round 2 (2026-10-14): NOT ACCEPTED, one new blocker (B5), fixed

B1-B4 verified fixed by the reviewer. New blocker B5: my fix for B1 made listings walk the creation counter,
so after ~450 consecutive deletions a page cost more instructions than the default ceiling allowed (a permanent
500). Replaced by a bucketed live-id index (64 ids per bucket record): a page now costs O(page + empty buckets).
Also fixed minors m11 (a truncated upload was executed: now rejected before the handler runs), m13 (`%2F` aliased a
route: exact mode matches the percent-encoded path and unescapes captured segments) and m14 (server timeouts are
now in `docs/cli.md`). Regression tests added (mass deletion, truncated upload, encoded slash) run under the
default ceilings. Carryover: m12 (port probe can lose a race and then fails with the runtime error).

## Review round 3 (2026-10-14): NOT ACCEPTED, one new blocker (B6), fixed

B5/m11/m13/m14 verified. B6: DELETE removed the ticket before its index entry, so a crash between the steps
left an index id without a ticket and every listing page touching it returned 500 (reproduced with a kill storm).
Fixed twice over: DELETE unindexes first (a crash then leaves an unindexed, still readable ticket, the same
benign state as a crash in create), and listing skips index ids that have no ticket. Regression
`b6_dangling_index_entry_does_not_break_listing` (mutation-checked). m17 partly addressed: the runtime's 400
for a truncated upload now carries the request id. Carryover: m12 port probe race, m15 no migration of
state written by an earlier build, m16 a crash in create can leave an unindexed ticket (unconfirmed request).

## Closure verification (2026-10-20)

Scope: close the accepted Milestone 2 only. Implementation base: `main` at
`7092bcffc7f40c32309b2c6f54654e2aaa9279c5`. The initial worktree contained only the
uncommitted Round 4 additions to REVIEW.md and MILESTONE2_DECISION.md; both are preserved,
including superseded findings. Closure changes only documentation and acceptance records.

Roles: the original journal identifies Claude (Sonnet 5.5) as implementer and an independent
pre-implementation author of the frozen scope. REVIEW.md identifies an independent final
reviewer without an implementation role. The supplied closure plan reports a separate read-only
audit, but no audit transcript or evaluator identity is available here. Historical evaluator identities
are incomplete: no cross-provider proof is claimed. Codex performed this documentation closure
and fresh executable checks, not a new independent acceptance review.

Host tools observed: Ubuntu 26.04 LTS, linux/amd64; Go 1.26.0; Python 3.14.4;
GNU Bash 5.3.9; Git 2.53.0; GNU tar 1.35. Reproduction requires Bash, Git (a repository
with HEAD for git archive), tar, mktemp, mkdir, tail and ordinary host utilities, Go and Python 3.
Allow module-download access for the complete Go suite with empty caches; the root application
build uses only the standard library. No external model service is required.

Fresh checks at the implementation base:

* `bash milestone2/scripts/clean_repro.sh`: exit 0; 101/101 acceptance, 9/9 review regressions
  at default runner ceilings, 13/13 DX checks. The script archives committed HEAD, not worktree edits.
* `go test -count=1 ./...`: exit 0, no failures (uncached execution).
* `go build ./...`, `go vet ./...`, `test -z "$(gofmt -l .)"`, `git diff --check`: exit 0.
* `git diff 10d9c2a8 HEAD --exit-code -- milestone2/ticketdesk/requirements milestone2/ticketdesk/tests/acceptance milestone2/ticketdesk/dx milestone2/PLAN.md`: exit 0, no frozen-file changes.

Isolation procedure: create a fresh directory with `task_repro=$(mktemp -d)`, then run:

```bash
env -i PATH=/usr/bin:/bin HOME="$task_repro/home" TMPDIR="$task_repro/tmp" \
  GOCACHE="$task_repro/go-build" GOMODCACHE="$task_repro/go-mod" GOTOOLCHAIN=local \
  bash -c 'mkdir -p "$HOME" "$TMPDIR"; bash milestone2/scripts/clean_repro.sh'
```

Isolated reproduction result: exit 0; 101/101 acceptance, 9/9 regressions, 13/13 DX.

This uses an empty inherited environment, fresh HOME, temporary directory and Go caches,
and host tools. It is an isolated build environment, not OS sandboxing. An initial attempt
with inherited PATH exited 127 because the user Go wrapper requires Distrobox environment;
the explicit system PATH above avoids that wrapper. The temporary archive is removed by
the reproduction script; outer cache directories are disposable.

Historical adversarial probes (ADV01–ADV100, t19.py SIGKILL storm and other temporary scripts)
are preserved as reviewer-reported evidence in REVIEW.md. Those scripts are unavailable and were
not rerun for closure. Fresh retained regression tests include the crafted dangling-index case;
they do not substitute for a rerun of the historical kill storm.

The Round 4 decision remains ACCEPTED for D29 A, C, D, G, H. Its non-blocking carryover is
retained. The handoff's pending-review and quadratic-sort descriptions were stale; the current
review and bucket-index implementation supersede them. Older host-specific exec failures above
remain historical records, not a claim about this Ubuntu verification run.

Final documentation verification: local Markdown file targets exist; evidence anchor matches this
section; `git diff --check` passes. Only README.md, change_log.md, the decision, review, handoff
and this journal are in the closure commit. No unrelated changes were present at entry.

Stop after the local documentation closure commit; no carryover implementation, Milestone 3,
push, pull request or merge is authorized by this closure task.
