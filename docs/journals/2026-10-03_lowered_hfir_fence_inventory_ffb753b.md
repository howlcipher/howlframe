# Fence inventory of `ffb753b`

This is a fence inventory of `main` at `ffb753b970c385aadef1a05ebaa253c9186e6dbc`. Commit subject: `Docs: score prod-flip readiness at 2e5c8e98 (#80)`. It is not a flip verdict. It is not a readiness score of a newer tip. It does not replace `docs/journals/2026-10-03_lowered_hfir_prod_flip_score_5a229d65.md`, `docs/journals/2026-10-03_lowered_hfir_prod_flip_score_33cb1325.md`, or `docs/journals/2026-10-03_lowered_hfir_prod_flip_score_2e5c8e98.md`.

The Decision line in `docs/reference/lowered_hfir_prod_flip_criteria.md` stays "Defer the production flip." Production `-compile-bc` stays `runHFIRGate` and then `bytecode.CompileToBytecode(root)` (`howlframe.go`, the `*compileBc` branch). Experimental `-compile-hfir-bc` stays `hfir.LowerToBytecode(graph)`. This note does not flip production. #90 stays Partial. It does not mark #90 Done. It does not retake the Assurance tip-lock `4d74dbcf9654caa05e0b1d9212b15bc5398359e3`.

The Owner can name one next experimental slice from group 1. This note does not implement that slice.

## Fence count

The promote-blocker block in the checklist has 27 names. `TestProdFlipCriteriaLock` recomputes that block from `construct.Supported`, the `compileNode` cases in `internal/bytecode/bytecode.go`, and the `LowerToBytecode` cases in `internal/hfir/bytecode.go`. `html_escape`, `attr_escape`, and `regex_match` are outside it. None of the 27 names below is a `LowerToBytecode` case. Each one hits the default diagnostic `node kind "<name>" is not in the Phase-1 executable subset` and returns no `BCProgram`.

`tools/difftest/difftest_test.go` declares `var hfirRejectedParity []string`. That list is empty. An empty `hfirRejectedParity` list is not a production flip while this fence is non-empty. The fence count on this SHA is 27.

`LowerAST` does not give these 27 names a semantic case. They fall through to the generic list lowerer, which sets `Kind` to the head and appends each remaining child as a data edge with an empty name (`internal/hfir/lowering.go`). The comment on `lowerSemanticList` says that generic fallback is not consumed by the direct bytecode lowerer.

## Group 1 — existing opcode, same shape as `regex_match` onto `REGEX_MATCH`

Three names. Each already has one opcode. `capability.ForConstruct` returns none. The opcode registry grants nothing. `LowerToBytecode` has no case. A later `-compile-hfir-bc` slice can emit that opcode the way `regex_match` emits `REGEX_MATCH`: no new opcode, no new capability. This inventory does not add the case.

| Name | Opcode | `LowerToBytecode` case | Parity check |
| --- | --- | --- | --- |
| `time_now` | `TIME_NOW` | no | Instruction identity of one `TIME_NOW`. There is no data edge. Do not compare two live Unix timestamps. |
| `sleep` | `SLEEP` | no | One duration edge, then `SLEEP`. Pops 1, pushes 0, empty operand list, no capability. |
| `read_line` | `READ_LINE` | no | Instruction identity of one `READ_LINE`. There is no data edge. Do not compare two stdin lines. |

### `time_now`

Motoko: `time_now` is still in the fence, `LowerToBytecode` has no case, the AST compiler already emits `TIME_NOW`, and parity is instruction identity rather than two live timestamps. Kept.

The checklist block still lists `time_now`. `internal/hfir/bytecode.go` has no `case "time_now"`. On `(cli_app (print (time_now)))`, `LowerAST` gives kind `time_now` with zero data edges, and `LowerToBytecode` returns one `HFIR_BYTECODE_UNSUPPORTED` diagnostic: `node kind "time_now" is not in the Phase-1 executable subset`.

`internal/bytecode/bytecode.go` does this:

```go
case "time_now":
    insts = append(insts, BCInstruction{OpString: "TIME_NOW", Op: OpTimeNow})
```

That program’s production main is `TIME_NOW` then `PRINT`. `internal/bytecode/opcode.go` defines `TIME_NOW` as pops 0, pushes 1, empty operand list, no capability. `capability.ForConstruct("time_now")` is none. The checker requires `(time_now)` with no arguments and types it `int`. The VM does `vm.push(int64(time.Now().Unix()))`. The interpreter returns `int64(time.Now().Unix())`. Two live runs print two clocks. The parity check is that both artifacts contain one `TIME_NOW` and no timestamp constant.

