# Standalone Runtime Planning Journal (2026-08-05)

## 1. Standalone Runtime Readiness Assessment

### Deterministic language core
- **Variables, Conditionals, Loops, Functions, Types (Int/Float/Bool/String/List/Dict), Conversions:** Mostly ready and implemented across all layers.
- **Time / Seedable Randomness:** `sleep` exists, but there is no seedable random number generator. 

### CLI interaction
- **Command-line arguments:** Supported (`cli_args`).
- **Standard input, line-oriented input:** Missing entirely.
- **Standard output:** Supported via `print`.
- **Standard error, explicit exit codes, terminal interaction:** Missing entirely.

### Files and structured data
- **Filesystem operations:** `read_file`, `write_file`, `mkdir` exist.
- **Paths:** Missing native path joining / manipulation primitives.
- **JSON:** `parse_json` exists. `res_json` exists but is tied to HTTP responses. No generic JSON serialization primitive for standard output/files.
- **Error handling:** `try_let` exists, though its flexibility with arbitrary errors in the VM needs expansion.

### Program structure
- **Modules (`use`, `export`):** Implemented in the semantic checker and Go/JS backends, but completely missing in ZIR lowering and the bytecode VM.
- **Multi-file compilation / nested modules:** Partial (via `include`), but proper module boundary encapsulation is pending.

### Testing
- **Executing Zero `test` blocks in the VM:** Missing. Currently only emitted to `server_test.go`.
- **Unit/Workflow tests, Test results/fixtures:** Missing native support for structured test output or capability-aware test mocks.

### Artifact and runtime contract
- **Bytecode packaging:** `gob` serialization works, but lacks hashes, source maps, a capability manifest envelope, artifact validation, or explicit versioning boundaries. 

### Tooling and distribution
- **Validation and Dev runs:** `zero -validate` and `zero -run` exist.
- **Project lifecycle (creation, tests, packaging, installation):** Missing.

## 2. Runtime Coverage Inventory

| Construct | Checker | ZIR | Verifier | Bytecode | VM | Interpreter | Capability | Tests | Status | Gap | Tracker |
|-----------|---------|-----|----------|----------|----|-------------|------------|-------|--------|-----|---------|
| `let`/`set` | Yes | Yes | Yes | Yes | Yes | Yes | None | Yes | stable | None | - |
| `if`/`while`/`for` | Yes | Yes | Yes | Yes | Yes | Yes | None | Yes | stable | None | - |
| `defun`/`call` | Yes | Yes | Yes | Yes | Yes | Yes | None | Yes | stable | None | - |
| `return` | Yes | Yes | Yes | Yes | Yes | Yes | None | Yes | stable | None | - |
| Int/Float/String/Bool | Yes | Yes | Yes | Yes | Yes | Yes | None | Yes | stable | None | - |
| `list`/`dict` | Yes | Yes | Yes | Yes | Yes | Yes | None | Yes | stable | None | - |
| Binops (`+`,`-`,`*`,`/`,`<`,etc) | Yes | Yes | Yes | Yes | Yes | Yes | None | Yes | stable | None | - |
| `append`/`map_set`/`map_delete` | Yes | Yes | Yes | Yes | Yes | Yes | None | Yes | stable | None | - |
| `map_get`/`list_get` | Yes | Yes | Yes | Yes | Yes | Yes | None | Yes | stable | None | - |
| `to_int`/`to_float`/`to_string` | Yes | Yes | Yes | Yes | Yes | Yes | None | Yes | stable | None | - |
| `str_split`/`str_join`/`regex_match` | Yes | Yes | Yes | Yes | Yes | Yes | None | Yes | stable | None | - |
| `read_file`/`write_file`/`mkdir` | Yes | Yes | Yes | Yes | Yes | No | Filesystem | Yes | partial | Not in `-run` interpreter | - |
| `exec`/`spawn` | Yes | Yes | Yes | Yes | Yes | No | Process | Yes | partial | Not in `-run` interpreter | - |
| `cli_args` | Yes | Yes | Yes | Yes | Yes | Yes | None | Yes | stable | None | - |
| `env` | Yes | Yes | Yes | Yes | Yes | Yes | Environment| Yes | stable | None | - |
| `sleep` | Yes | Yes | Yes | Yes | Yes | Yes | None | Yes | stable | None | - |
| `try_let` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | partial | Not in `-run` interpreter | - |
| `parse_json` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | partial | Not in `-run` interpreter | - |
| HTTP/Network primitives | Yes | Yes | Yes | Yes | Yes | No | Network | Yes | partial | Not in `-run` interpreter | - |
| Database/Store primitives | Yes | Yes | Yes | Yes | Yes | No | Database | Yes | partial | Not in `-run` interpreter | - |
| AI Primitives | Yes | Yes | Yes | Yes | Yes | Partial | Network/Proc | Yes | partial | `achieve` in interp, others vary | - |
| `use`/`export` | Yes | No | No | No | No | No | None | No | unsupported | Missing in ZIR/Bytecode/VM | #95 |
| `test` blocks | Yes | No | No | No | No | No | None | No | unsupported | Missing in ZIR/Bytecode/VM | #96 |
| `stdin`/`stderr` | No | No | No | No | No | No | None | No | unsupported | No opcodes exist | #97 |
| `exit` | No | No | No | No | No | No | None | No | unsupported | No opcodes exist | #97 |

