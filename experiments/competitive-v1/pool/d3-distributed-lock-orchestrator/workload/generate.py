#!/usr/bin/env python3
"""Generate a deterministic workload for d3-distributed-lock-orchestrator."""

import json
import os

def main():
    target_dir = os.path.dirname(os.path.abspath(__file__))
    job_file = os.path.join(target_dir, "job.json")
    mock_file = os.path.join(target_dir, "mock.json")

    job = {
        "lock_name": "benchmark-shared-resource",
        "owner_id": "bench-worker-01",
        "lease_ttl_ms": 3000,
        "renew_interval_ms": 500,
        "max_acquire_wait_ms": 2000,
        "command": ["/usr/bin/coreutils", "echo", "workload task completed"],
        "timeout_ms": 5000
    }

    mock = {
        "acquire_status": 200,
        "token": "tok-bench-8888"
    }

    with open(job_file, "w") as f:
        json.dump(job, f, indent=2)

    with open(mock_file, "w") as f:
        json.dump(mock, f, indent=2)

    print(f"Generated job and mock files in {target_dir}")

if __name__ == "__main__":
    main()
