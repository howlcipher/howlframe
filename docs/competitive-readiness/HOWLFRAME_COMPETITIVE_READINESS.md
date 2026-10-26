# competitive-v1: readiness review

Reviewed 2026-10-23/24 on draft PR #213, head `1116c48becd22790f1bed8c0e899487f808be2cc` (Revision 2; 28 commits behind HowlFrame
`main` `056fbf89`). The reviewer is Claude Opus 5.5, the protocol's own author, so this is **not** an independent review. It is a
readiness and operational audit. No further fairness-review cycle was started.

## 1. Verdict

**Not ready to freeze.** Methodology is sound: R3 found no blocking problem, and its three suggestions are verifiably applied (§2).
What blocks a freeze is a set of operational gaps (§4) and missing infrastructure (§5). None of these needs a HowlFrame language or runtime
change. The verified baseline (`HOWLFRAME_CURRENT_VERIFICATION.md`) is suitable to pin.

## 2. Recorded decisions are preserved

| Decision | Where recorded | Status in the draft |
|---|---|---|
| Providers: Claude Code and Codex | Protocol §7.8; `parameters.json` `owner_options_selected.providers` | Present. Model IDs are null ("fixed at freeze") |
| H2: Owner option A (shared credit for equal non-zero results; mutual failures earn nothing) | §7.2, §7.8; `owner_options_selected.h2 = "A"` | Present |
| H4: default; Python, Go and JavaScript all gate | §7.4, §7.8; `h4_gate_baselines` lists all three | Present |
| No spend cap; no dry run or pilot | `owner_spend_cap: null`; `owner_options_selected.spend_cap`; Owner decisions doc, I | Present |
| No trial authorized | Protocol status line; §11 gates 2–4 open | Present |

Fairness review R3's three non-blocking suggestions are all applied:
* S1: Table 1 now cites the §7.7 *not met* outcome for H1 and H3–H5.
* S2: `h2_reported` mirrors §7.2.
* S3: §3.2.1 now says "≥ 120 candidates for 80 tasks".

## 3. Existing evidence (what this study must not be confused with)

As protocol §2 states and I confirmed: the write-cost, real-app, `token-efficiency-v0` and held-out v0–v3 results measure **reference
source size**, not model outcomes. The held-out v0 Python comparison (0/1/29) is the honest prior that HowlFrame is more verbose on unseen
tasks. The `live-v1`/`falsifiable-v0` work compares HowlFrame with **JSON action plans**, not with Python, Go or JavaScript. No existing
study answers D16.

## 4. Gaps that must close before freeze (proposed Revision 3)

