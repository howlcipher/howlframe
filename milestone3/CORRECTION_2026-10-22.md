# Milestone 3: dated correction, 2026-10-22

Owner decisions C and D, recorded in `docs/adoption/HOWLFRAME_OWNER_DECISIONS.md`. This file corrects how the Milestone 3
outcome is described. [MILESTONE3_DECISION.md](MILESTONE3_DECISION.md) and the other ACCEPTED records stay as written.

## D30 verdicts

| Objective | Verdict | Basis |
|---|---|---|
| D30 A: a complete HowlFrame full-stack application | **Demonstrated** | EvidenceDesk's API, persistence, real-Chromium and XSS workflows pass; reproduced by the 2026-10-21 audit and again on `main` in the adoption mission (`docs/adoption/evidence/08_*`) |
| D30 B: a useful component adopted by an existing project | **Partially adopted** | HowlBoard's production import path validates input with EvidenceDesk's ledger module (`milestone3/evidencedesk/app/ledger.howl`, vendored byte-identical with a hash check; HowlFrame #211, HowlBoard #20, merged 2026-10-22). EvidenceDesk's projector is **not** used |
| D30 J: evidence-based sequencing | **Demonstrated** | The M3 plan and decision records |

## What changed from the original decision

* The original decision says the standalone projector "usefully replaces the ledger-import responsibility" of HowlBoard.
  Measured against HowlBoard's own verification-truth tests, it does not: it fails 8 assertions, including recording a
  failed check's exit code 7 as 0, and it drops summaries and provenance (`docs/adoption/evidence/03_truth_contract.txt`).
  The projector should not be described as a replacement. HowlBoard keeps its own projection.
* What HowlBoard adopted is the **validation module**: invalid ledgers are refused with a line number before anything is
  written, where the previous importer skipped, double-counted, truncated, crashed or (empty file) wiped the timeline.
  Output on valid input is unchanged. Details: `docs/adoption/HOWLFRAME_ADOPTION_REPORT.md`.

## R5 compatibility addendum (decision D)

[R5_COMPATIBILITY_ADDENDUM.md](evidencedesk/requirements/R5_COMPATIBILITY_ADDENDUM.md) is **superseded** as a current
requirement and kept as historical evidence, together with `board_unadapted_failure.txt`. It was approved by the
implementing orchestrator, not the Owner. It was written against HowlBoard `cd2f2fd`; current HowlBoard declares its own
root grants and needs no adapter, and applying the adapter to it is a compile error (a second `(grants ...)` form).

Consequence: `evidencedesk/tests/verify_workflows.py` applied the adapter unconditionally, so its real-consumer step passed
only against `cd2f2fd`.

**Update 2026-10-23:** the script now applies the adapter only when the consumer declares no root grants. A HowlBoard that
declares its own is built unmodified and run with exactly the authority it declares (as HowlBoard's own Makefile runs it); that
declaration is pinned in the script, so a Board asking for more fails instead of being granted it, and the Board process gets no
host environment beyond `PATH`. `scripts/test.sh` passes against current HowlBoard `0b9236b` and against `cd2f2fd`.
