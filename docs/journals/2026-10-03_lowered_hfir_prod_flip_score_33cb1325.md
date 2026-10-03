# Production `-compile-bc` flip score of `33cb1325`

## Verdict

Verdict: flip deferred.

Scored tree: `main` at `33cb13252bc8e29f4a1bb9d58ce95a82ef6c7358`. Commit subject: `Experimental HFIR attr_escape onto existing ATTR_ESCAPE (#77)`. After `git fetch origin main`, `git rev-parse HEAD` and `git rev-parse origin/main` were that SHA. The checklist is `docs/reference/lowered_hfir_prod_flip_criteria.md`. The Decision line there stays "Defer the production flip."

#90 stays Partial. `improvements.md` still records improvement 90 as Partial. A GitHub API read of `howlcipher/howlframe` issue 90 returned 404. That miss is not a close. This score does not mark #90 Done.

This note is a score of `33cb13252bc8e29f4a1bb9d58ce95a82ef6c7358`. It is a measurement against the checklist. It does not authorize a flip. It does not refresh the Assurance tip-lock. The lock named in the checklist remains `4d74dbcf9654caa05e0b1d9212b15bc5398359e3`, and it is stale for this SHA. This score does not retake it.

`docs/journals/2026-10-03_lowered_hfir_prod_flip_score_5a229d65.md` scores `5a229d6553588f6916f9fe4d6b596cd1e6fa0de6`. That SHA is before experimental `attr_escape`. It is not a score of this tip. This note does not edit or replace that journal.

## How a mark is read

Met means the promote condition holds on this SHA, an accepted limit still matches the checklist, or the named kill shape is absent from this tree. Not met means the promote condition does not hold. Unknown means this score did not verify the row. A Met kill row means that shape is absent. It does not clear the fence.

## What this tip shows

These rows are facts checked in the tree at the scored SHA. They are not a promote.

| Fact | Status | Evidence |
| --- | --- | --- |
| Experimental `attr_escape` lowers onto the existing `ATTR_ESCAPE` opcode on `-compile-hfir-bc` only | Met | `internal/hfir/bytecode.go` has a `case "attr_escape"` that compiles the `value` edge and emits `bytecode.OpAttrEscape` (`ATTR_ESCAPE`) with no string operand. `internal/hfir/lowering.go` gives `(attr_escape text)` kind `attr_escape` and one `value` edge, and the comment says experimental `-compile-hfir-bc` only. `howlframe.go` `*compileHfirBc` calls `hfir.LowerToBytecode(graph)`. `*compileBc` does not. `internal/bytecode/opcode.go` is unchanged from the Assurance tip-lock, so `OpAttrEscape` is the opcode that already existed. `TestProdFlipCriteriaLock`'s `attr_escape` subtest requires `ATTR_ESCAPE` from both `LowerToBytecode` and `bytecode.CompileToBytecode`, and that test passed. |
| Production `-compile-bc` still calls `bytecode.CompileToBytecode` | Met | `howlframe.go` `*compileBc` calls `runHFIRGate` and then `bytecode.CompileToBytecode(root)`. That branch does not call `LowerToBytecode`. The AST `attr_escape` case in `internal/bytecode/bytecode.go` still emits `ATTR_ESCAPE`. The lock test passed. |
| `tests/parity/13_html_escape.howl` left `hfirRejectedParity` | Met | `tools/difftest/difftest_test.go` lists `13_html_escape.howl` in `hfirSupportedParity` and not in `hfirRejectedParity`. The file calls `html_escape` and `attr_escape`. `TestHFIRBytecodeSupportedParity` passed, including that file. `TestHFIRBytecodeMixedEscapeParityFile` passed inside `TestHFIRBytecode`. |
| `regex_match` and `tests/parity/07_strings.howl` still fail closed | Met | `internal/hfir/bytecode.go` has no `regex_match` case. The switch default returns one diagnostic and no instructions. `07_strings.howl` calls `regex_match` and is the only name in `hfirRejectedParity`. `TestHFIRBytecodeRejectsUnsupportedParity` passed. The lock test's `regex_match` subtest expects one `HFIR_BYTECODE_UNSUPPORTED`, no program, and production `REGEX_MATCH`. That test passed. Production `compileNode` still has a `regex_match` case. |
| Fetch body is still unimplemented, and `OpFetch` is untouched | Met | `opcode.go` from the lock to this SHA has an empty diff. `OpFetch` is `FETCH`, pops 2, has an empty operand list, and is `network`. The AST `fetch` case compiles children 1 and 2 and emits `FETCH`. The HFIR `fetch` case compiles the `url` and `method` edges, skips a `body` edge, and emits `FETCH`. Neither diff of those two compilers from the lock touches the fetch case. The lock test's `fetch body` subtest requires `FETCH` and requires the body string `body-not-sent` to be absent from both artifacts. It passed. |
| The Assurance tip-lock is still `4d74dbcf9654caa05e0b1d9212b15bc5398359e3`, and it is stale | Met | The checklist, `docs/journals/2026-09-30_lowered_hfir_prod_flip_tip_lock.md`, and `docs/journals/2026-09-30_assurance_tip_lock_4d74dbcf.md` still name that SHA. `git rev-parse origin/main` on this score was `33cb13252bc8e29f4a1bb9d58ce95a82ef6c7358`. Both bytecode compilers changed after the lock, as the Assurance section records. This score does not write a new lock. |

