# QA — HowlFrame #90 tip-lock (Assurance)

**Role:** QA verification (behavioral / suite / oracle fidelity)  
**Tip:** `4d74dbcf9654caa05e0b1d9212b15bc5398359e3` (`origin/main`)  
**Worktree:** `/workspace/howlframe` (detached HEAD at tip; existing clone fetched)  
**Oracle:** `docs/reference/lowered_hfir_prod_flip_criteria.md` + `TestProdFlipCriteriaLock`  
**Recorded:** 2026-09-30 ~15:55 EDT  
**Agent:** Assurance tip-lock (box); did **not** flip production `-compile-bc`; did **not** mark #90 Done; did **not** push/PR.

## Environment

| Tool | Path / version |
| --- | --- |
| go | `/usr/bin/go` — go1.24.4 linux/amd64 |
| node | `/usr/bin/node` — v20.19.2 (**present**; JS host exercised; not `BACKEND_UNSUPPORTED`) |

## Suite commands and results

All run from `/workspace/howlframe` with `HEAD=4d74dbcf9654caa05e0b1d9212b15bc5398359e3`.

| # | Command | Result | Log |
| --- | --- | --- | --- |
| 1 | `go test ./tools/difftest -count=1 -timeout 900s -run 'TestLoweredHFIRABIConformance\|TestHFIRBytecodeSupportedParity\|TestHFIRBytecodeRejectsUnsupportedParity'` | **PASS** (~22s) | `suite-difftest.log` |
| 2 | `go test ./internal/vm -count=1 -timeout 900s -run 'TestHFIRBytecode'` | **PASS** (~0.06s) | `suite-vm.log` |
| 3 | `go test . -count=1 -timeout 180s -run 'TestProdFlipCriteriaLock'` | **PASS** (all subtests) | `suite-prodflip.log` |

### Difftest highlights (QA)

- `TestLoweredHFIRABIConformance`: **PASS** — includes mediated effects grant/deny (`env`, `exec`, `read_file`, `write_file`, `mkdir`, `fetch`), nested control, `nested_multi_effect_{denied,partial,granted}`, `html_escape` (+ type error). Hosts include interpreter / bytecode / hfir_bytecode / go / javascript with node present.
- `TestHFIRBytecodeSupportedParity`: **PASS** — 11 supported parity files (01–06, 08–12).
- `TestHFIRBytecodeRejectsUnsupportedParity`: **PASS** — `07_strings.howl`, `13_html_escape.howl` correctly rejected on experimental path (`hfirRejectedParity` still non-empty — expected defer, not tip-lock fail).

### VM highlights (QA)

- Equivalence / fixture / operand-order / nested multi-effect / capability denial consistency: **PASS**.
- HTML_ESCAPE fixture and type-error parity with AST: **PASS**.

### Prod-flip lock (QA vs oracle)

`TestProdFlipCriteriaLock` confirms on tip:

- `*compileBc` still calls `bytecode.CompileToBytecode(root)` (no `LowerToBytecode` in prod branch).
- `*compileHfirBc` still calls `hfir.LowerToBytecode(graph)`.
- Document fence == computed promote-blockers; `html_escape` emits on both paths; `regex_match` / `attr_escape` fail-closed on HFIR, still emit on AST.
- Fetch body string absent from both artifacts; `match`/`try` ControlEdges empty as accepted limits.

## Spot-check (source) — QA angle

| Check | Finding |
| --- | --- |
| Prod emission | `howlframe.go` `*compileBc` → `bytecode.CompileToBytecode` |
| Experimental emission | `*compileHfirBc` → `hfir.LowerToBytecode` |
| Fence after html_escape leave | Fence **non-empty** (29 names); **`html_escape` absent**; **`regex_match`** and **`attr_escape` still present** |
| `hfirRejectedParity` | `07_strings.howl` (`regex_match`), `13_html_escape.howl` (`attr_escape`; file also calls `html_escape` which now lowers) |
| Accepted limits | empty match/try ControlEdges; `WasmInfeasibleKinds` = exec, spawn_agent, http_server_start; linking via `ast.ResolveModules` (no VM module opcode) |

## QA verdict

**PASS** — suite green on tip; oracle statements still hold; mediation hosts agree under present node; no unclassified parity files observed. Promote remains deferred (see JOURNAL / security) without converting tip-lock to FAIL.
