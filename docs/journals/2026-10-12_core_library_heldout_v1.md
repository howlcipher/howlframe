# 2026-10-12: Core library, judged on a second frozen held-out set (heldout-v1)

Base: main `6ed0184e` (#137 merged). No production `-compile-bc`/HFIR flip,
#90 untouched, no tip-lock, no live LLM/H1, no DOM.

## Step A: freeze (commit `31f85c14`)

`experiments/token-efficiency-heldout-v1/tasks/` was committed first, before
any library code, solution or measurement. It has 34 tasks in 7 categories:
text, table, numbers, lists, strings, records and logic. None repeats a task
from v0 or heldout-v0. `TestHeldoutV1SpecsFrozen` pins these hashes:

| File | sha256 |
|---|---|
| tasks/README.md | c7837434a99bac3f3289342ac77505a2aed74ec34b6316a19e74e5ad8eb2dd29 |
| tasks/build_specs.py | 7eb654f478aadebb316769d7757986352d96821828d78e8d93b58475245badd4 |
| tasks/manifest.json | b10c646bf5ed04f552b1ef28b5f29adb9a693501a66595cfc400043c6887af29 |

## Step B: core library (pure)

Every built-in is sugar in `internal/ast/desugar_corelib.go`. It expands into
existing lowered constructs, so it adds no opcode, capability or effect. It
behaves the same on the interpreter, AST bytecode (`-run-bc`) and HFIR
bytecode. The built-ins are:

- `list_slice lst a [b]` and `str_slice s a [b]`. Negative indices count from
  the end. With two arguments, the slice runs to the end.
- `str_len`
- `index_of` (returns -1 when absent)
- `map_list x lst expr`
- `filter x lst pred`
- `reduce acc init x lst expr`
- `sum`
- `min` and `max`
- `reverse`
- `count_by x lst key` (returns a dict of counts)
- `sort` (stable, ascending)

**String ordering.** `<`, `>`, `<=` and `>=` now accept two strings and compare
them in byte order, on both the interpreter and the bytecode VM. HFIR shares
the VM. Mixed string and number operands are still a TYPE_ERROR.

**Registry.** The constructs are registered as `CompileTimeOnly` in
`internal/construct`. The bytecode construct drift test lists them, and
`construct_coverage.md` was regenerated.

**Tests.**
- `TestCoreLibraryBuiltinsArePureOnEveryPath` checks the default and governed
  profiles, that `inspect --caps` reports no capabilities, and that the
  interpreter, governed bytecode and HFIR bytecode give identical output.
- `TestStringOrderingRejectsMixedOperands`.

**Gaps.**
- **Go/JS targets:** the sugar does not compile there. Both backends lower
  let-in-expression as a statement; this is the same pre-existing bug as `nz`.
  It is documented and not fixed in this slice.
- **Bytes, not characters:** `str_split s ""` splits bytes, so `str_slice` and
  `str_len` are byte-based.
- **No negative literals:** the lexer does not read `-1` as an integer.
- **No `%`.**

## Step C: results (measurement; see heldout-v1 REPORT.md)

All 136 references pass the verifier. `TestHeldoutV1VerificationCoverage`
checks hash integrity and coverage.

| cl100k_base | Median per-task ratio | W/T/L |
|---|---|---|
| HowlFrame vs Python | 1.026 | 16/0/18 |
| HowlFrame vs JS | 0.922 | 26/2/6 |
| HowlFrame vs Go | 0.339 | 34/0/0 |

- Medians: HowlFrame 52.5, Python 53.0, JS 58.0, Go 153.5.
- o200k_base gives the same picture against Python (16/0/18), and 29/0/5
  against JS.

**Honest reading:** HowlFrame is now level with Python on unseen tasks and
loses slightly per task. It wins on lists and text (0.77× and 0.83×). It loses
on logic (1.56×), table (1.12×), strings (1.11×) and records (1.07×).

**Hot spots:**
- No `%`, `break`, `elif` or stack pop.
- No `max_by`/`min_by` (the extreme-then-`index_of` pattern).
- No row destructuring.
- No negative literals.

**Caveat:** the same agent wrote the tasks and the library. The freeze came
first, but this is weaker than a third-party blind set.

## Secondary: heldout-v0 rerun (NOT blind)

Nine heldout-v0 HowlFrame solutions were rewritten with the library. The
median per-task ratio vs Python went from 1.309 to 1.109 (W/T/L 0/1/29 to
5/0/25). HowlFrame still loses to Python on heldout-v0. h31 and h32 remain
gaps. The details are in the heldout-v0 REPORT addendum.