## Suite on this SHA

Recorded 2026-10-03T18:34:57Z through 2026-10-03T18:35:21Z, on the scored SHA before this journal was added. Go was `go1.22.2 linux/amd64`. Node was `v22.14.0` at `/exec-daemon/node`. Each command exited 0.

```
go test . -count=1 -timeout 180s -run 'TestProdFlipCriteriaLock'
ok  	github.com/howlcipher/howlframe	0.004s

go test ./internal/vm -count=1 -timeout 900s -run 'TestHFIRBytecode'
ok  	github.com/howlcipher/howlframe/internal/vm	0.049s

go test ./tools/difftest -count=1 -timeout 900s -run 'TestLoweredHFIRABIConformance|TestHFIRBytecodeSupportedParity|TestHFIRBytecodeRejectsUnsupportedParity'
ok  	github.com/howlcipher/howlframe/tools/difftest	12.410s
```

`go test ./...` was not run for this score. This note does not claim that bar. The three commands above are the suite the checklist names. A green run on this SHA is score evidence. It is not a new Assurance tip-lock.

After this journal was linked from the criteria doc, `TestProdFlipCriteriaLock` was run again and exited 0 (`ok github.com/howlcipher/howlframe 0.004s`). That repeat checks the linked checklist text. It is not a new Assurance tip-lock.

A second run hid `node` (`PATH=/usr/bin:/bin` plus the Go binary directory, `command -v node` found nothing) and executed `TestLoweredHFIRABIConformance/env_denied` with `-v`. The log line was `javascript skipped: node runtime not available`, and that case passed (`0.506s`).

## HFIR↔AST parity

