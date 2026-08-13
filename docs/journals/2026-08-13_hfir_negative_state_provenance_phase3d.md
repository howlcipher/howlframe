# Goal

Close the absent-map-key causality gap identified by Phase 3C without broadening
HFIR execution or implementing content-addressed storage.

# Starting SHA

`632610b584dc7e0025e5ba9ed29983a83b22a689`, the green main merge of Phase 3C.

# Phase 3C result

PR #18 merged Phase 3C after green PR and main CI. The 26/53/109-node corpus
reported pooled precision 80.0%, recall 66.7%, and macro precision/recall
66.7%. The 53-node case was a safe but unexplained abstention.

# Failing 53-node case

Two executed `map_set`s wrote `wrong-key/wrong-value` and `noise/ignored` to
one map before `map_get(result)` missed. No expected-value correlation existed,
so neither writer was granted authority.

# Positive provenance today

Runner-sealed trace provenance localizes executed scalar/list writers and a
map writer for a matching key. Trace order is execution order.

# Negative provenance gap

A miss previously had only a key fingerprint and binding-name resource. It
could not distinguish never-present, effective delete, alias/rebind identity,
or active wrong-key writers.

# Threat model

The program/model cannot supply ledger events, capacity, resource IDs,
sequences, graph identity, expected observation seals, or editable IDs.
Forged/stale/truncated evidence, unexecuted/post-read writes, overflow,
capability denial, instruction denial, and repair widening must fail closed.

# State-history design

An ephemeral map ledger records completed `INIT`, `SET`, `DELETE`, and `GET`
events after type checks, with backing-map ID, sequence, canonical key/value
fingerprints, versions, hit/miss, and effective-delete status. It is sealed to
the graph hash and ordered lowering origins, bounded to 64 resources/256
events, and is not HFBC data.

# Candidate-writer rules

Replay proves a direct read miss. Effective requested-key delete is unique.
Otherwise candidates are active, same-map, pre-read `SET`s only. A candidate is
editable only when a sealed expected string matches exactly one active scalar
writer on a direct `map_get -> print` output path.

# Fail-closed rules

No latest-write rule exists. More than one matching writer, no correlation,
candidate overflow, malformed replay, missing init, ledger overflow, or trace
truncation returns no repair context. The ambiguity result contains read-only
candidates only.

# Experiments

## Scenario

Unique wrong key plus unrelated writer.

## Executed writers

`wrong=ready`, `noise=ignored` before the miss.

## Writer order

Wrong writer then noise writer then reader.

## Writer key fingerprints

Runner-generated canonical fingerprints; no raw key authority.

## Writer value fingerprints

Only `ready` matches the sealed expected value.

## Reader requested-key fingerprint

Runner-generated fingerprint for `result`.

## Read hit/miss

Miss, replayed as `NEVER_PRESENT`.

## Ground-truth cause — TEST ONLY

The wrong-key writer and its key input.

## Automatically derived candidates

One active expected-value writer.

## Editable candidates

That writer's executed key/value inputs.

## Precision

The repair target is precise for the one-key correction.

## Recall

The required key input is present.

## Repair result

Passes `ApplyRepair`, verification, direct lowering, execution, and oracle.

## False candidates

Zero; the unrelated writer is read-only context.

## Why each candidate was selected

Same backing map, active before reader, direct output path, and unique sealed
expected-value fingerprint.

| Scenario | Executed writers and order | Writer key/value fingerprints | Reader key and result | Ground-truth cause — TEST ONLY | Automatically derived candidates | Editable candidates | Precision / recall | Repair result | False candidates and selection reason |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Unique wrong key | `write_0`, then `write_1` | opaque `wrong/ready`, `noise/ignored` | opaque `result`, miss | `write_0` key | `write_0` | its executed key/value inputs | 1.0 / 1.0 | pass | 0; unique expected-value match |
| Never written | none | none | opaque `result`, miss | none | none | none | n/a | abstain | 0; no active mutation |
| Write then delete | `write_0`, then delete | opaque `result/ready`, then `result` | opaque `result`, miss | effective delete | delete | delete key input | 1.0 / 1.0 | pass | 0; replay proves effective delete |
| Delete then rewrite | set, delete, set | opaque `result/old`, delete, `result/ready` | opaque `result`, hit | none | none | none | n/a | no negative repair | 0; hit is not absence |
| Equal wrong values | `write_0`, then `write_1` | opaque `wrong_a/ready`, `wrong_b/ready` | opaque `result`, miss | intentionally non-unique | both writers | none | n/a | abstain | 0 asserted; two equal matches are ambiguity |
| Later unrelated write | `write_0`, then noise | opaque `wrong/ready`, `noise/ignored` | opaque `result`, miss | `write_0` key | `write_0` | its executed key/value inputs | 1.0 / 1.0 | pass | 0; only first value matches |
| Post-read writer | reader, then `write_0` | opaque `wrong/ready` after reader | opaque `result`, miss | none before read | none | none | n/a | abstain | 0; sequence excludes post-read mutation |
| Unexecuted writer | branch not taken | opaque writer absent from ledger | opaque `result`, miss | none executed | none | none | n/a | abstain | 0; no completed event |
| Multi-node key construction (3 variants) | one `str_join` key writer | opaque joined wrong key/`ready` | opaque `result`, miss | two key fragments | writer | two fragment inputs | 1.0 / 1.0 | all 3 pass | 0; unique expected-value match |

The corpus also exercises never-written, effective delete, delete/rewrite,
two equal-value wrong keys, later unrelated writer, post-read writer, and
unexecuted writer. The duplicate-value case is explicit ambiguity. A companion
three-case set repairs two-node `str_join` key construction, proving that the
precise writer rule supports coordinated multi-node edits.

# Metrics

Eight deterministic scenarios: three automatic repairs, four safe abstentions,
and one already-correct hit. The companion corpus adds three automatic
two-node repairs. There are zero incorrect writer selections, zero post-read
selections, zero unexecuted writer selections, and zero observed authority
bypasses. The Phase 3C 53-node
case has zero editable recall by design but now a two-candidate trustworthy
ambiguity explanation. Existing 26- and 109-node repairs remain passing.

# Repair result

Unique wrong-key, effective-delete, and later-unrelated-writer repairs succeed
without manual target IDs; three multi-node key constructions do as well.
Ambiguous candidates receive no editable region.

Full local validation passed: format, build, vet, normal and race tests,
benchmark harness, differential test, code generation, and diff check.
ChangeOps integration, adversarial, and authority-bypass consumers passed in a
mount-isolated run using the candidate binary. HowlBoard backend HTTP smoke
passed; browser smoke was attempted but Chromium is not installed.

# Payload result

No transport optimization was attempted. Phase 3C full/context/delta results
remain 3,743/2,478/2,684 B at 26 nodes and 16,486/9,685/9,891 B at 109 nodes;
accepted repair/full ratios remain above one when context and delta are added.

# Decision

Negative state provenance is sufficient for deterministic absence, effective
delete, precise single value-correlated wrong-key repair, and honest bounded
ambiguity. Compact authenticated repair context is the next bottleneck; #89 is
not complete or justified by this milestone.
