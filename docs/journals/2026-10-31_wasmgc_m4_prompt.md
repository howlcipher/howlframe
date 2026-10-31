<!-- The M4 task prompt, written at the M3 exit gate (2026-10-31) so a later session or another agent can start M4.
     M3 record: 2026-10-31_wasmgc_m3_journal.md. Start the M4 journal beside this file. -->

# Task: HowlFrame WasmGC backend, milestone M4 (site runtime)

## Goal
HowlScene HF must run as HowlFrame compiled to WebAssembly, with no hand-written JS, Go
or other languages in the app. Where HowlFrame cannot express something, fix HowlFrame
through reviewed PRs. The point is to find these gaps and build the fixes, so size is
not a reason to stop.

M4: HowlScene HF's published-site runtime (`src/site/site.howl`) runs on WebAssembly in
the pages HowlScene HF publishes; all its motion and story tests pass that way; and a
published page's initial download stays within 250 KiB.

## Repositories and paths
- HowlFrame (howlcipher/howlframe): /run/media/howlcipher/tallgeese/dev/howlframe.
  This is where most work happens. (Older sessions used /run/media/system/... and
  /run/host/run/media/system/...; their transcripts are filed under those paths.)
- HowlScene HF (howlcipher/howlscene-hf, private): /run/media/howlcipher/tallgeese/dev/howlscene-hf
  Read and run it. M4 changes its build so the site runtime is built with
  `--target wasmgc` and its pages load it: do that on a branch of its own, with a PR
  there, after the HowlFrame side has landed. Change nothing else in it.
- Never read or reuse howlcipher/howlscene (a separate track). howlcipher/william_elias
  is read-only.
- Keep temporary files and worktrees in the session scratchpad, never in the repos.
  Scratchpad worktrees do not survive the session: commit work to a branch before
  stopping.

## Tools on this machine (all installed)
- Go 1.26.0 is the default. Go 1.27.2 is cached: `GOTOOLCHAIN=go1.27.2 go test ./...`
  reproduces CI's latest-go job (Go "stable").
- Node v22.22.1 locally; CI's ubuntu-24.04 runner has Node 22.23.x. Node's behavior
  can differ between them (M2's ZIP finding).
- wasm-tools 1.254.0, wasmtime 47.0.3. CI's `build` job installs both, pinned by SHA-256.
- HowlScene HF's end-to-end tests are Playwright 1.64.1 (`npm run test:e2e`, specs in
  `tests/e2e/`); Chromium builds are in `~/.cache/ms-playwright`.
- gh is logged in for howlcipher.
- CI jobs: build, acceptance and race on Go 1.25 (releases build with 1.25), and
  latest-go on stable.

## State at the start (main after the M3 PRs, 2026-10-31)
M1, M2 and M3 are done. M3 landed as #286–#293 (main at `9266ee56`; the M3 journal's
"Slices" gives each PR's merge commit):
- `build --target wasmgc` of a `web_app` (host `browser`, its default) writes
  `app.wasm` and a generated classic-script loader, `app.js`. Each browser host
  operation is a typed import from `capability.HostOperations` (53 operations, in
  `internal/capability/host.go`, shared by the checker, the JavaScript backend and this
  backend), in a module named for its capability; the loader runs the JavaScript
  backend's own runtime function for each (`javascript.HostRuntime`), so errors match
  by construction. Page objects are `$Host` (an externref); text crosses through the
  exported memory as UTF-8.
- Handlers (`on_event`, `on_media`, `on_visible`, `on_scroll`, `spawn`) are closures
  over cells, sharing variables as JavaScript closures do.
- Asynchronous operations (storage, files, images, `sleep`, `fetch`) suspend by
  resumable frames (A-normal form, then frame save and steering); decided against JSPI
  with measurements (plan: "Asynchronous operations").
- `fetch` needs the `network` capability; without a grant the program does not link
  (`CAPABILITY_DENIED`). It resolves to the VM's bytes (a list of byte values).
- Exit gate: HowlFrame's 53 browser tests (`internal/backend/javascript/browser_*`,
  now `package javascript_test`) run every fake-DOM program on both backends and pass
  on both. `TestHowlSceneSiteGate` (opt-in, `HOWLSCENE_HF`) runs the site runtime on a
  stand-in published page and requires the same page on both backends. HowlScene HF's
  editor also builds (272 KB module, 40 KB loader) but is untested on WebAssembly (M5).
