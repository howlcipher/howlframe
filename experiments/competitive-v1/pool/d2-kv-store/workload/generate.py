#!/usr/bin/env python3
import json
import os
import random

def main():
    random.seed(9999)
    dir_path = os.path.dirname(os.path.abspath(__file__))

    requests = []
    # Seed keys
    for i in range(1, 51):
        k = f"item:{i:03d}"
        requests.append({"method": "PUT", "path": f"/keys/{k}", "body": {"value": {"index": i}, "expected_version": 0, "ttl_seconds": 3600}})

    # Batch operations
    for i in range(250):
        target = f"item:{random.randint(1, 50):03d}"
        op = random.choice(["get", "put", "delete", "list"])
        if op == "get":
            requests.append({"method": "GET", "path": f"/keys/{target}"})
        elif op == "put":
            requests.append({"method": "PUT", "path": f"/keys/{target}", "body": {"value": {"updated": True}}})
        elif op == "delete":
            requests.append({"method": "DELETE", "path": f"/keys/{target}"})
        elif op == "list":
            requests.append({"method": "GET", "path": "/keys?prefix=item:&limit=10"})

    with open(os.path.join(dir_path, "requests.jsonl"), "w") as f:
        for req in requests:
            f.write(json.dumps(req) + "\n")

    with open(os.path.join(dir_path, "store.json"), "w") as f:
        json.dump({"entries": {}}, f)

if __name__ == "__main__":
    main()
