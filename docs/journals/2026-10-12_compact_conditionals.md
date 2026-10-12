# Compact conditionals (2026-10-12)

## Goal

Close the conditional-task gap from #134. There, HowlFrame's mean on the 22
conditional tasks was 344.55 tokens, against Python's 196.00. The language
additions had to stay pure, and the governed checks had to stay fail-closed.

## What shipped

All four are `ast.Desugar` sugar, expanding into constructs that already lower
(`internal/ast/desugar_compact.go`). No new opcode, capability or effect is added.

- `(ver_gt a b)` and `(ver_ge a b)`: strict numeric x.y.z comparison.
- `(all x list pred)` and `(any x list pred)`.
- `list_get` accepts any expression as its list. Non-symbol operands are bound to a gensym.
- `(parse_json T <expr>)` binds its operand to a gensym. The governed
  pre-desugar profile scan (`construct.scanSupported`) now descends into the
  operand instead of rejecting the shape. The default construct scan stays strict.

`all`, `any`, `ver_gt` and `ver_ge` are registered as `CompileTimeOnly`, in
sorted position, and are listed in the bytecode drift test as consumed before lowering.

## Guards (Lain)

- **Purity:**
  - The unit test checks that expansions use only a closed set of pure heads.
  - The CLI test builds a governed program that uses every form. `inspect --caps`
    must report no capabilities, and running it must match the expected truth table.
- **Fail-closed ordering:**
  - `exec`, `llm_generate` or `spawn` is hidden inside an inline `parse_json`,
    preceded by `write_file`.
  - Governed `check` and `build` must both fail with `PROFILE_FORBIDDEN_CONSTRUCT`.
  - No artifact, side file or exec output may exist afterwards.
  - The allowed shape still builds.

## Motoko follow-up

`experiments/token-efficiency-v0/verification_test.go` checks that
`results/verification.json` has 60 × 4 entries, all passing, each with a SHA-256 that matches the source.

## PR-range correction

#134 cited "#124–#130" and its brief cited "#128–#130". In fact, `git log -S` on
`internal/ast/desugar.go` shows the following sources:

| Sugar used by the references | Introduced in |
|---|---|
| root `do` | #117 |
| `read_text`, `lines` | #118 |
| multi-binding `let` | #120 |
| `when` | #124 |

No #128–#130 sugar is used. The report and the #134 journal are corrected.

## Results (cl100k / o200k)

| Measure | Before | After |
|---|---|---|
| HowlFrame median | 124 / 123 | **117 / 116** |
| vs Python (median per-task ratio) | 1.059 / 1.058 | **0.983 / 0.983** |
| vs JS | 1.019 / 0.953 | **0.959 / 0.896** |
| vs Go | 0.496 / 0.492 | **0.445 / 0.446** |
| Conditional-task mean | 344.55 | **186.00** (Python 196.00) |

Python and JS medians are unchanged at 119 / 118 and 122 / 129.

HowlFrame now uses no more tokens than Python on any task: it is smaller on 44
and ties on the 16 write-record tasks. It is smaller than JS on all 60 tasks.

## Caveats

- The features were shaped by this corpus, so a held-out corpus is needed.
- `ver_*` errors at runtime on version strings with fewer than three parts.
  It does not return false.
- The gensym counter makes desugared temporary names process-order dependent.
  This does not affect semantics.

## Next hot spots

1. KV lookup for `current=`/`proposed=`: 35 tokens.
2. A pure `filter` to replace the for/when/append loop: about 27 tokens against Python's 17.
3. `dict` pair syntax and the escaped JSON literal for the blocked payload.
