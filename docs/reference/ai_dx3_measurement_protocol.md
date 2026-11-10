# AI-DX3 compact graph repair preregistered protocol

Frozen before implementation, 2026-11-10 UTC. Baseline: 51edd94ea349f05a289e80190f51023e35aab5b5.

Compare the same existing localization corpus candidates, localization evidence,
trusted editable regions, replacements and behavioral assertions. Preserve the
recorded 26-, 53- and 109-node scenarios, including unsuccessful localization;
never widen authority to make a scenario repairable. No paid calls or live-agent
comparisons: zero live agents in benchmark execution. This is deterministic
transport measurement, not model efficiency measurement.

Serialize exact production structs, compact JSON without whitespace. Record full
canonical graph bytes; legacy context bytes plus legacy response-delta bytes;
compact request bytes plus compact response bytes; each component separately;
These are context+delta boundary totals only: common Task, Diagnostics, Metadata
and provider framing are excluded identically from both sides. Do not call these
complete provider requests or infer prompt/token savings;
retained local authority bytes; graph/core/protected counts; success/rejection;
constructor/apply elapsed time and repeated benchmark allocation costs. Include
schema, context ID, preconditions and all replacement content in totals. The full
graph is a regeneration proxy, not measured regeneration. Report every scenario,
including failures, and no token or overall efficiency claims. Do not assert every
small graph becomes smaller. Repeat timings with fixed Go/tool versions, matched
starting candidates and same verification gate; include construction, hashing,
JSON handling, authority retention, and full verification overhead.

Held-out correctness uses unchanged existing behavioral expectations and existing
HFIR verifier/lowering/artifact gate. New tests cover success, stale candidate,
wrong/cross-session IDs, altered authority, out-of-scope edits/references, protected
nodes, atomic multi-operation rejection, malformed/duplicate/unknown/trailing JSON,
UTF-8 and size bounds, and five-operation/same-kind restrictions. Repairs must
leave original candidates unchanged. No test expectation is weakened.

Scope: internal process-local opaque authority session only. Existing trusted
contexts and full protected hashes remain local. A digest identifies context and
is not standalone authentication; decoded request bytes cannot grant authority.
No persistent restoration, cryptographic signer, replay service, source edits,
disk writes, CLI contract, AST production path replacement, incremental compiler,
new execution grants, dependency insertion, or widened region limits.
