# HowlFrame Product Directive

**Status:** Finalized product-direction and autonomous-development directive  
**Decision authority:** User-approved interview decisions D1–D32  
**Interview completed:** October 13, 2026  
**Historical technical baseline:** HowlFrame repository assessment of October 12, 2026; **not** a fresh inspection of current `main`  
**Repository:** <https://github.com/howlcipher/HowlFrame>  
**Intended readers:** Human maintainers and AI engineering agents (including Claude, Codex, AGY, and integrations through HowlPlane)

> **Primary directive:** Make HowlFrame capable of building useful, independently executable software through autonomous, correctness-first AI development. Prove this with independently selected, increasingly difficult applications and independently verified acceptance evidence. Improve the language, VM, HFIR, security, and tooling when needed to deliver those applications. **A completed product milestone outranks another cycle of dogfooding.**

This document records decisions made during the 32-question interview. It defines product intent and proposed operating policy; it does **not** grant an agent access beyond its actual runtime permissions, authorize new remote Git operations, assert that aspirational features are implemented, or certify the present repository without inspection.

## 1. How to interpret this directive

1. **Preserve the actual decisions.** The selected letters, meaning, explicit qualifications, and temporal scope in the decision ledger (§13) are authoritative. Do not silently transform a desired feature into a shipped capability or a deferred item into a permanent prohibition.
2. **Unselected is not forbidden.** Unless separately established by another decision or existing security policy, a choice not selected in the interview is simply *not a newly mandated requirement*. Some choices remain open; some capabilities are allowed but not prioritized.
3. **Distinguish three different kinds of goal:** (a) Milestone 1 acceptance, (b) Milestone 2/3 scope, and (c) **later** competitive targets. Competitive benchmarks must not be inserted as extra Milestone 1 pass percentages.
4. **Treat the October 12 assessment as historical.** Before implementation, inspect current code, branch, test results, documentation, runtime behavior, and permissions. Never represent past findings as freshly verified.
5. **Use demonstrated results to refine implementation choices.** Record material changes and evidence. Do not weaken frozen application requirements or override these decisions merely to make a test pass.
6. **Operate within granted authority.** The interview specifies allowed *categories* of future agent activity; repository settings, execution boundaries, and explicit human authorization still control real actions.
7. **Stop when the milestone is accepted.** Close the milestone with evidence; do not continue automatically into unrelated research, speculative optimization, or a new indefinite improvement loop.

## 2. Product identity, users, and success

HowlFrame is intended simultaneously as:

- An **AI-first programming language** designed to be created and manipulated by AI systems.
- A **capability-governed execution and policy runtime** with explicit authority and safe execution objectives.
- A **practical software development language** for independently useful programs, not just experiments.
- An **AI-native software-creation platform** that supports building software from natural-language requirements.
- A **foundation of the Howl ecosystem**, while remaining usable outside it.
- An **experimental programming-language research project** whose innovations must be judged against practical evidence.

Those identities may have different priorities at different stages. The first priority is not to maximize language features or prove every security ambition; it is to deliver verified autonomous software development with standalone execution. Governed execution and security remain foundational throughout.

### Target outcomes

Prove that HowlFrame can produce useful, independent applications; measurably improve AI software construction; support sophisticated full-stack work; serve real Howl workflows; be straightforward for human and AI users; and demonstrate meaningful AI-native programming ideas.

Long-term supported application categories include **CLI/developer tools, backend APIs, full-stack web software, data pipelines and integrations, desktop applications, games/simulations, mobile applications, and reusable libraries/packages**. Autonomous agents were **not** selected as a separate target *application category*; AI-assisted authoring, external agent access, and HowlPlane orchestration are still central.

### Definition of progress

Evidence of progress is a useful application that executes independently, satisfies independently authored requirements and tests, remains reproducible, and can be maintained. Internal code churn, increased feature counts, benchmark claims without matched tests, and endless dogfood iterations are **not** substitutes for that proof.

## 3. Architectural direction and representation policy

### 3.1 Core execution architecture

- Prioritize the **standalone HowlFrame runtime/VM** over maintaining perfect equivalence with secondary backends.
- Permit real-world OS integration through **governed capabilities**—for example filesystem, process, network, database, and other host access as needed and explicitly authorized. Governed access is not unrestricted host access.
- Support reusable modules, standard libraries, and packages. Prefer libraries to needless compiler primitives when they solve the same application requirement.
- Maintain independently useful language tooling and AI-programming interfaces so developers need not install HowlPlane or the rest of the ecosystem.
- Keep web, desktop, game, and other application frameworks **outside the language/runtime core**, in separate packages when developed.
- Compilation to other targets can continue where useful, but full Go/JavaScript/Wasm parity is not an initial delivery gate.