| Item | Status | Evidence |
| --- | --- | --- |
| Experimental and production artifacts match on every program production `-compile-bc` accepts today | Not met | `hfirRejectedParity` still lists `07_strings.howl`. `TestHFIRBytecodeRejectsUnsupportedParity` passed: `LowerToBytecode` returns one `HFIR_BYTECODE_UNSUPPORTED` and no program, and production `-compile-bc` still runs that file. The promote-blocker fence is non-empty. `13_html_escape.howl` is no longer in the rejected set. |
| Every `tests/conformance/lowered_hfir_abi_v1.json` case that lists `hfir_bytecode` matches the `bytecode` host on normalized stdout, stderr, and exit, including nested `if` / `while` / `defun`, nested `for`, and `nested_multi_effect` under the empty grant, the grant that omits `network`, and the full grant | Met | The manifest has 39 cases. Every case lists `hfir_bytecode`, `bytecode`, `interpreter`, `go`, and `javascript`. Names include `nested_if_while_defun`, `nested_for_if_while_defun`, `nested_multi_effect_denied`, `nested_multi_effect_partial`, `nested_multi_effect_granted`, `attr_escape`, and `attr_escape_type_error`. `TestLoweredHFIRABIConformance` passed with node on `PATH`. |
| `TestHFIRBytecodeSupportedParity` passes, and every file in `tests/parity` is classified | Met | The directory has 13 `*.howl` files. `hfirSupportedParity` has 12 (`01`–`06`, `08`–`13`). `hfirRejectedParity` has `07_strings.howl`. The test's classification check passed inside the difftest command above. |
| `hfirRejectedParity` is empty | Not met | `tools/difftest/difftest_test.go` holds `07_strings.howl` (`regex_match`). `13_html_escape.howl` has left that list. |
| Operand order stays the AST order: `EXEC` is command then arguments with `IntOperand` equal to the argument count; `FETCH` is URL then method; `JUMP_IF_FALSE`, `JUMP`, `FOR_INIT`, `FOR_NEXT`, `CALL`, and `RETURN` keep the relative offsets the AST compiler already emits | Met | `internal/bytecode/bytecode.go` and `internal/hfir/bytecode.go` emit the same shapes. `EXEC` compiles the command and then the arguments, and `IntOperand` is the argument count. `FETCH` compiles the URL and then the method. `if` uses `then+1` or `then+2` plus a `JUMP` of `else+1`. `while` uses `JUMP_IF_FALSE` of `body+2` and a negative back edge of `-(cond+1+body)`. `for` uses `FOR_NEXT` of `body+2`. `CALL` stores the argument count. `RETURN` has no offset operand. `TestHFIRBytecode` passed, including `TestHFIRBytecodeExecOperandOrder` and `TestHFIRBytecodeFetchOperandOrder`. |
| The promote-blocker fence is empty | Not met | `TestProdFlipCriteriaLock` recomputed the fence from `construct.Supported`, `compileNode`, and `LowerToBytecode` and matched the checklist's `promote-blockers` block. That block has 28 names. `html_escape` and `attr_escape` are outside it. |

Names the checklist already places outside the fence, because they lower under another kind: `cli_app` (`program`), `do` (`sequence`), `try_let` (`try`), the arithmetic and comparison heads (`binary`), and `to_int`, `to_float`, `to_string`, `bytes_to_string`, and `encode_json` (`convert`). Status: Met. The same lock test uses that alias map. A missing alias would have failed the fence comparison.

`html_escape` has left the fence onto the existing `HTML_ESCAPE` opcode. `attr_escape` has left the fence onto the existing `ATTR_ESCAPE` opcode, on `-compile-hfir-bc` only. Status: Met. The lock test's `html_escape` and `attr_escape` subtests require those opcodes on both bytecode compilers, and that test passed. Neither name is in the 28-name block.

Each name still in the fence is Not met. Evidence for every row is the same passing fence comparison: the name is in the checklist block and in the recomputed set, so `LowerToBytecode` has no case for it while `compileNode` does. This score did not execute a separate parity program per name.

| Promote blocker | Status |
| --- | --- |
| `achieve` | Not met |
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

`regex_match` has an extra check. The lock test lowers `(regex_match "^a$" "a")`, expects one `HFIR_BYTECODE_UNSUPPORTED` and no program, and expects production bytecode to contain `REGEX_MATCH`. That subtest passed. `attr_escape` is not in this table. Its subtest expects `ATTR_ESCAPE` from both compilers, and that subtest passed.

## Mediated host effects

