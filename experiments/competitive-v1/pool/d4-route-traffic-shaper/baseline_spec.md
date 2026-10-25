# Baseline Specification: Gateway Route Dispatcher (v1)

`route-dispatcher` resolves HTTP request paths to backend services using static path rules.

## Invocation

```bash
./run <routing_table.json> <requests.json> <dispatch_log.json>
```

## Baseline Input Formats

### `<routing_table.json>`
```json
{
  "routes": [
    {
      "path_prefix": "/api/v1/users",
      "target": "user-service"
    },
    {
      "path_prefix": "/api/v1/orders",
      "target": "order-service"
    }
  ]
}
```

### `<requests.json>`
```json
{
  "requests": [
    {
      "id": "req-1",
      "path": "/api/v1/users/42"
    }
  ]
}
```

## Baseline Behavior

1. Evaluates routes in list order.
2. If `request.path` starts with `route.path_prefix`:
   - Match found! Dispatch to `route.target`.
3. If no route matches:
   - Record `status: "404_not_found"`, `target: null`.
4. Outputs `<dispatch_log.json>`.

## Baseline Output Format (`<dispatch_log.json>`)

```json
{
  "total_requests": 1,
  "routed_count": 1,
  "failed_count": 0,
  "dispatches": [
    {
      "id": "req-1",
      "matched_route": "/api/v1/users",
      "selected_target": "user-service",
      "status": "routed"
    }
  ]
}
```

## Exit Codes

- `0`: All requests routed.
- `1`: One or more requests failed to route (404).
- `2`: Missing arguments or malformed JSON.
