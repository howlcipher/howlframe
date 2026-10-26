# Append-Only Event Stream Topic Broker

Build an HTTP service and offline replay processor that functions as an append-only distributed event stream broker. The service organizes events into topics, assigns monotonic sequence offsets, coordinates consumer group cursor commits, trims logs based on retention bounds, and persists state to local storage.

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
Executes requests sequentially offline through the identical broker logic, printing one JSON response line per request to standard output:
```json
{"status": 201, "body": {"offset": 0, "timestamp": 1700000000.0}}
```

## Broker Architecture & Offsets

- **Topics**: Each topic maintains an append-only event sequence with an advancing `next_offset` (starts at `0`).
- **Retention**: If `max_events` is reached, publishing an event purges the oldest event in that topic. `next_offset` continues monotonically advancing.
- **Consumer Groups**: Each consumer group tracks a separate committed offset cursor per topic.

## HTTP Endpoints

All request and response bodies use JSON (`Content-Type: application/json`).

### 1. `POST /topics`
Create a new topic.
- Request Body:
  ```json
  {
    "topic": "sensor-stream",
    "max_events": 500
  }
  ```
  `topic` is required. `max_events` is optional and defaults to `500`.
- If topic already exists, return `409 Conflict` with `{"error": "topic_exists"}`.
- Response: `201 Created`
  ```json
  {
    "topic": "sensor-stream",
    "max_events": 500,
    "head_offset": 0
  }
  ```

### 2. `POST /topics/<topic>/publish`
Append an event to the topic.
- Request Body:
  ```json
  {
    "event_type": "reading",
    "payload": {"temp": 21.5}
  }
  ```
  `event_type` is required.
- If topic does not exist: return `404 Not Found`.
- Logic:
  - Assigns `offset = next_offset`, increments `next_offset`.
  - Records timestamp.
  - Trims oldest events if total stored events exceed `max_events`.
  - Persists state to `--store`.
- Response: `201 Created`
  ```json
  {
    "offset": 0,
    "timestamp": 1700000000.0
  }
  ```

### 3. `POST /subscriptions/<group>/consume`
Consume events for a consumer group.
- Request Body:
  ```json
  {
    "topic": "sensor-stream",
    "limit": 10,
    "auto_commit": true
  }
  ```
  `topic` is required. `limit` defaults to `10`. `auto_commit` defaults to `true`.
- If topic does not exist: return `404 Not Found`.
- Logic:
  - Retrieves group's current cursor for `topic`. If group has never consumed `topic`, cursor starts at earliest available event in the topic (or `0` if topic is empty).
  - Fetches up to `limit` active events with `offset >= cursor`.
  - If `auto_commit == true` and events are returned: updates cursor to `highest_returned_offset + 1`.
  - Persists to `--store`.
- Response: `200 OK`
  ```json
  {
    "topic": "sensor-stream",
    "group": "<group>",
    "events": [
      {
        "offset": 0,
        "event_type": "reading",
        "payload": {"temp": 21.5},
        "timestamp": 1700000000.0
      }
    ],
    "next_offset": 1
  }
  ```

### 4. `POST /subscriptions/<group>/seek`
Reposition a consumer group's offset cursor.
- Request Body:
  ```json
  {
    "topic": "sensor-stream",
    "offset": 5
  }
  ```
- If topic does not exist: return `404 Not Found`.
- Sets group's cursor for that topic to `offset`.
- Persists to `--store`.
- Response: `200 OK`
  ```json
  {
    "group": "<group>",
    "topic": "sensor-stream",
    "committed_offset": 5
  }
  ```

### 5. `GET /topics/<topic>`
Inspect topic metadata.
- If topic does not exist: return `404 Not Found`.
- Response: `200 OK`
  ```json
  {
    "topic": "sensor-stream",
    "max_events": 500,
    "event_count": 1,
    "earliest_offset": 0,
    "next_offset": 1
  }
  ```
  (`earliest_offset` is `null` if topic has 0 events).
