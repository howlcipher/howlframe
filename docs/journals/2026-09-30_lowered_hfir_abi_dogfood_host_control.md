# Lowered HFIR ABI, dogfood: supported core, env, and read_file

## Why this slice

The experimental lowerer already emits the pure core (`let`, arithmetic, `if`, `dict`, `map_keys`, `map_get`, `list`, `str_join`, `is_nil`) and the host effects `env` and `read_file`. Those conformance cases still ran only the AST hosts. `write_file` and `mkdir` already had a nested control fixture. `env` and `read_file` did not. `exec` and `fetch` are still outside `LowerToBytecode`. This slice compares the two bytecode compilers on the forms the lowerer already accepts, and adds one program that nests `env` and `read_file` with `if`, `while`, `for`, and `defun`. It does not add an opcode. #90 stays Partial.

## What was compared

`hfir_bytecode` (`-compile-hfir-bc`, then `-run-bc`) is now a target on these existing cases. The canonical result is still production `-compile-bc`.

* `arith_if`, `map_keys`, `map_get_absence`, `map_keys_sort_bmp`, `map_keys_sort_nonbmp`, and `map_keys_type_error`. Empty grant. The sort cases keep UTF-8 byte order. The type-error case stays `TYPE_ERROR`.
* `env_denied` and `env_granted`. An empty grant is `CAPABILITY_DENIED` before the variable is read and does not include `phase1-token`. The `environment` grant prints that value.
* `read_file_denied` and `read_file_granted`. An empty grant is `CAPABILITY_DENIED` before the read and does not include the marker. The `filesystem` grant prints `phase2c-read-marker`. `bytes_to_string` is the existing `CONVERT` opcode.

`TestABIPropertyArithmetic` now includes `hfir_bytecode` on the same eight seeded `+`, `-`, and `*` expressions. The canonical integer is still the production bytecode VM.

`tests/conformance/abi_v1/18_nested_env_read.howl` defines `probe` and calls it twice.

* `(call probe 1)` runs a `while` whose body is a `for`. That `for` has an `if`/`else`. The else reads `HOWLFRAME_ABI_SECRET` and `/tmp/howlframe-abi-v1-hostread-read.txt`, then runs a `for` over the bound list `names`. The inner `for` has an `if`/`else`. A `for` over an empty list and a `while` whose condition is `false` sit on that same branch. Those three would read another key or path and print `no`.
* After the counting loop, an `if` reads `HOWLFRAME_ABI_SECRET` again when the count is positive. `(call probe 0)` skips the counting loop, prints `miss`, and reads `/tmp/howlframe-abi-v1-hostread-miss.txt`.

The taken lines are `phase1-token`, `dogfood-read-marker`, `L a 0`, `L b 0`, `kept 2`, `phase1-token`, `result 2`, `miss`, `dogfood-miss-marker`, and `empty 0`. Exit code is 0. Stderr is empty. The grant is `environment,filesystem`. With no grant, the first reached host effect is `env`. Both bytecode compilers deny with `CAPABILITY_DENIED` and `capability denied: environment` before any file is read. The denial text does not include the secret or the path. The `no` branches are absent from stdout, and their paths are absent from the filesystem, when both grants are present. The two marker files are not modified.

| Host | How this case reaches it |
| --- | --- |
| `hfir_bytecode` | `-compile-hfir-bc`, then `-run-bc`. Experimental path. |
| `bytecode` | `-compile-bc`, then `-run-bc`. Production AST bytecode. Canonical result. |
| `interpreter`, `go`, `javascript` | Still the AST. The fixture uses only forms those hosts already run, so they are included. JavaScript rewrites the root to `web_app`. |

`tools/difftest` runs `nested_env_read_denied` and `nested_env_read_granted` from `tests/conformance/lowered_hfir_abi_v1.json`. A stdout, stderr, exit, or error-class mismatch fails the case. The forbidden token fails it too.

`internal/vm/hfir_equivalence_test.go` reads the same file, rewrites each absolute path into a temp directory, round-trips both artifacts, and compares grant and denial. The test also checks the control-edge shape: one `defun`, two `call`s, two `while` headers, three `if`/`else` nodes, three `for` headers, five `env` nodes, and six `read_file` nodes. A `for` sits inside a `for` and inside a `while`. `env` and `read_file` sit inside the `defun`.

`exec` and `fetch` stay unsupported. `LowerToBytecode` on `06_exec_capability.howl` and `08_fetch_capability.howl` returns one `HFIR_BYTECODE_UNSUPPORTED` diagnostic and no `BCProgram`. Their conformance cases stay on the AST hosts. `tests/parity` stays off this pass: it includes forms the lowerer does not emit, including `html_escape` and `exec`.

## How to run it

CI runs `go test ./...` (`.github/workflows/ci.yml`), which includes both tests.

Focused commands:

```
go test ./tools/difftest -run 'TestLoweredHFIRABIConformance/nested_env_read|TestLoweredHFIRABIConformance/env_|TestLoweredHFIRABIConformance/read_file|TestLoweredHFIRABIConformance/arith_if|TestLoweredHFIRABIConformance/map_|TestABIPropertyArithmetic'
go test ./internal/vm -run 'TestHFIRBytecodeNestedEnvReadFixture|TestHFIRBytecodeExecAndFetchStayUnsupported'
```

Manual pair, from the repository root. The conformance test writes the two marker files first. Grant both capabilities:

```
go run howlframe.go -compile-bc tests/conformance/abi_v1/18_nested_env_read.howl -o /tmp/host-read-ast.hfbc
go run howlframe.go -run-bc -allow-caps environment,filesystem /tmp/host-read-ast.hfbc
go run howlframe.go -compile-hfir-bc tests/conformance/abi_v1/18_nested_env_read.howl -o /tmp/host-read-hfir.hfbc
go run howlframe.go -run-bc -allow-caps environment,filesystem /tmp/host-read-hfir.hfbc
```

## What this does not do

No new opcode. No new capability. Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST. Go, JavaScript, and the interpreter still consume the AST. `exec` and `fetch` are not lowered. `match` and `try` stay without control edges. No Wasm and no module linker. Matching outcomes on these fixtures does not mean `-compile-bc` consumes HFIR. #90 stays Partial. #102–#105 and #108 stay Done and are not reopened.
