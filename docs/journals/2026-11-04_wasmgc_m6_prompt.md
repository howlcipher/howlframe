# HowlFrame WasmGC M6 handoff — 2026-11-04

## Objective and authority

Remove the remaining authored HTML/CSS application source from HowlScene HF by generating styles and the editor page shell from .howl. Enforce a policy allowing only .howl application source under src/. Preserve generated platform glue and the existing security/capability boundaries. Do not delete HowlFrame's JavaScript backend.

M5 remains complete with owner-accepted exceptions. M6 is now also complete
with owner-accepted exceptions; see the [closeout](2026-11-08_wasmgc_m6_trial.md).
Consumer PR #4 is merged. The owner accepted Chromium as the routine gate,
deferred M6 Firefox acceptance and approved the final local Chromium reserve.
Private hosted CI was skipped under the no-payment publishing route; no hosted
success or deployment is claimed. Original M5 [acceptance](2026-11-04_wasmgc_m5_acceptance.md)
and [integrated closeout](2026-11-07_m5_integrated_closeout.md) remain historical.

## Starting revisions and dependencies

- Merged compiler repair: `db1cbc85e2b58662a053e5d2b68df508684982ea` ([PR #303](https://github.com/howlcipher/howlframe/pull/303)); public Build, Acceptance, Race and Latest Go checks passed.
- Merged consumer: `fe626c8606bde6d500facb1b41d413196eb198d7` ([PR #3](https://github.com/howlcipher/howlscene-hf/pull/3)), pinned to that compiler. Verify remote heads before acting.
- Earlier `cac48ccdad09489598123734be3c28815565146d` compiler and `68384f9257e42a437035cd9039e4593db1a9528b` measurement candidates remain historical evidence, not the starting revisions.
- No billable workflow, deployment or release publishing is authorized.

## M5 evidence and limitations

Final local source policy, build, provenance, core 19/19, SDK 22/22, runner infrastructure 31/31, Chromium 81/81 and Firefox 81/81 passed. Firefox used four sequential fresh bounded scopes (27 + 22 + 16 + 16), with zero skips, retries or expected failures. See the integrated record for exact revisions and evidence hashes. The earlier uninterrupted Firefox suite exhausted its task limit; final shards did not. Private hosted CI was intentionally skipped, and WebKit remains unverified. No deployment is claimed.

## Original source-purity gaps, now closed

The original maintained src/editor/index.html, src/editor/editor.css and src/site/site.css are replaced by generated presentation from src/presentation/*.howl. Only regular UTF-8 .howl files are admitted under src/. Read scripts/assemble_editor.py, scripts/build.sh, scripts/check_source_policy.sh, provenance checks and current .howl UI/template/rendering sources. Generated output must remain distinguishable from maintained source. Do not inspect the separate howlcipher/howlscene repository or modify howlcipher/william_elias.

## Prioritized dependency

Follow [AI-DX1 → M6 → AI-DX2 → AI-DX3](../design/ai_dx_roadmap.md). The roadmap defines the proposed JSON asset manifest architecture, alternatives and measurement protocol. M6 is integrated; AI-DX2 is next in that order and is not implemented here.

## Original focused slices, completed

1. Inventory shell/style inputs and output contracts. Establish byte/DOM/CSP equivalence tests and an explicit generation design; compare small .howl-based generation options before selecting an architecture.
2. Generate site/editor styles from .howl while preserving CSS behavior, accessibility, motion and size budgets. Add focused equivalence and browser tests.
3. Generate the editor shell from .howl. Preserve metadata, restrictive CSP, loader ordering, keyboard behavior and source provenance.
4. Tighten the source policy to reject non-.howl maintained application source under src/. Update build/provenance/docs and remove obsolete maintained shell/style inputs only after equivalence passes.
5. Run final compiler and consumer gates sequentially; review focused PRs and merge only under applicable repository policy and no-payment authorization. Record exact commits and CI conclusions.

## Safety and validation

Before heavy validation inspect memory, PSI, swap, scratch space, active workloads and ports. Retain at least 8 GiB scratch. The owner separately approved the final local Chromium gate on 2026-11-09 with 9 GiB launch minimum and 6 GiB runtime host reserve, unchanged 3/5 GiB scope caps, zero scope swap, 150% CPU, 128 tasks, one worker and pressure/event/final-counter checks. Other heavy checks retain the original 10 GiB host floor. Optional Firefox validation requires 13 GiB available before launch (10 GiB host floor plus the 3 GiB workload high watermark); its runtime guards are unchanged. Verify the entire process tree: MemoryHigh 3 GiB, MemoryMax 5 GiB, MemorySwapMax 0, CPUQuota 150%, TasksMax 256 for Firefox (128 for focused Go where sufficient), one browser worker, GOMAXPROCS=2, Go package parallelism 1. Preserve essential Go flags. One heavy command at a time; no parallel agents or browser engines. Do not touch desktop Firefox or unrelated processes. Stop on resource denial/unsafe pressure; retain true exit codes, process ownership, task/memory peaks and logs. Do not raise ceilings automatically or weaken sandboxing/tests. Reuse successful M5 evidence unless changes justify reruns.

## Exit criteria and journal

All application source under src/ is .howl; generated shell/styles have provenance; application behavior, accessibility, security, exports and performance retain executable coverage. Required final-tree local gates and actual CI pass on merged commits unless the owner explicitly accepts a separately documented exception. Start docs/journals/<current-date>_wasmgc_m6_journal.md with exact checkout SHAs, dependencies, scope/peak measurements, commands/exit codes, tests and PR state. Update at every coherent slice and before ending a session. Commit safe durable work and leave exact next steps; temporary worktree paths are not a substitute for journaled commit IDs.

## Accepted routine gate revision, 2026-11-09

The owner accepted Chromium as the required routine browser gate and deferred
M6 Firefox acceptance. Preserve Firefox as an on-demand compatibility track,
including every assertion, inventory check and resource guard. Require fresh
passing Firefox evidence before claiming Firefox support for a release. No paid
or unverified private hosted execution is authorized. This supersedes the
original M6 full-Firefox requirement above; final routine gates still apply.
