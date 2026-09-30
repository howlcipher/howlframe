# Lowered HFIR ABI, phase 2e: write_file and mkdir grants on Go and JavaScript

## Why this slice

Phase 2a put `(env)` behind `HOWLFRAME_ALLOW_CAPS`. Phase 2b did the same for `(exec)`. Phase 2c did the same for `(read_file)`. Phase 2d did the same for `(fetch)`. `(write_file path data)` on generated Go was still a direct `os.WriteFile`, and `(mkdir path)` was still a direct `os.MkdirAll`. JavaScript had neither form. The bytecode VM already denies `OpWriteFile` and `OpMkdir` before the filesystem call. The interpreter already denied a missing `filesystem` grant, then rejected the form as unsupported, so a granted write or mkdir there was not an honest filesystem effect. This slice makes the generated hosts check the grant, and makes the interpreter write or create the directory only after that same check.

Phase 2 as a whole is still one lowered graph for every host. This change does not take that step. #90 stays Partial.

## What landed

`(write_file path data)` and `(mkdir path)` in generated Go and JavaScript go through `howlFrameWriteFile` and `howlFrameMkdir`. The helpers read the runner grant `HOWLFRAME_ALLOW_CAPS` (comma-separated, the same names as `-allow-caps`). The grant name is `filesystem`, matching `capability.ForConstruct("write_file")`, `capability.ForConstruct("mkdir")`, `OpWriteFile`, and `OpMkdir`. It panics or throws `CAPABILITY_DENIED: capability denied: filesystem` when `filesystem` is absent, and only then would it write or create a directory. An empty grant, an unset grant, and a grant of some other capability all deny. The denial does not include the path, and the file is not written and the directory is not created.

A `filesystem` grant writes the file or creates the directory. Go's statement still discards the error from `os.WriteFile` or `os.MkdirAll`, which is the call the backend already emitted; the grant check panics before that return. JavaScript calls `writeFileSync` or `mkdirSync` after the check. The interpreter now uses `os.WriteFile` and `os.MkdirAll` after its existing `filesystem` check, so the conformance cases can include it. The bytecode VM is unchanged: `OpWriteFile` and `OpMkdir` are already `filesystem`, and the gate runs before the call. No new opcode and no new capability.

`tools/difftest` already passes `HOWLFRAME_ALLOW_CAPS` to generated Go and `node`. `tests/conformance/lowered_hfir_abi_v1.json` runs `write_file_denied`, `write_file_granted`, `mkdir_denied`, and `mkdir_granted` on the interpreter, the bytecode VM, Go, and JavaScript. The write fixture uses `/tmp/howlframe-abi-v1-phase2e-write.txt` and the mkdir fixture uses `/tmp/howlframe-abi-v1-phase2e-dir`, because generated Go does not share the caller's working directory. The marker is absent and the path is not created on denial. The marker is on disk, and the directory exists, when `filesystem` is granted.

The JavaScript checker accepts `write_file` and `mkdir` so a `web_app` body can host the same fixtures. Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST.

## What is still Phase 2

* `-compile-bc` still compiles the AST. `-compile-hfir-bc` is still the experimental lowerer.
* `defun`, `call`, `while`, `if`, and `for` are executable on `-compile-hfir-bc` only. They are not one lowered graph for every host.
* `ControlEdges` are still empty outside those experimental headers, so the graph is not SSA.
* Generated `spawn`, stores, and the other host effects still do not consult a grant. `env`, `exec`, `read_file`, `fetch`, `write_file`, and `mkdir` do.
* Feasibility is still a Wasm-only set. This slice adds no Wasm opcode and does not execute Wasm.
* No module linker, no HFIR module graph, and no VM module opcode.

## What this does not do

No new capability kind. No new opcode. #102–#105 and #108 stay Done and are not reopened. #88 stays closed. #90 stays Partial.