The bytecode path still uses the Phase 2a–2e mediators. Go and JavaScript still consume the AST. `howlFrameGrantHas` in `internal/backend/gogen/gogen.go` is the helper definition plus the six calls in `howlFrameEnv`, `howlFrameExec`, `howlFrameReadFile`, `howlFrameFetch`, `howlFrameWriteFile`, and `howlFrameMkdir`. The JavaScript helpers check the same six names.

| Effect | Status | Evidence |
| --- | --- | --- |
| `env` / `environment` / `howlFrameEnv` / `ENV` | Met | `opcode.go` `OpEnv` is `ENV`, pops 1, empty operand list, capability `environment`. The Go and JavaScript helpers check `environment` before `Getenv` / `process.env`. Conformance cases `env_denied` and `env_granted` are in the passing suite, including nested `nested_env_read_denied` and `nested_env_read_granted`. |
| `exec` / `process` / `howlFrameExec` / `EXEC` | Met | `OpExec` capability `process`, `IntOperand` argument count. The Go helper spawns only after `process` is present. Cases `exec_denied`, `exec_granted`, `nested_exec_denied`, `nested_exec_granted` are in the passing suite. |
| `read_file` / `filesystem` / `howlFrameReadFile` / `READ_FILE` | Met | `OpReadFile` pops 1, capability `filesystem`. Cases `read_file_denied`, `read_file_granted`, and the nested env/read fixtures are in the passing suite. |
| `write_file` / `filesystem` / `howlFrameWriteFile` / `WRITE_FILE` | Met | `OpWriteFile` pops 2, capability `filesystem`. Cases `write_file_denied` and `write_file_granted` are in the passing suite. The conformance test also checks that denial creates no marker file and that the grant writes the marker body. |
| `mkdir` / `filesystem` / `howlFrameMkdir` / `MKDIR` | Met | `OpMkdir` pops 1, capability `filesystem`. Cases `mkdir_denied` and `mkdir_granted` are in the passing suite, including the directory-created check on grant and the absent-path check on denial. |
| `fetch` / `network` / `howlFrameFetch` / `FETCH` | Met | `OpFetch` is `FETCH`, pops 2, empty operand list, capability `network`. Cases `fetch_denied`, `fetch_granted`, `nested_fetch_denied`, and `nested_fetch_granted` are in the passing suite. The body string stays off the opcode, as the accepted-limit row records. |

