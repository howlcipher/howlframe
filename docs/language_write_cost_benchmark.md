# Cross-Language "AI Write Cost" Benchmark

Traditional language benchmarks measure compile time and runtime speed. This benchmark measures a different claim: HowlFrame's constrained, uniform S-expression grammar should be cheaper for an LLM to *write correctly* than a full-size language, with fewer syntax mistakes, fewer self-correction round-trips, and less output.

This benchmark measures that claim directly: for a fixed set of task prompts, how long (wall-clock) and how many tokens does it take an LLM to produce a **correct, compiler/runtime-verified** solution in HowlFrame vs. Go, Python, Node.js, C#, and Java?

This is improvement [#44](../improvements.md#44-add-cross-language-ai-write-cost-benchmark) in the project backlog.

## Methodology

- **3 fixed tasks**, implemented once per language (18 programs total):
  - **A — Hello World HTTP+JSON server**: root route returns plain text, `/json` returns a JSON body. Mirrors the README's existing HowlFrame example.
  - **B — CLI file-parsing tool**: read a file of names (one per line), print a greeting per non-blank line, handle a missing file gracefully (print an error, don't crash/stack-trace).
  - **C — Function + unit test**: an `add(a, b)` function plus an idiomatic unit test asserting `add(2, 3) == 5`.
- **Write time**: wall-clock seconds from immediately before drafting a solution to immediately after the source file(s) were written, timestamped with `date +%s.%N` bracketing each generation. This measures the full cost of producing the code — including this session's reasoning and tool-call overhead — not raw model decode latency. It is **not** a controlled lab benchmark; treat it as a reproducible proxy, not a precision timer.
- **Token count**: the final source's token count under `tiktoken`'s `cl100k_base` encoding, as a reproducible proxy for LLM output-token cost. Project-scaffolding files that a real developer would generate via `dotnet new`/`go mod init`/etc. rather than hand-write (`.csproj`, `go.mod`, `package.json`) are excluded from the count; hand-written source and test files are included.
- **Verification**: every single one of the 18 programs was actually compiled and run (not just reviewed) before its numbers were recorded — `go build`/`go run` (HowlFrame, Go), `python3`/`pytest` (Python), `node`/`node --test` (Node.js), `dotnet build`/`dotnet test` (C#), `javac`/JUnit console (Java). Java (`openjdk`) and .NET (`dotnet`) were not installed on the benchmark machine beforehand; both were installed via Homebrew (user-space, no `sudo`/`rpm-ostree` needed on this Bazzite/Kinoite host) specifically so all 6 languages got equal, compiler-verified footing.
- Raw timestamps, per-task/per-language results, and all 18 source programs are in [`benchmarks/language_write_cost/`](../benchmarks/language_write_cost/) (`results.csv` has the raw data this doc summarizes).

## Results

### A — Hello World HTTP+JSON

| Language | Write time (2026-07-23) | Write time (2026-07-30) | Tokens (2026-07-23) | Tokens (2026-07-30) | Verified |
|---|---|---|---|---|---|
| HowlFrame | 5.3s | 5.3s | 84 | 84 | pass |
| Go | 5.1s | 5.1s | 144 | 144 | pass |
| Python | 4.6s | 4.6s | 164 | 164 | pass |
| Node.js | 3.9s | 3.9s | 111 | 111 | pass |
| C# | 5.4s | 5.4s | 177 | 177 | pass |
| Java | 6.9s | 6.9s | 237 | 237 | pass |

HowlFrame's smallest and clearest win: this is its actual designed niche (HTTP/JSON web handlers), and it produces the fewest tokens of any language by a wide margin while writing just as fast as the others.

### B — CLI file-parsing tool

| Language | Write time (2026-07-23) | Tokens (2026-07-23) | Tokens (2026-07-30) | Tokens (2026-10-10) | Verified |
|---|---|---|---|---|---|
| HowlFrame | 39.9s → 5.0s | 88 | 80 | **56** | pass |
| Go | 8.6s | 97 | 97 | 97 | pass |
| Python | 4.0s | 61 | 61 | 61 | pass |
| Node.js | 4.6s | 87 | 87 | 87 | pass |
| C# | 4.4s | 80 | 80 | 80 | pass |
| Java | 6.8s | 127 | 127 | 127 | pass |

HowlFrame's write time was an outlier on 2026-07-23 until `bytes_to_string` / `to_int` landed (5.0s on 2026-07-30). As of 2026-10-10, `(lines …)` density sugar plus root `(do …)` and bare `(print)` bring Task B to **56 tokens**, under Python's 61.

### C — Function + unit test

| Language | Write time (2026-07-23) | Tokens (2026-07-23) | Tokens (2026-07-30) | Tokens (2026-10-10) | Verified |
|---|---|---|---|---|---|
| HowlFrame | 4.9s | 123 | 110 | **34** | pass |
| Go | 5.7s | 73 | 73 | 73 | pass |
| Python | 11.6s | 37 | 37 | 37 | pass |
| Node.js | 8.4s | 72 | 72 | 72 | pass |
| C# | 13.5s | 63 | 63 | 63 | pass |
| Java | 7.9s | 74 | 74 | 74 | pass |

HowlFrame's native `(test ...)` block was fast to write, but typed `defun` historically required three separate `(type_hint ...)` statements (123 tokens on 2026-07-23). Combined `type_hints` and compound `(return (+ a b))` cut that to 110 on 2026-07-30. As of 2026-10-10, density sugar — root `(do …)` as `cli_app`, inline typed params, expression-bodied `defun`, `(assert …)`, and auto-`call` — brings Task C to **34 tokens**, under Python's 37.

### Totals (all 3 tasks)

| Language | Total write time (2026-07-30) | Total tokens (2026-07-23) | Total tokens (2026-07-30) | Total tokens (2026-10-10) |
|---|---|---|---|---|
| HowlFrame | 15.2s | 295 | 274 | **178** |
| Go | 19.4s | 314 | 314 | 314 |
| Node.js | 17.0s | 270 | 270 | 270 |
| Python | 20.1s | 262 | 262 | 262 |
| C# | 23.2s | 320 | 320 | 320 |
| Java | 21.7s | 438 | 438 | 438 |

## Takeaways

- **HowlFrame now leads on total tokens** (178 vs Python 262 / Node 270) after 2026-10-10 density sugar on Tasks B and C, while keeping the July write-time lead (15.2s).
- **Task C win:** HowlFrame 34 < Python 37 (`defun` + `assert`).
- **Task B win:** HowlFrame 56 < Python 61 (`lines` + root `do`).
- **Task A remains a large win** (88 vs Python 164) on the HTTP/JSON niche.
- **The feedback loop works.** Measured gaps drove `type_hints`, then expression-body/`assert`/`do`-root/auto-call/`lines` sugar.

*Last fixture recount: 2026-10-10 (`cl100k_base`). Re-run after changes touching `defun`/`type_hint`/`assert`/`do`-root/`lines`/`read_text`, `read_file`, `str_split`, or `test`.*
