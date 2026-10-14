# Positive exec fixture portability

Native full verification repeated14 exec/receipt/ABI fixture failures on this
host. The runtime correctly rejects symlinked executable paths, while positive
fixtures assumed /usr/bin coreutils were regular files. Here printf/touch/env/echo
resolve into /usr/lib/cargo/bin/coreutils and true resolves to gnustrue.

Resolve only trusted positive-fixture text and its matching argv grants through
the existing testutil package. Missing tools fail. Keep runtime policy and explicit
symlink-denial tests unchanged. Mount overlays would hide the host layout; copied
mock commands would reduce real-process coverage; changing runtime policy would
weaken a security boundary.

The frozen ABI assets remain byte-for-byte unchanged. The differential runner
writes an isolated canonical-path fixture copy and adjusts only the associated
allowlist; existing output, error, side-effect and parity assertions remain.
Direct/legacy/generatedGo/generatedJS/receipt and HFIR conformance exec tests
pass after the correction. A compile-time typed-capability mismatch in the first
test adaptation was corrected; its failed output remains in the mission record.
Full native and supportedCI verification and independent review are required.
