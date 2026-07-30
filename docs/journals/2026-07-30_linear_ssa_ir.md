# Linear SSA-based IR

## Backlog Item
- Improvement #65: Linear SSA-based IR
- OpenAI model: `gpt-5.6-sol`
- Started: 2026-07-30

## Current State
- No in-flight journal existed under `docs/journals/`.
- No additional git worktrees were present.
- No live `claude` or `agy` processes were found by the concurrent-session check.
- `codex exec -m gpt-5.6-sol` returned `MODEL_AVAILABLE`, so the listed OpenAI model is available.

## Goal
Lower the existing high-level shared tree IR into a flat SSA-oriented representation with control-flow blocks, without replacing existing backends in this slice. The result should be directly testable and preserve typed metadata from the checker.

## Delegations
- Pending: delegate the first implementation pass to `gpt-5.6-sol`.

## Verification
- Pending.

## Next Step
Delegate the implementation brief to `codex exec -m gpt-5.6-sol`, then review the resulting diff before running tests.
