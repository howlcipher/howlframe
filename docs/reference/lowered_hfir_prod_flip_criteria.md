# Production `-compile-bc` flip criteria (#90)

Version: checklist beside `lowered-hfir-abi/v1`. #90 stays Partial.

## Decision

Defer the production flip. Production `-compile-bc` stays Path 1 from HOWL-CANON-010: `runHFIRGate`, then `bytecode.CompileToBytecode` on the checked AST (`howlframe.go`, the `*compileBc` branch). Experimental `-compile-hfir-bc` stays Path 2: the same gate, then `hfir.LowerToBytecode`. That flag is the dogfood path until the Owner authorizes a flip PR.

This note is the kill, defer, and promote checklist for that future PR. Publishing it leaves the emission source where it is. The recorded tip is `a9f00bcc91680abe505616fb9b9ed6e643ffa3bc` on `main` (PR #69). That SHA is the baseline this checklist was written against. It is not an Assurance release, and it does not authorize a flip.

The ABI text is `docs/reference/lowered_hfir_abi_v1.md`. The suite is `tests/conformance/lowered_hfir_abi_v1.json`. The journal for this spike is `docs/journals/2026-09-30_lowered_hfir_prod_flip_criteria.md`.

## Promote

A flip PR may change the `*compileBc` branch only when every row below is true on the Assurance tip-lock for that PR. A missing row defers. This document is not that authorization. After a flip that meets these rows, #90 stays Partial: one lowered graph for every host is still Phase 2.

### HFIR↔AST parity evidence

The experimental artifact and the production artifact match on every program production `-compile-bc` accepts today.

* Every case in `tests/conformance/lowered_hfir_abi_v1.json` that lists `hfir_bytecode` matches the `bytecode` host on normalized stdout, stderr, and exit. That includes the nested `if` / `while` / `defun` fixture, the nested `for` fixture, and `nested_multi_effect` under the empty grant, the grant that omits `network`, and the full grant.
* `TestHFIRBytecodeSupportedParity` passes, and every file in `tests/parity` is classified as supported or rejected. An unclassified file blocks.
* `hfirRejectedParity` is empty. Today it holds `07_strings.howl` (`regex_match`) and `13_html_escape.howl` (`attr_escape`; the file also calls `html_escape`, which now lowers). Those two files block.
* Operand order stays the AST order on the existing opcodes: `EXEC` is command then arguments, with `IntOperand` equal to the argument count; `FETCH` is URL then method. `JUMP_IF_FALSE`, `JUMP`, `FOR_INIT`, `FOR_NEXT`, `CALL`, and `RETURN` keep the relative offsets the AST compiler already emits.
* The promote-blocker fence below is empty. Each name leaves the fence only after `LowerToBytecode` emits that construct's existing opcode and a parity case matches production `-compile-bc` on grant and on denial when the construct has a capability.

These names already lower under a different HFIR kind, so they are outside the fence: `cli_app` is `program`, `do` is `sequence`, `try_let` is `try`, the arithmetic and comparison heads are `binary`, and `to_int`, `to_float`, `to_string`, `bytes_to_string`, and `encode_json` are `convert`.

### Mediated host effects

The bytecode flip keeps the host mediators that Phase 2a–2e already shipped. It does not retarget Go, JavaScript, or the interpreter onto HFIR.

| Effect | Grant | Mediator | Opcode |
| --- | --- | --- | --- |
| `env` | `environment` | `howlFrameEnv` | `ENV` |
| `exec` | `process` | `howlFrameExec` | `EXEC` |
| `read_file` | `filesystem` | `howlFrameReadFile` | `READ_FILE` |
| `write_file` | `filesystem` | `howlFrameWriteFile` | `WRITE_FILE` |
| `mkdir` | `filesystem` | `howlFrameMkdir` | `MKDIR` |
| `fetch` | `network` | `howlFrameFetch` | `FETCH` |

An empty grant, a missing grant, and a grant that omits the name are `CAPABILITY_DENIED` before the effect. The denial text omits the secret, the command, the path, and the URL. The conformance cases for those six effects, including the nested and multi-effect fixtures, pass on `interpreter`, `bytecode`, `hfir_bytecode`, `go`, and `javascript` when `node` is on `PATH`. A missing `node` is `BACKEND_UNSUPPORTED` for JavaScript only. A present `node` that disagrees fails the lock.

Generated host effects outside that table stay unmediated. The flip PR leaves them unmediated. Moving Go and JavaScript onto one lowered graph is the rest of Phase 2 and is a later bar than this bytecode flip. #90 stays Partial either way.

### Assurance tip-lock

The Assurance tip-lock is the measurement record for the flip PR. It is a named `main` SHA plus the suite result on that SHA. Dogfood journals do not refresh it.

1. Fetch `main` and record `git rev-parse origin/main`. That commit is the tip. On the tip, the `*compileBc` branch still calls `bytecode.CompileToBytecode(root)` and the `*compileHfirBc` branch still calls `hfir.LowerToBytecode(graph)`. `TestProdFlipCriteriaLock` fails when that stops being true before the flip PR itself.
2. Run the suite below on that tip. Production `-compile-bc` results are the oracle. Experimental `-compile-hfir-bc` results are the candidate. Stdout, stderr, exit, capability-denial class, and the filesystem and request side effects the suite already records must match.
3. The lock expires when either compiler changes after the SHA, including a dogfood slice that teaches `LowerToBytecode` a new kind. Take a new lock before the flip PR.
4. The flip PR cites the SHA, the commands, and the pass result, and it checks every promote row and names every accepted limit that is still in force.
5. The Owner authorizes the flip by merging that PR. No journal, including this one, is that merge.

Suite the lock runs:

```
go test ./tools/difftest -count=1 -timeout 900s -run 'TestLoweredHFIRABIConformance|TestHFIRBytecodeSupportedParity|TestHFIRBytecodeRejectsUnsupportedParity'
go test ./internal/vm -count=1 -timeout 900s -run 'TestHFIRBytecode'
go test . -count=1 -timeout 180s -run 'TestProdFlipCriteriaLock'
```

`go test ./...` remains the CI bar for the tree that contains the lock.

### Flip diff

The production-emission edit is the `*compileBc` branch. After `runHFIRGate` returns the graph, that branch calls `hfir.LowerToBytecode` and fails closed on `HFIR_BYTECODE_UNSUPPORTED` with no artifact. One emission source: the branch does not keep `bytecode.CompileToBytecode` for a leftover subset of constructs. The gate's blocking codes stay `HFIR_INVALID_REF` and `HFIR_TARGET_INFEASIBLE`.

`-compile-hfir-bc` remains in that PR, so the dogfood flag is still spelled the same way. The Owner may retire the flag in a later change.

## Fail-closed and accepted limits

### Promote blockers

These `construct.Supported` names have a `compileNode` case and no `LowerToBytecode` case. Production `-compile-bc` emits an artifact. The experimental lowerer returns one `HFIR_BYTECODE_UNSUPPORTED` diagnostic and no `BCProgram`. A flip fails those programs closed. Each name stays a blocker until the experimental path emits the existing opcode and parity matches. Adding a new opcode to clear a name is a kill, below.

`regex_match` and `attr_escape` remain in this fence. `html_escape` left it when experimental `-compile-hfir-bc` started emitting the existing `HTML_ESCAPE` opcode with parity against production `-compile-bc`. `tests/parity/07_strings.howl` and `tests/parity/13_html_escape.howl` are the current rejected parity files. `13_html_escape.howl` still blocks because it also calls `attr_escape`. The same fence holds the other production bytecode constructs the dogfood subset never emitted: stores, the HTTP server, spawn, database, model calls, `time_now`, `sleep`, and `read_line`.

`TestProdFlipCriteriaLock` recomputes this fence from `internal/construct`, `internal/bytecode/bytecode.go`, and `internal/hfir/bytecode.go`. The list below is that fence. The recorded tip in the Decision section is the baseline the checklist was written against. The fence moves when a dogfood slice teaches `LowerToBytecode` an existing opcode.

```promote-blockers
achieve
attr_escape
confidence
db_connect
ephemeral_circuit
http_server
lazy_synthesize
llm_generate
neural_circuit
optimize_block
optimize_signature
read_line
regex_match
req_method
res
res_header
res_json
schema_bridge
sleep
spawn
spawn_agent
sql_query
store_delete
store_get
store_keys
store_open
store_put
task
time_now
```

### Accepted limits

These stay as they are on both bytecode compilers. A flip PR that implements one of them in order to flip is outside this checklist. A flip PR that makes the two compilers disagree on one of them fails parity and defers.

| Item | What both bytecode compilers do | Promote rule |
| --- | --- | --- |
| Fetch body | `(fetch url method body)` may lower a `body` edge. `OpFetch` pops 2, has an empty operand list, and is `network`. `bytecode.CompileToBytecode` compiles the URL and the method. `LowerToBytecode` compiles the `url` edge and the `method` edge. Neither artifact contains the body string. The interpreter, Go, and JavaScript can send a body when that call runs. | Accepted shared bytecode limit. The body stays off `OpFetch`. A later body design is a separate change on both compilers together. |
| `match` control edges | `construct.Lookup("match")` is `Unsupported`. There is no match opcode. `LowerAST` leaves `ControlEdges` empty. `LowerToBytecode` returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. Production `compileNode` has no `match` case. | Accepted shared fail-closed limit. A promote that claims a full CFG while `match` edges are empty is refused. |
| `try` control edges | `try_let` lowers to kind `try` and emits the existing `TRY_LET` opcode from data edges (`expression`, `success_body`, `catch`). `ControlEdges` stay empty. `tests/parity/08_io_cli.howl` is in the supported parity set. | Accepted graph limit. Parity on `TRY_LET` must hold. Filling `ControlEdges` is later CFG work. A promote that claims SSA while `try` edges are empty is refused. #90 stays Partial. |
| Wasm | Wasm is not an execution host in v1. `hfir.WasmInfeasibleKinds` is `exec`, `spawn_agent`, and `http_server_start`. The diagnostic is `HFIR_TARGET_INFEASIBLE`. | Accepted limit. The flip PR leaves Wasm on that closed set. |
| Module linker | #106 stays the do-not-build decision. `use`, `export`, and `module` stay `CompileTimeOnly`. `ast.ResolveModules` runs before either bytecode compiler. The VM has no module opcode. | Accepted limit. The flip PR leaves linking on the AST. |
| Model-adapter transport | `DecodeCandidate` rejects a kind outside `nodeRoles` with `HFIR_TRANSPORT_KIND` and no graph. The rejected kinds include `defun`, `call`, `return`, `while`, `for`, `read_file`, `write_file`, `mkdir`, `exec`, `fetch`, `regex_match`, `html_escape`, `attr_escape`, `match`, and `try`. `if` stays accepted. The transport schema has no control-edge field. #88 stays Pending. | Accepted limit. The flip reads source through `LowerAST`. It leaves the transport allow-list on the Phase-1 adapter subset. |

## Kill

End the flip track, and keep production emission on `bytecode.CompileToBytecode`, when a proposed PR uses any of these shapes to justify the flip:

* A new opcode, or a new capability, so the experimental subset can cover a promote blocker.
* A Wasm opcode, Wasm collection, or Wasm host import (#73, #84, and #92 stay behind the rest of Phase 2).
* A bytecode import section, a VM module opcode, or an HFIR module linker (#106).
* An `OpFetch` body operand, or any other fetch-body implementation.
* Two production emitters inside `-compile-bc`, with some constructs left on `bytecode.CompileToBytecode`.
* Dropping a promote-blocker construct out of production so the experimental subset fits. `regex_match` and `attr_escape` keep working on `-compile-bc` until they lower onto the opcodes they already have. `html_escape` already lowers onto `HTML_ESCAPE` on the experimental path and still works on `-compile-bc`.
* Widening `nodeRoles` so the model-adapter transport accepts kinds the source lowerer already emits. That widening is #88.
* Marking #90 Done because the current dogfood subset matches. One lowered graph for every host is still open.
* Treating this checklist, or any dogfood journal, as the Owner's flip PR.

A killed track leaves `-compile-hfir-bc` as research. #90 stays Partial until a separate decision says otherwise.

## Defer

Defer is the decision at the recorded tip, and it stays the decision while any of these is true:

* The promote-blocker fence is non-empty.
* `hfirRejectedParity` is non-empty.
* The Assurance tip-lock is missing or stale.
* Mediated host-effect conformance for `env`, `exec`, `read_file`, `write_file`, `mkdir`, or `fetch` disagrees across the hosts in the table above.
* The Owner has not merged a flip PR that cites a fresh lock.

While deferred, further dogfood lands on `-compile-hfir-bc` only. A slice that teaches the lowerer an existing opcode for a fenced name may shrink the fence. That slice still leaves `*compileBc` on `bytecode.CompileToBytecode`, and #90 stays Partial.

Clearing the fence is the promote path for the names still in it: lower `regex_match` onto `REGEX_MATCH`, and `attr_escape` onto `ATTR_ESCAPE`, with parity. `html_escape` has left the fence onto `HTML_ESCAPE` with parity on `-compile-hfir-bc`. That work is dogfood. It is not permission to flip.

## Dogfood path

Experimental `-compile-hfir-bc` stays the dogfood path until the Owner authorizes a flip PR. Callers who need production bytecode keep using `-compile-bc`. The two flags share the HFIR gate and differ at emission. Equivalence tests and `tools/difftest` compare them. Matching stdout on the subset the lowerer already emits is evidence for a future lock. It leaves production emission on the AST.

## What this spike does

It records the checklist and locks it with `TestProdFlipCriteriaLock`. The test recomputes the promote-blocker fence, checks the `*compileBc` branch still calls `bytecode.CompileToBytecode`, checks `OpFetch` still pops 2 with no body operand, checks the fetch body string is absent from both compilers' artifacts, checks `regex_match` and `attr_escape` still fail closed on the experimental path and still emit on the AST path, checks `html_escape` emits `HTML_ESCAPE` on both paths and sits outside the fence, checks the model-adapter transport still rejects `html_escape`, checks `match` and `try` control edges stay empty, checks the model-adapter transport still rejects the kinds in the accepted-limit row, and checks `hfir.WasmInfeasibleKinds` stays the three Wasm host effects.

## Acceptance criteria

1. The decision is defer. #90 stays Partial.
2. Promote names HFIR↔AST parity, mediated host effects, and the Assurance tip-lock.
3. The promote-blocker fence lists every production-supported construct the experimental lowerer does not emit, including `regex_match` and `attr_escape`. `html_escape` is outside the fence.
4. Fetch body, `match` / `try` control edges, Wasm, the module linker, and the model-adapter transport rejects are accepted limits with an explicit promote rule.
5. Kill and defer each have their own conditions. The dogfood path stays `-compile-hfir-bc` until the Owner authorizes a flip PR.
6. Production `-compile-bc` behavior is unchanged.
