# wasm_app bytecode write-and-stop

Default wasm_app invocation, including source followed by -o, now gates for
bytecode, compiles children in sequence with CompileToBytecode, writes
<stem>.hfbc, and returns before VM execution. The construct registry marks the
root Supported; unsupported child forms still fail closed. No DOM, Wasm, or
browser opcodes were added.

The consumer regression uses examples/wasm_math.howl with decoy go, node, and
wat2wasm tools and a three-second timeout. Default build, flagged default
invocation, and no-flag default invocation produce nonempty identical HFBC
artifacts. Those paths emit no app.wat, app.wasm, app.js, or server.go.

Explicit build --target=wasm remains the WAT route and writes app.wat when -o
names a directory. Existing Wasm backend tests now request that target
explicitly, so they continue to exercise GenerateWasmCode and the WAT
diagnostics while the default wasm_app path dogfoods bytecode write-and-stop.

Construct coverage was regenerated after marking wasm_app Supported. The
living promote-blocker fence now includes wasm_app because production bytecode
accepts it and experimental -compile-hfir-bc has no matching root lowering yet.
This does not retake the Assurance tip-lock or rescore readiness.