### 3.2 `.howl` source and HFIR

**Accepted decisions:** AI may directly author either `.howl` or HFIR. For each task, agents should choose the representation most likely to produce **verified correct software**, based on the task, with the ability for a human to override the choice. Neither representation was designated the permanent canonical source of truth.

- Aim for a **stable, documented, versioned HFIR format** suitable for dependable semantic operations.
- Subject directly AI-authored HFIR to rigorous semantic validity, correctness, capability/security checks, and execution-related validation. It must not become a route around safeguards applied to `.howl`.
- **Defer permanent canonical ownership** until practical application results support an architectural decision.
- Do **not** infer that bidirectional, lossless `.howl`↔HFIR conversion, production HFIR bytecode generation, or source-of-truth parity was selected as a required Milestone 1 capability.
- Preserve proven behavior in accordance with the stability-over-rewrites principle; replacing a working compilation path requires verified evidence, not architectural taste.

**Historical nuance:** As assessed October 12, HFIR served as a production semantic verification gate while the AST-to-bytecode path remained the production compiler. Direct HFIR-to-bytecode lowering was experimental, and the production flip was deferred. Reinspect before treating any of this as current.

### 3.3 Security, reliability, and authority

The security/reliability direction encompasses **all nine** D12 requirements:

1. Explicit capability permissions and enforcement.
2. Secure-by-default behavior.
3. Strict resource limits.
4. OS sandbox/container isolation for untrusted execution **as a future assurance goal where not yet delivered**.
5. Predictable errors and recovery behavior.
6. Auditable and reproducible execution.
7. Integrity checks for runtimes and executable artifacts.
8. Optional, explicitly authorized trusted mode.
9. Strong correctness and regression testing.

A capability-aware VM is **not automatically an OS sandbox**. Never advertise OS-level isolation or other guarantees unless verified against the present implementation. Trusted mode is not carte blanche to bypass actual permissions or weaken security rules.

### 3.4 Required developer experience

Developer tooling should emphasize an intuitive CLI; effortless installation and setup; actionable errors; machine-readable diagnostics for AI agents; debugging and inspection; module/dependency handling; session-friendly autonomous workflows; visibility into AI-generated changes for human review; and up-to-date documentation. Project scaffolding/examples were not chosen as a separate D13 priority, but are not prohibited.

## 4. Boundaries across the Howl ecosystem

| Component | Primary ownership under the directive |
|---|---|
| **HowlFrame** | Language, compiler, standalone VM/runtime, HFIR and semantic verification, capability/execution authority, standard libraries/package ecosystem, AI-native authoring interfaces, developer CLI/debugging/tooling. |
| **HowlPlane** | Provider/model selection (Claude, Codex, AGY and others), reasoning effort, budgets and failover; autonomous task decomposition, orchestration, retries and agent/session handoffs. |
| **HowlCreate** | Software-creation workflows connecting requirements with construction, checking, building, and execution using HowlFrame. |
| **HowlDream** | Discovery, ideation, exploration, and candidate requirements. |
| **HowlBoard** | Human-facing dashboards, approvals, and mission/progress visibility. |
| **Separate framework packages** | Application-specific frameworks (e.g. web, desktop, games), without expanding the core compiler/runtime unnecessarily. |
| **Shared integration contracts** | Stable, versioned APIs/interfaces between independently deployable components. |

HowlFrame must remain **independently usable**. Other AI tools may interact with it directly; they are not required to route through HowlPlane. Responsibilities may change when credible evidence warrants it, but avoid duplicating model/provider management and development orchestration inside HowlFrame.

## 5. Prioritization and engineering trade-offs

All ten D20 principles apply. When trade-offs arise, prefer:

1. **Working applications** to speculative features without demonstrated need.
2. **Correctness** to shorter code or fewer tokens.
3. **Security and explicit authority** to unrestricted convenience.
4. **Standalone execution** to perfect backend parity.
5. **Reusable libraries** to needless compiler/language expansion.
6. **Usability** to architectural cleverness that makes the system harder to operate.
7. **Stability and proven behavior** to speculative rewrites.
8. **Measured results** to theoretical advantages.
9. **Maintainability** to uncontrolled feature growth.
10. **Acceptance of the current milestone** before substantial, unrelated expansion.

These are complementary preferences rather than a mathematically complete tie-break algorithm; resolve any remaining conflict using validated application needs, risk, and recorded evidence. Speculative language features were **not** separately adopted as an explicit D21 non-goal, though practical work remains higher priority by D20.

