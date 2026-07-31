# Bug 36: confidence fixture decimal literals

Status: Selected

Next Step: Delegate the decimal-literal lexer/parser fix to `gpt-5.6-luna`, then review the diff and verify `tests/test_confidence.zero`.

## Context

- Selected via `$work_next_item` on 2026-07-31.
- No active task journals were present in `documentation/task_journals/` or `docs/journals/`.
- No live `claude` or `agy` processes were found.
- `git worktree list` showed only the main worktree.
- The listed OpenAI model for bug #36 is `gpt-5.6-luna`; verified available with `codex exec -m gpt-5.6-luna`.

## Re-evaluation

Bug #36 still reproduces:

```text
go run zero.go -o /tmp/zero_confidence_check tests/test_confidence.zero
{"reason":"\u003e expects 2 arguments, got 3","line":3,"column":9}
```

The fixture uses `(> score 0.8)`. Current lexer behavior parses `0.8` as integer token `0` followed by symbol `.8`, giving the comparison three operands. Decimal literals appear intended to be valid: the type checker has `ast.Float`, `confidence` returns `float64`, bytecode/SSA already recognize `FLOAT`, and project docs show confidence thresholds like `0.95`.

## Delegations

- Pending: `gpt-5.6-luna` via `codex exec`.
