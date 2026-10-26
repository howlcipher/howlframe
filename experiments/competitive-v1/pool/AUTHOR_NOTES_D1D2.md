# Author Notes: Competitive-V1 Pilot Candidate Pool (Domains D1 & D2)

## Attestation

Every task, specification, test suite, and screening implementation in this pool was written new by the author for this experiment. No task was copied or adapted from any existing benchmark (e.g. HumanEval, SWE-bench, MBPP), online course, kata repository, or HowlFrame codebase.

---

## Domain Variation & Diversity Matrix

To ensure the pilot evaluation tests broad agent reasoning and does not overfit to a single domain pattern, the 14 candidates span 14 distinct problem domains with zero reused scenarios:

| ID | Domain | Title | Primary Focus | Input / Protocol |
| :--- | :---: | :--- | :--- | :--- |
| `d1-sensor-telemetry` | D1 | Greenhouse Sensor Telemetry Auditor | Time-series IoT validation & anomaly detection | JSONL data + JSON config |
| `d1-logistics-manifest` | D1 | Freight Container Packing Optimizer | Multi-constraint bin-packing & safety rules | CSV manifest + JSON rules |
| `d1-ledger-reconcile` | D1 | Dual-Ledger Financial Reconciliation | Financial discrepancy & mismatch matching | JSON journal + CSV statement |
| `d1-package-sbom` | D1 | SBOM Capability & License Policy Verifier | Security privilege boundary enforcement | JSON SBOM + JSON policy |
| `d1-rbac-audit` | D1 | Cloud IAM Access Control & Escalation Auditor | Toxic privilege escalation & boundary containment | JSON IAM + JSON boundaries |
| `d1-gradebook-stats` | D1 | Academic Gradebook & Distribution Analyzer | Weighted scoring, curving, transcripts | JSON roster + CSV grades + JSON rubric |
| `d1-metric-timeseries` | D1 | Metrics Timeseries Downsampler | Downsampling, bucket windows, p95 nearest rank | JSONL telemetry |
| `d2-rate-limiter` | D2 | API Gateway Rate Limiter & Quota Authority | Token-bucket quotas & route scope authorization | HTTP REST + Replay CLI |
| `d2-feature-flag` | D2 | Feature Entitlement Governance Engine | Gating, role authorization, tenant tier levels | HTTP REST + Replay CLI |
| `d2-kv-store` | D2 | Versioned Key-Value Store with TTL | Optimistic concurrency (CAS) & expiration | HTTP REST + Replay CLI |
| `d2-job-queue` | D2 | Priority Job Queue Broker with Leasing | Priority scheduling, worker leasing, retries, DLQ | HTTP REST + Replay CLI |
| `d2-order-book` | D2 | Commodity Matching Engine Order Book | Double auction, price-time matching, trade logging | HTTP REST + Replay CLI |
| `d2-event-stream` | D2 | Append-Only Event Stream Topic Broker | Partitioned logs, monotonic offsets, cursor commits | HTTP REST + Replay CLI |
| `d2-cache-invalidator` | D2 | Cache Tag Index & Purge Coordinator | Multi-tag surrogate invalidation, purge tracking | HTTP REST + Replay CLI |

---

## Authority Labels and Rationale

The experimental protocol (§3.2) requires marking `authority_centred: true` if and only if the core intellectual problem of the task centres on capability, permission, privilege boundary, or access-control enforcement. Ordinary validation, schema checking, or an authentication token check in passing do NOT qualify a task as authority-centred.

In accordance with the guideline to aim for at most 2 authority-centred tasks per domain:

### Domain D1 Authority Candidates (2 / 7)
1. **`d1-package-sbom` (`authority_centred: true`)**:
   - *Why*: The entire task evaluates security capability grants (`network`, `process_spawn`, `filesystem_write`) against trust tier boundaries and checks license authorization policies for dependencies. The logic directly models principle-of-least-privilege enforcement.
2. **`d1-rbac-audit` (`authority_centred: true`)**:
   - *Why*: The task centres on finding toxic privilege escalation paths (such as `iam:PassRole` paired with `lambda:CreateFunction`) and identifying wildcard permission escapes that pierce organization boundary limits.
- *Non-authority D1 candidates*:
  - `d1-sensor-telemetry` (validation & statistical thresholding)
  - `d1-logistics-manifest` (greedy multi-dimensional bin packing)
  - `d1-ledger-reconcile` (relational record matching & diffing)
  - `d1-gradebook-stats` (weighted arithmetic & statistical distribution)
  - `d1-metric-timeseries` (downsampling & mathematical percentiles)

### Domain D2 Authority Candidates (2 / 7)
1. **`d2-rate-limiter` (`authority_centred: true`)**:
   - *Why*: Evaluates route scope capabilities (`read`, `write`, `admin`), authenticates client keys, and governs token quota allowances per subscription tier to protect downstream resources from unauthorized or excessive invocation.
2. **`d2-feature-flag` (`authority_centred: true`)**:
   - *Why*: Functions as a centralized entitlement engine evaluating subject authority against subscription tier hierarchies, role permission matrices, and explicit user privilege override grants.
- *Non-authority D2 candidates*:
  - `d2-kv-store` (data persistence, version incrementing, TTL timer expiration)
  - `d2-job-queue` (task queuing, FIFO/priority dispatching, leasing timeouts)
  - `d2-order-book` (financial matching engine, price-time limit order matching)
  - `d2-event-stream` (append-only log publishing, cursor offsets, topic retention)
  - `d2-cache-invalidator` (multi-tag surrogate cache index, purge coordination)

---

## Cross-Language Feasibility and Neutrality

1. **Standard Library Purity**:
   - Every candidate is 100% solvable in Python 3.12+, Go 1.25+, and Node.js 22+ using only standard library modules.
   - Standard HTTP servers (`http.server` in Python, `net/http` in Go, `node:http` in Node) and clients are used throughout D2.
   - Persistence is strictly JSON file-based on loopback/local filesystem. No SQL databases, no CGO bindings, no third-party libraries.
2. **Deterministic Data Rules**:
   - Numerical values are compared and persisted by value, avoiding language-dependent distinctions between `1` and `1.0`.
   - String comparisons and sorting specify explicit lexicographical / code-point ordering.
   - Percentile computations specify the exact nearest-rank formulation (`ceil(0.95 * count) - 1`).
   - Rounding rules specify exact decimal precision (2 decimal places for financial amounts and percentages).
3. **Dual Execution Modes for Services (D2)**:
   - Every D2 candidate supports both a long-running HTTP server mode (`--port <PORT>`) and an offline batch replay mode (`./run replay ... <requests.jsonl>`) that prints responses line-by-line to standard output. This satisfies the requirement for a fixed-input one-shot execution mode that can be reliably benchmarked from program start to exit.
