#!/usr/bin/env python3
"""Generate a deterministic webhook events workload."""

import json
import os

def main():
    target_dir = os.path.dirname(os.path.abspath(__file__))
    out_file = os.path.join(target_dir, "events.json")

    events = []
    for i in range(25):
        events.append({
            "id": f"evt-{i:04d}",
            "topic": "telemetry.metric" if i % 2 == 0 else "transaction.processed",
            "payload": {
                "sequence": i,
                "node": f"worker-node-{i % 4}",
                "value": round(i * 1.5, 2),
                "active": True
            },
            "max_attempts": 3,
            "timeout_ms": 1000
        })

    with open(out_file, "w") as f:
        json.dump({"events": events}, f, indent=2)

    print(f"Generated {len(events)} events in {out_file}")

if __name__ == "__main__":
    main()
