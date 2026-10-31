# WasmGC backend M2 (test blocks): working journal

The running record of M2, updated with every slice so that a later session or another agent can resume without the
conversation that produced it. The task prompt is `2026-10-30_wasmgc_m2_prompt.md`; the plan and its task list are
`docs/design/wasm_gc_backend.md` ("M2 tasks: tests"), which stays the source of truth for decisions. Newest entry
last. Times are America/Detroit.

## How to resume

1. Read the prompt, the plan's M2 task list and the last entry below.
2. `git fetch origin && gh pr list --state open` and compare with "Slices" below. Branches named there hold the
   committed work; a session's scratchpad worktrees do not survive it, so nothing lives only in one.
3. `go test ./internal/backend/wasmgc/ ./tools/difftest/`, then the HowlScene gates with
   `HOWLSCENE_HF=<howlscene-hf checkout> go test ./internal/backend/wasmgc -run TestHowlScene -v`.
4. Add an entry here in the same PR as the work.

## Slices

| Slice | Branch | PR | State |
| --- | --- | --- | --- |
| 0. Harness runs the VM in a temp dir | `chore/harness-vm-tempdir` | #273 | Merged |
| bugs.md #66: JS arithmetic and ordering on run-time types | `fix/js-arith-operand-types` | #274 | Merged |
| bugs.md #68: Go backend test blocks as `go test` tests | `fix/go-test-blocks` | #275 | Merged (`7e14bf92`) |
| bugs.md #67: JS `try_let` caught a test's `exit` | `fix/js-test-exit-caught` | #278 | Merged (`2bcdde48`) |
| ZIP: `zip_build`, `zip_read`, inflate in `std/zip.howl` | `feat/wasmgc-zip` | #277 | Merged (`d38ce70a`) |
| 1–3. Probe, roots, test blocks and runners | `feat/wasmgc-test-blocks` | #280 | Merged (`e46839ff`) |
| Regression gate: wasm tools in CI, `scripts/wasmgc_gate.sh` | `ci/wasmgc-regression-gate` | #279 | Merged (`993d008e`) |
| Follow-up: bugs.md #69, VM case mapping pinned to Unicode 15 | `fix/vm-unicode-case` | #282 | Open |
| 4. Exit gate on HowlScene HF core tests | `feat/wasmgc-test-blocks` | #280 | Merged; 19 of 19 |

## Entries

### 2026-10-30 16:30: state when the first M2 session ended

- `build --target wasmgc --tests` builds `app.test.wasm`; `app.test.mjs` runs it in Node, and `--host wasi` makes a
  WASI command for wasmtime. `TestTestsLikeJavaScript` compares reports with the JavaScript backend's `app.test.js`
  under `node --test`. HowlScene HF's render (5) and schema (8) core tests pass on WebAssembly; backup (6) needs ZIP.
- ZIP: 74 differential fixtures match the JavaScript backend in Node and wasmtime, with archives (deflated, stored
  and damaged) generated in the test by Go's `archive/zip` and `compress/flate`. About 1.3 s for a 1 MB `zip_read`
  or a 550 KB `zip_build`.

Findings:

- A misplaced paren in `std/zip.howl` closed the module early, and `stdDefuns` ignored the text after it, so the
  remaining functions silently disappeared. Std modules now reject trailing text (`TestStdModulesParse`, shown to
  fail without the guard). On `feat/wasmgc-zip`.
- `str_repeat` builds by repeated concatenation, so 550 KB takes 36 s on WebAssembly; the VM stops the same program
  at its 64 MB memory limit. Recorded in the plan as a follow-up, not an M2 blocker.

### 2026-10-30 16:50: resumed in a new session

- The first session's transcript is filed under the old checkout path (`/run/host/run/media/system/...`), which is
  why a resumed session did not find it; this journal and the prompt copy now live in the repository instead.
- Its scratchpad worktrees for #67, #68, ZIP and #66 were gone; every branch was committed, so nothing was lost.
  The stale worktree entries were removed.
- The merge of main into `fix/go-test-blocks` (`5497c38e`) was local and unverified; the full gate is rerunning on
  it before the push.

### 2026-10-30 17:20: #275 merged

- The merged tree passed `go test ./...` under Go 1.26 and 1.27.2 locally and all four CI checks; merged as
  `7e14bf92`. Next: #67 (only bugs.md conflicts with main), then ZIP, then the test-blocks PR.

### 2026-10-30 17:45: journal, ZIP and #67 up as PRs

- The journal and prompt copy merged as #276 (`6893ff4b`).
- ZIP opened as #277 after the full gate on its merged tree (Go 1.26 `./...`, Go 1.27.2 wasmgc and difftest, both
  HowlScene gates in Node and wasmtime).
- #67 merged main; only bugs.md conflicted (both sides added rows), resolved as #68, #67, #66 in descending order.
  Full gate under Go 1.26 and 1.27.2 passed on the merged tree. Opened as #278.

### 2026-10-30 18:05: ZIP CI failure, data after a deflate stream

- #277 failed CI (build, race, latest-go) on one fixture, `deflate junk`: a deflated entry with three bytes after
  its final block. The JavaScript reference rejected it on CI (`ZIP_CORRUPT: j is not valid deflate data`) and
  accepted it locally, so the WebAssembly build, written to match the local run, accepted it.
