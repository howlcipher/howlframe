# HowlScene HF on the WebAssembly GC backend: the M1 rendering gate

HowlScene HF (`main` `247365e`) keeps its project model in three HowlFrame modules: `src/core/registry.howl`,
`schema.howl` and `render.howl`. Driven from a `cli_app` over its two neutral fixture projects
(`tests/fixtures/neutral/portfolio.json` and `legacy-v1.json`), embedded as text, the driver runs `parse_project`.
Each project is then rendered with `render_document` twice, with the export options and with inline CSS and motion
off, and its normalized configuration is printed with `encode_json`. On the bytecode VM
(`howlframe run --max-instructions 500000000`) and built with `--target wasmgc` and run in Node 22, the output is
byte-identical: 21,872 bytes, SHA-256 prefix `6b1ba5ed1153a334` in a first run with shorter fixture labels. The module
validates with `wasm-tools validate --features all`.

The gate is reproducible: `HOWLSCENE_HF=<checkout> go test ./internal/backend/wasmgc/ -run TestHowlSceneGate -v`
generates the driver, runs both engines and compares every byte. Without the variable the test is skipped, since CI has
no HowlScene HF checkout.

What it took on the Frame side, in order: dicts (#251), the core-library sugar (#252), `try_let` with the VM's error
codes (#254), modules (#255), and a standard library written in HowlFrame: `encode_json` (#256), `regex_match` (#257)
and `parse_json` (#258). No HowlScene HF change was needed.

Findings for Frame:

* The VM's default instruction limit (100,000) stops the renderer on the portfolio fixture: one project needs a few
  million instructions. The WebAssembly build has no limit yet (an open question in the plan).
* `howlframe -max-instructions N prog.howl` writes `prog.hfbc` and exits 0 without running anything or printing what
  it wrote; `howlframe run --max-instructions N prog.howl` is the form that runs (bugs.md #62).
* `use` paths are joined to the using file's directory, so a driver outside the application tree must use relative
  paths; an absolute path loses its leading slash.
* `backup.howl` also builds once `base64_text` exists; with `sniff_media_type`, it is a browser host operation of the
  JavaScript backend with no VM reference, so it belongs to M3.

Not yet part of the gate: running the corpus under wasmtime, which needs the remaining host imports (float digits and
parsing) written in HowlFrame and a WASI host, and the `tools/difftest` target.
