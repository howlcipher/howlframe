# Goal

Determine whether deterministic trusted evidence can localize a bounded HFIR
repair region without a human or privileged caller supplying semantic node IDs.

# Starting SHA

`524753d43b504f42fc89e13f529916145b216004` on `main`, verified live before
the Phase-3 branch was created.

# Existing repair pipeline

Phase 2 accepts a one to five operation, same-kind `replace_node` transaction.
A trusted caller supplies targets; the repair context derives their one-hop
data-flow neighborhood, protects every other node by hash, then decode,
verification, direct HFIR lowering, bytecode validation, and execution remain
fail closed.

# Current target-selection dependency

`NewSemanticRepairContext` takes trusted node IDs. The Phase-2 behavioral
dictionary proof supplies `update_key` and `update_value` directly. Existing
diagnostics contain only node-local verifier or lowerer information, and VM
errors identify bytecode offsets rather than HFIR nodes.

# Provenance inventory

HFIR nodes contain source provenance metadata. Direct lowering emits plain
`BCInstruction` values without an HFIR node reference. VM errors contain the
runtime instruction offset and opcode. The initial live inventory therefore
has no trustworthy complete instruction-to-semantic-node mapping.

# Current control-flow representation

HFIR has `ControlEdges`, but Phase-1 adapter transport and direct lowering use
ordered `DataInputs` for `program`, `sequence`, and `if`. Phase-2 region
derivation follows data producers and consumers only. This journal will record
whether the existing control edges are semantically maintained or whether a
minimal derived relation is needed.

# Threat model

The model may forge node IDs, provenance labels, trace-like data, diagnostics,
or repair envelopes; it may try to select authority-adjacent nodes, widen a
region, use stale evidence, or alter backend and execution authority. Only
trusted graph canonicalization, lowerer-produced provenance, trusted VM trace,
trusted oracle, and hash-bound repair context may influence editable scope.

# Hypothesis

For the current Phase-1 executable subset, a lowerer-owned instruction
provenance sidecar plus a bounded trusted execution trace and deterministic
data/control dependency derivation can expose a small useful repair region for
selected behavioral and runtime failures, without modifying durable HFBC.

# Non-goals

No new repair operations, syntax, durable HFBC versioning, provider expansion,
cache, functions, loops, recovery, HTTP, or public AST-backed path changes are
part of this experiment. Bug #48 is separate work and is not mixed into this
branch.

# Baseline experiment

The existing Phase-2 test corpus requires direct trusted targets before
`NewSemanticRepairContext` can construct a valid repair request. With only a
behavioral mismatch such as expected `ready\\n`, actual `empty\\n`, there is no
current public API that produces `update_key` and `update_value`; passing no
targets yields no repair region. This establishes the manual target-selection
bottleneck without using fixture names as localization input.

## Failure

Dictionary update prints `empty\n` instead of the oracle output `ready\n`.

## Failure class

Behavioral oracle mismatch.

## Ground-truth affected nodes

`update_key`, `update_value` in test evaluation only.

## Automatically selected nodes

The trace anchors `print_result`, follows `read_result`, identifies the last
writer `write_result`, and derives `update_key`, `update_value`, plus the
bounded existing repair context.

## Region size

5 nodes of 12.

## Precision

2 / 5 = 40% against the test-only affected nodes.

## Recall

2 / 2 = 100%.

## Repair attempt

An offline restricted-context author proposal changes the two constants. Its
operations are checked to be inside the automatically derived context before
the unchanged Phase-2 hash-guarded transaction applies them.

## Repair success

Success on the first valid proposal; direct HFIR lowering and VM execution
produce `ready\n`.

## Unrelated nodes exposed

Three context nodes are exposed for semantic explanation and hash-guarded
repair validation. This is useful but not yet high precision.

## Result

Automatic target selection is demonstrably possible for this bounded
multi-node mutation case; no manual IDs were supplied to localization.

## Lesson

Trace-informed last-writer evidence solves a gap that data-flow alone cannot,
but region precision and corpus breadth still prevent an autonomy claim.

## Failure

Wrong constant output in valid graphs of 10, 25, 50, and 100 nodes.

## Failure class

Behavioral oracle mismatch.

## Ground-truth affected nodes

The test-only `defect` constant in each graph.

## Automatically selected nodes

Traced `print`, then its bounded value dependency.

## Region size

3 nodes at every size.

## Precision

1 / 3 = 33.3% per graph.

## Recall

1 / 1 = 100% per graph.

## Repair attempt

Localization only; no extra repair transcript was fabricated.

## Repair success

Not measured for these scale probes.

## Unrelated nodes exposed

Two nodes per graph.

## Result

The repair surface stayed local while the graph increased tenfold.

## Lesson

Locality is encouraging but does not establish cost savings or high precision.

# Security checks

A canonical graph hash mismatch and a runtime error naming a nonexistent node
both fail closed. Lowering provenance disappears after HFBC artifact round
trip and after `BCProgram.Main` mutation. A capability denial maps to the
trusted `env` origin but produces `HFIR_LOCALIZATION_AUTHORITY`, not a repair
region. No tested path lets model labels, offsets, trace records, capability
grants, or instruction policy select an editable region.

# Consumer regression

The HowlBoard HTTP integration suite passed using the Phase-3 binary. The
ChangeOps integration suite passed from an isolated temporary copy with the
same binary; its dirty-repository denial, approval gate, stale-evidence block,
and missing-HFBC failure all remained intact. Neither consumer worktree was
used as a source of Phase-3 changes.

# Validation

Passed: `gofmt -l .`, `go build ./...`, `go vet ./...`, `go test ./...`,
`go test -race ./...`, `python3 -m unittest benchmarks/v2/harness/test_harness.py`,
`go run tools/difftest/main.go`, `go run ./cmd/codegen`, and `git diff --check`.
Bug #48 remains untouched because it belongs on an isolated fix branch.
