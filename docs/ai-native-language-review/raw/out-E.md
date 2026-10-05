# Independent Review: HowlFrame AI-Native Product Direction

## Executive Summary
HowlFrame's core thesis—"intent is not authority," where models propose programs executed within deterministic, verified, capability-bounded machinery—is sound and addresses the primary failure mode of autonomous agent architectures.
Factually today, HowlFrame is a working Go-based bytecode compiler and virtual machine executing bounded CLI/HTTP scripts with instruction ceilings and coarse capability gates, alongside legacy Go/JS/WASM transpilation heads.
Its aspirational "AI-native paradigms" (swarms, teleological solvers, JIT self-mutating runtimes) are either paperware or thin wrappers executing unauthenticated HTTP calls to a local Ollama instance.
The runtime sandbox has severe security vulnerabilities: `filesystem` and `network` grants lack path/domain scoping, host commands execute directly via `OpExec`, and memory limits (`MaxMemoryBytes`) are unenforced.
To succeed, the solo builder must discard polyglot transpilation heads and speculative AI primitives, adopt fine-grained resource scoping, and pivot the execution backend toward standard WASM/Wasmtime sandboxing rather than maintaining a bespoke Go VM.

---

## Numbered Evaluation Questions

### 1. What is HowlFrame, factually, today (what actually runs end-to-end)? What is merely documented/aspirational?

