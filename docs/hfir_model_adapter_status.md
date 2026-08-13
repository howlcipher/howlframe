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

Eleven stored black-box candidates were evaluated: ten first-pass behavioral
successes plus one deliberately wrong semantic value repaired by one bounded
delta. The candidate schema/integrity and verifier pass rate was 11/11; the
first-pass behavioral rate for intended-correct tasks was 10/10. Average graph
size was 8.64 nodes. The recorded public-documentation `.howl` comparison
compiled and behaved correctly for 3/10 equivalent first attempts. This small,
single restricted-author sample is evidence, not a provider-performance claim.

## Diagnostic quality

Schema/integrity failures are stable diagnostics with code, target, and node
ID where one exists. The repair case was an oracle mismatch tied to node
`greeting`; the trusted repair context carries that node, its immediate
neighborhood, schema/hash preconditions, and permitted reference IDs. Invalid
transport has no canonical identity and is regenerated rather than patched.

## Delta behavior

One `replace_node` delta repaired one of three nodes in the deliberately tiny
greeting graph. It cannot change graph/entry/version, node identity or kind,
introduce an effect, widen authority, use undeclared references, or bypass
reverification. Stale graph and node hashes fail closed.

## Adversarial results

Malformed/duplicate JSON, dangling and duplicate IDs, wrong roles, cycles,
function/loop/HTTP kinds, raw opcode/Go/JavaScript/Wasm injection, capability
and instruction-budget self-grants, stale repairs, out-of-scope repairs, and
identity-changing repairs are rejected before lowering or execution. No policy
bypass succeeded.

## `.howl` vs HFIR comparison

The HFIR experiment used direct semantic transport and all intended first-pass
tasks ran correctly. The recorded `.howl` retry used only public source docs
and reached 3/10 correct first attempts. Its diagnostics were source-oriented;
HFIR diagnostics additionally name semantic node IDs and support bounded
deltas. No conclusion beyond this compact, offline sample is warranted.

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

## Roadmap decision

Choose **B. Semantic repair/delta Phase 2** next. Initial graph authoring was
strong across this bounded subset, while the restricted author's first repair
used a stale delta shape and the accepted experiment intentionally permits only
one same-kind node replacement. The highest-value evidence-driven extension is
to improve diagnostic-derived edit regions and multi-node semantic deltas—not
to widen executable HFIR or add a provider ecosystem yet.
