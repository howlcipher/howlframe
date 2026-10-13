# Browser relative fetch compatibility

The documented Board showcase fetches `missions.json` from its own page.
Current generated browser code constructed `new URL(raw)` without a base,
so correctly scoped relative fetches failed before a request. Two positive
same-origin browser harness cases reproduced CAPABILITY_DENIED before edits.

Relative URLs now use the host page location only in hosts without Node
process. The existing exact-host/port, scheme, DNS and redirect controls still
run on the resolved URL. The request uses the checked absolute URL; a different
document base cannot redirect it. Credential-bearing browser URLs fail before
fetch. Node retains absolute-URL requirements and environment authority.

Regression cases verify exact delivered URLs, relative path resolution,
explicit/default ports, denied other hosts, scheme-relative hosts, coarse
grants, missing bases, file bases and credential bases. A real Node test with
fake page globals remains denied. Existing browser grant, provider-error and
Node-authority regressions pass.

The corrected compiler plus the separate Board host-grant/runtime candidate
passed real Chromium desktop/mobile ingestion of the immutable 35-event,
17-mission historical ledger and the five-mission static showcase. This does
not imply live campaign telemetry or default-branch availability. Separate
native full-host failures on symlinked Rust coreutils reproduce unchanged in
pristine Frame; supported Ubuntu full CI and final PR head must be inspected.
