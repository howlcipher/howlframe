# Change Request: Tagged Retention, Disk Quota Trimming, and Dry-Run (v2)

You are updating `log-purger` according to `baseline_spec.md`. The operations team requires fine-grained retention rules based on log categorization tags, total disk quota capping, and a dry-run mode.

## CLI Invocation

```bash
./run <rules.json> <log_dir> <report.json> [--dry-run] [--now-epoch <seconds>]
```

Arguments:
- `<rules.json>`: Path to retention and quota configuration.
- `<log_dir>`: Path to directory containing `.json` log files.
- `<report.json>`: Output audit manifest path.
- `--dry-run` (optional): If provided, calculates deletions and writes the report without actually deleting any files from `<log_dir>`.
- `--now-epoch <seconds>` (optional, integer): Evaluates age relative to this timestamp rather than system time.

## Updated Rules Format (`<rules.json>`)

```json
{
  "default_retention_days": 30,
  "tag_retention_days": {
    "compliance": 365,
    "audit": 180,
    "debug": 3
  },
  "max_storage_bytes": 10485760
}
```

Fields:
- `default_retention_days` (integer, required): Base retention days for logs without matching tags. (Must also accept legacy `retention_days` as alias).
- `tag_retention_days` (object, optional, default: `{}`): Map of tag name to integer retention days.
- `max_storage_bytes` (integer, optional): Maximum allowed total bytes of all remaining log files combined in `<log_dir>`.

## Updated Log File Format

Log files in `<log_dir>` may contain an optional `"tags"` array of strings:

```json
{
  "timestamp_epoch": 1695000000,
  "tags": ["audit", "auth"],
  "message": "user logged in"
}
```

## Retention & Quota Logic (v2)

1. **Tag-Based Expiry**:
   - For each `.json` file in `<log_dir>`:
     - Check its `tags` array.
     - If the file matches one or more tags in `tag_retention_days`, effective retention days = `max(tag_retention_days[t] for t in file.tags if t in tag_retention_days)`.
     - If no matching tags, effective retention days = `default_retention_days`.
     - File is expired if `(now_epoch - timestamp_epoch) > (effective_retention_days * 86400)`.
     - If expired, mark for purging with `reason: "age_expired"`.
     - If not `--dry-run`, delete the expired file.
2. **Quota Trimming (Storage Ceiling)**:
   - If `max_storage_bytes` is specified:
     - Calculate `current_storage_bytes` = sum of file sizes (in bytes) of all surviving (non-expired) log files.
     - While `current_storage_bytes > max_storage_bytes`:
       - Identify all surviving non-compliance files (files whose `tags` do NOT include `"compliance"`).
       - If no non-compliance files remain: break (compliance files are permanently exempt from quota pruning).
       - Select the oldest non-compliance file (lowest `timestamp_epoch`, tie-break by filename ascending).
       - Mark file for purging with `reason: "quota_exceeded"`.
       - Deduct file size from `current_storage_bytes`.
       - If not `--dry-run`, delete the file.
3. **Report Output**:
   - Write `<report.json>`.
   - Purged files must be listed sorted by `filename` ascending.

## Updated Report Specification (`<report.json>`)

```json
{
  "dry_run": false,
  "total_scanned": 10,
  "age_purged_count": 2,
  "quota_purged_count": 1,
  "retained_count": 7,
  "purged_files": [
    {
      "filename": "auth-01.json",
      "reason": "age_expired",
      "bytes": 2048
    },
    {
      "filename": "debug-99.json",
      "reason": "quota_exceeded",
      "bytes": 4096
    }
  ],
  "total_bytes_freed": 6144,
  "final_storage_bytes": 52400
}
```

Fields:
- `dry_run`: Boolean indicating whether `--dry-run` was active.
- `total_scanned`: Total `.json` files found in `<log_dir>`.
- `age_purged_count`: Number of files purged due to age.
- `quota_purged_count`: Number of files purged due to disk quota.
- `retained_count`: Number of files remaining.
- `purged_files`: Array of objects sorted by `filename` ascending.
- `total_bytes_freed`: Total bytes of all purged files combined.
- `final_storage_bytes`: Sum of bytes of all retained files.

## Exit Codes

- `0`: Success (even if quota required purging non-compliance files).
- `1`: Storage quota could not be achieved (`final_storage_bytes > max_storage_bytes`) because all remaining files are protected compliance files.
- `2`: Missing CLI arguments, directory does not exist, or malformed JSON.
