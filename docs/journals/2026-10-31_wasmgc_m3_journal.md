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
| 1. Typed host operation table | `feat/host-operation-registry` | | Open |

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
