# Milestone 3 evidence statement (correction), 2026-10-21

Status: **dated correction. It does not rewrite, supersede or re-judge** `MILESTONE3_DECISION.md`,
`evidencedesk/INDEPENDENT_REVIEW.md`, `evidencedesk/requirements/R5_COMPATIBILITY_ADDENDUM.md` or
`evidencedesk/evidence/board_unadapted_failure.txt`. Those records stay as written. This statement
adds what a fresh check against the current upstream HowlBoard shows, and says what is still undecided.

Basis: independent audit of 2026-10-21 (finding HF-06) and the remediation re-run recorded in
`docs/remediation/evidence/hf06_board_current_vs_historical.log`. Compiler under test: this branch
(`fix/post-audit-stabilization`). HowlBoard upstream `main` at `f4c7bb544773a717d2bd0640e91ff9f154bdd9b6`
(2026-10-20T23:50:18-04:00), cloned fresh. No HowlBoard file, store or repository was modified.

## 1. Historical compatibility (as originally recorded) — reproduced

The original Milestone 3 integration test compiled the sibling checkout
`cd2f2fdb7126cca922604483724beb008621a60a` (2026-10-01). Its `backend/server.howl` declares no
route-level grants. Run unmodified with the scoped grants the test uses, its routes fail with
`CAPABILITY_DENIED` on `STORE_OPEN` (HTTP 500), exactly as `board_unadapted_failure.txt` records.
Reproduced again on this branch (source SHA-256 `4d5233b5…89d9`).
The recorded adapter (one inserted `(grants "database" "inherit:filesystem")` line, in a temporary copy)
is what let that revision pass. That record is accurate **for that revision**.

## 2. Compatibility with the current upstream — differs from the record

| Check against upstream `f4c7bb5` (source SHA-256 `664ee147…567f`) | Result |
|---|---|
| Unmodified source, run with the grants its own source declares (`network,database,filesystem,environment,network:fetch=raw.githubusercontent.com,network:fetch=github.com`), reading the projector output of the 3-record fixture | **Passes**: 3/3 timeline events, states and provenance as required |
| Same, 72-record sanitized fixture (`data/fixtures/ledger_sample.jsonl`) | **Passes**: 72/72 events |
| Unmodified source under the narrow scoped grants used by the Milestone 3 test (`network,database,filesystem:read=<dir>,filesystem:write=<dir>`) | **Does not start**: current upstream needs the grants it declares, including coarse `filesystem` |
| Is the capability adapter still required against current upstream? | **No.** Upstream declared its route grants on 2026-10-13 (`bda6746`, "align Board workflows with scoped Frame runtime"). |

Source of the discrepancy: the sibling checkout used for Milestone 3 was `cd2f2fdb`, an ancestor of upstream
`main` that is **12 commits behind** and predates `bda6746` (an ancestor of `f4c7bb5`). The "original unadapted
failure" is a property of that stale checkout, not of current HowlBoard. The Milestone 3 record's phrase
"existing HowlBoard consumer requires a capability adaptation" was therefore not true of the then-current upstream
(`bda6746` landed seven days before the work).
One real difference remains: unmodified upstream runs with a **broader** grant set (coarse `filesystem`) than the
adapted test used. Against upstream, "compatible" means "compatible under Board's own declared, broader grants".

## 3. Integration or deployment — none

Checked in the fresh upstream clone: zero references to EvidenceDesk, its projector, or `milestone3/` anywhere in
`f4c7bb5`; no open pull request on `howlcipher/howlboard`; Board's `Makefile` still compiles its own
`tools/ledger_import/ledger_import.howl` (its last change, 2026-10-20 `09a6463`, fixed the default ledger path).
**The projector has not replaced, been adopted by, or been deployed into HowlBoard.** What exists is a tested,
compatible, standalone alternative and a repeatable consumption test. Calling that "adoption" or a "replacement"
of the ecosystem component would be inaccurate. Directive D30 B asks for "a useful implementation or replacement of a
genuine Howl ecosystem component"; whether a compatible but unadopted alternative satisfies it is an Owner judgment (section 5).

## 4. Owner-approved changes — what the repository actually shows

* The R5 addendum (post-freeze requirement change: a capability adapter is permitted, original failed run retained) states
  it was "Approved by the parent agent within the user's all-milestone implementation scope". The parent agent is the
  implementing orchestrator, and the broad scope it cites is a general instruction to rebuild all three milestones.
  **No approval of that requirement change by the Owner, specifically, is recorded.** Directive section 8 asks for a
  "separately approved requirements revision" for any change to frozen requirements.
* The frozen `ACCEPTANCE.md` step 4 says no sibling modification is needed. The addendum changed that for an environment
  reason that, per section 2 above, did not apply to the current upstream.
* Pull request #208, which carried all of this, was merged on 2026-10-21. A merge is not recorded as a decision on the
  addendum, and this statement does not treat it as one. **This correction does not retroactively approve the addendum.**

## 5. Owner decisions still pending (none is made here)

1. **Verdict wording for D30 B.** Options: (a) keep "ACCEPTED" with this statement attached; (b) reclassify D30 B as
   "compatibility demonstrated, adoption not demonstrated", the audit's reading; (c) hold D30 B open until upstream
   HowlBoard uses the projector. The audit and this remediation recommend (b) or (c) unless integration is wanted.
2. **The R5 addendum.** Approve, reject, or supersede it. If kept, record who approved it and on what basis. Since
   current upstream no longer needs the adapter, it can also be retired.
3. **Integration.** Whether to upstream the projector into HowlBoard (a separate task in that repository, not done
   here and not started), or to keep it a standalone tool.
4. **Test baseline.** Whether the integration test should track upstream `main` (recommended) or stay pinned to a recorded commit.

## 6. Not touched

The separate receipt/grant-audit verifier specification (`M3_ACCEPTANCE_SPEC.md`, PR #206) is unchanged and remains
a draft with no implementation. No HowlBoard production code or data was read or modified.
