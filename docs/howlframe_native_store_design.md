# HowlFrame Native Store Design

## Summary

HowlFrame already has database support through `db_connect`, `sql_query`, `schema`, the Go backend, and the bytecode VM. That is useful, but it is still a traditional SQL escape hatch: the AI must write SQL strings, reason about driver behavior, and accept target-specific runtime semantics.

The missing feature is not "add a database" in the normal application-stack sense. The missing feature is a HowlFrame-native persistence and retrieval substrate that an AI can operate through structured bytecode instructions.

Recommended direction: build an embedded HowlFrame store in the bytecode VM first, with SQL remaining as an interoperability escape hatch.

## Current Evidence

| Existing surface | Evidence | Limitation |
| --- | --- | --- |
| SQL connection primitive | `internal/bytecode/opcode.go` defines `DB_CONNECT` with the `database` capability | Connection is external and driver-defined |
| SQL query primitive | `internal/bytecode/opcode.go` defines `SQL_QUERY`; `internal/vm/vm.go` executes it through `database/sql` | Query is an opaque string to the compiler |
| Declarative schema | `internal/backend/gogen/gogen.go` expands schema DDL around `db_connect` | Schema still lowers to SQL tables |
| Bytecode VM | `-compile-bc` and `-run-bc` execute serialized HowlFrame instructions | Database behavior is not yet HowlFrame-native |

The live source shows that HowlFrame has persistence plumbing, but not a compiler-visible data model that replaces SQL or NoSQL for AI-authored programs.

## Design Goals

1. Keep the source grammar small and S-expression friendly.
2. Avoid SQL-string generation for normal agent workflows.
3. Make persistence visible to the semantic checker and bytecode verifier.
4. Support backend, CLI, and web-adjacent workflows through one data model.
5. Preserve SQL as an optional import/export target, not the primary abstraction.
6. Make runtime behavior deterministic by default, with explicit semantic retrieval when requested.

## Proposed Surface

The first useful surface should be four concepts: stores, records, indexes, and queries.

```lisp
(cli_app
  (store_open memory "memory://session")

  (store_put memory "task:1"
    (dict
      ("kind" "task")
      ("title" "Add login")
      ("status" "open")
      ("priority" "high")
    )
  )

  (let (open_tasks
    (store_query memory
      (where (= "kind" "task") (= "status" "open"))
      (order "priority" "desc")
      (limit 10)
    ))
    (print open_tasks)
  )
)
```

This should not be a SQL clone. The query form should be an AST the compiler can inspect, type check, and lower to bytecode directly.

## Bytecode Shape

Add store opcodes instead of overloading `SQL_QUERY`:

| Opcode | Purpose |
| --- | --- |
| `STORE_OPEN` | Create or attach a named store handle |
| `STORE_PUT` | Upsert a record by key |
| `STORE_GET` | Fetch a record by key |
| `STORE_DELETE` | Delete a record by key |
| `STORE_QUERY` | Evaluate a structured predicate over indexed records |
| `STORE_INDEX` | Declare an exact, range, full-text, or semantic index |
| `STORE_TXN` | Execute a bounded transaction block |

The VM can implement these against an embedded engine without exposing that engine as language syntax. Possible internal engines include a simple append-only log plus in-memory indexes, SQLite used only as a storage engine, or a dedicated embedded key-value library. The bytecode contract should remain stable even if the engine changes.

## Candidate Architectures

| Option | Pros | Cons |
| --- | --- | --- |
| Keep only SQL primitives | Already shipped, broad ecosystem, easy interoperability | AI still writes opaque strings; weak compiler validation; does not replace SQL/NoSQL |
| Wrap SQLite behind HowlFrame store opcodes | Durable quickly; transactions and indexing are mature; small implementation risk | Still inherits relational storage internally; care needed to avoid leaking SQL concepts into HowlFrame syntax |
| Build append-only document store in VM | Strongest HowlFrame identity; simple mental model; bytecode-native; easy audit trail | More work for queries, compaction, concurrency, and durability guarantees |
| Use external NoSQL or vector database | Rich retrieval features; scalable if needed | Adds deployment burden; not embedded; weak fit for HowlFrame's small self-contained toolchain |

Recommendation: start with a bytecode-native API backed by an embedded implementation. SQLite can be a private implementation detail for durability, but HowlFrame programs should see records, predicates, indexes, and transactions, not tables and SQL strings.

## Query Model

Use structured query nodes that map cleanly to bytecode:

```lisp
(store_query memory
  (where
    (= "kind" "task")
    (in "status" (list "open" "blocked"))
    (> "priority_score" 5)
  )
  (select "title" "status" "priority")
  (limit 20)
)
```

For AI-native retrieval, add semantic search as an explicit index and predicate:

```lisp
(store_index memory "notes" (semantic "summary"))

(store_query memory
  (where (semantic_near "summary" "authentication bug reports"))
  (limit 5)
)
```

Semantic retrieval should be opt-in because it is probabilistic, may require embeddings or LLM calls, and should be covered by the existing capability model.

## Safety And Verification

The store needs stricter boundaries than the existing SQL escape hatch:

- Enforce a `database` or new `store` capability at bytecode verification time.
- Reject unsupported query operators during semantic checking.
- Bound result size with a default limit.
- Make transaction blocks time-bounded in the VM.
- Keep semantic indexes separate from exact indexes.
- Emit structured runtime errors rather than panics.

## Recommended Phases

| Phase | Deliverable | Verification |
| --- | --- | --- |
| 1 | Design and backlog the HowlFrame store surface | Documentation links and backlog entry exist |
| 2 | Implement in-memory `store_open`, `store_put`, `store_get`, `store_delete` in bytecode VM | Bytecode fixture compiles and runs through `-run-bc` |
| 3 | Add `store_query` with exact predicates, `limit`, and simple indexes | VM tests prove deterministic filtering and ordering |
| 4 | Add durable storage behind the same bytecode contract | Restart test proves persistence |
| 5 | Add semantic indexes and `semantic_near` as an explicit AI capability | Tests distinguish deterministic and semantic retrieval paths |
| 6 | Add SQL import/export helpers | Round-trip tests against existing `db_connect`/`sql_query` path |

## Recommendation

Build this as a HowlFrame-native store in bytecode, not as more SQL syntax.

The key architectural move is to make persistence part of HowlFrame's validated instruction set: records are structured values, queries are AST nodes, indexes are declarations, and storage engines are implementation details. That gives the AI a replacement for SQL and NoSQL at the language level while preserving the option to interoperate with traditional databases when needed.
