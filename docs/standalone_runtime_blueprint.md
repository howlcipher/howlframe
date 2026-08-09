# HowlFrame Standalone Runtime Blueprint

## Status and authority

This document defines the architectural direction for making HowlFrame useful as an independent programming language and runtime rather than primarily as a source-to-source transpiler.

It is a **design blueprint, not a second backlog**.

- `improvements.md` remains the authoritative tracker for planned work.
- `bugs.md` remains the authoritative tracker for confirmed defects.
- Every implementation phase described here must be represented by a numbered improvement or bug before coding begins.
- Status, ranking, model routing, dependencies, and completion history belong in those tracking files, not here.

This blueprint complements [the architecture roadmap](architecture_roadmap.md):

- HFIR remains HowlFrame's canonical semantic representation.
- The HowlFrame bytecode VM becomes the first complete standalone application runtime.
- WebAssembly remains the first portable native artifact target.
- Go and JavaScript remain valuable compatibility and deployment backends, but they do not define HowlFrame's semantics.

## Product decision

HowlFrame should become a language that can build, validate, test, package, and run useful applications without generating Go, JavaScript, or WAT as an intermediate product step.

The first complete standalone category should be **command-line applications**.

The target execution path is:

```text
HowlFrame source
-> parser and compatibility frontend
-> semantic checker
-> semantic HFIR
-> HFIR verifier
-> lowered HFIR
-> HowlFrame bytecode
-> HowlFrame VM
-> structured execution result
```

A source-level convenience command may perform several of these stages in memory, but it must not silently fall back to generated Go or another transpiler backend.

Once this standalone foundation is trustworthy, HowlFrame should evolve beyond ordinary application development into a **verified AI-native runtime**: deterministic HowlFrame code owns state, permissions, invariants, and execution boundaries, while bounded probabilistic components can reason, plan, synthesize, and propose semantic changes under explicit verification and capability controls.

The long-term goal is not “a normal language with LLM API calls.” It is software whose adaptive behavior is a first-class, inspectable part of the language/runtime contract.

## What "standalone" means

A HowlFrame application counts as standalone when all of the following are true:

1. It can be built and run using the HowlFrame toolchain and HowlFrame runtime.
2. Running it does not require a Go compiler, Node.js, Python, or a target-language build tool.
3. It does not generate human-language source files as a required intermediate step.
4. Its behavior is defined by HowlFrame semantics rather than accidental Go or JavaScript behavior.
5. Unsupported constructs fail before execution with structured diagnostics.
6. Effects such as filesystem, network, environment, process, and database access are explicit and policy-controlled.
7. The resulting artifact has a versioned compatibility and validation envelope.
8. Source locations, runtime errors, tests, and permissions remain inspectable by humans and AI agents.

An application may still intentionally require an external service, database, model endpoint, or operating-system facility when its declared capability policy allows it. That does not make the language itself a transpiler.

## Current implementation position

As of August 5, 2026, HowlFrame has crossed the line from being only a transpiler into being an experimental language runtime:

- `-run` directly interprets a bounded `cli_app` subset.
- `-compile-bc` produces versioned HowlFrame bytecode.
- `-run-bc` executes bytecode through the HowlFrame VM.
- The VM supports control flow, functions, collections, file and process operations, HTTP operations, database operations, AI-oriented operations, and VM-native stores with varying maturity.
- Runtime capabilities are deny-by-default for protected bytecode instructions.
- HFIR verification is wired into source-based compiler paths.

The standalone path is not yet the broadest or most polished route. Generated Go remains the most complete backend, and direct AST execution and bytecode execution still have bounded or uneven coverage.

The goal of this blueprint is to remove that qualifier for a focused application class before expanding further.

## Runtime ownership decisions

### HFIR owns semantic meaning

HowlFrame semantics must be defined before backend or VM implementation.

For every portable construct:

- HFIR represents its meaning.
- The verifier checks its invariants, effects, capabilities, and target feasibility.
- The bytecode compiler lowers it without changing its meaning.
- The VM implements the defined behavior.
- Other backends either conform or reject it explicitly.

The bytecode VM must not become a second, competing semantic language.

### The bytecode VM is the primary standalone runtime

The bytecode VM should become the product path for standalone CLI applications because it already provides:

- a HowlFrame-owned instruction set,
- runtime capability enforcement,
- a compact execution representation,
- functions and control flow,
- host operations,
- and a direct route from verified HowlFrame programs to execution.

The tree-walking AST interpreter should remain useful for:

- compiler debugging,
- semantic reference tests,
- very small development runs,
- and differential execution checks.

It should not remain the long-term primary application runtime because it duplicates semantics and has a narrower feature surface.

### WebAssembly remains the first portable native artifact

The VM-first plan does not replace the Wasm direction.

The responsibilities are different:

- **HowlFrame VM:** first complete and controllable application runtime.
- **Wasm:** first portable native deployment artifact with a stable host ABI.
- **Go and JavaScript:** compatibility, interoperability, and deployment targets.

