VERIFIED — HowlFrame has a working checked-source → bytecode → capability-gated VM pipeline (`howlframe.go:177`, `internal/vm/vm.go:1940`).  
VERIFIED — Production HFIR is a selective verification gate, not the canonical executable representation (`howlframe.go:466`, `:490`).  
INFERRED — The thesis is credible for small, governed automation, but the evidence does not yet justify a hostile-code security promise.  
VERIFIED — I reproduced interpreter/VM versus Go disagreement over whether a denied effect executes (`internal/vm/vm.go:1095`, `internal/backend/gogen/gogen.go:1412`).  
INFERRED — The next investment should prove one useful workflow against an existing alternative, while tightening authority and execution receipts.

### 1. What is HowlFrame today?

**VERIFIED:** It is an experimental language, compiler suite, and embedded execution runtime. Source passes through parsing, includes, module resolution, AST transformations, and checking. Production bytecode then passes through the HFIR gate before AST-to-bytecode emission (`howlframe.go:134`, `:177`). Artifacts can be loaded and executed independently through the VM (`howlframe.go:124`).

The bare path writes bytecode for `cli_app`, `http_server`, `web_app`, and `wasm_app`; only an unflagged CLI program automatically executes. Those root names do not guarantee execution in a browser or WebAssembly engine (`howlframe.go:233`). Explicit Go, JavaScript, and Wasm build paths remain separate (`howlframe.go:784`, `:813`, `:765`).

**VERIFIED:** Model JSON can directly become a bounded HFIR candidate, pass verification and lowering, and become structurally validated bytecode (`internal/hfir/model_adapter.go:162`, `:182`). This is real, but the transport excludes functions, loops, files, network requests, and process operations (`internal/hfir/model_adapter.go:689`).

**INFERRED:** Broad application synthesis, complete semantic verification, durable execution auditing, and uniform backend security remain aspirations. The architecture roadmap itself acknowledges overlapping semantic owners (`docs/architecture_roadmap.md:35`).

### 2. What is it already becoming?

**VERIFIED:** The inspected forty-commit history emphasizes standalone bytecode adoption, bounded HFIR lowering, numeric parity, and VM hardening. The current implementation removes implicit Go fallback and executes task bodies cooperatively inside the VM (`howlframe.go:256`, `internal/vm/vm.go:3002`). The latest commit addresses malformed spawn-body lengths (`docs/journals/2026-10-05_spawn_agent_bodylen_bounds.md:5`).

**INFERRED:** It is becoming a small application runtime with authority checks, rather than primarily a transpiler. However, HTTP, browser, Wasm, database, and swarm compatibility still pull it toward a general platform. That breadth competes directly with the narrower product thesis.

The architectural documents also contain competing directions: canonical semantic graph, Wasm-first execution, and a frozen HFIR verification-gate role (`docs/architecture_roadmap.md:5`, `:59`, `:85`). Product prioritization needs to resolve that tension without changing the production compiler default.

### 3. Is the thesis sound?

**INFERRED — strongest argument for:** Models produce proposals more reliably when the available vocabulary is finite and rejection is deterministic. HowlFrame already separates candidate content from runner authority: transport cannot express grants or budgets, and VM dispatch checks supplied capabilities (`internal/hfir/model_adapter.go:117`, `internal/vm/vm.go:1945`). This is a useful foundation.

**INFERRED — strongest argument against:** Most of the security value comes from the trusted host API, resource policy, and evidence handling. Those can surround existing languages. A new language adds compiler defects and unfamiliar semantics without automatically improving governance.

The thesis is sound; the necessity of HowlFrame is unproven. “AI-generated” identifies the source of untrusted programs, not a defensible technical advantage by itself.

### 4. Who would use it, for what job?

**INFERRED:** The best initial customer is an engineer building internal automation who needs models to compose several approved operations: inspect evidence, transform records, select an allowed artifact, and request a bounded mutation.

That customer might otherwise use JSON tool calls with a Go executor, Starlark with host functions, or OPA/CEL plus imperative glue.

**VERIFIED:** Action Executor demonstrates a finite catalog and restricts artifact choices to `app-v1` or `app-v2` (`apps/action_executor/action_executor.howl:54`, `:68`). That is a plausible job boundary.

**INFERRED:** The product earns its place only when programs need meaningful composition beyond a single tool call, and its review/repair workflow materially reduces engineering effort. Simple approval rules are unlikely to justify adopting another language.

### 5. What is the trusted computing base?

**VERIFIED:** For source execution, it includes parser, module resolver, transformations, checker, construct registry, HFIR lowering/gate, bytecode compiler, artifact decoder, VM, host-effect implementations, and runner policy (`howlframe.go:134`, `:490`, `internal/construct/construct.go:10`). Artifact-only execution bypasses the source checker and HFIR gate (`howlframe.go:124`).

