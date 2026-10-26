# Publication reconciliation, 2026-10-21

The user authorized pushing and merging the fresh milestone work. Local main
contained four unpublished commits and had diverged from origin/main. This
publication preserves both histories; no force push or unrelated PR merge is
part of the task.

## Integration decisions

* Merged the remote assurance/runtime work through `e6fdd48a` in `6a77b844`.
  Both listener-host and timing flags remain, with timing capture placed around
  the new framing/serialization logic. The remote deferred store-lock release,
  panic recovery and store timing implementations are retained.
* Preserved complete local and remote M2 decision/review records under explicit
  provenance headings. The newer Owner waivers apply only to their recorded
  pin and are not extended to replacement code. No archived B6 isolation result
  has been relabeled PASS.
* The remote `milestone3/M3_ACCEPTANCE_SPEC.md` and sealed-corpus manifest are
  preserved. That separate receipt-verifier/web specification is not claimed
  implemented by the independently frozen EvidenceDesk project track.
* Included concurrent main update `8d8cb168` (interop/schema synchronization).
  The interop flag descriptor lists the new listen-host flag on both command
  interfaces; the descriptor/flag tests pass without relaxing their assertions.
* Kept the remote legacy unindexed-survivor regression. For the new atomic
  format, it explicitly constructs the old interrupted-delete state and uses
  offline migration; all canonical survivors must be recovered, readable and
  deletable. The old-format path retains its original expectation. This is a
  schema-specific fixture adaptation, not removal of the original failure case.

## Verification

The first combined runtime passed an uncached full Go suite, build/vet/format,
101 Ticketdesk acceptance tests, 11 retained adversarial cases and all 10 legacy
review regressions. Repository write-cost, v2 reference, real-app parity, harness
unit and SEO checks passed. Generated benchmark report side effects were
restored; incoming raw assurance logs were preserved byte-for-byte rather than
whitespace-normalized. Whitespace is checked against the PR target, where those
logs already exist.

After the concurrent interop merge, targeted interop/listener/timing tests,
build/vet/format and the pinned-schema producer checks passed. EvidenceDesk's
full manual workflow passed again against the combined compiler: six frozen
checks, Chromium, restart/encoded IDs/size bounds, actual adapted HowlBoard
consumption and capability denials. Final full Go and GitHub required-build
results are recorded by the publication run and PR checks.

GitHub main is protected by the `build` status. Publish through a dedicated PR
and merge only after that status succeeds. No unrelated open PR is included.
