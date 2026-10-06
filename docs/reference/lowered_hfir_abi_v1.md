# Lowered HFIR backend ABI v1

Version: `lowered-hfir-abi/v1`

This is the contract a backend must meet for the deterministic core, and the shape a later lowering has to grow into. It is not a claim that HFIR owns execution. Production compilation is still the checked AST: `runHFIRGate`, then `bytecode.CompileToBytecode` (`-compile-bc`). `-compile-hfir-bc` remains the experimental research lowerer. Go, JavaScript, and the interpreter still consume the AST.

The executable suite is `tests/conformance/lowered_hfir_abi_v1.json`, run by `tools/difftest`. The constant `hfir.LoweredABIV1` must match the manifest's `abi` field.

## In scope for v1

The suite runs one fixture on every applicable host and requires the same observable outcome.

| Outcome | What must match |
| --- | --- |
| Pass | Normalized stdout, stderr, and process exit code |
| Rejection | One error class and a nonzero exit. Stdout is empty unless the case records text printed before the denial. |

Rejection exit codes do not have to be the same number. The bytecode VM exits 1. A generated Go panic exits 2. The class is the contract.

Hosts for a `cli_app` body:

| Host | How the suite reaches it |
| --- | --- |
| Bytecode VM | `-compile-bc` then `-run-bc`. Canonical result. |
| Interpreter | `-run` |
| Go | Default `cli_app` backend, then `go build` |
| JavaScript | The same body with the root rewritten to `web_app`, then `node`, when `node` is on `PATH` |

A missing `node` is `BACKEND_UNSUPPORTED` for that host only. A present `node` that disagrees is a failure.

Where the experimental lowerer already emits the fixture, the suite also runs `hfir_bytecode`: `-compile-hfir-bc`, then `-run-bc` on the same VM. That host is not production `-compile-bc`. The pure core, `env`, `read_file`, `write_file`, `mkdir`, `exec`, `fetch`, and the nested multi-effect cases include it.

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
* `dict`, `map_get`, `map_keys`, `list`, `list_len`, `str_join`, `is_nil`, `html_escape`, `attr_escape`, `regex_match`.

`(map_get dict key)` on a missing key is `""`. `is_nil` of that result is false. A present value is returned as stored. This is the #103 absence rule. `map_get` grants nothing.

`(map_keys dict)` returns a list of strings in UTF-8 byte order (Go `sort.Strings`, Go string `<`). An empty dict is an empty list, so its length is `0`. A value that is not a dict fails at runtime with `TYPE_ERROR` and the text `map_keys expected dict`. `map_keys` grants nothing. The order, including keys outside the Basic Multilingual Plane, is the #109 rule. The suite replays `tests/fixtures/map_keys_sort_bmp.howl` and `tests/fixtures/map_keys_sort_nonbmp.howl`. Wasm does not emit `map_keys`; #73 must use this byte order if it ever does.

