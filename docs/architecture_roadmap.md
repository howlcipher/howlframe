# Zero Architecture Roadmap

## Decision

Zero should evolve from an AI-friendly S-expression language into an AI-native software synthesis platform. The existing `.zero` syntax remains a useful transitional interchange format, but it must not remain the product's canonical representation. The canonical artifact should become a versioned, typed semantic program graph called Zero IR (ZIR).

The intended path is:

```text
human intent -> model adapter -> ZIR -> verifier -> optimizer -> backend -> executable
                                  |             |             |
                                  |             |             +-> conformance evidence
                                  |             +-> diagnostics and proof artifacts
                                  +-> stable node identities and incremental cache keys
```

ZIR is machine-authored and machine-consumed. It is not a human programming language and does not need surface-language conveniences. Humans should interact with prompts, generated application behavior, diagnostics, and reviewable evidence.

## Evidence-Based Current State

The checked-in toolchain is real and useful:

| Area | Strength | Constraint |
| --- | --- | --- |
| Front end | Lexer, parser, AST transforms, semantic checker, and JSON diagnostics are present. | The parser and AST remain the source of truth. There is no stable semantic artifact. |
| Execution | Direct interpreter and versioned bytecode VM avoid generated source for bounded `cli_app` programs. | Interpreter and VM each define execution semantics separately. |
| Lowering | `internal/ir` has a shared tree adapter and a typed SSA/CFG graph. | The shared adapter covers only selected forms; SSA is effectively a Wasm-specific path. |
| Backends | Go, JavaScript, legacy WAT, and typed SSA-to-WAT paths demonstrate target diversity. | Backend coverage and semantics diverge; Go remains the broadest path. |
| Verification | Checker, WAT validator, unit tests, fixtures, deterministic mask plans, and optimization plans exist. | No program-wide invariant/proof model, backend conformance contract, or CI. |
| AI integration | Provider-neutral mask plans and a local structured-bytecode experiment exist. | The orchestrator is provider-specific in practice and its JSON schema is not a verified ZIR contract. |
| Safety | Bytecode opcodes label capabilities and the VM has instruction limits. | Capability labels are not enforced at runtime yet. |

`go test ./...` and `python -m pytest -q tests/test_observer_agent.py` passed during this audit. That proves the current tests pass; it does not establish cross-backend semantic equivalence or native-binary readiness.

## Principal Architectural Risks

1. There is no canonical semantic representation. AST, shared `IRNode`, SSA, bytecode, and individual backend walkers overlap without a single ownership boundary. Fixing one semantics bug can require changes in several paths.
2. `zero.go` is the documented CLI. An ignored local `cmd/zero/main.go` scaffold contains an older path that does not invoke `checker.Check`, so it must remain outside the documented or shipped compiler surface unless it is intentionally reconciled in a future change.
3. Output handling is not a reliable compiler contract. Bug #39 was reproduced during this audit: placing `-o` after a source argument wrote generated `server.go` into the repository root instead of the requested directory.
4. Current "AI-native" primitives mostly call runtime model services or record metadata. They do not provide an AI-native program representation, proof boundary, or model-independent synthesis interface.
5. The `gob` bytecode artifact is Go-runtime-coupled and lacks an explicit compatibility, validation, or reproducibility envelope. It is a useful runtime format, not a durable interchange contract.
6. A capability vocabulary exists but is advisory. LLM-authored bytecode can execute filesystem, process, network, environment, and database operations without a policy gate.
7. The current write-cost benchmark measures authoring of human-visible `.zero` text. It is valuable baseline evidence, but it cannot validate the proposed ZIR workflow or localized semantic edits.

## Architecture Direction

### ZIR v1

ZIR v1 should be a deterministic serialized graph, with stable node IDs, explicit data and control edges, typed values, effects, source/provenance spans, declared capabilities, and invariant references. It should represent application intent independently of Go, JavaScript, Wasm, and model providers.

It must have three separate forms:

| Form | Purpose | Must not contain |
| --- | --- | --- |
| Semantic ZIR | Stable, model-editable program graph | Backend syntax or raw target-code strings |
| Lowered ZIR | Typed CFG/SSA and target-independent optimization facts | Model prompts or provider tokens |
| Artifact manifest | Hashes, target, toolchain version, capability grant, proof/test results | Secrets or full private prompt context |

