# Change Log

## Unreleased

### Changed
* A differential dogfood case compiles `write_file` and `mkdir` with experimental `-compile-hfir-bc` and production `-compile-bc`, granted and denied under `filesystem`. The nested fixture places both effects inside `if`, `while`, `for`, and `defun`. The experimental lowerer emits the existing `WRITE_FILE` and `MKDIR` opcodes. Production `-compile-bc` is still AST bytecode. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_dogfood_fs_write.md`.
* Generated Go and JavaScript mediate `(write_file)` and `(mkdir)`. An empty, missing, or non-`filesystem` `HOWLFRAME_ALLOW_CAPS` grant fails with `CAPABILITY_DENIED` before any file write or directory creation, and the denial does not include the path. The `filesystem` grant writes the file and creates the directory. The production `-compile-bc` path is unchanged. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase2e_fs_write.md`.
* A differential dogfood case compiles `tests/conformance/abi_v1/14_nested_for_if_while_defun.howl` with experimental `-compile-hfir-bc` and production `-compile-bc` and requires the same stdout, stderr, and exit. The program nests `for` with `if`, `while`, and `defun`, including a `for` inside a `for`. Production `-compile-bc` is still AST bytecode. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_dogfood_nested_for.md`.
* Experimental `-compile-hfir-bc` compiles and runs `for` with the existing `FOR_INIT`, `FOR_NEXT`, and `JUMP` opcodes. A for header's control edges are the iterable and then the body. Production `-compile-bc` is still AST bytecode after the gate. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase3d_for.md`.
* A differential dogfood case compiles `tests/conformance/abi_v1/12_nested_if_while_defun.howl` with experimental `-compile-hfir-bc` and production `-compile-bc` and requires the same stdout, stderr, and exit. The program nests `if` and `while` inside `defun` and calls that function. Production `-compile-bc` is still AST bytecode. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_dogfood_nested_control.md`.
* Experimental `-compile-hfir-bc` compiles and runs `if` with the existing `JUMP_IF_FALSE` and `JUMP` opcodes. An if node's control edges are the condition, the then-branch, and an optional else. Production `-compile-bc` is still AST bytecode after the gate. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase3c_if.md`.
* Experimental `-compile-hfir-bc` compiles and runs `while` with the existing `JUMP_IF_FALSE` and `JUMP` opcodes. A while header's control edges are the condition and then the body. Production `-compile-bc` is still AST bytecode after the gate. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase3b_while.md`.
* Experimental `-compile-hfir-bc` compiles and runs `defun`, `call`, and `return` with the existing `CALL` and `RETURN` opcodes. Production `-compile-bc` is still AST bytecode after the gate. #90 stays Partial. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase3a_defun_call.md`.
* Generated Go and JavaScript mediate `(fetch)`. An empty, missing, or non-`network` `HOWLFRAME_ALLOW_CAPS` grant fails with `CAPABILITY_DENIED` before any HTTP request, and the denial does not include the URL. The `network` grant performs the request. The production `-compile-bc` path is unchanged. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase2d_fetch.md`.
* Generated Go and JavaScript mediate `(read_file)`. An empty, missing, or non-`filesystem` `HOWLFRAME_ALLOW_CAPS` grant fails with `CAPABILITY_DENIED` before any filesystem read, and the denial does not include the path. The `filesystem` grant reads the file. The production `-compile-bc` path is unchanged. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase2c_read_file.md`.
* Generated Go and JavaScript mediate `(exec)`. An empty, missing, or non-`process` `HOWLFRAME_ALLOW_CAPS` grant fails with `CAPABILITY_DENIED` before a subprocess starts, and the denial does not include the command. The `process` grant runs it. The production `-compile-bc` path is unchanged. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase2b_exec.md`.
* Generated Go and JavaScript mediate `(env)`. An empty or non-`environment` `HOWLFRAME_ALLOW_CAPS` grant fails with `CAPABILITY_DENIED` before the variable is read. The `environment` grant returns the value. The production `-compile-bc` path is unchanged. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase2a_env.md`.
* `map_keys` order is UTF-8 byte order on the interpreter, the bytecode VM, the Go backend, and JavaScript. JavaScript no longer uses UTF-16 code-unit sort, so a supplementary-plane key sorts with the same list as Go `sort.Strings`. No new opcode and no capability change. Journal: `docs/journals/2026-09-30_dict_key_sort.md`.
* Locked the pure-versus-store capability split in docs and tests. `map_keys` grants nothing and runs under an empty grant. `store_keys` stays `database`. A `file://` store additionally requires `filesystem`. No opcode grant changed. Journal: `docs/journals/2026-09-30_capability_surface_honesty.md`.

