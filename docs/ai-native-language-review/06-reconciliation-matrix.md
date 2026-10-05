# 06: Reconciliation matrix: claims against the codebase

Each row is a capability or claim from the thesis, the repo docs, or a
reviewer, checked against `d496d97` plus this PR's fix. Statuses:

| Status | Meaning |
| --- | --- |
| **EXISTS** | Implemented, wired to the CLI or runtime, and tested |
| **PARTIAL** | Implemented with material gaps |
| **PROTOTYPE** | Library or experimental flag only; not on the production path |
| **PLANNED** | Documented intent; no code |
| **ABSENT** | Neither code nor a concrete plan |
| **CONFLICTS** | The claim contradicts the code or the thesis |

## Execution pipeline

| Claim | Status | Evidence | Raised by |
| --- | --- | --- | --- |
| Source → checker → HFIR gate → AST bytecode → `.hfbc` → VM | **EXISTS** | `howlframe.go:134-177`. 38/38 Go packages pass. | All |
| HFIR is the canonical executable representation | **PROTOTYPE** | Production runs AST-compiled bytecode. Lowering exists only behind `-compile-hfir-bc`. #90 stays Partial. | A, C, D, F |
| HFIR gate "verifies" programs | **PARTIAL** | Blocks only `HFIR_INVALID_REF` and `HFIR_TARGET_INFEASIBLE` (`howlframe.go:466`). The verifier's "cycle" comment has no traversal behind it. | A, D, F |
| Construct registry stops silent drops | **EXISTS** | 87 Supported, 9 CompileTimeOnly, 11 Unsupported. `construct_coverage.md` has a drift test. | F |
| Artifact envelope (magic, version, SHA-256) | **EXISTS** | `internal/bytecode/artifact.go:115-198` | D, F |
| Artifact validation (stack, operands) | **PARTIAL** | Checks only opcode existence, jumps, and function references | B, D, F |
| Cross-backend semantic equivalence | **CONFLICTS** | `and`/`or`: the VM and interpreter evaluate eagerly; Go short-circuits (S10). Go/JS gate only 6 effects (S11). | A, B |
| `docs/architecture_roadmap.md` "capabilities are advisory, no envelope" | **CONFLICTS** (stale) | The VM enforces capabilities (`vm.go:1945`), and an envelope exists | D, F |

## Authority and limits

| Claim | Status | Evidence | Raised by |
| --- | --- | --- | --- |
| Default-deny capability grants on `-run-bc` | **EXISTS** (after this PR) | Before the PR, `CONFIDENCE`, `NEURAL_CIRCUIT`, and `EPHEMERAL_CIRCUIT` ran with zero grants. Fixed by bugs.md #57 and `model_call_capability_test.go`. | B, D, F, executor |
| Interpreter (`-run`) capability parity | **EXISTS** (after this PR) | `lazy_synthesize` was ungated in the interpreter. Fixed by #57. | Executor |
| Resource-scoped grants (paths, hosts, env keys) | **ABSENT** | There are five coarse classes (`capability.go:8-15`). Only the store URI adds `filesystem`. | All |
| Instruction budget | **EXISTS** | `--max-instructions`, default 100,000. Spawned agents are counted. | All |
| Memory budget (`MaxMemoryBytes`) | **CONFLICTS** | Declared and defaulted but never enforced. About 2 GB RSS was reproduced (S4). | A, B, D, E, F |
| Call-depth limit (`MaxCallDepth`) | **CONFLICTS** | Declared but not checked on `CALL`. Ends in a Go fatal stack overflow (S5). | A, executor |
| Wall-clock and host-call deadlines, byte caps | **ABSENT** | `fetch` and `exec` read everything with no deadline. `sleep` is outside the budget. | B, D, F |
| Child-agent grant attenuation | **ABSENT** | `SPAWN_AGENT` inherits the full grant | B |
| Model calls as explicit, budgeted effects | **PARTIAL** (after this PR) | They are now gated by `network`. There is no model, provider, or token budget. | All |
| Runtime code generation is checked | **CONFLICTS** | VM `lazy_synthesize` compiles model output without checker, HFIR, or registry, then mutates `fn.Instructions` (S3) | A, B, D, E |

