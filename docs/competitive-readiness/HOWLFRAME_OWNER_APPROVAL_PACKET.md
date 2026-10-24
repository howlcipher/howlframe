# competitive-v1: Owner approval packet

Prepared 2026-10-24. **Nothing below is approved.** Each item needs an explicit Owner answer. "Do what you recommend" counts as approval
only for the items it is given for. No paid model call, trial, freeze or infrastructure build happens before the corresponding item is
approved.

Already decided, and **not reopened** here: providers are Claude Code and Codex; H2 uses option A; H4 keeps the default (all three
baselines gate). The current state is no spend cap and no trial authorized.

| # | Decision | Recommendation | Alternatives | Evidence |
|---|---|---|---|---|
| 1 | Exact Claude model ID | **`claude-opus-5-5`**, `--effort high`, standard speed, global endpoint | `claude-fable-5-1` (2.5x price; not ahead on published agentic-coding figures) | Model selection §2–3 |
| 2 | Exact Codex model ID | **`gpt-6.1-sol`**, `model_reasoning_effort="high"`, standard tier. Pin the dated snapshot if `GET /v1/models` lists one at freeze | `gpt-6-astra` (5x price), which goes with option 1's alternative as a price-matched frontier pair | Model selection §2–3 |
| 3 | Provider for the 4-run feasibility test | **Claude Code (`claude-opus-5-5`)**. It is the higher-priced arm, so its costs give a conservative calibration for the pilot cap. It also has a native second budget limit (`--max-budget-usd`), and harness code from #178 can be reused | Codex: tests the proxy-only enforcement path, which the pilot will also need. Either is acceptable to the protocol | Budget §3; readiness §5 |
| 4 | Hard spending cap for the 4-run test | **$150 total**, with a **$30 per-run ceiling**, enforced by the proxy, harness admission, the CLI flag and a provider-side limit with prepaid credit and auto-reload off | $100, accepting that one infrastructure rerun might not fit | Budget §3, §5 |
| 5 | May the revised preregistration be frozen once all prerequisites are met? | **Not yet.** First approve Revision 3 (G1–G12), then have one scoped different-provider review of its deltas. Freezing also needs the task pool, tests, references, documentation audit and jail hashes from independent authors. Recommend: "approve Revision 3 drafting; freeze comes back for a separate yes" | Approve freezing now, conditionally on the gates. **Not recommended**, because G1 changes what the dry run is | Readiness §4 |
| 6 | May the agent build the missing experiment-only infrastructure? | **Yes, scoped:** under `experiments/competitive-v1/` only, in a worktree and draft PR, with **no** HowlFrame language or runtime change, **no** paid calls, and validated only against mock providers: the proxy, jail, runner, scorer, results schema and beacon draw. No task authoring by a trial-provider agent | No (wait for an external harness), or Podman instead of bubblewrap (needs a system package install, so authorize that separately) | Readiness §5; budget §5 |

## Further decisions surfaced by this review

| # | Decision | Recommendation |
|---|---|---|
| 7 | Billing mode | Dedicated **API keys**: an Anthropic workspace and an OpenAI project created for the experiment. Not subscription sign-in |
| 8 | Who authors tasks and tests | Antigravity (Gemini) or a human for the pilot pool and calibration task. Held-out tests by a different author and provider |
| 9 | Sandbox technology | Bubblewrap jails with hashed toolchain tarballs (no system change), with the protocol wording revised to match (G4) |
| 10 | 96-run pilot cap | **Defer** until feasibility results exist. Provisional exposure limit: $3,200 at a $30 per-run ceiling; expected about $600 |

## What a "yes" to 1–4 does **not** authorize

It does not authorize running the feasibility test. That still needs items 5 and 6 complete, the infrastructure built and checked against
mocks, the calibration task authored, and a final go-ahead naming the date. It does not authorize the 96-run pilot, merging #213, or any
HowlFrame change.

## Suggested Owner reply (edit as needed)

> 1: claude-opus-5-5 high. 2: gpt-6.1-sol high. 3: Claude Code. 4: $150 total, $30 per run. 5: draft Revision 3 and get it reviewed;
> freeze comes back to me. 6: yes, scoped as written. 7: API keys. 8: Antigravity authors. 9: bubblewrap. 10: defer.
