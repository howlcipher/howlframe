# Author Notes: Competitive-v1 Pilot Candidate Pool (Domains D3 & D4)

## Attestation

All candidate tasks in this pool were written new specifically for this competitive-v1 pilot pool. No candidate has been copied, adapted, or derived from any preexisting benchmark, course, coding kata, or HowlFrame repository.

---

## Domain Variation and Candidate Overview

The pool provides **14 candidates** across two domains: **7 candidates for D3** (Integration / Orchestration) and **7 candidates for D4** (Maintenance). Each candidate explores a distinct, realistic engineering scenario without thematic reuse:

### Domain D3: Integration / Orchestration (Greenfield)
Tasks in D3 require managing processes, timing, coordination, or communicating with local HTTP dependencies over loopback:

1. **`d3-pipeline-runner`**: Directed acyclic graph (DAG) subprocess execution with cycle validation, per-task timeouts, retry counts, and dependency failure skip logic.
2. **`d3-webhook-dispatcher`**: Event queue dispatcher sending JSON events over HTTP POST with HMAC-SHA256 signatures, bounded backoff retries on 5xx/timeouts, and delivery audit logging.
3. **`d3-sandbox-exec-policy`**: Role-based capability gatekeeper enforcing binary whitelists, argument bans, path jail boundaries, and environment sanitization prior to subprocess execution.
4. **`d3-distributed-lock-orchestrator`**: Distributed mutual exclusion coordinator acquiring leases via HTTP `/lock/acquire`, sending background renewal heartbeats during execution, and safely releasing the lock.
5. **`d3-token-vault-agent`**: Delegation token mediator checking subject/scope capability grants, minting scoped bearer tokens via a local token authority, and scrubbing leaked credentials from forwarded requests.
6. **`d3-parallel-test-matrix`**: Concurrent worker pool test harness orchestrating parameterized subprocess test executions with concurrency limits, deadlines, and fail-fast cancellation.
7. **`d3-cluster-health-aggregator`**: High-availability monitor probing primary service nodes over HTTP, determining operational health, and triggering standby activation failover requests.

### Domain D4: Maintenance
Tasks in D4 specify an evolution from a fully documented baseline application (`baseline_spec.md`) to an updated contract (`prompt.md`):

8. **`d4-inventory-allocator`**: Upgrades a simple FIFO warehouse allocator to customer priority tiers (`VIP`, `STANDARD`, `BULK`), FIFO lot selection by expiration date, and partial allocation with backorders.
9. **`d4-rbac-audit-filter`**: Upgrades a simple RBAC system to attribute-based access control with tenant isolation enforcement, resource ownership requirements, and time-bounded delegation tokens.
10. **`d4-log-retention-purger`**: Upgrades an age-based log purger to tag-based retention policies, disk storage quota trimming with compliance exemptions, and dry-run execution.
11. **`d4-api-key-permission-manager`**: Upgrades an API key store to enforce granular capability scopes, IPv4 and CIDR subnet whitelists, and key rotation with grace periods.
12. **`d4-invoice-tax-calculator`**: Upgrades a flat-tax invoicing engine to multi-category tax rates, luxury threshold surcharges, proportional discount distribution, and banker's penny rounding balancing.
13. **`d4-timeseries-anomaly-detector`**: Upgrades a rolling-window statistical aggregator to Z-score anomaly detection, forward-fill timestamp gap imputation, and alert throttling cooldowns.
14. **`d4-route-traffic-shaper`**: Upgrades a static path-prefix router to deterministic SHA-256 hash canary percentage traffic splitting, header-matching rules, and unhealthy backend fallback redirection.

---

## Authority Labels (`authority_centred`)

Per protocol §3.2, a candidate is marked `authority_centred: true` only if the task **centres on capability, permission, or access-control enforcement**. Routine input validation, simple status flags, or an authentication check in passing do not qualify.

The pool includes **exactly 2 authority-centred candidates per domain** (4 total across the 14 candidates), meeting the target requirement of at most 2 per domain:

### D3 Authority Classification:
- **`d3-sandbox-exec-policy` (`true`)**: The entire core of the task is capability and boundary enforcement: evaluating role privileges, checking execution whitelists, checking path escape boundaries, and sanitizing execution environments.
- **`d3-token-vault-agent` (`true`)**: The task centres on capability scoping, validating authorization policies, minting temporary delegated credentials, and scrubbing untrusted authorization headers to prevent privilege escalation.
- *The other 5 candidates (`d3-pipeline-runner`, `d3-webhook-dispatcher`, `d3-distributed-lock-orchestrator`, `d3-parallel-test-matrix`, `d3-cluster-health-aggregator`)* are marked `false` as they focus on DAG resolution, HTTP retries, concurrency, or lease heartbeats without access-control enforcement.

### D4 Authority Classification:
- **`d4-rbac-audit-filter` (`true`)**: The change request centres on security authorization: multi-tenant access boundaries, resource ownership constraints, and time-bounded delegation authority.
- **`d4-api-key-permission-manager` (`true`)**: The task centres on credential permissions: capability scopes (`*` or specific action scopes), network boundary enforcement (IP / CIDR whitelisting), and credential rotation lifecycle states.
- *The other 5 candidates (`d4-inventory-allocator`, `d4-log-retention-purger`, `d4-invoice-tax-calculator`, `d4-timeseries-anomaly-detector`, `d4-route-traffic-shaper`)* are marked `false` as they focus on business calculations, storage quotas, sliding window math, or traffic balancing without access-control enforcement.

---

## Language-Neutral Feasibility and Standard Library Compliance

Every candidate contract is completely language-neutral (CLI arguments, files, standard JSON schemas, and loopback HTTP). Every task has been verified to be implementable using only standard libraries:
- **Python**: `sys`, `os`, `json`, `subprocess`, `urllib.request`, `urllib.error`, `time`, `threading`, `hashlib`, `hmac`, `math`, `decimal`, `ipaddress`, `concurrent.futures`.
- **Go**: `os`, `os/exec`, `encoding/json`, `net/http`, `time`, `sync`, `crypto/sha256`, `crypto/hmac`, `math`, `sort`, `path/filepath`, `net`, `strings`.
- **JavaScript (Node.js)**: `node:fs`, `node:path`, `node:child_process`, `node:http`, `node:crypto`, `node:process`, `node:url`.

No third-party packages, external networks, SQL engines, compression libraries, or TLS are required. All floating-point ambiguities are mitigated by clear rounding specifications (banker's rounding) or integer-cent representations.

---

## Workload Generation

Every candidate includes a deterministic generator `workload/generate.py` that writes self-contained test input fixtures next to itself. The `task.json` includes `workload.args` that exercise each solution in one-shot mode from program start to exit, producing predictable runtime benchmarks.


---

## Revision 1

### Child Process Sandboxing and Canonical System Binaries

Following Revision Request 1, all acceptance tests, contracts, and workload generators that require subprocess execution were revised to remove dependencies on language interpreters (`python3`, `node`, `go`) and symlinked utilities that may not exist or may be rejected in language-isolated evaluation sandboxes (e.g., Go, JavaScript, and HowlFrame jails).

#### Remediations Applied:
1. **Canonical Absolute Binary Paths**:
   - Replaced all invocations of `python3`, `echo`, and relative commands in tests, contracts (`prompt.md`), and deterministic workloads (`workload/`) with real, non-symlink executable files from the shared Ubuntu base system:
     - Printing output: `["/usr/bin/coreutils", "echo", ...]`
     - Non-zero exits: `["/usr/bin/dash", "-c", "exit <code>"]`
     - Timers / sleeps: `["/usr/bin/coreutils", "sleep", "<duration>"]`
     - Helper scripts and environment inspection: `["/usr/bin/dash", "<path>"]` using dash shell built-ins (`echo`, `[ -f ... ]`, parameter expansion `${VAR:-DEFAULT}`).
2. **Environment Isolation Compliance**:
   - Guaranteed subprocess commands do not rely on `PATH` or inherited environment variables by using absolute binary paths and self-contained shell builtins for all assertions.
3. **Audited Candidates**:
   - Updated contracts, acceptance tests, workloads, and verified screen implementations for the four affected D3 integration tasks:
     - `d3-distributed-lock-orchestrator`
     - `d3-parallel-test-matrix`
     - `d3-pipeline-runner`
     - `d3-sandbox-exec-policy`
   - Verified that no other candidate across the entire pool (all remaining D3 and D4 candidates) launches `python3`, `node`, `go`, or any other interpreter.
4. **Verification Status**:
   - All 14 candidate tasks across D3 and D4 pass 100% of acceptance tests across both Python and JavaScript screen implementations.
