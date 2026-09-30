# Lowered HFIR ABI, dogfood: fetch grant and denial

## Why this slice

Phase 2d mediates `(fetch)` on the interpreter, the bytecode VM, generated Go, and JavaScript. Those conformance cases compiled with production `-compile-bc` only. Experimental `-compile-hfir-bc` still rejected `fetch` with `HFIR_BYTECODE_UNSUPPORTED`, so the two bytecode compilers were not compared on an HTTP request. This slice puts `fetch` on the experimental lowerer with the opcode the AST compiler already emits, then compares the two compilers on grant and denial. One program also nests that effect with `if`, `while`, `for`, and `defun`. #90 stays Partial.

## What was compared

`LowerAST` gives `(fetch url method)` a `url` edge and then a `method` edge. An optional third argument is a `body` edge. `LowerToBytecode` emits the existing `FETCH` opcode. The URL is compiled first, then the method. That is the same operand order as `bytecode.CompileToBytecode`. The VM pops the method and then the URL. `OpFetch` has no body operand. The AST compiler does not compile that child, and this lowerer does not either, so neither bytecode path sends a body. A node without the URL and method edges, or with the method before the URL, returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. The model-adapter transport still rejects `fetch`. There is no new opcode and no new capability. The grant name stays `network`.

`tests/conformance/abi_v1/08_fetch_capability.howl` now includes `hfir_bytecode` with the hosts Phase 2d already runs. An empty grant is `CAPABILITY_DENIED` before any HTTP request. The denial does not include the URL or `phase2d-fetch-marker`, and the fixture server sees no request. The `network` grant prints `phase2d-fetch-marker`.

`tests/conformance/abi_v1/20_nested_fetch.howl` defines `probe` and calls it twice.

* `(call probe 1)` runs a `while` whose body is a `for`. That `for` has an `if`/`else`. The else runs `(fetch "http://127.0.0.1:47653/howlframe-abi-v1-phase2d" "GET")` and a `for` over the bound list `names`. The inner `for` has an `if`/`else`. A `for` over an empty list and a `while` whose condition is `false` sit on that same branch. Those three would fetch and print `no`.
* After the counting loop, an `if` fetches again when the count is positive. `(call probe 0)` skips the counting loop and fetches on the miss branch.

The taken lines are `phase2d-fetch-marker`, `L a 0`, `L b 0`, `kept 2`, `phase2d-fetch-marker`, `result 2`, `miss`, `phase2d-fetch-marker`, and `empty 0`. Exit code is 0. Stderr is empty. With no grant, the first reached network effect is `fetch`. Both bytecode compilers deny with `CAPABILITY_DENIED` and `capability denied: network` before any request. The denial text does not include the URL or the marker. The `no` branches are absent from stdout when `network` is granted, and the fixture server sees a request only for each taken fetch.

A separate comparison runs `(fetch url "GET")`, `(fetch url "POST")`, and `(fetch url "PUT" "phase2d-body-not-sent")` against that same URL. Both compilers emit `FETCH` with URL then method. The body string is not an operand. The granted stdout is the marker three times, and the server records an empty body on every request.

| Host | How this case reaches it |
| --- | --- |
| `hfir_bytecode` | `-compile-hfir-bc`, then `-run-bc`. Experimental path. |
| `bytecode` | `-compile-bc`, then `-run-bc`. Production AST bytecode. Canonical result. |
| `interpreter`, `go`, `javascript` | Still the AST. The fixture uses only forms those hosts already run, so they are included. JavaScript rewrites the root to `web_app`. |

`tools/difftest` runs `fetch_denied`, `fetch_granted`, `nested_fetch_denied`, and `nested_fetch_granted` from `tests/conformance/lowered_hfir_abi_v1.json`. A stdout, stderr, exit, or error-class mismatch fails the case. The forbidden token fails it too. A denied case that opens a connection fails it, and a granted case that never reaches the fixture server fails it.

`internal/vm/hfir_equivalence_test.go` reads the same files, round-trips both artifacts, and compares the `FETCH` operand windows. The nested test also checks the control-edge shape: one `defun`, two `call`s, two `while` headers, three `if`/`else` nodes, three `for` headers, and seven `fetch` nodes. A `for` sits inside a `for` and inside a `while`. `fetch` sits inside the `defun`. The two outcomes must match, including a denial that sends no request.

## How to run it

CI runs `go test ./...` (`.github/workflows/ci.yml`), which includes both tests.

Focused commands:

```
go test ./tools/difftest -run 'TestLoweredHFIRABIConformance/fetch_|TestLoweredHFIRABIConformance/nested_fetch'
go test ./internal/vm -run 'TestHFIRBytecodeFetchFixture|TestHFIRBytecodeFetchOperandOrder|TestHFIRBytecodeNestedFetchFixture'
go test ./internal/hfir -run 'TestLowerToBytecodeFetch'
```

Manual pair, from the repository root, with `-allow-caps network` for the grant and without it for the denial. The fixture URL is `http://127.0.0.1:47653/howlframe-abi-v1-phase2d`, and something on that address must answer `phase2d-fetch-marker` before the granted run:

```
go run howlframe.go -compile-bc tests/conformance/abi_v1/20_nested_fetch.howl -o /tmp/fetch-ast.hfbc
go run howlframe.go -run-bc -allow-caps network /tmp/fetch-ast.hfbc
go run howlframe.go -compile-hfir-bc tests/conformance/abi_v1/20_nested_fetch.howl -o /tmp/fetch-hfir.hfbc
go run howlframe.go -run-bc -allow-caps network /tmp/fetch-hfir.hfbc
```

## What this does not do

No new opcode. No new capability. Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST. Go, JavaScript, and the interpreter still consume the AST. The model-adapter transport still rejects `fetch`. The optional request body stays off `OpFetch`: the interpreter, Go, and JavaScript still send it, and both bytecode compilers still drop it. `match` and `try` stay without control edges. No Wasm and no module linker. `exec` stays `HFIR_TARGET_INFEASIBLE` for Wasm. `fetch` is not added to that set. Matching outcomes on these fixtures does not mean `-compile-bc` consumes HFIR. #90 stays Partial.
