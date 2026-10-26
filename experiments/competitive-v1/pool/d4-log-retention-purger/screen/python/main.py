#!/usr/bin/env python3
import argparse
import json
import os
import sys
import time

def fail(msg, code=2):
    sys.stderr.write(f"Error: {msg}\n")
    sys.exit(code)

def main():
    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("rules_path", nargs="?")
    parser.add_argument("log_dir", nargs="?")
    parser.add_argument("report_path", nargs="?")
    parser.add_argument("--dry-run", action="store_true", default=False)
    parser.add_argument("--now-epoch", type=int, default=None)

    try:
        args, extra = parser.parse_known_args()
        if extra or not args.rules_path or not args.log_dir or not args.report_path:
            fail("Missing required arguments", 2)
    except Exception as e:
        fail(f"Invalid CLI args: {e}", 2)

    try:
        with open(args.rules_path, "r", encoding="utf-8") as f:
            rules = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse rules JSON: {e}", 2)

    if not isinstance(rules, dict):
        fail("Rules must be an object", 2)

    if not os.path.isdir(args.log_dir):
        fail(f"Log directory '{args.log_dir}' does not exist or is not a directory", 2)

    default_days = rules.get("default_retention_days", rules.get("retention_days"))
    if default_days is None or not isinstance(default_days, int) or default_days < 0:
        fail("Missing valid 'default_retention_days' or 'retention_days'", 2)

    tag_retention = rules.get("tag_retention_days", {})
    if not isinstance(tag_retention, dict):
        fail("'tag_retention_days' must be an object", 2)

    max_storage_bytes = rules.get("max_storage_bytes", None)
    if max_storage_bytes is not None and (not isinstance(max_storage_bytes, int) or max_storage_bytes < 0):
        fail("Invalid 'max_storage_bytes'", 2)

    now_epoch = args.now_epoch if args.now_epoch is not None else int(time.time())

    # Scan directory for .json files
    scanned_files = []
    try:
        entries = sorted(os.listdir(args.log_dir))
    except Exception as e:
        fail(f"Failed to list directory: {e}", 2)

    for fname in entries:
        if not fname.endswith(".json"):
            continue
        fpath = os.path.join(args.log_dir, fname)
        if not os.path.isfile(fpath):
            continue

        try:
            fsize = os.path.getsize(fpath)
            with open(fpath, "r", encoding="utf-8") as f:
                content = json.load(f)
        except Exception:
            continue

        ts = content.get("timestamp_epoch", now_epoch)
        tags = content.get("tags", [])
        if not isinstance(tags, list):
            tags = []

        scanned_files.append({
            "filename": fname,
            "path": fpath,
            "size": fsize,
            "timestamp_epoch": ts,
            "tags": [str(t) for t in tags]
        })

    purged = {} # filename -> {"filename": str, "reason": str, "bytes": int}
    surviving = []

    # 1. Tag-based age expiry
    for file_info in scanned_files:
        matching_days = [tag_retention[t] for t in file_info["tags"] if t in tag_retention and isinstance(tag_retention[t], int)]
        effective_days = max(matching_days) if matching_days else default_days

        age_seconds = now_epoch - file_info["timestamp_epoch"]
        if age_seconds > (effective_days * 86400):
            purged[file_info["filename"]] = {
                "filename": file_info["filename"],
                "reason": "age_expired",
                "bytes": file_info["size"]
            }
        else:
            surviving.append(file_info)

    # 2. Disk Quota Trimming
    quota_purged_count = 0
    if max_storage_bytes is not None:
        current_storage = sum(f["size"] for f in surviving)
        while current_storage > max_storage_bytes:
            # Non-compliance files
            candidates = [f for f in surviving if "compliance" not in f["tags"]]
            if not candidates:
                break # All remaining are protected compliance files!

            # Oldest non-compliance file: lowest timestamp, then filename ascending
            candidates.sort(key=lambda x: (x["timestamp_epoch"], x["filename"]))
            victim = candidates[0]

            purged[victim["filename"]] = {
                "filename": victim["filename"],
                "reason": "quota_exceeded",
                "bytes": victim["size"]
            }
            surviving.remove(victim)
            current_storage -= victim["size"]
            quota_purged_count += 1

    # Perform actual deletions if not --dry-run
    if not args.dry_run:
        for fname, info in purged.items():
            fpath = os.path.join(args.log_dir, fname)
            try:
                if os.path.exists(fpath):
                    os.remove(fpath)
            except Exception:
                pass

    age_purged_count = sum(1 for p in purged.values() if p["reason"] == "age_expired")
    purged_sorted = sorted(list(purged.values()), key=lambda x: x["filename"])
    total_bytes_freed = sum(p["bytes"] for p in purged.values())
    final_storage_bytes = sum(f["size"] for f in surviving)

    report = {
        "dry_run": args.dry_run,
        "total_scanned": len(scanned_files),
        "age_purged_count": age_purged_count,
        "quota_purged_count": quota_purged_count,
        "retained_count": len(surviving),
        "purged_files": purged_sorted,
        "total_bytes_freed": total_bytes_freed,
        "final_storage_bytes": final_storage_bytes
    }

    try:
        with open(args.report_path, "w", encoding="utf-8") as f:
            json.dump(report, f, indent=2)
    except Exception as e:
        fail(f"Failed to write report: {e}", 2)

    quota_exceeded_error = (max_storage_bytes is not None and final_storage_bytes > max_storage_bytes)
    sys.exit(1 if quota_exceeded_error else 0)

if __name__ == "__main__":
    main()
