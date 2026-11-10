# Compact graph repair session

The initial AI-DX3 interface is an internal Go API in `internal/hfir`, not an `agent` CLI command. It operates on the existing provider-neutral graph subset, not arbitrary production `.howl` programs.

The API is:

```go
NewCompactRepairSession(candidate Candidate, trusted RepairContext) (*CompactRepairSession, error)
session.RequestJSON() []byte
session.Apply(current Candidate, response []byte) (Candidate, []Diagnostic)
```

The request uses `hfir-compact-repair/v1`, a `context_id`, graph hash/version, editable node replacements with expected node hashes, allowed references and maximum operations. The response uses the same schema and context ID with an `operations` array in the existing `replace_node` format. Unknown fields, duplicate fields, trailing documents, invalid UTF-8 and unpaired Unicode surrogate escapes are rejected. Valid surrogate pairs and literal backslash text remain valid. The existing 1 MiB JSON limit applies.

Trusted code derives a repair region from existing diagnostic or executed-path machinery and creates an opaque session. The session validates and privately copies that authority. Models receive deterministic JSON with a context identifier, graph preconditions, editable node views and hashes, reference limits and operation bounds. Models return the identifier and existing replacement operations. Protected-node hashes stay local and are reconstructed into the existing v2 transaction before `ApplyRepair` performs atomic validation and recompilation.

The context digest is an identifier, not authentication material or permission. Exported JSON cannot create a trusted session. Callers must retain the private session and decide which trusted region to authorize; this API does not infer repair intent or task correctness. Identical original candidates may accept a repeated proposal; changed graphs invalidate it. No persistent replay ledger is provided.

Protected authority remains O(graph size) in local memory, with context cloning and hashing costs. Small wire payloads do not bypass the existing 1 MiB reconstructed legacy-delta bound. The new interface conservatively rejects type metadata deeper than 64 traversal levels.

Authority remains bounded by the existing 16-node region and five same-kind replacement limit, allowed references, graph and node preconditions, protected hashes and verifier/lowerer gate. Unknown or malformed transport fails closed. Caller mutation must not widen retained authority. Successful compilation is not proof of the requested behavior; run independent affected checks.

The initial process-local design avoids key storage and exposes less authority to transport, but cannot resume after process interruption. Authenticated durable restoration needs a separate trust/key/replay design. A standalone digest does not provide that guarantee.

This API never edits source files. Line/column provenance does not establish lossless `.howl` replacement spans, preserve comments/whitespace or prove source round trips. Arbitrary restructuring, dependency insertion, source editing and production AST-path changes remain deferred.

See the [measurement protocol](ai_dx3_measurement_protocol.md) and [implementation journal](../journals/2026-11-09_ai_dx3_compact_repair.md). No token, time or live-agent efficiency improvement is claimed.
