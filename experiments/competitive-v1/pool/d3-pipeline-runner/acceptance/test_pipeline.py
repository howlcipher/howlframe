import json
import os
import tempfile
import unittest
import testkit

class TestPipelineRunner(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.tmpdir.cleanup()

    def test_simple_success(self):
        pipe_path = os.path.join(self.tmpdir.name, "pipe.json")
        rep_path = os.path.join(self.tmpdir.name, "rep.json")
        with open(pipe_path, "w") as f:
            json.dump({
                "tasks": [
                    {"id": "t1", "command": ["/usr/bin/coreutils", "echo", "step 1"], "depends_on": []},
                    {"id": "t2", "command": ["/usr/bin/coreutils", "echo", "step 2"], "depends_on": ["t1"]}
                ]
            }, f)
        res = testkit.run([pipe_path, rep_path])
        self.assertEqual(res.returncode, 0)
        with open(rep_path) as f:
            rep = json.load(f)
        self.assertTrue(rep["success"])
        self.assertEqual(rep["total_tasks"], 2)
        self.assertEqual(rep["completed_tasks"], 2)
        self.assertEqual(rep["failed_tasks"], 0)
        self.assertEqual(rep["tasks"]["t1"]["status"], "success")
        self.assertEqual(rep["tasks"]["t1"]["exit_code"], 0)
        self.assertIn("step 1", rep["tasks"]["t1"]["stdout"])
        self.assertEqual(rep["tasks"]["t2"]["status"], "success")

    def test_missing_args(self):
        res = testkit.run([])
        self.assertEqual(res.returncode, 2)
        self.assertTrue(len(res.stderr) > 0)

    def test_malformed_json(self):
        pipe_path = os.path.join(self.tmpdir.name, "bad.json")
        rep_path = os.path.join(self.tmpdir.name, "rep.json")
        with open(pipe_path, "w") as f:
            f.write("{invalid json")
        res = testkit.run([pipe_path, rep_path])
        self.assertEqual(res.returncode, 2)
        self.assertFalse(os.path.exists(rep_path))

    def test_missing_required_fields(self):
        pipe_path = os.path.join(self.tmpdir.name, "missing.json")
        rep_path = os.path.join(self.tmpdir.name, "rep.json")
        with open(pipe_path, "w") as f:
            json.dump({"tasks": [{"id": "t1"}]}, f) # missing command
        res = testkit.run([pipe_path, rep_path])
        self.assertEqual(res.returncode, 2)

    def test_unknown_dependency(self):
        pipe_path = os.path.join(self.tmpdir.name, "unknown.json")
        rep_path = os.path.join(self.tmpdir.name, "rep.json")
        with open(pipe_path, "w") as f:
            json.dump({
                "tasks": [
                    {"id": "t1", "command": ["/usr/bin/coreutils", "echo", "1"], "depends_on": ["non_existent"]}
                ]
            }, f)
        res = testkit.run([pipe_path, rep_path])
        self.assertEqual(res.returncode, 2)

    def test_cycle_detection_direct(self):
        pipe_path = os.path.join(self.tmpdir.name, "cycle.json")
        rep_path = os.path.join(self.tmpdir.name, "rep.json")
        with open(pipe_path, "w") as f:
            json.dump({
                "tasks": [
                    {"id": "t1", "command": ["/usr/bin/coreutils", "echo", "1"], "depends_on": ["t1"]}
                ]
            }, f)
        res = testkit.run([pipe_path, rep_path])
        self.assertEqual(res.returncode, 2)

    def test_cycle_detection_indirect(self):
        pipe_path = os.path.join(self.tmpdir.name, "cycle3.json")
        rep_path = os.path.join(self.tmpdir.name, "rep.json")
        with open(pipe_path, "w") as f:
            json.dump({
                "tasks": [
                    {"id": "a", "command": ["/usr/bin/coreutils", "echo", "a"], "depends_on": ["c"]},
                    {"id": "b", "command": ["/usr/bin/coreutils", "echo", "b"], "depends_on": ["a"]},
                    {"id": "c", "command": ["/usr/bin/coreutils", "echo", "c"], "depends_on": ["b"]}
                ]
            }, f)
        res = testkit.run([pipe_path, rep_path])
        self.assertEqual(res.returncode, 2)

    def test_task_failure_and_skip(self):
        pipe_path = os.path.join(self.tmpdir.name, "fail.json")
        rep_path = os.path.join(self.tmpdir.name, "rep.json")
        with open(pipe_path, "w") as f:
            json.dump({
                "tasks": [
                    {"id": "failing", "command": ["/usr/bin/dash", "-c", "exit 7"], "max_retries": 0},
                    {"id": "downstream", "command": ["/usr/bin/coreutils", "echo", "never runs"], "depends_on": ["failing"]}
                ]
            }, f)
        res = testkit.run([pipe_path, rep_path])
        self.assertEqual(res.returncode, 1)
        with open(rep_path) as f:
            rep = json.load(f)
        self.assertFalse(rep["success"])
        self.assertEqual(rep["failed_tasks"], 1)
        self.assertEqual(rep["skipped_tasks"], 1)
        self.assertEqual(rep["tasks"]["failing"]["status"], "failed")
        self.assertEqual(rep["tasks"]["failing"]["exit_code"], 7)
        self.assertEqual(rep["tasks"]["downstream"]["status"], "skipped")
        self.assertEqual(rep["tasks"]["downstream"]["reason"], "dependency_failed")

    def test_task_timeout(self):
        pipe_path = os.path.join(self.tmpdir.name, "timeout.json")
        rep_path = os.path.join(self.tmpdir.name, "rep.json")
        with open(pipe_path, "w") as f:
            json.dump({
                "tasks": [
                    {"id": "slow", "command": ["/usr/bin/coreutils", "sleep", "10"], "timeout_ms": 300, "max_retries": 0}
                ]
            }, f)
        res = testkit.run([pipe_path, rep_path], timeout=15)
        self.assertEqual(res.returncode, 1)
        with open(rep_path) as f:
            rep = json.load(f)
        self.assertFalse(rep["success"])
        self.assertTrue(rep["tasks"]["slow"]["timed_out"])
        self.assertEqual(rep["tasks"]["slow"]["status"], "failed")

    def test_retry_success(self):
        marker = os.path.join(self.tmpdir.name, "attempt.marker")
        cmd = [
            "/usr/bin/dash", "-c",
            f'if [ -f "{marker}" ]; then exit 0; else : > "{marker}"; exit 1; fi'
        ]
        pipe_path = os.path.join(self.tmpdir.name, "retry.json")
        rep_path = os.path.join(self.tmpdir.name, "rep.json")
        with open(pipe_path, "w") as f:
            json.dump({
                "tasks": [
                    {"id": "flaky", "command": cmd, "max_retries": 2}
                ]
            }, f)
        res = testkit.run([pipe_path, rep_path])
        self.assertEqual(res.returncode, 0)
        with open(rep_path) as f:
            rep = json.load(f)
        self.assertTrue(rep["success"])
        self.assertEqual(rep["tasks"]["flaky"]["attempts"], 2)
        self.assertEqual(rep["tasks"]["flaky"]["status"], "success")

    def test_diamond_dag_execution(self):
        pipe_path = os.path.join(self.tmpdir.name, "diamond.json")
        rep_path = os.path.join(self.tmpdir.name, "rep.json")
        with open(pipe_path, "w") as f:
            json.dump({
                "tasks": [
                    {"id": "a", "command": ["/usr/bin/coreutils", "echo", "A"], "depends_on": []},
                    {"id": "b", "command": ["/usr/bin/coreutils", "echo", "B"], "depends_on": ["a"]},
                    {"id": "c", "command": ["/usr/bin/coreutils", "echo", "C"], "depends_on": ["a"]},
                    {"id": "d", "command": ["/usr/bin/coreutils", "echo", "D"], "depends_on": ["b", "c"]}
                ]
            }, f)
        res = testkit.run([pipe_path, rep_path])
        self.assertEqual(res.returncode, 0)
        with open(rep_path) as f:
            rep = json.load(f)
        self.assertTrue(rep["success"])
        self.assertEqual(rep["completed_tasks"], 4)

    def test_duplicate_task_id(self):
        pipe_path = os.path.join(self.tmpdir.name, "dup.json")
        rep_path = os.path.join(self.tmpdir.name, "rep.json")
        with open(pipe_path, "w") as f:
            json.dump({
                "tasks": [
                    {"id": "same", "command": ["/usr/bin/coreutils", "echo", "1"]},
                    {"id": "same", "command": ["/usr/bin/coreutils", "echo", "2"]}
                ]
            }, f)
        res = testkit.run([pipe_path, rep_path])
        self.assertEqual(res.returncode, 2)
