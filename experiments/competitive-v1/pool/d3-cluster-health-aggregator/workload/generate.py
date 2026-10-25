#!/usr/bin/env python3
"""Generate deterministic workload for d3-cluster-health-aggregator."""

import json
import os

def main():
    target_dir = os.path.dirname(os.path.abspath(__file__))
    cluster_file = os.path.join(target_dir, "cluster.json")
    fixtures_file = os.path.join(target_dir, "fixtures.json")

    services = []
    fixtures = {}

    for i in range(12):
        sname = f"cluster-svc-{i:02d}"
        p_url = f"http://127.0.0.1:81{i:02d}/health"
        s_url = f"http://127.0.0.1:82{i:02d}/health"
        a_url = f"http://127.0.0.1:82{i:02d}/v1/activate"

        services.append({
            "name": sname,
            "primary_url": p_url,
            "standby_url": s_url,
            "activate_url": a_url
        })

        if i % 3 == 0:
            # Primary healthy
            fixtures[p_url] = {"status": 200, "body": {"status": "healthy"}}
        else:
            # Primary down, standby healthy, activation succeeds
            fixtures[p_url] = {"status": 503, "body": {"status": "down"}}
            fixtures[s_url] = {"status": 200, "body": {"status": "healthy"}}
            fixtures[a_url] = {"status": 200, "body": {"activated": True}}

    cluster = {
        "timeout_ms": 2000,
        "services": services
    }

    with open(cluster_file, "w") as f:
        json.dump(cluster, f, indent=2)

    with open(fixtures_file, "w") as f:
        json.dump(fixtures, f, indent=2)

    print(f"Generated cluster with {len(services)} services in {target_dir}")

if __name__ == "__main__":
    main()
