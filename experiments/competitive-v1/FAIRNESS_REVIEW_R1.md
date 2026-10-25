# Independent Fairness Review: HowlFrame Competitive Protocol (`competitive-v1`)

**Review Date:** 2026-10-21  
**Reviewer:** Independent Reviewer (Gemini / Google DeepMind)  
**Target Documents:** `HOWLFRAME_COMPETITIVE_PROTOCOL.md`, `parameters.json`  
**Context Documents:** `context/DIRECTIVE_section9_competitive_targets.md`, `context/PR178_live-v1.md`, `context/benchmarks-v2.README.md`, `context/falsifiable-v0.README.md`, `context/live-v1-blind.README.md`, `context/token-efficiency-heldout-v0..v3.REPORT.md`, `context/token-efficiency-v0.REPORT.md`

---

## Overall Verdict

**NEEDS REVISION**

The protocol demonstrates commendable rigor regarding preregistration principles, container isolation, frozen black-box oracles, and transparent negative reporting. However, it cannot be certified as ready for Owner decision in its current form. It contains critical methodological flaws that simultaneously introduce bias **in favor of** HowlFrame in task scoping and bias **unfairly against** HowlFrame in scoring and hypothesis formulation. Additionally, it exhibits statistical pseudoreplication in sample sizing, lacks test-tampering protections, and contains factual inaccuracies regarding prior evidence.

---

## Executive Summary of Core Findings

| ID | Area | Severity | Protocol Section | Summary |
|---|---|---|---|---|
| **Finding 1** | Prior Evidence | **Important** | §2 | Factual misstatement of heldout-v0 W/T/L and overstatement of Benchmark v2 Track B harness. |
| **Finding 2** | Bias Guards | **Blocking** | §3.2 (3), §8 | "At most one authority task per domain" allows up to 100% of pilot tasks to centre on HowlFrame's niche. |
| **Finding 3** | Task Steering | **Blocking** | §3.2 (1, 4) | In-sample contamination via `live-v1-blind` and vulnerability to seed-mining from manifest hashes. |
| **Finding 4** | Feasibility Screen | **Important** | §3.2 (2, 5) | Stdlib-only screen masks HowlFrame expressibility gaps, conflicting with failure accounting in step 5. |
| **Finding 5** | Correctness Scoring | **Blocking** | §1 (H2), §7.2 | Counting ties as losses against HowlFrame creates an artificial ceiling; asymmetric acceptance rule. |
| **Finding 6** | Speed Metric | **Important** | §1 (H3), §7.3 | Imputing 45-min budget cutoff to unverified runs distorts medians and equates failure with slow success. |
| **Finding 7** | Runtime Target | **Important** | §1 (H4), §7.4 | Comparing against the "fastest baseline" tightens D16 unfairly by forcing parity with compiled Go. |
| **Finding 8** | Statistics & Sizing | **Blocking** | §9 | Pseudoreplication: treats 25 tasks × 6 runs as 153 independent trials, ignoring severe intra-task clustering. |
| **Finding 9** | Documentation Asymmetry | **Important** | §4, §5 | Uncontrolled doc prompt effects; baselines denied offline stdlib API references; token confounding. |
| **Finding 10** | Test Tampering | **Blocking** | §3.3, §4, §5 | Mutable workspace exposes acceptance tests to modification/deletion by autonomous coding agents. |
| **Finding 11** | Execution Mechanics | **Important** | §5, §7.3 | Unspecified completion handshake for interactive CLIs; wall-clock conflation with vendor API jitter. |
| **Finding 12** | Directive Completeness | **Important** | §1 (H5, H6), §6, §7.5 | Omits baseline maintenance comparisons; composite H6 conjunctive rule creates excessive alpha penalty. |

---

## Detailed Numbered Findings

