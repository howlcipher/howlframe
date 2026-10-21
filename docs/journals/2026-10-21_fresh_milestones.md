# Fresh requirements-first milestone rebuild

User authority: start over all three product milestones as if Cursor's prior
work had not been completed, compare new work, and replace it where better.
The initial review-and-fix approach was explicitly corrected after the user's
clarification. No prior accepted label substitutes for this rebuild's evidence.

Original requirement freezes remain unchanged. Milestone 3's independent plan,
requirements and initial tests were committed at `9210f20b` before application
implementation. Its evidence-driven scope combines a complete evidence-review
web application and a real HowlBoard ledger-projector replacement. Original
HowlBoard route declarations were incompatible with current attenuation; the
independent R5 addendum transparently permits a capability-only temporary adapter,
retains the failed original consumer run and forbids domain changes.

## Roles and replacement decisions

Root implemented the fresh Ticketdesk candidate, narrow runtime repairs and
review repairs. Separate agents wrote new Ledgerlens/Taskflow and Healthwatch
from the frozen contracts, and a fourth implementer wrote EvidenceDesk. Separate
evaluators authored M1/M2 held-out probes and independently reran application
tests. This is role separation in the same Codex session, not cross-provider
proof. Review service quota briefly interrupted evaluation; the review agents
subsequently resumed. No human implementation or repair was supplied.

Canonical M1/M2 sources now match the fresh candidate snapshots in `rebuild/`.
Prior implementations remain in Git history. Ledgerlens removes the observed
budget cap and aggregate overflow. Taskflow preserves typed literal/reference
segments and rejects denied output grants before creating any history/report.
Healthwatch recreates the complete monitoring workflow with explicit validation;
its replacement reflects the user's fresh-rebuild/maintainability preference,
not an unsupported claim of superior speed or functionality. Its initial empty
state regression is retained in the independent review and was repaired.

Ticketdesk uses one atomic record per ticket bucket including membership, and
one atomic token collection. This removes split-index crash states at the cost
of linear token authentication and whole-state persistence. The optional offline
migration preserves old canonical records and never overwrites existing output.
The legacy B6 regression still crafts the original dangling-index state; version
2 now recovers it through this explicit migration, since that index no longer
exists in the new storage design. The frozen suite was not changed.

## Verification ledger

Host tools: Ubuntu 26.04 userspace on Linux 7.2.4-ogc3.1.fc44.x86_64;
Go 1.26.0, Python 3.14.4, GNU Bash 5.3.9, Git 2.53.0, GNU tar 1.35,
Node and Python Playwright with Chromium 148.0.7778.96. Execution crossed from
2026-10-20 into 2026-10-21 UTC; dated initial selection/review records retain
their original dates. Isolation means fresh archive, inherited-environment
removal, HOME/temp/Go caches and installed host tools. It is not OS sandboxing.

Fresh candidate verification performed before committed reproduction:

* M1 frozen acceptance: Ledgerlens 35/35, Healthwatch 29/29, Taskflow 32/32,
  independently rerun after repairs. Reviewer large-positive and mixed-negative
  92,234-row exact-cent workloads pass with an explicitly raised 1 TiB allocation
  accounting budget (not reserved host memory). The original 4 GiB accounting
  limit legitimately stops that workload; the high-budget run distinguishes the
  arithmetic defect from permitted finite-resource termination.
* M2 frozen acceptance 101/101, DX 13/13, original review regressions 9/9,
  additional retained reassessment 11/11. Independent final source/hash review
  and actual migration execution are in the new candidate review.
* M3 independent frozen tests 6/6, real Chromium import/filter/select/claims/
  annotations/reload/keyboard/error/XSS and arbitrary-ID workflows, exact restart
  persistence, denied grants and finite instruction bounds. Real adapted Board
  consumes both the three-record review ledger and the committed sanitized
  72-record/16-task fixture. No live private telemetry is read.
* Core regression: uncached `HOWL_FORBID_LOCAL_INFERENCE=1 go test -count=1 ./...`,
  build, vet, formatting and whitespace checks. Two old arity diagnostic goldens
  required the truthful new fetch upper bound (4); their behavioral rejection and
  no-artifact assertions remain intact.
* CI benchmark proofs are rerun with the rebuilt CLI. Their generated metric
  files are test side effects, not milestone source changes. The first v2 proof
  used a stale ignored executable; the CI's explicit CLI build precedes rerun.

## Evidence and reproduction entry points

* [M1 independent review](../../milestone1/rebuild/INDEPENDENT_REVIEW.md)
  and [clean reproduction](../../milestone1/rebuild/scripts/clean_repro.sh).
* [M2 independent review](../../milestone2/rebuild/ticketdesk/REVIEW.md)
  and [clean reproduction](../../milestone2/rebuild/scripts/clean_repro.sh).
* [M3 independent review](../../milestone3/evidencedesk/INDEPENDENT_REVIEW.md),
  [user guide](../../milestone3/evidencedesk/README.md),
  [clean reproduction](../../milestone3/evidencedesk/scripts/clean_repro.sh),
  and [compatibility addendum](../../milestone3/evidencedesk/requirements/R5_COMPATIBILITY_ADDENDUM.md).

These committed-HEAD reproduction scripts require Bash, Git, tar and ordinary
host utilities, Go, Python and module-download access when needed. M3 additionally
requires the installed Playwright package/browser and the actual sibling Board
checkout's committed source and sanitized fixture. Its package/browser path is
explicitly retained as a host-tool prerequisite while the owner HOME/environment
are replaced. Paths and hashes are printed by the executable evidence scripts.

Status: independent application checks pass. Final committed-tree reproduction,
evidence-link review and closure commit remain to be recorded below. No push,
pull request, merge, new milestone or recursive optimization phase is authorized.