- Cause: Node's `DecompressionStream` rejects trailing data from 22.23 (CI's ubuntu-24.04 runner has 22.23.4) and
  accepts it in 22.22.1 (this machine). The Compression Streams standard requires the rejection, as browsers do.
- Fix: `inflate` in `std/zip.howl` rejects bytes left after the final block; the fixture pins the standard's
  result and compares with the JavaScript backend only where its Node rejects too. Shown to fail without the fix.
- Lesson for the harness: a JavaScript reference can depend on the Node version. A fixture that passes locally can
  fail on CI's newer Node; check the runner's Node (`gh api repos/actions/runner-images/...`) when they disagree.

### 2026-10-30 18:40: regression gate

- Asked to make sure each change is regression tested. Gaps found: CI had no wasmtime or wasm-tools, so every
  wasmtime leg and every module validation skipped there; the HowlScene HF gates skip without `HOWLSCENE_HF`
  (private repository, not in CI); and a skipped leg reads as a pass.
- CI's `build` job now installs wasmtime 47.0.3 and wasm-tools 1.254.0, pinned by SHA-256.
- `scripts/wasmgc_gate.sh` is the local gate before every merge: `go test ./...` on Go 1.26 and 1.27.2 and the
  HowlScene HF gates. It refuses to run without node, wasmtime, wasm-tools and the checkout, and fails when a
  WebAssembly test skips for one of them (both checked by removing wasmtime from PATH and unsetting the variable).
  Its summary goes in each PR's Test Impact Assessment. Plan: "Verification".
- First full run (this branch, `74b4c258`): passed; 4204 tests on Go 1.26, 4203 on Go 1.27.2, both HowlScene
  gates. Finding: on Go 1.27.2 `TestUnicodeTableIsCurrent` skips because Go 1.27 moved to Unicode 17.0.0 and
  `std/unicode.howl` was generated from Go 1.26's tables, so WebAssembly case mapping can differ from a VM built
  with newer Go for characters added since. Not a failure today (CI builds releases with Go 1.25); regenerate the
  tables when the minimum Go moves.

### 2026-10-30 19:00: #279 merged; M2 exit gate

- #279 merged (`993d008e`). Its CI run installed both tools (checksums OK) and ran the wasmtime legs: the wasmgc
  package took 122 s in `build` against 52-55 s in the jobs without wasmtime; the only WebAssembly skips there are
  the HowlScene HF gates, which run locally.
- Test blocks (#280): with #67 in main the `exit passes try_let` fixture matches; 13 report fixtures in
  `TestTestsLikeJavaScript` and 5 build errors in `TestTestsFailClosed` pass in Node and wasmtime.
- Exit gate, HowlScene HF `247365e`: `TestHowlSceneTestsGate` passes 19 of 19 (backup 6, render 5, schema 8) in
  Node and wasmtime, each report equal to the JavaScript backend's; `scripts/test_core.sh` with its pinned compiler
  (`8e16f799`) also passes 19 of 19.
- Next after #280: show the evidence, then write the M3 prompt (browser host) in the form of the M2 prompt.
- The first gate run on #280's final tree failed: `TestInteropDescriptorMatchesFlagSets`. The WIP commit had added
  the `--tests` build flag without listing it in `interopCommands` (the CLI's machine-readable descriptor). Fixed;
  the gate caught it before a push. Its failure report also printed unrelated lines; it now prints each failing
  test with its own output.

### 2026-10-30 19:40: M2 done

- #280 merged (`e46839ff`) after the regression gate passed on its final tree (4304 tests on Go 1.26, 4303 on Go
  1.27.2, HowlScene HF gates) and all four CI checks. M2 is complete: #273-#280.
- The M3 prompt is `2026-10-30_wasmgc_m3_prompt.md`. Note for it: the M2 prompt's `documentation/TESTING.md`
  does not exist, so the M3 prompt drops it.

### 2026-10-30 20:10: follow-ups to M2's findings

Asked to fix the problems found along the way as part of M2. Already fixed: the ZIP trailing-data mismatch (#277)
and the missing `--tests` interop entry (#280). Three remain, one branch each:

1. Unicode (bugs.md #69, `fix/vm-unicode-case`). Not only the WebAssembly table: the VM itself mapped case by the
   toolchain's Unicode version. howlframe built with Go 1.25 prints `ɤ ꟛ ꟛ Ɤ` for
   `(print (to_upper "ɤ ꟛ") (to_lower "ꟛ Ɤ"))`, with Go 1.27.2 `Ɤ Ꟛ ꟛ ɤ`; 110 code points differ, none of them
   cased in Unicode 15, and `unicode.IsSpace` is unchanged. Fix: `internal/textcase` owns a Unicode 15.0.0 table
   (generated, checked against Go's on Go 1.25/1.26) used by the interpreter, the bytecode VM and the
   `std/unicode.howl` generator. The VM test and the differential fixture fail on Go 1.27.2 without the fix.
2. `str_repeat` quadratic on WebAssembly (`perf/wasmgc-str-repeat`).
3. ZIP duplicate paths lowercased by Go's simple case mapping where the JavaScript reference uses `toLowerCase`
   (`fix/wasmgc-zip-lowercase`).
