import hashlib
import hmac
import http.server
import json
import os
import socket
import tempfile
import threading
import time
import unittest
import testkit

class MockWebhookServer:
    def __init__(self, handler_fn=None):
        self.received = []
        self.handler_fn = handler_fn
        self.port = testkit.free_port()

        owner = self

        class RequestHandler(http.server.BaseHTTPRequestHandler):
            def do_POST(self):
                content_len = int(self.headers.get("Content-Length", 0))
                body = self.rfile.read(content_len)
                # Store lower-cased headers for robust case-insensitive lookup
                headers = {k.lower(): v for k, v in self.headers.items()}
                owner.received.append({"path": self.path, "headers": headers, "body": body})

                if owner.handler_fn:
                    status, resp_body = owner.handler_fn(self.path, headers, body, len(owner.received))
                else:
                    status, resp_body = 200, b'{"ok": true}'

                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(resp_body)

            def log_message(self, format, *args):
                pass # suppress stderr logs

        self.server = http.server.HTTPServer(("127.0.0.1", self.port), RequestHandler)
        self.thread = threading.Thread(target=self.server.serve_forever)
        self.thread.daemon = True
        self.thread.start()

    def stop(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=2)


class TestWebhookDispatcher(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.tmpdir.cleanup()

    def test_single_event_successful_delivery(self):
        server = MockWebhookServer()
        try:
            evt_path = os.path.join(self.tmpdir.name, "events.json")
            rep_path = os.path.join(self.tmpdir.name, "report.json")
            secret = "test-secret-123"
            with open(evt_path, "w") as f:
                json.dump({
                    "events": [
                        {"id": "ev-1", "topic": "test.topic", "payload": {"foo": "bar"}}
                    ]
                }, f)

            res = testkit.run([
                evt_path, rep_path,
                "--endpoint", f"http://127.0.0.1:{server.port}/events",
                "--secret", secret
            ])
            self.assertEqual(res.returncode, 0)
            with open(rep_path) as f:
                rep = json.load(f)
            self.assertEqual(rep["total_events"], 1)
            self.assertEqual(rep["delivered"], 1)
            self.assertEqual(rep["failed"], 0)
            self.assertEqual(rep["deliveries"][0]["id"], "ev-1")
            self.assertEqual(rep["deliveries"][0]["status"], "delivered")
            self.assertEqual(rep["deliveries"][0]["attempts"], 1)
            self.assertEqual(rep["deliveries"][0]["last_status_code"], 200)

            # Check server received request
            self.assertEqual(len(server.received), 1)
            rec = server.received[0]
            self.assertEqual(rec["headers"].get("x-event-id"), "ev-1")
            self.assertEqual(rec["headers"].get("x-event-topic"), "test.topic")
            sig = rec["headers"].get("x-signature")
            self.assertTrue(sig.startswith("sha256="))
            expected_hash = hmac.new(secret.encode(), rec["body"], hashlib.sha256).hexdigest()
            self.assertEqual(sig, f"sha256={expected_hash}")
        finally:
            server.stop()

    def test_multiple_events_sequence(self):
        server = MockWebhookServer()
        try:
            evt_path = os.path.join(self.tmpdir.name, "events.json")
            rep_path = os.path.join(self.tmpdir.name, "report.json")
            with open(evt_path, "w") as f:
                json.dump({
                    "events": [
                        {"id": "ev-1", "topic": "t1", "payload": 1},
                        {"id": "ev-2", "topic": "t2", "payload": 2}
                    ]
                }, f)
            res = testkit.run([
                evt_path, rep_path,
                "--endpoint", f"http://127.0.0.1:{server.port}/events"
            ])
            self.assertEqual(res.returncode, 0)
            with open(rep_path) as f:
                rep = json.load(f)
            self.assertEqual(rep["delivered"], 2)
            self.assertEqual(len(server.received), 2)
        finally:
            server.stop()

    def test_client_error_400_no_retry(self):
        server = MockWebhookServer(lambda p, h, b, count: (400, b'{"error":"bad_request"}'))
        try:
            evt_path = os.path.join(self.tmpdir.name, "events.json")
            rep_path = os.path.join(self.tmpdir.name, "report.json")
            with open(evt_path, "w") as f:
                json.dump({
                    "events": [
                        {"id": "ev-err", "topic": "t", "payload": {}, "max_attempts": 3}
                    ]
                }, f)
            res = testkit.run([
                evt_path, rep_path,
                "--endpoint", f"http://127.0.0.1:{server.port}/events"
            ])
            self.assertEqual(res.returncode, 1)
            with open(rep_path) as f:
                rep = json.load(f)
            self.assertEqual(rep["failed"], 1)
            self.assertEqual(rep["deliveries"][0]["attempts"], 1) # no retry on 4xx
            self.assertEqual(rep["deliveries"][0]["last_status_code"], 400)
        finally:
            server.stop()

    def test_retry_on_500_eventual_success(self):
        # Fail first call with 500, then succeed with 200
        server = MockWebhookServer(lambda p, h, b, count: (500, b'{"error":"server_error"}') if count == 1 else (200, b'{"ok":true}'))
        try:
            evt_path = os.path.join(self.tmpdir.name, "events.json")
            rep_path = os.path.join(self.tmpdir.name, "report.json")
            with open(evt_path, "w") as f:
                json.dump({
                    "events": [
                        {"id": "ev-retry", "topic": "t", "payload": {}, "max_attempts": 3}
                    ]
                }, f)
            res = testkit.run([
                evt_path, rep_path,
                "--endpoint", f"http://127.0.0.1:{server.port}/events"
            ])
            self.assertEqual(res.returncode, 0)
            with open(rep_path) as f:
                rep = json.load(f)
            self.assertEqual(rep["delivered"], 1)
            self.assertEqual(rep["deliveries"][0]["attempts"], 2)
            self.assertEqual(rep["deliveries"][0]["last_status_code"], 200)
        finally:
            server.stop()

    def test_retry_exhaustion_500(self):
        server = MockWebhookServer(lambda p, h, b, count: (503, b'{"error":"unavailable"}'))
        try:
            evt_path = os.path.join(self.tmpdir.name, "events.json")
            rep_path = os.path.join(self.tmpdir.name, "report.json")
            with open(evt_path, "w") as f:
                json.dump({
                    "events": [
                        {"id": "ev-fail", "topic": "t", "payload": {}, "max_attempts": 3}
                    ]
                }, f)
            res = testkit.run([
                evt_path, rep_path,
                "--endpoint", f"http://127.0.0.1:{server.port}/events"
            ])
            self.assertEqual(res.returncode, 1)
            with open(rep_path) as f:
                rep = json.load(f)
            self.assertEqual(rep["failed"], 1)
            self.assertEqual(rep["deliveries"][0]["attempts"], 3)
            self.assertEqual(rep["deliveries"][0]["last_status_code"], 503)
        finally:
            server.stop()

    def test_offline_mode_success(self):
        evt_path = os.path.join(self.tmpdir.name, "events.json")
        rep_path = os.path.join(self.tmpdir.name, "report.json")
        with open(evt_path, "w") as f:
            json.dump({
                "events": [
                    {"id": "ev-off", "topic": "t", "payload": {"x": 10}}
                ]
            }, f)
        res = testkit.run([evt_path, rep_path, "--offline-status", "200"])
        self.assertEqual(res.returncode, 0)
        with open(rep_path) as f:
            rep = json.load(f)
        self.assertEqual(rep["delivered"], 1)
        self.assertEqual(rep["deliveries"][0]["last_status_code"], 200)

    def test_offline_mode_failure(self):
        evt_path = os.path.join(self.tmpdir.name, "events.json")
        rep_path = os.path.join(self.tmpdir.name, "report.json")
        with open(evt_path, "w") as f:
            json.dump({
                "events": [
                    {"id": "ev-off", "topic": "t", "payload": {"x": 10}, "max_attempts": 2}
                ]
            }, f)
        res = testkit.run([evt_path, rep_path, "--offline-status", "500"])
        self.assertEqual(res.returncode, 1)
        with open(rep_path) as f:
            rep = json.load(f)
        self.assertEqual(rep["failed"], 1)
        self.assertEqual(rep["deliveries"][0]["attempts"], 2)

    def test_missing_args(self):
        res = testkit.run([])
        self.assertEqual(res.returncode, 2)

    def test_malformed_json(self):
        evt_path = os.path.join(self.tmpdir.name, "bad.json")
        rep_path = os.path.join(self.tmpdir.name, "report.json")
        with open(evt_path, "w") as f:
            f.write("{invalid")
        res = testkit.run([evt_path, rep_path])
        self.assertEqual(res.returncode, 2)

    def test_missing_event_topic(self):
        evt_path = os.path.join(self.tmpdir.name, "missing.json")
        rep_path = os.path.join(self.tmpdir.name, "report.json")
        with open(evt_path, "w") as f:
            json.dump({"events": [{"id": "ev-1", "payload": {}}]}, f)
        res = testkit.run([evt_path, rep_path])
        self.assertEqual(res.returncode, 2)

    def test_connection_refused(self):
        free_p = testkit.free_port()
        evt_path = os.path.join(self.tmpdir.name, "events.json")
        rep_path = os.path.join(self.tmpdir.name, "report.json")
        with open(evt_path, "w") as f:
            json.dump({
                "events": [
                    {"id": "ev-conn", "topic": "t", "payload": {}, "max_attempts": 2, "timeout_ms": 500}
                ]
            }, f)
        res = testkit.run([
            evt_path, rep_path,
            "--endpoint", f"http://127.0.0.1:{free_p}/nowhere"
        ])
        self.assertEqual(res.returncode, 1)
        with open(rep_path) as f:
            rep = json.load(f)
        self.assertEqual(rep["failed"], 1)
        self.assertEqual(rep["deliveries"][0]["attempts"], 2)
