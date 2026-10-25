# Baseline Specification: RBAC Policy Evaluator (v1)

`rbac-evaluator` is a command-line utility for evaluating access requests against a role-based access control (RBAC) policy.

## Invocation

```bash
./run <policy.json> <requests.json> <decisions.json>
```

## Baseline Input Formats

### `<policy.json>`
```json
{
  "roles": {
    "admin": ["*"],
    "editor": ["document:read", "document:write"],
    "viewer": ["document:read"]
  }
}
```

### `<requests.json>`
```json
{
  "requests": [
    {
      "id": "req-1",
      "user": "alice",
      "roles": ["viewer"],
      "action": "document:read",
      "resource": "doc-101"
    }
  ]
}
```

## Baseline Evaluation Logic

1. For each request:
   - Expand the user's `roles` into a set of permitted actions based on `policy.roles`.
   - Wildcard `"*"` in role permissions matches any action.
   - If the requested `action` is contained in the user's permitted actions, decision is `"ALLOW"` with reason `"role_authorized"`.
   - Otherwise, decision is `"DENY"` with reason `"insufficient_permissions"`.

## Baseline Output Format (`<decisions.json>`)

```json
{
  "total_evaluated": 1,
  "allowed_count": 1,
  "denied_count": 0,
  "decisions": [
    {
      "id": "req-1",
      "decision": "ALLOW",
      "reason": "role_authorized"
    }
  ]
}
```

## Exit Codes

- `0`: All requests evaluated and allowed (`denied_count == 0`).
- `1`: One or more requests were denied (`denied_count > 0`).
- `2`: Malformed JSON, missing files, or validation errors.
