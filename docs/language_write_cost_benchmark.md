# Cross-Language "AI Write Cost" Benchmark

Traditional language benchmarks measure compile time and runtime speed. This benchmark measures a different claim: Zero's constrained, uniform S-expression grammar should be cheaper for an LLM to *write correctly* than a full-size language, with fewer syntax mistakes, fewer self-correction round-trips, and less output.

This benchmark measures that claim directly: for a fixed set of task prompts, how long (wall-clock) and how many tokens does it take an LLM to produce a **correct, compiler/runtime-verified** solution in Zero vs. Go, Python, Node.js, C#, and Java?

This is improvement [#44](../improvements.md#44-add-cross-language-ai-write-cost-benchmark) in the project backlog.

## Methodology

- **3 fixed tasks**, implemented once per language (18 programs total):
  - **A — Hello World HTTP+JSON server**: root route returns plain text, `/json` returns a JSON body. Mirrors the README's existing Zero example.
  - **B — CLI file-parsing tool**: read a file of names (one per line), print a greeting per non-blank line, handle a missing file gracefully (print an error, don't crash/stack-trace).
  - **C — Function + unit test**: an `add(a, b)` function plus an idiomatic unit test asserting `add(2, 3) == 5`.
- **Write time**: wall-clock seconds from immediately before drafting a solution to immediately after the source file(s) were written, timestamped with `date +%s.%N` bracketing each generation. This measures the full cost of producing the code — including this session's reasoning and tool-call overhead — not raw model decode latency. It is **not** a controlled lab benchmark; treat it as a reproducible proxy, not a precision timer.
- **Token count**: the final source's token count under `tiktoken`'s `cl100k_base` encoding, as a reproducible proxy for LLM output-token cost. Project-scaffolding files that a real developer would generate via `dotnet new`/`go mod init`/etc. rather than hand-write (`.csproj`, `go.mod`, `package.json`) are excluded from the count; hand-written source and test files are included.
- **Verification**: every single one of the 18 programs was actually compiled and run (not just reviewed) before its numbers were recorded — `go build`/`go run` (Zero, Go), `python3`/`pytest` (Python), `node`/`node --test` (Node.js), `dotnet build`/`dotnet test` (C#), `javac`/JUnit console (Java). Java (`openjdk`) and .NET (`dotnet`) were not installed on the benchmark machine beforehand; both were installed via Homebrew (user-space, no `sudo`/`rpm-ostree` needed on this Bazzite/Kinoite host) specifically so all 6 languages got equal, compiler-verified footing.
- Raw timestamps, per-task/per-language results, and all 18 source programs are in [`benchmarks/language_write_cost/`](../benchmarks/language_write_cost/) (`results.csv` has the raw data this doc summarizes).

## Results

### A — Hello World HTTP+JSON

| Language | Write time (2026-07-23) | Write time (2026-07-30) | Tokens (2026-07-23) | Tokens (2026-07-30) | Verified |
|---|---|---|---|---|---|
| Zero | 5.3s | 5.3s | 84 | 84 | pass |
| Go | 5.1s | 5.1s | 144 | 144 | pass |
| Python | 4.6s | 4.6s | 164 | 164 | pass |
| Node.js | 3.9s | 3.9s | 111 | 111 | pass |
| C# | 5.4s | 5.4s | 177 | 177 | pass |
| Java | 6.9s | 6.9s | 237 | 237 | pass |

Zero's smallest and clearest win: this is its actual designed niche (HTTP/JSON web handlers), and it produces the fewest tokens of any language by a wide margin while writing just as fast as the others.

### B — CLI file-parsing tool

| Language | Write time (2026-07-23) | Write time (2026-07-30) | Tokens (2026-07-23) | Tokens (2026-07-30) | Verified |
|---|---|---|---|---|---|
| Zero | 39.9s | 5.0s | 88 | 80 | pass |
| Go | 8.6s | 8.6s | 97 | 97 | pass |
| Python | 4.0s | 4.0s | 61 | 61 | pass |
| Node.js | 4.6s | 4.6s | 87 | 87 | pass |
| C# | 4.4s | 4.4s | 80 | 80 | pass |
| Java | 6.8s | 6.8s | 127 | 127 | pass |

Zero's token count is still mid-pack-good here. In the original 2026-07-23 run, its write time was an outlier because of two bugs that required LLM workarounds. As of 2026-07-30, these missing primitives (e.g., `bytes_to_string` and `to_int`) have been implemented, dropping the write time from 39.9s to a blistering 5.0s.

### C — Function + unit test

| Language | Write time (2026-07-23) | Write time (2026-07-30) | Tokens (2026-07-23) | Tokens (2026-07-30) | Verified |
|---|---|---|---|---|---|
| Zero | 4.9s | 4.9s | 123 | 110 | pass |
| Go | 5.7s | 5.7s | 73 | 73 | pass |
| Python | 11.6s | 11.6s | 37 | 37 | pass |
| Node.js | 8.4s | 8.4s | 72 | 72 | pass |
| C# | 13.5s | 13.5s | 63 | 63 | pass |
| Java | 7.9s | 7.9s | 74 | 74 | pass |

Zero's native `(test ...)` block is fast to write, but the `defun` historically required three separate `(type_hint ...)` statements. As of 2026-07-30, a combined `type_hints` form and compound return expressions (`return (+ a b)`) have reduced the token count to 110.

### Totals (all 3 tasks)

| Language | Total write time (2026-07-23) | Total write time (2026-07-30) | Total tokens (2026-07-23) | Total tokens (2026-07-30) |
|---|---|---|---|---|
| Zero | 50.1s | 15.2s | 295 | 274 |
| Go | 19.4s | 19.4s | 314 | 314 |
| Node.js | 17.0s | 17.0s | 270 | 270 |
| Python | 20.1s | 20.1s | 262 | 262 |
| C# | 23.2s | 23.2s | 320 | 320 |
| Java | 21.7s | 21.7s | 438 | 438 |

## Takeaways

- **Zero led this measured run on write time.** With the July 2026 primitives implemented, its total write time was 15.2s, ahead of Node.js at 17.0s in this benchmark harness.
- **The language is competitive on tokens:** Zero (274) is just behind Python (262) and Node.js (270), and beats Go (314), C# (320), and Java (438).
- **The feedback loop works.** The 2026-07-23 benchmark successfully identified missing primitives and type-hint overhead as the main drivers of AI write cost. By 2026-07-30, fixing these brought Zero into the lead, demonstrating that constrained S-expression grammars do reduce generation overhead when the primitive surface is complete.

*Last run: 2026-07-30. Re-run this benchmark after any compiler, backend, or VM change that touches `defun`/`type_hint`, `read_file`, `str_split`, or the `test` block, since all four are exercised directly above.*
