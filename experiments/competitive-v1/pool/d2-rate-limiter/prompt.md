# API Gateway Rate Limiter and Token Quota Authority

Build an HTTP service and offline replay processor that acts as an API gateway access authority. The service authenticates access keys, verifies scope permissions, manages token-bucket rate limits according to subscription tiers, and persists key state to disk.

## Execution Modes

The executable is located at `./run`. It must support two execution modes:

### 1. HTTP Server Mode
```bash
./run [--store <store_path>] --port <PORT>
```
Starts an HTTP JSON server on `127.0.0.1:<PORT>`.
- `--port <PORT>`: Integer port on loopback. Required in server mode. (Also check `PORT` environment variable if `--port` is absent).
- `--store <store_path>`: Path to persistence JSON file. Optional; defaults to `store.json`.

### 2. Batch Replay Mode
```bash
./run replay [--store <store_path>] <requests_jsonl>
```
Executes requests sequentially offline through the identical business logic without binding a network socket, and writes each response as a JSON line to standard output:
```json
{"status": 200, "body": {"allowed": true, "remaining_tokens": 19.0}}
```

## Tier Configuration & Token Bucket Rules

Each key belongs to a subscription tier determining its token capacity and continuous refill rate:
- `"starter"`: capacity `5.0` tokens, refill rate `1.0` token per second.
- `"standard"`: capacity `20.0` tokens, refill rate `5.0` tokens per second.
- `"enterprise"`: capacity `100.0` tokens, refill rate `20.0` tokens per second.

When checking or inspecting a key, tokens are refilled according to elapsed time:
`refilled = min(capacity, current_tokens + (elapsed_seconds * refill_rate))`

## HTTP Endpoints

All request and response payloads are JSON (`Content-Type: application/json`).

### 1. `POST /keys`
Register or update an API key with assigned tier and authorized scopes.
- Request Body:
  ```json
  {
    "key": "key-prod-01",
    "tier": "standard",
    "scopes": ["read", "write"]
  }
  ```
- Validation:
  - If `tier` is not one of `"starter"`, `"standard"`, `"enterprise"`: return `400 Bad Request` with `{"error": "invalid_tier"}`.
  - If `key` is missing or empty, or `scopes` is not an array: return `400 Bad Request` with `{"error": "invalid_payload"}`.
- Logic:
  - Sets the key with full capacity tokens for that tier.
  - Persists state to `--store`.
- Response: `201 Created`
  ```json
  {
    "key": "key-prod-01",
    "tier": "standard",
    "scopes": ["read", "write"],
    "tokens": 20.0
  }
  ```

### 2. `POST /check`
Evaluate admission for an API request against authority scopes and rate quota.
- Request Body:
  ```json
  {
    "key": "key-prod-01",
    "scope": "write",
    "cost": 1
  }
  ```
  (`cost` is optional and defaults to `1.0`).
- Authority and Quota Checks:
  1. If `key` does not exist: return `401 Unauthorized`:
     ```json
     {"allowed": false, "reason": "invalid_key"}
     ```
  2. Refill tokens based on elapsed time.
  3. If `scope` is NOT in `key.scopes`: return `403 Forbidden`:
     ```json
     {"allowed": false, "reason": "insufficient_scope"}
     ```
  4. If `tokens < cost`: return `429 Too Many Requests`:
     ```json
     {"allowed": false, "reason": "rate_limited", "remaining_tokens": 0.5}
     ```
  5. Otherwise, access is granted:
     - Deduct `cost` from `tokens`.
     - Persist state to `--store`.
     - Return `200 OK`:
       ```json
       {"allowed": true, "remaining_tokens": 19.0}
       ```

### 3. `GET /keys/<key>`
Inspect current key status and remaining quota.
- If key does not exist: return `404 Not Found` with `{"error": "key_not_found"}`.
- Refills tokens and returns `200 OK`:
  ```json
  {
    "key": "key-prod-01",
    "tier": "standard",
    "scopes": ["read", "write"],
    "tokens": 19.0
  }
  ```

### 4. `POST /reset`
Reset token buckets back to full capacity.
- Request Body (optional):
  `{"key": "key-prod-01"}`
- Logic:
  - If `key` is provided: resets that specific key's tokens to its tier capacity. If key not found, return `404 Not Found`.
  - If no `key` or empty body: resets all registered keys to their tier capacities.
- Response: `200 OK`
  ```json
  {"status": "reset"}
  ```