### Explicit initial non-goals / deferrals (D21 B–J)

Avoid: duplicating HowlPlane; placing application frameworks in the core; gating standalone VM progress on total backend parity; premature compiler/runtime rewrites; attempting to outperform every language on everything; **endless dogfooding without deliverables**; requiring the entire Howl ecosystem; premature broad expansion into advanced application categories; and major optimization without evidence from real workloads. These are initial boundaries and deferrals, not an assertion that each possibility is forever prohibited.

## 6. Roadmap and milestone gates

The three-stage roadmap is a **sequence of product proofs**, not a mandate to implement all later features in parallel.

### Milestone 1 — Three distinct, autonomous, standalone CLI applications

**Primary proof:** AI can independently implement real CLI software from frozen requirements, using HowlFrame's standalone VM, and satisfy independently authored acceptance tests **without human implementation or repair**.

**Non-negotiable selection and coverage rules:**

- Complete **at least three** distinct applications. This is the explicitly selected quantitative Milestone 1 threshold; do **not** invent any additional minimum pass percentage, model count, duration target, or task-count threshold.
- An independent evaluator **selects or approves** the applications; the implementing agent does not get to pick only tasks that are easy for the language.
- Choose **three substantially different problem domains** and increase complexity across the progression.
- Collectively exercise meaningful standalone runtime capabilities and governed access; no arbitrary enumeration of mandatory capabilities per application is imposed beyond the chosen requirements.
- Include software **unrelated to the Howl ecosystem** so independent usefulness is demonstrated. An ecosystem application was not selected as a requirement for these initial three.
- **Freeze each application's functional requirements before implementation.** Agents may not remove or dilute failing requirements to claim success.
- Reproduce application installation, building, and execution **from clean environments**.

**Per-application acceptance package — all ten items required (D24 A–J):**

| ID | Required artifact or evidence |
|---|---|
| A | Written original requirements and acceptance criteria, fixed before development. |
| B | Complete program source (`.howl` or qualifying HFIR) and a standalone executable HowlFrame artifact; it must not depend on a generated Go/Python application for execution. |
| C | Independently authored functional acceptance tests and their results, including applicable edge cases. |
| D | Reproducible clean-environment installation/build/run instructions **and evidence** that they succeed. |
| E | Evidence of autonomous AI implementation and repair from requirements, without human-written implementation fixes. |
| F | Clear user documentation and a working, useful end-to-end CLI workflow. |
| G | Regression tests and honest documentation of known limitations or defects. |
| H | Capability/security verification for relevant permissions, denials, and resource limits. |
| I | Preserved development history, relevant blocker/fix evidence, and handoff/commit records. |
| J | Independent final review against the frozen original requirements, with a recorded acceptance decision. |

**Pass condition:** An independent reviewer accepts all three distinct applications, with the evidence packages above and clean-environment execution. A self-reported agent success message is insufficient. No later language-comparison target is part of this gate.

**Permitted supporting work during this milestone (D22 A,B,C,D,F,G,H,I,J):** Build demonstrably missing functionality; fix compiler/VM defects; address actual security and runtime reliability gaps; improve encountered UX; advance HFIR or AI-native interfaces when relevant; repair installation/reproduction; add independent tests/evidence; improve external-AI interfaces; and conduct **bounded exploratory research**. Separate reusable-module/library work (D22 E) was **not** selected as a named Milestone 1 activity, though the longer-term module/library architecture remains affirmed by D11/D17. Do not automatically convert it into a requirement for this phase.

**Stop rule:** Upon independent acceptance of the milestone, record the outcome, document carryover work, and **stop this milestone's autonomous loop**. Do not continue recursively finding more fixes merely because opportunities exist.

### Milestone 2 — Practical standalone backend APIs and better developer UX

Following Milestone 1, invest in **both** backend APIs and developer experience. The selected Milestone 2 acceptance scope is exactly D29 **A,C,D,G,H**:

- **A — Practical HTTP API:** Working HTTP routing, request/response processing, structured payloads, and common HTTP operations for a real backend use case.
- **C — Authentication and authorization:** Authenticate requests and enforce meaningful application-level permissions.
- **D — Validation and reliable error behavior:** Reject malformed/invalid inputs appropriately; provide predictable failures and actionable diagnostics.
- **G — Standalone installation/execution:** Build/install/run from a clean environment without the rest of the Howl ecosystem or a generated application in another language.
- **H — Demonstrably improved developer experience:** Show improvements in real workflows for commands, errors, logs, debugging, documentation, and building/maintaining backend software.

