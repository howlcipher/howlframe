#!/usr/bin/env python3
"""Generate deterministic workload for d4-log-retention-purger."""

import json
import os

def main():
    target_dir = os.path.dirname(os.path.abspath(__file__))
    rules_file = os.path.join(target_dir, "rules.json")
    logs_dir = os.path.join(target_dir, "logs")
    os.makedirs(logs_dir, exist_ok=True)

    rules = {
        "default_retention_days": 30,
        "tag_retention_days": {
            "compliance": 365,
            "debug": 7,
            "audit": 90
        },
        "max_storage_bytes": 15000
    }

    with open(rules_file, "w") as f:
        json.dump(rules, f, indent=2)

    # Base epoch: 1700000000
    # Day = 86400
    for i in range(25):
        fname = f"log_{i:03d}.json"
        tags = []
        if i % 5 == 0:
            tags = ["compliance"]
            days_ago = 100
        elif i % 5 == 1:
            tags = ["debug"]
            days_ago = 10 # expired under 7d debug limit
        elif i % 5 == 2:
            tags = ["audit"]
            days_ago = 45 # kept under 90d audit limit
        else:
            tags = ["general"]
            days_ago = i # some expired under 30d default

        data = {
            "timestamp_epoch": 1700000000 - (days_ago * 86400),
            "tags": tags,
            "message": f"Log payload line index {i} " + ("padding " * 20)
        }

        with open(os.path.join(logs_dir, fname), "w") as f:
            json.dump(data, f, indent=2)

    print(f"Generated rules and 25 log files in {target_dir}")

if __name__ == "__main__":
    main()
