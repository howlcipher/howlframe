# Milestone reconciliation after remediation

This document relates the repairs to the three milestones. **It does not rewrite, supersede or re-grade** any
historical acceptance record (`MILESTONE{1,2,3}_DECISION.md`, the independent reviews, the Owner waiver records). Official
verdicts remain the Owner's to set. The audit's verdicts (M1 PASS, M2 PARTIAL, M3 PARTIAL) are used as the starting
point; what follows says which of the audit's *reasons* have been addressed and what is left.
Evidence is in `HOWLFRAME_REGRESSION_RESULTS.md`.

## Milestone 1 (LedgerLens, HealthWatch, TaskFlow)

* **Frozen tests unchanged and passing:** 35 + 29 + 32 on the remediated build. Frozen requirement and test files were not
  edited (`git diff 5d2983b -- 'milestone1/*/requirements' 'milestone1/*/tests/acceptance'` is empty).
* **Addressed from the audit:** HF-02 (#181) was the one runtime-level finding that touched TaskFlow ("deadline leaves
  descendants running"); the audit's own probe now passes.
* **Behavior note:** TaskFlow children now run under the exec supervisor. Exit codes, signal reporting, output capture and the
  exec output limit are unchanged (pinned by existing tests plus the frozen suite).
* **Still open (not fixed here):** raw runtime JSON on runner-limit stops (HF-12) conflicts with the frozen hygiene
  requirements and remains the known carryover; reviewer independence (HF-07) is a process matter this remediation addresses only by
  adding a different-provider review (see the handoff).
* **Net:** the audit's one runtime blocker for Milestone 1's "H" evidence category is addressed; nothing about the milestone's
  functional acceptance changed.

## Milestone 2 (TicketDesk)

| Audit reason for PARTIAL | Status |
|---|---|
| Restart fails under the documented command at ~300 tickets, a regression from the previous implementation (HF-03) | **Addressed.** 1,000 tickets + 100 tokens restart under the documented command in 0.3 s; verified at 6,000 tickets; permanent regression added and run in CI |
| One non-reading client stalls the server (HF-04) | **Addressed**, with a loopback regression |
| Retained Assurance suite red (HF-08) | **Addressed for the regression**: the port-in-use diagnostic names the port again (`5 tests, 0 failed, 1 expected failures`, the state recorded at the previous pin). The lost-race case (m12) stays a documented expected failure |
| CI did not run the suite (HF-09) | **Addressed**: all M2 suites run in CI |
| Raw runtime JSON on failure paths (R1.3) | Restart path no longer reaches it; the lost-race path (m12) still does; deferred |

Frozen acceptance 101/101, DX 13/13, reassessment 11/11, historical regressions 10/10 all pass on the remediated build; frozen
M2 files are unchanged. **One application source change was made:** the port message in `ticketdesk.howl` (both byte-identical
copies, so `milestone2/ticketdesk/src` and `milestone2/rebuild/ticketdesk/src` still match). Source hashes quoted in older
review documents refer to the pre-remediation file and are historical.
**Not changed and still true:** the historical Owner waiver (`W-B6-venue`) and the INFRA-VOID Round 7 records apply to the
older implementation and are not turned into a pass. Crash consistency was re-verified under process kill at default
ceilings; **power-loss durability remains untested.**
Whether these repairs justify re-labelling Milestone 2 from PARTIAL is for the Owner and a fresh independent review.

## Milestone 3 (EvidenceDesk and the HowlBoard component)

* **EvidenceDesk itself:** unaffected in behavior. One real regression was introduced and fixed during this remediation
  (single-file filesystem grants, which the projector uses); the fix has its own tests. The project's own
  `scripts/test.sh` workflow (6 frozen tests, real Chromium, restart, XSS, grant denials, adapted-consumer runs) is rerun
  in the regression results.
* **The ecosystem claim:** see `milestone3/EVIDENCE_STATEMENT_2026-10-21.md`. Summary: the recorded failure reproduces on the stale
  checkout; current upstream consumes the output unmodified under its declared grants; nothing is integrated upstream;
  the R5 addendum has no recorded Owner approval.
* **Verdict:** nothing here moves D30 B from "compatibility demonstrated" to "adopted". That would need an
  Owner decision plus integration work in the HowlBoard repository, neither of which is part of this mission.
* **Separate receipt-verifier specification:** untouched (draft, unimplemented).

## What this remediation does not establish
Production readiness; OS-level isolation; any competitive result; independent verification by a human; behavior of the
generated Go and JavaScript targets (not changed); Windows or macOS behavior of the new containment code (compiled for,
not executed on).
