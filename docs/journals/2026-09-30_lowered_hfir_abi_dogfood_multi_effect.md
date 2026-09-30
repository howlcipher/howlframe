# Lowered HFIR ABI, dogfood: mixed host effects and supported parity

## Why this slice

Each host effect already had its own grant, denial, and nested control fixture. `env` and `read_file` shared one nest. `write_file` and `mkdir` shared another. `exec` and `fetch` each had a nest, and they never ran in the same program. A grant that lets one effect succeed and then denies a later effect was not compared. The supported files in `tests/parity` still ran only the AST hosts. #90 stays Partial.

## What was compared

Every case already in `tests/conformance/lowered_hfir_abi_v1.json` already listed `hfir_bytecode`. This slice does not retag those cases. It adds `tests/conformance/abi_v1/21_nested_multi_effect.howl`.

That program defines `probe` and calls it twice. The taken path, the kept path, the miss path, and the untaken branches each run `env`, `read_file`, `write_file`, `mkdir`, `exec`, and `fetch`, in that order, inside `if`, `while`, `for`, and `defun`. A `for` sits inside a `for` and inside a `while`. One untaken `fetch` is `(fetch url "PUT" "phase2d-body-not-sent")`. That call has a `body` edge on the graph. `OpFetch` still has no body operand. The AST bytecode compiler does not compile that child, and `LowerToBytecode` does not either, so neither artifact contains the body string. The interpreter, Go, and JavaScript would send a body if that branch ran. It does not run.

Three grants:

* Empty. The first reached effect is `env`. Every host rejects with `CAPABILITY_DENIED` and `capability denied: environment` before a file is read, a path is created, a process starts, or an HTTP request is sent. Stdout is empty. The denial does not include `phase1-token`, the URL, or the body string.
* `environment,filesystem,process`, omitting `network`. The first site reads `HOWLFRAME_ABI_SECRET`, reads the marker file, writes `multi-write-marker`, creates the directory, and runs `printf`. The following `fetch` is `CAPABILITY_DENIED` with `capability denied: network`. Stdout is `phase1-token`, `multi-read-marker`, and `phase2b-exec-marker`. The later `kept` and `miss` sites do not run. The fixture server sees no request and no body.
* `environment,filesystem,process,network`. The taken lines are `phase1-token`, `multi-read-marker`, `phase2b-exec-marker`, `phase2d-fetch-marker`, `L a 0`, `L b 0`, `kept 2`, `phase1-token`, `multi-kept-read`, `phase2b-exec-kept`, `phase2d-fetch-marker`, `result 2`, `miss`, `phase1-token`, `multi-miss-read`, `phase2b-exec-miss`, `phase2d-fetch-marker`, and `empty 0`. The three taken writes and directories exist. The untaken paths do not. Each successful host sends three requests, and every body is empty.

| Host | How this case reaches it |
| --- | --- |
| `hfir_bytecode` | `-compile-hfir-bc`, then `-run-bc`. Experimental path. |
| `bytecode` | `-compile-bc`, then `-run-bc`. Production AST bytecode. Canonical result. |
| `interpreter`, `go`, `javascript` | Still the AST. The fixture uses only forms those hosts already run, so they are included. JavaScript rewrites the root to `web_app`. |

A rejection case whose manifest stdout is empty must still print nothing. `nested_multi_effect_partial` records the three lines printed before the network denial, and each host must print those lines.

`TestHFIRBytecodeSupportedParity` runs the `tests/parity` files the lowerer already emits through `hfir_bytecode`, production `-compile-bc`, the interpreter, and Go. JavaScript stays off that corpus; `TestParityCorpus` never included it. The files are `01_primitives`, `02_variables`, `03_operators`, `04_conversions`, `05_collections`, `06_control_flow`, `08_io_cli`, `09_boundary_values`, `10_governed_policy`, `11_error_undefined_var`, and `12_error_div_zero`. `08_io_cli` uses `try_let`, `write_file`, `read_file`, `exec`, and `stderr`. `try` still has no control edges. `11_error_undefined_var` fails in the checker before either compiler emits bytecode, so that file is a shared compile failure, not a lowering comparison. `12_error_div_zero` is `DIVISION_BY_ZERO` on every host in the run.

`07_strings.howl` (`regex_match`) and `13_html_escape.howl` (`html_escape`, `attr_escape`) are not lowered. `LowerToBytecode` returns one `HFIR_BYTECODE_UNSUPPORTED` diagnostic and no `BCProgram`. `-compile-hfir-bc` fails closed. Production `-compile-bc` still runs them. A new file in `tests/parity` has to be classified as one or the other.

`internal/vm/hfir_equivalence_test.go` reads the multi-effect file, rewrites each absolute path into a temp directory, round-trips both artifacts, and compares the `EXEC` and `FETCH` operand windows. The graph has one `defun`, two `call`s, two `while` headers, three `if`/`else` nodes, three `for` headers, and seven of each host effect. One `fetch` has the body edge. The two outcomes match on the empty grant, the partial grant, and the full grant, including the filesystem snapshot and a denial that sends no request.

## How to run it

CI runs `go test ./...` (`.github/workflows/ci.yml`), which includes these tests.

Focused commands:

```
go test ./tools/difftest -run 'TestLoweredHFIRABIConformance/nested_multi_effect|TestHFIRBytecodeSupportedParity|TestHFIRBytecodeRejectsUnsupportedParity'
go test ./internal/vm -run TestHFIRBytecodeNestedMultiEffectFixture
```

Manual pair, from the repository root. The conformance test writes the read markers and serves `http://127.0.0.1:47653/howlframe-abi-v1-phase2d` before the run. Grant all four capabilities, or omit `network` for the partial denial:

```
go run howlframe.go -compile-bc tests/conformance/abi_v1/21_nested_multi_effect.howl -o /tmp/multi-ast.hfbc
go run howlframe.go -run-bc -allow-caps environment,filesystem,process,network /tmp/multi-ast.hfbc
go run howlframe.go -compile-hfir-bc tests/conformance/abi_v1/21_nested_multi_effect.howl -o /tmp/multi-hfir.hfbc
go run howlframe.go -run-bc -allow-caps environment,filesystem,process,network /tmp/multi-hfir.hfbc
```

## What this does not do

No new opcode. No new capability. Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST. Go, JavaScript, and the interpreter still consume the AST. The model-adapter transport still rejects `defun`, `while`, `for`, `write_file`, `mkdir`, `exec`, and `fetch`. `match` and `try` stay without control edges. `spawn` and `spawn_agent` stay on the AST opcodes and are not lowered here. `req_query`, `req_header`, `req_path`, and `cli_args` are not in this dogfood. The optional request body stays off `OpFetch`. No Wasm and no module linker. Matching outcomes on these fixtures does not mean `-compile-bc` consumes HFIR. #90 stays Partial.
