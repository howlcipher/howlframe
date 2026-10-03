# Lowered HFIR ABI: time_now on the experimental bytecode path

## Why this slice

`time_now` already returns a Unix timestamp on the interpreter, the AST bytecode VM, Go, and JavaScript. `TIME_NOW` is an existing opcode. It pops 0, pushes 1, and grants nothing. Experimental `-compile-hfir-bc` still rejected it with `HFIR_BYTECODE_UNSUPPORTED`, so the two bytecode compilers were not compared on that construct. `regex_match` already lowers onto `REGEX_MATCH`. This slice lowers `time_now` only, in that same shape, onto the opcode that already exists. #90 stays Partial.

## What was compared

`LowerAST` gives `(time_now)` kind `time_now` and no data edge. `LowerToBytecode` emits the existing `TIME_NOW` opcode, with no operand, the same instruction `bytecode.CompileToBytecode` already emits. A node with a data edge returns `HFIR_BYTECODE_UNSUPPORTED` and no `BCProgram`. The model-adapter transport still rejects `time_now`. There is no new opcode and no new capability.

The oracle is instruction identity. `(cli_app (print (time_now)))` is one bare `TIME_NOW`, then `PRINT`, on both compilers. A second program nests `time_now` with `let`, `if`, `defun`, `while`, and `for`. Both artifacts contain six identical `TIME_NOW` instructions and no timestamp constant. The test does not run either artifact. Two live runs would print two clocks, so stdout is not the parity check.

| Host | How this case reaches it |
| --- | --- |
| `hfir_bytecode` | `-compile-hfir-bc`. Experimental path. Compared as an artifact, not as a live clock. |
| `bytecode` | `-compile-bc`. Production AST bytecode. Canonical artifact. |

`internal/vm/hfir_equivalence_test.go` round-trips both artifacts and requires the instruction streams to match, including one bare `TIME_NOW` per call. `hfirRejectedParity` stays empty. The promote-blocker fence is not. `time_now` leaves that fence. The decision stays Defer the production flip.

## How to run it

CI runs `go test ./...` (`.github/workflows/ci.yml`), which includes these tests.

Focused commands:

```
go test ./internal/vm -run TestHFIRBytecodeTimeNowMatchesAST
go test ./internal/hfir -run 'TestLowerToBytecodeTimeNow|TestLowerASTTimeNow'
go test . -run TestProdFlipCriteriaLock
```

## What this does not do

No new opcode. No new capability. Production `-compile-bc` is still `runHFIRGate` and then `bytecode.CompileToBytecode` on the AST. The model-adapter transport still rejects `time_now`. No fetch-body implementation. `OpFetch` is untouched. No new Assurance tip-lock. The named lock stays `4d74dbcf9654caa05e0b1d9212b15bc5398359e3`, and this slice leaves it stale. The readiness scores `docs/journals/2026-10-03_lowered_hfir_prod_flip_score_5a229d65.md`, `docs/journals/2026-10-03_lowered_hfir_prod_flip_score_33cb1325.md`, and `docs/journals/2026-10-03_lowered_hfir_prod_flip_score_2e5c8e98.md` stay scores of those SHAs. This slice does not edit those journals. The fence inventory `docs/journals/2026-10-03_lowered_hfir_fence_inventory_ffb753b.md` stays a record of `ffb753b` and is not rewritten. The Decision line stays "Defer the production flip." No Wasm and no module linker. Matching artifacts on these programs does not mean `-compile-bc` consumes HFIR. #90 stays Partial.