These are the specific Milestone 2 requirements. Persistent data (B), a separate governed networking/database gate (E), dedicated concurrency/long-running service gate (F), standalone Milestone 2 autonomy/independent-testing criterion (I), and a separate Milestone 2 independent-final-review criterion (J) were **not chosen as additional named exit conditions**. Do not invent them. General security, correctness, and authorization obligations continue to apply to whatever is built. Also do not invent a fixed number of endpoints, services, users, or pass-rate thresholds.

**Pass condition:** Demonstrate the selected five capabilities with inspectable execution and UX evidence appropriate to the chosen backend, then record the completed milestone and stop the phase. The exact demonstration plan must be established before execution, rather than retroactively rewriting requirements.

### Milestone 3 — Full-stack application and real Howl ecosystem adoption

Pursue **both**:

- A **complete full-stack web application** with meaningful frontend and backend behavior.
- A **useful implementation or replacement of a genuine Howl ecosystem component** using HowlFrame, not an artificial integration.

The **scope and order** of these two objectives are deliberately evidence-driven (D30 J): use the results and demonstrated limitations from Milestones 1 and 2 to sequence them. The interview did **not** establish extra numeric Milestone 3 gates, a required specific component, or unconditional parallel execution. Define testable acceptance before starting each project, keep app frameworks separately packaged, and verify useful execution.

### Beyond the first three milestones

Broader desktop, game, mobile, data-integration, and library/package ecosystems remain legitimate long-term areas. Security isolation, reliability, HFIR, semantic authoring, and competitive evaluations may mature as evidence and readiness justify. Do not expand into these categories merely to produce more internal activity while an earlier milestone remains unaccepted.

## 7. Autonomous engineering operating policy

### 7.1 Approved activity scope (D25 A–C)

Subject to real sandbox/repository policy and assigned workspaces, future agents may:

- Independently create, modify, test, and manage **application files in designated workspaces**.
- Repair a **demonstrated HowlFrame defect or missing capability** that genuinely blocks the assigned milestone, with evidence and tests.
- Run builds, automated tests, local program executions, and diagnostics within approved capability, security, and resource boundaries.

This does **not** authorize arbitrary destructive actions, unrestricted host access, changes outside the designated workspace, credential exposure, security-policy bypass, or remote Git mutation.

### 7.2 Persistence and Git scope (D26 A,C,H)

Agents should maintain:

- A **durable development journal** recording decisions, blockers, root causes, repairs, tests, and next actions.
- **Automatic local Git commits** of validated work at meaningful checkpoints with informative messages.
- **Structured session handoff records** sufficient for a fresh AI session to resume without redoing previously completed investigation.

**Do not treat remote `git push`, pull request creation, or merges as authorized by this directive.** These operations were not selected in D25 or D26. A separate explicit authorization and actual repository rights are required before performing them. Automatic provider switching and orchestration are HowlPlane's responsibility where separately configured/authorized; D26 did not newly authorize remote operations or silently grant HowlPlane broader powers.

### 7.3 Blockers, research, recovery, and stop conditions (D23 A,B,C,E,G,H,I)

1. **Reproduce a genuine blocker** against the frozen application requirements; capture evidence.
2. **Prefer a narrow fix** to the demonstrated problem, with an appropriate regression test, then retry the original application task.
3. **Bound exploratory work** before beginning it; record hypotheses, what was tested, and its stopping condition. The interview did not set one universal time or retry number.
4. **Bound failure/recovery cycles** so a task cannot restart indefinitely. On repeated failure, change the strategy or escalate the unresolved problem while retaining state.
5. **Maintain journals, commits, and handoffs** to survive session limits and context loss.
6. **Stop once independent acceptance is satisfied**; do not initiate another general dogfood cycle.

D23 did **not** select an additional blanket rule that every unrelated improvement must always be deferred (D), a newly mandated human approval for every major decision (F), or acceptance of temporary workarounds as a general policy (J). Their omission is not permission to violate applicable security or scope restrictions. Default to the explicit authority in §§1 and 7.1–7.2.

### 7.4 Reviewable work products

Keep enough evidence that a separate reviewer can reconstruct the outcome without trusting the developing agent's summary. Suggested file/folder naming conventions are implementation examples, **not new mandatory artifact formats**:

```text
<assigned-workspace>/
  requirements/             # Original, frozen specs and acceptance criteria
  src/                      # .howl and/or HFIR sources
  tests/                    # Regression and independent acceptance tests
  evidence/                 # Reproduction runs, security tests, logs
  DEVELOPMENT_JOURNAL.md    # Durable factual chronology
  HANDOFF.md                # Current status, next action, blockers
```

