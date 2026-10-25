#!/usr/bin/env python3
"""Generate a deterministic workload for d3-sandbox-exec-policy."""

import json
import os

def main():
    target_dir = os.path.dirname(os.path.abspath(__file__))
    policy_file = os.path.join(target_dir, "policy.json")
    requests_file = os.path.join(target_dir, "requests.json")

    policy = {
        "jail_root": target_dir,
        "roles": {
            "worker": {
                "allowed_binaries": ["/usr/bin/coreutils", "/usr/bin/dash"],
                "disallowed_args": [";", "&", "|", ".."],
                "allow_env": ["WORKER_ID", "LANG"],
                "max_timeout_ms": 3000,
                "max_output_bytes": 8192
            },
            "restricted": {
                "allowed_binaries": ["/usr/bin/coreutils"],
                "disallowed_args": ["sudo", "rm", ".."],
                "allow_env": [],
                "max_timeout_ms": 1000,
                "max_output_bytes": 1024
            }
        }
    }

    requests = []
    for i in range(20):
        if i % 5 == 0:
            # Denied request (unauthorized binary)
            requests.append({
                "id": f"req-{i:03d}",
                "role": "restricted",
                "command": ["/usr/bin/dash", "-c", "echo denied"],
                "working_dir": target_dir,
                "timeout_ms": 1000
            })
        elif i % 5 == 1:
            # Denied request (forbidden argument)
            requests.append({
                "id": f"req-{i:03d}",
                "role": "worker",
                "command": ["/usr/bin/coreutils", "echo", "test;bad"],
                "working_dir": target_dir,
                "timeout_ms": 1000
            })
        else:
            # Allowed request
            requests.append({
                "id": f"req-{i:03d}",
                "role": "worker",
                "command": ["/usr/bin/coreutils", "echo", f"output message {i}"],
                "working_dir": target_dir,
                "env": {"WORKER_ID": f"w-{i}", "SECRET": "hidden"},
                "timeout_ms": 1000
            })

    with open(policy_file, "w") as f:
        json.dump(policy, f, indent=2)

    with open(requests_file, "w") as f:
        json.dump({"requests": requests}, f, indent=2)

    print(f"Generated policy and {len(requests)} requests in {target_dir}")

if __name__ == "__main__":
    main()