## 3. Reference-Application Readiness

1. **File report CLI**
   - *Requires*: `cli_args`, `read_file`, `print`, `str_split`, `to_int`, `exit`, `stderr`.
   - *Missing*: Explicit exit codes, writing to `stderr` for errors.
   - *Prerequisite*: Add `stdin`/`stderr`/`exit` semantics to the VM.

2. **JSON transformation CLI**
   - *Requires*: `read_file`, `parse_json`, JSON serialization, `write_file`, `stdin`/`stdout`.
   - *Missing*: Streaming `stdin`, generic JSON serialization (only `res_json` exists).
   - *Prerequisite*: Add stdin streams and a generic `serialize_json` construct.

3. **Network status checker**
   - *Requires*: `fetch`, `sleep`, `print`, `exit`.
   - *Missing*: Explicit exit codes.
   - *Prerequisite*: Add `exit` opcode.

4. **Interactive terminal game**
   - *Requires*: line-oriented input, standard output, clear screen (terminal interaction), seedable random.
   - *Missing*: Line-oriented `stdin`, terminal manipulation, randomness.
   - *Prerequisite*: Add `read_line` or `stdin` buffering primitives.

## 4. Backlog Analysis & Updates

### Reused / Updated Existing Entries
- **Improvement #84**: SSA IR lowering for missing nodes. Remains relevant for WASM, but standalone runtime should focus on VM first.
- **Bug #43**: `use` fails to resolve. This points to the module system gap.

### New Bugs Added
No new bugs added. Discrepancies between checker and VM (like `use`/`export` and `test`) represent missing implementation phases, not broken implementations, so they belong in improvements.

### New Improvements Added
- **Improvement #95**: Standalone Runtime: ZIR and Bytecode VM support for module `use` and `export`.
- **Improvement #96**: Standalone Runtime: Execute `test` blocks natively in the VM.
- **Improvement #97**: Standalone Runtime CLI Semantics: Phase 1 (stdin, stderr, exit codes).
- **Improvement #98**: Standalone Runtime Artifacts: Define explicit artifact validation, hashing, and versioning.

### Dependency Relationships
- #97 (CLI semantics) is a prerequisite for any reliable file/transform/network CLI apps.
- #95 (Modules in VM) is required before #96 (Test blocks in VM), because test blocks often test exported functions.
- #98 (Artifact validation) should happen after CLI semantics are firm.

## 5. Next Item Recommendation

**The highest-ranked currently actionable item:** 
**Improvement #97: Standalone Runtime CLI Semantics: Phase 1 (stdin, stderr, explicit exit codes).**

**Why it should be next:**
Prioritization principle #5 says "Add stdin, stderr, and explicit exit codes." Without these, building reliable CLI applications is impossible. The VM currently can only write to stdout (via `print`) and always exits 0 unless a panic occurs. 

**Risks and blockers:**
None. This is well-isolated. It requires updating `internal/bytecode/opcode.go`, AST lowering, the VM execution loop, and `zero.go` flag outputs. Implementation code was not changed during this planning session.
