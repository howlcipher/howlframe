# Lowered HFIR backend ABI v1

Version: `lowered-hfir-abi/v1`

This is the contract a backend must meet for the deterministic core, and the shape a later lowering has to grow into. It is not a claim that HFIR owns execution. Production compilation is still the checked AST: `runHFIRGate`, then `bytecode.CompileToBytecode` (`-compile-bc`). `-compile-hfir-bc` remains the experimental research lowerer. Go, JavaScript, and the interpreter still consume the AST.

The executable suite is `tests/conformance/lowered_hfir_abi_v1.json`, run by `tools/difftest`. The constant `hfir.LoweredABIV1` must match the manifest's `abi` field.

## In scope for v1

The suite runs one fixture on every applicable host and requires the same observable outcome.

| Outcome | What must match |
| --- | --- |
| Pass | Normalized stdout, stderr, and process exit code |
| Rejection | One error class, a nonzero exit, and empty stdout |

Rejection exit codes do not have to be the same number. The bytecode VM exits 1. A generated Go panic exits 2. The class is the contract.

Hosts for a `cli_app` body:

| Host | How the suite reaches it |
| --- | --- |
| Bytecode VM | `-compile-bc` then `-run-bc`. Canonical result. |
| Interpreter | `-run` |
| Go | Default `cli_app` backend, then `go build` |
| JavaScript | The same body with the root rewritten to `web_app`, then `node`, when `node` is on `PATH` |

A missing `node` is `BACKEND_UNSUPPORTED` for that host only. A present `node` that disagrees is a failure.

Wasm is not an execution host in v1. Its feasibility rule is the rejection set below. This revision does not add Wasm opcodes, collections, or host imports.

### Values

| Value | v1 rule |
| --- | --- |
| int | Signed integer. Printed with no decimal point when the value is a small exact integer. |
| float | IEEE-754 binary64. The v1 suite does not use inexact floats. |
| string | Unicode text compared and printed as UTF-8. |
| bool | `true` or `false`. |
| list | Ordered. An empty list is empty, not nil. |
| dict | String keys. Values stay as stored. |
| nil | The store-miss sentinel. Distinct from `""`. |

### Pure operations

These grant nothing. The suite runs them with an empty capability grant.

* Arithmetic `+`, `-`, `*` on integers, and `/` only when the quotient is an exact integer (`(/ 20 4)` is `5`).
* Comparisons and `if`.
* `let` and `print`. `print` writes one line. Several arguments are separated by a single space.
* `dict`, `map_get`, `map_keys`, `list`, `list_len`, `str_join`, `is_nil`.

`(map_get dict key)` on a missing key is `""`. `is_nil` of that result is false. A present value is returned as stored. This is the #103 absence rule. `map_get` grants nothing.

`(map_keys dict)` returns a list of strings in UTF-8 byte order (Go `sort.Strings`, Go string `<`). An empty dict is an empty list, so its length is `0`. A value that is not a dict fails at runtime with `TYPE_ERROR` and the text `map_keys expected dict`. `map_keys` grants nothing. The order, including keys outside the Basic Multilingual Plane, is the #109 rule. The suite replays `tests/fixtures/map_keys_sort_bmp.howl` and `tests/fixtures/map_keys_sort_nonbmp.howl`. Wasm does not emit `map_keys`; #73 must use this byte order if it ever does.

Integer `/` is not one rule yet. The interpreter truncates `int64`. The bytecode VM divides `float64`. JavaScript divides IEEE numbers. Go uses the operand types it emitted. Exact quotients agree. Non-exact quotients are outside v1. Division by zero is already in `tests/parity/12_error_div_zero.howl` and normalizes to `DIVISION_BY_ZERO`.

### Calls

A `defun` has a name, a parameter list, optional `type_hints`, and a body. `return` leaves the function. `(call name arg ...)` passes arguments by position. The existing harness already compares that shape on the interpreter, the bytecode VM, and Go: `tests/parity/06_control_flow.howl` (`TestParityCorpus`).

The experimental lowerer does not emit `defun`, `call`, or `while`. `LowerToBytecode` fails those graphs with `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. v1 does not move call execution onto HFIR.

### Memory and runtime imports

v1 defines no linear memory and no Wasm import table.

Observability imports `print`, `stderr`, and `exit` grant nothing.

Host effects are named by `capability.ForConstruct`. The v1 suite uses four of them. `(env "KEY")` requires `environment`. An empty grant denies it before the variable is read. The grant `environment` returns the value. `(exec cmd args...)` requires `process`, the same name as `OpExec`. An empty grant denies it before any subprocess starts. The grant `process` runs the command and returns its output. `(read_file path)` requires `filesystem`, the same name as `OpReadFile`. An empty grant denies it before any filesystem read. The grant `filesystem` returns the file bytes. `(fetch url method)` requires `network`, the same name as `OpFetch`. An empty grant denies it before any HTTP request. The grant `network` returns the response bytes. Database imports stay on that same table and are not given new opcodes here.

### Errors

Backends may use different JSON. The suite compares the class from `tools/difftest.NormalizeError`.

| Class | When |
| --- | --- |
| `TYPE_ERROR` | A pure operation rejects a wrong receiver, including `map_keys` on a non-dict. |
| `CAPABILITY_DENIED` | A host effect runs without its grant. |
| `DIVISION_BY_ZERO` | Division by zero. |
| `HFIR_TARGET_INFEASIBLE` | A target cannot execute a construct. Diagnostic contract `v1`. |
| `HFIR_BYTECODE_UNSUPPORTED` | The experimental lowerer is given a node outside its executable subset. No partial program. |

`map_keys` of a list literal is a checker diagnostic (`map_keys target must be dict, got list`) and never reaches the runtime. The runtime class is locked with a dynamically typed receiver, the same shape as `internal/vm/collection_type_test.go`.

### Effects and capability boundaries

Pure operations declare no capability. `map_keys` and `map_get` stay pure (#107). `store_keys` stays `database`, and a `file://` store also requires `filesystem`. v1 does not change that split.

