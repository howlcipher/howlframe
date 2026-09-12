# Goal

Work `WI-howlframe-6b65d61e2dafb2a9`: "Define a provider-neutral HFIR
model-adapter protocol" (improvement #88).

# Starting SHA

`97b88301442de0c575393bc2bbd37c00e531b973` (`origin/main`).

# Context & Finding

Improvement #88 was initially prototyped on 2026-08-13 (`73ab47c`, `61f1888`,
`2aba4e6`), defining the `Adapter` interface (`Generate`/`Repair`), the
`hfir-model-adapter/v1` candidate transport, and the `hfir-model-repair/v1`
bounded delta protocol in `internal/hfir/model_adapter.go`.

However, review of the candidate decoder and type validation against adversarial
and multi-provider scenarios surfaced several defects:
1. `validateTransportTypes` erroneously typed `BOOL` literals as `"number"` and
   hardcoded `convert` operations to `"string"`, rejecting valid boolean logic
   and arithmetic on converted values while accepting invalid combinations like
   `bool + int`.
2. Ordering comparisons (`<`, `>`, `<=`, `>=`) were not separated from equality
   comparisons (`==`, `!=`), allowing invalid string-to-string and bool-to-bool
   order comparisons, and string concatenation via `+` was unhandled.
3. In `validateTransport`, `validateTransportTypes` was executed before cycle
   detection (`transportCycle`), and `infer()` had no cycle guard, leading to
   unbounded recursion and host runtime stack overflow on cyclic candidate graphs.
4. `validateTransportTypes` omitted validation for `case "if"`, permitting
   non-boolean condition expressions into the bytecode compiler and causing VM panics.
5. Adapter test doubles (`anthropicFormatAdapter` and `openAIFormatAdapter`) were
   static stubs returning a single hardcoded computation rather than adapting
   fixture graphs from `docs/fixtures/hfir_model_adapter_black_box_phase1.json`.
6. `openAIFormatAdapter.Repair` was unexercised in tests, and `mockAdapterRepair`
   relied on fragile external pre-injected target node hashes.
7. `TestSchemaConformance` caught `ImportError` inside Python without failing or
   skipping, causing false-positive pass results when `jsonschema` was missing.
8. `TestAdapterInterfaceMetadataIsNotProgramSemantics` was gutted in prior drafts,
   losing coverage for metadata isolation against canonical graph hashing.

# What Changed This Session

1. **Type Inference & Validation (`internal/hfir/model_adapter.go`)**:
   - Added support for `BOOL` literals (`typ = "bool"`), distinguishing them from
     numeric literals (`INT`, `FLOAT` -> `"number"`) and strings (`"string"`).
   - Added typed `convert` inference: `to_int`/`to_float` -> `"number"`,
     `to_string`/`bytes_to_string`/`encode_json` -> `"string"`.
   - Separated ordering comparisons (`<`, `>`, `<=`, `>=`) from equality
     comparisons (`==`, `!=`). Ordering requires numeric operands; equality
     allows identical known types (string==string, bool==bool, number==number)
     while rejecting mismatched types like bool==int.
   - Allowed string concatenation via `+` alongside numeric addition.
   - Added `case "if"` validation in `validateTransportTypes`, ensuring condition
     operands are boolean before compiler emission.

2. **Cycle Protection & Fail-Closed Guard (`internal/hfir/model_adapter.go`)**:
   - Reordered `validateTransport` so `transportCycle` runs before
     `validateTransportTypes`.
   - Added recursion cycle tracking (`visiting` map) inside `validateTransportTypes`
     to guarantee fail-closed termination with `HFIR_TRANSPORT_CYCLE` without
     relying solely on external cycle detection.

3. **Schema Specification (`docs/reference/hfir_model_adapter_phase1.schema.json`)**:
   - Added `"BOOL"` to `literal_kind` enum under `$defs.node.properties.literal_kind`.

4. **Adapter Test Doubles & Repair Coverage (`internal/hfir/model_adapter_test.go`)**:
   - Updated `anthropicFormatAdapter` and `openAIFormatAdapter` to handle both
     default tasks (`compute_sum`) and benchmark fixtures from
     `docs/fixtures/hfir_model_adapter_black_box_phase1.json`, with distinct provider
     serialization envelopes (compact JSON vs indented/reordered nodes).
   - Added `TestAdapterTestDoublesPassValidFixtureGraphs`, executing all valid
     phase-1 benchmark fixtures through both test doubles and asserting canonical
     graph hash invariance and identical VM execution.
   - Restored and retained the metadata mutation assertion in
     `TestAdapterInterfaceMetadataIsNotProgramSemantics`.
   - Converted `TestAdapterRepairLifecycle` to a table-driven test exercising both
     `anthropicFormatAdapter` and `openAIFormatAdapter`, with context validation
     in `mockAdapterRepair` and missing-context rejection.
   - Added `TestDecodeCandidateBooleanLogic`, `TestDecodeCandidateConvertArithmetic`,
     `TestDecodeCandidateRejectsInvalidTypes` (using refactored `assertTransportTypeRejected`),
     `TestDecodeCandidateCyclicBinaryDependencyFailsClosed`, and
     `TestDecodeCandidateNonBooleanIfConditionRejected`.
   - Hardened `TestSchemaConformance` with explicit `t.Skip` when `python3` or
     `jsonschema` is not installed, eliminating false-positive passes.

5. **Backlog & Governance Tracking (`improvements.md`)**:
   - Marked #88 as `Done (2026-08-13) — Phase 1` in the table, with detailed status
     note documenting the verified acceptance criteria and pointing to proving tests.
   - Corrected commit attribution and updated #100's unblocking status note.

# Verification

Executed live verification in the environment:
- `go test -count=1 -v ./internal/hfir`: All tests pass cleanly (including all adversarial, boolean logic, convert arithmetic, invalid type rejection, cyclic binary dependency, if condition, and adapter double suites).
- `go vet ./...`: Clean (exit code 0).
- `go build ./...`: Clean (exit code 0).
- `go test -count=1 ./...`: All 32 packages pass cleanly.

# What This Does Not Do

Does not implement broader multi-vendor provider API integration (e.g. live network
clients for Anthropic or OpenAI), complete-HFIR coverage beyond the direct bytecode
subset (`docs/hfir_execution_status.md`), constrained-decoding integration with a real
inference API, or provider token accounting. Those remain explicitly out of scope for
Phase 1 and are tracked as future work.
