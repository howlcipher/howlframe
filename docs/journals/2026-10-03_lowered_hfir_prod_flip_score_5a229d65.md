# Production `-compile-bc` flip score of `5a229d65`

## Verdict

Verdict: flip deferred.

Scored tree: `main` at `5a229d6553588f6916f9fe4d6b596cd1e6fa0de6`. Commit subject: `fix(wasm): preserve fail-closed integer division in Wasm SSA per contract`. At the score, `git rev-parse HEAD` and `git rev-parse origin/main` were that SHA. The checklist is `docs/reference/lowered_hfir_prod_flip_criteria.md`. The Decision line there stays "Defer the production flip."

#90 stays Partial. `improvements.md` still records improvement 90 as Partial. A GitHub API read of `howlcipher/howlframe` issue 90 returned 404. That miss is not a close. This score does not mark #90 Done.

This note is a score of that SHA. It is a measurement against the checklist. It does not authorize a flip. It does not refresh the Assurance tip-lock. The lock named in the checklist remains `4d74dbcf9654caa05e0b1d9212b15bc5398359e3`.

## How a mark is read

Met means the promote condition holds on this SHA, an accepted limit still matches the checklist, or the named kill shape is absent from this tree. Not met means the promote condition does not hold. Unknown means this score did not verify the row. A Met kill row means that shape is absent. It does not clear the fence.

## Suite on this SHA

Recorded 2026-10-03T16:38:20Z. Go was `go1.22.2 linux/amd64`. Node was `v22.14.0` at `/exec-daemon/node`. Each command exited 0.

```
go test . -count=1 -timeout 180s -run 'TestProdFlipCriteriaLock'
ok  	github.com/howlcipher/howlframe	0.005s

go test ./internal/vm -count=1 -timeout 900s -run 'TestHFIRBytecode'
ok  	github.com/howlcipher/howlframe/internal/vm	0.054s

go test ./tools/difftest -count=1 -timeout 900s -run 'TestLoweredHFIRABIConformance|TestHFIRBytecodeSupportedParity|TestHFIRBytecodeRejectsUnsupportedParity'
ok  	github.com/howlcipher/howlframe/tools/difftest	13.850s
```

`go test ./...` was not run for this score. This note does not claim that bar. The three commands above are the suite the checklist names. A green run on this SHA is score evidence. It is not a new Assurance tip-lock.

A second run hid `node` (`PATH=/usr/bin:/bin`, `command -v node` found nothing) and executed `TestLoweredHFIRABIConformance/env_denied`. The log line was `javascript skipped: node runtime not available`, and that case passed.

## HFIR↔AST parity

| Item | Status | Evidence |
| --- | --- | --- |
| Experimental and production artifacts match on every program production `-compile-bc` accepts today | Not met | `hfirRejectedParity` still lists `07_strings.howl` and `13_html_escape.howl`. `TestHFIRBytecodeRejectsUnsupportedParity` passed: `LowerToBytecode` returns one `HFIR_BYTECODE_UNSUPPORTED` and no program, and production `-compile-bc` still runs those files. The promote-blocker fence is non-empty. |
| Every `tests/conformance/lowered_hfir_abi_v1.json` case that lists `hfir_bytecode` matches the `bytecode` host on normalized stdout, stderr, and exit, including nested `if` / `while` / `defun`, nested `for`, and `nested_multi_effect` under the empty grant, the grant that omits `network`, and the full grant | Met | The manifest has 37 cases. Every case lists `hfir_bytecode`, `bytecode`, `interpreter`, `go`, and `javascript`. Names include `nested_if_while_defun`, `nested_for_if_while_defun`, `nested_multi_effect_denied`, `nested_multi_effect_partial`, and `nested_multi_effect_granted`. `TestLoweredHFIRABIConformance` passed with node on `PATH`. |
| `TestHFIRBytecodeSupportedParity` passes, and every file in `tests/parity` is classified | Met | The directory has 13 `*.howl` files. `hfirSupportedParity` has 11 (`01`–`06`, `08`–`12`). `hfirRejectedParity` has the other two. The test's classification check passed inside the difftest command above. |
| `hfirRejectedParity` is empty | Not met | `tools/difftest/difftest_test.go` holds `07_strings.howl` (`regex_match`) and `13_html_escape.howl` (`attr_escape`; the file also calls `html_escape`). |
| Operand order stays the AST order: `EXEC` is command then arguments with `IntOperand` equal to the argument count; `FETCH` is URL then method; `JUMP_IF_FALSE`, `JUMP`, `FOR_INIT`, `FOR_NEXT`, `CALL`, and `RETURN` keep the relative offsets the AST compiler already emits | Met | `internal/bytecode/bytecode.go` and `internal/hfir/bytecode.go` emit the same shapes. `EXEC` compiles the command and then the arguments. `FETCH` compiles the URL and then the method. `if`, `while`, and `for` use the same `IntOperand` formulas (`then+1` / `then+2`, `body+2` and the negative back edge, `FOR_NEXT` of `body+2`). `CALL` stores the argument count. `RETURN` has no offset operand. `TestHFIRBytecode` passed, including `TestHFIRBytecodeExecOperandOrder` and `TestHFIRBytecodeFetchOperandOrder`. |
| The promote-blocker fence is empty | Not met | `TestProdFlipCriteriaLock` recomputed the fence from `construct.Supported`, `compileNode`, and `LowerToBytecode` and matched the checklist's `promote-blockers` block. That block has 29 names. `html_escape` is outside it. |

