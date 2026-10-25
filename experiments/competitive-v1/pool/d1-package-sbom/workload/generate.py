#!/usr/bin/env python3
import json
import os
import random

def main():
    random.seed(54321)
    dir_path = os.path.dirname(os.path.abspath(__file__))

    policy = {
        "allowed_licenses": ["MIT", "Apache-2.0", "BSD-3-Clause", "ISC"],
        "tier_capabilities": {
            "tier_1_core": ["network", "filesystem_read", "filesystem_write", "process_spawn"],
            "tier_2_plugin": ["filesystem_read", "network"],
            "tier_3_untrusted": []
        },
        "package_tiers": {
            "core-auth": "tier_1_core",
            "db-connector": "tier_1_core",
            "metrics-agent": "tier_2_plugin"
        },
        "default_tier": "tier_3_untrusted"
    }
    with open(os.path.join(dir_path, "policy.json"), "w") as f:
        json.dump(policy, f, indent=2)

    all_caps = ["network", "filesystem_read", "filesystem_write", "process_spawn"]
    licenses = ["MIT", "Apache-2.0", "BSD-3-Clause", "GPL-3.0", "AGPL-3.0"]

    packages = []
    for i in range(1, 201):
        name = f"pkg-{i:03d}"
        if i == 1:
            name = "core-auth"
        elif i == 2:
            name = "metrics-agent"
        lic = random.choice(licenses)
        caps = random.sample(all_caps, k=random.randint(0, 3))
        packages.append({
            "name": name,
            "version": f"1.{i}.0",
            "license": lic,
            "requested_capabilities": caps
        })

    sbom = {
        "application": "enterprise-cluster-daemon",
        "packages": packages
    }
    with open(os.path.join(dir_path, "sbom.json"), "w") as f:
        json.dump(sbom, f, indent=2)

if __name__ == "__main__":
    main()