### 1. Factual Inaccuracies in §2 Prior Evidence Summary
* **Severity:** **Important**
* **Section:** §2 (Table rows `token-efficiency-heldout-v0..v3` and `benchmarks/v2 Track A/B harness`)
* **Evaluation:**
  1. §2 summarizes `token-efficiency-heldout-v0..v3` as: `v0 1/0/29 (wins on 1 of 30 tasks it can express, ties 1)`. In `context/token-efficiency-heldout-v0.REPORT.md`, HowlFrame achieved **0 wins, 1 tie, and 29 losses** across the 30 tasks it could express (and 2 expressibility gaps, yielding 0/1/31 overall). Crediting HowlFrame with 1 win is factually false and contradicts the parenthetical "ties 1" (which sums to 30 expressible tasks only if wins = 0).
  2. §2 describes `benchmarks/v2 Track A/B harness` as "A runnable harness, reference results only". In `context/benchmarks-v2.README.md`, Track B agent execution is explicitly documented as unautomated and not runnable ("What Benchmark v2 still does NOT measure automatically: Track B agent interactions and automatic API fetching"). Only Track A possesses a runnable verification script (`run_track_a.py`).
* **Suggested Change:**
  - Amend row 3 of §2 to state: `v0 0/1/29 over expressible tasks (0/1/31 overall: 0 wins, 1 tie, 29 losses)`.
  - Clarify row 4 of §2: `benchmarks/v2 Track A harness (Track B unautomated) | A runnable Track A verification harness, reference results only`.

---

### 2. Bias Guard Under-Correction for Authority-Centred Tasks
* **Severity:** **Blocking**
* **Section:** §3.2 (Item 3), §8
* **Evaluation:**
  - §3.2 item 3 establishes: *"No more than one task per domain may centre on capability or authority enforcement, the area where HowlFrame has a built-in advantage."*
  - In the Pilot (§8), exactly 4 tasks are drawn (1 each from D1, D2, D3, and D4). Under this rule, **all 4 tasks (100% of the pilot)** could legitimately be authority-centred tasks.
  - In the Full Study (§9), with ~5 tasks per domain, up to 20% of the entire suite could centre on authority enforcement. For a benchmark testing general software engineering across general-purpose languages, this permits massive over-representation of HowlFrame's proprietary niche.
* **Suggested Change:**
  - Replace the per-domain rule with a **global benchmark cap**:
    - For the Pilot: at most **1 task across the entire 4-task pilot** (25%) may centre on capability/authority enforcement.
    - For the Full Study: at most **10%–15% of total tasks across all domains** may centre on capability/authority enforcement.

---

### 3. Task Selection Steering, Seed Mining, and In-Sample Contamination
* **Severity:** **Blocking**
* **Section:** §3.2 (Items 1, 4)
* **Evaluation:**
  1. *Task Contamination:* §3.2 item 1 allows: *"live-v1-blind cards may be included in D1 after removing any HowlFrame-specific grant wording."* As documented in `context/live-v1-blind.README.md`, those cards were generated inside the HowlFrame repository (`bot-rnd-0001`) during PR #178 / #180, and overlap earlier held-out corpora. The HowlFrame implementation team already knows these task specifications and has calibrated built-ins against them. Admitting them violates true task blinding and independent generation.
  2. *Seed Mining:* Deriving the draw seed from `SHA-256(frozen pool manifest)` allows an author or operator with pre-freeze access to manipulate whitespace, comments, or candidate order until the hash produces a favorable random draw.
* **Suggested Change:**
  - Explicitly prohibit the re-use of `live-v1-blind` task cards or any prior internal HowlFrame test cards. All candidates must be authored *de novo* by an independent third party.
  - Require an external, un-mineable public randomness beacon (e.g., the block hash of a pre-announced future Bitcoin block or NIST randomness beacon timestamp committed in `parameters.json` prior to authoring the manifest).

---

### 4. Feasibility Screen Masking Expressibility Deficits
* **Severity:** **Important**
* **Section:** §3.2 (Items 2, 5)
* **Evaluation:**
  - §3.2 item 2 excludes candidate tasks if any language cannot solve them using only standard library built-ins.
  - In prior studies (`heldout-v0`), HowlFrame suffered hard expressibility gaps on basic tasks (e.g. recursive file-tree traversal). Under §3.2 item 2, any task requiring language features or standard library routines that HowlFrame currently lacks is filtered out before the draw.
  - This directly conflicts with §3.2 item 5, which states: *"A task with no passing reference in some language is not silently dropped: it is recorded, and counted as a failure for that language in a secondary analysis."*
  - Pre-filtering the candidate pool protects HowlFrame from exposing its standard library gaps, creating an artificial playing field restricted solely to features HowlFrame has already implemented.
