VERIFIED: HowlFrame runs standalone bytecode, but its authority boundary has concrete holes.
VERIFIED: Three VM model operations attempt HTTP requests with no network grant.
VERIFIED: A process-only grant allowed filesystem writes through a child shell.
INFERRED: The thesis is credible for small, brokered workflows; broad application synthesis weakens it.
INFERRED: Spend the next two weeks closing effect paths and testing one customer job.

### 1. What is HowlFrame today?

**VERIFIED:** A working experimental toolchain: `.howl` parsing, source transformations, semantic checking, an HFIR gate, production AST-to-bytecode compilation, versioned artifacts, and VM execution. Production `-compile-bc` explicitly calls `bytecode.CompileToBytecode`; experimental `-compile-hfir-bc` calls the graph lowerer (`howlframe.go:134`, `howlframe.go:177`, `howlframe.go:194`). I built the CLI and executed scratch programs successfully.

**VERIFIED:** The VM implements file/process/network/environment operations, native stores, HTTP serving, and model calls (`internal/vm/vm.go:1989`, `:2115`, `:2163`, `:2300`, `:2527`). The interpreter supports a smaller subset and rejects unsupported forms (`internal/vm/vm.go:827`). Go/JS generation also exists, but shares neither complete semantics nor complete authority enforcement.

**VERIFIED:** Direct model-authored graph intake exists as a bounded library contract with strict decoding and verification before lowering (`internal/hfir/model_adapter.go:159`, `:182`). Broad synthesis, formal correctness proofs, complete effect auditing, and resource containment are not established. Some architecture prose is stale: it still says runtime capability enforcement is absent, despite actual VM gates (`docs/architecture_roadmap.md:31`; `internal/vm/vm.go:1945`).

### 2. What is it becoming?

**VERIFIED:** Recent commits move default execution toward standalone artifacts, expand VM dogfooding across application roots, and harden cooperative agents. The latest four commits repair agent body execution, failure isolation, return handling, and length validation ([captured history](/tmp/hf-ainative-scratch-B/git-log.txt:1)). Earlier commits incrementally extend experimental lowering ([history](/tmp/hf-ainative-scratch-B/git-log.txt:24)).

**INFERRED:** It is becoming a small application runtime with a synthesis-facing verification layer. That direction is more concrete than “AI-native language,” but the accumulation of HTTP, persistence, concurrency, and model-management features risks recreating a general-purpose platform.

**VERIFIED:** The production-lowering freeze remains explicit; #90 stays Partial (`docs/reference/lowered_hfir_prod_flip_criteria.md:3`, `:7`).

### 3. Is the thesis sound?

**INFERRED:** Yes, conditionally. The strongest argument is that models can propose bounded computation while a trusted host independently supplies authority. A small program can express branching and transformations more flexibly than a fixed tool call without receiving arbitrary host access.

**VERIFIED:** The adapter already excludes grants, instruction policies, backend details, and behavioral oracles from model proposals (`docs/hfir_model_adapter_status.md:22`).

**INFERRED:** The strongest argument against is economic and architectural: structured tool calls plus a trusted executor may solve the same jobs with far less implementation and security burden. Current unchecked model effects undermine the specific security promise. The product must demonstrate that *programmability* provides measurable value beyond a finite action catalog.

### 4. Who would use it?

**INFERRED:** An internal automation/platform engineer allowing an agent to assemble a short workflow: inspect supplied release evidence, transform structured records, choose approved actions, and submit a bounded mutation.

The alternatives are JSON tool calls plus Go glue, Starlark with host functions, or a policy engine plus an executor. The buyer would choose HowlFrame only if it reduces integration and review effort while preserving independently enforced authority.

**VERIFIED:** Action Executor is close to this job: proposals select a finite catalog; trusted arguments supply approval/state; execution maps to fixed effects (`apps/action_executor/action_executor.howl:28`, `:43`, `:130`). It does not establish demand for arbitrary AI-generated applications.

### 5. What is the trusted computing base?

**INFERRED:** For source intake: parser, include/module expansion, transformations, checker, HFIR gate, construct registry, compiler, decoder, VM, effect implementations, Go runtime, dependencies, runner, and OS. Generated Go/JS add backend generators and their host runtimes. Arbitrary subprocesses expand the practical boundary to everything those subprocesses can access.

**VERIFIED:** Concrete gaps include unchecked model opcodes, unscoped grants, incomplete artifact structural validation, runtime synthesis bypassing the normal checker/HFIR pipeline, and unenforced memory limits (`internal/bytecode/opcode.go:122`, `:150`; `internal/bytecode/artifact.go:229`; `internal/vm/vm.go:2666`; `internal/vm/error.go:39`).