For generated Go/JS, the generators, helpers, target runtime, and accessible imports add further trust.

Concrete gaps:

- **VERIFIED:** HFIR production blocking is limited to invalid references and target infeasibility; missing-role diagnostics are not universally blocking (`howlframe.go:466`, `internal/hfir/verifier.go:58`).
- **VERIFIED:** Artifact validation checks opcode identity, jumps, and function existence, but has no stack/type dataflow analysis (`internal/bytecode/artifact.go:229`).
- **VERIFIED:** `MaxMemoryBytes` is declared, but inspection found no enforcement in `vm.go`; ordinary calls recurse without checking `MaxCallDepth` (`internal/vm/error.go:39`, `internal/vm/vm.go:2697`). Spawn depth is checked (`internal/vm/vm.go:3009`).
- **VERIFIED:** Fetch uses the default HTTP client and reads the entire response; process execution uses `CombinedOutput` (`internal/vm/vm.go:2122`, `:2127`, `:2170`).
- **VERIFIED:** Lazy synthesis parses model output and compiles it directly, without the normal checker/HFIR pipeline (`internal/vm/vm.go:2666`).

**INFERRED:** Trustworthy enough for controlled experiments; insufficiently established as an adversarial execution boundary.

### 6. Does `.howl` syntax matter?

**INFERRED:** Keep `.howl` for compatibility, debugging, and human-readable examples. For the thesis, model-facing semantic JSON is more valuable because it names nodes, roles, and repair preconditions explicitly.

**VERIFIED:** The existing adapter already rejects unsupported transport forms and bounds candidates to 128 nodes and 64 KiB (`internal/hfir/model_adapter.go:32`, `:159`). Repairs carry graph/node preconditions and trusted edit regions (`internal/hfir/model_adapter.go:262`).

The model-facing contract should be a separately versioned, supported semantic subset—not arbitrary serialized `Graph`, which contains AST type structures (`internal/hfir/graph.go:19`). It should specify operand order, binding scope, evaluation order, errors, effects, and size limits. Authority, oracles, compiler identity, and execution policy belong to the runner.

Do not widen the transport merely to match source lowering.

### 7. Comparison with existing systems

The following external descriptions are **VERIFIED against primary documentation**; positioning judgments are **INFERRED**.

