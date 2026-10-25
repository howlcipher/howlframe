#!/usr/bin/env python3
"""Generate deterministic workload for d4-api-key-permission-manager."""

import json
import os

def main():
    target_dir = os.path.dirname(os.path.abspath(__file__))
    keystore_file = os.path.join(target_dir, "keystore.json")
    requests_file = os.path.join(target_dir, "requests.json")

    keys = {
        "key-admin": {
            "user_id": "usr-admin",
            "status": "active",
            "scopes": ["*"],
            "allowed_ips": ["127.0.0.1", "10.0.0.0/8"]
        },
        "key-service-ro": {
            "user_id": "usr-svc",
            "status": "active",
            "scopes": ["analytics:read", "data:read"],
            "allowed_ips": []
        },
        "key-legacy": {
            "user_id": "usr-legacy",
            "status": "active",
            "scopes": ["data:read"],
            "rotation": {
                "new_key": "key-legacy-v2",
                "grace_until_epoch": 1700050000
            }
        },
        "key-compromised": {
            "user_id": "usr-bad",
            "status": "revoked"
        }
    }

    requests = []
    for i in range(25):
        kname = "key-admin" if i % 4 == 0 else ("key-service-ro" if i % 4 == 1 else ("key-legacy" if i % 4 == 2 else "key-compromised"))
        req = {
            "id": f"call-{i:03d}",
            "key": kname,
            "client_ip": "10.1.2.3" if i % 2 == 0 else "192.168.1.1",
            "required_scope": "data:read" if i % 3 != 0 else "admin:full"
        }
        requests.append(req)

    with open(keystore_file, "w") as f:
        json.dump({"keys": keys}, f, indent=2)

    with open(requests_file, "w") as f:
        json.dump({"requests": requests}, f, indent=2)

    print(f"Generated {len(keys)} keys and {len(requests)} requests in {target_dir}")

if __name__ == "__main__":
    main()
