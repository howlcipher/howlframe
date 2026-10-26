# Screening Log: Candidate Pool Evolution and Screening Decisions

This log documents every task candidate considered during the authoring of the competitive-v1 pilot candidate pool for domains **D1 (CLI data processing)** and **D2 (HTTP JSON service)**, including candidates accepted into the pool and candidate concepts that were evaluated and dropped during screening.

---

## Accepted Candidates (14 Total)

### Domain D1: CLI Data Processing (Greenfield)

1. **`d1-sensor-telemetry`**
   - **Scenario**: IoT greenhouse environmental sensor logs auditing.
   - **Contract**: CLI `audit --data <file.jsonl> --config <file.json> --out <report.json> [--summary]`.
   - **Authority Centred**: `false`.
   - **Screening Result**: Passed. Python: 13/13 passed. JavaScript: 13/13 passed.
   - **Feasibility**: High. Clean stdlib JSON parsing, threshold bounds checking, deterministic sorting.

2. **`d1-logistics-manifest`**
   - **Scenario**: Freight shipping container packing and load balancing.
   - **Contract**: CLI `pack --packages <file.csv> --rules <file.json> --output <file.json>`.
   - **Authority Centred**: `false`.
   - **Screening Result**: Passed. Python: 13/13 passed. JavaScript: 13/13 passed.
   - **Feasibility**: High. Standard CSV parsing, multi-key greedy packing algorithm, constraint satisfaction.

3. **`d1-ledger-reconcile`**
   - **Scenario**: Dual-ledger financial transaction journal reconciliation.
   - **Contract**: CLI `reconcile --internal <file.json> --external <file.csv> --out <file.json>`.
   - **Authority Centred**: `false`.
   - **Screening Result**: Passed. Python: 13/13 passed. JavaScript: 13/13 passed.
   - **Feasibility**: High. Discrepancy isolation, duplicate detection, float rounding consistency.

