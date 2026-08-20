# Previous bottleneck

Phase 2 required a trusted caller to name HFIR repair targets before it could
derive a repair context. A behavioral oracle mismatch alone could not create a
repair transaction.

# Provenance design

Direct HFIR lowering now attaches an ephemeral instruction-origin sidecar to
the in-memory `BCProgram`. Each emitted instruction maps to its canonical HFIR
node ID. The sidecar is structurally bound to `BCProgram.Main`; any mutation
invalidates lookup. It is intentionally unexported, so HFBC serialization and
its durable version remain unchanged. Artifact round trips have no provenance
and cannot be localized from that map.

# Control-flow representation

The existing persisted `ControlEdges` field is not used: it has no role,
direction, ordering, lowering, or model-transport semantics. Phase 3 instead
derives a read-only Phase-1 control view from validated canonical roles:
program and sequence body order, let body continuation, and if then or else
branch containment. No model-supplied control edge is accepted. `exit` remains
terminal runtime behavior rather than an arbitrary graph edge.

# Runtime trace

`vm.RunBytecodeWithEvidence` returns a runner-bounded instruction trace. An
event records offset, opcode, lowerer-owned node ID, and a conditional jump
outcome. VM failures expose the same verified origin when available. Trace
capacity is an explicit runner argument; code cannot increase it.

# Localization algorithm

`LocalizeFailure` accepts only the canonical graph hash and version plus
trusted diagnostics, runner evidence, and an oracle comparison. Verifier and
lowerer diagnostics use `RelatedNode`. Runtime failures use the verified
instruction sidecar. Behavioral mismatches anchor to a traced print, stderr,
or exit; the algorithm adds bounded data dependencies, a traced last dictionary
writer where applicable, and an executed if condition. Missing, stale,
truncated, or forged evidence fails closed.

# Repair-region contract

The result includes deterministic selections and reasons, graph and region
hashes, and the existing Phase-2 `RepairContext`. `ApplyRepair` independently
rederives that context from the graph, preserves its protected-node hashes,
limits operations to five, and limits the editable region to sixteen nodes.
The model cannot widen the region.

# Metrics

Current executable evidence is deliberately small.

| Metric | Result |
| --- | --- |
| Behavioral localization scenarios | 5 of 5 localized |
| Full automatic behavioral repair flows | 1 of 1 succeeds |
| Multi-node localization scenarios | 1 of 1 (dictionary key and value) |
| Multi-node target requirement | Not met: fewer than five scenarios |
| Recall against test-only affected nodes | 6 of 6 (100%) |
| Region precision against test-only affected nodes | 6 of 17 (35.3%) |
| Median region size | 3 nodes |
| p95 region size | 5 nodes |
| Median graph exposed | 12% |
| Manual targets in demonstrated repair | 0 |
| Stale evidence rejection | 1 focused case |
| Forged runtime-origin rejection | 1 focused case |
| Authority-node repair after denial | 0; denial is rejected |
| Successful authority bypasses | 0 observed |

The five behavioral scenarios are the dictionary write and local wrong-output
constant graphs at 10, 25, 50, and 100 nodes. The only full repair transcript
is the two-node dictionary correction. Precision is intentionally reported on
the editable region, not just the seed list; it shows that this first strategy
is still too broad for a strong autonomy claim.

# Behavioral localization

The demonstrated flow is incorrect direct-executable HFIR, VM output
`empty\n`, trusted expected output `ready\n`, automatic output anchor and
last-writer selection, hash-bound region derivation, offline repair proposal,
full re-verification, direct lowering, and correct execution. No `.howl`, AST,
or legacy bytecode lowering participates.

# Multi-node localization

The dictionary failure has two coordinated defects: wrong key and wrong value.
The trace observes the later read and identifies the last `map_set`; its input
constants are in the automatic region. The repair changes both atomically. The
corpus is not yet large enough to claim broad multi-node localization.

# Scale behavior

At graph sizes 10, 25, 50, and 100, a local wrong-output defect derives a
three-node region. This is a locality observation over four samples, not an
asymptotic claim or a cache-cost result.

# Security and adversarial results

Model labels do not become provenance truth. Provenance comes only from the
canonical lowerer sidecar. A stale graph hash and a nonexistent runtime node
are rejected. A mutated bytecode program and an HFBC artifact round trip lose
trusted provenance. `CAPABILITY_DENIED` and `LIMIT_EXCEEDED` are authority or
policy failures and receive no semantic repair region.

# Consumer compatibility

The public AST-backed build path and durable HFBC encoding are unchanged.
HowlBoard's HTTP integration suite passes with the Phase-3 binary. HowlChangeOps'
isolated integration suite also passes, including approval, stale-evidence,
and missing-HFBC failure boundaries.