* **Suggested Change:**
  - Allow real-world candidate tasks from standard software benchmarks even if they challenge standard library boundaries.
  - If a task contract is implementable within idiomatic standard libraries of Python/Go/JS but HowlFrame lacks the built-in expressiveness to implement a passing reference, retain the task in the pool and record an expressibility failure (loss) for HowlFrame in both primary and secondary analyses.

---

### 5. Flawed Win/Tie Scoring Rule and Asymmetry in H2
* **Severity:** **Blocking**
* **Section:** §1 (H2), §7.2, `parameters.json`
* **Evaluation:**
  1. *Unfair Tie Rule:* §7.2 specifies: *"The win rate is wins / all pairs, so ties count against HowlFrame."* In software benchmarks with deterministic oracles, ceiling effects are common: modern models (Claude Opus, Codex) frequently produce 100% correct, verified solutions in both HowlFrame and baseline languages. If both arms achieve a 100% held-out pass rate on 50% of the tasks, HowlFrame's maximum achievable win rate is mathematically capped at 50%, guaranteeing failure of H2 (≥ 60%) despite zero correctness defects. Counting successful ties as losses is methodologically invalid and severely biased *against* HowlFrame.
  2. *Asymmetric Acceptance Requirement:* §7.2 requires an acceptance pass on the HowlFrame side to score a win, but does not impose this requirement on baselines. A baseline run failing acceptance tests could technically win if its held-out fraction is slightly higher than an uncompleted HowlFrame run.
* **Suggested Change:**
  - Align scoring with standard statistical paired-comparison methodology:
    - **Primary Metric:** Bradley-Terry / common language effect size: `Score = (Wins + 0.5 * Ties) / Total Pairs`.
    - **Alternative Metric:** Win rate over decisive pairs: `Wins / (Wins + Losses)`, with the condition that decisive pairs represent at least a minimum fraction of trials.
    - Double failures (0% / 0%) may count as losses against HowlFrame, but double successes (100% / 100%) must never count as a failure.
  - Require that for *either* arm to claim a win, its solution must achieve a 100% pass on frozen visible acceptance tests.

---

### 6. Distorted Censoring in Time-to-Verified Speed Metric (H3)
* **Severity:** **Important**
* **Section:** §1 (H3), §7.3
* **Evaluation:**
  - §7.3 specifies: *"Runs that never verify enter at the budget limit (censored)."*
  - Imputing 45 minutes to unverified runs treats an agent that completely failed or wrote garbage code as if it produced verified, correct software at the 45th minute. This conflates catastrophic task failure with slow task completion.
  - Furthermore, if more than 50% of runs fail to verify in both arms, the median of both arms collapses to 45.0 minutes, yielding an uninformative ratio of 1.0 regardless of underlying performance differences.
* **Suggested Change:**
  - Compute H3 time-to-verified primarily on **concordant verified pairs** (pairs where both HowlFrame and the baseline successfully passed verification).
  - For full-cohort time-to-event comparisons, apply standard survival analysis: evaluate the **Restricted Mean Survival Time (RMST)** over the 45-minute interval or report hazard ratios from a Cox proportional hazards model, rather than taking ratios of imputed medians.

---

### 7. Unwarranted Tightening of Runtime Performance Target (H4)
* **Severity:** **Important**
* **Section:** §1 (H4), §7.4
* **Evaluation:**
  - Directive D16 specifies: *"Runtime performance: Execution time within 2× ... Use agreed, comparable representative workloads; do not imply parity for every workload."*
  - Protocol §1 (H4) and §7.4 mandate: *"Per-task workload time ratio, HowlFrame / fastest baseline ... Refuted if Median ratio > 2.0"*.
  - Across Python, Node.js, and Go, Go is a statically compiled, highly optimized systems language whose execution times are frequently 10×–50× faster than Python or Node.js.
  - Mandating that HowlFrame (an interpreted/bytecode language) be within 2× of the *fastest* baseline forces HowlFrame to match Go, an extreme tightening beyond D16 that penalizes HowlFrame unfairly.
