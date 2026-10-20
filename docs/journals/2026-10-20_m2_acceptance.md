# Milestone 2 accepted by the Owner (B6 venue waiver), 2026-10-20

## Decision

At 11:24 AM EDT on 2026-10-20, William (Owner) answered in the Grok Bot chat with Rintaro: "Waive the B6 venue
requirement and accept M2". He had said he could not run the B6 kill storm on his PC because he is not at home, and
asked: "Can we skip this or work around?".

* **W-B6-venue** waives the B6 gate for M2 only, at code pin `7e9ef32e`, without an isolated-venue run. B6 has no
  ACCEPTED verdict: both runs stay INFRA-VOID.
* The planned PC run is superseded and will not be done for M2. Whether #202's run counts as the isolation rerun is
  now moot.
* **Milestone 2 (ticketdesk) is ACCEPTED by the Owner** under waivers W-m2, W-m3, W-m4, W-m12, W-m17 (granted
  11:02 EDT) and W-B6-venue.

## Evidence it rests on

* B6: Round 7 (#201, `e41b6830`; evidence `milestone2/ticketdesk/evidence/round7_2026-10-19/`) and the isolation rerun
  (#202, `883f26de`; evidence `milestone2/ticketdesk/evidence/round7_isolation_2026-10-20/`) each passed 225/225 runs
  at `7e9ef32e`, 450/450 in total, with 0 failures and no `CLASSIFY` line. Both were INFRA-VOID only because of noise
  breaches; cause not proven (VM-fork and balloon kmsg records, stall- and freeze-type heartbeat gaps). No B6
  behavioural failure was observed.
* Gates A, C, D, G, H: Lain's final independent assurance review (#203, `dd8f2728`) passes A, C, D and G under the
  five Owner waivers and H unconditionally. The m10 regression tests are manual-only until a workflow-scoped change
  adds the CI step.

## Residual risk and reopen triggers

* Isolation was never demonstrated, and wall-time behaviour under the noise gaps is unverified: on wall time, three
  gap-overlapping requests would have exceeded 10 s and the verdicts would have been NOT ACCEPTED (infra).
* B6 reopens if a B6-type product failure is seen at or after `7e9ef32e`, if an isolated run is ever done and fails,
  or if the code, script or pin changes beyond what the 450/450 evidence covers. Full terms:
  `milestone2/MILESTONE2_DECISION.md`, top section.

## Outcome

`milestone2/MILESTONE2_DECISION.md` records the acceptance and the waiver at the top. The PC-run notes in that file,
`milestone2/ticketdesk/REVIEW.md` and `docs/journals/2026-10-20_ticketdesk_b6_round7_isolation.md` are marked
superseded; Lain's review is covered by the pointer in the decision file. No product code, script, pin or frozen files
changed. The M2 loop stops here; M3 planning is next.
