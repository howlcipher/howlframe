# Greenhouse Sensor Telemetry Auditor

Build an audit CLI tool that processes greenhouse IoT sensor readings, validates each record against threshold configurations, detects anomalous values, and generates a structured JSON report.

## CLI Interface

The executable is located at `./run`. It must support the `audit` subcommand:

```bash
./run audit --data <data_path> --config <config_path> --out <out_path> [--summary]
```

Arguments:
- `audit`: Required subcommand. If absent or different, exit with code `1`.
- `--data <data_path>`: Path to a JSON Lines (`.jsonl`) file containing sensor readings. Required.
- `--config <config_path>`: Path to a JSON file defining sensor thresholds. Required.
- `--out <out_path>`: Path where the audit report JSON will be written. Required.
- `--summary`: Optional flag. If present, print a compact JSON summary to standard output.

Options may be provided in any order.

### Exit Codes
- `0`: Execution succeeded and the report was written.
- `1`: Invalid CLI arguments, unrecognized subcommand, or missing required flags.
- `2`: The specified `--data` or `--config` file does not exist or cannot be read.

## Input File Formats

### 1. Configuration File (`--config`)
A JSON object mapping sensor type names (e.g., `"temperature"`, `"humidity"`) to an object with:
- `"min"`: number (minimum allowable reading value, inclusive)
- `"max"`: number (maximum allowable reading value, inclusive)
- `"unit"`: string (expected unit of measurement, e.g., `"celsius"`)

Example `config.json`:
```json
{
  "temperature": {"min": 15.0, "max": 32.0, "unit": "celsius"},
  "humidity": {"min": 40.0, "max": 80.0, "unit": "percent"},
  "co2": {"min": 350.0, "max": 1200.0, "unit": "ppm"}
}
```

### 2. Telemetry Data File (`--data`)
A JSON Lines file where each line is a JSON object with 5 required fields:
- `"timestamp"`: string (ISO 8601 timestamp string)
- `"greenhouse_id"`: string (identifier, e.g. `"GH-1"`)
- `"sensor_type"`: string
- `"value"`: number
- `"unit"`: string

Example line:
```json
{"timestamp": "2026-10-01T08:00:00Z", "greenhouse_id": "GH-1", "sensor_type": "temperature", "value": 24.5, "unit": "celsius"}
```

## Validation and Processing Rules

Process the data file line by line:
1. **Malformed Records**:
   - If a line is empty, not valid JSON, missing any of the 5 required fields, or `"value"` is not a numeric type (float or integer), count it as a malformed record (`malformed_records`). Do not produce an anomaly for it.
2. **Invalid Records**:
   - If the JSON object contains all 5 fields and `"value"` is numeric, but `"sensor_type"` is not present in the configuration file, OR `"unit"` does not match the configured unit for that sensor type, count it as an invalid record (`invalid_records`). Do not produce an anomaly for it.
3. **Valid Records and Anomalies**:
   - If the record passes all validations above, count it as a valid record (`valid_records`).
   - Check threshold bounds:
     - If `value < min`: classify as an anomaly with `"type": "below_min"` and `"threshold": min`.
     - If `value > max`: classify as an anomaly with `"type": "above_max"` and `"threshold": max`.
     - If `min <= value <= max`: normal reading (no anomaly).
   - Anomaly object format:
     ```json
     {
       "timestamp": "<timestamp>",
       "greenhouse_id": "<greenhouse_id>",
       "sensor_type": "<sensor_type>",
       "value": <value>,
       "threshold": <threshold>,
       "type": "below_min" | "above_max"
     }
     ```
   - Sort the final list of anomalies by `timestamp` ascending (lexicographical/code-point sort), then `greenhouse_id` ascending, then `sensor_type` ascending.
4. **Greenhouse Summary**:
   - Aggregate statistics across all valid records grouped by `greenhouse_id`:
     - `"reading_count"`: integer total valid readings for this greenhouse.
     - `"anomaly_count"`: integer total anomalies for this greenhouse.
     - `"sensor_types"`: sorted list (alphabetical / code-point ascending) of distinct valid sensor types observed for this greenhouse.
   - The keys of `"greenhouse_summary"` must be sorted by code point.

## Output Report (`--out`)

Write a JSON object to `<out_path>` with the following schema:

```json
{
  "valid_records": 10,
  "malformed_records": 1,
  "invalid_records": 1,
  "anomalies": [
    {
      "timestamp": "2026-10-01T08:15:00Z",
      "greenhouse_id": "GH-1",
      "sensor_type": "temperature",
      "value": 35.5,
      "threshold": 32.0,
      "type": "above_max"
    }
  ],
  "greenhouse_summary": {
    "GH-1": {
      "reading_count": 10,
      "anomaly_count": 1,
      "sensor_types": ["co2", "temperature"]
    }
  }
}
```

## Summary Flag (`--summary`)

If `--summary` is passed, write exactly one line of JSON to standard output followed by a newline:
```json
{"valid": 10, "anomalies": 1, "malformed": 1}
```
If `--summary` is not passed, standard output may remain empty.