`(html_escape text)` encodes `&`, `<`, `>`, `"`, and `'` as `&amp;`, `&lt;`, `&gt;`, `&#34;`, and `&#39;`. An empty string stays empty. A non-string fails at runtime with `TYPE_ERROR` and the text `html_escape expected string`. `html_escape` grants nothing. Experimental `-compile-hfir-bc` lowers it onto the existing `HTML_ESCAPE` opcode: one `value` edge, then that instruction, with no string operand. The AST bytecode compiler emits the same instruction. A node without that edge returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. The model-adapter transport still rejects `html_escape`. `(attr_escape text)` is the same encoding under the existing `ATTR_ESCAPE` opcode. Experimental `-compile-hfir-bc` lowers it the same way: one `value` edge, then that instruction, with no string operand. The AST bytecode compiler emits the same instruction. A node without that edge returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. `attr_escape` grants nothing. A non-string fails at runtime with `TYPE_ERROR` and the text `attr_escape expected string`. The model-adapter transport still rejects `attr_escape`. `(regex_match pattern text)` pushes a bool. Experimental `-compile-hfir-bc` lowers it onto the existing `REGEX_MATCH` opcode: a `pattern` edge, then a `string` edge, then that instruction, with no string operand. The opcode pops 2 and pushes 1. The AST bytecode compiler emits the same instruction in that order. A node without those edges, or with them swapped, returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. `regex_match` grants nothing. The model-adapter transport still rejects `regex_match`. A provable non-string is rejected by the checker before either bytecode compiler runs. A value the checker cannot prove, such as `map_get` on a mixed dict, fails at runtime on both bytecode compilers with `TYPE_ERROR` and the text `regex_match expected string pattern and string`. The interpreter stringifies that value, generated Go does not compile the mixed dict into `MatchString`, and JavaScript coerces it, so that type-error case lists only `hfir_bytecode` and `bytecode`. `tests/parity/13_html_escape.howl` runs on `-compile-hfir-bc` because both escapes lower. `tests/parity/07_strings.howl` runs on `-compile-hfir-bc` because `regex_match` lowers. The conformance cases `html_escape` and `html_escape_type_error` are `tests/conformance/abi_v1/22_html_escape.howl` and `tests/conformance/abi_v1/23_html_escape_type_error.howl`. The conformance cases `attr_escape` and `attr_escape_type_error` are `tests/conformance/abi_v1/24_attr_escape.howl` and `tests/conformance/abi_v1/25_attr_escape_type_error.howl`. The escape cases use the same five hosts as `defun_call`. The passing escape case prints the five characters, the combined string, an empty escape inside `[]`, a `let` binding, the taken `if` branch, a `defun` result, one `while` step, and one `for` item. The false branch must not print. The escape type-error case prints nothing. The conformance cases `regex_match` and `regex_match_type_error` are `tests/conformance/abi_v1/26_regex_match.howl` and `tests/conformance/abi_v1/27_regex_match_type_error.howl`. The passing regex case uses the same five hosts as `defun_call` and prints `true` or `false` for anchored and unanchored patterns, an empty match, a `let` binding, the taken `if` branch, a `defun` result, one `while` step, and one `for` item. The false branch must not print `untaken`. The regex type-error case lists only `hfir_bytecode` and `bytecode`, and it prints nothing. `(time_now)` pushes an int. Experimental `-compile-hfir-bc` lowers it onto the existing `TIME_NOW` opcode. That instruction pops 0, pushes 1, and has no operand. The AST bytecode compiler emits the same instruction. A node with a data edge returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. `time_now` grants nothing. The model-adapter transport still rejects `time_now`. Parity is one identical `TIME_NOW` in both artifacts and no timestamp constant. The suite does not compare two live Unix timestamps. This comparison does not switch `-compile-bc` to HFIR. `(sleep …)` takes one duration. Experimental `-compile-hfir-bc` lowers it onto the existing `SLEEP` opcode. The duration edge keeps the empty name, because that opcode's operand list is empty. The lowerer compiles that child and then emits `SLEEP`. The instruction pops 1, pushes 0, and grants nothing. The AST bytecode compiler emits the same instruction after the same child. A node without that one empty-named edge returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. `sleep` grants nothing. The model-adapter transport still rejects `sleep`. Parity is one identical `SLEEP` after the compiled duration child. The comparison does not sleep. This comparison does not switch `-compile-bc` to HFIR.

Integer `/` is not one rule yet. The interpreter truncates `int64`. The bytecode VM divides `float64`. JavaScript divides IEEE numbers. Go uses the operand types it emitted. Exact quotients agree. Non-exact quotients are outside v1. Division by zero is already in `tests/parity/12_error_div_zero.howl` and normalizes to `DIVISION_BY_ZERO`.

### Calls

A `defun` has a name, a parameter list, optional `type_hints`, and a body. `return` leaves the function. `(call name arg ...)` passes arguments by position. The existing harness already compares that shape on the interpreter, the bytecode VM, and Go: `tests/parity/06_control_flow.howl` (`TestParityCorpus`).

