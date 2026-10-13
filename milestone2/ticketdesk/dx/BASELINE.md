# DX baseline ("before" evidence)

Recorded 2026-10-13 from the **unmodified** toolchain at the freeze commit
`52c0c531` (milestone2 plan freeze; the earlier crypto built-ins commit `2d8d7945` does not touch
diagnostics). Produced by exporting that commit with `git archive 52c0c531`, running
`go build -o <scratch>/hf .` and invoking `<scratch>/hf check|build <fixture>` from `dx/fixtures/`.
(The live working tree was being modified by the implementer and did not compile at that moment,
so the commit export is the reproducible "before".) `howlframe version`: `HowlFrame 0.1.0`, `HFBC format: 1`.

Reproduce: `git archive 52c0c531 | tar -x -C /tmp/base && (cd /tmp/base && go build -o /tmp/hf .)`, then
`HF_BIN=/tmp/hf python3 dx/check_dx.py -v`.

## Raw outputs (stdout+stderr, exit code 1 unless noted)

### `unclosed_deep.howl` - `howlframe check`
```
{"reason":"Expected ')'","line":37,"column":1}
[exit 1]
```
### `unclosed_deep.howl` - `howlframe build`
```
{"reason":"Expected ')'","line":37,"column":1}
[exit 1]
```
### `extra_close.howl` - `howlframe check`
```
{"reason":"Unexpected tokens after EOF","line":7,"column":3}
[exit 1]
```
### `extra_close.howl` - `howlframe build`
```
{"reason":"Unexpected tokens after EOF","line":7,"column":3}
[exit 1]
```
### `extra_close_end.howl` - `howlframe check`
```
{"reason":"Unexpected tokens after EOF","line":4,"column":1}
[exit 1]
```
### `extra_close_end.howl` - `howlframe build`
```
{"reason":"Unexpected tokens after EOF","line":4,"column":1}
[exit 1]
```
### `missing_call.howl` - `howlframe check`
```
OK: missing_call.howl
[exit 0]
```
### `missing_call.howl` - `howlframe build`
```
[{"code":"HFIR_TARGET_INFEASIBLE","severity":"ERROR","message":"construct \"triple\" cannot be compiled for the standalone bytecode target: no bytecode lowering exists for it","location":{"filename":"missing_call.howl","line":10,"column":30},"contract_version":"v1","target":"bytecode"}]
[exit 1]
```
### `let_three_args.howl` - `howlframe check`
```
{"reason":"ARITY_ERROR: let expects 2 args, got 3","line":3,"column":4}
[exit 1]
```
### `let_three_args.howl` - `howlframe build`
```
{"reason":"ARITY_ERROR: let expects 2 args, got 3","line":3,"column":4}
[exit 1]
```
### `if_void_int.howl` - `howlframe check`
```
{"reason":"if branches have incompatible types void and int","line":7,"column":7}
[exit 1]
```
### `if_void_int.howl` - `howlframe build`
```
{"reason":"if branches have incompatible types void and int","line":7,"column":7}
[exit 1]
```

## Interpretation per PLAN.md section 3

| Item | Fixture | Before | Why it hurts |
|---|---|---|---|
| 1 | `unclosed_deep.howl` | `Expected ')'` at line 37 col 1 (EOF); the real culprit, the `(if` on line 33 col 5, is not named | the fix location must be found by bisecting |
| 1 | `extra_close.howl`, `extra_close_end.howl` | `Unexpected tokens after EOF` at the first token after the early close; the stray `)` and the form it closed are never named | wording hides that a `)` is extra |
| 2 | `missing_call.howl` | `howlframe check` prints `OK`; `build` says `construct "triple" cannot be compiled for the standalone bytecode target: no bytecode lowering exists for it` (reported only at the inner call) | suggests a missing language feature, not a missing `(call ...)` |
| 3 | `let_three_args.howl` | `ARITY_ERROR: let expects 2 args, got 3` | no hint about `(do ...)` |
| 4 | `if_void_int.howl` | `if branches have incompatible types void and int` at the `if` only | neither branch location nor remedy |

Note on item 2: `howlframe check` passes this program today (the defect is only found by `build`), and a
nested implicit call such as `(print (to_string (triple 5)))` happens to compile; only the nested form
`(triple (triple 2))` fails. The fixture reproduces the failing form.

## Checker result against the unmodified compiler

`HF_BIN=<scratch>/hf python3 dx/check_dx.py` ends with `13 checks, 2 passed, 11 failed`: both passes are
the `ok_program` control; every improvement check FAILs, as required before implementation.
