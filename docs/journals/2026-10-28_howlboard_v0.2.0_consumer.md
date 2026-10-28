# HowlBoard on the published v0.2.0 binary

HowlBoard (`main` `2e17439`) builds its backend, interface and ledger importer with HowlFrame and pinned CI to source
commit `e77e008f`. With the published v0.2.0 linux/amd64 executable (`af921689…`) placed at `howlframe_bin`, and no
HowlFrame checkout, every HowlBoard CI step passed unchanged: `make build`, backend contract tests (23 tests, 119
subtests), the import step (`entries=72`, `missions=16`), docs, SEO and the Chromium layout test. The vendored
EvidenceDesk ledger module is byte-identical to v0.2.0's copy. No HowlFrame change was needed.

Consumer change: howlcipher/howlboard#24 installs the release through a script that checks a SHA-256 pinned in
HowlBoard; its CI on GitHub installed v0.2.0 that way and passed.

Findings for Frame, from running the importer with different grants:

* `CAPABILITY_DENIED` for a path outside a `filesystem:read=` root, and for a store file that needs read and write on
  its own path, both say only `capability denied: filesystem`. It took three runs to find a working path-scoped grant
  (`filesystem:read=<repo>,filesystem:write=<repo>,database`); HowlBoard's Makefile still uses the coarse grant.
* Runtime errors give the function, instruction index and opcode (`instruction 481 STORE_OPEN`), not a source
  position. HFBC instructions carry no positions.
* The importer catches a denied read and prints `Failed to read ledger file`; the cause is lost in application code.

Details and the recommendation that follows from them:
`docs/release-readiness/v0.2.0/HOWLFRAME_POST_RELEASE_VALIDATION.md`.