Avoid exposing secrets in any journal, artifact, commit, or evidence log. Before any execution, align paths and repositories with actual user instructions and current permissions.

## 8. Independent evaluation and evidence-driven decisions

- **Separate requirements/evaluation from implementation:** An independent party selects or approves initial CLI projects, freezes their requirements, designs/owns acceptance criteria, and reviews the result. The same agent's claim of completion does not substitute for this review.
- **Use matched, reproducible evidence:** For major changes, record the baseline, current behavior, required behavior, smallest credible change, and test results. Distinguish evidence from assumptions.
- **Test capabilities honestly:** Include the actual permission grants/denials and resource-control evidence relevant to each application. Never declare complete host isolation without independent verification of the host boundary.
- **Use real workload pressure:** Application failures can justify language/compiler, HFIR, library, UX, or runtime work; do not do broad rewrites or low-level optimization without compelling proof.
- **Make results falsifiable:** Retain failures, incomplete criteria, limitations, and inconclusive benchmarks in reports. A milestone remains pending if its independently selected requirements are not satisfied.
- **Record decisions without retroactive goalpost changes:** If previously frozen requirements truly must change, clearly distinguish a separately approved requirements revision from the original test. Do not silently erase failures.

## 9. Later competitive validation program — NOT early milestone gates

The competitive goals express hypotheses and targets, **not measured results** and **not additional Milestone 1 or Milestone 2 criteria**.

### Desired competitive advantages (D14 A,B,D,E,F,G,H,I,J)

Higher AI development correctness; less code/token consumption; more effective autonomous development; built-in security/governed execution; useful semantic-programming capabilities; easier development of relevant application categories; lower overall engineering cost; efficient runtime execution; and maintainable software. *Fewer repair cycles* was **not** separately selected as its own competitive advantage (D14 C), although reliability and correctness are important.

### Evaluation methods (D15 A,C,F,G,H,I)

Run controlled, matched head-to-head software-development trials; compare with **Python, Go, and JavaScript**; benchmark runtime on comparable workloads; obtain independent security/reliability evidence; evaluate long-term maintenance tasks; and test with multiple independent AI models/providers. Avoid unfair baseline designs, hand-picked only-successful cases, or unsupported claims of superiority.

### Quantitative later-stage targets (D16 A,B,C,D,G,H)

| Measure | Later target | Interpretation / caution |
|---|---|---|
| Autonomous task completion | **At least 80%** | At least 8 of 10 for a ten-task trial; benchmark sample size/protocol still to be agreed. |
| Matched correctness wins | **At least 60% against each of Python, Go, and JavaScript** | Compare against **each baseline separately** on matched tasks; this is not a generic 60% pass-rate requirement. Define a fair win/tie protocol before experiments. |
| Verified development speed | **At least 20% faster** | Measure time or equivalent agreed development-effort protocol to independently verified, correct software; method to be finalized. |
| Runtime performance | **Execution time within 2×** | Use agreed, comparable representative workloads; do not imply parity for every workload. |
| Maintenance success | **At least 80%** | At least 8 successful changes of 10 in a ten-change trial, if that sample size is used; define success and cases independently. |
| Cross-provider support | **At least two independent AI providers** | Reproduce relevant results across providers; not merely two sessions of one provider. |

No baseline workload suite, provider pair, evaluation sample size (except the illustrative 8/10 interpretations), tie policy, or absolute performance result was approved as already complete. Finalize experimental protocols before measuring, publish limitations, and distinguish implementation improvements from evaluation results.

## 10. Repository reality check — historical October 12, 2026 assessment

**This section is a historical reference only. It is not a representation of the repository on October 13 or at any future execution date.**

The October 12 assessment found an experimental AI-native language with `.howl` S-expression source; a capability-governed standalone VM and `.hfbc` artifacts with execution grants and instruction budgets; interpreter plus Go, JavaScript and Wasm compilation-related targets; CLI examples, HTTP-related functionality and HowlBoard integration. HFIR was used as a semantic verification gate while conventional AST-to-bytecode lowering remained the production path. Direct HFIR bytecode lowering was experimental/deferred. Capabilities and agent/process authority had recently been hardened; this **was not equivalent to OS-level sandboxing**.

The assessment also observed incomplete or changing language/backend/tooling features, mixed evidence on token density and application-level superiority, and unresolved security/scoping/correctness/performance/test-stability issues. Historical references included main commit `5009e035` and open issues numbered `#166`, `#165`, `#164`, `#157`, and `#147`. **Do not presume those commits, issue statuses, tests, or capabilities remain unchanged.**

### Required diligence before actually executing this directive

