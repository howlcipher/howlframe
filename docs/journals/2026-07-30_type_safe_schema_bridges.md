# Type-Safe Schema Bridges

**Item:** Improvement #70, Type-Safe Schema Bridges
**Date:** 2026-07-30
**Status:** In progress
**OpenAI model:** gpt-5.6-terra
**Next Step:** Inspect checker/masking/schema-related code, then delegate the implementation to `codex exec -m gpt-5.6-terra`.

## Selection

- Checked active journals in `docs/journals/` and `documentation/task_journals/`; only archived journals exist.
- Checked concurrent `claude`/`agy` processes; none were live.
- Checked `git worktree list`; one generated Antigravity worktree exists, but it has no uncommitted changes, no active journal, and no unmerged commits ahead of `main`.
- Selected improvement #70 because it is the highest-scored pending improvement at 2.5. Bug #32 is pending at 2.0, so #70 remains the top current item.
- Verified the listed OpenAI model with `codex exec -m gpt-5.6-terra 'respond with ok'`; it is available.

## Re-evaluation

The item is still worth doing, but the broad BAML-inspired framing should stay small. Existing `struct`, `parse_json`, semantic checker metadata, and masking plans already cover part of the schema story. A native Zero bridge form that reuses those systems is lower risk than adding a BAML compiler or JSON Schema interop layer.

Design options considered:

- BAML-style schema compiler: strongest schema extraction framing, but too broad for effort 2 and duplicates existing Zero struct syntax.
- JSON Schema interop: familiar and portable, but looser than the compiler-visible type information already present in the checker.
- Native Zero bridge form: smallest language surface, keeps schemas compiler-visible, and can reuse current struct/type analysis and mask planning. This is the preferred route.
