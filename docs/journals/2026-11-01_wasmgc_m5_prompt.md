<!-- The M5 task prompt, written at the M4 exit gate (2026-11-01) so a later session or another agent can start M5.
     M4 record: 2026-10-31_wasmgc_m4_journal.md. Start the M5 journal beside this file. -->

# Task: HowlFrame WasmGC backend, milestone M5 (editor)

## Goal
HowlScene HF must run as HowlFrame compiled to WebAssembly, with no hand-written JS, Go
or other languages in the app. Where HowlFrame cannot express something, fix HowlFrame
through reviewed PRs. The point is to find these gaps and build the fixes, so size is
not a reason to stop.

M5: HowlScene HF's editor (`src/editor/editor.howl`) runs on WebAssembly; the full
HowlScene HF browser suite passes that way, the beginner journey (`journey.spec.mjs`)
included; and HowlScene HF stops using the JavaScript backend (`--target js`) for the
editor, the site runtime and the engine package.

## Repositories and paths
- HowlFrame (howlcipher/howlframe): /run/media/howlcipher/tallgeese/dev/howlframe.
  This is where most work happens.
- HowlScene HF (howlcipher/howlscene-hf, private): /run/media/howlcipher/tallgeese/dev/howlscene-hf
  Read and run it. M5 changes its build (the editor and the engine package built with
  `--target wasmgc`, then `--target js` gone): do that on branches of its own, with PRs
  there, after the HowlFrame side has landed. Change nothing else in it unless a spec
  finding needs it, and say so in the PR.
- Never read or reuse howlcipher/howlscene (a separate track). howlcipher/william_elias
  is read-only.
- Keep temporary files and worktrees in the session scratchpad, never in the repos.
  Scratchpad worktrees do not survive the session: commit work to a branch before
  stopping.

## Tools on this machine (all installed)
- Go 1.26.0 is the default. Go 1.27.2 is cached: `GOTOOLCHAIN=go1.27.2 go test ./...`
  reproduces CI's latest-go job (Go "stable").
- Node v22.22.1 locally; CI's ubuntu-24.04 runner has Node 22.23.x.
- wasm-tools 1.254.0, wasmtime 47.0.3. CI's `build` job installs both, pinned by SHA-256.
- HowlScene HF's browser tests are Playwright 1.64.1 (`npx playwright test`, specs in
  `tests/e2e/`, Chromium only in `playwright.config.mjs`). Chromium 156 and Firefox 157
  run here; WebKit 27.2 is installed but does not start (missing system libraries).
- gh is logged in for howlcipher.
- CI jobs: build, acceptance and race on Go 1.25 (releases build with 1.25), and
  latest-go on stable.

