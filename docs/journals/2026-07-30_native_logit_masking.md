# Native Logit Masking

Item: improvement #68, `Native Logit Masking`
Started: 2026-07-30
Status: In progress
Next Step: Delegate implementation to `gpt-5.6-sol` with a self-contained brief, then verify the real diff.

## Context

Selected via `$work_next_item` protocol. No active journal existed in `documentation/task_journals/` or `docs/journals/`. `ps aux | grep -E "claude|agy"` showed no live peer agent beyond the grep process. `git worktree list` showed one Antigravity worktree, but it is clean, has no task journal, and its commit is already contained by `main`.

Highest pending item across `bugs.md` and `improvements.md` is improvement #68 at score 2.5. Improvement #70 is tied at 2.5 but appears later in the ranked table, so #68 wins the single-item selection. The only pending bug currently visible is bug #32 at score 2.0, below #68.

## Model Check

The row lists OpenAI model `gpt-5.6-sol`. Verified availability with:

```bash
codex exec -m gpt-5.6-sol "Reply with exactly: model-ok"
```

The command ran under model `gpt-5.6-sol` and returned `model-ok`.

## Item Detail

From `improvements.md`:

- Description: Allow Zero types to natively compile into inference-level logit masks to restrict LLM generation space.
- Why: Strongly inspired by LMQL to eliminate syntax hallucination at the token-generation level.
- Impact: 5/10, valuable for AI authoring reliability but dependent on type/checker maturity and model-inference integration.
- Groomed 2026-07-30: Ranked behind #64 because typed constraints need the checker foundation first. Likely approaches include grammar/logit masks, type-derived masks, or external constrained decoding libraries.

## Delegations

None yet.
