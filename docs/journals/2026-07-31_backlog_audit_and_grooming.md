# Implementation Journal

Task: Repository audit and backlog grooming toward the Zero AI-first language mission (no implementation)
Date: 2026-07-31

## Scope

This was a planning-only run: repository inspection, architecture/product audit, bug discovery,
backlog creation/grooming, ranking, model/execution-mode routing, and a recommendation for the
next implementation item. No implementation, compiler, runtime, backend, parser, checker, test,
example, website, packaging, or CI code was modified. Only `improvements.md` and this journal were
changed.

## Files inspected

- `README.md`, `bugs.md` (full), `improvements.md` (full) — read directly, not just via agent summary.
- `.agents/skills/zero_transpiler/SKILL.md` (lives in the sibling `ai_knowledge_library` repo, not in
  the `zero` repo itself — noted as a location surprise, not a defect).
- `docs/journals/` (archive contents, naming convention `YYYY-MM-DD_task_name.md`).
- Core pipeline via a dedicated Explore agent: `internal/lexer`, `internal/parser`, `internal/ast`,
  `internal/checker` (`checker.go`, `types.go`), `internal/ir` (`ir.go`, `ssa.go`),
  `internal/backend/gogen`, `internal/backend/javascript`, `internal/backend/wasm`
  (`wasm.go`, `ssa_serializer.go`, `validate.go`), `internal/bytecode`, `internal/vm`
  (`vm.go`, `error.go`), `internal/masking`, `internal/optimization`, `zero.go`, `cmd/codegen`,
  `cmd/zero` (untracked), `zero_test.go`, `tests/*.zero` (42 fixtures, counted not fully read).
- No `.github/` directory, no `CONTRIBUTING.md` exist in this repo (confirmed absent).

## Commands run

- `git status`, `git remote -v`, `git ls-files --error-unmatch cmd/zero/main.go` (confirmed untracked).
- Repository-wide greps for `TODO`/`not implemented`/`panic("not supported")`, `capability`/`effect`
  in `internal/vm`, `break`/`continue` language-wide, `*.yml`/`.yaml` (none), `*stdlib*`/`*std*` (none),
  `*contribut*`/`*develop*` (none).
- No mutating commands were run before this point in the session (build/test/go commands were left to
  the exploration agents' read-only greps and code reads; nothing was executed that changes state).

## Findings

### Confirmed bugs
**None new.** The only open bug remains #39 (`-o <dir>` silently ignored when placed after the input
file for the Go/JS backend, Pending, Score 6.0) — verified still accurately described against the
current `bugs.md`. Considered but deliberately did NOT file as new bugs:
- The untracked `cmd/zero/main.go` skips `checker.Check` entirely before codegen. This is real,
  reproducible code — but it isn't part of the tracked, documented product surface (git doesn't know
  about it), so it doesn't meet the bar of "Zero accepts invalid programs" for the shipped CLI. Flagging
  it as a repo-hygiene question for the user instead of filing a bug or improvement against it (see
  Unresolved Questions below) — didn't want to guess whether it's the user's own in-progress work.
- `internal/backend/wasm/wasm.go`'s commented-out `ast.ReportError` calls (the legacy, non-SSA Wasm
  backend). This makes `GenerateWasmCode` unsafe to call without the checker running first, but the
  only documented call path (the real `zero.go` CLI) always runs the checker first, so no invalid
  program is actually accepted today. Real latent risk, but not a currently-reproducible defect against
  the documented CLI — didn't file as a bug per the "must be reproducible" bar; could be raised as a
  small future hygiene item if someone wants to remove dead code, but wasn't scored as a backlog item
  this session to avoid backlog noise for a non-reproducible risk.
- The checker's silent "unknown symbol → `ast.Unknown`" behavior for unbound identifiers is a
  **documented, deliberate design choice** (`types.go:65-67`), not a bug — filed as improvement #77
  instead (a semantics/diagnostics gap, not a defect against spec).

