import json
import os
import tempfile
import unittest
import testkit

class TestParallelMatrix(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()
        self.matrix_path = os.path.join(self.tmpdir.name, "matrix.json")
        self.results_path = os.path.join(self.tmpdir.name, "results.json")

    def tearDown(self):
        self.tmpdir.cleanup()

    def test_all_cases_pass_sequential(self):
        with open(self.matrix_path, "w") as f:
            json.dump({
                "concurrency": 1,
                "cases": [
                    {"id": "c1", "command": ["/usr/bin/coreutils", "echo", "pass1"]},
                    {"id": "c2", "command": ["/usr/bin/coreutils", "echo", "pass2"]}
                ]
            }, f)
        res = testkit.run([self.matrix_path, self.results_path])
        self.assertEqual(res.returncode, 0)
        with open(self.results_path) as f:
            data = json.load(f)
        self.assertEqual(data["total"], 2)
        self.assertEqual(data["passed"], 2)
        self.assertEqual(data["failed"], 0)
        self.assertEqual(data["skipped"], 0)
        self.assertEqual(data["cases"][0]["id"], "c1")
        self.assertEqual(data["cases"][0]["status"], "passed")
        self.assertEqual(data["cases"][0]["exit_code"], 0)
        self.assertIn("pass1", data["cases"][0]["stdout"])

    def test_concurrent_execution(self):
        with open(self.matrix_path, "w") as f:
            json.dump({
                "concurrency": 4,
                "cases": [
                    {"id": f"t-{i}", "command": ["/usr/bin/coreutils", "echo", f"msg-{i}"]}
                    for i in range(8)
                ]
            }, f)
        res = testkit.run([self.matrix_path, self.results_path])
        self.assertEqual(res.returncode, 0)
        with open(self.results_path) as f:
            data = json.load(f)
        self.assertEqual(data["total"], 8)
        self.assertEqual(data["passed"], 8)
        # Check order is preserved
        self.assertEqual([c["id"] for c in data["cases"]], [f"t-{i}" for i in range(8)])

    def test_environment_variables_merged(self):
        with open(self.matrix_path, "w") as f:
            json.dump({
                "cases": [
                    {
                        "id": "c-env",
                        "command": ["/usr/bin/dash", "-c", "echo \"$CUSTOM_ENV_KEY\""],
                        "env": {"CUSTOM_ENV_KEY": "hello-custom-val"}
                    }
                ]
            }, f)
        res = testkit.run([self.matrix_path, self.results_path])
        self.assertEqual(res.returncode, 0)
        with open(self.results_path) as f:
            data = json.load(f)
        self.assertEqual(data["cases"][0]["stdout"].strip(), "hello-custom-val")

    def test_failure_exit_code(self):
        with open(self.matrix_path, "w") as f:
            json.dump({
                "cases": [
                    {"id": "pass", "command": ["/usr/bin/coreutils", "echo", "ok"]},
                    {"id": "fail", "command": ["/usr/bin/dash", "-c", "exit 9"]}
                ]
            }, f)
        res = testkit.run([self.matrix_path, self.results_path])
        self.assertEqual(res.returncode, 1)
        with open(self.results_path) as f:
            data = json.load(f)
        self.assertEqual(data["passed"], 1)
        self.assertEqual(data["failed"], 1)
        self.assertEqual(data["cases"][1]["status"], "failed")
        self.assertEqual(data["cases"][1]["exit_code"], 9)

    def test_timeout_recording(self):
        with open(self.matrix_path, "w") as f:
            json.dump({
                "cases": [
                    {"id": "slow", "command": ["/usr/bin/coreutils", "sleep", "5"], "timeout_ms": 300}
                ]
            }, f)
        res = testkit.run([self.matrix_path, self.results_path], timeout=10)
        self.assertEqual(res.returncode, 1)
        with open(self.results_path) as f:
            data = json.load(f)
        self.assertEqual(data["failed"], 1)
        self.assertEqual(data["cases"][0]["status"], "timed_out")
        self.assertIsNone(data["cases"][0]["exit_code"])

    def test_fail_fast_cancellation(self):
        with open(self.matrix_path, "w") as f:
            json.dump({
                "concurrency": 1,
                "fail_fast": True,
                "cases": [
                    {"id": "c1", "command": ["/usr/bin/dash", "-c", "exit 1"]},
                    {"id": "c2", "command": ["/usr/bin/coreutils", "echo", "should_not_run"]},
                    {"id": "c3", "command": ["/usr/bin/coreutils", "echo", "should_not_run_either"]}
                ]
            }, f)
        res = testkit.run([self.matrix_path, self.results_path])
        self.assertEqual(res.returncode, 1)
        with open(self.results_path) as f:
            data = json.load(f)
        self.assertEqual(data["failed"], 1)
        self.assertEqual(data["skipped"], 2)
        self.assertEqual(data["cases"][1]["status"], "skipped")
        self.assertEqual(data["cases"][1]["reason"], "fail_fast_triggered")
        self.assertEqual(data["cases"][2]["status"], "skipped")
        self.assertEqual(data["cases"][2]["reason"], "fail_fast_triggered")

    def test_cli_concurrency_override(self):
        with open(self.matrix_path, "w") as f:
            json.dump({
                "concurrency": 1,
                "cases": [
                    {"id": "c1", "command": ["/usr/bin/coreutils", "echo", "1"]},
                    {"id": "c2", "command": ["/usr/bin/coreutils", "echo", "2"]}
                ]
            }, f)
        res = testkit.run([self.matrix_path, self.results_path, "--concurrency", "2"])
        self.assertEqual(res.returncode, 0)
        with open(self.results_path) as f:
            data = json.load(f)
        self.assertEqual(data["passed"], 2)

    def test_duplicate_case_id(self):
        with open(self.matrix_path, "w") as f:
            json.dump({
                "cases": [
                    {"id": "dup", "command": ["/usr/bin/coreutils", "echo", "1"]},
                    {"id": "dup", "command": ["/usr/bin/coreutils", "echo", "2"]}
                ]
            }, f)
        res = testkit.run([self.matrix_path, self.results_path])
        self.assertEqual(res.returncode, 2)

    def test_missing_args(self):
        res = testkit.run([])
        self.assertEqual(res.returncode, 2)

    def test_malformed_json(self):
        bad_path = os.path.join(self.tmpdir.name, "bad.json")
        with open(bad_path, "w") as f:
            f.write("invalid json")
        res = testkit.run([bad_path, self.results_path])
        self.assertEqual(res.returncode, 2)

    def test_missing_command_field(self):
        with open(self.matrix_path, "w") as f:
            json.dump({"cases": [{"id": "bad-case"}]}, f)
        res = testkit.run([self.matrix_path, self.results_path])
        self.assertEqual(res.returncode, 2)
