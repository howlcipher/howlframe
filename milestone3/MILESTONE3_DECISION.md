# Milestone 3 decision

ACCEPTED, 2026-10-21, implementation commit `b028e813`.

Both D30 objectives are demonstrated: EvidenceDesk is a complete HowlFrame
backend/browser application, and its standalone projector usefully replaces the
ledger-import responsibility of the real existing HowlBoard component.

* [Original independent plan](PLAN.md) and frozen
  [requirements](evidencedesk/requirements/REQUIREMENTS.md), committed at `9210f20b`
  before implementation. [Acceptance protocol](evidencedesk/requirements/ACCEPTANCE.md).
* [Final independent review](evidencedesk/INDEPENDENT_REVIEW.md): R1–R6 accepted,
  six frozen tests, real Chromium, restart/encoded-ID/oversize/error/XSS workflows,
  capability/resource tests and actual adapted HowlBoard consumer execution.
* [Committed clean reproduction output](../docs/journals/evidence/fresh_milestones/m3_clean.txt)
  and [script](evidencedesk/scripts/clean_repro.sh), with fresh environment,
  HOME/temp/Go caches and explicitly installed host browser/package prerequisites.
* [User guide](evidencedesk/README.md) and
  [overall verification journal](../docs/journals/2026-10-21_fresh_milestones.md).

The original unadapted Board route failure is retained. Its actual source needs
one explicit capability declaration in a temporary copy; the independent
[R5 clarification](evidencedesk/requirements/R5_COMPATIBILITY_ADDENDUM.md) records
that compatibility procedure, hashes and constraints. No sibling domain code or
production stores were changed. This is an existing HowlFrame component
replacement, not first language adoption or production deployment.

Limits remain explicit: bounded inputs, single-user loopback operation, a subset
of the evidence schema, whole-state persistence, and two-file projector output
publication (validation precedes writes, but OS write failures can leave partial
outputs). No OS sandbox or cross-provider proof is claimed.

Stop after recording local closure. The user-requested three milestones are
complete; no further feature work, push, pull request or merge is authorized.
