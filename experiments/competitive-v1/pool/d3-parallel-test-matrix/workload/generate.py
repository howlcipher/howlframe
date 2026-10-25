#!/usr/bin/env python3
"""Generate deterministic workload for d3-parallel-test-matrix."""

import json
import os

def main():
    target_dir = os.path.dirname(os.path.abspath(__file__))
    matrix_file = os.path.join(target_dir, "matrix.json")

    cases = []
    for i in range(16):
        cases.append({
            "id": f"matrix-case-{i:03d}",
            "command": ["/usr/bin/coreutils", "echo", f"case {i} pass"],
            "env": {"CASE_NUM": str(i)},
            "timeout_ms": 3000
        })

    matrix = {
        "concurrency": 4,
        "fail_fast": False,
        "default_timeout_ms": 5000,
        "cases": cases
    }

    with open(matrix_file, "w") as f:
        json.dump(matrix, f, indent=2)

    print(f"Generated matrix with {len(cases)} cases in {matrix_file}")

if __name__ == "__main__":
    main()
