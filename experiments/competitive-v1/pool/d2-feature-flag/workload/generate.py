#!/usr/bin/env python3
import json
import os
import random

def main():
    random.seed(6789)
    dir_path = os.path.dirname(os.path.abspath(__file__))

    requests = []
    # Create flags
    requests.append({"method": "POST", "path": "/flags", "body": {
        "key": "beta_search",
        "enabled": True,
        "min_tier": "basic",
        "allowed_roles": ["admin", "editor", "viewer"],
        "user_overrides": ["u-beta-tester"]
    }})
    requests.append({"method": "POST", "path": "/flags", "body": {
        "key": "export_pdf",
        "enabled": True,
        "min_tier": "pro",
        "allowed_roles": ["admin", "editor"],
        "user_overrides": []
    }})
    requests.append({"method": "POST", "path": "/flags", "body": {
        "key": "llm_agent",
        "enabled": True,
        "min_tier": "enterprise",
        "allowed_roles": ["admin"],
        "user_overrides": ["u-special"]
    }})

    flag_keys = ["beta_search", "export_pdf", "llm_agent"]
    tiers = ["basic", "pro", "enterprise"]
    roles = ["admin", "editor", "viewer", "guest"]
    users = ["u-1", "u-2", "u-special", "u-beta-tester"]

    for i in range(300):
        fk = random.choice(flag_keys)
        t = random.choice(tiers)
        r = random.choice(roles)
        u = random.choice(users)
        requests.append({"method": "POST", "path": "/evaluate", "body": {
            "key": fk,
            "user_id": u,
            "tier": t,
            "role": r
        }})
        if i % 60 == 0:
            requests.append({"method": "GET", "path": "/flags"})

    with open(os.path.join(dir_path, "requests.jsonl"), "w") as f:
        for req in requests:
            f.write(json.dumps(req) + "\n")

    with open(os.path.join(dir_path, "store.json"), "w") as f:
        json.dump({"flags": {}}, f)

if __name__ == "__main__":
    main()
