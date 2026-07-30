# Change Log

## Unreleased

### Added

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
  `.zero` replacement in an isolated project copy, installs it atomically only
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

### Changed

* Long valid `let` chains now use shared iterative traversal across AST
  preprocessing, semantic checking, and Go/JavaScript emission, with a
  2,000-binding regression covering transpilation and generated Go builds.
* Synchronized the Zero language write-cost benchmark fixtures with the
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
