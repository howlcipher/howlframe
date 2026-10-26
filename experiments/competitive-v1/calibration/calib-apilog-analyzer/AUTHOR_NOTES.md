# Author Notes: calib-apilog-analyzer

## Domain and Problem Representation

* **Task ID**: `calib-apilog-analyzer`
* **Domain**: `D1` (CLI data processing)
* **Title**: API Access Log & SLA Analyzer

### Why the Task is Typical
API log aggregation, service-level agreement (SLA) monitoring, and traffic profiling are standard tasks in modern cloud systems and microservices architectures. A CLI utility that consumes NDJSON event streams, verifies record integrity, computes multi-dimensional aggregations (per-service traffic, client quotas, tumbling time windows), and outputs structured summary reports represents everyday systems engineering work.

This task provides an ideal calibration workload because:
1. **Language-Neutral Interface**: Uses standard command-line flags, standard streams (`stdin`/`stdout`), standard exit codes, and JSON/NDJSON file formats.
2. **Standard Library Only**: Requires no third-party libraries across Python, Go, and JavaScript (Node.js).
3. **Broad Feature Coverage**: Exercises CLI parsing (flag values and `--flag=value` forms), streaming I/O, robust schema validation, dictionary/map aggregations, arithmetic metrics, custom sorting/tie-breaking, and time bucketing.
4. **Clean Difficulty**: Requires roughly 30–60 minutes for a competent engineer and 15–30 minutes for a capable AI coding agent.

---

## Design Decisions and Omissions

1. **NDJSON over CSV**:
   Newline-delimited JSON is the industry standard for structured log pipelines. Using NDJSON avoids standard library discrepancies (such as Node lacking a built-in CSV parser) while testing robust streaming parsing.
2. **Explicit Validation Rules**:
   Real log streams contain corrupt lines and malformed entries. The contract specifies exact validation rules (non-negative timestamps, integer types excluding booleans, allowed HTTP methods, status code boundaries). Invalid records increment error counters without aborting execution.
3. **Tumbling Windows via Integer Epoch Math**:
   Tumbling windows are computed as `timestamp - (timestamp % window_seconds)`. This tests time-series bucketing deterministically without timezone or calendar parsing discrepancies.
4. **Omission of Complex Percentile Interpolation**:
   Percentile interpolation methods (e.g. nearest-rank, linear, R6/R7) vary across languages and standard libraries. To prevent arbitrary floating-point edge cases, the task relies on exact statistics (`min`, `max`, `sum`, `avg`, `error_rate`). Furthermore, acceptance tests accept floating-point comparisons within a tolerance of ±0.001.
5. **Deterministic Sorting & Tie-Breaking**:
   All output collections have deterministic order: `services` is sorted alphabetically by service name; `top_clients` is sorted by request count descending with ties broken alphabetically by client ID; `windows` is sorted chronologically by window start.

---

## Workload Sizing and Timings

### Sizing Analysis
In Revision 1, the test harness operator reported the following median timings (excluding process startup) measured in the scoring sandbox with pinned toolchains (CPython 3.12.15, Go 1.25.14, Node.js 22.23.4) at 100,000 records:
* **Python**: `0.71s`
* **Go**: `1.14s`
* **JavaScript**: `0.28s` (below the 0.5s floor)

To ensure all three baseline implementations run within the protocol requirement of **0.5s to 5.0s** with comfortable margins on both ends, the record count was scaled by $2.5\times$ to **250,000 records**:
* **JavaScript**: $\approx 0.28\text{s} \times 2.5 = 0.70\text{s}$ ($> 0.5\text{s}$ by a 40% margin)
* **Python**: $\approx 0.71\text{s} \times 2.5 = 1.78\text{s}$ (centered within the window)
* **Go**: $\approx 1.14\text{s} \times 2.5 = 2.85\text{s}$ ($< 5.0\text{s}$ by a 43% margin)

### Workload Generation & Verification
The workload is generated on demand by `workload/generate.py` with default arguments (`--records 250000 --seed 42`):
```sh
cd workload && python3 -I generate.py && sha256sum requests.jsonl
```
* **SHA-256 Pin**: `40b9455b5705883f35af5728d1e0c40680cf76c8093c08cb76934c649ad174c4`

### Local Measured Timings (250,000 records, mean of 3 runs)
* **Python reference**: `1.806s` (runs: `[1.782s, 1.800s, 1.836s]`)
* **Go reference**: `2.635s` (runs: `[2.709s, 2.679s, 2.517s]`)
* **JavaScript reference**: `0.826s` (runs: `[0.818s, 0.844s, 0.817s]`)

---

## Revision 2 Notes

Following independent evaluation with held-out tests, two contract edge cases were explicitly decided and standardized across `prompt.md` and all reference implementations:

1. **Whole-Number Floating Point Values**:
   * An integer field (`timestamp`, `status`, `latency_ms`, `bytes_sent`) must be a finite JSON number representing a whole number. Standard JSON number formats without fractional parts such as `1` and `1.0` are accepted as integer value `1`.
   * Numbers with non-zero fractional parts (e.g. `1.5`), non-finite numbers (`NaN`, `Infinity`, `1e400`), numbers exceeding the safe integer limit $2^{53} - 1$ ($9007199254740991$), and non-number types (strings, booleans, null) are rejected as invalid records.
   * This standardizes behavior across standard library parsers: Python (`math.floor(val) == val`), Go (`math.Floor(f) == f`), and JavaScript (`Number.isSafeInteger(val)`).

2. **Directory Passed as Input Path**:
   * Clarified that if `--input <path>` is a directory, does not exist, or cannot be read, the program must exit with code `2` and write an error message to `stderr`.
   * All three reference implementations explicitly check whether the input path is a directory and exit with code `2`.

---

## Attestation

Written new for this purpose, not copied or adapted from any benchmark or HowlFrame repository task.