Names the checklist already places outside the fence, because they lower under another kind: `cli_app` (`program`), `do` (`sequence`), `try_let` (`try`), the arithmetic and comparison heads (`binary`), and `to_int`, `to_float`, `to_string`, `bytes_to_string`, and `encode_json` (`convert`). Status: Met. The same lock test uses that alias map. A missing alias would have failed the fence comparison.

`html_escape` has left the fence onto the existing `HTML_ESCAPE` opcode. Status: Met. The lock test's `html_escape` subtest requires `HTML_ESCAPE` on both bytecode compilers for `(html_escape "a<b")`, and that test passed. `html_escape` is not in the 29-name block.

Each name still in the fence is Not met. Evidence for every row is the same passing fence comparison: the name is in the checklist block and in the recomputed set, so `LowerToBytecode` has no case for it while `compileNode` does. This score did not execute a separate parity program per name.

| Promote blocker | Status |
| --- | --- |
| `achieve` | Not met |
| `attr_escape` | Not met |
| `confidence` | Not met |
| `db_connect` | Not met |
| `ephemeral_circuit` | Not met |
| `http_server` | Not met |
| `lazy_synthesize` | Not met |
| `llm_generate` | Not met |
| `neural_circuit` | Not met |
| `optimize_block` | Not met |
| `optimize_signature` | Not met |
| `read_line` | Not met |
| `regex_match` | Not met |
| `req_method` | Not met |
| `res` | Not met |
| `res_header` | Not met |
| `res_json` | Not met |
| `schema_bridge` | Not met |
| `sleep` | Not met |
| `spawn` | Not met |
| `spawn_agent` | Not met |
| `sql_query` | Not met |
| `store_delete` | Not met |
| `store_get` | Not met |
| `store_keys` | Not met |
| `store_open` | Not met |
| `store_put` | Not met |
| `task` | Not met |
| `time_now` | Not met |

`regex_match` and `attr_escape` have an extra check. The lock test lowers each sample, expects one `HFIR_BYTECODE_UNSUPPORTED` and no program, and expects production bytecode to contain `REGEX_MATCH` or `ATTR_ESCAPE`. That subtest passed.

## Mediated host effects

The bytecode path still uses the Phase 2a–2e mediators. Go and JavaScript still consume the AST. `howlFrameGrantHas` appears only in the six helpers below, in `internal/backend/gogen/gogen.go` and `internal/backend/javascript/javascript.go`.