| # | Gap | Why it matters | Proposed resolution |
|---|---|---|---|
| G1 | **The 4-run dry run is under-specified.** §8 says only "a 4-run dry run, one per language with one provider". The task, scoring, timing relative to the draw, and how results may change parameters are all undefined | If it reuses a drawn pilot task, it pre-exposes the pilot. If it changes budgets after the freeze, it breaks preregistration | Define a **feasibility test**: one *calibration task* written by the independent task author, **outside** the pilot pool and never reused. It runs under the full harness after infrastructure is frozen and **before** the pilot draw. It is scored and published as an infrastructure check, never as evidence. Its only permitted outputs are budget calibration and harness fixes, recorded as one revision before the pilot freeze |
| G2 | The per-run "provider token ceiling" is undefined | It is the only cost limit, and it cannot be enforced as written | A per-run **dollar ceiling**, identical across arms within a provider ($30 proposed), enforced by the proxy (budget §5). Hitting it means *not completed: budget exhausted* |
| G3 | **Beacon semantics are ambiguous.** NIST v2 `/pulse/time/{ms}` returns the pulse *at* a minute boundary, while `/pulse/time/next/{ms}` returns the *next* one. Probe: 2026-10-20T00:00Z gives pulse 1990929 for the first and 1990930 for the second (`evidence/probes/`). "outputValue ‖ manifest SHA-256" does not say whether these are hex text or raw bytes, or what case | Two honest implementers can derive different seeds | Pin the endpoint (`/pulse/time/{ms}` at a minute boundary), the concatenation (raw bytes of both hex values, decoded), the per-domain shuffle algorithm (e.g. Python `random.Random(int.from_bytes(seed)).shuffle` on the sorted ID list), and archive the full signed pulse JSON |
| G4 | **No container runtime on this host.** Docker and Podman are absent. Bubblewrap is present and verified (§6) | "One container image per arm" and "container digests" cannot be produced as written | Either (a) the Owner authorizes installing rootless Podman, or (b) revise to bubblewrap jails over a pinned read-only root plus per-arm toolchain tarballs, recording SHA-256 of each tarball and of the root manifest. (b) needs no system change |
| G5 | **Toolchain parity.** Python 3.12 is not installed (host has 3.14.4). Go 1.25.x is not installed (host has 1.26.0). Node 22.22.1 is present. The HowlFrame "skill text" lives in the HowlPlane repository (`.agents/skills/howlframe-*`), not in HowlFrame | The protocol pins versions the host cannot supply, and the skill's version is unpinned | Per-arm user-space toolchain tarballs (Go 1.25.x, CPython 3.12.x standalone build, Node 22.x) with hashes. Pin the skill by HowlPlane commit and SHA-256, or vendor its text into the experiment directory |
| G6 | Server-side tools and subagents are not addressed | Provider-hosted web search bypasses a network jail. Subagents change the agent design and the cost | Disable both for every arm, and have the proxy refuse requests that declare them (model selection §4) |
| G7 | **Agent configuration contamination.** On this host, `~/.claude/CLAUDE.md` imports HowlPlane `AGENTS.md`, which lists the HowlFrame skills. Codex reads `~/.codex/AGENTS.md`, skills and memories | A default-config trial run would inject HowlFrame guidance into every arm. That favours HowlFrame and violates "identical instructions" (§5) | Run each CLI with an isolated `HOME`/`CLAUDE_CONFIG_DIR`/`CODEX_HOME`, `--bare`/`--ignore-user-config`, and no memories or skills. Audit the effective system prompt from the first logged request of each run |
| G8 | Served-model verification. PR #178 notes "codex … CLI does not report the serving model id" | H6 depends on knowing which model ran | The proxy records the response `model` on every call; a run with a mismatch is an infrastructure failure |
| G9 | Billing mode is unspecified. Codex on this host uses ChatGPT sign-in | The cap cannot be enforced without metered API keys | Dedicated API keys (Owner decision) |
| G10 | Runtime measurement on a shared desktop | Background load biases H4 | Run runtime scoring separately from agent runs, on a quiescent host with pinned CPU affinity. Record load average |
| G11 | Stale text in §12. The R1 table still describes Revision 1 rules ((W+½T)/N primary; Go exempt; 60–90 tasks). §8's "4-run dry run" wording predates G1 | Readers may cite superseded rules | Mark those rows "superseded by R2" |
| G12 | The draft is 28 commits behind `main` | The freeze must pin a merged HowlFrame commit (§11.4) | Rebase or update the branch at freeze. Pin `056fbf89`, or later only after re-verification |

**Scope of re-review.** Revision 3 changes operational definitions only: G1, G2, G3 and G4(b). Protocol §8's exit rule already requires a
revised protocol to be re-reviewed. A **single scoped, different-provider review of the Revision 3 deltas only** is proportionate, and it
addresses specific unresolved issues. It is not an open-ended cycle.

## 5. Infrastructure inventory

