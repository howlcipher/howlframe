# 2026-07-30 Deep AST Nesting Stack Limits

## Item
Bug #32: Deep AST Nesting Stack Limits

## Status
In progress

## Model Routing
- OpenAI model listed in `bugs.md`: `gpt-5.6-terra`
- Availability check: `codex exec -m gpt-5.6-terra "Respond with only: available"` succeeded on 2026-07-30.

## Scope
The AST traversal currently relies on recursive walks in parser include expansion, AST transforms, semantic analysis/checking, bytecode compilation, and backend emission. Long generated programs, especially deeply nested `let` chains, can hit arbitrary recursion limits or stack growth before producing useful output.

## Notes
- No active task journal existed in `documentation/task_journals/` or `docs/journals/`.
- Concurrent process check found only this session's `ps`/`grep`.
- An old Antigravity worktree exists on branch `subagent-Zero-Transpiler-Developer-self-7e8805d7`, but it has no dirty state and only archived journals.
- Current main branch matched `origin/main` at selection (`0b78adb feat(schema): add type-safe schema bridges`).

## Delegations
- Pending: delegate implementation to `gpt-5.6-terra` via `codex exec`.

## Next Step
Delegate a focused implementation pass that removes the deep `let` chain recursion hazard, adds regression coverage for a long valid chain, and preserves existing JSON diagnostics for malformed programs.
