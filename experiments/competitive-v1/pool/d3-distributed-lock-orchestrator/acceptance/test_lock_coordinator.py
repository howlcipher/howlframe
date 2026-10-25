import http.server
import json
import os
import tempfile
import threading
import time
import unittest
import testkit

class MockLockServer:
    def __init__(self, handler_fn=None):
        self.received = []
        self.handler_fn = handler_fn
        self.port = testkit.free_port()
        self.locks = {} # name -> {"owner": str, "token": str, "expires": float}
        owner = self

        class RequestHandler(http.server.BaseHTTPRequestHandler):
            def do_POST(self):
                content_len = int(self.headers.get("Content-Length", 0))
                body = json.loads(self.rfile.read(content_len)) if content_len else {}
                owner.received.append({"path": self.path, "body": body})

                if owner.handler_fn:
                    status, resp = owner.handler_fn(self.path, body, len(owner.received))
                else:
                    status, resp = owner.default_handle(self.path, body)

                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(json.dumps(resp).encode())

            def log_message(self, format, *args):
                pass

        self.server = http.server.HTTPServer(("127.0.0.1", self.port), RequestHandler)
        self.thread = threading.Thread(target=self.server.serve_forever)
        self.thread.daemon = True
        self.thread.start()

    def default_handle(self, path, body):
        if path == "/lock/acquire":
            lname = body.get("lock_name")
            token = f"tok-{lname}-999"
            self.locks[lname] = {"owner": body.get("owner_id"), "token": token}
            return 200, {"acquired": True, "token": token}
        elif path == "/lock/renew":
            return 200, {"renewed": True}
        elif path == "/lock/release":
            lname = body.get("lock_name")
            self.locks.pop(lname, None)
            return 200, {"released": True}
        return 404, {"error": "not found"}

    def stop(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=2)


