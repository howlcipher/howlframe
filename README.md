# HowlFrame

## What is HowlFrame?

HowlFrame is an experimental AI-native language and capability-bounded execution runtime.

`.howl` source can compile to standalone `.hfbc` bytecode executed by the HowlFrame VM.

The VM enforces runner-owned capability grants and finite instruction budgets.

## Why does it exist?

The governing principle is simple: **intent is not authority**.

Adaptive operations may reason, classify, generate, rank, plan, and propose. Deterministic machinery continues to own permissions, capability grants, persistent state, irreversible mutations, invariant enforcement, verification, and approval.

LLMs are good at producing structure, but they often lose time on syntax details, invalid APIs, and large rewrites. HowlFrame keeps the source grammar small and uniform so generation can be constrained and validated before runtime.

## Install

### Release binaries
Download the latest release binary for your platform from the GitHub Releases page. Extract the archive and place `howlframe` in your PATH.

### Build from source
```bash
git clone https://github.com/howlcipher/howlframe.git
cd howlframe
go build -o howlframe howlframe.go
```

## 60-second example

Write a simple program `hello.howl`:
```lisp
(cli_app
  (print "Hello from HowlFrame")
)
```

Then check, build, and run:

```bash
howlframe check hello.howl
howlframe build hello.howl
howlframe run hello.hfbc
```

Output:
```
Hello from HowlFrame
```

## How it works

The compiler pipeline provides semantic feedback. Invalid shared forms and type/layout mistakes fail with localized JSON errors before backend code is emitted. The language can only express behavior that has an implemented AST, IR, backend, or VM mapping. 

The standalone bytecode target enforces this against an authoritative construct-support registry, so anything it cannot lower fails before an artifact is written rather than being silently dropped. The repository also has an internal, directly executable HFIR Phase-1 subset used for differential conformance evidence; public builds still use the AST bytecode compiler. See [HFIR execution status](docs/hfir_execution_status.md).

## Security / authority model

Intent is not authority.

HowlFrame demonstrates an untrusted AI proposing deployment actions while standalone HowlFrame independently enforces evidence, approval, capability, and state-transition policy. 

**Demo: Release Authority / Action Executor**
AI/user proposal
↓
HowlFrame authority
↓
ALLOW / DENY / REQUIRE_APPROVAL
↓
bounded effect

## Applications Built With HowlFrame

* [Status API](apps/status_api/README.md) — Proves HTTP serving, deterministic routing, and environment inspection.
* [Log Analyzer](apps/log_analyzer/README.md) — Proves file parsing, deterministic string logic, and graceful capability denial.
* [KV CLI](apps/kv_cli/README.md) — Proves in-memory store functionality and sequential deterministic state.
* [Todo CLI](apps/todo_cli/README.md) — Dogfooding Phase 2 task manager proving stateful CRUD logic atop the native store.
* [Task API](apps/task_api/README.md) — Dogfooding Phase 3 HTTP task service proving native-store state shared across independent requests.
* [Release Authority](apps/release_authority/README.md) — Demonstrates an untrusted AI proposing deployment actions while standalone HowlFrame independently enforces evidence, approval, capability, and state-transition policy.
* [Action Executor](apps/action_executor/README.md) — Bounded Action Executor — demonstrates a finite trusted action catalog where untrusted AI/user proposals can request operations but cannot select arbitrary code, paths, capabilities, or execution primitives.
* [Repo Analyst](examples/repo_analyst/README.md) — Proves larger multi-module applications and finite instruction budgets.

## Current State

HowlFrame is a working experimental language toolchain. The Go backend is the most complete target and supports HTTP servers, CLI apps, tests, structs, JSON parsing, file and process primitives, database calls, middleware, imports, includes, observability hooks, and AI-oriented primitives.

The JavaScript backend supports `web_app` logic and Node test generation. The WebAssembly backend is intentionally narrower: it emits locally parsed and type-validated WAT for typed numeric/control-flow expressions, static and dynamic aggregate reads, dynamic dictionary keys, and runtime initialization of integer and string aggregate expressions. Direct AST execution and bytecode VM execution cover bounded `cli_app` subsets and reject unsupported nodes with explicit errors: the interpreter fails closed at the point of evaluation, and `-compile-bc` fails closed at compile time with a `HFIR_TARGET_INFEASIBLE` diagnostic naming the construct, its source location, and the backlog item that owns the gap. The bytecode VM also supports an in-memory native store for structured records through `store_open`, `store_put`, `store_get`, and `store_delete`.

See:

- [Direct execution design](docs/direct_execution_design.md)
- [HowlFrame native store design](docs/howlframe_native_store_design.md)
- [Language write-cost benchmark (v1)](docs/language_write_cost_benchmark.md)
- [Benchmark v2](benchmarks/v2/README.md)
- [Architecture roadmap](docs/architecture_roadmap.md)
- [HFIR execution status](docs/hfir_execution_status.md)
- [Bytecode reference](docs/reference/bytecode_reference.md)
- [Improvement backlog](improvements.md)
- [Bug log](bugs.md)

## Requirements

- Go 1.21 or newer.
- Python 3.10 or newer if you use the optional orchestrator.
- Ollama or another OpenAI-compatible local model endpoint if you use LLM-backed orchestration.

Install optional Python dependencies for the orchestrator:

```bash
pip install outlines openai
```

## Quick Start

Clone the repo and run a HowlFrame program:

```bash
git clone https://github.com/howlcipher/howlframe.git
cd howlframe
go run howlframe.go examples/cli_hello.howl
```

The default Go backend writes `server.go`. Run the generated program with:

```bash
go run server.go
```

To build the canonical compiler executable:

```bash
go build -o howlframe howlframe.go
./howlframe -validate examples/cli_hello.howl
```

For a `cli_app` supported by the interpreter subset, run without generating Go:

```bash
go run howlframe.go -run examples/cli_hello.howl
```

For bytecode:

```bash
go run howlframe.go -compile-bc examples/cli_hello.howl
go run howlframe.go -run-bc examples/cli_hello.howl.bc.bin
go run howlframe.go -compile-bc tests/test_store_bytecode.howl -o /tmp/test_store_bytecode.hfbc
go run howlframe.go -run-bc /tmp/test_store_bytecode.hfbc
go run howlframe.go -run-bc --max-instructions 1000000 /tmp/test_store_bytecode.hfbc
```

For WebAssembly Text:

```bash
go run howlframe.go -o build examples/wasm_math.howl
```

That writes `build/app.wat`.

To compile a checked `cli_app` expression through the typed SSA/CFG backend:

```bash
go run howlframe.go -compile-wasm examples/native_math.howl
go run howlframe.go -compile-wasm examples/native_math.howl -o build/native_math.wat
```

The default artifact is `<input>.ssa.wat`. This native serializer supports
integer, float, and boolean constants, arithmetic and comparisons, boolean
`and`/`or`, lexical `let` value flow, `if`/phi merges, returns, structured
`while` loops with loop-carried `set` mutation, and calls between top-level
`defun`s with scalar (int/float/bool) parameters and return values,
including recursion — all runnable standalone with no Go process, verified
against `wasmtime`. Aggregates (`list`/`dict`), printing, strings, and type
conversions still fail with an explicit backend error; see improvements.md
items #73/#74 for that remaining language-surface work.

## Language Roots

HowlFrame programs use one root form:

- `(cli_app ...)` for command-line programs.
- `(http_server port ...)` for Go-backed HTTP servers.
- `(web_app ...)` for JavaScript browser logic.
- `(wasm_app expression)` for the current WAT backend.

## Examples

### CLI

```lisp
(cli_app
  (print "Hello, World!")
  (let (name "HowlFrame")
    (print "Welcome to" name)
  )
)
```

### HTTP Server

```lisp
(http_server 8080
  (route "/" (lambda (req)
    (res 200 "text/plain" "Hello from HowlFrame")
  ))

  (route "/json" (lambda (req)
    (let (msg (dict ("status" "ok") ("runtime" "go")))
      (res_json 200 msg)
    )
  ))
)
```

Run it:

```bash
go run howlframe.go examples/hello.howl
go run server.go
```

For a standalone HTTP server that accepts JSON request data, compose the
existing JSON parser with the route request value:

```lisp
(try_let (body (parse_json Any req.body))
  (catch err (res_json 400 (dict ("error" "invalid_json"))))
  (res_json 200 (dict ("title" (map_get body "title"))))
)
```

In the standalone bytecode VM, `req.body` is valid only as the input to
`parse_json` in an HTTP route. `try_let` can turn malformed JSON into an
application response. This is a JSON-only route-input path; it does not
provide a general request-body value, streaming input, or a configurable
body-size limit. Request bodies are untrusted input, so applications should
keep payloads small and validate their decoded fields.

### Web App Logic