**Factually Running Today:**
- **AST-to-Bytecode Pipeline & VM Execution:** Source `.howl` parses to an AST, validates through `runHFIRGate` ([howlframe.go:1362](file:///tmp/hf-ainative-agents/repo-E/howlframe.go#L1362)), compiles to `.hfbc` bytecode ([howlframe.go:1363](file:///tmp/hf-ainative-agents/repo-E/howlframe.go#L1363)), serializes to binary `gob` with SHA-256 integrity hashing ([internal/bytecode/artifact.go:2452-2488](file:///tmp/hf-ainative-agents/repo-E/internal/bytecode/artifact.go#L2452-L2488)), and executes on a stack-based VM (`BCVM`, [internal/vm/vm.go:4930-6611](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L4930-L6611)) [VERIFIED].
- **CLI & VM Language Subset:** Scalar arithmetic, logical operators, lexical bindings (`let`, `set`), branches (`if`), loops (`while`, `for`), functions (`defun`, `call`, `return`), collections (`list`, `dict`, `map_get`, `map_set`, `map_delete`, `map_keys`), string utilities, and structured JSON parsing/encoding ([internal/construct/construct.go:2719-2827](file:///tmp/hf-ainative-agents/repo-E/internal/construct/construct.go#L2719-L2827)) [VERIFIED].
- **Persistence Layer:** An in-memory and file-backed structured record store accessible via `store_open`, `store_put`, `store_get`, `store_delete`, and `store_keys` ([internal/vm/vm.go:5578-5659](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5578-L5659)) [VERIFIED].
- **HTTP Serving:** A standalone HTTP dispatcher supporting `http_server`, `http_route`, route parameter extraction (`req_query`, `req_path`, `req_header`), and JSON responses (`res_json`) ([internal/vm/vm.go:5821-5937](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5821-L5937)) [VERIFIED].
- **Execution Budgets & Gate:** Runner-owned finite instruction limits (`--max-instructions`, default 100,000; [internal/vm/error.go:3533](file:///tmp/hf-ainative-agents/repo-E/internal/vm/error.go#L3533); [internal/vm/vm.go:5485-5489](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5485-L5489)) and coarse opcode capability checks (`requireCapability`, [internal/vm/vm.go:5301-5308](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5301-L5308)) [VERIFIED].
- **Model Adapter Prototype:** JSON transport decoding directly into an `hfir.Graph` for a bounded 8–10 node subset, with single-node replacement deltas (`hfir-model-adapter/v1`, [internal/hfir/model_adapter.go:3235-3394](file:///tmp/hf-ainative-agents/repo-E/internal/hfir/model_adapter.go#L3235-L3394)) [VERIFIED].
- **Legacy Transpilation:** Go codegen (`gogen`, [howlframe.go:1967-1995](file:///tmp/hf-ainative-agents/repo-E/howlframe.go#L1967-L1995)), JS codegen (`javascript`, [howlframe.go:1996-2024](file:///tmp/hf-ainative-agents/repo-E/howlframe.go#L1996-L2024)), and SSA-to-WAT generation (`compileToWasm`, [howlframe.go:1521-1551](file:///tmp/hf-ainative-agents/repo-E/howlframe.go#L1521-L1551)) [VERIFIED].

**Merely Documented / Aspirational:**
- **Swarm Primitives & Autonomous Agents:** Documented as "autonomous subagents as first-class concurrency objects... negotiating tasks via typed message-passing channels" ([docs/extreme_ai_paradigms.md:769-775](file:///tmp/hf-ainative-agents/repo-E/docs/extreme_ai_paradigms.md#L769-L775)). In reality, `SPAWN_AGENT` is a synchronous, cooperative, single-threaded in-process interpreter loop that merely prints stdout lines ([docs/journals/2026-10-05_vm_task_spawn_agent.md:17-18](file:///tmp/hf-ainative-agents/repo-E/docs/journals/2026-10-05_vm_task_spawn_agent.md#L17-L18); [internal/vm/vm.go:6548-6603](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L6548-L6603)) [VERIFIED].
- **Teleological Execution:** Documented as constraint solving over goal states `(achieve ...)` ([docs/extreme_ai_paradigms.md:776-782](file:///tmp/hf-ainative-agents/repo-E/docs/extreme_ai_paradigms.md#L776-L782)). In reality, it stringifies arguments and calls an unauthenticated local Ollama endpoint (`http://localhost:11434/api/generate`) with `llama3` ([internal/vm/vm.go:6027-6048](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L6027-L6048)) [VERIFIED].
- **Auto-Mutating Runtime:** Documented as runtime JIT-LLM optimization (`optimize_block`) ([docs/extreme_ai_paradigms.md:783-789](file:///tmp/hf-ainative-agents/repo-E/docs/extreme_ai_paradigms.md#L783-L789)). In reality, `optimize_block` merely executes its body statements sequentially ([internal/vm/vm.go:3880-3888](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L3880-L3888)) [VERIFIED].
- **Stochastic Control Flow:** Documented as AST-level probability distribution branches (
# Independent Review: HowlFrame AI-Native Product Direction

## Executive Summary
HowlFrame’s central thesis—"intent is not authority," where models propose bounded programs verified and executed by deterministic machinery—is intellectually sound and targets the primary vulnerability of autonomous LLM agents.
Factually today, HowlFrame is a Go-based bytecode compiler and virtual machine executing small CLI and HTTP programs with finite instruction budgets and coarse capability gates, surrounded by legacy Go/JS/WASM transpilation heads.
Its aspirational "AI-native paradigms" (swarms, teleological solvers, self-mutating JITs) are largely paperware or synchronous wrappers calling an unauthenticated local Ollama instance (`localhost:11434`).
The runtime sandbox has major security gaps: `filesystem` and `network` grants lack path or domain scoping, `OpExec` invokes raw host shell binaries, and documented memory limits (`MaxMemoryBytes`) are completely unenforced in the VM loop.
To survive, the solo builder must immediately quarantine the polyglot transpilation heads and speculative AI opcodes, introduce fine-grained resource descriptors, and pivot toward standard WebAssembly (Wasmtime) sandboxing instead of maintaining a custom, vulnerable Go runtime.

---

## Numbered Evaluation Questions

### 1. What is HowlFrame, factually, today (what actually runs end-to-end)? What is merely documented/aspirational?

**Factually Running Today:**
- **AST-to-Bytecode Toolchain:** Source `.howl` parses to an AST, validates through the ahead-of-time semantic verification gate `runHFIRGate` ([howlframe.go:1362](file:///tmp/hf-ainative-agents/repo-E/howlframe.go#L1362)), compiles to `.hfbc` bytecode ([howlframe.go:1363](file:///tmp/hf-ainative-agents/repo-E/howlframe.go#L1363)), serializes to binary `gob` with SHA-256 integrity hashing ([internal/bytecode/artifact.go:2452-2488](file:///tmp/hf-ainative-agents/repo-E/internal/bytecode/artifact.go#L2452-L2488)), and executes on a stack-based VM (`BCVM`, [internal/vm/vm.go:4930-6611](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L4930-L6611)) [VERIFIED].
- **Core Executable Subset:** Arithmetic, comparisons, lexical bindings (`let`, `set`), control flow (`if`, `while`, `for`), functions (`defun`, `call`, `return`), collections (`list`, `dict`, `map_get`, `map_set`, `map_delete`, `map_keys`), string utilities, and structured JSON parsing/encoding ([internal/construct/construct.go:2719-2827](file:///tmp/hf-ainative-agents/repo-E/internal/construct/construct.go#L2719-L2827)) [VERIFIED].
- **Bytecode Native Store:** An in-memory and file-backed structured record store accessible via `store_open`, `store_put`, `store_get`, `store_delete`, and `store_keys` ([internal/vm/vm.go:5578-5659](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5578-L5659)) [VERIFIED].
- **Deterministic HTTP Dispatcher:** Standalone HTTP serving supporting `http_server`, `http_route`, route parameters (`req_query`, `req_path`, `req_header`), and JSON responses (`res_json`) ([internal/vm/vm.go:5821-5937](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5821-L5937)) [VERIFIED].
- **Execution Budgets & Gate:** Runner-owned finite instruction limits (`--max-instructions`, default 100,000; [internal/vm/error.go:3533](file:///tmp/hf-ainative-agents/repo-E/internal/vm/error.go#L3533); [internal/vm/vm.go:5485-5489](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5485-L5489)) and coarse opcode capability checks (`requireCapability`, [internal/vm/vm.go:5301-5308](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5301-L5308)) [VERIFIED].
- **Model Adapter Prototype:** JSON transport decoding directly into an `hfir.Graph` for a bounded 23-kind scalar subset, with single-node replacement deltas (`hfir-model-adapter/v1`, [internal/hfir/model_adapter.go:3235-3394](file:///tmp/hf-ainative-agents/repo-E/internal/hfir/model_adapter.go#L3235-L3394)) [VERIFIED].

**Merely Documented / Aspirational:**
- **Swarm Primitives & Autonomous Agents:** Documented as "autonomous subagents as first-class concurrency objects... negotiating tasks via typed message-passing channels" ([docs/extreme_ai_paradigms.md:769-775](file:///tmp/hf-ainative-agents/repo-E/docs/extreme_ai_paradigms.md#L769-L775)). In reality, `SPAWN_AGENT` is a synchronous, cooperative, single-threaded in-process interpreter loop that merely prints stdout lines ([docs/journals/2026-10-05_vm_task_spawn_agent.md:17-18](file:///tmp/hf-ainative-agents/repo-E/docs/journals/2026-10-05_vm_task_spawn_agent.md#L17-L18); [internal/vm/vm.go:6548-6603](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L6548-L6603)) [VERIFIED].
- **Teleological Execution:** Documented as a solver planning state-space paths `(achieve ...)` ([docs/extreme_ai_paradigms.md:776-782](file:///tmp/hf-ainative-agents/repo-E/docs/extreme_ai_paradigms.md#L776-L782)). In reality, it stringifies arguments and calls an unauthenticated local Ollama endpoint (`http://localhost:11434/api/generate`) with `llama3` ([internal/vm/vm.go:6027-6048](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L6027-L6048)) [VERIFIED].
- **Auto-Mutating Runtime:** Documented as continuous JIT-LLM optimization (`optimize_block`) ([docs/extreme_ai_paradigms.md:783-789](file:///tmp/hf-ainative-agents/repo-E/docs/extreme_ai_paradigms.md#L783-L789)). In reality, `optimize_block` merely executes its body statements sequentially ([internal/vm/vm.go:3880-3888](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L3880-L3888)) [VERIFIED].
- **Stochastic Control Flow:** Documented as AST-level probability distribution branches ([docs/extreme_ai_paradigms.md:790-796](file:///tmp/hf-ainative-agents/repo-E/docs/extreme_ai_paradigms.md#L790-L796)). In reality, `confidence` evaluates a zero-shot prompt via Ollama and casts the returned text to a float ([internal/vm/vm.go:6050-6070](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L6050-L6070)) [VERIFIED].

---

### 2. Based on the last ~40 commits and docs, what is it already becoming?

The git log over the last 40 commits (`d496d97` down to `5385814`, [REVIEW_CONTEXT_PACK.md:5-45](file:///tmp/hf-ainative-agents/repo-E/REVIEW_CONTEXT_PACK.md#L5-L45)) reveals a clear architectural trajectory:
1. **Bytecode-First Consolidation:** The codebase is rapidly moving away from multi-target transpilation toward the standalone bytecode VM (`-run-bc`). Commits `#85` through `#97` systematically forced every root (`cli_app`, `http_server`, `web_app`, `wasm_app`) to emit `.hfbc` bytecode artifacts and fail closed on unnamed roots rather than falling back to Go transpilation ([howlframe.go:1417-1456](file:///tmp/hf-ainative-agents/repo-E/howlframe.go#L1417-L1456)) [VERIFIED].
2. **Defensive Hardening of the VM:** Commits `#101`–`#103` (`fca8307`, `9b8f48d`, `d496d97`) isolated child failures, returns, and out-of-range body lengths in `SPAWN_AGENT` [VERIFIED].
3. **Rigid Parity Gating for HFIR Lowering:** Commits `#77`–`#83` incrementally added lowerings for `attr_escape`, `regex_match`, `time_now`, and `sleep` into experimental `-compile-hfir-bc`, while rigorously locking production `-compile-bc` behind parity criteria ([docs/reference/lowered_hfir_prod_flip_criteria.md:1040-1060](file:///tmp/hf-ainative-agents/repo-E/docs/reference/lowered_hfir_prod_flip_criteria.md#L1040-L1060)) [VERIFIED].

HowlFrame is already shedding its identity as a general-purpose transpiler and becoming an **embedded, deterministically bounded, bytecode execution engine with runner-enforced policy controls** [VERIFIED].

---

### 3. Is the "constrained/verified/auditable execution for AI-generated programs" thesis sound? Strongest argument for and against.

**Verdict: The thesis is SOUND, but its current implementation is severely misaligned.**

- **Strongest Argument For:** Current LLM agent architectures give models direct tool access (e.g., executing arbitrary bash commands or invoking APIs via function calling). A multi-step task requires multiple round trips. If an agent emits a structured program with loops, branches, and intermediate data transformations, executing that program inside a capability-bounded sandbox with instruction quotas and audit manifests eliminates round-trip latency, enforces invariants before execution, and guarantees non-escalation of privilege ("intent is not authority") [INFERRED].
- **Strongest Argument Against:** Writing code in a bespoke, non-standard language (`.howl`) or custom intermediate representation (`HFIR`) introduces massive model cognitive friction and syntax hallucination. Industry models are pre-trained on billions of lines of Python, JavaScript, and WebAssembly. A sandboxed WebAssembly runtime (Wasmtime) with fuel metering, WASI capability scoping, or Starlark/CEL already solves constrained execution without inventing a proprietary grammar, compiler, VM, and debugger [INFERRED].

---

### 4. Who would actually use this, for what job, instead of what alternative?

**Current Target User:** Platform engineers building autonomous agent runtimes, CI/CD automated release gates, or data-transformation pipelines where untrusted AI agents must operate on sensitive infrastructure.

**Concrete Jobs:**
1. **Automated Release Authority:** An untrusted agent ingests test results, security scans, and change-window metrics, proposing a deployment action. HowlFrame evaluates policy deterministically and gates the mutation ([apps/release_authority/release_authority.howl:7116-7263](file:///tmp/hf-ainative-agents/repo-E/apps/release_authority/release_authority.howl#L7116-L7263)) [VERIFIED].
2. **Bounded Action Execution:** A task requires fetching an API, filtering JSON records, and writing an artifact without allowing arbitrary network calls or shell access ([apps/action_executor/action_executor.howl:6898-7111](file:///tmp/hf-ainative-agents/repo-E/apps/action_executor/action_executor.howl#L6898-L7111)) [VERIFIED].

**Alternative Comparison:**
- Instead of **raw Python/Bash inside Docker**: HowlFrame provides deterministic termination (instruction ceilings) and microsecond startup without container overhead [INFERRED].
- Instead of **multi-turn tool-calling loops (MCP)**: HowlFrame compresses 10 tool calls with data joins into a single client-side program execution [INFERRED].
- **Why they would NOT use it today:** OPA/Rego and CEL already dominate policy evaluation; Wasmtime/WASI dominates sandboxed multi-language execution [INFERRED].

---

### 5. What is the trusted computing base, and is it trustworthy today? Name concrete gaps.

**The Trusted Computing Base (TCB) consists of:**
1. The Go runtime and OS kernel.
2. The front-end parser and checker ([internal/parser/](file:///tmp/hf-ainative-agents/repo-E/internal/parser/), [internal/checker/](file:///tmp/hf-ainative-agents/repo-E/internal/checker/)).
3. The AOT HFIR verifier and construct scanner ([internal/hfir/verifier.go](file:///tmp/hf-ainative-agents/repo-E/internal/hfir/verifier.go), [internal/construct/construct.go](file:///tmp/hf-ainative-agents/repo-E/internal/construct/construct.go)).
4. The bytecode compiler ([internal/bytecode/](file:///tmp/hf-ainative-agents/repo-E/internal/bytecode/)).
5. The standalone bytecode VM (`BCVM`, [internal/vm/vm.go](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go)).

**Is it trustworthy today? NO.** It is an early-stage prototype with severe security and isolation gaps:
- **Gap 1: Unscoped Capability Grants (Ambient Authority).** The capability model is binary. If `filesystem` is granted, `OpReadFile` and `OpWriteFile` accept arbitrary host paths (`os.ReadFile(path)`, [internal/vm/vm.go:5679](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5679); [internal/vm/vm.go:5698](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5698)). There is no chroot, directory jail, or path prefix validation. If `network` is granted, `OpFetch` sends HTTP requests to any IP/URL ([internal/vm/vm.go:5663](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5663)) [VERIFIED].
- **Gap 2: Arbitrary Host Execution via `OpExec`.** Granting `process` enables `OpExec`, which directly runs `exec.Command(cmdStr, args...).CombinedOutput()` ([internal/vm/vm.go:5715](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5715)). This entirely bypasses sandboxing, allowing the VM to spawn arbitrary host binaries [VERIFIED].
- **Gap 3: Memory Limit is Completely Dead Code.** While `VMLimits.MaxMemoryBytes` is declared (default 64MB, [internal/vm/error.go:3534](file:///tmp/hf-ainative-agents/repo-E/internal/vm/error.go#L3534)), `MaxMemoryBytes` is **never checked anywhere** in `BCVM.run` or allocation routines. A script creating a 2GB list will crash the host via OOM [VERIFIED].
- **Gap 4: Dynamic Re-compilation via `LazySynthesize`.** In `BCVM.run`, `OpCall` with `LazySynthesize` executes an HTTP POST to `localhost:11434`, parses the returned LLM string as Lisp, calls `bytecode.CompileToBytecode`, and mutates the program function table at runtime ([internal/vm/vm.go:6185-6230](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L6185-L6230)). This is a textbook arbitrary code injection vector [VERIFIED].

---

### 6. Does the `.howl` surface syntax matter for this thesis, or should models target HFIR/JSON directly? What should the model-facing contract be?

**`.howl` surface syntax does NOT matter for the product thesis.**

- **Evidence from the Repo:** The author’s own experimental model adapter (`hfir_model_adapter_phase1`) completely bypasses `.howl` and AST parsing, accepting canonical JSON transport directly into `hfir.Graph` ([docs/hfir_model_adapter_status.md:804-806](file:///tmp/hf-ainative-agents/repo-E/docs/hfir_model_adapter_status.md#L804-L806); [internal/hfir/model_adapter.go:3235-3304](file:///tmp/hf-ainative-agents/repo-E/internal/hfir/model_adapter.go#L3235-L3304)) [VERIFIED].
- **Why Text S-Expressions Fail:** Generating text S-expressions requires models to track parenthesis balancing, escaping, and formatting. In the adapter benchmark, raw `.howl` generation achieved only a 30% first-pass pass rate (3/10), whereas constrained JSON schema achieved 10/10 ([docs/hfir_model_adapter_status.md:834-840](file:///tmp/hf-ainative-agents/repo-E/docs/hfir_model_adapter_status.md#L834-L840)) [VERIFIED].
- **Recommended Model-Facing Contract:** Models should output **strictly typed JSON or Protobuf representations of HFIR** constrained via grammar-guided decoding (Outlines/JSON Schema). S-expression `.howl` should remain purely a human debugging and inspection syntax [INFERRED].

---

### 7. How does it compare with WASM/WASI, Starlark, CEL, OPA/Rego, Lua, Rhai, Deno? What is genuinely differentiated vs. reinvented?

*(See the extensive comparison table in Section E for granular technical dimensions.)*

**Genuinely Differentiated:**
- **Co-designed Semantic Repair Protocol:** The combination of stable node identities, verification diagnostics referencing specific `NodeID`s, and single-node replacement deltas (`replace_node` in [internal/hfir/model_adapter.go:3360-3374](file:///tmp/hf-ainative-agents/repo-E/internal/hfir/model_adapter.go#L3360-L3374)) is tailored for LLM self-correction loops without full-context regeneration [VERIFIED].
- **AOT Verifier Gate on Program Graphs:** Checking effect inference and construct feasibility before compilation ([internal/hfir/verifier.go:2895-2989](file:///tmp/hf-ainative-agents/repo-E/internal/hfir/verifier.go#L2895-L2989)) provides a structured bridge between probabilistic generation and execution [VERIFIED].

**Reinvented Worse:**
- **Custom Stack VM vs. Wasmtime:** Reinvented a Go-based bytecode interpreter instead of using standard WASM bytecode with Wasmtime's battle-tested fuel consumption and epoch deadlines [INFERRED].
- **Coarse Capabilities vs. Deno / WASI:** Capability gates are coarse strings (`network`, `filesystem`) without granular path sandboxing or domain allowlists [VERIFIED].
- **Custom Policy vs. CEL / Rego:** Policy scripts in HowlFrame require Turing-complete loops and mutable state, whereas CEL and Rego provide mathematically bounded non-Turing-complete evaluation [INFERRED].

---

### 8. What existing features conflict with the thesis and should be quarantined, cut, or redesigned?

The following features directly contradict the core thesis ("intent is not authority", determinism, verification) and must be excised:

| Feature / Primitive | Location | Defect / Conflict | Recommendation |
| :--- | :--- | :--- | :--- |
| **`lazy_synthesize`** | [internal/vm/vm.go:4454-4485](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L4454-L4485), [6182-6231](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L6182-L6231) | JIT HTTP call to Ollama; compiles and executes unverified code at runtime. | **CUT IMMEDIATELY.** Model calls belong at generation time, not VM runtime. |
| **`ephemeral_circuit` / `neural_circuit` / `achieve` / `confidence`** | [internal/vm/vm.go:5938-6071](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5938-L6071) | Hardcodes unauthenticated HTTP calls to `localhost:11434`; nondeterministic runtime effects. | **QUARANTINE / CUT.** Strip from VM opcodes. |
| **`OpExec` (exec)** | [internal/vm/vm.go:5708-5719](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5708-L5719) | Spawns arbitrary host processes; destroys sandbox isolation. | **CUT OR REDESIGN** into a pre-registered action dispatch catalog. |
| **Transpilation Heads (Go, JS, WAT)** | [howlframe.go:1967-2024](file:///tmp/hf-ainative-agents/repo-E/howlframe.go#L1967-L2024) | Massive maintenance burden for a solo builder; diverts focus from runtime verification. | **QUARANTINE** into legacy compatibility maintenance. |
| **`fuzzy_cast` & `assert_semantic`** | [internal/construct/construct.go:2734](file:///tmp/hf-ainative-agents/repo-E/internal/construct/construct.go#L2734), [2753](file:///tmp/hf-ainative-agents/repo-E/internal/construct/construct.go#L2753) | Unsupported in bytecode; Go-codegen only; relies on subjective LLM prompts. | **CUT.** |

---

### 9. Top 5 risks (technical, security, product), ranked.

1. **Security — Ambient Host Authority (Rank 1):** The capability gate allows arbitrary path writes and shell execution upon granting coarse flags. A prompt injection producing `(exec "rm" "-rf" "/")` executes directly on the host if `process` is granted ([internal/vm/vm.go:5708-5719](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5708-L5719)) [VERIFIED].
2. **Product — Lack of Market Moat / "Why Not Wasm/Starlark?" (Rank 2):** Why would a platform team adopt a custom Go VM and bespoke Lisp when they can embed Wasmtime or Starlark with zero language-adoption overhead? [INFERRED]
3. **Technical — Resource Starvation via Unenforced Memory Limits (Rank 3):** `MaxMemoryBytes` is dead code ([internal/vm/vm.go:4930-6611](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L4930-L6611)). An untrusted program can allocate unbounded memory, crashing the host agent process [VERIFIED].
4. **Product — Solo Builder Maintenance Exhaustion (Rank 4):** The author is maintaining a parser, checker, SSA IR, WAT emitter, Go generator, JS generator, bytecode VM, and native store. Progress on core verification is stalled due to surface sprawl ([docs/architecture_roadmap.md:577-586](file:///tmp/hf-ainative-agents/repo-E/docs/architecture_roadmap.md#L577-L586)) [VERIFIED].
5. **Technical — Fragile Binary Serialization (`gob`) (Rank 5):** Bytecode artifacts rely on Go's internal `gob` serialization ([internal/bytecode/artifact.go:2454](file:///tmp/hf-ainative-agents/repo-E/internal/bytecode/artifact.go#L2454)). Any struct change risks breaking artifact compatibility across toolchain updates [VERIFIED].

---

### 10. What should the capability/authority model become (granularity, resource scoping, budgets, model calls as effects)?

The authority model must graduate from boolean flags to **scoped capability descriptors**:

1. **Fine-Grained Resource Scoping:**
   - **Filesystem:** Replace `"filesystem"` with read/write directory trees: `fs:read:[/tmp/sandbox, ./fixtures]`, `fs:write:[/tmp/sandbox/out]`. Paths outside the jail fail closed.
   - **Network:** Replace `"network"` with domain/method rules: `net:http:[api.github.com:GET, internal.auth:POST]`.
   - **Process:** Remove arbitrary `exec`. Replace with pre-registered action handles: `action:[run_test, stage_build]`.
2. **Deterministic Budgets:**
   - Instruction budget (gas/fuel) per invocation ([internal/vm/vm.go:5485](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5485)) [VERIFIED].
   - **Implement strict heap allocation budgets:** Track bytes allocated in `push`, `MakeList`, and `MakeDict`, enforcing `MaxMemoryBytes`.
   - Wall-clock deadlines via context cancellation.
3. **Model Calls as External Effect Nodes:**
   - LLMs must **never be invoked by opcodes inside the VM**.
   - If an agent needs an LLM call, it must yield an effect request (`YieldEffect`) to the host runner, carrying a token budget and schema. The runner executes the call, records the prompt/response in the audit log, and resumes the VM [INFERRED].

---

### 11. What does "verified" concretely mean here — what can be statically checked, what must be runtime-enforced, what cannot be claimed?

- **What CAN be statically checked (AOT Gate):**
  - Graph reference integrity and absence of dangling edges ([internal/hfir/verifier.go:2974-2983](file:///tmp/hf-ainative-agents/repo-E/internal/hfir/verifier.go#L2974-L2983)) [VERIFIED].
  - Construct support for the target backend via `construct.Scan` ([internal/hfir/constructs.go:29-55](file:///tmp/hf-ainative-agents/repo-E/internal/hfir/constructs.go#L29-L55)) [VERIFIED].
  - Coarse capability requirement inference ([internal/hfir/verifier.go:2899-2912](file:///tmp/hf-ainative-agents/repo-E/internal/hfir/verifier.go#L2899-L2912)) [VERIFIED].
  - Static type assertions for scalar operations ([internal/checker/checker.go](file:///tmp/hf-ainative-agents/repo-E/internal/checker/checker.go)) [VERIFIED].
- **What MUST be runtime-enforced:**
  - Dynamic type safety on JSON payloads and mixed collections (`TYPE_ERROR`, [internal/vm/vm.go:5442](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5442)) [VERIFIED].
  - Instruction budget exhaustion (`LIMIT_EXCEEDED`, [internal/vm/vm.go:5486](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5486)) [VERIFIED].
  - Memory consumption limits.
  - Granular I/O path and domain access.
- **What CANNOT be claimed:**
  - **Formal Verification:** HowlFrame does not prove program correctness, termination, or functional invariants. Calling this "formal verification" is misleading [VERIFIED].
  - **Semantic Soundness of Prompts:** Qualitative properties (`assert_semantic`) cannot be verified; they are non-deterministic inferences [INFERRED].

---

### 12. Prioritized Roadmap: P0 (now) .. P4 (later/never)

| Priority | Item | Size | Rationale |
| :--- | :--- | :---: | :--- |
| **P0** | **Enforce memory limits in `BCVM` & sandbox path reads** | **S** | Fixes critical DoS and path traversal vulnerabilities; makes sandboxing claims truthful. |
| **P0** | **Excise runtime LLM opcodes (`lazy_synthesize`, `neural_circuit`, etc.)** | **S** | Eliminates code injection vectors and cleans the VM instruction set. |
| **P1** | **Standardize HFIR JSON Schema & Model Adapter Protocol (#88)** | **M** | Formalizes the machine-authoring contract; enables Outlines/BAML constrained generation. |
| **P1** | **Scoped capability descriptors (paths, domains, methods)** | **M** | Transforms coarse capability flags into real production sandboxing. |
| **P2** | **Production flip of `-compile-bc` to Lowered HFIR (#90)** | **L** | Unifies compiler pipeline, retiring parallel AST bytecode compilation once parity checklist passes. |
| **P2** | **Multi-node semantic repair transaction protocol (#91)** | **M** | Enables localized model self-correction loops on verified graphs. |
| **P3** | **Retire Go/JS transpilation heads; target WASM runtime** | **L** | Frees solo builder from maintaining 4 distinct backend emitters. |
| **P4** | **"Extreme AI" Paradigms (Swarm concurrency, auto-mutating runtime)** | **L** | Pure distraction; speculative marketing features that dilute the core security value. |

---

### 13. The single first falsifiable experiment

- **Hypothesis:** An LLM generating structured JSON-HFIR under schema-constrained decoding (Outlines/JSON Schema) achieves >90% first-pass executable verification and costs 50% fewer output tokens compared to generating equivalent Python scripts for bounded data-filtering and policy tasks.
- **Setup:** 20 real-world tasks (e.g., parse JSON, filter by criteria, check permission rules, write result). Run 50 iterations per task across Claude 3.5 Sonnet and GPT-4o.
  - *Condition A:* Model outputs standard Python; executed in a restricted subprocess.
  - *Condition B:* Model outputs HFIR JSON; verified by `hfir.NewVerifier` and executed on `howlframe run-bc`.
- **Metric:** (1) First-pass compile & verify rate. (2) Output token count. (3) Sandbox escape / violation rate.
- **Pass/Kill Criteria:**
  - *Pass:* HFIR achieves ≥85% first-pass pass rate with ≥30% token reduction and zero privilege escapes.
  - *Kill:* HFIR pass rate is lower than Python, or models repeatedly fail schema validation, proving the custom IR adds friction without reliability gains.
- **Cost:** ~$50 in inference API calls; 3 days of evaluation scripting.

---

### 14. Advising the Owner (Solo Builder) for the Next 2 Weeks

**What EXACTLY to do:**
1. **Fix the memory leak/DoS bug in `internal/vm/vm.go`:** Add a heap allocation counter inside `BCVM` and enforce `vm.Limits.MaxMemoryBytes`.
2. **Delete the dead Ollama runtime calls:** Strip `lazy_synthesize`, `neural_circuit`, `ephemeral_circuit`, `achieve`, and `confidence` from `vm.go` and `construct.go`.
3. **Harden Path Operations:** Modify `OpReadFile` and `OpWriteFile` to validate paths against a configured root directory jail (`--sandbox-dir`).
4. **Complete the HFIR Model Adapter (#88):** Write a clean Python/Node test harness verifying that an LLM generating JSON conforming to `hfir_model_adapter_phase1.schema.json` executes cleanly without ever writing `.howl` text.

**What EXACTLY NOT to do:**
1. **DO NOT touch the Go/JS transpilation backends:** Do not add features to `gogen` or `javascript`. Leave them frozen.
2. **DO NOT implement "Swarm actor models" or agent concurrency:** Resist building multi-agent messaging in the VM.
3. **DO NOT invent DOM APIs or web features:** Keep the runtime strictly headless.
4. **DO NOT prematurely flip `-compile-bc` default:** Maintain standing rule HOWL-CANON-010 until parity criteria are 100% satisfied.

---

## Role E: Competitive & Prior-Art Analysis

### Rigorous Architectural Comparison Table

| System | Execution Engine | Determinism | Termination / Cost Bounds | Capability Model | Static Verifiability | Audit / Provenance | Model-Gen Friendliness | Ecosystem Maturity |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **HowlFrame** | Custom Go Stack VM (`BCVM`) | High (strict key sorting, integer rules) | Instruction ceiling (`MaxInstructions`), Unenforced RAM | Coarse boolean grants (`network`, `filesystem`) | Ahead-of-time HFIR verifier, construct scan | SHA-256 manifests, execution trace, state ledger | High via JSON-HFIR; Low via `.howl` text | Experimental (Solo prototype) |
| **WASM / WASI (Wasmtime)** | Native JIT / AOT (C/Rust Engine) | Very High (IEEE-754 strict, WASM spec) | Built-in fuel metering, epoch deadlines | Capability-based file descriptors, network sockets | Bytecode validation pass (`wasmparser`) | Component model interfaces (WIT), external hashes | Medium (models cannot write raw Wasm bytecode) | Industry Standard (Bytecode Alliance) |
| **Starlark (Bazel)** | Tree-walk / Bytecode (Go/Java/Rust) | Absolute (hermetic, no thread/time/IO) | Bounded execution steps, call stack limits | Zero I/O by design (pure hermetic data logic) | Static syntax & binding checks | Deterministic build action caches | Very High (Python subset, near-zero syntax errors) | Production Mature (Google, Meta) |
| **CEL (Common Expression Lang)** | Expression Evaluator (Go/C++/Java) | High (pure functions, no side-effects) | Non-Turing complete (no loops, bounded cost estimator) | Context-variable bindings only | Strong static type checking & cost estimation | Evaluation traces, AST audit logs | Very High (designed for one-shot evaluation) | Production Mature (Kubernetes, Envoy) |
| **OPA / Rego** | Datalog / Query Engine (Go/Wasm) | High (declarative policy queries) | Stratified Datalog (bounded recursion) | Query evaluation over input documents; no ambient IO | Comprehensive static analysis, rule indexing | Detailed decision logs & query provenance | Medium (Rego syntax requires specialized prompting) | Cloud Native Standard (CNCF Graduated) |
| **Lua / Luau Sandbox** | Register-based VM (C) | High (when os/io libraries removed) | Instruction counts via debug hooks / Luau limits | Explicit environment table (`_ENV`) sandboxing | Luau has gradual static typing | External host logging | Very High (widely targeted, tiny grammar) | Very High (Roblox, Gaming, Nginx) |
| **Rhai** | AST / Bytecode Evaluator (Rust) | High (when configured) | Max operations, max call depth, string size limits | Fine-grained module/function exclusion | Syntax checks; dynamic typing | Engine-level evaluation hooks | High (JavaScript/Rust-like intuitive syntax) | Mature (Rust Embedded scripting) |
| **Deno** | V8 Engine (C++/Rust) | Medium (full JS/TS engine) | Process-level timeouts, V8 heap limits | Granular CLI flags (`--allow-read=/tmp`, `--allow-net=api.com`) | TypeScript type checking | Permission prompt logs | High (models write excellent TypeScript) | Production Mature |
| **eBPF Verifier** | In-Kernel VM / JIT | Absolute (kernel space) | Strict instruction limit, guaranteed termination | Helper function allowlists per BPF program type | Formal verification: DAG acyclicity, pointer bounds | BPF subsystem logs | Extremely Low (requires C/LLVM toolchain) | Linux Kernel Core |
| **CUE / Dhall** | Unification / Proof Engine | Absolute (Hermetic) | Total functional languages (Guaranteed termination) | Pure data calculations; No external I/O | Strong mathematical type proofs | Content-addressed hashes | Medium (specialized syntax, steep learning curve) | Mature (Configuration / Infra) |
| **Agent Tool Schemas (MCP)** | JSON-RPC over Host / Stdio | None (depends on host tool implementation) | None (per-tool timeouts) | Explicit tool list exposed to agent | Schema validation (JSON Schema draft-07/2020-12) | Server transaction logs | Universal (Native LLM function calling) | Exploding Standard (Anthropic, Industry) |

---

### What HowlFrame Should Steal

1. **Wasmtime’s Fuel and Epoch Interruption Model:**
   HowlFrame’s manual instruction counter ([internal/vm/vm.go:5488](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5488)) requires checking an integer on every loop step. Wasmtime implements low-overhead asynchronous epoch deadlines and gas/fuel decrementing compiled directly into control blocks. HowlFrame should adopt epoch-based timeouts to prevent runaway CPU loops during native Go calls.
2. **CEL’s Static Cost Estimation:**
   CEL statically computes the worst-case computational complexity of an expression tree *before* executing a single instruction. HowlFrame’s AOT HFIR verifier ([internal/hfir/verifier.go:2895](file:///tmp/hf-ainative-agents/repo-E/internal/hfir/verifier.go#L2895)) already inspects the full graph; it should compute a static upper bound on execution steps and memory allocations, rejecting infeasible graphs ahead of time.
3. **Deno’s Scoped Permission Descriptors:**
   Deno’s permission flags (`--allow-read=/var/data`, `--allow-net=api.internal.com`) represent the industry standard for user-space sandboxing. HowlFrame must replace its boolean strings (`capability.Filesystem`) with scoped resource filters.
4. **OPA’s Decision Log Format:**
   OPA records the exact input context, package evaluated, rules triggered, and decision output. HowlFrame’s `ArtifactManifest` ([internal/hfir/manifest.go:3029](file:///tmp/hf-ainative-agents/repo-E/internal/hfir/manifest.go#L3029)) and execution trace ([internal/vm/vm.go:5037-5076](file:///tmp/hf-ainative-agents/repo-E/internal/vm/vm.go#L5037-L5076)) should serialize into an immutable, signable decision log.

---

### Where HowlFrame is Reinventing Something Worse

1. **A Custom Go-Based Bytecode VM instead of WebAssembly:**
   Building a custom stack-based bytecode virtual machine in Go is an enormous liability. Go’s garbage collector and memory model make strict memory limiting difficult (evidenced by the unenforced `MaxMemoryBytes`). Compiling HFIR to standard Wasm bytecode and executing it inside an embedded `wasmtime-go` engine would instantly provide military-grade memory sandboxing, true CPU fuel metering, cross-platform speed, and multi-language interoperability.
2. **Proprietary Lisp (`.howl`) instead of Starlark or JSON/CEL:**
   Creating a custom S-expression language for models to write is counterproductive. LLMs frequently hallucinate S-expression syntax (e.g., nesting rules for `let`, [bugs.md:48](file:///tmp/hf-ainative-agents/repo-E/bugs.md#L48)). Starlark (Python syntax) has millions of times more representation in model training data.
3. **Boolean Capability Flags instead of Object Capabilities (OCaps):**
   A capability in HowlFrame is an ambient boolean string granted to the runtime ([internal/capability/capability.go:2135-2144](file:///tmp/hf-ainative-agents/repo-E/internal/capability/capability.go#L2135-L2144)). This is not true capability-based security; it is an ambient access control list. In true OCaps (or WASI Component Model), code cannot even name a file or socket unless it holds an explicit handle passed by its caller.

---

## Confidence and Blind Spots

### Verified Evidence:
- Full inspection of `howlframe.go`, `internal/vm/vm.go`, `internal/bytecode/`, `internal/construct/construct.go`, `internal/capability/capability.go`, `internal/hfir/` (verifier, manifest, model_adapter, constructs, incremental, storage), `apps/action_executor/`, `apps/release_authority/`, `bugs.md`, `improvements.md`, `docs/architecture_roadmap.md`, `docs/hfir_execution_status.md`, `docs/hfir_model_adapter_status.md`, and `git log --oneline -40`.
- Verified dead-code status of memory limiting (`MaxMemoryBytes`).
- Verified ambient, unconfined execution of `OpReadFile`, `OpWriteFile`, and `OpExec`.
- Verified live HTTP calls to local Ollama endpoints in `lazy_synthesize`, `neural_circuit`, `ephemeral_circuit`, `achieve`, and `confidence`.

### Blind Spots & Unverified Items:
- **No Test Execution Ran in this Session:** Shell execution was strictly prohibited by instructions. All claims are based on direct code inspection, regression fixtures, committed journals, and git history.
- **`contracts/howl/` Directory:** The directory `contracts/howl/` was unresolvable as a file path via `view_file` (stat error).
- **Go/JS Transpiler Code Generation:** Inspected top-level entry points in `howlframe.go` and backlog notes; did not line-by-line audit the full implementations of `internal/backend/gogen/` or `internal/backend/javascript/`.
- **Live Performance / Latency:** Instruction budget overhead and VM dispatch performance were inferred from Go runtime characteristics, not measured via live benchmarks in this turn.
