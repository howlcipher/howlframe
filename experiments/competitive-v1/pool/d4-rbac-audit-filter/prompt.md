# Change Request: Tenant Isolation, Ownership Constraints, and Delegation (v2)

You are updating the access evaluation engine according to `baseline_spec.md`. The enterprise security model requires enforcing tenant boundaries, resource ownership constraints, and time-bounded delegation tokens.

## CLI Invocation

```bash
./run <policy.json> <requests.json> <decisions.json> [--timestamp <epoch_seconds>]
```

- `--timestamp <epoch_seconds>` (optional, integer): Evaluation reference timestamp in Unix epoch seconds. Defaults to current system time if omitted.

## Updated Policy Format (`<policy.json>`)

`<policy.json>` now includes optional policy constraints:

```json
{
  "roles": {
    "admin": ["*"],
    "editor": ["document:read", "document:write", "document:delete"],
    "viewer": ["document:read"]
  },
  "owner_restricted_actions": ["document:delete"]
}
```

- `owner_restricted_actions` (array of strings, optional, default: `[]`): Actions that strictly require the caller to be the owner of the resource, regardless of role permissions (unless the user has the wildcard `"admin"` role).

## Updated Requests Format (`<requests.json>`)

Requests now include contextual attributes:

```json
{
  "requests": [
    {
      "id": "req-201",
      "user": "alice",
      "tenant_id": "tenant-corp",
      "roles": ["viewer"],
      "action": "document:write",
      "resource": "doc-555",
      "resource_tenant_id": "tenant-corp",
      "resource_owner": "bob",
      "delegation": {
        "delegator": "bob",
        "roles": ["editor"],
        "valid_until_epoch": 1700050000
      }
    }
  ]
}
```

New fields per request:
- `tenant_id` (string, required): Tenant context of the user.
- `resource_tenant_id` (string, required): Tenant owning the resource.
- `resource_owner` (string, optional): Username of the resource owner.
- `delegation` (object, optional):
  - `delegator` (string, required): Delegating user id.
  - `roles` (array of strings, required): Roles delegated to the caller.
  - `valid_until_epoch` (integer, required): Expiration epoch seconds of the delegation.

## Evaluation Semantics (v2)

For each request in order:
1. **Tenant Isolation Enforcement**:
   - Check if `request.tenant_id == request.resource_tenant_id`.
   - If they do NOT match:
     - Immediately deny: `decision: "DENY"`, `reason: "tenant_mismatch"`.
     - Skip subsequent checks.
2. **Delegation Processing**:
   - Calculate effective roles: start with `request.roles`.
   - If `delegation` is present:
     - If `--timestamp` <= `delegation.valid_until_epoch`:
       - Merge `delegation.roles` into the effective roles set.
       - Record `delegation_active: true`.
     - Else (delegation expired):
       - Do not add delegated roles.
       - Record `delegation_active: false`.
   - Deduplicate and sort `effective_roles` alphabetically.
3. **Permission Evaluation**:
   - Expand permissions from all roles in `effective_roles` based on `policy.roles`.
   - If the requested `action` is NOT permitted by any role (taking wildcard `"*"` into account):
     - Deny: `decision: "DENY"`, `reason: "insufficient_permissions"`.
4. **Resource Ownership Constraint**:
   - If the requested `action` is listed in `policy.owner_restricted_actions`:
     - If user has wildcard `"*"` permission (e.g. from `"admin"` role), ownership is bypassed.
     - Otherwise, check if `request.user == request.resource_owner`.
     - If user is not the owner:
       - Deny: `decision: "DENY"`, `reason: "not_resource_owner"`.
5. **Authorization Grant**:
   - If all checks pass: `decision: "ALLOW"`, `reason: "authorized"`.

## Updated Output Format (`<decisions.json>`)

```json
{
  "total_evaluated": 2,
  "allowed_count": 1,
  "denied_count": 1,
  "decisions": [
    {
      "id": "req-201",
      "decision": "ALLOW",
      "reason": "authorized",
      "effective_roles": ["editor", "viewer"],
      "delegation_active": true
    },
    {
      "id": "req-202",
      "decision": "DENY",
      "reason": "tenant_mismatch",
      "effective_roles": ["viewer"],
      "delegation_active": false
    }
  ]
}
```

Fields in each decision object:
- `id`: Request ID.
- `decision`: `"ALLOW"` or `"DENY"`.
- `reason`: `"authorized"`, `"tenant_mismatch"`, `"not_resource_owner"`, or `"insufficient_permissions"`.
- `effective_roles`: Array of unique role strings sorted alphabetically.
- `delegation_active`: Boolean (`true` if valid delegation was applied, `false` otherwise).

## Exit Codes

- `0`: All requests were evaluated and granted (`denied_count == 0`).
- `1`: One or more requests were denied (`denied_count > 0`).
- `2`: Malformed JSON, missing files, or validation errors (e.g. missing required tenant fields).