## Desired user workflow

The exact command names must follow the final CLI design, but the intended experience is:

```bash
howlframe new cli invoice-report
howlframe run invoice-report
howlframe test invoice-report
howlframe build invoice-report -o dist/invoice-report.zapp
howlframe exec dist/invoice-report.zapp
```

The source-level run command should conceptually perform:

```text
parse -> check -> HFIR -> verify -> lower -> bytecode -> execute
```

without writing intermediate source files.

Advanced users should be able to inspect each stage:

```bash
howlframe inspect app.howl --emit hfir
howlframe inspect app.howl --emit lowered-hfir
howlframe inspect app.howl --emit bytecode
howlframe capabilities app.howl
howlframe validate app.howl --json
```

## Standalone CLI v1 definition

HowlFrame should not claim a complete standalone CLI runtime until one coherent application lifecycle satisfies all of the following.

### Language and runtime surface

Standalone CLI v1 must support:

- lexical variables and mutation,
- conditions and loops,
- functions, recursion, and returns,
- strings, integers, floats, and booleans,
- lists and dictionaries,
- deterministic equality and collection behavior,
- command-line arguments,
- standard input and line-oriented input,
- standard output and standard error,
- explicit process exit codes,
- file reads, writes, directory creation, and path operations,
- JSON parsing and serialization,
- deterministic number and string conversion,
- time and deterministic or seedable randomness,
- structured error values and catch/recovery behavior,
- modules through the supported `use`/`export` contract,
- unit and workflow tests,
- and capability declarations for every effectful operation.

HTTP clients, subprocesses, environment access, native stores, databases, and model calls may be included when their behavior and capability policies are sufficiently tested, but they must not block the minimum deterministic CLI milestone.

### Tooling lifecycle

Standalone CLI v1 must provide a coherent workflow for:

- project creation,
- source validation,
- formatting,
- test execution,
- development runs,
- release builds,
- artifact inspection,
- dependency or module validation,
- and runtime execution.

A user should not need to know which internal engine is selected for normal usage.

### Runtime artifact

The standalone artifact must include or reference:

- a bytecode format version,
- a language version,
- a compiler/runtime compatibility range,
- a deterministic program hash,
- required capabilities,
- module and dependency metadata,
- source/provenance mappings,
- and verification status.

The current Go-coupled bytecode serialization may remain an internal development format while this envelope is designed, but it must not be presented as a durable distribution contract until compatibility and validation are explicit.

### Failure behavior

The runtime must fail predictably with:

- stable error codes,
- structured machine-readable details,
- human-readable messages,
- HowlFrame source locations when available,
- nonzero exit codes,
- and no partial artifacts or hidden transpiler fallback.

Unsupported bytecode versions, missing capabilities, corrupt artifacts, invalid modules, and target-infeasible constructs must fail closed.

## Reference applications

Standalone readiness should be proven through complete reference applications, not only isolated opcode tests.

### 1. File report tool

A CLI that:

- accepts an input directory and output file,
- reads multiple files,
- parses structured data,
- aggregates results,
- reports malformed inputs without losing valid work,
- and writes a deterministic report.

This proves arguments, files, JSON, collections, errors, modules, tests, and filesystem permissions.

### 2. JSON transformation CLI

A CLI that:

- reads JSON from standard input or a file,
- validates required fields,
- transforms records,
- writes JSON to standard output,
- and returns meaningful exit codes.

This proves stdin, stdout, JSON, schema-oriented validation, deterministic output, and Unix-style composition.

### 3. Network status checker

A CLI that:

- reads a list of allowed endpoints,
- performs HTTP requests only with network permission,
- records structured results,
- and handles timeouts and failures.

This proves scoped network effects and structured runtime errors.

### 4. Terminal game

A small turn-based or guessing game that:

- reads interactive input,
- uses seedable randomness,
- maintains state,
- saves and loads progress,
- and can be tested deterministically.

This proves that the runtime is usable for interactive programs, not only batch transformations.

Each reference application must run without generated Go, JavaScript, or WAT.

## Milestones

These milestones describe sequencing, not backlog status. Before implementation, search `improvements.md` and `bugs.md`, reuse existing entries, and add or split numbered work only where necessary.

### Milestone 0: Runtime coverage inventory

Create one generated or test-enforced matrix that records, for every language construct:

- semantic-checker support,
- HFIR representation,
- HFIR verification,
- bytecode lowering,
- VM execution,
- direct interpreter execution,
- required capability,
- and test evidence.

No feature should be described as standalone-supported merely because an opcode or AST branch exists.

**Exit gate:** the repository can identify every gap between accepted HowlFrame source and VM-executable behavior without manual code archaeology.

### Milestone 1: Deterministic CLI core

Complete and align:

- variables,
- control flow,
- functions,
- strings,
- numbers,
- collections,
- command-line arguments,
- stdin/stdout/stderr,
- exit codes,
- time,
- seedable randomness,
- and deterministic conversions.

