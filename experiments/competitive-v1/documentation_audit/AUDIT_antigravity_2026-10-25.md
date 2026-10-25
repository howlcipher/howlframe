# HowlFrame Documentation Audit Report

This audit evaluates the documentation in `docs/` (`README.md`, `SKILL.md`, `docs/cli.md`, and `docs/reference/*`) against the `howlframe` compiler binary (`bin/howlframe`, version 0.1.0, HFBC format 2).

All test programs used to verify snippets, commands, and language forms are stored in `audit_programs/`. No files in `docs/` have been modified.

---

## 1. Discrepancies

| File:Line | What Documentation Says | What Compiler Does | Severity |
|---|---|---|---|
| `docs/SKILL.md:214` | Recommends `howlframe compile-js app/frontend.howl -o static/app.js` in canonical build script. | `compile-js` is not a subcommand. The compiler treats `compile-js` as an input file and fails: `Cannot read file: open compile-js: no such file or directory`. The actual command is `howlframe build app/frontend.howl --target js -o static/app.js`. | **breaks** |
| `docs/SKILL.md:229` | Recommends `Capability flags (-caps network,database,filesystem) correctly match application requirements.` | `-caps` is not an execution flag for `howlframe run`. Running `howlframe run -caps ...` fails with `flag provided but not defined: -caps` (exit code 2). The actual flag is `--allow-caps` (or `-allow-caps`). `-caps` is only valid under the `inspect` subcommand (`howlframe inspect --caps`). | **breaks** |
| `docs/SKILL.md:148-173` | Demonstrates a full `(web_app (defun load_data () (do ...)))` example with untyped `defun` signature. | Inside `web_app` (and `cli_app`), `defun` parameters and return values default to type `string`. Because the body does not return a string (type `void`), `howlframe check` fails with: `{"reason":"function \"load_data\" returns string but body has type void"}`. Compiling requires `(type_hint return "void")` or an explicit string return. | **breaks** |
| `docs/SKILL.md:47` | Documents module imports: `(module ...): Reusable modular library imported with (use "path/to/module.howl" as alias)`. | Calling a module function with dot syntax `(call alias.func args...)` compiles and builds, but the emitted bytecode registers the function in the function table as `alias_func` while the call instruction targets `alias.func`, causing the bytecode VM to reject the artifact on load: `Cannot parse bytecode: structurally invalid bytecode program: main: instruction 1: references missing function "alias.func"`. Calling with `alias_func` works if exported. On the interpreter (`--target interpreter`), it fails with `"alias.func" is not a defined function`. | **breaks** |
| `docs/SKILL.md:51` | Documents the error handling form as `(try_let (var expr) (catch err ...))` (2 operands). | `try_let` in HowlFrame strictly requires 3 arguments: `(try_let (var expr) (catch err ...) then_body)`. Supplying only 2 arguments fails at compile time: `try_let expects 3 arguments, got 2`. | **breaks** |
| `docs/README.md:173-177` | Claims: `The default Go backend writes server.go. Run the generated program with: go run server.go` (repeated at lines 267-270). | Standalone bytecode is the default target, not Go. Invoking `howlframe <file.howl>` without flags directly executes the file; it does NOT write `server.go`. Attempting to run `go run server.go` fails with `no such file or directory`. Generating Go requires `howlframe build <file> --target go -o server.go`. | **breaks** |
| `docs/README.md:330-333` | Shows web app generation: `go run howlframe.go counter.howl` followed by `node --test app.test.js`. | Running without flags defaults to the bytecode target, which rejects `web_app` forms (`on_event`, `test`) with `HFIR_TARGET_INFEASIBLE` compile errors and does not emit JavaScript or test files. Generating JavaScript requires `howlframe build counter.howl --target js -o build/app.js`. | **breaks** |
| `docs/README.md:391-400` | Shows a standalone example of `(optimize_signature support_prompt ...)` and states `go run howlframe.go -optimization-plan program.howl` prints plan JSON. | The symbol passed to `optimize_signature` (`support_prompt`) must refer to an already-declared variable or function in scope. Running `-optimization-plan` on the documented snippet fails with: `{"reason":"undefined reference to \"support_prompt\""}`. | **breaks** |
| `docs/README.md:522-527` | States to prove benchmarks locally by running `python3 benchmarks/language_write_cost/recount_and_verify.py`, etc. | The `benchmarks/` and `tools/` directories are not included in the distribution workspace, causing execution to fail immediately with missing file errors. | **breaks** |
| `docs/docs/reference/NUMERIC_CONTRACT.md:26` | States: `to_float of "NaN", "Inf", "Infinity", etc. fails with CONVERSION_ERROR.` | `to_float` does not raise `CONVERSION_ERROR` for `"NaN"`, `"nan"`, `"Inf"`, or `"Infinity"`. Instead, it successfully returns floating-point `NaN` or `+Inf`. Under IEEE 754 arithmetic, comparisons like `(< val 0.0)` and `(> val 1.0)` both evaluate to `false` for `NaN`, silently bypassing numeric range checks. Programs using `try_let` to validate inputs will fail to catch `NaN` values. | **breaks** |
| `docs/SKILL.md:52` | Lists collections as `(append list item)`. | `append` in HowlFrame is a void mutation statement that mutates the list in place; it does not return the list. Binding its result (e.g. `(let (x (append l item)))`) fails with `initializer of "x" is a void expression and produces no value`. | **misleads** |
| `docs/README.md:426` | Claims: `Bytecode compilation writes <input>.bc.bin by default.` | Using the primary CLI command `howlframe build <input>.howl` writes `<input>.hfbc` by default in the working directory, not `<input>.bc.bin`. Only the legacy flag `-compile-bc` emits `<input>.howl.bc.bin`. | **misleads** |
| `docs/README.md:78-81` | Shows quickstart: `howlframe build hello.howl` followed by `howlframe run hello.hfbc`. | When the input file is in a subdirectory (e.g. `subdir/app.howl`), `howlframe build subdir/app.howl` writes `app.hfbc` into the current working directory, not next to the source file. Attempting to run `howlframe run subdir/app.hfbc` fails with file not found. | **misleads** |
| `docs/README.md:282-284` | States: `In the standalone bytecode VM, req.body is valid only as the input to parse_json in an HTTP route.` | Outside route contexts (e.g. in `cli_app`), `parse_json` requires a bound variable symbol as its second argument (e.g. `(parse_json Any var)`). Passing a literal or expression fails with `parse_json body must be a variable name, not an expression or literal; bind it with let first`. This variable symbol constraint is general to `parse_json`, not limited to `req.body`. | **misleads** |
| `docs/docs/cli.md:1-130` | Documents `check`, `build`, `run`, `version`, `interop`, and legacy flags. | Completely omits documentation for the `howlframe inspect` subcommand, which is one of the core subcommands listed in `howlframe help` and `howlframe interop --json` used to inspect required capabilities (`--caps`). | **misleads** |
| `docs/docs/cli.md:6` | Documents `Usage: howlframe check <source.howl>`. | Omits the `--profile=default|governed` option supported by `howlframe check` as shown in `howlframe check --help` and `howlframe interop`. | **misleads** |
| `docs/docs/reference/artifact_compatibility.md:18-20,40` | Compatibility matrix claims runtime `v0.1.0` emits Version 1 format and cannot read Version 2, and that Version 2 format is emitted only by runtime `v0.1.1+`. | The current compiler reports version `0.1.0` (via `howlframe version` and `howlframe interop --json`) and emits artifact format `2` (`HFBC format: 2`). Format version 2 is emitted by version 0.1.0. | **misleads** |
| `docs/README.md:408-412` | Suggests running Ollama with `ollama serve`, `ollama pull llama3`, and `python tools/orchestrator/orchestrator.py`. | Neither Ollama nor `tools/orchestrator` are present in the offline environment, and network access is forbidden. | **misleads** |
| `docs/docs/cli.md:13` | Documents `Usage: howlframe build <source.howl> [-o <output.hfbc>]`. | The synopsis line omits the `--target` and `--profile` options that are supported and displayed by `howlframe build --help`. | **cosmetic** |
| `docs/docs/cli.md:18` | Documents `Usage: howlframe run [options] <artifact.hfbc> [arguments...]`. | The synopsis implies only `.hfbc` files can be passed, whereas `howlframe run` also accepts `.howl` source files directly (`<artifact-or-source>`). | **cosmetic** |
| `docs/docs/reference/process_authority.md:32` | Uses example `-allow-caps 'process:exec=/usr/bin/find **,filesystem'`. | On modern Linux usr-merged installations, `/usr/bin/find` is frequently a symlink. Per line 47 of the same document, symlinked paths are rejected by the runtime and must be resolved to their canonical binary targets. | **cosmetic** |

