import json
import os
import tempfile
import unittest
import testkit

class TestTimeseriesAnomalyDetector(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()
        self.cfg_path = os.path.join(self.tmpdir.name, "config.json")
        self.ts_path = os.path.join(self.tmpdir.name, "timeseries.json")
        self.ana_path = os.path.join(self.tmpdir.name, "analysis.json")

        self.default_config = {
            "window_size": 3,
            "z_threshold": 2.5,
            "step_seconds": 10,
            "fill_gaps": False,
            "alert_cooldown_samples": 2
        }
        with open(self.cfg_path, "w") as f:
            json.dump(self.default_config, f)

    def tearDown(self):
        self.tmpdir.cleanup()

    def test_steady_state_no_anomalies_exit_0(self):
        # 6 samples with steady value 10.0
        with open(self.ts_path, "w") as f:
            json.dump({
                "metric": "m1",
                "samples": [
                    {"timestamp": 100 + i * 10, "value": 10.0}
                    for i in range(6)
                ]
            }, f)
        res = testkit.run([self.cfg_path, self.ts_path, self.ana_path])
        self.assertEqual(res.returncode, 0)
        with open(self.ana_path) as f:
            data = json.load(f)
        self.assertEqual(data["total_samples"], 6)
        self.assertEqual(data["anomalies_detected"], 0)
        self.assertEqual(data["alerts_emitted"], 0)
        self.assertEqual(data["alerts_suppressed"], 0)

    def test_warmup_phase_no_anomalies(self):
        # Window size is 3 -> first 3 samples (indices 0, 1, 2) are warmup
        with open(self.ts_path, "w") as f:
            json.dump({
                "metric": "m1",
                "samples": [
                    {"timestamp": 100, "value": 10.0},
                    {"timestamp": 110, "value": 999.0}, # spike during warmup!
                    {"timestamp": 120, "value": 10.0}
                ]
            }, f)
        res = testkit.run([self.cfg_path, self.ts_path, self.ana_path])
        self.assertEqual(res.returncode, 0)
        with open(self.ana_path) as f:
            data = json.load(f)
        self.assertFalse(data["samples"][1]["is_anomaly"])
        self.assertIsNone(data["samples"][1]["z_score"])

    def test_single_spike_triggers_anomaly_and_alert(self):
        # Window size 3:
        # samples 0, 1, 2: values 10.0, 11.0, 10.0 (mean ~10.33, std ~0.47)
        # sample 3: value 30.0 -> z-score > 2.5 -> anomaly!
        with open(self.ts_path, "w") as f:
            json.dump({
                "metric": "m-spike",
                "samples": [
                    {"timestamp": 100, "value": 10.0},
                    {"timestamp": 110, "value": 11.0},
                    {"timestamp": 120, "value": 10.0},
                    {"timestamp": 130, "value": 30.0}
                ]
            }, f)
        res = testkit.run([self.cfg_path, self.ts_path, self.ana_path])
        self.assertEqual(res.returncode, 1)
        with open(self.ana_path) as f:
            data = json.load(f)
        self.assertEqual(data["anomalies_detected"], 1)
        self.assertEqual(data["alerts_emitted"], 1)
        spike_sample = data["samples"][3]
        self.assertTrue(spike_sample["is_anomaly"])
        self.assertTrue(spike_sample["alert_emitted"])
        self.assertFalse(spike_sample["suppressed"])
        self.assertGreater(spike_sample["z_score"], 2.5)

    def test_alert_cooldown_throttling(self):
        # cooldown is 2.
        # Two consecutive spike samples (indices 3 and 4)
        with open(self.ts_path, "w") as f:
            json.dump({
                "metric": "m-cooldown",
                "samples": [
                    {"timestamp": 100, "value": 10.0},
                    {"timestamp": 110, "value": 11.0},
                    {"timestamp": 120, "value": 10.0},
                    {"timestamp": 130, "value": 35.0}, # anomaly 1 (alert emitted)
                    {"timestamp": 140, "value": 100.0}  # anomaly 2 (within cooldown of 2 samples, suppressed)
                ]
            }, f)
        res = testkit.run([self.cfg_path, self.ts_path, self.ana_path])
        self.assertEqual(res.returncode, 1)
        with open(self.ana_path) as f:
            data = json.load(f)
        self.assertEqual(data["anomalies_detected"], 2)
        self.assertEqual(data["alerts_emitted"], 1)
        self.assertEqual(data["alerts_suppressed"], 1)
        self.assertTrue(data["samples"][3]["alert_emitted"])
        self.assertFalse(data["samples"][3]["suppressed"])
        self.assertFalse(data["samples"][4]["alert_emitted"])
        self.assertTrue(data["samples"][4]["suppressed"])

    def test_alert_emits_again_after_cooldown_passes(self):
        # cooldown is 1 sample.
        # index 3: spike (alert emitted)
        # indices 4, 5, 6: recovery with slight variance
        # index 7: spike (alert emitted again)
        cfg = {
            "window_size": 3,
            "z_threshold": 2.0,
            "alert_cooldown_samples": 1
        }
        with open(self.cfg_path, "w") as f:
            json.dump(cfg, f)

        with open(self.ts_path, "w") as f:
            json.dump({
                "metric": "m-cooldown-pass",
                "samples": [
                    {"timestamp": 100, "value": 10.0},
                    {"timestamp": 110, "value": 10.5},
                    {"timestamp": 120, "value": 10.0},
                    {"timestamp": 130, "value": 30.0}, # spike 1 (alert)
                    {"timestamp": 140, "value": 10.0},
                    {"timestamp": 150, "value": 10.5},
                    {"timestamp": 160, "value": 10.0},
                    {"timestamp": 170, "value": 35.0}  # spike 2 (alert)
                ]
            }, f)
        res = testkit.run([self.cfg_path, self.ts_path, self.ana_path])
        self.assertEqual(res.returncode, 1)
        with open(self.ana_path) as f:
            data = json.load(f)
        self.assertEqual(data["alerts_emitted"], 2)

    def test_timestamp_gap_imputation(self):
        # Gap between timestamp 100 and 130 with step_seconds = 10
        # Should insert timestamps 110 and 120 with forward-filled value 5.0
        cfg = {
            "window_size": 2,
            "step_seconds": 10,
            "fill_gaps": True
        }
        with open(self.cfg_path, "w") as f:
            json.dump(cfg, f)

        with open(self.ts_path, "w") as f:
            json.dump({
                "metric": "m-gap",
                "samples": [
                    {"timestamp": 100, "value": 5.0},
                    {"timestamp": 130, "value": 8.0}
                ]
            }, f)
        res = testkit.run([self.cfg_path, self.ts_path, self.ana_path])
        self.assertEqual(res.returncode, 0)
        with open(self.ana_path) as f:
            data = json.load(f)
        self.assertEqual(data["total_samples"], 4)
        self.assertEqual(data["imputed_count"], 2)
        # Check timestamps
        ts_list = [s["timestamp"] for s in data["samples"]]
        self.assertEqual(ts_list, [100, 110, 120, 130])
        # Imputed samples have value 5.0 and imputed=True
        self.assertTrue(data["samples"][1]["imputed"])
        self.assertEqual(data["samples"][1]["value"], 5.0)
        self.assertTrue(data["samples"][2]["imputed"])
        self.assertEqual(data["samples"][2]["value"], 5.0)

    def test_zero_variance_handled(self):
        # All preceding values identical -> std dev is 0
        with open(self.ts_path, "w") as f:
            json.dump({
                "metric": "m-zero-var",
                "samples": [
                    {"timestamp": 100, "value": 15.0},
                    {"timestamp": 110, "value": 15.0},
                    {"timestamp": 120, "value": 15.0},
                    {"timestamp": 130, "value": 15.0}
                ]
            }, f)
        res = testkit.run([self.cfg_path, self.ts_path, self.ana_path])
        self.assertEqual(res.returncode, 0)
        with open(self.ana_path) as f:
            data = json.load(f)
        self.assertEqual(data["samples"][3]["z_score"], 0.0)
        self.assertFalse(data["samples"][3]["is_anomaly"])

    def test_unordered_timestamps_sorted(self):
        with open(self.ts_path, "w") as f:
            json.dump({
                "metric": "m-order",
                "samples": [
                    {"timestamp": 120, "value": 12.0},
                    {"timestamp": 100, "value": 10.0},
                    {"timestamp": 110, "value": 11.0}
                ]
            }, f)
        res = testkit.run([self.cfg_path, self.ts_path, self.ana_path])
        self.assertEqual(res.returncode, 0)
        with open(self.ana_path) as f:
            data = json.load(f)
        ts_list = [s["timestamp"] for s in data["samples"]]
        self.assertEqual(ts_list, [100, 110, 120])

    def test_missing_args(self):
        res = testkit.run([])
        self.assertEqual(res.returncode, 2)

    def test_malformed_json(self):
        bad_path = os.path.join(self.tmpdir.name, "bad.json")
        with open(bad_path, "w") as f:
            f.write("invalid")
        res = testkit.run([self.cfg_path, bad_path, self.ana_path])
        self.assertEqual(res.returncode, 2)

    def test_invalid_window_size(self):
        with open(self.cfg_path, "w") as f:
            json.dump({"window_size": 1}, f) # window_size must be >= 2
        with open(self.ts_path, "w") as f:
            json.dump({"metric": "m", "samples": []}, f)
        res = testkit.run([self.cfg_path, self.ts_path, self.ana_path])
        self.assertEqual(res.returncode, 2)