**Exit gate:** deterministic CLI programs run through the VM with no generated source and pass differential tests against the defined HowlFrame semantics.

### Milestone 2: Errors, files, and modules

Complete:

- structured runtime errors,
- source-mapped stack traces,
- error recovery,
- filesystem operations,
- path handling,
- JSON serialization,
- and reliable `use`/`export` behavior.

**Exit gate:** the file-report and JSON-transform reference applications pass end to end.

### Milestone 3: Native test runner

Make HowlFrame `test` blocks executable through the standalone runtime rather than only generating Go or Node tests.

Support:

- unit tests,
- workflow tests,
- deterministic fixtures,
- capability-aware test policies,
- structured test results,
- and CI-friendly exit codes.

**Exit gate:** the standalone reference applications are tested without generating another language's test files.

### Milestone 4: Hardened artifact format

Define a durable package or executable envelope around lowered HFIR and bytecode.

Include:

- explicit schema versions,
- validation before execution,
- deterministic serialization,
- hashes,
- capability manifest,
- module metadata,
- source map,
- compatibility checks,
- and reproducible build tests.

**Exit gate:** the same source produces byte-identical artifacts under the same declared toolchain and configuration, or the repository documents every intentional nondeterministic field.

### Milestone 5: Project and package lifecycle

Provide a minimal project format for:

- project metadata,
- entry point,
- language/runtime version constraints,
- module paths,
- declared capabilities,
- test configuration,
- and build profiles.

Do not begin with a public package registry. Local and repository modules should become reliable first.

**Exit gate:** a new user can create, run, test, build, and execute a multi-file CLI application through documented commands.

### Milestone 6: Scoped capability security

Advance from broad capability names to enforceable scopes where practical:

- filesystem paths,
- network hosts,
- environment-variable names,
- process execution rules,
- database handles,
- and model endpoints.

Distinguish:

- static capability inference,
- user approval,
- VM enforcement,
- operating-system isolation,
- and audit reporting.

Do not describe the VM as sandboxed unless a real isolation boundary exists.

**Exit gate:** the network checker and file-report applications can be granted only the resources they require, and attempts to exceed the grant fail closed.

### Milestone 7: Debugging and observability

Add standalone-runtime support for:

- source-mapped stack traces,
- structured execution results,
- effect summaries,
- instruction and resource limits,
- trace points,
- deterministic replay data where feasible,
- and developer inspection commands.

**Exit gate:** a developer or coding agent can diagnose a failed VM application without reading generated Go or VM implementation code.

### Milestone 8: Installation and distribution

Provide:

- versioned HowlFrame compiler/runtime releases,
- cross-platform release binaries,
- checksums,
- installation instructions,
- upgrade and compatibility guidance,
- and a clean application distribution story.

A first application distribution may require the HowlFrame runtime to be installed. A later mode may embed the runtime into a self-contained executable.

**Exit gate:** a user can install HowlFrame and run a packaged HowlFrame CLI application on a supported platform without installing Go.

### Milestone 9: Expand beyond CLI

After standalone CLI v1 is complete, expand deliberately.

Recommended order:

1. long-running jobs and schedulers,
2. HTTP services,
3. database-backed applications,
4. terminal applications and text games,
5. browser and full-stack application frameworks,
6. browser-based 2D games,
7. native 2D adapters,
8. gameplay scripting for larger engines.

Do not use an incomplete web or game runtime as proof that the standalone core is mature.

## AI-native evolution after the standalone foundation

The standalone runtime is the trust boundary that makes HowlFrame's more unusual AI-first direction credible.

HowlFrame should not become:

```text
prompt
-> magic
-> hope
```

The intended model is:

```text
AI freedom
inside
machine-verifiable boundaries
```

A concise product thesis is:

> **HowlFrame is a verified runtime for adaptive, agentic software where deterministic code surrounds bounded probabilistic reasoning.**

That is materially different from adding an `llm_generate()` function to an otherwise conventional language.

### Deterministic and adaptive regions

HowlFrame should make a clear semantic distinction between deterministic behavior and probabilistic/adaptive behavior.

Deterministic regions should own operations where correctness and authority must not depend on a model, including:

- authentication and authorization,
- money and billing,
- persistent state transitions,
- capability grants,
- filesystem mutation,
- process execution,
- database writes,
- module loading,
- invariants,
- validation,
- test decisions,
- artifact verification,
- and final approval of semantic changes.

Adaptive regions may perform bounded tasks such as:

- classification,
- summarization,
- planning,
- ranking,
- content generation,
- tool selection,
- schema interpretation,
- repair proposals,
- workflow adaptation,
- and strategy generation.

An adaptive operation must never implicitly gain authority merely because a model requested it.

### AI operations are explicit effects

AI-backed behavior should be represented as an explicit effect in HFIR and the runtime, not as an ordinary pure function call.