### Added
* Lowered-HFIR ABI v1 (`docs/reference/lowered_hfir_abi_v1.md`) and a conformance suite (`tests/conformance/lowered_hfir_abi_v1.json`) run by `tools/difftest` across the interpreter, the bytecode VM, Go, and JavaScript. The production compiler is still the AST bytecode path. Journal: `docs/journals/2026-09-30_lowered_hfir_abi_phase1.md`.
* `html_escape` and `attr_escape`, with `HTML_ESCAPE` and `ATTR_ESCAPE`. Both
  encode `&`, `<`, `>`, `"`, and `'` the same way (`&amp;`, `&lt;`, `&gt;`,
  `&#34;`, `&#39;`) and grant no capability. `attr_escape` is the attribute-context
  name for that encoding. A non-string fails closed with `TYPE_ERROR`. An empty
  string stays empty. Contract tests reject a handler body that interpolates an
  untrusted id into a `<script>` string or an inline handler; data attributes
  carry the escaped text and handler names stay constants.
* `req_query`, `req_header`, and `req_path`, with `HTTP_REQ_QUERY`,
  `HTTP_REQ_HEADER`, and `HTTP_REQ_PATH`. A handler can read one query
  parameter, one request header, and one `{name}` path segment. A missing
  name is an empty string. A bad receiver or name is `TYPE_ERROR`. The reads
  grant no capability. Literal routes stay on the existing mux.
* `map_keys` construct and `MAP_KEYS` bytecode instruction. It returns every key
  of a dictionary as a sorted list and requires no capability. HowlBoard could
  not iterate the keys of a record it received, so object-shaped responses had
  to be rewritten as lists. An empty dictionary returns an empty list, and a
  non-dictionary fails closed with `TYPE_ERROR`. The same sorted enumeration is
  available to the interpreter and to the Go and JavaScript backends.
* `time_now` in the JavaScript backend. It was supported by the bytecode VM and
  the Go backend but rejected as an unknown statement for `web_app` programs, so
  a browser interface had no way to read the clock and render relative times.
* `store_keys` construct and `STORE_KEYS` bytecode instruction, returning every
  record key in a native store as a sorted list. Go randomizes map iteration, so
  enumeration is sorted to keep listing deterministic. Every prior HowlFrame
  application (`kv_cli`, `todo_cli`, `task_api`, HowlBoard) had to maintain a
  parallel index record that could silently diverge from the records it indexed;
  `store_keys` removes that workaround. Requires the `database` capability, and
  `filesystem` additionally for `file://` stores.
* Runner-sealed, bounded negative map-state provenance for the internal direct
  HFIR experiment. It records completed map mutations and reads by backing-map
  identity, proves never-present versus effectively deleted keys, and fails
  closed on ambiguous wrong-key writers, forged evidence, or bounded-history
  overflow. Durable HFBC and public AST-backed execution remain unchanged.
* Internal HFIR failure-localization evidence: direct-lowering instruction
  provenance, a runner-bounded execution trace, fail-closed behavioral and
  runtime localization, and an automatically derived repair context for the
  bounded semantic-repair experiment. Durable HFBC artifacts and public
  AST-backed builds remain unchanged.
* An internal Phase-1 `HFIR -> BCProgram` lowering path that consumes semantic
  HFIR directly, with explicit operand-role normalization, deterministic
  differential artifact/VM evidence, capability consistency checks, preserved
  compile diagnostic provenance, and fail-closed
  `HFIR_BYTECODE_UNSUPPORTED` diagnostics. Public bytecode builds continue to
  use the AST compiler while the subset expands.
* `list_len` construct and bytecode instruction `OpListLen` to measure list lengths without manual loops.
* `is_nil` intrinsic to natively detect the absence of a value (e.g., a missing key in a dict) instead of string formatting checks.
* HowlFrame Repo Analyst, a deterministic five-module standalone application that
  discovers and classifies repository files, counts tests, configuration,
  entry points, and TODO/FIXME markers, emits or writes a versioned report, and
  proves its bytecode artifact runs after imported `.howl` sources are removed.
