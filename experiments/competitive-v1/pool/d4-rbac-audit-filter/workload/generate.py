#!/usr/bin/env python3
"""Generate deterministic workload for d4-rbac-audit-filter."""

import json
import os

def main():
    target_dir = os.path.dirname(os.path.abspath(__file__))
    policy_file = os.path.join(target_dir, "policy.json")
    requests_file = os.path.join(target_dir, "requests.json")

    policy = {
        "roles": {
            "admin": ["*"],
            "analyst": ["report:read", "report:export"],
            "operator": ["job:start", "job:stop", "report:read"],
            "auditor": ["report:read", "audit:view"]
        },
        "owner_restricted_actions": ["job:stop"]
    }

    requests = []
    tenants = ["corp-alpha", "corp-beta"]
    for i in range(25):
        t_user = tenants[i % 2]
        t_res = tenants[i % 2] if i % 5 != 0 else tenants[(i + 1) % 2] # 20% tenant mismatch

        req = {
            "id": f"req-{i:04d}",
            "user": f"user-{i % 6}",
            "tenant_id": t_user,
            "roles": ["analyst"] if i % 2 == 0 else ["operator"],
            "action": "report:read" if i % 3 != 0 else "job:stop",
            "resource": f"res-{i:03d}",
            "resource_tenant_id": t_res,
            "resource_owner": f"user-{i % 6}"
        }

        if i % 4 == 0:
            req["delegation"] = {
                "delegator": "admin-bob",
                "roles": ["operator"],
                "valid_until_epoch": 1700050000
            }

        requests.append(req)

    with open(policy_file, "w") as f:
        json.dump(policy, f, indent=2)

    with open(requests_file, "w") as f:
        json.dump({"requests": requests}, f, indent=2)

    print(f"Generated policy and {len(requests)} requests in {target_dir}")

if __name__ == "__main__":
    main()
