# competitive-v1: preregistered protocol, HowlFrame vs Python, Go and JavaScript

**Status: DRAFT, Revision 2. Not frozen. No trial has run, and no result exists.**
Authored 2026-10-21 by Claude Opus 5.5, the agent that also implemented HowlFrame changes in the same mission. That
conflict is why §11 requires a different-provider fairness review and an Owner budget decision before anything runs.
Revision 1 resolved the 12 findings of the first fairness review (Antigravity / Gemini, `FAIRNESS_REVIEW_R1.md`), and
Revision 2 resolves the 5 new problems found in its re-review (`FAIRNESS_REVIEW_R2.md`). Dispositions are in §12.
**Interpretation rule:** wherever this protocol could read a D16 target more loosely than its plain wording, the
**default is the plain wording**, and the looser reading is listed as an explicit Owner option (§7.8).

## 1. Research question and hypotheses

Can AI agents deliver software that is more correct, more maintainable, more reliable or cheaper to produce in
HowlFrame than in Python, Go or JavaScript? The answer is not presupposed. Each hypothesis below has a stated way to fail,
and each is computed **per baseline language and per provider**.

| ID | Hypothesis (directive D16 target) | Primary measure | Refuted if |
|---|---|---|---|
| H1 | Autonomous completion ≥ 80% in HowlFrame | Share of HowlFrame runs whose final code passes every frozen acceptance test with no human input | < 80% |
| H2 | Matched correctness wins ≥ 60% against **each** baseline | §7.2 win rate per baseline | §7.7 outcome *not met* against any one baseline |
| H3 | ≥ 20% faster verified development | §7.3 restricted mean time-to-verified, HowlFrame / baseline | Ratio > 0.8 against any baseline |
| H4 | Runtime within 2× on agreed comparable workloads | §7.4 per-workload execution-time ratio | Median ratio > 2.0 against any gated baseline (default: Python, Go and JavaScript; §7.4) |
| H5 | Maintenance success ≥ 80% | Share of HowlFrame change requests completed with no held-out regressions; compared with each baseline (§7.5) | HowlFrame success < 80% |
| H6 | Results reproduce across ≥ 2 independent providers | §7.6 consistency rule | §7.6 fails |

Secondary questions, reported without targets: security and authorization failures, diagnostic quality, code size,
tokens consumed and cost.

## 2. What existing HowlFrame evidence does and does not show (reviewed 2026-10-21)