## AI-facing contract and evidence

| Claim | Status | Evidence | Raised by |
| --- | --- | --- | --- |
| Strict JSON candidate transport → verified graph → bytecode | **PROTOTYPE** | `internal/hfir/model_adapter.go:162-196`. Library only, no CLI. | D, F, E |
| Bounded repair (`replace_node` v1, semantic repair v2) | **PROTOTYPE** | `model_adapter.go:25-39`, `:376-439` | D, F (C wrongly said absent) |
| Model synthesis evidence | **PARTIAL** (tiny) | 11 stored candidates. HFIR 10/10 against `.howl` 3/10, from one author (`hfir_model_adapter_status.md:35-43`). Benchmarks: v1 has 3 tasks; v2 has a harness and no results. | D, E, F |
| Token-density advantage over Python | **CONFLICTS** | v1 CSV: HowlFrame used fewer tokens than Python in 1 of 3 tasks | C |
| Required-capabilities inspection | **ABSENT** (CLI) | Effect inference exists in the HFIR verifier, but nothing is exposed for artifacts | F, executor |
| Execution receipt / audit record | **ABSENT** | The CLI throws away the bounded evidence object (`vm.go:1629-1644`) | A, F, executor |
| CAS manifest and incremental compile | **PROTOTYPE** | `internal/hfir/manifest.go`, `storage.go`, `incremental.go`. Library only. No issuer, compiler version, or grant binding. | A, D |
| Governed profile (construct allow-list for model-authored code) | **ABSENT** | None. See 11, C2. | D, F, executor |

## "AI-native" primitives and demos

| Claim | Status | Evidence | Raised by |
| --- | --- | --- | --- |
| Swarm and autonomous agents | **PARTIAL** | `SPAWN_AGENT` runs cooperatively in-process. There is no message passing between agents. | E, F |
| `optimize_block` / `optimize_signature` self-optimization | **CONFLICTS** (doc against code) | They run their body and record metadata. They do not run candidate tests. | D, E |
| Stochastic control flow and an auto-mutating runtime (`extreme_ai_paradigms.md`) | **CONFLICTS** (with the thesis) | `confidence` casts model text to a float. A self-mutating runtime is the opposite of auditable execution. | E, executor |
| `semantic_match`, `fuzzy_cast`, `assert_semantic` | **PLANNED** / Unsupported in bytecode | `construct.go:96-157` | D |
| `release_authority` demo: "intent is not authority" | **PARTIAL** | Policy decisions are right, but the output JSON can be forged through `target` (S16). Approval comes from CLI strings. | D, executor |
| `action_executor` "failure atomicity" | **CONFLICTS** | Writes the file, then traps on `STORE_OPEN` with a filesystem-only grant (S17) | F |
| Production `-compile-bc` should flip to HFIR lowering now | **CONFLICTS** (with the standing constraints) | `lowered_hfir_prod_flip_criteria.md` defers it. Rejected here. | C |
| Pivot execution to Wasmtime | **CONFLICTS** (with "no wholesale redesign") | Reasonable as an outer isolation layer or a future target only (05) | E |

## Summary count (34 rows)

| Status | Rows |
| --- | --- |
| EXISTS | 6 (2 of them only after this PR) |
| PARTIAL | 6 |
| PROTOTYPE | 4 |
| PLANNED | 1 |
| ABSENT | 6 |
| CONFLICTS | 11 |

Most CONFLICTS rows are **documentation running ahead of the code**, or
limits that are declared but not enforced. They are not deep design
contradictions. That is good news: most can be fixed with wording, tests,
or small enforcement changes.
