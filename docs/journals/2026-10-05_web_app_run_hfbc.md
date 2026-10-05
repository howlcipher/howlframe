# web_app artifact execution dogfood

Base HEAD was confirmed as 3d668620ffe176d6644cfbd4e9eaca81616706b0 on
web-app-hfbc-dogfood before editing. No rebase or assurance tip-lock update.

The existing -run-bc path reads the artifact and invokes RunBytecodeWithPolicy;
howlframe.go needs no change. The existing web_app write-and-return path still
uses runHFIRGate plus CompileToBytecode. Production HFIR and -compile-bc remain
unflipped, and no DOM/document opcodes were invented (including for Motoko).
Explicit build --target=go|js|wasm and the other application roots are unchanged.

A CLI probe compiled tests/test_swarm_js.howl to a temporary .hfbc successfully,
then -run-bc exited 1 with UNSUPPORTED_CONSTRUCT at instruction 0, opcode TASK.
The artifact also contains SPAWN_AGENT; these constructs remain unsupported by
the VM, so this fixture proves writing but cannot prove successful execution.

The new examples/web_app_hello.howl has only a print child. TestWebAppRunHFBC
builds the CLI, plants decoy go/node/wat2wasm executables, and compares both
no-flag and flagged -o artifacts against default build bytecode. Compile-only
stdout must be empty, artifacts must be nonempty, and no app.js/app.test.js may
appear. Both artifacts and howlframe run source exit 0 and print exactly
"Hello from web_app hfbc\n", without any allowed capabilities. PRINT's opcode
metadata has no required capability. Decoy tools are never invoked.

Validation uses Go 1.24.4 and GOCACHE=/tmp/howlframe-web-dogfood-gocache.
The requested TestWebApp|TestHTTPServer command passes TestWebAppRunHFBC (both
subtests), TestWebAppNoFlagAndFlaggedWriteBuildBytecode,
TestHTTPServerFlaggedWritesBuildBytecode, and TestHTTPServerNoFlagWritesBuildBytecode.
TestHTTPServerServeHFBC fails because this sandbox rejects listening on :8080
with "socket: operation not permitted". Assertions and production behavior were
not weakened. Build, vet, formatting, SEO, and diff whitespace checks pass.
The benchmark harness was rerun in an isolated /tmp copy with the writable cache
and VCS stamping disabled for that copy; seven tests pass, while its HTTP
reference test fails with the same socket denial. Initial harness attempts used
a read-only default cache / a copy lacking VCS metadata; those setup issues were
corrected on rerun. No generated artifacts were added to the repository root.

Validation is incomplete in this restricted environment; no local commit is
created while checks are red, per the repository instructions.

The full Go suite also exits nonzero: status_api and task_api hit the 180-second
package timeout; examples, Go/JavaScript backends, VM, and difftest encounter
socket-denied failures. The JavaScript backend also reports process execution
assertion failures. The root package, bytecode, construct, and HFIR packages pass.
Full output is retained locally at /tmp/howlframe-web-dogfood-probe/full-test.log.

Outside the Codex workspace-write sandbox (parent agent validation), the focused
command passes including TestHTTPServerServeHFBC:

```
GOCACHE=/tmp/howlframe-web-dogfood-gocache go test -count=1 -timeout 180s -v ./examples/ -run 'TestWebApp|TestHTTPServer'
```

PASS: TestHTTPServerFlaggedWritesBuildBytecode, TestHTTPServerNoFlagWritesBuildBytecode,
TestHTTPServerServeHFBC, TestWebAppNoFlagAndFlaggedWriteBuildBytecode,
TestWebAppRunHFBC (flagged, no-flag).