Phase 3a makes that shape executable on the experimental lowerer only. `LowerAST` gives `defun` a name, `param` edges, and `body` edges, and erases `type_hint`, `type_hints`, and `type_param`. A return-type symbol between the parameter list and the body is erased the same way the AST bytecode compiler skips it. `call` stores the callee name and `arg` edges. `return` has an optional `value`. `LowerToBytecode` emits the existing `CALL` and `RETURN` opcodes and registers a `BCFunction`. It does not add an opcode. The model-adapter transport still rejects `defun`.

The conformance case `defun_call` is `tests/conformance/abi_v1/09_defun_call.howl`. Its hosts are:

| Host | Path |
| --- | --- |
| `hfir_bytecode` | Experimental `-compile-hfir-bc`, then `-run-bc`. This is the Phase 3a host. |
| `bytecode` | Production `-compile-bc` (AST bytecode after the gate), then `-run-bc`. Canonical result. |
| `interpreter`, `go`, `javascript` | Still the AST. Included because this fixture is already in their executable subset. |

Those hosts must print the same stdout. A program the experimental lowerer rejects is not a shared case: the AST hosts run it and `-compile-hfir-bc` fails closed.

### Loops

`(while cond body)` re-evaluates `cond` and runs `body` while the condition is true. The body runs zero times when the condition is false. A condition that is not a bool is `TYPE_ERROR` at runtime, and the checker rejects a known non-bool before either compiler.

Phase 3b makes that shape executable on the experimental lowerer only. `LowerAST` stores a `condition` data edge, a `body` data edge, and `ControlEdges` in that order: the test, then the body. The back edge is the existing `JUMP` to the test. `LowerToBytecode` follows those control edges and emits the existing `JUMP_IF_FALSE` and `JUMP` opcodes, with the same relative offsets as the AST bytecode compiler. A `while` whose control edges are missing or are not that pair fails with `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. It does not add an opcode. The model-adapter transport still rejects `while`.

The conformance case `while_control` is `tests/conformance/abi_v1/10_while.howl`. Its hosts are the same five as `defun_call`. The false loop must not print. The counting loop prints `1`, `2`, and `3`.

`(for item iterable body)` binds `item` to each element of `iterable` and runs `body`. An empty list runs the body zero times. An iterable that is not a list is `TYPE_ERROR` at runtime, and the checker rejects a known non-list before either compiler.

Phase 3d records that shape on the experimental lowerer. `LowerAST` stores an `iterable` data edge, a `body` data edge, and `ControlEdges` in that order. The iterator name stays on the node value. The back edge is the existing `JUMP` to `FOR_NEXT`. `LowerToBytecode` follows those control edges and emits the existing `FOR_INIT`, `FOR_NEXT`, and `JUMP` opcodes, with the same relative offsets as the AST bytecode compiler. A `for` whose control edges are missing or swapped fails with `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. It does not add an opcode. The model-adapter transport still rejects `for`.

The conformance case `for_control` is `tests/conformance/abi_v1/13_for.howl`. Its hosts are the same five as `defun_call`. The empty list must not print. The taken loop prints `a`, `b`, and `c`.

### Branches

`(if cond then)` and `(if cond then else)` evaluate `cond` and run one branch. A false condition skips `then`. A condition that is not a bool is `TYPE_ERROR` at runtime, and the checker rejects a known non-bool before either compiler.

