# Security — HowlFrame #90 tip-lock (Assurance)

**Role:** Security / capability / fail-closed / kill-shape review (distinguishable from QA suite green)  
**Tip:** `4d74dbcf9654caa05e0b1d9212b15bc5398359e3`  
**Oracle:** `docs/reference/lowered_hfir_prod_flip_criteria.md` Kill / Defer / accepted limits  
**Recorded:** 2026-09-30 ~15:55 EDT  
**Constraints honored:** no production compile-bc flip; no #90 Done; no invented secrets; no push/PR.

## Security posture on tip

### Emission split (attack / trust boundary)

- Production `-compile-bc` remains Path 1: gate then **AST** `bytecode.CompileToBytecode` — experimental HFIR lowerer is **not** the production emitter.
- Experimental `-compile-hfir-bc` remains Path 2 dogfood: gate then `hfir.LowerToBytecode`.
- **No kill-shape observed:** no dual production emitters; no dropping fenced constructs from prod; no OpFetch body operand; no module linker / Wasm opcode expansion; #90 not marked Done.

### Capability mediation (Phase 2a–2e table)

Conformance grant/deny cases for `env` / `exec` / `read_file` / `write_file` / `mkdir` / `fetch` passed across hosts with **node present** (v20.19.2). Denial class remains `CAPABILITY_DENIED` before effect; suite includes secret-leak checks (e.g. host-read markers / test token env) — no invented secrets used in this tip-lock.

### OpFetch / body limit (shared fail-closed)

- Registry: `OpFetch` Name=FETCH, **Pops=2**, empty Operands, Capability=Network (`internal/bytecode/opcode.go`).
- AST compiles URL + method children only; HFIR compiles `url` + `method` edges only; optional body **not** in either artifact (`TestProdFlipCriteriaLock/fetch_body` PASS).
- Accepted limit: body stays off bytecode OpFetch (interpreter/Go/JS may still send body at runtime when that call path runs).

### Promote-blocker fence (fail-closed experimental)

After `html_escape` left the fence onto existing `HTML_ESCAPE`:

- Fence still lists **`regex_match`** and **`attr_escape`** (plus stores, HTTP server, spawn, DB, model, `time_now`, `sleep`, `read_line`, … — 29 names).
- HFIR: no `case "regex_match"` / `case "attr_escape"`; both return `HFIR_BYTECODE_UNSUPPORTED` with no BCProgram; AST still emits `REGEX_MATCH` / `ATTR_ESCAPE`.
- `hfirRejectedParity` non-empty → **Promote DEFERRED** (expected; not tip-lock FAIL).

### Transport / Wasm / linker accepted limits

- Model-adapter `DecodeCandidate` still rejects kinds including control/effects/escape/`match`/`try` with `HFIR_TRANSPORT_KIND` (lock subtests PASS).
- `hfir.WasmInfeasibleKinds` = `exec`, `spawn_agent`, `http_server_start` only.
- Module linking remains AST `ResolveModules`; no bytecode import section / VM module opcode found in registry scan.

### Kill checklist (negative scan)

| Kill shape | Present on tip? |
| --- | --- |
| New opcode/capability to clear a blocker | No (html_escape reused existing HTML_ESCAPE) |
| Wasm opcode / host import growth | No |
| Bytecode module linker | No |
| OpFetch body operand / fetch-body in artifact | No |
| Two emitters inside `-compile-bc` | No |
| Dropping blocker from production | No (`regex_match`/`attr_escape` still on AST path) |
| Widening transport `nodeRoles` (#88) | No (rejects still hold) |
| Marking #90 Done | No (oracle + this tip-lock: Partial) |
| Treating checklist/journal as Owner flip | No (Promote DEFERRED; no flip PR) |

## Security verdict

**PASS** for tip-lock security properties: fail-closed experimental blockers hold; prod emission unchanged; capability denials consistent; accepted limits intact; **no kill-shape**.  

**Promote: DEFERRED** while fence and `hfirRejectedParity` remain non-empty and Owner has not merged a flip PR citing a fresh lock.
