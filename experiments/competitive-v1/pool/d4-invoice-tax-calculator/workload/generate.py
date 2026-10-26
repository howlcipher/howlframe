#!/usr/bin/env python3
"""Generate deterministic workload for d4-invoice-tax-calculator."""

import json
import os

def main():
    target_dir = os.path.dirname(os.path.abspath(__file__))
    cfg_file = os.path.join(target_dir, "tax_config.json")
    inv_file = os.path.join(target_dir, "invoice.json")

    config = {
        "categories": {
            "standard": 0.10,
            "reduced": 0.05,
            "exempt": 0.00,
            "services": 0.08
        },
        "luxury_surcharge": {
            "threshold_cents": 10000,
            "rate": 0.05
        }
    }

    categories = ["standard", "reduced", "exempt", "services"]
    items = []
    for i in range(25):
        price = 2000 + (i * 1250) # some will exceed 10000 threshold
        items.append({
            "id": f"inv-item-{i:03d}",
            "description": f"Line item description {i}",
            "category": categories[i % 4],
            "unit_price_cents": price,
            "quantity": 1 + (i % 3)
        })

    invoice = {
        "items": items,
        "discount_cents": 1500
    }

    with open(cfg_file, "w") as f:
        json.dump(config, f, indent=2)

    with open(inv_file, "w") as f:
        json.dump(invoice, f, indent=2)

    print(f"Generated tax config and invoice with {len(items)} items in {target_dir}")

if __name__ == "__main__":
    main()
