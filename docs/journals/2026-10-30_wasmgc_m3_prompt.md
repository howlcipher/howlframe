<!-- The M3 task prompt, written at the M2 exit gate (2026-10-30) so a later session or another agent can start M3.
     M2 record: 2026-10-30_wasmgc_m2_journal.md. Start the M3 journal beside this file. -->

# Task: HowlFrame WasmGC backend, milestone M3 (browser host)

## Goal
HowlScene HF must run as HowlFrame compiled to WebAssembly, with no hand-written JS, Go
or other languages in the app. Where HowlFrame cannot express something, fix HowlFrame
through reviewed PRs. The point is to find these gaps and build the fixes, so size is
not a reason to stop.

## Repositories and paths
- HowlFrame (howlcipher/howlframe): /run/media/howlcipher/tallgeese/dev/howlframe.
  This is where the work happens. (Older sessions used /run/media/system/... and
  /run/host/run/media/system/...; their transcripts are filed under those paths.)
- HowlScene HF (howlcipher/howlscene-hf, private): /run/media/howlcipher/tallgeese/dev/howlscene-hf
  Read and run it; do not change it unless a task says so.
- Never read or reuse howlcipher/howlscene (a separate track). howlcipher/william_elias
  is read-only.
- Keep temporary files and worktrees in the session scratchpad, never in the repos.
  Scratchpad worktrees do not survive the session: commit work to a branch before
  stopping.

## Tools on this machine (all installed)
- Go 1.26.0 is the default. Go 1.27.2 is cached: `GOTOOLCHAIN=go1.27.2 go test ./...`
  reproduces CI's latest-go job (Go "stable").
- Node v22.22.1 locally; CI's ubuntu-24.04 runner has Node 22.23.x. Node's behavior
  can differ between them (see the ZIP finding below).
- wasm-tools 1.254.0, wasmtime 47.0.3 (`wasmtime run -W gc=y,function-references=y app.wasm`).
  CI's `build` job installs the same two, pinned by SHA-256, so the wasmtime legs and
  module validation run on every PR.
- gh is logged in for howlcipher.
- CI jobs: build, acceptance and race on Go 1.25 (releases build with 1.25), and
  latest-go on stable.

## State at the start (main after the M2 PRs, 2026-10-30)
M1 and M2 are done. M2 landed as #273–#280:
- `howlframe build prog.howl --target wasmgc [--host node|wasi] [--tests] -o DIR`.
  `--tests` compiles a file's `test` blocks, whatever its root (`cli_app` or `web_app`),
  into `app.test.wasm` with a runner that reports like node:test's spec reporter
  (`app.test.mjs` for Node; a WASI command for wasmtime). A `web_app` without
  `--tests` still fails with `WASM_UNSUPPORTED`; making it build is this milestone.
- Fixtures: about 400 differential fixtures against the VM, 74 ZIP fixtures and 13
  test-report fixtures against the JavaScript backend, all in Node and wasmtime.
- HowlScene HF gates: rendering (M1), backup, and the M2 core tests (19 of 19 pass in
  both engines with the same report as `scripts/test_core.sh`).
- Already in the standard library, ahead of M3: `zip_build` and `zip_read`
  (`std/zip.howl`, synchronous here, asynchronous on the JavaScript backend),
  `base64_text`, `sniff_media_type`.
- Bugs fixed on the way: bugs.md #66 (JS arithmetic on run-time types), #67 (JS
  `try_let` caught `exit` in tests), #68 (Go backend test blocks).
- Open PRs #206 (M3 spec freeze — an unrelated "Milestone 3") and #178 (live-v1
  harness) are someone else's drafts awaiting human review. Do not merge or touch them.

## Read first
1. docs/design/wasm_gc_backend.md: decisions (host access as typed, capability-gated
   imports from a generated host module), value representation, error propagation,
   "Asynchronous host operations", milestones and exit gates, the M3 task list, "What
   HowlScene HF uses", findings, open questions, Resuming and Verification. It is the
   source of truth; update it in the same PR as the work.
2. docs/journals/2026-10-30_wasmgc_m2_journal.md (how M2 went, findings, traps) and
   this prompt, saved as docs/journals/2026-10-30_wasmgc_m3_prompt.md. Start
   docs/journals/<date>_wasmgc_m3_journal.md in the same form and add an entry with
   every slice.
3. Load the howlframe-transpiler skill. Also read bugs.md, improvements.md and
   AGENTS.md.