| Alternative | Relationship to HowlFrame |
|---|---|
| WASM/WASI + components | WASI provides capability-oriented host access; components provide typed interoperability. Wasmtime supplies fuel/interruption. HowlFrame’s opportunity is semantic proposals and repair evidence above that layer. ([Security](https://docs.wasmtime.dev/security.html), [interruption](https://docs.wasmtime.dev/examples-interrupting-wasm.html), [components](https://component-model.bytecodealliance.org/design/why-component-model.html)) |
| Starlark | A close competitor for deterministic, embedded automation with host-defined functions. Its hermetic core already addresses much of the restricted-language premise. ([Design principles](https://github.com/bazelbuild/starlark)) |
| CEL | Preferable for bounded expressions and predicates; non-Turing-complete and restricted to host-provided data. HowlFrame could serve larger procedural compositions. ([CEL](https://cel.dev/?hl=en)) |
| OPA/Rego | Strong alternative for policy decisions. Execution still needs an external executor; OPA explicitly warns against using `http.send` for mutations. ([OPA HTTP built-ins](https://www.openpolicyagent.org/docs/policy-reference/builtins/http)) |
| Lua | Established embedding primitives, configurable environments, and instruction hooks; host integration determines containment. HowlFrame must demonstrate better contracts and reviewability. ([Lua manual](https://www.lua.org/manual/5.4/manual.html)) |
| Rhai | Already supplies operation ceilings and embedded scripting. Budgeting is reinvented here; semantic repair may differentiate. ([Operation limits](https://rhai.rs/book/safety/max-operations.html)) |
| Deno permissions | Already scopes permissions to resources such as directories, hosts, and environment keys. HowlFrame’s category-wide grants are less expressive today. ([Deno security](https://docs.deno.com/runtime/fundamentals/security/)) |
| gVisor/Firecracker | Process/container isolation and microVM isolation address host containment, not intent correctness. They are complementary deployment layers. ([gVisor](https://gvisor.dev/docs/), [Firecracker](https://github.com/firecracker-microvm/firecracker)) |

**INFERRED:** The credible differentiation is a bounded semantic program plus localized repair and reviewable authority evidence. Bytecode, permission checks, JSON schemas, and instruction counting are established techniques.

### 8. Which features conflict with the thesis?

**INFERRED recommendations:**

- Quarantine `lazy_synthesize` from the governed execution profile. It changes executable behavior after initial checking (`internal/vm/vm.go:2666`).
- Treat semantic assertions, fuzzy routing, and confidence as advisory model outputs. The paradigm document describes qualitative assertions as enforcement, which is too strong (`docs/ai_native_paradigms.md:31`).
- Keep arbitrary `exec`, broad network access, and file-backed stores outside the default model profile. Grants currently name entire categories (`internal/capability/capability.go:8`).
- Freeze new Go/JS/WAT feature expansion while the core execution contract stabilizes.
- Make child failures structured results. Spawn currently prints failures and may continue the parent (`internal/vm/vm.go:3036`).

Preserve existing compatibility surfaces; quarantine through an explicit profile rather than deleting constructs casually.

### 9. Top five risks, ranked

1. **INFERRED — authority confusion:** Trusted caller-supplied assertions may be presented as independently established evidence. Approval and state arrive through CLI arguments (`apps/action_executor/action_executor.howl:43`).
2. **INFERRED — incomplete resource containment:** Instruction fuel does not bound blocking calls, allocation, response size, or output (`internal/vm/vm.go:2127`, `:2996`).
3. **INFERRED — semantic divergence:** Different execution paths disagree about effect evaluation (`internal/bytecode/bytecode.go:703`, `internal/backend/gogen/gogen.go:1412`).
4. **INFERRED — no demonstrated adoption advantage:** The documented synthesis sample is small and explicitly limited (`docs/hfir_model_adapter_status.md:35`).
5. **INFERRED — audit overclaim:** Build hashes and a `Verified` boolean can be mistaken for authenticated execution evidence (`internal/hfir/storage.go:88`, `internal/hfir/manifest.go:15`).

### 10. What should authority become?

**INFERRED:** Move from category booleans toward runner-created resource handles:

- Files: separate read/write authority, rooted directories, byte quotas, safe path resolution.
- Network: destination, method, redirect, response-size, and deadline constraints.
- Processes: approved executable/action IDs, bounded arguments, environment, output, and lifetime.
- Stores: namespace, operation, record-size, and mutation limits.
- Children: attenuated grants and shared aggregate budgets.
- Models: explicit provider/model access, request/token/cost ceilings, deadlines, and replay records.

**VERIFIED:** File stores already require both database and filesystem permission, showing useful resource-dependent enforcement (`internal/capability/capability.go:46`).

**INFERRED:** Approval should bind artifact hash, normalized action, resource, state version, expiry, and nonce. Model output must never redefine the policy or evidence oracle.

### 11. What does “verified” mean?

**VERIFIED today:** Selected structural checks, supported-construct rejection, inferred capability labels, narrow feasibility checks, artifact integrity, and limited bytecode validation (`internal/hfir/verifier.go:33`, `howlframe.go:510`, `internal/bytecode/artifact.go:166`).

**INFERRED desired static checks:** Schema/version, IDs/references, role cardinality, binding resolution, known types, stack discipline, operand domains, supported targets, and conservative effect requirements.

**INFERRED runtime obligations:** Resource authorization, dynamic type checks, instruction/allocation/output quotas, deadlines, cancellation, state preconditions, and actual effect receipts.

**INFERRED forbidden claims:** Functional correctness, factual model accuracy, termination within wall-clock bounds from fuel alone, absence of host vulnerabilities, or backend equivalence outside measured cases.

Verification results should name the checks performed and their versions. A bare `Verified: true` is too ambiguous.

### 12. Prioritized roadmap

All items below are **INFERRED recommendations**.

| Priority | Item | Size | Rationale |
|---|---|---:|---|
| P0 | Define a governed execution profile and truthful verification report | S | Establish exactly what is promised; exclude lazy synthesis |
| P0 | Capture the boolean effect-order mismatch as differential regression | S | A reproduced defect affects authority and behavior |
| P0 | Add runner deadlines and bounded effect input/output | M | Fuel cannot constrain host calls |
| P1 | Implement one scoped filesystem or action capability | M | Demonstrate useful least authority |
| P1 | Emit artifact-bound execution receipts | M | Connect proposal, policy, outcome, and effects |
| P1 | Run the falsifiable customer experiment below | M | Test whether the language earns its complexity |
| P2 | Strengthen bytecode validation and runtime accounting | L | Validate embedded bodies, stack/control flow, recursion, allocations |
| P2 | Broaden trusted-oracle repair evaluation | M | Test the plausible differentiation |
| P3 | Consolidate semantics only for a proven subset | L | Reduce duplication without a wholesale migration |
| P4 | Broad language growth, native code, provider ecosystem | L | Defer until demand warrants maintenance cost |

Keep production AST emission, the experimental lowering fence, and #90 Partial (`docs/reference/lowered_hfir_prod_flip_criteria.md:157`).

### 13. First falsifiable experiment

**INFERRED proposal:** Test whether generated programs improve bounded release-preparation work over a JSON action-plan executor written in Go.

Use 30 held-out tasks requiring three to six steps: read supplied evidence, transform records, choose an allowed artifact, and propose a staging mutation. Run the same model with equal budgets in both systems. Give both identical trusted operations and hidden behavioral oracles. Include malformed proposals, denied resources, and stale approvals.

Measure correct completion, forbidden effects, human review minutes, repairs, and model cost.

Pass only with zero forbidden effects, at least 90% correct completion, and at least 25% lower median review effort without higher total cost. Kill the current product framing on any authority bypass or no meaningful advantage; use the simpler executor.

Cost: **INFERRED estimate**, five to seven builder-days and a capped $200 model budget. Preregister criteria and preserve transcripts. Existing adapter results should inform setup, not substitute for this experiment (`docs/hfir_model_adapter_status.md:35`).

### 14. Exactly what to do for two weeks

**INFERRED plan:**

Days 1–2: specify the governed subset, evidence boundary, and experiment. Correct misleading entry-point documentation; README still describes implicit Go output despite the current bytecode path (`README.md:140`, `howlframe.go:233`).

Days 3–5: resolve boolean evaluation semantics, add meaningful differential coverage, and introduce deadlines plus bounded reads/output for the experiment’s operations.

Days 6–8: build the Go JSON-plan baseline, hidden oracles, scoped action boundary, and execution receipts.

Days 9–10: run held-out trials, inspect every failure, and publish the decision with consumer findings in `docs/journals/`, as repository instructions require (`AGENTS.md:7`).

Do **not** expand DOM/browser APIs, add opcodes for coverage’s sake, flip production HFIR lowering, build a linker, rewrite all backends, or polish branding before obtaining comparative evidence.

### Compiler & runtime architect assessment

**VERIFIED:** Semantic ownership is distributed across checker, interpreter, AST bytecode compiler, HFIR lowerer, shared tree IR, SSA, and target generators (`howlframe.go:148`, `:179`, `:196`, `:288`; `internal/backend/gogen/gogen.go:1400`). HFIR is a real experimental executable graph, but production emission discards that graph and compiles the AST. Construct verification itself still walks the AST (`howlframe.go:502`).

The most concrete divergence is:

```lisp
(cli_app
  (print (and (= 1 2) (= (env "HF_TEST") "x"))))
```

**VERIFIED by execution:** Without grants, interpreter and VM fail on the environment read; generated Go prints `false` successfully. Interpreter and bytecode eagerly evaluate both operands; Go emits `&&` (`internal/vm/vm.go:1095`, `internal/bytecode/bytecode.go:703`, `internal/backend/gogen/gogen.go:1387`). This changes effects and denial behavior, not merely formatting.

**VERIFIED:** Fetch is another known divergence: bytecode compiles only URL/method, while the other hosts can send a body (`internal/bytecode/bytecode.go:480`, `docs/reference/lowered_hfir_prod_flip_criteria.md:132`).

**INFERRED:** The smallest architecture is one governed subset, one versioned semantic contract, one execution engine, and one effect broker. Keep source compilation as compatibility and direct HFIR as the bounded experiment; do not make every backend a product requirement.

**VERIFIED:** HFBC has meaningful improvements over naked gob: magic/version, SHA-256, a 10 MiB payload ceiling, trailing-data rejection, structural checks, sorted function serialization, and v1 reader compatibility (`internal/bytecode/artifact.go:14`, `:146`, `:171`, `:196`). Determinism tests passed (`internal/bytecode/artifact_determinism_test.go:36`).

**INFERRED:** It is a reasonable internal artifact, not yet a durable authenticated audit format. Checksums establish integrity, not authorization or source correspondence. Preserve compatibility; add canonical disassembly and external receipts binding compiler identity, artifact, policy, inputs, effects, and outcome. Current manifests omit several of those fields (`internal/hfir/manifest.go:15`).

### Confidence and blind spots

- **VERIFIED:** Required implementation areas and forty commits inspected; checkout remained clean.
- **VERIFIED:** CLI build and seven focused package suites passed. Selected VM tests also passed.
- **VERIFIED:** Broader VM validation remains incomplete: loopback-listener creation was prohibited by this environment.
- **Not verified:** Full CI, race/fuzz campaigns, live model providers, browser execution, external consumers, OS containment, cross-version gob behavior, or customer demand.
- **INFERRED:** High confidence in pipeline ownership and the reproduced divergence; moderate confidence in security completeness; low confidence in market advantage until the comparative experiment runs.