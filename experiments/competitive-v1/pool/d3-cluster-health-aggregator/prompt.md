# Multi-Node Service Health Aggregator and Standby Activator

Implement a cluster health monitor that checks primary service nodes, determines operational health, and triggers failover activation to standby instances when primaries become unhealthy.

## Invocation

```bash
./run <cluster.json> <status.json> [--offline-fixtures <fixtures.json>]
```

- `<cluster.json>`: Path to cluster topology configuration defining monitored services.
- `<status.json>`: Path to output JSON reporting individual service states and cluster-wide metrics.
- `--offline-fixtures <fixtures.json>` (optional): Path to static URL response mappings for offline evaluation.

## Cluster Specification

`<cluster.json>` has the following structure:

```json
{
  "timeout_ms": 1500,
  "services": [
    {
      "name": "payment-api",
      "primary_url": "http://127.0.0.1:8101/health",
      "standby_url": "http://127.0.0.1:8102/health",
      "activate_url": "http://127.0.0.1:8102/v1/activate"
    }
  ]
}
```

Fields:
- `timeout_ms` (integer, optional, default: 2000): HTTP request timeout in milliseconds for all health probes and activation calls.
- `services` (array of objects, required): List of services to inspect:
  - `name` (string, required): Unique service name.
  - `primary_url` (string, required): HTTP endpoint for primary instance health probe.
  - `standby_url` (string, required): HTTP endpoint for standby instance health probe.
  - `activate_url` (string, required): HTTP endpoint to trigger standby activation if primary is down.

## Monitoring and Failover Protocol

For each service in `cluster.json` (in order):
1. **Primary Health Check**:
   - Send an HTTP `GET` request to `primary_url` with `timeout_ms`.
   - The primary is considered **healthy** if:
     - HTTP response code is `200`.
     - Response body is valid JSON with `{"status": "healthy"}`.
   - If healthy:
     - Set `state: "primary_active"`, `primary_health: "healthy"`, `standby_health: "not_checked"`, `activated: false`.
     - Do not poll standby or call activation.
2. **Failover to Standby**:
   - If primary probe returns non-200, invalid JSON, `"status"` other than `"healthy"`, or fails with network/timeout error:
     - Record `primary_health: "unhealthy"`.
     - Probe standby instance: send HTTP `GET` to `standby_url`.
     - If standby returns HTTP `200` with JSON `{"status": "healthy"}`:
       - Standby is healthy (`standby_health: "healthy"`).
       - Send HTTP `POST` request to `activate_url` with empty JSON object `{}`.
       - If activation returns HTTP `200` or `204`:
         - Set `state: "failed_over_to_standby"`, `activated: true`.
       - If activation fails (non-200/204 or network error):
         - Set `state: "degraded"`, `activated: false`.
     - If standby probe fails:
       - Set `standby_health: "unhealthy"`.
       - Set `state: "degraded"`, `activated: false`.

3. **Offline Fixtures Mode**:
   - If `--offline-fixtures <fixtures.json>` is provided, read simulated responses from that file instead of making network calls.
   - Fixture file structure:
     ```json
     {
       "http://127.0.0.1:8101/health": {"status": 200, "body": {"status": "healthy"}},
       "http://127.0.0.1:8102/v1/activate": {"status": 200, "body": {}}
     }
     ```

## Output Status Report Specification

`<status.json>` must contain:

```json
{
  "total_services": 1,
  "healthy": 1,
  "failed_over": 0,
  "degraded": 0,
  "services": [
    {
      "name": "payment-api",
      "state": "primary_active",
      "primary_health": "healthy",
      "standby_health": "not_checked",
      "activated": false
    }
  ]
}
```

Fields:
- `total_services`: Total number of configured services.
- `healthy`: Count of services with `state: "primary_active"`.
- `failed_over`: Count of services with `state: "failed_over_to_standby"`.
- `degraded`: Count of services with `state: "degraded"`.
- `services`: Array of result objects in the order of `cluster.services`.

## Exit Codes

- `0`: Cluster evaluation completed and `degraded == 0` (all services either active on primary or successfully failed over).
- `1`: One or more services are `degraded` (`degraded > 0`).
- `2`: Invalid CLI arguments, malformed JSON, or missing required service fields.
