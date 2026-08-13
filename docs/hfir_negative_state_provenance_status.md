# HFIR negative state provenance status

Phase 3D adds an ephemeral, runner-sealed map mutation ledger to the direct
HFIR execution experiment. It does not alter HFBC, add providers, broaden
execution coverage, or implement content-addressed HFIR.

## Positive state provenance

Existing scalar, list, and matching-map-key reads retain their executed
producer localization. The new map ledger is only used when the output anchor
is a single direct `map_get` and that get is a proven miss.

## Negative observation model

Each completed map operation records a monotonic sequence, opaque backing-map
resource ID, instruction/node origin, canonical map-key fingerprint, version,
and operation result. `GET` records hit or miss. `DELETE` records whether it
removed an active key. `INIT` records dictionary key presence. The ledger is
bounded to 64 maps and 256 events, retained only for one run, MAC-sealed with
the exact direct-lowered graph identity and instruction origins, and never
serialized into HFBC.

## Absence witness and candidates

The localizer replays completed events for the read resource. A miss becomes
either `NEVER_PRESENT` or `DELETED`; an empty stored string remains a hit.
An effective exact-key delete is a unique editable cause. For never-present
keys, only active pre-read `SET`s on the same backing map are candidates.
Aliases share a resource and rebindings do not.

Wrong-key authority requires a host-minted expected string observation, an
exact direct `map_get -> print` path, an exact expected/actual output shape,
and exactly one active writer whose scalar value fingerprint matches. The
expected observation is sealed and cannot be synthesized from JSON transport.
No string-key similarity, fixture metadata, source labels, or latest-write
rule participates.

## Ambiguity and security

At two or more value-matching writers, or when no trusted value relationship
exists, `HFIR_LOCALIZATION_AMBIGUOUS_STATE_CAUSE` returns bounded read-only
candidates and no editable context. Candidate ceiling is four. Ledger,
resource, event, trace, or candidate overflow fails closed. Mutating a ledger
field, expected observation, graph identity, or repair context cannot create
authority; post-read and unexecuted mutations are absent from the replay.

## Corpus and results

The deterministic corpus contains eight absent-read scenarios: unique wrong
key, never-written key, effective delete, delete then rewrite, duplicate
expected values, an expected writer with unrelated writer, post-read writer,
and unexecuted-branch writer. Three precise automatic repair flows pass in
that corpus. A companion set proves three coordinated two-node key-construction
repairs. Duplicate and insufficient-evidence cases abstain safely.

The former 53-node Phase 3C case now emits explicit ambiguity with its two
executed, pre-read same-resource candidate writers and no editable nodes. It
is no longer an unexplained 0% recall result. The 26- and 109-node Phase 3C
repairs remain unchanged.

## Payload and limitations

Phase 3C payload measurements remain authoritative: accepted repair
transactions were 5,162 B versus 3,743 B full graph at 26 nodes and 19,576 B
versus 16,486 B at 109 nodes. Phase 3D adds no content-addressed transport and
does not claim byte savings. The ledger is intentionally narrow: only direct
Phase-1 map reads, scalar expected strings, and in-process host oracle tokens
are covered. It is not a general debugger or a durable provenance format.

## Recommendation

Localization now correctly proves negative state, repairs uniquely correlated
wrong-key and delete cases, and names unavoidable ambiguity. The next
architecture bottleneck is compact authenticated repair context: protected
hash context plus delta can still exceed full graph transport. Improvement #89
remains pending.