- Sizes today: site runtime 45,774 B module + 17,086 B loader (12,925 + 5,078 B
  gzipped); editor 272,072 B + 39,984 B (65,403 + 11,098 B gzipped).
- bugs.md #70 (JS raw DOM operations' `TypeError`) and #71 (JS conversion errors named
  `typeof`) were fixed on the way.
- Open PRs #206 (M3 spec freeze — an unrelated "Milestone 3") and #178 (live-v1
  harness) are someone else's drafts awaiting human review. Do not merge or touch them.

## Read first
1. docs/design/wasm_gc_backend.md: decisions, "Browser host", "Handlers",
   "Asynchronous operations", the operation table ("Browser host operations"),
   milestones, "What HowlScene HF uses", findings, open questions, Resuming and
   Verification. It is the source of truth; update it in the same PR as the work.
2. docs/journals/2026-10-31_wasmgc_m3_journal.md (how M3 went, findings, traps) and
   this prompt, saved as docs/journals/2026-10-31_wasmgc_m4_prompt.md. Start
   docs/journals/<date>_wasmgc_m4_journal.md in the same form and add an entry with
   every slice.
3. Load the howlframe-transpiler skill. Also read bugs.md, improvements.md and
   AGENTS.md.
4. internal/backend/wasmgc/ (browser.go, closure.go, async.go, compile.go,
   runtime.go, browser_test.go, gate_site_test.go) and
   internal/backend/javascript/host.go, browser_backends_test.go.
5. HowlScene HF: src/site/site.howl and site.css, src/core/render.howl (how a page
   includes the runtime: an inline script with a CSP hash, or `assets/site.js` with
   `script-src 'self'`), src/core/export.howl, scripts/build.sh and
   scripts/assemble_editor.py (how site.js is embedded in the editor for export), and
   tests/e2e/motion.spec.mjs, engine-site.spec.mjs, pages.spec.mjs, journey.spec.mjs and
   performance.spec.mjs.