| Effect | Status | Evidence |
| --- | --- | --- |
| `env` / `environment` / `howlFrameEnv` / `ENV` | Met | `opcode.go` `OpEnv` pops 1, capability `environment`. Helpers check `environment` before `Getenv` / `process.env`. Conformance cases `env_denied` and `env_granted` are in the passing suite, including nested `nested_env_read_denied` and `nested_env_read_granted`. |
| `exec` / `process` / `howlFrameExec` / `EXEC` | Met | `OpExec` capability `process`, `IntOperand` argument count. The Go helper spawns only after `process` is present. Cases `exec_denied`, `exec_granted`, `nested_exec_denied`, `nested_exec_granted` are in the passing suite. |
| `read_file` / `filesystem` / `howlFrameReadFile` / `READ_FILE` | Met | `OpReadFile` pops 1, capability `filesystem`. Cases `read_file_denied`, `read_file_granted`, and the nested env/read fixtures are in the passing suite. |
| `write_file` / `filesystem` / `howlFrameWriteFile` / `WRITE_FILE` | Met | `OpWriteFile` pops 2, capability `filesystem`. Cases `write_file_denied` and `write_file_granted` are in the passing suite. The conformance test also checks that denial creates no marker file and that the grant writes the marker body. |
| `mkdir` / `filesystem` / `howlFrameMkdir` / `MKDIR` | Met | `OpMkdir` pops 1, capability `filesystem`. Cases `mkdir_denied` and `mkdir_granted` are in the passing suite, including the directory-created check on grant and the absent-path check on denial. |
| `fetch` / `network` / `howlFrameFetch` / `FETCH` | Met | `OpFetch` is `FETCH`, pops 2, empty operand list, capability `network`. Cases `fetch_denied`, `fetch_granted`, `nested_fetch_denied`, and `nested_fetch_granted` are in the passing suite. |

| Denial rule | Status | Evidence |
| --- | --- | --- |
| An empty grant is `CAPABILITY_DENIED` before the effect | Met | `deny_all` cases in the manifest expect `CAPABILITY_DENIED`. The VM calls `requireCapability` from the registry capability before the opcode switch (`internal/vm/vm.go`). The panic text is `capability denied:` plus the capability name. Go and JavaScript helpers return that class before the read, spawn, filesystem call, or HTTP call. The suite passed. |
| A missing grant is `CAPABILITY_DENIED` before the effect | Met | `parseAllowedCaps("")` returns nil, and the `-allow-caps` default is empty. `deny_all` omits `-allow-caps` for bytecode. Go `howlFrameGrantHas` and the JavaScript helper treat an empty or unset `HOWLFRAME_ALLOW_CAPS` as denial. The suite's generated-host denial sets that variable to empty. |
| A grant that omits the name is `CAPABILITY_DENIED` before the effect | Met | `nested_multi_effect_partial` grants `environment,filesystem,process` and expects `CAPABILITY_DENIED`, with stdout for the granted effects and `forbid` `phase2d-fetch-marker`. That case passed. The grant helper denies when the name is absent from the list. |
| Denial text omits the secret, the command, the path, and the URL | Met | Denied cases set `forbid` to the marker or secret (`phase1-token`, `phase2b-exec-marker`, `phase2c-read-marker`, `phase2d-fetch-marker`, `phase2e-write-marker`). The multi-effect cases also reject a blob that contains the fetch URL, the dropped body string, or the multi-effect path prefix. Those checks passed inside `TestLoweredHFIRABIConformance`. The VM denial format is the capability name. |
| The six effects, including nested and multi-effect fixtures, pass on `interpreter`, `bytecode`, `hfir_bytecode`, `go`, and `javascript` when `node` is on `PATH` | Met | Full difftest command above, node `v22.14.0`. |
| A missing `node` is `BACKEND_UNSUPPORTED` for JavaScript only | Met | `executeJSBackend` returns `BACKEND_UNSUPPORTED` with `node runtime not available` when `exec.LookPath("node")` fails. The conformance loop skips only that JavaScript result. The `env_denied` rerun with node hidden logged that skip and passed, so the other hosts for that case still matched. The other 36 cases were not re-run with node hidden. |
| A present `node` that disagrees fails the lock | Met | With node present, the full conformance run passed, so JavaScript agreed with the oracle on every case. `conformance_test.go` calls `t.Errorf` when a target's error class, exit, stdout, or forbidden marker disagrees. This score did not inject a broken node binary. |
| Generated host effects outside the mediator table stay unmediated | Met | `howlFrameGrantHas` is referenced only by the six helpers. `spawn`, `regex_match`, `sleep`, and the other generated heads in `gogen.go` have no grant check. |
| The flip PR leaves those outside-table effects unmediated | Not met | No flip PR is in this tree. Production `-compile-bc` is still `bytecode.CompileToBytecode`. |
| Moving Go and JavaScript onto one lowered graph stays later than this bytecode flip | Met | Both generators still walk the AST. #90 stays Partial in `improvements.md`. |
| #90 stays Partial either way | Met | See the verdict. The GitHub 404 is not a close. |