A model-backed operation should be able to declare or infer:

- provider-neutral model requirements,
- input and output schema,
- data visibility,
- allowed tools,
- runtime capabilities,
- token or cost budget,
- latency or attempt budget,
- confidence requirements,
- fallback behavior,
- deterministic test doubles,
- replay/provenance metadata,
- and whether its output may influence state or only propose a change.

The verifier should be able to reason about these facts before execution.

Validation itself must never require an external model call.

### Intent-to-execution architecture

The longer-term AI-native path should look conceptually like:

```text
human or agent intent
        |
        v
provider-neutral model planning
        |
        v
typed HFIR proposal
        |
        v
HFIR verification
        |
        v
capability and budget policy
        |
        v
bounded execution
        |
        v
tests, invariants, and evidence
        |
        +---- success ----> structured result + provenance
        |
        +---- failure ----> bounded semantic repair context
                                |
                                v
                          model-proposed HFIR delta
                                |
                                v
                          verify -> test -> approve
```

A model should not need to regenerate an entire source file when only one semantic region is invalid.

### AI-native application model

A future high-level application description may express ideas such as intent, capabilities, budgets, verification, and repair policy.

The following is **illustrative product syntax, not a committed language design**:

```lisp
(agent_app
  (intent "Keep my project dependencies healthy")

  (capabilities
    (network "github.com")
    (file_write "./go.mod" "./go.sum"))

  (budget
    (model_calls 10)
    (tokens 50000))

  (goal
    "Detect outdated dependencies and prepare safe upgrades")

  (verify
    (test "go test ./...")
    (test "go vet ./..."))

  (on_failure
    (repair
      (scope changed_nodes)
      (max_attempts 3))))
```

The important idea is not these exact forms. The architectural requirements are:

- intent is inspectable,
- permissions are explicit,
- AI work has budgets,
- model outputs are typed or schema-constrained,
- tests and invariants are first-class evidence,
- repairs are localized,
- and execution authority remains with the deterministic runtime.

Do not add this surface syntax until the underlying HFIR, capability, verification, and model-adapter contracts justify it.

### Bounded semantic self-repair

One of HowlFrame's most differentiated long-term capabilities could be safe self-adaptation.

The desired flow is:

```text
running HowlFrame application
        |
        v
detect failed invariant or incompatible input
        |
        v
produce bounded repair context
        |
        v
model proposes semantic HFIR delta
        |
        v
verify graph + types + effects + capabilities
        |
        v
run required tests
        |
        v
policy approves or rejects
        |
        v
atomically apply approved delta
        |
        v
continue, restart, or roll back
```

This is **not** permission for an application to arbitrarily rewrite itself.

A valid semantic repair system must enforce:

- bounded editable graph regions,
- stable node identities,
- minimal deltas rather than whole-program regeneration,
- no capability escalation through a repair,
- no hidden modification of tests or invariants used to approve the repair,
- deterministic validation,
- explicit attempt limits,
- provenance for the model/provider/input that proposed the change,
- pre-apply tests,
- atomic application,
- rollback,
- and human approval when policy requires it.

The runtime and verifier, not the model, decide whether a proposed repair is legal.

### Learning and adaptation architecture

HowlFrame should treat "learning from itself" as a progression of increasingly powerful state changes rather than one undifferentiated self-modification feature.

The recommended model has four levels:

| Level | What changes | Typical evidence | Default autonomy | Risk |
| --- | --- | --- | --- | --- |
| Experience memory | Stored observations, successes, failures, corrections, and examples | execution records, user feedback, test results | automatic within retention/privacy policy | Low |
| Policy adaptation | Strategy selection, ranking, routing, thresholds, and fallback order | comparative success/cost/latency metrics | automatic when bounded by declared policy | Low-Medium |
| Verified skills | Reusable behavior synthesized from repeated successful workflows | fixtures, tests, capability manifest, provenance | requires verification and promotion gate | Medium |
| Semantic program adaptation | HFIR nodes/edges and executable application behavior | verifier, regression suite, shadow results, rollback point | tightly scoped; policy or human approval | High |

This hierarchy should be explicit in the runtime and documentation. A system should not need permission to rewrite its program merely to remember that one strategy worked better than another.

#### Separate program, memory, policy, and skills

Adaptive state should not be stored as an opaque mutation of application source.

Conceptually, a HowlFrame application should maintain separate versioned domains:

```text
program
  verified semantic HFIR and immutable/protected regions

memory
  observations, examples, outcomes, failures, feedback, and retrieved context

policy
  strategy preferences, routing rules, thresholds, budgets, and promotion criteria

skills
  reusable verified semantic behaviors with declared inputs, outputs, effects, tests, and provenance
```

This separation provides several benefits:

- memory can evolve without changing executable semantics,
- policy changes can be compared and rolled back independently,
- skills can be tested and promoted before becoming callable,
- program changes remain exceptional and highly governed,
- provenance can explain why behavior changed,
- and a damaged or poisoned memory store cannot automatically rewrite protected program logic.

