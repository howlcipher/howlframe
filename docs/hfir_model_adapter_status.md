# HFIR Model Adapter Status

## Adapter contract

Phase 1 accepts only the versioned transport in
[`hfir_model_adapter_phase1.schema.json`](reference/hfir_model_adapter_phase1.schema.json).
It mechanically decodes to the existing `internal/hfir.Graph`; it neither
creates an AST nor invokes an AST compiler. A provider-neutral `Adapter`
interface exposes `Generate` and `Repair`; provider/model/token fields are
observability metadata, never program semantics.

## Executable subset

The transport exposes only the direct bytecode subset recorded in
`docs/hfir_execution_status.md`: program/sequence/bindings/branches, scalar
and collection values, deterministic string/list/map operations, output/exit,
and `env`. Functions, loops, recovery, routes, JSON parsing, stores, network,
files, process operations, and model operations are excluded.

## Trust boundary

Models may propose supported semantic nodes, constants, binding names, and
bounded provenance labels. They cannot propose effects, capability grants,
instruction limits, backend or opcode details, verification rules, artifacts,
or behavioral oracles. A trusted runner independently supplies capability
grants and its finite execution policy. Intent is not authority.

## Model synthesis benchmark

The deterministic benchmark and recorded black-box transcript fixtures are
executed by the adapter tests. They run candidate transport through decode,
verification, direct lowering, artifact validation, and an isolated VM—never
through `.howl` or an AST.

## Repair protocol

The experimental repair transport in
[`hfir_model_repair_phase1.schema.json`](reference/hfir_model_repair_phase1.schema.json)
allows one `replace_node` operation only.
It requires the canonical graph hash, target-node ID, and old-node hash, and
may replace only the trusted diagnostic target without changing its ID. The
caller must decode, verify, lower, and run the replacement afresh.
Schema-invalid candidates have no canonical graph to patch and require full
regeneration. This is not completion of improvement #91.

## What remains AST-owned

Public `howlframe build`, parsing, checking, modules, transformations, and
all unsupported constructs remain AST-owned. This experiment does not change
the production build path.

## What this does not prove

It does not prove broad application synthesis, provider reliability, a stable
artifact format, content-addressed HFIR, multi-node repairs, or support for
the consumer applications. Its evidence and metrics are recorded in the
Phase-1 journal after the deterministic experiment runs.
