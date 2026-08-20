# Goal

Secure the file-backed store authority boundary, fix the three HowlBoard-discovered VM semantic defects, and verify HowlBot as a second direct-HFIR policy consumer before recommending further HFIR expansion.

# Starting SHAs

| Repository | SHA | State |
| --- | --- | --- |
| HowlFrame | `3262cb266fabcaa38e9185d9d364a78d21a6696` | Existing user changes present; preserved |
| HowlChangeOps | `1e36965228330d6ae12dec137ca06ef8067c6be1` | Clean |
| HowlBoard | `947ff42cbf8dcf1c63b7f5747d7ba573e2d00540` | Clean |
| HowlBot | `c0056fab8074736b5c56dd4ecbc7cbb05394679e` | Clean |

# Consumer roles

* HowlChangeOps is the mature authority and direct-HFIR regression consumer.
* HowlBot is the second independent trusted-adapter and direct-HFIR policy consumer.
* HowlBoard is the broad bytecode runtime stress consumer for HTTP, functions, persistence, JSON, and state transitions.

# File-store authority investigation

Before changes, a minimal `cli_app` fixture using `file:///tmp/howlframe-file-store-gate/store.json` was compiled successfully. Results:

| Grant | Exit | File effect | Result |
| --- | ---: | --- | --- |
| none | 1 | absent | Structured `CAPABILITY_DENIED` for `database` at `STORE_OPEN` |
| database | 0 | created, 38 bytes | **Authority bypass:** physical file was written with database only |
| filesystem | 1 | absent | Structured `CAPABILITY_DENIED` for `database` at `STORE_OPEN` |
| database,filesystem | 0 | created, 38 bytes | File-backed store succeeded |

The database-only success proves intent classification was being used as filesystem authority. The initial reproduction was retried with `GOCACHE` in `/tmp` because the sandbox's default Go cache was read-only; that environment issue did not affect the runtime result.

# HowlBoard runtime bugs

Initial source inspection confirmed the reported response-context failure mechanism: route request environments contain `w`, while bytecode `OpCall` built a nil-parent call environment. The checker recognizes `true` and `false` as boolean types, but the legacy bytecode compiler emitted all symbols as `LOAD_VAR`; the direct HFIR graph/lowerer also admitted only numeric and string constants. `OpMakeList` started from a nil slice, so `(list)` serialized as JSON `null`.

The fixes are request-scoped call-environment inheritance, canonical boolean constants in the interpreter, legacy bytecode, and HFIR transport/lowering, and non-nil zero-length list construction. Focused tests cover direct and nested response-header calls, concurrent `httptest` requests, outside-request failure, boolean print/condition/dict/store behavior, and empty-list HTTP/store/JSON behavior. The focused VM suite passes, including the HTTP context test under `go test -race`.

Clean HowlBoard backend compilation passed. Its unmodified contract test initially failed closed with `CAPABILITY_DENIED: filesystem` because the current backend uses `file://` persistence while granting only `network,database`; a temporary test checkout granting `network,database,filesystem` passed the contract suite. This is the expected post-fix capability correction, not a consumer rewrite. A separate temporary restart test created a task, restarted the backend, and read it back successfully.

The existing `set_html` concern remains present: HowlBoard constructs HTML strings from task data and HowlFrame maps `set_html` to `innerHTML`. The issue remains tracked in `docs/howlboard_xss_set_html_issue.md`; it was not mixed into this runtime gate.

# HowlBot direct-HFIR conformance

The clean current policy compiled successfully through both legacy bytecode and direct HFIR bytecode without consumer changes. Ten normalized trusted-event cases were run through interpreter, legacy bytecode, and direct HFIR bytecode: `status`, `policy`, `history`, `repo_status` with no role, maintainer, and admin, `repo_release` unprivileged, admin/unapproved, admin/approved, and a forged/non-normalized role string. All three paths produced identical normalized decision, reason, action, and action target values. HowlBot's own Go tests also passed.

The construct inventory is small and overlaps the HowlChangeOps direct subset: `cli_app`, `cli_args`, `let`, `set`, `if`, `do`, `for`, `list`, string splitting/joining, boolean predicates, map-like dictionaries, printing, and comparisons. HowlBot-only policy concerns are trusted role evidence and Discord identity, which remain Go-adapter inputs rather than HowlFrame concepts. HowlChangeOps-only constructs include proposal-file reads, JSON parsing, recovery, and evidence conversion. The shared core direct-HFIR result generalizes. Role/set intersection and JSON-array ergonomics did not block execution and remain backlog items.

# HowlChangeOps regression

The clean current HowlChangeOps checkout remained unchanged. Its temporary test checkout passed the existing integration and adversarial suites, including ALLOW/DENY/REQUIRE_APPROVAL outcomes, ignored AI self-approval, trusted approval/tag handling, stale evidence rejection, approval tampering rejection, replay prevention, traversal rejection, and command-injection rejection. Its direct-HFIR artifact and legacy bytecode artifact produced matching `REQUIRE_APPROVAL` behavior for a normalized release-candidate event. The current interpreter path reports the existing explicit `try_let` Phase 1 limitation, so it was not treated as a passing interpreter lane.

The full `authority_bypass_test.sh` passed when run with the documented external temporary HMAC key: branch bypass was denied, action bypass was denied, and decision mutation was caught with `DENIED: Decision modified`. The script itself does not initialize `CHANGEOPS_APPROVAL_KEY_FILE`; the key was supplied by the harness without changing the consumer checkout.

# Security tests

The store capability resolver is centralized in `internal/capability/capability.go`. `memory://` requires database; `file://` requires database and filesystem. The VM now resolves the URI requirements before opening a file, and persistence read, JSON decode, marshal, and write failures become structured VM errors. File mutations commit only after a successful write.

Focused tests pass for memory isolation, cross-runtime file persistence, database-only inability to create, modify, or read a file, filesystem-only denial, database-plus-filesystem success, invalid persisted JSON, unreadable paths, write failure, and persistent delete. Pre-fix database-only file creation was reproduced above; post-fix database-only cases fail before physical I/O. No model/source program can add capabilities to its own allowed set.

# Consumer tests

HowlBoard's clean backend compiled and its contract smoke passed in a temporary checkout with the explicit capabilities required by its `file://` store. The restart persistence check passed. HowlBot's three-path matrix passed all ten cases. HowlChangeOps integration, adversarial, authority, and legacy/direct-HFIR differential suites passed.

# Remaining gaps

HowlBoard is not direct-HFIR compatible for functions, calls, HTTP, or stores yet, and its raw-HTML safety issue remains open. Existing dirty HowlFrame changes are unrelated `time_now` support and were preserved.

The requested validation set is green: `gofmt -l .`, `go build ./...`, `go vet ./...`, `go test ./...`, `go test -race ./...`, the Python benchmark harness, the difftest runner, `go run ./cmd/codegen`, and `git diff --check`. HTTP-dependent checks required localhost-enabled execution outside the restricted sandbox; no source or consumer edits were made for that environment requirement.

# Roadmap decision

Runtime integrity, HowlBot direct-HFIR conformance, HowlChangeOps authority regression, and HowlBoard backend stability are green locally. However, this work remains uncommitted on the pre-existing `main` worktree: no PR, remote CI, merge, or post-merge main verification was performed. Therefore the immediate next milestone is runtime-integrity PR/CI closure, not HFIR expansion. After that external gate is green, HFIR Phase 2B should target the HowlBoard backend, beginning with `defun`/`call`/`return`, then stores, and then HTTP request/response semantics. The existing `set_html` XSS concern remains a separate safety backlog item and must not be silently treated as fixed.
