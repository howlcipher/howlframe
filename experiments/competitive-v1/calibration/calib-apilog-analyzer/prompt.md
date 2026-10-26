# API Access Log & SLA Analyzer

Build a command-line tool `calib-apilog-analyzer` that processes newline-delimited JSON (NDJSON) HTTP API access logs, validates entries, computes traffic metrics, evaluates Service Level Agreement (SLA) compliance, identifies top active clients, and aggregates requests into tumbling time windows. The tool emits a structured JSON report.

---

## 1. Command-Line Interface

Your program must be executable via `/workspace/run`.

```sh
/workspace/run [OPTIONS]
```

### Supported Options

* `--input <path>`: Path to the input file containing newline-delimited JSON logs. If omitted or `-`, read from standard input (`stdin`).
* `--output <path>`: Path to write the output JSON report. If omitted or `-`, write to standard output (`stdout`).
* `--sla-latency-ms <int>`: Latency SLA threshold in milliseconds. Default: `500`. Must be a non-negative integer (`>= 0`).
* `--error-rate-threshold <float>`: Error rate threshold fraction between `0.0` and `1.0` inclusive. Default: `0.05`.
* `--window-seconds <int>`: Duration of tumbling time windows in seconds. Default: `300`. Must be a positive integer (`> 0`).

Options may be passed either as separate arguments (e.g. `--input file.jsonl`) or with an equals sign (e.g. `--input=file.jsonl`).

### Exit Codes

* `0`: Success. The program processed the input (even if input was empty or contained invalid records) and generated the report.
* `1`: Command-line usage error (e.g. unrecognized option, missing value for an option, invalid threshold values such as negative latency or error rate outside `[0.0, 1.0]`) or failure opening/writing to the output file (including if the output path is a directory). Print an informative error message to standard error (`stderr`).
* `2`: Input file error: `--input <path>` was provided (and was not `-`), but the path does not exist, is a directory, or cannot be read. Print an informative error message to standard error (`stderr`).

---

## 2. Input Format and Validation

The input is a stream of newline-delimited JSON (NDJSON) records.

### Line Filtering & Parsing

1. **Blank Lines:** Lines consisting solely of whitespace (or empty lines) are ignored and are **not** counted toward `total_records`, `valid_records`, or `invalid_records`.
2. **Record Validation:** Each non-blank line must parse as a JSON object (a dictionary / map). Non-object JSON (e.g. arrays, strings, numbers, booleans, null) is invalid.
3. **Required Fields & Constraints:**
   * **Integer Fields Rule (`timestamp`, `status`, `latency_ms`, `bytes_sent`):** Must be a finite JSON number representing a whole number (integer value); for example, `1` and `1.0` are accepted as integer value `1`. Numbers with a fractional part (such as `1.5`), non-finite numbers (`NaN`, `Infinity`, `1e400`), numbers exceeding $2^{53} - 1$ ($9007199254740991$), and non-number types (strings, booleans, null) are invalid.
   * `timestamp`: Unix timestamp in seconds. Must be a non-negative whole number (`0 <= timestamp <= 2^53 - 1`).
   * `service`: Non-empty string (`len(service) >= 1`).
   * `client_id`: Non-empty string (`len(client_id) >= 1`).
   * `method`: HTTP method string. Must be exactly one of: `"GET"`, `"POST"`, `"PUT"`, `"DELETE"`, `"PATCH"`, `"HEAD"`, `"OPTIONS"` (case-sensitive).
   * `path`: Non-empty string starting with `"/"` (e.g. `"/api/v1/orders"`).
   * `status`: HTTP response status code. Must be a whole number between `100` and `599` inclusive (e.g. `200` and `200.0` are valid; `200.5`, strings, or booleans are invalid).
   * `latency_ms`: Elapsed request time in milliseconds. Must be a non-negative whole number (`0 <= latency_ms <= 2^53 - 1`).
   * `bytes_sent`: Payload bytes sent. Must be a non-negative whole number (`0 <= bytes_sent <= 2^53 - 1`).
   * Any additional fields present in the JSON object are allowed and ignored.

4. **Error Handling on Records:**
   * If a non-blank line cannot be parsed as valid JSON, is not a JSON object, is missing any required field, or contains a field that violates the types or constraints above:
     * It is counted as an **invalid record** (`invalid_records += 1`, and `total_records += 1`).
     * It is excluded from all service, client, window, and summary metrics.
     * The program must continue processing subsequent lines without terminating.

---

## 3. Metrics and Computations

All metrics are computed exclusively over **valid records**.

### A. Summary (`summary`)

* `total_records`: Integer count of non-blank lines processed (`valid_records + invalid_records`).
* `valid_records`: Integer count of records that passed validation.
* `invalid_records`: Integer count of records that failed validation.
* `earliest_timestamp`: Integer minimum `timestamp` among valid records, or `null` if `valid_records == 0`.
* `latest_timestamp`: Integer maximum `timestamp` among valid records, or `null` if `valid_records == 0`.
* `total_bytes_sent`: Integer sum of `bytes_sent` across all valid records (`0` if `valid_records == 0`).
* `total_services`: Integer count of distinct service names among valid records (`0` if `valid_records == 0`).
* `services_with_sla_breach`: Integer count of services whose `sla_breach` field is `true` (`0` if `valid_records == 0`).

