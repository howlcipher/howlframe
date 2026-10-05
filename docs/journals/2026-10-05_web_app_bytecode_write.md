# web_app bytecode write-and-stop

Default web_app invocation, including source followed by -o, now gates for bytecode,
compiles children in sequence with CompileToBytecode, writes <stem>.hfbc, and returns
before VM execution. The construct registry marks the root Supported; unsupported
child forms still fail closed. No DOM or document opcodes were added.

There is no examples/*.howl web_app root. The consumer regression uses
tests/test_swarm_js.howl (spawn_agent/task) with decoy go/node/wat2wasm tools and a
three-second timeout. Both invocations produce nonempty artifacts identical to
default howlframe build and emit no app.js or app.test.js. Explicit build --target=js
retains DOM JavaScript generation; the differential JavaScript runner now requests
that target explicitly.

Required web/HTTP, output-directory, and construct tests passed. Build, vet,
formatting, SEO checks, and the eight benchmark harness tests passed. The full Go
suite exposed construct coverage drift and the newly supported root's absence from
the experimental HFIR promote-blocker fence; coverage was regenerated and web_app
added to that fence. This does not retake the assurance tip-lock or rescore it.

Both initially failing Go packages passed on rerun; all other packages passed in
the full suite, including tools/difftest. Local socket tests required execution
outside the socket-restricted sandbox, and Go used a writable /tmp cache.
