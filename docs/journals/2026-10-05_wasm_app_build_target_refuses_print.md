# build --target=wasm still refuses print (#97 nit)

Base HEAD was confirmed as 0afc82c11b41fc304a8bf7d44cb2aea63cb318da on
wasm-app-build-target-refuses-print before editing. Test-only follow-up to #97:
host HFBC accepts print, but explicit WAT transpile must stay fail-closed. No
tip-lock retake, production -compile-bc flip, opcode, TASK, SPAWN_AGENT, or argv
work.

## Gap

#97 opened checkWasmExpression for print so examples/wasm_app_print.howl dogfoods
under -run-bc / run, and left GenerateWasmCode reporting
`Wasm backend does not support "print"` (WAT has no stdout import). Manual CLI
checks covered that refusal; automated coverage only proved successful
`build --target=wasm` on wasm_math.howl (TestWasmAppBuildTargetWasmStillEmitsWAT).

## Change

Add TestWasmAppBuildTargetWasmRefusesPrint in examples/wasm_app_bytecode_test.go:
`howlframe build --target=wasm examples/wasm_app_print.howl -o <dir>` must exit
nonzero with the existing diagnostic and must not write app.wat.

## Tests

```
GOCACHE=/tmp/howlframe-wasm-print-gocache go test -count=1 -timeout 180s -v ./examples/ -run 'TestWasmAppBuildTargetWasm'
GOCACHE=/tmp/howlframe-wasm-print-gocache go build ./...
GOCACHE=/tmp/howlframe-wasm-print-gocache go vet ./examples/
gofmt -l examples/wasm_app_bytecode_test.go
git diff --check
```

## Deliberately not changed

- production code (checker, WAT emitter, VM)
- production -compile-bc / HFIR flip
- tip-lock
- opcodes / TASK / SPAWN_AGENT / argv
- examples/wasm_app_print.howl body
