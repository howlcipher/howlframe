# HowlFrame WasmGC M6 handoff — 2026-11-04

## Objective and authority

Remove the remaining authored HTML/CSS application source from HowlScene HF by generating styles and the editor page shell from .howl. Enforce a policy allowing only .howl application source under src/. Preserve generated platform glue and the existing security/capability boundaries. Do not delete HowlFrame's JavaScript backend.

M5 was closed by explicit owner disposition using local evidence with deferred hosted CI and merges. Read `2026-11-04_wasmgc_m5_acceptance.md` before starting. Do not claim the original M5 CI-and-merge contract passed. This document plans M6; no M6 implementation has begun.

## Starting revisions and dependencies

- Merged consumer production: ea1eeeadb724feb6d4013c9b05733cf9f1a25bfd.
- Merged compiler implementation: e2eb0ba84b499670e2576d1658ff8add86420c65; later main documentation: 41ee7b80c6a5eb6b08941f9415a88156b9d0875f. Verify remote heads before acting.
- Local compiler fix: cac48ccdad09489598123734be3c28815565146d on fix/firefox-richtext-selection, worktree /tmp/howlframe-firefox-richtext.
- Local consumer measurement correction: 68384f9257e42a437035cd9039e4593db1a9528b on fix/m5-firefox-baseline, worktree /tmp/howlscene-hf-firefox-fixes.
- PR #3 https://github.com/howlcipher/howlscene-hf/pull/3 has unpushed follow-up evidence on test/m5-firefox-bounded in /tmp/howlscene-hf-m5-firefox. Its hosted Firefox check is red. Resolve the dependency sequence from the M5 acceptance record before building M6 upon those candidate fixes. No billable workflow, deployment or release publishing is authorized.

## M5 evidence and limitations

Chromium 81/81; core 19/19; SDK 22/22; provenance and recorded compiler gates passed. Final Firefox candidate 81/81 in sequential shards, including the beginner journey; paired JavaScript reference TXT-002/A11Y-007 2/2 passed with the candidate compiler and corrected measurement. See the exact case ledger and acceptance record. The previous uninterrupted Firefox suite exhausted its task limit; final shards did not. Hosted CI on follow-up changes remains unexecuted; WebKit launch/compatibility remains unverified. Preserve these gaps honestly.

## Source-purity gaps

Remaining maintained application source includes src/editor/index.html, src/editor/editor.css and src/site/site.css. Read scripts/assemble_editor.py, scripts/build.sh, scripts/check_source_policy.sh, provenance checks and current .howl UI/template/rendering sources. Generated output must remain distinguishable from maintained source. Do not inspect the separate howlcipher/howlscene repository or modify howlcipher/william_elias.

## Prioritized dependency

Follow [AI-DX1 → M6 → AI-DX2 → AI-DX3](../design/ai_dx_roadmap.md). The roadmap defines the proposed JSON asset manifest architecture, alternatives and measurement protocol. This handoff does not authorize implementing subsequent milestones during M5 closeout.

## Suggested focused slices

1. Inventory shell/style inputs and output contracts. Establish byte/DOM/CSP equivalence tests and an explicit generation design; compare small .howl-based generation options before selecting an architecture.
2. Generate site/editor styles from .howl while preserving CSS behavior, accessibility, motion and size budgets. Add focused equivalence and browser tests.
3. Generate the editor shell from .howl. Preserve metadata, restrictive CSP, loader ordering, keyboard behavior and source provenance.
4. Tighten the source policy to reject non-.howl maintained application source under src/. Update build/provenance/docs and remove obsolete maintained shell/style inputs only after equivalence passes.
5. Run final compiler and consumer gates sequentially; review focused PRs and merge only under applicable repository policy and no-payment authorization. Record exact commits and CI conclusions.

## Safety and validation

Before heavy validation inspect memory, PSI, swap, scratch space, active workloads and ports. Require at least 10 GiB MemAvailable and 8 GiB scratch for the approved local profile. Verify the entire process tree: MemoryHigh 3 GiB, MemoryMax 5 GiB, MemorySwapMax 0, CPUQuota 150%, TasksMax 256 for Firefox (128 for focused Go where sufficient), one browser worker, GOMAXPROCS=2, Go package parallelism 1. Preserve essential Go flags. One heavy command at a time; no parallel agents or browser engines. Do not touch desktop Firefox or unrelated processes. Stop on resource denial/unsafe pressure; retain true exit codes, process ownership, task/memory peaks and logs. Do not raise ceilings automatically or weaken sandboxing/tests. Reuse successful M5 evidence unless changes justify reruns.

## Exit criteria and journal

All application source under src/ is .howl; generated shell/styles have provenance; application behavior, accessibility, security, exports and performance retain executable coverage. Required final-tree local gates and actual CI pass on merged commits unless the owner explicitly accepts a separately documented exception. Start docs/journals/<current-date>_wasmgc_m6_journal.md with exact checkout SHAs, dependencies, scope/peak measurements, commands/exit codes, tests and PR state. Update at every coherent slice and before ending a session. Commit safe durable work and leave exact next steps; temporary worktree paths are not a substitute for journaled commit IDs.
