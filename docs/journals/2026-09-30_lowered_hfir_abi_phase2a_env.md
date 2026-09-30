# Lowered HFIR ABI, phase 2a: env grants on Go and JavaScript

## Why this slice

Phase 1 published `lowered-hfir-abi/v1` and showed that the interpreter and the bytecode VM deny `(env)` when the grant is empty. Generated Go still called `os.Getenv` with no grant check. JavaScript had no `env` form, so a denial there was not an honest `CAPABILITY_DENIED`. That is the hole this slice closes.

Phase 2 as a whole is still one lowered graph for every host. This change does not take that step.

## What landed

`(env key)` in generated Go and JavaScript goes through `howlFrameEnv`. The helper reads the runner grant `HOWLFRAME_ALLOW_CAPS` (comma-separated, the same names as `-allow-caps`). It panics or throws `CAPABILITY_DENIED: capability denied: environment` when `environment` is absent, and only then would it read the requested key. An empty grant, an unset grant, and a grant of some other capability all deny. The value is not written to stdout, stderr, or the Go `crash.json` error.

`tools/difftest` passes that grant to the generated Go binary and to `node`. `deny_all` sends an empty grant. `allow_caps` sends that list. The historical default, used by the parity corpus, remains every known capability. `tests/conformance/lowered_hfir_abi_v1.json` now runs `env_denied` and `env_granted` on Go and JavaScript as well as the interpreter and the bytecode VM.

The JavaScript checker accepts `env` so a `web_app` body can host the same fixture. Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST.

## What is still Phase 2

* `-compile-bc` still compiles the AST. `-compile-hfir-bc` is still the experimental lowerer.
* `defun`, `call`, and `while` are still not executable HFIR.
* `ControlEdges` are still empty, so the graph is not SSA.
* Generated `read_file`, `exec`, and the other host effects still do not consult a grant. Only `env` does.
* Feasibility is still a Wasm-only set of three host effects. This slice adds no Wasm opcode and does not execute Wasm.
* No module linker, no HFIR module graph, and no VM module opcode.

## What this does not do

No new capability kind. No new opcode. #102–#105 and #108 stay Done and are not reopened. #90 stays Partial.
