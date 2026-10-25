# Baseline Specification: Log Retention Purger (v1)

`log-purger` is a CLI tool that cleans up old JSON log files from an archive directory based on a flat age threshold.

## Invocation

```bash
./run <rules.json> <log_dir> <report.json>
```

## Baseline Input Formats

### `<rules.json>`
```json
{
  "retention_days": 30
}
```

### Log Files in `<log_dir>`
Files ending with `.json` containing a top-level JSON object with at least:
```json
{
  "timestamp_epoch": 1695000000,
  "message": "service initialized"
}
```

## Baseline Behavior

1. Scans `<log_dir>` for files with a `.json` extension.
2. For each file:
   - Reads `timestamp_epoch` from the JSON content.
   - Calculates age: `age_seconds = current_time - timestamp_epoch`.
   - If `age_seconds > retention_days * 86400`:
     - Delete the file from `<log_dir>`.
     - Record in `deleted_files`.
   - Otherwise, record in `retained_files`.
3. Writes `<report.json>`.

## Baseline Output Format (`<report.json>`)

```json
{
  "total_scanned": 1,
  "deleted_count": 1,
  "retained_count": 0,
  "deleted_files": ["log-01.json"],
  "retained_files": []
}
```

## Exit Codes

- `0`: Success.
- `2`: Missing arguments, invalid directory, or malformed JSON.
