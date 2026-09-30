# Lowered HFIR ABI, phase 2c: read_file grants on Go and JavaScript

## Why this slice

Phase 2a put `(env)` behind `HOWLFRAME_ALLOW_CAPS`. Phase 2b did the same for `(exec)`. `(read_file path)` on generated Go was still a direct `os.ReadFile`, and JavaScript had no `read_file` form. The bytecode VM already denies `OpReadFile` before `os.ReadFile`. The interpreter already denied a missing `filesystem` grant, then rejected the form as unsupported, so a granted read there was not an honest file read. This slice makes the generated hosts check the grant, and makes the interpreter read only after that same check.

Phase 2 as a whole is still one lowered graph for every host. This change does not take that step. #90 stays Partial.

## What landed

`(read_file path)` in generated Go and JavaScript goes through `howlFrameReadFile`. The helper reads the runner grant `HOWLFRAME_ALLOW_CAPS` (comma-separated, the same names as `-allow-caps`). The grant name is `filesystem`, matching `capability.ForConstruct("read_file")` and `OpReadFile`. It panics or throws `CAPABILITY_DENIED: capability denied: filesystem` when `filesystem` is absent, and only then would it read. An empty grant, an unset grant, and a grant of some other capability all deny. The denial does not include the path, and the file is not opened.

A `filesystem` grant reads the file. Go's `try_let` still binds `(bytes, error)` from `howlFrameReadFile`, so a granted miss stays an IO error instead of a capability denial. `let` and `bytes_to_string` use `howlFrameReadFileBytes`, which returns one `[]byte` after that same check. JavaScript returns the UTF-8 text from `readFileSync` after the check. The interpreter now reads with `os.ReadFile` after its existing `filesystem` check and returns those bytes, so the conformance case can include it. The bytecode VM is unchanged: `OpReadFile` is already `filesystem`, and the gate runs before the read.

`tools/difftest` already passes `HOWLFRAME_ALLOW_CAPS` to generated Go and `node`. `tests/conformance/lowered_hfir_abi_v1.json` runs `read_file_denied` and `read_file_granted` on the interpreter, the bytecode VM, Go, and JavaScript. The fixture reads `/tmp/howlframe-abi-v1-phase2c.txt`. The conformance test writes `phase2c-read-marker` there first, because generated Go does not share the caller's working directory. The marker is absent on denial and printed when `filesystem` is granted.

The JavaScript checker accepts `read_file` so a `web_app` body can host the same fixture. Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST.

## What is still Phase 2

* `-compile-bc` still compiles the AST. `-compile-hfir-bc` is still the experimental lowerer.
* `defun`, `call`, and `while` are still not executable HFIR.
* `ControlEdges` are still empty, so the graph is not SSA.
* Generated `fetch`, `write_file`, `mkdir`, and the other host effects still do not consult a grant. `env`, `exec`, and `read_file` do. `write_file` and `mkdir` stayed out: JavaScript and the interpreter do not implement them, and folding them into this helper would be a second host effect.
* Feasibility is still a Wasm-only set. This slice adds no Wasm opcode and does not execute Wasm.
* No module linker, no HFIR module graph, and no VM module opcode.

## What this does not do

No new capability kind. No new opcode. #102–#105 and #108 stay Done and are not reopened. #90 stays Partial.