The existing AST may lower into ZIR as a compatibility frontend. New model integrations should produce ZIR directly through a schema and validator, not free-form bytecode. The existing bytecode VM should eventually consume a lowered ZIR-derived bytecode, rather than become a competing semantic source.

### Verification Boundary

Verification must precede backend emission. The minimum verifier sequence is:

1. graph schema and reference integrity
2. type, ownership, and control-flow invariants
3. effect and capability inference, followed by an explicit policy grant
4. target feasibility checks
5. generated property, example, and differential tests
6. deterministic artifact manifest creation

Probabilistic operations such as LLM calls, semantic search, and confidence routing must be explicit effect nodes with declared budgets, replay data or test doubles, and policy-granted capabilities. They cannot be ordinary pure expressions.

### Backend Direction

Treat Wasm binary as the first production native artifact because it is portable, sandboxable, and already has an SSA/WAT foothold. Keep Go and JavaScript as compatibility backends, not the semantic reference. Introduce LLVM or direct machine code only after ZIR, verifier contracts, backend conformance tests, and a stable runtime ABI exist.

## Backlog Review

### Bugs

All historical bugs #1 through #38 are marked Done. Their root causes fall into four completed but still instructive classes: lexer/parser robustness (#1, #10, #12, #23), AST/backend shape divergence (#3, #13, #16, #18 through #31), fixture and dependency hygiene (#15, #22, #34 through #38), and late validation/stack safety (#5, #9, #32, #33). Retain their regression tests, but do not reopen them as isolated work.

| Bug | Root cause and current recommendation | Priority | Complexity |
| --- | --- | --- | --- |
| #1–#38 | Done. Preserve regression coverage; migrate shared semantic checks into ZIR verification rather than adding more backend-local guards. | Maintenance | No new work |
| #39 | Go's `flag` parsing stops at the first positional argument; only bytecode and SSA-Wasm paths compensate with `outputFlagAfterInput`. The documented Go/JS path does not. Fix while consolidating the CLI, define one artifact-output contract, and test every target/order combination. | P0 | Small alone; medium with CLI consolidation |

Bug #39 aligns directly with the long-term vision: deterministic, isolated artifacts are a prerequisite for reproducible synthesis. It must be fixed before treating generated applications as trustworthy build outputs.

### Existing Improvements

| Existing work | Decision |
| --- | --- |
| #1–#48, #50–#72 | Retain as historical shipped capability records. They establish useful compatibility surfaces but do not define the future architecture. Do not add feature variants before ZIR verification has one canonical owner. |
| #49 and #61 | Reclassify mentally as experiments, not completion of direct AI-native synthesis. Direct AST execution and binary `gob` bytecode prove useful execution paths; neither is a provider-neutral semantic IR or native executable pipeline. |
| #53 and #62 | Superseded as the long-term abstraction plan. The partial shared adapter and SSA are valuable inputs, but ZIR must replace their overlapping semantic ownership rather than extending per-backend AST walkers indefinitely. |
| #64, #65, #68–#70 | Retain and elevate as ZIR prerequisites. Type/layout facts, SSA, masking, optimization metadata, and schema bridges should become verifier and adapter products of ZIR. |
| #73 and #84 | Keep, but sequence after ZIR v1's semantic and lowering contracts. Expanding Wasm surface first would multiply a currently target-specific IR design. |
| #74 and #83 | Defer. LLM HTTP primitives and broad JS parity add runtime surface without advancing the canonical representation or verification boundary. |
| #75 | Keep as immediate foundation. CI is required before broad compiler evolution. |
| #76 | Keep and expand into a no-side-effect `validate` command that emits machine-readable diagnostics and a ZIR validation report. |
| #77 | Keep, but design it as part of stable diagnostic codes and severity levels, not a one-off unknown-symbol message. |
| #78 | Keep as the first conformance harness. It should compare only explicitly supported, deterministic subsets and become a ZIR backend test suite. |
| #79 | Keep as the first enforcement step. Evolve opcode labels into inferred ZIR effects plus a signed or supplied policy manifest. |
| #80 | Keep. Modules must be graph namespaces/import contracts, not textual includes or Go import passthrough. |
| #81 | Defer until the ZIR frontend contract is set. A formatter is useful for compatibility `.zero`, but source formatting is not foundational to the AI-native path. |
| #82 | Keep as low-risk regression work; land before #83 if JS expansion resumes. |

