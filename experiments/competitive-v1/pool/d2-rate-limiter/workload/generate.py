#!/usr/bin/env python3
import json
import os
import random

def main():
    random.seed(12345)
    dir_path = os.path.dirname(os.path.abspath(__file__))

    requests = []
    # Register keys
    requests.append({"method": "POST", "path": "/keys", "body": {"key": "key-starter", "tier": "starter", "scopes": ["read"]}})
    requests.append({"method": "POST", "path": "/keys", "body": {"key": "key-standard", "tier": "standard", "scopes": ["read", "write"]}})
    requests.append({"method": "POST", "path": "/keys", "body": {"key": "key-enterprise", "tier": "enterprise", "scopes": ["read", "write", "admin"]}})

    # Batch of checks
    keys = ["key-starter", "key-standard", "key-enterprise"]
    scopes = ["read", "write", "admin"]

    for i in range(300):
        k = random.choice(keys)
        s = random.choice(scopes)
        requests.append({"method": "POST", "path": "/check", "body": {"key": k, "scope": s, "cost": 1}})
        if i % 50 == 0:
            requests.append({"method": "GET", "path": f"/keys/{k}"})

    with open(os.path.join(dir_path, "requests.jsonl"), "w") as f:
        for req in requests:
            f.write(json.dumps(req) + "\n")

    # Initial empty store
    with open(os.path.join(dir_path, "store.json"), "w") as f:
        json.dump({"keys": {}}, f)

if __name__ == "__main__":
    main()
