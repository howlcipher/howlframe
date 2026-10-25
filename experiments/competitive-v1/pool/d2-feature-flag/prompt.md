# Feature Entitlement and Authority Governance Engine

Build an HTTP service and offline batch replay runner that enforces access governance and capability entitlements for application features. The engine evaluates client context against tier thresholds, role permissions, and direct override grants, with local file persistence.

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
Executes requests sequentially offline through the identical governance logic, printing one JSON response line per request to standard output:
```json
{"status": 200, "body": {"granted": true, "reason": "entitled"}}
```

## Entitlement Model

Tiers are strictly ordered by ascending entitlement level:
`"basic"` (1) < `"pro"` (2) < `"enterprise"` (3)

A feature flag definition contains:
- `"key"`: string identifier (e.g. `"ai_copilot"`).
- `"enabled"`: boolean master kill-switch.
- `"min_tier"`: string (`"basic"`, `"pro"`, or `"enterprise"`).
- `"allowed_roles"`: array of string roles (e.g. `["admin", "editor"]`).
- `"user_overrides"`: array of user_id strings granted capability regardless of tier/role.

## HTTP Endpoints

All request and response bodies use JSON (`Content-Type: application/json`).

### 1. `POST /flags`
Create or update a feature flag.
- Request Body:
  ```json
  {
    "key": "ai_copilot",
    "enabled": true,
    "min_tier": "enterprise",
    "allowed_roles": ["admin", "editor"],
    "user_overrides": ["user-vip"]
  }
  ```
- Validation:
  - If `key` is missing or empty, or `min_tier` is not one of `"basic"`, `"pro"`, `"enterprise"`, or `allowed_roles` is not an array: return `400 Bad Request` with `{"error": "invalid_payload"}`.
- Logic:
  - Saves the flag definition to memory and persists to `--store`.
- Response: `201 Created` with the full flag object.

### 2. `POST /evaluate`
Evaluate entitlement authorization for a subject.
- Request Body:
  ```json
  {
    "key": "ai_copilot",
    "user_id": "u-101",
    "tier": "enterprise",
    "role": "editor"
  }
  ```
- Evaluation Hierarchy:
  1. If flag does not exist: return `404 Not Found` with `{"error": "flag_not_found"}`.
  2. If `flag.enabled == false`: return `200 OK`:
     ```json
     {"granted": false, "reason": "flag_disabled"}
     ```
  3. If `user_id` is present in `flag.user_overrides`: return `200 OK`:
     ```json
     {"granted": true, "reason": "user_override"}
     ```
  4. Compare `tier` against `flag.min_tier`. If subject's tier level is lower than `min_tier`: return `200 OK`:
     ```json
     {"granted": false, "reason": "insufficient_tier"}
     ```
  5. Check `role`. If `role` is not present in `flag.allowed_roles`: return `200 OK`:
     ```json
     {"granted": false, "reason": "role_unauthorized"}
     ```
  6. Otherwise, access is granted: return `200 OK`:
     ```json
     {"granted": true, "reason": "entitled"}
     ```

### 3. `GET /flags/<key>`
Retrieve details of a feature flag.
- Returns `200 OK` with the flag object, or `404 Not Found` if missing.

### 4. `DELETE /flags/<key>`
Delete a feature flag.
- Returns `204 No Content` if deleted, or `404 Not Found` if not found.

### 5. `GET /flags`
List all flags.
- Returns `200 OK`:
  ```json
  {
    "flags": [
      {
        "key": "ai_copilot",
        "enabled": true,
        "min_tier": "enterprise",
        "allowed_roles": ["admin", "editor"],
        "user_overrides": ["user-vip"]
      }
    ]
  }
  ```
- The `flags` array must be sorted by `key` ascending (code point order).