### Confirmed improvements (10 new items, #75-#84)
All ten are grounded in direct code evidence gathered by the exploration agents (file:line references
are in each item's Detail section in `improvements.md`). Summary:

| # | Title | Score | Theme |
|---|---|---|---|
| 75 | Add CI (go build/vet/test on push/PR) | 3.5 | Foundation / Engineering |
| 76 | `zero validate` (validation without side effects) | 3.5 | CLI maturity / AI-agent workflow |
| 77 | Checker: unbound variable/function diagnostics | 2.0 | Foundation / semantics |
| 80 | Module system Phase 1: inventory & design | 2.0 | Foundation / modules |
| 82 | JS backend unit test coverage | 2.0 | Testing |
| 78 | Cross-backend differential testing harness | 1.75 | Foundation / testing |
| 79 | Capability enforcement Phase 1: VM allow/deny gate | 1.6 | Security / capabilities |
| 73 | (existing) Standalone Runtime Phase 2b: Collections | 1.5 | Wasm/SSA |
| 81 | Formatter Phase 1: canonical `zero fmt` | 1.5 | DX / formatter |
| 83 | JS backend AI-primitive parity | 1.25 | Application readiness |
| 84 | SSA IR: lower `for`/`match`/`try_let`/`spawn` | 1.2 | Language completeness |
| 74 | (existing) Standalone Runtime Phase 2c: LLM HTTP | 0.43 (below floor) | Wasm/SSA |

Each new item's `improvements.md` entry includes: Description, Why, Impact, Subsystem/files,
Dependencies, Acceptance criteria, Required tests, Documentation requirements, recommended execution
mode, and recommended Claude model (with rationale), per this run's required granularity. Gemini/OpenAI
model columns were left as `—` for all ten new rows — this session had no `agy`/`codex` tool access, so
per the "do not infer availability for one provider from another provider's model listing" rule, I did
not guess slugs for those columns.

### Duplicates avoided
- Did not re-file Wasm/SSA/collections work — #73 (collections) and #74 (LLM/HTTP + long-tail builtins)
  already cover that ground; #84 was scoped narrowly to the specific `for`/`match`/`try_let`/`spawn` gap
  that is genuinely outside both #73 and #74's stated scope (verified by re-reading both items' exact
  wording before filing #84).
- Did not re-file "JS backend missing AI primitives" as a bug — it's already correctly classified as a
  coverage gap (checker rejects it cleanly) in bug #37's own Done note, which explicitly deferred it as
  a follow-up that was never filed until now (#83).
- Did not create a competing roadmap/TODO file — all new items went into the existing `improvements.md`
  Ranked Backlog table plus Details section, following the exact existing format.

### Required themes explicitly NOT filed this session (deferred, noted for a future grooming pass)
To keep this pass focused and each item genuinely session-sized rather than shallow, the following
required-theme categories were reviewed and found to already have partial coverage or were judged lower
priority than the 10 items above — not filed as new items this session:
- **Machine-readable agent context / constrained generation**: substantially covered already by shipped
  #68 (Native Logit Masking), #69 (Optimization Signatures), #70 (Schema Bridges), and
  `cmd/codegen`'s orchestrator schema generation. A gap audit here would be a reasonable future item but
  wasn't obviously as high-value as the 10 filed.
- **Versioned diagnostic schema / error codes**: real gap (diagnostics are message-string-only, no error
  code field) but overlaps enough with #77 (checker diagnostics) that filing both in one pass risked
  double-scoping the same subsystem before #77 lands and reveals the real shape needed.
- **Bounded repair context, source-preserving localized patching, LSP/editor services, intent-to-test
  traceability**: real per the mission doc, but no direct code evidence gathered this session to scope
  them concretely — would need a dedicated exploration pass rather than being bolted onto this one.
- **CLI subcommand grammar** (`zero <verb> file`) unifying the current flag-only surface: intentionally
  deferred until #76 (`zero validate`) and #81 (`zero fmt`) exist as standalone flags first, so a future
  subcommand-grammar redesign has two working examples to generalize from rather than guessing the shape
  up front.

## Ranking decisions

Scores computed as Value×Decay÷Effort per the existing formula; all ten new items use Decay=1.0 (each
opens a genuinely new theme in this backlog — no prior same-theme item to decay against). Inserted into
the Ranked Backlog table in descending-score order, interleaved with the existing pending #73/#74 rows
rather than appended after them, so the table's "best ROI first" framing stays locally accurate at the
actionable frontier (the table as a whole is not, and has never been, strictly globally sorted — many
early Done rows predate the scoring convention entirely).

#75 (CI) and #76 (`zero validate`) tied at 3.5. Broke the tie in CI's favor: CI is the more foundational,
cross-cutting multiplier (protects every other item in both backlog files going forward, including the
other 9 new items just filed), matching the ranking rule that foundational work should rank highly when
it unlocks/protects many later items. `zero validate` remains the clear #2 and a strong follow-on.

## Dependency relationships (new items)

- #75 (CI): no dependencies; recommended to land first since #78 (differential testing) benefits from
  running inside it.
- #76 (`zero validate`): no dependencies. Related to #77 and #84 (same checker/CLI surface).
- #77 (unbound-identifier diagnostics): no dependencies. Extends the already-Done #64 (Semantic Type
  Checker Pass) rather than replacing it.
- #78 (differential testing harness): benefits from #75 (CI) but not blocked by it.
- #79 (capability enforcement Phase 1): no hard dependency; recommended to land after #75/#76 exist so
  its new CLI flag surface has test/CI coverage from day one.
- #80 (module system Phase 1): design-only, no dependencies; blocks a not-yet-filed Phase 2.
- #81 (formatter Phase 1): no dependencies; a future Phase 2 (source-preserving) would build on it.
- #82 (JS backend tests): no dependencies; recommended before or alongside #83.
- #83 (JS AI-primitive parity): benefits from #82 landing first (regression net before adding surface).
- #84 (SSA IR gaps): related to but not blocked by #73/#74 (same subsystem family, disjoint scope);
  recommended alongside or after #73 since realistic `for`-fixtures usually iterate a collection.

## Unresolved questions for the user

1. **`cmd/zero/main.go` is untracked, duplicates a subset of `zero.go`'s flags, and skips
   `checker.Check` entirely.** Is this your own in-progress scaffolding, or should it be deleted /
   committed / reconciled with `zero.go`? Not touched this session — flagged rather than guessed, per
   the instruction to investigate unfamiliar state rather than delete or assume.
2. Confirm whether the two "deferred theme" categories above (versioned diagnostic schema; CLI
   subcommand grammar) should be filed as concrete items in the *next* grooming pass, or left implicit
   until #77/#76/#81 land and reveal more of their real shape.

## Recommended first implementation item

**Improvement #75 — Add CI: run go build/vet/test on every push and PR.**

Reasons: tied for the highest score (3.5) in the backlog; zero dependencies; lowest risk of the top
candidates (new, isolated file — no application/runtime/parser/checker code touched); explicitly named
as the top summary finding of the pipeline-maturity audit ("no CI at all... every cross-backend/fixture
verification claim in this backlog is manual and unrepeatable"); and it protects every one of the other
nine items just filed, plus the entire existing backlog, from silent regressions going forward — the
clearest "unlocks/protects many later items" case in this pass. `zero validate` (#76) is the strong
second choice and a natural follow-on next session.

## Confirmation

No implementation, compiler, runtime, backend, parser, checker, test, example, website, packaging, or
CI code was changed this session. Only `improvements.md` (10 new ranked items + detail sections) and
this journal file were modified.
