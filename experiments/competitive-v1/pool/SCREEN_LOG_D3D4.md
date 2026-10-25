# Candidate Screening Log

This document records all candidate tasks considered during the pilot candidate pool authoring process for domains **D3** (Integration / Orchestration) and **D4** (Maintenance), including candidates dropped during feasibility and design evaluation.

---

## Dropped Candidates

1. **Candidate: Git Repository Hook & Branch Merge Coordinator (D3)**
   - *Concept*: Orchestrate multi-branch git hooks and verify signature commits.
   - *Reason Dropped*: Exceeds standard-library-only constraints; requires `git` binary and git repository state setup, which introduces platform dependencies and environmental variability across non-POSIX test runners.

2. **Candidate: SQLite WAL File Replication Daemon (D3)**
   - *Concept*: Replicate database WAL frames over HTTP to a backup peer.
   - *Reason Dropped*: Violates Rule 2 ("No SQL, no third-party packages"). Go standard library does not include a builtin SQLite driver without CGO/external packages.

3. **Candidate: PNG Metadata and Watermark Pipeline (D1/D3)**
   - *Concept*: Pipe binary image assets through an external compression and metadata watermarking process.
   - *Reason Dropped*: Violates Rule 2 ("no compression or image codecs"). Standard libraries across Go, Python, and Node have inconsistent native support for raw image manipulations without third-party libraries (e.g. Pillow, Sharp).

4. **Candidate: TLS Certificate Mutual Authentication Proxy (D3)**
   - *Concept*: Enforce mTLS client certificate verification when forwarding HTTP requests.
   - *Reason Dropped*: Violates Rule 2 ("No TLS, no external network"). The benchmark protocol explicitly specifies plaintext HTTP on loopback (127.0.0.1) without TLS.

5. **Candidate: Full OAuth 2.0 PKCE Authorization Server (D3 / D4)**
   - *Concept*: Complete OAuth2 authorization code exchange with proof key for code exchange.
   - *Reason Dropped*: Too large for the target duration (30-90 minutes human, <45 minutes agent). Scope reduced and focused cleanly into `d3-token-vault-agent`.

6. **Candidate: Multi-Archive Gzip Deduplication Tool (D4)**
   - *Concept*: Maintenance patch to an archive compressor to deduplicate tar/gzip blocks.
   - *Reason Dropped*: Violates Rule 2 ("no compression or image codecs").

---

## Accepted Candidates (14 Total)

### Domain D3: Integration / Orchestration

1. **`d3-pipeline-runner`**
   - *Summary*: Orchestrates dependency-ordered subprocess tasks with per-task deadlines, retry limits, and status reporting.
   - *Authority-Centred*: `false`.
   - *Feasibility*: Python (`subprocess`), Go (`os/exec`), Node (`node:child_process`). Standard library only.
   - *Screening*: Passed acceptance test suite in both Python and JavaScript.

2. **`d3-webhook-dispatcher`**
   - *Summary*: Dispatches queued JSON webhook payloads to an HTTP endpoint with HMAC-SHA256 signatures, bounded backoff retries, and delivery audit logging.
   - *Authority-Centred*: `false`.
   - *Feasibility*: Python (`urllib.request`, `hmac`), Go (`net/http`, `crypto/hmac`), Node (`node:http`, `node:crypto`). Standard library only.
   - *Screening*: Passed acceptance test suite in both Python and JavaScript.

3. **`d3-sandbox-exec-policy`**
   - *Summary*: Executes role-based command requests against strict capability policies, enforcing binary whitelists, argument bans, path jail boundaries, and security audit logs.
   - *Authority-Centred*: `true`.
   - *Feasibility*: Python (`subprocess`), Go (`os/exec`), Node (`node:child_process`). Standard library only.
   - *Screening*: Passed acceptance test suite in both Python and JavaScript.

4. **`d3-distributed-lock-orchestrator`**
   - *Summary*: Coordinates critical tasks via an HTTP distributed lock coordinator, maintaining lease renewal heartbeats and safe release upon termination.
   - *Authority-Centred*: `false`.
   - *Feasibility*: Python (`urllib.request`, `threading`, `subprocess`), Go (`net/http`, `sync`, `os/exec`), Node (`node:http`, `node:child_process`). Standard library only.
   - *Screening*: Passed acceptance test suite in both Python and JavaScript.

