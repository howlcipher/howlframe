# Bare path fail-closed for unnamed roots

Base HEAD was confirmed as `7c9c052ed4063df7a6267f4b44300f030aa9c2b4`
(`Dogfood wasm_app .hfbc under -run-bc (#95)`) before editing. No tip-lock
retake and no production `-compile-bc` flip.

After #85–#95, every named root (`cli_app`, `http_server`, `web_app`,
`wasm_app`) writes `.hfbc` on the bare/default path. The leftover `else`
branch still called `gogen.GenerateCode` and wrote `server.go` /
`server_test.go` for any root that was not one of those four. That branch is
now fail-closed with a clear diagnostic naming the unsupported root and
pointing at `howlframe build --target=go` for explicit Go emission.

The checker already rejects unnamed roots before dispatch, so the visible
rejection for `(defun …)` / `(module …)` / unknown heads remains the existing
checker diagnostic. The bare-path `else` is defense in depth: it no longer
emits Go if dispatch and the checker ever diverge. Named-kind `.hfbc`
behavior is unchanged. Explicit `build --target=go` still writes `server.go`.

Tests:

- `TestBarePathUnnamedRootsFailClosed`
- `TestBarePathNamedRootsStillWriteHFBC`
- `TestBarePathExplicitGoTargetStillEmitsServerGo`

No new opcodes. No docs readiness rescore. No tip-lock retake.
