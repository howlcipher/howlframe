"""Acceptance tests for calib-apilog-analyzer."""

import json
import os
import tempfile
import unittest
import testkit


class TestApiLogAnalyzer(unittest.TestCase):
    def test_basic_stdin_to_stdout(self):
        records = [
            {
                "timestamp": 1710000000,
                "service": "auth-service",
                "client_id": "client-1",
                "method": "POST",
                "path": "/api/v1/login",
                "status": 200,
                "latency_ms": 45,
                "bytes_sent": 512,
            },
            {
                "timestamp": 1710000010,
                "service": "auth-service",
                "client_id": "client-1",
                "method": "GET",
                "path": "/api/v1/verify",
                "status": 200,
                "latency_ms": 15,
                "bytes_sent": 128,
            },
        ]
        stdin_data = "\n".join(json.dumps(r) for r in records) + "\n"
        proc = testkit.run(args=(), stdin=stdin_data)
        self.assertEqual(proc.returncode, 0, f"stderr: {proc.stderr}")
        data = json.loads(proc.stdout)
        self.assertEqual(data["summary"]["total_records"], 2)
        self.assertEqual(data["summary"]["valid_records"], 2)
        self.assertEqual(data["summary"]["invalid_records"], 0)
        self.assertEqual(len(data["services"]), 1)
        self.assertEqual(data["services"][0]["service"], "auth-service")
        self.assertEqual(data["services"][0]["total_requests"], 2)

    def test_file_input_and_file_output(self):
        records = [
            {
                "timestamp": 1710000000,
                "service": "billing",
                "client_id": "c1",
                "method": "POST",
                "path": "/charge",
                "status": 201,
                "latency_ms": 100,
                "bytes_sent": 250,
            }
        ]
        with tempfile.TemporaryDirectory() as td:
            in_path = os.path.join(td, "in.jsonl")
            out_path = os.path.join(td, "out.json")
            with open(in_path, "w", encoding="utf-8") as f:
                f.write(json.dumps(records[0]) + "\n")

            proc = testkit.run(args=["--input", in_path, "--output", out_path])
            self.assertEqual(proc.returncode, 0, f"stderr: {proc.stderr}")
            self.assertEqual(proc.stdout.strip(), "")
            self.assertTrue(os.path.exists(out_path))

            with open(out_path, "r", encoding="utf-8") as f:
                data = json.load(f)
            self.assertEqual(data["summary"]["valid_records"], 1)
            self.assertEqual(data["services"][0]["service"], "billing")

    def test_explicit_dash_for_stdin_stdout(self):
        record = {
            "timestamp": 1710000050,
            "service": "order",
            "client_id": "c99",
            "method": "GET",
            "path": "/orders",
            "status": 200,
            "latency_ms": 50,
            "bytes_sent": 1000,
        }
        proc = testkit.run(
            args=["--input", "-", "--output", "-"],
            stdin=json.dumps(record) + "\n",
        )
        self.assertEqual(proc.returncode, 0, f"stderr: {proc.stderr}")
        data = json.loads(proc.stdout)
        self.assertEqual(data["summary"]["valid_records"], 1)

    def test_summary_aggregations(self):
        records = [
            {
                "timestamp": 1710000100,
                "service": "s1",
                "client_id": "c1",
                "method": "GET",
                "path": "/a",
                "status": 200,
                "latency_ms": 20,
                "bytes_sent": 100,
            },
            {
                "timestamp": 1710000050,
                "service": "s2",
                "client_id": "c2",
                "method": "POST",
                "path": "/b",
                "status": 500,
                "latency_ms": 800,
                "bytes_sent": 300,
            },
            {
                "timestamp": 1710000200,
                "service": "s1",
                "client_id": "c1",
                "method": "GET",
                "path": "/c",
                "status": 404,
                "latency_ms": 40,
                "bytes_sent": 200,
            },
        ]
        stdin_data = "\n".join(json.dumps(r) for r in records) + "\n"
        proc = testkit.run(args=(), stdin=stdin_data)
        self.assertEqual(proc.returncode, 0, f"stderr: {proc.stderr}")
        summary = json.loads(proc.stdout)["summary"]
        self.assertEqual(summary["total_records"], 3)
        self.assertEqual(summary["valid_records"], 3)
        self.assertEqual(summary["invalid_records"], 0)
        self.assertEqual(summary["earliest_timestamp"], 1710000050)
        self.assertEqual(summary["latest_timestamp"], 1710000200)
        self.assertEqual(summary["total_bytes_sent"], 600)
        self.assertEqual(summary["total_services"], 2)
        # s2 has error_rate 1.0 > 0.05 and avg_latency 800 > 500 -> breach
        # s1 has 1 error out of 2 = 0.50 > 0.05 -> breach
        self.assertEqual(summary["services_with_sla_breach"], 2)

    def test_service_metrics_and_sorting(self):
        records = [
            # zebra service
            {
                "timestamp": 1710000000,
                "service": "zebra",
                "client_id": "c1",
                "method": "GET",
                "path": "/z",
                "status": 200,
                "latency_ms": 10,
                "bytes_sent": 100,
            },
            # alpha service: 200, 301, 404, 500, 100
            {
                "timestamp": 1710000001,
                "service": "alpha",
                "client_id": "c1",
                "method": "GET",
                "path": "/1",
                "status": 200,
                "latency_ms": 10,
                "bytes_sent": 10,
            },
            {
                "timestamp": 1710000002,
                "service": "alpha",
                "client_id": "c1",
                "method": "GET",
                "path": "/2",
                "status": 301,
                "latency_ms": 20,
                "bytes_sent": 20,
            },
            {
                "timestamp": 1710000003,
                "service": "alpha",
                "client_id": "c1",
                "method": "GET",
                "path": "/3",
                "status": 404,
                "latency_ms": 30,
                "bytes_sent": 30,
            },
            {
                "timestamp": 1710000004,
                "service": "alpha",
                "client_id": "c1",
                "method": "GET",
                "path": "/4",
                "status": 500,
                "latency_ms": 40,
                "bytes_sent": 40,
            },
            {
                "timestamp": 1710000005,
                "service": "alpha",
                "client_id": "c1",
                "method": "GET",
                "path": "/5",
                "status": 101,
                "latency_ms": 50,
                "bytes_sent": 50,
            },
        ]
        proc = testkit.run(
            args=(), stdin="\n".join(json.dumps(r) for r in records) + "\n"
        )
        self.assertEqual(proc.returncode, 0, f"stderr: {proc.stderr}")
        data = json.loads(proc.stdout)
        svcs = data["services"]
        self.assertEqual(len(svcs), 2)
        # Alphabetical sorting
        self.assertEqual(svcs[0]["service"], "alpha")
        self.assertEqual(svcs[1]["service"], "zebra")

        a = svcs[0]
        self.assertEqual(a["total_requests"], 5)
        self.assertEqual(a["successful_requests"], 2)  # 200, 301
        self.assertEqual(a["client_error_requests"], 1)  # 404
        self.assertEqual(a["server_error_requests"], 1)  # 500
        # (1 + 1) / 5 = 0.4
        self.assertAlmostEqual(a["error_rate"], 0.4000, places=3)
        self.assertEqual(a["min_latency_ms"], 10)
        self.assertEqual(a["max_latency_ms"], 50)
        # avg = (10+20+30+40+50)/5 = 30.0
        self.assertAlmostEqual(a["avg_latency_ms"], 30.0, places=2)
        self.assertEqual(a["total_bytes_sent"], 150)

    def test_sla_breach_detection(self):
        # Service with 10% errors and avg latency 200ms
        records = [
            {
                "timestamp": 1710000000 + i,
                "service": "svc",
                "client_id": "c1",
                "method": "GET",
                "path": "/test",
                "status": 500 if i == 0 else 200,
                "latency_ms": 200,
                "bytes_sent": 100,
            }
            for i in range(10)
        ]
        stdin_data = "\n".join(json.dumps(r) for r in records) + "\n"

        # Default: error_rate_threshold=0.05, latency=500. Error rate is 0.10 > 0.05 -> breach
        proc1 = testkit.run(args=(), stdin=stdin_data)
        d1 = json.loads(proc1.stdout)
        self.assertTrue(d1["services"][0]["sla_breach"])

        # With error_rate_threshold=0.15: 0.10 <= 0.15, and 200 <= 500 -> no breach
        proc2 = testkit.run(
            args=["--error-rate-threshold", "0.15"], stdin=stdin_data
        )
        d2 = json.loads(proc2.stdout)
        self.assertFalse(d2["services"][0]["sla_breach"])

        # With sla_latency_ms=150: avg latency 200 > 150 -> breach
        proc3 = testkit.run(
            args=[
                "--error-rate-threshold",
                "0.15",
                "--sla-latency-ms",
                "150",
            ],
            stdin=stdin_data,
        )
        d3 = json.loads(proc3.stdout)
        self.assertTrue(d3["services"][0]["sla_breach"])

    def test_top_clients_ranking_and_tiebreaking(self):
        # 12 clients.
        # client-a: 5 requests
        # client-b: 5 requests
        # client-c through client-l: varying requests
        records = []
        # client-01 to client-12
        counts = {
            "client-z": 2,
            "client-y": 3,
            "client-x": 4,
            "client-w": 5,
            "client-v": 6,
            "client-u": 7,
            "client-t": 8,
            "client-s": 9,
            "client-r": 10,
            "client-q": 11,
            "client-b": 12,
            "client-a": 12,  # tie with b on count 12
        }
        for cid, cnt in counts.items():
            for i in range(cnt):
                records.append(
                    {
                        "timestamp": 1710000000 + i,
                        "service": "svc",
                        "client_id": cid,
                        "method": "GET",
                        "path": "/p",
                        "status": 500 if i == 0 else 200,
                        "latency_ms": 10,
                        "bytes_sent": 50,
                    }
                )

        proc = testkit.run(
            args=(), stdin="\n".join(json.dumps(r) for r in records) + "\n"
        )
        self.assertEqual(proc.returncode, 0, f"stderr: {proc.stderr}")
        top = json.loads(proc.stdout)["top_clients"]
        # Must return at most 10 clients
        self.assertEqual(len(top), 10)
        # client-a and client-b tie at 12 requests; client-a comes first alphabetically
        self.assertEqual(top[0]["client_id"], "client-a")
        self.assertEqual(top[0]["request_count"], 12)
        self.assertEqual(top[0]["error_count"], 1)
        self.assertEqual(top[0]["total_bytes"], 12 * 50)

        self.assertEqual(top[1]["client_id"], "client-b")
        self.assertEqual(top[1]["request_count"], 12)

        # 3rd should be client-q (11)
        self.assertEqual(top[2]["client_id"], "client-q")
        self.assertEqual(top[2]["request_count"], 11)

        # 10th client should have count 4 (client-x)
        self.assertEqual(top[9]["client_id"], "client-x")
        self.assertEqual(top[9]["request_count"], 4)

    def test_tumbling_windows(self):
        # Window size 60s
        records = [
            # bucket 1710000000: ts 1710000010, 1710000050
            {
                "timestamp": 1710000010,
                "service": "s",
                "client_id": "c",
                "method": "GET",
                "path": "/1",
                "status": 200,
                "latency_ms": 20,
                "bytes_sent": 10,
            },
            {
                "timestamp": 1710000050,
                "service": "s",
                "client_id": "c",
                "method": "GET",
                "path": "/2",
                "status": 400,
                "latency_ms": 40,
                "bytes_sent": 20,
            },
            # bucket 1710000120: ts 1710000130 (bucket 1710000060 is skipped because empty)
            {
                "timestamp": 1710000130,
                "service": "s",
                "client_id": "c",
                "method": "GET",
                "path": "/3",
                "status": 200,
                "latency_ms": 100,
                "bytes_sent": 30,
            },
        ]
        proc = testkit.run(
            args=["--window-seconds", "60"],
            stdin="\n".join(json.dumps(r) for r in records) + "\n",
        )
        self.assertEqual(proc.returncode, 0, f"stderr: {proc.stderr}")
        windows = json.loads(proc.stdout)["windows"]
        self.assertEqual(len(windows), 2)
        w0 = windows[0]
        self.assertEqual(w0["window_start"], 1710000000)
        self.assertEqual(w0["window_end"], 1710000060)
        self.assertEqual(w0["total_requests"], 2)
        self.assertEqual(w0["error_requests"], 1)
        self.assertAlmostEqual(w0["avg_latency_ms"], 30.0, places=2)

        w1 = windows[1]
        self.assertEqual(w1["window_start"], 1710000120)
        self.assertEqual(w1["window_end"], 1710000180)
        self.assertEqual(w1["total_requests"], 1)
        self.assertEqual(w1["error_requests"], 0)
        self.assertAlmostEqual(w1["avg_latency_ms"], 100.0, places=2)

    def test_invalid_records_handling(self):
        lines = [
            "not a json object at all",
            '["json", "array", "not", "dict"]',
            json.dumps({"timestamp": -1}),  # negative timestamp
            json.dumps(
                {
                    "timestamp": 1710000000,
                    "service": "",
                    "client_id": "c",
                    "method": "GET",
                    "path": "/p",
                    "status": 200,
                    "latency_ms": 1,
                    "bytes_sent": 1,
                }
            ),  # empty service
            json.dumps(
                {
                    "timestamp": 1710000000,
                    "service": "s",
                    "client_id": "c",
                    "method": "FOO",
                    "path": "/p",
                    "status": 200,
                    "latency_ms": 1,
                    "bytes_sent": 1,
                }
            ),  # invalid method
            json.dumps(
                {
                    "timestamp": 1710000000,
                    "service": "s",
                    "client_id": "c",
                    "method": "GET",
                    "path": "no_slash",
                    "status": 200,
                    "latency_ms": 1,
                    "bytes_sent": 1,
                }
            ),  # invalid path
            json.dumps(
                {
                    "timestamp": 1710000000,
                    "service": "s",
                    "client_id": "c",
                    "method": "GET",
                    "path": "/p",
                    "status": 99,
                    "latency_ms": 1,
                    "bytes_sent": 1,
                }
            ),  # status < 100
            json.dumps(
                {
                    "timestamp": 1710000000,
                    "service": "s",
                    "client_id": "c",
                    "method": "GET",
                    "path": "/p",
                    "status": 600,
                    "latency_ms": 1,
                    "bytes_sent": 1,
                }
            ),  # status > 599
            json.dumps(
                {
                    "timestamp": 1710000000,
                    "service": "s",
                    "client_id": "c",
                    "method": "GET",
                    "path": "/p",
                    "status": 200,
                    "latency_ms": -5,
                    "bytes_sent": 1,
                }
            ),  # negative latency
            json.dumps(
                {
                    "timestamp": 1710000000,
                    "service": "s",
                    "client_id": "c",
                    "method": "GET",
                    "path": "/p",
                    "status": 200,
                    "latency_ms": 1,
                    "bytes_sent": -1,
                }
            ),  # negative bytes
            json.dumps(
                {
                    "timestamp": True,
                    "service": "s",
                    "client_id": "c",
                    "method": "GET",
                    "path": "/p",
                    "status": 200,
                    "latency_ms": 1,
                    "bytes_sent": 1,
                }
            ),  # bool instead of int
            json.dumps(
                {
                    "timestamp": 1710000000.5,
                    "service": "s",
                    "client_id": "c",
                    "method": "GET",
                    "path": "/p",
                    "status": 200,
                    "latency_ms": 1,
                    "bytes_sent": 1,
                }
            ),  # fractional float instead of whole number
            json.dumps(
                {
                    "timestamp": 2**54,
                    "service": "s",
                    "client_id": "c",
                    "method": "GET",
                    "path": "/p",
                    "status": 200,
                    "latency_ms": 1,
                    "bytes_sent": 1,
                }
            ),  # integer > 2^53 - 1
            # One valid record
            json.dumps(
                {
                    "timestamp": 1710000000,
                    "service": "s",
                    "client_id": "c",
                    "method": "GET",
                    "path": "/p",
                    "status": 200,
                    "latency_ms": 10,
                    "bytes_sent": 50,
                }
            ),
        ]
        proc = testkit.run(args=(), stdin="\n".join(lines) + "\n")
        self.assertEqual(proc.returncode, 0, f"stderr: {proc.stderr}")
        data = json.loads(proc.stdout)
        self.assertEqual(data["summary"]["total_records"], 14)
        self.assertEqual(data["summary"]["valid_records"], 1)
        self.assertEqual(data["summary"]["invalid_records"], 13)
        self.assertEqual(len(data["services"]), 1)
        self.assertEqual(data["services"][0]["total_requests"], 1)

    def test_blank_lines_ignored(self):
        rec = json.dumps(
            {
                "timestamp": 1710000000,
                "service": "s",
                "client_id": "c",
                "method": "GET",
                "path": "/ok",
                "status": 200,
                "latency_ms": 10,
                "bytes_sent": 20,
            }
        )
        content = f"\n   \n\t\n{rec}\n\n   \n"
        proc = testkit.run(args=(), stdin=content)
        self.assertEqual(proc.returncode, 0, f"stderr: {proc.stderr}")
        data = json.loads(proc.stdout)
        self.assertEqual(data["summary"]["total_records"], 1)
        self.assertEqual(data["summary"]["valid_records"], 1)
        self.assertEqual(data["summary"]["invalid_records"], 0)

    def test_empty_input(self):
        proc = testkit.run(args=(), stdin="")
        self.assertEqual(proc.returncode, 0, f"stderr: {proc.stderr}")
        data = json.loads(proc.stdout)
        summary = data["summary"]
        self.assertEqual(summary["total_records"], 0)
        self.assertEqual(summary["valid_records"], 0)
        self.assertEqual(summary["invalid_records"], 0)
        self.assertIsNone(summary["earliest_timestamp"])
        self.assertIsNone(summary["latest_timestamp"])
        self.assertEqual(summary["total_bytes_sent"], 0)
        self.assertEqual(summary["total_services"], 0)
        self.assertEqual(summary["services_with_sla_breach"], 0)
        self.assertEqual(data["services"], [])
        self.assertEqual(data["top_clients"], [])
        self.assertEqual(data["windows"], [])

    def test_cli_error_missing_flag_value(self):
        proc = testkit.run(args=["--input"])
        self.assertEqual(proc.returncode, 1)
        self.assertTrue(len(proc.stderr.strip()) > 0)

    def test_cli_error_unknown_option(self):
        proc = testkit.run(args=["--non-existent-option", "foo"])
        self.assertEqual(proc.returncode, 1)
        self.assertTrue(len(proc.stderr.strip()) > 0)

    def test_cli_error_invalid_thresholds(self):
        # Negative latency
        proc1 = testkit.run(args=["--sla-latency-ms", "-10"])
        self.assertEqual(proc1.returncode, 1)
        self.assertTrue(len(proc1.stderr.strip()) > 0)

        # Error rate > 1.0
        proc2 = testkit.run(args=["--error-rate-threshold", "1.5"])
        self.assertEqual(proc2.returncode, 1)
        self.assertTrue(len(proc2.stderr.strip()) > 0)

        # Window seconds <= 0
        proc3 = testkit.run(args=["--window-seconds", "0"])
        self.assertEqual(proc3.returncode, 1)
        self.assertTrue(len(proc3.stderr.strip()) > 0)

    def test_input_file_not_found(self):
        proc = testkit.run(
            args=["--input", "/tmp/definitely_nonexistent_file_4219.jsonl"]
        )
        self.assertEqual(proc.returncode, 2)
        self.assertTrue(len(proc.stderr.strip()) > 0)

    def test_input_path_is_directory(self):
        with tempfile.TemporaryDirectory() as td:
            proc = testkit.run(args=["--input", td])
            self.assertEqual(proc.returncode, 2)
            self.assertTrue(len(proc.stderr.strip()) > 0)

    def test_whole_number_floats_accepted(self):
        record_str = '{"timestamp": 1710000000.0, "service": "svc", "client_id": "c1", "method": "GET", "path": "/p", "status": 200.0, "latency_ms": 15.0, "bytes_sent": 100.0}\n'
        proc = testkit.run(args=(), stdin=record_str)
        self.assertEqual(proc.returncode, 0, f"stderr: {proc.stderr}")
        data = json.loads(proc.stdout)
        self.assertEqual(data["summary"]["valid_records"], 1)
        self.assertEqual(data["summary"]["invalid_records"], 0)
        self.assertEqual(data["services"][0]["total_requests"], 1)
        self.assertEqual(data["services"][0]["successful_requests"], 1)

    def test_output_file_cannot_be_opened(self):
        # Target a path inside a nonexistent directory
        proc = testkit.run(
            args=[
                "--output",
                "/nonexistent_dir_99812/sub/output.json",
            ],
            stdin="{}",
        )
        self.assertEqual(proc.returncode, 1)
        self.assertTrue(len(proc.stderr.strip()) > 0)


if __name__ == "__main__":
    unittest.main()
