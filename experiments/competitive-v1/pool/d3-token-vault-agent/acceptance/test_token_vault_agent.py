import http.server
import json
import os
import tempfile
import threading
import time
import unittest
import testkit

class MockVaultAndTargetServer:
    def __init__(self):
        self.port = testkit.free_port()
        self.vault_requests = []
        self.target_requests = []
        self.target_response_status = 200
        self.target_response_body = b'{"success": true}'

        owner = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def do_POST(self):
                clen = int(self.headers.get("Content-Length", 0))
                body = self.rfile.read(clen)
                headers = {k.lower(): v for k, v in self.headers.items()}
                parsed = json.loads(body) if body else {}

                if self.path == "/v1/token/mint":
                    owner.vault_requests.append({"path": self.path, "headers": headers, "body": parsed})
                    token = f"bearer-tok-{parsed.get('subject')}-{parsed.get('scope')}"
                    self.send_response(200)
                    self.send_header("Content-Type", "application/json")
                    self.end_headers()
                    self.wfile.write(json.dumps({"token": token, "expires_in": 60}).encode())
                else:
                    owner.target_requests.append({"method": "POST", "path": self.path, "headers": headers, "body": parsed})
                    self.send_response(owner.target_response_status)
                    self.send_header("Content-Type", "application/json")
                    self.end_headers()
                    self.wfile.write(owner.target_response_body)

            def do_GET(self):
                headers = {k.lower(): v for k, v in self.headers.items()}
                owner.target_requests.append({"method": "GET", "path": self.path, "headers": headers})
                self.send_response(owner.target_response_status)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(owner.target_response_body)

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


