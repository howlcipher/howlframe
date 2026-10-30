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
| bugs.md #67: JS `try_let` caught a test's `exit` | `fix/js-test-exit-caught` | none yet | Committed locally (`e02c84d9`) |
| ZIP: `zip_build`, `zip_read`, inflate in `std/zip.howl` | `feat/wasmgc-zip` | none yet | Committed locally (`efe12fa0`) |
| 1–3. Probe, roots, test blocks and runners | `feat/wasmgc-test-blocks` | none yet | Committed locally (WIP) |
| 4. Exit gate on HowlScene HF core tests | `feat/wasmgc-test-blocks` | none yet | Gate test written; backup needs ZIP |

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