`env` is a negative capability case. The suite binds it with `let` and prints the binding. With no grant, the interpreter, the bytecode VM, generated Go, and JavaScript reject with `CAPABILITY_DENIED`, exit nonzero, write no stdout, and do not include the secret value. With the `environment` grant, those hosts print the value.

`exec` is the process case. The suite binds `(exec "printf" "phase2b-exec-marker")` and prints that output as text. With no grant, the same four hosts reject with `CAPABILITY_DENIED`, exit nonzero, write no stdout, and do not include the marker. With the `process` grant, those hosts print the marker. The command is not a shell pipeline.

`read_file` is the filesystem case. The suite binds `(read_file "/tmp/howlframe-abi-v1-phase2c.txt")` and prints those bytes as text. The conformance test writes `phase2c-read-marker` to that absolute path before the case, because generated Go runs in its own directory. With no grant, the same four hosts reject with `CAPABILITY_DENIED`, exit nonzero, write no stdout, and do not include the marker. With the `filesystem` grant, those hosts print the marker.

`fetch` is the network case. The suite binds `(fetch "http://127.0.0.1:47653/howlframe-abi-v1-phase2d" "GET")` and prints the response body as text. The conformance test serves `phase2d-fetch-marker` at that address before the case, because every host must reach the same URL. With no grant, the same four hosts reject with `CAPABILITY_DENIED`, exit nonzero, write no stdout, and do not include the marker. The denial does not open a connection to that URL. With the `network` grant, those hosts print the marker. The shared form has no request body. `OpFetch` already had no body operand; this slice does not add one.

The interpreter and the bytecode VM consult `-allow-caps`. Generated Go and JavaScript do the same check in `howlFrameEnv`, `howlFrameExec`, `howlFrameReadFile`, and `howlFrameFetch`. Their runner grant is `HOWLFRAME_ALLOW_CAPS`, a comma-separated list of the same names as `-allow-caps`. An empty or unset value denies. A grant that omits the required name denies. `howlFrameEnv` may read that grant variable. It does not read the requested key until `environment` is present. `howlFrameExec` does not spawn until `process` is present, and the denial text does not contain the command. `howlFrameReadFile` does not call `os.ReadFile` or `readFileSync` until `filesystem` is present, and the denial text does not contain the path. `howlFrameFetch` does not call `http.NewRequest`, `http.DefaultClient.Do`, or `fetch` until `network` is present, and the denial text does not contain the URL. Other generated host effects, including `write_file` and `mkdir`, are still not mediated.

### Feasibility

`isFeasible` rejects a closed set for target `wasm`, and only that target:

* `exec`
* `spawn_agent`
* `http_server_start`

The diagnostic is `HFIR_TARGET_INFEASIBLE`. The same kinds are not rejected for `bytecode`, `interpreter`, `go`, `javascript`, or the empty `-validate` target. `hfir.WasmInfeasibleKinds` is that set. Adding a kind, or rejecting a v1 pure kind such as `map_keys` or `print`, changes this contract.

Bytecode construct support stays on `hfir.VerifyConstructs` over the AST (`internal/construct`), which also emits `HFIR_TARGET_INFEASIBLE`. That scan is unchanged.

Other targets do not yet share one feasibility table. A passing verifier result for `go` or `javascript` does not mean the effect is implemented there.

### CFG and SSA

A later lowering that owns meaning has to be a typed CFG in SSA:

* Blocks end in a jump, a conditional branch, or a return.
* Each value is assigned once.
* Data edges name operand roles (`key`, `value`, `body`, and so on).
* Control edges connect blocks.
* Node kinds are constructs, not user binding names.

`hfir.LowerAST` is not that form. It fills data edges for the semantic subset and leaves `ControlEdges` empty on every node, including the v1 arithmetic fixture. Kinds outside `lowerSemanticList` still come from the list head, so a user name can appear as a kind. v1 records that fact. It does not pretend the graph is SSA.

## Deferred (Phase 2)

Phase 2 is one lowered graph consumed by every host, with identical outcomes or the same feasibility rejection.

* Production `-compile-bc` still compiles the AST. Flipping that path is Phase 2.
* `defun`, `call`, and `while` become executable HFIR, or every host rejects them with one code. Today the hosts run them and the experimental lowerer rejects them.
* `ControlEdges` are populated and the graph is SSA.
* Go and JavaScript mediate `env` (Phase 2a), `exec` (Phase 2b), `read_file` (Phase 2c), and `fetch` (Phase 2d). Other generated host effects, including `write_file` and `mkdir`, still do not. One lowered graph for every host is still the rest of Phase 2.
* One feasibility table covers every target, not only the three Wasm host effects.
* Non-exact integer division picks one rule.
* Wasm collections (#73) and `for` / `match` / `try_let` / `spawn` SSA lowering (#84) target this ABI. They are not part of v1, and this revision does not grow them.
* No bytecode module linker, no HFIR module graph, no VM module opcode (#106).
* No new Frame opcode and no new capability.

## What the suite does not prove

Agreement among the AST backends is not proof that HFIR is the source of that agreement. `internal/vm/hfir_equivalence_test.go` is separate evidence that the experimental lowerer matches the bytecode VM on the subset it already emits, including `map_keys` and a granted `env`. That test is not the production compiler.
