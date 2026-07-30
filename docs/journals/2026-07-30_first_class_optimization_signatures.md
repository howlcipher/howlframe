# First-Class Optimization Signatures

Date: 2026-07-30

## Item

Improvement #69: First-Class Optimization Signatures

## Selection Evidence

- No active files in `documentation/task_journals/`.
- No active files in `docs/journals/`.
- `git worktree list` shows only `/run/media/system/tallgeese/dev/zero` on `main`.
- Concurrent-session check found only the check command and its `grep`.
- Top pending item across `bugs.md` and `improvements.md` is improvement #69 with score `1.5`; pending bugs #35-#37 are score `1.0`.
- Listed OpenAI model is `gpt-5.6-sol`.
- Model availability probe: `codex exec -m gpt-5.6-sol ... 'Reply with exactly: MODEL_AVAILABLE'` returned `MODEL_AVAILABLE`.

## Re-evaluation

The item is still valid. The shipped `optimize_block` primitive covers runtime hot-swapping after execution metrics are observed. Improvement #69 asks for compile-time optimization signatures, a separate compile loop that records what should be optimized, how tests should be run, and how prompt/program variants can be evaluated before runtime.

## Delegation

Pending.

## Verification

Pending.

## Next Step

Inspect the current code paths for `optimize_block`, bytecode lowering, type checking, and CLI flags, then delegate the implementation to Codex with `gpt-5.6-sol`.
