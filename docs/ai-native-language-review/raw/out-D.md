**INFERRED:** Narrow HowlFrame sharply; the evidence does not justify funding a general synthesis platform.  
**VERIFIED:** Checked source → HFBC → VM works, and the restricted model adapter has a real validation boundary (`howlframe.go:177`, `internal/hfir/model_adapter.go:162`).  
**VERIFIED:** Authority enforcement has holes: three model primitives attempt HTTP calls with no grants (`internal/bytecode/opcode.go:122`, `internal/vm/vm.go:2394`).  
**INFERRED:** JSON action schemas plus trusted policy and execution code are the strongest default alternative.  
**INFERRED:** Fund only a time-boxed experiment proving that bounded semantic programs outperform that simpler design.

Labels below distinguish implementation or executable evidence (**VERIFIED**) from judgments, recommendations, and hypotheses (**INFERRED**). External comparisons marked VERIFIED refer to primary documentation, not benchmarks I ran.

## 1. What is HowlFrame today?

**VERIFIED:** It is a working experimental compiler and runtime. The source path parses, expands includes, resolves modules, applies transformations, and invokes the checker. Production `-compile-bc` then runs the HFIR gate and compiles the AST; experimental `-compile-hfir-bc` lowers the graph directly (`howlframe.go:134`, `howlframe.go:177`, `howlframe.go:194`).

**VERIFIED:** I built the CLI and ran a small `.howl` program through compilation, artifact loading, and VM execution. Capability, bytecode, construct, HFIR, release-authority, action-executor, and ecosystem-contract package tests passed. Selected VM instruction-limit and spawn tests also passed.

**VERIFIED:** Direct model transport → graph → verification → bytecode is implemented. Its decoder bounds input size, rejects unknown and duplicate fields, validates node identities and roles, and restricts executable kinds (`internal/hfir/model_adapter.go:32`, `internal/hfir/model_adapter.go:547`, `internal/hfir/model_adapter.go:570`).

**VERIFIED:** Universal typed CFG/SSA verification, comprehensive target feasibility, and automatic production manifest enforcement are ahead of implementation. The production gate explicitly acknowledges its limited coverage; artifact execution loads HFBC without demanding a manifest (`howlframe.go:124`, `howlframe.go:471`). The roadmap’s complete verification sequence is a direction, not a delivered guarantee (`docs/architecture_roadmap.md:70`).

## 2. What is it already becoming?

**VERIFIED:** The last 40 commits emphasize VM execution, artifact-producing default paths, numeric parity, experimental lowering, and repeated `SPAWN_AGENT` boundary repairs. Examples include `bda8660`, `716d8f6`, `5174d7c`, and the sequence `a286c64` through `d496d97`. The resulting spawn implementation executes captured task bodies cooperatively and accounts for child instructions (`internal/vm/vm.go:3000`, `internal/vm/vm.go:3030`).

**INFERRED:** It is becoming a small application runtime with an increasingly explicit authority vocabulary. That is closer to embedded scripting than a demonstrated AI synthesis product.

**VERIFIED:** The architectural documents also contain historical descriptions that no longer match current implementation: the roadmap says capabilities are advisory and artifacts lack an envelope, while code now enforces registered capabilities and reads a versioned, checksummed envelope (`docs/architecture_roadmap.md:31`, `docs/architecture_roadmap.md:41`, `internal/vm/vm.go:1945`, `internal/bytecode/artifact.go:115`). Documentation needs dated status boundaries.

## 3. Is the thesis sound?

**INFERRED — strongest argument for:** Untrusted generation separated from trusted execution is sound. A bounded semantic graph can make permitted behavior, diagnostics, and repairs easier to inspect than arbitrary generated source. This project has an actual implementation of that separation.

**VERIFIED:** The model-facing transport excludes grants and budgets; repair machinery rejects identity changes, kind changes, and unauthorized references (`docs/hfir_model_adapter_status.md:20`, `internal/hfir/model_adapter.go:425`).

