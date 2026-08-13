# HFIR Semantic Repair Status

## Repair contract

Phase 2 accepts `hfir-semantic-repair/v2` only for an atomic transaction of
one to five existing-node `replace_node` operations. The transport requires
the canonical graph hash, graph version, every protected-node hash, and an
old-node hash for each operation. Replacement nodes retain identity and kind.
No insert, removal, field-only, edge-only, or general JSON patch operation is
supported because the failure corpus has not required one.

## Repair-region derivation

Trusted code receives diagnostic target IDs and deterministically derives the
editable set as target nodes plus one-hop data-input producers and consumers,
deduplicated and sorted, with a 16-node ceiling. The model cannot expand this
set. Current model transport does not carry executable control edges, so this
is a data-flow neighborhood only, not a control-flow analysis.

## Atomicity and stale protection

All preconditions are checked before a copy-on-write reconstruction. The
result is decoded, all protected hashes are compared, and it must pass
verification, direct HFIR bytecode lowering, and bytecode artifact validation
before it is returned. Any error returns no candidate and leaves the input
canonical graph unchanged. Graph hash, graph version, per-node old hashes,
and protected-node hashes reject stale or concurrent repairs; no automatic
rebase exists.

## Protected authority

The repair schema contains no capability grant, instruction budget, verifier,
oracle, adapter policy, backend, opcode, entry-node, or graph-version mutation
field. Strict decoding rejects these injections. Nodes outside the derived
region, including authority-relevant nodes, are hash-protected; same ID/kind
and capability classification are mandatory within the region.

## Evidence

The deterministic failure catalog is intentionally limited to the Phase-1
executable subset. It contains twelve realistic candidates: dangling
reference, invalid operator, invalid role composition, wrong constant, wrong
comparison, wrong branch result, wrong dictionary key and value, wrong list
item and separator, missing sequence operation, output and exit disagreement,
dictionary update pair, and branch condition plus output pair. Six entries are
coordinated multi-node defects. The executable Phase-2 benchmark proves the
dictionary update pair: either independent correction still prints `empty`,
while the two-operation transaction prints `ready`.

The recorded restricted-context black-box author received only the task,
actual/expected behavior, editable nodes, immutable context, hashes, and
schema. It made two schema-shape mistakes, received stable diagnostics, and
produced a correct two-operation proposal on its final permitted attempt. The
transcript is recorded in
`docs/fixtures/hfir_semantic_repair_black_box_phase2.json`; CI remains offline
and does not need a provider account.

## Adversarial results

Focused deterministic tests reject stale graph and node patches, region escape,
self-widening context, omitted or mismatched protected hashes, identity and
capability changes, undeclared references, backend/opcode injection,
entry/version substitution, and a later invalid operation in an otherwise
valid transaction. No authority bypass succeeded.

## Full-regeneration comparison

At the demonstrated 12-node graph, the accepted repair changes two nodes and
leaves ten unchanged. Full regeneration would send and replace all 12 nodes
and is operationally simpler at this scale. The experiment establishes bounded
semantic repair, but does not establish a cost crossover or token saving: no
provider token telemetry was collected and no content-addressed graph cache
exists.

The deterministic graph-size test builds valid 10, 25, 50, and 100-node
Phase-1 graphs with one localized defect. At each size the derived repair
region is two nodes and all remaining nodes are hash-protected. This measures
surface area only, not provider tokens or an execution-time cache benefit.

## Diagnostic and architectural gaps

Stable schema diagnostics corrected black-box envelope errors. Behavioral
diagnostics currently need trusted target selection; HFIR lacks rich
control-flow edges and runtime instruction-to-node provenance. Functions,
iteration, recovery, HTTP, stores, files, network, and process operations are
outside executable HFIR Phase 1 and were not added to inflate this benchmark.

## What this proves

The direct HFIR path can accept an offline, hash-guarded, bounded multi-node
semantic delta, preserve protected graph content, re-verify, lower directly to
bytecode, and execute the repaired behavior without `.howl`, AST rebuilding,
or legacy bytecode fallback.

## What this does not prove

It does not prove broad useful multi-node repair, autonomous diagnostic target
selection, provider reliability, lower model cost than regeneration, rich
control-flow repair, content-addressed invalidation, or consumer application
coverage.

## Roadmap decision

Choose **C. Repair Phase 3**. The bounded transaction works for the evidence
available, but richer diagnostic-to-region derivation and conflict handling
need another bounded iteration before content-addressed storage or provider
expansion is justified.