Each domain should have its own versioning, retention, validation, and rollback rules.

#### Level 1: experience memory

The first useful form of learning should be persistent, structured experience.

A runtime execution may record facts such as:

```text
task class
bounded input/context fingerprint
strategy or skill used
tools invoked
capabilities exercised
result
structured error
tests passed/failed
latency
token/model cost
confidence
user correction or approval
program/policy/skill version
```

Future executions may retrieve relevant prior experience as bounded context.

The architecture should support both positive and negative evidence.

HowlFrame should remember:

- successful strategies,
- failed attempts,
- rejected repairs,
- capability denials,
- regressions,
- expensive approaches,
- user corrections,
- stale assumptions,
- and cases where uncertainty was correctly escalated.

A repeated failure is often more useful than another successful example because it lets the system avoid known-bad actions.

Memory retrieval must remain bounded and inspectable. The model should not receive an unbounded transcript of historical execution.

Memory should be subject to:

- application-defined retention,
- privacy/redaction rules,
- provenance,
- relevance scoring,
- deterministic test fixtures,
- and limits on how much retrieved memory can influence one model operation.

Long-term weight updates or automatic model fine-tuning are **not required** for this layer. HowlFrame's first learning system should improve behavior through explicit runtime state that can be inspected, tested, deleted, and rolled back.

#### Level 2: policy adaptation

The next level should let HowlFrame improve **which verified strategy it selects** without changing the strategy implementation itself.

For example:

```text
task class: vendor_invoice_parse

strategy A
success: 73%
median latency: 420 ms
cost: low

strategy B
success: 94%
median latency: 610 ms
cost: medium

strategy C
success: 88%
median latency: 380 ms
cost: low
```

A policy layer may learn:

```text
prefer B for ambiguous invoices
prefer C for known schema v2
fallback to B after C fails validation
retire A after minimum evidence threshold
```

Policy adaptation should operate only inside declared bounds.

A policy update must not:

- grant new runtime capabilities,
- widen editable HFIR regions,
- change protected invariants,
- modify the tests used to approve itself,
- exceed application budgets,
- or invent an unverified strategy.

Useful policy dimensions include:

- model/provider routing,
- tool selection,
- skill selection,
- fallback ordering,
- confidence thresholds,
- retry limits,
- context selection,
- latency/cost tradeoffs,
- and escalation-to-human thresholds.

Policy updates should be versioned and measured against a baseline so the runtime can automatically revert a policy that performs worse.

#### Level 3: verified skill creation

HowlFrame should eventually be able to recognize repeated successful workflows and propose them as reusable skills.

The lifecycle is:

```text
repeated executions
        |
        v
detect recurring semantic pattern
        |
        v
propose typed skill contract
        |
        v
synthesize HFIR implementation
        |
        v
infer effects and capabilities
        |
        v
build fixtures from prior examples
        |
        v
verify + test
        |
        v
shadow against existing behavior
        |
        v
promote into versioned skill registry
```

A verified skill should include:

- stable identifier and version,
- typed inputs and outputs,
- semantic HFIR body or reference,
- declared/inferred effects,
- required capabilities,
- deterministic fixtures,
- tests and invariants,
- provenance for the evidence used to create it,
- model/provider metadata for the proposal,
- success, cost, and latency metrics,
- promotion history,
- deprecation status,
- and rollback predecessor.

Illustrative metadata:

```text
skill: parse_acme_invoice
version: 4
derived_from_executions: 27
baseline_success: 0.68
candidate_success: 0.96
tests: 43/43
capabilities: [filesystem-read]
promoted_at: 2026-08-07
previous_version: 3
rollback_available: true
```

A skill should not become trusted merely because it was generated from many examples. It must pass the same semantic and capability boundaries as hand-authored behavior.

#### Level 4: bounded semantic program adaptation

Only after memory, policies, and verified skills are insufficient should HowlFrame modify the program's semantic graph.

Programs should be able to distinguish regions conceptually equivalent to:

```text
PROTECTED
authentication
authorization
billing
capability policy
approval rules
verification rules
artifact trust roots

ADAPTIVE
classifiers
parsers
ranking strategies
workflow ordering
prompt/program templates
tool-selection logic
domain heuristics
```

Protected should be the default.

An adaptive region must declare:

- which nodes may be changed,
- which interfaces must remain stable,
- which capabilities may be retained,
- required tests/invariants,
- allowed repair attempts,
- whether shadow execution is required,
- promotion policy,
- and rollback policy.

A model must never be allowed to turn a protected region into an adaptive one as part of the change it is proposing.

#### Adaptation lifecycle

All adaptive behavior beyond simple memory should converge on one first-class lifecycle:

