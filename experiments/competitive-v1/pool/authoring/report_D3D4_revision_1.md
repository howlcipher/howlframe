# Revision 1 Report: Subprocess Sandboxing & Canonical Paths

In accordance with [REVISION_1.md](REVISION_1.md), all candidates in `pool/` have been audited and updated to ensure that child processes invoked in acceptance tests and workloads exist in every language evaluation jail (Python, Go, JavaScript, and HowlFrame) and do not depend on language toolchains, symlinked utilities, or inherited `PATH` variables.

---

### Key Remediations

1. **Canonical Base Binaries**:
   - Replaced all invocations of `python3`, `echo`, and relative commands in tests, contracts, and deterministic workloads with canonical non-symlinked base executables:
     - Print: `["/usr/bin/coreutils", "echo", ...]`
     - Non-zero exit: `["/usr/bin/dash", "-c", "exit <code>"]`
     - Timers / sleeps: `["/usr/bin/coreutils", "sleep", "<seconds>"]`
     - Shell scripts & environment inspection: `["/usr/bin/dash", "<script.sh>"]` utilizing dash built-ins (`echo`, `[ -f ... ]`, `${VAR:-DEFAULT}`).
2. **Contracts Updated**:
   - Updated command examples in [`prompt.md`](pool/d3-distributed-lock-orchestrator/prompt.md) for all four affected integration candidates (`d3-distributed-lock-orchestrator`, `d3-parallel-test-matrix`, `d3-pipeline-runner`, and `d3-sandbox-exec-policy`).
3. **Workload Fixtures Updated**:
   - Updated `workload/generate.py` and regenerated all workload JSON inputs to use canonical base binaries.
4. **Interpreter Audit**:
   - Audited all candidate tasks across domains D3 and D4. Confirmed that no other candidate has any test that launches `python3`, `node`, `go`, or any interpreter.
5. **Author Notes**:
   - Appended a `"Revision 1"` section to [`pool/AUTHOR_NOTES.md`](pool/AUTHOR_NOTES.md).

---

### Candidate Verification Summary

| Candidate ID | Domain | Authority Centred | Acceptance Tests | Python Screen | JavaScript Screen |
| :--- | :---: | :---: | :---: | :---: | :---: |
| [`d3-cluster-health-aggregator`](pool/d3-cluster-health-aggregator) | D3 | `false` | 11 | **PASS** (11/11) | **PASS** (11/11) |
| [`d3-distributed-lock-orchestrator`](pool/d3-distributed-lock-orchestrator) | D3 | `false` | 11 | **PASS** (11/11) | **PASS** (11/11) |
| [`d3-parallel-test-matrix`](pool/d3-parallel-test-matrix) | D3 | `false` | 11 | **PASS** (11/11) | **PASS** (11/11) |
| [`d3-pipeline-runner`](pool/d3-pipeline-runner) | D3 | `false` | 12 | **PASS** (12/12) | **PASS** (12/12) |
| [`d3-sandbox-exec-policy`](pool/d3-sandbox-exec-policy) | D3 | `true` | 11 | **PASS** (11/11) | **PASS** (11/11) |
| [`d3-token-vault-agent`](pool/d3-token-vault-agent) | D3 | `true` | 10 | **PASS** (10/10) | **PASS** (10/10) |
| [`d3-webhook-dispatcher`](pool/d3-webhook-dispatcher) | D3 | `false` | 11 | **PASS** (11/11) | **PASS** (11/11) |
| [`d4-api-key-permission-manager`](pool/d4-api-key-permission-manager) | D4 | `true` | 12 | **PASS** (12/12) | **PASS** (12/12) |
| [`d4-inventory-allocator`](pool/d4-inventory-allocator) | D4 | `false` | 11 | **PASS** (11/11) | **PASS** (11/11) |
| [`d4-invoice-tax-calculator`](pool/d4-invoice-tax-calculator) | D4 | `false` | 12 | **PASS** (12/12) | **PASS** (12/12) |
| [`d4-log-retention-purger`](pool/d4-log-retention-purger) | D4 | `false` | 12 | **PASS** (12/12) | **PASS** (12/12) |
| [`d4-rbac-audit-filter`](pool/d4-rbac-audit-filter) | D4 | `true` | 11 | **PASS** (11/11) | **PASS** (11/11) |
| [`d4-route-traffic-shaper`](pool/d4-route-traffic-shaper) | D4 | `false` | 11 | **PASS** (11/11) | **PASS** (11/11) |
| [`d4-timeseries-anomaly-detector`](pool/d4-timeseries-anomaly-detector) | D4 | `false` | 11 | **PASS** (11/11) | **PASS** (11/11) |