## M4 tasks, in order
0. Probe first: how a published page gets the site runtime today (inline script and
   its CSP hash; the multi-file export's `assets/site.js`; the editor preview), which
   Playwright specs cover motion and the story (chapters, reveal, scroll progress,
   reduced motion, the motion toggle) and what they assert, and the size of every
   file a published page downloads today. Record it in the plan.
1. Delivery: decide how a published page loads the WebAssembly runtime, with the CSP
   it needs. Compiling WebAssembly under a CSP needs `'wasm-unsafe-eval'` in
   `script-src`; fetching `app.wasm` needs `connect-src` (the inline page has
   `default-src 'none'`). Options include the module's bytes inline in the loader
   (base64, one file, no fetch) and a sibling file. Measure each against the budget
   and the CSP; prefer the strictest policy that works in Chromium, Firefox and Safari.
2. HowlFrame side of the decision: build output for it (for example an option that
   inlines the module), generated loader changes, tests.
3. Size: the initial download (HTML, CSS, loader, module) within 250 KiB. Measure what
   the module carries that the site does not use (runtime functions, std modules,
   runtime JavaScript helpers in the loader) and cut it with dead-code removal in the
   compiler; do not depend on an external optimizer to meet the budget. Add a test that
   fails when the site's download exceeds the budget.
4. Speed: text conversion between the module and the page costs about twice the
   JavaScript backend's (5.7 µs against 2.7 µs per call with text in and out). The site
   runtime runs on every scroll frame; measure it in performance.spec.mjs's terms and
   make conversion faster if it matters (for example copying with fewer exports calls
   or caching small strings).
5. HowlScene HF side: its build makes the site runtime with `--target wasmgc` (behind
   a switch first, so both runtimes can be compared), its render and export include it
   with the decided CSP, and its motion and story Playwright specs run against it.
   Fix in HowlFrame whatever they find.
6. Exit gate: every motion and story spec passes with the WebAssembly runtime in
   Chromium (and the other Playwright browsers installed, if any); the initial
   download of a published page measured within 250 KiB; `TestHowlSceneSiteGate` and
   the browser tests still pass on both backends. Show the evidence before starting
   M5.

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
- A test or gate run piped through `grep | tail` reports the pipe's status, not the
  tests'. In M3 a wide run "passed" while two tests failed. Write the log, check
  `${PIPESTATUS[0]}` or the logged exit status, then grep.
- The JavaScript backend awaits every call, so its handlers finish in later
  microtasks: a harness that inspects the page must let them settle first (M3's site
  gate did not, once). Two tasks racing timers of nearly equal length can finish in a
  different order on the two backends; fixtures must not depend on such races.
- A harness snapshot must copy what it records (M3's site gate shared live attribute
  objects between steps).
- Objects from the harness's own realm reach a node:vm page as plain objects of
  another realm, so the loader treats them as page objects; objects the runtime
  makes in the page's realm are dicts. Keep that in mind when a stand-in returns data.
- The desugarer puts `let` in value positions (`list_get`, `map_get` paths); anything
  that rewrites code must handle them. A rewrite once put a `let`'s name in scope
  before its value (the full suite caught it through ZIP); `let shadows its own value`
  is the fixture.
- Differences from the JavaScript backend by design: a caught error is the VM's
  report, not an `Error`; `print` of a list or dict prints the VM's text; `fetch`
  resolves to bytes. Browser tests compare caught errors by code and message.
- Not supported yet (fail closed): a lambda of more than two parameters; an
  asynchronous call in a branch of `if`, `and` or `or` used as a value, in a dict
  literal, or nested inside a `try_let`'s value; `sleep` and `spawn` outside the
  browser.
- Text built by appending to the result so far is quadratic on WebAssembly; watch for
  it in the site runtime and add a linear-time test like `TestLongTextIsLinear` for
  anything that builds long text.
- The VM's `append` copies the list (improvements.md #111, pending).
- Case mapping is pinned to Unicode 15.0.0 (`internal/textcase`, bugs.md #69). Host
  operations follow the engine; std operations HowlFrame's table.
- Escape sequences: a literal backslash-u plus four hex digits written through a tool
  can arrive decoded. Build such text from parts, then grep for U+FFFD.
- gofmt realigns map literals; match by key or line in edit scripts.
- Merges: two branches adding rows to bugs.md or entries to the journal conflict at
  the same spot. Keep both, in order, and run `go vet` after any code merge.
- Branch protection requires branches up to date with main: `gh pr update-branch N`
  or merge origin/main, then rerun the gate.
- The checker rejects mixed-type lists and some literals (`(list 1 2.5)`, `1e30`); an
  untyped parameter can be inferred as text, so fixtures type their parameters.
- New CLI flags must be added to `interopCommands` in howlframe.go
  (TestInteropDescriptorMatchesFlagSets).
- Run long gates with `run_in_background` and wait for the notification. Never commit
  or open a PR from a chain that does not check the gate's exit status.

## Verification before every PR
Run the regression gate on the final tree (after any merge of main):

    HOWLSCENE_HF=/run/media/howlcipher/tallgeese/dev/howlscene-hf scripts/wasmgc_gate.sh

It runs gofmt, vet, build, `go test ./...` on Go 1.26 and 1.27.2, and the HowlScene HF
gates (M1 rendering, backup, M2 core tests, M3 site runtime), and fails when a tool or
the checkout is missing or a WebAssembly test skips for one. Also do a direct
`build --target wasmgc` run of a representative program, and for HowlScene HF changes
its own `npm run test:e2e`. Put the Test Impact Assessment (behavior changed, tests that
prove it, gaps, tests actually run, the gate's summary) in the PR body. Results must
match the final tree.

## Git and authority
- One branch per slice from the latest main, one PR each, merged with a merge commit
  once all four CI checks pass. You are authorized to merge your own green PRs and
  continue. HowlScene HF PRs follow the same rule in its repository.
- Do not merge other people's PRs (#206, #178).
- No force-pushes, and no tag rewrites. Do not touch the v0.2.0 tag or release.
- Ask first before tagging a release, making a repo public, or deploying GitHub Pages.
- Commit messages and PR bodies end with the session's attribution lines.

## Working style
Work autonomously and ask only when truly blocked. Journal as you go
(docs/journals/<date>_wasmgc_m4_journal.md), so another session or agent can resume.
After each merge give a short status: what landed, sizes and test counts, findings,
and what's next. At the M4 exit gate, show the evidence, then write a self-contained
prompt for M5 (editor: the full HowlScene HF browser suite including the beginner
journey on WebAssembly, and HowlScene HF dropping `--target js`) in the same form as
this one.
