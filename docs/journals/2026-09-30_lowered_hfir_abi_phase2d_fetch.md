# Lowered HFIR ABI, phase 2d: fetch grants on Go and JavaScript

## Why this slice

Phase 2a put `(env)` behind `HOWLFRAME_ALLOW_CAPS`. Phase 2b did the same for `(exec)`. Phase 2c did the same for `(read_file)`. `(fetch url method)` on generated Go was still a direct `http.DefaultClient.Do`, and JavaScript called `fetch` with no grant check. The bytecode VM already denies `OpFetch` before `http.NewRequest`. The interpreter already denied a missing `network` grant, then rejected the form as unsupported, so a granted fetch there was not an honest request. This slice makes the generated hosts check the grant, and makes the interpreter send the request only after that same check.

Phase 2 as a whole is still one lowered graph for every host. This change does not take that step. #90 stays Partial.

## What landed

`(fetch url method [body])` in generated Go and JavaScript goes through `howlFrameFetch`. The helper reads the runner grant `HOWLFRAME_ALLOW_CAPS` (comma-separated, the same names as `-allow-caps`). The grant name is `network`, matching `capability.ForConstruct("fetch")` and `OpFetch`. It panics or throws `CAPABILITY_DENIED: capability denied: network` when `network` is absent, and only then would it build or send the request. An empty grant, an unset grant, and a grant of some other capability all deny. The denial does not include the URL, and no connection is opened.

A `network` grant performs the request. Go's `try_let` still binds `(bytes, error)` from `howlFrameFetch`, so a granted transport failure stays an IO error instead of a capability denial. `let` and `bytes_to_string` use `howlFrameFetchBytes`, which returns one `[]byte` after that same check. JavaScript returns the response text from `fetch` after the check. The interpreter now uses `http.DefaultClient.Do` after its existing `network` check and returns those bytes, so the conformance case can include it. The bytecode VM is unchanged: `OpFetch` is already `network`, and the gate runs before the request. The shared conformance form is `(fetch url method)` with no body. `OpFetch` still has no body operand.

`tools/difftest` already passes `HOWLFRAME_ALLOW_CAPS` to generated Go and `node`. `tests/conformance/lowered_hfir_abi_v1.json` runs `fetch_denied` and `fetch_granted` on the interpreter, the bytecode VM, Go, and JavaScript. The fixture requests `http://127.0.0.1:47653/howlframe-abi-v1-phase2d`. The conformance test serves `phase2d-fetch-marker` there first, because every host must use the same absolute URL. The marker is absent on denial, the server sees no request on denial, and the marker is printed when `network` is granted.

Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST.

## What is still Phase 2

* `-compile-bc` still compiles the AST. `-compile-hfir-bc` is still the experimental lowerer.
* `defun`, `call`, and `while` are still not executable HFIR.
* `ControlEdges` are still empty, so the graph is not SSA.
* Generated `write_file`, `mkdir`, and the other host effects still do not consult a grant. `env`, `exec`, `read_file`, and `fetch` do. `write_file` and `mkdir` stayed out: this slice is one host effect.
* Feasibility is still a Wasm-only set. This slice adds no Wasm opcode and does not execute Wasm.
* No module linker, no HFIR module graph, and no VM module opcode.

## What this does not do

No new capability kind. No new opcode. #102–#105 and #108 stay Done and are not reopened. #90 stays Partial.
