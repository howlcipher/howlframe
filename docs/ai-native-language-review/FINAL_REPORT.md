# HowlFrame AI-native architecture and product direction: final report

- **Prepared for:** William Elias, via Rintaro Okabe (EM, HowlFutureWorks Engineering)
- **Date:** 2026-10-05
- **Baseline:** `origin/main` @ `d496d9721122681b0adbb5d27764f55a21c43a3d` (#103)
- **Branch:** `rintaro/ai-native-language-review`
- **Method:**
  - The executor inspected the code directly and ran probes.
  - Six independent agent reviewers covered roles A to F: 4 Codex and 2 AGY/Gemini (see 03).
  - Every agent claim was then reconciled against the code (06).

---

## TL;DR FOR WILLIAM

1. **The idea holds up, but it is narrower than the docs say.** HowlFrame
   works best as *a deny-by-default runner for small programs that an AI
   proposes*. The AI can only suggest. The runner decides what is allowed
   and keeps a record. It should not be an "AI-native language" or compete
   with Python or Go.
2. **There was a real hole, and it is fixed in this PR.** Three AI
   primitives (`confidence`, `neural_circuit`, `ephemeral_circuit`) could
   reach the network with *zero permissions*. One of them could create and
   delete models on the host. Three separate agent reviewers found this
   independently. The fix makes them require `network`. It has tests,
   changes no artifact format, and leaves the HFIR flip and #90 alone.
3. **"Bounded" is not true yet.** Memory, recursion depth, and wall-clock
   time are unlimited. About 100 instructions used 2 GB of RAM. Grants are
   five broad on/off switches. Close these before anyone runs untrusted
   programs.
4. **The demos need care.** Release Authority's JSON output can be forged:
   a proposal can make a parser read `ALLOW` when the VM said `DENY`.
   Action Executor is not atomic, although the docs say it is. These are
   small fixes, but they sit right at the heart of the pitch.
5. **Next, prove it beats the simple alternative.** Run one 5 to 7 day
   experiment. Have a model write HowlFrame programs and, separately, plain
   JSON action plans for 30 multi-step tasks, at equal permissions. If
   HowlFrame doesn't win by ≥15 points with zero unauthorized effects,
   stop growing the language and keep the runner, policy, and receipt
   parts.

---

## Answers to the 14 questions

### 1. What is HowlFrame today?
A working experimental toolchain of about 22.7k Go LOC (plus ~27.9k test
LOC):

- **Pipeline:** parser → checker → selective HFIR gate → AST bytecode
  compiler → checksummed `.hfbc` → stack VM.
- **Runner controls:** capability grants (`-allow-caps`) and an
  instruction budget, both owned by the runner.
- **Other targets:** Go, JS, and WAT backends.
- **Status of the AI-facing parts:**

| Part | Status |
| --- | --- |
| Strict JSON model transport, bounded repair, CAS manifests | **PROTOTYPE** (library only, no CLI) |
| HFIR-as-executable (`-compile-hfir-bc`) | Experimental only. #90 stays Partial. |
| "AI-native paradigms" (swarms, `achieve`, `optimize_*`, stochastic flow) | Mostly thin wrappers around a hard-coded local Ollama URL, or doc-only (06) |

### 2. What is it already becoming?
The last ~40 commits mostly do three things:

- make the bytecode VM the default output for every root;
- harden `SPAWN_AGENT`;
- keep experimental HFIR lowering in parity behind a fence.

In practice HowlFrame is becoming **a small embedded runtime for bounded
automation with an explicit authority vocabulary**. It is not becoming a
program-synthesis platform or a general language (01).

### 3. Is the thesis sound?
**Sound but unproven.**

- **Strongest argument for:** multi-step agent tasks need more than one
  tool call. Code that is constrained, runs under the runner's authority,
  and can be repaired locally (`replace_node`) sits in a real gap between
  JSON tool calls and a container sandbox.
- **Strongest argument against:** the security value lives in the host API
  and runner, not in a new language. JSON action plans with a trusted
  broker, Starlark, or CEL could deliver it with a smaller trusted base.
  No user has yet shown that programmability wins (02).

### 4. Who would use it?
An agent-platform team that wants a model to compose small multi-step
evidence-to-action workflows under fixed authority, for example release
gating, record normalization, or conditional approval requests.

- **Alternatives they would otherwise use:** fixed JSON action catalogs,
  Starlark with host functions, or a sandboxed Python.
- **Who it is not for:** general app developers, and teams whose only need
  is a policy decision. CEL and OPA already serve those teams.

### 5. What is the TCB, and is it trustworthy?
- **TCB:** parser, include/module resolution, transformations, checker,
  construct registry, HFIR gate, AST compiler, artifact decoder, VM, host
  effect handlers, and the Go runtime.
- **Is it trustworthy?** Not yet for hostile code.

| Gap | Ref |
| --- | --- |
| Zero-grant model calls | S1/S2, fixed here |
| `lazy_synthesize` runs unchecked model-generated code | S3 |
| No memory limit | S4 |
| No call-depth limit | S5 |
| No deadlines or byte caps | S6/S7 |
| Child grants are inherited | S9 |
| Artifact validation is structural only | S12 |
| Compile-time includes read arbitrary paths | S13 |
| Generated Go/JS leave most effects ungated | S11 |

Full list: 07.

### 6. Does `.howl` syntax matter?
Not for the thesis.

- **Keep `.howl`** for humans, fixtures, and existing apps. Don't redesign
  it.
- **Model contract:** the versioned HFIR JSON transport (stable node IDs,
  closed construct set, structured diagnostics, bounded repair
  transactions). It must never carry grants or budgets, and never expose
  opcodes. That contract exists as a prototype. Widen it only for jobs
  the experiment shows matter.

### 7. How does it compare with prior art?
- **Reinvented, mostly no better:**
  - the VM and instruction fuel (Wasmtime fuel/epochs, Lua hooks, Rhai
    operation limits, Starlark step limits);
  - coarse permission flags (Deno already scopes per path, host, and
    variable);
  - checksummed artifacts.
- **Genuinely differentiated (if it holds up):**
  1. a closed semantic contract with stable node identity and
     authority-preserving, hash-preconditioned local repair;
  2. effect-requirement inference before running;
  3. "intent is not authority" built into the runner;
  4. (planned) runner-written receipts.
- **CaMeL-style provenance tracking** is the most relevant research to
  borrow. See 05.

### 8. Which features conflict with the thesis?
- **Quarantine behind a profile, don't delete (artifact compatibility):**
  - `lazy_synthesize` (runtime self-modification);
  - `ephemeral_circuit` (creates and deletes host models);
  - `neural_circuit`, `confidence`, `achieve`, `llm_generate` inside
    trusted execution;
  - `exec` and `spawn` in model-authored code;
  - Go/JS targets as "governed" outputs.
- **Mark superseded:** `docs/extreme_ai_paradigms.md` (auto-mutating
  runtime, stochastic control).
- **Fix overclaiming wording:** `optimize_signature` (says it runs tests;
  it does not), the "failure atomicity" claim, and the stale "capabilities
  are advisory" line in the roadmap.

### 9. Top 5 risks, ranked
1. **False authority assurance.** Effect paths bypass grants. One case is
   fixed here; C3 now adds executable conformance checks against recurrence.
2. **No advantage over the simple alternative.** JSON plans plus a broker
   may be enough.
3. **Resource exhaustion.** Memory, recursion, wall-clock, and output are
   all unbounded.
4. **Semantic drift across backends.** Example: `and`/`or` short-circuiting
   differs and changes which effects run.
5. **Evidence inflation and solo-builder overload.** 3-task and 11-candidate
   samples, and many parallel fronts (backends, lowering, apps, paradigms).

### 10. What should authority become?
Steps, in order:

1. **Now:** make every effect path gated and tested by conformance (P0).
2. Keep the five coarse names as aliases, but add **resource-scoped
   grants**: `filesystem:read=/in`, `filesystem:write=/out`,
   `network=host:port`, `environment=VAR`, `process=<action id>`.
3. Attenuate grants for child agents.
4. Add budgets for memory, depth, deadline, and output/response bytes.
5. **Model calls become their own effect** (`model`), with provider and
   model, token and cost budget, and a disclosure policy. Responses are
   recorded for replay and treated as untrusted data.
6. **Approvals bind** the artifact hash, action, target, and expiry, and
   arrive over a trusted channel, never inside the proposal.

### 11. What does "verified" mean?
**Today** it means a set of named checks passed:

- the checker;
- the construct registry (Supported / CompileTimeOnly / Unsupported);
- an HFIR gate that blocks only `HFIR_INVALID_REF` and
  `HFIR_TARGET_INFEASIBLE`;
- an artifact envelope plus checksum and structural validation;
- for model candidates, strict transport decoding and graph checks.

What can and cannot be claimed:

| Kind | Examples |
| --- | --- |
| **Can be checked before running** | Schema and version; references; closed construct set; known types; required capabilities (over-approximated); bytecode stack/operand validity (not yet done) |
| **Must be enforced while running** | Actual resource targets, budgets, deadlines, approvals, state preconditions |
| **Cannot be claimed** | Business correctness, truthfulness of model output, prompt-injection immunity, termination of arbitrary programs, isolation from the host OS |

Replace the bare `Verified` boolean with a report: verifier X, version Y,
checks Z (P1.3).

### 12. Prioritized roadmap
See the table below and 08.

### 13. First falsifiable experiment
- **Comparison:** HowlFrame programs against JSON action plans run by a
  trusted Go broker. Optionally add Starlark as a third arm.
- **Tasks:** 30 held-out multi-step release-evidence tasks, plus 30
  prompt-injection variants. Same model, catalog, and grants in every arm.
- **PASS:** ≥15 pp higher success, zero unauthorized effects, and ≤1.5×
  token cost.
- **KILL:** ≤5 pp better, or any unauthorized effect. HowlFrame then
  pivots to a broker, policy, and receipt layer.
- **Cost:** about 5 to 7 builder-days and ≤$150 in model spend. Full
  design in 09. Four reviewers proposed essentially this comparison
  independently.

### 14. Next two weeks for William
**Do:**

| Days | Work |
| --- | --- |
| 1–2 | Merge this fix (#57). Add the effect-gate conformance test (C3). Fix the demo JSON forgery and the atomicity wording (C7). |
| 3–4 | Add the required-capabilities report (C1) and governed profile v0 (C2). Fix the stale or overclaiming docs. |
| 5–6 | Enforce call depth and a basic allocation/output cap (C4, minimal). Decide `and`/`or` semantics. |
| 7–10 | Build the JSON-broker baseline and harness. Run the experiment. |
| 11–14 | Write the continue / pivot / stop decision. Fix only the blockers the experiment found. |

**Don't:**

- flip production `-compile-bc` to HFIR, or mark #90 Done;
- retake the `4d74dbcf` tip-lock;
- add DOM APIs, new "AI-native" primitives, swarm features, or backends;
- redesign the syntax or change the artifact format;
- wire runtime synthesis further.

---

## Priority table

| Pri | Item | Size | Status in this PR |
| --- | --- | --- | --- |
| **P0** | Gate model-call opcodes and interpreter `lazy_synthesize` behind `network` | S | **DONE** (commit 1, bugs.md #57) |
| P0 | Effect-gate conformance test (no effectful opcode with an empty capability) | S | Done in this change (C3: source scan + empty-grant coverage) |
| P0 | Governed profile v0 (`--profile governed` construct allow-list) | S/M | Planned (C2) |
| P0 | Required-capabilities report for artifacts | S | Planned (C1) |
| P0 | Honest wording: what "verified", "bounded", and "atomic" mean; mark extreme paradigms superseded; fix the stale roadmap | S | Planned (docs) |
| P0 | Demo-app output integrity (`encode_json`) and pre-flight or "ordered, not atomic" | S | Done in this change (C7: JSON integrity + pre-flight, ordered effects) |
| **P1** | Memory/allocation, call-depth, deadline, and byte limits | M/L | Planned (C4) |
| P1 | Execution receipt v0, written by the runner | M | Planned (C5) |
| P1 | Verification report instead of the bare boolean | S/M | Planned |
| P1 | `and`/`or` semantics decision plus a differential test | S/M | Planned (C6) |
| P1 | **Run the first falsifiable experiment** | M | Designed (09) |
| P1 | Regenerate codegen drift (`SPAWN_AGENT` row, orchestrator schema) | S | Noted (pre-existing) |
| **P2** | Resource-scoped grants, child attenuation, a `model` effect | L | After the experiment says go |
| P2 | Full artifact validation, rooted includes | M/L | After the experiment |
| **P3** | CLI for HFIR propose/repair; widen the transport for proven jobs; provenance tags; signed receipts | M/L | Only with demand |
| **P4** | Production HFIR flip, more backends, DOM growth, new AI primitives, general-language competition | L | Later or never |

---

## What changed in this PR

1. **Commit 1, code** (`vm: require network for model-call opcodes and
   interpreter lazy_synthesize`):
   - **Changes:** `OpConfidence`, `OpNeuralCircuit`, and
     `OpEphemeralCircuit` declare `capability.Network`. `ForConstruct`
     maps those three constructs to `network`. The interpreter checks
     `network` before the `lazy_synthesize` request.
   - **Tests:** a recording-listener test (red on old code, green on new)
     plus registry tests.
   - **Bookkeeping:** bugs.md #57, change_log, journal, the 3 opcode rows
     in `bytecode_reference.md`, and a regenerated `construct_coverage.md`.
   - **Compatibility:** no artifact format change. Programs using these
     primitives under `-run-bc` must now pass `-allow-caps network`. That
     is the intended fail-closed behavior.
2. **Commit 2, docs:** this directory.

## Test outcomes
- **Baseline at `d496d97`:** `go test ./...` passed 38/38 packages (49.6 s).
- **After the fix:**
  - `gofmt` and `go vet` are clean, and the build succeeds.
  - `go test ./...` passes 38/38 packages.
  - `scripts/test_seo.py` and the `benchmarks/v2` harness unittest pass.
- **Agent-sandbox failures:** the Codex sandboxes' `internal/vm` failures
  were caused by the sandbox forbidding socket listeners. They are not
  repo failures.

## Index
| File | Contents |
| --- | --- |
| 00 | Current state |
| 01 | What HowlFrame is becoming |
| 02 | Product thesis |
| 03 | Agent polling method |
| 04 | Agent responses and grading |
| 05 | Competitive and prior art |
| 06 | Reconciliation matrix |
| 07 | Security and authority findings |
| 08 | Roadmap |
| 09 | Experiment |
| 10 | Risks and non-goals |
| 11 | Implementation candidates |
| `MISSION_JOURNAL.md` | Mission journal |
| `raw/` | Raw agent outputs and the common prompt |
