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
