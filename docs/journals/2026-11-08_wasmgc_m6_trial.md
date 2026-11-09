# WasmGC M6 application trial closeout

M6 is **complete with owner-accepted exceptions**. The consumer trial is merged;
M5's recorded disposition and historical candidates remain unchanged. No release,
deployment, new Firefox compatibility claim or efficiency gain is established.

## Revisions and integration

| Revision | Role |
| --- | --- |
| `fe626c8606bde6d500facb1b41d413196eb198d7` | Consumer M5 baseline |
| `db1cbc85e2b58662a053e5d2b68df508684982ea` | Unchanged compiler pin |
| `99823a7e8daf13d9b872dfe3c556b8df24f29840` | Signed M6 implementation and final test evidence |
| `9b8c53e45df91026c3888cc37a4e2875073b0e75` | Signed journal-only follow-up / PR head |
| `e1a2136c9f73e62d117eba5ee23a97a605d3c08f` | Merged consumer [PR #4](https://github.com/howlcipher/howlscene-hf/pull/4) |

GitHub verified implementation, evidence and merge signatures. The merge tree
exactly matches the PR head. All 137 source/test/configuration/CI/policy files
were frozen through final tests; only evidence-journal entries changed afterward.
The later AI-DX1 compiler merge `77ed3f88d9e8a2fb20a1b2611cbbca07908087fd` was not
needed for generation. No compiler source, pin or backend changes were required.

## Architecture and limits

Four `.howl` modules generate the editor shell and editor/site styles. A pure
WasmGC Node program emits a bounded versioned JSON manifest; the adapter validates
all three allowlisted assets before creating a fresh output directory. Existing
assembly, CSP and loader ordering remain intact. Only regular UTF-8 `.howl`
application source is admitted under `src/`; symlinks, special files, unreadable
trees and raw dotted interop are refused. Provenance verifies every metadata
field and output against a rebuild, including generated presentation hashes.

This reuses existing compiler features and preserves shipped bytes, but retains
string-based HTML/CSS and escaping. A structured presentation language offers
composition at substantially greater compiler scope. The Node generator is
trusted build input, not a sandbox; the private temporary parent remains part of
the filesystem trust boundary. No additional runtime payload is shipped.

## Final routine verification and test impact

| Gate | Result |
| --- | --- |
| Focused generation/policy/provenance | 9/9 |
| Lint, source policy, build and provenance | Passed |
| Shipped payloads | All 11 byte-identical to baseline, excluding provenance |
| Core / SDK | 19/19 and 22/22 |
| Firefox infrastructure | 33/33, zero skips |
| Chromium | 81/81, zero skips/retries, 2.6 minutes |
| Full M6 Firefox | Deferred; no acceptance pass claimed |

The complete Chromium run used a fresh owner-approved scope: MemoryHigh 3 GiB,
MemoryMax 5 GiB, zero swap, CPUQuota 150%, TasksMax 128 and one worker. The host
launch minimum was 9 GiB and runtime reserve 6 GiB, explicitly approved after
the inherited 10 GiB floor blocked matched Chromium and Firefox launches. Kernel
memory peak was 866,791,424 bytes, task peak 100, minimum host available memory
10,390,708,224 bytes. Scope swap, resource events, sampled memory PSI maxima and
host swap-out delta were zero; final cleanup and exit 0 were verified.

The final non-browser pipeline used the original 10 GiB host floor and sampled
256,909,312 bytes / 39 tasks with zero events. Actual new WasmGC compilation and
execution prove the generator; unchanged compiler sources/pin reuse prior gates.

Existing UI/export/accessibility/CSP/motion contracts remain intended. The guide
control-name test now reads the generated shell. New tests protect malformed
manifest paths/schema/encoding, shell authority, source-policy edges, stale
provenance, real generation and frozen bytes, and resource admission boundaries.
No assertion was removed or weakened. Two sequential independent reviews found
duplicate attributes, nested `srcdoc` and SVG script URLs; all were reproduced
and fixed through rejection-before-write regressions. Final reviews found no
remaining blocker, including the resource monitor's exception cleanup, final
counter enforcement and asynchronous-exit grace handling.

## Owner-accepted exceptions and preserved failures

On 2026-11-09 the owner accepted Chromium-first routine validation and explicit
M6 Firefox deferral, then separately approved the local Chromium reserve above.
Automatic Firefox CI and its transfer artifact are removed; the complete bounded
suite remains on demand with unchanged assertions, strict inventory, zero skips
and all resource safeguards. Require fresh passing evidence before claiming
Firefox release compatibility. Its optional 13 GiB launch / 10 GiB runtime
profile is unchanged. Safari/WebKit remains unverified.

Private hosted CI was skipped to preserve no-payment execution, using the
existing local-validation publishing route with `[skip ci]` on head and merge.
Actions API checks found zero runs for both. Billing API access could not verify
a no-payment private hosted route with current credentials; no scope refresh,
upgrade, hosted dispatch or deployment was attempted. Skipped CI is not passed CI.

Earlier Firefox attempts stopped on global swap-out (156 and 481 pages), and a
later application attempt stopped below the host floor. Matched Chromium also
hit that floor; successful blank-page Firefox rendering did not establish full
application acceptance. One approved-profile Chromium attempt stopped after
49 passes on host PSI full 1.28. Its cleanup check raced asynchronous exit;
bounded grace/reaping fixed that issue, a delayed-SIGTERM probe passed, and the
retry completed with PSI zero. No pressure threshold or safety ceiling changed.
All failures, ownership/resource samples and actual exits remain retained.

## Evidence and next item

The consumer M6 journal records exact commands, candidate hashes and outcomes.
Full logs, baseline output, original asset text, snapshots and the reviewed
monitor remain under the local external `m6-evidence` directory. Evidence SHA-256:

* `final-candidate-inputs.json`: `d6a2c1dce743d0408226e6797ae9a71b38f0cc97a927fd8e6cc6ee54b6e0dd2e`
* `final-policy-pipeline.log`: `8e48a47642bf865d3d608d9d6051949db39f64a2110fa18c6e7c64b8e6906e79`
* `final-chromium-retry.log`: `13365bf5ee8f0b22f3edf7d42f97609d504fde8a7d820b46788fb64d880add30`
* `chromium_bounded.py`: `fb1a3ee9d40074bff012f4ad5cb02bcc9ddcd5217c62182a52c9fc0b8524bb86`
* `chromium-resource-profile-approved.json`: `7bec7dfce0b69398cdada98a69776cab3a9769f296bff242865c67b2c98556ce`

Follow the [AI-DX roadmap](../design/ai_dx_roadmap.md): AI-DX2 is next, with a
local versioned evidence manifest and conservative invalidation before session
interfaces. AI-DX2/3 and broader AI-DX1 target/signature coverage remain future
work. The preregistered measurement protocol remains in force; no live-agent
comparison or claimed efficiency improvement is part of M6.