* **Suggested Change:**
  - Refactor H4 to evaluate runtime performance against each baseline language separately:
    - Target: HowlFrame runtime within 2× of matched interpreted/managed baselines (Python 3.12 and Node.js 22 LTS).
    - Benchmark against Go as a secondary characterization tier rather than an absolute pass/fail refutation gate for competitive application software.
  - Specify that execution time isolates workload computation from VM/runtime process startup latency (especially on sub-second workloads).

---

### 8. Statistical Pseudoreplication and Clustered Sample Sizing
* **Severity:** **Blocking**
* **Section:** §9
* **Evaluation:**
  - §9 computes that 153 matched pairs are required for power = 0.8 at one-sided $\alpha = 0.05$ (normal approximation of a binomial proportion test testing 60% vs 50%). It then states: *"At 3 repetitions × 2 providers, that is ~25 tasks...".*
  - This calculation assumes 153 independent and identically distributed Bernoulli trials.
  - In reality, running 3 repetitions × 2 providers over 25 tasks creates **clustered data** with high intra-cluster correlation (ICC): repetitions on the same task are not independent.
  - Treating 25 tasks as 153 independent trials is textbook pseudoreplication. With an estimated ICC of $\rho \approx 0.5$ on software tasks, the design effect is $1 + (m - 1)\rho = 1 + 5(0.5) = 3.5$, reducing the effective sample size from 150 to $N_{\text{eff}} \approx 43$. The study as currently sized will be drastically underpowered.
* **Suggested Change:**
  - Re-derive statistical power using a clustered design framework: either mixed-effects logistic regression with random intercepts per task or Generalized Estimating Equations (GEE).
  - Explicitly state sample size requirements in terms of **independent tasks** (likely requiring 60–80+ unique tasks across domains) rather than multiplying a small pool of 25 tasks across repetitions.

---

### 9. Documentation Asymmetry Confounders and Baseline Reference Parity
* **Severity:** **Important**
* **Section:** §4, §5
* **Evaluation:**
  - §4 grants the HowlFrame arm full access to `README.md`, `docs/cli.md`, `docs/reference/`, and skill text, while baselines receive *"None beyond the task"*.
  - While models require documentation to learn HowlFrame's syntax, this creates dual uncontrolled confounders:
    1. *Few-Shot Leakage:* HowlFrame documentation may contain idiomatic patterns, standard idioms, or code skeletons closely resembling the benchmark tasks, conferring an unearned prompt-engineering advantage.
    2. *Baseline Parity:* Real-world agentic software engineering in Python, Go, and Node.js routinely utilizes local documentation (`pydoc`, `go doc`, Node API docs). Depriving baselines of offline stdlib reference docs in their container workspace is an artificial handicap.
    3. *Cost Confounding:* HowlFrame incurs a substantial prompt token overhead on every turn, biasing token consumption and context window utilization against it.
* **Suggested Change:**
  - Place standard offline reference documentation in the container workspaces of the baseline arms (Python stdlib HTML/man, Go doc, Node.js API docs) and add identical documentation pointers to their prompt templates.
  - Establish an independent documentation audit gate: verify that frozen HowlFrame docs contain zero task-specific solutions, templates, or near-isomorphic patterns.
  - Separate initial documentation prompt tokens from iterative generation/repair tokens in all cost and token-efficiency accounting.

---

### 10. Test Suite Integrity and Workspace Tampering Vulnerability
* **Severity:** **Blocking**
* **Section:** §3.3, §4, §5
* **Evaluation:**
  - §3.3 notes that visible acceptance tests are written in Python against the contract, and §4 indicates that the container runs with broad tool/shell access in the workspace.
  - The protocol does not isolate the acceptance test files or enforce read-only filesystem permissions on the test directory.
  - Autonomous agents equipped with terminal tools (e.g. `rm`, `sed`, Python scripts) that encounter persistent test failures frequently attempt to edit, bypass, or delete the test harness itself to achieve an exit code 0. If the scoring harness evaluates the agent's modified workspace files, tampered tests would falsely register as successful completions.
