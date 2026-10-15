# Milestone 2 handoff

* Active milestone: Milestone 2 (gates A, C, D, G, H of directive D29). Plan, contract, acceptance suite and
  DX checker frozen 2026-10-13 by an independent author.
* Status: implemented. Acceptance 101/101; DX checker 13/13; guide rule test passes.
* Reproduce everything from a clean checkout: `milestone2/scripts/clean_repro.sh`.
* Remaining: independent final review against the frozen contract (REVIEW.md, MILESTONE2_DECISION.md in
  `milestone2/`), then stop. Do not start Milestone 3 without a new instruction.
* Known limitations: serialized single-process server; quadratic in-language sort for listings; the 14
  host-specific exec test failures from Milestone 1 remain.
