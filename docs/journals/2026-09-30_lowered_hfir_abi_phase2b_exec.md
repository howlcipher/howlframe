# Lowered HFIR ABI, phase 2b: exec grants on Go and JavaScript

## Why this slice

Phase 2a put `(env)` on generated Go and JavaScript behind `HOWLFRAME_ALLOW_CAPS`. `(exec …)` was still a direct spawn: Go called `exec.Command` with no grant check, and JavaScript had no `exec` form. The interpreter already denied a missing `process` grant before it could run anything, and the bytecode VM denies `OpExec` before `CombinedOutput`. This slice makes the generated hosts do that too.

Phase 2 as a whole is still one lowered graph for every host. This change does not take that step. #90 stays Partial.

## What landed

`(exec cmd args...)` in generated Go and JavaScript goes through `howlFrameExec`. The helper reads the runner grant `HOWLFRAME_ALLOW_CAPS` (comma-separated, the same names as `-allow-caps`). The grant name is `process`, matching `capability.ForConstruct("exec")` and `OpExec`. It panics or throws `CAPABILITY_DENIED: capability denied: process` when `process` is absent, and only then would it spawn. An empty grant, an unset grant, and a grant of some other capability all deny. The denial does not include the command, and the command does not run.

A `process` grant runs the command with no shell. Go uses `exec.Command(...).CombinedOutput()`, the same call as the bytecode VM. JavaScript uses `spawnSync`. The captured output is what `(print (bytes_to_string ...))` shows. The interpreter now runs the same command after its existing `process` check, so the conformance case can include it.

`tools/difftest` already passes `HOWLFRAME_ALLOW_CAPS` to generated Go and `node`. `tests/conformance/lowered_hfir_abi_v1.json` runs `exec_denied` and `exec_granted` on the interpreter, the bytecode VM, Go, and JavaScript. The marker `phase2b-exec-marker` is absent on denial and printed when `process` is granted.

The JavaScript checker accepts `exec` so a `web_app` body can host the same fixture. Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST.

## What is still Phase 2

* `-compile-bc` still compiles the AST. `-compile-hfir-bc` is still the experimental lowerer, and `exec` is still outside that lowerer's executable subset.
* `defun`, `call`, and `while` are still not executable HFIR.
* `ControlEdges` are still empty, so the graph is not SSA.
* Generated `read_file`, `fetch`, `write_file`, `mkdir`, and the other host effects still do not consult a grant. `env` and `exec` do.
* Feasibility is still a Wasm-only set. `exec` stays `HFIR_TARGET_INFEASIBLE` for Wasm. This slice adds no Wasm opcode and does not execute Wasm.
* No module linker, no HFIR module graph, and no VM module opcode.

## What this does not do

No new capability kind. No new opcode. #102–#105 and #108 stay Done and are not reopened. #90 stays Partial.