```lisp
(web_app
  (defun increment (n)
    (type_hint n "int")
    (type_hint return "int")
    (return (+ n 1))
  )

  (on_event (dom_query "#btn") "click" (lambda (e)
    (set_text (dom_query "#label") (call increment 1))
  ))

  (test "increment works"
    (if (!= (call increment 1) 2)
      (print "failed")
    )
  )
)
```

```bash
go run howlframe.go counter.howl
node --test app.test.js
```

### WebAssembly Text

```lisp
(wasm_app
  (+ 10 32)
)
```

```bash
go run howlframe.go -o build examples/wasm_math.howl
```

### Reference Application

[HowlFrame Repo Analyst](examples/repo_analyst/README.md) is a deterministic, five-module application that compiles to HowlFrame bytecode and analyzes repositories without generated Go or JavaScript. Its dogfooding tests prove both the unchanged default instruction ceiling and a larger finite budget explicitly authorized by the trusted runner.

## Common Language Features

HowlFrame supports the core control-flow and data primitives expected by the shipped backends:

- `let`, `set`, `if`, `while`, `for`, `match`, and `do`.
- `defun`, `call`, `return`, `type_hint`, and `type_param`.
- `list`, `dict`, `append`, `map_set`, `map_delete`, `map_get`, and `list_get`.
- `str_split`, `str_join`, `regex_match`, `to_int`, `to_float`, `to_string`, and `bytes_to_string`.
- `read_file`, `write_file`, `mkdir`, `exec`, `sleep`, and `cli_args`.
- `spawn`, `fetch`, `middleware`, `next`, `env`, `import`, and `include` where supported by the target backend.
- `test` blocks that generate Go or Node tests depending on the target.
- Bytecode-native record stores through `store_open`, `store_put`, `store_get`, and `store_delete`.

AI-oriented primitives include `llm_generate`, `fuzzy_cast`, `assert_semantic`, `semantic_match`, `neural_circuit`, `ephemeral_circuit`, `achieve`, `lazy_synthesize`, `optimize_block`, `optimize_signature`, `patch`, `with_context`, `spawn_agent`, and `task`. Backend and VM coverage differs by primitive. On the standalone bytecode target that difference is enforced: `internal/construct` classifies every construct as `Supported`, `CompileTimeOnly`, or `Unsupported`, and `-compile-bc` rejects the unsupported ones before emitting an artifact. The Go backend deliberately stays permissive, because it supports a large open set of heads.

## Constrained Decoding Plans

The `internal/masking` package compiles semantic `ast.TypeInfo` values and complete `checker.Analysis` results into deterministic, JSON-serializable mask plans. Plans describe provider-neutral token classes, literals, collection delimiters, struct fields, function parameter and return constraints, and schema bridges for use by downstream constrained decoders.

Use `(schema_bridge User output)` to bind an output source to a declared `User` struct. The checker rejects unknown targets and the plan records the exact struct constraint; `go run howlframe.go -mask-plan program.howl` prints the complete `howlframe.mask_plan/v1` plan.

This API does not call a model or map constraints to provider-specific token IDs. Live logit masking remains the responsibility of an inference integration that consumes the plan.

## Compile-Time Optimization Plans

Use `optimize_signature` to record optimization intent around an otherwise normal body expression:

```lisp
(optimize_signature support_prompt
  (metric "accuracy")
  (test "go test ./...")
  (candidate "baseline" "Answer clearly.")
  (candidate "strict" "Answer only with verified facts.")
  (print "body"))
```

The form requires a symbol name, one metric, one or more test commands, one or more labeled candidate payloads, and one body. `go run howlframe.go -optimization-plan program.howl` prints deterministic `howlframe.optimization_plan/v1` JSON containing that metadata, source coordinates, and the inferred body type. The compiler records commands as strings only: it does not run tests, call a model, rewrite source, or select a candidate. Normal Go codegen, direct execution, and bytecode execution evaluate only the wrapped body.

## Orchestrator

`tools/orchestrator/orchestrator.py` is an optional local-model experiment. It currently uses Outlines with an OpenAI-compatible Ollama endpoint to generate JSON bytecode that is executed by the HowlFrame VM through `-run-bc`.

Start Ollama, make sure the configured model exists, then run:

```bash
ollama serve
ollama pull llama3
python tools/orchestrator/orchestrator.py
```

The orchestrator is not required for normal HowlFrame source files. Manual `.howl` development uses `go run howlframe.go ...` directly.

The generated bytecode schema used by that script lives at `tools/orchestrator/orchestrator_schema.py`, and its model prompt context lives at `tools/orchestrator/ai_prompt.md`.

## Output Files

By default, HowlFrame writes generated files into the current directory:

- Go targets write `server.go` and, when tests exist, `server_test.go`.
- JavaScript targets write `app.js` and, when tests exist, `app.test.js`.
- WebAssembly targets write `app.wat`.
- SSA WebAssembly compilation writes `<input>.ssa.wat` by default.
- Bytecode compilation writes `<input>.bc.bin` by default.

Use `-o <dir>` to write generated artifacts elsewhere:

```bash
go run howlframe.go -o build examples/hello.howl
```

`-o <dir>` may also follow the input file for Go, JavaScript, and legacy WAT
generation; the directory is created when it does not exist:

```bash
go run howlframe.go examples/hello.howl -o build
```

For bytecode compilation, `-o <file>` can also name the exact bytecode output file:

```bash
go run howlframe.go -compile-bc examples/cli_hello.howl -o build/cli_hello.hfbc
```

The same exact-output handling applies to SSA WebAssembly compilation when
`-o <file>` follows the input:

```bash
go run howlframe.go -compile-wasm examples/native_math.howl -o build/native_math.wat
```

## Execution Policy

Every `-run-bc` execution uses a runner-owned execution policy. The default remains 100,000 instructions. A trusted runner may select another positive, finite ceiling by placing `--max-instructions` before the bytecode artifact:

```bash
go run howlframe.go -run-bc --max-instructions 1000000 /tmp/test_store_bytecode.hfbc
```

The ceiling is the maximum number of VM instructions allowed. A program that needs exactly the configured count succeeds; the next attempted instruction fails before dispatch with structured `LIMIT_EXCEEDED`. Zero, negative, malformed, and overflowed values are rejected before bytecode execution. There is no unlimited value.

Execution policy is not HowlFrame source syntax and is not stored in HFIR or the bytecode artifact. Application code cannot raise its own budget. Instruction budget and capabilities are independent: more instructions never grant access to external resources.

## Capability Security

Every bytecode opcode declares a `Capability` in `internal/bytecode/opcode.go` (`network`, `filesystem`, `process`, `environment`, `database`, or none). `-run-bc` enforces this at the VM level: before executing an instruction, `internal/vm.BCVM` checks the instruction's capability against an allow-list, and panics with a structured `CAPABILITY_DENIED` runtime error if it isn't present. The default is fail-closed — with no allow-list, every capability-gated instruction is denied and only `CapNone` instructions (arithmetic, control flow, printing, in-process data structures, etc.) run.

Pass an allow-list with `-allow-caps`, a comma-separated list of capability names, placed *before* the input file (like all of `howlframe.go`'s other flags — Go's `flag` package stops parsing at the first positional argument):

```bash
go run howlframe.go -run-bc -allow-caps network,filesystem examples/cli_hello.howl.bc.bin
```

An unrecognized capability name in `-allow-caps` is rejected outright rather than silently granting nothing. See `docs/reference/bytecode_reference.md` for the full opcode-to-capability mapping.

## Observability

Generated Go programs include built-in tracing and crash capture. `tools/observer_agent/observer_agent.py` can tail telemetry, inspect crashes, and run an opt-in patch workflow against an isolated project copy when all required patch-mode options are supplied.

Manual trace points are available with `(trace var)`.

## Project Layout

The project root intentionally keeps only the primary compiler entry point, Go module files, high-level docs, and backlog files.

| Path | Purpose |
| --- | --- |
| `howlframe.go` | Main CLI entry point for parsing, checking, code generation, direct execution, bytecode, mask plans, and optimization plans. |
| `internal/` | Go packages for AST, parser, checker, IR, backends, bytecode, VM, masking, and optimization internals. |
| `cmd/` | Developer commands such as generated reference and schema output. |
| `examples/` | User-facing `.howl` programs and reference applications. |
| `tests/` | HowlFrame fixtures plus Python and Go regression tests. |
| `docs/` | GitHub Pages site, design notes, journals, archived historical source, and generated reference material. |
| `benchmarks/` | Checked-in benchmark programs and result data. |
| `tools/` | Optional Python tools, including the orchestrator and observer agent. |
| `observer/` | Go runtime tracing package imported by generated Go programs. |

## Benchmark

The write-cost benchmark compares the time and token cost of having an LLM produce working programs in HowlFrame, Go, Python, Node.js, C#, and Java. The benchmark is measured from checked-in runs, not estimated.

See [docs/language_write_cost_benchmark.md](docs/language_write_cost_benchmark.md) for methodology, raw result links, limitations, and current results.

## License

HowlFrame is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