```text
OBSERVE
   |
   v
REMEMBER
   |
   v
REFLECT
   |
   v
PROPOSE
   |
   v
VERIFY
   |
   v
TEST
   |
   v
SHADOW / SIMULATE
   |
   v
PROMOTE
   |
   v
MONITOR
   |
   +---- improvement sustained ----> RETAIN
   |
   +---- regression detected ------> ROLLBACK
```

The lifecycle should produce structured evidence at every transition.

"Reflect" and "propose" may use a model. "Verify," policy authorization, promotion gates, and rollback authority must remain deterministic runtime responsibilities.

#### Evidence-driven promotion

Every learned behavior should carry evidence rather than only a confidence score.

Promotion evidence may include:

- number and diversity of executions,
- held-out fixtures,
- regression-suite results,
- success-rate delta,
- cost delta,
- latency delta,
- capability delta,
- failure modes,
- shadow execution results,
- human feedback,
- and statistical confidence appropriate to the domain.

The default promotion rules should become stricter as the adaptation level increases:

```text
memory update
  -> automatic under retention/privacy policy

policy update
  -> automatic only within declared bounds and minimum evidence

skill creation/promotion
  -> verifier + tests + shadow evidence

semantic HFIR modification
  -> verifier + full required tests + policy gate + rollback point

security/capability/approval-rule modification
  -> human approval by default
```

HowlFrame should make these promotion rules explicit rather than bury them inside agent prompts.

#### Immutable approval evidence

A self-adapting system must not be allowed to win by changing the definition of success.

When evaluating an adaptation, the candidate must not be permitted to modify:

- the tests that approve that candidate,
- protected invariants,
- capability ceilings,
- evaluation datasets designated immutable,
- rollback metadata,
- audit history,
- or promotion thresholds governing that candidate.

Changes to those artifacts are separate governed changes evaluated under the previous trusted policy.

This should become a verifier/runtime invariant.

#### Shadow execution before promotion

Where side effects permit it, adaptive changes should be evaluated in shadow mode before becoming authoritative.

Shadow execution may:

- receive the same deterministic input,
- run candidate and current behavior,
- compare structured outputs,
- block or stub external side effects,
- measure cost and latency,
- and record disagreement.

For irreversible or external operations, use test doubles, simulation, or explicit human review instead of pretending a perfect shadow environment exists.

The runtime must never duplicate real money movement, destructive filesystem actions, outbound communications, or other irreversible side effects merely to compare candidates.

#### Learning from failure

A failed adaptation should produce reusable evidence.

A rejected proposal should record:

```text
proposal identity
bounded context
candidate delta or skill
verification failures
tests failed
capability violations
runtime errors
performance regression
human rejection reason, when supplied
```

Future models may retrieve this record to avoid repeating an already-known failure.

Repeated proposals that are semantically equivalent to a previously rejected change should be detectable where practical through normalized semantic hashes rather than only source-text comparison.

#### Creation as a first-class behavior

"Learning" should include the ability to create new useful semantic structures, not only tune existing parameters.

Given sufficient evidence and policy permission, HowlFrame may eventually propose:

- a new parser,
- a new reusable skill,
- a new workflow branch,
- a new tool adapter,
- a new validation rule,
- a new test fixture,
- a new strategy,
- or a new bounded subagent role.

Creation follows the same trust path:

```text
need detected
-> candidate created
-> typed contract
-> capability inference
-> verification
-> tests
-> shadow evidence
-> promotion
```

Creation must not imply authority. A newly created tool or skill has no capabilities beyond those explicitly granted by policy.

#### Adaptation provenance

Every promoted adaptive artifact should be answerable to questions such as:

- What changed?
- Why was it proposed?
- Which observations influenced it?
- Which model/provider produced the proposal?
- Which program/policy version generated the evidence?
- Which tests approved it?
- What capabilities changed?
- What performance improved or regressed?
- Who or what authorized promotion?
- What version can it roll back to?

This provenance should be machine-readable so humans and agents can audit adaptation without reconstructing it from logs.

#### Runtime architecture for learned state

The preferred high-level relationship is:

```text
                  +----------------------+
                  | verified program HFIR |
                  +----------+-----------+
                             |
                             v
+-------------+       +------+-------+       +----------------+
| experience  | ----> | policy/skill | ----> | runtime choice |
| memory      |       | selection    |       | + execution    |
+-------------+       +------+-------+       +-------+--------+
                             |                       |
                             | insufficient          | outcome
                             v                       v
                      +------+-----------------------+--+
                      | bounded adaptation proposal     |
                      +---------------+-----------------+
                                      |
                                      v
                               verify / test
                                      |
                              +-------+-------+
                              |               |
                            reject          promote
                              |               |
                              v               v
                         failure memory   versioned state
```

This preserves a deterministic ownership boundary even while behavior evolves over time.

#### What HowlFrame should not call learning

Avoid using "learning" as a label for behavior that is merely:

- adding a longer prompt,
- appending the entire conversation to context,
- blindly retrying a model,
- rewriting a file after a failure,
- storing opaque model text with no provenance,
- or selecting a different model without measured evidence.

