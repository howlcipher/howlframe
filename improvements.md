# 🚀 Improvement Backlog

This document is the authoritative, ranked backlog for the Zero transpiler project. It mirrors the format used in the main AI Knowledge Library.

## Working Protocol

This protocol applies to every worked task in the Zero project:

1. **Open a task journal.** Record your steps in a `YYYY-MM-DD_task_name.md` file if the task is complex. This project's actual convention is `docs/journals/` (with completed journals moved to `docs/journals/archive/`) — the generic cross-project protocol points at `documentation/task_journals/`, which exists in this repo but has stayed empty; use `docs/journals/` so history stays with the ones already there. Trivial, fully-specified fixes (a one-file copy, a one-line diff with an exact fix sketch) don't need a journal at all — apply and verify directly.
2. **Re-evaluate the model.** Pick the least expensive available model (e.g., local Ollama, Claude, or Gemini) that can do the job well for the Zero transpiler.
3. **Route the crafted skills.** `.agents/skills/zero_transpiler/SKILL.md` exists in the AI Knowledge Library as of 2026-07-23 — consult it first for syntax, the AST node reference, and known-bug workarounds before falling back to the general-purpose `software_development` and `automation` skills.
4. **Scan for helpful free tools.** Ensure you aren't rebuilding something already available.
5. **Finish the loop.** Every code change ships with relevant tests. Run Go builds (`go build`) and Python script validations before committing.
6. **Resuming after a delegate session limit.** If a task journal exists and the working tree already has uncommitted changes matching that journal's brief, don't assume the delegate failed or start over — a delegate (e.g. `agy`) can hit a session/quota limit *after* finishing real edits. Build, vet, and test the uncommitted diff first; if it's complete and correct, finish and commit it as-is rather than re-delegating from scratch. Confirmed 2026-07-23 when improvement #16 (Native Unit Test Blocks) was found fully implemented and working in the tree after its agy delegate hit a session limit.
7. **Never run bare `go build .` in the repo root to verify a generated `server.go`.** The repo directory is itself named `zero`, `zero.go` carries `//go:build ignore` (so it's excluded from a plain `go build .`), and an unnamed `go build .` output binary defaults to the *directory* name — `zero` — silently overwriting the tracked transpiler binary with a build of whatever `server.go`/`server_test.go` happen to be sitting in the working tree at the time. Hit live on 2026-07-23 during a doc-example verification pass; caught immediately via `git status` showing `zero` modified with no corresponding `zero.go` change, and fixed by rebuilding with `go build -o zero zero.go`. Always verify a generated `server.go` with an explicit `-o` to a scratch path (e.g. `go build -o /tmp/servercheck .`), and only ever run `go build -o zero zero.go` when you actually intend to rebuild the transpiler binary itself.
8. **Autonomous `agy --mode accept-edits` calls can be blocked by the Claude Code permission classifier.** In auto-mode sessions, invoking `agy -p "..." --mode accept-edits ...` from Bash can be denied outright by the session's auto-mode classifier (observed 2026-07-23), even though the same command works when the user is prompted interactively. When this happens, do not retry the identical call — either fall back to `--mode manual`/a mode that surfaces edits for review, ask the user to approve the Bash permission rule, or, for genuinely trivial and fully-specified diffs (e.g. a one-clause fix with an exact fix sketch already in the backlog), apply the edit directly with Edit/Write instead of delegating.
9. **Benchmark Regression Gate.** Any transpiler change touching `defun`/`type_hint`, `read_file`, `str_split`, or the `test` block must re-run the benchmark harness in `benchmarks/language_write_cost/`, update `results.csv` and `docs/language_write_cost_benchmark.md`'s tables, and note the delta. Exception: a change that provably produces byte-identical transpiler output for every existing fixture (e.g. an internal refactor with no language-surface change) doesn't need a re-run — nothing about what an AI would write in Zero source changed. Document the byte-diff proof instead.
10. **Delegate availability varies session to session — check live, don't assume.** Confirmed 2026-07-24: agy can be fully unavailable for an entire session (user's weekly Antigravity quota exhausted) with local Ollama also not installed (`curl localhost:11434/api/tags` connection-refused, `ollama` not on `$PATH`). When both rungs of the delegation ladder are down, the fallback is this session's own model doing the work directly (same as groom_backlogs's rung-4 fallback) — don't block a high-priority item just because delegation isn't available; re-scope it if its full effort is too large/risky to do solo in one sitting (see item #53's Phase 1/2 split for a worked example), but don't skip it outright when it's the top of the ranked backlog.
11. **Verify build artifacts never land in the repo root before committing.** A `servercheck` binary (9.3 MB) and a `test.db` SQLite file were found tracked in git as of 2026-07-24 (introduced in the improvement #18 commit) — a violation of point 7's guidance to build scratch verification binaries to `/tmp`, not the repo root. Removed from tracking and added to `.gitignore` (`servercheck`, `*.db`) in the 2026-07-24 groom pass. Run `git status` before committing and look for anything binary/generated that doesn't belong.
12. **Check the listed model against its owning tool.** The `OpenAI model` column maps to Codex/OpenAI, not Antigravity; verify those slugs through Codex state or invoke Codex with `codex exec -m <model>`, and do not treat `agy models` as evidence that an OpenAI/Codex model is unavailable. `agy models` only covers Antigravity delegation models such as Gemini and whatever extra providers Antigravity exposes in that session.

## Ranked Backlog (best ROI first)

Pending rows are ranked by a diminishing-returns score:

**Score = (Value × Decay) ÷ Effort**
- **Value (1–8):** pain or risk removed if the item ships.
- **Decay:** geometric halving per already-shipped item in the same theme (1.0 → 0.5 → 0.25 …).
- **Effort (1–8):** roughly log-scale; 1 = minutes, 8 = weeks.

| # | Improvement | Status | Score (V×D÷E) | Claude model | Gemini model | OpenAI model | ROI rationale |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | [Add Routing Support](#1-add-routing-support) | Done | — | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Highest value to allow building web apps with multiple endpoints instead of just the root path. |
| 3 | [Extend Python Orchestrator Grammar](#3-extend-python-orchestrator-grammar) | Done | — | Haiku 3 | Gemini 1.5 Flash | gpt-5.6-luna | Must update the grammar in `orchestrator.py` immediately after adding new Go AST features so the LLM can use them. |
| 2 | [Add Conditionals and Variables](#2-add-conditionals-and-variables) | Done | — | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Necessary for basic logic flow in handlers (checking methods, parsing headers). |
| 4 | [Add Database Connections (SQL)](#4-add-database-connections-sql) | Done | — | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Crucial for dynamic data and actual web service capabilities. |
| 5 | [Add JSON Request/Response Handling](#5-add-json-requestresponse-handling) | Done | — | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Needed to build standard REST APIs. Decay 0.125 because three Go AST features shipped. |
| 6 | [Add Function Definitions (defun)](#6-add-function-definitions-defun) | Done | — | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Critical for code modularity (DRY principle). |
| 7 | [Add Structs and Type Definitions](#7-add-structs-and-type-definitions) | Done | — | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Necessary for strict Input Validation schemas, adhering to software_development skill guidelines, and mapping SQL/JSON to Go. |
| 8 | [Add Iteration and Data Structures](#8-add-iteration-and-data-structures) | Done | — | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Essential for handling arrays of SQL results (list, map, for). |
| 9 | [Add Environment Variables Access](#9-add-environment-variables-access) | Done | — | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Follows 'Secure by Default' guidelines to prevent hardcoding database credentials or secrets in S-expressions. Decay 0.125. |
| 10 | [Add External Module Imports](#10-add-external-module-imports) | Done | — | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Allows importing third-party Go packages, unlocking the entire Go ecosystem. Decay 0.125. |
| 11 | [Add Concurrency (spawn)](#11-add-concurrency-spawn) | Done | — | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Allows AI to effortlessly run background jobs without blocking HTTP responses. |
| 12 | [Add Error Handling (try/catch)](#12-add-error-handling-trycatch) | Done | — | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Crucial for safe execution. Maps to Go's `if err != nil` idiom. |
| 13 | [Add File Inclusions (include)](#13-add-file-inclusions-include) | Done | 2.33 (7×1.0÷3) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Prevents massive monolithic `.zero` files by allowing modular codebases. |
| 14 | [Add Basic Math and Logic Operators](#14-add-basic-math-and-logic-operators) | Done | — | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Necessary for computing values natively in Zero instead of relying entirely on DB logic. |
| 15 | [Add Middleware Support](#15-add-middleware-support) | Done | 0.41 (5×0.25÷3) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Required for adding authentication and request logging across routes. |
| 42 | [Clean up file structure](#42-clean-up-file-structure) | Done (2026-07-23) | 4.00 (4×1.0÷1) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | The root directory is cluttered with `.zero` test files and examples. Needs organized folders. |
| 35 | [Add LLM-powered Type Coercion (fuzzy_cast)](#35-add-llm-powered-type-coercion-fuzzy_cast) | Done (2026-07-23) | 2.00 (8×1.0÷4) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Universal parser using LLM structured outputs to map messy, unstructured text to strict structs. |
| 36 | [Add Intent-based Validation (assert_semantic)](#36-add-intent-based-validation-assert_semantic) | Done (2026-07-23) | 2.00 (6×1.0÷3) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Enforces complex, qualitative natural language boundaries effortlessly using zero-shot prompts. |
| 44 | [Add Cross-Language "AI Write Cost" Benchmark](#44-add-cross-language-ai-write-cost-benchmark) | Done (2026-07-23) | 1.20 (6×1.0÷5) | Sonnet 5 | — | gpt-5.6-sol | Validates Zero's core hallucination-reduction pitch with measured evidence instead of assertion; published to README/docs for adoption/marketing. |
| 47 | [Document undocumented shipped primitives](#47-document-undocumented-shipped-primitives) | Done (2026-07-23) | 1.50 (6×1.0÷4) | Sonnet 5 | — | gpt-5.6-sol | Roughly a dozen already-shipped primitives (`db_connect`/`sql_query`, `import`, `middleware`, `include`, `append`/`map_set`/`map_delete`, `str_split`/`str_join`/`regex_match`, `env`, `spawn`, `fetch`, `match`, `rate_limit`/`retry`, `struct`) have zero mention or working example in README.md or docs/index.html — effectively invisible to both the orchestrator's target AI and human readers. |
| 20 | [Auto-Tracing (`trace`)](#20-auto-tracing-trace) | Done (2026-07-23) | 1.5 (3×1.0÷2) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Merged into the main table 2026-07-23 groom pass — was previously only tracked in the legacy V2 table below, invisible to a scan of this table alone. AI debugs by spamming `print`; a `(trace var)` macro auto-injects line numbers and variable names into `fmt.Println`. |
| 46 | [Close the Benchmark-Found Gaps and Make It a Standing Metric](#46-close-the-benchmark-found-gaps-and-make-it-a-standing-metric) | Done | 1.17 (7×0.5÷3) | Sonnet 5 | — | gpt-5.6-sol | Uses improvement #44's measured results as a design input instead of a one-off marketing artifact: fixes the concrete `type_hint` token-overhead finding from Task C, and formalizes re-running the benchmark as a regression gate for changes touching `defun`/`type_hint`, `read_file`, `str_split`, or `test`. |
| 45 | [Add Zero-to-JavaScript Compilation Target](#45-add-zero-to-javascript-compilation-target) | Done | 1.14 (8×1.0÷7) | Sonnet 5 | — | gpt-5.6-sol | Second codegen backend lets the same AI-facing S-expression grammar target the browser, extending Zero's hallucination-reduction pitch from backend-only to full-stack. Scoped to JS logic only — HTML/CSS stay native (see 2026-07-23 conversation). |
| 18 | [Declarative Schema Migrations](#18-declarative-schema-migrations) | Done (2026-07-23) | 1.0 (5×1.0÷5) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Merged into the main table 2026-07-23 groom pass — was previously only tracked in the legacy V2 table below, invisible to a scan of this table alone. `(schema "users" (column "id" "int"))` would let the transpiler auto-generate `CREATE TABLE IF NOT EXISTS`, building on the already-shipped `db_connect`/`sql_query` (#4) and `struct` (#7) primitives. |
| 48 | [Add CLI flag for output directory](#48-add-cli-flag-for-output-directory) | Done | 1.0 (2×1.0÷2) | Sonnet 5 | Gemini 1.5 Pro | gpt-5.6-sol | The transpiler always outputs `server.go` and `server_test.go` to the current working directory. Adding an output directory flag (e.g. `-o`) would allow keeping the workspace clean. |
| 43 | [Support for Go Generics](#43-support-for-go-generics) | Done (2026-07-23) | 0.8 (4×1.0÷5) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Merged into the main table 2026-07-23 groom pass — was previously only tracked in the legacy V2 table below, invisible to a scan of this table alone. `(type_param T)` syntax in `defun` would enable generating generic Go functions. |
| 53 | [Decouple AST from Go Codegen (IR Abstraction)](#53-decouple-ast-from-go-codegen-ir-abstraction) | Done (2026-07-24) — Phase 1 | 1.33 (8×1.0÷6) | Sonnet 5 | — | gpt-5.6-sol | Requisite for pure binary generation. High effort refactor. |
| 49 | [Direct Neural Bytecode Synthesis](#49-direct-neural-bytecode-synthesis) | Done (2026-07-29) | 1.00 (8×1.0÷8) | Sonnet 5 | — | gpt-5.6-sol | Monumental shift; massive effort but maximum value. |
| 61 | [Direct Binary Bytecode Serialization](#61-direct-binary-bytecode-serialization) | Done (2026-07-29) | 1.0 (8×1.0÷8) | Sonnet 5 | — | gpt-5.6-sol | Replaces the legacy human-readable JSON bytecode with a fully binary Go gob encoding. |
| 60 | [Add Collection Read Access (map_get/list_get)](#60-add-collection-read-access-map_getlist_get) | Done (2026-07-24) | 1.75 (7×0.5÷2) | Sonnet 5 | — | gpt-5.6-sol | Found while building improvement #49 Phase 1's regression fixture: `dict`/`list` have `map_set`/`map_delete`/`append` (write) but no way to read a value back by key/index at all — same theme as #31 Mutable Collections, decay 0.5. |
| 59 | [Auto-Patching Loop](#59-auto-patching-loop) | Done (2026-07-27) | 0.66 (8×0.5÷6) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Closes the loop on #58. High effort integration. Decay 0.5 from 1 shipped self-healing item (#58). |
| 54 | [WebAssembly (Wasm) Backend Prototype](#54-webassembly-wasm-backend-prototype) | Done (2026-07-27) | 0.50 (7×0.5÷7) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Delivered a bounded third code generator for portable numeric/control-flow WAT; decay 0.5 from the prior JS backend (#45). |
| 41 | [Add Stochastic Control Flow](#41-add-stochastic-control-flow) | Done (2026-07-29) | 0.29 (2×1.0÷7) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Introduces fuzzy logic natively; deferred as non-essential for initial MVP. |
| 38 | [Add Swarm Primitives](#38-add-swarm-primitives) | Done | 0.25 (2×1.0÷8) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Extremely advanced futurist concept; deferred to maintain MVP scope. |
| 39 | [Add Teleological Execution](#39-add-teleological-execution) | Done (2026-07-29) | 0.25 (2×1.0÷8) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Radical paradigm shift, non-critical enhancement deferred from MVP. |
| 50 | [Agentic Observability Layer](#50-agentic-observability-layer) | Done (2026-07-29) | 0.25 (8×0.25÷8) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Architectural shift; high effort. Decay 0.25 from 2 shipped observability items (#55, #56). |
| 52 | [Automated Counterfactual Debugging](#52-automated-counterfactual-debugging) | Done (2026-07-29) | 0.25 (8×0.25÷8) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Self-healing reasoning layer. Decay 0.25 from 2 shipped self-healing items (#58, #59). |
| 34 | [Add Semantic Routing (semantic_match)](#34-add-semantic-routing-semantic_match) | Done (2026-07-29) | 0.175 (7×0.125÷5) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Natively understands intent, replacing brittle traditional conditional routing and regexes. Re-scored 2026-07-23: same "LLM-backed runtime primitive" theme as shipped #26/#35/#36 (3 prior ships → decay 0.125, was uncounted at 1.0). |
| 57 | [`(neural_circuit)` Runtime Primitive](#57-neural_circuit-runtime-primitive) | Done | 0.15 (6×0.125÷5) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | LLM-backed runtime primitive (3 prior ships → decay 0.125). |
| 51 | [Ephemeral Neural Circuits](#51-ephemeral-neural-circuits) | Done (2026-07-29) | 0.146 (7×0.125÷6) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | LLM-backed runtime primitive (3 prior ships → decay 0.125). |
| 40 | [Add Auto-Mutating Runtime](#40-add-auto-mutating-runtime) | Done (2026-07-30) | 0.125 (1×1.0÷8) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Highly experimental runtime evolution; deferred per strict MVP boundaries. |
| 37 | [Add Just-In-Time Function Generation (lazy_synthesize)](#37-add-just-in-time-function-generation-lazy_synthesize) | ✅ Done | 0.089 (5×0.125÷7) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Defers boilerplate generation to runtime, allowing AI to focus only on high-level logic. Re-scored 2026-07-23: at runtime it would itself call an LLM to synthesize code, placing it in the same "LLM-backed runtime primitive" theme as shipped #26/#35/#36 (3 prior ships → decay 0.125, was uncounted at 1.0). |
| 62 | [Phase 2 IR Abstraction (let, try_let, call, for, spawn)](#62-phase-2-ir-abstraction) | Done | 2.0 (8×1÷4) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Necessary to migrate remaining core nodes to the unified IR so backends are completely decoupled. |
| 63 | [Add Code Examples for Semantic Match and Counterfactual Debugging](#63-add-code-examples) | Done | 1.0 (2×1÷2) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Documentation gap identified; these features are in README but lack code examples. |
| 71 | [Zero Native Store Bytecode](#71-zero-native-store-bytecode) | Proposed | 1.0 (8×0.5÷4) | Sonnet 5 | Gemini 1.5 Pro | gpt-5.6-sol | Add a compiler-visible persistence layer so agents can query structured records without emitting SQL strings. Decay 0.5 because SQL persistence already shipped, but this is a different AI-native abstraction. |
| 64 | [Semantic Type Checker Pass](#64-semantic-type-checker-pass) | Done (2026-07-30) | 3.0 (6×1÷2) | — | — | gpt-5.6-sol | Essential for native code generation (explicit memory layouts). |
| 68 | [Native Logit Masking](#68-native-logit-masking) | Done (2026-07-30) | 2.5 (5×1÷2) | — | — | gpt-5.6-sol | Compile types into inference constraints like LMQL. |
| 70 | [Type-Safe Schema Bridges](#70-type-safe-schema-bridges) | Pending | 2.5 (5×1÷2) | — | — | gpt-5.6-terra | Strongly-typed API boundaries for LLM outputs like BAML. |
| 65 | [Linear SSA-based IR](#65-linear-ssa-based-ir) | Pending | 2.0 (8×1÷4) | — | — | gpt-5.6-sol | Flatten the IR into control flow graphs and basic blocks. |
| 67 | [Native Backend Code Generators](#67-native-backend-code-generators) | Pending | 1.5 (6×1÷4) | — | — | gpt-5.6-sol | Emit LLVM IR or WebAssembly natively. |
| 69 | [First-Class Optimization Signatures](#69-first-class-optimization-signatures) | Pending | 1.5 (6×1÷4) | — | — | gpt-5.6-sol | Teleprompter-style compile-time optimizations like DSPy. |
| 66 | [Standalone Zero Runtime Environment](#66-standalone-zero-runtime-environment) | Pending | 1.0 (8×1÷8) | — | — | gpt-5.6-sol | Replace Go runtime dependency with C/Rust/Zig library. |
## Details

### 62. Phase 2 IR Abstraction
* **Description:** Extend the `IRNode`/`lowerShared`/`emitGoIR`/`emitJSIR` abstraction introduced in improvement #53 to support the remaining core nodes: `let`, `try_let`, `call`, `for`, and `spawn`.
* **Why:** Phase 1 intentionally deferred these because of per-backend implementation differences (like JS's `await`/async threading vs Go's sync, or `env` let-binding special cases). A third backend (Wasm) shipped (#54), which means a unified IR for these nodes is highly valuable to avoid duplicating logic.
* **Impact:** 6/10 (Medium-High — critical unblocker for adding more target backends cleanly).

### 63. Add Code Examples
* **Description:** The `README.md` file correctly describes `semantic_match` (Semantic Routing) and Automated Counterfactual Debugging, but lacks code blocks demonstrating their usage.
* **Why:** Developers and AIs learn via examples. Providing concrete code blocks clarifies the syntax and reduces hallucination.
* **Impact:** 3/10 (Low/Medium — pure documentation fix but high visibility).

### 71. Zero Native Store Bytecode
* **Description:** Add a bytecode-native store abstraction with structured records, exact query predicates, indexes, transactions, and an explicit future semantic-retrieval path. The proposed design is documented in `docs/zero_native_store_design.md`.
* **Why:** Zero already has `db_connect`/`sql_query` and schema DDL, but those primitives still make the AI write SQL strings. A Zero-native store would make persistence compiler-visible and VM-verifiable, giving agents a replacement for SQL/NoSQL at the language level while keeping SQL as an interoperability escape hatch.
* **Impact:** 8/10 (High — persistence is the one major full-stack capability that still leaks a traditional host-language abstraction into AI-authored Zero programs).
* **Fix sketch:** Start in the bytecode VM with in-memory `store_open`, `store_put`, `store_get`, and `store_delete`, then add `store_query` over structured `(where ...)` predicates, then durability behind the same bytecode contract. Add semantic indexes only after deterministic query semantics and capability checks are solid.

### 60. Add Collection Read Access (map_get/list_get)
* **Description:** `zero.go` has no `[` bracket token anywhere in the lexer (`grep -n "'\['" zero.go` → no match) and no `map_get`/`list_get`/index node in the AST. `dict` and `list` support construction (`(dict ...)`, `(list ...)`), mutation (`append`, `map_set`, `map_delete`), and iteration (`for`) — but there is no way to read a single value back out by key or index. A dict built with `map_set` can only be inspected by printing the whole thing; a list can only be consumed by iterating every element.
* **Why:** Found while writing improvement #49 Phase 1's regression fixture (`tests/test_interpret_basic.zero`): tried to prove a `map_set` actually stored the right value by reading it back with a natural `(map_get d "k2")` and discovered no such primitive exists in either backend — had to fall back to printing the whole dict instead. This is the same "write-only collection" gap for `dict`/`list` that `str_split`/`str_join` don't have (strings round-trip fine), and blocks any realistic pattern beyond "build a collection, then either mutate it blindly or dump the whole thing" — e.g. looking up a config value by key, or indexing a specific parsed result out of a list.
* **Impact:** 7/10 (High — collections without random-access read are usable for very little beyond blind accumulation; this is a basic, expected operation for any dict/list-shaped data).
* **Repro:** no lex/parse error occurs; there's simply no node to reach for. `(let (v (map_get d "k1")) (print v))` transpiles with `Unknown statement: map_get` (falls through to the generic "Unknown statement" error at the end of `generateStatementRaw`).
* **Fix sketch:** add `map_get`/`list_get` node kinds mirroring `map_set`/`map_delete`'s existing pattern (zero.go's `map_set`/`map_delete` IR cases are a direct template): `(map_get dict key)` → Go `dict[key]` (with Go's zero-value-on-missing-key semantics, matching how `map_set`/`map_delete` already don't distinguish "key absent" from "key present with zero value"), `(list_get list idx)` → Go `list[idx]` (will panic on out-of-bounds, same as any other unchecked Go slice index in this language today — e.g. `cli_args`'s index path already tolerates out-of-range by returning `""` rather than panicking, so consider matching that safer convention instead of raw Go indexing). Needs both Go and JS (`generateJSStatementRaw`) backend cases, plus fixtures. A natural pairing with #53's shared IR (`map_set`/`map_delete` are IR-unified kinds already) if the Go/JS semantics end up identical.
* **Done (2026-07-24):** Implemented exactly per the fix sketch as shared IR kinds (`lowerShared`, alongside `map_set`/`map_delete`) so both the Go and JS backends dispatch through one arity check. `emitGoIR`'s `map_get` emits raw `dict[key]` indexing (Go's `map[string]string` zero-value `""` on a missing key needs no special-case code); `list_get` uses a bounds-checked closure mirroring `cli_args`' existing safer convention (zero.go:1581) — returns `""` on out-of-range instead of panicking. `emitJSIR` mirrors both with a trailing `?? ""` so JS's `undefined`-on-miss/out-of-range matches Go's `""` exactly. Also added to the Phase 1 interpreter (`evalMapGet`/`evalListGet` in `evalList`'s dispatch, improvement #49) for consistency — every other collection op (`append`/`map_set`/`map_delete`) was already interpreter-supported, so leaving reads out would have been an inconsistent gap in the "bounded subset" it claims to cover; the interpreter's dynamic `any`-typed dict returns Go's real zero value (`nil`, printed as `<nil>`) rather than `""` on a missing key, a deliberate, documented divergence from the Go backend in the same spirit as #49 Phase 1's other noted deviations, not a bug. New regression fixture `tests/test_collection_read.zero` (hit/miss `map_get`, in-range/out-of-range `list_get`) verified through both the interpreter (`./zero -run`) and the Go backend (`./zero` + `go build` + `go run server.go`) with matching output apart from the documented nil/"" divergence. Full `tests/*.zero` suite re-run through transpile + `go build -o /tmp/servercheck .`: only the pre-existing, expected `tests/routes.zero` failure (an `include`-only fragment, not a standalone root) reproduces — no new regressions. Documented in `README.md`'s "Collections & Mutability" section (both new primitives added to the existing example, re-verified transpiling/building/running before publishing per the bugs #23/#24 lesson) and `docs/index.html`'s feature list.
* **Description:** Add an `-o` flag to `zero.go` (and the built `zero` binary) to specify an output directory for `server.go` and `server_test.go`.
* **Why:** Running `./zero tests/some_test.zero` overwrites `server.go` and `server_test.go` in the root directory. This makes running tests concurrently or keeping the repo clean difficult.
* **Impact:** 2/10 (Minor but highly convenient for DX).

### 1. Add Routing Support
* **Description:** Update the compiler to accept multiple `(route path handler)` definitions inside a web server block.
* **Why:** The prototype only builds a single server with a hardcoded route. Real applications need routers.
* **Impact:** 2/10 (Minor - helpful but not strictly blocking).

### 2. Add Conditionals and Variables
* **Description:** Introduce `let` and `if` blocks to handle internal request logic. For example: `(if (= req.method "POST") ...)`. This will require updating the Lexer to handle operators like `=` and the Code Generator to output Go `if` statements.
* **Why:** Web handlers need to implement dynamic logic based on request types and data.
* **Impact:** 8/10 (High).

### 3. Extend Python Orchestrator Grammar
* **Description:** Currently, `orchestrator.py` uses a strict regex for the proof-of-concept single endpoint. As we implement improvements 1 and 2, this regex needs to be translated into a full Context Free Grammar (CFG) using Outlines to support nested expressions and arbitrary routes.
* **Why:** The LLM agent loop breaks if it cannot generate valid syntax for new AST nodes.
* **Impact:** 4/10 (Medium - blocks orchestrator but not manual transpiler usage).

### 4. Add Database Connections (SQL)
* **Description:** Implement SQL database connections via Go's `database/sql` mapping to an S-expression like `(sql_query db "SELECT * FROM users")`.
* **Why:** Real-world applications require state and persistence.
* **Impact:** 6/10 (Medium).

### 5. Add JSON Request/Response Handling
* **Description:** Implement a way to parse JSON bodies into variables and output JSON responses cleanly via `encoding/json`. E.g., `(parse_json req.body)` and `(res_json 200 data)`.
* **Why:** The modern web runs on JSON; text/plain is insufficient.
* **Impact:** 5/10 (Medium).

### 6. Add Function Definitions (defun)
* **Description:** Allow defining standard functions `(defun name (args) body)` outside of routes that can be called anywhere.
* **Why:** Needed to adhere to modularity and DRY principles.
* **Impact:** 8/10 (High).

### 7. Add Structs and Type Definitions
* **Description:** Implement `(struct Name (field type) ...)` to enforce Go's strict typing system for parsing JSON and scanning SQL rows.
* **Why:** Strictly typed inputs are a core requirement of defensive programming and input validation.
* **Impact:** 7/10 (High).

### 8. Add Iteration and Data Structures
* **Description:** Support loops `(for ...)` and basic collections `(list ...)` and `(dict ...)`.
* **Why:** Essential for mapping over database query results or iterating through JSON arrays.
* **Impact:** 6/10 (Medium).

### 9. Add Environment Variables Access
* **Description:** Introduce a `(env "KEY")` node to retrieve environment variables.
* **Why:** Vital for securely injecting database credentials and API keys without hardcoding them in the S-expressions.
* **Impact:** 3/10 (Low/Medium - security critical).

### 10. Add External Module Imports
* **Description:** Allow defining `(import "github.com/pkg")` at the root level to pull in external Go code.
* **Why:** Makes Zero extensible and leverages the massive open-source Go ecosystem.
* **Impact:** 3/10 (Low - advanced feature).

### 11. Add Concurrency (spawn)
* **Description:** Add a `(spawn (lambda () ...))` node that maps to Go's `go func() {}` to execute non-blocking routines.
* **Why:** AI agents building web applications often need to trigger background processes (like sending emails or metrics) without delaying the HTTP response.
* **Impact:** 7/10 (High).

### 12. Add Error Handling (try/catch)
* **Description:** Implement `(try (expression) (catch err ...))` to wrap Go expressions that return `(value, error)`. 
* **Why:** Go relies heavily on `if err != nil`. We need a clean, Lisp-like way to handle these errors safely in Zero without panicking.
* **Impact:** 8/10 (High - critical for production safety).

### 13. Add File Inclusions (include)
* **Description:** Implement `(include "routes.zero")` to dynamically merge multiple Zero files during the transpilation step.
* **Why:** A full-fledged language needs modularity. Right now, everything must live in one massive S-expression.
* **Impact:** 7/10 (High).

### 14. Add Basic Math and Logic Operators
* **Description:** Support native mathematical and logical operators like `(+ 1 2)`, `(- a b)`, `(and x y)`.
* **Why:** Computing logic natively (like paginating data or computing totals) is currently impossible without external SQL/Go functions.
* **Impact:** 8/10 (High).

### 15. Add Middleware Support
* **Description:** Introduce a `(middleware auth_func)` block that can wrap a set of `(route ...)` blocks.
* **Why:** Modern APIs require authentication headers, logging, and CORS handling. Middleware is the standard pattern for this.
* **Impact:** 5/10 (Medium).

### 34. Add Semantic Routing (semantic_match)
* **Status Note:** Done (2026-07-29). Implemented native control flow structure via LLM intent matching in `gogen` and the legacy VM, using explicit user confirmation to bypass the ROI floor.
* **Description:** A control flow structure that routes execution based on the semantic proximity (intent and meaning) of an input string compared to a set of natural language descriptions.
* **Why:** Natively understands intent. Acknowledges that human language is fuzzy and allows the code to handle it gracefully without exhaustive mapping or complex regexes.
* **Impact:** 7/10 (High - unlocks intent-based routing).

### 35. Add LLM-powered Type Coercion (fuzzy_cast)
* **Description:** A casting function `fuzzy_cast[T]` that uses structured-output LLM APIs to automatically coerce messy, unstructured text into a strictly typed struct `T`.
* **Why:** Traditional serialization requires perfect 1:1 schema matches. This acts as a universal, intelligent parser that infers required mapping.
* **Impact:** 8/10 (High - eliminates brittle parsing code).

### 36. Add Intent-based Validation (assert_semantic)
* **Description:** An assertion primitive that evaluates qualitative, subjective natural language conditions against a variable. E.g. `assert_semantic(user_bio, "is professional")`.
* **Why:** Allows the code to enforce complex, qualitative boundaries effortlessly, removing the need for massive heuristic functions.
* **Impact:** 6/10 (Medium - powerful for data safety).

### 37. Add Just-In-Time Function Generation (lazy_synthesize)
* **Status Note:** Done (2026-07-29). Implemented `lazy_synthesize` in the tree-walking interpreter and bytecode compiler/VM to synthesize Zero Lisp function bodies at runtime via local LLM.
* **Description:** A declarative primitive for defining a function using only its signature and a natural language docstring. The implementation is dynamically generated the first time it is invoked.
* **Why:** AI writing the language doesn't have to waste tokens generating mundane utility functions, delegating implementation to the runtime.
* **Impact:** 5/10 (Medium - innovative but complex to execute).

### 38. Add Swarm Primitives
* **Status Note:** Done (2026-07-29). Implemented basic `spawn_agent` and `task` nodes in AST.
* **Description:** Introduces autonomous subagents as first-class concurrency objects. Developers orchestrate a swarm of agents using primitives like `(spawn_agent "Researcher" (task "find sources"))` that communicate via typed message-passing channels and autonomously negotiate tasks.
* **Why:** Concurrency shifts from deterministic CPU scheduling to non-deterministic, autonomous orchestration, breaking conventional rules and allowing agents to independently verify upstream outputs.
* **Impact:** 2/10 (Low - extremely advanced, deferred for strict MVP scoping).

### 39. Add Teleological Execution
* **Status Note:** Done (2026-07-29). Native `achieve` node implemented for direct VM execution and Go codegen.
* **Description:** A goal-driven syntax where developers define a target state (e.g., `(achieve (is_sorted list) (using "quick sort algorithm"))`) rather than imperative steps. The runtime acts as a solver to dynamically search for the execution path and execute necessary steps.
* **Why:** Abandons imperative control flow entirely. Code becomes a set of constraints and objectives, making execution a continuous planning and state-space search process.
* **Impact:** 2/10 (Low - radical shift, deferred for MVP).

### 40. Add Auto-Mutating Runtime
* **Status Note:** Done (2026-07-30). Implemented `optimize_block` for Go Codegen utilizing runtime compilation and hot-swapping via Go plugins, and mapped natively for evaluation in the interpreter VMs.
* **Description:** A self-rewriting primitive `(optimize_block ...)` that monitors execution metrics and automatically employs an LLM to rewrite and hot-swap its underlying Go implementation at runtime if bottlenecks are detected.
* **Why:** Code becomes active and evolutionary in production rather than immutable, natively incorporating model evaluation and code generation into the execution cycle.
* **Impact:** 1/10 (Low - highly experimental).

### 41. Add Stochastic Control Flow
* **Status Note:** Done (2026-07-29). Native `confidence` node implemented for direct VM execution and Go codegen, evaluating conditional uncertainty directly via LLM.
* **Description:** Natively handles uncertainty in the AST. Conditions evaluate to probability distributions, allowing control flow primitives like `(if (> (confidence (is_fraud tx)) 0.95) ...)` to branch based on statistical certainty.
* **Why:** Eliminates hardcoded heuristics by bringing fuzzy logic directly into the core execution loop, perfectly matching the probabilistic nature of AI models.
* **Impact:** 3/10 (Low/Medium - complex but powerful for AI).

### 44. Add Cross-Language "AI Write Cost" Benchmark
* **Description:** Build a benchmark comparing Zero against Go, Python, Node.js, C#, and Java on the cost of *writing* a correct, working program with an LLM — not runtime/compile speed. Metrics: (1) wall-clock time for the LLM to produce a working solution to a fixed task prompt, including any compile-error self-correction retries; (2) token count of the final generated source, measured with `tiktoken` as a reproducible proxy for LLM output-token cost. Fixed task set (same 3 tasks in all 6 languages, 18 programs total):
  * **A — Hello World HTTP+JSON server:** mirrors the existing README example (root route returns text, `/json` route returns JSON).
  * **B — CLI file-parsing tool:** read a file of names (one per line), print a greeting per non-blank name, and handle a missing file gracefully (print an error, don't crash). *Revised from an original "sum a numeric column" design after discovering Zero has no deterministic string-to-number primitive — see bug #17 — which would have made Task B unwritable in Zero without abusing the LLM-backed `fuzzy_cast`; the string-only version keeps the 6-language comparison fair while still exercising file I/O, iteration, and error handling.*
  * **C — Function + unit test:** an `add(a, b)` function with an accompanying test, showcasing Zero's native `(test ...)` block against each language's idiomatic test boilerplate.

  Every program must actually compile and run (not just be reviewed) before its numbers count — Go, Python, and Node were already available locally; Java (`openjdk`) and .NET (`dotnet`) SDKs were installed via Homebrew (`brew install openjdk dotnet`, user-space, no `rpm-ostree`/sudo needed on this Bazzite/Kinoite host) specifically for this benchmark so all 6 languages get equal, compiler-verified footing. Results are published as a standalone file (e.g. `docs/benchmarks/language_write_cost.md`) with a summary table, linked from both `README.md` and `docs/index.html`.
* **Why:** Zero's entire pitch is reducing hallucination/retry cost for LLM-authored code, not runtime performance (see "Why Zero?" in `README.md`). A benchmark that measures write-time and token cost directly tests that claim with live evidence instead of assertion, per the Grounding Protocol's "answer requires live data → query it, don't estimate" rule. Wall-clock time in this harness includes model reasoning and tool-call overhead, not raw decode latency — that limitation is stated explicitly in the published results so the numbers aren't mistaken for a controlled lab benchmark.
* **Impact:** 6/10 (High marketing/validation value — not blocking core transpiler functionality).
* **Done (2026-07-23):** All 18 programs (3 tasks × 6 languages) written, timed, token-counted, and verified via actual compile/run (`go build`/`go run`, `python3`/`pytest`, `node`/`node --test`, `dotnet build`/`dotnet test`, `javac`+JUnit console). Source lives in `benchmarks/language_write_cost/` (raw data in `results.csv`); full write-up with per-task tables and honest findings (Zero wins clearly on Task A, is mid-pack on total tokens, and is *most* token-heavy on Task C due to mandatory `type_hint` boilerplate) published at `docs/language_write_cost_benchmark.md`, linked from `README.md` and a new section in `docs/index.html`. Discovered and filed two real transpiler gaps along the way: [bug #17](bugs.md#17-no-string-to-number-parsing-primitive) (no string-to-number primitive) and its `read_file`/`[]byte`-to-`string` addendum. Journal archived at `docs/journals/archive/2026-07-23_ai_write_cost_benchmark.md`.

### 47. Document undocumented shipped primitives
* **Description:** A README.md/docs/index.html coverage audit against every "Done" row in this backlog's Ranked table (2026-07-23) found roughly a dozen already-shipped, working primitives with zero mention or working example in either document: `db_connect`/`sql_query` (#4, Database Connections — the biggest gap, explicitly called "crucial for actual web service capabilities" when it shipped), `import` (#10, external module imports), `middleware` (#15), `include` (#13, file inclusion), `append`/`map_set`/`map_delete` (#31, mutable collections), `str_split`/`str_join`/`regex_match` (#30, string manipulation suite), `env` (#9 — despite being flagged "security critical"), `spawn` (#11 — alluded to in docs/index.html prose but never shown as real syntax), `fetch` (#21 HTTP client), `match` and `rate_limit`/`retry` (named in docs/index.html prose only, no code example), and `struct` (only a bare one-line inline declaration with no field-access/JSON-mapping demo).
* **Why:** An undocumented primitive is invisible both to human adopters reading README.md and to the AI orchestrator's prompting flow (`orchestrator.py`'s grammar exposes what the LLM can attempt, but nothing points a human or an LLM's own reasoning at capabilities the README never surfaces). Given the project's stated goal of eventually being installed and used by "AI agents anywhere," a large silent gap between shipped and documented capability directly undermines adoption — this is a completeness/marketing gap, not a functional one.
* **Impact:** 6/10 (Medium-High — doesn't block any functionality, but roughly a third of shipped primitives are effectively unusable by anyone who only reads the docs).
* **Fix sketch:** add one compact, verified (`go run zero.go` + `go build`) example per undocumented primitive above to README.md (grouped naturally — e.g. a "Database & Persistence" section for `db_connect`/`sql_query`, a "Modularity" section for `import`/`include`, a "Collections" section for `append`/`map_set`/`map_delete`) and mirror the highlights into docs/index.html's feature list/examples. Reuse `tests/*.zero` fixtures where one already demonstrates the primitive cleanly (e.g. `tests/test_middleware.zero`, `tests/test_mutable.zero`) rather than authoring net-new snippets from scratch. Verify every new snippet actually transpiles and builds before publishing, per the bugs #23/#24 lesson that unverified doc examples silently rot.
* **Done (2026-07-23):** Delegated to Antigravity CLI (`agy`, `gemini-3.1-pro-high`) with a self-contained brief covering all 12 primitives grouped into 7 new README.md sections (Database & Persistence, Modularity, Collections & Mutability, String Manipulation & Regex, Security & Auth Middleware, Concurrency/Networking/Control Flow, Typed Structs & Field Access) plus new `docs/index.html` "Why Zero?" bullets for the previously-unmentioned primitives. Diff verified real via `git diff` before trusting the delegate's summary; independently re-verified all 7 new/changed `.zero` snippets myself (not just trusting the delegate's own claim) by transpiling each (`./zero`) and building the result (`go build -o <scratch> .`, never bare `go build .` in the repo root) — all 7 passed with no errors.

### 46. Close the Benchmark-Found Gaps and Make It a Standing Metric
* **Description:** Two concrete, measured weaknesses came out of the [#44 benchmark](docs/language_write_cost_benchmark.md) — treat both as required inputs to language design instead of a one-time write-up:
  1. **Cut `type_hint` boilerplate.** Task C showed Zero is the *most* token-heavy of all six languages on a simple `add(a, b)` function, purely because typing a two-argument `defun` costs three separate `(type_hint ...)` forms (one per argument plus the return value) versus Go's single native parameter list. Add a terser inline form — e.g. typed params directly in the `defun` arg list, `(defun add ((a int) (b int)) int ...)`, or a single combined `(type_hints (a int) (b int) (return int))` node — that lowers to the same Go signature the current three-statement form produces, without removing the existing longhand `type_hint` node (existing `.zero` files must keep compiling).
  2. **Close [bug #17](bugs.md#17-no-string-to-number-parsing-primitive)** (no deterministic string-to-number primitive) and its `read_file`/`[]byte`-to-`string` addendum — the other concrete gap Task B surfaced. Tracked as its own bug, but listed here because it's benchmark-sourced and should be closed before the next comparison run.
  3. **Formalize the benchmark as a regression gate**, not just a one-off report: any transpiler change touching `defun`/`type_hint`, `read_file`, `str_split`, or the `test` block must re-run the harness in `benchmarks/language_write_cost/`, update `results.csv` and `docs/language_write_cost_benchmark.md`'s tables, and note the delta — the doc's own closing line already asks for this but nothing currently enforces it. Add this as a Working Protocol step (above) so it isn't lost.
* **Why:** The benchmark's own conclusion was explicit that these two findings "should be treated as backlog items to close before re-running this benchmark for a future comparison" — leaving it as a static report would waste the one piece of live evidence the project has for where the language actually costs more tokens than it should. Per the Grounding Protocol, live measured data beats assertion; this item is what "acting on" that data looks like instead of just having collected it.
* **Impact:** 7/10 (High — directly targets Zero's only measured weakness against mainstream languages, and turns a single benchmark into a repeatable design feedback loop).

### 45. Add Zero-to-JavaScript Compilation Target
* **Description:** Add a second code-generation backend so the existing Zero lexer/parser (unchanged — same S-expression grammar) can emit JavaScript instead of Go, selected by a new browser-appropriate root block (e.g. `(web_app ...)`) alongside the existing `(http_server ...)` and `(cli_app ...)` roots, which don't apply outside a server/CLI context. `generateCode` in `zero.go` currently returns `(mainCode, testCode string)` for one Go-shaped target; this adds a parallel `generateJSCode`-style path dispatching on the same AST. Needs a small set of new DOM/browser primitives with no Go analog — e.g. `(dom_query selector)`, `(on_event el "click" (lambda (e) ...))`, `(set_text el val)`, `(set_attr el name val)` — reusing existing primitives (`let`, `if`, `for`, `defun`, `fetch`, `try`/`catch`, math/logic operators) as-is since they're target-agnostic. Existing `(test ...)` blocks should compile to a JS test runner (e.g. Node's built-in `node --test`, matching the precedent set by improvement #16's Go `_test.go` output) for parity with the Go target's TDD workflow.
* **Why:** Zero's core pitch is a simple, constrained AI-facing grammar that a compiler can validate immediately and feed errors back for self-correction (see `README.md`, "Why Zero?"). That benefit currently stops at the backend. A JS target extends it to the one place in the stack where LLMs also reliably hallucinate — application logic and API usage, just in the browser instead of the server — without touching HTML/CSS, which don't have the same hallucination failure mode and are better left native (browsers are forgiving of markup/style syntax in a way they aren't of broken JS logic; reinventing the cascade/box model isn't worth the engineering cost). Keeps a single language for an AI agent to learn across the whole stack instead of switching grammars at the API boundary.
* **Impact:** 8/10 (High — doubles Zero's addressable surface from backend-only to full-stack; direct extension of the core value proposition rather than a new one).
* **Scope boundary:** JS only. HTML and CSS are explicitly out of scope for this item — see 2026-07-23 conversation notes; a future, separate item could add a thin Hiccup-style S-expression sugar that transpiles to plain HTML if wanted, but that's not part of this improvement.

### 42. Clean up file structure
* **Description:** Move all `.zero` test files (e.g. `test_*.zero`) into a `tests/` directory, and example files (`hello.zero`, `cli_hello.zero`) into an `examples/` directory. Move or gitignore generated binaries.
* **Why:** The project root is getting messy, making it hard to find core files like `zero.go` and `orchestrator.py`.
* **Impact:** 4/10 (Quality of life, helps AI reasoning speed).

### 20. Auto-Tracing (`trace`)
* **Description:** A `(trace var)` macro that auto-injects the variable's name, its current value, and the source line number into a `fmt.Println` call, so an AI debugging a `.zero` script doesn't have to hand-write ad hoc print statements.
* **Why:** AI debugs by spamming `print`. A native `trace` node standardizes that habit into consistent, greppable output (name + value + line) with a single node instead of a hand-rolled `fmt.Sprintf`.
* **Impact:** 3/10 (Low/Medium — pure developer-experience convenience, no new capability unlocked).
* **Groomed (2026-07-23):** confirmed unimplemented (`grep -n '"trace"' zero.go` → no match). This item previously existed only as a one-line row in the legacy "V2" table below with no detail section and no row in the main Ranked Backlog table above — invisible to anyone scanning only the primary table, which is how `work_next_item` selects. Backfilled this detail section from the V2 row's text and added a corresponding row to the main table.

### 18. Declarative Schema Migrations
* **Description:** A `(schema "users" (column "id" "int") (column "name" "string") ...)` root-level node that the transpiler expands into a `CREATE TABLE IF NOT EXISTS` statement, run automatically against the connection established by `db_connect` (#4, Done).
* **Why:** Currently every `.zero` script that wants a table must hand-write the `CREATE TABLE` DDL as a raw string passed to `sql_query` (#4) — declarative schema definition would let the AI describe the shape of its data once (reusing the same field/type syntax as `struct`, #7, Done) and have both the Go `struct` and the SQL DDL derived from a single source of truth, rather than keeping them manually in sync.
* **Impact:** 5/10 (Medium — quality-of-life and correctness win for any script using `db_connect`, but `sql_query` already provides a working, if manual, escape hatch).
* **Groomed (2026-07-23):** confirmed unimplemented (`grep -n '"schema"' zero.go` → no match). Like #20, this item previously existed only as a one-line row in the legacy "V2" table with no detail section and no row in the main Ranked Backlog table — backfilled here and added to the main table. Not treated as same-theme/decayed against #4 (`db_connect`/`sql_query`) or #7 (`struct`) since it's a new declarative-codegen capability built *on top of* those primitives, not a repeat of either.

---

## V2: AI-First Language Optimizations

Now that Zero V1 is complete (a full Turing-complete web server and CLI language), the next phase is optimizing it specifically for **Autonomous AI Development**. Since Zero does not need to be human-readable, we can bend the language features to perfectly suit AI agents.

**Groom note (2026-07-23):** this table's Done rows are historical record only — they're already reflected as shipped capabilities elsewhere and need no further action. Its three Pending rows (#18, #20, #43) were, until this groom pass, tracked *only* here with no row in the main Ranked Backlog table above and (for #18/#20) no detail section — both gaps are now fixed (rows added to the main table, detail sections backfilled just above this one). Treat the main Ranked Backlog table as the single source of truth for open work; this table's own Score/AI Rationale columns are left as-is for history but are superseded by the main table's rows for #18/#20/#43.

### Proposed Improvements

| # | Improvement | Status | Score | Claude model | Gemini model | OpenAI model | AI rationale |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 17 | **Type Hinting for `defun`** | Done (2026-07-22) | 3.5 (7×1.0÷2) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Currently, all `defun` arguments compile to `string`. Adding `(type_hint var "int")` ensures the AI gets immediate compile-time errors from Go. |
| 19 | **Context/Intent Nodes (`intent`)** | Done (2026-07-22) | 2.0 (4×1.0÷2) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | `(intent "I am building a login flow")`. The transpiler strips these out, but agents can parse them to instantly understand context. |
| 21 | **Native HTTP Client (`fetch`)** | Done (2026-07-23) | 4.0 (8×1.0÷2) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Essential for an AI language to interact with external APIs (like LLM providers or GitHub) without writing raw Go `net/http` code. |
| 31 | **Mutable Collections (`append`, `map_set`)** | Done (2026-07-23) | 8.0 (8×1.0÷1) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Needed to build up dynamic lists (like AST children) and manage state. |
| 26 | **LLM-Native Primitives (`llm_generate`)** | Done (2026-07-23) | 6.0 (6×1.0÷1) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Built-in nodes like `(llm_generate "prompt")` to make it trivial for an AI to utilize other AIs. |
| 27 | **AST-Level Semantic Patching** | Done (2026-07-23) | 5.0 (5×1.0÷1) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | `(patch function (body))` allows the AI to surgically update specific functions without rewriting the whole file. |
| 28 | **Built-in Rate Limiting / Circuit Breakers** | Done (2026-07-23) | 4.0 (4×1.0÷1) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Native `(rate_limit "10/s" (fetch ...))` provides essential guardrails against AI DDoS or loops. |
| 22 | **Subprocess Execution (`exec`)** | Done (2026-07-23) | 3.5 (7×1.0÷2) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Crucial for automation tasks (e.g. `(exec "git status")`). Follows automation skills for script consolidation. |
| 30 | **String Manipulation Suite (`str_split`, `str_join`, `regex`)** | Done (2026-07-23) | 3.5 (7×0.5÷1) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Essential for parsing and lexing text, required for self-hosting. Decay 0.5. |
| 32 | **Advanced Control Flow (`while`, `match`)** | Done (2026-07-23) | 3.25 (6.5×0.5÷1) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | State machines and parsers require `while` loops and pattern matching for tokens. Decay 0.5. |
| 23 | **File I/O Operations (`read_file`)** | Done (2026-07-23) | 3.0 (6×1.0÷2) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Needed to replace Bash/Python for file manipulation. `(write_file "log.txt" data)` and `(read_file "config.json")`. |
| 29 | **Implicit Context Threading** | Done (2026-07-23) | 3.0 (3×1.0÷1) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | `(with_context db ...)` auto-generates Go code that threads dependencies implicitly, saving cognitive load. |
| 33 | **Full File System I/O (`write_file`, `mkdir`)** | Done (2026-07-23) | 3.0 (6×0.5÷1) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Necessary for the transpiler to write out generated `.go` files and manage projects. Decay 0.5. |
| 24 | **CLI Argument Parsing (`cli_args`)** | Done (2026-07-23) | 2.5 (5×1.0÷2) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Required for workflow consolidation (per `automation` skill). Allows Zero scripts to take parameters effortlessly. |
| 25 | **Timers and Backoff (`sleep`)** | Done (2026-07-23) | 2.0 (4×1.0÷2) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Fault tolerance (per `automation` skill) requires exponential backoff and deliberate delays `(sleep 1000)` during API rate limits. |
| 16 | **Native Unit Test Blocks (`test`)** | Done (2026-07-23) | 1.5 (6×1.0÷4) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | AI iterates faster with TDD. A native `(test "desc" ...)` block at the root that compiles directly to `_test.go` allows seamless testing. |
| 20 | **Auto-Tracing (`trace`)** | Done (2026-07-23) | 1.5 (3×1.0÷2) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | AI debugs by spamming `print`. A `(trace var)` macro auto-injects line numbers and variable names into `fmt.Println`. |
| 18 | **Declarative Schema Migrations** | Done (2026-07-23) | 1.0 (5×1.0÷5) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | If `(schema "users" (column "id" "int"))` is in `.zero`, the transpiler can auto-generate `CREATE TABLE IF NOT EXISTS`. |
| 43 | **Support for Go Generics** | Done (2026-07-23) | 0.8 (4×1.0÷5) | Sonnet 3.5 | Gemini 1.5 Pro | gpt-5.6-terra | Add `(type_param T)` syntax to `defun` to enable generating generic Go functions, useful for reusable AI-generated components. |

### 43. Support for Go Generics
* **Description:** Add `(type_param T)` syntax inside `defun` definitions, allowing the generated Go functions to utilize Go generics (e.g. `func MyFunc[T any](val T)`).
* **Why:** AI models frequently generate reusable utility functions. Without generics, they have to use `any` and perform runtime type assertions, losing the benefits of Go's strict typing system.
* **Impact:** 4/10 (Valuable for building typed standard libraries).

### 26. LLM-Native Primitives
* **Description:** Add built-in nodes like `(llm_generate "prompt" model="...")` and `(vector_embed text)`.
* **Why:** Makes it trivial for an AI to write applications that spawn or utilize other AIs.
* **Impact:** 9/10 (Critical for an AI-first language).

### 27. AST-Level Semantic Patching
* **Description:** Introduce a `(patch function_name (new_body))` node.
* **Why:** LLMs struggle with rewriting large files perfectly. A patch node would allow surgical updates.
* **Impact:** 8/10 (High).

### 28. Built-in Rate Limiting / Circuit Breakers
* **Description:** Add a native `(rate_limit "10/s" (fetch ...))` or `(retry 3 (fetch ...))`.
* **Why:** AI agents writing automation can accidentally DDoS APIs or fall into infinite loops.
* **Impact:** 7/10 (High).

### 29. Implicit Context Threading
* **Description:** A `(with_context db ...)` block that auto-generates Go code threading dependencies.
* **Why:** Removes the need for the AI to remember to pass `req`, `db`, or `context.Context` to every sub-function.
* **Impact:** 6/10 (Medium).

### 30. String Manipulation Suite
* **Description:** Add standard string operations such as `(str_split s sep)`, `(str_join list sep)`, `(str_sub s start end)`, and `(regex_match pattern s)`.
* **Why:** Self-hosting a transpiler requires reading and manipulating raw text efficiently (e.g., tokenizing source code).
* **Impact:** 8/10 (High - blocking for self-hosting).

### 31. Mutable Collections
* **Description:** Add `(append list item)`, `(map_set dict key val)`, and `(map_delete dict key)` to mutate data structures after creation.
* **Why:** The AST builder needs to push parsed child nodes into an array dynamically. Currently, only static lists exist.
* **Impact:** 9/10 (Critical - blocking for self-hosting).

### 32. Advanced Control Flow
* **Description:** Introduce `(while cond body)` for unbounded loops, and `(match var (val body)...)` for cleanly branching on token types.
* **Why:** Writing state machines (Lexers and Parsers) with just basic `for` range loops is extremely difficult.
* **Impact:** 7/10 (High - blocking for self-hosting).

### 33. Full File System I/O
* **Description:** Expand the planned file I/O to include robust writing and directory management: `(write_file path data)`, `(mkdir path)`, and `(list_dir path)`.
* **Why:** A compiler needs to manage projects, traverse source directories, and write output binary/code files to disk.
* **Impact:** 8/10 (High - blocking for self-hosting).

### 16. Native Unit Test Blocks
* **Description:** A native `(test "description" body...)` block at the root of `http_server` or `cli_app` compiles directly into a Go `TestXxx(t *testing.T)` function in `server_test.go`, sitting alongside the generated `server.go`. The description is slugified into a valid Go identifier.
* **Why:** AI iterates faster with TDD; without this, the AI has to hand-write separate Go test files outside the `.zero` source of truth.
* **Impact:** 6/10 (Medium-High - unlocks native test-driven workflows).
* **Done (2026-07-23):** Implemented in `generateCode` (now returns `(mainCode, testCode string)`); `main()` writes `server_test.go` when test blocks are present and removes it otherwise. Verified with `tests/test_feature.zero` — `go build`, `go vet`, and `go test` all pass. Delegated to agy; picked up and closed out after the delegate hit a session limit mid-task (see former journal `2026-07-23_native_unit_test_blocks.md`).

### 34. AI Uncertainty Blocks
* **Description:** Introduce a specific uncertain wrapper block for generated code. The Go transpiler allows the code to run in a test environment but strictly refuses to compile a production binary until a human reviews and removes the tag.
* **Why:** LLMs operate on probability and they often know when they are guessing. If the agent generates a complex algorithm but is not highly confident in its logic, it needs a way to flag it for human review.
* **Impact:** 7/10 (High - improves safety and trustworthiness of generated code).

### 35. Cryptographic Code Provenance
* **Description:** Automatically hash and sign every single block of generated code. This signature would include the exact prompt context, the LLM version, and the timestamp.
* **Why:** Supply chain security is a massive concern with AI code generation. If a vulnerability is discovered later, auditors can query the binary to see exactly why the agent wrote that specific function.
* **Impact:** 6/10 (Medium-High - crucial for enterprise and security audits).

### 36. Semantic Codebase Querying
* **Description:** Expose a native `query_graph` primitive that allows the AI to ask the compiler architectural questions (e.g., all functions that modify a specific database table) and receive a clean JSON response.
* **Why:** Instead of making the agent use grep to search through text files, it leverages the fact that the AI is writing structured code, enabling much more accurate codebase exploration.
* **Impact:** 8/10 (High - drastically improves the agent's ability to navigate and refactor code).

### 37. Native Token and Cost Budgets
* **Description:** Introduce a `with_budget` primitive. An agent can wrap a subtask in a block that specifies a hard token limit or monetary cap. 
* **Why:** Agents can easily get stuck in loops and burn through API credits. The runtime needs a way to safely halt execution before costs spiral out of control.
* **Impact:** 7/10 (High - essential for resource management and preventing runaway costs).

### 38. Test Driven Self Healing
* **Description:** Introduce an `assert_and_patch` block. If the assertion fails during the Go testing phase, the transpiler captures the memory state, stack trace, and expected output, sending it to the agent to rewrite the function automatically.
* **Why:** Instead of a normal test failure, this automates the debugging loop behind the scenes before presenting the final application to the user.
* **Impact:** 8/10 (High - greatly accelerates the development loop and auto-fixes bugs).

---

## V3: AI-Native Execution & Agentic Observability (The End Goal)

As Zero matures past transpilation into Go and JS, the ultimate objective is to bypass human-readable intermediate languages entirely. The future of Zero is an execution environment where logic is represented natively for machines, and debugging is handled autonomously by AI.

### Actionable Milestones & Proposed Improvements (Ranked)

| # | Improvement | Status | Score | AI Rationale |
| --- | --- | --- | --- | --- |
| 58 | **Crash-State Serialization** | Done (2026-07-23) | 2.33 (7×1.0÷3) | High value self-healing foundation, low effort. |
| 55 | **Native Telemetry Injection** | Done (2026-07-23) | 1.50 (6×1.0÷4) | Observability requisite; compiler hook injection. |
| 56 | **Standalone Observer Agent (`observer.py`)** | Done (2026-07-23) | 1.40 (7×1.0÷5) | Standalone daemon; independent effort from transpiler. |
| 53 | **Decouple AST from Go Codegen (IR Abstraction)** | Done (2026-07-24) — Phase 1 | — | Requisite for pure binary generation. See item #53 below for Phase 1/2 scoping. |

**Groom note (2026-07-24):** This table's remaining open rows (#49, #50, #51, #52, #54, #57, #59) were removed here — they live only in the main Ranked Backlog table now (as of the 2026-07-23 groom pass), which is the single source of truth for open work and scores. Keeping duplicate rows with independently-drifting scores in both tables was actively misleading (this table's pre-groom numbers didn't reflect theme decay applied in the main table). Rows for items fully closed out under this V3 theme stay here for narrative continuity only.

### 53. Decouple AST from Go Codegen (IR Abstraction)
* **Description:** Right now, `zero.go` parses an S-expression and immediately spits out a Go string. We need to introduce a middle layer (an IR graph) so we can support multiple backends.
* **Impact:** 9/10 (Critical unblocker for pure binary generation).
* **Done (2026-07-24) — Phase 1:** agy unavailable this session (weekly quota) and local Ollama not installed, so implemented directly. Re-scoped on re-evaluation: a full IR covering every node kind isn't meaningful for backend-native runtime primitives (`db_connect`, `read_file`, `llm_generate`, `fetch`, `res_json`, DOM primitives, etc.) — these embed target-runtime code with no generic cross-backend semantics, so a "generic IR" for them is just "emit this backend's code." Phase 1 introduced `IRNode`/`lowerShared`/`emitGoIR`/`emitJSIR` in `zero.go` covering the 19 kinds with identical semantics across the Go and JS (#45) backends: `return`, `if`, `while`, `do`, `set`, `match`, `sleep`, `to_int`, `to_float`, `to_string`, `bytes_to_string`, `str_split`, `str_join`, `regex_match`, `append`, `map_set`, `map_delete`, `print`, and the binop group. `let`/`try_let`/`call`/`for`/`spawn` keep per-backend implementations (real divergence: JS's `await`/async threading vs Go's sync + `parse_json`/`env` let-binding special cases). Verified via full byte-diff of every `tests/*.zero`/`examples/*.zero` fixture's generated output against the pre-refactor binary (caught one real bug this way — see `docs/journals/2026-07-24_ir_abstraction.md` — before landing clean), plus `go build`/`go vet`/`go test ./...`. `generateStatementRaw`/`generateJSStatementRaw` combined line count dropped from ~1000 to ~370 for the migrated kinds. Phase 2 (extending the IR to `let`/`try_let`/`call`/`for`/`spawn`) not filed as a separate backlog item — re-open this item or file fresh if a third backend (#54 Wasm) makes it worth pursuing.

### 49. Direct Neural Bytecode Synthesis
* **Description:** Bypass text-based codegen entirely: an LLM emits target bytecode/IR directly from the S-expression AST, no intermediate source-text backend at all.
* **Impact:** 8/10 (Maximum value, but effort-8/"weeks" scale — no concrete design exists yet; #53 Phase 1's `IRNode` is a plausible target representation for this to lower into, but that link hasn't been designed). No detail section existed for this item before the 2026-07-24 groom pass beyond its one-line backlog rationale; added here for consistency with the rest of the backlog's structure.
* **Done (2026-07-24) — Phase 1:** agy unavailable this session (user's weekly Antigravity quota exhausted, per explicit user instruction) and local Ollama not installed, so implemented directly, same as #53 Phase 1. Re-scoped on re-evaluation, following #53's exact precedent: the literal vision ("an LLM emits bytecode directly") depends on a bytecode *format* that doesn't exist yet, so Phase 1 instead proves the item's real underlying claim — bypassing text-based codegen entirely is viable — by adding a tree-walking `Interpret` function to `zero.go` that executes a `cli_app` AST directly (same lexer/parser/`expandIncludes`/`applyPatches` front end, unchanged), with no Go/JS text ever generated and no `go build`/`go run`/`node` subprocess ever invoked. Wired via a new `-run` boolean flag on the existing binary. Full design at `docs/direct_execution_design.md`.
  * **Covered node kinds:** `let`, `set`, `if`, `while`, `do`, `for`, `return`, `defun`/`call`, `print`, the binop set (`+ - * / < > <= >= == != = and or`), `to_int`, `to_float`, `to_string`, `bytes_to_string`, `str_split`, `str_join`, `regex_match`, `append`, `map_set`, `map_delete`, `list`, `dict`, `cli_args`, `sleep`, `env`. Explicitly deferred to Phase 2: `http_server`/`route`/`middleware`, `struct`/`parse_json`/`res_json`, `db_connect`/`sql_query`, `spawn`, `fetch`, `llm_generate`/`fuzzy_cast`/`assert_semantic`, `try_let`, `patch`, `with_context`, `test`, `match`, `exec`, `read_file`/`write_file`/`mkdir` (deferred alongside `try_let`, which they depend on for error handling — no error-tuple convention exists yet without it), `web_app`/JS target.
  * **Value representation:** dynamically typed (`any`) — `int64`/`string`/`bool`/`[]any`/`map[string]any` — since there's no compilation step to satisfy. This directly demonstrates improvement #46's `type_hint`-boilerplate finding in practice: `defun` params need zero `type_hint`s under `-run` versus three lines for a simple two-argument `add` under the Go backend.
  * **Architecture note:** first attempt used a separate `interpreter.go` file, which broke the universal `go run zero.go yourfile.zero` invocation (README/skill docs/every test script) two ways — see the 2026-07-24 journal for the full explanation. Merged directly into `zero.go` instead (now ~2725 lines) so the single-file invocation is unchanged; `-run` is just a new flag on the same binary.
  * **Verification:** `tests/test_interpret_basic.zero` (Go-backend-compatible: explicit `type_hint`s, string-only list/dict elements) produces byte-identical stdout via `./zero -run tests/test_interpret_basic.zero` and the unchanged `./zero tests/test_interpret_basic.zero && go run server.go` path. Full existing `tests/*.zero` suite re-run through the unchanged Go path: zero regressions (only the two pre-existing, unrelated failures — `tests/routes.zero`, an `include`-only fragment, and `tests/test_schema.zero`'s missing `go.sum` entry for `go-sqlite3` — reproduce identically to before this change). `go vet zero.go` and `gofmt -l zero.go` both clean.
  * **Side finding:** empirically re-verified bug #18 (if/while compound/`and`/`or` conditions) is genuinely fixed in current `zero.go`, contradicting a stale "still reproduces" groom note left in its own detail section after the real fix landed — corrected in `bugs.md`.
  * **Phase 2 (done):** a real bytecode serialization format (a versioned instruction list Phase 1's AST-walking `eval` would lower to once instead of re-walking the tree per run — the actual "bytecode" the item's name refers to), plus extending node coverage to `http_server`/`route`, `try_let`, and the I/O-heavy primitives.
  * **Phase 3 (done):** the original literal vision — an LLM emits the Phase 2 bytecode format directly via `orchestrator.py`'s structured-generation grammar, skipping `.zero` source text entirely — not attempted until Phase 2's format exists and is stable.

### 50. Agentic Observability Layer
* **Description:** Full closed-loop observability: correlate #55's telemetry stream and #56's observer agent's anomaly flags into a queryable, AI-consumable view of running application health (not just raw `telemetry.jsonl`).
* **Impact:** 8/10, but ⚠️ below the 0.5 ROI floor (decay 0.25 from 2 already-shipped observability items, #55/#56) — architectural shift, high effort, confirm/re-scope/close with the user before working. No detail section existed for this item before the 2026-07-24 groom pass; added here for consistency.
* **Groomed (2026-07-27):** #59 shipped a crash-triggered source-repair loop, not the queryable telemetry-health view described here, so it does not add another same-theme observability ship. Score remains 0.25.
* **Done (2026-07-29):** Created Python structures (`HealthState`, `ObservabilityLayer`) in `observer.py` to continuously track telemetry anomalies and export a summarized JSON health view. Relevant test fixtures added to `test_observer.py`.

### 51. Ephemeral Neural Circuits
* **Status Note:** Done
* **Description:** Generate a narrowly specialized model or executable reasoning circuit for one task, use it for that task, then discard it.
* **Impact:** 7/10 (Potentially reduces repeated general-model cost, but requires model generation, lifecycle isolation, and safe execution machinery that Zero does not currently have).
* **Done (2026-07-29):** Implemented `ephemeral_circuit` primitive in AST and Bytecode VMs as well as Go backend. It dynamically generates a unique `ephemeral-<uuid>` model via Ollama API using a highly specialized system prompt for the task, evaluates the inputs, generates the output, and deletes the model before returning.


### 52. Automated Counterfactual Debugging
* **Description:** Given a crash dump from #58, have an LLM reason about *what input/state would not have crashed* (counterfactual), not just patch the immediate symptom — the reasoning layer #59's Auto-Patching Loop would call into.
* **Impact:** 8/10, effort-8/"weeks" scale — the self-healing capstone. #59 now provides a concrete integration point, but its bounded whole-source repair prompt does not perform or expose explicit counterfactual reasoning.
* **Done (2026-07-29):** Added a pre-patch reasoning layer in `observer.py`. The crash dump and source are sent to the model to reason about the counterfactual ("what input/state would not have crashed") before generating the patch. This reasoning is injected into the final whole-source repair prompt, completing the automated self-healing loop.

### 54. WebAssembly (Wasm) Backend Prototype
* **Description:** Implement a third code generator that emits standards-compliant WebAssembly Text (`.wat`) from the shared IR, with optional compilation to `.wasm` when a validator/toolchain is available. This proves the IR can support a portable low-level target; WAT is itself human-readable text, so it does not prove text-free code generation.
* **Impact:** 7/10 (A valuable portability and backend-abstraction proof, but not direct bytecode synthesis by itself).
* **2026-07-24 groom note:** #53 Phase 1 shipped `IRNode`/`emitGoIR`/`emitJSIR` covering the 19 control-flow/expression node kinds shared with the Go/JS backends — a real (if partial) head start: a Wasm backend's `emitWasmIR` could reuse that shape for those 19 kinds. The remaining ~30 backend-specific kinds (`let`, `try_let`, `call`, `for`, `spawn`, all I/O/HTTP/LLM primitives) still need full from-scratch Wasm implementations, so effort stays at 7 rather than dropping a full tier. Decay held at 0.5 (from #45) rather than compounding with #53 — #53 was infra/tooling work, not "another backend shipped," so it doesn't independently discount a third backend's value the way a second one already shipping does.
* **Groomed (2026-07-24, later pass):** improvement #60 added `map_get`/`list_get` to the same shared `IRNode`/`emitGoIR`/`emitJSIR` machinery (now 21 shared kinds, not 19) — no rescoring needed, just noting a Wasm backend's head start grew slightly. No other pending item shares this theme; score, effort, and decay all unchanged.
* **Groomed (2026-07-24, later pass):** bug #26's fix added `list`/`dict` construction to the same shared machinery (now 23 shared kinds) — a Wasm backend's `emitWasmIR` head start grows again. Still no rescoring: this is infra reuse, not a shipped backend/theme item, so it doesn't independently discount #54's own value the way a second full backend shipping would.
* **Groomed (2026-07-27):** corrected the stale "bypass human-readable text" claim because `.wat` is textual, and aligned the detail value (7) with the ranked-table calculation. No `wat2wasm`, `wasm-tools`, Wasmtime, or Wasmer executable is installed; Node can host a compiled module but cannot validate WAT directly. The tooling gap and roughly 30 backend-specific node kinds keep effort at 7. With one prior shipped backend (#45), score remains exactly 7×0.5÷7 = 0.50.
* **Done (2026-07-27):** Added the `wasm_app` root and `generateWasmCode`/`emitWasmIR`. It emits a standards-compliant `app.wat` module for a deliberately portable subset: numeric literals, arithmetic/comparisons, `if` with both branches, `do`, and `return`. Unsupported nodes fail with a clear compiler diagnostic instead of being silently miscompiled. `examples/wasm_math.zero` and subprocess tests cover output shape and rejection behavior. `go test ./...`, `go vet`, and the standalone Go fixture compile loop pass. No WAT validator was installed, so `.wasm` compilation/instantiation remains optional external-tool verification rather than a claimed result.
* **Groomed (2026-07-30):** The semantic checker now feeds type/layout metadata into the Wasm backend. Zero `int` results emit as `i64`, floats as `f64`, boolean control-flow values remain `i32`, aggregate values are represented as `i32` pointers, and static int/string lists plus homogeneous static int/string dictionaries have linear-memory data segments and checked read paths. The backend now performs a local structural WAT validation pass; full instruction/type validation still requires `wat2wasm` or `wasm-tools` when available.
* **Groomed (2026-07-30):** #54 is now the second shipped code-generation backend after #45. Current Pending improvement rows score from 1.0 to 3.0, so no open improvement is below the 0.5 ROI floor in this pass.

### 55. Native Telemetry Injection
* **Description:** The transpiler should invisibly inject `observer.Trace(...)` calls at the start and end of every function block, logging variable states.
* **Impact:** 8/10 (Foundation for Agentic Observability).
* **Done (2026-07-23):** Implemented local `observer` package and injected `Trace` hooks into `defun`, `route`, `middleware`, and `spawn` blocks in `zero.go`.

### 56. Standalone Observer Agent
* **Description:** A Python daemon that listens to the telemetry generated by #55 and prompts a local LLM to flag anomalous behavior.
* **Impact:** 8.5/10 (Replaces the human debugger).

### 57. `(neural_circuit)` Runtime Primitive
* **Status Note:** Done
* **Description:** A new primitive where the developer only writes `(neural_circuit (args) "sort list alphabetically")`. At runtime, Zero fetches the logic from an LLM and executes it.
* **Impact:** 7.5/10 (First iteration of ephemeral models).

### 58. Crash-State Serialization
* **Description:** Wrap the generated Go application in a global recovery block. On panic, dump all local variables and call stacks to disk before exiting.
* **Impact:** 8/10 (Allows the AI to see the exact state of the crash without human repro steps).
* **Done (2026-07-23):** Implemented global panic handler in generated Go code that captures stack traces and writes them to `crash.json`. Verified via native `zero_test.go`.

### 59. Auto-Patching Loop
* **Description:** The holy grail of self-healing. When `observer.py` detects a crash dump from #58, it writes a patch to the `.zero` file, runs `go test`, and automatically restarts the service.
* **Impact:** 9/10 (Closes the loop on automated counterfactual debugging).
* **Done (2026-07-27):** Implemented as a bounded, opt-in mode in
  `observer.py`. The operator configures one project root, one `.zero` source,
  one test command, and one restart command. Each new crash dump is parsed as
  JSON and sent with the current source to the local OpenAI-compatible model,
  whose response must be strict JSON containing only a complete replacement
  source. The project is copied without Git or transient runtime artifacts,
  the candidate is tested in that isolated copy, and only a passing candidate
  is atomically installed into the configured live source with its mode
  preserved. Restart runs afterward and is reported separately if it fails.
  All configured commands execute as argument vectors with `shell=False`;
  source and crash paths must resolve inside the project root; the model
  cannot choose commands, paths, or additional files. Eight isolated tests
  cover traversal rejection, root-relative paths, malformed output, non-Zero
  source rejection, failed-test rollback behavior, restart ordering, and a
  real subprocess end-to-end flow. `pytest`, `flake8`, and Python bytecode
  compilation pass.

### 61. Direct Binary Bytecode Serialization
* **Description:** Transitioned the bytecode `.bc.json` serialization output format to `.bc.bin` by utilizing Go's `encoding/gob`. Now the Zero codebase transpiles fully to a binary bytecode representation, leaving JSON text fallback dependencies.
* **Why:** Aligns with the project's goal of bringing Zero into a fully agentic language, not relying on human text, but direct to binary or other machine style. Replaces the legacy human-readable JSON bytecode with fully binary `gob` encoding, reducing parsing overhead.
* **Impact:** 8/10 (High — fulfills the pure binary and machine-agentic goal of the Zero language model workflow).
* **Done (2026-07-29):** Replaced `encoding/json` with `encoding/gob` for `--compile-bc` and `--run-bc` in `zero.go`. Verified serialization of types (like float64 and strings) correctly mapping to binary.

### 64. Semantic Type Checker Pass
* **Description:** Introduce a strict type-inference pass before lowering to IR to define precise byte sizes, alignments, and pointer types.
* **Why:** Necessary for targeting native machine code or LLVM IR, which lack Go's runtime typing.
* **Impact:** 6/10 (Medium-High - Phase 1 shipped the semantic/layout foundation needed before native compilation).
* **Groomed (2026-07-30):** Kept Pending. `docs/journals/2026-07-30_semantic_validation_pass.md` shows a typed value lattice, checker pass, IR metadata propagation, and Wasm layout consumption are in place, but full WAT instruction/type validation and remaining aggregate/dynamic layout decisions are still outstanding. Re-scored to 3.0 (6×1÷2) rather than the original full-gap value.
* **Done (2026-07-30):** Replaced the text-only WAT shape gate with a local parser and instruction/type validator for the backend's emitted folded subset. Aggregate metadata now records dictionary key and value types and rejects incompatible construction, access, and mutation before lowering. The Wasm backend runtime-initializes integer and string list/dictionary expressions, supports dynamic dictionary key expressions through interned string pointers, reads integer dictionary values from memory, allocates independent aligned regions for multiple aggregate literals, grows aggregate table offsets safely, and sizes memory pages from payloads. Installed `wasm-tools` locally, parsed and validated representative generated WAT modules, then instantiated the compiled Wasm through Node and called `main` for numeric and pointer-returning cases.

### 65. Linear SSA-based IR
* **Description:** Lower the high-level tree IR into a flat Static Single Assignment (SSA) format modeling control flow graphs and basic blocks.
* **Why:** Essential step for backend native code generation.
* **Impact:** 8/10 (High).
* **Groomed (2026-07-30):** Still Pending and still a distinct compiler architecture step after #64; no same-theme shipment closes SSA/CFG lowering. Score remains 2.0 (8×1÷4), with effort unchanged because it cuts across control flow and backend contracts.

### 66. Standalone Zero Runtime Environment
* **Description:** Implement a lightweight runtime library (in C, Zig, or Rust) to handle memory management, GC, and OS syscalls natively.
* **Why:** Zero currently depends entirely on Go's runtime. We must shed this dependency to emit true standalone binaries.
* **Impact:** 8/10 (High).
* **Groomed (2026-07-30):** Still Pending and above the ROI floor, but ranked last among current Pending improvements because it is the largest remaining implementation slice. Score remains 1.0 (8×1÷8). Architecture options should be evaluated explicitly before implementation: C is smallest and ABI-stable but puts memory safety burden on Zero; Rust is safer but adds toolchain/runtime complexity; Zig is a pragmatic systems-language fit but smaller ecosystem.

### 67. Native Backend Code Generators
* **Description:** Implement LLVM IR or Wasm backend serializers to replace textual Go string concatenations.
* **Why:** Achieving the core goal of Zero as a direct-to-machine-code language.
* **Impact:** 6/10 (Medium-High - the Wasm prototype lowered the remaining value, but production-native codegen is still a major milestone).
* **Groomed (2026-07-30):** Still Pending after #54's Wasm prototype and #64's typed metadata work. Value is decayed/rescoped to 6 because a partial Wasm backend already exists; effort remains 4 until LLVM/native serializers and stronger validators are chosen. Score remains 1.5 (6×1÷4). Technology tradeoff: LLVM gives mature optimization and tooling at the cost of a large dependency surface; direct Wasm is simpler and sandbox-friendly but less native; a custom emitter maximizes control but is the riskiest path.

### 68. Native Logit Masking
* **Description:** Allow Zero types to natively compile into inference-level logit masks to restrict LLM generation space.
* **Why:** Strongly inspired by LMQL to eliminate syntax hallucination at the token-generation level.
* **Impact:** 5/10 (Medium - valuable for AI authoring reliability, but it depends on type/checker maturity and model-inference integration).
* **Groomed (2026-07-30):** Still Pending and now ranked behind #64 because typed constraints need the checker foundation first. Score remains 2.5 (5×1÷2). Likely approaches: grammar/logit masks are direct and fast but syntax-focused; type-derived masks are stronger but more complex; external constrained decoding libraries reduce effort but add integration coupling.
* **Done (2026-07-30):** Added a provider-neutral Go masking package that compiles individual semantic types and complete checker analyses into deterministic JSON plans. Plans cover primitive token classes, collection delimiters and member constraints, normalized struct fields, and function parameter and return constraints. Provider-specific tokenizer and live inference integration remain deliberately outside this item.

### 69. First-Class Optimization Signatures
* **Description:** Built-in "Teleprompter" step that runs tests and automatically optimizes embedded logic/prompts during compilation.
* **Why:** Inspired by DSPy to automate prompt engineering within the compilation loop.
* **Impact:** 6/10 (Medium - advanced agentic feature).
* **Groomed (2026-07-30):** Still Pending. The shipped `optimize_block` runtime primitive (#40) handles runtime hot-swapping, not compile-time prompt/program optimization signatures, so it does not close this item. Score remains 1.5 (6×1÷4).

### 70. Type-Safe Schema Bridges
* **Description:** Force LLM outputs into strongly typed interfaces via semantic type checking rather than loose JSON mapping.
* **Why:** Inspired by BAML for robust schema extraction.
* **Impact:** 5/10 (Medium - improves reliability of agent outputs, but it builds on the semantic checker and overlaps with existing JSON/struct primitives).
* **Groomed (2026-07-30):** Still Pending and tied with #68 at 2.5 (5×1÷2). This remains a new-capability curve for LLM output boundaries, but current value is lower than the original broad framing because `struct`, `parse_json`, and checker metadata already cover part of the schema story. Design options: BAML-style schema compiler gives strong guarantees but may be larger than Zero needs; JSON Schema interop is familiar but looser; native Zero schema forms keep the grammar small but require more compiler work.
