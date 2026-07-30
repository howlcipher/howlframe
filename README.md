# Zero

Zero is an AI-first, S-expression programming language and toolchain. It is designed to be easy for language models to write, easy for tooling to validate, and flexible enough to target more than one runtime.

The current toolchain includes:

- A lexer, parser, AST layer, semantic checker, shared tree IR, and flat SSA/CFG lowering pipeline.
- Go code generation for `http_server` and `cli_app` programs.
- JavaScript generation for `web_app` programs.
- A WebAssembly Text prototype for `wasm_app` programs.
- Direct AST execution for a bounded `cli_app` subset with `-run`.
- Binary bytecode generation and VM execution with `-compile-bc` and `-run-bc`, including VM-local native stores.

Zero still uses generated Go as its broadest backend, but the project now spans several output and execution paths. The compiler also exposes a typed SSA control-flow graph layer for future native backends, reducing dependence on human-readable intermediate code where direct execution, bytecode, or lower-level targets are a better fit.

## Why Zero?

LLMs are good at producing structure, but they often lose time on syntax details, invalid APIs, and large rewrites. Zero keeps the source grammar small and uniform so generation can be constrained and validated before runtime.

Key design points:

- **Uniform syntax:** Zero source is built from balanced S-expressions, which are easier to grammar-constrain than full-size general-purpose languages.
- **Semantic feedback:** Invalid shared forms and type/layout mistakes fail with localized JSON errors before backend code is emitted.
- **Explicit surface area:** The language can only express behavior that has an implemented AST, IR, backend, or VM mapping.
- **Multiple execution paths:** The same front end can feed Go, JavaScript, WAT, direct interpretation, or bytecode depending on the root node and flags.

## Current State

Zero is a working experimental language toolchain. The Go backend is the most complete target and supports HTTP servers, CLI apps, tests, structs, JSON parsing, file and process primitives, database calls, middleware, imports, includes, observability hooks, and AI-oriented primitives.

The JavaScript backend supports `web_app` logic and Node test generation. The WebAssembly backend is intentionally narrower: it emits locally parsed and type-validated WAT for typed numeric/control-flow expressions, static and dynamic aggregate reads, dynamic dictionary keys, and runtime initialization of integer and string aggregate expressions. Direct AST execution and bytecode VM execution cover bounded `cli_app` subsets and reject unsupported nodes with explicit errors. The bytecode VM also supports an in-memory native store for structured records through `store_open`, `store_put`, `store_get`, and `store_delete`.

See:

- [Direct execution design](docs/direct_execution_design.md)
- [Zero native store design](docs/zero_native_store_design.md)
- [Language write-cost benchmark](docs/language_write_cost_benchmark.md)
- [Bytecode reference](bytecode_reference.md)
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

Clone the repo and run a Zero program:

```bash
git clone https://github.com/howlcipher/zero.git
cd zero
go run zero.go examples/cli_hello.zero
go run server.go
```

For a `cli_app` supported by the interpreter subset, run without generating Go:

```bash
go run zero.go -run examples/cli_hello.zero
```

For bytecode:

```bash
go run zero.go -compile-bc examples/cli_hello.zero
go run zero.go -run-bc examples/cli_hello.zero.bc.bin
go run zero.go -compile-bc tests/test_store_bytecode.zero -o /tmp/test_store_bytecode.zbc
go run zero.go -run-bc /tmp/test_store_bytecode.zbc
```

For WebAssembly Text:

```bash
go run zero.go -o build examples/wasm_math.zero
```

That writes `build/app.wat`.

## Language Roots

Zero programs use one root form:

- `(cli_app ...)` for command-line programs.
- `(http_server port ...)` for Go-backed HTTP servers.
- `(web_app ...)` for JavaScript browser logic.
- `(wasm_app expression)` for the current WAT backend.

## Examples

### CLI

```lisp
(cli_app
  (print "Hello, World!")
  (let (name "Zero")
    (print "Welcome to" name)
  )
)
```

### HTTP Server