An implementing agent should inspect the current default branch and working tree; applicable documentation/roadmaps; standalone VM instructions; latest `.howl` and HFIR paths; relevant capability implementations; fresh build and test outcomes; current open issues; and actual tool/Git authority. Report what is **verified now**, what is **historical**, what remains **uncertain**, and what must be developed. This is a proposed starting procedure, not a claim that such an inspection was performed for this artifact.

Suggested historical reading targets (recheck their present paths):

- <https://github.com/howlcipher/HowlFrame>
- `README.md`
- `docs/hfir_execution_status.md`
- `docs/architecture_roadmap.md`

## 11. Agent-ready execution playbook

The following is the operational reading of the approved decisions. It does not extend privileges beyond §§1 and 7.

**Step 1 — Verify present reality.** Read this directive and the current code/docs; establish the exact branch/commit and permission boundary; run applicable baseline tests. Mark pre-existing failures, uncertainty, and gaps without inventing success.

**Step 2 — Establish the current milestone.** If Milestone 1 is not independently accepted, do not start broad Milestone 2 or 3 expansion. Otherwise proceed to the next approved milestone. Maintain a concise progress/evidence ledger.

**Step 3 — Obtain independent, frozen requirements.** For Milestone 1, use an independent selector/evaluator to establish three different, increasingly complex CLI applications with meaningful runtime coverage and at least one use case outside the Howl ecosystem. Record original acceptance criteria before development.

**Step 4 — Implement within scope.** Have the AI produce `.howl` and/or HFIR according to task suitability and verified correctness; maintain a versioned HFIR discipline where relevant; preserve human override. Build for the standalone VM. Use real governed OS access only when approved.

**Step 5 — Resolve genuine blockers, narrowly.** Reproduce defects; fix missing semantics/runtime/UX/security problems that prevent application acceptance; add regression tests; retry the original requirement. Keep exploratory work bounded. Do not turn every observation into a redesign.

**Step 6 — Record and hand off.** Keep a journal, structured handoff, test evidence, and validated **local commits**. Never assume `git push`, PR creation, or merging is authorized. When a session reaches limits, preserve a usable restart point within available permissions.

**Step 7 — Independently verify.** For each CLI, assemble all ten D24 evidence classes; execute clean-environment reproduction and security checks; have a separate evaluator review against the frozen requirements. If any criterion fails, report *not accepted* and the concrete blocker.

**Step 8 — Stop correctly.** Once all three applications pass, declare Milestone 1 complete. Do not restart the dogfood cycle to search for more improvements. Only then plan the Milestone 2 HTTP/auth/validation/standalone/UX proof. For Milestone 3, choose the sequence of full-stack and genuine Howl ecosystem work using prior evidence.

**Step 9 — Evaluate competitive claims separately.** When ready, establish controlled Python/Go/JavaScript protocols and pursue the D16 targets without retroactively changing earlier acceptance standards.

### Mandatory handoff questions

A handoff should make these matters answerable from durable records: *Which milestone and frozen requirement are active? What was verified? What failed, why, and what was tried? What code was changed and locally committed? What permissions are available? What exact action should the next agent take? What constitutes acceptance and stop?*

## 12. Outstanding choices and interpretation notes

**Deliberately unresolved by earlier decisions:**

- The canonical `.howl`-versus-HFIR source-of-truth model (D5, D28); AI chooses per task, human override remains, and rigorous HFIR checks are required.
- The exact sequencing and project identity within Milestone 3 (D30); decide from earlier application evidence.
- The detailed benchmark workloads and measurement protocols for later quantitative competition (D15–D16); target values are agreed but results are not.
- The specific research/retry budgets and escalation implementation (D23); limits are required, universal numeric thresholds were not selected.
- How to implement/verify robust OS-level isolation (D12); do not confuse policy enforcement with complete host sandboxing.

