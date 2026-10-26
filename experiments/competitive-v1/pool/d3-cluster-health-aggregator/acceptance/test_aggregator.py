import http.server
import json
import os
import tempfile
import threading
import time
import unittest
import testkit

class MockClusterNodeServer:
    def __init__(self, routes=None):
        self.port = testkit.free_port()
        self.routes = routes or {}
        self.received = []
        owner = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                owner.received.append({"method": "GET", "path": self.path})
                status, body = owner.routes.get(self.path, (404, {"error": "not found"}))
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(json.dumps(body).encode())

            def do_POST(self):
                clen = int(self.headers.get("Content-Length", 0))
                req_body = self.rfile.read(clen)
                owner.received.append({"method": "POST", "path": self.path, "body": req_body})
                status, body = owner.routes.get(self.path, (404, {"error": "not found"}))
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(json.dumps(body).encode())

            def log_message(self, format, *args):
                pass

        self.server = http.server.HTTPServer(("127.0.0.1", self.port), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever)
        self.thread.daemon = True
        self.thread.start()

    def stop(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=2)


class TestClusterHealthAggregator(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()
        self.cluster_path = os.path.join(self.tmpdir.name, "cluster.json")
        self.status_path = os.path.join(self.tmpdir.name, "status.json")

    def tearDown(self):
        self.tmpdir.cleanup()

    def test_primary_healthy_active(self):
        srv = MockClusterNodeServer({
            "/primary/health": (200, {"status": "healthy"}),
            "/standby/health": (200, {"status": "healthy"}),
            "/standby/activate": (200, {"ok": True})
        })
        try:
            with open(self.cluster_path, "w") as f:
                json.dump({
                    "services": [
                        {
                            "name": "svc-a",
                            "primary_url": f"http://127.0.0.1:{srv.port}/primary/health",
                            "standby_url": f"http://127.0.0.1:{srv.port}/standby/health",
                            "activate_url": f"http://127.0.0.1:{srv.port}/standby/activate"
                        }
                    ]
                }, f)
            res = testkit.run([self.cluster_path, self.status_path])
            self.assertEqual(res.returncode, 0)
            with open(self.status_path) as f:
                data = json.load(f)
            self.assertEqual(data["total_services"], 1)
            self.assertEqual(data["healthy"], 1)
            self.assertEqual(data["failed_over"], 0)
            self.assertEqual(data["degraded"], 0)
            self.assertEqual(data["services"][0]["state"], "primary_active")
            self.assertEqual(data["services"][0]["primary_health"], "healthy")
            self.assertEqual(data["services"][0]["standby_health"], "not_checked")
            self.assertFalse(data["services"][0]["activated"])
            # Standby should not have been polled
            paths = [r["path"] for r in srv.received]
            self.assertNotIn("/standby/health", paths)
            self.assertNotIn("/standby/activate", paths)
        finally:
            srv.stop()

    def test_primary_fail_standby_activated(self):
        srv = MockClusterNodeServer({
            "/primary/health": (500, {"status": "error"}),
            "/standby/health": (200, {"status": "healthy"}),
            "/standby/activate": (200, {"activated": True})
        })
        try:
            with open(self.cluster_path, "w") as f:
                json.dump({
                    "services": [
                        {
                            "name": "svc-failover",
                            "primary_url": f"http://127.0.0.1:{srv.port}/primary/health",
                            "standby_url": f"http://127.0.0.1:{srv.port}/standby/health",
                            "activate_url": f"http://127.0.0.1:{srv.port}/standby/activate"
                        }
                    ]
                }, f)
            res = testkit.run([self.cluster_path, self.status_path])
            self.assertEqual(res.returncode, 0)
            with open(self.status_path) as f:
                data = json.load(f)
            self.assertEqual(data["healthy"], 0)
            self.assertEqual(data["failed_over"], 1)
            self.assertEqual(data["degraded"], 0)
            self.assertEqual(data["services"][0]["state"], "failed_over_to_standby")
            self.assertEqual(data["services"][0]["primary_health"], "unhealthy")
            self.assertEqual(data["services"][0]["standby_health"], "healthy")
            self.assertTrue(data["services"][0]["activated"])
        finally:
            srv.stop()

    def test_primary_down_standby_down_degraded(self):
        srv = MockClusterNodeServer({
            "/primary/health": (503, {"error": "unavailable"}),
            "/standby/health": (503, {"error": "standby unavailable"})
        })
        try:
            with open(self.cluster_path, "w") as f:
                json.dump({
                    "services": [
                        {
                            "name": "svc-all-down",
                            "primary_url": f"http://127.0.0.1:{srv.port}/primary/health",
                            "standby_url": f"http://127.0.0.1:{srv.port}/standby/health",
                            "activate_url": f"http://127.0.0.1:{srv.port}/standby/activate"
                        }
                    ]
                }, f)
            res = testkit.run([self.cluster_path, self.status_path])
            self.assertEqual(res.returncode, 1)
            with open(self.status_path) as f:
                data = json.load(f)
            self.assertEqual(data["degraded"], 1)
            self.assertEqual(data["services"][0]["state"], "degraded")
            self.assertFalse(data["services"][0]["activated"])
        finally:
            srv.stop()

    def test_standby_activate_failed_degraded(self):
        srv = MockClusterNodeServer({
            "/primary/health": (500, {}),
            "/standby/health": (200, {"status": "healthy"}),
            "/standby/activate": (500, {"error": "activation exception"})
        })
        try:
            with open(self.cluster_path, "w") as f:
                json.dump({
                    "services": [
                        {
                            "name": "svc-activate-fail",
                            "primary_url": f"http://127.0.0.1:{srv.port}/primary/health",
                            "standby_url": f"http://127.0.0.1:{srv.port}/standby/health",
                            "activate_url": f"http://127.0.0.1:{srv.port}/standby/activate"
                        }
                    ]
                }, f)
            res = testkit.run([self.cluster_path, self.status_path])
            self.assertEqual(res.returncode, 1)
            with open(self.status_path) as f:
                data = json.load(f)
            self.assertEqual(data["degraded"], 1)
            self.assertEqual(data["services"][0]["state"], "degraded")
        finally:
            srv.stop()

    def test_primary_connection_refused_failover(self):
        free_p = testkit.free_port()
        srv = MockClusterNodeServer({
            "/standby/health": (200, {"status": "healthy"}),
            "/standby/activate": (204, {})
        })
        try:
            with open(self.cluster_path, "w") as f:
                json.dump({
                    "services": [
                        {
                            "name": "svc-conn-refused",
                            "primary_url": f"http://127.0.0.1:{free_p}/primary/health",
                            "standby_url": f"http://127.0.0.1:{srv.port}/standby/health",
                            "activate_url": f"http://127.0.0.1:{srv.port}/standby/activate"
                        }
                    ]
                }, f)
            res = testkit.run([self.cluster_path, self.status_path])
            self.assertEqual(res.returncode, 0)
            with open(self.status_path) as f:
                data = json.load(f)
            self.assertEqual(data["failed_over"], 1)
            self.assertEqual(data["services"][0]["state"], "failed_over_to_standby")
        finally:
            srv.stop()

    def test_offline_fixtures_mode(self):
        fix_path = os.path.join(self.tmpdir.name, "fix.json")
        with open(fix_path, "w") as f:
            json.dump({
                "http://127.0.0.1:9001/health": {"status": 200, "body": {"status": "healthy"}}
            }, f)
        with open(self.cluster_path, "w") as f:
            json.dump({
                "services": [
                    {
                        "name": "svc-offline",
                        "primary_url": "http://127.0.0.1:9001/health",
                        "standby_url": "http://127.0.0.1:9002/health",
                        "activate_url": "http://127.0.0.1:9002/activate"
                    }
                ]
            }, f)
        res = testkit.run([self.cluster_path, self.status_path, "--offline-fixtures", fix_path])
        self.assertEqual(res.returncode, 0)
        with open(self.status_path) as f:
            data = json.load(f)
        self.assertEqual(data["healthy"], 1)

    def test_missing_args(self):
        res = testkit.run([])
        self.assertEqual(res.returncode, 2)

    def test_malformed_json(self):
        bad_path = os.path.join(self.tmpdir.name, "bad.json")
        with open(bad_path, "w") as f:
            f.write("invalid json")
        res = testkit.run([bad_path, self.status_path])
        self.assertEqual(res.returncode, 2)

    def test_missing_service_field(self):
        with open(self.cluster_path, "w") as f:
            json.dump({"services": [{"name": "s1", "primary_url": "http://127.0.0.1/1"}]}, f)
        res = testkit.run([self.cluster_path, self.status_path])
        self.assertEqual(res.returncode, 2)

    def test_multiple_services_mixed(self):
        srv = MockClusterNodeServer({
            "/p1": (200, {"status": "healthy"}),
            "/p2": (500, {}),
            "/s2": (200, {"status": "healthy"}),
            "/a2": (200, {"activated": True})
        })
        try:
            with open(self.cluster_path, "w") as f:
                json.dump({
                    "services": [
                        {
                            "name": "s1",
                            "primary_url": f"http://127.0.0.1:{srv.port}/p1",
                            "standby_url": f"http://127.0.0.1:{srv.port}/s1",
                            "activate_url": f"http://127.0.0.1:{srv.port}/a1"
                        },
                        {
                            "name": "s2",
                            "primary_url": f"http://127.0.0.1:{srv.port}/p2",
                            "standby_url": f"http://127.0.0.1:{srv.port}/s2",
                            "activate_url": f"http://127.0.0.1:{srv.port}/a2"
                        }
                    ]
                }, f)
            res = testkit.run([self.cluster_path, self.status_path])
            self.assertEqual(res.returncode, 0)
            with open(self.status_path) as f:
                data = json.load(f)
            self.assertEqual(data["total_services"], 2)
            self.assertEqual(data["healthy"], 1)
            self.assertEqual(data["failed_over"], 1)
            self.assertEqual(data["degraded"], 0)
        finally:
            srv.stop()

    def test_empty_service_name_rejected(self):
        with open(self.cluster_path, "w") as f:
            json.dump({
                "services": [
                    {
                        "name": "",
                        "primary_url": "http://127.0.0.1/p",
                        "standby_url": "http://127.0.0.1/s",
                        "activate_url": "http://127.0.0.1/a"
                    }
                ]
            }, f)
        res = testkit.run([self.cluster_path, self.status_path])
        self.assertEqual(res.returncode, 2)
