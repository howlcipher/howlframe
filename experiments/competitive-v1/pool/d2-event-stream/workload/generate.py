#!/usr/bin/env python3
import json
import os
import random

def main():
    random.seed(888123)
    dir_path = os.path.dirname(os.path.abspath(__file__))

    requests = []
    # Create topics
    requests.append({"method": "POST", "path": "/topics", "body": {"topic": "clicks", "max_events": 500}})
    requests.append({"method": "POST", "path": "/topics", "body": {"topic": "views", "max_events": 200}})

    topics = ["clicks", "views"]
    groups = ["analytics-group", "reporting-group"]

    for i in range(1, 301):
        t = random.choice(topics)
        requests.append({"method": "POST", "path": f"/topics/{t}/publish", "body": {
            "event_type": "interaction",
            "payload": {"seq": i, "user_id": f"u-{i%20}"}
        }})

        if i % 5 == 0:
            g = random.choice(groups)
            requests.append({"method": "POST", "path": f"/subscriptions/{g}/consume", "body": {
                "topic": t,
                "limit": 5,
                "auto_commit": True
            }})

        if i % 50 == 0:
            requests.append({"method": "GET", "path": f"/topics/{t}"})

    with open(os.path.join(dir_path, "requests.jsonl"), "w") as f:
        for req in requests:
            f.write(json.dumps(req) + "\n")

    with open(os.path.join(dir_path, "store.json"), "w") as f:
        json.dump({"topics": {}, "subscriptions": {}}, f)

if __name__ == "__main__":
    main()
