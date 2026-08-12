# Task API Development Notes

Phase 3 dogfooding: a genuine stateful HTTP CRUD service, running as
standalone bytecode, exercised across multiple independent HTTP requests
against one long-lived server process.

## What worked well

* **State sharing across HTTP requests.** Every `route` handler runs in a
  fresh child `BCVM` per request, but all child VMs share the *same*
  `*bcStoreRegistry` pointer as the parent server VM, and the registry and
  each named store are protected by real `sync.Mutex`/`sync.RWMutex`
  (`internal/vm/vm.go:987-998`). A `POST /echo` followed by a separate
  `GET /last` request round-tripped a stored value correctly through two
  independent HTTP requests against one running process — this is not
  application misuse, it is real, correctly synchronized runtime behavior.
* **JSON request bodies parse into usable dicts with no `struct`
  declaration.** `(parse_json AnyNameSymbol req.body)` decodes into a native
  Go `map[string]any` when the JSON is an object, which is exactly the type
  `map_get`/`map_set` expect. `(map_get body "title")` on a POSTed
  `{"title": "..."}` worked immediately. This directly contradicts an older
  note in `docs/dogfooding_report.md` claiming dynamic dict parsing needs a
  `struct`; that claim is stale for the bytecode target as of this
  investigation.
* **`try_let` around `parse_json` gives clean, correct HTTP error
  responses.** Malformed JSON bodies produced a proper `400
  {"error":"invalid_json"}` on every endpoint, verified with real `curl`
  requests before the automated suite existed.
* **The capability model held up under real HTTP load.** `network` gates
  `HTTP_SERVER_START`/`HTTP_ROUTE`/`RES`/`RES_JSON`; `database` gates every
  `STORE_*` opcode. No caps → immediate fail-closed start failure.
  Database-only → same immediate start failure (the server itself needs
  `network`). Network-only → server starts, but store access inside any
  handler is denied. Network+database → full CRUD works. Unrelated
  capabilities (`filesystem`, `process`, `environment`) never substituted
  for either. This is a clean, repeated demonstration of "intent is not
  authority."
* **Deterministic id/list handling from Todo CLI carried over unchanged** —
  the `next_id` counter record and the `0..max_id` scan for `list` needed no
  adaptation for the HTTP context.

## Awkward but workable

* **`store_open` cannot appear at `http_server` top level** — only `route`,
  `defun`, `struct`, `import`, `test`, `module`, `export`, `middleware` are
  accepted there. Every route handler that touches the store must call
  `store_open` itself; it is a cheap, idempotent re-attach to the same named
  store by URI string, so this is more a naming-convention tax than a real
  limitation.
* **A route lambda body must be a single node**, exactly like Todo CLI's
  `let` bodies — multiple statements need a wrapping `do`. Unlike `let`,
  `defun` bodies genuinely accept multiple sequential top-level statements
  in codegen, but see "Bugs discovered" below for where that decomposition
  path broke down in practice.
* **`let`-nesting was noticeably shallower than Todo CLI's 16+ levels** —
  the deepest chain in `task_api.howl` is about 6 levels (parse → validate →
  lookup → branch → mutate → respond), because splitting the five actions
  into five separate `route` bodies already does the decomposition that
  Todo CLI's single flat `while` loop over `cli_args` could not. Route
  separation, not `defun`, turned out to be the load-bearing structural
  tool here (see "Bugs discovered" for why `defun` didn't end up carrying
  this weight for dict-returning helpers).

## Missing capability

* **No atomic increment / compare-and-swap on the store.** See the
  concurrency finding below — `next_task_id`'s read-then-write counter
  pattern is not atomic across two opcodes, so concurrent creates can
  collide. A native atomic-increment primitive would close this; not
  something to add speculatively from a single app's evidence, but worth
  tracking if a future app needs guaranteed-unique concurrent ids.
* **No store key enumeration**, same finding as KV CLI and Todo CLI — the
  `next_id` counter scan workaround was used again, unchanged, and remains
  adequate friction, not a blocker.

## Bugs discovered

Two genuine, reproducible defects were found and worked around at the
application level rather than patched in core, per this task's scope
(neither blocks Task API; both are documented here as evidence for a future
core-improvement pass):

