# Lowered HFIR ABI, phase 3b: while and control edges on the experimental bytecode path

## Why this slice

Phase 3a made `defun`, `call`, and `return` executable on `-compile-hfir-bc`. `while` still failed closed with `HFIR_BYTECODE_UNSUPPORTED`, and `LowerAST` still left `ControlEdges` empty. The production hosts already run `while`. This slice makes the experimental path run that loop, and records the two control successors the jump layout follows.

This is not a CFG or SSA pass. #90 stays Partial.

## What landed

`LowerAST` gives `(while cond body)` a condition data edge, a body data edge, and `ControlEdges` in that order: the test, then the body. The back edge is the existing `JUMP` back to the test. `if`, `for`, `defun`, and every other node stay without control edges.

`LowerToBytecode` follows those control edges and emits the existing `JUMP_IF_FALSE` and `JUMP` opcodes, with the same relative offsets as the AST bytecode compiler. A `while` whose control edges are missing, swapped, or not the condition and the body returns one `HFIR_BYTECODE_UNSUPPORTED` diagnostic and no `BCProgram`. There is no new opcode and no new capability. A `defun` body may contain `while`. The model-adapter transport still rejects kind `while`, so #88 stays closed. Localization still does not treat a model-supplied control edge as authority.

`tests/conformance/abi_v1/10_while.howl` is the shared fixture. `tools/difftest` runs it as `while_control`. The false loop must not print. The counting loop prints `1`, `2`, and `3`.

| Host | How this case reaches it |
| --- | --- |
| `hfir_bytecode` | `-compile-hfir-bc`, then `-run-bc`. Experimental path. |
| `bytecode` | `-compile-bc`, then `-run-bc`. Production AST bytecode. Canonical result. |
| `interpreter`, `go`, `javascript` | Still the AST. The fixture is already in their subset, so they are included. JavaScript rewrites the root to `web_app`. |

`internal/vm/hfir_equivalence_test.go` also compares the experimental artifact with AST bytecode on a counting loop, a loop that does not enter, a nested loop, a loop inside `defun`, and a non-bool condition. The non-bool condition is `TYPE_ERROR` on both emitters, with empty stdout. The checker rejects that program before either compiler on the CLI; the comparison is the two bytecode paths.

Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST. `sleep` still compiles on that flag and `-compile-hfir-bc` still rejects it without writing an artifact.

## What is still open

* `-compile-bc` still compiles the AST. One lowered graph for every host is still the rest of Phase 2.
* Control edges exist on `while` headers only. The graph is not SSA, and the verifier still does not check cycles.
* `if` and `for` do not grow control edges in this slice.
* Generated `write_file`, `mkdir`, and the other host effects that Phase 2 has not mediated still do not consult a grant. `env`, `exec`, `read_file`, and `fetch` do.
* Feasibility is still a Wasm-only set. This slice adds no Wasm opcode and does not execute Wasm.
* No module linker, no HFIR module graph, and no VM module opcode.

## What this does not do

No new opcode. No new capability. #102–#105 and #108 stay Done and are not reopened. #88 stays closed. #90 stays Partial. `write_file` and `mkdir` stay deferred.