5. **`d3-token-vault-agent`**
   - *Summary*: Mediates downstream HTTP service requests by validating capability scopes against authority grants, minting delegation bearer tokens from a local token vault, and forwarding requests with sanitized credentials.
   - *Authority-Centred*: `true`.
   - *Feasibility*: Python (`urllib.request`), Go (`net/http`), Node (`node:http`). Standard library only.
   - *Screening*: Passed acceptance test suite in both Python and JavaScript.

6. **`d3-parallel-test-matrix`**
   - *Summary*: Executes a parameterized test command matrix across environments with bounded worker concurrency, per-case deadlines, fail-fast thresholds, and structured summary reporting.
   - *Authority-Centred*: `false`.
   - *Feasibility*: Python (`subprocess`, `concurrent.futures`), Go (`os/exec`, `sync`), Node (`node:child_process`). Standard library only.
   - *Screening*: Passed acceptance test suite in both Python and JavaScript.

7. **`d3-cluster-health-aggregator`**
   - *Summary*: Polls health endpoints across a distributed service cluster, aggregates availability status, and dispatches activation requests to standby instances when primaries fail.
   - *Authority-Centred*: `false`.
   - *Feasibility*: Python (`urllib.request`), Go (`net/http`), Node (`node:http`). Standard library only.
   - *Screening*: Passed acceptance test suite in both Python and JavaScript.

---

### Domain D4: Maintenance

8. **`d4-inventory-allocator`**
   - *Summary*: Upgrades inventory allocation engine from basic FIFO to customer priority tiering, expiry-date lot selection, and backorder splitting.
   - *Authority-Centred*: `false`.
   - *Feasibility*: Pure JSON and sorting in Python, Go, and Node standard libraries.
   - *Screening*: Passed acceptance test suite in both Python and JavaScript.

9. **`d4-rbac-audit-filter`**
   - *Summary*: Upgrades an RBAC access policy evaluator to support multi-tenant boundaries, resource ownership constraints, and time-bounded delegation tokens.
   - *Authority-Centred*: `true`.
   - *Feasibility*: Pure JSON data structures and string/set operations in Python, Go, and Node standard libraries.
   - *Screening*: Passed acceptance test suite in both Python and JavaScript.

10. **`d4-log-retention-purger`**
    - *Summary*: Upgrades a basic log cleaner with tag-based retention policies, disk storage quota enforcement with compliance exemptions, and dry-run execution.
    - *Authority-Centred*: `false`.
    - *Feasibility*: File system scanning and JSON parsing in Python, Go, and Node standard libraries.
    - *Screening*: Passed acceptance test suite in both Python and JavaScript.

11. **`d4-api-key-permission-manager`**
    - *Summary*: Upgrades an API key validator with capability scopes, IP whitelist restrictions, and key rotation grace-period handling.
    - *Authority-Centred*: `true`.
    - *Feasibility*: Standard library JSON, string, and integer/IP logic in Python, Go, and Node.
    - *Screening*: Passed acceptance test suite in both Python and JavaScript.

12. **`d4-invoice-tax-calculator`**
    - *Summary*: Upgrades invoice computation engine to multi-category tax rates, luxury surcharges, proportional discount distribution, and banker's penny rounding balancing.
    - *Authority-Centred*: `false`.
    - *Feasibility*: Standard library math and rounding algorithms across Python, Go, and Node.
    - *Screening*: Passed acceptance test suite in both Python and JavaScript.

13. **`d4-timeseries-anomaly-detector`**
    - *Summary*: Upgrades a time-series window statistics calculator with dynamic Z-score anomaly detection, forward-fill timestamp gap imputation, and cooldown-based alert suppression.
    - *Authority-Centred*: `false`.
    - *Feasibility*: Standard library sliding window statistics and math in Python, Go, and Node.
    - *Screening*: Passed acceptance test suite in both Python and JavaScript.

14. **`d4-route-traffic-shaper`**
    - *Summary*: Upgrades a route dispatch engine with deterministic hash canary splitting, header matching conditions, and unhealthy backend fallback redirection.
    - *Authority-Centred*: `false`.
    - *Feasibility*: Standard library SHA-256 and JSON parsing in Python, Go, and Node.
    - *Screening*: Passed acceptance test suite in both Python and JavaScript.
