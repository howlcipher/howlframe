import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../kit")))
import testkit


class TestMetricTimeseries(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.input_path = os.path.join(self.temp_dir.name, "metrics.jsonl")
        self.out_path = os.path.join(self.temp_dir.name, "rollups.json")

    def tearDown(self):
        self.temp_dir.cleanup()

    def _write_lines(self, lines):
        with open(self.input_path, "w") as f:
            for line in lines:
                if isinstance(line, dict):
                    f.write(json.dumps(line) + "\n")
                else:
                    f.write(str(line) + "\n")

    def test_basic_rollup(self):
        records = [
            {"timestamp": 125, "host_id": "h1", "metric": "cpu", "value": 20.0},
            {"timestamp": 140, "host_id": "h1", "metric": "cpu", "value": 30.0}
        ]
        self._write_lines(records)
        res = testkit.run(["rollup", "--input", self.input_path, "--interval", "60", "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["interval_seconds"], 60)
        self.assertEqual(out["total_points_processed"], 2)
        self.assertEqual(len(out["windows"]), 1)
        w = out["windows"][0]
        self.assertEqual(w["bucket_start"], 120)
        self.assertEqual(w["count"], 2)
        self.assertEqual(w["min"], 20.0)
        self.assertEqual(w["max"], 30.0)
        self.assertEqual(w["avg"], 25.0)

    def test_multiple_windows(self):
        records = [
            {"timestamp": 125, "host_id": "h1", "metric": "cpu", "value": 20.0},
            {"timestamp": 190, "host_id": "h1", "metric": "cpu", "value": 40.0}
        ]
        self._write_lines(records)
        res = testkit.run(["rollup", "--input", self.input_path, "--interval", "60", "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(len(out["windows"]), 2)
        self.assertEqual(out["windows"][0]["bucket_start"], 120)
        self.assertEqual(out["windows"][1]["bucket_start"], 180)

    def test_different_hosts_different_windows(self):
        records = [
            {"timestamp": 125, "host_id": "host-A", "metric": "cpu", "value": 20.0},
            {"timestamp": 125, "host_id": "host-B", "metric": "cpu", "value": 30.0}
        ]
        self._write_lines(records)
        res = testkit.run(["rollup", "--input", self.input_path, "--interval", "60", "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(len(out["windows"]), 2)
        self.assertEqual(out["windows"][0]["host_id"], "host-A")
        self.assertEqual(out["windows"][1]["host_id"], "host-B")

    def test_different_metrics_different_windows(self):
        records = [
            {"timestamp": 125, "host_id": "h1", "metric": "cpu", "value": 20.0},
            {"timestamp": 125, "host_id": "h1", "metric": "ram", "value": 1024.0}
        ]
        self._write_lines(records)
        res = testkit.run(["rollup", "--input", self.input_path, "--interval", "60", "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(len(out["windows"]), 2)
        self.assertEqual(out["windows"][0]["metric"], "cpu")
        self.assertEqual(out["windows"][1]["metric"], "ram")

    def test_host_filter(self):
        records = [
            {"timestamp": 125, "host_id": "host-A", "metric": "cpu", "value": 20.0},
            {"timestamp": 125, "host_id": "host-B", "metric": "cpu", "value": 30.0}
        ]
        self._write_lines(records)
        res = testkit.run(["rollup", "--input", self.input_path, "--interval", "60", "--output", self.out_path, "--host", "host-A"])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["total_points_processed"], 1)
        self.assertEqual(len(out["windows"]), 1)
        self.assertEqual(out["windows"][0]["host_id"], "host-A")

    def test_p95_percentile_calculation(self):
        # 20 points from 1 to 20 at timestamp 60..79:
        # count = 20, 0.95 * 20 = 19. ceil(19) - 1 = 18 -> index 18 has value 19.
        records = [{"timestamp": 60 + i, "host_id": "h1", "metric": "m", "value": float(i + 1)} for i in range(20)]
        self._write_lines(records)
        res = testkit.run(["rollup", "--input", self.input_path, "--interval", "60", "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        w = out["windows"][0]
        self.assertEqual(w["p95"], 19.0)

    def test_min_max_sum_avg_rounding(self):
        records = [
            {"timestamp": 10, "host_id": "h1", "metric": "m", "value": 10.333},
            {"timestamp": 20, "host_id": "h1", "metric": "m", "value": 20.666}
        ]
        self._write_lines(records)
        res = testkit.run(["rollup", "--input", self.input_path, "--interval", "60", "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        w = out["windows"][0]
        self.assertEqual(w["min"], 10.333)
        self.assertEqual(w["max"], 20.666)
        self.assertEqual(w["sum"], 31.0)
        self.assertEqual(w["avg"], 15.5)

    def test_skipped_corrupt_lines(self):
        lines = [
            "{corrupt: json}",
            {"timestamp": "not_an_int", "host_id": "h1", "metric": "m", "value": 10.0},
            {"timestamp": 10, "host_id": "h1", "metric": "m", "value": "not_a_num"},
            {"timestamp": 10, "host_id": "h1", "value": 10.0}, # missing metric
            {"timestamp": 10, "host_id": "h1", "metric": "m", "value": 10.0}
        ]
        self._write_lines(lines)
        res = testkit.run(["rollup", "--input", self.input_path, "--interval", "60", "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["total_points_processed"], 1)
        self.assertEqual(out["skipped_lines"], 4)

    def test_sorting_of_windows(self):
        records = [
            {"timestamp": 125, "host_id": "h1", "metric": "b", "value": 1.0},
            {"timestamp": 10, "host_id": "h2", "metric": "a", "value": 1.0},
            {"timestamp": 10, "host_id": "h1", "metric": "z", "value": 1.0},
            {"timestamp": 10, "host_id": "h1", "metric": "a", "value": 1.0}
        ]
        self._write_lines(records)
        res = testkit.run(["rollup", "--input", self.input_path, "--interval", "60", "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        keys = [(w["bucket_start"], w["host_id"], w["metric"]) for w in out["windows"]]
        expected = [
            (0, "h1", "a"),
            (0, "h1", "z"),
            (0, "h2", "a"),
            (120, "h1", "b")
        ]
        self.assertEqual(keys, expected)

    def test_missing_required_flags_exit_1(self):
        res = testkit.run(["rollup", "--input", self.input_path])
        self.assertEqual(res.returncode, 1)

    def test_invalid_interval_exit_1(self):
        res = testkit.run(["rollup", "--input", self.input_path, "--interval", "0", "--output", self.out_path])
        self.assertEqual(res.returncode, 1)

    def test_wrong_subcommand_exit_1(self):
        res = testkit.run(["downsample", "--input", self.input_path, "--interval", "60", "--output", self.out_path])
        self.assertEqual(res.returncode, 1)

    def test_missing_input_file_exit_2(self):
        res = testkit.run(["rollup", "--input", "/missing/file.jsonl", "--interval", "60", "--output", self.out_path])
        self.assertEqual(res.returncode, 2)


if __name__ == "__main__":
    unittest.main()
