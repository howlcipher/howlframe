# Lowered HFIR ABI, phase 3c: if and control edges on the experimental bytecode path

## Why this slice

Phase 3b made `while` executable on `-compile-hfir-bc` and filled `ControlEdges` on that header only. `if` already emitted `JUMP_IF_FALSE` and `JUMP` from data edges, and `LowerAST` still left those edges empty. The production hosts already run `if`. This slice records the control successors that jump layout follows, and fails closed when they are missing or swapped.

This is not a CFG or SSA pass. `for` stays without control edges. #90 stays Partial.

## What landed

`LowerAST` gives `(if cond then)` and `(if cond then else)` named data edges and `ControlEdges` in that order: the test, the then-branch, and an optional else. `LowerToBytecode` follows those control edges and emits the existing `JUMP_IF_FALSE` and `JUMP` opcodes, with the same relative offsets as the AST bytecode compiler. A then-only `if` emits `JUMP_IF_FALSE` and no `JUMP`. An `if` whose control edges are missing, swapped, or not that sequence returns one `HFIR_BYTECODE_UNSUPPORTED` diagnostic and no `BCProgram`. There is no new opcode and no new capability. A `defun` body may contain `if`. `while` and `defun` keep the Phase 3a and Phase 3b behavior. `for` still lowers through its data edges and the existing `FOR_INIT` / `FOR_NEXT` opcodes, and its `ControlEdges` stay empty.

The model-adapter transport still has no control-edge field. Decoding an `if` derives the successors from the validated roles, which is the same order `LowerAST` writes. A payload that names `control_edges` is rejected. Kind `while` and kind `defun` stay outside the transport allow-list, so #88 stays closed. Localization still does not treat a model-supplied control edge as authority. An `if` relation is published only when the persisted successors match the condition, then, and optional else. The published roles stay `then` and `else`.

`tests/conformance/abi_v1/11_if.howl` is the shared fixture. `tools/difftest` runs it as `if_control`. The false branches must not print. The taken branches print `else`, `then`, `greater`, and `only`.

| Host | How this case reaches it |
| --- | --- |
| `hfir_bytecode` | `-compile-hfir-bc`, then `-run-bc`. Experimental path. |
| `bytecode` | `-compile-bc`, then `-run-bc`. Production AST bytecode. Canonical result. |
| `interpreter`, `go`, `javascript` | Still the AST. The fixture is already in their subset, so they are included. JavaScript rewrites the root to `web_app`. |

`internal/vm/hfir_equivalence_test.go` also compares the experimental artifact with AST bytecode on a then/else branch, a then-only branch, a nested branch, and a branch inside `defun`.

Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST. `sleep` still compiles on that flag and `-compile-hfir-bc` still rejects it without writing an artifact.

## What is still open

* `-compile-bc` still compiles the AST. One lowered graph for every host is still the rest of Phase 2.
* Control edges exist on `while` headers and `if` nodes. The graph is not SSA, and the verifier still does not check cycles.
* `for` does not grow control edges in this slice.
* Generated `write_file`, `mkdir`, and the other host effects that Phase 2 has not mediated still do not consult a grant. `env`, `exec`, `read_file`, and `fetch` do.
* Feasibility is still a Wasm-only set. This slice adds no Wasm opcode and does not execute Wasm.
* No module linker, no HFIR module graph, and no VM module opcode.

## What this does not do

No new opcode. No new capability. #102–#105 and #108 stay Done and are not reopened. #88 stays closed. #90 stays Partial. `write_file` and `mkdir` stay deferred. `for` is not packed into this slice.
