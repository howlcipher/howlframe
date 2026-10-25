#!/usr/bin/env python3
"""Generate deterministic workload for d3-token-vault-agent."""

import json
import os

def main():
    target_dir = os.path.dirname(os.path.abspath(__file__))
    grants_file = os.path.join(target_dir, "grants.json")
    requests_file = os.path.join(target_dir, "requests.json")

    grants = {
        "subjects": {
            "worker-agent": {
                "allowed_scopes": ["compute:run", "data:query", "report:generate"],
                "max_token_ttl_sec": 300
            },
            "guest-agent": {
                "allowed_scopes": ["data:query"],
                "max_token_ttl_sec": 60
            }
        }
    }

    requests = []
    for i in range(20):
        if i % 4 == 0:
            # Denied: unauthorized scope
            requests.append({
                "id": f"call-{i:03d}",
                "subject": "guest-agent",
                "requested_scope": "admin:write",
                "target_url": f"http://127.0.0.1:9000/api/{i}",
                "method": "POST",
                "body": {"task": i}
            })
        elif i % 4 == 1:
            # Denied: unknown subject
            requests.append({
                "id": f"call-{i:03d}",
                "subject": "intruder-agent",
                "requested_scope": "data:query",
                "target_url": f"http://127.0.0.1:9000/api/{i}",
                "method": "GET"
            })
        else:
            # Authorized
            requests.append({
                "id": f"call-{i:03d}",
                "subject": "worker-agent",
                "requested_scope": "compute:run",
                "target_url": f"http://127.0.0.1:9000/api/{i}",
                "method": "POST",
                "headers": {"Authorization": "stale-secret-to-scrub"},
                "body": {"compute_job": i}
            })

    with open(grants_file, "w") as f:
        json.dump(grants, f, indent=2)

    with open(requests_file, "w") as f:
        json.dump({"requests": requests}, f, indent=2)

    print(f"Generated grants and {len(requests)} requests in {target_dir}")

if __name__ == "__main__":
    main()
