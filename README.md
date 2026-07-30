# Zero Transpiler

Zero is an AI-first, Lisp-like coding language designed specifically to be written by Large Language Models (LLMs). It transpiles directly into robust, production-ready Go.

## Why Zero? (Why not just write Go?)

You might wonder: *If Zero just compiles into Go, why not have the AI write Go directly?*

1. **Hallucination-Proof Generation**: Modern LLMs often hallucinate invalid syntax or complex abstractions in strictly typed languages. Zero uses simple, uniformly structured S-expressions (Lisp-like grammar). Because the syntax is so simple, we can use tools like `outlines` (as seen in `orchestrator.py`) to mathematically guarantee the AI generates perfectly balanced, structurally valid code.
2. **Immediate Semantic Feedback**: If the AI attempts to do something semantically invalid (e.g., calling a method that doesn't exist), the Go transpiler immediately catches it and returns a clean, localized JSON error. The Orchestrator automatically feeds this back to the AI for self-correction.
3. **Abstraction Constraints**: The AI is strictly constrained by what the transpiler supports. It cannot hallucinate complex, dangerous, or unintended behavior unless an explicit AST mapping exists for it.

Zero combines the **predictability and simplicity of S-expressions** (for the AI to write) with the **performance, safety, and ecosystem of Go** (for the server to run).

### How much does writing Zero actually cost, compared to Go, Python, Node.js, C#, and Java?

See [`docs/language_write_cost_benchmark.md`](docs/language_write_cost_benchmark.md) — a measured (not estimated) comparison of LLM write-time and token cost across all six languages, using the same fixed set of task prompts, with every program actually compiled and run. Zero wins clearly on its designed niche (HTTP/JSON handlers) and is mid-pack on tokens overall; the benchmark also reports where it currently loses and the two transpiler bugs discovered while building it.

## Project Roadmap & The End Goal

Zero has evolved from a local script into a standalone transpiler toolchain, now living in its own independent repository ([howlcipher/zero](https://github.com/howlcipher/zero)).

Beyond simple language mechanics, the ultimate **End Goal** of Zero is to completely bypass human-readable code. If AI is writing the code, we no longer need text-based syntaxes (like Go or JS transpilers) designed for human eyes. 

The roadmap to this future includes:
- **Direct Neural Bytecode Synthesis:** Outputting raw machine instructions or Neural Intermediate Representation (NIR) directly. **Shipped 2026-07-29**: direct binary bytecode execution (`-compile-bc` and `-run-bc`) proves the core premise — real Zero programs are compiled to binary gob encoding and executed with zero Go/JS text ever generated. Full design and coverage are in `docs/direct_execution_design.md`.
- **Latent Execution:** Processing inputs and outputs directly through the model's neural pathways, skipping compilation entirely.
- **Ephemeral Neural Circuits:** Generating highly specialized micro-models for a single task that delete themselves after execution.
- **Agentic Observability:** Replacing traditional debugging with observer AI models that monitor system behavior, analyze full context traces, and trigger self-healing workflows autonomously.
- **Leveraging Agent Skills:** Utilizing autonomous agent skills and the unique reasoning capabilities of AI (which often exceed human understanding) to act as the verification and observability layer, ensuring safety without needing to read code.

This will allow:
- Seamless installation by AI agents anywhere via `curl` and GitHub Releases.
- Proper semantic versioning and cross-platform CI/CD pipelines.
- A dedicated standard library (`std.zero`, `http.zero`).
- Focus on pure AI optimizations like LLM-native primitives, AST-level semantic patching, and implicit context threading without being tied to a general knowledge library.

## Installation & Requirements

To write and run Zero manually, you only need **Go**. To use the AI Orchestrator to generate Zero code, you need a few additional tools.

### Prerequisites
1. **Go 1.20+**: Required to compile the transpiled Go code.
2. **Python 3.10+**: Required for the AI Orchestrator script.
3. **Ollama**: Required to run local LLMs (like `llama3`) for generating code.

### Setup Steps
1. Clone the repository:
   ```bash
   git clone https://github.com/howlcipher/zero.git
   cd zero
   ```
2. (Optional but recommended) Set up a Python virtual environment:
   ```bash
   python -m venv venv
   source venv/bin/activate
   ```
3. Install the required Python packages for the orchestrator:
   ```bash
   pip install outlines openai
   ```
4. Start Ollama and download the Llama 3 model (in a separate terminal):
   ```bash
   ollama serve
   ollama pull llama3
   ```

## Hello World Example

Here is a basic HTTP server written in Zero that serves a text response and a JSON endpoint.

Create a file called `hello.zero`:

```lisp
(http_server 8080
  (route "/" (lambda (req)
    (res 200 "text/plain" "Hello, World! Zero language is alive!")
  ))
  
  (route "/json" (lambda (req)
    (let (msg (dict ("status" "success") ("message" "Hello from Zero JSON endpoint!")))
      (res_json 200 msg)
    )
  ))
)
```

### CLI "Hello World" Example

If you want to build a simple command-line script instead of a web server, you can use the `(cli_app ...)` root block instead:

Create `cli_hello.zero`:

```lisp
(cli_app
  (print "Hello, World!")
  (let (name "Zero")
    (print "Welcome to" name)
  )
)
```

### Command Line Arguments

Zero scripts can effortlessly read command-line parameters using `(cli_args)`:

```lisp
(cli_app
  (print "All arguments:" (cli_args))
  (print "First argument:" (cli_args 0))
)
```

### AI Orchestration Example

Zero comes with built-in primitives to orchestrate other AIs natively and enforce constraints effortlessly. This example calls another LLM directly (`llm_generate`), coerces messy text into a strict struct (`fuzzy_cast`), and applies an intent-based qualitative validation (`assert_semantic`):

```lisp
(cli_app
  ;; Ask another LLM directly and handle its error like any other fallible call
  (try_let (resp (llm_generate "Translate 'Hello World' to French" "llama3"))
    (catch err (print "Error:" err))
    (print "AI says:" resp)
  )

  (struct User (name string) (age int))

  ;; Coerce messy, unstructured text into a strict struct
  (try_let (user_struct (fuzzy_cast User "{ \"name\": \"Alice\", \"age\": 30 }"))
    (catch err (print err))
    (print user_struct)
  )

  ;; Enforce a qualitative, natural-language condition instead of a brittle regex
  (let (is_valid (assert_semantic "Alice is a doctor" "is professional"))
    (if (= is_valid true)
      (print "Approved")
      (print "Rejected")
    )
  )
)
```

### AST-Level Semantic Patching

Zero supports surgically updating functions without rewriting the entire file, which is highly beneficial for LLMs struggling with large file generation:

```lisp
(cli_app
  (defun foo (x) (return "Old behavior: "))
  (patch foo (return "New patched behavior: "))
  (let (v (call foo "test")) (print v))
)
```

### Implicit Context Threading

Zero can automatically inject context variables into function calls within a specific block, reducing the cognitive load for AIs to remember to thread variables like `req`, `db`, or `ctx`. Inside `(with_context (db) ...)` below, `(call fetch_user "123")` is automatically expanded to `(call fetch_user db "123")`:

```lisp
(cli_app
  ;; db is captured by with_context below, so callers never pass it explicitly
  (defun fetch_user (db user_id)
    (type_hint user_id "string")
    (type_hint return "string")
    (return (+ "Fetched user " (+ user_id (+ " from " db))))
  )
  (let (db "PostgreSQL")
    (with_context (db)
      (print (call fetch_user "123"))
    )
  )
)
```

### Teleological Execution

Zero supports goal-driven execution where the runtime dynamically searches for a solution rather than relying on imperative steps. You can specify a target state and a constraint, and Zero delegates execution to a local LLM via the `achieve` node.

```lisp
(cli_app
  (let (result (achieve (is_sorted list) (using "quick sort algorithm")))
    (print result)
  )
)
```

### Auto-Mutating Runtime

The `optimize_block` primitive enables Zero to be self-rewriting. It monitors execution metrics and automatically employs an LLM to rewrite and hot-swap its underlying Go implementation at runtime (via Go plugins) if performance bottlenecks are detected.

```lisp
(cli_app
  (optimize_block "heavy_computation" 500
    ;; If this block takes > 500ms, Zero will generate and load an optimized CGo/Go plugin in the background
    (let (result (call expensive_operation))
      (print result)
    )
  )
)
```

### Ephemeral Neural Circuits

Using `ephemeral_circuit`, Zero can provision a narrowly specialized micro-model dynamically on the fly, configured with a system prompt for a single task. Once the inputs are evaluated, the model is immediately torn down to release resources.

```lisp
(cli_app
  (let (ans (ephemeral_circuit (list "hello" "world") "Translate to French"))
    (print ans)
  )
)
```

### Neural Circuits

The `neural_circuit` primitive lets developers write simple logic specifications as natural language instructions. At runtime, Zero fetches the logic from a local LLM and executes it seamlessly as part of the execution flow.

```lisp
(cli_app
  (let (sorted_list (neural_circuit (list "b" "c" "a") "sort list alphabetically"))
    (print sorted_list)
  )
)
```

### Semantic Routing

With `semantic_match`, you can route execution based on semantic intent rather than brittle regexes or exact string matching. It dynamically evaluates the user input against a set of constraints using a local LLM, then executes the corresponding branch.

### Automated Counterfactual Debugging

When Zero encounters a crash, its agentic observability layer can capture a crash dump and utilize an LLM to reason counterfactually about what input or state would not have crashed. The runtime automatically feeds this back into an auto-patching loop, generating self-healing code that prevents the crash from recurring.

## How to Run

1. **Transpile and Run in one step**:
   To immediately transpile your `.zero` code into Go and execute it, run:
   ```bash
   go run zero.go hello.zero && go run server.go
   ```

2. **Build a Standalone Binary**:
   If you want to compile the transpiled Go code into a highly optimized, standalone binary:
   ```bash
   # 1. Transpile to server.go
   go run zero.go hello.zero
   
   # 2. Compile Go into an executable
   go build -o hello server.go
   
   # 3. Run the binary
   ./hello
   ```

3. **Interpret Directly (no Go compilation step)**:
   For a `cli_app` script using a supported subset of the language (control flow, functions, math/string/collection ops — see `docs/direct_execution_design.md` for the exact coverage), `-run` executes the script's AST directly, with no `server.go` ever written and no `go build`/`go run` invoked:
   ```bash
   go run zero.go -run cli_hello.zero
   ```

4. **Emit a portable WebAssembly prototype**:
   A `wasm_app` emits `app.wat`, a standards-compliant WebAssembly Text module
   with an exported `main(): i32`. The prototype intentionally supports only
   integer literals, arithmetic/comparisons, `if` with both branches, `do`,
   and `return`; I/O, strings, variables, functions, and collection primitives
   are rejected rather than silently miscompiled. Compile `app.wat` with a
   WAT toolchain when one is available.
   ```bash
   go run zero.go -o build examples/wasm_math.zero
   ```
   This is Phase 1 of the "bypass text-based codegen entirely" end goal below — see [Project Roadmap](#project-roadmap--the-end-goal). `http_server`/`web_app` scripts and a handful of primitives that depend on `try_let` (`read_file`, `write_file`, `db_connect`, etc.) aren't supported under `-run` yet and produce a clear error naming the unsupported node.

The server will spin up on `http://localhost:8080`.

## Generating Code with AI (Orchestrator)

Zero is designed to be written by an AI. We provide an orchestrator script (`orchestrator.py`) that handles the interaction with the LLM, strictly enforces syntax boundaries using `outlines`, and handles error feedback loops.

1. Ensure Ollama is running (`ollama serve`) and the `llama3` model is available.
2. Open `orchestrator.py` and modify the `prompt` variable to instruct the AI on what to build.
   ```python
   prompt = "Build a web server on port 8080 with a root route returning 'root' and an /api route returning 'api'."
   ```
3. Run the orchestrator:
   ```bash
   python orchestrator.py
   ```
4. The AI will generate a `.zero` file (by default `app.zero`). The orchestrator will automatically run the Go transpiler.
5. **Self-Correction loop**: If the transpiler encounters a semantic error (e.g. invalid arguments or missing variables), it outputs a localized JSON error. The orchestrator intercepts this error and sends it back to the AI for automatic self-correction.
6. Once transpilation succeeds, the orchestrator compiles the Go binary and executes the newly generated application.

### Automation and Advanced Control Flow

Zero has native support for file operations, subprocess execution, advanced loops (`while`, `match`), and string manipulation for easy automation scripting and state-machine building:

```lisp
(cli_app
  (write_file "hello.txt" "Hello from Zero!")
  (try_let (content (read_file "hello.txt"))
    (catch err (print err))
    (print content)
  )
  (exec "rm" "hello.txt")
)
```

### Native Unit Test Blocks

Zero supports Test-Driven Development natively. You can include `(test "description" ...)` blocks in your code, which the transpiler will extract and convert directly into Go test functions (`_test.go`). This allows AIs to iterate rapidly with test-driven workflows:

```lisp
(cli_app
  (defun add (a b)
    (type_hint a "int")
    (type_hint b "int")
    (type_hint return "int")
    (return (+ a b))
  )

  (test "add function returns correct sum"
    (let (result (call add 2 3))
      (if (!= result 5)
        (print "Error: expected 5 got" result)
      )
    )
  )
)
```

> Note: as of 2026-07-23, `return` supports inline compound expressions like `(return (+ a b))` and `(return (call f x))` directly (bug #13, fixed) — no need to bind through a `let` first. Single-branch `if` with no `else`, shown above, was fixed as bug #16. Void functions are supported using `(type_hint return "void")` (bug #24, fixed). As of 2026-07-24, `if`/`while` conditions also support `and`/`or` and nested arithmetic (e.g. `(if (and (> a 1) (< a 10)) ...)`), fixed as bug #18. See `bugs.md` for current status.

### Database & Persistence

Zero provides native primitives `db_connect` and `sql_query` for managing database connections and executing SQL statements. They transpile directly to Go's standard `database/sql` package calls.

```lisp
(cli_app
  ;; Note: Live database connections require importing a Go driver (e.g. (import "github.com/lib/pq"))
  (db_connect db "postgres" "host=localhost dbname=test")
  (sql_query db "SELECT 1")
)
```

### Modularity

Zero supports importing standard Go packages with `import` and composable file modularity with `include`. An `(include "filename.zero")` block splices module route definitions and functions into the host file at transpile time.

```lisp
(http_server 8080
  (import "strings")
  (include "routes.zero")
  (route "/" (lambda (req)
    (let (msg (call strings.ToUpper "welcome"))
      (res 200 "text/plain" msg)
    )
  ))
)
```

### Collections & Mutability

In-memory slices and dictionaries can be mutated directly using `append` (for appending list items), `map_set` (for assigning dictionary key-value pairs), and `map_delete` (for removing dictionary keys). Values can be read back out with `map_get` (returns the Go zero value, `""`, on a missing key) and `list_get` (bounds-checked, returns `""` on an out-of-range index rather than panicking).

```lisp
(cli_app
  (let (my_list (list "1" "2" "3"))
    (do
      (append my_list "4")
      (print "List:" my_list)
      (print "Second item:" (list_get my_list 1))
    )
  )
  (let (my_dict (dict ("a" "1") ("b" "2")))
    (do
      (map_set my_dict "c" "3")
      (map_delete my_dict "a")
      (print "Dict:" my_dict)
      (print "Value of b:" (map_get my_dict "b"))
    )
  )
)
```

### String Manipulation & Regex

Zero includes string utilities for splitting, joining, and pattern matching text via `str_split`, `str_join`, and `regex_match` (which transpiles to Go's `regexp.MatchString`).

```lisp
(cli_app
  (let (joined (str_join (str_split "hello world" " ") "-"))
    (print "Joined string:" joined)
  )
  (try_let (matched (regex_match "^[a-z]+$" "hello"))
    (catch err (print "Regex error:" err))
    (print "Regex matched:" matched)
  )
)
```

### Type Conversion

Zero provides deterministic primitives for type casting, useful when reading unstructured strings from file I/O or CLI arguments: `to_int`, `to_float`, `to_string`, and `bytes_to_string`.

```lisp
(cli_app
  (try_let (num (to_int "42"))
    (catch err (print "Error:" err))
    (print (+ num 1)) ;; Outputs 43
  )
  
  ;; bytes_to_string is especially useful with read_file which returns []byte
  (try_let (content (read_file "config.txt"))
    (catch err (print "IO error:" err))
    (print "File says:" (bytes_to_string content))
  )
)
```

### Security & Auth Middleware

HTTP servers can intercept and protect routes using `middleware` blocks that read environment variables via `env` and call `(next)` to pass execution down the handler stack.

```lisp
(http_server 8080
  (middleware (lambda (mreq)
    (let (token (env "API_TOKEN"))
      (if (= token "secret-key")
        (next)
        (res 403 "text/plain" "Forbidden")
      )
    )
  )
    (route "/protected" (lambda (req)
      (res 200 "text/plain" "Welcome to the protected route!")
    ))
  )
)
```

### Concurrency, Networking & Control Flow

Zero provides primitives for asynchronous background execution (`spawn`), HTTP requests (`fetch`), request rate limiting (`rate_limit`), automatic retry policies (`retry`), and value matching (`match`).

```lisp
(cli_app
  (spawn (lambda ()
    (print "Background task running")
  ))

  (try_let (body (fetch "https://example.com" "GET"))
    (catch err (print "Fetch error:" err))
    (print "Fetched response bytes")
  )

  (rate_limit "10/s"
    (print "Rate-limited action")
  )

  (retry 3
    (print "Retrying action")
  )

  (let (status 200)
    (match status
      (200 (print "Success"))
      (404 (print "Not Found"))
      (default (print "Unknown status"))
    )
  )
)
```

### Swarm Primitives

Zero supports autonomous subagents as first-class concurrency objects. You can orchestrate a swarm of agents using the `spawn_agent` and `task` primitives.

```lisp
(cli_app
  (spawn_agent "Researcher" (task "find sources on quantum computing"))
  (spawn_agent "Writer" (task "summarize the findings"))
)
```

### Typed Structs & Field Access

In addition to struct declarations, Zero allows parsing JSON payloads into typed struct instances and accessing their fields directly using dot notation (`instance.Field`).

```lisp
(http_server 8080
  (struct UserPayload (Name string) (Role string))
  (route "/user" (lambda (req)
    (try_let (user (parse_json UserPayload req.body))
      (catch err (res 400 "text/plain" "Invalid JSON"))
      (do
        (print "User:" user.Name "Role:" user.Role)
        (res_json 200 user)
      )
    )
  ))
)
```

### Type Parameters (Go Generics)

`(type_param T)` inside a `defun`, combined with `type_hint`, generates a real Go generic function (`func name[T any](...)`) instead of falling back to `any` and runtime type assertions.

```lisp
(cli_app
  (defun identity (x)
    (type_param T)
    (type_hint x T)
    (type_hint return T)
    (return x)
  )
  (test "Generics work"
    (let (res (call identity "hello"))
      (if (!= res "hello")
        (print "failed")
      )
    )
  )
)
```

### Output Directory

By default the transpiler writes `server.go`/`server_test.go` (or `app.js`/`app.test.js` for `web_app`, see below) into the current directory. Pass `-o <dir>` to write elsewhere — useful for keeping a workspace clean or transpiling multiple `.zero` files without them overwriting each other. Run both commands from the repo root, since generated code imports the local `zero/observer` module by path:

```bash
go run zero.go -o build/ hello.zero
go build -o build/hello build/server.go
```

### Observability: Tracing, Crash Dumps & the Observer Agent

Every generated Go program includes three layers of built-in observability with no extra syntax required:

1. **Native telemetry injection** — the transpiler automatically wraps every `defun`, `route`, `middleware`, and `spawn` block with `defer observer.Trace(...)()`, which logs a JSON `{"event":"enter"/"exit", "func":..., "vars":...}` line per call to `telemetry.jsonl`.
2. **Crash-state serialization** — every generated `main()` wraps execution in a global `recover()`. On an unhandled panic, the error and full stack trace are dumped to `crash.json` before the process exits, so an AI debugging the failure has the exact crash state without needing a human to reproduce it.
3. **Standalone Observer Agent** (`observer.py`) — a Python daemon that tails `telemetry.jsonl` in real time and asks a local LLM (via Ollama) to flag anomalous behavior:
   ```bash
   ollama serve
   python observer.py
   ```
   Run it alongside a Zero-generated binary to get live anomaly flags as the program executes.
4. **Bounded auto-patching** — opt-in patch mode watches a configured
   `crash.json`, asks the local model for one complete replacement of one
   configured `.zero` source, and tests that candidate in an isolated project
   copy. The live source is replaced atomically and the restart command runs
   only after the test command succeeds:
   ```bash
   python observer.py patch \
     --project-root /path/to/project \
     --source app.zero \
     --test-command 'go test ./...' \
     --restart-command 'systemctl --user restart zero-app'
   ```
   Use `{source}` in the test command when it must receive the isolated
   candidate path, for example
   `--test-command './zero {source}'`. Add `--once` to process the current
   crash dump and exit. Source and crash paths are constrained to the project
   root, commands run without a shell, model responses cannot select paths or
   commands, and failed candidates never modify the live source or trigger a
   restart. Patch mode is deliberately disabled unless every required option
   is supplied.

You can also manually inject a trace point mid-function with `(trace var)`, which prints the variable's name, value, and source line — see [Automation and Advanced Control Flow](#automation-and-advanced-control-flow) above.

### Compiling to JavaScript

The same Zero grammar can target the browser instead of Go: use `(web_app ...)` as the root block instead of `(http_server ...)`/`(cli_app ...)`. This unlocks browser-only primitives — `(dom_query selector)`, `(on_event el "event" (lambda (e) body))`, `(set_text el val)`, `(set_attr el name val)` — while reusing every other primitive (`let`, `if`, `for`, `defun`, `fetch`, `try_let`, math/logic operators) unchanged. `(test ...)` blocks compile to a Node.js test file (`node --test`) instead of a Go `_test.go` file.

```lisp
(web_app
  (defun increment (n)
    (return (+ n 1))
  )

  (on_event (dom_query "#btn") "click" (lambda (e)
    (let (count 0)
      (do
        (set count (call increment count))
        (set_text (dom_query "#label") count)
      )
    )
  ))

  (test "increment works"
    (if (!= (call increment 1) 2)
      (print "failed")
    )
  )
)
```

```bash
go run zero.go counter.zero   # writes app.js and app.test.js
node --test app.test.js
```

HTML and CSS are intentionally out of scope — Zero's constrained-grammar/hallucination-reduction pitch targets application *logic*, where LLMs reliably hallucinate; markup and styling don't share that failure mode and are better left native.
