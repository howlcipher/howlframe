# WasmGC M5 execution journal

## Objective and current state

Implement M5: run HowlScene HF's editor and engine SDK through HowlFrame WasmGC, preserve browser behavior and source purity, remove HowlScene HF's production JavaScript backend dependency, pass required local gates and CI, merge focused reviewed PRs, and write the M6 handoff.

- Started from HowlFrame remote `main` at `ee25181a3f3f534e25009631b1379bb98d473d03` (fetched 2026-11-01).
- HowlScene HF remote `main` at `a6715f480e8b170742b03e7a0b8b97f69e8ce011` (fetched 2026-11-01).
- HowlFrame task branch: `feat/wasmgc-m5-editor` in `/tmp/wasmgc-m5`.
- HowlScene HF worktree is clean; no M5 branch/worktree existed at inspection.
- This branch is a discovery checkpoint only; no implementation changes yet.

## DX reconciliation

- HowlPlane at `/run/media/howlcipher/tallgeese/dev/howlplane` is clean on `main` at `414569a`. Its recent commits are unrelated to M5 DX.
- HowlFrame `origin/main` is `ee25181`; its post-M4 additions are the merged Go test-name fix (#299) and bug tracker closure (#300), neither changes compiler diagnostics, capability metadata, or CLI discovery.
- Live WasmGC diagnostics still use `WASM_UNSUPPORTED` and source positions; typed host operations are in the existing capability registry. No newer structured diagnostic or discovery interface was found in the current repository history.
- The expected HowlPlane path under `/run/media/system/...` is absent; the alternate current checkout above exists. Continue looking for owner-reported local-only DX work before relying on any interface.
- Loaded HowlPlane `howlframe-transpiler`, `howlframe-app-development`, `environment_doctor`, `test_and_verify`, and `anti_manipulation` guidance. Repository `.agents/skills` and `.agents/rules` directories are absent in the HowlFrame checkout.

## Checkpoints and evidence

### 2026-11-01 — initial live audit

Commands and exit codes:

- HowlFrame `git fetch origin main --prune`, `git rev-parse origin/main`: exit 0; remote head `ee25181...`.
- HowlScene HF `git fetch origin main --prune`, `git rev-parse origin/main`: exit 0; remote head `a6715f4...`.
- Both repositories' status: clean; HowlFrame's primary `main` checkout was two commits behind and was left untouched.
- Worktree inventories show no active M5 worktree. Existing unrelated worktrees/branches were left untouched.
- Source inspection: HowlScene HF `SITE_TARGET=wasmgc` only covers the site runtime; editor and SDK builds still invoke `--target js`.
- Baseline builds, browser tests, compiler tests, and gate have not run yet.

Resource preflight (2026-11-01):

- `/proc/meminfo`: `MemAvailable` 15,790,604 kB; swap total 12,258,812 kB; swap free 10,314,112 kB.
- Memory and I/O PSI avg10/avg60/avg300 all 0.00.
- Load average 0.07 / 0.12 / 0.17.
- Free disk: `/tmp` 9.6 GiB; HowlFrame filesystem 193 GiB.
- Existing Firefox processes and three long-lived Python HTTP servers were observed. Their elapsed times were 2 days / 1 day and no active Playwright, Go-test, compiler, or gate command was present. These processes are not owned by this task and were not touched.
- cgroup v2 is mounted and user systemd is active. Effective scope limits and child membership still need explicit verification.
- No validation workload has been launched by this task; no concurrent test runs.

## PR slices

| Slice | Repository/branch | PR | State | Evidence |
|---|---|---|---|---|
| Discovery and editor baseline | HowlFrame `feat/wasmgc-m5-editor` | — | In progress | No builds/tests yet |
| Compiler/backend gaps | HowlFrame, one branch/PR per defect | — | Not started | — |
| Editor integration | HowlScene HF, separate branch/PR | — | Not started | — |
| Engine SDK packaging | HowlScene HF, focused branch/PR | — | Not started | — |
| Backend removal and final verification | HowlScene HF, focused branch/PR | — | Not started | — |
| M6 handoff | HowlFrame journal | — | Not started | — |

## Open blockers and remaining work

1. Locate/confirm any owner-reported DX changes that are local-only and not represented in current HowlFrame/HowlPlane heads.
2. Verify the cgroup-v2/systemd scope and resource controls against a representative child process tree.
3. Run the sequential JavaScript and WasmGC editor baseline builds, then isolated browser smoke comparison; record the gap matrix before implementation.
4. Execute M5 slices and required gates in sequence; do not expand into M6.

## Exact next steps for another Luna session

1. Read this journal and `docs/journals/2026-11-01_wasmgc_m5_prompt.md`.
2. Confirm branches/worktrees and process/resource state are still safe.
3. Verify systemd scope limits and process-tree membership.
4. Inspect HowlScene HF's pinned toolchain and run the two baseline builds sequentially, with outputs/logs under `/tmp`.
5. Complete the editor probe and gap matrix before changing compiler code.

### 2026-11-01 — scope verification and editor baseline probe

Branch/checkpoint: HowlFrame `feat/wasmgc-m5-editor`, commit `e8dae7cc` before this journal update; source code remains unchanged. Compiler used for baseline: HowlScene HF's pinned HowlFrame compiler at `d210aa49035cf3953c06fd8ec4da093644169e21`.

Verified containment:

- Scope probe initially used incompatible `systemd-run --wait --scope`; exit 1 with `--wait may not be combined with --scope`.
- A follow-up probe used an awk expression expanded by the child shell; exit 1 (`$1: unbound variable`).
- A subsequent cgroup path lookup through `/sys/fs/cgroup` did not resolve because the Distrobox cgroup namespace reports `/../../../app.slice/...`; this was a path-resolution mistake, not absent limits.
- Correct probe command used `systemd-run --user --scope --unit=howlframe-m5-cgroup-probe.scope -p MemoryHigh=3G -p MemoryMax=5G -p MemorySwapMax=0 -p CPUQuota=150% -p TasksMax=128 -- env GOMAXPROCS=2 GOMEMLIMIT=2GiB GOFLAGS=-p=1 bash -c ...`; exit 0. Parent and `sleep` child both reported `0::/../../../app.slice/howlframe-m5-cgroup-probe.scope`. `systemctl --user show` reported ControlGroup `/user.slice/user-1000.slice/user@1000.service/app.slice/howlframe-m5-cgroup-probe.scope`, MemoryHigh 3,221,225,472, EffectiveMemoryHigh 3,221,225,472, MemoryMax 5,368,709,120, EffectiveMemoryMax 5,368,709,120, MemorySwapMax 0, CPUQuotaPerSecUSec 1.5s, TasksMax 128. Effective process-tree limit is verified.

Builds and fixtures:

- Pinned JS editor build command (`howlframe build .../src/editor/editor.howl -o /tmp/howlframe-m5/js-editor --target js`), scoped with the verified properties and `GOMAXPROCS=2`, `GOMEMLIMIT=2GiB`, `GOFLAGS=-p=1`: exit 0; app.js 237,182 bytes.
- Pinned WasmGC editor build (`--target wasmgc --inline-module`, output `/tmp/howlframe-m5/wasm-editor`), same limits: exit 0; app.js 390,080 bytes. No unsupported construct was reported.
- Site runtime JS build and HTML assembly into `/tmp/howlframe-m5/{js-editor,wasm-editor}`: exit 0; the same site JS/CSS and page shell were used for both. The first fixture-prep invocation accidentally invoked Hugging Face's `hf` CLI due to an unquoted shell variable; exit 1 with `No such command 'build'`. Corrected invocation used `"$hf" build` and succeeded. This was a command construction error, not a compiler/test defect.
- Focused custom Chromium smoke script `/tmp/howlframe-m5/browser-smoke.mjs`, with one Chromium browser, two sequential fresh contexts (closed after each), and the verified resource scope: exit 0. Both targets reached `body[data-ready=true]`, displayed 6 sections, handled Get started and Templates events, opened the template dialog, selected Personal portfolio, rendered preview heading `Your Name`, and had zero page/console errors.
- Single-run startup timing (navigation start to `data-ready`): JS 239 ms; WasmGC 149.7 ms. This is exploratory, un-repeated timing only.
- app.js raw/gzip: JS 237,182 / 43,195 bytes; WasmGC 390,080 / 102,308 bytes. WasmGC is larger by 58,885 gzip bytes (~2.37x); optimization is not yet justified without complete functional parity and workload measurements.
- Chromium scope maximum observed from systemd `MemoryCurrent`/`MemoryPeak`: 247,275,520 bytes (~236 MiB). No cgroup event/kill, PSI pressure, or timeout observed. The machine's existing Firefox session and unrelated long-lived local HTTP servers remained untouched; no test suite or other browser engine was run concurrently by this task.

Initial gap matrix (not exhaustive; only Task 0 smoke evidence):

| Construct/behavior | Minimal probe | JS reference | WasmGC result | Capability | Follow-up test |
|---|---|---|---|---|---|
| `web_app` compile and inline Wasm loader | `src/editor/editor.howl` build | Builds | Builds; no unsupported diagnostic | Browser startup | full-page init plus compiler gate |
| Initial state/rendering | isolated page load | 6 sections, ready, preview | Same observed behavior | DOM, storage init, rendering | existing editor spec |
| Browser event handlers and template selection | Get started → Templates → Personal portfolio | Opens and preview shows `Your Name` | Same observed behavior | DOM/event dispatch; rendering | editor/journey specs |
| Async storage, images, backups, export, recovery/error handling | not yet exercised | M4 JavaScript baseline | unverified | storage/file/image/ZIP host operations | backup, editor, export specs and beginner journey |
| Large text/performance | not yet exercised | not measured | unverified | string/list building and WasmGC runtime | existing performance spec plus bounded samples |

Commands/corrections affecting evidence:

- An attempted `cat .../howlframe --help` invoked `cat`, not the compiler; exit 0 but produced no compiler evidence. This was excluded from build evidence.
- `scripts/assemble_editor.py` was run using the same source HTML, CSS and JS site runtime for both targets; generated artifacts are only under `/tmp/howlframe-m5`.
- All compiler/build and browser workloads were sequential and scoped. No source edits outside this journal have occurred.

## Updated remaining work

1. Create an isolated HowlScene HF worktree based on `a6715f4`; add the temporary `EDITOR_TARGET` switch without changing production default.
2. Rebuild WasmGC editor/site variants and run focused existing browser specs (editor, backup, journey) with `--workers=1`; diagnose behavior differences before compiler modifications.
3. Broaden to the complete Chromium suite only after focused tests pass; use a single scope and record every gate result.
4. Inspect every editor construct against the live WasmGC compiler and capability registry, add a concrete minimal reproducer for each failure, and only then open a HowlFrame compiler slice if warranted.

### 2026-11-01 — focused WasmGC/JS browser comparison

HowlScene HF branch `feat/wasmgc-m5-editor` is isolated at base `a6715f480e8b170742b03e7a0b8b97f69e8ce011`; temporary only change so far is `scripts/build.sh`'s `EDITOR_TARGET` switch. Its output directories and `node_modules` link are under `/tmp`/temporary worktree; nothing changed on HF `main`.

- Full HowlScene build with `EDITOR_TARGET=wasmgc` (site runtime and SDK remain JS) using the cached pinned compiler: exit 0, output `/tmp/howlscene-hf-m5/dist-wasm-editor`.
- Playwright focused run on WasmGC editor: `playwright test tests/e2e/editor.spec.mjs tests/e2e/backup.spec.mjs tests/e2e/journey.spec.mjs --workers=1 --project=main --output=/tmp/howlframe-m5/focused-wasm-results --reporter=line`; exit 0, **28 passed** in 49.0 sec. It covers backup recovery/rejection/import, output exports, image upload and invalid images, storage failures, keyboard/accessibility, preview/theme/section behavior, SEC-004, and JRN-01..20.
- Same focused command on JS editor reference: exit 0, **28 passed** in 45.2 sec. Both executions use the exact same source specs and the same site JS runtime. No observed behavior divergence in these checks.
- Scope peaks: WasmGC 718,450,688 bytes; JS 688,451,584 bytes. Both were below MemoryHigh; MemoryMax/OOM/OOM-kill counters were not raised and no timeout/process kill occurred.
- Playwright/Node emitted the environment warning `NO_COLOR env is ignored due to FORCE_COLOR being set` on both runs. This is an environment variable conflict, not an application/test assertion; unset the conflicting variable for later gates and document if it persists.
- `dist` in the scratch worktree is only a symlink to the `/tmp` build output so existing specs can consume it. Generated files stay outside source directories.

Current confirmed gap matrix update: the focused real browser suite now covers the previously untested storage, image, backup/import, security rejection, export, async host operations exercised by those user journeys, and preview operations. All 28 passed for both backends. Remaining unknowns are tests/spec paths outside these three files, performance, other browser engines, independent SDK consumer behavior under WasmGC, and full final-tree gates.

### 2026-11-01 — complete Chromium suite and performance investigation

- Full HowlScene HF suite on both `SITE_TARGET=wasmgc` and `EDITOR_TARGET=wasmgc`, one Chromium worker, verified cgroup limits, generated outputs in `/tmp`: **79 passed, 1 failed**, 2.6 minutes, exit 1. All editor, backup, security, storage, export, image, accessibility, pages, motion/story, and JRN-01..20 functional tests passed. The only failure was `performance.spec.mjs:79` (“scroll-linked motion adds little frame time over a still page”). At this point the suite's 250 ms systemd memory sampler was active; cgroup peak was 861,413,376 bytes, and memory PSI stayed 0.00.
- Failure evidence with sampler: still median 16.7 ms; best moving median 66.6 ms (moving samples 83.3, 66.6, 83.3); threshold 29.05 ms. This was classified as a performance regression candidate, not a pass.
- Ran the six-test performance project by itself, with one worker, no memory sampler, under the same cgroup limits. The first isolated attempt (WasmGC) had 4 passed/2 failed, exit 1: scroll performance failed (33.3 ms best moving median; samples 66.6, 33.3, 83.2), and the afterAll artifact writer failed because the temporary `test-results` symlink target directory had not been created. The latter is a test-output setup error; no source/test assertion was changed. The target directory was created outside the worktree before the reference comparison.
- Ran the same performance project on the JS reference build, same 150% CPU quota, same Chromium, no sampler: **5 passed, 1 failed**, exit 1. The only assertion failure was the same scroll metric: still 16.7 ms, best moving median 50.0 ms (samples 83.3, 50.0, 66.6). Cgroup peak 515,764,224 bytes. WasmGC isolated cgroup peak was 512,446,464 bytes.
- M4's recorded local reference for JS and WasmGC sites was 16.7 ms median for both, so current timing differs from that baseline on both implementations. The task branch changes only the editor build flag; this scroll test runs the example site's runtime and is independent of the editor. The failure is therefore not attributable to the editor target based on current evidence. Root cause of the local timing change remains unresolved (likely environment/scheduling/rendering; not asserted as fact). Do not alter or skip this test. Required local performance gate remains red/unverified pending explanation or a clean run; CI evidence will be needed as well.
- The first isolated-performance afterAll output error was fixed only in scratch paths (`/tmp/howlframe-m5/test-results`); no HowlScene HF test file changed.

Failure classifications:

- `No such command 'hf build'`, systemd `--wait` misuse, awk expansion, and dangling test-results target: task command/setup errors, corrected without code changes.
- Motion frame threshold: baseline-shared timing failure on both JS and WasmGC; product/environment classification unresolved. No resource limit was hit; no process was killed; memory and PSI stayed safe.

## Updated remaining work

1. Investigate why the existing motion benchmark fails on both reference and WasmGC under the required scope; preserve the existing assertion and M4 historical result. Recheck machine/browser/runtime versions and compare the actual generated site runtimes, then continue only with a scoped benchmark.
2. Keep the HowlScene integration branch changes isolated while running required source-policy, provenance, core/SDK, and browser validations; do not claim the browser gate green while motion remains red.
3. Inspect live WasmGC module/export facilities for an SDK Node ES-module package; create a minimal consumer reproducer and test before designing any adapter.
4. Continue M5 PR slices, final local gates, GitHub CI, and M6 handoff. M5 remains incomplete.

### 2026-11-01 — WasmGC Node SDK module slice

Repositories and revisions:

- HowlFrame remote `main` re-fetched as `ee25181a3f3f534e25009631b1379bb98d473d03` (includes PRs #299 and #300). SDK compiler work is isolated in `/tmp/howlframe-m5-sdk-module`, branch `feat/wasmgc-sdk-module`, based on that revision; changes are not committed yet.
- HowlScene HF branch/worktree `/tmp/howlscene-hf-m5`, `feat/wasmgc-m5-editor`, base `a6715f480e8b170742b03e7a0b8b97f69e8ce011`; SDK build/package integration changes are uncommitted. `dist`, `node_modules`, and `test-results` are scratch symlinks only.
- This journal remains on `/tmp/wasmgc-m5`, branch `feat/wasmgc-m5-editor`, last committed source checkpoint `eb61a81b`; journal needs another commit after this entry.

DX recheck: latest compiler interfaces are unchanged from the earlier audit. No owner-reported local-only DX interface was surfaced. This slice uses the current `--target wasmgc --module` interface added here; it does not change diagnostics or capability discovery.

Implementation and behavior:

- Added a backend-neutral `internal/moduleapi` for exported top-level functions and TypeScript declarations. The WasmGC compiler path no longer imports JavaScript code-generation helpers.
- Added `wasmgc.CompileModule` for exported `web_app` functions. It emits Wasm exports and refuses browser host operations. The Node ESM adapter initializes the module, serializes exported calls against the current single-fiber instance ABI, converts JS values to/from HowlFrame values, and returns Promises.
- `build --target wasmgc --module` emits `app.wasm`, `app.mjs`, and `app.d.mts`. `--host`, `--tests`, and `--inline-module` cannot be combined with this module mode.
- Added a minimal external Node consumer regression with numeric and async calls, list/dict/Unicode round trips, TypeScript declaration checks, and a rejected conversion error. Added a capability refusal regression.
- HowlScene HF's scratch `scripts/build.sh` now has a temporary `SDK_TARGET` switch; the package manifest includes `app.wasm`, and the packed-consumer assertion accepts the artifact when present.

Regression sensitivity:

- Before compiler edits, the new test run against original `ee25181` failed with `--module requires --target js`; command `GOFLAGS=-p=1 go test ./internal/backend/wasmgc -run '^TestModuleExportsAsNodeESM$' -count=1 -parallel=1`; exit 1. No generated package was produced. This is the original implementation failure, not a later regression.
- The first implementation test found a missing compiler import; fixed without changing test expectation. A later fixture edit initially omitted the `web_app` closing parenthesis; fixed as a test-fixture defect.
- After implementation, `go test ./internal/backend/wasmgc -run '^(TestModuleExportsAsNodeESM|TestModuleRejectsBrowserHostOperations)$' -count=1 -parallel=1`: exit 0.
- Focused relevant regression command across root, WasmGC, JavaScript, and moduleapi packages, including JS module parity and browser async/fail-closed coverage: exit 0. Output recorded at `/tmp/howlframe-m5/sdk-shared-tests.log`.
- Actual HowlScene `src/sdk/sdk.howl` compiled with the new WasmGC module path; `wasm-tools validate --features all` passed. A separate Node consumer initialized the registry, parsed `portfolio.json`, rendered HTML, and received structured errors for invalid JSON.
- HowlScene SDK suite against the packed WasmGC package: **22/22 passed**, including clean npm packing/install, strict TypeScript compilation, and all core SDK behaviors. Output `/tmp/howlframe-m5/howlscene-sdk-tests.log`.
- One first suite invocation exited 9 before tests because this Node build rejects `--test-concurrency` inside `NODE_OPTIONS`; the rerun supplied `--test-concurrency=1` directly to `node --test` and passed. This was runner configuration, not a test/product failure.

Resource safety for the SDK work:

- Before compiler/backend test workloads: MemAvailable ranged 15.65–15.73 GiB; `/tmp` free 9.4 GiB; swap free 10.31 GiB; memory PSI avg10/avg60/avg300 stayed 0.00; CPU load stayed below 1.0.
- Verified active `systemd-run --user --scope` child membership and effective limits: MemoryHigh 3 GiB, MemoryMax 5 GiB, MemorySwapMax 0, CPUQuota 150%, TasksMax 128. Go workloads used `GOMAXPROCS=2`, `GOMEMLIMIT=2GiB`, `GOFLAGS=-p=1`, `-parallel=1`; Node suite ran one test worker. Scope peak was not retained by systemd after units stopped, so no peak is claimed for these recent Go/Node tests.
- The initial compiler test failure, fixture failure, and rejected `NODE_OPTIONS` invocation were not resource events. No process was killed or timed out. All workloads initiated by this task were serial.

Current blockers and next steps:

1. Review the WasmGC module ABI and export-name safety; run focused compiler and JavaScript-module tests again after any changes.
2. Run the actual HowlScene SDK suite and clean packed consumer against the new final compiler output (currently verified; rerun after final compiler edits).
3. Decide whether/how to land the HowlFrame compiler slice, then pin the merged revision in HowlScene HF before final integration.
4. The full Chromium editor gate remains **79 passed, 1 failed** on the existing scroll-motion frame-time assertion, and the isolated benchmark fails on both JS and WasmGC baselines. Root cause remains unresolved; do not claim gate success or change the threshold.
5. Continue the editor integration/performance investigation, finish SDK package integration, remove production JS-target paths only after parity, and then execute final gates/CI sequentially.


### 2026-11-01 — SDK compiler PR opened

- HowlFrame SDK compiler slice committed as `0108c4a84f55ad9851bbd0af4daa586642fb8668` on `feat/wasmgc-sdk-module`, pushed to origin, and opened as draft [PR #301](https://github.com/howlcipher/howlframe/pull/301).
- PR scope: WasmGC `--module` output (`app.wasm`, Node ESM adapter, TypeScript declarations), backend-neutral module API helpers, host-operation refusal, and focused regressions. The generated ESM keeps Promise behavior, including `undefined` for `void`; calls are serialized for the current single-fiber instance.
- Staged diff was reviewed; `git diff --cached --check` passed and a credential-pattern scan found no matches. Commit subject follows repository conventions.
- The first GitHub connector PR-creation call returned HTTP 403 (`Resource not accessible by integration`). The already-authorized repository `gh` CLI was authenticated (`repo` scope), so draft PR creation succeeded with `gh pr create`. This is not a GitHub access blocker now.
- Fresh `gh pr view 301` observed head SHA `0108c4a84f55ad9851bbd0af4daa586642fb8668`, PR open/draft, merge state blocked. `build` and `acceptance` were in progress; `race` and `latest-go` queued. No CI job is claimed as passed yet.
- PR description records root cause, minimal reproducer, before/after evidence, Test Impact Assessment, commands, resource controls, and the unrelated local motion benchmark failure.

Updated M5 status: editor implementation is behaviorally at parity on the focused 28-test browser slice and the broader suite has 79 passes with one baseline-shared motion timing failure. SDK WasmGC package has 22/22 tests passing against both the WasmGC implementation and JS reference. No production target migration, merge, full gate, or final CI result yet; M5 remains in progress.

### 2026-11-01 — SDK CI complete; motion investigation resumed

- Current PR #301 head remains `0108c4a84f55ad9851bbd0af4daa586642fb8668`; observed completed GitHub conclusions: Build `SUCCESS`, Acceptance `SUCCESS`, Race `SUCCESS`, Latest Go `SUCCESS`. The PR is still open and marked draft; there is no submitted review. It has not been merged.
- HowlScene HF remains on `a6715f480e8b170742b03e7a0b8b97f69e8ce011` with the temporary SDK/editor target switches, package manifest adjustment, and consumer assertion edits described above. A candidate `src/site/site.howl` chapter lookup optimization is still unverified.
- Safety preflight on resume observed MemAvailable 15.60 GiB, SwapFree 10.31 GiB, `/tmp` free 9.4 GiB, memory PSI avg10/avg60/avg300 0.00, and load 0.51. Existing Firefox desktop processes were active and were not started by this task; no test/build workload was launched while they remained active. No process was killed or suspended.
- Static delimiter count for `src/site/site.howl` is balanced after adding the missing close delimiters in `chapter_contains`/`chapter_at`. `git diff --check` passed. This is only a source-level check; the pinned compiler build has not yet validated the candidate.
- The candidate change checks the currently active chapter before falling back to the full scan. It must still be compiled and compared against chapter/motion behavior, and the scroll performance test must be rerun without changing its assertion. Do not include it as a fix unless it passes those checks.
- Exact next action: wait for the browser slot to be free; recheck system resources and process state; verify the systemd scope cgroup files using the actual scope cgroup path (not `/sys/fs/cgroup` root); compile the candidate site and run the focused chapter/motion and performance checks serially. If the optimization fails semantics or the original performance assertion, revert only this candidate and record the cross-backend failure as unresolved. Then finish HowlScene target migration, required gates, PR review/merge flow, and M6 handoff only after all M5 exit criteria pass.

### 2026-11-01 — bounded chapter lookup build

- HowlScene HF candidate remains based on `a6715f480e8b170742b03e7a0b8b97f69e8ce011`; working tree changes are uncommitted. HowlFrame PR #301 remains open/draft at `0108c4a84f55ad9851bbd0af4daa586642fb8668`; latest live checks were all four `SUCCESS`, with no review submitted and no merge.
- Current M5-specific change in `src/site/site.howl` adds `chapter_contains`/`chapter_at` to check the active chapter and contiguous overlapping successors first, then use the original full scan if the current chapter no longer contains the viewport midpoint. The original full-scan selection of the last matching index is retained in the fallback and contiguous-overlap fast path. This is a performance hypothesis, not verified behavior yet.
- Bounded compile of the candidate with pinned compiler `d210aa49035cf3953c06fd8ec4da093644169e21` and `--target wasmgc --inline-module` ultimately passed and emitted `/tmp/howlframe-m5/site-optimized-wasm/app.js`, exit 0. Earlier compiler diagnostics found and corrected: `while` body arity (3 args instead of condition plus one body), an unclosed `web_app` due to a missing delimiter in the new function, and a `let` with multiple body expressions instead of one `do`. The delimiter-only precheck had missed structural errors. A few earlier scope-controller invocations exited before starting the compiler because their own cgroup-path/CPU-quota assertions or shell quoting were wrong; these were controller errors, not compiler or resource failures. No resource-limit kill occurred.
- Verified live systemd scope immediately before the successful build: task and child cgroup paths shared `/../../../app.slice/<scope>`; `systemctl --user show` for that scope reported MemoryHigh 3 GiB, MemoryMax 5 GiB, MemorySwapMax 0, CPUQuotaPerSecUSec 1.500000s (150%), TasksMax 128. The build was invoked with GOMAXPROCS=2, GOMEMLIMIT=2GiB, GOFLAGS=-p=1, and a 120s timeout. Exact compiler peak memory was not captured.
- Fresh snapshot after the build: MemAvailable 15.59 GiB, SwapFree 10.31 GiB, `/tmp` free 9.4 GiB, memory PSI avg10/avg60/avg300 0.00, load 0.46. Thirteen Firefox processes remained active; they were not started or touched by this task. No browser, Playwright, or Go test workload is active for this task. Thus motion, editor, and full browser checks remain unrun on this latest source edit.
- Next step is still to obtain a free browser slot and then, after a fresh preflight, run the focused chapter/motion and performance checks serially. Keep existing performance assertions unchanged. If this candidate fails behavior or does not address the shared JS/WasmGC motion regression, revert it and retain the baseline-shared failure evidence. M5 is not complete.

### 2026-11-02 — merged SDK compiler and WasmGC-only consumer verification

Repository revisions:

- HowlFrame PR #301 ([merged PR](https://github.com/howlcipher/howlframe/pull/301)) is merged. The live `origin/main` SHA is `e2eb0ba84b499670e2576d1658ff8add86420c65`, merge commit for PR head `cd7bd3f6216332906473a88b7319160bc36ee943`. All four observed CI conclusions were SUCCESS: Build, Acceptance, Race, Latest Go.
- HowlScene HF M5 worktree is `/tmp/howlscene-hf-m5`, branch `feat/wasmgc-m5-editor`, based on `a6715f480e8b170742b03e7a0b8b97f69e8ce011`; final consumer changes are not yet committed or pushed. `dist`, `build`, and `node_modules` are task scratch symlinks, not source changes.
- HowlFrame journal branch/worktree is `/tmp/wasmgc-m5`, branch `feat/wasmgc-m5-editor`, latest journal commit before this checkpoint `9ac141d1c80630b5a41efdb22d89da3b5cf2feba`.

Final merged-compiler consumer build:

- Rebuilt the definitive WasmGC-only production tree at `/tmp/howlframe-m5/howlscene-dist-final-e2eb0ba` from HowlFrame `e2eb0ba...`, with `scripts/build.sh` and no target-switch environment variables; exit 0.
- `scripts/build.sh` now hardcodes WasmGC for site and editor (`--target wasmgc --inline-module`) and SDK (`--target wasmgc --module`). The only remaining production-tree textual `--target js` invocation is `scripts/test_core.sh`, an explicit JavaScript reference backend test build. No production build path contains a JS target.
- Verified compiler provenance by rebuilding all outputs: 11 outputs match HowlFrame `e2eb0ba...`, exit 0.
- `scripts/check_source_policy.sh`: pass, exit 0.
- `scripts/test_core.sh`: 19/19 pass, exit 0 (backup 6, renderer 5, config 8).
- `scripts/test_sdk.sh`: 22/22 pass, including clean packed Node consumer, TypeScript declarations, real Wasm SDK calls and JS reference behavior; exit 0.
- The full Chromium suite against the definitive WasmGC output passed 80/80, including beginner journey JRN-01..20, storage, file/image handling, backups, exports, security, preview, themes/layout, accessibility, and performance. `npx playwright test --workers=1`, exit 0, 2.5 minutes.
- Motion performance output: still median/p95 16.7/16.7ms; moving median 16.7ms, p95 49.9ms; each moving sample 16.7ms. Existing benchmark assertion passed unchanged.

Motion root cause and repair:

- Root cause is the neon `hs-rise` CSS animation moving two radial-gradient backgrounds with `background-position`, causing non-composited repaint work. Disabling only that animation on a same-origin temporary diagnostic CSS copy made the unchanged performance test pass. The prior candidate `.howl` chapter lookup optimization did not address the test and was reverted.
- `src/site/site.css` now separates the two patterns onto pseudo-elements and animates them with compositor-friendly `transform: translateY(...)`, retaining tile distances/speeds and pause/reduced-motion behavior. The original scroll regression test passed on both JS reference and WasmGC, and the full WasmGC suite passed on the final output. This is an M5 runtime performance correction; M6 still owns converting remaining authored CSS and editor-page shell to `.howl`.

Resource safety:

- Before the final build/core/SDK/provenance/browser workloads: MemAvailable 14.5–15.5 GiB, SwapFree 10.3 GiB, `/tmp` free 9.4 GiB, memory PSI avg10/full remained 0.00, load under 1.0. No test/build process or Playwright/Chromium workload was active at the starts of stages. Existing Firefox desktop processes and unrelated local Python servers were inspected and left untouched.
- Scopes verified at runtime had MemoryHigh 3 GiB, MemoryMax 5 GiB, MemorySwapMax 0, CPUQuota 150%, TasksMax 128. Go package builds used `GOFLAGS=-p=1`, `GOMAXPROCS=2`, `GOMEMLIMIT=2GiB`; browser suite had one worker. The final Chromium scope reached `MemoryPeak=919244800` bytes (about 876 MiB), `TasksCurrent=87`; memory PSI stayed 0.00. All task-launched heavy workloads ran sequentially.
- Three final-build controller setup attempts exited 1 before compiler work: incompatible `--pipe`/`--wait` use with a scope and a cgroup path comparison mismatch. A fourth preflight reached a scope but its inline shell variable check was expanded by systemd and stopped before the build. No compiler process ran in these attempts. The controller was moved to a standalone script, then the final build passed within the verified scope. No timeout, OOM/limit kill, or pressure event occurred.

Remaining work and exact next steps:

1. Run `scripts/wasmgc_gate.sh` exactly once from a detached worktree at the live HowlFrame `origin/main` `e2eb0ba...`, with `HOWLSCENE_HF=/tmp/howlscene-hf-m5`, inside verified 3/5 GiB, no-swap, 150%, 128-task systemd scope. It invokes gofmt, vet/build, full Go tests on Go 1.26 and Go 1.27.2, and the M1–M4 HowlScene integration gates. Preserve the gate's real exit code and all skip output.
2. After the gate, final diff review and raw/gzip artifact sizes plus performance report. Finish build docs and source-path scan; ensure current production build contains no `SITE_TARGET`, `EDITOR_TARGET`, `SDK_TARGET` or JS backend invocation.
3. Create the HowlScene HF focused PR, inspect actual CI job conclusions, and merge only after required checks are green. Reverify if merge changes the tested tree.
4. Run final HowlScene HF CI-equivalent build, provenance, core/SDK and full Chromium on the final merged production tree, sequentially if merge altered it.
5. Run required Firefox baseline checks where practical; WebKit remains an environment limitation unless dependencies become available. Record the prior baseline-only A11Y-007/TXT-002 results accurately.
6. Create the M6 handoff only when all M5 exit evidence and merge/CI conditions are met. M5 is still IN PROGRESS.

### 2026-11-02 — prescribed HowlFrame gate passed

- Re-fetched HowlFrame `origin/main`, verified both `git ls-remote` and `git rev-parse origin/main` as `e2eb0ba84b499670e2576d1658ff8add86420c65`, and created detached gate worktree `/tmp/howlframe-m5-gate` at that exact SHA.
- Gate command: `HOWLSCENE_HF=/tmp/howlscene-hf-m5 scripts/wasmgc_gate.sh`. Final successful exit: **0**, `wasmgc gate: passed`.
- Results: gofmt, `go vet ./...`, `go build ./...` passed; Go 1.26.0: 4,534 tests passed, 33 design skips; Go 1.27.2: 4,532 tests passed, 35 design skips; HowlScene HF M1–M4 integration: 8 passed, 0 skipped. No WasmGC tests were skipped for missing tools or consumer checkout.
- The first scoped attempt used an undersized 10-minute wall-clock bound and exited 124 before completing Go 1.26. The second attempt used a 30-minute bound and exited 124 during Go 1.27.2 differential tests. Both were controller timeouts, not test failures, OOMs, or cgroup kills. The final run used a 45-minute bound and passed. All attempts ran the same clean detached HowlFrame `e2eb0ba...` tree and `/tmp/howlscene-hf-m5`; no source edits occurred during a run.
- Final scope verified MemoryHigh 3 GiB, MemoryMax 5 GiB, MemorySwapMax 0, CPUQuota 150%, TasksMax 128; environment GOMAXPROCS=1, GOMEMLIMIT=2GiB, GOFLAGS=-p=1. Peak observed during the passing run: 1,475,153,920 bytes (about 1.38 GiB); maximum observed task count 29. MemAvailable stayed above 15 GiB, memory PSI avg10/full 0.00, and swap did not change materially. No process kill or memory pressure event occurred.

Updated status: HowlFrame PR #301 is merged with all four CI jobs green; the definitive WasmGC-only HowlScene HF build, source policy, provenance, core 19/19, SDK 22/22 and Chromium 80/80 all pass. The required prescribed HowlFrame full gate is now green. Remaining M5 work is final consumer review/commit/PR, actual consumer CI and merge, final measurements, applicable Firefox baseline checks, and M6 handoff after the M5 exit gate passes. M5 remains IN PROGRESS.

### 2026-11-02 — final Chromium and measured editor operations

- HowlFrame remains merged at `e2eb0ba84b499670e2576d1658ff8add86420c65`; HowlScene HF worktree remains `/tmp/howlscene-hf-m5`, branch `feat/wasmgc-m5-editor`, based on live `origin/main` `a6715f480e8b170742b03e7a0b8b97f69e8ce011`. Fresh `git ls-remote origin refs/heads/main` returned that HF main SHA.
- Consumer SDK test was tightened to require `app.wasm` in the packed consumer package; `scripts/test_sdk.sh`: 22/22 passed, exit 0. This confirms the WasmGC binary is actually shipped by the package.
- Re-ran full Chromium suite after the performance measurement and strict package assertion: `npx playwright test --workers=1`: 81/81 passed, exit 0, including JRN-01..20 and the new M5 operation measurement. The final measurements and artifact raw/gzip sizes are recorded in HowlScene HF `docs/benchmark/performance.md`. The added large-text check sends one 945-character `InputEvent` and waits for the preview update, avoiding per-character Playwright injection time. Full measurements are local single-run observations (three samples only for short heading edits), not broad performance guarantees.
- The final Chromium systemd scope enforced MemoryHigh 3 GiB, MemoryMax 5 GiB, MemorySwapMax 0, CPUQuota 150%, TasksMax 128, and one Playwright worker. Observed peak was 919,810,048 bytes (~877 MiB), TasksCurrent peak snapshot 89; memory PSI remained zero, MemAvailable stayed above 15 GiB, and no swap or kill event occurred. No task-launched heavy workloads overlapped.

### 2026-11-02 — Firefox stopped at process-task ceiling

- A temporary config isolated the Firefox project without editing repository configuration; `--list` enumerated all 81 browser tests. Full command `npx playwright test --config=/tmp/howlframe-m5/playwright.firefox.config.mjs --workers=1` ran inside the same verified scope ceilings (including TasksMax 128).
- After a few tests, Firefox emitted `SandboxBroker: thread creation failed (1 brokers): EAGAIN`, followed by content-process failures/SIGSEGV and Playwright page-closed errors. Scope observation showed `TasksCurrent=128` and memory peak 631,447,552 bytes; available memory stayed ample and PSI was not pressured. Classified as environment/process-task exhaustion, not an application assertion or memory OOM.
- Stopped only the task-owned `m5-final-firefox-suite.scope` (result 143); did not touch the owner’s existing desktop Firefox or unrelated processes. No retry and no ceiling increase, per safety requirement. Firefox full suite is UNVERIFIED; it did not reach prior baseline A11Y-007/TXT-002 tests. WebKit remains unverified due unavailable host libraries as recorded in M4. M5 cannot be declared complete while this required browser evidence is unavailable.

### 2026-11-02 — consumer change review checkpoint

- Reviewed the full consumer diff (13 intended tracked files); `git diff --check` passed. Untracked `build`, `dist`, and `node_modules` are task scratch symlinks/artifacts and must not be committed.
- Production `scripts/build.sh` now hardcodes WasmGC for editor/site (`--target wasmgc --inline-module`) and SDK (`--target wasmgc --module`). Removed `SITE_TARGET`/`EDITOR_TARGET`/`SDK_TARGET` switches. Search finds `--target js` only in the explicit JS-reference core test build and explanatory capability docs. CI duplicate selector-based WasmGC build/test steps were removed because the sole default build is now definitive WasmGC output.
- Performance repair is CSS-only: neon radial-gradient layers are separated and transformed to avoid repaint animation. Full Chromium 81/81 and unchanged motion performance assertions passed; no `.howl` app logic or separate repositories were touched.
- At this checkpoint the consumer diff was still uncommitted. The HowlFrame journal branch `/tmp/wasmgc-m5` was at `39bdae8d`. The consumer diff was then reviewed, committed and pushed as recorded in the subsequent checkpoint. Keep the PR unmerged if required Firefox local evidence remains blocked by the task ceiling. Do not raise task limits or retry Firefox. M5 remains IN PROGRESS, so do not create the M6 handoff yet.

### 2026-11-02 — CI exposed onboarding race in new measurement test

- Committed consumer PR slice `932353836f61c57489a3310aa7610f3dc1a8cb94` (`feat(wasm): make WasmGC the production backend`) and pushed branch `feat/wasmgc-m5-editor`; PR [HowlScene HF #2](https://github.com/howlcipher/howlscene-hf/pull/2) is open as draft. PR head/base were observed as `9323538...` / current HF `main` `a6715f4...`. No merge or review is claimed.
- GitHub run `41395580214` (`verify`) completed with Build, Source policy, Provenance, Core tests and Engine/package tests all SUCCESS. Browser tests ran 81 cases: 80 passed and the new M5 performance case failed because the preview heading remained `Your Name` after immediate fill. This is a genuine test sequencing defect; other cases (including EDT-002 heading editing) passed.
- Root cause: the measurement test clicked the onboarding “Get started” button but did not wait for its modal to close before interacting with the editor. The existing editor helper waits for the dialog to become invisible. Added that same observable wait to the performance test; no app logic or test assertion was weakened.
- A focused local regression was selected with a temporary one-project Playwright config to avoid the default `performance` project’s dependency on the entire `main` suite. `--list` verified exactly one test. Scoped command `/tmp/howlscene-hf-m5/node_modules/.bin/playwright test --config=/tmp/howlframe-m5/playwright.focus.config.mjs --workers=1 --grep='M5 records editor startup'`: **1 passed**, exit 0. Verified child cgroup had MemoryHigh 3 GiB, MemoryMax 5 GiB, MemorySwapMax 0, CPUQuota 150%, TasksMax 128; snapshot MemoryCurrent 469,553,152 bytes and TasksCurrent 81. No pressure/kill event.
- One command-selection mistake used `--project=performance` from the checked-in config, whose project explicitly depends on `main`; it unexpectedly started the full 75-test dependency set. Stopped only that task-owned scope after 25 passing tests; command exit 143. No unrelated processes were touched and no failure was hidden. Corrected the selection via the isolated one-project config above.
- Exact next actions: commit/push the dialog-close wait, inspect the new GitHub `verify` result and all steps (browser must be SUCCESS); preserve PR as draft/unmerged until local Firefox requirement is resolved. Firefox remains UNVERIFIED due prior TasksMax exhaustion. M5 remains IN PROGRESS; no M6 handoff yet.

### 2026-11-02 — consumer CI green; M5 held for Firefox safety blocker

- Consumer fix committed and pushed as `3dc71725e97d18b87c3033a928ecd6eda3d9cf69` on `feat/wasmgc-m5-editor`. The branch now contains the definitive WasmGC build migration and onboarding synchronization for the performance test. Worktree `/tmp/howlscene-hf-m5`; base `a6715f480e8b170742b03e7a0b8b97f69e8ce011`; only untracked `build`, `dist`, and `node_modules` scratch links/artifacts remain. They are not included in the branch.
- Final consumer PR: [HowlScene HF #2](https://github.com/howlcipher/howlscene-hf/pull/2), merged by owner instruction at `ea1eeeadb724feb6d4013c9b05733cf9f1a25bfd`, head `3dc71725...`, base `a6715f4...`. GitHub `verify` was SUCCESS before merge. The merge commit’s tree hash was checked against the tested head and both were `c455302c69e8118b8bbe20fc58e5afb28a6885e9`; no code-tree change occurred during squash merge. The PR description was updated to record that Firefox remains unverified and M5 is not complete.
- Final CI run `41396186834`: overall SUCCESS, job `verify` SUCCESS. Actual completed steps: npm ci, Chromium install, source policy, WasmGC production build, provenance, core tests, SDK/package tests, browser tests, cleanup all SUCCESS. Browser log reports `Running 81 tests using 1 worker`; `81 passed (1.7m)`, including the new M5 performance observation. The prior run `41395580214` correctly exposed the modal synchronization defect (80 passed, 1 failed); after adding the dialog-close wait, focused local test passed and the full hosted rerun passed. The failed attempt and correction remain documented rather than suppressed.
- Current browser evidence: local final WasmGC Chromium 81/81, hosted final Chromium 81/81, HowlFrame gate passed at merged `e2eb0ba...`, core 19/19, SDK 22/22, source policy and provenance pass. Performance and artifact sizes are recorded in `docs/benchmark/performance.md` in the consumer PR.
- Firefox remains UNVERIFIED; prior full-suite attempt hit the enforced scope `TasksMax=128` with Firefox `SandboxBroker ... EAGAIN` and content process crashes. It was stopped at the cap, no retry or limit increase occurred. The owner explicitly requested merge despite this gap; PR #2 was merged, but the task still requires applicable Firefox checks. WebKit still cannot launch because host libraries are missing. **M5 status is IN PROGRESS/BLOCKED on safe Firefox verification**, not complete; do not create the M6 prompt until the gate is satisfied or the owner explicitly dispositions this remaining requirement.
- Machine safety note: after changing only the test wait, focused Playwright passed under 3 GiB/5 GiB/no-swap/150%/128-task scope (observed current memory 469,553,152 B and current tasks 81). A project selection with default `performance` dependency started 25 of 75 tests unintentionally; its own scope was stopped, exit 143, with no external process affected. The corrected temporary one-project config listed exactly one target test and passed it. Do not repeat the accidental dependency suite.
- Handoff next step: determine whether Firefox can be verified with a safe policy-compliant method, without raising the cgroup ceiling or touching the owner’s Firefox. If no safe method is available, report the precise blocker and obtain owner disposition of the remaining Firefox requirement. No production release or deployment action was taken; HowlScene HF is merged at `ea1eeea...`, and M6 has not started.
- Final workspace reconciliation: removed only the task-created `build`, `dist`, and `node_modules` symlinks from `/tmp/howlscene-hf-m5` after confirming their targets; the HF branch has no leftover scratch symlinks. Final remote `origin/main` for HowlFrame was rechecked as `e2eb0ba84b499670e2576d1658ff8add86420c65`. Journal worktree `/tmp/wasmgc-m5` is locally committed; its branch remains ahead/behind `origin/main` and was not force-pushed or merged.

### 2026-11-03 — Firefox resource gate remains blocked

Current repository revisions and PR state:

- HowlFrame journal branch `docs/m5-firefox-safety-blocker` is pushed at `dea64377930b38c5cd43f31ad754488ca4d42244`, based on `origin/main` `41ee7b80c6a5eb6b08941f9415a88156b9d0875f`.
- HowlScene HF `main` is merged at `ea1eeeadb724feb6d4013c9b05733cf9f1a25bfd` (tested PR head `3dc71725e97d18b87c3033a928ecd6eda3d9cf69`, identical tree).
- Firefox-validation follow-up is open and unmerged as [HowlScene HF PR #3](https://github.com/howlcipher/howlscene-hf/pull/3), branch `test/m5-firefox-bounded`, latest pushed commit `07a8ad04a673ad1d4a9389b30d10ccc3cff6010b`. Do not merge while its required Firefox check is red or unverified.

Remote validation attempts, in order:

- Run `41568963393` was canceled by this task after discovering the harness expected 80 tests from a stale local checkout. It had already passed source policy, WasmGC build, provenance, core, and SDK steps; Chromium was canceled before completion and the Firefox job never started. This is not a test result.
- Run `41569853523`: `verify` passed, including 81 Chromium cases. The Firefox job stopped before dependency installation/test discovery because Docker exposed `memory.high=max`; no Firefox browser test ran.
- Run `41570598965`: `verify` passed, including 81 Chromium cases. A second Docker cgroup setup step failed before dependencies/tests; the step did not expose which hard-limit/writable-file assertion failed. No Firefox browser test ran.
- Run `41571046108`: `verify` passed, including 81 Chromium cases. `systemd-run` stopped immediately because `--pipe` is incompatible with `--scope`; no validation command ran.
- Run `41571481459`: `verify` passed, including 81 Chromium cases. `systemd-run --scope` successfully created `run-r0eb2f2224d2e4c16b018d22bcca1e51a.scope`. The bounded runner verified cgroup settings in order, then stopped at the host-memory check: `MemAvailable=6.79 GiB`, below the mandatory 10 GiB. Exit 2 occurred before `npm ci`, Firefox installation, test listing, or browser tests. No cgroup limit event or process kill occurred. The child-inheritance check was not reached.

Resource investigation and decision:

- The systemd scope is configured for MemoryHigh 3 GiB, MemoryMax 5 GiB, MemorySwapMax 0, CPUQuota 150%, TasksMax 128. The runner uses one Playwright worker, `GOMAXPROCS=2`, no retries, and aborts on new cgroup limit events or swap use.
- GitHub’s standard private-repository `ubuntu-24.04` runner has 8 GiB RAM; the observed runner exposed 6.79 GiB available. The repository owner is the GitHub user `howlcipher`; the repository API reports zero self-hosted runners. GitHub larger runners require an organization/enterprise configuration. Do not switch to a paid/unavailable runner or create one without authorization.
- Fresh local snapshot: MemAvailable 11,679,232 kB (~11.13 GiB), `/tmp` free 9.0 GiB, SwapFree 8,552,828 kB, memory/IO/CPU PSI avg10 all 0.00, load 0.27/0.43/0.49. However, the host had an active Firefox process tree including a content process with 162 tabs, other Firefox content processes, and active Codex/Claude/agy agents. No local browser or compiler workload was started. These processes were not touched.
- No browser test executed in any failed remote preflight. There were no task-owned process kills, swap events, OOM events, or resource-limit events. The only task-owned remote workload was serial CI verification; no local tests were run concurrently.

Classification: environment/resource-availability blocker, not an application failure. M5 remains **IN PROGRESS/BLOCKED** on required Firefox evidence. Existing M5 evidence remains unchanged: WasmGC production build/provenance, source policy, core 19/19, SDK 22/22, Chromium 81/81, and HowlFrame gates are passing. A11Y-007 and TXT-002 are still only known M4 shared-baseline Firefox differences; the new suite has not reproduced them on the current tree. WebKit remains unavailable as recorded earlier. Do not create the M6 handoff yet.

Exact next steps for another Luna session:

1. Recheck runner availability and host load. Use an already-authorized host with MemAvailable >=10 GiB, >=8 GiB scratch, no active heavy browser/agent workload, and the same verified cgroup-v2 limits; do not raise the ceilings.
2. Resume `test/m5-firefox-bounded` at `07a8ad04a673ad1d4a9389b30d10ccc3cff6010b` or a later review commit. Run PR #3 CI and inspect the actual systemd cgroup values/child membership before tests.
3. Require all 81 discovered Firefox cases to execute exactly once: 79 pass, while only the two documented M4 baseline cases produce expected failures with logged diagnostics. Require zero skips, no other failures, no memory/pids/swap events, and inspect peak memory/tasks.
4. Merge PR #3 only after its complete CI run is green and the above Firefox evidence is verified. Then append final M5 evidence here and create the M6 prompt. Until then leave M5 in progress and M6 unstarted.

### 2026-11-03 — refreshed state; Firefox still unsafe to start

- Refreshed authoritative heads: HowlFrame `origin/main` remains `41ee7b80c6a5eb6b08941f9415a88156b9d0875f`; HowlScene HF `main` remains `ea1eeeadb724feb6d4013c9b05733cf9f1a25bfd`. HowlScene HF PR #2 remains merged at that commit. PR #3 remains open, head `07a8ad04a673ad1d4a9389b30d10ccc3cff6010b`.
- PR #3 latest checks are unchanged: `verify` SUCCESS, `firefox` FAILURE. Run `41571481459` confirms the Firefox job stopped at its 10 GiB host-memory preflight (`MemAvailable=6.79 GiB`); no Firefox test ran. The latest CI still has not provided current-tree Firefox outcomes.
- Local fresh preflight at 2026-11-03 03:28 UTC: `MemAvailable=11,732,348 kB` (~11.19 GiB), `/tmp` free 9.0 GiB, `SwapFree=8,552,932 kB`, load 0.61/0.54/0.53; memory, I/O, and CPU PSI avg10 are all 0.00. These headline figures meet the numeric threshold, but the machine is not idle: an existing Firefox process tree has been running since Oct 30, with multiple large browser content processes (largest observed RSS ~865 MiB), and Codex, Claude, and agy processes are also active. No task-owned browser or test process was started; no existing process was touched. The Firefox validation is therefore still deferred for machine safety.
- No new test or CI run was started in this checkpoint. The previous run evidence and resource decisions remain as recorded above. PR #3 is not merge-ready while its required Firefox job is red and tests are unverified. M5 remains **IN PROGRESS/BLOCKED**; M6 has not started.
- Next step: after the existing browser/agent workload has ended naturally or on an already-authorized idle host, repeat the preflight and run the 81-case Firefox suite under the same verified cgroup ceilings. Require the complete expected outcomes, no skips or unexpected failures, and no cgroup/swap events. Then merge PR #3 only after checks pass, record final evidence, and create the M6 handoff. Do not raise limits, stop other users' processes, or create/pay for new compute to bypass the blocker.

### 2026-11-03 — local Firefox preflight cannot safely run

- Rechecked host immediately after the owner requested the outstanding Firefox step: MemAvailable `11,875,128 kB` (~11.33 GiB), `/tmp` free `9.0 GiB`, SwapFree `8,553,016 kB`, memory/IO/CPU PSI avg10 `0.00`, load `0.42/0.47/0.51`. No Playwright, browser-test, Go-test, or headless-browser process was active. An existing interactive Firefox tree (Firefox plus 14 content processes) and other long-running agent/browser processes were present; these were not stopped or modified.
- Verified a transient user systemd scope can enforce MemoryHigh 3 GiB, MemoryMax 5 GiB, MemorySwapMax 0, CPUQuota 150%, TasksMax 128. During a representative child-process probe, `/proc/<child>/cgroup` and the scope's `cgroup.procs` both confirmed child membership; cgroup event counters remained zero. The scope was then stopped after the probe.
- Inspected PR #3's actual runner. Its preflight rejects if any Firefox/Chrome process is already visible, before dependency installation or test discovery. The local Firefox session therefore makes this host ineligible even though the headline memory and disk thresholds pass.
- Ran only `node scripts/test_firefox_bounded.mjs --preflight-only` inside the verified limits; command exit **1**, before any tests. The local user-scope cgroup is exposed as `/../../../app.slice/<scope>` in `/proc/self/cgroup`, while the script requires a direct `*.scope` path under `/sys/fs/cgroup`; it failed closed with `the process is not in its own systemd scope`. No dependency install, browser install, or test started. The scope itself was inspectable under `/run/host/sys/fs/cgroup`, but changing or bypassing the PR's cgroup validation was not justified.
- PR #3 remains open at `07a8ad04a673ad1d4a9389b30d10ccc3cff6010b`; latest GitHub conclusions remain `verify` SUCCESS and `firefox` FAILURE at the 10 GiB preflight. Existing tests still have no Firefox execution result. M5 remains **IN PROGRESS/BLOCKED**, not complete; the Firefox PR must not be merged with a red/unverified required check.
- Resume condition: use an already-authorized host with at least 10 GiB available, 8 GiB scratch, no visible interactive browser or competing heavy work, and a systemd/cgroup layout the bounded runner can verify. Keep the 3/5 GiB, zero-swap, 150%, 128-task limits. Then run PR #3's exact bounded runner, inspect the 81 actual outcomes and resource counters, and merge only after the full check is green. Do not hide the current Firefox process from the preflight or raise limits.

### 2026-11-03 05:14 UTC — one requested GitHub Firefox retry

- Owner requested one more attempt. Re-ran only the failed Firefox job for HowlScene HF PR #3 / workflow run `41571481459`; the successful `verify` job was not rerun. GitHub started the required systemd scope, then the bounded preflight exited **2** with `BLOCKED: MemAvailable is 6.85 GiB; need at least 10 GiB` (job `125740985888`). This was before `npm ci`, Firefox installation, test discovery, or browser launch. No Firefox tests ran. `verify` remains SUCCESS and `firefox` remains FAILURE.
- Concurrent local snapshot: MemAvailable `11,970,576 kB` (~11.42 GiB), SwapFree `8,555,664 kB`, `/tmp` free `9.0 GiB`, memory/IO/CPU pressure avg10 `0.00`, load below 1. An interactive Firefox process and its content processes remain visible. The PR's local preflight rejects an existing browser, and the prior local preflight-only attempt also failed closed because user-scope cgroup paths do not match `/sys/fs/cgroup` as expected. No local browser tests were started.
- This was the single retry requested; no further retry is planned without a qualifying host or a material environment change. PR #3 remains open and unmerged; M5 remains **IN PROGRESS/BLOCKED**. Merge only after the Firefox suite executes all 81 cases with the documented outcomes and the required checks pass.

### 2026-11-03 — runner options exhausted; awaiting safe host

- GitHub Actions account/repository state: authenticated account is personal user `howlcipher`, `gh org list` returned no organizations, and HowlScene HF has zero self-hosted runners. Official GitHub runner documentation says private-repository standard x64 and ARM Linux runners each provide 8 GiB; ARM does not solve the 10 GiB preflight. Larger hosted runners require an organization/enterprise with a Team or Enterprise Cloud plan. No plan change, payment, external service, or new runner was configured. Sources: https://docs.github.com/en/actions/reference/runners/github-hosted-runners and https://docs.github.com/en/actions/how-tos/manage-runners/larger-runners/manage-larger-runners.
- The one owner-requested GitHub retry is complete: run `41571481459`, Firefox job `125740985888`, preflight exit **2** at `MemAvailable=6.85 GiB`; the test workflow did not install dependencies or launch Firefox. `verify` remained SUCCESS; `firefox` remained FAILURE.
- Current local host has >10 GiB available and >8 GiB scratch with no PSI pressure, but an existing interactive Firefox session with a large multi-process/tab tree remains active. The runner script refuses any visible browser. The user-scoped cgroup can enforce and verify all requested limits and child membership, but this environment's cgroup namespace path is incompatible with the current script; the host systemd manager is unavailable in this container. No local tests started, and no user process was changed.
- No further retries, runner setup, billing changes, or process termination will be attempted absent an environment change. Resume when the owner closes Firefox or supplies an already-authorized safe host; adapt only the cgroup path verification if necessary while retaining the same hard limits and browser-presence check. Then run all 81 cases once and merge only after required outcomes/checks pass. M5 remains **IN PROGRESS/BLOCKED**; M6 remains unstarted.

### 2026-11-03 — preparing a safe local Firefox run

- Following the owner's instruction to run locally after they close Firefox, updated the bounded runner's cgroup lookup on HowlScene HF branch `test/m5-firefox-bounded`. Commit `dc2b197` is local only (branch ahead by one); it accepts the containerized user-scope layout only after resolving the systemd `ControlGroup` and proving the current PID is in that scope's `cgroup.procs`. The direct GitHub `/sys/fs/cgroup` path remains supported. Resource limits and active-browser checks are unchanged.
- `node --check scripts/test_firefox_bounded.mjs` and `git diff --check` passed. A `--preflight-only` invocation inside the exact 3/5 GiB, zero-swap, 150%, 128-task scope reached the active-process check and exited **2** with `BLOCKED: an existing browser process is active in the Firefox validation scope`. This confirms the cgroup path now resolves; no dependencies were installed and no browser tests ran.
- Latest host snapshot still shows the owner's interactive Firefox process tree. It was not touched. The bounded suite remains unstarted until the process exits and the full preflight passes. Afterward, avoid `playwright install --with-deps` on this Bazzite host because it can mutate system packages; use the already-installed system libraries and Playwright's browser-only install, stopping if dependencies are missing. M5 remains **IN PROGRESS/BLOCKED**.

### 2026-11-03 — Firefox closed; two pre-test blockers

- The owner confirmed Firefox was fully closed. Fresh host preflight showed `MemAvailable=16,072,184 kB` (~15.33 GiB), `SwapFree=9,013,796 kB`, `/tmp` free 9.0 GiB, no memory pressure, and no Firefox process. The exact user systemd scope limits were verified through the runner: `memory.high=3 GiB`, `memory.max=5 GiB`, `memory.swap.max=0`, `cpu.max=150%`, `pids.max=128`, current process and representative child in the scope. Preflight-only exited **0**; no tests started.
- First bounded sequence: `npm ci` and `npx playwright install firefox` succeeded (no `--with-deps`; Playwright removed obsolete cached Chromium versions). The runner then exited **2** before browser launch because its test-list parser required a `[project]` prefix, while the installed single-project Playwright output omits it. This is a runner defect, not a Firefox test result.
- The Playwright `--list` diagnostic exited **0** and listed all 81 tests in the current unprefixed format. `node --check scripts/test_firefox_bounded.mjs && git diff --check` exited **0** after the consumer runner parser was corrected to support both prefixed and unprefixed forms. Consumer branch `test/m5-firefox-bounded` now has local commit `7dbe3c42`; this includes the parser correction and a HowlScene HF journal. It is two commits ahead of its remote and is not pushed.
- Second bounded runner attempt exited **2** during the required three-second safety observation: `BLOCKED: host swap activity detected during preflight`. It stopped before test discovery or browser launch. Current `pswpin=254271`, `pswpout=1330993`; no task-owned browser/test process was started or killed. M5 remains **IN PROGRESS/BLOCKED**. Do not immediately retry; require a new stable preflight and no swap-counter movement. Keep the existing limits.
- Exact successful preflight values from the second scope: `MemAvailable=16,445,734,912` bytes, scratch free `9,637,150,720` bytes, cgroup current memory 19,836,928 bytes, cgroup tasks 7. The subsequent swap movement supersedes that snapshot as a go/no-go result.
- Next step: on a later turn, inspect current machine safety and branch/PR state. If safety remains stable, run the corrected bounded Firefox suite once under the same verified scope; otherwise stop. Do not merge HowlScene HF PR #3 while its `firefox` job is red or unverified. Do not create the M6 handoff until every M5 exit requirement is met.

### 2026-11-03 — bounded Firefox run stopped by browser crash

- Fresh systemd-scope preflight passed: `MemAvailable=16,439,349,248` bytes, `/tmp` free `9,577,463,808` bytes, stable swap observation, no memory pressure, and verified 3 GiB/5 GiB/zero-swap/150%/128-task controls. The corrected runner discovered all 81 tests across 66 isolated source locations.
- It stopped on the first test (`backup.spec.mjs:79`) with `Target page, context or browser has been closed` (wrapper exit **2**, underlying Playwright invocation exit **1**). The remaining 80 tests did not execute.
- After another successful preflight, the first test alone reproduced in 1.1 seconds with `browserContext.newPage: Target crashed` (Playwright exit **1**). Systemd recorded Firefox and sandbox child core dumps; stack traces include `libLLVM.so.21.1` and Firefox `libxul.so`. Scope peaks were 460.8 MiB and 488.8 MiB, far below the 5 GiB cap. No OOM evidence or residual task browser processes were found. `coredumpctl` is unavailable in the container, but the user journal has systemd-coredump stack traces. Classify as a Firefox/toolchain environment crash with cause not established, not a product test pass/failure.
- No further browser suite or focused test retry this turn. Next investigate Firefox 1555 and host/container library compatibility, then select a safe compatible environment or a minimal justified diagnostic. Do not weaken coverage or limits. PR #3 remains unmerged; M5 is **IN PROGRESS/BLOCKED**, and M6 remains unstarted.
- Read-only diagnostics: the consumer container reports Ubuntu 26.04; Playwright Firefox is Nightly 157.0, build `20261002045259`; host `libllvm21` is `1:21.1.8-6ubuntu1`. `ldd` reported no missing direct Firefox binary dependencies. Node is `v22.22.1`; npm `12.0.2` warns it does not support that exact Node version. `file` and `coredumpctl` are unavailable. This narrows the investigation but does not prove a compatibility cause; no packages or global toolchain were changed.
- Workaround investigation: Mozilla source docs describe Firefox's default three preallocated content processes and `dom.ipc.processPrelaunch.fission.number`; Playwright supports per-run `firefoxUserPrefs`. With official graphics-disable prefs in a temporary `/tmp` config, the isolated backup test still crashed. A direct Playwright smoke test without the app launched Firefox but failed creating its first page. A correctly configured second smoke setting preallocated process count to zero failed the same way with `SandboxBroker: thread creation failed (4 brokers): EAGAIN`. No sandbox setting was changed. Host-wide limits were not exhausted (`ulimit -u=94765`, `threads-max=189530`, `pid_max=4194304`, ~1,502 host threads); the scope limit was 128 tasks. The scope's `pids.events` were not retained, so cgroup task exhaustion is plausible but unconfirmed. Do not raise `TasksMax`, disable sandboxing, or retry browser workloads until a safe diagnostic/compatible host is available.
- Correction to the prelaunch diagnosis: Playwright's installed `firefox/playwright.cfg` already sets `dom.ipc.processPrelaunch.enabled=false` and `dom.ipc.processCount=60000` so pages from distinct contexts use separate processes. Thus setting `dom.ipc.processPrelaunch.fission.number=0` was redundant; lowering Playwright's `processCount` would reduce its intended context process isolation and is not an acceptable workaround. Host CPU affinity is 0-11 (12 CPUs). Candidate safe diagnostic: run `/tmp/firefox-pids-probe.mjs` under `taskset -c 0,1` inside the unchanged systemd scope, sampling `pids.current`, `pids.peak`, and `pids.events`. Fresh preflight for that diagnostic exited **2** because host swap counters moved; no test started. Wait for a stable swap observation before any further browser workload.

### 2026-11-03 — Firefox task-cap cause confirmed

- Consumer worktree `/tmp/howlscene-hf-m5-firefox`, branch `test/m5-firefox-bounded`, revision `3b3f2a22a65261a642f282c9f4581a4c1b5c59f3` before this journal-only checkpoint. Fresh bounded preflight exited **0**: MemAvailable `16,474,877,952` bytes, scratch free `9,575,632,896` bytes, no memory/CPU pressure, load `1.03/0.51/0.38`, and verified scope limits of MemoryHigh 3 GiB, MemoryMax 5 GiB, MemorySwapMax 0, CPUQuota 150%, TasksMax 128. Swap stability check passed.
- A single minimal Firefox process-count diagnostic, no app loaded, ran with `taskset -c 0,1` under those same limits. Scope `run-p1777607-i10172047.scope`; exit **1**. Firefox launch succeeded; first `browserContext.newPage` failed `Target crashed`. `pids.peak=128`, `pids.max=128`, and `pids.events:max` rose from 0 to **8**. Memory peak was `293,416,960` bytes, with zero memory high/max/OOM events. This demonstrates that Firefox's task/thread creation is denied at the enforced cgroup task ceiling; limiting CPU affinity to two cores did not resolve it.
- Stop browser workloads. Do not raise TasksMax, reduce Playwright's configured per-context process isolation (`dom.ipc.processCount=60000`), or disable the Firefox sandbox. No task-owned or unrelated process was killed; no tests ran concurrently. M5 remains **IN PROGRESS/BLOCKED** on Firefox evidence; HowlScene HF PR #3 remains open/unmerged and M6 has not started.
- Exact commands: `node scripts/test_firefox_bounded.mjs --preflight-only` in the bounded scope exited 0 with no test started; `taskset -c 0,1 env GOMAXPROCS=2 RUNNER_TEMP=/tmp node /tmp/firefox-pids-probe.mjs` exited 1 as above. This was a one-page Playwright smoke, not the Firefox suite.
- Resume only with an explicitly approved environment where the exact Firefox build and isolation model fit within the approved task cap, or after a separately reviewed safety-policy change. No compliant workaround is currently demonstrated on this host. Keep PR #3 open and do not create the M6 handoff.

### 2026-11-03 — Bazzite host / Ubuntu Distrobox context

- Owner clarified that development runs on Bazzite with an Ubuntu Distrobox. Read-only live inspection confirmed `CONTAINER_ID=devbox`, Ubuntu `26.04 LTS`, host-shared kernel `7.2.11-ogc4.1.fc44.x86_64`, cgroup v2, and visible namespace-root values `pids.max=max`, `memory.max=max`, and `cpu.max=max 100000`. No outer Distrobox pids cap was visible. The earlier Firefox scope explicitly set `TasksMax=128` and reached `pids.peak=128`; the confirmed denied task creation was therefore at our own scope, not an observed outer Distrobox task limit.
- This context helps interpret container cgroup paths and potential host/container library compatibility, but it does not change the reproduced task-cap cause. systemd documents `TasksMax` as controlling the pids cgroup limit and counts each thread as a task: https://www.freedesktop.org/software/systemd/man/261/systemd.resource-control.html.
- No browser workload was started during the read-only inspection. Retrying with a higher task cap would change the specified safety ceiling and is not authorized by the current execution constraints. Any bounded TasksMax adjustment must be a separate safety-policy decision; retain memory, swap, and CPU limits.

### 2026-11-03 — bounded Mesa thread-pool workaround did not fit

- A single minimal Firefox probe `/tmp/firefox-pids-probe.mjs` ran with `LP_NUM_THREADS=1` under the unchanged verified 3 GiB/5 GiB/zero-swap/150%/128-task scope. Mesa documents this variable as a limit on llvmpipe renderer threads: https://docs.mesa3d.org/envvars.html. Firefox launched but closed during first navigation. Exit **1**; `pids.peak=128`, `pids.max=128`, `pids.events:max` increased by **13**, memory peak `279,089,152` bytes, and no memory high/max/OOM events. The Mesa thread-pool limit did not prevent task-cap exhaustion. No suite ran.
- Immediately preceding fresh preflight exited **0**: MemAvailable `16,485,244,928` bytes, scratch free `9,575,624,704` bytes, no memory/CPU PSI pressure, load `0.18/0.21/0.15`, swap stability passed, cgroup controls verified, starting tasks 7. No browser workload overlapped another workload, and no process was killed by this agent.
- Stop browser attempts. Further Firefox validation requires a proven task-lighter compatible browser/runtime or an explicitly reviewed TasksMax policy change. Do not automatically raise the limit or weaken Firefox sandbox/process isolation. M5 remains **IN PROGRESS/BLOCKED**; PR #3 remains open with `verify` SUCCESS and `firefox` FAILURE; M6 is unstarted.
