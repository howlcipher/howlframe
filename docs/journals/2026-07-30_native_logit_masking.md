# Native Logit Masking

Item: improvement #68, `Native Logit Masking`
Started: 2026-07-30
Status: Implementation complete, pending orchestrator review
Next Step: Inspect the implementation diff and independently run the broader repository verification.

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

Implementation was delegated to Codex using `gpt-5.6-sol` with instructions to edit the shared checkout directly, avoid commits and provider calls, preserve this journal, and add a provider-neutral type-derived mask plan API with focused tests.

The delegate selected a separate `internal/masking` package so semantic analysis remains independent from inference-provider concerns. The package consumes `checker.Analysis`, uses ordered slices for stable JSON, and normalizes the checker-generated capitalized aliases for source struct fields.

## Files Changed

- `internal/masking/masking.go`: Added type, struct, function, and full-program mask plan types plus compilation APIs.
- `internal/masking/masking_test.go`: Added isolated coverage for primitives, collections, structs, recursive constraints, complete checker analysis, deterministic JSON, and nil analysis.
- `README.md`: Documented the provider-neutral constrained-decoding boundary.
- `improvements.md`: Marked improvement #68 Done on 2026-07-30 without changing #70.
- `docs/journals/2026-07-30_native_logit_masking.md`: Recorded delegation and handoff details.

## Verification

The delegate ran:

```bash
GOCACHE=/tmp/zero_go_cache go test ./internal/masking
CCACHE_DISABLE=1 GOCACHE=/tmp/zero_go_cache go test ./internal/...
CCACHE_DISABLE=1 GOCACHE=/tmp/zero_go_cache go test -cover ./internal/masking
CCACHE_DISABLE=1 GOCACHE=/tmp/zero_go_cache go vet ./internal/masking
git diff --check
```

Result: all commands passed. Focused masking coverage was 88.4% of statements. The first broad run reached the existing internal packages but the host compiler wrapper failed when its global ccache was read-only; disabling ccache produced a clean full internal result.

## Next Verification Step

The orchestrator should inspect `git diff`, rerun `go test ./internal/...` with a writable build cache, and confirm no unrelated tracked or generated files changed before accepting the implementation.