### `sleep`

Hange: `sleep` lowers onto existing `SLEEP` (one duration edge, pops 1, no capability, no new opcode). Kept as the shape of a later slice. `LowerToBytecode` does not lower `sleep` on this SHA.

`internal/hfir/bytecode.go` has no `case "sleep"`. On `(cli_app (sleep 0) (print "ok"))`, `LowerAST` gives kind `sleep` one data edge. The edge name is empty. The source is an `INT` const `0`. `LowerToBytecode` returns `node kind "sleep" is not in the Phase-1 executable subset` and no program.

The AST compiler compiles the duration child and then emits `SLEEP`:

```go
case "sleep":
    insts = append(insts, c.compileNode(node.Children[1])...)
    insts = append(insts, BCInstruction{OpString: "SLEEP", Op: OpSleep})
```

Production main for that program is `LOAD_CONST`, `SLEEP`, `LOAD_CONST`, `PRINT`. `SLEEP` is pops 1, pushes 0, empty operand list, no capability. `capability.ForConstruct("sleep")` is none. The interpreter requires `(sleep ms)` and calls `time.Sleep`. The VM pops one value, requires a number, and sleeps that many milliseconds. There is no new opcode to add.

The one-edge parity check compiles that duration edge and compares the instruction stream with production: the duration’s instructions, then one `SLEEP`. A live check can use duration `0` and a following `print`, so both compilers print the same line. Wall-clock delay is not the oracle. A slice that names the edge would be naming an edge the generic lowerer currently leaves blank. This note does not name it.

### `read_line`

`READ_LINE` is the same nullary shape as `TIME_NOW`. `internal/hfir/bytecode.go` has no `case "read_line"`. On `(cli_app (print (read_line)))`, kind `read_line` has zero data edges, and `LowerToBytecode` rejects it with `node kind "read_line" is not in the Phase-1 executable subset`.

The AST compiler emits a bare instruction:

```go
case "read_line":
    insts = append(insts, BCInstruction{OpString: "READ_LINE", Op: OpReadLine})
```

Production main is `READ_LINE` then `PRINT`. The registry row is pops 0, pushes 1, empty operand list, no capability. `capability.ForConstruct("read_line")` is none. The checker expects zero arguments and types the result `string`. The VM reads one line from `lineReader`. The interpreter does the same. The parity check is one `READ_LINE` in both artifacts. Two live runs would consume two lines.

## Group 2 — new opcode, new capability, or a host effect

Twenty-four names. A `regex_match`-shaped slice does not clear them. This note does not implement any of them. Fetch body is the known example of this class and is not itself a fence name: `OpFetch` is `FETCH`, pops 2, empty operand list, capability `network`. `bytecode.CompileToBytecode` compiles the URL and the method. `LowerToBytecode` compiles the `url` edge and the `method` edge. Neither artifact contains the body. The design note is `docs/reference/fetch_body_bytecode_design.md`. It stays design.

Lain: the remaining names are host-effect rather than existing-opcode. Discarded as a blanket. Twenty of these twenty-four already have an opcode in `internal/bytecode/opcode.go`. Four emit no opcode of their own: `lazy_synthesize`, `optimize_block`, `optimize_signature`, and `schema_bridge`. Those four match today’s AST compiler without a new opcode, because the AST compiler already emits none for the wrapper. They are in this group because there is no existing opcode to lower onto.

