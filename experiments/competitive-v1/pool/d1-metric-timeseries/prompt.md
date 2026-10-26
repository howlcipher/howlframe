# Infrastructure Metrics Timeseries Downsampler

Build a timeseries aggregation CLI tool that downsamples high-frequency streaming infrastructure telemetry into structured time-bucketed rollup windows, calculating statistical summaries including minimum, maximum, mean, and 95th percentile (p95).

## CLI Interface

The executable is located at `./run`. It must implement the `rollup` subcommand:

```bash
./run rollup --input <metrics_jsonl> --interval <seconds> --output <rollups_json> [--host <host_id>]
```

Arguments:
- `rollup`: Required subcommand. If absent or different, exit with code `1`.
- `--input <metrics_jsonl>`: Path to JSON Lines input file containing telemetry records. Required.
- `--interval <seconds>`: Duration of each aggregation bucket in integer seconds (must be > 0). Required.
- `--output <rollups_json>`: Path where the rollup JSON report will be saved. Required.
- `--host <host_id>`: Optional filter. If provided, process only metrics emitted by this specific host.

Options may be passed in any order.

### Exit Codes
- `0`: Execution succeeded and rollups file written.
- `1`: Invalid CLI arguments, missing required options, invalid interval (non-integer or <= 0), or unrecognized subcommand.
- `2`: The specified `--input` file does not exist or cannot be read.

## Input File Format (`--input`)

A JSON Lines (`.jsonl`) file where each non-empty line represents a measurement:
```json
{"timestamp": 1700000015, "host_id": "srv-prod-01", "metric": "cpu_utilization", "value": 45.2}
```

Fields:
- `"timestamp"`: integer epoch timestamp in seconds.
- `"host_id"`: non-empty string.
- `"metric"`: non-empty string metric name.
- `"value"`: numeric floating-point or integer value.

Malformed lines (lines missing any of the 4 keys, invalid JSON, non-integer timestamp, or non-numeric value) must be skipped and counted under `"skipped_lines"`.

## Aggregation and Windowing Logic

1. **Filtering**:
   - If `--host` is specified, ignore records where `record.host_id != host_id`. Filtered records are not counted in `total_points_processed`.

2. **Bucket Windowing**:
   - For each valid point, compute its bucket window start:
     `bucket_start = timestamp - (timestamp % interval)`

3. **Metrics Aggregation**:
   - Group records by tuple `(bucket_start, host_id, metric)`.
   - For each group with values sorted ascending `vals = [v_0, v_1, ..., v_{n-1}]`:
     - `"count"`: integer total values in bucket.
     - `"min"`: minimum numeric value.
     - `"max"`: maximum numeric value.
     - `"sum"`: sum of values, rounded to 2 decimal places.
     - `"avg"`: arithmetic mean (`sum / count`), rounded to 2 decimal places.
     - `"p95"`: 95th percentile using nearest-rank definition:
       `index = max(0, min(count - 1, ceil(0.95 * count) - 1))`
       `p95 = vals[index]`

4. **Ordering**:
   - The resulting `"windows"` array must be sorted by `bucket_start` ascending, then `host_id` ascending (code point order), then `metric` ascending.

## Output Format (`--output`)

Write a JSON object to `<rollups_json>`:

```json
{
  "interval_seconds": 60,
  "total_points_processed": 1,
  "skipped_lines": 0,
  "windows": [
    {
      "bucket_start": 1700000000,
      "host_id": "srv-prod-01",
      "metric": "cpu_utilization",
      "count": 1,
      "min": 45.2,
      "max": 45.2,
      "sum": 45.2,
      "avg": 45.2,
      "p95": 45.2
    }
  ]
}
```
