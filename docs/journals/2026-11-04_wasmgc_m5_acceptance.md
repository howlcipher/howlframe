# M5 acceptance record — 2026-11-04

## Owner disposition

M5 status: COMPLETE WITH OWNER-ACCEPTED EXCEPTIONS.

The owner instructed: “do all the stuff that doesn't require payment then let's call m5 complete”. This explicitly accepts closing the milestone on the available local evidence while deferring potentially billable hosted CI and the merges that require it. This is a milestone disposition, not a claim that the original CI-and-merge exit contract passed.

## Verified local evidence

- Previously recorded production M5 evidence: Chromium 81/81, core 19/19, SDK 22/22, provenance and HowlFrame regression checks passed. These suites were not repeated solely for closure.
- Final Firefox 157.0 / Playwright 1.64.1 WasmGC candidate: 81/81 passed, zero skips, four sequential shards of 27, 22, 16 and 16 unique cases, all exit 0. The beginner journey passed.
- Pinned pre-migration consumer source a6715f480e8b170742b03e7a0b8b97f69e8ce011 compiled with the candidate JavaScript backend: TXT-002 and exported-text A11Y-007 passed, 2/2. Both also passed on the WasmGC candidate.
- TXT-002 needed the shared browser-host rich-text selection fix; A11Y-007 needed accurate inline fragment/clipping measurement. The final candidate has no expected-failure annotations for these cases. The prior baseline failures remain historical evidence, superseded by the candidate runs.
- Tested HowlFrame source change: cac48ccdad09489598123734be3c28815565146d; branch fix/firefox-richtext-selection. Focused rich-text Go regressions passed. Current journal-only follow-up before closure: 0c6bc9f3ab224249254c6e1a4d0f66da2877f139.
- Tested consumer test change: 68384f9257e42a437035cd9039e4593db1a9528b; branch fix/m5-firefox-baseline. Its evidence follow-up before closure: da1fa19e4c728a9fa39961702bd71fce98464243.
- Each final Firefox shard used a verified scope: MemoryHigh 3 GiB, MemoryMax 5 GiB, MemorySwapMax 0, CPUQuota 150%, TasksMax 256, one worker. Peak tasks 243; peak memory 1,116,987,392 bytes; zero cgroup task-denial or memory-limit/OOM events. No overlapping browser invocations or changes to desktop Firefox.
- The uninterrupted-suite task-cap exhaustion remains a separate infrastructure finding. Aggregate sequential shard evidence supplies complete local case coverage; it does not establish an uninterrupted run.
- Exact 81-case inventory and resource measurements: 2026-11-04_wasmgc_m5_firefox_coverage.md. Older ledgers and journal entries retain their historical outcomes and are superseded for current acceptance by this record.

## Deferred integration and checks

- Merged M5 production consumer commit: ea1eeeadb724feb6d4013c9b05733cf9f1a25bfd. Merged HowlFrame M5 compiler commit: e2eb0ba84b499670e2576d1658ff8add86420c65; subsequent main documentation revision: 41ee7b80c6a5eb6b08941f9415a88156b9d0875f.
- Follow-up rich-text fix and consumer measurement correction are locally committed, unpushed and unmerged. The final Firefox success applies to those candidates, not to the unchanged production compiler pin.
- HowlScene HF PR #3 remains open at remote head 07a8ad04a673ad1d4a9389b30d10ccc3cff6010b. Its recorded verify job passed; firefox failed at the memory preflight before browser tests. No new hosted CI was run, and no green CI conclusion or merge is claimed.
- The proposed 8 GiB hosted-runner profile remains unexecuted. Actions allowance cannot be established with the current token (billing endpoint returns 404; token lacks user scope). The owner's no-payment instruction forbids triggering potentially billable runs.
- Required HowlFrame CI jobs for the new rich-text branch have not run. WebKit compatibility remains unverified due the previously documented environment gap.

## Integration sequence when a no-charge route is established

1. Publish/review the HowlFrame rich-text fix, run its required Build, Acceptance, Race and Latest Go checks, and merge only after legitimate gates pass.
2. Update the consumer compiler pin to that merged commit; integrate the tested consumer measurement correction and remove PR #3's obsolete expected-failure annotations. Do not merely relabel the old production output as the tested candidate.
3. Run necessary final-tree checks sequentially within verified resource scopes. Publish PR #3 changes only when hosted execution is confirmed free or separately authorized. Require actual CI conclusions before merging.
4. Reconcile PR #3's description and milestone status with this accepted-exception record. M6 planning is permitted by the owner's disposition; M6 implementation must account for these unmerged dependencies first.

## 2026-11-05 closeout reconciliation

Live remote heads remain compiler main `41ee7b80c6a5eb6b08941f9415a88156b9d0875f`, consumer main `ea1eeeadb724feb6d4013c9b05733cf9f1a25bfd`, and private PR #3 `07a8ad04a673ad1d4a9389b30d10ccc3cff6010b` (verify SUCCESS, firefox FAILURE). The compiler fix has no published PR yet.

The earlier payment uncertainty does not apply to HowlFrame's standard public CI: live GitHub repository visibility is PUBLIC, `.github/workflows/ci.yml` uses standard `ubuntu-latest` runners, and [GitHub's billing policy](https://docs.github.com/en/actions/concepts/billing-and-usage) confirms standard public-repository runner usage is free. Publishing the compiler fix is authorized after local validation; require independent review and Build, Acceptance, Race and Latest Go success before merge. Private consumer cost uncertainty remains unchanged. This corrects the previous blanket deferral of hosted CI; retain the original historical record above.

New selection-state and runner changes are unverified candidates. They do not inherit the historical 81/81 results. See `2026-11-05_m5_closeout.md` for current work and remaining gates and [the proposed roadmap](../design/ai_dx_roadmap.md) for subsequent milestones.
