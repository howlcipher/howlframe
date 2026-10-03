# Lowered HFIR ABI: sleep on the experimental bytecode path

## Why this slice

`sleep` already pauses on the interpreter, the AST bytecode VM, Go, and JavaScript. `SLEEP` is an existing opcode. It pops 1, pushes 0, has an empty operand list, and grants nothing. Experimental `-compile-hfir-bc` still rejected it with `HFIR_BYTECODE_UNSUPPORTED`, so the two bytecode compilers were not compared on that construct. `time_now` already lowers onto `TIME_NOW`. This slice lowers `sleep` only, in that same shape, onto the opcode that already exists. #90 stays Partial.

## What was compared

`LowerAST` gives `(sleep …)` kind `sleep` and one data edge. The edge name stays empty. That empty name is the duration. `SLEEP` has no operand field to store a name, so this slice does not assign `duration`, `ms`, or `value`. `LowerToBytecode` compiles that child and emits the existing `SLEEP` opcode, with empty operands, the same instruction `bytecode.CompileToBytecode` already emits after the same child. A node without that one empty-named edge, or with a name on the edge, returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. The model-adapter transport still rejects `sleep`. There is no new opcode and no new capability.

The oracle is instruction identity. `(cli_app (sleep 0) (print "ok"))` is `LOAD_CONST` of `0`, then one `SLEEP`, then `LOAD_CONST` of `"ok"`, then `PRINT`, on both compilers. A second program nests `sleep` with `let`, `if`, `defun`, `while`, and `for`. Both artifacts contain six identical `SLEEP` instructions. Each one sits after that call's compiled duration. The test does not run either artifact. Wall-clock delay is not the parity check.

| Host | How this case reaches it |
| --- | --- |
| `hfir_bytecode` | `-compile-hfir-bc`. Experimental path. Compared as an artifact, not as a live delay. |
| `bytecode` | `-compile-bc`. Production AST bytecode. Canonical artifact. |

`internal/vm/hfir_equivalence_test.go` round-trips both artifacts and requires the instruction streams to match, including one `SLEEP` after each compiled duration. `hfirRejectedParity` stays empty. The promote-blocker fence is not. `sleep` leaves that fence. The decision stays Defer the production flip.

## How to run it

CI runs `go test ./...` (`.github/workflows/ci.yml`), which includes these tests.

Focused commands:

```
go test ./internal/vm -run TestHFIRBytecodeSleepMatchesAST
go test ./internal/hfir -run 'TestLowerToBytecodeSleep|TestLowerASTSleep'
go test . -run TestProdFlipCriteriaLock
```

## What this does not do

No new opcode. No new capability. Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST. The model-adapter transport still rejects `sleep`. No fetch-body implementation. `OpFetch` is untouched. `read_line` stays outside the lowerer. No new Assurance tip-lock. The named lock stays `4d74dbcf9654caa05e0b1d9212b15bc5398359e3`, and this slice leaves it stale. The readiness scores `docs/journals/2026-10-03_lowered_hfir_prod_flip_score_5a229d65.md`, `docs/journals/2026-10-03_lowered_hfir_prod_flip_score_33cb1325.md`, and `docs/journals/2026-10-03_lowered_hfir_prod_flip_score_2e5c8e98.md` stay scores of those SHAs. This slice does not edit those journals. The fence inventory `docs/journals/2026-10-03_lowered_hfir_fence_inventory_ffb753b.md` stays a record of `ffb753b` and is not rewritten. The `time_now` journal is not rewritten. The Decision line stays "Defer the production flip." No Wasm and no module linker. Matching artifacts on these programs does not mean `-compile-bc` consumes HFIR. #90 stays Partial.
