# Versioned Key-Value Store with TTL

Build an HTTP service and offline replay processor that functions as a lightweight versioned key-value datastore. The service supports optimistic concurrency control using version preconditions, automatic time-to-live (TTL) expiration, prefix search queries, and local file persistence.

## Execution Modes

The executable is located at `./run`. It must support two execution modes:

### 1. HTTP Server Mode
```bash
./run [--store <store_path>] --port <PORT>
```
Starts an HTTP JSON server on `127.0.0.1:<PORT>`.
- `--port <PORT>`: Loopback port. Required in server mode. (Also check `PORT` environment variable).
- `--store <store_path>`: Path to persistence JSON file. Optional; defaults to `store.json`.

### 2. Batch Replay Mode
```bash
./run replay [--store <store_path>] <requests_jsonl>
```
Executes requests sequentially offline through the identical storage logic, printing one JSON response line per request to standard output:
```json
{"status": 200, "body": {"key": "user:1", "version": 2, "value": "updated"}}
```

## HTTP Endpoints

All request and response bodies use JSON (`Content-Type: application/json`).

### 1. `PUT /keys/<key>`
Set or update a value with optional optimistic locking and TTL.
- Request Body:
  ```json
  {
    "value": "arbitrary JSON data",
    "expected_version": 1,
    "ttl_seconds": 60
  }
  ```
  `value` is required. `expected_version` and `ttl_seconds` are optional.
- Optimistic Concurrency Checks:
  - If `expected_version` is `0`: the key must NOT exist or must be expired. If it exists and is active, return `409 Conflict`:
    ```json
    {"error": "version_conflict", "current_version": <current_version>}
    ```
  - If `expected_version > 0`: the key must exist, be unexpired, and its current version must equal `expected_version`. If versions differ or key does not exist, return `409 Conflict`:
    ```json
    {"error": "version_conflict", "current_version": <current_version_or_0>}
    ```
- Write Logic:
  - If key does not exist (or expired): new version is `1`. Response status is `201 Created`.
  - If key exists and active: new version is `current_version + 1`. Response status is `200 OK`.
  - If `ttl_seconds` is provided: `expires_at = current_time_seconds + ttl_seconds`. Otherwise `expires_at = null`.
  - Persists state to `--store`.
- Response:
  ```json
  {
    "key": "<key>",
    "version": 1,
    "value": "arbitrary JSON data",
    "expires_at": 1700000060.0
  }
  ```

### 2. `GET /keys/<key>`
Retrieve an entry.
- If key does not exist or `current_time >= expires_at`: return `404 Not Found` with `{"error": "not_found"}`.
- Otherwise, return `200 OK`:
  ```json
  {
    "key": "<key>",
    "version": 1,
    "value": "arbitrary JSON data",
    "expires_at": 1700000060.0
  }
  ```

### 3. `DELETE /keys/<key>`
Delete an entry.
- If key does not exist or expired: return `404 Not Found` with `{"error": "not_found"}`.
- Otherwise, remove entry, persist to `--store`, and return `204 No Content`.

### 4. `GET /keys`
List active keys matching a prefix.
- Query parameters:
  - `prefix`: string (optional, defaults to empty string `""` matching all keys).
  - `limit`: integer (optional, maximum number of keys to return).
- Filters out expired keys.
- Sorts matching keys by key name ascending (code point order).
- Response `200 OK`:
  ```json
  {
    "keys": ["user:1", "user:2"]
  }
  ```
