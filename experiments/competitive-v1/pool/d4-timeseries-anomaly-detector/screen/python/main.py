#!/usr/bin/env python3
import json
import math
import os
import sys

def fail(msg, code=2):
    sys.stderr.write(f"Error: {msg}\n")
    sys.exit(code)

def main():
    if len(sys.argv) != 4:
        fail("Usage: ./run <config.json> <timeseries.json> <analysis.json>", 2)

    cfg_path = sys.argv[1]
    ts_path = sys.argv[2]
    ana_path = sys.argv[3]

    try:
        with open(cfg_path, "r", encoding="utf-8") as f:
            cfg = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse config JSON: {e}", 2)

    try:
        with open(ts_path, "r", encoding="utf-8") as f:
            ts_data = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse timeseries JSON: {e}", 2)

    if not isinstance(cfg, dict):
        fail("Config must be an object", 2)
    if not isinstance(ts_data, dict) or "samples" not in ts_data or not isinstance(ts_data["samples"], list):
        fail("Timeseries must contain 'samples' array", 2)

    window_size = cfg.get("window_size", 5)
    if not isinstance(window_size, int) or window_size < 2:
        fail("Invalid window_size: must be integer >= 2", 2)

    z_threshold = float(cfg.get("z_threshold", 3.0))
    step_seconds = cfg.get("step_seconds", 0)
    if not isinstance(step_seconds, int) or step_seconds < 0:
        fail("Invalid step_seconds", 2)

    fill_gaps = bool(cfg.get("fill_gaps", False))
    alert_cooldown = cfg.get("alert_cooldown_samples", 0)
    if not isinstance(alert_cooldown, int) or alert_cooldown < 0:
        fail("Invalid alert_cooldown_samples", 2)

    raw_samples = ts_data["samples"]
    cleaned_samples = []
    for s in raw_samples:
        if not isinstance(s, dict) or "timestamp" not in s or "value" not in s:
            fail("Sample must be an object with 'timestamp' and 'value'", 2)
        ts = s["timestamp"]
        val = s["value"]
        if not isinstance(ts, int):
            fail("Sample timestamp must be an integer", 2)
        if not isinstance(val, (int, float)):
            fail("Sample value must be numeric", 2)
        cleaned_samples.append({
            "timestamp": ts,
            "value": float(val),
            "imputed": False
        })

    # Sort by timestamp ascending
    cleaned_samples.sort(key=lambda s: s["timestamp"])

    # Gap imputation
    processed_samples = []
    imputed_count = 0

    if fill_gaps and step_seconds > 0 and len(cleaned_samples) > 1:
        for idx in range(len(cleaned_samples)):
            curr = cleaned_samples[idx]
            if idx > 0:
                prev = processed_samples[-1]
                diff = curr["timestamp"] - prev["timestamp"]
                if diff > step_seconds:
                    t = prev["timestamp"] + step_seconds
                    while t < curr["timestamp"]:
                        processed_samples.append({
                            "timestamp": t,
                            "value": prev["value"],
                            "imputed": True
                        })
                        imputed_count += 1
                        t += step_seconds
            processed_samples.append(curr)
    else:
        processed_samples = cleaned_samples

    output_samples = []
    anomalies_detected = 0
    alerts_emitted = 0
    alerts_suppressed = 0
    last_alert_index = None

    for i, s in enumerate(processed_samples):
        val = s["value"]
        is_anomaly = False
        alert_flag = False
        suppressed_flag = False
        z_score = None

        if i >= window_size:
            # Window of preceding window_size samples: i - window_size to i - 1
            window_vals = [processed_samples[j]["value"] for j in range(i - window_size, i)]
            mu = sum(window_vals) / float(window_size)
            variance = sum((x - mu) ** 2 for x in window_vals) / float(window_size)
            sigma = math.sqrt(variance)

            if sigma > 0:
                z = abs(val - mu) / sigma
                z_score = round(z, 2)
                if z >= z_threshold:
                    is_anomaly = True
            else:
                z_score = 0.0
                is_anomaly = False

        if is_anomaly:
            anomalies_detected += 1
            if last_alert_index is None or (i - last_alert_index) > alert_cooldown:
                alert_flag = True
                suppressed_flag = False
                alerts_emitted += 1
                last_alert_index = i
            else:
                alert_flag = False
                suppressed_flag = True
                alerts_suppressed += 1

        output_samples.append({
            "timestamp": s["timestamp"],
            "value": s["value"],
            "imputed": s["imputed"],
            "is_anomaly": is_anomaly,
            "alert_emitted": alert_flag,
            "suppressed": suppressed_flag,
            "z_score": z_score
        })

    report = {
        "metric": ts_data.get("metric", "metric"),
        "total_samples": len(output_samples),
        "imputed_count": imputed_count,
        "anomalies_detected": anomalies_detected,
        "alerts_emitted": alerts_emitted,
        "alerts_suppressed": alerts_suppressed,
        "samples": output_samples
    }

    try:
        with open(ana_path, "w", encoding="utf-8") as f:
            json.dump(report, f, indent=2)
    except Exception as e:
        fail(f"Failed to write analysis JSON: {e}", 2)

    sys.exit(1 if alerts_emitted > 0 else 0)

if __name__ == "__main__":
    main()
