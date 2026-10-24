# HowlFrame competitive-v1: model selection

Prepared 2026-10-23 by Claude Opus 5.5 (`claude-opus-5-5`) in Claude Code 2.1.309. **This document recommends; it decides nothing.**
No model session was started to test any configuration below.

Conflict of interest, disclosed: the recommending agent runs on the Claude model recommended for the Claude arm. It also wrote the
competitive-v1 draft and earlier HowlFrame changes. Prices and catalog data below come from live sources, cited, so the Owner or
a different-provider reviewer can check them independently.

## 1. Sources (checked 2026-10-23)

| Fact | Source | Kind |
|---|---|---|
| Claude prices, cache multipliers, data-residency 1.1x, batch and fast-mode prices | https://platform.claude.com/docs/en/about-claude/pricing (fetched 2026-10-23) | Primary, live |
| Claude model IDs, context and output limits, effort and thinking rules | Claude API reference bundled with Claude Code 2.1.309 (model table cached 2026-10-06) | Primary, cached |
| OpenAI prices (Standard, long-context and Fast tiers) | https://developers.openai.com/api/docs/pricing (fetched 2026-10-23; `platform.openai.com/docs/pricing` redirects there) | Primary, live |
| Codex model catalog: slugs, context windows, reasoning levels, `supported_in_api` | `~/.codex/models_cache.json`, fetched by Codex CLI 0.163.0 at 2026-10-23T21:35:09Z | Primary, local |
| Agentic-coding benchmark figures | Third-party aggregators quoting vendor tables (links in §5) | **Secondary, vendor-reported, not same-harness** |

The bundled reference gave Claude Sonnet 5.5's cache-read price as $0.20/MTok; the live pricing page says **$0.10/MTok** (0.05x).
The live page is used here. That is a library-update candidate for the bundled reference, not a HowlFrame issue.

## 2. Candidates

### Anthropic (Claude Code CLI)

| Model ID | Context / max output | Input | 5m cache write | Cache read | Output | Notes |
|---|---|---|---|---|---|---|
| `claude-opus-5-5` | 1M / 128K | $4 | $5 | $0.20 | $20 | Current Opus. Thinking is always on, so effort is the only depth control, and its API default is `medium` |
| `claude-fable-5-1` | 1M / 128K | $10 | $12.50 | $0.25 | $50 | Most capable widely released model; requires 30-day retention |
| `claude-sonnet-5-5` | 1M / 128K | $2 | $2.50 | $0.10 | $10 | Current Sonnet |

All per 1M tokens, global endpoint. `inference_geo: "us"` multiplies every token class by 1.1.

### OpenAI (Codex CLI)

| Model slug | Codex context (max) | Input ≤272K / >272K | Cached ≤272K | Cache write 30m | Output ≤272K / >272K | Catalog description |
|---|---|---|---|---|---|---|
| `gpt-6.1-sol` | 500K (872K) | $2 / $4 | $0.10 | $2.50 | $10 / $15 | "Latest workhorse model for coding and everyday work" (priority 1) |
| `gpt-6-astra` | 272K (872K) | $10 / $20 | $1.00 | $12.50 | $50 / $75 | "Frontier intelligence for the most demanding work" |
| `gpt-6-sol` | 272K (872K) | $2 / $4 | $0.20 | $2.50 | $10 / $15 | "Previous generation workhorse model" |

Reasoning tokens bill as output. Every request whose input is over 272K tokens is billed entirely at the long-context rate.

## 3. Recommendation (Owner decision 1 and 2)

| | Claude Code arm | Codex arm |
|---|---|---|
| **Model ID** | **`claude-opus-5-5`** | **`gpt-6.1-sol`**, plus the dated snapshot ID the API returns in each response's `model` field, recorded at freeze |
| Effort | `--effort high` (set explicitly; the API default is `medium`) | `-c model_reasoning_effort="high"` (the Codex default is `low`) |
| Thinking display | Default (omitted). Thinking tokens are billed and counted either way | Default reasoning summary |
| Speed tier | Standard. **Fast mode off** | Standard. **Fast/priority tier off** |
| Data residency | Global (no 1.1x) | Global (no uplift) |
| CLI version | Claude Code `2.1.309` or the version at freeze; record the binary SHA-256; `DISABLE_AUTOUPDATER=1` | Codex CLI `0.163.0` or the version at freeze; record the binary SHA-256; disable update checks |
| Billing | Dedicated API key in a dedicated workspace (§6) | Dedicated API key in a dedicated project (§6) |

**Why this pair:**
1. Each is its vendor's currently recommended model for agentic coding. Codex lists `gpt-6.1-sol` first. Anthropic's bundled reference makes
   `claude-opus-5-5` the default, and vendor-reported agentic-coding tables put it ahead of `claude-fable-5-1`
   (Terminal-Bench 4.0: 66.4% vs 55.8%).
2. Neither arm is a weak baseline. OpenAI reports `gpt-6.1-sol` matching `gpt-6-astra` on its own coding benchmark at about a fifth of
   the cost. That claim is vendor-reported and was not verified here.
3. Both run at the same nominal effort (`high`), a common cost/quality setting for long agentic work. The two vendors' effort scales are
   **not** calibrated against each other, so this is nominal parity only.