**INFERRED:** Trustworthy for supervised experiments; insufficient as the sole containment boundary for hostile programs.

### 6. Does `.howl` syntax matter?

**INFERRED:** Keep it for debugging, human review, existing applications, and compatibility. Its compact syntax is not the security differentiator.

Models should target a versioned semantic JSON contract when that contract covers their job. It should expose allowed operations, typed inputs, bounded collections, explicit effect requests, stable identities, and localized diagnostics. Authority must remain separate.

**VERIFIED:** Today’s transport is much narrower than source execution: functions, loops, files, stores, networking, processes, and model operations are excluded (`docs/hfir_model_adapter_status.md:14`). Intake has concrete byte/node/input limits (`internal/hfir/model_adapter.go:32`).

**INFERRED:** Expand that contract only through consumer experiments. Do not expose raw bytecode or quietly substitute the experimental lowerer for production.

### 7. Comparison with existing systems

**VERIFIED — external documentation; INFERRED — product assessment:**

| Alternative | Assessment |
|---|---|
| WASM/WASI + components | Host-controlled capabilities and typed component interfaces already address isolation/interoperability. HowlFrame could differentiate through semantic synthesis and evidence, not a new sandbox. [WASI security](https://wasi.dev/security) |
| Starlark | Deterministic core and host-defined functions are a strong baseline for short workflows. HowlFrame must prove added value from graph intake and repair. [Language specification](https://raw.githubusercontent.com/google/starlark-go/master/doc/spec.md) |
| CEL | Better baseline for predicates and small transformations; non-Turing-complete and host-data-oriented. HowlFrame’s opportunity is bounded sequencing of effects. [CEL](https://cel.dev/) |
| OPA/Rego | Established policy evaluation. Effect freedom depends on configuration: `http.send` exists. Pairing policy with a trusted executor competes directly with the release demo. [HTTP built-ins](https://www.openpolicyagent.org/docs/policy-reference/builtins/http) |
| Lua | Mature embeddable execution; the host must constrain libraries and functions such as `os.execute`. HowlFrame currently reinvents much of that integration responsibility. [Manual](https://www.lua.org/manual/5.4/manual.html) |
| Rhai | Embedded scripting with explicit safety controls is another credible baseline. A novel grammar alone offers little advantage. [Safety controls](https://rhai.rs/book/safety/index.html) |
| Deno permissions | Existing effect permissions, including the same subprocess escape limitation: children execute outside parent restrictions. [Permissions](https://docs.deno.com/runtime/reference/permissions/) |
| gVisor / Firecracker | Complementary containment for hostile code, not semantic authorization. They reduce host exposure but cannot decide whether deployment is approved. [gVisor](https://gvisor.dev/docs/), [Firecracker design](https://github.com/firecracker-microvm/firecracker/blob/main/docs/design.md) |

**INFERRED:** The plausible differentiation is a *small semantic proposal contract plus independently brokered effects and reviewable execution evidence*. All three must work together.

### 8. Which features conflict?

**INFERRED:** Quarantine runtime synthesis, ephemeral model creation/deletion, arbitrary `exec`, ambient imports, and unconstrained concurrency from the AI-safe profile. Preserve compatibility through explicit legacy profiles rather than immediate deletion.

**VERIFIED:** Lazy synthesis parses model output and directly compiles it into a function at runtime, then mutates the function instructions (`internal/vm/vm.go:2666`, `:2678`, `:2684`). Ephemeral circuits create and delete provider-side models (`internal/backend/gogen/gogen.go:2106`, `:2112`).

**INFERRED:** Probabilistic semantic assertions should provide advisory results, never authorization. The documented aspiration that they “enforce” qualitative boundaries invites misuse (`docs/ai_native_paradigms.md:35`).

### 9. Top five risks, ranked

1. **VERIFIED:** Missing effect gates permit network attempts under empty grants. See red-team finding A.
2. **VERIFIED:** Coarse process authority defeats separation from filesystem/environment/network authority. See B.
3. **VERIFIED/INFERRED:** Instruction accounting leaves blocking I/O, allocation, output, and concurrency inadequately contained. See C.
4. **VERIFIED/INFERRED:** “Verified artifact” may be confused with approved identity or correct behavior; checksum validation provides neither (`internal/bytecode/artifact.go:166`; `internal/hfir/manifest.go:15`).
5. **INFERRED:** A solo builder spends months maintaining multiple semantics and backends before finding a user who needs programmable proposals.

### 10. What should authority become?

**INFERRED:** Runner-owned immutable resource handles, not five global booleans:

- Files: separate read/write, preopened roots or exact objects, byte quotas, symlink-safe resolution.
- Network: endpoint/service, method, redirect policy, resolved-address restrictions, timeout and transfer quotas.
- Database: named connection, allowed operations/transactions, row/byte limits.
- Processes: trusted action IDs and validated arguments; otherwise isolated execution.
- Models: separate inference from model administration; provider/model, prompt-data policy, token/cost/request limits.
- Streams/time: bounded input/output handles and explicit clock access.

Child tasks should receive attenuated handles and share aggregate budgets. Approval should bind action, resource, artifact hash, state version, and expiry; validate state atomically when committing.

**VERIFIED:** Current grants are five classes, and child VMs inherit the parent list (`internal/capability/capability.go:8`; `internal/vm/vm.go:2233`, `:3030`).

### 11. What does “verified” mean?

**VERIFIED:** Current checks establish selected semantic roles, reference existence, construct support, and structural artifact properties—not full program correctness (`internal/hfir/verifier.go:52`, `:111`; `internal/construct/construct.go:287`; `internal/bytecode/artifact.go:208`). Production blocks only selected HFIR diagnostic codes (`howlframe.go:466`, `:514`).

**INFERRED:** Static checks can establish schema validity, permitted operation sets, references, operand layouts, effect requirements, and limited type/control invariants. Runtime must enforce concrete resources, dynamic arguments, quotas, deadlines, and state transitions.

Neither proves that a model’s plan is appropriate, an approval is authentic, external data is truthful, or deployed behavior is correct. Use precise wording: “passes verifier version X for profile Y,” with an explicit list of properties.

### 12. Prioritized roadmap

**INFERRED — recommendations:**

| Priority | Work | Size / rationale |
|---|---|---|
| P0 | Gate every implemented effect; quarantine unsupported authority paths; add zero-grant side-effect assertions. | **M:** fixes the central promise. |
| P0 | Publish a restricted execution profile excluding arbitrary subprocesses and runtime synthesis. | **S:** reduces exposed surface immediately. |
| P1 | Central effect broker, deadlines, transfer/output limits, aggregate task accounting; external memory containment. | **L:** makes “bounded” operational. |
| P1 | Strengthen artifact validation and approval-to-artifact binding. | **M:** hostile artifacts need independent validation. |
| P2 | Scoped resource handles and atomic state/approval checks for one workflow. | **M:** replaces ambient authority with useful authority. |
| P2 | Run the falsifiable experiment below. | **M:** determines whether programmability earns its cost. |
| P3 | Extend model transport and repair only for proven consumer needs; durable effect receipts. | **M:** develops the actual differentiation. |
| P4 | General language expansion, more backends, speculative DOM APIs, production HFIR flip. | **L / defer:** distracts from security and demand. |

Keep #90 Partial and artifact compatibility intact.

### 13. First falsifiable experiment

**INFERRED:** Hypothesis: model-generated small programs solve variable release-evidence workflows more successfully than fixed JSON action proposals, without expanding authority or increasing review time.

Setup: 30 predefined tasks, including branching/transformation needs; compare restricted HowlFrame proposals with JSON plans executed by the same trusted broker. Run 100 adversarial variants involving forged approvals, path substitutions, state changes, and unauthorized endpoints. Use one model, identical inputs, and identical action catalog.

Measure task completion, repair count, reviewer minutes, total model cost, unauthorized effects, and receipt completeness.

Pass: zero unauthorized effects, complete receipts, and either ≥15 percentage-point completion improvement or ≥30% review-time reduction. Kill the *programmable-product thesis for this niche* if it delivers no meaningful advantage. Any unauthorized effect blocks further execution trials until repaired.

Cost: approximately one builder-week plus a capped model budget of $100. Predeclare criteria before collecting results.

### 14. Exactly the next two weeks

**INFERRED:**

- Days 1–3: fix the three unchecked VM operations; audit every opcode against actual effects; test denied operations with recording transports.
- Days 4–5: restrict the AI-safe profile; remove runtime synthesis/arbitrary processes from it; document generated-code exclusions.
- Days 6–8: implement one brokered workflow with scoped resources, deadlines, bounded streams, approval binding, and state-version checks.
- Days 9–10: run the comparative experiment and show results to two prospective users.

Do not add paradigms, backend targets, swarm sophistication, DOM APIs, or lowerer coverage merely to empty a checklist. Do not market hostile-code safety while the authority holes remain.

### B — Security/capability red-team findings

**A. High: zero-grant model network effects — VERIFIED.**  
`CONFIDENCE`, `NEURAL_CIRCUIT`, and `EPHEMERAL_CIRCUIT` have no registry capability, yet invoke HTTP services (`internal/bytecode/opcode.go:122`, `:150`; `internal/vm/vm.go:2412`, `:2440`, `:2513`). Reproduce with:

```lisp
(cli_app (print (confidence "true")))
```

Compile to `/tmp`, then `-run-bc` without grants. I observed `IO_ERROR` from attempted socket creation rather than `CAPABILITY_DENIED`; neural and ephemeral programs behaved likewise ([results](/tmp/hf-ainative-scratch-B/probe-results.txt:1)). Successful requests were blocked by the environment, not HowlFrame.

**B. High: process grant carries host authority — VERIFIED.**  
Both VM and interpreter ran:

```lisp
(cli_app
  (exec "/bin/sh" "-c"
    "printf escaped > /tmp/hf-ainative-scratch-B/process-effect"))
```

with only `process`, creating the file. `exec.Command` has no containment configuration (`internal/vm/vm.go:750`, `:2170`). This is a policy-design gap, not a missing process gate. Generated Go/JS use equivalent unrestricted child execution (`internal/backend/gogen/gogen.go:496`; `internal/backend/javascript/javascript.go:317`).

**C. High: resource limits do not bound execution — VERIFIED/INFERRED.**  
A five-second sleep with a three-instruction ceiling remained blocked until externally killed ([results](/tmp/hf-ainative-scratch-B/probe-results.txt:11)). HTTP responses and subprocess output are read without VM byte quotas (`internal/vm/vm.go:2127`, `:2170`). `MaxMemoryBytes` is declared but not enforced in execution; ordinary calls do not check `MaxCallDepth` (`internal/vm/error.go:39`; `internal/vm/vm.go:2717`). Legacy `SPAWN` resets accounting and inherits grants (`internal/vm/vm.go:2233`). Aggregate exhaustion is **INFERRED**; I did not deliberately exhaust memory.

**D. High for generated-code security claims: incomplete gates — VERIFIED.**  
Generated Go directly emits model HTTP, SQL, listener, and goroutine operations (`internal/backend/gogen/gogen.go:1987`, `:1875`, `:1288`, `:1546`). Running generated `confidence` with empty grants attempted networking and then created `crash.json`; the panic handler writes it unconditionally (`internal/backend/gogen/gogen.go:1252`). JS `spawn` also lacks its process gate (`internal/backend/javascript/javascript.go:736`).

**E. Medium: compiler and artifact boundaries — VERIFIED/INFERRED.**  
Source includes read paths before checking/grants (`howlframe.go:142`; `internal/parser/parser.go:103`). I executed an included scratch file without filesystem permission. Compilation therefore needs its own restricted input boundary. Artifact SHA-256 detects corruption, not authorized authorship; validation checks opcodes/jumps/function existence but lacks comprehensive operand/block/stack checks (`internal/bytecode/artifact.go:166`, `:229`). Approval replay and TOCTOU attacks remain **INFERRED** risks.

**VERIFIED effect coverage:** VM file operations, fetch, SQL/database, HTTP response/listener operations, `LLM_GENERATE`, `ACHIEVE`, environment, and process operations receive registry gates before dispatch (`internal/bytecode/opcode.go:106`; `internal/vm/vm.go:1945`). File-backed stores additionally require filesystem authority (`internal/capability/capability.go:50`). Interpreter core effects gate before evaluation; unsupported operations reject, with extra network checks for circuits (`internal/vm/vm.go:305`). Generated Go/JS gate the six core operations named in README, not all effects (`README.md:59`).

**VERIFIED:** stdin, stdout/stderr, sleep, clock, and exit are ungated; CLI runners supply host streams (`internal/bytecode/opcode.go:129`, `:145`, `:163`; `internal/vm/vm.go:1659`, `:2734`). **INFERRED:** inherited stdin can disclose data, output can forge apparent receipts, and unbounded streams can exhaust resources. Treat them as explicit host channels.

### Confidence and blind spots

**VERIFIED:** Checkout remained unchanged. Capability, bytecode, HFIR, demo, and contract packages passed; focused VM grant/budget/agent tests passed. Broader validation failed on prohibited sockets and JS subprocess `EPERM`; it is not green ([test log](/tmp/hf-ainative-scratch-B/tests.log:3)).

I did not verify successful outbound requests, real model/database services, browser behavior, every generated construct, artifact fuzzing, memory exhaustion, races, approval replay, or customer demand. Findings distinguish executed effects, attempted effects, inspected code, and inferred exploitation.