## Assurance tip-lock

| Item | Status | Evidence |
| --- | --- | --- |
| `origin/main` on this score is the recorded Assurance tip-lock | Not met | `git rev-parse origin/main` was `5a229d6553588f6916f9fe4d6b596cd1e6fa0de6`. The checklist, `docs/journals/2026-09-30_lowered_hfir_prod_flip_tip_lock.md`, and `docs/journals/2026-09-30_assurance_tip_lock_4d74dbcf.md` name `4d74dbcf9654caa05e0b1d9212b15bc5398359e3`. |
| On this SHA, `*compileBc` still calls `bytecode.CompileToBytecode(root)` and does not call `LowerToBytecode` | Met | `howlframe.go` `*compileBc` branch calls `runHFIRGate` and then `bytecode.CompileToBytecode(root)`. `TestProdFlipCriteriaLock` passed. |
| On this SHA, `*compileHfirBc` still calls `hfir.LowerToBytecode(graph)` | Met | `howlframe.go` `*compileHfirBc` branch. The same lock test passed. |
| The suite has been run on this SHA, with production results as the oracle | Met | The three commands above exited 0 on this SHA. Stdout, stderr, exit, denial class, and the filesystem and request checks inside `TestLoweredHFIRABIConformance` passed. |
| That suite result is the Assurance tip-lock for this SHA | Not met | The lock record is still the older SHA. This journal does not replace `docs/journals/2026-09-30_lowered_hfir_prod_flip_tip_lock.md`. |
| The lock is fresh: neither bytecode compiler changed after the locked SHA | Not met | `git diff 4d74dbcf9654caa05e0b1d9212b15bc5398359e3..5a229d6553588f6916f9fe4d6b596cd1e6fa0de6` changes `internal/bytecode/bytecode.go` (`LOAD_CONST` for an integer literal keeps the `int64` value) and `internal/hfir/bytecode.go` (`literalValue` returns that integer). `internal/vm/vm.go` and the Go and JavaScript backends also changed in that range. The scored commit itself edits `internal/backend/wasm/ssa_serializer.go`. The checklist says the lock expires when either compiler changes, and a new lock is required before a flip PR. |
| A flip PR cites this SHA, the commands, the pass result, every promote row, and every accepted limit still in force | Not met | `*compileBc` is still the AST compiler. This score found no merged flip. |
| The Owner has authorized the flip by merging that PR | Not met | No such merge is in this tree. |

## Flip diff

| Item | Status | Evidence |
| --- | --- | --- |
| After `runHFIRGate`, `*compileBc` calls `hfir.LowerToBytecode` and fails closed on `HFIR_BYTECODE_UNSUPPORTED` with no artifact | Not met | The branch calls `bytecode.CompileToBytecode(root)` and writes that artifact (`howlframe.go`). |
| One emission source, with no leftover `bytecode.CompileToBytecode` subset inside `-compile-bc` | Not met | Production emission is still `bytecode.CompileToBytecode` for the constructs it accepts. |
| The gate's blocking codes stay `HFIR_INVALID_REF` and `HFIR_TARGET_INFEASIBLE` | Met | `internal/hfir/verifier.go` still emits those codes. The lock test passed. The flip diff has not edited the gate. |
| `-compile-hfir-bc` remains spelled the same way | Met | `howlframe.go` still registers `-compile-hfir-bc` and the experimental branch still calls `hfir.LowerToBytecode`. |