1. **`defun` return-type annotations compile but crash at runtime on the
   bytecode target.** The checker's `functionSignature` (`internal/checker
   /types.go`) supports an optional return-type symbol —
   `(defun f (args) TypeName body...)` — and correctly excludes it from the
   inferred body. The bytecode compiler's `case "defun"`
   (`internal/bytecode/bytecode.go:106-120`) does not know about this
   convention: it compiles every node from `Children[3:]` as body,
   including the type-name token, which compiles to a variable-load
   instruction for an undefined variable. Any annotated `defun` panics
   immediately when called (`undefined variable: int`, `undefined variable:
   any`, etc.), silently manifesting to an HTTP client as the empty-200
   behavior described below. **Minimal repro:** `(defun f () int (return
   1))` inside an `http_server`, called from a route, compiles cleanly and
   panics on every call.
   **Workaround used:** never annotate `defun` return types; keep helper
   functions returning plain strings (the checker's default return type),
   which matches their default inferred type with no mismatch.
2. **Checker return-type inference for `defun` is inconsistent with its own
   default, for any function that must return a non-string value (e.g. a
   dict) and is consumed by `map_get` at the call site.** Without an
   annotation, `functionSignature` defaults every `defun`'s declared return
   type to `string` (`internal/checker/types.go` around
   `functionSignature`). A helper that legitimately returns a `dict` (e.g.
   record lookup or dict construction) either fails to compile at its own
   definition (concrete `dict`-typed body vs. declared `string`) or, if the
   mismatch happens to be masked, fails at every *call site* instead
   (`map_get target must be dict, got string`), because the caller trusts
   the function's declared signature type, not the body's real type. Since
   annotating the return type hits bug #1 above, there is currently no
   working path to a `defun` that returns a dict and is safely consumed by
   `map_get`.
   **Workaround used:** dropped the `find_task`/`make_task` helper
   functions entirely; inlined `store_open`/`store_get`/`dict` construction
   directly in each of the five route bodies instead. This cost a small
   amount of duplication (the same three-line store-open-and-lookup pattern
   appears in `get`, `complete`, and `delete`) in exchange for correctness.
   `next_task_id` remains a working `defun` example precisely because it
   was rewritten to return a string.

Neither bug was patched in `internal/`. Both are real, generalized,
reproducible checker/bytecode-compiler divergences independent of Task API,
but the application-level workaround (string-returning helpers only;
inline dict-producing logic) fully satisfies every requirement this task
sets, so no core change was made — see `docs/application_dogfooding_phase_3.md`
for the classification rationale.

## HTTP findings

Investigated directly by reading `internal/vm/vm.go`'s `OpHttpRoute`,
`internal/bytecode/bytecode.go`'s `case "route"`, and `internal/checker
/checker.go`'s `route`/`http_server` validation, then confirmed empirically
against a running compiled server:

* **Routing is literal-path-only.** `route` registers its path string
  directly with Go's `http.ServeMux`. This repo's `go.mod` declares `go
  1.21`, which — verified by building a throwaway probe binary under a `go
  1.21` module — disables Go 1.22's method-prefixed patterns (`"GET
  /tasks"`) and `{wildcard}` path segments; both degrade to literal path
  text and 404. **There is no dynamic path-parameter support and no
  method-based route dispatch.**
* **No opcode exposes method, query string, headers, or path segments** to
  application code. The only thing the bytecode VM does with the raw
  `*http.Request` bound in a route lambda is the special-cased
  `(parse_json Name req.body)` body read. **All request input must arrive
  as a JSON body**, which is why every Task API action is `POST` with a
  JSON payload, even `list` and `get` (which would conventionally be `GET`
  with a path/query parameter).
* **Method is never read or enforced.** Confirmed with `curl -X GET
  http://localhost:8081/tasks/list -d '{}'` returning the identical `200`
  response as the equivalent `POST` — a real, deliberate deviation from
  REST semantics, not an oversight.
* **Unmatched routes fall through to Go's stock `http.ServeMux` 404**
  (`404 page not found`, plain text) — Task API does not attempt a custom
  catch-all JSON 404, since a `"/"` catch-all route interacts with the same
  literal-path-matching limitation above and wasn't worth the added
  surface area for a fallback response body.
* **An unhandled panic inside a route handler is silently swallowed as a
  successful response.** `OpHttpRoute`'s handler wraps the compiled body in
  `defer func() { recover() }()`, which only `fmt.Println`s to the server's
  stdout. Since nothing is written to the `http.ResponseWriter`, Go's HTTP
  server sends a default **`200` with an empty body**. Verified directly:
  a database-capability-denied `store_open` inside a handler returns `200`
  with zero bytes to the client, with the only evidence server-side in the
  process's own stdout log. This **contradicts an unverified claim in
  `apps/status_api/DEVELOPMENT_NOTES.md`** ("returns a 500 equivalent") —
  that status_api claim was never actually exercised over HTTP by its own
  test suite, and is now known to be wrong for the bytecode target.
  **Task API avoids this for every client-facing validation path**
  (missing/empty title, missing/unknown/malformed id, malformed JSON) by
  using `try_let` around `parse_json` and explicit `if`/`is_nil`/empty-
  string checks before any store or JSON operation, so no *routine client
  mistake* ever reaches this path — see `docs/application_dogfooding_phase_3.md`
  for why this was deliberately not patched in core.

## State findings

