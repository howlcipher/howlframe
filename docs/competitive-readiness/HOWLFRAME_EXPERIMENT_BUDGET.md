# HowlFrame competitive-v1: experiment budget

Prepared 2026-10-23. **These are estimates and proposals. No spend is authorized.** Prices were checked on 2026-10-23
(sources: `HOWLFRAME_MODEL_SELECTION.md` §1). All amounts are USD at Standard/global list prices, with no batch discount, fast
tier or data-residency uplift.

## 1. What the estimate rests on

**Per-run limits from the protocol (§5):** 45 minutes wall clock, 150 tool calls, and a "provider token ceiling" that the protocol does
not define. Readiness gap G2 proposes a dollar ceiling per run in its place.

**Empirical token profile.** I computed it from this host's own agent logs, numeric `usage` fields only, so no content was read.
The script is `evidence/usage_stats.py` and `evidence/codex_stats.py`.

| Source | Sessions | Calls per session (p50 / p90) | Average context per call (p50 / p90) | Output per call (p50 / p90) | Cache write per call (p50 / p90) |
|---|---|---|---|---|---|
| Claude Code, Opus-family sessions with 20–200 API calls | 20 | 103 / 165 | 166K / 243K | 1.05K / 1.42K | 2.7K / 5.0K |
| Codex sessions with 20–200 tool calls (mostly `gpt-5.5`, `gpt-6.1-sol`) | 48 | 47 / 101 | n/a | n/a | n/a |

Repriced at today's list prices:

| Repriced as | p50 / session | p90 / session | Max / session | Scaled to 150 calls: p50 / p90 / max |
|---|---|---|---|---|
| `claude-opus-5-5` | $7.44 | $14.05 | $16.77 | $10.48 / $13.51 / $15.24 |
| `gpt-6.1-sol` (short-context rates only) | $1.56 | $8.23 | $39.29 | $7.38 / $28.37 / **$168.38** |

**Caveats.** These are orchestration sessions on this host, not competitive-v1 trials. They carry a much larger system prompt than a
`--bare` trial would, and most ran far longer than 45 minutes, so they probably **overstate** a typical trial. The Codex repricing
ignores the >272K long-context rate, so its tail is **understated**. The Codex tail is heavy, and Codex has no client-side budget flag.
For that reason the cap must be **enforced, not estimated**.

## 2. Per-run cost scenarios

| Scenario | Assumptions | `claude-opus-5-5` | `gpt-6.1-sol` |
|---|---|---|---|
| Typical | 60 calls, 100K average context, ~97% cache hits, 1.2K output per call | ≈ $3.50 | ≈ $2 |
| High | 150 calls, 243K average context, 5K cache write and 1.4K output per call (Codex: a third of calls over 272K) | ≈ $15 | ≈ $12 |
| Pathological | High, plus ~10 full cache rewrites of a 300K context (5-minute cache expiry while a long test runs), or a Codex long-context loop | ≈ $30 | ≈ $30+ (unbounded without enforcement) |

The 45-minute wall clock usually ends a run before 150 calls. Retries after 429 or 5xx errors are not billed for failed requests, but they
consume wall clock. Infrastructure reruns (at most one per run, protocol §5) are counted separately below.

**Proposed per-run ceiling: $30**, the same for every arm of a provider. A run that reaches it ends as *not completed: budget
exhausted* and is reported, never dropped (protocol §7.9). The ceiling should bind rarely. If it binds in more than 1 of 4 feasibility
runs, budget calibration is the first methodology finding.

## 3. A. Four-run feasibility test (the next paid experiment)

One provider, one task, four languages (HowlFrame, Python, Go, JavaScript), one run each.

| Item | `claude-opus-5-5` (recommended provider) | `gpt-6.1-sol` (alternative) |
|---|---|---|
| Estimated tokens, all 4 runs | ≈ 24–146M input (≥ 95% cache reads), 0.3–0.8M output | Similar order. Tokenizers differ |
| Expected model cost | **$14–$60** | **$8–$48** |
| Ceiling-bounded worst case (4 × $30) | $120 | $120 |
| Infrastructure-rerun reserve (one rerun) | $30 | $30 |
| Execution and compute charges | $0 incremental (local host; no cloud runners; server-side web search disabled) | $0 incremental |
| **Recommended hard cap** | **$150** | **$150** |

