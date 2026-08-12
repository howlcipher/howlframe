# HowlFrame Milestone 0 Construct Coverage Matrix

> **Auto-generated** by `cmd/codegen` from the `internal/construct` registry. Do not edit manually.

| Construct | Checker | HFIR | Verifier | Bytecode | VM | Interpreter | Capability | Tests | Status | Gap | Tracker |
|---|---|---|---|---|---|---|---|---|---|---|---|
| `!=` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | compileNode binop case | - |
| `*` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | compileNode binop case | - |
| `+` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | compileNode binop case | - |
| `-` | Yes | Yes | Yes | Yes | Yes | No | None | No | Supported | compileNode binop case | - |
| `/` | Yes | Yes | Yes | Yes | Yes | No | None | No | Supported | compileNode binop case | - |
| `<` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | compileNode binop case | - |
| `<=` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | compileNode binop case | - |
| `=` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | compileNode binop case, normalized to == | - |
| `==` | Yes | Yes | Yes | Yes | Yes | No | None | No | Supported | compileNode binop case | - |
| `>` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | compileNode binop case | - |
| `>=` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | compileNode binop case | - |
| `achieve` | Yes | Yes | Yes | Yes | Yes | No | network | Yes | Supported | ACHIEVE; operands are stringified via ast.Stringify, not compiled | - |
| `and` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | compileNode binop case | - |
| `append` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | APPEND | - |
| `assert_semantic` | No | No | Yes | No | No | No | None | Yes | Unsupported | LLM intent assertion; no opcode, Go backend only | - |
| `bytes_to_string` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | CONVERT | - |
| `call` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | CALL | - |
| `cli_app` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | compiles every child in sequence | - |
| `cli_args` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | CLI_ARGS / CLI_ARGS_GET | - |
| `confidence` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | CONFIDENCE | - |
| `db_connect` | Yes | Yes | Yes | Yes | Yes | No | database | Yes | Supported | DB_CONNECT; reads child .Value directly | - |
| `defun` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | registers a BCFunction; the name and parameter list are structural | - |
| `dict` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | MAKE_DICT; key/value pairs are destructured by the case | - |
| `do` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | compiles every child in sequence | - |
| `env` | Yes | Yes | Yes | Yes | Yes | No | environment | Yes | Supported | ENV | - |
| `ephemeral_circuit` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | EPHEMERAL_CIRCUIT | - |
| `exec` | Yes | Yes | Yes | Yes | Yes | No | process | Yes | Supported | EXEC | - |
| `exit` | Yes | Yes | Yes | Yes | Yes | No | None | No | Supported | EXIT | - |
| `export` | Yes | Yes | Yes | N/A | N/A | N/A | None | Yes | CompileTimeOnly | unwrapped by ast.ResolveModules inside a top-level (module ...) child before checker.Check or lowering ever run (improvements.md #95); a surviving export means the source is malformed and fails earlier, in checker.Check | - |
| `fetch` | No | Yes | Yes | Yes | Yes | No | network | No | Supported | FETCH | - |
| `for` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | FOR_INIT / FOR_NEXT | - |
| `fuzzy_cast` | Yes | No | Yes | No | No | No | None | Yes | Unsupported | LLM struct coercion; no opcode, Go backend only | - |
| `go_import` | No | No | Yes | No | No | No | None | Yes | Unsupported | Go-backend-only directive; the standalone VM cannot honour a Go package dependency | - |
| `http_server` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | HTTP_SERVER_START / HTTP_ROUTE / HTTP_SERVER_SERVE | - |
| `if` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | JUMP_IF_FALSE / JUMP | - |
| `import` | Yes | No | Yes | No | No | No | None | No | Unsupported | Go-backend-only directive; same reasoning as go_import | - |
| `include` | No | Yes | Yes | N/A | N/A | N/A | None | No | CompileTimeOnly | expanded by parser.ExpandIncludes before lowering | - |
| `is_nil` | No | Yes | Yes | Yes | Yes | No | None | No | Supported | IS_NIL | - |
| `lazy_synthesize` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | registers a function; children are name, params and docstring | - |
| `let` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | STORE_VAR | - |
| `list` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | MAKE_LIST | - |
| `list_get` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | LIST_GET | - |
| `list_len` | No | Yes | Yes | Yes | Yes | No | None | No | Supported | LIST_LEN | - |
| `llm_generate` | No | Yes | Yes | Yes | Yes | No | network | Yes | Supported | LLM_GENERATE | - |
| `map_delete` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | MAP_DELETE | - |
| `map_get` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | MAP_GET | - |
| `map_set` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | MAP_SET | - |
| `match` | Yes | No | Yes | No | No | No | None | Yes | Unsupported | no opcode; the interpreter also fails closed on it (internal/vm/vm.go) | - |
| `middleware` | No | No | Yes | No | No | No | None | Yes | Unsupported | HTTP middleware chain; no opcode, Go backend only | - |
| `mkdir` | Yes | Yes | Yes | Yes | Yes | No | filesystem | No | Supported | MKDIR | - |
| `module` | Yes | Yes | Yes | N/A | N/A | N/A | None | Yes | CompileTimeOnly | parser.ExpandIncludes/ast.ResolveModules consume (module ...) children of the root before checker.Check or lowering ever run (improvements.md #95); a root that is itself a module (no importer) is a malformed standalone program and fails earlier, in checker.Check | - |
| `neural_circuit` | Yes | Yes | Yes | Yes | Yes | No | None | No | Supported | NEURAL_CIRCUIT | - |
| `optimize_block` | No | Yes | Yes | Yes | Yes | No | None | No | Supported | compiles children[3:] | - |
| `optimize_signature` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | compiles the final child; the preceding metric/test/candidate forms are metadata | - |
| `or` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | compileNode binop case | - |
| `parse_json` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | PARSE_JSON; reads child .Value directly | - |
| `patch` | No | Yes | Yes | N/A | N/A | N/A | None | Yes | CompileTimeOnly | applied and removed by ast.ApplyPatches before lowering | - |
| `print` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | PRINT | - |
| `read_file` | Yes | Yes | Yes | Yes | Yes | No | filesystem | Yes | Supported | READ_FILE | - |
| `read_line` | Yes | Yes | Yes | Yes | Yes | No | None | No | Supported | READ_LINE | - |
| `regex_match` | Yes | Yes | Yes | Yes | Yes | No | None | No | Supported | REGEX_MATCH | - |
| `res` | No | Yes | Yes | Yes | Yes | No | network | Yes | Supported | RES | - |
| `res_json` | Yes | Yes | Yes | Yes | Yes | No | network | Yes | Supported | RES_JSON | - |
| `return` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | RETURN | - |
| `schema` | Yes | No | Yes | No | No | No | None | Yes | Unsupported | declarative SQL migration; no opcode, Go backend only | - |
| `schema_bridge` | Yes | Yes | Yes | Yes | Yes | No | None | No | Supported | compiles the wrapped source expression | - |
| `semantic_match` | Yes | No | Yes | No | No | No | None | No | Unsupported | LLM intent dispatch; no opcode, Go backend only | - |
| `set` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | SET_VAR | - |
| `sleep` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | SLEEP | - |
| `spawn` | Yes | Yes | Yes | Yes | Yes | No | process | Yes | Supported | SPAWN | - |
| `spawn_agent` | No | Yes | Yes | Yes | Yes | No | process | Yes | Supported | SPAWN_AGENT | - |
| `sql_query` | Yes | Yes | Yes | Yes | Yes | No | database | Yes | Supported | SQL_QUERY; reads child .Value directly | - |
| `stderr` | Yes | Yes | Yes | Yes | Yes | No | None | No | Supported | STDERR | - |
| `store_delete` | Yes | Yes | Yes | Yes | Yes | No | database | Yes | Supported | STORE_DELETE | - |
| `store_get` | Yes | Yes | Yes | Yes | Yes | No | database | Yes | Supported | STORE_GET | - |
| `store_open` | Yes | Yes | Yes | Yes | Yes | No | database | Yes | Supported | STORE_OPEN; handle and URI are read as literals | - |
| `store_put` | Yes | Yes | Yes | Yes | Yes | No | database | Yes | Supported | STORE_PUT | - |
| `str_join` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | STR_JOIN | - |
| `str_split` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | STR_SPLIT | - |
| `struct` | Yes | No | Yes | No | No | No | None | Yes | Unsupported | the VM has no struct representation; accepting the declaration would hide the gap | - |
| `task` | No | Yes | Yes | Yes | Yes | No | None | Yes | Supported | TASK; reads child .Value directly | - |
| `test` | Yes | No | Yes | No | No | No | None | Yes | Unsupported | test blocks are extracted into Go _test.go output; the VM cannot run them | improvements.md #96 |
| `to_float` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | CONVERT | - |
| `to_int` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | CONVERT | - |
| `to_string` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | CONVERT | - |
| `trace` | No | No | Yes | No | No | No | None | Yes | Unsupported | auto-tracing macro expanded by the Go backend; no opcode | - |
| `try_let` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | TRY_LET | - |
| `type_hint` | Yes | Yes | Yes | N/A | N/A | N/A | None | Yes | CompileTimeOnly | deliberate empty case in compileNode; a pure annotation | - |
| `type_hints` | Yes | Yes | Yes | N/A | N/A | N/A | None | Yes | CompileTimeOnly | pure annotation; children are type names, not code | - |
| `type_param` | No | Yes | Yes | N/A | N/A | N/A | None | Yes | CompileTimeOnly | generic parameter annotation; children are type names, not code | - |
| `use` | No | Yes | Yes | N/A | N/A | N/A | None | Yes | CompileTimeOnly | resolved by parser.ExpandIncludes/ast.ResolveModules before checker.Check or lowering ever run (improvements.md #95); nested use inside an already-used module is rejected at parse time instead | - |
| `wasm_app` | Yes | No | Yes | No | No | No | None | No | Unsupported | a Wasm program root, not a bytecode program root | - |
| `web_app` | Yes | No | Yes | No | No | No | None | Yes | Unsupported | a JavaScript program root, not a bytecode program root | - |
| `while` | Yes | Yes | Yes | Yes | Yes | No | None | Yes | Supported | JUMP_IF_FALSE / JUMP | - |
| `with_context` | No | Yes | Yes | N/A | N/A | N/A | None | Yes | CompileTimeOnly | eliminated by ast.ApplyWithContext before lowering; its variable list is structural, not code | - |
| `write_file` | Yes | Yes | Yes | Yes | Yes | No | filesystem | Yes | Supported | WRITE_FILE | - |