* **Lifetime:** memory-bound to the server process; discarded on process
  exit or restart, exactly like Todo CLI's single-execution store, but now
  proven to persist correctly *across* many independent HTTP requests
  within that process's lifetime, not just within one execution.
* **Sharing:** verified safe and correct — see "What worked well" above.
  `bcStoreRegistry` and each `bcMemoryStore` are mutex-protected, and every
  per-request child VM references the same registry instance as the parent
  server VM.
* **Missing-key semantics:** unchanged from Todo CLI — `is_nil` on
  `store_get` cleanly distinguishes "not found" from a real record;
  `map_get` on a missing dict key returns `""` rather than panicking, which
  conveniently doubles as the "missing/empty field" validation check on
  parsed JSON bodies (a missing `"title"` and an explicit `"title": ""`
  are indistinguishable, and are treated identically on purpose).
* **Deletion:** `store_delete` removes the record; a subsequent `get`
  correctly reports `404`, and `list`'s `0..max_id` scan correctly skips
  the resulting gap without special-casing it.

## Capability findings

See "What worked well" for the full matrix (no caps / network-only /
database-only / both). The one caveat worth repeating: the "network-only"
denial case is enforced correctly (the store operation really is blocked),
but the *client-visible signal* for that denial is the silent-200-empty-body
behavior described above, not a clean error — the enforcement is real, the
ergonomics are poor.

## Concurrency findings

A bounded concurrency probe (20 concurrent `POST /tasks/create` requests,
followed by 10 concurrent `GET`-equivalent `POST /tasks/get` reads against
one existing task) was run as part of the automated suite and with `go test
-race`. Results:

* **No crashes, no corrupted records, no lost server stability** under
  concurrent load — every response was well-formed JSON with a valid status
  code.
* **Concurrent reads always agreed** — 10/10 concurrent lookups of the same
  task id returned identical, correct data.
* **Concurrent creates are not guaranteed unique ids.** `next_task_id`'s
  read-then-increment-then-write sequence is two separate mutex-protected
  store operations, not one atomic operation, so two concurrent requests
  can both read the same counter value before either writes back the
  increment. This reproduced reliably: runs observed between 1 and 5 id
  collisions out of 20 concurrent creates. This is a genuine, real
  concurrency limitation of the application's counter pattern (shared with
  Todo CLI's identical pattern, previously untested under concurrency
  because Todo CLI is a single-execution CLI, not a multi-request server) —
  not a store data race (no crash, no corruption, no torn writes), but a
  classic non-atomic-counter lost-update. Documented, not invented away;
  see "Missing capability" above.

## Instruction-budget findings

Confirmed by reading `internal/vm/vm.go:1431` / `:1494`: each HTTP request
runs in a freshly allocated child `BCVM`, and the executed-instruction
counter (`vm.executed`, checked in `vm.go:1171`) is a field on that struct
— it is **not cumulative across requests**. A long-running server does not
"use up" instruction budget from prior requests; every request gets the
full default ceiling (100,000) independently. The 100-task stress workload
(100 sequential creates, a list, 50 completes, 50 deletes, 100 verification
gets — 300+ total requests) ran to completion with the unmodified default
ceiling and no `LIMIT_EXCEEDED` errors on any single request.

## Developer experience

| Area | Rating | Notes |
| --- | --- | --- |
| HTTP routing | Reasonable | Simple and predictable once literal-path-only is understood; no dynamic routes. |
| Request parsing | Awkward | Only the JSON body is readable; no method/query/header access at all. |
| Response generation | Easy | `res_json`/`res` are direct and predictable. |
| JSON | Easy | `parse_json` into a dict worked immediately, better than expected from prior docs. |
| CRUD logic | Reasonable | Maps cleanly onto `store_get`/`store_put`/`store_delete`, same as Todo CLI. |
| State | Easy | Correctly shared across requests with no extra effort once `store_open` is called per-handler. |
| Collections | Reasonable | `list_len`/`append`/the `next_id` scan workaround all carried over cleanly from Todo CLI. |
| Errors | Reasonable | `try_let` + explicit checks work well for validation; capability-denial panics remain a real, undocumented-until-now client-visible gap. |
| Capabilities | Easy | Predictable, fail-closed, and correctly scoped to `network`/`database`. |
| Modules / decomposition | Awkward | Route separation reduced nesting well; `defun` helpers work for string-returning functions but are currently broken for dict-returning ones (see Bugs discovered). |
| Diagnostics | Reasonable | Compile-time checker errors were clear (`"undefined reference to \"kv\""`, type mismatches); the runtime panic-to-silent-200 path is the one weak spot. |
| Testing | Easy | The `status_api_test.go` process-group-kill pattern extended directly to a much larger CRUD/concurrency/stress/standalone-bytecode suite with no new infrastructure needed. |
