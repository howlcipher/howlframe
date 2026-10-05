# wasm_app top-level return and print dogfood

Base HEAD was confirmed as 2010eadff4aa3e6a97e0e669532f535e3b1474e6 on
wasm-app-return-print-dogfood before editing. This is a minimal host-VM fix so
examples/wasm_math.howl and print can dogfood under test-only -run-bc. No
tip-lock retake or production -compile-bc flip.

## Diagnosis

After #95, wasm_app_hello.howl (expression-only) ran under -run-bc, but
examples/wasm_math.howl still exited 1 with VM_INTERNAL and message "{42}".
Bytecode OpReturn panics VmReturn for callee unwinding; RunBytecodeWithEvidence
only recovered VmExit and *VMError, so a top-level RETURN became VM_INTERNAL.
AST Interpret already recovers returnSignal and uses an int64 as the exit code.

(wasm_app (print ...)) was rejected by checkWasmExpression ("Wasm backend does
not support \"print\""). HFBC compilation and the VM already implement PRINT;
only the checker blocked the host path. Explicit build --target=wasm has no
stdout import, so WAT stays fail-closed for print.

## Fix

- RunBytecodeWithEvidence recovers VmReturn like Interpret recovers
  returnSignal: int64 becomes evidence.ExitCode; other values exit 0.
- checkWasmExpression accepts print and recurses on arguments.
- GenerateWasmCode reports "Wasm backend does not support \"print\"" so WAT
  does not emit an empty/invalid print expression.

examples/wasm_math.howl is unchanged. examples/wasm_app_print.howl is
(do (print (* 6 7)) 0) for host print dogfood (exit 0, stdout "42\n").

## Tests

```
GOCACHE=/tmp/howlframe-wasm-return-gocache go test -count=1 -timeout 60s -v ./internal/vm/ -run 'TestTopLevelReturn|TestWasmAppPrint'
GOCACHE=/tmp/howlframe-wasm-return-gocache go test -count=1 -timeout 180s -v ./examples/ -run 'TestWasmApp|TestWebApp'
GOCACHE=/tmp/howlframe-wasm-return-gocache go build ./...
GOCACHE=/tmp/howlframe-wasm-return-gocache go vet ./...
gofmt -l .
git diff --check
```

PASS: focused VM return/print tests; TestWasmAppNoFlagAndFlaggedWriteBuildBytecode;
TestWasmAppBuildTargetWasmStillEmitsWAT; TestWasmAppRunHFBC; TestWasmAppMathRunHFBC
(exit 42, empty stdout); TestWasmAppPrintRunHFBC (exit 0, stdout "42\n");
TestWebApp* dogfoods. Manual CLI: wasm_math -run-bc / run exit 42; print HFBC
prints 42; build --target=wasm on print still fail-closed.

## Deliberately not changed

- production -compile-bc / HFIR flip
- tip-lock
- opcodes / TASK / SPAWN_AGENT
- howlframe.go compile-only write path from #94
- examples/wasm_math.howl body