**Important D32 distinction:** In the *final* question, **only J (“I'm ready—generate the final Markdown directive”) was selected**. Options A–I of Question 32 were **not** additionally adopted as new decisions. The previously established unresolved matters above remain unresolved **because of their earlier confirmed decisions or lack of implementation evidence**, not because the user selected Q32 A–I. D32 J authorizes creation of this artifact; it does not authorize GitHub writes, agent execution, or unselected scope changes.

**Not selected but not prohibited:** Scaffolding as a standalone UX priority (D13 F), agent systems as a separate software category (D3 D), reduced repair cycles as a separate comparative target (D14 C), mandatory reusable-library work *during* Milestone 1 (D22 E), blanket handling rules omitted in D23, additional Milestone 2 gates omitted in D29, and remote Git operations omitted in D25–D26. Preserve their exact distinction from positively selected requirements.

## 13. Complete 32-decision interview ledger

Each entry retains the exact selected choice letters and their approved substantive meaning. **`All` means every option for that question**, not necessarily the choices for another question. A missing letter is **not** a blanket ban. The narrative sections above operationalize these decisions without overriding this ledger.

| Decision | Selected options | Confirmed decision and material qualifications |
|---|---|---|
| **D1 — Product purposes** | **A, B, C, D, E, F (all)** | AI-first language; governed execution/policy runtime; practical applications; AI-native software-creation platform; foundation for Howl ecosystem; programming-language research. Different stages may prioritize them differently. |
| **D2 — Proof goals** | **A, B, D, E, F, G** | Useful independent software; measurable AI coding improvement; sophisticated full-stack apps; useful Howl workflows; ease for people and AI; next-generation AI-native programming. Security remained foundational even though separate initial-proof choice C was not selected. |
| **D3 — Application categories** | **A, B, C, E, F, G, H, I** | CLIs/developer tools; backend APIs; full-stack web; data pipelines/integrations; desktop; games/simulations; mobile; libraries/packages. AI agent systems (D) not selected as an application category. |
| **D4 — Authoring approach** | **A, C, D, E** | AI directly writes `.howl`; natural-language-to-software; AI authors/edits semantic HFIR; hybrid human/AI collaboration. Not a mandate to optimize hand-authoring, visual tools, or invisible integration first. |
| **D5 — AI-written representation** | **C only** | AI may author either `.howl` or HFIR. Neither is declared canonical. |
| **D6 — Representation choice** | **A, D, H** | Choose for highest likelihood of verified correctness, match to task, allow human override. Token reduction/speed not the dominant decision criterion. |
| **D7 — First-wave examples** | **A, B, C, I, J** | Practical CLI; standalone backend API; full-stack web; existing Howl ecosystem application; independent autonomous natural-language-to-application project. First-wave does not mean all simultaneously. |
| **D8 — Initial emphasis** | **A, E** | Standalone CLI software and fully autonomous creation from natural-language requirements come first. |
| **D9 — Initial proof gate** | **A, B, C** | Application executes independently in HowlFrame VM (not generated Go/Python); autonomous AI implementation/repair without human implementation; independently authored functional acceptance tests pass. |
| **D10 — Initial quantitative/reproduction requirements** | **A, H** | At least **three distinct** independently AI-developed CLIs and **clean-environment reproducibility**. No other first-milestone numerical performance or completion threshold was approved. |
| **D11 — Architectural priorities** | **A, B, E** | Prefer standalone execution; practical governed OS capabilities; modular reusable libraries/packages. |
| **D12 — Security/reliability** | **A–I (all)** | Capabilities; secure defaults; resource limits; future OS isolation for untrusted code; errors/recovery; auditability/reproducibility; runtime/artifact integrity; optional explicitly approved trusted mode; strong tests. OS sandboxing not assumed implemented. |
| **D13 — Developer UX** | **A, B, C, D, E, G, H, I, J** | Intuitive CLI; installation; actionable errors; AI-readable diagnostics; debugging; modules/dependencies; autonomous workflows/handoffs; human change visibility; docs. F scaffolding not selected as a separate priority. |
| **D14 — Desired advantages** | **A, B, D, E, F, G, H, I, J** | AI correctness; fewer tokens/less code; autonomous development; security; semantic programming; easier application building; lower engineering cost; runtime efficiency; maintainability. Fewer repair cycles (C) not a separate selected objective. |
| **D15 — Evidence methods** | **A, C, F, G, H, I** | Controlled matched trials; comparisons against Python/Go/JavaScript; runtime benchmarks; independent security/reliability testing; maintenance studies; multiple AI providers/models. |
| **D16 — LATER competitive thresholds** | **A, B, C, D, G, H** | ≥80% autonomous completion; ≥60% matched correctness wins against **each** Python/Go/JS baseline; ≥20% faster verified development; runtime execution within 2× agreed workloads; ≥80% successful maintenance changes; at least two independent AI providers. **Not** Milestone 1 gates or achieved metrics. |
| **D17 — HowlFrame ownership** | **A, B, C, D, E, H** | Language/compiler/runtime; HFIR verification; capability/security authority; standard library/package ecosystem; AI authoring interfaces; developer CLI/tools. Provider management, orchestration, core app frameworks, secondary target parity not selected as direct ownership priorities. |
| **D18 — Ecosystem ownership** | **A–J (all)** | HowlPlane owns AI providers and orchestration; HowlCreate creation workflows; HowlDream ideation; HowlBoard human interfaces; frameworks in separate packages; stable shared interfaces; independently usable HowlFrame; direct access by external AIs; responsibilities flexible based on evidence. |
| **D19 — After CLI milestone** | **A, F** | Invest next in complete standalone backend APIs **and** developer experience/tooling. |
| **D20 — Engineering trade-offs** | **A–J (all)** | Working products; correctness; security; standalone VM; reusable libraries; usability; stability; evidence; maintainability; completion before broad expansion prevail when priorities conflict. |
| **D21 — Non-goals/deferrals** | **B, C, D, E, F, G, H, I, J** | Avoid Plane duplication; core framework bloat; perfect backend parity as gate; premature rewrites; beating all languages on all metrics; endless dogfooding; mandatory ecosystem; premature advanced apps; unsupported performance optimization. A speculative features not selected as an explicit non-goal. |
| **D22 — Milestone 1 authorized work** | **A, B, C, D, F, G, H, I, J** | Missing functionality; compiler/VM correctness; security; UX; HFIR/AI authoring; clean install; independent evidence; external AI integration; bounded research. E reusable libraries not selected specifically for Milestone 1, but not prohibited. |
| **D23 — Blockers and stop** | **A, B, C, E, G, H, I** | Auto-fix actual blockers; preserve evidence/regression tests; narrow fixes; bounded research; durable session progress; bounded retries; stop when accepted. D blanket unrelated-work deferral, F new mandatory human approval, J blanket workarounds not selected. |
| **D24 — Evidence per CLI** | **A–J (all)** | Frozen written requirements; complete source/standalone artifact; independent acceptance tests; clean reproduction; proof of autonomous implementation; user workflow/docs; regressions/limitations; capability/security tests; development history; independent final acceptance review. **Each** of three apps supplies all ten. |
| **D25 — Agent activity authority** | **A, B, C only** | Develop freely in assigned workspaces; autonomously repair demonstrated HowlFrame blockers; run authorized builds/tests/diagnostics. Does not grant automatic remote Git actions. |
| **D26 — Continuity and Git** | **A, C, H only** | Durable journal; automatic **local** Git commits of validated work; structured session handoffs. Remote pushes, pull requests, merges not authorized by this directive. |
| **D27 — Choose initial CLI apps** | **D, E, F, G, I, J** | Independent selection; three substantially different domains; progressively increasing complexity; meaningful runtime capabilities; at least one app outside Howl ecosystem; freeze requirements first. Other idea-sourcing approaches not mandated. |
| **D28 — `.howl` and HFIR** | **D, F, G, J** | AI chooses representation per task (earlier human override); stable/versioned HFIR; rigorous verification of AI-authored HFIR; defer canonical architecture. No requirement adopted for always-source-first, always-HFIR-first, or immediate production HFIR lowering. |
| **D29 — Backend Milestone 2 gate** | **A, C, D, G, H** | Practical HTTP functionality; authentication/authorization; validation/error handling; standalone clean installation/execution; demonstrably better UX. B/E/F/I/J not selected as extra Milestone 2-specific gates. |
| **D30 — Milestone 3** | **A, B, J** | Full-stack web application; useful Howl ecosystem component; choose exact next priority/order using prior milestone results. |
| **D31 — Directive format/content** | **A–J (all)** | Product vision/success; architectural ownership; complete ledger; phased acceptance roadmap; agent/Git scope; non-goals/anti-dogfood; evidence-driven decisions; competitive benchmarks; historical-vs-current reality check; agent-ready execution playbook. |
| **D32 — Finalization** | **J only** | User explicitly completed the interview and authorized generating this Markdown directive. Q32 A–I were not newly selected; maintain existing unresolved choices from earlier decisions, do not infer blanket approvals. |

## 14. Closing directive and completion test

**Build three real CLI applications. Prove that independent AIs can create them autonomously in HowlFrame and that independent evaluators can reproduce and accept them. Fix only the real obstacles, preserve evidence and local commits, and stop when they pass.** Then establish practical standalone HTTP backends with authentication, authorization, reliable validation/errors, and better developer UX. Next, apply HowlFrame to a complete full-stack application and a useful Howl ecosystem component, sequenced by evidence. Keep comparative claims separate until rigorously measured.

**The project wins by creating verified, independently useful software—not by perpetually improving the infrastructure that might someday create it.**

---

*Document-generation note: This file was authored from the user's confirmed D1–D32 interview decisions and the specifically identified October 12, 2026 historical repository assessment. No fresh repository inspection, live benchmark, Git mutation, or application verification was performed in creating this Markdown document.*