Phase 3c records that shape on the experimental lowerer. `LowerAST` stores a `condition` data edge, a `then` data edge, an optional `else` data edge, and `ControlEdges` in that order. `LowerToBytecode` follows those control edges and emits the existing `JUMP_IF_FALSE` and, when an else is present, `JUMP`, with the same relative offsets as the AST bytecode compiler. An `if` whose control edges are missing or swapped fails with `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. It does not add an opcode. The model-adapter transport still accepts `if` and still rejects `while`, `defun`, and `for`. The transport schema has no control-edge field; decoding an `if` derives the successors from those roles.

The conformance case `if_control` is `tests/conformance/abi_v1/11_if.howl`. Its hosts are the same five as `defun_call`. The false branches must not print. The taken branches print `else`, `then`, `greater`, and `only`.

The conformance case `nested_if_while_defun` is `tests/conformance/abi_v1/12_nested_if_while_defun.howl`. Its hosts are the same five. One `defun` nests a `while` that contains `if`, and an `if` that contains `while`, and the program calls that function twice. The false loop must not print. The taken lines are `low 0`, `low 1`, `mid 2`, `hit 3`, `mid 4`, `again 1`, `result 2`, `miss`, and `empty 0`. This case is evidence that the experimental lowerer and the AST bytecode compiler execute that combination the same way. It does not switch `-compile-bc` to HFIR.

The conformance case `nested_for_if_while_defun` is `tests/conformance/abi_v1/14_nested_for_if_while_defun.howl`. Its hosts are the same five. One `defun` nests a `while` that contains `for`, a `for` that contains `for`, an `if` that contains `for`, and a `for` that contains `while`. The program calls that function twice. The false loop and the empty list must not print. The taken lines are `L a 0`, `step a`, `L b 0`, `step b`, `L a 1`, `step a`, `L b 1`, `step b`, `kept 4`, `result 4`, `miss`, and `empty 0`. This case is evidence that the experimental lowerer and the AST bytecode compiler execute that combination the same way. It does not switch `-compile-bc` to HFIR.

### Memory and runtime imports

v1 defines no linear memory and no Wasm import table.

Observability imports `print`, `stderr`, and `exit` grant nothing.

Host effects are named by `capability.ForConstruct`. The v1 suite uses six of them. `(env "KEY")` requires `environment`. An empty grant denies it before the variable is read. The grant `environment` returns the value. `(exec cmd args...)` requires `process`, the same name as `OpExec`. An empty grant denies it before any subprocess starts. The grant `process` runs the command and returns its output. `(read_file path)` requires `filesystem`, the same name as `OpReadFile`. An empty grant denies it before any filesystem read. The grant `filesystem` returns the file bytes. `(write_file path data)` requires `filesystem`, the same name as `OpWriteFile`. An empty grant denies it before any filesystem write. The grant `filesystem` writes the bytes. `(mkdir path)` requires `filesystem`, the same name as `OpMkdir`. An empty grant denies it before any directory is created. The grant `filesystem` creates the directory. `(fetch url method)` requires `network`, the same name as `OpFetch`. An empty grant denies it before any HTTP request. The grant `network` returns the response bytes. Database imports stay on that same table and are not given new opcodes here.

### Errors

Backends may use different JSON. The suite compares the class from `tools/difftest.NormalizeError`.

| Class | When |
| --- | --- |
| `TYPE_ERROR` | A pure operation rejects a wrong receiver, including `map_keys` on a non-dict, `html_escape` on a non-string, `attr_escape` on a non-string, and `regex_match` on a non-string the checker did not prove. |
| `CAPABILITY_DENIED` | A host effect runs without its grant. |
| `DIVISION_BY_ZERO` | Division by zero. |
| `HFIR_TARGET_INFEASIBLE` | A target cannot execute a construct. Diagnostic contract `v1`. |
| `HFIR_BYTECODE_UNSUPPORTED` | The experimental lowerer is given a node outside its executable subset. No partial program. |

`map_keys` of a list literal is a checker diagnostic (`map_keys target must be dict, got list`) and never reaches the runtime. The runtime class is locked with a dynamically typed receiver, the same shape as `internal/vm/collection_type_test.go`.

### Effects and capability boundaries

Pure operations declare no capability. `map_keys` and `map_get` stay pure (#107). `store_keys` stays `database`, and a `file://` store also requires `filesystem`. v1 does not change that split.

`env` is a negative capability case. The suite binds it with `let` and prints the binding. With no grant, the interpreter, the production bytecode VM, experimental `-compile-hfir-bc`, generated Go, and JavaScript reject with `CAPABILITY_DENIED`, exit nonzero, write no stdout, and do not include the secret value. With the `environment` grant, those hosts print the value. The experimental lowerer already emits the existing `ENV` opcode.

