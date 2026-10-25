#!/usr/bin/env python3
"""Generate deterministic workload for d4-inventory-allocator."""

import json
import os

def main():
    target_dir = os.path.dirname(os.path.abspath(__file__))
    inv_file = os.path.join(target_dir, "inventory.json")
    ord_file = os.path.join(target_dir, "orders.json")

    items = {}
    for i in range(10):
        sku = f"SKU-{i:03d}"
        items[sku] = {
            "lots": [
                {"lot_id": f"LOT-{i}-A", "quantity": 50, "expires": f"2026-11-{10 + (i % 15):02d}"},
                {"lot_id": f"LOT-{i}-B", "quantity": 100, "expires": f"2026-12-{10 + (i % 15):02d}"}
            ]
        }

    orders = []
    priorities = ["VIP", "STANDARD", "BULK"]
    for j in range(25):
        req_items = {}
        for k in range(3):
            sku_idx = (j + k) % 10
            req_items[f"SKU-{sku_idx:03d}"] = 10 + (j % 5) * 5

        orders.append({
            "id": f"order-{j:04d}",
            "priority": priorities[j % 3],
            "items": req_items
        })

    with open(inv_file, "w") as f:
        json.dump({"items": items}, f, indent=2)

    with open(ord_file, "w") as f:
        json.dump({"orders": orders}, f, indent=2)

    print(f"Generated {len(items)} items and {len(orders)} orders in {target_dir}")

if __name__ == "__main__":
    main()