# Remaining limitations

Only direct Phase-1 main bytecode has provenance. There are no functions,
loops, recovery regions, or durable artifact source maps. Behavioral anchors
cover traced output and exit only. The derived Phase-1 control view is not yet
used to exclude all unexecuted branch context during repair-context expansion.
The multi-node corpus is one demonstrated case, not the requested five.

# What this proves

HowlFrame can automatically derive and enforce a bounded target region for one
real two-node behavioral repair, with trusted direct bytecode provenance,
bounded execution evidence, stale rejection, and no manual target selection.

# What this does NOT prove

It does not prove reliable broad behavioral localization, five coordinated
multi-node cases, high precision, provider success, a full debugger, durable
artifact provenance, content-addressed cache value, or a repair success rate
comparable with manual Phase-2 targeting.

# Phase 3B — executed-path precision experiment

Phase 3B added runner-sealed evidence, runtime map-key fingerprints,
executed-only producer closure, and core-only edit authority. Eight varied
behavioral scenarios now have executable metric assertions; five are
coordinated multi-node automatic repairs, and all five repair flows pass with
no manual target IDs. Pooled precision improved from 35.3% to 72.2%, macro
precision is 77.1%, and recall remains 100%. Median editable core size is 2
and p95 is 3.

The result is encouraging but not a cache justification. Small diverse graphs
still expose a median 31% of nodes, and only the direct-output 10/25/50/100
probe establishes scale locality. Sealed evidence rejects forged existing-node
origins; unexecuted branches, mismatched map writers, authority denial, and
instruction-limit denial are bounded or rejected. The recommended next
milestone is **Localization Phase 3C**, not #89. See
`docs/journals/2026-08-13_hfir_failure_localization_phase3b.md`.

# Phase 3C decision gate

Phase 3C exercised three executable semantic graphs with 26, 53, and 109
nodes. They contain real independent arithmetic, map, list, and scalar-state
regions, multiple outputs, shared producers, nested executed conditionals, and
three coordinated two-node defects. This replaces the earlier sequence-wrapper
scale probe as decision evidence.

Runner-sealed state provenance now covers `set` and `let` writes observed by a
symbol read, `append` observed by `list_get`, and `map_set` or `map_delete`
observed by `map_get`. It is namespaced by state kind and resource, carries an
opaque runner-created fingerprint, and remains inside the execution-evidence
seal. Selection considers only a write before the concrete observed read. A
later unrelated write is excluded. Ambiguous absent-key maps intentionally do
not guess a writer.

That final condition exposed the current blocker: in the 53-node graph, an
absent target key has two earlier map writers. The output's explicit key input
is selected, but neither writer can be safely attributed to the read. The
actual faulty key and value are therefore absent from the editable core: 0%/0%
precision/recall for that scenario and no automatic repair. This is a
generalized wrong-state-writer gap, not a fixture-name rule.

The 26-node and 109-node independent-output defects localize to their two
true constants and repair automatically. Their conservative reverse
data-dependency closure is respectively 5/26 and 9/109 nodes. However, the
actual repair transport includes protected hashes for every unchanged node:
the complete context plus accepted delta is 5,162 bytes versus a 3,743-byte
full graph at 26 nodes, and 19,576 versus 16,486 bytes at 109 nodes. This is
not a provider-token measurement. It is sufficient to show that the present
transport and full `ApplyRepair` re-verification/lowering path do not yet show
an incremental-storage economic win.

**Decision:** Localization remains the bottleneck. Improvement #89 is
deferred. The next focused gap is deterministic, runner-sealed attribution for
an absent map-key observation with multiple preceding writers, or a deliberately
bounded diagnostic that represents the ambiguity without granting an unrelated
writer edit authority. Do not use content addressing to conceal this failure.
See `docs/journals/2026-08-13_hfir_localization_phase3c_decision_gate.md` for
the full measurements and consumer evidence.

# Phase 3D negative state provenance

Phase 3D adds a bounded, runner-sealed map-state ledger for the direct
execution experiment. It distinguishes a proven `NEVER_PRESENT` key from an
effective requested-key `DELETED` state, uses backing-map identity instead of
binding names, and records only completed operations. A uniquely matching
host-sealed expected scalar can make one active wrong-key writer editable;
multiple or insufficient candidates return
`HFIR_LOCALIZATION_AMBIGUOUS_STATE_CAUSE` with read-only evidence and no
repair context. The 53-node Phase 3C case is now explicit two-writer ambiguity,
not an unexplained miss. See `docs/hfir_negative_state_provenance_status.md`.
