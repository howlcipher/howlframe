# 04: Agent responses: summary and evidence grading

The raw outputs are in `raw/out-{A..F}.md`. The grades judge evidence
quality, checked by the executor against the code at `d496d97`. They do
not measure agreement with the executor.

| Grade | Meaning |
| --- | --- |
| **A** | Claims are checked or reproduced. Citations resolve. No material false claims. |
| **B** | Mostly correct, with minor citation drift or unverified extrapolation. |
| **C** | Useful ideas, but material false "VERIFIED" claims or misaligned citations. |
| **D** | Not reliable as evidence. |

## Scorecard

| Role | Engine | Grade | Headline verdict | Ran code? |
| --- | --- | --- | --- | --- |
| A: architect | Codex | **A** | Credible for small governed automation; not a hostile-code boundary yet. Prove one workflow against an alternative. | Yes |
| B: red team | Codex | **A** | The authority boundary has concrete holes. Fix effect paths, then test one customer job. | Yes, with a listener |
| C: AI/LLM | AGY Gemini 3.1 Pro | **C** | "Compelling target". Double down on HFIR/JSON and finish the HFIR path. | No (no shell) |
| D: skeptic | Codex | **A** | Narrow sharply. Fund only a time-boxed experiment against JSON actions plus policy. | Yes |
| E: prior art | AGY Gemini 3.8 Flash | **C** (output truncated) | Thesis sound, implementation misaligned. Drop the transpilers, pivot execution to Wasmtime. | No (no shell) |
| F: DX/dogfood | Codex | **A** | Promising as a small governed-program runner. Close authority gaps, then test an outside developer workflow. | Yes |

## Where they converged (independently)