## Prioritized Milestones

| Milestone | Rationale | Dependencies | Impact | Effort |
| --- | --- | --- | --- | --- |
| 0. Stabilization and trust boundary | One checked CLI, no surprise artifacts, CI, stable diagnostic envelope. | None | Prevents unverified paths and protects all later work. | 2–4 weeks |
| 1. Core compiler contract | Define ZIR v1, AST-to-ZIR compatibility lowering, schema, canonical serialization, and node identity. | Milestone 0 | Creates the representation AI can edit locally. | 4–6 weeks |
| 2. Verification layer | ZIR verifier, effect inference, capability policy, generated tests, and deterministic manifests. | Milestone 1 | Changes Zero from “compile then find out” to verify before emission. | 6–8 weeks |
| 3. Model adapter layer | Provider-neutral ZIR schema adapters, constrained decoding integration, repair-delta protocol, and provenance redaction rules. | Milestones 1–2 | Enables Claude, ChatGPT, Gemini, and open models to produce the same verified artifact. | 4–6 weeks |
| 4. Optimizer and incremental store | Content-addressed ZIR cache, dependency invalidation, pure optimization passes, and localized recompilation. | Milestones 1–2 | Reduces session context, regeneration size, and compile cost. | 6–10 weeks |
| 5. Backend abstraction | Lowered ZIR ABI, deterministic target feasibility, and cross-backend conformance suite. | Milestones 1–4 | Separates semantic meaning from target implementation. | 6–8 weeks |
| 6. Native binary generation | Produce and validate Wasm binaries, package runtime/imports, and benchmark startup, size, and speed. | Milestone 5 | First portable native executable without Go/JS at runtime. | 8–12 weeks |
| 7. Performance | Profile compiler/runtime, optimize SSA and memory layout, add reproducible performance benchmarks. | Milestone 6 | Improves compile latency and executable performance with evidence. | Continuous |
| 8. Ecosystem | Module registry/lockfiles, plugin ABI, debugger/explanation views, IDE and CI integrations. | Milestones 2, 5 | Makes verified synthesis operational beyond one repository. | 8–16 weeks |
| 9. Future research | Formal proof backends, semantic-store indexes, autonomous repair with ZIR deltas, and additional native targets. | Milestones 2–6 | High leverage but intentionally non-blocking. | Research |

## Technology Choices

| Decision | Pros | Cons | Recommendation |
| --- | --- | --- | --- |
| Typed graph ZIR | Local edits, explicit dependencies/effects, backend independence, validation-friendly. | Requires migration and graph tooling. | Adopt as canonical model. |
| Keep `.zero` as canonical | Existing fixtures and small grammar. | Text rewrites remain coarse; human-language concepts leak into the core. | Compatibility frontend only. |
| Use existing `gob` bytecode as ZIR | Fast and already executable. | Go-specific, unstable as interchange, too low-level for semantic repair. | Keep as internal VM artifact only. |
| Wasm first for native output | Portable, sandboxable, existing SSA work, mature tooling. | Runtime/GC/host ABI design still required. | First native target. |
| LLVM first | Broad native performance and many targets. | Larger toolchain and target ABI complexity before semantic contracts are mature. | Defer until post-Wasm. |
| Runtime LLM primitives as normal operations | Expressive demos and rapid feature addition. | Nondeterminism, cost, security, and replay problems. | Model them as explicit effects with budgets and policy. |

## Success Measures

1. A localized change edits a bounded ZIR subgraph and recompiles only its reverse dependency closure.
2. Every emitted artifact has a deterministic manifest containing ZIR hash, compiler version, target, capability grant, and verification results.
3. Backends pass the same deterministic conformance cases or explicitly reject unsupported effects before emission.
4. Model adapters can exchange valid ZIR without provider-specific source syntax.
5. A Wasm binary runs without Go, Node, or a model service unless its verified capability policy explicitly requests one.