**Alternative (frontier tier):** `claude-fable-5-1` with `gpt-6-astra`, both at effort `high`. These are price-matched ($10/$50 each),
which the recommended pair is not, but they cost about 2.5x (Claude) and 5x (OpenAI) more per token. Published figures do not show
Fable 5.1 ahead of Opus 5.5 on agentic coding. Choose this pair only if "each vendor's most expensive model" is the fairness criterion the
Owner wants.

**Not recommended:** Sonnet 5.5 or `gpt-6-luna` as a primary arm (weaker tier). Mixing tiers across providers, for example Fable against
Sol, is also not recommended.

## 4. Parity configuration (required for both providers)

These settings are needed for fairness and isolation. None of them exist yet (see `HOWLFRAME_COMPETITIVE_READINESS.md`).

| Item | Claude Code | Codex | Why |
|---|---|---|---|
| Isolated configuration | Fresh `HOME` and `CLAUDE_CONFIG_DIR`, `--bare`, `--no-session-persistence`, no `CLAUDE.md`, memory or skills | Fresh `CODEX_HOME`, `--ignore-user-config`, `--strict-config`, `--ephemeral`, no `AGENTS.md`, skills, plugins or memories | **This host's global `~/.claude/CLAUDE.md` imports HowlPlane `AGENTS.md`, which lists the HowlFrame skills.** A trial run with default configuration would load HowlFrame guidance into every arm |
| Web access | `--disallowedTools WebSearch,WebFetch`, and the proxy refuses requests that declare server-side web tools | `web_search` disabled in config, and the proxy refuses requests that declare it | Server-side search runs at the provider, so a network jail cannot block it. The web could also leak solutions |
| Subagents | `Task`/`Agent` tool disallowed | Multi-agent features disabled | Equal single-agent design. Subagents also multiply cost |
| MCP, plugins, hooks | None (`--bare`; `--strict-mcp-config` with an empty config) | None | Equal tool surface |
| Approvals and sandbox | `--permission-mode bypassPermissions` **inside** the external jail only | `--sandbox danger-full-access` and `approval_policy=never` **inside** the external jail only | No human prompts (H1 autonomy). The jail (bubblewrap) is the security boundary for both |
| Output capture | `-p --output-format stream-json --verbose` | `exec --json` | Per-call usage, tool calls and timing |
| Served-model check | Proxy records `model` from every response | Same | PR #178 noted that the Codex CLI does not report the served model, so the proxy is the only reliable record |

**Differences that cannot be normalized; the study must disclose them:**
1. **Tool sets differ.** Claude Code has Read/Edit/Write/Bash/Grep/Glob; Codex has a shell and `apply_patch`. Equal across languages
   within a provider, but not across providers.
2. **Context and compaction differ.** Opus 5.5 has 1M context with Claude Code's auto-compaction; `gpt-6.1-sol` has 500K in Codex, with a
   price step above 272K input and Codex's own truncation policy.
3. **Effort scales are vendor-specific.** "high" on one is not "high" on the other.
4. **Tokenizers differ.** Token counts are not comparable across providers. Cost in dollars and wall time are.
5. **Author/model overlap.** Claude Opus 5.5 wrote much of HowlFrame's documentation, the skill text and this protocol. Any advantage that
   gives the Claude arm on HowlFrame shows up as a provider × language interaction, which H6's per-provider reporting exposes. Report it
   explicitly.
6. **Neither CLI exposes a sampling seed.** Runs cannot be replayed exactly. Reproducibility rests on pinned IDs, CLI binaries, prompts,
   images, full transcripts and repeated sessions.

## 5. Version pinning and reproducibility

* Record in `parameters.json` at freeze: model ID; served model ID observed on the first call (and reject a run whose served model
  differs); CLI version and binary SHA-256; effort; every CLI flag; the proxy build hash; the price table used for cost computation.
* `claude-opus-5-5` is a complete, undated ID. Anthropic's guidance is not to append dates. Record the `model` echoed in responses.
* For `gpt-6.1-sol`, call `GET /v1/models` with the experiment key at freeze. If a dated snapshot exists, **pin the snapshot ID**, not
  the alias.
* Keep full JSONL transcripts and proxy logs for every run. They are the reproducibility record.

Benchmark sources (secondary; for orientation only): [benchmarklist.com: Claude Opus 5.5](https://benchmarklist.com/models/anthropic-claude-opus-5.5/),
[vals.ai: Opus 5.5 vs GPT-6 Sol](https://www.vals.ai/comparisons/anthropic_claude-opus-5-5-vs-openai_gpt-6-sol),
[metricnexus.ai: GPT-6.1 Sol benchmarks](https://metricnexus.ai/blog/gpt-6-1-sol-benchmarks-pricing),
[IT Brief: GPT-6.1 Sol launch](https://itbrief.com.au/story/openai-launches-gpt-6-1-sol-as-cheaper-coding-model).
None of these is a same-harness comparison. This study exists precisely because such numbers cannot settle the question.

## 6. Billing mode (Owner decision)

This host's Codex CLI is signed in with a ChatGPT account (`auth_mode: chatgpt`). Subscription sign-in is **not recommended** for trials:
* It gives no per-token billing to enforce a dollar cap against, and usage limits could end runs for reasons unrelated to the task.
* Routing through the metering proxy (§4) needs API-key authentication with a configurable base URL (`ANTHROPIC_BASE_URL` for Claude Code,
  a custom `model_providers` base URL for Codex).
* A dedicated key per provider isolates experiment spend from the Owner's other work.
