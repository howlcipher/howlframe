#!/usr/bin/env python3
import json
import os
import random

def main():
    random.seed(55555)
    dir_path = os.path.dirname(os.path.abspath(__file__))

    requests = []
    # Enqueue jobs
    for i in range(1, 101):
        prio = random.choice([0, 5, 10, 20])
        requests.append({"method": "POST", "path": "/jobs", "body": {
            "topic": "tasks",
            "payload": {"task_num": i},
            "priority": prio,
            "max_retries": 2,
            "lease_seconds": 60
        }})

    # Workers leasing and acking/failing
    workers = ["w-1", "w-2", "w-3"]
    for i in range(150):
        w = random.choice(workers)
        requests.append({"method": "POST", "path": "/jobs/lease", "body": {"topic": "tasks", "worker_id": w}})
        if i % 3 == 0:
            target_id = f"job-{random.randint(1, min(i+1, 100))}"
            requests.append({"method": "POST", "path": f"/jobs/{target_id}/ack", "body": {"worker_id": w}})
        elif i % 5 == 0:
            target_id = f"job-{random.randint(1, min(i+1, 100))}"
            requests.append({"method": "POST", "path": f"/jobs/{target_id}/fail", "body": {"worker_id": w, "error": "simulated"}})
        if i % 40 == 0:
            requests.append({"method": "GET", "path": "/stats"})

    with open(os.path.join(dir_path, "requests.jsonl"), "w") as f:
        for req in requests:
            f.write(json.dumps(req) + "\n")

    with open(os.path.join(dir_path, "store.json"), "w") as f:
        json.dump({"jobs": {}, "next_id": 1}, f)

if __name__ == "__main__":
    main()