```lisp
(http_server 8080
  (route "/" (lambda (req)
    (res 200 "text/plain" "Hello from Zero")
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
go run zero.go examples/hello.zero
go run server.go
```

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
go run zero.go counter.zero
node --test app.test.js
```

### WebAssembly Text

```lisp
(wasm_app
  (+ 10 32)
)
```

```bash
go run zero.go -o build examples/wasm_math.zero
```

## Common Language Features

Zero supports the core control-flow and data primitives expected by the shipped backends:

- `let`, `set`, `if`, `while`, `for`, `match`, and `do`.
- `defun`, `call`, `return`, `type_hint`, and `type_param`.
- `list`, `dict`, `append`, `map_set`, `map_delete`, `map_get`, and `list_get`.
- `str_split`, `str_join`, `regex_match`, `to_int`, `to_float`, `to_string`, and `bytes_to_string`.
- `read_file`, `write_file`, `mkdir`, `exec`, `sleep`, and `cli_args`.
- `spawn`, `fetch`, `middleware`, `next`, `env`, `import`, and `include` where supported by the target backend.
- `test` blocks that generate Go or Node tests depending on the target.
- Bytecode-native record stores through `store_open`, `store_put`, `store_get`, and `store_delete`.

AI-oriented primitives include `llm_generate`, `fuzzy_cast`, `assert_semantic`, `semantic_match`, `neural_circuit`, `ephemeral_circuit`, `achieve`, `lazy_synthesize`, `optimize_block`, `patch`, `with_context`, `spawn_agent`, and `task`. Backend and VM coverage differs by primitive; unsupported combinations should fail during checking or execution instead of being silently accepted.

## Constrained Decoding Plans

The `internal/masking` package compiles semantic `ast.TypeInfo` values and complete `checker.Analysis` results into deterministic, JSON-serializable mask plans. Plans describe provider-neutral token classes, literals, collection delimiters, struct fields, function parameter and return constraints, and schema bridges for use by downstream constrained decoders.

Use `(schema_bridge User output)` to bind an output source to a declared `User` struct. The checker rejects unknown targets and the plan records the exact struct constraint; `go run zero.go -mask-plan program.zero` prints the complete plan.

This API does not call a model or map constraints to provider-specific token IDs. Live logit masking remains the responsibility of an inference integration that consumes the plan.

## Orchestrator

`orchestrator.py` is an optional local-model experiment. It currently uses Outlines with an OpenAI-compatible Ollama endpoint to generate JSON bytecode that is executed by the Zero VM through `-run-bc`.

Start Ollama, make sure the configured model exists, then run:

```bash
ollama serve
ollama pull llama3
python orchestrator.py
```

The orchestrator is not required for normal Zero source files. Manual `.zero` development uses `go run zero.go ...` directly.

## Output Files

By default, Zero writes generated files into the current directory:

- Go targets write `server.go` and, when tests exist, `server_test.go`.
- JavaScript targets write `app.js` and, when tests exist, `app.test.js`.
- WebAssembly targets write `app.wat`.
- Bytecode compilation writes `<input>.bc.bin` by default.

Use `-o <dir>` to write generated artifacts elsewhere:

```bash
go run zero.go -o build examples/hello.zero
```

For bytecode compilation, `-o <file>` can also name the exact bytecode output file:

```bash
go run zero.go -compile-bc examples/cli_hello.zero -o build/cli_hello.zbc
```

## Observability

Generated Go programs include built-in tracing and crash capture. `observer.py` can tail telemetry, inspect crashes, and run an opt-in patch workflow against an isolated project copy when all required patch-mode options are supplied.

Manual trace points are available with `(trace var)`.

## Benchmark

The write-cost benchmark compares the time and token cost of having an LLM produce working programs in Zero, Go, Python, Node.js, C#, and Java. The benchmark is measured from checked-in runs, not estimated.

See [docs/language_write_cost_benchmark.md](docs/language_write_cost_benchmark.md) for methodology, raw result links, limitations, and current results.
