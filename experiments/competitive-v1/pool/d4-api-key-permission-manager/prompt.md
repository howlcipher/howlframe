# Change Request: Scope Authorization, IP Restrictions, and Key Rotation (v2)

You are updating the API key validator according to `baseline_spec.md`. The platform security team requires enforcing permission capability scopes, IP whitelist restrictions, and seamless zero-downtime key rotation with grace periods.

## CLI Invocation

```bash
./run <keystore.json> <requests.json> <audit.json> [--now-epoch <seconds>]
```

- `--now-epoch <seconds>` (optional, integer): Current reference time in seconds since Unix epoch. Defaults to system time.

## Updated Keystore Format (`<keystore.json>`)

```json
{
  "keys": {
    "key-prod-01": {
      "user_id": "usr-101",
      "status": "active",
      "scopes": ["orders:read", "orders:write"],
      "allowed_ips": ["192.168.1.50", "10.0.0.0/16"],
      "rotation": {
        "new_key": "key-prod-02",
        "grace_until_epoch": 1700050000
      }
    }
  }
}
```

New fields in key records:
- `scopes` (array of strings, optional, default: `[]`): Granular capability scopes permitted for this key. A scope of `"*"` grants all scopes.
- `allowed_ips` (array of strings, optional, default: `[]`): Allowed client IPv4 addresses or CIDR subnets (e.g. `"10.0.0.0/24"` or `"127.0.0.1"`). If empty or omitted, requests from any IP address are permitted.
- `rotation` (object, optional):
  - `new_key` (string, required): The successor key.
  - `grace_until_epoch` (integer, required): Cutoff timestamp after which the old key is rejected.

## Updated Requests Format (`<requests.json>`)

```json
{
  "requests": [
    {
      "id": "call-1",
      "key": "key-prod-01",
      "client_ip": "192.168.1.50",
      "required_scope": "orders:write"
    }
  ]
}
```

Fields:
- `id` (string, required): Request ID.
- `key` (string, required): Presented API key.
- `client_ip` (string, required): IPv4 address of caller.
- `required_scope` (string, optional): Capability scope required to perform the requested operation.

## Validation Semantics (v2)

For each request:
1. **Key Existence & Revocation**:
   - Check if `key` exists in `keystore.keys`. If missing: `decision: "DENIED"`, `reason: "key_not_found"`.
   - Check `key.status`: if `"revoked"`: `decision: "DENIED"`, `reason: "key_revoked"`.
   - If `status` is anything other than `"active"`: `decision: "DENIED"`, `reason: "key_inactive"`.
2. **Rotation & Grace Period**:
   - If `key` has a `rotation` block:
     - If `now_epoch > rotation.grace_until_epoch`:
       - Deny: `decision: "DENIED"`, `reason: "key_expired_after_rotation"`.
     - Else:
       - Set `rotation_notice: "key_rotated_use_new_key"`.
   - If no rotation or rotation not expired, default `rotation_notice: null`.
3. **IP Whitelist Enforcement**:
   - If `key.allowed_ips` is non-empty:
     - Check if `request.client_ip` matches any exact IP or falls within any CIDR block in `allowed_ips`.
     - If no match: `decision: "DENIED"`, `reason: "ip_forbidden"`.
4. **Scope Capability Enforcement**:
   - If `request.required_scope` is provided:
     - Check if `"*"` or `request.required_scope` is in `key.scopes`.
     - If not: `decision: "DENIED"`, `reason: "insufficient_scope"`.
5. **Success**:
   - If all checks pass: `decision: "VALID"`, `user_id: key.user_id`, `scope_granted: request.required_scope || null`.

## Updated Audit Output Format (`<audit.json>`)

```json
{
  "total_requests": 2,
  "valid_count": 1,
  "denied_count": 1,
  "evaluations": [
    {
      "id": "call-1",
      "decision": "VALID",
      "user_id": "usr-101",
      "scope_granted": "orders:write",
      "rotation_notice": null
    },
    {
      "id": "call-2",
      "decision": "DENIED",
      "reason": "ip_forbidden"
    }
  ]
}
```

## Exit Codes

- `0`: All requests were `VALID` (`denied_count == 0`).
- `1`: One or more requests were `DENIED` (`denied_count > 0`).
- `2`: Missing CLI arguments, malformed JSON, or missing required request fields (`key`, `client_ip`).