A HowlFrame learning mechanism should produce durable, inspectable, versioned state and measurable behavioral change.

#### Learning/adaptation completion gates

Do not claim that HowlFrame applications "learn from themselves" until at least the first two levels are real and measured.

**Learning v1 — experience and policy:**

1. Executions produce structured experience records.
2. Success and failure evidence are both retained.
3. Relevant memory can be retrieved with explicit context bounds.
4. Policies can select among already-verified strategies using measured evidence.
5. Policy versions and metrics are inspectable and rollback-capable.
6. Memory or policy updates cannot grant capabilities or alter protected invariants.
7. Deterministic fixtures can reproduce adaptation decisions in CI.

**Learning v2 — skill synthesis:**

1. Repeated workflows can produce a candidate typed skill.
2. Candidate skills lower to verified semantic HFIR.
3. Capabilities/effects are inferred before promotion.
4. Tests are generated or assembled from independent evidence and cannot be rewritten by the candidate.
5. Shadow or equivalent evaluation compares candidate and current behavior.
6. Promotion and rollback are versioned and auditable.

**Learning v3 — semantic adaptation:**

1. Programs define protected and adaptive regions.
2. Model proposals are minimal HFIR deltas scoped to adaptive regions.
3. The candidate cannot change its own capability ceiling or approval evidence.
4. Full required verification and regression tests run before promotion.
5. Promotion is atomic.
6. Rollback restores the last verified state.
7. Security, trust-root, capability-policy, and approval-rule changes require human approval by default.

### Examples beyond ordinary applications

Once the verified AI-native layer exists, HowlFrame could support application categories that are awkward to express safely in conventional languages:

- **Self-adapting integrations:** detect an upstream API/schema change, propose a new mapping, test it against fixtures, and adopt it only after verification.
- **Adaptive data pipelines:** synthesize or revise parsers for previously unseen structured inputs while preserving output invariants.
- **Operations agents:** inspect service state, propose bounded remediations, and execute only capability-approved actions.
- **Research systems:** generate analysis procedures, run them against controlled datasets, compare results, and retain only verified procedures.
- **Agentic business workflows:** learn new exception-handling paths without bypassing approval, authorization, or accounting rules.
- **Adaptive games and simulations:** generate NPC strategies, quests, dialogue, or scenario logic inside deterministic game-state and capability boundaries.
- **Developer tools:** diagnose failures, propose semantic patches, run tests, and return reviewable evidence rather than directly rewriting arbitrary source.
- **Long-lived software agents:** evolve workflow logic over time while retaining versioned state, budgets, invariants, provenance, and rollback.

These should be treated as evidence for the architecture, not as reasons to add isolated AI-themed syntax prematurely.

### AI-native v1 completion gate

HowlFrame should not claim a complete AI-native runtime until at least the following are true:

1. Model integration is provider-neutral at the semantic contract.
2. AI operations are explicit HFIR effects.
3. Model inputs and outputs can be constrained by stable schemas.
4. Every model operation declares or receives a capability and budget policy.
5. Model output cannot directly bypass the verifier to mutate protected state.
6. Semantic repair works through bounded deltas with stable node identities.
7. Repair cannot silently widen capabilities or weaken its own approval tests.
8. Tests and invariants run before an adaptive change is accepted.
9. Model-dependent execution produces inspectable provenance and structured results.
10. Validation, compilation, and artifact verification work without contacting an external model.
11. Deterministic test doubles make AI-dependent workflows reproducible in CI.
12. A failed adaptive change can be rejected or rolled back without corrupting the last verified program state.
13. Experience memory stores successes, failures, corrections, and provenance as structured versioned state.
14. Policy adaptation can improve strategy selection without changing executable semantics or widening capabilities.
15. Newly synthesized skills require typed contracts, verification, tests, evidence, and promotion before becoming callable.
16. Semantic adaptation is restricted to declared adaptive regions; protected trust, security, and approval regions remain immutable to the proposing candidate.

### Sequencing with the standalone roadmap

Do not jump directly to self-repair or broad agent syntax while the core runtime is still incomplete.

The preferred order is:

```text
complete standalone CLI trust boundary
        |
        v
reliable modules and native tests
        |
        v
durable, validated runtime artifacts
        |
        v
provider-neutral model adapter
        |
        v
structured experience memory + deterministic retrieval
        |
        v
evidence-driven policy adaptation
        |
        v
verified skill synthesis and promotion
        |
        v
bounded semantic repair/deltas
        |
        v
content-addressed HFIR and incremental state
        |
        v
adaptive/agentic application framework
```

As of the August 2026 backlog, the current items most closely aligned with this sequence include:

- standalone CLI semantics,
- standalone module support,
- native VM test execution,
- standalone artifact validation/versioning,
- provider-neutral HFIR model adapters,
- semantic patch deltas and bounded repair context,
- and content-addressed HFIR/incremental compilation.