## Accepted limits

`TestProdFlipCriteriaLock` passed, including `fetch_body`, `match control edges`, `try control edges`, the transport subtests, and the Wasm set.

| Item | Status | Evidence |
| --- | --- | --- |
| Fetch body stays off `OpFetch` on both bytecode compilers | Met | `OpFetch` pops 2, has an empty operand list, and is `network`. The AST `fetch` case compiles children 1 and 2. The HFIR `fetch` case compiles the `url` and `method` edges and skips a `body` edge. The lock test's `fetch body` subtest requires `FETCH` and requires the body string to be absent from both artifacts. It passed. |
| `match` stays unsupported, with empty control edges and no program | Met | `construct.Lookup("match")` is `Unsupported` in `internal/construct/construct.go`. `lowering.go` does not assign `ControlEdges` for `match`. The lock test expects empty `ControlEdges` and one `HFIR_BYTECODE_UNSUPPORTED` with no `BCProgram`. It passed. Production `compileNode` has no `match` case. |
| `try` emits `TRY_LET` from data edges, and `ControlEdges` stay empty | Met | `try_let` lowers to kind `try` and appends `expression`, `success_body`, and `catch` data edges (`internal/hfir/lowering.go`). It does not set `ControlEdges`. The lock test expects empty `ControlEdges` and `TRY_LET`. It passed. `tests/parity/08_io_cli.howl` is in `hfirSupportedParity`, and supported parity passed. |
| Wasm stays the closed set `exec`, `spawn_agent`, `http_server_start` | Met | `hfir.WasmInfeasibleKinds` in `internal/hfir/abi.go` is those three strings. The lock test requires that slice. It passed. |
| Module linker stays the #106 do-not-build decision | Met | `improvements.md` records #106 as Done with do-not-build-yet, journal `docs/journals/2026-09-30_bytecode_modules_spike.md`. `use`, `export`, and `module` are `CompileTimeOnly` in `internal/construct/construct.go`. `internal/bytecode/opcode.go` has no module opcode. This score adds no linker. |
| Model-adapter transport stays the Phase-1 allow-list | Met | `nodeRoles` returns nil for kinds outside that switch, including `defun`, `call`, `return`, `while`, `for`, `read_file`, `write_file`, `mkdir`, `exec`, `fetch`, `regex_match`, `html_escape`, `attr_escape`, `match`, and `try`. `if` stays in the switch. The lock test's `transport/<kind>` subtests expect `HFIR_TRANSPORT_KIND` and no graph. They passed. `improvements.md` records #88 as Pending. The transport schema in this tree still has no control-edge field. |

## Kill shapes

These rows are Met when the shape is absent from this tree. Absence is the current state. It is not a promote.

