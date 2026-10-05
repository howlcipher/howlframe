# 00: Current state (verified at `main` = `d496d972`)

This section describes what the code actually does at
`d496d9721122681b0adbb5d27764f55a21c43a3d` (#103), the `main` tip on
2026-10-05. Every claim was checked against code, tests, or a command run in
a clean clone. Source locations are path:line at that commit.

## Size and pace

| Measure | Value |
| --- | --- |
| Commits | 434, the first on 2026-07-23 (about 10 weeks) |
| Non-test Go | about 22.7k lines (`internal/` plus CLI, apps, tools, examples) |
| Test Go | about 27.9k lines. There are more test lines than production lines. |
| Bytecode opcodes | 69 (`internal/bytecode/opcode.go`) |
| Construct registry | 87 `Supported`, 9 `CompileTimeOnly`, 11 `Unsupported` (`internal/construct/construct.go`) |
| Baseline `go test ./...` | all 38 test packages ok, 49.6 s wall clock (log in mission journal) |

## The pipeline as it runs today

```text
.howl source
  -> lexer/parser -> includes/modules -> patch/with_context AST transforms
  -> checker.Check (types, arity, layout; JSON diagnostics)
  -> runHFIRGate: hfir.LowerAST + Verifier.Verify + VerifyConstructs
       (blocking codes: HFIR_INVALID_REF, HFIR_TARGET_INFEASIBLE only; howlframe.go:466)
  -> bytecode.CompileToBytecode(checked AST)      <- PRODUCTION (-compile-bc, build)
     hfir.LowerToBytecode(graph)                  <- EXPERIMENTAL (-compile-hfir-bc)
  -> HFBC artifact: magic + version 2 + SHA-256 + gob payload (internal/bytecode/artifact.go)
  -> -run-bc / howlframe run: artifact validation, then BCVM
       per-instruction: instruction budget check, then registry capability check
       (internal/vm/vm.go:1940-1948)
```

Other paths that are still in the binary:

- `-run`: a tree-walking AST interpreter for a bounded `cli_app` subset. It
  has its own capability checks (`internal/vm/vm.go:94`, `:305`).
- `build --target=go|js|wasm`: generated Go (the broadest backend),
  JavaScript for `web_app`, and legacy WAT. `-compile-wasm` is a typed SSA to
  WAT path for scalar programs.
- `-mask-plan` and `-optimization-plan`: deterministic JSON metadata. Neither
  calls a model.

## What "governed execution" consists of today

| Mechanism | Status | Evidence |
| --- | --- | --- |
| Runner-owned capability grants (`-allow-caps`, `--allow-caps`) | EXISTS. Five coarse classes: `network`, `filesystem`, `process`, `environment`, `database`. | `internal/capability/capability.go:8-15`. The VM checks before dispatch (`vm.go:1945`). |
| Deny-by-default | EXISTS for `-run-bc` and `-run`. Until this PR, three model opcodes were an exception (bugs.md #57). | `vm.go:1756` |
| Instruction budget (default 100,000; the runner may raise it, the program may not) | EXISTS | `vm.go:1940`, README "Execution Policy" |
| Wall-clock, memory, output, and response-size limits | ABSENT. `MaxMemoryBytes` and `MaxCallDepth` are declared (`internal/vm/error.go:41-42`) but only spawn depth is enforced (`vm.go:3013`). | See 07 |
| Construct registry fail-closed at compile time | EXISTS. An unsupported construct gives `HFIR_TARGET_INFEASIBLE` before any artifact is written. | `internal/construct`, `howlframe.go:502` |
| HFIR verifier | PARTIAL. It runs on every production build but blocks only on dangling references and target infeasibility. Capability effects are inferred but never block. | `howlframe.go:466-520` |
| Artifact integrity | EXISTS. Magic, version, SHA-256, a 10 MiB cap, trailing-data rejection, opcode and jump validation. No signature, no stack or type validation. | `internal/bytecode/artifact.go` |
| Deterministic artifacts | EXISTS (v2 sorted function table) | `artifact_determinism_test.go` |
| Model-facing contract | PROTOTYPE, library only. Strict JSON candidate transport (128 nodes, 64 KiB), verifier, direct lowering, bounded `replace_node` repair with hash preconditions. It is not wired to the CLI. | `internal/hfir/model_adapter.go`, `docs/hfir_model_adapter_status.md` |
| Build manifest and CAS | PROTOTYPE, library only. `hfir-manifest/v1` has graph, node, and module hashes, dependency maps, evidence hash, and artifact hash. It has no compiler version, policy, or grant. Disk and memory CAS. Incremental compiler. | `internal/hfir/manifest.go`, `storage.go`, `incremental.go` |
| Execution receipt (what ran, with which grant, what effects happened) | ABSENT | none |
| Static "what does this artifact need" report | ABSENT from the CLI. The data exists: per-opcode capabilities plus store URIs. | `opcode.go` |
| Signed or attested artifacts | ABSENT | none |

## AI-oriented primitives at runtime

These are not part of the governed-execution story. They are model calls
inside the program:

| Construct | Bytecode VM | Gate before this PR | After this PR |
| --- | --- | --- | --- |
| `llm_generate`, `achieve` | HTTP to `127.0.0.1:11434` | `network` | `network` |
| `confidence` | HTTP | **none** | `network` |
| `neural_circuit` | HTTP | **none** | `network` |
| `ephemeral_circuit` | creates, queries, and deletes a model on the host | **none** | `network` |
| `lazy_synthesize` | asks the model for source, parses and compiles it with **no checker, no HFIR gate, no construct registry**, then runs it with the program's full grant (`vm.go:2638-2690`) | `network` (VM), **none** (interpreter) | `network` on both |
| `semantic_match`, `fuzzy_cast`, `assert_semantic` | Unsupported in bytecode. Go backend only. | n/a | n/a |
| `spawn_agent` / `task` | In-process, synchronous child VM. Inherits the parent grant, limits, and stores (`vm.go:3030`). | `process` | `process` |

## Dogfood evidence

- Apps in the repo: `release_authority`, `action_executor`,
  `candidate_evaluator`, `status_api`, `task_api`, `kv_cli`, `todo_cli`,
  `log_analyzer`, and `examples/repo_analyst`. Each has Go tests that compile
  and run it.
- External: HowlBoard, a full-stack reference app.
- The last 15 commits (#89 to #103) all move toward one thing: every named
  root writes `.hfbc` and stops, with no Go/JS fallback on the default path.
  They also run TASK and SPAWN_AGENT bodies inside the VM, and harden failure
  isolation.

## Evidence about AI authoring

- `docs/language_write_cost_benchmark.md`: 3 tasks × 6 languages, one model
  run per cell, 18 programs. HowlFrame came out first on total write time and
  second-to-third on tokens. This is a small sample, and the author measured
  their own language.
- `docs/hfir_model_adapter_status.md`: 11 stored candidates and one repair.
  Schema and verifier pass was 11/11, and first-pass behavior was 10/10 on
  JSON graphs. The same tasks as `.howl` were correct 3/10 times. That is
  suggestive but anecdotal: one author, tiny graphs (about 8.6 nodes).
- `benchmarks/v2`: the harness exists, but no model results are checked in.

## Standing decisions in force (unchanged by this review)

- Production `-compile-bc` = AST bytecode. HFIR = AOT verifier gate
  (HOWL-CANON-010, `docs/architecture_roadmap.md:59-68`).
- #90 Partial. The Assurance tip-lock `4d74dbcf` is Overall PASS / Promote
  DEFERRED and does not authorize a flip.
- Generated Go/JS mediate only `env`, `exec`, `read_file`, `write_file`,
  `mkdir`, and `fetch` (README "Capability Security").