* A typed flat SSA/CFG lowering layer in `internal/ir`, with ordered basic
  blocks, explicit branch/jump/return terminators, phi nodes for control-flow
  joins and loop-carried values, source locations, and graph validation.
* Provider-neutral constrained-decoding mask plans in `internal/masking`,
  derived from semantic checker `TypeInfo` and full `checker.Analysis` output
  for downstream tokenizer/logit integrations.
* A local parser and instruction/type validator for the folded WebAssembly Text
  subset emitted by the Wasm backend, including function results, operands,
  control flow, memory access, constants, and data bounds.
* Semantic diagnostics for incompatible aggregate elements, keys, mutations,
  indexes, function arity, numeric operands, and branch layouts before IR
  lowering.
* Front-loaded shared-form shape diagnostics for malformed control-flow,
  conversion, string, collection, call, binding, and loop forms before backend
  emission.
* OpenAI model recommendations across the bug and improvement backlogs,
  matching the existing Claude/Gemini routing columns with GPT-5.6 Luna,
  Terra, and Sol tiers.
* Bounded, opt-in auto-patching for the observer. It validates a model-proposed
  `.howl` replacement in an isolated project copy, installs it atomically only
  after the configured tests pass, and then runs an explicit restart command.
* Isolated unit and subprocess integration coverage for path confinement,
  malformed model responses, test failure, atomic installation, and restart
  ordering.
* Bytecode-native in-memory store opcodes for structured records:
  `STORE_OPEN`, `STORE_PUT`, `STORE_GET`, and `STORE_DELETE`.
* Compiler and VM regression coverage for store lowering, named store
  attachment, idempotent deletion, missing-record behavior, record-copy
  isolation, and bytecode fixture execution.
* Compile-time optimization signatures through `optimize_signature` and
  deterministic `-optimization-plan` JSON, with checker metadata, transparent
  backend execution, documentation, and regression coverage.
* One authoritative, backend-independent construct-support registry in
  `internal/construct`, classifying every HowlFrame construct as `Supported`,
  `CompileTimeOnly`, or `Unsupported` for the standalone bytecode target, with
  parent-scoped sub-forms and a context-aware AST scan. A drift test parses
  `compileNode`'s own `switch head` so the registry and the compiler cannot
  diverge silently.

### Changed
* Dict values may now mix types. Dicts are the language's record literal, and the
  VM and native store both carry `map[string]any`, so a record combining strings,
  ints, lists, and nested dicts already executed correctly; only the analyzer
  rejected it. Heterogeneous dict literals and `map_set` writes now widen the
  element type to `any` through the existing `join` helper instead of reporting
  `dict value N has type X, want Y`. Key checks, target-kind checks, and list
  element homogeneity are unchanged.

* Documented the existing standalone HTTP JSON request composition
  (`parse_json ... req.body` with `try_let`), its bounded scope, and the
  JavaScript `web_app` compatibility-generation command. Added focused
  bytecode and JavaScript-generator conformance coverage for the external
  consumer request/response flow.
* Completed the HowlFrame identity cutover across the repository, compiler,
  canonical Go module, `.howl` source corpus, `.hfbc` bytecode examples, HFIR
  packages and diagnostics, machine-readable namespaces, documentation,
  website metadata and artwork, benchmarks, tools, and reference application.
* Output directories are now created consistently, and Go, JavaScript, and
  legacy WAT generation accept `-o` before or after the input path.
* Long valid `let` chains now use shared iterative traversal across AST
  preprocessing, semantic checking, and Go/JavaScript emission, with a
  2,000-binding regression covering transpilation and generated Go builds.
* Synchronized the HowlFrame language write-cost benchmark fixtures with the
  published 2026-07-30 token counts for Tasks B and C.
* Integer and string list/dictionary expressions now initialize Wasm linear
  memory with typed stores, dynamic dictionary keys compare interned string
  pointers, integer dictionary reads use typed loads, multiple aggregate
  literals receive independent aligned memory regions, and aggregate tables and
  memory page counts scale with their payloads.
* Refreshed the README and GitHub Pages content to reflect the current
  semantic checker, typed backend metadata, direct binary bytecode, and expanded
  WebAssembly Text backend scope.