class TestTokenVaultAgent(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()
        self.grants_path = os.path.join(self.tmpdir.name, "grants.json")
        self.requests_path = os.path.join(self.tmpdir.name, "requests.json")
        self.audit_path = os.path.join(self.tmpdir.name, "audit.json")

        self.default_grants = {
            "subjects": {
                "svc-app": {
                    "allowed_scopes": ["read:data", "write:data"],
                    "max_token_ttl_sec": 300
                }
            }
        }
        with open(self.grants_path, "w") as f:
            json.dump(self.default_grants, f)

    def tearDown(self):
        self.tmpdir.cleanup()

    def test_authorized_dispatch_and_credential_sanitization(self):
        srv = MockVaultAndTargetServer()
        try:
            with open(self.requests_path, "w") as f:
                json.dump({
                    "requests": [
                        {
                            "id": "r-ok",
                            "subject": "svc-app",
                            "requested_scope": "read:data",
                            "target_url": f"http://127.0.0.1:{srv.port}/api/endpoint",
                            "method": "GET",
                            "headers": {
                                "Authorization": "untrusted-leak",
                                "Proxy-Authorization": "proxy-leak",
                                "X-Tracking": "track-123"
                            }
                        }
                    ]
                }, f)
            res = testkit.run([
                self.grants_path, self.requests_path, self.audit_path,
                "--vault-url", f"http://127.0.0.1:{srv.port}"
            ])
            self.assertEqual(res.returncode, 0)
            with open(self.audit_path) as f:
                audit = json.load(f)
            self.assertEqual(audit["total_requests"], 1)
            self.assertEqual(audit["authorized_dispatched"], 1)
            self.assertEqual(audit["denied"], 0)
            self.assertEqual(audit["dispatches"][0]["decision"], "authorized")
            self.assertTrue(audit["dispatches"][0]["token_issued"].startswith("bearer-tok-"))
            self.assertEqual(audit["dispatches"][0]["target_status_code"], 200)

            # Check target received request with sanitized auth
            self.assertEqual(len(srv.target_requests), 1)
            tgt_req = srv.target_requests[0]
            auth_header = tgt_req["headers"].get("authorization")
            self.assertEqual(auth_header, f"Bearer {audit['dispatches'][0]['token_issued']}")
            self.assertNotIn("proxy-authorization", tgt_req["headers"])
            self.assertEqual(tgt_req["headers"].get("x-tracking"), "track-123")
        finally:
            srv.stop()

    def test_deny_unauthorized_subject(self):
        with open(self.requests_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "r-bad-subj",
                        "subject": "unknown-subject",
                        "requested_scope": "read:data",
                        "target_url": "http://127.0.0.1:8000/nowhere"
                    }
                ]
            }, f)
        res = testkit.run([self.grants_path, self.requests_path, self.audit_path])
        self.assertEqual(res.returncode, 0)
        with open(self.audit_path) as f:
            audit = json.load(f)
        self.assertEqual(audit["denied"], 1)
        self.assertEqual(audit["dispatches"][0]["decision"], "denied")
        self.assertEqual(audit["dispatches"][0]["reason"], "unauthorized_subject")

    def test_deny_scope_not_permitted(self):
        with open(self.requests_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "r-bad-scope",
                        "subject": "svc-app",
                        "requested_scope": "admin:super_delete",
                        "target_url": "http://127.0.0.1:8000/nowhere"
                    }
                ]
            }, f)
        res = testkit.run([self.grants_path, self.requests_path, self.audit_path])
        self.assertEqual(res.returncode, 0)
        with open(self.audit_path) as f:
            audit = json.load(f)
        self.assertEqual(audit["denied"], 1)
        self.assertEqual(audit["dispatches"][0]["decision"], "denied")
        self.assertEqual(audit["dispatches"][0]["reason"], "scope_not_permitted")

    def test_downstream_500_causes_exit_1(self):
        srv = MockVaultAndTargetServer()
        srv.target_response_status = 502
        try:
            with open(self.requests_path, "w") as f:
                json.dump({
                    "requests": [
                        {
                            "id": "r-500",
                            "subject": "svc-app",
                            "requested_scope": "read:data",
                            "target_url": f"http://127.0.0.1:{srv.port}/api/endpoint"
                        }
                    ]
                }, f)
            res = testkit.run([
                self.grants_path, self.requests_path, self.audit_path,
                "--vault-url", f"http://127.0.0.1:{srv.port}"
            ])
            self.assertEqual(res.returncode, 1)
            with open(self.audit_path) as f:
                audit = json.load(f)
            self.assertEqual(audit["dispatches"][0]["target_status_code"], 502)
        finally:
            srv.stop()

    def test_downstream_network_error_causes_exit_1(self):
        srv = MockVaultAndTargetServer()
        free_p = testkit.free_port()
        try:
            with open(self.requests_path, "w") as f:
                json.dump({
                    "requests": [
                        {
                            "id": "r-net-err",
                            "subject": "svc-app",
                            "requested_scope": "read:data",
                            "target_url": f"http://127.0.0.1:{free_p}/nowhere",
                            "timeout_ms": 500
                        }
                    ]
                }, f)
            res = testkit.run([
                self.grants_path, self.requests_path, self.audit_path,
                "--vault-url", f"http://127.0.0.1:{srv.port}"
            ])
            self.assertEqual(res.returncode, 1)
        finally:
            srv.stop()

    def test_offline_vault_mode(self):
        with open(self.requests_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "r-offline",
                        "subject": "svc-app",
                        "requested_scope": "write:data",
                        "target_url": "http://127.0.0.1:9999/dummy"
                    }
                ]
            }, f)
        res = testkit.run([self.grants_path, self.requests_path, self.audit_path, "--offline-vault"])
        self.assertEqual(res.returncode, 0)
        with open(self.audit_path) as f:
            audit = json.load(f)
        self.assertEqual(audit["authorized_dispatched"], 1)
        self.assertIn("vault-mock-svc-app-write:data", audit["dispatches"][0]["token_issued"])
        self.assertEqual(audit["dispatches"][0]["target_status_code"], 200)

    def test_multiple_mixed_requests(self):
        with open(self.requests_path, "w") as f:
            json.dump({
                "requests": [
                    {"id": "r1", "subject": "svc-app", "requested_scope": "read:data", "target_url": "http://127.0.0.1/1"},
                    {"id": "r2", "subject": "bad-subj", "requested_scope": "read:data", "target_url": "http://127.0.0.1/2"},
                    {"id": "r3", "subject": "svc-app", "requested_scope": "forbidden", "target_url": "http://127.0.0.1/3"}
                ]
            }, f)
        res = testkit.run([self.grants_path, self.requests_path, self.audit_path, "--offline-vault"])
        self.assertEqual(res.returncode, 0)
        with open(self.audit_path) as f:
            audit = json.load(f)
        self.assertEqual(audit["total_requests"], 3)
        self.assertEqual(audit["authorized_dispatched"], 1)
        self.assertEqual(audit["denied"], 2)

    def test_missing_args(self):
        res = testkit.run([])
        self.assertEqual(res.returncode, 2)

    def test_malformed_json(self):
        bad_path = os.path.join(self.tmpdir.name, "bad.json")
        with open(bad_path, "w") as f:
            f.write("bad")
        res = testkit.run([self.grants_path, bad_path, self.audit_path])
        self.assertEqual(res.returncode, 2)

    def test_missing_required_request_fields(self):
        with open(self.requests_path, "w") as f:
            json.dump({"requests": [{"id": "r-missing", "subject": "svc-app"}]}, f)
        res = testkit.run([self.grants_path, self.requests_path, self.audit_path])
        self.assertEqual(res.returncode, 2)
