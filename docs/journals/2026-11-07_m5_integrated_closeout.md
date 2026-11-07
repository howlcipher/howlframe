# M5 integrated closeout: 2026-11-07

M5 remains **complete with owner-accepted exceptions**. Compiler and consumer follow-ups are merged. This supersedes the deferred-integration state in the earlier acceptance and handoff records; historical candidate results remain attached to their original revisions.

## Integrated revisions and authority

- Compiler [PR #303](https://github.com/howlcipher/howlframe/pull/303): merged `db1cbc85e2b58662a053e5d2b68df508684982ea`; reviewed head `b5b99f9a11ccbc86efe2a25169ea5d11821bd4e5`. Both have tree `5f2df9b82801264d656b08fe6d39860e1c48697b`.
- Consumer [PR #3](https://github.com/howlcipher/howlscene-hf/pull/3): merged `fe626c8606bde6d500facb1b41d413196eb198d7`; final signed head `d21d0bc86396794db8a85ad6ef93a78696d1d68a`, implementation `46ad92514fee739d7d6c140da7b5b0053fd8da6f`. Head and merge share tree `0cdd689ecef59a92766eb7461f2e209b525122e8`. The consumer pins the merged compiler above.
- The owner explicitly approved merging the consumer using local validation and skipping private hosted CI. Both head and merge carry `[skip ci]`; Actions reported zero runs for both revisions. The old red hosted check was not reclassified as passing. No paid execution, deployment or release was performed.

## Verification and test impact

The compiler repair preserves toolbar selection while clearing stale selection on explicit caret intent, keyboard/focus transitions and content replacement; independent editors remain isolated. Focused regressions and independent review passed. The unchanged-source full WasmGC gate passed on both toolchains: Go 1.26, 4,548 passes and 31 design skips; Go 1.27, 4,546 passes and 33 design skips; consumer gate 8 passes, zero skips. Earlier host-memory stops and two VCS build failures are retained, not erased. Public [candidate CI](https://github.com/howlcipher/howlframe/actions/runs/42235237482) and [merged-main CI](https://github.com/howlcipher/howlframe/actions/runs/42236093141) passed Build, Acceptance, Race and Latest Go.

Consumer source policy, build and provenance (all 11 outputs), core 19/19, SDK 22/22 and runner infrastructure 31/31 passed. Final Chromium passed 81/81 after removing optional root scroll snapping that interfered with keyboard anchor navigation; URL, active chapter and viewport assertions remain enforced. Firefox passed 81/81 in four fresh sequential shards (27, 22, 16, 16), with complete inventory, actual passes and zero skips, retries or expected failures. Obsolete failure annotations and the exactly-two-failures contract were removed. Candidate hashing, report coverage, controller and resource-policy tests protect runner failure paths; the resource policy allows recorded host swap-ins while requiring zero scope swap and no host swap-out.

Firefox used MemoryHigh 3 GiB, MemoryMax 5 GiB, zero scope swap, CPUQuota 150%, TasksMax 256 and one worker. Task peaks were 243, 243, 246 and 244; memory peaks were 1,474,711,552, 1,083,580,416, 1,085,009,920 and 1,152,716,800 bytes. Resource event counts were zero. All shards used candidate SHA-256 `38898aabb061a555c8a22c4ad1fa9bcdb1de8ce032ac366f647c769d11ebd8a5`. Saved source hashes confirm tested application, tests, configuration and toolchain inputs did not change during signing or subsequent documentation updates.

The smallest appropriate tiers were focused compiler/runner regressions, subsystem tests, the prescribed compiler gate and full consumer browser acceptance. Existing behavioral contracts remain intended; no additional application behavior changed during this documentation reconciliation. Final documentation validation checks local links, SEO and whitespace; public CI supplies the integration gate. Browser evidence does not establish deployed-site state, uninterrupted Firefox execution or WebKit support.

Durable local logs, candidate/source manifests, resource samples and earlier failures are retained under `/run/media/howlcipher/tallgeese/dev/worktrees/m5-closeout-evidence.z5tKQmAE`, including `fullgate-traced.log`, `consumer-final-c.log`, `chromium-final-c.log`, `firefox-final-c.log`, `consumer-tested-source.json` and `howlscene-firefox-shards.HBQGP9Fq/summary.json`.

## Next work

Follow the proposed [AI-DX1 → M6 → AI-DX2 → AI-DX3 roadmap](../design/ai_dx_roadmap.md) and [M6 handoff](2026-11-04_wasmgc_m6_prompt.md). Architecture tradeoffs, acceptance designs and matched-task measurement protocol are documented there. No AI-DX capability or efficiency gain is claimed as implemented; live-agent comparisons remain deferred until a no-payment route is available.