* Removed stale transpiler branding and Go-only positioning from the README,
  GitHub Pages landing page, and current public docs.
* Re-scored the remaining self-healing backlog after auto-patching shipped,
  corrected the WebAssembly prototype's WAT scope, and restored the missing
  Ephemeral Neural Circuits detail section.
* `-compile-bc` now accepts `-o <file>` after the input path for exact bytecode
  output files while keeping the existing output-directory behavior.
* `-compile-bc` now fails closed on any construct the bytecode compiler cannot
  lower, reporting a `HFIR_TARGET_INFEASIBLE` diagnostic that names the
  construct, its source location, and the backlog item that owns the gap, and
  writing no artifact. Type annotations and forms consumed by earlier passes
  are classified separately and keep compiling unchanged.

### Fixed
* `for` over an expression no longer silently miscompiles in the JavaScript and
  Go backends. Both read the iterable's raw node value, which is empty for
  anything but a bound symbol, so `(for m (map_get d "missions") ...)` emitted
  `for (let m of )` and `for _, m := range {` - invalid output produced with no
  diagnostic, in a toolchain whose contract is to fail closed.
* `on_event` now terminates its statement. Automatic semicolon insertion does
  not apply before `(`, so any following top-level statement was parsed as a
  call of the `addEventListener` result.
* A `web_app`'s top-level statements are wrapped in an async IIFE. They routinely
  contain awaited calls, and a classic `<script>` has no top-level await, so
  every generated interface failed to parse in the browser. Function
  declarations remain at top level so inline handlers can still reach them as
  globals.
* Route handlers fail closed. A panic inside an `http_server` route handler wrote
  nothing to the `ResponseWriter`, so Go emitted `200` with an empty body and a
  denied capability was indistinguishable from a completed request. Handlers that
  fail before responding now return `500` with the structured `VMError` JSON
  (preserving codes such as `CAPABILITY_DENIED` and `LIMIT_EXCEEDED`), and the
  failure is reported on the VM's error stream rather than process stdout. A
  handler that already committed a response is left untouched.
