#!/usr/bin/env python3
"""Generate deterministic workload for d4-route-traffic-shaper."""

import json
import os

def main():
    target_dir = os.path.dirname(os.path.abspath(__file__))
    tbl_file = os.path.join(target_dir, "routing_table.json")
    req_file = os.path.join(target_dir, "requests.json")

    table = {
        "backends_health": {
            "api-core-v1": True,
            "api-core-v2": True,
            "api-payment": False,
            "api-payment-fallback": True,
            "api-legacy": True
        },
        "routes": [
            {
                "path_prefix": "/api/v2/core",
                "headers_match": {"X-Internal-Tier": "canary"},
                "target": "api-core-v2"
            },
            {
                "path_prefix": "/api/v2/core",
                "target": "api-core-v1",
                "canary": {
                    "target": "api-core-v2",
                    "weight_pct": 30
                }
            },
            {
                "path_prefix": "/api/v2/payment",
                "target": "api-payment",
                "fallback_target": "api-payment-fallback"
            },
            {
                "path_prefix": "/api/v1",
                "target": "api-legacy"
            }
        ]
    }

    requests = []
    for i in range(25):
        req = {
            "id": f"disp-{i:04d}",
            "path": f"/api/v2/core/resource/{i}" if i % 2 == 0 else f"/api/v2/payment/charge/{i}",
            "user_id": f"customer-{i:03d}",
            "client_ip": f"192.168.1.{i + 10}",
            "headers": {"X-Internal-Tier": "canary"} if i % 7 == 0 else {}
        }
        requests.append(req)

    with open(tbl_file, "w") as f:
        json.dump(table, f, indent=2)

    with open(req_file, "w") as f:
        json.dump({"requests": requests}, f, indent=2)

    print(f"Generated routing table and {len(requests)} requests in {target_dir}")

if __name__ == "__main__":
    main()
