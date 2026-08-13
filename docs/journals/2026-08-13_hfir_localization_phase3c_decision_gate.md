# Goal

Decide from executable evidence whether the localized HFIR semantic-repair
loop justifies Improvement #89 content-addressed HFIR storage and incremental
compilation. This journal records Phase 3C evidence only. It does not claim
that the Phase-1 direct execution subset supports broad application synthesis.

# Starting SHA

`289a045313067443fb39b0d2bdead282cc613630` on `origin/main`, verified before
creating `feat/hfir-localization-phase3c-gate`.

# Phase 3B baseline

The live `go test -v ./internal/hfir` and `go test -race ./internal/hfir`
runs passed at the starting SHA. The executable Phase 3B corpus has eight
behavioral scenarios, five multi-node automatic repairs, no manual target IDs,
pooled precision 72.2%, macro precision 77.1%, and recall 100%. Its documented
median editable core is two nodes and p95 is three. The focused stale,
provenance, authority, instruction-limit, scope-widening, backend-injection,
and authority-self-grant cases passed, so observed authority bypasses remain
zero.

# Decision criteria

Improvement #89 needs evidence that automatic localization stays reliable on
large semantic graphs, keeps the editable core and supporting context bounded,
substantially reduces repair payload, and leaves an affected dependency closure
substantially smaller than the full graph. Byte and node measures are proxies;
no provider token telemetry exists. Under-invalidation is not acceptable.

# Large-graph corpus design

The executable corpus uses only Phase-1 direct HFIR and no sequence padding.
All test-only truth IDs are read after production localization.

| Scenario | Nodes | Semantic shape | True affected nodes | Automatic editable core | Precision | Recall |
| --- | ---: | --- | --- | --- | ---: | ---: |
| Independent regions | 26 | Arithmetic, map mutation/read, list append/read, four outputs | `region_c_left`, `region_c_right` | same two nodes | 100% | 100% |
| Multiple writers | 53 | Set/read, append/read, nested executed conditionals, multiple map writers, six outputs | `region_c_update_key`, `region_c_update_value` | `region_c_good_key` | 0% | 0% |
| Independent-subgraph probe | 109 | Nine arithmetic, three map, two list, one set/read, shared binding, nested control, 18 outputs | `region_c_left`, `region_c_right` | same two nodes | 100% | 100% |

All three defects are coordinated multi-node defects. The 26- and 109-node
repair flows applied a restricted black-box-style delta built only from the
automatically supplied editable core, then fully verified, lowered, and ran.
The 53-node flow did not attempt a repair because localization missed both
test-only affected nodes. This preserves the three-attempt maximum and does
not turn ground-truth IDs into a recovery shortcut.

Pooled precision is 4/5 = 80.0%; pooled recall is 4/6 = 66.7%; macro precision
and recall are both 66.7%. The weak 53-node result cannot be hidden behind the
small or successful graphs. Median editable core size is 2 and p95 is 2;
median graph exposure is 1.9% and p95 is 7.7%.

# State/observation provenance gaps

The runner now seals state events for `var:<name>` at load/store/set and
`list:<name>` at append/list_get. Map resource names are likewise explicitly
`map:<name>`. Each event carries only a runner-created state key fingerprint.
The localizer can identify the mutation producing the read before an observed
print, stderr, or exit anchor for `set` or `let` -> symbol, `append` ->
`list_get`, and `map_set` or `map_delete` -> `map_get`. Focused tests reject a
mutated state event via the existing seal and exclude later unrelated writes.

The deliberate gap is an absent map key with more than one preceding writer.
There is no runner evidence connecting an absent read to one of those writers.
Selecting the last one was proven wrong by the 53-node corpus; selecting one
without evidence would be unsafe. Current behavior keeps the direct read key
editable but does not grant the ambiguous writers authority.

# Payload measurement method

The exact candidate transport JSON, trusted localized `RepairContext` JSON,
and accepted repair-delta JSON will be serialized with the production structs.
There is no exact full-regeneration transport, so its deterministic proxy is
the full canonical candidate graph transport. Provider-token claims are out of
scope without telemetry.

| Scenario | Full graph | Full context | Editable core | Repair delta | Changed | Protected | Repair/full | Closure proxy |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| 26 nodes | 3,743 B | 2,478 B | 102 B | 2,684 B | 2 | 24 | 71.7% | 5/26 (19.2%) |
| 53 nodes | 7,972 B | 4,814 B | 60 B | N/A | 2 test-only | 52 | N/A | 6/53 (11.3%) |
| 109 nodes | 16,486 B | 9,685 B | 102 B | 9,891 B | 2 | 107 | 60.0% | 9/109 (8.3%) |

The exact repair request includes both its full context and delta. Thus the
accepted 26-node request is 5,162 B and the 109-node request is 19,576 B,
each larger than full canonical transport. The compact editable core alone is
small, but it is not an independently sufficient or authorized repair request.
Median full graph is 7,972 B, median context is 4,814 B, and median accepted
delta across the two successful repairs is 6,288 B. There is no provider token
telemetry and no claim of token savings.

# Incremental-work proxy

No incremental compiler exists. The planned proxy is a conservative reverse
dependency closure over actual data-input edges plus the derived Phase-1
control containment relation. It measures potential invalidation only; it does
not claim verification or lowering reuse.

The conservative proxy follows reverse canonical data-input consumers from
the test-only changed nodes. It deliberately has no caching semantics. Its
median affected fraction is 11.3% and p95 is 19.2%, showing independent
regions exist structurally. It does not override the more important observed
facts: `ApplyRepair` currently always decodes, verifies, lowers, and validates
the complete graph, while the actual protected-hash transport is not smaller
than full graph transport.

# Consumer regression

Clean shallow checkouts were created from ChangeOps
`e7bc44d3307eb9a4192d7391b113d1b05cb4aa86` and HowlBoard
`6d9b67cd1e59665494c96f651591cf5da934c6a7`. A binary built from the starting
SHA passed ChangeOps integration and adversarial scripts. It compiled
HowlBoard's backend and frontend, served `GET /api/tasks` correctly from the
compiled backend, and produced the expected task JSON. Browser smoke remains
blocked because Playwright Chromium is not installed locally.
No consumer was modified.

# Security

Starting security evidence is the passing Phase 3B sealed-evidence and
adversarial suites. Phase 3C adds passing state-provenance cases for set,
append, map delete, print/stderr/exit anchors, later-write exclusion, and a
modified sealed state event. Existing stale graph/seal/repair, unexecuted
branch, wrong map writer, capability denial, instruction-limit denial,
region-widening, context escalation, backend/opcode injection, and authority
self-grant tests remain passing. Successful bypasses: 0.

# Decision

**Outcome C: localization remains the bottleneck.** #89 is not justified and
was not implemented. The specific next gap is runner-sealed localization of
an absent `map_get` key with multiple preceding writers. Content addressing
would not make that repair trustworthy, and the present full protected-hash
transport plus delta is not an economic win over full regeneration.