| Denial rule | Status | Evidence |
| --- | --- | --- |
| An empty grant is `CAPABILITY_DENIED` before the effect | Met | `deny_all` cases in the manifest expect `CAPABILITY_DENIED`. The VM calls `requireCapability` from the registry capability before the opcode switch (`internal/vm/vm.go`). The panic text is `capability denied:` plus the capability name. Go and JavaScript helpers return that class before the read, spawn, filesystem call, or HTTP call. The suite passed. |
| A missing grant is `CAPABILITY_DENIED` before the effect | Met | Go `howlFrameGrantHas` and the JavaScript helper treat an empty or unset `HOWLFRAME_ALLOW_CAPS` as denial. The suite's generated-host denial sets that variable to empty. `deny_all` omits `-allow-caps` for bytecode. |
| A grant that omits the name is `CAPABILITY_DENIED` before the effect | Met | `nested_multi_effect_partial` grants `environment,filesystem,process` and expects `CAPABILITY_DENIED`, with stdout for the granted effects and `forbid` `phase2d-fetch-marker`. That case passed. The grant helper denies when the name is absent from the list. |
| Denial text omits the secret, the command, the path, and the URL | Met | Denied cases set `forbid` to the marker or secret (`phase1-token`, `phase2b-exec-marker`, `phase2c-read-marker`, `phase2d-fetch-marker`, `phase2e-write-marker`). The multi-effect cases also reject a blob that contains the fetch URL, the dropped body string, or the multi-effect path prefix. Those checks passed inside `TestLoweredHFIRABIConformance`. The VM denial format is the capability name. |
| The six effects, including nested and multi-effect fixtures, pass on `interpreter`, `bytecode`, `hfir_bytecode`, `go`, and `javascript` when `node` is on `PATH` | Met | Full difftest command above, node `v22.14.0`. |
| A missing `node` is `BACKEND_UNSUPPORTED` for JavaScript only | Met | `executeJSBackend` returns `BACKEND_UNSUPPORTED` with `node runtime not available` when `exec.LookPath("node")` fails. The conformance loop skips only that JavaScript result. The `env_denied` rerun with node hidden logged that skip and passed, so the other hosts for that case still matched. The other 38 cases were not re-run with node hidden. |
| A present `node` that disagrees fails the lock | Met | With node present, the full conformance run passed, so JavaScript agreed with the oracle on every case. `conformance_test.go` calls `t.Errorf` when a target's error class, exit, stdout, or forbidden marker disagrees, and `t.Fatalf` when parity disagrees. This score did not inject a broken node binary. |
| Generated host effects outside the mediator table stay unmediated | Met | `howlFrameGrantHas` is referenced only by the six helpers. `spawn`, `regex_match`, and `sleep` in `gogen.go` have no grant check. |
| The flip PR leaves those outside-table effects unmediated | Not met | No flip PR is in this tree. Production `-compile-bc` is still `bytecode.CompileToBytecode`. |
| Moving Go and JavaScript onto one lowered graph stays later than this bytecode flip | Met | Both generators still walk the AST. #90 stays Partial in `improvements.md`. |
| #90 stays Partial either way | Met | See the verdict. The GitHub 404 is not a close. |

## Assurance tip-lock

| Item | Status | Evidence |
| --- | --- | --- |
| `origin/main` on this score is the recorded Assurance tip-lock | Not met | `git rev-parse origin/main` was `33cb13252bc8e29f4a1bb9d58ce95a82ef6c7358`. The checklist, `docs/journals/2026-09-30_lowered_hfir_prod_flip_tip_lock.md`, and `docs/journals/2026-09-30_assurance_tip_lock_4d74dbcf.md` name `4d74dbcf9654caa05e0b1d9212b15bc5398359e3`. |
| On this SHA, `*compileBc` still calls `bytecode.CompileToBytecode(root)` and does not call `LowerToBytecode` | Met | `howlframe.go` `*compileBc` branch calls `runHFIRGate` and then `bytecode.CompileToBytecode(root)`. `TestProdFlipCriteriaLock` passed. |
| On this SHA, `*compileHfirBc` still calls `hfir.LowerToBytecode(graph)` | Met | `howlframe.go` `*compileHfirBc` branch. The same lock test passed. |
| The suite has been run on this SHA, with production results as the oracle | Met | The three commands above exited 0 on this SHA. Stdout, stderr, exit, denial class, and the filesystem and request checks inside `TestLoweredHFIRABIConformance` passed. |
| That suite result is the Assurance tip-lock for this SHA | Not met | The lock record is still the older SHA. This journal does not replace `docs/journals/2026-09-30_lowered_hfir_prod_flip_tip_lock.md`. |
| The lock is fresh: neither bytecode compiler changed after the locked SHA | Not met | `git diff 4d74dbcf9654caa05e0b1d9212b15bc5398359e3..33cb13252bc8e29f4a1bb9d58ce95a82ef6c7358` changes `internal/bytecode/bytecode.go` (an integer `LOAD_CONST` keeps the `int64` value, and `extractParamNames` calls `ast.ParamNames`) and `internal/hfir/bytecode.go` (`literalValue` returns that integer, and `attr_escape` emits the existing `ATTR_ESCAPE`). `internal/bytecode/opcode.go` is unchanged in that range. `internal/vm/vm.go` and the Go and JavaScript backends also changed. The checklist says the lock expires when either compiler changes, including a dogfood slice that teaches `LowerToBytecode` a new kind, and a new lock is required before a flip PR. This score does not take that lock. |
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