The tracker numbers and status remain authoritative in `improvements.md`; this blueprint defines the architectural ordering, not daily backlog state.

### Design rule for future AI features

Before adding any new AI-specific primitive or framework feature, answer:

1. What deterministic capability does this require?
2. What data may the model see?
3. What structured output is allowed?
4. What does the verifier prove before the result can execute?
5. What budget limits the probabilistic operation?
6. What tests or invariants must pass?
7. Can the model change the tests that approve its own change?
8. How is the result replayed, audited, or mocked?
9. What happens when the model is unavailable?
10. Can the same application still be understood and controlled without trusting hidden model behavior?

If these questions do not have concrete answers, the feature is not ready to become part of HowlFrame's AI-native core.

## Standard library direction

Avoid indefinitely expanding the compiler with a dedicated AST node for every operation.

The desired layering is:

```text
core language and HFIR primitives
-> portable HowlFrame standard library
-> runtime host interfaces
-> domain libraries
-> framework libraries
-> backend adapters
```

Candidate standalone standard-library areas include:

```text
core
collections
strings
math
io
files
paths
json
time
random
testing
http
process
stores
```

Each library must declare:

- semantic behavior,
- effects,
- capabilities,
- target support,
- error behavior,
- and deterministic test evidence.

## Compatibility-backend policy

Go and JavaScript should remain supported where they provide real value.

They may be used for:

- interoperability,
- deployment environments,
- feature incubation,
- ecosystem access,
- and differential conformance testing.

They must not silently define the semantics of portable HowlFrame programs.

A construct claimed as portable must either:

- behave consistently in the VM and supported backends,
- or fail target-feasibility validation before output generation.

The standalone runtime must never quietly generate Go because the VM lacks coverage.

## Testing and conformance gates

Standalone runtime work should require:

- unit tests for lowering and VM instructions,
- parser/checker/HFIR/VM integration tests,
- real CLI subprocess tests,
- deterministic artifact tests,
- capability denial and grant tests,
- source-location tests,
- module tests,
- reference-application tests,
- and differential tests for portable deterministic semantics.

Recommended release gates:

1. No accepted standalone-v1 construct lacks bytecode and VM coverage.
2. No capability-bearing instruction runs without an explicit grant.
3. No standalone command writes generated target-language source.
4. No standalone test requires Go, Node.js, a paid model, or an external service.
5. Every artifact is validated before execution.
6. Every unsupported feature fails with a stable diagnostic.
7. Reference applications pass on every supported platform.

## Success measures

Track evidence such as:

- percentage of accepted CLI constructs supported by HFIR, bytecode, and VM,
- number of documented VM/backend semantic mismatches,
- first-pass success rate for AI-generated standalone programs,
- average repair rounds,
- runtime startup time,
- artifact size,
- deterministic-build rate,
- test execution time,
- percentage of runtime errors with accurate HowlFrame source locations,
- and percentage of effectful operations covered by enforced capability policy.

The primary product measure is:

> Can a person or coding agent create, validate, test, package, and run a useful multi-file CLI application using HowlFrame alone, without relying on generated source or another language toolchain?

After that milestone is reached, the primary AI-native product measure becomes:

> Can a HowlFrame application use probabilistic reasoning to adapt its behavior or propose semantic changes while the deterministic runtime still enforces capabilities, budgets, tests, invariants, provenance, and rollback?

A complementary learning measure is:

> Does the application measurably improve from accumulated experience while every behavioral change remains attributable, testable, bounded, versioned, and reversible?

## Non-goals for standalone CLI v1

Do not block the first complete runtime on:

- autonomous model weight training or self-fine-tuning,
- self-hosting the compiler,
- a public package registry,
- a full IDE,
- native 3D graphics,
- a complete browser framework,
- replacing HTML and CSS,
- direct machine-code generation,
- formal verification of every program,
- or perfect parity with every compatibility backend.

## Backlog integration procedure

When working from this blueprint:

1. Read `bugs.md` and `improvements.md` completely.
2. Search for an existing item covering the selected gap.
3. Reuse or phase that item rather than creating a duplicate.
4. If no item exists, add one narrowly scoped improvement with acceptance criteria, dependencies, tests, model routing, and execution mode.
5. Confirmed incorrect behavior belongs in `bugs.md`.
6. Work only one numbered item or phase per implementation session.
7. Update this blueprint only when an architectural decision or completion gate changes; do not use it to track daily status.

## Recommended immediate planning action

Run a backlog-grooming session focused only on standalone CLI readiness.

That session should:

1. Build the runtime coverage inventory from Milestone 0.
2. Map every gap to an existing bug or improvement.
3. Add only missing session-sized items.
4. Rank the deterministic CLI core, native test runner, module reliability, stdin, error model, artifact validation, and packaging work.
5. Select one highest-value actionable item.
6. Stop before implementation and produce a single-item prompt for the next session.

This preserves one source of implementation truth while giving the project a clear destination.