| Material | What it measures | Why it cannot answer §1 |
|---|---|---|
| `benchmarks/language_write_cost`, `benchmarks/real_apps` | Token counts of hand-densified HowlFrame references vs idiomatic Python/Go/Node | In-sample: tasks were chosen and the language tuned around them. References, not model output. README calls these "wins" (audit HF-10) |
| `experiments/token-efficiency-v0` | 60 in-sample tasks: HowlFrame uses fewer tokens than Python on 44, ties 16 | Same in-sample caveat; source size only |
| `token-efficiency-heldout-v0..v3` | Held-out reference token counts. HowlFrame vs Python W/T/L: **v0 0/1/29** over the 30 tasks it can express (0/1/31 counting its 2 expressibility gaps), v1 16/0/18, v2 2/0/35, v3 8/0/27 after a feature slice | Held-out, which is good, but source size of references only. **The honest prior is that HowlFrame is more verbose than Python on unseen tasks** |
| `benchmarks/v2` | A runnable **Track A** verification harness (single-turn outputs), with reference results only. Track B (agentic) is described, but not automated | No model trials recorded |
| `experiments/falsifiable-v0`, `live-v1` (PR #178), `live-v1-blind` | HowlFrame programs vs a **JSON action-plan** representation on 60 + 27 filesystem tasks | Compares two representations of governed effects, not general-purpose languages. Not evidence for or against H1–H6. Its task cards were written inside the HowlFrame project and are **not reused** here (§3.2) |

No study to date has measured model success, defects, maintenance, time-to-verified or runtime across the four languages.

## 3. Tasks

### 3.1 Domains

| Domain | Kind of work | Example shape (illustrative, not a selected task) |
|---|---|---|
| D1 CLI data processing | Greenfield | Read files, validate, transform, write a report |
| D2 HTTP JSON service | Greenfield | Authenticated CRUD API with persistence and limits |
| D3 Integration / orchestration | Greenfield | Call a local HTTP dependency and run subprocesses with deadlines |
| D4 Maintenance | Change an existing application | Add a feature, fix a reported bug, or tighten validation in a provided codebase |
| D5 Full-stack | Greenfield | Small web UI plus API (full study only) |

### 3.2 Selection procedure (preregistered; must not be tuned after seeing outcomes)

1. **Candidate pool, authored de novo.** A task author **from a provider not implementing HowlFrame changes, and not
   used as a trial agent for that task**, writes new candidates. The **pilot pool** has ≥ 6 candidates for each of D1–D4.
   The **full-study pool** is authored after the pilot and frozen before its draw. It holds at least 1.5× the
   full-study task count from §9 (for example, ≥ 135 candidates for 90 tasks), with every domain represented in proportion.
   Pilot tasks are not reused in the full study. Each is a language-neutral contract:
   CLI arguments, stdin/stdout/exit codes, files, or HTTP routes, with examples. Candidates are drawn from common
   real-world task categories. **No task card from any HowlFrame-repository corpus** (`live-v1-blind`, the
   token-efficiency sets, `falsifiable-v0`, `benchmarks/`) may be used or adapted. The author attests to this, and the
   reviewer of §11 checks the pool against those corpora.
2. **Neutral feasibility screen, defined by the baselines.** A candidate is admitted if **Python, Go and JavaScript** can each
   meet it with their **standard library**, using only effects all four arms can perform in principle: files, an HTTP
   server, HTTP client, subprocess, JSON and local persistence. A candidate needing a third-party library in any
   baseline (SQL drivers, image codecs) is excluded for every language. Every exclusion is logged with its reason.
   **HowlFrame's built-ins are not part of the screen**: a task HowlFrame cannot express stays in the pool (step 5).
3. **Bias guard, global.** Tasks that *centre* on capability or authority enforcement, where HowlFrame has a built-in
   advantage, are capped at **at most 1 of the 4 pilot tasks** and **at most 15% of full-study tasks**, across all
   domains. Security properties are still tested in every task (§6), through black-box tests any language can pass by
   writing correct code. The task author labels each candidate *authority-centred* or not, and the reviewer audits the labels.
4. **Random draw from an external beacon.** The admitted pool's manifest is frozen and its SHA-256 committed **before**
   the beacon value exists. The draw seed is the first **NIST Randomness Beacon** (v2) pulse at or after a timestamp
   recorded in `parameters.json` at least 24 hours after that commit. The seed is SHA-256(beacon output value ‖ manifest
   SHA-256), and tasks are drawn per domain by a seeded shuffle, subject to step 3's cap. Nobody can choose the seed or
   iterate the manifest against it.
5. **Reference feasibility proof.** For each drawn task, a reference implementation in each language is written by
   someone other than the trial agents, and must pass the frozen tests. It calibrates the runtime workload and is
   never shown to trial agents. **If no passing HowlFrame reference can be written** for a task the baselines meet, the task
   stays in the study. Every HowlFrame run on it is scored as a failure **in the primary analysis**, and the gap is
   reported as an expressibility finding. A task with no passing reference in a *baseline* language is a screen error:
   it is reported, and replaced by the next task in the seeded order.
6. **Maintenance baselines (D4).** For each D4 task, a baseline application is implemented in all four languages by
   one independent author, to one spec, passing the same acceptance tests, with comparable size and structure. Change
   requests are written against the spec, not against any one implementation.

### 3.3 Tests per task

* **Frozen acceptance tests** (visible to the agent): black-box, language-agnostic, written in Python against the
  contract (subprocess or HTTP). They are frozen and hashed before trials, and mounted **read-only, outside the agent's
  writable workspace** (§4).
* **Held-out tests** (never visible): written by a **different** author and provider from the acceptance tests.
  They cover the same contract with edge, failure, boundary, concurrency, authorization and malformed-input cases.
  They are sealed with a published SHA-256 and opened only for scoring.
* **Runtime workload**: one fixed input set per task, sized so the reference runs 0.5–5 s, excluding process startup.
* **Scoring isolation**: final code is copied out of the agent's workspace and scored in a **fresh container** against the
  hash-verified tests from the harness repository. Any agent modification of tests or fixtures is ignored for scoring
  and reported as a tampering attempt.

## 4. Arms and environment

| Arm | Toolchain (pinned at freeze) | Allowed dependencies | Documentation available offline in the container |
|---|---|---|---|
| HowlFrame | Compiler at a recorded merged `main` commit | Built-ins only | `README.md`, `docs/cli.md`, `docs/reference/`, the HowlFrame skill text |
| Python | CPython 3.12.x | Standard library | The CPython 3.12 library reference (offline HTML or text) and `pydoc` |
| Go | Go 1.25.x | Standard library | `go doc` and the standard-library package documentation |
| JavaScript | Node.js 22.x LTS | Built-in modules | The Node.js 22 API documentation (offline) |

* Every arm's prompt names its documentation location identically. HowlFrame needs its documentation to be usable at
  all; baselines get theirs for parity.
* **Documentation audit gate (before freeze):** an independent reviewer confirms that no arm's documentation contains a
  solution, template or near-isomorphic example for any drawn task. The audit is recorded.
* One container image per arm, identical apart from the toolchain and documentation: same OS, CPU and memory limits,
  and no network except the model API (and loopback for the task). The task workspace starts empty, or holds the D4
  baseline. Acceptance tests are a read-only mount.
* Every arm runs under the same OS-level limits. HowlFrame capability grants are additional, not a substitute.
  Security scoring (§6) is black-box and identical for all arms.

## 5. Agents, providers and budgets

* **Providers:** at least two independent providers, for example Anthropic (Claude Code CLI) and OpenAI (Codex
  CLI). Two sessions of one provider do not satisfy H6. Exact model IDs and versions are recorded at freeze.
* **Instructions:** one template for every arm, differing only in the language name, toolchain commands and
  documentation location (`prompts/agent_task.md`, frozen with the tasks).
* **Completion handshake:** the container provides a `task-complete` command. A run ends when the agent runs it, or when
  a budget is exhausted. Agent output after `task-complete` is ignored.
* **Budget per run (identical across arms):** 45 minutes wall clock, 150 tool calls, and the provider's token ceiling for
  one session. Exhausting any budget ends the run as *not completed*. Provider wait time (request latency, rate-limit
  backoff) is logged per call. Runs of all four arms for a given (task, provider, repetition) are launched **together**,
  so provider load affects arms equally.
* **Repetitions:** 3 per (task, language, provider). Neither CLI exposes a sampling seed, so repetitions are
  independent sessions; launch order across tasks is randomized.
* **Retry, repair and stop rules:**
  * The agent may run the visible acceptance tests and its own tools as often as the budget allows. It never sees
    held-out tests, references, or other arms' outputs.
  * There is no second attempt.
  * **Infrastructure failure** (harness crash, provider outage, container fault, all logged) allows one rerun of that
    run only, and of its paired runs in the other arms if they were affected. A second infrastructure failure makes the
    run *not completed*. Language-specific failures are never infrastructure failures.
  * **Human intervention** of any kind marks the run *not autonomous* (it fails H1). The intervention is logged.

## 6. Measures

| Group | Measure | Definition |
|---|---|---|
| Correctness | Acceptance pass | All frozen acceptance tests pass on the final code, scored in isolation (§3.3) |
| | Held-out pass fraction | Passing held-out tests / total held-out tests |
| | Defects | Distinct held-out failures, grouped by root cause by a blind grader |
| | Security / authorization failures | Held-out cases for authorization bypass, path escape, injection and secret leakage. Canary files and loopback canary listeners detect any access outside the contract, for every arm |
| | Tampering attempts | Any agent change to tests or fixtures (§3.3) |
| Productivity | Time to verified | Wall time from start until visible tests pass and `task-complete` runs (§7.3); also active time (wall time minus logged provider wait) |
| | Tool calls, iterations, tokens, cost | From CLI logs. Documentation-reading tokens (tool results that read the documentation) are reported separately from generation and repair tokens. Cost is at published list price where the provider reports tokens |
| | Human interventions | Count (target 0) |
| Maintainability (D4) | Change success | Change request implemented and all held-out tests (old and new behavior) pass, **for every arm** |
| | Regressions | Previously passing held-out tests that fail after the change |
| | Diagnostic quality | Blind graders rate the first compiler or runtime error each agent hit (1–5 rubric frozen with the tasks) |
| Runtime | Execution time, peak RSS | On the frozen workload: median of 5 runs after 1 warm-up, same container limits, measured from program start to exit **minus** a measured empty-program startup for that arm |
| | Startup | Empty-program startup per arm, reported separately |
| | Resource-limit behavior | Behavior when the workload is 10× the frozen size (crash, clean error, or success) |

## 7. Scoring rules

7.1 **Completion (H1, per provider):** completed runs / all runs, including runs ended by budget or intervention.

7.2 **Matched correctness (H2):** pair each HowlFrame run with the baseline run of the same task, provider and repetition
index. A side that fails its visible acceptance tests is scored at a held-out fraction of 0 for this comparison; the
rule is symmetric. A **win** means a strictly higher (adjusted) held-out fraction. A **shared success** means equal,
non-zero fractions. A **mutual failure** means both at 0. Anything else is a loss.
* **Default H2 metric (the plain reading of D16, "matched correctness wins"):** strict win rate = wins / pairs.
* **Owner option A:** shared-credit rate = (wins + ½ shared successes) / pairs. Mutual failures earn **no** credit.
  This avoids penalizing HowlFrame when both arms are perfect.
* **Owner option B:** decisive rate = wins / (wins + losses).
All three are always reported, with W / shared / mutual-failure / L counts and task-clustered intervals (§9). Only the
metric selected before freeze decides H2 (§7.7).

7.3 **Speed (H3):** restricted mean time-to-verified (RMST) over the 45-minute window, per arm, with unverified runs
censored at 45 minutes. The ratio is HowlFrame RMST / baseline RMST. Secondary: median time-to-verified over
*concordant verified pairs* (both arms verified), and the same on active time.

7.4 **Runtime (H4):** per workload, the HowlFrame execution time divided by each baseline's, using both the reference and
the agent-produced implementations. **Default gate: Python, Go and JavaScript** (the plain reading of D16). **Owner
option C:** gate on Python and JavaScript only, and report Go as a characterization. This treats an interpreted VM
measured against compiled Go as outside "agreed comparable workloads". It applies only if ratified before freeze.

7.5 **Maintenance (H5):** successful changes / change requests attempted, per arm. H5's threshold applies to HowlFrame.
The matched maintenance score against each baseline (computed as in §7.2) is reported as a secondary comparative result.

7.6 **Provider consistency (H6):** each of H1–H5 is computed per provider and pooled. H6 holds for a hypothesis if the
**pooled** estimate meets its target **and** no provider's estimate falls on the opposite side of the target with its
interval excluding the target. Results are reported per provider as well; no conjunctive pass across all hypotheses is required.

7.7 **Decision outcomes (H1–H5).** Each hypothesis gets one of three preregistered outcomes, per baseline where
applicable, using task-clustered one-sided 95% intervals Bonferroni-adjusted across the three baselines (α = 0.05/3 each):
* **Met:** the point estimate meets the target, **and** the interval excludes the no-advantage value (for H2: lower
  bound > 0.50; for H1 and H5: lower bound > 0.70; for H3: upper bound of the RMST ratio < 1.0; for H4: upper bound of
  the median ratio < 3.0).
* **Not met:** the interval lies entirely on the wrong side of the target (for H2: upper bound < 0.60).
* **Inconclusive:** anything else. It is reported as such, never rounded to met or not met.

7.8 **Owner options** (must be chosen before freeze; otherwise the defaults apply): A or B for the H2 metric
(default: strict win rate); C for the H4 gate (default: all three baselines). The choice is recorded in `parameters.json`.

7.9 **No exclusions after the fact.** Every run is reported: failures, budget exhaustion, infrastructure reruns,
interventions and tampering attempts. A task cannot be removed after the draw, except under §3.2 step 5's baseline-reference
rule. If a task proves defective (an acceptance test contradicts its spec), it is fixed for **all** arms and rerun for
**all** arms, and both results are published.

## 8. Pilot (bounded; methodology check, not evidence)

* **Tasks:** 4, one each from D1, D2, D3 and D4, drawn by §3.2, with at most 1 authority-centred.
* **Cells:** 4 tasks × 4 languages × 2 providers × 3 repetitions = **96 runs**; 24 matched pairs per baseline, from only 4
  independent tasks.
* **Purpose:** find methodology defects (leaky tests, documentation asymmetry, harness bias, budget miscalibration, grader
  disagreement, tampering), and **estimate the intra-task correlation** needed for §9. With 4 tasks, no interval can
  support a hypothesis decision. **The pilot cannot confirm or refute any hypothesis, and must not be reported as doing so.**
* **Budget:** up to 96 × 45 min ≈ 72 agent-hours wall clock (parallelizable). Token cost is to be estimated from a 4-run dry
  run, one per language with one provider, before the Owner approves the cap.
* **Exit:** a written methodology review, then either a revised protocol (re-reviewed) or approval of the full study.

## 9. Full study (sizing, after the pilot)

Runs on the same task are correlated, so tasks, not runs, are the independent units. H2 is analysed with a mixed-effects
logistic model (random intercepts per task, provider as a fixed effect), or with generalized estimating equations
clustered by task. The intervals in §7 use the same clustering.

Sizing targets a 0.8 probability of a **Met** outcome (§7.7) against **all three baselines jointly**, so 0.928 power per
baseline, at one-sided α = 0.05/3, when HowlFrame's true win rate is p₁. i.i.d. pairs needed: p₁ = 0.65 → 138;
p₁ = 0.70 → 75; p₁ = 0.75 → 46. With m = 6 runs per task (3 repetitions × 2 providers) and intra-task correlation ρ, the
design effect is 1 + (m − 1)ρ, giving independent tasks of:

| True win rate p₁ | ρ = 0.3 | ρ = 0.5 |
|---|---|---|
| 0.65 | 57 | 80 |
| 0.70 | 31 | 44 |
| 0.75 | 19 | 27 |

The planning range is **45–80 tasks** (p₁ between 0.65 and 0.70), with at least 15% of them D4. The final count is
re-derived from the pilot's ρ estimate and preregistered as a revision before the full study. A true win rate of exactly
0.60 cannot be reliably distinguished from just below it at any feasible size; the three-outcome rule (§7.7) reports
such cases as inconclusive rather than forcing a verdict. Adding repetitions does not substitute for adding tasks.

## 10. Reporting

* Raw logs, final code, test outputs and scores for every run are published. The report leads with losses and with
  hypotheses that failed.
* The report separates "implemented improvement" from "evaluation result", and never cites pilot numbers as D16 evidence.
* Any later change to HowlFrame made in response to trial outcomes invalidates comparison with earlier runs. It needs a
  new preregistered round on newly drawn tasks.

## 11. Before any trial (gates)

1. **Fairness re-review** of this Revision 2 by a provider different from its author, limited to the R2 findings. Every
   remaining blocking finding is resolved in a recorded revision.
2. **Owner decisions:** providers and model IDs, options A/B/C (§7.8; defaults otherwise), the spend cap for the dry run
   and pilot, and approval of this protocol.
3. The candidate pool, frozen tests, sealed held-out hashes, references, documentation audit and container digests are
   committed. Then `parameters.json` is completed (including the beacon timestamp) and its hash recorded as the freeze.
4. HowlFrame is pinned to a merged commit. No HowlFrame change may land between freeze and the end of the pilot.

None of these gates has been passed as of this draft.

## 12. Disposition of fairness review R1 (Antigravity / Gemini, 2026-10-21)

| # | Finding (severity) | Disposition |
|---|---|---|
| 1 | §2 misstated held-out v0 and the v2 Track B harness (important) | Accepted; corrected (verified against the reports) |
| 2 | Per-domain authority cap allowed 4/4 in the pilot (blocking) | Accepted; global cap: ≤ 1 of 4 in the pilot, ≤ 15% in the full study (§3.2.3) |
| 3 | Reuse of in-project task cards; seed mining (blocking) | Accepted; de novo tasks only, with a reuse check; seed from a NIST beacon pulse after the manifest freeze (§3.2.1, §3.2.4) |
| 4 | The screen hid HowlFrame expressibility gaps (important) | Accepted; the screen is defined by the baselines, and HowlFrame inexpressibility is a primary-analysis failure (§3.2.2, §3.2.5) |
| 5 | Ties counted as losses; asymmetric acceptance rule (blocking) | Accepted; (W + ½T)/N primary, symmetric acceptance rule, strict and decisive rates reported (§7.2) |
| 6 | Censored median conflated failure with slowness (important) | Accepted; RMST primary, concordant-pair medians secondary (§7.3) |
| 7 | "Fastest baseline" made Go the runtime gate (important) | Accepted in part: the gate is Python and JavaScript, Go reported, and the interpretation goes to the Owner (§7.4) |
| 8 | Pseudoreplication in sizing (blocking) | Accepted; task-clustered analysis, and sizing in independent tasks, 60–90 (§9) |
| 9 | Documentation asymmetry; token confounding (important) | Accepted; offline stdlib documentation for baselines, a documentation audit gate, documentation tokens reported separately (§4, §6) |
| 10 | Agents could edit acceptance tests (blocking) | Accepted; read-only mount, isolated scoring, tampering reported (§3.3, §4) |
| 11 | No completion handshake; API latency (important) | Accepted; `task-complete`, co-scheduled arms, logged provider wait and active time (§5, §6) |
| 12 | Maintenance only for HowlFrame; conjunctive H6 (important) | Accepted; maintenance for all arms with a matched comparison, and H6 as a consistency rule (§7.5, §7.6) |

### Fairness re-review R2 (Antigravity / Gemini): R1 findings 5, 7 and 8 partially resolved; 5 new problems

| # | Problem (R2) | Disposition in Revision 2 |
|---|---|---|
| R2-1 | Pool of ≥ 6 per domain cannot supply 60–90 tasks | Separate pilot and full-study pools; the full-study pool is ≥ 1.5× the task count, authored after the pilot (§3.2.1) |
| R2-2 | Sizing tested superiority over 0.5 with a point-estimate refutation rule (≈ 50% false refutation at 0.60), ignoring three baselines | Three-outcome decision rule with Bonferroni α/3 (§7.7); sizing for 0.8 joint power at stated true effects, with the table in §9 |
| R2-3 | Mutual failures (0/0) earned half credit | Mutual failures earn no credit in any metric (§7.2) |
| R2-4 | (W + ½T)/N diverged from D16's "matched correctness wins" wording | Default is the strict win rate; shared credit is Owner option A, decisive rate option B (§7.2, §7.8) |
| R2-5 | Go was exempted from the runtime gate without ratification | Default gate is all three baselines; the Go exemption is Owner option C (§7.4, §7.8) |