| Kill shape | Status | Evidence |
| --- | --- | --- |
| A new opcode, or a new capability, so the experimental subset can cover a promote blocker | Met | `git diff 4d74dbcf9654caa05e0b1d9212b15bc5398359e3..HEAD -- internal/bytecode/opcode.go` is empty. The capability package diff in that range is empty. The fence still has 29 names. |
| A Wasm opcode, Wasm collection, or Wasm host import | Met | The scored commit deletes the `i64` to `f64` conversion before Wasm SSA division in `internal/backend/wasm/ssa_serializer.go`. The diff adds no opcode, collection, or host import. `WasmInfeasibleKinds` is unchanged. |
| A bytecode import section, a VM module opcode, or an HFIR module linker (#106) | Met | No module opcode in `opcode.go`. #106 remains the do-not-build journal cited above. |
| An `OpFetch` body operand, or any other fetch-body implementation | Met | `OpFetch` still pops 2 with an empty operand list. The body string stays out of both artifacts (`fetch body` subtest passed). `docs/reference/fetch_body_bytecode_design.md` is still a design note. |
| Two production emitters inside `-compile-bc` | Met | The `*compileBc` branch contains `bytecode.CompileToBytecode(root)` and the lock test rejects `LowerToBytecode` in that branch. The test passed. |
| Dropping a promote-blocker construct out of production so the experimental subset fits | Met | `regex_match` and `attr_escape` still emit on the AST path in the lock test. The other fence names still have `compileNode` cases, because the fence comparison passed. |
| Widening `nodeRoles` so the transport accepts kinds the source lowerer already emits | Met | The transport subtests passed, including `html_escape`, `defun`, `while`, `for`, `exec`, and `fetch`. |
| Marking #90 Done because the current dogfood subset matches | Met | `improvements.md` and the checklist still say Partial. This score keeps Partial. The GitHub 404 is not a close. |
| Treating this checklist, or any dogfood journal, as the Owner's flip PR | Met | The checklist Decision line stays defer. This journal is the score. Production emission is unchanged. |

## Defer

Verdict: flip deferred. These defer conditions are active on this SHA:

| Defer condition | Status as a cleared promote gate | Evidence |
| --- | --- | --- |
| The promote-blocker fence is empty | Not met | 29 names. The fence comparison passed, which means the block is still that set. |
| `hfirRejectedParity` is empty | Not met | Two files, listed above. |
| The Assurance tip-lock is present and fresh for this SHA | Not met | The `4d74dbcf` record is present and is stale. Both bytecode compilers changed after it. This score does not take a new lock. |
| Mediated host-effect conformance for the six effects agrees across the hosts in the table | Met | The conformance command passed with node `v22.14.0`. This agreement does not clear the other defer conditions. |
| The Owner has merged a flip PR that cites a fresh lock | Not met | `*compileBc` still calls `bytecode.CompileToBytecode`. |

While deferred, further dogfood stays on `-compile-hfir-bc`. This score does not teach `LowerToBytecode` a fenced name. `regex_match` and `attr_escape` remain in the fence. Clearing them would be dogfood. It would not be permission to flip.

## Dogfood path

| Item | Status | Evidence |
| --- | --- | --- |
| Experimental `-compile-hfir-bc` stays the dogfood path until the Owner authorizes a flip PR | Met | The flag is still marked experimental in `howlframe.go`, and its branch still calls `hfir.LowerToBytecode`. |
| Callers who need production bytecode keep using `-compile-bc` | Met | That branch still calls `bytecode.CompileToBytecode(root)`. |
| The two flags share the HFIR gate and differ at emission | Met | Both branches call `runHFIRGate`. Emission differs as above. |

## Checklist acceptance criteria

These rows score the checklist document on this SHA. They are the spike's acceptance criteria. They are not a promote.

| Item | Status | Evidence |
| --- | --- | --- |
| The decision is defer. #90 stays Partial | Met | The Decision section still begins "Defer the production flip." `improvements.md` still says Partial. This score's verdict is flip deferred. |
| Promote names HFIR↔AST parity, mediated host effects, and the Assurance tip-lock | Met | Those sections are still in `docs/reference/lowered_hfir_prod_flip_criteria.md`. |
| The promote-blocker fence lists every production-supported construct the experimental lowerer does not emit, including `regex_match` and `attr_escape`, and `html_escape` is outside it | Met | `TestProdFlipCriteriaLock` matched the block to the recomputed set and passed. |
| Fetch body, `match` / `try` control edges, Wasm, the module linker, and the model-adapter transport rejects are accepted limits with a promote rule | Met | The accepted-limits table is still in the checklist, and the lock test passed. |
| Kill and defer each have their own conditions. The dogfood path stays `-compile-hfir-bc` until the Owner authorizes a flip PR | Met | The checklist still has separate Kill and Defer sections. The dogfood flag is unchanged. |
| Production `-compile-bc` behavior is unchanged by this score | Met | This change edits docs. It does not edit `howlframe.go`, `internal/bytecode`, `internal/hfir`, the VM, or conformance tests. |

## What this score does not do

No production `-compile-bc` flip. No `attr_escape` lowering. No fetch-body implementation. No new opcode. No new capability. No edit to conformance lock tests. No new Assurance tip-lock. The `4d74dbcf` lock stays the named lock, and it stays stale for `5a229d6553588f6916f9fe4d6b596cd1e6fa0de6`. Verdict: flip deferred. #90 stays Partial.
