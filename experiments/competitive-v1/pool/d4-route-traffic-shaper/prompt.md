# Change Request: Canary Traffic Splitting, Header Match, and Circuit Fallback (v2)

You are updating `route-dispatcher` according to `baseline_spec.md`. The traffic engineering team requires canary deployment routing with deterministic user hashing, header-based routing overrides, and health-checked fallback redirection.

## Updated Routing Table (`<routing_table.json>`)

```json
{
  "backends_health": {
    "user-service-v1": true,
    "user-service-v2": true,
    "order-service": false,
    "order-service-fallback": true
  },
  "routes": [
    {
      "path_prefix": "/api/v1/users",
      "headers_match": {
        "X-Beta-Tester": "true"
      },
      "target": "user-service-beta"
    },
    {
      "path_prefix": "/api/v1/users",
      "target": "user-service-v1",
      "canary": {
        "target": "user-service-v2",
        "weight_pct": 20
      }
    },
    {
      "path_prefix": "/api/v1/orders",
      "target": "order-service",
      "fallback_target": "order-service-fallback"
    }
  ]
}
```

New fields:
- `backends_health` (object, required): Map of backend service name to boolean health state (`true` = healthy, `false` = unhealthy). Default for unspecified backend names is `true`.
- In route objects:
  - `headers_match` (object, optional): Exact header key-value match requirements (case-insensitive header key comparison).
  - `canary` (object, optional):
    - `target` (string, required): Canary backend name.
    - `weight_pct` (integer, 0 to 100): Percentage of traffic allocated to canary.
  - `fallback_target` (string, optional): Alternative backend to use if primary (or canary) target is unhealthy.

## Updated Requests (`<requests.json>`)

```json
{
  "requests": [
    {
      "id": "req-101",
      "path": "/api/v1/users/profile",
      "user_id": "usr-9876",
      "client_ip": "192.168.1.1",
      "headers": {
        "x-beta-tester": "true"
      }
    }
  ]
}
```

Fields:
- `id` (string, required): Request ID.
- `path` (string, required): Request path.
- `user_id` (string, optional): User ID for canary hash determination.
- `client_ip` (string, optional, default: `"127.0.0.1"`): Fallback for canary hash if `user_id` is missing.
- `headers` (object, optional, default: `{}`): Request HTTP headers.

## Routing Logic (v2)

1. **Route Precedence & Matching**:
   - Check routes in the defined order of `routes`.
   - A route matches if:
     - `request.path` starts with `route.path_prefix`.
     - AND if `route.headers_match` is present, every key-value pair in `headers_match` exists in `request.headers` (comparing header names case-insensitively, and header values exactly as strings).
   - If no route matches: record `status: "404_not_found"`, `selected_target: null`.
2. **Target Selection (Canary)**:
   - For a matched route, determine candidate target:
     - If `route.canary` is defined and `canary.weight_pct > 0`:
       - Deterministic hash key = `request.user_id` if non-empty, else `request.client_ip`.
       - Compute SHA-256 hex digest of the UTF-8 encoded hash key string.
       - Parse first 8 hex characters as integer: `hash_int = int(hex_str[:8], 16)`.
       - `bucket = hash_int % 100`.
       - If `bucket < canary.weight_pct`:
         - Candidate target = `canary.target`, `canary_routed: true`.
       - Else:
         - Candidate target = `route.target`, `canary_routed: false`.
     - Else:
       - Candidate target = `route.target`, `canary_routed: false`.
3. **Health Check & Fallback Redirection**:
   - Check health of candidate target in `backends_health`.
   - If candidate target is healthy (`true`):
     - `selected_target = candidate_target`, `fallback_triggered: false`, `status: "routed"`.
   - If candidate target is unhealthy (`false`):
     - If `route.fallback_target` is defined and `backends_health.get(route.fallback_target, true)` is `true`:
       - `selected_target = route.fallback_target`, `fallback_triggered: true`, `status: "routed"`.
     - Else:
       - No healthy backend available!
       - `selected_target: null`, `fallback_triggered: false`, `status: "503_service_unavailable"`.

## Updated Output Format (`<dispatch_log.json>`)

```json
{
  "total_requests": 2,
  "routed_count": 2,
  "failed_count": 0,
  "dispatches": [
    {
      "id": "req-101",
      "matched_route": "/api/v1/users",
      "selected_target": "user-service-beta",
      "canary_routed": false,
      "fallback_triggered": false,
      "status": "routed"
    }
  ]
}
```

Fields:
- `total_requests`: Total count of requests.
- `routed_count`: Number of requests with `status: "routed"`.
- `failed_count`: Number of requests with `status: "404_not_found"` or `"503_service_unavailable"`.
- `dispatches`: Array of dispatch results in original request order.

## Exit Codes

- `0`: All requests were successfully routed (`failed_count == 0`).
- `1`: One or more requests failed to route (404 or 503).
- `2`: Missing CLI arguments, malformed JSON, or missing required request path.