`TestProdFlipCriteriaLock` passed, including `fetch body`, `match control edges`, `try control edges`, the transport subtests, and the Wasm set.

| Item | Status | Evidence |
| --- | --- | --- |
| Fetch body stays off `OpFetch` on both bytecode compilers | Met | `OpFetch` pops 2, has an empty operand list, and is `network`. The AST `fetch` case compiles children 1 and 2. The HFIR `fetch` case compiles the `url` and `method` edges and skips a `body` edge. The lock test's `fetch body` subtest requires `FETCH` and requires the body string to be absent from both artifacts. It passed. `docs/reference/fetch_body_bytecode_design.md` is still the design note the checklist names. This score does not implement that design. |
| `match` stays unsupported, with empty control edges and no program | Met | `construct.Lookup("match")` is `Unsupported` in `internal/construct/construct.go`. `lowering.go` does not assign `ControlEdges` for `match`. The lock test expects empty `ControlEdges` and one `HFIR_BYTECODE_UNSUPPORTED` with no `BCProgram`. It passed. Production `compileNode` has no `match` case. |
| `try` emits `TRY_LET` from data edges, and `ControlEdges` stay empty | Met | `try_let` lowers to kind `try` and appends `expression`, `success_body`, and `catch` data edges (`internal/hfir/lowering.go`). It does not set `ControlEdges`. The lock test expects empty `ControlEdges` and `TRY_LET`. It passed. `tests/parity/08_io_cli.howl` is in `hfirSupportedParity`, and supported parity passed. |
| Wasm stays the closed set `exec`, `spawn_agent`, `http_server_start` | Met | `hfir.WasmInfeasibleKinds` in `internal/hfir/abi.go` is those three strings. The lock test requires that slice. It passed. |
| Module linker stays the #106 do-not-build decision | Met | `improvements.md` records #106 as Done with do-not-build-yet, journal `docs/journals/2026-09-30_bytecode_modules_spike.md`. `use`, `export`, and `module` are `CompileTimeOnly` in `internal/construct/construct.go`. `internal/bytecode/opcode.go` has no module opcode, and that file is unchanged from the lock. This score adds no linker. |
| Model-adapter transport stays the Phase-1 allow-list | Met | `nodeRoles` returns nil for kinds outside that switch, including `defun`, `call`, `return`, `while`, `for`, `read_file`, `write_file`, `mkdir`, `exec`, `fetch`, `regex_match`, `html_escape`, `attr_escape`, `match`, and `try`. `if` stays in the switch. `transportNode` has `id`, `kind`, `value`, `literal_kind`, `inputs`, and `provenance`. It has no control-edge field. The lock test's `transport/<kind>` subtests expect `HFIR_TRANSPORT_KIND` and no graph. They passed. `improvements.md` records #88 as Pending. |

## Kill shapes

These rows are Met when the shape is absent from this tree. Absence is the current state. It is not a promote.

