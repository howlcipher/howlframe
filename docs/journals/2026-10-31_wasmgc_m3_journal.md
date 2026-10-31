# WasmGC backend M3 (browser host): working journal

The running record of M3, updated with every slice so that a later session or another agent can resume without the
conversation that produced it. The task prompt is `2026-10-30_wasmgc_m3_prompt.md`; the plan and its task list are
`docs/design/wasm_gc_backend.md` ("M3 tasks: browser host" and "Browser host operations"), which stays the source of
truth for decisions. Newest entry last. Times are America/Detroit.

## How to resume

1. Read the prompt, the plan's M3 task list and the last entry below.
2. `git fetch origin && gh pr list --state open` and compare with "Slices" below. Branches named there hold the
   committed work; a session's scratchpad worktrees do not survive it, so nothing lives only in one.
3. `go test ./internal/backend/wasmgc/ ./tools/difftest/`, then the HowlScene gates with
   `HOWLSCENE_HF=<howlscene-hf checkout> go test ./internal/backend/wasmgc -run TestHowlScene -v`.
4. Add an entry here in the same PR as the work.

## Slices

| Slice | Branch | PR | State |
| --- | --- | --- | --- |
| 0. Probe: every browser host operation | `docs/wasmgc-m3-probe` | #286 | Merged (`13362363`) |
| 1. Typed host operation table | `feat/host-operation-registry` | #287 | Merged (`b2093457`) |
| bugs.md #70: JS DOM operations' `TypeError` on a missing element | `fix/js-raw-dom-type-error` | | Open |
| 2. Browser host: `$Host`, `web_app` root, loader, synchronous operations | `feat/wasmgc-browser-host` | | Open |
| 3. Handlers: closures, `on_event`, subscriptions, `spawn` | `feat/wasmgc-handlers` | | Open |
| 4. Asynchronous operations: resumable frames | `feat/wasmgc-async` | | Open |

## Entries

### 2026-10-31 09:00: probe

- Read the JavaScript backend's browser runtime and the checker's host tables, and counted HowlScene HF's use of each
  operation (`247365e`). The table is in the plan ("Browser host operations"): the 51 operations the desugarer lists
  as browser host operations, plus `fetch`, `sleep`, `spawn` and the output operations. The site uses 20 of them, all
  synchronous (layout, classes, style variables and the three subscriptions); the editor uses 29, among them the
  asynchronous storage, file and image operations, `sleep` and `spawn`.
- Findings: lambdas occur only as the last argument of `on_event`, the three subscriptions and `spawn`, never as a
  callable value; no browser operation yields bytes (base64 text everywhere, `fetch` resolves to text), so no byte
  value type is needed in M3; seven "raw" operations (`dom_query`, `set_text`, `set_html`, `set_attr`,
  `toggle_class`, `dom_value`, `on_event`) throw the engine's `TypeError` on a missing element instead of a
  HowlFrame error, which needs a JavaScript backend fix before "the same errors" can hold.
- JSPI: Node 22.22 (V8 12.4) has only the pre-standard API behind `--experimental-wasm-stack-switching`
  (`WebAssembly.Suspender`), not `WebAssembly.Suspending`/`promising`; no other engine with JSPI is installed. To be
  measured in the asynchronous slice.
- Host text: std operations follow HowlFrame's Unicode 15 table, host operations the engine (recorded in the plan).

Plan of slices (each its own branch and PR):

1. Typed host operation registry shared by the checker, the JavaScript backend and this backend.
2. JavaScript backend: the raw operations report `TYPE_ERROR` (bugs.md entry), so both backends can match.
3. `$Host`, a `web_app` root, a generated browser loader, value marshalling and the synchronous DOM operations.
4. Closures, `on_event`, the subscriptions and `spawn`.
5. Asynchronous operations (resumable frames or JSPI, measured), storage, files, images, `sleep`, `fetch`.
6. The remaining operations (rich text, downloads, motion), then the exit gate: the browser tests on both backends.

### 2026-10-31 10:30: typed host operation table

- `capability.HostOperations`: 51 operations, each with parameters (name and kind: text, number, bool, value, list,
  element, elements, event, subscription, lambda with its parameters), result kind, capability and the asynchronous
  flag. The checker's `jsHostOperations`, `jsSubscriptionOperations` and eleven inline arity branches are replaced by
  one lookup; the usage texts are generated and equal the old ones (`TestHostOperationUsage`).