`exec` is the process case. The suite binds `(exec "printf" "phase2b-exec-marker")` and prints that output as text. With no grant, the interpreter, the production bytecode VM, experimental `-compile-hfir-bc`, generated Go, and JavaScript reject with `CAPABILITY_DENIED`, exit nonzero, write no stdout, and do not include the marker or the command. With the `process` grant, those hosts print the marker. The command is not a shell pipeline. Experimental `-compile-hfir-bc` lowers `(exec cmd args...)` to a `cmd` edge and then one `arg` edge per argument. `LowerToBytecode` emits the existing `EXEC` opcode, command then arguments, with `IntOperand` equal to the argument count, the same operand order as the AST bytecode compiler. It does not add an opcode. An `exec` without the command edge, or with an argument before the command, returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. The model-adapter transport still rejects `exec`.

`read_file` is the filesystem case. The suite binds `(read_file "/tmp/howlframe-abi-v1-phase2c.txt")` and prints those bytes as text. The conformance test writes `phase2c-read-marker` to that absolute path before the case, because generated Go runs in its own directory. With no grant, the interpreter, the production bytecode VM, experimental `-compile-hfir-bc`, generated Go, and JavaScript reject with `CAPABILITY_DENIED`, exit nonzero, write no stdout, and do not include the marker. With the `filesystem` grant, those hosts print the marker. The experimental lowerer already emits the existing `READ_FILE` opcode. `bytes_to_string` is the existing `CONVERT` opcode.

`fetch` is the network case. The suite binds `(fetch "http://127.0.0.1:47653/howlframe-abi-v1-phase2d" "GET")` and prints the response body as text. The conformance test serves `phase2d-fetch-marker` at that address before the case, because every host must reach the same URL. With no grant, the interpreter, the production bytecode VM, generated Go, and JavaScript reject with `CAPABILITY_DENIED`, exit nonzero, write no stdout, and do not include the marker. The denial does not open a connection to that URL. With the `network` grant, those hosts print the marker. The shared form has no request body. `OpFetch` has no body operand, and this slice does not add one. Experimental `-compile-hfir-bc` lowers `(fetch url method)` to a `url` edge and then a `method` edge. `LowerToBytecode` emits the existing `FETCH` opcode, URL then method, the same operand order as the AST bytecode compiler. It does not add an opcode. An optional third argument is a `body` edge. The AST bytecode compiler does not compile that child, and the experimental lowerer does not either, so a body is not sent on either bytecode path. A `fetch` without the URL and method edges, or with the method before the URL, returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. The model-adapter transport still rejects `fetch`. With no grant, experimental `-compile-hfir-bc` rejects with the same `CAPABILITY_DENIED` as the other hosts and does not open a connection. `fetch_denied` and `fetch_granted` include `hfir_bytecode`.

`write_file` is a filesystem write. The suite runs `(write_file "/tmp/howlframe-abi-v1-phase2e-write.txt" "phase2e-write-marker")` and then prints `phase2e-wrote`. The path is absolute because generated Go runs in its own directory. With no grant, the interpreter, the production bytecode VM, experimental `-compile-hfir-bc`, Go, and JavaScript reject with `CAPABILITY_DENIED`, exit nonzero, write no stdout, do not include the marker, and do not create the file. With the `filesystem` grant, those hosts print `phase2e-wrote` and the file contains the marker.

`mkdir` is directory creation under that same grant. The suite runs `(mkdir "/tmp/howlframe-abi-v1-phase2e-dir")` and then prints `phase2e-made`. With no grant, the same five hosts reject with `CAPABILITY_DENIED`, exit nonzero, write no stdout, and do not create the directory. With the `filesystem` grant, those hosts print `phase2e-made` and the directory exists.

