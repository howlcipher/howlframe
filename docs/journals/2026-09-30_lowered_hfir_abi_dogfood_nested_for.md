# Lowered HFIR ABI, dogfood: for nested with if, while, and defun

## Why this slice

Phase 3d made `for` executable on experimental `-compile-hfir-bc`, and the fixture `13_for.howl` covers one empty list and one flat list. The earlier dogfood fixture nests `if`, `while`, and `defun` and does not contain `for`. `internal/vm/hfir_equivalence_test.go` already compares a nested `for` and a `for` inside `defun` as separate snippets. This slice compiles one program that combines those forms and checks that the bytecode VM prints the same result. It does not change either compiler. #90 stays Partial.

## What was compared

`tests/conformance/abi_v1/14_nested_for_if_while_defun.howl` defines `walk` and calls it twice.

* `(call walk 2)` runs a `while` whose body has a `for`. That `for` has an `if`/`else`. The else is another `for`, over the bound list `cells`. The inner `for` has an `if`/`else`, a then-only `if`, and a `while`. A `while` whose condition is `false` sits on that outer `for` and would print `no`.
* After the counting loop, an `if` runs a `for` when the count is positive and prints `miss` when it is zero. `(call walk 0)` takes `miss`.
* An empty `for` sits on both paths. It would print `no`. That token is absent from the live labels. The `skip` and `z` branches would also print `no`, and neither value is in the lists.

The taken lines are `L a 0`, `step a`, `L b 0`, `step b`, `L a 1`, `step a`, `L b 1`, `step b`, `kept 4`, `result 4`, `miss`, and `empty 0`. Exit code is 0. Stderr is empty. There is no intentional difference between the two bytecode paths.

The lists in this fixture are flat strings. Generated Go lowers a list literal to `[]string`, so a list of lists is outside that host. Nested iteration where the inner iterable is the outer element stays on `TestHFIRBytecodeEquivalence` (`nested for`), which already requires the two bytecode emitters to match. This fixture's inner `for` reads the bound name `cells`, which those hosts already range.

| Host | How this case reaches it |
| --- | --- |
| `hfir_bytecode` | `-compile-hfir-bc`, then `-run-bc`. Experimental path. |
| `bytecode` | `-compile-bc`, then `-run-bc`. Production AST bytecode. Canonical result. |
| `interpreter`, `go`, `javascript` | Still the AST. The fixture uses only forms those hosts already run, so they are included. JavaScript rewrites the root to `web_app`. |

`tools/difftest` runs the case as `nested_for_if_while_defun` from `tests/conformance/lowered_hfir_abi_v1.json`. A stdout, stderr, or exit mismatch fails the case. The forbidden token `no` fails it too.

`internal/vm/hfir_equivalence_test.go` (`TestHFIRBytecodeNestedForIfWhileDefunFixture`) reads the same file, checks the control-edge shape (one `defun` with empty control edges, two `call`s, three `while` headers, one then-only `if`, three `if`/`else` nodes, four `for` headers), checks that a `for` sits inside a `for`, a `while`, an `if`, and the `defun`, round-trips both artifacts, and requires the outcomes to be equal, including stdout, stderr, exit, and VM error.

## How to run it

CI runs `go test ./...` (`.github/workflows/ci.yml`), which includes both tests.

Focused commands:

```
go test ./tools/difftest -run 'TestLoweredHFIRABIConformance/nested_for_if_while_defun'
go test ./internal/vm -run TestHFIRBytecodeNestedForIfWhileDefunFixture
```

Manual pair, from the repository root:

```
go run howlframe.go -compile-bc tests/conformance/abi_v1/14_nested_for_if_while_defun.howl -o /tmp/nested-for-ast.hfbc
go run howlframe.go -run-bc /tmp/nested-for-ast.hfbc
go run howlframe.go -compile-hfir-bc tests/conformance/abi_v1/14_nested_for_if_while_defun.howl -o /tmp/nested-for-hfir.hfbc
go run howlframe.go -run-bc /tmp/nested-for-hfir.hfbc
```

## What this does not do

No new opcode. No new capability. Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST. `match` and `try` stay without control edges. `write_file` and `mkdir` stay deferred. No Wasm and no module linker. Matching stdout on this fixture does not mean `-compile-bc` consumes HFIR. #90 stays Partial.
