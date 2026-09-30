# Lowered HFIR ABI, dogfood: exec grant and denial

## Why this slice

Phase 2b mediates `(exec)` on the interpreter, the bytecode VM, generated Go, and JavaScript. Those conformance cases compiled with production `-compile-bc` only. Experimental `-compile-hfir-bc` still rejected `exec` with `HFIR_BYTECODE_UNSUPPORTED`, so the two bytecode compilers were not compared on a process spawn. This slice puts `exec` on the experimental lowerer with the opcode the AST compiler already emits, then compares the two compilers on grant and denial. One program also nests that effect with `if`, `while`, `for`, and `defun`. #90 stays Partial.

## What was compared

`LowerAST` gives `(exec cmd args...)` a `cmd` edge and then one `arg` edge per argument, in source order. `LowerToBytecode` emits the existing `EXEC` opcode. The command is compiled first, then each argument, and `IntOperand` is the argument count. That is the same operand order as `bytecode.CompileToBytecode`. The VM pops the arguments and then the command. A node without the command edge, or with an argument before the command, returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. The model-adapter transport still rejects `exec`. There is no new opcode and no new capability. The grant name stays `process`. `fetch` stays outside the lowerer.

`tests/conformance/abi_v1/06_exec_capability.howl` now includes `hfir_bytecode` with the hosts Phase 2b already runs. An empty grant is `CAPABILITY_DENIED` before any subprocess starts. The denial does not include `printf` or `phase2b-exec-marker`. The `process` grant prints `phase2b-exec-marker`. The command is not a shell pipeline.

`tests/conformance/abi_v1/19_nested_exec.howl` defines `probe` and calls it twice.

* `(call probe 1)` runs a `while` whose body is a `for`. That `for` has an `if`/`else`. The else runs `(exec "printf" "phase2b-exec-marker")` and a `for` over the bound list `names`. The inner `for` has an `if`/`else`. A `for` over an empty list and a `while` whose condition is `false` sit on that same branch. Those three would run `printf` and print `no`.
* After the counting loop, an `if` runs `(exec "printf" "phase2b-exec-kept")` when the count is positive. `(call probe 0)` skips the counting loop and runs `(exec "printf" "phase2b-exec-miss")`.

The taken lines are `phase2b-exec-marker`, `L a 0`, `L b 0`, `kept 2`, `phase2b-exec-kept`, `result 2`, `miss`, `phase2b-exec-miss`, and `empty 0`. Exit code is 0. Stderr is empty. With no grant, the first reached process effect is `exec`. Both bytecode compilers deny with `CAPABILITY_DENIED` and `capability denied: process` before any subprocess starts. The denial text does not include the command or the marker. The `no` branches are absent from stdout when `process` is granted.

A separate comparison runs `(exec "true")`, `(exec "printf" "only")`, and `(exec "printf" "%s:%s" "left" "right")`. Both compilers emit `EXEC` with argument counts 0, 1, and 3, and the granted stdout is a blank line, `only`, and `left:right`.

| Host | How this case reaches it |
| --- | --- |
| `hfir_bytecode` | `-compile-hfir-bc`, then `-run-bc`. Experimental path. |
| `bytecode` | `-compile-bc`, then `-run-bc`. Production AST bytecode. Canonical result. |
| `interpreter`, `go`, `javascript` | Still the AST. The fixture uses only forms those hosts already run, so they are included. JavaScript rewrites the root to `web_app`. |

`tools/difftest` runs `exec_denied`, `exec_granted`, `nested_exec_denied`, and `nested_exec_granted` from `tests/conformance/lowered_hfir_abi_v1.json`. A stdout, stderr, exit, or error-class mismatch fails the case. The forbidden token fails it too.

`internal/vm/hfir_equivalence_test.go` reads the same files, round-trips both artifacts, and compares the `EXEC` operand windows. The nested test also checks the control-edge shape: one `defun`, two `call`s, two `while` headers, three `if`/`else` nodes, three `for` headers, and seven `exec` nodes. A `for` sits inside a `for` and inside a `while`. `exec` sits inside the `defun`. The two outcomes must match, including a denial that starts no subprocess.

## How to run it

CI runs `go test ./...` (`.github/workflows/ci.yml`), which includes both tests.

Focused commands:

```
go test ./tools/difftest -run 'TestLoweredHFIRABIConformance/exec_|TestLoweredHFIRABIConformance/nested_exec'
go test ./internal/vm -run 'TestHFIRBytecodeExecFixture|TestHFIRBytecodeExecOperandOrder|TestHFIRBytecodeNestedExecFixture|TestHFIRBytecodeFetchStaysUnsupported'
go test ./internal/hfir -run 'TestLowerToBytecodeExec'
```

Manual pair, from the repository root, with `-allow-caps process` for the grant and without it for the denial:

```
go run howlframe.go -compile-bc tests/conformance/abi_v1/19_nested_exec.howl -o /tmp/exec-ast.hfbc
go run howlframe.go -run-bc -allow-caps process /tmp/exec-ast.hfbc
go run howlframe.go -compile-hfir-bc tests/conformance/abi_v1/19_nested_exec.howl -o /tmp/exec-hfir.hfbc
go run howlframe.go -run-bc -allow-caps process /tmp/exec-hfir.hfbc
```

## What this does not do

No new opcode. No new capability. Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST. Go, JavaScript, and the interpreter still consume the AST. The model-adapter transport still rejects `exec`. `fetch` is not lowered. `match` and `try` stay without control edges. No Wasm and no module linker. `exec` stays `HFIR_TARGET_INFEASIBLE` for Wasm. Matching outcomes on these fixtures does not mean `-compile-bc` consumes HFIR. #90 stays Partial.