| Component | Exists? | Notes |
|---|---|---|
| Protocol and parameters | Draft (#213) | Revision 3 needed (§4) |
| Agentic trial runner (launch, 45-minute kill, `task-complete`, transcript capture, co-scheduling of 4 arms) | **No** | `benchmarks/v2` Track A and the `live-v1` harness (#178) are single-turn. `live-v1`'s `ClaudeModel` (JSON usage parsing, served-model allowlist) is reusable code |
| Per-arm sandbox | **No** (feasibility shown) | Bubblewrap probe: read-only test mount refuses append, delete and move (hash unchanged); no external network (`ENETUNREACH`); loopback listen works |
| Metering, cap and tool-call proxy | **No** | Budget §5. Must be validated against a mock upstream before any paid call |
| Pinned toolchains and offline docs per arm | **No** | G5. HowlFrame docs: `README.md`, `docs/cli.md`, `docs/reference/` ≈ 182 KB, ~45K tokens. `docs/reference/` also holds internal design notes (HFIR schemas, prod-flip criteria), so the documentation audit must decide what an agent sees |
| Scoring harness (copy-out, fresh jail, hash-verified tests, held-out runner, runtime median-of-5 minus startup, RSS, 10× workload, canaries) | **No** | |
| Results schema covering *completed*, *not completed* (budget, time, tool calls), *infrastructure failure and rerun*, *intervention*, *tampering*, *expressibility failure* | **No** | Protocol §7.9 requires every outcome, so the schema must have these from the start |
| Beacon draw and manifest hashing | **No** | G3. The NIST v2 API is reachable and returns historic pulses |
| Analysis (W / shared / mutual-failure / L, RMST, task-clustered intervals) | **No** | Needed for the pilot only. The feasibility test is descriptive |
| Candidate pool, frozen and held-out tests, references, D4 baselines | **No** | **Human or independent-provider authors only** (§7) |

**Hidden-test protection.** The design is sound: held-out tests are never mounted during a run, acceptance tests are mounted read-only
outside the workspace, scoring copies the final code into a fresh jail with hash-verified tests, and any modification counts as tampering.
The bubblewrap probe confirms that the read-only mount holds against append, delete and rename. One residual risk: an agent can copy
the visible tests and edit the copy. That is harmless, because scoring ignores workspace tests.

## 6. Non-billable checks performed (infrastructure only; not language-comparison results)

| Check | Result |
|---|---|
| NIST beacon v2 historic lookup | OK. `time/{t}` and `time/next/{t}` differ (G3) |
| Bubblewrap read-only mount (append, rm, mv) | All refused; SHA-256 unchanged |
| Bubblewrap network: external, loopback | External `ENETUNREACH`; loopback bind OK |
| Toolchains on host | Node 22.22.1 OK; Python 3.12 missing; Go 1.25.x missing; no Docker/Podman |
| CLI flags needed for parity | Present in Claude Code 2.1.309 (`--bare`, `--effort`, `--max-budget-usd`, `--disallowedTools`, `--strict-mcp-config`, `--no-session-persistence`) and Codex 0.163.0 (`--ignore-user-config`, `--strict-config`, `--ephemeral`, `--json`, `model_providers`/`base_url`) |

No mock-provider run of a harness was possible, because no harness exists. Building one is Owner decision 6.

## 7. What needs human or independent approval before task selection

1. **Task author:** the protocol requires a provider "not implementing HowlFrame changes, and not used as a trial agent for that task".
   Claude and Codex are both trial providers. Antigravity (Gemini) qualifies, or a human. The author writes the pilot pool (≥ 6 per D1–D4),
   the calibration task (G1), the acceptance tests, and the *authority-centred* labels.
2. **Held-out test author:** a different author and provider from the acceptance-test author.
3. **Reviewer (§11):** checks the pool against existing HowlFrame corpora, audits the labels and runs the documentation audit gate.
4. **Reference implementers:** someone other than the trial agents, in all four languages.
5. **Owner:** the approval packet items.

**Proposed pool composition (not authored here):** D1 CLI data processing (e.g. log or CSV reconciliation with a validation report), D2
authenticated JSON CRUD with limits and persistence, D3 a local HTTP dependency plus subprocess deadlines, D4 a change request against a
four-language baseline. At most one authority-centred task in the pilot.

## 8. What a four-run feasibility test can and cannot show

**Can:** that the harness, jail, proxy, cap enforcement, transcript capture, scoring and outcome reporting work end-to-end; and real
per-run token, cost and time figures for calibrating the pilot cap.
**Cannot:** anything about D16 or H1–H6. That is one task, one provider, one run per language, with no repetitions and no intervals.
The report must say so in its first line.