### B. Service Metrics (`services`)

An array of objects representing distinct services, sorted in ascending alphabetical order by `service` name.
If `valid_records == 0`, `services` is `[]`.

For each service:
* `service`: String service name.
* `total_requests`: Integer count of valid records for this service.
* `successful_requests`: Integer count of valid records with `200 <= status < 400`.
* `client_error_requests`: Integer count of valid records with `400 <= status < 500`.
* `server_error_requests`: Integer count of valid records with `500 <= status <= 599`.
  *(Note: Informational responses `100 <= status < 200` count toward `total_requests`, but are neither success nor error).*
* `error_rate`: Float `(client_error_requests + server_error_requests) / total_requests`, rounded to 4 decimal places (`0.0` if `total_requests == 0`).
* `min_latency_ms`: Integer minimum `latency_ms` observed for this service.
* `max_latency_ms`: Integer maximum `latency_ms` observed for this service.
* `avg_latency_ms`: Float average of `latency_ms` for this service, rounded to 2 decimal places (`0.0` if `total_requests == 0`).
* `total_bytes_sent`: Integer sum of `bytes_sent` for this service.
* `sla_breach`: Boolean `true` if `error_rate > error_rate_threshold` OR `avg_latency_ms > sla_latency_ms`; otherwise `false`.

### C. Top Clients (`top_clients`)

An array of up to 10 distinct clients with the highest request counts (or all clients if fewer than 10 exist).
If `valid_records == 0`, `top_clients` is `[]`.

Sorting order:
1. `request_count` descending.
2. Ties broken by `client_id` ascending (ASCII lexicographical order).

For each client:
* `client_id`: String client identifier.
* `request_count`: Integer count of valid records for this client.
* `total_bytes`: Integer sum of `bytes_sent` for this client.
* `error_count`: Integer count of valid records with `status >= 400` for this client.

### D. Tumbling Time Windows (`windows`)

Tumbling time windows of duration `window_seconds`.
For each valid record, its window start timestamp is computed as:
$$\text{window\_start} = \text{timestamp} - (\text{timestamp} \bmod \text{window\_seconds})$$
$$\text{window\_end} = \text{window\_start} + \text{window\_seconds}$$

An array of window objects, sorted by `window_start` ascending.
Only window buckets that contain at least one valid record are included in the array.
If `valid_records == 0`, `windows` is `[]`.

For each window bucket:
* `window_start`: Integer window start timestamp in epoch seconds.
* `window_end`: Integer window end timestamp in epoch seconds.
* `total_requests`: Integer count of valid records falling into this window.
* `error_requests`: Integer count of valid records in this window with `status >= 400`.
* `avg_latency_ms`: Float average of `latency_ms` for valid records in this window, rounded to 2 decimal places.

*(Note: Acceptance tests compare floating-point metrics with a tolerance of ±0.001).*

---

## 4. Example

### Input (`sample.jsonl`)

```json
{"timestamp": 1710000000, "service": "auth-service", "client_id": "client-1", "method": "POST", "path": "/api/v1/login", "status": 200, "latency_ms": 45, "bytes_sent": 512}
{"timestamp": 1710000050, "service": "billing-service", "client_id": "client-2", "method": "POST", "path": "/api/v1/charge", "status": 500, "latency_ms": 650, "bytes_sent": 128}
{"timestamp": 1710000100, "service": "auth-service", "client_id": "client-1", "method": "GET", "path": "/api/v1/profile", "status": 200, "latency_ms": 15, "bytes_sent": 256}
invalid json line
```

### Command

```sh
/workspace/run --input sample.jsonl --sla-latency-ms 500 --error-rate-threshold 0.05 --window-seconds 300
```

### Output (`stdout`)

```json
{
  "summary": {
    "total_records": 4,
    "valid_records": 3,
    "invalid_records": 1,
    "earliest_timestamp": 1710000000,
    "latest_timestamp": 1710000100,
    "total_bytes_sent": 896,
    "total_services": 2,
    "services_with_sla_breach": 1
  },
  "services": [
    {
      "service": "auth-service",
      "total_requests": 2,
      "successful_requests": 2,
      "client_error_requests": 0,
      "server_error_requests": 0,
      "error_rate": 0.0,
      "min_latency_ms": 15,
      "max_latency_ms": 45,
      "avg_latency_ms": 30.0,
      "total_bytes_sent": 768,
      "sla_breach": false
    },
    {
      "service": "billing-service",
      "total_requests": 1,
      "successful_requests": 0,
      "client_error_requests": 0,
      "server_error_requests": 1,
      "error_rate": 1.0,
      "min_latency_ms": 650,
      "max_latency_ms": 650,
      "avg_latency_ms": 650.0,
      "total_bytes_sent": 128,
      "sla_breach": true
    }
  ],
  "top_clients": [
    {
      "client_id": "client-1",
      "request_count": 2,
      "total_bytes": 768,
      "error_count": 0
    },
    {
      "client_id": "client-2",
      "request_count": 1,
      "total_bytes": 128,
      "error_count": 1
    }
  ],
  "windows": [
    {
      "window_start": 1710000000,
      "window_end": 1710000300,
      "total_requests": 3,
      "error_requests": 1,
      "avg_latency_ms": 236.67
    }
  ]
}
```
