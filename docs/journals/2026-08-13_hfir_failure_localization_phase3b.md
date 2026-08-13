# Goal

Test whether direct-executable HFIR can derive a small, trusted, automatic
repair core for varied behavioral failures without manual target IDs.

# Starting SHA

`a107adeb62d0dad0016a7f869a7ffb7588388bbc` (`origin/main`), verified before
the Phase-3B branch was created. The original local Phase-3 branch was
`88a7684`; it was behind merged `origin/main`.

# Phase-3 metrics

The reproduced baseline was five executions: one dictionary write/read case
and four sequence-wrapper variants of one direct constant output. Its recorded
pooled precision was 6/17 (35.3%), recall 6/6 (100%), median region size 3,
and one successful automatic two-node repair. Focused baseline tests passed.

# Hypothesis

An in-process runner seal, executed-only data closure, opaque observed map-key
fingerprints, and a core-only repair contract will remove static adjacency and
unexecuted-branch false positives without reducing recall in the Phase-1
subset.

# Existing localization algorithm

Phase 3 selected an output anchor, a depth-two static producer closure, and a
same-resource last map writer, then gave `NewSemanticRepairContext` permission
to edit every one-hop producer and consumer. This made nonexecuted consumers
editable and did not authenticate exported trace structures.

# Known precision problems

The baseline did not have executable aggregate metrics. Its scale examples
only wrapped a direct print in `sequence` nodes. The old last-writer lookup did
not distinguish map keys. The old offline repair helper also embedded target
IDs.

# Threat model

A model can submit forged canonical node IDs, trace events, diagnostics, stale
hashes, widened contexts, replacement references, capability changes, or
policy changes. Only a current canonical graph, fresh direct lowering, and a
runner-minted sealed evidence object may determine a localized core.

# Non-goals

No HFBC source-map format, cache, AST reconstruction, `.howl` regeneration,
legacy compiler fallback, new language features, dynamic taint engine,
functions, loops, or consumer changes were introduced.

# Phase-3B implementation and evidence

`ExecutionEvidence` now receives a process-private HMAC after VM execution;
it is bound to the exact direct-lowered main program and its trace/failure
contents. Localization recompiles the canonical graph and rejects any evidence
whose seal does not verify. It no longer treats caller-provided diagnostics as
repair seeds. Map events record a SHA-256 fingerprint of the runtime consumed
key, so a matching read selects the matching final writer rather than a later
write to another key. A missing-key read retains a documented bounded fallback
to the latest writer for the same resource, which is necessary to diagnose the
existing wrong-key write experiment.

Automatic localization now creates a core-only repair context. Supporting
evidence can be reported by deterministic reason, but only the executed leaf
producers in the core are editable; every other graph node is protected by
hash. Reasons are `OUTPUT_ANCHOR`, `EXECUTED_VALUE_PRODUCER`,
`DIRECT_DATA_DEPENDENCY`, `LAST_STATE_WRITER`,
`EXECUTED_BRANCH_CONDITION`, and `RUNTIME_ERROR_ORIGIN`.

The executable corpus has eight behaviorally distinct scenarios:

| Scenario | Graph size | Test-only true nodes | Core size | Precision | Recall | Automatic repair |
| --- | ---: | --- | ---: | ---: | ---: | --- |
| dictionary wrong key and value | 12 | 2 | 2 | 100% | 100% | pass |
| numeric binary operands | 5 | 2 | 2 | 100% | 100% | pass |
| list item and join separator | 7 | 2 | 3 | 66.7% | 100% | pass |
| split then wrong join separator | 7 | 1 | 3 | 33.3% | 100% | localization only |
| executed conditional binary output | 10 | 2 | 2 | 100% | 100% | pass |
| nested numeric expression | 7 | 2 | 3 | 66.7% | 100% | pass |
| matching map writer value | 9 | 1 | 2 | 50% | 100% | localization only |
| direct output constant | 10 | 1 | 1 | 100% | 100% | localization only |

All truth lists are read only after `LocalizeFailure`; they are not supplied to
the localizer. Five scenarios are coordinated multi-node repairs. The
restricted author scans only the automatically provided core views by task
values and immutable hashes; it is never passed a ground-truth ID list. The
five attempted automatic repairs all verify, directly lower, execute, and pass
on the first permitted attempt.

Aggregate results: pooled precision is 13/18 (72.2%), pooled recall 13/13
(100%), macro precision is 77.1%, macro recall is 100%, median core size is
2, and p95 core size is 3. The corpus's median graph exposure is 31%; this is
not yet a strong large-graph locality result because several diverse cases are
small. The existing 10/25/50/100 direct-output scale probe remains local at
one editable node, but it is not broad scale evidence.

Focused adversarial checks prove that an unexecuted branch and its shared
consumer are not editable; a same-map later write to another key is not
selected; fabricated evidence naming an existing canonical node is rejected;
and `CAPABILITY_DENIED` and `LIMIT_EXCEEDED` produce no semantic repair region.
Existing repair transaction tests continue to reject stale hashes, widened
contexts, capability self-grants, instruction/backend injection, and edits
outside the core. Successful authority bypasses: 0.

# Repair and regeneration comparison

For the five repaired cases, bounded repair sends only 2, 2, 3, 2, and 3 node
views respectively and rewrites only the true replacement nodes. Full graph
regeneration would transmit and rewrite 5 to 12 nodes per case and has no
tested stale/protected-node transaction boundary in this experiment. Byte
payload measurement and a real external provider transcript are still absent,
so this is a node-surface comparison, not a token or operational-cost claim.

# Consumer regression and validation

The public AST and HFBC paths were not changed. Consumer integrations were not
available in this checkout, so no new ChangeOps or HowlBoard execution claim is
made. Focused HFIR localization, repair, and adversarial tests passed before
the full repository validation gate.

# Decision

Choose **C — Localization Phase 3C**. Precision materially improved while
recall held, and five distinct coordinated repairs passed. However, the corpus
is still small, median exposure is high for small graphs, state provenance is
limited to map keys, output-observation binding is not generalized, and scale
coverage is only a direct-output shape. This does not earn content-addressed
HFIR (#89).
