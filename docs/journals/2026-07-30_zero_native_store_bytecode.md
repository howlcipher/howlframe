# Zero Native Store Bytecode

Date: 2026-07-30

## Item

Improvement #71, "Zero Native Store Bytecode".

## Current Evidence

- `improvements.md` lists #71 as the first open row in the ranked table with OpenAI model `gpt-5.6-sol`.
- `codex exec -m gpt-5.6-sol --ephemeral --json --skip-git-repo-check 'Respond with exactly AVAILABLE.'` returned `AVAILABLE`.
- No active `documentation/task_journals/` or `docs/journals/` item exists.
- The Antigravity worktree `subagent-Zero-Transpiler-Developer-self-7e8805d7` is clean and has no active journal.
- `docs/zero_native_store_design.md` defines Phase 2 as in-memory `store_open`, `store_put`, `store_get`, and `store_delete` in the bytecode VM.

## Scope

Implement the first executable slice of the design:

- Add bytecode opcodes for `store_open`, `store_put`, `store_get`, and `store_delete`.
- Compile those AST nodes into bytecode.
- Execute them in the bytecode VM using an in-memory store implementation.
- Add a focused `.zero` regression fixture that compiles with `-compile-bc` and runs with `-run-bc`.
- Keep `store_query`, indexes, transactions, durability, and semantic retrieval out of scope unless needed for a minimal contract.

## Delegation

Planned delegate: Codex CLI using OpenAI model `gpt-5.6-sol`.

## Next Step

Commit this journal, then launch the delegate with a self-contained implementation brief from a clean worktree.
