# External Consumer Conformance Slice

## Scope

This is a deliberately bounded v0.1.1 evidence slice for the constructs an
external HowlBoard-style task board actually uses. It is not the lowered-HFIR
backend ABI or the complete conformance suite tracked by improvement #90.

## Proven behavior

| Composition | Backend | Executable evidence |
|---|---|---|
| `parse_json Any req.body` reads a route request body and produces dynamic object/list values usable with `map_get` | Standalone bytecode VM | `internal/vm.TestBytecodeParseJSONRequestBody` |
| `try_let` maps malformed route JSON to an explicit application response without state mutation | Standalone bytecode VM | `internal/vm.TestBytecodeParseJSONRequestBody`; `apps/task_api.TestTaskAPI/validation` |
| Create, list, complete, and delete preserve JSON dict/list values over independent HTTP requests | Standalone bytecode VM | `apps/task_api.TestTaskAPI` |
| Network and database capability grants remain independently enforced for the HTTP task application | Standalone bytecode VM | `apps/task_api.TestTaskAPICapabilities` |
| Generated JavaScript parses application-provided JSON expressions through `JSON.parse` | JavaScript generator | `internal/backend/javascript.TestGenerateJSTryLetParseJson` |
| Generated JavaScript emits the optional POST body used by the board before parsing the text response | JavaScript generator | `internal/backend/javascript.TestGenerateJSFetchWithBody` |

`req.body` is not claimed as JavaScript parity: a JavaScript `web_app` has no
inbound HTTP route request object. The Go HTTP backend also has a different
typed-decoder path, so this slice does not claim dynamic `Any` request-body
parity there.

HowlBoard uses `fetch` in its JavaScript frontend, so the generated POST-body
shape is covered here. This is generator evidence, not a claim that inbound
HTTP route requests have JavaScript parity.

## Limits and remaining #90 work

This slice does not define a lowered-HFIR ABI, place HFIR on the execution
path, compare common fixtures across Go, JavaScript, bytecode, interpreter,
and Wasm, establish error or capability equivalence across those targets, or
provide property-based differential testing. Those remain pending under
improvement #90.

The bytecode implementation reads `req.body` only through the existing
`parse_json` special case. It has no standalone `req_body` construct, no
streaming/body-size contract, and no general request input conformance claim.
