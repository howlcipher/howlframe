# Agent understanding interface

The understanding subcommands (`agent capabilities`, `agent describe`, `agent check --json` and `agent explain`) expose a read-only bytecode discovery slice. Every response from these commands is one JSON object with `schema: "howlframe.agent/v1"`. No command executes the inspected source. The separate [session evidence interface](agent_session_interface.md) implements compiler-check snapshots and verified task recovery. These understanding commands do not implement repair, M6 application generation or measured efficiency improvements.

```sh
howlframe agent capabilities
howlframe agent capabilities --target wasmgc
howlframe agent describe print --target bytecode
howlframe agent describe read_file
howlframe agent check --json app.howl
howlframe agent check --json -- -source.howl
howlframe agent explain ARITY_ERROR
```

Arguments may put `--target TARGET` or `--target=TARGET` before or after the positional argument. `--` terminates option parsing. Unknown options, duplicate options, extra operands and missing operands fail with `AGENT_ARGUMENT`; `check` requires `--json`, while other commands always produce JSON and do not accept that flag. Exit status is 0 on success and 1 on an error. Target values are consumed literally: for example, `--target --json` is an unrecognized target rather than a missing-value diagnostic. Target validation precedes source access.

## Envelope and coverage

The stable envelope contains `schema`, `command`, `target`, `coverage`, `ok`, `actual_grants`, `limitations`, `constructs`, `examples`, `compiler_notes` and `diagnostics`. Arrays are always arrays, including when empty. There are no timestamps, temporary paths or compiler installation paths in successful output. Registry ordering and capability-site ordering follow their authoritative deterministic registries. Diagnostics retain meaningful compiler source locations and messages; therefore different source paths can produce different output.

The catalog recognizes `bytecode` and its `bc` alias, canonicalized to `bytecode`. The known backend names `go`, `js`, `javascript`, `wasm`, `wasmgc` and `interpreter` return `coverage: "unknown"` and empty constructs/examples from `capabilities`. This says nothing about those backends' implementation support. `describe` and `check` reject these targets with `AGENT_COVERAGE_UNKNOWN`. A misspelled/unrecognized target fails with `AGENT_UNKNOWN_TARGET`.

`actual_grants` is always `[]`: inspection supplies no runner grants and does not read authority from the caller's desired capabilities. Catalog availability never authorizes effects. The check command may read source, includes and modules through the existing compiler. It does not provide a filesystem sandbox for compilation.

## Constructs and signatures

`capabilities` lists every entry from `internal/construct.Table()`, including `Supported`, `CompileTimeOnly` and `Unsupported`. `describe NAME` returns exactly one matching entry, or `AGENT_UNKNOWN_CONSTRUCT`. Each entry contains `name`, `support`, `note`, `arity`, `signature_coverage` and `requirement`, plus `arity_exemption` when applicable. `arity` is either `{ "min": N, "max": N }` or null; `max: -1` means variadic. Counts exclude the head symbol. The authoritative exemption reason explains null arity.

These are argument counts, not full typed signatures: parameter names, parameter types and return types remain unknown. In particular, `print` takes zero or more arguments; it is not a fixed one-argument operation. A construct marked `Supported` still requires valid syntax, context and operands. Support does not promise every program containing it will compile.

`requirement` reuses `internal/capability.ForConstruct` and explicitly states when metadata records no requirement, rather than claiming proven purity. Outbound `fetch`/`http_request` require host-scoped `network:fetch=host[:port]`; coarse network authority is insufficient. `exec` requires an exact `process:exec=<absolute program path>[ <pattern>...]` grant; coarse process authority is insufficient. `store_open` additionally requires filesystem authority for file-backed URIs. For example, sugar such as `read_text`/`read_json` and nested `task` bodies can have effects absent from their construct metadata. This construct metadata is partial; the compiled artifact's conservative requirements are more specific.

## Production check and diagnostics

`check --json` resolves the source to an absolute path and reexecutes the current compiler's `build --target bytecode`, writing its artifact into an owned temporary directory removed on return. This preserves the production parse, module/include handling, arity, checker, HFIR and bytecode compilation gates. It never invokes `run`. Success has `ok: true`, empty diagnostics and a `requirements` object from `bytecode.RequiredCapabilities`, containing sorted `capabilities` and instruction `sites`. This is a conservative requirement inventory, not a grant or a guarantee that runtime effects will succeed.

Compilation has a 30-second wall-clock limit and each captured output stream is bounded to 1 MiB. Timeout fails with `AGENT_CHECK_TIMEOUT`. Source lookup failures use `AGENT_SOURCE_IO`; temporary-scope and artifact inspection failures use `AGENT_CHECK_INTERNAL`. Compiler launch failures and other unstructured compilation failures use `AGENT_CHECK_FAILED`. This slice is a local compiler interface, not an untrusted-source resource sandbox.

Each normalized diagnostic contains `code`, `message`, `filename`, `line`, `column` and `classification`. Existing structured HFIR codes and locations are preserved as reported by the compiler; this does not certify complete imported-source provenance. Legacy JSON diagnostics preserve reason and line/column, but set filename to the empty string because the compiler does not identify the originating source file (which can be an imported module); an existing leading uppercase `CODE:` is retained with classification `compiler`. Otherwise `AGENT_CHECK_FAILED` and classification `legacy-uncoded` explicitly identify missing coded coverage. Successful compilation retains extra stdout/stderr lines in `compiler_notes`, removing only the exact artifact success line and replacing owned temporary-directory names. Truncated successful output fails closed. Unstructured/mixed output (including include warnings followed by a JSON error) is not accepted as structured JSON and uses a stable interface fallback. No synthetic parser/type diagnostic taxonomy is claimed. Errors have `ok: false`; source locations that do not exist use zero line/column.

`explain CODE` has intentionally partial coverage: interface codes plus `ARITY_ERROR` and `HFIR_TARGET_INFEASIBLE`. A known code returns `explanation`; unknown codes return `AGENT_UNKNOWN_CODE`. A compiler diagnostic can therefore be valid even if this slice cannot explain its code. Explanations do not certify a proposed source edit.

## Executable example

The catalog advertises only the deterministic `pure-add` example for bytecode:

```howl
(cli_app (print (+ 20 22)))
```

Its expected stdout is exactly `42\n`. The CLI regression builds the compiler with the supported file-explicit invocation `go build howlframe.go`, checks this source, builds its bytecode and runs that artifact without `--allow-caps`. It also verifies that checking a filesystem-writing program inventories requirements without writing the requested file. These tests establish the example's behavior; they do not establish model token savings or other backend support.
