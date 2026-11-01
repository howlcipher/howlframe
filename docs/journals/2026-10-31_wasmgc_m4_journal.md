# WasmGC backend M4 (site runtime): working journal

The running record of M4, updated with every slice so that a later session or another agent can resume without the
conversation that produced it. The task prompt is `2026-10-31_wasmgc_m4_prompt.md`; the plan and its task list are
`docs/design/wasm_gc_backend.md` ("M4 tasks: site runtime" and "Site delivery"), which stays the source of truth for
decisions. Newest entry last. Times are America/Detroit.

## How to resume

1. Read the prompt, the plan's M4 task list and the last entry below.
2. `git fetch origin && gh pr list --state open` and compare with "Slices" below, in this repository and in
   howlcipher/howlscene-hf. Branches named there hold the committed work; a session's scratchpad worktrees do not
   survive it.
3. Run the gate: `HOWLSCENE_HF=<howlscene-hf checkout> scripts/wasmgc_gate.sh`.
4. Add an entry here in the same PR as the work.

## Slices

| Slice | Branch | PR | State |
| --- | --- | --- | --- |
| 0. Probe: site delivery, specs, sizes, policies | `docs/wasmgc-m4-probe` | #295 | Merged (`34b94023`) |
| 1. Delivery: `--inline-module` | `feat/wasmgc-inline-module` | #296 | Merged (`2f32365a`) |
| 2. Size: dead functions, locals, budget test | `feat/wasmgc-dead-code` | #297 | Merged (`d210aa49`) |
| 3. HowlScene HF: `SITE_TARGET=wasmgc`, policies, specs on both runtimes | howlscene-hf `build/site-runtime-wasmgc` | howlscene-hf#1 | Merged (`a6715f48`) |
| 4. Exit gate, M5 prompt | `docs/wasmgc-m4-exit` | | Open |

## Entries

### 2026-10-31 18:00: probe

- Started from main at `9266ee56` (M3 merged). The prompt's own PR, #294, was green but not merged in this session:
  the session's permission check refused `gh pr merge`, so it waits for the owner.
- HowlScene HF `247365e` built with its pinned compiler (`8e16f799`, `--target js`) and the William Elias example
  exported; the site runtime built with `9266ee56 --target wasmgc`. Results are in the plan ("Site delivery"): how the
  runtime reaches each page, which specs run it, the size of everything a page downloads, and the policy matrix.
- Decision: the module inline in the loader as base64. The runtime stays one text file (`site_files`, the preview's
  hash and the engine package unchanged); pages add `'wasm-unsafe-eval'` and nothing else. A sibling `app.wasm` needs
  the same keyword plus `connect-src` and fails in the preview, whose `default-src 'none'` blocks the fetch.
- Findings:
  - The 250 KiB budget already holds (108.6 KiB raw, 32.5 KiB gzipped for HTML, CSS and the inline runtime). The
    harder number is `performance.spec.mjs`'s rule that the runtime is at most a quarter of the editor's `app.js`
    gzipped: 23,583 B against a limit of 10,226 B while the editor is JavaScript (40,907 B). Dead-code removal will
    not close a factor of 2.3; the rule compares two builds of different targets until M5. To decide in the size
    slice, with the owner if it comes to changing the spec.
  - The program's own code is 72% of the module, and the error check after each call (about 9 bytes, 816 times) is
    its most frequent sequence; function-level dead code is about 10% of the runtime's text.
  - WebKit 27.2 does not launch here (missing system libraries: libgstgl, libflite, libavif, libsoup-3, libmanette and
    others), so only Chromium and Firefox can run the specs on this machine.
  - HowlScene HF's Playwright config runs Chromium only.

The policy probe (`node probe.mjs <scratchpad>`, run from the scratchpad with HowlScene HF's Playwright; it copies the
exported ink-wash page, swaps the runtime and the policy, and reports whether `hs-motion` appears):

