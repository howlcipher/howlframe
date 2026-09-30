# Lowered HFIR ABI, phase 3d: for and control edges on the experimental bytecode path

## Why this slice

Phase 3c made `if` follow `ControlEdges` on `-compile-hfir-bc`. `for` already emitted `FOR_INIT`, `FOR_NEXT`, and `JUMP` from data edges, and `LowerAST` still left those edges empty. The production hosts already run `for`. This slice records the control successors that jump layout follows, and fails closed when they are missing or swapped.

This is not a CFG or SSA pass. #90 stays Partial.

## What landed

`LowerAST` gives `(for item iterable body)` named data edges and `ControlEdges` in that order: the iterable, then the body. The iterator name stays on the node value. The back edge is the existing `JUMP` to `FOR_NEXT`. `LowerToBytecode` follows those control edges and emits the existing `FOR_INIT`, `FOR_NEXT`, and `JUMP` opcodes, with the same relative offsets as the AST bytecode compiler. A `for` whose control edges are missing, swapped, or not that pair returns one `HFIR_BYTECODE_UNSUPPORTED` diagnostic and no `BCProgram`. There is no new opcode and no new capability. A `defun` body may contain `for`. `while`, `if`, and `defun` keep the Phase 3a–3c behavior. `match`, `try`, and every other node still have empty `ControlEdges`.

The model-adapter transport still has no control-edge field. Kind `for` stays outside the transport allow-list, with `while` and `defun`, so #88 stays closed. Localization still does not treat a model-supplied control edge as authority. A `for` relation is published only when the persisted successors match the iterable and the body. The published roles are `iterable` and `body`.

`tests/conformance/abi_v1/13_for.howl` is the shared fixture. `tools/difftest` runs it as `for_control`. The empty list must not print. The taken loop prints `a`, `b`, and `c`.

| Host | How this case reaches it |
| --- | --- |
| `hfir_bytecode` | `-compile-hfir-bc`, then `-run-bc`. Experimental path. |
| `bytecode` | `-compile-bc`, then `-run-bc`. Production AST bytecode. Canonical result. |
| `interpreter`, `go`, `javascript` | Still the AST. The fixture is already in their subset, so they are included. JavaScript rewrites the root to `web_app`. |

`internal/vm/hfir_equivalence_test.go` also compares the experimental artifact with AST bytecode on a list iteration, an empty list, a nested loop, a loop inside `defun`, and a non-list iterable. The non-list iterable is `TYPE_ERROR` on both emitters, with empty stdout. The checker rejects that program before either compiler on the CLI; the comparison is the two bytecode paths.

Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST. `sleep` still compiles on that flag and `-compile-hfir-bc` still rejects it without writing an artifact.

## What is still open

* `-compile-bc` still compiles the AST. One lowered graph for every host is still the rest of Phase 2.
* Control edges exist on `while` headers, `if` nodes, and `for` headers. The graph is not SSA, and the verifier still does not check cycles.
* `match`, `try`, and the other control forms do not grow control edges in this slice.
* Generated `write_file`, `mkdir`, and the other host effects that Phase 2 has not mediated still do not consult a grant. `env`, `exec`, `read_file`, and `fetch` do.
* Feasibility is still a Wasm-only set. This slice adds no Wasm opcode and does not execute Wasm.
* No module linker, no HFIR module graph, and no VM module opcode.

## What this does not do

No new opcode. No new capability. #102–#105 and #108 stay Done and are not reopened. #88 stays closed. #90 stays Partial. `write_file` and `mkdir` stay deferred. Production `-compile-bc` is unchanged.
