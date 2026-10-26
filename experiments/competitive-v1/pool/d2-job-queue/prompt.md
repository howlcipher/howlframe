# Priority Job Queue Broker with Lease Acknowledgment

Build an HTTP service and offline replay processor that acts as a priority job queue broker. The service manages task lifecycles (pending, leased, completed, dead-letter), enforces priority-based task dispatching, tracks worker lease timeouts, and handles automatic retries.

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
Executes requests sequentially offline through the identical queuing logic, printing one JSON response line per request to standard output:
```json
{"status": 201, "body": {"job_id": "job-1", "status": "pending"}}
```

## Job Model

A job contains:
- `job_id`: string sequential identifier (`"job-1"`, `"job-2"`, ...).
- `topic`: string queue topic name.
- `payload`: arbitrary JSON data.
- `priority`: integer (higher numbers indicate higher priority, defaults to `0`).
- `max_retries`: positive integer maximum lease attempts before moving to dead-letter (defaults to `3`).
- `lease_seconds`: positive integer lease duration in seconds (defaults to `30`).
- `status`: string (`"pending"`, `"leased"`, `"completed"`, or `"dead_letter"`).
- `attempts`: integer count of times the job has been leased (starts at `0`).
- `leased_by`: worker_id string or `null`.
- `lease_expires_at`: float epoch seconds or `null`.

## HTTP Endpoints

All request and response bodies use JSON (`Content-Type: application/json`).

### 1. `POST /jobs`
Enqueue a new job.
- Request Body:
  ```json
  {
    "topic": "notifications",
    "payload": {"user_id": "u-10", "message": "welcome"},
    "priority": 5,
    "max_retries": 3,
    "lease_seconds": 20
  }
  ```
  `topic` is required. Other fields default as specified above.
- Response: `201 Created` with full job object.

### 2. `POST /jobs/lease`
Acquire a lease on the next available job for a topic.
- Request Body:
  ```json
  {
    "topic": "notifications",
    "worker_id": "worker-alpha"
  }
  ```
- Scheduling Algorithm:
  1. Find candidate jobs for `topic` where:
     - `status == "pending"`, OR
     - `status == "leased"` AND `current_time >= lease_expires_at` (expired lease).
  2. If multiple candidates exist, sort by:
     - `priority` descending (highest priority first).
     - `created_at` ascending (FIFO for identical priority).
     - `job_id` ascending.
  3. If no candidate exists: return `204 No Content`.
  4. If candidate selected:
     - `status = "leased"`
     - `attempts += 1`
     - `leased_by = worker_id`
     - `lease_expires_at = current_time + job.lease_seconds`
     - Persist to `--store`.
     - Return `200 OK` with updated job object.

### 3. `POST /jobs/<id>/ack`
Worker confirms successful execution of a job.
- Request Body:
  ```json
  {"worker_id": "worker-alpha"}
  ```
- Validation:
  - If job not found: return `404 Not Found`.
  - If `job.status != "leased"` or `job.leased_by != worker_id`: return `409 Conflict` with `{"error": "not_leased_by_worker"}`.
- Logic:
  - `status = "completed"`.
  - Persist to `--store`.
  - Return `200 OK` with `{"status": "completed"}`.

### 4. `POST /jobs/<id>/fail`
Worker reports job failure.
- Request Body:
  ```json
  {"worker_id": "worker-alpha", "error": "database connection timeout"}
  ```
- Validation:
  - If job not found: return `404 Not Found`.
  - If `job.status != "leased"` or `job.leased_by != worker_id`: return `409 Conflict` with `{"error": "not_leased_by_worker"}`.
- Logic:
  - If `job.attempts >= job.max_retries`:
    `status = "dead_letter"`
  - Else:
    `status = "pending"`
  - `leased_by = null`, `lease_expires_at = null`.
  - Persist to `--store`.
  - Return `200 OK` with `{"status": "<new_status>", "attempts": <attempts>}`.

### 5. `GET /jobs/<id>`
Retrieve job status and details.
- Returns `200 OK` with job object, or `404 Not Found`.

### 6. `GET /stats`
Inspect queue statistics across all topics.
- Response: `200 OK`
  ```json
  {
    "pending": 3,
    "leased": 1,
    "completed": 10,
    "dead_letter": 0
  }
  ```