```js
// CSP probe: the site runtime as WebAssembly with the module inline (base64) or as a sibling file.
import { chromium, firefox, webkit } from "/run/media/howlcipher/tallgeese/dev/howlscene-hf/node_modules/playwright/index.mjs";
import { readFileSync, writeFileSync, mkdirSync, cpSync, rmSync } from "node:fs";
import { createHash } from "node:crypto";
import { spawn } from "node:child_process";
const S = process.argv[2];
const wasm = readFileSync(`${S}/sitewasm/app.wasm`), loader = readFileSync(`${S}/sitewasm/app.js`, "utf8");
const page0 = readFileSync(`${S}/site/ink-wash/index.html`, "utf8");
const inlineJS = `var howlFrameWasm=(()=>{const s=atob("${wasm.toString("base64")}"),b=new Uint8Array(s.length);for(let i=0;i<s.length;i++)b[i]=s.charCodeAt(i);return b;})();\n` + loader;
const cases = [];
const self = (scriptSrc) => `default-src 'self'; img-src 'self' data:; script-src ${scriptSrc}; style-src 'self'; object-src 'none'; base-uri 'none'; form-action 'none'`;
const setCSP = (html, csp) => html.replace(/<meta http-equiv="Content-Security-Policy" content="[^"]*">/, `<meta http-equiv="Content-Security-Policy" content="${csp}">`);
function site(name, csp, js, sibling) {
  const d = `${S}/probe/www/${name}`; cpSync(`${S}/site/ink-wash`, d, { recursive: true });
  writeFileSync(`${d}/assets/site.js`, js); if (sibling) writeFileSync(`${d}/assets/app.wasm`, wasm);
  writeFileSync(`${d}/index.html`, setCSP(page0, csp)); cases.push(name);
}
rmSync(`${S}/probe/www`, { recursive: true, force: true });
site("inline-self", self("'self'"), inlineJS);
site("inline-wue", self("'self' 'wasm-unsafe-eval'"), inlineJS);
site("sibling-wue", self("'self' 'wasm-unsafe-eval'"), loader, true);
site("sibling-wue-noconnect", self("'self' 'wasm-unsafe-eval'") + "; connect-src 'none'", loader, true);
// Preview form: everything inline under default-src 'none' and a script hash.
const hash = createHash("sha256").update(inlineJS).digest("base64");
for (const [n, extra] of [["preview-hash", ""], ["preview-hash-wue", " 'wasm-unsafe-eval'"]]) {
  const csp = `default-src 'none'; img-src data:; style-src 'unsafe-inline'; script-src 'sha256-${hash}'${extra}; base-uri 'none'; form-action 'none'`;
  const d = `${S}/probe/www/${n}`; mkdirSync(d, { recursive: true });
  writeFileSync(`${d}/index.html`, setCSP(page0, csp).replace('<script src="assets/site.js" defer></script>', `<script>${inlineJS}</script>`)); cases.push(n);
}
const port = 18000 + Math.floor(Math.random() * 1000);
const srv = spawn("python3", ["-I", "-m", "http.server", String(port), "--bind", "127.0.0.1", "--directory", `${S}/probe/www`], { stdio: "ignore" });
await new Promise((r) => setTimeout(r, 800));
for (const [bn, bt] of [["chromium", chromium], ["firefox", firefox], ["webkit", webkit]]) {
  const b = await bt.launch();
  for (const c of cases) {
    const p = await b.newPage(); const errs = [];
    p.on("pageerror", (e) => errs.push(e.message)); p.on("console", (m) => { if (m.type() === "error") errs.push(m.text()); });
    await p.goto(`http://127.0.0.1:${port}/${c}/`); await p.waitForTimeout(1500);
    const cls = await p.evaluate(() => document.documentElement.className);
    console.log(bn.padEnd(8), c.padEnd(22), /hs-motion/.test(cls) ? "RUNS " : "FAILS", errs.slice(0, 1).join(" ").slice(0, 140));
    await p.close();
  }
  await b.close();
}
srv.kill();
```

Output (2026-10-31, abridged to the first error; the preview rows' image and style errors come from reusing the exported page's links under
`default-src 'none'`, not from the runtime):

```
chromium inline-self            FAILS WebAssembly.compile(): ... violates the following Content Security policy directive because 'unsafe-eval' ...
chromium inline-wue             RUNS
chromium sibling-wue            RUNS
chromium sibling-wue-noconnect  FAILS Connecting to '.../assets/app.wasm' violates the following Content Security Policy directive: "connect-src 'none'"
chromium preview-hash           FAILS
chromium preview-hash-wue       RUNS
firefox  inline-self            FAILS Content-Security-Policy: The page's settings blocked WebAssembly (script-src) ...
firefox  inline-wue             RUNS
firefox  sibling-wue            RUNS
firefox  sibling-wue-noconnect  FAILS Content-Security-Policy: The page's settings blocked the loading of a resource (connect-src) ...
firefox  preview-hash           FAILS
firefox  preview-hash-wue       RUNS
```

Plan of slices (each its own branch and PR):

1. Delivery: `build --target wasmgc --inline` writes one `app.js` holding the module; tests in Node and a CSP test.
2. Size: whole-module dead-code removal (functions, exports the loader does not use), a shorter error check, and a
   budget test on the site runtime.
3. Speed: measure the scroll frame and text conversion; faster conversion if it matters.
4. HowlScene HF: a build switch for `--target wasmgc --inline`, `'wasm-unsafe-eval'` in its three policies, its specs
   on WebAssembly (and Firefox), then the toolchain pin.
5. Exit gate.

### 2026-10-31 19:00: delivery

- `build --target wasmgc --inline-module` writes one `app.js`: the browser loader with the module's bytes as base64
  (`CompileBrowserInline`). The loader's source of bytes is now its own part (`browserFileSource`, which fetches
  `app.wasm` or takes `howlFrameWasm`, and `inlineSource`), so the inline loader has no `fetch` and no file name.
  The option fails closed for anything but a `web_app` for the browser, and without `--target wasmgc`.
- Tests: `TestBrowserInlineModule` (same page as with `app.wasm`, handlers and scroll included; no `</script`, no
  `fetch(`), `TestBrowserInlineModuleFailsClosed`, and `TestHowlSceneSiteGate` now runs three builds of the site
  runtime (JavaScript, WebAssembly with `app.wasm`, inline) and requires one page.
- Found on the way: the site gate's stand-in page had no `atob`, which every browser page and Node have; the inline
  build failed there with "atob is not defined" until the stand-in got it.
- The site runtime built inline: 79,765 B, 24,084 B gzipped. The probe script, with its inline case reading this
  build, ran it in Chromium 156 and Firefox 157 under the exported page's policy and the preview's, each with
  `'wasm-unsafe-eval'`: it runs in all four (`hs-motion` set, no runtime errors).
- Not merged by this session: `gh pr merge` was refused by the session's permission check (also for #294), so this
  branch carries #295's commit and its PR is stacked on it.

### 2026-10-31 20:00: size

- #294 (the M4 prompt) merged at `faf9bd90` on the owner's request; main now has the prompt.
- Where the bytes were (site module, text format): 131 functions, 18 reachable from no export; the program's own
  functions are 72% of the code. Dead-code removal in the encoder (`Module.live`) and merged local runs: numbers in the
  plan's M4 task list. Unbalanced or missing bodies are still reported for functions nothing reaches (the existing
  `TestEncoderReportsUnbalancedBlocks` caught that the first version dropped them silently).
- Measured and dropped: a fail block with `br_if` per error check (smaller raw, larger gzipped).
- `TestHowlSceneSiteBudget` (opt-in, `HOWLSCENE_HF`, so the gate runs it): 94,623 B / 28,801 B gzipped for the
  portfolio fixture's page. With the budget set to 90 KiB it failed naming the page, then was restored.
- Still open, for the owner: `performance.spec.mjs`'s "runtime at most a quarter of the editor's app.js" needs the
  runtime under 10,226 B gzipped while the editor is JavaScript; the inline runtime is 21,893 B.
- Tests: `go test ./internal/backend/wasmgc/ ./internal/backend/javascript/ ./tools/difftest/` pass.

### 2026-11-01 00:30: HowlScene HF on WebAssembly, speed and Firefox

- Owner's decision on the quarter rule (the owner said to do what I recommended): compare like with like. When
  `site.js` is a WebAssembly build, `performance.spec.mjs` measures it against the editor built the same way
  (`--target wasmgc --inline-module`); the rule itself stays at a quarter. It also now checks HTML + CSS + runtime
  together against 250 KiB.
- HowlScene HF (scratch branch `build/site-runtime-wasmgc`, toolchain pinned to this branch's `c3668764`) built with
  `SITE_TARGET=wasmgc`: the full Playwright suite passes in Chromium, 80 of 80. Sizes: `site.js` 22,241 B gzipped,
  the editor as WebAssembly 101,721 B, HTML 5,097 B, CSS 4,582 B.
- Speed: numbers in the plan's M4 task list; no change to text conversion.
- Firefox 157 (an ad-hoc config with `devices["Desktop Firefox"]`, not committed): 72 of 74 pass on WebAssembly. The
  two failures, A11Y-007 (exported text at 320 px, "nothing cut off") and TXT-002 (the editor's link dialog), fail the
  same way with the JavaScript runtime, so they are HowlScene HF's Firefox differences, not this backend's. Every
  motion and story spec passes in Firefox.

### 2026-11-01 02:00: exit gate

- HowlScene HF #1 merged (`a6715f48`), toolchain pinned to HowlFrame `d210aa49`. Its CI (`verify`) ran the browser
  suite on both runtimes and passed.
- Evidence on both merged mains is in the plan (M4 tasks, "Exit gate"). In short: the HowlFrame gate passed; Chromium
  80 of 80 on the WebAssembly site runtime; Firefox every motion and story spec; initial download 31,920 B gzipped.
- Flake seen once: SEC-004 (editor, JavaScript) failed its 5 s wait for a 1,001-file backup to be refused in the run
  that started right after the gate; three runs alone and the full rerun passed. Not the site runtime. Watch it in M5,
  where the editor moves to WebAssembly and ZIP reading gets slower or faster.
- M4 done. The M5 prompt is `2026-11-01_wasmgc_m5_prompt.md`.
