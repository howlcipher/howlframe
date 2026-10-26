import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../kit")))
import testkit


class TestSensorTelemetry(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.config_path = os.path.join(self.temp_dir.name, "config.json")
        self.data_path = os.path.join(self.temp_dir.name, "data.jsonl")
        self.out_path = os.path.join(self.temp_dir.name, "report.json")

        self.default_config = {
            "temperature": {"min": 18.0, "max": 28.0, "unit": "celsius"},
            "humidity": {"min": 40.0, "max": 80.0, "unit": "percent"}
        }
        with open(self.config_path, "w") as f:
            json.dump(self.default_config, f)

    def tearDown(self):
        self.temp_dir.cleanup()

    def _write_data(self, lines):
        with open(self.data_path, "w") as f:
            for line in lines:
                if isinstance(line, dict):
                    f.write(json.dumps(line) + "\n")
                else:
                    f.write(str(line) + "\n")

    def test_basic_audit(self):
        records = [
            {"timestamp": "2026-10-01T10:00:00Z", "greenhouse_id": "GH-1", "sensor_type": "temperature", "value": 22.0, "unit": "celsius"},
            {"timestamp": "2026-10-01T10:05:00Z", "greenhouse_id": "GH-1", "sensor_type": "temperature", "value": 30.0, "unit": "celsius"}
        ]
        self._write_data(records)
        res = testkit.run(["audit", "--data", self.data_path, "--config", self.config_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            report = json.load(f)
        self.assertEqual(report["valid_records"], 2)
        self.assertEqual(report["malformed_records"], 0)
        self.assertEqual(report["invalid_records"], 0)
        self.assertEqual(len(report["anomalies"]), 1)
        self.assertEqual(report["anomalies"][0]["type"], "above_max")

    def test_anomaly_detection_above(self):
        records = [
            {"timestamp": "2026-10-01T10:00:00Z", "greenhouse_id": "GH-2", "sensor_type": "temperature", "value": 29.5, "unit": "celsius"}
        ]
        self._write_data(records)
        res = testkit.run(["audit", "--data", self.data_path, "--config", self.config_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            report = json.load(f)
        self.assertEqual(len(report["anomalies"]), 1)
        a = report["anomalies"][0]
        self.assertEqual(a["type"], "above_max")
        self.assertEqual(a["threshold"], 28.0)
        self.assertEqual(a["value"], 29.5)

    def test_anomaly_detection_below(self):
        records = [
            {"timestamp": "2026-10-01T10:00:00Z", "greenhouse_id": "GH-1", "sensor_type": "humidity", "value": 35.0, "unit": "percent"}
        ]
        self._write_data(records)
        res = testkit.run(["audit", "--data", self.data_path, "--config", self.config_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            report = json.load(f)
        self.assertEqual(len(report["anomalies"]), 1)
        a = report["anomalies"][0]
        self.assertEqual(a["type"], "below_min")
        self.assertEqual(a["threshold"], 40.0)

    def test_normal_readings_no_anomaly(self):
        records = [
            {"timestamp": "2026-10-01T10:00:00Z", "greenhouse_id": "GH-1", "sensor_type": "temperature", "value": 18.0, "unit": "celsius"},
            {"timestamp": "2026-10-01T10:01:00Z", "greenhouse_id": "GH-1", "sensor_type": "temperature", "value": 28.0, "unit": "celsius"}
        ]
        self._write_data(records)
        res = testkit.run(["audit", "--data", self.data_path, "--config", self.config_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            report = json.load(f)
        self.assertEqual(report["valid_records"], 2)
        self.assertEqual(len(report["anomalies"]), 0)

    def test_malformed_json_lines(self):
        lines = [
            '{"not_json": ',
            '{"timestamp": "2026-10-01T10:00:00Z", "sensor_type": "temperature"}',  # missing greenhouse_id, value, unit
            '{"timestamp": "2026-10-01T10:00:00Z", "greenhouse_id": "GH-1", "sensor_type": "temperature", "value": "twenty", "unit": "celsius"}', # value not number
            {"timestamp": "2026-10-01T10:00:00Z", "greenhouse_id": "GH-1", "sensor_type": "temperature", "value": 21.0, "unit": "celsius"}
        ]
        self._write_data(lines)
        res = testkit.run(["audit", "--data", self.data_path, "--config", self.config_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            report = json.load(f)
        self.assertEqual(report["valid_records"], 1)
        self.assertEqual(report["malformed_records"], 3)
        self.assertEqual(report["invalid_records"], 0)

    def test_invalid_sensor_type(self):
        records = [
            {"timestamp": "2026-10-01T10:00:00Z", "greenhouse_id": "GH-1", "sensor_type": "radiation", "value": 150.0, "unit": "sievert"}
        ]
        self._write_data(records)
        res = testkit.run(["audit", "--data", self.data_path, "--config", self.config_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            report = json.load(f)
        self.assertEqual(report["valid_records"], 0)
        self.assertEqual(report["invalid_records"], 1)
        self.assertEqual(len(report["anomalies"]), 0)

    def test_invalid_unit(self):
        records = [
            {"timestamp": "2026-10-01T10:00:00Z", "greenhouse_id": "GH-1", "sensor_type": "temperature", "value": 72.0, "unit": "fahrenheit"}
        ]
        self._write_data(records)
        res = testkit.run(["audit", "--data", self.data_path, "--config", self.config_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            report = json.load(f)
        self.assertEqual(report["valid_records"], 0)
        self.assertEqual(report["invalid_records"], 1)

    def test_greenhouse_summary_aggregation(self):
        records = [
            {"timestamp": "2026-10-01T10:00:00Z", "greenhouse_id": "GH-2", "sensor_type": "humidity", "value": 90.0, "unit": "percent"}, # anomaly
            {"timestamp": "2026-10-01T10:01:00Z", "greenhouse_id": "GH-1", "sensor_type": "temperature", "value": 20.0, "unit": "celsius"},
            {"timestamp": "2026-10-01T10:02:00Z", "greenhouse_id": "GH-1", "sensor_type": "humidity", "value": 50.0, "unit": "percent"}
        ]
        self._write_data(records)
        res = testkit.run(["audit", "--data", self.data_path, "--config", self.config_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            report = json.load(f)
        gh1 = report["greenhouse_summary"]["GH-1"]
        self.assertEqual(gh1["reading_count"], 2)
        self.assertEqual(gh1["anomaly_count"], 0)
        self.assertEqual(gh1["sensor_types"], ["humidity", "temperature"])

        gh2 = report["greenhouse_summary"]["GH-2"]
        self.assertEqual(gh2["reading_count"], 1)
        self.assertEqual(gh2["anomaly_count"], 1)
        self.assertEqual(gh2["sensor_types"], ["humidity"])

    def test_summary_stdout_flag(self):
        records = [
            {"timestamp": "2026-10-01T10:00:00Z", "greenhouse_id": "GH-1", "sensor_type": "temperature", "value": 35.0, "unit": "celsius"}
        ]
        self._write_data(records)
        res = testkit.run(["audit", "--data", self.data_path, "--config", self.config_path, "--out", self.out_path, "--summary"])
        self.assertEqual(res.returncode, 0)
        summary = json.loads(res.stdout.strip())
        self.assertEqual(summary, {"valid": 1, "anomalies": 1, "malformed": 0})

    def test_missing_required_args(self):
        res = testkit.run(["audit", "--data", self.data_path])
        self.assertEqual(res.returncode, 1)

    def test_unknown_subcommand(self):
        res = testkit.run(["scan", "--data", self.data_path, "--config", self.config_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 1)

    def test_missing_input_file(self):
        res = testkit.run(["audit", "--data", "/non/existent/file.jsonl", "--config", self.config_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 2)

    def test_anomaly_sorting(self):
        records = [
            {"timestamp": "2026-10-01T12:00:00Z", "greenhouse_id": "GH-B", "sensor_type": "temperature", "value": 35.0, "unit": "celsius"},
            {"timestamp": "2026-10-01T10:00:00Z", "greenhouse_id": "GH-A", "sensor_type": "temperature", "value": 35.0, "unit": "celsius"},
            {"timestamp": "2026-10-01T10:00:00Z", "greenhouse_id": "GH-B", "sensor_type": "temperature", "value": 35.0, "unit": "celsius"}
        ]
        self._write_data(records)
        res = testkit.run(["audit", "--data", self.data_path, "--config", self.config_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            report = json.load(f)
        anomalies = report["anomalies"]
        self.assertEqual(len(anomalies), 3)
        self.assertEqual(anomalies[0]["timestamp"], "2026-10-01T10:00:00Z")
        self.assertEqual(anomalies[0]["greenhouse_id"], "GH-A")
        self.assertEqual(anomalies[1]["timestamp"], "2026-10-01T10:00:00Z")
        self.assertEqual(anomalies[1]["greenhouse_id"], "GH-B")
        self.assertEqual(anomalies[2]["timestamp"], "2026-10-01T12:00:00Z")
        self.assertEqual(anomalies[2]["greenhouse_id"], "GH-B")


if __name__ == "__main__":
    unittest.main()