4. internal/backend/wasmgc/ (encode.go, code.go, runtime.go, compile.go, loader.go,
   std/*.howl, diff_test.go, tests_test.go, zip_test.go, gate_test.go).
5. The reference for every browser operation is the JavaScript backend:
   internal/backend/javascript/ — dom.go, browser_io.go, motion.go, richtext.go,
   archive.go, exit.go, javascript.go — and its browser tests
   (internal/backend/javascript/browser_*_test.go, 51 tests), which run the generated
   app.js in node:vm against a fake DOM.

## M3 tasks, in order
0. Probe first: list every browser host operation the JavaScript backend supports,
   with its parameters, result, errors, capability and whether it is asynchronous,
   and which of them HowlScene HF's site runtime and editor use (`src/`). Record the
   list in the plan.
1. Typed host operation table: name, parameter and result kinds, capability,
   synchronous or asynchronous. Shared by the checker and the backends, and the
   browser loader is generated from it. A program not granted a capability cannot
   link. Prefer one authoritative registry (internal/construct, internal/capability)
   over a parallel table.
2. Host object handles (`$Host` with `externref`) and a `web_app` root that builds
   for the browser: the loader instantiates the module against the page.
3. Closures and event dispatch: `lambda`, `on_event`, `spawn`, and the subscription
   operations (`on_media`, `on_visible`, `on_scroll`, `off`).
4. Asynchronous operations (storage, files, image decode and encode, `sleep`; ZIP is
   synchronous here already). Decide between resumable frames (the plan's default)
   and JSPI with measurements: code size, speed, and which engines run it. Record the
   decision with its numbers.
5. Every DOM, storage, file, image, archive and rich text operation the JavaScript
   backend supports, with the same errors. Includes the VM's `[]byte` values that
   M1 left for M3 (files and the network): decide their representation.
6. Exit gate: HowlFrame's browser tests (browser_*_test.go) run against both
   backends — the same fake-DOM programs, built with `--target js` and
   `--target wasmgc`, with the same output. Add an opt-in HowlScene HF gate if any of
   its browser code can run under that harness. Show the evidence before starting M4.

## Method (every slice)
- Probe the reference with small programs before writing code.
- Work test-first. Add differential fixtures for the happy path, errors and edge cases.
- Check that each fixture is not vacuous: the reference must run and print what it
  should. Prove that a regression test fails without the fix by reverting the source
  files, then restore them.
- A JavaScript reference can depend on the Node version. When a fixture passes locally
  and fails on CI (or the reverse), check the runner's Node and pin the standard's
  result rather than either Node's.
- Validate every module with `wasm-tools validate --features all`.
- Fail closed: an unsupported construct gives WASM_UNSUPPORTED with a source location.
- A bug in the VM or another backend gets its own branch and PR, with a bugs.md entry
  (ranked row plus details section) and a regression test.
- Host imports stay typed and capability-gated. No JS beyond generated loaders and
  runners.

## Findings and traps carried over
- `str_repeat` builds by repeated concatenation: 550 KB takes 36 s on WebAssembly.
  The site runtime's 250 KiB budget and speed make string building matter in M4;
  consider a string builder.
- `std/unicode.howl` was generated from Go 1.26's Unicode tables; Go 1.27 has Unicode
  17.0.0 and `TestUnicodeTableIsCurrent` skips there. Regenerate when the minimum Go
  moves.
- Duplicate ZIP paths are compared in lower case, and JavaScript's `toLowerCase`
  (full mapping) differs from Go's simple mapping for U+0130 and final sigma: not
  matched yet.
- Escape sequences: a literal backslash-u plus four hex digits written through a tool
  can arrive decoded. Build such text from parts, then grep for U+FFFD.
- gofmt realigns map literals; match by key or line in edit scripts.
- Merges: two branches adding rows to bugs.md or entries to the journal conflict at
  the same spot. Keep both, in order, and run `go vet` after any code merge.
- Branch protection requires branches up to date with main: `gh pr update-branch N`
  or merge origin/main, then rerun the gate.
- The checker rejects mixed-type lists and some literals (`(list 1 2.5)`, `1e30`).
- New CLI flags must be added to `interopCommands` in howlframe.go
  (TestInteropDescriptorMatchesFlagSets).
- Run long gates with `run_in_background` and wait for the notification. Never commit
  or open a PR from a chain that does not check the gate's exit status.

## Verification before every PR
Run the regression gate on the final tree (after any merge of main):

    HOWLSCENE_HF=/run/media/howlcipher/tallgeese/dev/howlscene-hf scripts/wasmgc_gate.sh

It runs gofmt, vet, build, `go test ./...` on Go 1.26 and 1.27.2, and the HowlScene HF
gates, and fails when a tool or the checkout is missing or a WebAssembly test skips for
one. Also do a direct `build --target wasmgc` run of a representative program. Put the
Test Impact Assessment (behavior changed, tests that prove it, gaps, tests actually run,
the gate's summary) in the PR body. Results must match the final tree.

## Git and authority
- One branch per slice from the latest main, one PR each, merged with a merge commit
  once all four CI checks pass. You are authorized to merge your own green PRs and
  continue.
- Do not merge other people's PRs (#206, #178).
- No force-pushes, and no tag rewrites. Do not touch the v0.2.0 tag or release.
- Ask first before tagging a release, making a repo public, or deploying GitHub Pages.
- Commit messages and PR bodies end with the session's attribution lines.

## Working style
Work autonomously and ask only when truly blocked. Journal as you go
(docs/journals/<date>_wasmgc_m3_journal.md), so another session or agent can resume.
After each merge give a short status: what landed, fixture counts, findings, and what's
next. At the M3 exit gate, show the evidence, then write a self-contained prompt for M4
(site runtime: HowlScene HF's published-site runtime on WebAssembly, motion and story
tests, the 250 KiB download budget) in the same form as this one.