Experimental `-compile-hfir-bc` lowers `(write_file path data)` to a `path` edge and a `data` edge, and `(mkdir path)` to a `path` edge. `LowerToBytecode` emits the existing `WRITE_FILE` and `MKDIR` opcodes, in the same operand order as the AST bytecode compiler. It does not add an opcode. The model-adapter transport still rejects both kinds. Production `-compile-bc` is still the AST.

The conformance cases `nested_fs_write_denied` and `nested_fs_write_granted` are `tests/conformance/abi_v1/17_nested_fs_write.howl`. Their hosts are the same five. One `defun` nests `write_file` and `mkdir` inside `if`, `while`, and `for`, including a `for` inside a `for`. With no grant, the first reached filesystem effect denies before any path is created, and the denial does not include the path. With the `filesystem` grant, the taken lines are `L a 0`, `L b 0`, `kept 2`, `result 2`, `miss`, and `empty 0`. The false loop, the empty list, and the untaken `if` branches must not print and must not create their paths. This case is evidence that the experimental lowerer and the AST bytecode compiler execute that combination the same way. It does not switch `-compile-bc` to HFIR.

The conformance cases `nested_env_read_denied` and `nested_env_read_granted` are `tests/conformance/abi_v1/18_nested_env_read.howl`. Their hosts are the same five. One `defun` nests `env` and `read_file` inside `if`, `while`, and `for`, including a `for` inside a `for`. With no grant, the first reached effect is `env`. The denial is `CAPABILITY_DENIED` before any file is read, and the denial does not include the secret or the path. With the `environment,filesystem` grant, the taken lines are `phase1-token`, `dogfood-read-marker`, `L a 0`, `L b 0`, `kept 2`, `phase1-token`, `result 2`, `miss`, `dogfood-miss-marker`, and `empty 0`. The false loop, the empty list, and the untaken `if` branches must not print and must not touch their paths. This case is evidence that the experimental lowerer and the AST bytecode compiler execute that combination the same way. It does not switch `-compile-bc` to HFIR.

The conformance cases `nested_exec_denied` and `nested_exec_granted` are `tests/conformance/abi_v1/19_nested_exec.howl`. Their hosts are the same five. One `defun` nests `exec` inside `if`, `while`, and `for`, including a `for` inside a `for`. With no grant, the first reached effect is `exec`. The denial is `CAPABILITY_DENIED` before any subprocess starts, and the denial does not include the command or the marker. With the `process` grant, the taken lines are `phase2b-exec-marker`, `L a 0`, `L b 0`, `kept 2`, `phase2b-exec-kept`, `result 2`, `miss`, `phase2b-exec-miss`, and `empty 0`. The false loop, the empty list, and the untaken `if` branches must not print and must not run their commands. This case is evidence that the experimental lowerer and the AST bytecode compiler execute that combination the same way. It does not switch `-compile-bc` to HFIR.

The conformance cases `nested_fetch_denied` and `nested_fetch_granted` are `tests/conformance/abi_v1/20_nested_fetch.howl`. Their hosts are the same five. One `defun` nests `fetch` inside `if`, `while`, and `for`, including a `for` inside a `for`. With no grant, the first reached effect is `fetch`. The denial is `CAPABILITY_DENIED` before any HTTP request, and the denial does not include the URL or the marker. With the `network` grant, the taken lines are `phase2d-fetch-marker`, `L a 0`, `L b 0`, `kept 2`, `phase2d-fetch-marker`, `result 2`, `miss`, `phase2d-fetch-marker`, and `empty 0`. The false loop, the empty list, and the untaken `if` branches must not print and must not send their requests. This case is evidence that the experimental lowerer and the AST bytecode compiler execute that combination the same way. It does not switch `-compile-bc` to HFIR.

