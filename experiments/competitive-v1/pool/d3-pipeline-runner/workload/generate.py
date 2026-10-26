#!/usr/bin/env python3
"""Generate a deterministic pipeline workload for d3-pipeline-runner."""

import json
import os
import sys

def main():
    target_dir = os.path.dirname(os.path.abspath(__file__))
    pipeline_file = os.path.join(target_dir, "pipeline.json")

    tasks = [
        {
            "id": "stage-0-prep",
            "command": ["/usr/bin/coreutils", "echo", "prep completed"],
            "timeout_ms": 2000,
            "max_retries": 1,
            "depends_on": []
        }
    ]

    for i in range(1, 15):
        parent = f"stage-{i-1}-task" if i > 1 else "stage-0-prep"
        tasks.append({
            "id": f"stage-{i}-task",
            "command": ["/usr/bin/coreutils", "echo", f"step {i}"],
            "timeout_ms": 3000,
            "max_retries": 1,
            "depends_on": [parent]
        })

    tasks.append({
        "id": "stage-final",
        "command": ["/usr/bin/coreutils", "echo", "all stages complete"],
        "timeout_ms": 2000,
        "max_retries": 0,
        "depends_on": ["stage-14-task"]
    })

    data = {"tasks": tasks}
    with open(pipeline_file, "w") as f:
        json.dump(data, f, indent=2)

    print(f"Generated {len(tasks)} pipeline tasks in {pipeline_file}")

if __name__ == "__main__":
    main()