* **Suggested Change:**
  - Mount acceptance test scripts and test fixtures as **read-only volumes** outside the agent's writable project workspace.
  - Require the execution harness to evaluate final code in a fresh, isolated container using the verified, pre-registered SHA-256 test suite directly from the harness repo, discarding any workspace test modifications.

---

### 11. Execution Mechanics: Undefined Completion Handshake and API Latency Confounders
* **Severity:** **Important**
* **Section:** §5, §7.3
* **Evaluation:**
  1. *Completion Signal:* §5 states: *"A run ends when the agent declares completion or a budget is exhausted."* Interactive terminal agent CLIs (Claude Code CLI, Codex CLI) do not possess a standardized native exit hook. Without a strict, programmatic completion mechanism (e.g., executing `exit-task 0` or emitting an immutable sentinel string), agents risk idling or continuing speculative tool calls until wall-clock exhaustion.
  2. *Wall-Clock vs API Jitter:* A 45-minute wall-clock budget conflates agent reasoning speed with external model provider API latency, backend queue delays, and rate-limit backoffs. An arm run during peak API traffic could time out without exhausting its tool-call budget.
* **Suggested Change:**
  - Define an explicit programmatic completion command (e.g., a CLI tool `hf-complete` invoked in the container) in `prompts/agent_task.md`.
  - Discount provider API wait time and rate-limit backoff intervals from the 45-minute budget, or measure development duration primarily via **active agent compute time** (sum of tool runtimes and agent execution turns) and tool call counts.

---

### 12. Gaps Against Directive Targets (Maintenance & Composite H6)
* **Severity:** **Important**
* **Section:** §1 (H5, H6), §6, §7.5
* **Evaluation:**
  1. *Maintenance Comparison:* Directive D15 G requires evaluating long-term maintenance tasks. §7.5 measures maintenance success (H5) solely on the HowlFrame arm (`successful changes / change requests attempted, HowlFrame arm`). It fails to compare HowlFrame's maintenance success against Python, Go, and JavaScript baselines, omitting the comparative evidence mandated by D15.
  2. *Composite Conjunctive Gate in H6:* §1 defines H6 refutation as: *"Any H fails for any provider"*. Requiring that 5 distinct hypotheses all simultaneously pass across 2 independent providers creates a 10-test conjunctive hurdle. Even if HowlFrame holds a true advantage on all metrics with individual test power 0.80, the joint probability of passing all 10 tests simultaneously is $0.80^{10} \approx 10.7\%$. This constitutes an extreme, statistically unsound kill switch not required by D16 ("Reproduce relevant results across providers; not merely two sessions of one provider").
* **Suggested Change:**
  - In §7.5, compute and report maintenance success (H5) for all four arms and calculate matched maintenance win rates against each baseline.
  - Formulate H6 as a test of cross-provider consistency (e.g., requiring that the pooled effect is significant and that neither provider shows a statistically significant contradiction), rather than a strict 10-point conjunctive pass/fail gate.

---

## Conclusion & Recommended Action Plan

Before freezing `parameters.json` and committing container digests, the protocol author must publish a **Revision 1** addressing these findings:
1. **Scoring:** Fix §7.2 so ties do not penalize HowlFrame; symmetrize acceptance pass rules; adjust H4 to compare against comparable managed baselines rather than compiled Go.
2. **Task Selection & Integrity:** Discard `live-v1-blind` tasks; adopt an external randomness beacon for seeds; institute a global cap on authority tasks (≤25% in pilot, ≤15% in full study); protect test suites from agent tampering.
3. **Statistical Power:** Re-estimate full-study sample sizes using clustered mixed-effects modeling across independent tasks rather than repeating 25 tasks.
4. **Environment Parity:** Supply offline standard library reference documentation to baseline arms and isolate documentation token costs.
