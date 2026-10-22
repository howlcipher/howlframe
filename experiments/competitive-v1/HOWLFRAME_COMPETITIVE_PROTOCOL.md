# competitive-v1: preregistered protocol, HowlFrame vs Python, Go and JavaScript

**Status: DRAFT for fairness review. Not frozen. No trial has run, and no result exists.**
Authored 2026-10-21 by Claude Opus 5.5, the agent that also implemented HowlFrame changes in the same mission. That
conflict is why §11 requires a different-provider fairness review and an Owner budget decision before anything runs.

## 1. Research question and hypotheses

Can AI agents deliver software that is more correct, more maintainable, more reliable or cheaper to produce in
HowlFrame than in Python, Go or JavaScript? The answer is not presupposed. Each hypothesis below has a stated way to fail.

| ID | Hypothesis (directive D16 target) | Primary measure | Refuted if |
|---|---|---|---|
| H1 | Autonomous completion ≥ 80% in HowlFrame | Share of HowlFrame runs whose final code passes every frozen acceptance test with no human input | < 80% |
| H2 | Matched correctness wins ≥ 60% against **each** baseline | §7.2 win rate per baseline language | < 60% against any one baseline |
| H3 | ≥ 20% faster verified development | Ratio of median time-to-verified, HowlFrame / baseline | Ratio > 0.8 against any baseline |
| H4 | Runtime within 2× | Per-task workload time ratio, HowlFrame / fastest baseline | Median ratio > 2.0 |
| H5 | Maintenance success ≥ 80% | Share of change requests completed with no held-out regressions | < 80% |
| H6 | Holds for ≥ 2 independent providers | H1–H5 computed per provider | Any H fails for any provider |

Secondary questions, reported without targets: security and authorization failures, diagnostic quality, code size,
tokens consumed and cost.

## 2. What existing HowlFrame evidence does and does not show (reviewed 2026-10-21)

