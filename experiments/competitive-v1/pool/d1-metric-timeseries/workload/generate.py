#!/usr/bin/env python3
import json
import os
import random

def main():
    random.seed(314159)
    dir_path = os.path.dirname(os.path.abspath(__file__))

    hosts = ["srv-prod-01", "srv-prod-02", "srv-db-01"]
    metrics = ["cpu_utilization", "memory_used_mb", "disk_io_iops"]

    start_ts = 1700000000
    lines = []

    for i in range(500):
        ts = start_ts + (i * 10)
        h = random.choice(hosts)
        m = random.choice(metrics)
        val = round(random.uniform(5.0, 95.0), 2)
        if i % 60 == 0:
            lines.append('{"corrupt": true}')
        else:
            lines.append(json.dumps({"timestamp": ts, "host_id": h, "metric": m, "value": val}))

    with open(os.path.join(dir_path, "metrics.jsonl"), "w") as f:
        f.write("\n".join(lines) + "\n")

if __name__ == "__main__":
    main()
