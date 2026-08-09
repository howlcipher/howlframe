# Standalone Runtime Blueprint Reconciliation (2026-08-07)

## Objective

Reconcile `docs/standalone_runtime_blueprint.md` against the actual repository, groom `improvements.md`
and `bugs.md` accordingly, and select exactly one next implementation task. Planning only — **no
implementation code was changed.**

Model: Claude Opus 5, Plan Mode → reviewed edits on planning documents only. No permission-bypass mode.

## Method

Documentation was not trusted. A transpiler binary was built to the session scratchpad
(`go build -o <scratch>/howlframebin howlframe.go`, go1.26.5) and every claim below was reproduced against tracked
fixtures. Code was read directly in `howlframe.go`, `internal/ast`, `internal/parser`, `internal/checker`,
`internal/hfir`, `internal/bytecode`, `internal/vm`, `internal/capability`, `tools/difftest`,
`docs/reference/`, and `.github/workflows/`.

## Findings

### F1 — CONFIRMED BUG: the standalone bytecode path silently drops unsupported constructs

`internal/bytecode/bytecode.go`'s `compileNode` head-dispatch switch (roughly lines 100-473) has **no
`default` case**. Any unrecognized head falls through the whole switch and returns an empty instruction
slice — the construct and everything nested inside it disappear from the artifact with no diagnostic and
exit code 0.

Reproduced against `tests/test_advanced_control.howl`, which should print `zero`, `one`, `other`:

```
$ ./howlframebin -run tests/test_advanced_control.howl
{"reason":"\"match\" is not supported under -run in Phase 1 (...)","line":5,"column":6}
exit 1                                       # correct — the interpreter fails closed

$ ./howlframebin -compile-bc tests/test_advanced_control.howl -o m.bin && ./howlframebin -run-bc m.bin
exit 0                                       # WRONG — no output at all, artifact written
```

`tests/test_void_defun.howl` behaves identically (its `test` block vanishes). An invented head passes
every source-based gate:

```
$ ./howlframebin -validate   t_bogus.howl        # exit 0
$ ./howlframebin -compile-bc t_bogus.howl        # exit 0
$ ./howlframebin -run-bc     b.bin               # prints before/after, skips (totally_made_up_head ...), exit 0
```

Why nothing catches it:

- `internal/checker`'s `checkGoStatement` default (`checker.go:524`) just recurses into children. It has
  to stay permissive — the Go backend supports a large open set of heads.
- `howlframe.go`'s `runHFIRGate` blocks only on `HFIR_INVALID_REF` / `HFIR_TARGET_INFEASIBLE`, and
  `internal/hfir/verifier.go`'s `isFeasible` has real rejection rules **only** for `target == "wasm"`.
  `"bytecode"` is permissive by default.
- `internal/vm`'s opcode dispatch does have a fail-closed `default` (`vm.go:1946`), but it is never
  reached — the compiler emitted no instruction to dispatch on.
- `tools/difftest` structurally cannot see it: it exempts 15 of ~40 fixtures as `"unsupported in run"`,
  which is precisely the set where the interpreter fails closed and the bytecode path silently lies. CI
  never runs difftest at all.

Heads reachable from tracked fixtures that currently vanish: `match`, `test`, `use`, `export`, `module`,
`struct`, `lambda`, `route`, `middleware`, `catch`, `trace`, `patch`, `with_context`, `fuzzy_cast`,
`assert_semantic`, `type_hints`, `type_param`, `go_import`.

This breaks the blueprint's standalone criterion #5 and release gate #6. `README.md` lines 25, 32, and 204
are factually wrong as a result.

Filed as **bug #45**.

### F2 — HFIR is a shadow pass, not the lowering source

The blueprint's target path is `HFIR -> verifier -> lowered HFIR -> bytecode -> VM`. The code does
`AST -> bytecode`: `howlframe.go:107` calls `bytecode.CompileToBytecode(root)` on the AST, with `runHFIRGate`
verifying a separately-lowered graph alongside. No backend consumes HFIR. `hfir.LowerAST` is a near-1:1 AST
mirror whose `ControlEdges` field is declared but never populated.

Improvements #86 and #87 are legitimately Done for what they scoped. But "HFIR owns semantic meaning" is
not yet true of the execution path, and #90's description did not say so. **#90's description was updated**
to record this rather than filing a duplicate.

A consequence that matters for #45's fix: `hfir.LowerAST` derives `Kind` from the head symbol of *every*
list, so `(let (x 0) body)` produces a HFIR node with `Kind == "x"`, and parameter lists, dict pairs, and
match arms produce kinds named after user identifiers and literals. Any HFIR-side rejection rule must be an
explicit deny-list, never "anything not supported is infeasible".

### F3 — Learning and adaptation: nothing exists

`grep -rniE "experience|policy|skill" --include=*.go internal/` returns **0** matches. All four levels of
the blueprint's learning model are unimplemented. Model integration is a hardcoded
`http://localhost:11434/api/generate` Ollama call duplicated across `internal/backend/gogen/gogen.go` and
`internal/vm/vm.go`; provider-neutrality exists only in `internal/masking` (mask plans), not as an adapter.

