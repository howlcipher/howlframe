# wasm_app artifact execution dogfood

Base HEAD was confirmed as 293fc9b9b6d3234c1187ad698c1ec1f6304e073d on
wasm-app-hfbc-dogfood before editing. This test-only change mirrors merged
PR #93's TestWebAppRunHFBC. No tip-lock retake or production change was needed.

Prior CLI probes reported that examples/wasm_math.howl writes bytecode, but
its top-level return makes both -run-bc and howlframe run exit 1 with
VM_INTERNAL and message "{42}". That fixture remains unchanged for existing
write tests. A wasm_app containing print is rejected by the checker with
"Wasm backend does not support \"print\"", including bytecode/default writes;
the checker remains unchanged.

The new examples/wasm_app_hello.howl contains only an arithmetic conditional:
(if (> (+ 2 3) 4) (* 6 7) 0). TestWasmAppRunHFBC builds the CLI from the
repository root, uses toolchainFreeEnv, and plants decoy go/node/wat2wasm
executables that record invocation and exit 99. Both flagged -o and no-flag
subtests compare nonempty .hfbc artifacts with howlframe build's default
bytecode. Compile-only commands have a three-second deadline and empty stdout;
all tested CLI commands must have empty stderr. No app.wat, app.wasm, app.js,
or server.go may appear in either temporary directory, and the decoys must
never be invoked.

Both -run-bc artifact and howlframe run source execute with the default
capability policy (no -allow-caps), exit 0, and produce identical empty stdout
and stderr. These assertions passed for both subtests. This establishes
execution for this expression fixture, without claiming general Wasm/DOM
coverage. No CLI flags, opcodes, TASK/SPAWN_AGENT, -compile-bc flip, or
howlframe.go changes were added. Generated artifacts stay in test temp dirs.

Validation used Go 1.24.4 and the writable cache
/tmp/howlframe-wasm-dogfood-gocache:

```
gofmt -w examples/wasm_app_run_hfbc_test.go
GOCACHE=/tmp/howlframe-wasm-dogfood-gocache go test -count=1 -timeout 180s -v ./examples/ -run 'TestWasmApp|TestWebApp'
GOCACHE=/tmp/howlframe-wasm-dogfood-gocache go build ./...
GOCACHE=/tmp/howlframe-wasm-dogfood-gocache go vet ./...
gofmt -l .
git diff --check
```

All commands passed; gofmt -l emitted no paths. The focused suite passed
TestWasmAppNoFlagAndFlaggedWriteBuildBytecode,
TestWasmAppBuildTargetWasmStillEmitsWAT, TestWasmAppRunHFBC (flagged, no-flag),
TestWebAppNoFlagAndFlaggedWriteBuildBytecode, and TestWebAppRunHFBC (flagged,
no-flag). Package execution took 3.475s. Validation covers the affected
application examples plus repository-wide build, vet, and formatting; the
full test suite and unrelated benchmark/SEO checks were not run for this
fixture-and-test-only change. The commit is local only; no push or PR creation.