**INFERRED — strongest argument against:** The soundness of the architecture does not establish the necessity of this runtime. Teams can obtain the same separation using JSON schemas, a policy engine, and trusted handlers. HowlFrame must demonstrate that *program composition and repair* provide enough value to pay for another compiler, artifact format, verifier, and runtime.

## 4. Who would use it?

**INFERRED:** The plausible buyer is an agent-platform team needing small, frequently generated evidence transformations or conditional action plans, with reproducible rejection and repair explanations.

**INFERRED:** Ordinary application developers are poor initial customers. Authorization-only customers are better served initially by CEL, Rego, or plain trusted code. Arbitrary generated-code customers have stronger reasons to choose an established sandbox.

**VERIFIED:** The action executor already illustrates the simpler alternative: proposals select a finite catalog, and trusted application code implements allowed effects (`apps/action_executor/action_executor.howl:54`, `apps/action_executor/action_executor.howl:155`).

**INFERRED:** That demo validates the proposal/executor pattern more directly than it validates a new language.

## 5. What is the trusted computing base?

**VERIFIED:** For source compilation, it includes parser, transformations, checker, construct classification, HFIR lowering/gate, AST compiler, artifact decoder, VM, and host-effect implementations (`howlframe.go:134`, `internal/construct/construct.go:10`, `internal/bytecode/artifact.go:196`). Model transport adds its decoder and repair machinery (`internal/hfir/model_adapter.go:162`, `internal/hfir/model_adapter.go:376`).

**INFERRED:** It is suitable for experiments, not currently a trustworthy hostile-code boundary. Concrete gaps:

- **VERIFIED:** `CONFIDENCE`, `NEURAL_CIRCUIT`, and `EPHEMERAL_CIRCUIT` lack opcode capability requirements but perform HTTP requests. My no-grant probes reached socket errors for all three, rather than capability denial (`internal/bytecode/opcode.go:122`, `internal/bytecode/opcode.go:150`, `internal/vm/vm.go:2412`, `internal/vm/vm.go:2447`, `internal/vm/vm.go:2513`).
- **VERIFIED:** `MaxMemoryBytes` is declared and defaulted but has no enforcement references under `internal/` (`internal/vm/error.go:39`).
- **VERIFIED:** Fetch reads the complete response; process execution captures complete output without a context deadline (`internal/vm/vm.go:2122`, `internal/vm/vm.go:2170`).
- **VERIFIED:** Runtime synthesis compiles returned source without invoking the normal checker/HFIR pipeline, then mutates function instructions (`internal/vm/vm.go:2666`).
- **VERIFIED:** Artifact validation checks opcode existence, selected jumps, and function references—not full stack/type/control-flow validity (`internal/bytecode/artifact.go:229`).

## 6. Does `.howl` syntax matter?

**INFERRED:** It matters for existing consumers and human debugging, but is not the thesis’s defensible core. Preserve compatibility; stop optimizing the product around a novel surface language.

**INFERRED:** Models should target a versioned semantic JSON contract, never raw bytecode or unrestricted internal `Graph` serialization. The contract should expose allowed operations, typed inputs, stable identities, structured diagnostics, and bounded repair transactions. The runner must supply authority and trusted behavioral checks.

**VERIFIED:** The existing adapter follows much of this design, but excludes functions, loops, files, process operations, network operations, and model calls (`internal/hfir/model_adapter.go:689`). Its exclusion boundary is materially narrower than source-level experimental lowering (`docs/reference/lowered_hfir_prod_flip_criteria.md:137`).

**INFERRED:** Do not widen it merely to match the language’s feature inventory.

## 7. Comparison with alternatives