The conformance cases `nested_multi_effect_denied`, `nested_multi_effect_partial`, and `nested_multi_effect_granted` are `tests/conformance/abi_v1/21_nested_multi_effect.howl`. Their hosts are the same five. One `defun` nests `env`, `read_file`, `write_file`, `mkdir`, `exec`, and `fetch` inside `if`, `while`, and `for`, including a `for` inside a `for`. With no grant, the first reached effect is `env`. The denial is `CAPABILITY_DENIED` before any later effect, stdout is empty, and no path is created. With `environment,filesystem,process` and without `network`, the first site prints `phase1-token`, `multi-read-marker`, and `phase2b-exec-marker`, writes one file, creates one directory, and then `fetch` is `CAPABILITY_DENIED`. No request is sent. With `environment,filesystem,process,network`, the taken lines continue through `phase2d-fetch-marker`, `L a 0`, `L b 0`, `kept 2`, the kept site, `result 2`, `miss`, the miss site, and `empty 0`. The untaken branches must not print, must not create their paths, and must not send their requests. One untaken `fetch` passes a body string. `OpFetch` has no body operand, and neither bytecode compiler loads that string. This case is evidence that the experimental lowerer and the AST bytecode compiler execute that combination the same way. It does not switch `-compile-bc` to HFIR.

The interpreter and the bytecode VM consult `-allow-caps`. Generated Go and JavaScript do the same check in `howlFrameEnv`, `howlFrameExec`, `howlFrameReadFile`, `howlFrameFetch`, `howlFrameWriteFile`, and `howlFrameMkdir`. Their runner grant is `HOWLFRAME_ALLOW_CAPS`, a comma-separated list of coarse capability names. Native VM/interpreter `-allow-caps` additionally accepts `filesystem:read=<root>` and `filesystem:write=<root>`; generated Go/JavaScript do not enforce these scopes. An empty or unset value denies. A grant that omits the required name denies. `howlFrameEnv` may read that grant variable. It does not read the requested key until `environment` is present. `howlFrameExec` does not spawn until `process` is present, and the denial text does not contain the command. `howlFrameReadFile` does not call `os.ReadFile` or `readFileSync` until `filesystem` is present, and the denial text does not contain the path. `howlFrameFetch` does not call `http.NewRequest`, `http.DefaultClient.Do`, or `fetch` until `network` is present, and the denial text does not contain the URL. `howlFrameWriteFile` does not call `os.WriteFile` or `writeFileSync` until `filesystem` is present, and the denial text does not contain the path. `howlFrameMkdir` does not call `os.MkdirAll` or `mkdirSync` until `filesystem` is present, and the denial text does not contain the path. Other generated host effects are still not mediated.

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

`hfir.LowerAST` is not that form. It fills data edges for the semantic subset. Phase 3b fills `ControlEdges` on a `while` header: the condition, then the body. Phase 3c fills `ControlEdges` on an `if`: the condition, the then-branch, and an optional else. Phase 3d fills `ControlEdges` on a `for` header: the iterable, then the body. The v1 arithmetic fixture has no `while` and no `for`. Its `if` nodes carry those branch successors, and every other node in that fixture still has empty `ControlEdges`. Kinds outside `lowerSemanticList` still come from the list head, so a user name can appear as a kind. v1 records that fact. It does not pretend the graph is SSA.

## Deferred (Phase 2)

Phase 2 is one lowered graph consumed by every host, with identical outcomes or the same feasibility rejection.

