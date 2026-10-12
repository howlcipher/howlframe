# 2026-10-12 — Issue #141: table-driven arity check and Go/JS target sweep

## What changed

### 1. One arity table, enforced before codegen
- `internal/construct/arity.go` declares an `Arity{Min,Max}` for every
  construct-registry entry. `Variadic` means there is no maximum. 134 entries
  have an arity and 20 are exempt, out of 154 registry entries.
- Each exemption carries a reason. Most are Unsupported constructs or
  directives (`include`, `use`, `patch`, `with_context`) that earlier passes
  consume. The rest are pure type annotations (`type_hint`, `type_hints`,
  `type_param`).
- `internal/construct/arity_test.go` is the drift guard. It fails if a
  registry entry has neither an arity nor an exemption. It also fails on
  stale keys, malformed ranges, or an entry that has both.
- `construct.EnforceArity` runs in `howlframe.go` immediately before each of
  the five `ast.Desugar` call sites. That covers every pipeline: bytecode,
  HFIR bytecode, the interpreter (`-run`), `check`, and Go/JS builds.
  - A wrong arity reports
    `ARITY_ERROR: read_file expects 1 arg, got 0`
    (or `at least N args`, or `N to M args`).
  - Exit code is 1, with no panic and no artifact, under both the default
    and governed profiles.
- The walk is context-aware. It skips let binding lists, defun and lambda
  parameter lists, dict pairs, and circuit argument lists, and it handles
  `switch` and `cond` clauses. So `(let (min 0 max 3) ...)` is not treated as
  a call to `min`.
- The AST bytecode compiler keeps the same table as a backstop at the top of
  `compileNode`. The #139 fetch-specific check moved onto the table:
  `fetch` is `{2,3}`.
- `and`/`or` are `{2, Variadic}` because desugar already folds the n-ary
  form. The bytecode `and`/`or` case now fails closed on an unfolded n-ary
  node. Before this change it silently dropped every operand after the
  second.

### 2. Tests
- `TestIssue141MotokoArityRepros` covers the full repro list from the issue,
  plus `fetch` with 1 arg.
- `TestIssue141EveryArityEntry` runs one too-few call and one too-many call
  for every table entry (where those exist) under both profiles. Each must
  give exit 1, an ARITY_ERROR, no panic and no artifact. Under the governed
  profile, a PROFILE_FORBIDDEN_CONSTRUCT rejection is also accepted.
- `TestIssue141ArityFuzzNoPanic` tries arities 0..max(4, Min+1) for all 154
  registry heads, about 4.3k CLI runs. The build paths are default bytecode,
  governed bytecode, `-compile-hfir-bc`, `--target=go` and `--target=js`.
  `-run` is used only at rejected arities, so no program executes and there
  is no network, LLM or exec.
- The fuzz test found panics, all now fixed:
  - Empty `(export)` and `(module)` panicked in the checker on every path.
    Both now have an arity of 1+.
  - On `--target=go`, `trace`, `fuzzy_cast`, `semantic_match`,
    `assert_semantic`, `web_app` with no port, and `web_app` with non-list
    handlers all panicked. Their `ast.ReportError` shape checks had been
    commented out. They are re-enabled, and the `semantic_match` case check
    now compares against the real node type `"List"`.

### 3. Go/JS: never a false "Built"
- `rejectUnsupportedTargetValueForms` (in `howlframe.go`, after desugar)
  fails closed on any `let`, `do`, `if`, `for` or `while` used as a value on
  `--target=go|js`, with `UNSUPPORTED_ON_TARGET: <form> used as a value ...`.
  - These backends emit those forms as statements, so the output did not
    compile.
  - Sugar that lowers to this shape is now rejected instead of printing
    "Built": all/any, ver_gt/ver_ge, not, starts_with/ends_with,
    list_contains, map_has, cond as a value, a value `if`, and
    list_get/parse_json on a computed operand.
- `time_now` used inside an expression on `--target=go` (for example via
  `unix_now`, i.e. `(to_string (time_now))`) emitted an empty expression.
  It now emits `int(time.Now().Unix())`. `(print (time_now))` used to compile
  to `fmt.Println()` and print an empty line; it now prints Unix seconds.
- `TestIssue141TargetNeverFalseBuilt` builds 31 construct probes for each
  target. Each build must either be rejected or produce output that passes
  `go vet` (with a replace-module) or `node --check`.

## Behavior changes
- Programs with a wrong arity that used to build, sometimes silently wrong,
  now fail with ARITY_ERROR. `check` also reports it.
- The test `TestCLIRequiredCapsAndGoverned` used a 1-arg `(achieve "goal")`.
  It now uses the 2-arg form.
- Go/JS builds of the value-position sugar listed above are rejected. Before,
  they printed "Built" and wrote output that did not compile.

## Not done / caveats
- Optional fetch body: not done. Making a bytecode fetch body fail closed
  breaks the locked `TestProdFlipCriteriaLock/fetch_body` and
  `nested_multi_effect_*` ABI conformance cases. Those cases pin the
  documented "body operand is skipped" behavior. Changing them is a decision
  about the flip criteria, so it stays out of this slice.
- Arity is checked on the head symbol only. Sub-form shapes (switch clauses,
  route tuples) are still validated by each backend.
- The Go/JS value-position guard is conservative. A value `if` is rejected
  even in a spot where a backend might have special-cased it. No existing
  test or benchmark relied on that.
- No production -compile-bc/HFIR flip, #90 untouched, no tip-lock, no live
  LLM, no DOM. No new language features and no token claims.