| Kill shape | Status | Evidence |
| --- | --- | --- |
| A new opcode, or a new capability, so the experimental subset can cover a promote blocker | Met | `git diff 4d74dbcf9654caa05e0b1d9212b15bc5398359e3..HEAD -- internal/bytecode/opcode.go` is empty. The capability package diff in that range is empty. `attr_escape` emits the existing `OpAttrEscape`. The fence still has 28 names. |
| A Wasm opcode, Wasm collection, or Wasm host import | Met | The wasm package diff from the lock to this SHA is `internal/backend/wasm/ssa_serializer_test.go` only. `ssa_serializer.go` has an empty net diff in that range. The test diff adds no opcode, collection, or host import. `WasmInfeasibleKinds` is unchanged. |
| A bytecode import section, a VM module opcode, or an HFIR module linker (#106) | Met | No module opcode in `opcode.go`. #106 remains the do-not-build journal cited above. |
| An `OpFetch` body operand, or any other fetch-body implementation | Met | `OpFetch` still pops 2 with an empty operand list. The body string stays out of both artifacts (`fetch body` subtest passed). `docs/reference/fetch_body_bytecode_design.md` is still a design note. |
| Two production emitters inside `-compile-bc` | Met | The `*compileBc` branch contains `bytecode.CompileToBytecode(root)` and the lock test rejects `LowerToBytecode` in that branch. The test passed. |
| Dropping a promote-blocker construct out of production so the experimental subset fits | Met | `regex_match` still emits on the AST path in the lock test. `attr_escape` still emits `ATTR_ESCAPE` on that path and now also emits it from `LowerToBytecode`. The other fence names still have `compileNode` cases, because the fence comparison passed. |
| Widening `nodeRoles` so the transport accepts kinds the source lowerer already emits | Met | The transport subtests passed, including `html_escape`, `attr_escape`, `defun`, `while`, `for`, `exec`, and `fetch`. |
| Marking #90 Done because the current dogfood subset matches | Met | `improvements.md` and the checklist still say Partial. This score keeps Partial. The GitHub 404 is not a close. |
| Treating this checklist, or any dogfood journal, as the Owner's flip PR | Met | The checklist Decision line stays defer. This journal is the score. Production emission is unchanged. |

## Defer

Verdict: flip deferred. These defer conditions are active on this SHA:

| Defer condition | Status as a cleared promote gate | Evidence |
| --- | --- | --- |
| The promote-blocker fence is empty | Not met | 28 names. The fence comparison passed, which means the block is still that set. `attr_escape` has left it. `regex_match` has not. |
| `hfirRejectedParity` is empty | Not met | One file, `07_strings.howl`. `13_html_escape.howl` has left the list. |
| The Assurance tip-lock is present and fresh for this SHA | Not met | The `4d74dbcf` record is present and is stale. Both bytecode compilers changed after it. This score does not take a new lock. |
| Mediated host-effect conformance for the six effects agrees across the hosts in the table | Met | The conformance command passed with node `v22.14.0`. This agreement does not clear the other defer conditions. |
| The Owner has merged a flip PR that cites a fresh lock | Not met | `*compileBc` still calls `bytecode.CompileToBytecode`. |

While deferred, further dogfood stays on `-compile-hfir-bc`. This score does not teach `LowerToBytecode` a fenced name. The scored tip already taught it `attr_escape` on that flag only. `regex_match` remains in the fence. Clearing it would be dogfood. It would not be permission to flip.

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
| The promote-blocker fence lists every production-supported construct the experimental lowerer does not emit, including `regex_match`, and `html_escape` and `attr_escape` are outside it | Met | `TestProdFlipCriteriaLock` matched the block to the recomputed set and passed. `attr_escape` is not in the block. `regex_match` is. |
| Fetch body, `match` / `try` control edges, Wasm, the module linker, and the model-adapter transport rejects are accepted limits with a promote rule | Met | The accepted-limits table is still in the checklist, and the lock test passed. |
| Kill and defer each have their own conditions. The dogfood path stays `-compile-hfir-bc` until the Owner authorizes a flip PR | Met | The checklist still has separate Kill and Defer sections. The dogfood flag is unchanged. |
| Production `-compile-bc` behavior is unchanged by this score | Met | This change edits docs. It does not edit `howlframe.go`, `internal/bytecode`, `internal/hfir`, the VM, or conformance tests. |

## What this score does not do

No production `-compile-bc` flip. No `regex_match` lowering. No fetch-body implementation. No new opcode. No new capability. No edit to conformance lock tests. No new Assurance tip-lock. The `4d74dbcf` lock stays the named lock, and it stays stale for `33cb13252bc8e29f4a1bb9d58ce95a82ef6c7358`. The `5a229d65` journal stays the score of that earlier SHA. Verdict: flip deferred. #90 stays Partial.