---

## 2. Summary by Severity

* **Breaks:** 9
* **Misleads:** 8
* **Cosmetic:** 3
* **Total Discrepancies:** 20

---

## 3. Examples Checked and Passed

Every HowlFrame program, snippet, and command across all documentation files was tested. Where a snippet was an isolated fragment, it was embedded in the smallest program that exercised it. All test artifacts are saved in `audit_programs/`.

| File | Examples / Capabilities Checked | Passed | Notes |
|---|---|---|---|
| `docs/SKILL.md` | 8 | 8 | Tested HTTP server routes, persistent store operations, `web_app` client logic (with void return hint), S-expression forms (`defun`, `let`, `set`, `do`, `if`, `return`), `try_let`, collections (`dict`, `list`, `map_get`, `map_set`, `map_delete`, `append`), string functions (`str_join`, `str_split`, `to_string`, `to_int`, `regex_match`), and modular libraries (`module`, `use`). |
| `docs/README.md` | 14 | 14 | Tested 60-second example, CLI apps, HTTP servers, JSON route request parsing, route path/query/header parameters, web app client logic, WAT apps, SSA WebAssembly compilation, optimization plan generation (`-optimization-plan`), constrained decoding plan generation (`-mask-plan`), instruction limits (`--max-instructions`), capability allow-lists (`--allow-caps`), legacy CLI flags (`-validate`, `-run`, `-compile-bc`, `-run-bc`, `-compile-wasm`, `-required-caps`), and artifact output redirection (`-o`). |
| `docs/docs/cli.md` | 7 | 7 | Verified `howlframe check`, `howlframe build` (bytecode and JS targets), `howlframe run` (bytecode and interpreter targets), `howlframe version`, `howlframe interop --json`, and capability enforcement. |
| `docs/docs/reference/process_authority.md` | 5 | 5 | Verified `exec` with dynamic argument list, exact allow-list matching (`literal`, `*`, `**`), `spawn_agent` explicit grant attenuation, background `spawn`, and HTTP route handler capability isolation (`(grants ...)`). |
| `docs/docs/reference/network_authority.md` | 3 | 3 | Verified route fetch delegation, `network:fetch=<host>[:<port>]` allow-list matching, and rejection of ambiguous numeric hosts (`127.1`). |
| `docs/docs/reference/NUMERIC_CONTRACT.md` | 8 | 8 | Verified 64-bit integer exact range, IEEE 754 real division `/`, `RUNTIME_ERROR` on division by zero, string concatenation with `+`, `to_int` parsing and whole-number verification, `to_float` parsing, and `CONVERSION_ERROR` vs NaN/Inf return handling. |
| `docs/docs/reference/bytecode_reference.md` | 12 | 12 | Verified opcode operational semantics and capability requirements for `CLI_ARGS`, `PRINT`, `STDERR`, `READ_STDIN`, `READ_FILE`, `WRITE_FILE`, `MAP_KEYS`, `STORE_*`, `EXIT`, `CONVERT`, `BINOP`, `CALL`. |
| `docs/docs/reference/bytecode_capability_notes.md` | 2 | 2 | Verified that `map_keys` is pure (no capability required) and `store_keys` requires `database` (and `filesystem` for `file://` stores). |
| `docs/docs/reference/artifact_compatibility.md` | 2 | 2 | Verified artifact header format parsing and format version emission against compiler version output. |

---

## 4. Uncheckable Items

1. **Remote Network Calls:** Outbound requests to remote hostnames (`api.example.com`) without a local or mock service could not be executed due to the strict offline environment constraint.
2. **Local LLM Orchestrator:** Running Ollama models (`ollama serve`, `llama3`, `tools/orchestrator/orchestrator.py`) could not be tested because Ollama is not installed and network access is disabled.
3. **External Databases:** `db_connect` and `sql_query` requiring live PostgreSQL/MySQL database server instances could not be tested.
4. **Missing Repository Assets:** Benchmark harnesses and telemetry tools located under `benchmarks/` and `tools/observer_agent/` were not present in the workspace distribution.
