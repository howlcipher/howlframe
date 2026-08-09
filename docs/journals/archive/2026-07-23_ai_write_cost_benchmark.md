# Cross-Language "AI Write Cost" Benchmark (Improvement #44)

**Date:** 2026-07-23
**Task:** Implement improvement #44 — benchmark HowlFrame vs Go, Python, Node.js, C#, and Java on LLM write-time and token cost (not runtime speed).

## Objective
HowlFrame's pitch is reduced hallucination/retry cost for LLM-authored code, not runtime performance. Build a reproducible benchmark that measures, for a fixed set of task prompts, how long (wall-clock) and how many tokens it takes an LLM (this session, Claude Sonnet 5) to produce a correct, compiler/runtime-verified solution in each of the 6 languages.

## Methodology
- 3 fixed tasks, same for every language:
  - **A — Hello World HTTP+JSON server** (mirrors README's existing example: root route returns text, `/json` returns JSON).
  - **B — CLI file-parsing tool**: read a file, sum a numeric column, handle file-not-found/parse errors without crashing.
  - **C — Function + unit test**: `add(a, b)` plus an accompanying test, comparing HowlFrame's native `(test ...)` block against each language's idiomatic test boilerplate.
- For each (task, language) pair: capture a start timestamp immediately before drafting, write the solution file(s) in one pass, capture an end timestamp immediately after. Elapsed wall-clock time is the "write time." This includes this harness's reasoning + tool-call overhead, not raw model decode latency — stated as a caveat in the published results, not hidden.
- Every program is then actually compiled/run to verify correctness (Go: `go build`; Python: `python3`; Node: `node`; Java: `javac`+`java`; C#: `dotnet run`; HowlFrame: `go run howlframe.go`). Verification time is NOT counted toward write time (that's the traditional runtime-benchmark thing we're explicitly not measuring).
- Token count of the final source is measured with `tiktoken` (`cl100k_base`) as a reproducible proxy for LLM output-token cost.
- JDK (`openjdk`) and .NET SDK (`dotnet`) were not present on this machine; installed via Homebrew (user-space, no sudo/rpm-ostree needed on this Bazzite/Kinoite host) specifically so all 6 languages get equal, compiler-verified footing.

## Result
All 18 (task × language) programs written, timed, token-counted, and verified via actual compile/run — none were review-only. JDK and .NET SDK were installed via Homebrew (user-space) specifically for this task since neither was present. Full results and honest findings (including where HowlFrame loses) are in `docs/language_write_cost_benchmark.md`, linked from `README.md` and `docs/index.html`. Raw data and all 18 source programs are in `benchmarks/language_write_cost/`.

Two real transpiler gaps were discovered and filed while building Task B, not invented for the benchmark: bug #17 (no string-to-number parsing primitive) and a related finding that `read_file` returns `[]byte` with no native conversion to `string`, forcing an undocumented `(call string x)` escape hatch. Task B was redesigned from "sum a numeric column" to string-only processing as a result, documented transparently in both `improvements.md` #44 and the results doc rather than silently worked around.

`improvements.md` #44 marked Done (2026-07-23). This journal is archived.
