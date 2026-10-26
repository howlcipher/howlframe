# Baseline Specification: API Key Validator (v1)

`apikey-validator` validates incoming API key tokens against a persistent keystore database.

## Invocation

```bash
./run <keystore.json> <requests.json> <audit.json>
```

## Baseline Input Formats

### `<keystore.json>`
```json
{
  "keys": {
    "key-abc123": {
      "user_id": "usr-101",
      "status": "active"
    },
    "key-revoked456": {
      "user_id": "usr-102",
      "status": "revoked"
    }
  }
}
```

### `<requests.json>`
```json
{
  "requests": [
    {
      "id": "req-1",
      "key": "key-abc123"
    }
  ]
}
```

## Baseline Behavior

1. For each request:
   - Check if `key` exists in `keystore.keys`.
   - If missing: `decision: "DENIED"`, `reason: "key_not_found"`.
   - If exists but `status` is not `"active"`: `decision: "DENIED"`, `reason: "key_inactive"`.
   - If active: `decision: "VALID"`, `user_id: user_id`.
2. Writes `<audit.json>`.

## Baseline Output Format (`<audit.json>`)

```json
{
  "total_requests": 1,
  "valid_count": 1,
  "denied_count": 0,
  "evaluations": [
    {
      "id": "req-1",
      "decision": "VALID",
      "user_id": "usr-101"
    }
  ]
}
```

## Exit Codes

- `0`: All requests were valid (`denied_count == 0`).
- `1`: One or more requests were denied (`denied_count > 0`).
- `2`: Missing arguments or malformed JSON.