| Material | What it measures | Why it cannot answer §1 |
|---|---|---|
| `benchmarks/language_write_cost`, `benchmarks/real_apps` | Token counts of hand-densified HowlFrame references vs idiomatic Python/Go/Node | In-sample: tasks were chosen and the language tuned around them. References, not model output. README calls these "wins" (audit HF-10) |
| `experiments/token-efficiency-v0` | 60 in-sample tasks: HowlFrame fewer tokens than Python on 44, ties 16 | Same in-sample caveat; source size only |
| `token-efficiency-heldout-v0..v3` | Held-out reference token counts. HowlFrame vs Python W/T/L: v0 1/0/29 (wins on 1 of 30 tasks it can express, ties 1), v1 16/0/18, v2 2/0/35, v3 8/0/27 after a feature slice | Held-out, which is good, but source size of references only. **The honest prior is that HowlFrame is more verbose than Python on unseen tasks** |
| `benchmarks/v2` Track A/B harness | A runnable harness, reference results only | No model trials recorded |
| `experiments/falsifiable-v0`, `live-v1` (PR #178), `live-v1-blind` | HowlFrame programs vs a **JSON action-plan** representation on 60 + 27 filesystem tasks | Compares two representations of governed effects, not general-purpose languages. Not evidence for or against H1–H6. `live-v1-blind`'s task cards are language-neutral and may seed one domain here (§3.2) |

No study to date has measured model success, defects, maintenance, time-to-verified or runtime across the four languages.

## 3. Tasks

### 3.1 Domains

Tasks come from realistic software domains, with maintenance weighted as heavily as greenfield work:

| Domain | Kind of work | Example shape (illustrative, not a selected task) |
|---|---|---|
| D1 CLI data processing | Greenfield | Read files, validate, transform, write a report |
| D2 HTTP JSON service | Greenfield | Authenticated CRUD API with persistence and limits |
| D3 Integration / orchestration | Greenfield | Call a local HTTP dependency and run subprocesses with deadlines |
| D4 Maintenance | Change an existing application | Add a feature, fix a reported bug, or tighten validation in a provided codebase |
| D5 Full-stack | Greenfield | Small web UI plus API (full study only) |

### 3.2 Selection procedure (preregistered; must not be tuned after seeing outcomes)

1. **Candidate pool.** A task author **from a provider not implementing HowlFrame changes, and not otherwise used as a
   trial agent for that task**, writes ≥ 6 candidates per domain. Each is a language-neutral contract: CLI arguments,
   stdin/stdout/exit codes, files, or HTTP routes, with examples. Candidates are drawn from common real-world tasks
   (public issue trackers, standard exercise categories). `live-v1-blind` cards may be included in D1 after removing
   any HowlFrame-specific grant wording.
2. **Neutral feasibility screen.** A candidate is admitted only if all four languages can meet it **with the standard
   library** (HowlFrame: documented built-ins, no unreleased features), using only effects every arm can perform:
   files, an HTTP server, HTTP client, subprocess, JSON and local persistence. A candidate needing a third-party
   library in any language (SQL drivers, image codecs) is excluded **for every language**. Every exclusion is logged
   with its reason. This screen deliberately admits tasks where HowlFrame has no built-in advantage. It also
   deliberately excludes features irrelevant to the application (for example GUI toolkits).
3. **Bias guards.** No more than one task per domain may *centre* on capability or authority enforcement, the
   area where HowlFrame has a built-in advantage. Security properties are still tested in every task (§6) through
   black-box tests any language can pass by writing correct code.
4. **Random draw.** From the admitted pool, draw tasks per domain with a seeded shuffle. The seed is the SHA-256 of the
   frozen pool manifest. It is published before the draw and cannot be chosen.
5. **Reference feasibility proof.** For each drawn task, a reference implementation in each of the four languages is
   written by someone other than the trial agents, and must pass the frozen tests. It proves the task is
   achievable and calibrates the runtime workload. References are never shown to trial agents. A task with no passing
   reference in some language is **not silently dropped**: it is recorded, and counted as a failure for that language
   in a secondary analysis.
6. **Maintenance baselines (D4).** For each D4 task, a baseline application is implemented in all four languages by
   one independent author, to one spec, passing the same acceptance tests, with comparable size and structure. Change
   requests are written against the spec, not against any one implementation.

### 3.3 Tests per task

* **Frozen acceptance tests** (visible to the agent): black-box, language-agnostic, written in Python against the
  contract (subprocess or HTTP). Frozen and hashed before trials.
* **Held-out tests** (never visible): written by a **different** author and provider from the acceptance tests.
  They cover the same contract with edge, failure, boundary, concurrency, authorization and malformed-input cases.
  They are sealed with a published SHA-256 and opened only for scoring.
* **Runtime workload**: one fixed input set per task, sized so the reference runs 0.5–5 s.

## 4. Arms and environment

| Arm | Toolchain (pinned at freeze) | Allowed dependencies | Documentation available to the agent |
|---|---|---|---|
| HowlFrame | Compiler at a recorded commit (a merged `main`, not a PR head) | Built-ins only | `README.md`, `docs/cli.md`, `docs/reference/`, the HowlFrame skill text. The model cannot know the language otherwise; this is part of the treatment and is reported |
| Python | CPython 3.12.x | Standard library | None beyond the task |
| Go | Go 1.25.x | Standard library | None beyond the task |
| JavaScript | Node.js 22.x LTS | Built-in modules | None beyond the task |

* One container image per arm, identical apart from the toolchain: same OS, CPU and memory limits, and no network
  except the model API (and loopback for the task). The task workspace starts empty, or holds the D4 baseline.
* Every arm runs under the same OS-level limits. HowlFrame capability grants are additional, not a substitute.
  Security scoring (§6) is black-box and identical for all arms.

## 5. Agents, providers and budgets

* **Providers:** at least two independent providers, for example Anthropic (Claude Code CLI) and OpenAI (Codex
  CLI). Two sessions of one provider do not satisfy H6. Exact model IDs and versions are recorded at freeze.
* **Instructions:** one template for every arm, differing only in the language name, toolchain commands and
  documentation pointer (`prompts/agent_task.md`, to be written and frozen with the tasks).
* **Budget per run (identical across arms):** 45 minutes wall clock, 150 tool calls, and the provider's token
  ceiling for one session. Exhausting any budget ends the run as *not completed*.
* **Repetitions:** 3 per (task, language, provider). Neither CLI exposes a sampling seed, so repetitions are
  independent sessions, run in randomized order.
* **Retry, repair and stop rules:**
  * The agent may run the visible acceptance tests and its own tools as often as the budget allows. It never sees
    held-out tests, references, or other arms' outputs.
  * A run ends when the agent declares completion or a budget is exhausted. There is no second attempt.
  * **Infrastructure failure** (harness crash, provider outage, container fault, all logged) allows one rerun of that
    run only. A second infrastructure failure makes the run *not completed*. Language-specific failures are never
    infrastructure failures.
  * **Human intervention** of any kind marks the run *not autonomous* (it fails H1). The intervention is logged.

## 6. Measures

| Group | Measure | Definition |
|---|---|---|
| Correctness | Acceptance pass | All frozen acceptance tests pass on the final code |
| | Held-out pass fraction | Passing held-out tests / total held-out tests |
| | Defects | Distinct held-out failures, grouped by root cause by a blind grader |
| | Security / authorization failures | Held-out cases for authorization bypass, path escape, injection and secret leakage. Canary files and loopback canary listeners detect any access outside the contract, for every arm |
| Productivity | Time to verified | Wall time from start until the visible tests pass and the agent declares done (censored at budget) |
| | Tool calls, iterations, tokens, cost | From CLI logs; cost at published list price where the provider reports tokens |
| | Human interventions | Count (target 0) |
| Maintainability (D4) | Change success | Change request implemented and all held-out tests (old and new behavior) pass |
| | Regressions | Previously passing held-out tests that fail after the change |
| | Diagnostic quality | Blind graders rate the first compiler or runtime error each agent hit (1–5 rubric frozen with the tasks) |
| Runtime | Execution time, peak RSS, startup | On the frozen workload: median of 5 runs after 1 warm-up, same container limits |
| | Resource-limit behavior | Behavior when the workload is 10× the frozen size (crash, clean error, or success) |

## 7. Scoring rules

7.1 **Completion (H1, per provider):** completed runs / all runs, including runs ended by budget or intervention.

7.2 **Matched correctness win (H2):** pair each HowlFrame run with the baseline run of the same task, provider and
repetition index. A **win** means a strictly higher held-out pass fraction, with an acceptance pass required on the
HowlFrame side. A **tie** means equal fractions. Anything else is a **loss**. The win rate is wins / all pairs, so
**ties count against HowlFrame**. Report W/T/L and a Wilson 95% interval for each baseline separately.

7.3 **Speed (H3):** median time-to-verified, HowlFrame / baseline. Runs that never verify enter at the budget limit (censored).

7.4 **Runtime (H4):** per task, HowlFrame time / fastest baseline reference *and* / fastest agent-produced baseline.
Report both.

7.5 **Maintenance (H5):** successful changes / change requests attempted, HowlFrame arm.

7.6 **No exclusions after the fact.** Every run is reported: failures, budget exhaustion, infrastructure reruns and
interventions. A task cannot be removed after the draw. If a task proves defective (an acceptance test contradicts
its spec), it is fixed for **all** arms and rerun for **all** arms, and both results are published.

## 8. Pilot (bounded; methodology check, not evidence)

* **Tasks:** 4, one each from D1, D2, D3 and D4, drawn by §3.2.
* **Cells:** 4 tasks × 4 languages × 2 providers × 3 repetitions = **96 runs**; 24 matched pairs per baseline.
* **Purpose:** find methodology defects: leaky tests, unfair documentation, harness bias, budget miscalibration,
  grader disagreement. With 24 pairs per baseline, a Wilson 95% interval has a half-width of about ±0.19 at a 50% win rate. **The pilot cannot
  confirm or refute any hypothesis, and must not be reported as doing so.**
* **Budget:** up to 96 × 45 min ≈ 72 agent-hours wall clock (parallelizable). Token cost is to be estimated from a
  4-run dry run, one per language with one provider, before the Owner approves the cap.
* **Exit:** a written methodology review, then either a revised protocol (re-reviewed) or approval of the full study.

## 9. Full study (sizing, after the pilot)

To test H2 at 60% against a null of 50% with one-sided α = 0.05 and power 0.8, about **153 matched pairs per baseline**
are needed (normal approximation). At 3 repetitions × 2 providers, that is ~25 tasks, of which at least 6 are D4. Exact sizing is re-derived
from pilot variance and preregistered as a revision before the full study.

## 10. Reporting

* Raw logs, final code, test outputs and scores for every run are published. The report leads with losses and with
  hypotheses that failed.
* The report separates "implemented improvement" from "evaluation result", and never cites pilot numbers as D16 evidence.
* Any later change to HowlFrame made in response to trial outcomes invalidates comparison with earlier runs. It needs a
  new preregistered round on newly drawn tasks.

## 11. Before any trial (gates)

1. **Fairness review** of this protocol by a provider different from its author, focused on task selection,
   documentation asymmetry, budgets and scoring. Its findings are resolved in a recorded revision.
2. **Owner decisions:** providers and model IDs, the spend cap for the pilot dry run and pilot, and approval of this protocol.
3. Task pool, frozen tests, sealed held-out hashes, references and container digests are committed. Then
   `parameters.json` is completed and its hash recorded as the freeze.
4. HowlFrame is pinned to a merged commit. No HowlFrame change may land between freeze and the end of the pilot.

None of these gates has been passed as of this draft.