## 4. B. 96-run pilot (a separate, later authorization)

4 tasks × 4 languages × 2 providers × 3 repetitions = 48 runs per provider.

| Item | Estimate |
|---|---|
| Expected | Claude 48 × ~$3.50–15 ≈ $170–720; Codex 48 × ~$2–12 ≈ $100–580; **total ≈ $270–1,300 (central ≈ $600)** |
| Ceiling-bounded worst case | 96 × $30 = $2,880 |
| Infrastructure-rerun reserve (10%) | $290 |
| **Conservative upper bound** | **≈ $3,170** |
| Compute | $0 incremental on the local host; 96 × 45 min ≈ 72 agent-hours, run 4 at a time ≈ 18 h wall clock plus scoring |
| Recommended cap | **Do not approve yet.** Set it after the feasibility test, from observed per-run cost: cap = 96 × min($30, 2 × observed max) + 10%. Provisional exposure limit if the $30 ceiling is kept: **$3,200** |

## 5. How the cap is actually enforced

A written budget is not enforcement. Four independent layers are proposed. All of them are experiment-only infrastructure that does
**not exist yet** (Owner decision 6).

1. **Metering proxy (primary, provider-neutral).** A loopback reverse proxy that both CLIs must use (`ANTHROPIC_BASE_URL`; a Codex
   `model_providers` entry with `base_url`). The trial jail has no other network route. For every response it computes cost from the
   returned `usage` and a pinned price table, then appends it to a ledger, fsynced per call. **Before forwarding** each request it checks:
   * `run_spent + worst_single_call ≤ run_ceiling` ($30). The worst single call is about $8 for Opus 5.5: 1M uncached input + 128K output.
   * `experiment_spent + worst_single_call ≤ experiment_cap`.
   * The run's tool-call count is below 150 (counted from returned tool calls).
   * The request declares no server-side web tool and names the pinned model.

   On any failure it returns an error without contacting the provider. The CLI stops, and the harness records the reason. Because
   of the worst-call reserve, spend **cannot exceed** the cap even when the last call is maximal.
2. **Harness admission.** A run starts only if `cap − spent ≥ run_ceiling`. That reserves the whole run up front, so a run can never be
   cut short by the experiment cap, only by its own ceiling.
3. **Client-side second limit (Claude only).** `claude -p --max-budget-usd 30`. Codex has no equivalent, so layer 1 is mandatory for it.
4. **Provider-side backstop.** A dedicated Anthropic workspace with a spend limit and a dedicated OpenAI project with a budget limit, both
   set no higher than the cap. Use prepaid credit with auto-reload **off**, loaded with no more than the cap plus $10. If a provider limit
   turns out to be advisory only, the prepaid balance is the hard backstop.

**Stop condition.** When the proxy refuses a request on the experiment cap, or admission fails, the harness stops launching runs. In-flight
runs are marked *not completed: budget*, it writes the final ledger and exits nonzero. Continuing requires a **new written Owner
authorization** with a new cap. There is no override flag.

**Verification before any paid call (non-billable).** Run the proxy against a deterministic mock upstream that returns scripted `usage`
blocks, and check that: the run ceiling stops a run; the experiment cap stops the queue; the worst-call reserve prevents overshoot; web-tool
and wrong-model requests are refused; and the ledger survives a proxy restart. Label this an infrastructure check, not a result.

## 6. Uncertainty, disclosed

* No competitive-v1 trial has run, so per-run token use is extrapolated from unrelated sessions. The feasibility test exists to replace
  that extrapolation with measurements.
* OpenAI's per-token prices are primary-source, but OpenAI's cache-write billing (listed at $2.50/MTok for 30-minute writes) depends on
  how Codex marks cache breakpoints. Expect ±30% on the Codex estimates.
* HowlFrame arms may read more documentation, about 45K tokens for `README.md`, `docs/cli.md` and `docs/reference/`. That cost is small
  in dollars but is reported separately (protocol §6).
* If the Owner chooses the frontier pair (Fable 5.1 / GPT-6 Astra), multiply the Claude figures by ~2.5 and the Codex figures by ~5, and
  raise the per-run ceiling to match ($75 per run), or the ceiling will bind in most runs.