This is expected at this stage. Per the blueprint's sequencing and the session brief, only **Level 1** was
given a tracker item (#100), explicitly blocked on #88. Levels 2-4 got nothing.

### F4 — Tracker hygiene defects

Improvements **#95, #96, #97, and #98** existed only as rows in the ranked table. Their anchors were
**dangling** — no `### 95.`…`### 98.` body sections existed anywhere in the file, so they carried no
Description, Why, Dependencies, Acceptance criteria, or Required tests. Every other item in the file has
one. #97's row additionally carried the journal filename `2026-08-07_improvement_97.md` in the **OpenAI
model** column, and #97 was marked Done with no completion note.

### F5 — Blueprint filename mismatch

The blueprint was committed as `docs/standalone_runtime_blueprint_learning_adaptation.md` and untracked.
Renamed to `docs/standalone_runtime_blueprint.md` per user decision. **Content unchanged** — no
architectural contradiction or factual mistake was found in it.

## Maturity assessment

| Dimension | Maturity |
| --- | --- |
| Standalone runtime | Early, and untrustworthy at the boundary. Deterministic core, `cli_args`, `read_line`/`stderr`/`exit` (#97), file ops, JSON parse, and deny-by-default capability enforcement all reach the VM — but unsupported constructs fail **open** (F1), modules and `test` blocks are absent from HFIR/bytecode/VM, artifacts are bare `gob`, and there is no project lifecycle, packaging, or release story. |
| AI-native runtime | Primitive-level only. AI opcodes exist but hardcode Ollama. No provider-neutral adapter (#88), no schema-constrained output boundary, no budgets, no provenance, no deterministic test doubles, no bounded repair (#91). AI operations are opcodes, not HFIR effects. |
| Learning / adaptation | Absent (Level 0). See F3. |

## Tracker changes applied

**`bugs.md`**
- Added **#45** — `-compile-bc` silently drops every construct the bytecode compiler does not handle.
  Score 2.67 (8×1÷3). Includes the full repro transcript, the fix sketch, the deny-list/allow-list caveat
  from F2, required tests, documentation impact, and non-goals.

**`improvements.md`**
- Wrote the missing body sections for **#95, #96, #97, #98** (F4), matching the shape of #88-#94. Anchors
  chosen to match the links the ranked table already used. #97 gained its Done note.
- Fixed **#97's row**: journal filename moved out of the *OpenAI model* column; status now `Done (2026-08-07)`.
- Updated **#90's Description** with the F2 finding (HFIR is off the execution path).
- Added **#99** — generated runtime coverage matrix (blueprint Milestone 0 exit gate), depends on bug #45.
- Added **#100** — structured experience memory (learning Level 1), explicitly blocked on #88.

**Duplicates deliberately avoided:** modules in the VM → #95; `test` blocks in the VM → #96; artifact
hashing/versioning → #98; model-adapter neutrality → #88; bounded semantic repair → #91; HFIR/backend
semantic drift → #90 (description updated instead); `use` resolution → bug #43. No item was renumbered, no
completed history was deleted, and no new roadmap, TODO, backlog, or status file was created.

## Priority order

1. **Bug #45** — fail-closed unsupported constructs on the standalone path *(selected)*
2. Bug #43 — `use` fails to resolve; blocks #95
3. #95 — modules in HFIR + bytecode + VM
4. #96 — native VM `test` block execution
5. #98 — artifact validation, hashing, versioning
6. Bug #42 — `HFIR_UNBOUND_REF` false positives
7. #99 — generated coverage matrix
8. #88 — provider-neutral HFIR model adapter
9. #100 — structured experience memory
10. #90 / #89 / #91 — lowered-HFIR ABI, content-addressed HFIR, semantic deltas
11. #92 — Wasm binary pipeline; then #73 / #84 / #81 / #83

## Next task

**bugs.md #45.** Claude **Sonnet 5** / Gemini **3.1 Pro (High)** / OpenAI **gpt-5.6-sol**, execution mode
`plan-then-reviewed-edit`.

It goes first because it is a confirmed correctness bug on the runtime the blueprint designates as
primary, and because every item below it is currently unfalsifiable — #95 and #96 appear to "work" under
`-compile-bc` precisely because their constructs vanish without complaint. It also delivers the
authoritative construct-support registry that Milestone 0 and #99 depend on, fits one focused session, and
has a ready-made regression fixture in `tests/test_advanced_control.howl`.

## Confirmation

No implementation code was changed in this session. Nothing under `internal/`, `tools/`, `cmd/`, `tests/`,
`examples/`, `benchmarks/`, `docs/reference/`, `.github/`, `howlframe.go`, or `howlframe_test.go` was modified. The
only repository changes are `bugs.md`, `improvements.md`, this journal, and the blueprint rename. Scratch
artifacts stayed in the session scratchpad; nothing was written to the repository root.
