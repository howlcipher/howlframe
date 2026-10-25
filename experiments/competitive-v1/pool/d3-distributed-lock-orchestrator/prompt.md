# Distributed Lease Lock Coordinator with Heartbeat Renewal

Implement a job coordination tool that acquires an HTTP distributed lease lock before executing a command, sends periodic heartbeat renewals while the process runs, and releases the lock upon completion.

## Invocation

```bash
./run <job.json> <result.json> [--server <url>] [--offline-mock <mock.json>]
```

- `<job.json>`: Path to job configuration specifying lock details and command to execute.
- `<result.json>`: Path to output JSON reporting lock acquisition, renewals, command output, and release status.
- `--server <url>` (optional, default: `http://127.0.0.1:8080`): Base URL of the distributed lock service.
- `--offline-mock <mock.json>` (optional): Path to a static mock response JSON for running offline without network.

## Job Specification

`<job.json>` has the following structure:

```json
{
  "lock_name": "database-migration",
  "owner_id": "worker-node-1",
  "lease_ttl_ms": 2000,
  "renew_interval_ms": 500,
  "max_acquire_wait_ms": 3000,
  "command": ["/usr/bin/coreutils", "echo", "migrating database"],
  "timeout_ms": 5000
}
```

Fields:
- `lock_name` (string, required): Identifier of the shared resource to lock.
- `owner_id` (string, required): Unique identifier for this worker instance.
- `lease_ttl_ms` (integer, optional, default: 2000): Duration in milliseconds for each lock lease grant.
- `renew_interval_ms` (integer, optional, default: 500): Frequency in milliseconds to send lease renewal heartbeats while command runs.
- `max_acquire_wait_ms` (integer, optional, default: 3000): Maximum time in milliseconds to wait trying to acquire lock.
- `command` (array of strings, required): Subprocess command and arguments to execute once lock is held.
- `timeout_ms` (integer, optional, default: 10000): Maximum allowable duration for command execution.

## HTTP Lock Protocol

1. **Acquire Lease**:
   - `POST <server>/lock/acquire`
   - Request body:
     ```json
     {
       "lock_name": "<lock_name>",
       "owner_id": "<owner_id>",
       "ttl_ms": <lease_ttl_ms>
     }
     ```
   - Success response (HTTP 200):
     ```json
     {
       "acquired": true,
       "token": "<lease_token_string>"
     }
     ```
   - Conflict response (HTTP 409): Lock is currently held by another worker.
     - Retry acquiring every 50ms until `max_acquire_wait_ms` is reached.
     - If not acquired before deadline, abort without executing `command`.

2. **Renew Lease (Heartbeat)**:
   - While `command` is executing, every `renew_interval_ms`, send:
   - `POST <server>/lock/renew`
   - Request body:
     ```json
     {
       "lock_name": "<lock_name>",
       "owner_id": "<owner_id>",
       "token": "<token>",
       "ttl_ms": <lease_ttl_ms>
     }
     ```
   - Increment `renew_count` for each successful renewal (HTTP 200).

3. **Release Lease**:
   - When the command finishes (or if it times out/fails), send:
   - `POST <server>/lock/release`
   - Request body:
     ```json
     {
       "lock_name": "<lock_name>",
       "owner_id": "<owner_id>",
       "token": "<token>"
     }
     ```
   - HTTP 200 confirms lock release.

4. **Offline Mock Mode**:
   - If `--offline-mock <mock.json>` is provided, read responses from that file instead of making HTTP requests.
   - Format: `{"acquire_status": 200, "token": "mock-tok"}`.

## Output Specification

`<result.json>` contains:

```json
{
  "lock_acquired": true,
  "token": "tok-12345",
  "renew_count": 2,
  "command_executed": true,
  "exit_code": 0,
  "stdout": "migrating database\n",
  "stderr": "",
  "lock_released": true
}
```

If lock could not be acquired within `max_acquire_wait_ms`:
```json
{
  "lock_acquired": false,
  "token": null,
  "renew_count": 0,
  "command_executed": false,
  "exit_code": null,
  "stdout": "",
  "stderr": "",
  "lock_released": false
}
```

## Exit Codes

- `0`: Lock was acquired, command was executed successfully (exit code 0), and lock was released.
- `1`: Failed to acquire lock OR command exited with a non-zero exit code or timed out.
- `2`: Invalid CLI arguments, malformed JSON, or missing required fields.
