# Goal

Determine whether a model-authored, mostly-correct executable HFIR graph can
receive a bounded multi-node semantic repair without source regeneration, AST
reconstruction, legacy bytecode lowering, or authority changes.

# Starting SHA

`797df3963865412c9344bb34a29a6da78eb02048` on `main`, verified live before
the Phase-2 branch was created.

# Existing repair contract

Phase 1 accepts exactly one `replace_node` operation in
`hfir-model-repair/v1`. It requires the canonical graph hash and the target
node hash, preserves node ID and kind, limits references to the trusted local
neighborhood, and relies on the caller to re-verify and compile the resulting
candidate.

# Existing evidence

The Phase-1 benchmark contains eleven compact candidates. Ten intended-correct
candidates passed first execution; one three-node greeting candidate repaired a
single incorrect string constant. This is evidence for a boundary, not for
multi-node semantic repair.

# Hypothesis

A deterministic repair-region algorithm and atomic, hash-guarded transaction
of same-kind node replacements can repair localized, related semantic errors
while preserving every node outside the authorized region byte-for-byte.

# Threat model

An untrusted provider can emit malformed or stale deltas, attempt edits outside
the permitted region, modify graph identity or protected metadata, introduce
undeclared references or effects, inject backend data, or alter the authority
and acceptance boundary. Every such repair must be rejected before execution.

# Protected state

Graph version and entry node, canonical graph identity, node identities and
kinds, capability grants, instruction budget, verifier policy, adapter policy,
backend and opcode selection, behavioral oracle, test acceptance criteria, and
all nodes outside the trusted repair region are protected.

# Model-editable state

Only a trusted-context target set may be replaced, using supported transport
nodes with the same identity and kind. Each replacement is guarded by the
current graph hash and its old node hash.

# Non-goals

This experiment does not add `.howl` syntax, AST reconstruction, legacy
fallbacks, new executable HFIR forms, functions, loops, recovery, HTTP,
stores, files, network, process execution, content-addressed storage, or live
provider access in CI.

# Baseline observations

`docs/hfir_model_adapter_status.md`, `docs/hfir_execution_status.md`,
improvement #91, the Phase-1 schema, implementation, tests, and benchmark
fixture were read before modifying the repair protocol. Direct HFIR lowering
already uses `CompileCandidate`, which verifies, lowers directly to bytecode,
and validates the artifact. Phase-1 `ApplyRepair` itself only reconstructs and
decodes the candidate, so Phase 2 must make acceptance transactionally require
the full compile gate.

# Failure corpus and execution proof

`docs/fixtures/hfir_semantic_repair_failure_corpus_phase2.json` records twelve
near-valid Phase-1 scenarios: three verifier-detectable shapes, one
lowerer/runtime boundary, and eight behavioral candidates. Six are coordinated
two-node defects. This is a catalog, not a claim that all twelve have
completed black-box repair.

The executed 12-node dictionary case initializes `result` to `empty`, then
writes wrong key/value constants. Expected `ready\n`, actual `empty\n`.
Either independent repair still fails. Trusted targets `update_key` and
`update_value` derive a three-node data-flow region with `write_result`.
The accepted v2 transaction replaces just the two constants, guarding the
canonical graph hash, v1 graph version, both old hashes, and every outside-node
hash. Decode, region re-derivation, hash proof, verifier, direct lowering,
artifact validation, and VM execution pass; output becomes `ready\n`. Ten of
twelve nodes remain unchanged. No insert/remove/edge operation has evidence.

# Black-box and adversarial evidence

An isolated author received only task, behavior, editable/immutable graph
views, hashes, and schema. It received no implementation, source, AST,
bytecode, oracle, or golden answer. Attempt one omitted schema version and used
an object for inputs; attempt two used `v2` rather than the full version; stable
diagnostics enabled a correct final two-node transaction on attempt three. The
offline transcript is `docs/fixtures/hfir_semantic_repair_black_box_phase2.json`.

Focused tests reject stale graph/node preconditions, region escape,
self-widened context, altered protected hashes, identity/kind changes,
capability self-grant, undeclared references, backend/opcode injection,
entry/version substitution, and a later invalid operation in an otherwise
valid transaction. No invalid repair returns a candidate; authority bypasses: 0.

# Graph-size experiment and metrics

Valid 10, 25, 50, and 100-node graphs each have one localized defect. Their
derived regions have two nodes, leaving 8, 23, 48, and 98 protected nodes:
20%, 8%, 4%, and 2% repair surfaces. This is node/byte surface evidence, not
token or cache savings.

| Metric | Evidence |
| --- | --- |
| Scenario count | 12 cataloged, 1 full execution proof |
| Multi-node scenarios | 6 cataloged, 1 full execution proof |
| First-repair success | 0/1 black-box transcript |
| Success within 3 attempts | 1/1 black-box transcript |
| Nodes changed | 2 |
| Repair region / graph | 3 / 12 |
| Unchanged graph | 83.3% |
| Stale rejection | 2/2 focused cases |
| Adversarial rejection | 14/14 focused cases |
| Authority bypasses | 0 |

# Decision

Choose **C. Repair Phase 3**. The bounded transaction works, but behavioral
targeting is still trusted/manual and transport lacks rich control-flow edges.
At a 12-node graph full regeneration is simpler, so no cost crossover is
proven. Improve diagnostic-to-region derivation and conflict handling before
content-addressed storage or provider expansion.
