# Resilient Webhook Event Dispatcher with HMAC Signature

Implement a command-line tool that dispatches queued JSON webhook payloads to an HTTP endpoint with HMAC-SHA256 signatures, bounded retries with backoff, and delivery audit logging.

## Invocation

```bash
./run <events.json> <report.json> [--endpoint <url>] [--secret <string>] [--offline-status <code>]
```

Arguments:
- `<events.json>`: Path to input JSON file containing webhook events.
- `<report.json>`: Path to output JSON file recording delivery results.
- `--endpoint <url>` (optional, default: `http://127.0.0.1:8080/events`): Destination HTTP URL for dispatch.
- `--secret <string>` (optional, default: `default-secret`): Secret key used to compute HMAC-SHA256 signatures.
- `--offline-status <code>` (optional): When provided, runs offline without making network requests, treating each attempt as receiving HTTP status `<code>` (e.g. `200` or `500`).

## Input Specification

The input file `<events.json>` contains an object with an `"events"` array:

```json
{
  "events": [
    {
      "id": "evt-001",
      "topic": "order.created",
      "payload": {
        "order_id": 101,
        "amount": 49.99
      },
      "max_attempts": 3,
      "timeout_ms": 1000
    }
  ]
}
```

Fields for each event:
- `id` (string, required): Unique identifier for the event.
- `topic` (string, required): Event category/topic string.
- `payload` (JSON value, required): Arbitrary JSON payload data.
- `max_attempts` (integer, optional, default: 3): Maximum delivery attempts (must be >= 1).
- `timeout_ms` (integer, optional, default: 2000): HTTP request timeout in milliseconds.

## Dispatch Semantics

For each event in the input list:
1. Serialize the `payload` to JSON string (using compact or standard JSON formatting).
2. Compute the HMAC-SHA256 hex digest over the encoded UTF-8 bytes of the serialized JSON payload using the configured secret key.
   The signature header value must be formatted as: `sha256=<hex_digest>` (lowercase hex).
3. Send an HTTP `POST` request to `--endpoint` with headers:
   - `Content-Type: application/json`
   - `X-Event-ID: <id>`
   - `X-Event-Topic: <topic>`
   - `X-Signature: sha256=<hex_digest>`
   - Request body: The serialized JSON payload bytes.
4. Response evaluation:
   - **2xx (200-299)**: Delivery is successful (`"status": "delivered"`). Do not make further attempts.
   - **4xx (400-499)**: Client error (e.g., rejected signature or invalid format). Mark as `"status": "failed"`. Do NOT retry.
   - **5xx (500-599)** or network error / timeout:
     - If current attempts < `max_attempts`: wait 10ms (multiplied by attempt number: 10ms, 20ms, 40ms, capped at 50ms) and retry.
     - If all `max_attempts` exhausted: mark as `"status": "failed"`.
5. If `--offline-status <code>` is specified:
   - Do not open network connections.
   - For each attempt, record HTTP status code `<code>`.
   - Apply the same 2xx/4xx/5xx retry logic as above.

## Output Report Specification

`<report.json>` must record:

```json
{
  "total_events": 1,
  "delivered": 1,
  "failed": 0,
  "deliveries": [
    {
      "id": "evt-001",
      "status": "delivered",
      "attempts": 1,
      "last_status_code": 200,
      "signature": "sha256=d3b07384d113edec49eaa6238ad5ff00..."
    }
  ]
}
```

Fields:
- `total_events`: Count of events in the input.
- `delivered`: Count of events with `"status": "delivered"`.
- `failed`: Count of events with `"status": "failed"`.
- `deliveries`: Array of delivery result objects in the same order as input events.
  - `id`: Event identifier.
  - `status`: `"delivered"` or `"failed"`.
  - `attempts`: Total number of attempts executed.
  - `last_status_code`: Integer HTTP status code of the final attempt (or `null` if network error/timeout with no status code).
  - `signature`: The `sha256=<hex_digest>` string calculated for this event.

## Exit Codes

- `0`: All events were successfully delivered (`delivered == total_events`).
- `1`: One or more events failed to deliver.
- `2`: Invalid CLI arguments, malformed JSON, or missing required event fields.