1. **Model-call opcodes bypass grants.** B, D, and F each found it
   independently by running an empty-grant probe. A noted that model
   primitives and `lazy_synthesize` bypass the checker. The executor had
   reproduced the hole before reading any output. **Fixed in this PR**
   (bugs.md #57).
2. **`MaxMemoryBytes` is declared but never enforced.** A, B, D, E, and F
   all report this. The executor reproduced about 2 GB RSS under default
   limits (S4).
3. **The production HFIR gate is selective, not a canonical executable
   artifact.** A, D, and F say so with citations (`howlframe.go:466`). C
   makes the same point as "verify what you execute".
4. **The model-facing contract should be semantic JSON (HFIR transport),
   not `.howl` text or bytecode.** Every reviewer agrees.
5. **The strongest rival is JSON action schemas plus trusted handlers and
   a policy engine.** A, B, D, and F each designed their first experiment
   against that baseline (F used Starlark plus fixed host actions).
6. **The authority vocabulary is too coarse.** All reviewers propose
   moving from five effect classes to resource-scoped handles or bindings.
   Several add deadlines and byte limits, and want model calls as an
   explicit effect.
7. **Respect the hard nos.** A, B, D, and F explicitly say not to flip
   production lowering or mark #90 Done. C is the exception (below).

## Per-agent notes

### A: compiler and runtime architect (Codex). Grade A
- **New and reproduced:** `and`/`or` divergence. The VM and interpreter
  evaluate both operands, so the right-hand `env` call triggers
  `CAPABILITY_DENIED`. Generated Go short-circuits and prints `false`.
  The executor reproduced it (S10).
- **Other findings:** `MaxCallDepth` is not checked on `CALL`, which the
  executor reproduced as a Go fatal stack overflow (S5). `lazy_synthesize`
  bypasses the checker and HFIR. The manifest does not bind the compiler
  version or the grant.
- **Experiment:** 30 release-preparation tasks, HowlFrame against a Go
  JSON-plan executor.

### B: security red team (Codex). Grade A
- Independently proved the zero-grant model-call hole. This was the most
  important convergence in the review.
- A `process`-only grant wrote files through `/bin/sh -c`. That is
  expected for an unscoped `process` grant, but it shows that `process`
  amounts to every other capability.
- `sleep` is not bounded by the instruction budget. `SPAWN_AGENT` children
  inherit the full grant. Generated Go and JS leave most effects ungated
  and write `crash.json`. Compile-time `include` reads arbitrary paths.
  Artifact validation is structural only.
- **Experiment:** generated programs against fixed JSON proposals for
  release-evidence workflows.

### C: AI/LLM researcher (AGY Gemini 3.1 Pro). Grade C
False or unsupported claims marked VERIFIED:

| C's claim | Reality at `d496d97` |
| --- | --- |
| Capabilities `NETWORK_READ` and `FS_WRITE` (`capability.go:20`) | **CONFLICTS.** The names are `network`, `filesystem`, `process`, `environment`, `database`. |
| "Language benchmarks prove token-density advantages over Python and Go" | **CONFLICTS.** `benchmarks/language_write_cost/results.csv` (2026-07-23) shows HowlFrame used fewer tokens only on task A (84 vs Python 164). It lost task B (88 vs 61) and task C (123 vs 37). Three tasks prove nothing either way. |
| "Generalized multi-node repair is entirely aspirational" | **CONFLICTS.** `hfir-semantic-repair/v2` exists: up to 5 operations in a region of at most 16 nodes, same-kind replacement only (`model_adapter.go:27-39`). |
| `contracts/howl` is the HFIR schema | **CONFLICTS.** It holds ecosystem contract schemas (SOURCE/HowlBoard), not HFIR. |
| Many `path:line` citations (`vm.go:45`, `vm.go:112`) | They do not point at the claimed code. |

- **Recommendation that breaks the hard nos:** P0 "complete the HFIR
  flip". This review rejects it.
- **What C gets right:** production executes AST-compiled bytecode, not
  the verified graph, so verification and execution are different
  objects. The proposed generate, verify, repair loop is reasonable.
- **Experiment:** HFIR against sandboxed Python, within 5 repair loops.
  It is reasonable but uses the weaker baseline. The JSON-actions
  baseline is the real competitor.

### D: skeptic (Codex). Grade A
- **Strongest case against:** the parts are each worthwhile, but nothing
  yet shows they need one new language and runtime. The synthesis
  evidence is 11 stored candidates averaging 8.64 nodes from one
  restricted author (`hfir_model_adapter_status.md:35`).
- **Found the `release_authority` JSON defect:** a quoted `target` breaks
  the output. The executor took it further, to forging `"decision":
  "ALLOW"` when the VM decided `DENY` (S16).
- **Would still fund:** bounded evidence transformation and repair, with
  immutable trusted policy and a fixed external action broker.
- **Experiment:** 30 held-out tasks against JSON actions plus CEL or
  trusted code, with pre-set pass and kill thresholds, 7 to 10 builder
  days, and a $300 to $1,000 model cap.

### E: prior-art analyst (AGY Gemini 3.8 Flash). Grade C, output truncated
- The header is duplicated and the output stops in question 8, so the
  role-specific comparison table is missing.
- **Citations:** they are context-pack offsets mislabeled as file lines
  (for example `howlframe.go:1362` and `vm.go:6027`, when `howlframe.go`
  has 941 lines).
- **Content claims that are correct:**
  - `optimize_block` just runs its body.
  - `achieve` and `confidence` call `localhost:11434` with `llama3`.
  - `SPAWN_AGENT` is cooperative and in-process.
  - The status document's 3/10 `.howl` first attempts against 10/10 HFIR
    first-pass results are real, but the sample is tiny.
- **Pivot advice:** E advises pivoting execution to Wasmtime. That is
  defensible for general isolation but runs against "do not redesign
  wholesale". 05 covers it: use WASI as an outer boundary or a future
  target, not as a replacement for the semantic layer.
- **What it missed:** E did not notice that the model calls bypass grants.
  It called them only "unauthenticated".

### F: dogfood and developer experience (Codex). Grade A
- The README quickstart and the action executor worked end to end.
- **Partial effects, reproduced:** with only a `filesystem` grant,
  `stage_artifact` prints `ALLOW`, writes the file, and then traps on
  `STORE_OPEN`. Phase-5 dogfooding docs claim "failure atomicity" (S17).
- **Smaller findings:**
  - `check` accepted `(wasm_app (print ...))` but the Wasm build rejected
    it, so "check passed" is not target-aware.
  - Absolute-path `build` writes the artifact into the current directory.
  - Some app denial tests accept any nonzero exit.
- **Workflow map:** propose (internal API only), check (yes), inspect
  requirements (no CLI), grant (yes), run (yes), audit (the evidence
  object is discarded by the CLI).
- **Experiment:** three outside developers, Starlark plus fixed host
  actions as the baseline, 5 builder days, $100 model cap.

## What the disagreement tells us

- **Engine split.** The four Codex reviewers that ran code all landed on
  "real but narrow; prove it against JSON actions". The two Gemini
  reviewers, which could not run code, were more enthusiastic (C) or more
  radical (E, the Wasmtime pivot). Both made more factual errors. The
  executor weights the reviewers that ran code more heavily.
- **Where the risk lies.** No reviewer argued for general-purpose language
  growth. Every reviewer put the risk in authority holes and missing
  evidence of demand, not in syntax or performance.