## State at the start (main after the M4 PRs, 2026-11-01)
M1 to M4 are done. M4 landed as HowlFrame #295–#297 and HowlScene HF #1 (the M4
journal's "Slices" gives each merge commit):
- `build --target wasmgc --inline-module` writes one `app.js` for a `web_app`: the
  browser loader with the module inside it as base64, decoded before
  `WebAssembly.compile`; it fetches nothing. Pages need `'wasm-unsafe-eval'` in
  `script-src` and nothing else (the plan's "Site delivery" has the policy probe).
- The encoder leaves out every function nothing reaches and merges local runs. Site
  module 40,227 B; editor module 261,440 B; the editor inline 101,721 B gzipped.
- HowlScene HF: `SITE_TARGET=wasmgc scripts/build.sh` builds the site runtime on
  WebAssembly (default still `js`); its three policies (exported site, preview, editor
  page) allow `'wasm-unsafe-eval'`; CI builds both runtimes and runs the browser suite
  on each (80 specs each). `performance.spec.mjs` compares a WebAssembly site runtime
  with the editor built the same way (the owner's decision, M4).
- Scroll frames are the same on both runtimes (median 16.7 ms), so text conversion
  (5.7 µs against 2.7 µs per host call with text in and out) was left as it is. The
  editor makes many more host calls; measure again there.
- In Firefox every motion and story spec passes on WebAssembly; A11Y-007 (exported text
  at 320 px) and TXT-002 (the editor's link dialog) fail in Firefox on both runtimes:
  HowlScene HF's own Firefox differences, not this backend's.
- The editor builds with `--target wasmgc` but has never run in a page. It uses 29 host
  operations, among them the asynchronous storage, file and image operations, `sleep`
  and `spawn` (the plan's "Browser host operations" table counts them).
- Open PRs #206 (M3 spec freeze — an unrelated "Milestone 3") and #178 (live-v1
  harness) are someone else's drafts awaiting human review. Do not merge or touch them.

## Read first
1. docs/design/wasm_gc_backend.md: decisions, "Browser host", "Handlers",
   "Asynchronous operations", "Browser host operations", "M4 tasks", "Site delivery",
   findings, open questions, Resuming and Verification. It is the source of truth;
   update it in the same PR as the work.
2. docs/journals/2026-10-31_wasmgc_m4_journal.md and this prompt, saved as
   docs/journals/2026-11-01_wasmgc_m5_prompt.md. Start
   docs/journals/<date>_wasmgc_m5_journal.md in the same form and add an entry with
   every slice.
3. The howlframe-transpiler skill (in howlcipher/howlplane's `.agents/skills/`).
   Also read bugs.md, improvements.md and AGENTS.md.
4. internal/backend/wasmgc/ (browser.go, closure.go, async.go, compile.go, encode.go,
   runtime.go, browser_test.go, gate_site_test.go) and
   internal/backend/javascript/host.go, browser_backends_test.go.
5. HowlScene HF: src/editor/editor.howl and index.html, scripts/build.sh and
   assemble_editor.py, src/sdk/sdk.howl (the engine package, built with
   `--target js --module` into `app.mjs` with TypeScript declarations), and
   tests/e2e/*.spec.mjs, journey.spec.mjs first.

## M5 tasks, in order
0. Probe first: build the editor with `--target wasmgc --inline-module`, load it in
   the editor page (its policy already allows `'wasm-unsafe-eval'`) and record what
   fails and where; list every construct the editor uses that this backend refuses
   (the fail-closed list below); measure its download and its start-up time against
   the JavaScript editor. Record it in the plan.
1. Close what the probe finds, in HowlFrame, one slice per gap, each with differential
   fixtures (the browser tests run every fake-DOM program on both backends).
2. HowlScene HF: an `EDITOR_TARGET` switch like `SITE_TARGET`; the browser suite on
   it, the beginner journey and backups first. Fix in HowlFrame whatever it finds.
3. The engine package (`src/sdk/sdk.howl`, `--module`): it is an ES module for Node
   with TypeScript declarations, used by the specs and the examples. Decide how it is
   built without the JavaScript backend (a `--module` form of this backend's Node
   loader, or a different split) and do it.
4. Speed: the editor's typing, preview refresh and export in the specs' terms; make
   conversion faster if it matters there.
5. HowlScene HF drops `--target js`: wasmgc becomes the only target, the switches
   go, CI runs one build.
6. Exit gate: the full browser suite, the journey included, passes on WebAssembly in
   Chromium (and Firefox, noting the two known differences); `scripts/build.sh` has
   no `--target js`; the HowlFrame gate and the browser tests pass. Show the evidence
   before starting M6.

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
- Measure size gzipped as well as raw: a change can shrink one and grow the other (M4's
  `br_if` error checks did).

## Findings and traps carried over
- A test or gate run piped through `grep | tail` reports the pipe's status, not the
  tests'. Write the log, check the logged exit status, then grep.
- Stopping a gate run (TaskStop) can leave its test servers running: in M4 an orphaned
  `apps/task_api` server kept port 8091 and the next gate failed `TestTaskAPI/lifecycle`
  (`id:126`). After stopping a run, check `ss -ltnp` for leftovers before the next.
- Never stash or switch branches in a checkout while a gate runs in it; use a second
  worktree.
- The JavaScript backend awaits every call, so its handlers finish in later
  microtasks: a harness that inspects the page must let them settle first. Two tasks
  racing timers of nearly equal length can finish in a different order on the two
  backends; fixtures must not depend on such races.
- A harness snapshot must copy what it records. A node:vm stand-in page must provide
  what every page has (M4's site gate lacked `atob`).
- Objects from the harness's own realm reach a node:vm page as plain objects of
  another realm, so the loader treats them as page objects; objects the runtime makes
  in the page's realm are dicts.
- The desugarer puts `let` in value positions (`list_get`, `map_get` paths); anything
  that rewrites code must handle them (`let shadows its own value` is the fixture).
- Differences from the JavaScript backend by design: a caught error is the VM's
  report, not an `Error`; `print` of a list or dict prints the VM's text; `fetch`
  resolves to bytes. Browser tests compare caught errors by code and message.
- Not supported yet (fail closed): a lambda of more than two parameters; an
  asynchronous call in a branch of `if`, `and` or `or` used as a value, in a dict
  literal, or nested inside a `try_let`'s value; `sleep` and `spawn` outside the
  browser. The editor is asynchronous throughout: expect these first.
- Text built by appending to the result so far is quadratic on WebAssembly; add a
  linear-time test like `TestLongTextIsLinear` for anything that builds long text
  (the editor's export and backups build long text).
- The VM's `append` copies the list (improvements.md #111, pending).
- Case mapping is pinned to Unicode 15.0.0 (`internal/textcase`, bugs.md #69). Host
  operations follow the engine; std operations HowlFrame's table.
- Escape sequences: a literal backslash-u plus four hex digits written through a tool
  can arrive decoded. Build such text from parts, then grep for U+FFFD.
- gofmt realigns map literals; match by key or line in edit scripts.
- Merges: two branches adding rows to the journal's slice table conflict there. Keep
  both, in order, and run `go vet` after any code merge.
- Branch protection requires branches up to date with main: `gh pr update-branch N`
  or merge origin/main, then rerun the gate.
- The checker rejects mixed-type lists and some literals (`(list 1 2.5)`, `1e30`); an
  untyped parameter can be inferred as text, so fixtures type their parameters.
- New CLI flags must be added to `interopCommands` in howlframe.go.
- HowlScene HF's toolchain pin must be a merged commit on HowlFrame main; for local
  trials set `HOWLFRAME_SRC` to the local clone and pin a local commit, then re-pin
  before the PR.
- Run long gates with `run_in_background` and wait for the notification. Never commit
  or open a PR from a chain that does not check the gate's exit status.

## Verification before every PR
Run the regression gate on the final tree (after any merge of main):

    HOWLSCENE_HF=/run/media/howlcipher/tallgeese/dev/howlscene-hf scripts/wasmgc_gate.sh

It runs gofmt, vet, build, `go test ./...` on Go 1.26 and 1.27.2, and the HowlScene HF
gates (M1 rendering, backup, M2 core tests, M3 site runtime, M4 site budget), and fails
when a tool or the checkout is missing or a WebAssembly test skips for one. Also do a
direct `build --target wasmgc` run of a representative program, and for HowlScene HF
changes its full CI sequence (source policy, build, provenance, core and SDK tests,
`npx playwright test` on each build). Put the Test Impact Assessment (behavior changed,
tests that prove it, gaps, tests actually run, the gate's summary) in the PR body.
Results must match the final tree.

## Git and authority
- One branch per slice from the latest main, one PR each, merged with a merge commit
  once all CI checks pass. You are authorized to merge your own green PRs and
  continue; the session's permission check may still ask the owner to confirm a
  merge. HowlScene HF PRs follow the same rule in its repository.
- Do not merge other people's PRs (#206, #178).
- No force-pushes, and no tag rewrites. Do not touch the v0.2.0 tag or release.
- Ask first before tagging a release, making a repo public, or deploying GitHub Pages
  (HowlScene HF's `pages` workflow included).
- Commit messages and PR bodies end with the session's attribution lines.

## Working style
Work autonomously and ask only when truly blocked. Journal as you go
(docs/journals/<date>_wasmgc_m5_journal.md), so another session or agent can resume.
After each merge give a short status: what landed, sizes and test counts, findings,
and what's next. At the M5 exit gate, show the evidence, then write a self-contained
prompt for M6 (no other languages in the app: styles and the editor page shell
generated from `.howl`, and the source policy allowing only `.howl` under `src/`) in
the same form as this one.