4. **`d1-package-sbom`**
   - **Scenario**: Software Bill of Materials (SBOM) license and system capability permission scanner.
   - **Contract**: CLI `verify-sbom --sbom <file.json> --policy <file.json> --report <file.json>`.
   - **Authority Centred**: `true` (Authority Candidate #1 for D1).
   - **Screening Result**: Passed. Python: 13/13 passed. JavaScript: 13/13 passed.
   - **Feasibility**: High. Focuses on capability access boundaries, privilege containment, license authorization.

5. **`d1-rbac-audit`**
   - **Scenario**: Cloud IAM permission boundary and privilege escalation path auditor.
   - **Contract**: CLI `audit-iam --iam <file.json> --boundaries <file.json> --out <file.json>`.
   - **Authority Centred**: `true` (Authority Candidate #2 for D1).
   - **Screening Result**: Passed. Python: 13/13 passed. JavaScript: 13/13 passed.
   - **Feasibility**: High. Evaluates wildcard permission boundaries, toxic escalation pairs, and session limits.

6. **`d1-gradebook-stats`**
   - **Scenario**: Academic course gradebook calculations, curving, and transcripts.
   - **Contract**: CLI `grade --students <file.json> --grades <file.csv> --rubric <file.json> --out <file.json>`.
   - **Authority Centred**: `false`.
   - **Screening Result**: Passed. Python: 13/13 passed. JavaScript: 13/13 passed.
   - **Feasibility**: High. Arithmetic weighting, progressive late penalty deductions, grade scale distribution.

7. **`d1-metric-timeseries`**
   - **Scenario**: Cloud infrastructure telemetry timeseries downsampler and p95 rollups.
   - **Contract**: CLI `rollup --input <file.jsonl> --interval <seconds> --output <file.json> [--host <host>]`.
   - **Authority Centred**: `false`.
   - **Screening Result**: Passed. Python: 13/13 passed. JavaScript: 13/13 passed.
   - **Feasibility**: High. Epoch window bucketing, nearest-rank p95 percentile, statistical aggregation.

---

### Domain D2: HTTP JSON Service (Greenfield)

8. **`d2-rate-limiter`**
   - **Scenario**: API Gateway token-bucket rate limiter and tier quota authority.
   - **Contract**: HTTP server `--port <P> --store <S>` and batch replay `./run replay --store <S> <reqs.jsonl>`.
   - **Authority Centred**: `true` (Authority Candidate #1 for D2).
   - **Screening Result**: Passed. Python: 12/12 passed. JavaScript: 12/12 passed.
   - **Feasibility**: High. Continuous token refill, scope permissions, 401/403/429 status codes, persistence.

9. **`d2-feature-flag`**
   - **Scenario**: Feature entitlement and capability governance service.
   - **Contract**: HTTP server `--port <P> --store <S>` and batch replay `./run replay --store <S> <reqs.jsonl>`.
   - **Authority Centred**: `true` (Authority Candidate #2 for D2).
   - **Screening Result**: Passed. Python: 13/13 passed. JavaScript: 13/13 passed.
   - **Feasibility**: High. Multi-tier license hierarchy, role authorization, user override bypasses.

10. **`d2-kv-store`**
    - **Scenario**: Versioned key-value store with TTL and optimistic concurrency control (CAS).
    - **Contract**: HTTP server `--port <P> --store <S>` and batch replay `./run replay --store <S> <reqs.jsonl>`.
    - **Authority Centred**: `false`.
    - **Screening Result**: Passed. Python: 13/13 passed. JavaScript: 13/13 passed.
    - **Feasibility**: High. Optimistic locking via `expected_version`, time-to-live expirations, prefix search.

11. **`d2-job-queue`**
    - **Scenario**: Priority task queue broker with lease acknowledgments and dead-letter queues.
    - **Contract**: HTTP server `--port <P> --store <S>` and batch replay `./run replay --store <S> <reqs.jsonl>`.
    - **Authority Centred**: `false`.
    - **Screening Result**: Passed. Python: 13/13 passed. JavaScript: 13/13 passed.
    - **Feasibility**: High. Priority scheduling, worker leasing, retry limits, lease timeout re-queuing.

12. **`d2-order-book`**
    - **Scenario**: Commodity matching engine order book under price-time priority.
    - **Contract**: HTTP server `--port <P> --store <S>` and batch replay `./run replay --store <S> <reqs.jsonl>`.
    - **Authority Centred**: `false`.
    - **Screening Result**: Passed. Python: 14/14 passed. JavaScript: 14/14 passed.
    - **Feasibility**: High. Double auction order matching, resting depth aggregation, trade execution history.

13. **`d2-event-stream`**
    - **Scenario**: Append-only event stream topic broker with consumer group offsets.
    - **Contract**: HTTP server `--port <P> --store <S>` and batch replay `./run replay --store <S> <reqs.jsonl>`.
    - **Authority Centred**: `false`.
    - **Screening Result**: Passed. Python: 13/13 passed. JavaScript: 13/13 passed.
    - **Feasibility**: High. Monotonic offset assignment, consumer group cursor tracking, retention trimming.

14. **`d2-cache-invalidator`**
    - **Scenario**: Distributed cache tag index and surrogate multi-tag purge coordinator.
    - **Contract**: HTTP server `--port <P> --store <S>` and batch replay `./run replay --store <S> <reqs.jsonl>`.
    - **Authority Centred**: `false`.
    - **Screening Result**: Passed. Python: 13/13 passed. JavaScript: 13/13 passed.
    - **Feasibility**: High. Tag set intersections, batch invalidation, TTL cleanup, purge metrics.

---

## Dropped Candidates and Rationale

During pool ideation and initial feasibility screening, the following candidate ideas were evaluated and dropped:

1. **`d1-exif-image-scrubber` (D1 CLI)**
   - *Concept*: Read JPEG/PNG image binary headers, strip GPS and camera serial tags, write sanitized images.
   - *Reason Dropped*: Violates Rule 2 ("no compression or image codecs, no third-party packages"). Python standard library does not parse EXIF without PIL/Pillow or raw binary hacks that cannot be cleanly replicated in Go or Node.js within 45 minutes.

2. **`d1-gzip-log-parser` (D1 CLI)**
   - *Concept*: Stream compressed `.log.gz` archives and filter HTTP error patterns.
   - *Reason Dropped*: Rule 2 explicitly advises against compression codecs. While `gzip` is in Python's standard library and `node:zlib` in Node, differences in streaming EOF handling, buffer chunking, and memory allocation create cross-language inconsistencies for zero algorithmic gain.

3. **`d1-yaml-to-json-converter` (D1 CLI)**
   - *Concept*: Parse YAML configuration files with anchors and emit canonical JSON.
   - *Reason Dropped*: Neither Python standard library nor Node.js standard library contains a native YAML parser. Fails Rule 2 feasibility completely.

4. **`d1-decimal-currency-arbitrage` (D1 CLI)**
   - *Concept*: Arbitrage loop detector requiring 128-bit fixed-point decimal arithmetic without IEEE 754 float inaccuracies.
   - *Reason Dropped*: Python has `decimal.Decimal`, Go has `math/big`, but Node.js standard library lacks a Decimal type (only BigInt and Number), forcing floating-point roundoff issues that violate Rule 2 ("prefer data rules every standard JSON parser handles the same way").

5. **`d2-tls-proxy-auth` (D2 Service)**
   - *Concept*: Reverse proxy enforcing mutual TLS client certificate verification.
   - *Reason Dropped*: Rule 2 explicitly rules out TLS ("no TLS, no external network"). Certificate generation, handshake configuration, and local trust stores introduce extreme platform friction without measuring core agent reasoning.

6. **`d2-sqlite-relational-store` (D2 Service)**
   - *Concept*: REST service backed by an embedded SQLite database file.
   - *Reason Dropped*: Rule 2 states "No SQL, no third-party packages... using only files, an HTTP server, an HTTP client, subprocesses, JSON and local file persistence". Go's `database/sql` requires an external driver (like `mattn/go-sqlite3` which requires CGO), making it non-neutral across arms.

7. **`d2-websocket-chat-hub` (D2 Service)**
   - *Concept*: Loopback real-time chat broker using WebSocket framing.
   - *Reason Dropped*: Python's standard library (`http.server` / `asyncio`) does not provide a WebSocket handshake and frame parser out of the box (requires the third-party `websockets` library). Violates standard-library feasibility.

8. **`d2-jwt-token-verifier` (D2 Service)**
   - *Concept*: API service verifying RSA/ECDSA signed JWTs and evaluating embedded claim permissions.
   - *Reason Dropped*: Asymmetric cryptographic signature decoding and ASN.1/PEM parsing across Node.js `crypto`, Python `ssl`/`cryptography`, and Go `crypto/x509` have subtle incompatibilities and padding quirks that exceed the 30-90 minute target size. Replaced by `d2-rate-limiter` and `d2-feature-flag`, which isolate authority and capability logic without cryptographic baggage.
