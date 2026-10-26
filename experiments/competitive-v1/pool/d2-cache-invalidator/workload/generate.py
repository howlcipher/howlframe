#!/usr/bin/env python3
import json
import os
import random

def main():
    random.seed(444333)
    dir_path = os.path.dirname(os.path.abspath(__file__))

    requests = []
    tags_pool = ["apparel", "footwear", "clearance", "featured", "summer", "winter"]

    # Seed entries
    for i in range(1, 151):
        k = f"prod-{i:03d}"
        tags = random.sample(tags_pool, k=random.randint(1, 3))
        requests.append({"method": "POST", "path": "/cache", "body": {
            "key": k,
            "value": {"item_id": i, "sku": f"SKU-{i}"},
            "tags": tags,
            "ttl_seconds": 3600
        }})

    # Mixed queries and purges
    for i in range(200):
        action = random.choices(["get", "purge", "tag_list", "stats"], weights=[0.5, 0.15, 0.25, 0.1])[0]
        if action == "get":
            k = f"prod-{random.randint(1, 150):03d}"
            requests.append({"method": "GET", "path": f"/cache/{k}"})
        elif action == "purge":
            purge_tags = random.sample(tags_pool, k=random.randint(1, 2))
            requests.append({"method": "POST", "path": "/purge", "body": {"tags": purge_tags}})
        elif action == "tag_list":
            t = random.choice(tags_pool)
            requests.append({"method": "GET", "path": f"/tags/{t}"})
        elif action == "stats":
            requests.append({"method": "GET", "path": "/stats"})

    with open(os.path.join(dir_path, "requests.jsonl"), "w") as f:
        for req in requests:
            f.write(json.dumps(req) + "\n")

    with open(os.path.join(dir_path, "store.json"), "w") as f:
        json.dump({"entries": {}, "total_purged": 0}, f)

if __name__ == "__main__":
    main()