| Alternative | VERIFIED baseline | INFERRED implication |
|---|---|---|
| WASM/WASI and component model | Wasmtime supports components; WASI filesystem access follows capability principles. [Wasmtime security](https://docs.wasmtime.dev/security.html), [WASI design](https://github.com/WebAssembly/WASI/blob/main/docs/DesignPrinciples.md) | Prefer it for general executable plugins. HFIR could eventually be an authoring layer above it. |
| Starlark | Go implementation provides deterministic execution-step limits, while warning that individual built-ins can consume substantial time or memory. [Starlark Go](https://github.com/google/starlark-go) | Strong rival for bounded configuration and transformations; avoids inventing language semantics. |
| CEL | Non-Turing-complete expression language accessing host-supplied data. [CEL](https://cel.dev/) | Better initial choice for predicates and simple evidence transformations. |
| OPA/Rego | Policy evaluation supports external-data retrieval; `http.send` is explicitly unsuitable for effecting changes. [OPA HTTP built-ins](https://www.openpolicyagent.org/docs/policy-reference/builtins/http) | Keep policy decisions separate from trusted execution. |
| Lua | Embedded runtime whose host controls exposed functions and environments. [Lua manual](https://www.lua.org/manual/5.4/manual.html) | Established scripting substrate; careful embedding remains necessary. |
| Rhai | Supports operation ceilings, but external calls can consume unspecified time. [Rhai limits](https://rhai.rs/book/safety/max-operations.html) | Closely resembles the bounded embedded-runtime proposition. |
| Deno permissions | Permissions can scope hosts, paths, and variables; subprocesses escape parent permission constraints. [Deno security](https://docs.deno.com/runtime/fundamentals/security/) | Already offers finer resource scoping than current HowlFrame grants. |
| gVisor / Firecracker | Provide system-level isolation machinery. [gVisor architecture](https://gvisor.dev/docs/architecture_guide/intro/), [Firecracker jailer](https://github.com/firecracker-microvm/firecracker/blob/main/docs/jailer.md) | Complement semantic policy; address a different boundary. |

**INFERRED:** The potential differentiation is stable semantic identity plus diagnostic-derived, authority-preserving repair and evidence. Capability checks, instruction counting, schemas, checksums, and sandboxing are established techniques.

**VERIFIED:** Bounded same-kind repair is implemented; that is more distinctive than the model-calling opcodes (`internal/hfir/model_adapter.go:376`, `internal/hfir/model_adapter.go:439`).

## 8. Which features conflict with the thesis?

**INFERRED:** Quarantine runtime synthesis, ephemeral model creation, semantic assertions used as authorization, unrestricted subprocesses, and permissive generated backends from the endorsed untrusted-program profile.

**VERIFIED:** Runtime synthesis changes executable instructions after initial compilation (`internal/vm/vm.go:2684`). Three advertised paradigms—`semantic_match`, `fuzzy_cast`, and `assert_semantic`—are unsupported by standalone bytecode (`internal/construct/construct.go:96`, `internal/construct/construct.go:115`, `internal/construct/construct.go:157`).

**VERIFIED:** `optimize_signature` records metadata and executes its body; it does not run the supplied tests or select candidates (`README.md:354`, `internal/construct/construct.go:139`).

**INFERRED:** Treat these as experiments with explicit status, not product pillars. Preserve artifacts and legacy behavior through profiles rather than casually deleting opcodes. Browser expansion is irrelevant to validating this thesis; do not invent DOM APIs.

## 9. Top five risks, ranked

1. **INFERRED — false authority assurance:** A “no grants” claim is currently contradicted by executable model-call probes. This threatens the central promise (`internal/bytecode/opcode.go:122`).
2. **INFERRED — no compelling customer advantage:** The strongest demos use fixed action catalogs readily implemented without generated programs (`apps/action_executor/action_executor.howl:54`).
3. **INFERRED — resource exhaustion:** Instruction counting cannot contain blocking calls or unbounded allocations; memory-limit enforcement is absent (`internal/vm/error.go:39`, `internal/vm/vm.go:2127`).
4. **INFERRED — semantic duplication:** AST production compilation, experimental lowering, interpreter, and generated backends create recurring compatibility work (`docs/hfir_execution_status.md:26`, `docs/hfir_execution_status.md:67`).
5. **INFERRED — evidence inflation and solo-builder overload:** Small stored benchmarks and extensive architecture work can create apparent progress without proving adoption or economic advantage (`docs/hfir_model_adapter_status.md:35`, `docs/architecture_roadmap.md:121`).

## 10. What should authority become?

**INFERRED:** Replace broad grants in the endorsed profile with runner-issued handles: read-only input, output sink, named action, approved destination, or bounded database operation.

**VERIFIED:** Current grants distinguish only network, filesystem, process, environment, and database classes (`internal/capability/capability.go:8`).

**INFERRED:** Scope operations and resources separately: read/write, destination/method, environment key, executable/argument template, database/table/action. Check actual resource use at dispatch, including redirects and path resolution.

**INFERRED:** Add deadlines, output/response byte limits, allocation containment, action counts, child budgets, and model token/spend limits. Model calls must be explicit effects with approved provider/model, disclosure policy, validated responses, and replay evidence.

**INFERRED:** Generated code must never own the policy that authorizes itself. Approval and authoritative state need a trusted channel; caller-supplied strings are acceptable only when the caller is explicitly trusted.

## 11. What does “verified” mean?

**VERIFIED:** Today it means specific checks passed. The transport checks schema-like integrity, roles, bounds, cycles, and reachability; compilation verifies and structurally validates its output (`internal/hfir/model_adapter.go:570`, `internal/hfir/model_adapter.go:933`, `internal/hfir/model_adapter.go:182`).

**VERIFIED:** The general HFIR verifier checks references, selected roles, inferred capability effects, and a limited Wasm feasibility set. Despite a “Cycle” comment, it has no cycle traversal. Production blocking codes are only `HFIR_INVALID_REF` and `HFIR_TARGET_INFEASIBLE` (`internal/hfir/verifier.go:111`, `internal/hfir/verifier.go:132`, `howlframe.go:466`).

**INFERRED:** Static checks can establish representation validity, supported operations, selected type invariants, and declared effect requirements. Runtime checks must enforce actual resource access, budgets, and state freshness. Neither establishes business correctness, trustworthy evidence, universal termination, or prompt-injection immunity.

**VERIFIED:** CAS manifests bind hashes and verification records; they do not authenticate an issuer or prove the verifier’s correctness (`internal/hfir/manifest.go:134`, `internal/hfir/storage.go:88`). The ecosystem contracts correctly distinguish valid shape from true claims (`contracts/howl/SOURCE.md:41`).

## 12. Prioritized roadmap

| Priority | INFERRED recommendation | Rationale | Size |
|---|---|---|---|
| P0 | Fix missing model-effect gates; mechanically audit all effectful handlers | Central authority promise currently fails | S–M |
| P0 | Publish one narrow supported execution profile and exact guarantees | Prevent misleading security expectations | S |
| P1 | Add deadlines, bounded I/O, and external memory containment | Fuel alone is inadequate | M |
| P1 | Bind artifact, runner policy, inputs, toolchain identity, and outcomes in execution records | Build hashes alone are insufficient audit evidence | M |
| P2 | Run the head-to-head experiment below | Test whether a product advantage exists | M |
| P3 | Extend transport and repair only for demonstrated customer bottlenecks | Avoid speculative language expansion | M–L |
| P4 | Defer broad backend migration, native targets, browser features, ecosystem, and autonomous synthesis | Excess scope before demand | L |

**VERIFIED:** The production flip criteria explicitly defer promotion and keep #90 Partial (`docs/reference/lowered_hfir_prod_flip_criteria.md:155`).

**INFERRED:** Leave that decision intact. Clearing the lowering fence is not the immediate product priority.

## 13. First falsifiable experiment

**INFERRED — hypothesis:** Small generated semantic programs deliver materially better task success and repair cost than JSON action plans with trusted policy handlers.

**INFERRED — setup:** Use one real design partner and 30 held-out evidence-transformation/action-planning tasks. Compare existing HFIR transport against JSON actions plus CEL or straightforward trusted code. Give both identical examples, model budgets, trusted inputs, allowed actions, and hidden behavioral tests. Include malformed proposals, misleading evidence, and attempts to widen authority. Execute effects through the same trusted broker.

**INFERRED — metrics:** Hidden-test success, prohibited effects, repair tokens, human intervention time, latency, and integration effort.

**INFERRED — pass:** Zero prohibited effects; at least 90% behavioral success; either a 15-percentage-point success advantage or 30% lower repair effort, without more than twice the integration effort.

**INFERRED — kill/pivot:** Any authority escape blocks release. No meaningful advantage means stop expanding the compiler and retain useful broker, diagnostic, or evidence components.

**INFERRED — cost:** Roughly 7–10 builder-days and a capped $300–$1,000 model budget. These are proposed experiment limits, not measured costs.

## 14. Exactly what to do for two weeks

**INFERRED:**

- **Days 1–2:** Reproduce and fix the three ungated model effects. Add behavioral no-effect assertions and audit every host-call handler.
- **Days 3–4:** Define the narrow profile, implement essential runtime containment, and correct guarantee language.
- **Days 5–6:** Build the JSON-policy baseline and recruit one independent design partner.
- **Days 7–10:** Run held-out generation and repair tasks; record failures and intervention costs.
- **Days 11–14:** Make a written continue/pivot/stop decision. Fix only blockers demonstrated by the experiment.

**INFERRED:** Do not flip production lowering, mark #90 Done, add DOM APIs, redesign syntax, break artifact compatibility, expand swarms, or spend the fortnight chasing backend parity.

## D — Strongest case for narrowing, pivoting, or stopping

**INFERRED:** The strongest objection is that HowlFrame combines several worthwhile ideas without proving that they require one new language/runtime. Established execution substrates can enforce containment; JSON schemas and trusted handlers can separate proposals from authority; policy engines can make decisions. The missing commercial evidence is why semantic programs improve that composition enough to justify ownership of another toolchain.

**VERIFIED:** The synthesis evidence is eleven stored candidates, averaging 8.64 nodes, from one restricted author. The status document explicitly disclaims provider reliability and broad application synthesis (`docs/hfir_model_adapter_status.md:35`, `docs/hfir_model_adapter_status.md:91`). Replay validates fixtures; it does not measure live generation reliability.

**VERIFIED:** Release Authority reads approval and evidence from CLI strings and writes an in-memory state record. Action Executor takes current state from CLI arguments (`apps/release_authority/release_authority.howl:33`, `apps/release_authority/release_authority.howl:118`, `apps/action_executor/action_executor.howl:43`). These are demonstrations, not authenticated deployment workflows.

**VERIFIED:** I also supplied an `inspect` proposal containing a quoted target. The app exited successfully but emitted invalid JSON because it interpolates the target directly (`apps/release_authority/release_authority.howl:108`). This is an output-contract defect, not an authorization bypass.

**INFERRED:** I would fund only bounded evidence transformation and repair, with immutable trusted policy and a fixed external action broker. I would change my mind after independent users demonstrate repetitive tasks where local semantic repair reduces effort substantially, while adversarial tests and resource containment hold. Without that evidence, pivot to the broker/evidence layer or stop.

## Confidence and blind spots

- **VERIFIED:** High confidence in the identified missing gates, unused memory limit, production gate scope, and invalid-JSON reproduction.
- **VERIFIED:** Validation is incomplete: the aggregate test command failed because VM `httptest` listeners are forbidden here (`internal/vm/ast_interp_cap_test.go:306`). Selected socket-free VM tests passed; I did not establish green CI.
- **NOT VERIFIED:** Live model quality, successful model-service calls, full cross-backend equivalence, browser consumers, hostile-input fuzzing, or independent customer demand.
- **VERIFIED:** Tracked files were unchanged. Scratch work and cache stayed under the requested `/tmp` paths. The pre-existing untracked `REVIEW_CONTEXT_PACK.md` remained untouched. No journal was written because this review was explicitly read-only.