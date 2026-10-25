#!/usr/bin/env python3
"""Generate deterministic workload for d4-timeseries-anomaly-detector."""

import json
import os

def main():
    target_dir = os.path.dirname(os.path.abspath(__file__))
    cfg_file = os.path.join(target_dir, "config.json")
    ts_file = os.path.join(target_dir, "timeseries.json")

    config = {
        "window_size": 5,
        "z_threshold": 3.0,
        "step_seconds": 10,
        "fill_gaps": True,
        "alert_cooldown_samples": 3
    }

    samples = []
    base_time = 1000
    for i in range(30):
        # Introduce a gap around sample 10
        gap = 30 if i == 10 else 10
        base_time += gap

        # Normal fluctuations around 20.0
        val = 20.0 + (i % 3) * 1.5
        # Spike anomalies at index 15 and 16
        if i == 15:
            val = 80.0
        elif i == 16:
            val = 85.0

        samples.append({
            "timestamp": base_time,
            "value": val
        })

    with open(cfg_file, "w") as f:
        json.dump(config, f, indent=2)

    with open(ts_file, "w") as f:
        json.dump({"metric": "node_mem_used_pct", "samples": samples}, f, indent=2)

    print(f"Generated config and timeseries with {len(samples)} samples in {target_dir}")

if __name__ == "__main__":
    main()
