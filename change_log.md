# Change Log

## Unreleased

### Added
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
