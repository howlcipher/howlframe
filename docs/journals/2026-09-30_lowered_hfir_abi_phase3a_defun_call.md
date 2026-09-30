# Lowered HFIR ABI, phase 3a: defun and call on the experimental bytecode path

## Why this slice

The experimental lowerer could already emit bytecode for `let`, `if`, `map_keys`, `env`, and the other forms in `internal/hfir/bytecode.go`. `defun`, `call`, and `while` still failed closed with `HFIR_BYTECODE_UNSUPPORTED`. The production hosts already run `defun` and `call`. This slice makes the experimental path run that pair, and only that pair.

Phase 2 is still one lowered graph for every host. This change does not take that step. #90 stays Partial.

## What landed

`LowerAST` now gives a `defun` a name, ordered `param` edges, and `body` edges. `type_hint`, `type_hints`, and `type_param` are erased, and so is a return-type symbol sitting between the parameter list and the body. That matches the AST bytecode compiler, which emits nothing for those annotations. `call` stores the callee name and ordered `arg` edges. `return` has an optional `value`.

`LowerToBytecode` registers a `BCFunction` and emits the existing `CALL` and `RETURN` opcodes. A definition adds no instruction to the caller. There is no new opcode and no new capability. `ControlEdges` stay empty. `while` is still outside the subset: the lowerer returns one `HFIR_BYTECODE_UNSUPPORTED` diagnostic and no `BCProgram`. A `defun` whose body contains `while` fails the same way. The model-adapter transport still rejects kind `defun`, so #88 stays closed.

`tests/conformance/abi_v1/09_defun_call.howl` is the shared fixture. `tools/difftest` runs it as `defun_call`.

| Host | How this case reaches it |
| --- | --- |
| `hfir_bytecode` | `-compile-hfir-bc`, then `-run-bc`. Experimental path. |
| `bytecode` | `-compile-bc`, then `-run-bc`. Production AST bytecode. Canonical result. |
| `interpreter`, `go`, `javascript` | Still the AST. The fixture is already in their subset, so they are included. JavaScript rewrites the root to `web_app`. |

All of those hosts print `42` and `phase3a`. `internal/vm/hfir_equivalence_test.go` also compares the experimental artifact with AST bytecode on a forward call, a typed parameter list, recursion, and an arity rejection.

Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST. A `while` program still compiles and runs on that flag, and `-compile-hfir-bc` still rejects it without writing an artifact.

## What is still open

* `-compile-bc` still compiles the AST. One lowered graph for every host is still the rest of Phase 2.
* `while` is still not executable HFIR. This slice is not a CFG or SSA pass.
* `ControlEdges` are still empty.
* Generated `write_file`, `mkdir`, and the other host effects that Phase 2 has not mediated still do not consult a grant. `env`, `exec`, `read_file`, and `fetch` do.
* Feasibility is still a Wasm-only set. This slice adds no Wasm opcode and does not execute Wasm.
* No module linker, no HFIR module graph, and no VM module opcode.

## What this does not do

No new opcode. No new capability. #102–#105 and #108 stay Done and are not reopened. #88 stays closed. #90 stays Partial. `write_file` and `mkdir` stay deferred.
