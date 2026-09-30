# Lowered HFIR ABI, phase 1

## Why this slice

Improvement #90 asks for one lowered contract so the interpreter, the bytecode VM, Go, JavaScript, and eventually Wasm mean the same thing. The production compiler does not consume HFIR. `runHFIRGate` verifies a graph, then `-compile-bc` calls `bytecode.CompileToBytecode` on the AST. `-compile-hfir-bc` is still the experimental lowerer. `LowerAST` still leaves `ControlEdges` empty.

Shipping "HFIR owns execution" in this change would be a false Done. Phase 1 publishes the contract and proves the hosts that already run the core agree on it.

## What landed

`docs/reference/lowered_hfir_abi_v1.md` is version `lowered-hfir-abi/v1` (`hfir.LoweredABIV1`). It states typed values, the call shape the hosts already share, the absence of a linear-memory import, error classes, pure effects versus `env`, and the Wasm feasibility set.

The Wasm set is exactly `exec`, `spawn_agent`, and `http_server_start`, rejected with `HFIR_TARGET_INFEASIBLE`. `isFeasible` reads `hfir.WasmInfeasibleKinds`. Other targets stay permissive. No Wasm opcode was added.

`tests/conformance/lowered_hfir_abi_v1.json` is the suite. `tools/difftest` already compared bytecode, the interpreter, and Go, and it already knew how to run JavaScript for a `web_app`. The suite extends that harness. A `cli_app` body is rewritten to `web_app` only for the JavaScript host. Empty grants are explicit. Passing cases match normalized stdout. Rejections match one error class, exit nonzero, and print nothing.

The core cases are exact integer arithmetic and `if`, `map_keys` including the #109 UTF-8 byte-order fixtures, `map_get` absence (#103), a dynamic `map_keys` `TYPE_ERROR` (#108), and `env` denial plus `env` granted. `map_keys` and `map_get` run with an empty grant (#107). A seeded property check generates eight arithmetic expressions and requires the same integer on every host. `defun`, `call`, and `while` stay in the existing `tests/parity` corpus for the AST hosts; `LowerToBytecode` still rejects them with `HFIR_BYTECODE_UNSUPPORTED`.

## Phase 2

One lowered graph is the source for every host.

* `-compile-bc` still compiles the AST.
* Calls and `while` are not executable HFIR.
* Control edges are still empty, so the graph is not SSA.
* Generated Go reads `env` with `os.Getenv` and does not deny a missing grant. JavaScript does not mediate `env`. The denial case therefore lists only the interpreter and the bytecode VM.
* Feasibility is still a Wasm-only set of three host effects.
* Non-exact `/` is not one rule.
* #73 and #84 stay behind this ABI. This slice does not grow them.

## What this does not do

No new opcode. No language surface. No module linker, HFIR module graph, or VM module opcode. No Factory or supervisor work. #102–#109 are not reopened. Wasm SSA collections are not started.
