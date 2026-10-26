#!/usr/bin/env python3
import json
import os
import random

def main():
    random.seed(42)
    dir_path = os.path.dirname(os.path.abspath(__file__))
    
    config = {
        "temperature": {"min": 18.0, "max": 28.0, "unit": "celsius"},
        "humidity": {"min": 45.0, "max": 75.0, "unit": "percent"},
        "co2": {"min": 400.0, "max": 1000.0, "unit": "ppm"},
        "soil_moisture": {"min": 30.0, "max": 60.0, "unit": "percent"}
    }
    with open(os.path.join(dir_path, "config.json"), "w") as f:
        json.dump(config, f, indent=2)

    greenhouses = ["GH-A", "GH-B", "GH-C", "GH-D"]
    sensors = [
        ("temperature", 15.0, 32.0, "celsius"),
        ("humidity", 40.0, 85.0, "percent"),
        ("co2", 350.0, 1100.0, "ppm"),
        ("soil_moisture", 25.0, 65.0, "percent"),
    ]

    with open(os.path.join(dir_path, "data.jsonl"), "w") as f:
        # Write 500 records
        for i in range(500):
            gh = random.choice(greenhouses)
            stype, vmin, vmax, unit = random.choice(sensors)
            val = round(random.uniform(vmin, vmax), 1)
            ts = f"2026-10-01T{i//60:02d}:{i%60:02d}:00Z"
            
            # occasional malformed or invalid
            if i % 50 == 0:
                f.write('{"timestamp": "' + ts + '", "corrupt": true}\n')
            elif i % 70 == 0:
                f.write(json.dumps({"timestamp": ts, "greenhouse_id": gh, "sensor_type": "unknown_sensor", "value": val, "unit": "lux"}) + "\n")
            else:
                f.write(json.dumps({"timestamp": ts, "greenhouse_id": gh, "sensor_type": stype, "value": val, "unit": unit}) + "\n")

if __name__ == "__main__":
    main()