class TestLockCoordinator(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()
        self.job_path = os.path.join(self.tmpdir.name, "job.json")
        self.res_path = os.path.join(self.tmpdir.name, "result.json")

    def tearDown(self):
        self.tmpdir.cleanup()

    def test_successful_acquisition_and_execution(self):
        server = MockLockServer()
        try:
            with open(self.job_path, "w") as f:
                json.dump({
                    "lock_name": "res-1",
                    "owner_id": "worker-1",
                    "command": ["/usr/bin/coreutils", "echo", "done_work"],
                    "timeout_ms": 2000
                }, f)
            res = testkit.run([self.job_path, self.res_path, "--server", f"http://127.0.0.1:{server.port}"])
            self.assertEqual(res.returncode, 0)
            with open(self.res_path) as f:
                report = json.load(f)
            self.assertTrue(report["lock_acquired"])
            self.assertTrue(report["command_executed"])
            self.assertEqual(report["exit_code"], 0)
            self.assertIn("done_work", report["stdout"])
            self.assertTrue(report["lock_released"])

            # Verify endpoints called
            paths = [r["path"] for r in server.received]
            self.assertIn("/lock/acquire", paths)
            self.assertIn("/lock/release", paths)
        finally:
            server.stop()

    def test_heartbeat_renewal_during_execution(self):
        server = MockLockServer()
        try:
            # Run command that sleeps 600ms with renew_interval_ms 200ms
            cmd = ["/usr/bin/coreutils", "sleep", "0.6"]
            with open(self.job_path, "w") as f:
                json.dump({
                    "lock_name": "res-renew",
                    "owner_id": "worker-renew",
                    "renew_interval_ms": 200,
                    "command": cmd,
                    "timeout_ms": 3000
                }, f)
            res = testkit.run([self.job_path, self.res_path, "--server", f"http://127.0.0.1:{server.port}"])
            self.assertEqual(res.returncode, 0)
            with open(self.res_path) as f:
                report = json.load(f)
            self.assertGreaterEqual(report["renew_count"], 2)
            renew_calls = [r for r in server.received if r["path"] == "/lock/renew"]
            self.assertGreaterEqual(len(renew_calls), 2)
        finally:
            server.stop()

    def test_conflict_retry_eventual_success(self):
        # Return 409 twice, then 200
        count = 0
        def handler(path, body, req_num):
            nonlocal count
            if path == "/lock/acquire":
                count += 1
                if count <= 2:
                    return 409, {"acquired": False, "error": "conflict"}
                return 200, {"acquired": True, "token": "tok-eventual"}
            return 200, {"ok": True}

        server = MockLockServer(handler)
        try:
            with open(self.job_path, "w") as f:
                json.dump({
                    "lock_name": "res-conflict",
                    "owner_id": "worker-c",
                    "max_acquire_wait_ms": 1000,
                    "command": ["/usr/bin/coreutils", "echo", "after_conflict"]
                }, f)
            res = testkit.run([self.job_path, self.res_path, "--server", f"http://127.0.0.1:{server.port}"])
            self.assertEqual(res.returncode, 0)
            with open(self.res_path) as f:
                report = json.load(f)
            self.assertTrue(report["lock_acquired"])
            self.assertTrue(report["command_executed"])
        finally:
            server.stop()

    def test_acquire_timeout_abort(self):
        # Always return 409
        server = MockLockServer(lambda p, b, c: (409, {"acquired": False}) if p == "/lock/acquire" else (200, {}))
        try:
            with open(self.job_path, "w") as f:
                json.dump({
                    "lock_name": "res-timeout",
                    "owner_id": "worker-t",
                    "max_acquire_wait_ms": 300,
                    "command": ["/usr/bin/coreutils", "echo", "should_not_run"]
                }, f)
            res = testkit.run([self.job_path, self.res_path, "--server", f"http://127.0.0.1:{server.port}"])
            self.assertEqual(res.returncode, 1)
            with open(self.res_path) as f:
                report = json.load(f)
            self.assertFalse(report["lock_acquired"])
            self.assertFalse(report["command_executed"])
            self.assertFalse(report["lock_released"])
        finally:
            server.stop()

    def test_command_failure_still_releases_lock(self):
        server = MockLockServer()
        try:
            cmd = ["/usr/bin/dash", "-c", "exit 7"]
            with open(self.job_path, "w") as f:
                json.dump({
                    "lock_name": "res-fail",
                    "owner_id": "worker-f",
                    "command": cmd
                }, f)
            res = testkit.run([self.job_path, self.res_path, "--server", f"http://127.0.0.1:{server.port}"])
            self.assertEqual(res.returncode, 1)
            with open(self.res_path) as f:
                report = json.load(f)
            self.assertTrue(report["lock_acquired"])
            self.assertTrue(report["command_executed"])
            self.assertEqual(report["exit_code"], 7)
            self.assertTrue(report["lock_released"])
            release_calls = [r for r in server.received if r["path"] == "/lock/release"]
            self.assertEqual(len(release_calls), 1)
        finally:
            server.stop()

    def test_command_timeout_still_releases_lock(self):
        server = MockLockServer()
        try:
            cmd = ["/usr/bin/coreutils", "sleep", "5"]
            with open(self.job_path, "w") as f:
                json.dump({
                    "lock_name": "res-cmd-timeout",
                    "owner_id": "worker-ct",
                    "command": cmd,
                    "timeout_ms": 300
                }, f)
            res = testkit.run([self.job_path, self.res_path, "--server", f"http://127.0.0.1:{server.port}"], timeout=10)
            self.assertEqual(res.returncode, 1)
            with open(self.res_path) as f:
                report = json.load(f)
            self.assertTrue(report["lock_acquired"])
            self.assertTrue(report["command_executed"])
            self.assertIsNone(report["exit_code"])
            self.assertTrue(report["lock_released"])
        finally:
            server.stop()

    def test_offline_mock_mode_success(self):
        mock_path = os.path.join(self.tmpdir.name, "mock.json")
        with open(mock_path, "w") as f:
            json.dump({"acquire_status": 200, "token": "mock-tok-123"}, f)
        with open(self.job_path, "w") as f:
            json.dump({
                "lock_name": "res-mock",
                "owner_id": "worker-m",
                "command": ["/usr/bin/coreutils", "echo", "offline_ok"]
            }, f)
        res = testkit.run([self.job_path, self.res_path, "--offline-mock", mock_path])
        self.assertEqual(res.returncode, 0)
        with open(self.res_path) as f:
            report = json.load(f)
        self.assertTrue(report["lock_acquired"])
        self.assertEqual(report["token"], "mock-tok-123")
        self.assertTrue(report["command_executed"])
        self.assertTrue(report["lock_released"])

    def test_offline_mock_mode_conflict(self):
        mock_path = os.path.join(self.tmpdir.name, "mock.json")
        with open(mock_path, "w") as f:
            json.dump({"acquire_status": 409}, f)
        with open(self.job_path, "w") as f:
            json.dump({
                "lock_name": "res-mock-fail",
                "owner_id": "worker-mf",
                "max_acquire_wait_ms": 100,
                "command": ["/usr/bin/coreutils", "echo", "never"]
            }, f)
        res = testkit.run([self.job_path, self.res_path, "--offline-mock", mock_path])
        self.assertEqual(res.returncode, 1)
        with open(self.res_path) as f:
            report = json.load(f)
        self.assertFalse(report["lock_acquired"])
        self.assertFalse(report["command_executed"])

    def test_missing_args(self):
        res = testkit.run([])
        self.assertEqual(res.returncode, 2)

    def test_malformed_json(self):
        bad_path = os.path.join(self.tmpdir.name, "bad.json")
        with open(bad_path, "w") as f:
            f.write("{invalid")
        res = testkit.run([bad_path, self.res_path])
        self.assertEqual(res.returncode, 2)

    def test_missing_required_fields(self):
        with open(self.job_path, "w") as f:
            json.dump({"lock_name": "only_name"}, f) # missing owner_id, command
        res = testkit.run([self.job_path, self.res_path])
        self.assertEqual(res.returncode, 2)
