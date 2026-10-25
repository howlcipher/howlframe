#!/usr/bin/env python3
import json
import os
import random

def main():
    random.seed(1337)
    dir_path = os.path.dirname(os.path.abspath(__file__))

    rules = {
        "max_container_weight_kg": 1200.0,
        "max_hazard_items_per_container": 3,
        "allowed_destinations": ["NORTH", "SOUTH", "EAST", "WEST", "CENTRAL"]
    }
    with open(os.path.join(dir_path, "rules.json"), "w") as f:
        json.dump(rules, f, indent=2)

    destinations = ["NORTH", "SOUTH", "EAST", "WEST", "CENTRAL"]
    with open(os.path.join(dir_path, "packages.csv"), "w") as f:
        f.write("package_id,destination,weight_kg,is_hazardous,priority\n")
        for i in range(1, 301):
            pkg_id = f"PKG-{i:04d}"
            dest = random.choice(destinations) if i % 25 != 0 else "UNKNOWN_ZONE"
            weight = round(random.uniform(10.0, 450.0), 1) if i % 40 != 0 else 1500.0
            is_haz = "true" if random.random() < 0.25 else "false"
            prio = random.randint(1, 5)
            f.write(f"{pkg_id},{dest},{weight},{is_haz},{prio}\n")

if __name__ == "__main__":
    main()
