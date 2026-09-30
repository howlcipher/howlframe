# Assurance tip-lock for the production `-compile-bc` flip

## Why this slice

`docs/reference/lowered_hfir_prod_flip_criteria.md` asks for an Assurance tip-lock: a named `main` SHA plus the suite result on that SHA. Dogfood journals do not refresh it. This slice records that lock for the current `main` tip and leaves the emission source where it is. Decision: Defer. The lock is not a promote. It does not authorize a flip. #90 stays Partial.

## Locked tip

`git rev-parse origin/main` at the lock:

`4d74dbcf9654caa05e0b1d9212b15bc5398359e3`

That commit is `feat: experimental HFIR html_escape onto existing HTML_ESCAPE (#71)` on `main`. The checklist baseline remains `a9f00bcc91680abe505616fb9b9ed6e643ffa3bc` (PR #69). The baseline is the SHA the checklist was written against. This lock is the measurement SHA.

On the locked tip, `howlframe.go` still splits emission the way the criteria require:

* The `*compileBc` branch calls `bytecode.CompileToBytecode(root)`.
* The `*compileHfirBc` branch calls `hfir.LowerToBytecode(graph)`.

`TestProdFlipCriteriaLock` fails if that split changes before a flip PR. This docs commit does not edit `howlframe.go`, `internal/bytecode`, or `internal/hfir`. The head of the record PR is a later SHA. The locked compiler tip stays `4d74dbcf9654caa05e0b1d9212b15bc5398359e3`.

## Decision

Decision: Defer.

Defer still holds on the locked SHA because the promote-blocker fence is non-empty, `hfirRejectedParity` is non-empty, and the Owner has not merged a flip PR that cites this lock. This journal is the measurement record. It is not that merge. Production `-compile-bc` stays `runHFIRGate` and then `bytecode.CompileToBytecode`. Experimental `-compile-hfir-bc` stays the dogfood path until the Owner authorizes a flip PR. #90 stays Partial.

## Assurance verdict

Assurance (Lain) verified this tip. Overall: PASS. Promote: DEFERRED. Node on that run was v20.19.2 and Go was go1.24.4. The verdict journal is `docs/journals/2026-09-30_assurance_tip_lock_4d74dbcf.md`. The logs are `evidence/howl-90-assurance-tip-lock-4d74dbcf/`. The command timings in the checklist below are a separate local run of the same suite. They are not those logs. Both runs passed. The decision on both is Defer.

## Evidence checklist

The suite named in the criteria was run on the locked tip before this docs commit. Each command exited 0.

```
go test . -count=1 -timeout 180s -run 'TestProdFlipCriteriaLock'
ok  	github.com/howlcipher/howlframe	0.004s

go test ./internal/vm -count=1 -timeout 900s -run 'TestHFIRBytecode'
ok  	github.com/howlcipher/howlframe/internal/vm	0.044s

go test ./tools/difftest -count=1 -timeout 900s -run 'TestLoweredHFIRABIConformance|TestHFIRBytecodeSupportedParity|TestHFIRBytecodeRejectsUnsupportedParity'
ok  	github.com/howlcipher/howlframe/tools/difftest	10.454s
```

`node` was on `PATH` (`v22.14.0`). Production `-compile-bc` results are the oracle. Experimental `-compile-hfir-bc` results are the candidate. `go test ./...` is the CI bar for the tree that contains the lock. It passed after this record was pinned.

### HFIR↔AST parity for mediated host effects

Phase 2a–2e mediators stay in force. The bytecode flip would keep them. It would not retarget Go, JavaScript, or the interpreter onto HFIR. Each row below is in `tests/conformance/lowered_hfir_abi_v1.json` with targets `hfir_bytecode`, `bytecode`, `interpreter`, `go`, and `javascript`. Denied rows expect `CAPABILITY_DENIED` and forbid the secret, the command, the path, or the URL. Granted rows expect `PASS` and the marker stdout. `TestLoweredHFIRABIConformance` passed on the locked tip, including these cases.

| Effect | Grant | Opcode | Denied | Granted | Nested with `if` / `while` / `for` / `defun` |
| --- | --- | --- | --- | --- | --- |
| `env` | `environment` | `ENV` | `env_denied` | `env_granted` | `nested_env_read_denied`, `nested_env_read_granted` |
| `exec` | `process` | `EXEC` | `exec_denied` | `exec_granted` | `nested_exec_denied`, `nested_exec_granted` |
| `read_file` | `filesystem` | `READ_FILE` | `read_file_denied` | `read_file_granted` | `nested_env_read_denied`, `nested_env_read_granted` |
| `write_file` | `filesystem` | `WRITE_FILE` | `write_file_denied` | `write_file_granted` | `nested_fs_write_denied`, `nested_fs_write_granted` |
| `mkdir` | `filesystem` | `MKDIR` | `mkdir_denied` | `mkdir_granted` | `nested_fs_write_denied`, `nested_fs_write_granted` |
| `fetch` | `network` | `FETCH` | `fetch_denied` | `fetch_granted` | `nested_fetch_denied`, `nested_fetch_granted` |

`nested_multi_effect_denied` (empty grant), `nested_multi_effect_partial` (grant omits `network`), and `nested_multi_effect_granted` (full grant) place `env`, `read_file`, `write_file`, `mkdir`, `exec`, and `fetch` in one program. Those three cases passed in the same conformance run.

`TestHFIRBytecodeSupportedParity` passed. Every file in `tests/parity` is classified. The supported set is `01_primitives.howl`, `02_variables.howl`, `03_operators.howl`, `04_conversions.howl`, `05_collections.howl`, `06_control_flow.howl`, `08_io_cli.howl`, `09_boundary_values.howl`, `10_governed_policy.howl`, `11_error_undefined_var.howl`, and `12_error_div_zero.howl`. Operand order on the locked tip stays the AST order: `EXEC` is command then arguments, and `FETCH` is URL then method.

### `html_escape` on the experimental path

`html_escape` lowers on `-compile-hfir-bc` onto the existing `HTML_ESCAPE` opcode. `HTML_ESCAPE` grants nothing. `tests/conformance/abi_v1/22_html_escape.howl` and `tests/conformance/abi_v1/23_html_escape_type_error.howl` are conformance cases `html_escape` and `html_escape_type_error`. Both list `hfir_bytecode` beside `bytecode`. The type-error case expects `TYPE_ERROR`. `TestHFIRBytecode`, which includes `TestHFIRBytecodeHTMLEscapeFixture` and `TestHFIRBytecodeHTMLEscapeTypeErrorMatchesAST`, passed. `TestProdFlipCriteriaLock` requires `HTML_ESCAPE` on both bytecode compilers for `(html_escape "a<b")`.

`html_escape` is outside the promote-blocker fence. The model-adapter transport still rejects kind `html_escape` with `HFIR_TRANSPORT_KIND`. Production `-compile-bc` still emits `HTML_ESCAPE` from the AST.

### Promote-blocker fence after `html_escape`

`TestProdFlipCriteriaLock` recomputes the fence from `construct.Supported`, `compileNode`, and `LowerToBytecode`. The checklist's `promote-blockers` block matches that computation at the locked tip. The block has twenty-nine names. `html_escape` has left it. `attr_escape` and `regex_match` are still in it.

`hfirRejectedParity` still holds two files, and `TestHFIRBytecodeRejectsUnsupportedParity` passed on both:

* `tests/parity/07_strings.howl` (`regex_match`)
* `tests/parity/13_html_escape.howl` (`attr_escape`; the file also calls `html_escape`, which now lowers)

`LowerToBytecode` on those files returns one `HFIR_BYTECODE_UNSUPPORTED` diagnostic and no `BCProgram`. Production `-compile-bc` still emits `REGEX_MATCH` and `ATTR_ESCAPE`. The rest of the fence is the same class of blocker: stores, the HTTP server, spawn, database, model calls, `time_now`, `sleep`, and `read_line`.

### Accepted limits

These limits hold on the locked tip. `TestProdFlipCriteriaLock` checks each one. A flip that implements one of them in order to flip stays outside the checklist.

| Item | Locked observation |
| --- | --- |
| Fetch body | `OpFetch` is `FETCH`, pops 2, has an empty operand list, and is `network`. A `(fetch url method body)` program on both bytecode compilers contains `FETCH` and omits the body string. The body stays off `OpFetch`. |
| `match` control edges | `construct.Lookup("match")` is `Unsupported`. `ControlEdges` stay empty. `LowerToBytecode` returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. |
| `try` control edges | Kind `try` emits `TRY_LET` from data edges. `ControlEdges` stay empty. |
| Wasm | `hfir.WasmInfeasibleKinds` is `exec`, `spawn_agent`, and `http_server_start`. |
| Module linker | #106 stays the do-not-build decision. `use`, `export`, and `module` stay `CompileTimeOnly`. The VM has no module opcode. This lock adds no linker. |
| Model-adapter transport | `DecodeCandidate` rejects `defun`, `call`, `return`, `while`, `for`, `read_file`, `write_file`, `mkdir`, `exec`, `fetch`, `regex_match`, `html_escape`, `attr_escape`, `match`, and `try` with `HFIR_TRANSPORT_KIND` and no graph. |

### Kill conditions

The Kill section of `docs/reference/lowered_hfir_prod_flip_criteria.md` is unchanged. These shapes still end the flip track:

* A new opcode, or a new capability, so the experimental subset can cover a promote blocker.
* A Wasm opcode, Wasm collection, or Wasm host import.
* A bytecode import section, a VM module opcode, or an HFIR module linker (#106).
* An `OpFetch` body operand, or any other fetch-body implementation.
* Two production emitters inside `-compile-bc`.
* Dropping a promote-blocker construct out of production so the experimental subset fits.
* Widening `nodeRoles` so the model-adapter transport accepts kinds the source lowerer already emits.
* Marking #90 Done because the current dogfood subset matches.
* Treating this checklist, or any dogfood journal, as the Owner's flip PR.

This lock uses none of those shapes. `regex_match` and `attr_escape` keep working on `-compile-bc`. `html_escape` already lowers onto `HTML_ESCAPE` on the experimental path and still works on `-compile-bc`.

## What this does not do

No production `-compile-bc` flip. No new opcode. No new capability. No Wasm host. No module linker. No fetch-body implementation. No edit to `nodeRoles`. `match` and `try` stay without control edges. The promote rows are not all true: the fence and `hfirRejectedParity` are non-empty, so the decision stays Defer. #90 stays Partial.