| Name | What the tree already has | Why this is not the `regex_match` shape |
| --- | --- | --- |
| `achieve` | `ACHIEVE`, pops 2, pushes 1, capability `network` | AST stringifies both children into `LOAD_CONST`, then `ACHIEVE`. The VM posts to `http://localhost:11434/api/generate`. Host effect. |
| `confidence` | `CONFIDENCE`, pops 1, pushes 1, no capability | One unnamed data edge. AST compiles that child, then `CONFIDENCE`. The VM posts to the same URL. `ForConstruct("confidence")` is none, and the interpreter does not add `network` here. Host effect. |
| `db_connect` | `DB_CONNECT`, pops 0, three string operands, capability `database` | Opaque. The AST compiler reads child `.Value` and does not compile edges. |
| `ephemeral_circuit` | `EPHEMERAL_CIRCUIT`, pops variable, one int operand, opcode capability none | The interpreter calls `requireCapability(network)` for this head. The VM creates a model, deletes it, and generates, all over HTTP. A grant on the opcode would be a capability change. |
| `http_server` | `HTTP_SERVER_START`, `HTTP_ROUTE`, `HTTP_SERVER_SERVE`, each capability `network` | Three opcodes, plus inlined route bodies. `ForConstruct("http_server")` is none. `hfir.WasmInfeasibleKinds` includes `http_server_start`. |
| `lazy_synthesize` | no opcode | Registers a `BCFunction` with `LazySynthesize`, params, and a docstring. Emits no instruction. |
| `llm_generate` | `LLM_GENERATE`, pops 1, model string operand, capability `network` | AST compiles the prompt and stores the model (`llama3` when omitted). The VM posts to Ollama. Host effect. |
| `neural_circuit` | `NEURAL_CIRCUIT`, pops variable, one int operand, opcode capability none | The interpreter requires `network`. The VM posts the instruction and inputs to Ollama. Host effect. |
| `optimize_block` | no opcode | AST compiles `children[3:]` only. The metric name and threshold are not instructions. |
| `optimize_signature` | no opcode | AST compiles the last child only. |
| `req_method` | `HTTP_REQ_METHOD`, pops 0, pushes 1, capability `network` | VM reads `req` from the environment as `*http.Request`. Host effect. |
| `res` | `RES`, registry pops 2, pushes 0, capability `network` | AST compiles three children: status, content type, body. The VM pops body, content type, and status. The negative test pushes three constants before `RES`. The registry pop count does not match that form. Same class of gap as fetch body. Host effect. |
| `res_header` | `HTTP_RES_HEADER`, pops 2, capability `network` | VM sets a header on the response writer. Host effect. |
| `res_json` | `RES_JSON`, pops 2, capability `network` | AST compiles two children. The VM pops data and status. Host effect. |
| `schema_bridge` | no opcode | AST compiles child 2, the wrapped expression. |
| `spawn` | `SPAWN`, pops 0, int operand, capability `process` | The operand is the inlined lambda body length. The VM starts a goroutine. Host effect. |
| `spawn_agent` | `SPAWN_AGENT`, pops 1, string operand, capability `process` | The VM handles `OpSpawnAgent` and `OpTask` in one case and panics `UNSUPPORTED_CONSTRUCT`. |
| `sql_query` | `SQL_QUERY`, pops 0, two string operands, capability `database` | Opaque. The AST compiler reads child `.Value`. |
| `store_delete` | `STORE_DELETE`, pops 1, handle operand, capability `database` | A `file://` store also requires `filesystem` in the VM. Host effect. |
| `store_get` | `STORE_GET`, pops 1, handle operand, capability `database` | Same `file://` filesystem grant. Host effect. |
| `store_keys` | `STORE_KEYS`, pops 0, handle operand, capability `database` | Same `file://` filesystem grant. Host effect. |
| `store_open` | `STORE_OPEN`, pops 0, two string operands, capability `database` | Opaque literals. `requireStoreCapabilities` asks `capability.StoreRequirements`: `memory://` is `database`, `file://` is `database` and `filesystem`. |
| `store_put` | `STORE_PUT`, pops 2, handle operand, capability `database` | A file-backed store requires `filesystem` before the write. Host effect. |
| `task` | `TASK`, pops 0, pushes 1, one string operand, no capability | Opaque. AST stores child `.Value` and does not compile it: `(print (task "work"))` is `TASK` then `PRINT`, with no `LOAD_CONST`. The VM panics `UNSUPPORTED_CONSTRUCT`. |

`res` is the fence name that matches the fetch-body example. `(res 200 "text/plain" "ok")` is three values. `OpRes` says pops 2. The VM pops three. Generated Go comments the same three-argument form. Closing that gap is a host-effect change on the opcode contract, not a one-opcode dogfood slice. This note does not close it.

## What stays

The three group-1 names stay in the promote-blocker fence. Teaching `LowerToBytecode` one of them is a later dogfood slice on `-compile-hfir-bc` only. That slice still leaves `*compileBc` on `bytecode.CompileToBytecode`, leaves the Decision line as "Defer the production flip.", and leaves #90 Partial. It expires the Assurance tip-lock named above. This inventory does not take a new lock.

Group 2 stays in the fence. Clearing a name there with a new opcode or a new capability is a kill in the checklist when a PR uses that shape to justify a production flip. This note does not widen `nodeRoles`. It does not implement fetch body, `time_now`, `sleep`, or `read_line`.