* `SPAWN_AGENT` and `TASK` opcodes in the standalone bytecode VM now report a
  structured `UNSUPPORTED_CONSTRUCT` runtime error naming the opcode rather than
  panicking with `VM_INTERNAL` as an unknown opcode. Both opcodes are emitted
  by the compiler and registered in the opcode table, but are unimplemented in the
  runtime; reporting them with an explicit unsupported-construct error distinguishes
  known unimplemented constructs from internal VM crashes. For `SPAWN_AGENT`,
  capability checks for `capability.Process` precede the unsupported construct error. Covered by
  `TestVMUnsupportedConstructs` in `internal/vm/vm_negative_test.go` and
  `TestBytecodeRunSwarmFixtureReportsUnsupportedConstruct`, `TestBytecodeRunSwarmFixtureWithoutCapabilities`,
  `TestBytecodeRunWithMalformedOpcode`, and `TestBytecodeRunWithTimeout` in `howlframe_test.go`. (bugs.md #54)
* Bytecode compiler properly extracts typed parameter names in `defun` and
  `lazy_synthesize`, and skips return-type annotations in `defun` body compilation.
  Previously, parameters written with types like `((a int) (b int))` had their names
  collapsed to empty strings in the bytecode function definition, and return-type
  symbols were erroneously emitted as `LOAD_VAR <type>` instructions in the function
  body, causing runtime `undefined variable: <type>` panics. In addition,
  `tools/difftest/main.go` now dynamically verifies that exemptions for `"unsupported in run"`
  are still earned, failing if an exempt fixture passes all backends; the stale exemption
  for `test_improvement_46.howl` in `tools/difftest/manifest.json` was removed and now
  passes cross-backend differential testing. Covered by unit regressions in
  `internal/bytecode/bytecode_test.go` (`TestCompileDefunWithTypedParametersAndReturnType`,
  `TestCompileDefunAdversarialEdgeCases`), `internal/vm/regression_test.go`
  (`TestDefunWithTypedParametersAndReturnType`), and `difftest`. (bugs.md #51)
* Flags written after the positional input file are honoured instead of
  discarded. Go's `flag` package stops parsing at the first non-flag argument,
  so `howlframe prog.howl -mask-plan` set no mode flag at all, fell through to
  the default Go backend, exited 0 with no output and wrote an unrequested
  `server.go` into the working directory. `main` now resumes parsing over the
  trailing tokens with a second `FlagSet` that re-registers every flag sharing
  the identical `flag.Value`, so every flag works on either side of the input
  and anything unconsumable is reported rather than ignored. This replaces
  `outputFlagAfterInput`, which rescanned raw argv for the `-o` and `-o=`
  spellings alone, so `-o` is no longer the only flag with after-input
  handling, while keeping its position-dependent meaning (output directory
  before the input, exact artifact file after it). The `-run-bc` and `-run`
  argv contract is untouched: those modes skip the resumed parse entirely and
  still hand every token after the input to the target program verbatim, and
  writing either of them after the input is now rejected with a diagnostic
  instead of silently reinterpreting the program's arguments. The `build`
  subcommand resumes its own parse the same way, so it too accepts any of its
  flags after the source file and now rejects a stray positional or an
  undefined flag there rather than ignoring it. `-h` and `-help` after the
  input print the same usage and exit 0 just as they do before it, instead of
  becoming a `flag: help requested` diagnostic. Covered by
  `TestCLIFlagPositionRelativeToInput`. (bugs.md #52)
* `route` handlers are validated before they are indexed. `checkGoAppHandler`'s
  `case "route"` read `Children[2].Children[1]` and `Children[2].Children[2]`
  without checking that the handler was a well-formed `(lambda (req) body)`, so
  `(http_server 8080 (route "/" (lambda (req))))` crashed the compiler with
  `index out of range [2] with length 2` rather than reporting an error. The
  guard the `middleware` case already applied to its own lambda is now applied
  to `route` as well, and the middleware nested-route loop delegates to
  `checkGoAppHandler` instead of duplicating the validation, so nested routes
  are checked identically without adding duplication debt. A parameter that is
  not a symbol is now rejected too. Covered by `TestCheckRouteHandlerValidation`
  with 12 subtests across both forms. (bugs.md #50)

* `docs/archive/old_howlframe.go` is no longer counted as production Go. The
  file is a 4177-line archive of the pre-cutover monolithic implementation
  whose first line is `//go:build ignore`, so Go itself never compiled it, but
  `.slop`'s `go_production` scope (`scan_path: .`, `pattern: "**/*.go"`)
  matched it and counted 4177 lines of dead history as production source. It
  was one side of roughly 81% of the repository's duplicated windows, which
  held `go_production` at 291 active clones against a committed ceiling of
  290 and failed the required repository-hygiene gate for every governed task
  regardless of what that task changed. Renamed to
  `docs/archive/old_howlframe.go.txt`, which states a fact that was already
  true rather than adding an ignore rule or tombstone to the hygiene policy.
  `go_production` fell from 291 active clones across 41 sources to 84 across
  40, and the committed ceiling was ratcheted down from 290 to 84 to match.
  `go_tests` is unaffected at 128. Build, vet, test and `gofmt` output are
  unchanged, since the file was never in the build. (bugs.md #49)

* Fixed a severe issue where `str_split`, `str_join`, `regex_match`, `append`, `map_set`, `map_delete`, `map_get`, `list_get`, and `list_len` bytecode VM instructions relied on unsafe Go type assertions, which caused immediate uncatchable internal panics (`interface {} is ...`) instead of deterministic script errors. The VM now performs safe checks and produces structured `TYPE_ERROR` runtime errors when given unexpected types, enabling correct `try_let` interception.

* Bytecode `try_let` now resumes after its complete embedded instruction region
  instead of re-executing the final branch instruction and corrupting an
  enclosing `for` loop's operand stack.
* `-compile-bc` silently dropped every construct the bytecode compiler did not
  recognize. `compileNode`'s head switch had no `default` case, so unknown
  heads compiled to zero instructions and the resulting program ran to exit 0
  while skipping them entirely: `tests/test_advanced_control.howl` produced no
  output at all instead of `zero`/`one`/`other`. Unsupported constructs are now
  rejected before an artifact is written, and `compileNode` has a fail-closed
  backstop for callers that bypass the gate.
* `type_hints` and `type_param` had no `compileNode` case and were only handled
  by accident, through that same silent fall-through. They now have explicit
  no-op cases alongside `type_hint`.
