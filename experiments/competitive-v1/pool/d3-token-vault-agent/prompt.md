# Scoped Capability Token Vault and Mediation Agent

Implement an authorization-mediating agent that evaluates caller capability scopes against granted policies, mints scoped delegation bearer tokens via a local token authority vault, and forwards authorized HTTP requests with sanitized credentials.

## Invocation

```bash
./run <grants.json> <requests.json> <audit.json> [--vault-url <url>] [--offline-vault]
```

- `<grants.json>`: Path to policy configuration file mapping subjects to permitted capability scopes.
- `<requests.json>`: Path to batch requests to mediate and dispatch.
- `<audit.json>`: Path to output audit log recording authorization decisions, minted tokens, and downstream outcomes.
- `--vault-url <url>` (optional, default: `http://127.0.0.1:8000`): Base URL of the local token authority vault.
- `--offline-vault` (optional): If present, mints deterministic tokens locally without calling the vault service, and records simulated offline responses for target URLs.

## Policy Grants Specification

`<grants.json>` has the following structure:

```json
{
  "subjects": {
    "service-billing": {
      "allowed_scopes": ["invoice:read", "invoice:create"],
      "max_token_ttl_sec": 300
    },
    "service-analytics": {
      "allowed_scopes": ["metrics:read"],
      "max_token_ttl_sec": 600
    }
  }
}
```

Fields:
- `subjects` (object, required): Map of subject names to permission limits:
  - `allowed_scopes` (array of strings, required): List of permission capability scopes granted to this subject.
  - `max_token_ttl_sec` (integer, optional, default: 300): Maximum allowable token lifespan in seconds.

## Requests Specification

`<requests.json>` contains an array of mediation requests:

```json
{
  "requests": [
    {
      "id": "req-101",
      "subject": "service-billing",
      "requested_scope": "invoice:read",
      "target_url": "http://127.0.0.1:9000/invoices",
      "method": "GET",
      "headers": {
        "Accept": "application/json",
        "Authorization": "untrusted-caller-token"
      },
      "body": null,
      "timeout_ms": 2000
    }
  ]
}
```

Fields:
- `id` (string, required): Request identifier.
- `subject` (string, required): Calling identity requesting access.
- `requested_scope` (string, required): Capability scope requested for downstream operation.
- `target_url` (string, required): Downstream HTTP service URL.
- `method` (string, optional, default: `"GET"`): HTTP method (`GET`, `POST`, `PUT`, `DELETE`).
- `headers` (object, optional, default: `{}`): HTTP headers from caller.
- `body` (JSON value or null, optional, default: `null`): Optional request body.
- `timeout_ms` (integer, optional, default: 3000): Request timeout in milliseconds.

## Capability & Mediation Semantics

For each request:
1. **Subject Authorization Check**:
   - Verify that `subject` exists in `grants.subjects`.
   - If missing: mark `decision: "denied"`, `reason: "unauthorized_subject"`. Do not mint token or contact downstream service.
2. **Scope Capability Check**:
   - Verify that `requested_scope` is present in `subject.allowed_scopes`.
   - If missing: mark `decision: "denied"`, `reason: "scope_not_permitted"`. Do not mint token or contact downstream service.
3. **Mint Delegation Token**:
   - If running with live vault:
     - `POST <vault-url>/v1/token/mint`
     - Request body:
       ```json
       {
         "subject": "<subject>",
         "scope": "<requested_scope>",
         "ttl_sec": <min(60, max_token_ttl_sec)>
       }
       ```
     - Response status 200 contains `{"token": "<minted_token_string>", "expires_in": 60}`.
   - If running with `--offline-vault`:
     - Generate deterministic token string: `vault-mock-<subject>-<requested_scope>`.
4. **Credential Sanitization & Downstream Dispatch**:
   - Strip any existing caller `Authorization` or `Proxy-Authorization` headers (case-insensitive check) to eliminate credential smuggling or token leaks.
   - Inject the newly minted token: `Authorization: Bearer <token>`.
   - If `--offline-vault` is set, do not open network connections; record `target_status_code: 200`, `response_body: "{\"offline\": true}"`.
   - Otherwise, send the HTTP request to `target_url` with method, sanitized headers, and body (serialized as JSON if not null).
   - Capture `target_status_code` and string `response_body`.

## Output Audit Log Specification

`<audit.json>` records:

```json
{
  "total_requests": 2,
  "authorized_dispatched": 1,
  "denied": 1,
  "dispatches": [
    {
      "id": "req-101",
      "decision": "authorized",
      "token_issued": "vault-bearer-abc123",
      "target_status_code": 200,
      "response_body": "{\"data\": [1, 2]}"
    },
    {
      "id": "req-102",
      "decision": "denied",
      "reason": "scope_not_permitted"
    }
  ]
}
```

Fields:
- `total_requests`: Total count of requests.
- `authorized_dispatched`: Count of requests authorized and sent to downstream service.
- `denied`: Count of requests rejected due to subject or scope authority violations.
- `dispatches`: Array of execution results in the same order as input requests.

## Exit Codes

- `0`: All requests were successfully evaluated/audited, and all dispatched downstream calls returned status `< 500`.
- `1`: One or more dispatched downstream calls returned HTTP 5xx or timed out / errored on network.
- `2`: Invalid CLI arguments, malformed JSON input, or missing required fields.
