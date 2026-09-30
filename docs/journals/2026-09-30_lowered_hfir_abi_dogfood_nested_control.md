# Lowered HFIR ABI, dogfood: nested if, while, and defun

## Why this slice

Phases 3a–3c made `defun`/`call`, `while`, and `if` executable on experimental `-compile-hfir-bc`, each with its own fixture. Those fixtures do not nest the three together. This slice compiles one program both ways and checks that the bytecode VM prints the same result. It does not change either compiler. #90 stays Partial.

## What was compared

`tests/conformance/abi_v1/12_nested_if_while_defun.howl` defines `scan` and calls it twice.

* `(call scan 5)` runs a `while` whose body has an `if`/`else` and a nested `if`/`else`, plus a then-only `if` that counts one hit. After the loop, an `if` runs a second `while`.
* `(call scan 0)` skips the counting loop and takes the other branch of that outer `if`.
* A `while` whose condition is `false` sits on both paths. It would print `no`. That token is absent from the live labels.

The taken lines are `low 0`, `low 1`, `mid 2`, `hit 3`, `mid 4`, `again 1`, `result 2`, `miss`, and `empty 0`. Exit code is 0. Stderr is empty. There is no intentional difference between the two bytecode paths.

| Host | How this case reaches it |
| --- | --- |
| `hfir_bytecode` | `-compile-hfir-bc`, then `-run-bc`. Experimental path. |
| `bytecode` | `-compile-bc`, then `-run-bc`. Production AST bytecode. Canonical result. |
| `interpreter`, `go`, `javascript` | Still the AST. The fixture uses only forms those hosts already run, so they are included. JavaScript rewrites the root to `web_app`. |

`tools/difftest` runs the case as `nested_if_while_defun` from `tests/conformance/lowered_hfir_abi_v1.json`. A stdout, stderr, or exit mismatch fails the case. The forbidden token `no` fails it too.

`internal/vm/hfir_equivalence_test.go` (`TestHFIRBytecodeNestedIfWhileDefunFixture`) reads the same file, checks the control-edge shape (one `defun` with empty control edges, two `call`s, three `while` headers, one then-only `if`, three `if`/`else` nodes, no `for`), round-trips both artifacts, and requires the outcomes to be equal, including stdout, stderr, exit, and VM error.

## How to run it

CI runs `go test ./...` (`.github/workflows/ci.yml`), which includes both tests.

Focused commands:

```
go test ./tools/difftest -run 'TestLoweredHFIRABIConformance/nested_if_while_defun'
go test ./internal/vm -run TestHFIRBytecodeNestedIfWhileDefunFixture
```

Manual pair, from the repository root:

```
go run howlframe.go -compile-bc tests/conformance/abi_v1/12_nested_if_while_defun.howl -o /tmp/nested-ast.hfbc
go run howlframe.go -run-bc /tmp/nested-ast.hfbc
go run howlframe.go -compile-hfir-bc tests/conformance/abi_v1/12_nested_if_while_defun.howl -o /tmp/nested-hfir.hfbc
go run howlframe.go -run-bc /tmp/nested-hfir.hfbc
```

## What this does not do

No new opcode. No new capability. Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST. `for` stays without control edges. `write_file` and `mkdir` stay deferred. No Wasm and no module linker. Matching stdout on this fixture does not mean `-compile-bc` consumes HFIR. #90 stays Partial.