- One wording change: `on_event` with a non-lambda last argument now reports "on_event expects a lambda as its last
  argument", as the subscriptions did ("on_event expects a lambda" before), and requires the lambda's parameter
  list, as they did.
- Placement: `internal/construct` imports `ast`, and the desugarer (in `ast`) needs the set of browser operations, so
  a table in `construct` could not feed it. `internal/capability` imports no HowlFrame package; `ast` still keeps
  its own set, which a checker test holds equal to the table.
- Checked not vacuous: dropping `storage_keys`'s asynchronous flag fails `TestHostOperationsMatchRegistry`.

### 2026-10-31 12:00: browser host (`feat/wasmgc-browser-host`)

- `build --target wasmgc` of a `web_app` writes `app.wasm` and `app.js`. Imports come from the table; the loader runs
  the JavaScript backend's runtime functions (`javascript.HostRuntime`), so "the same errors" holds by construction,
  and converts values. First end-to-end run matched the JavaScript backend's page exactly apart from the caught error
  text (VM report against `Error: ...`), which is by design.
- The test page (`pageJS` in `browser_test.go`) is a superset of the JavaScript backend's DOM and motion stand-ins;
  each fixture builds with `--target js` and `--target wasmgc`, runs on the page and must print the same JSON (caught
  errors compared by code and message). 18 fixtures.
- Findings: a host number can be a float, and a program whose only floats come from the page trapped `unreachable`
  in the float bridge: host operations whose results can hold numbers now load the float module. An uncaught host
  error first surfaced as `RUNTIME_ERROR: TypeError: ...`; the loader now rethrows the host's own error, as the
  JavaScript backend's page sees it. A page needs `atob` and `TextEncoder` (every browser has them; the vm context of
  the test page is given them).
- Trap for the next session: a wide test run piped through `grep | tail` reported success while two tests failed (the
  pipeline's status was `tail`'s). Check `${PIPESTATUS[0]}` or write the log and grep it.

### 2026-10-31 12:30: handlers (`feat/wasmgc-handlers`)

- Closures: variables become bindings (a local, a local holding a cell, or a cell in the environment); any variable a
  lambda in the function names is boxed. The refactor touches every binding site (`let`, `for`, `try_let`, `set`,
  `append`, `map_set`, parameters); the differential corpus still passes.
- First run of a click handler that sets a captured counter matched the JavaScript backend exactly (`11` after the
  top level set it to 10). The test page now records listeners, clicks every element with a click listener
  (awaiting each handler), drives scroll, media and visibility, and reports unhandled rejections.
- `on_event` now calls one runtime function in both backends (`howlFrameOnEvent`); the table gives a lambda's
  parameters kinds, which the loader converts by.

### 2026-10-31 14:00: asynchronous operations (`feat/wasmgc-async`)

- Resumable frames, A-normal form first (plan: "Asynchronous operations"). First run of an asynchronous defun with a
  `while`, `sleep` in a `for` and a spawned task matched the JavaScript backend except the order of two timer races
  (5 ms against three 1 ms sleeps), which depends on wall time; fixtures avoid near races.
- Bugs found on the way, all before any commit was pushed: a `drop` inside an `if` (blocks do not see the outer
  operand stack); frames that could not save a `(ref null $Str)` local (print's accumulator); the frame local needing
  its own type; and, caught only by the full suite (ZIP fixtures), a `let` that put its name in scope before compiling
  its value, so `(let (x (+ x 10)) ...)` read the new, unset `x`. The last has a differential fixture (`let shadows
  its own value`), shown to fail without the fix.
- The desugarer makes `let` in value positions (`list_get` becomes `(let (__lgN xs) (list_get __lgN i))`), so the
  rewrite lifts a value-position `let`, renaming its variable.
- JSPI: V8 12.4 has only the pre-standard API (`WebAssembly.Function` with `suspending`/`promising`, behind
  `--experimental-wasm-stack-switching`); a micro-benchmark ran 100,000 suspensions in 52 ms.
- With handlers, frames and `time_now`, HowlScene HF's editor builds to WebAssembly (272 KB, 40 KB loader; 76 KB
  gzipped) and its site runtime too (46 KB and 17 KB; 18 KB gzipped). Both validate.
- The JavaScript backend's `to_int` of nil read "cannot convert object to int": bugs.md #71, its own PR.