* Production `-compile-bc` still compiles the AST. Flipping that path is still Phase 2. Phase 3a, Phase 3b, Phase 3c, and Phase 3d do not flip it. The kill, defer, and promote checklist is `docs/reference/lowered_hfir_prod_flip_criteria.md`. #90 stays Partial.
* `defun`, `call`, and `return` are executable on `-compile-hfir-bc` (Phase 3a). `while` is executable on that same flag (Phase 3b), with control edges on the loop header. `if` on that flag follows control edges for the condition, the then-branch, and an optional else (Phase 3c). `for` on that flag follows control edges for the iterable and then the body (Phase 3d). The interpreter, the production bytecode VM, Go, and JavaScript still run calls, loops, and branches from the AST. One lowered graph for every host is still open.
* `ControlEdges` on every control form, and an SSA graph, are still open. Phase 3b fills them for `while`. Phase 3c fills them for `if`. Phase 3d fills them for `for`. `match` and `try` stay empty.
* Go and JavaScript mediate `env` (Phase 2a), `exec` (Phase 2b), `read_file` (Phase 2c), `fetch` (Phase 2d), and `write_file` and `mkdir` (Phase 2e). Experimental `-compile-hfir-bc` also lowers `exec` onto the existing `EXEC` opcode and `fetch` onto the existing `FETCH` opcode. The optional `fetch` body stays off that opcode. Other generated host effects still do not. One lowered graph for every host is still the rest of Phase 2.
* One feasibility table covers every target, not only the three Wasm host effects.
* Non-exact integer division picks one rule.
* Wasm collections (#73) and `for` / `match` / `try_let` / `spawn` SSA lowering (#84) target this ABI. They are not part of v1, and this revision does not grow them.
* No bytecode module linker, no HFIR module graph, no VM module opcode (#106).
* No new Frame opcode and no new capability.

## What the suite does not prove

Agreement among the AST backends is not proof that HFIR is the source of that agreement. `internal/vm/hfir_equivalence_test.go` is separate evidence that the experimental lowerer matches the bytecode VM on the subset it already emits, including `map_keys`, a granted `env`, Phase 3a `defun` / `call`, Phase 3b `while`, Phase 3c `if`, Phase 3d `for`, the nested `if` / `while` / `defun` fixture, the nested `for` / `if` / `while` / `defun` fixture, the nested `env` / `read_file` fixture, `exec` granted and denied, including `exec` nested with `if`, `while`, `for`, and `defun`, `fetch` granted and denied, including `fetch` nested with `if`, `while`, `for`, and `defun`, and one program that nests `env`, `read_file`, `write_file`, `mkdir`, `exec`, and `fetch` together. That test is not the production compiler. The pure-core cases, `env_denied`, `env_granted`, `read_file_denied`, `read_file_granted`, `exec_denied`, `exec_granted`, `defun_call`, `while_control`, `if_control`, `for_control`, `nested_if_while_defun`, `nested_for_if_while_defun`, `nested_env_read_denied` / `nested_env_read_granted`, `nested_exec_denied` / `nested_exec_granted`, `fetch_denied` / `fetch_granted`, `nested_fetch_denied` / `nested_fetch_granted`, and `nested_multi_effect_denied` / `nested_multi_effect_partial` / `nested_multi_effect_granted`, `html_escape`, `html_escape_type_error`, `attr_escape`, `attr_escape_type_error`, `regex_match`, and `regex_match_type_error` compare `-compile-hfir-bc` with the AST hosts. `regex_match_type_error` compares the two bytecode compilers only. `TestHFIRBytecodeSupportedParity` compares that lowerer with the AST hosts on the `tests/parity` files it already emits, including `tests/parity/13_html_escape.howl` and `tests/parity/07_strings.howl`. Experimental `-compile-hfir-bc` lowers `(html_escape text)` onto the existing `HTML_ESCAPE` opcode and `(attr_escape text)` onto the existing `ATTR_ESCAPE` opcode. Experimental `-compile-hfir-bc` lowers `(regex_match pattern text)` onto the existing `REGEX_MATCH` opcode, pattern then text. The suite compares that path with production `-compile-bc`, including `regex_match` nested with `if`, `while`, `for`, and `defun`. A non-string the checker cannot prove is `TYPE_ERROR` on both bytecode compilers. `tests/parity/07_strings.howl` runs on `-compile-hfir-bc`. Experimental `-compile-hfir-bc` lowers `(time_now)` onto the existing `TIME_NOW` opcode. The instruction pops 0, pushes 1, and has no operand. Parity with production `-compile-bc` is one identical `TIME_NOW` per call and no timestamp constant, including `time_now` nested with `if`, `while`, `for`, and `defun`. That comparison does not match two live Unix timestamps. Matching stdout there does not mean `-compile-bc` consumes HFIR.
