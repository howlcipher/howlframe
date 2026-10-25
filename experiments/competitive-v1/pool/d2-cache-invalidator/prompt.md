# Distributed Cache Tag Index and Purge Coordinator

Build an HTTP service and offline replay processor that coordinates tag-based cache management. The service stores cached payloads associated with sets of categorical tags, enables high-efficiency surrogate multi-tag invalidation purges, monitors TTL expirations, and persists state across restarts.

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
Executes requests sequentially offline through the identical cache coordination logic, printing one JSON response line per request to standard output:
```json
{"status": 201, "body": {"key": "item:1", "status": "stored"}}
```

## Cache and Tag Model

An entry contains:
- `key`: non-empty string unique key.
- `value`: arbitrary JSON value.
- `tags`: array of unique string tags (e.g. `["category:apparel", "brand:nike"]`).
- `expires_at`: float epoch seconds or `null`.

## HTTP Endpoints

All request and response bodies use JSON (`Content-Type: application/json`).

### 1. `POST /cache`
Store or replace a cache entry.
- Request Body:
  ```json
  {
    "key": "product-101",
    "value": {"title": "Running Shoes", "price": 89.99},
    "tags": ["shoes", "running", "sale"],
    "ttl_seconds": 120
  }
  ```
  `key` and `value` are required. `tags` is required and must be an array of strings. `ttl_seconds` is optional.
- If `key` is empty or `tags` is not an array: return `400 Bad Request` with `{"error": "invalid_payload"}`.
- Logic:
  - If `ttl_seconds` provided: `expires_at = current_time + ttl_seconds`. Otherwise `expires_at = null`.
  - Persists entry to `--store`.
- Response: `201 Created`
  ```json
  {
    "key": "product-101",
    "value": {"title": "Running Shoes", "price": 89.99},
    "tags": ["shoes", "running", "sale"],
    "expires_at": 1700000120.0
  }
  ```

### 2. `GET /cache/<key>`
Retrieve a cache entry.
- If key does not exist or `current_time >= expires_at`: return `404 Not Found` with `{"error": "not_found"}`.
- Response: `200 OK` with the entry object.

### 3. `POST /purge`
Invalidate all active entries matching any specified tag.
- Request Body:
  ```json
  {
    "tags": ["running", "discontinued"]
  }
  ```
  `tags` must be a non-empty array of strings. If invalid: return `400 Bad Request` with `{"error": "invalid_payload"}`.
- Logic:
  - Scans all unexpired entries. If an entry has at least one tag matching any tag in `tags`, the entry is invalidated and deleted.
  - Updates running cumulative `total_purged` count.
  - Persists changes to `--store`.
- Response: `200 OK`
  ```json
  {
    "purged_count": 1,
    "purged_keys": ["product-101"]
  }
  ```
  `purged_keys` must be sorted ascending by key (code point order).

### 4. `DELETE /cache/<key>`
Manually invalidate a single key.
- If key does not exist or expired: return `404 Not Found`.
- Otherwise: delete entry, increment `total_purged` by 1, persist to `--store`, and return `204 No Content`.

### 5. `GET /tags/<tag>`
List unexpired keys currently associated with `<tag>`.
- Response: `200 OK`
  ```json
  {
    "tag": "shoes",
    "keys": ["product-101"]
  }
  ```
  `keys` must be sorted ascending by key (code point order).

### 6. `GET /stats`
Inspect cache metrics.
- Response: `200 OK`
  ```json
  {
    "active_entries": 10,
    "total_purged": 5,
    "active_tags": 8
  }
  ```
  `active_tags` is the number of distinct tags present across all unexpired entries.
