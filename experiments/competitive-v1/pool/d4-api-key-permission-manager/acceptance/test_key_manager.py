import json
import os
import tempfile
import unittest
import testkit

class TestApiKeyPermissionManager(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()
        self.ks_path = os.path.join(self.tmpdir.name, "keystore.json")
        self.req_path = os.path.join(self.tmpdir.name, "requests.json")
        self.aud_path = os.path.join(self.tmpdir.name, "audit.json")
        self.now = 1700000000

        self.default_keystore = {
            "keys": {
                "key-valid": {
                    "user_id": "usr-1",
                    "status": "active",
                    "scopes": ["orders:read", "orders:write"],
                    "allowed_ips": ["192.168.1.10", "10.0.0.0/24"]
                },
                "key-wildcard": {
                    "user_id": "usr-admin",
                    "status": "active",
                    "scopes": ["*"],
                    "allowed_ips": []
                },
                "key-revoked": {
                    "user_id": "usr-3",
                    "status": "revoked"
                },
                "key-rotated": {
                    "user_id": "usr-4",
                    "status": "active",
                    "scopes": ["orders:read"],
                    "rotation": {
                        "new_key": "key-new",
                        "grace_until_epoch": 1700050000
                    }
                }
            }
        }
        with open(self.ks_path, "w") as f:
            json.dump(self.default_keystore, f)

    def tearDown(self):
        self.tmpdir.cleanup()

    def test_valid_request_with_scope_and_ip(self):
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "c1",
                        "key": "key-valid",
                        "client_ip": "192.168.1.10",
                        "required_scope": "orders:read"
                    }
                ]
            }, f)
        res = testkit.run([self.ks_path, self.req_path, self.aud_path, "--now-epoch", str(self.now)])
        self.assertEqual(res.returncode, 0)
        with open(self.aud_path) as f:
            data = json.load(f)
        self.assertEqual(data["valid_count"], 1)
        self.assertEqual(data["evaluations"][0]["decision"], "VALID")
        self.assertEqual(data["evaluations"][0]["user_id"], "usr-1")
        self.assertEqual(data["evaluations"][0]["scope_granted"], "orders:read")
        self.assertIsNone(data["evaluations"][0]["rotation_notice"])

    def test_wildcard_scope_and_empty_allowed_ips(self):
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "c-wild",
                        "key": "key-wildcard",
                        "client_ip": "203.0.113.199",
                        "required_scope": "any:scope:here"
                    }
                ]
            }, f)
        res = testkit.run([self.ks_path, self.req_path, self.aud_path, "--now-epoch", str(self.now)])
        self.assertEqual(res.returncode, 0)
        with open(self.aud_path) as f:
            data = json.load(f)
        self.assertEqual(data["evaluations"][0]["decision"], "VALID")

    def test_cidr_ip_match(self):
        # 10.0.0.55 matches 10.0.0.0/24
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "c-cidr",
                        "key": "key-valid",
                        "client_ip": "10.0.0.55"
                    }
                ]
            }, f)
        res = testkit.run([self.ks_path, self.req_path, self.aud_path, "--now-epoch", str(self.now)])
        self.assertEqual(res.returncode, 0)
        with open(self.aud_path) as f:
            data = json.load(f)
        self.assertEqual(data["evaluations"][0]["decision"], "VALID")

    def test_ip_forbidden_denied(self):
        # 172.16.0.1 not in allowed_ips
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "c-bad-ip",
                        "key": "key-valid",
                        "client_ip": "172.16.0.1"
                    }
                ]
            }, f)
        res = testkit.run([self.ks_path, self.req_path, self.aud_path, "--now-epoch", str(self.now)])
        self.assertEqual(res.returncode, 1)
        with open(self.aud_path) as f:
            data = json.load(f)
        self.assertEqual(data["evaluations"][0]["decision"], "DENIED")
        self.assertEqual(data["evaluations"][0]["reason"], "ip_forbidden")

    def test_insufficient_scope_denied(self):
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "c-scope-fail",
                        "key": "key-valid",
                        "client_ip": "192.168.1.10",
                        "required_scope": "billing:admin"
                    }
                ]
            }, f)
        res = testkit.run([self.ks_path, self.req_path, self.aud_path, "--now-epoch", str(self.now)])
        self.assertEqual(res.returncode, 1)
        with open(self.aud_path) as f:
            data = json.load(f)
        self.assertEqual(data["evaluations"][0]["decision"], "DENIED")
        self.assertEqual(data["evaluations"][0]["reason"], "insufficient_scope")

    def test_rotation_grace_period_active(self):
        # grace_until is 1700050000; now is 1700000000 -> within grace period!
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "c-rot-active",
                        "key": "key-rotated",
                        "client_ip": "127.0.0.1",
                        "required_scope": "orders:read"
                    }
                ]
            }, f)
        res = testkit.run([self.ks_path, self.req_path, self.aud_path, "--now-epoch", str(self.now)])
        self.assertEqual(res.returncode, 0)
        with open(self.aud_path) as f:
            data = json.load(f)
        self.assertEqual(data["evaluations"][0]["decision"], "VALID")
        self.assertEqual(data["evaluations"][0]["rotation_notice"], "key_rotated_use_new_key")

    def test_rotation_grace_period_expired(self):
        # now is 1700060000 (> 1700050000) -> expired!
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "c-rot-exp",
                        "key": "key-rotated",
                        "client_ip": "127.0.0.1"
                    }
                ]
            }, f)
        res = testkit.run([self.ks_path, self.req_path, self.aud_path, "--now-epoch", "1700060000"])
        self.assertEqual(res.returncode, 1)
        with open(self.aud_path) as f:
            data = json.load(f)
        self.assertEqual(data["evaluations"][0]["decision"], "DENIED")
        self.assertEqual(data["evaluations"][0]["reason"], "key_expired_after_rotation")

    def test_revoked_key_denied(self):
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {"id": "c-rev", "key": "key-revoked", "client_ip": "127.0.0.1"}
                ]
            }, f)
        res = testkit.run([self.ks_path, self.req_path, self.aud_path, "--now-epoch", str(self.now)])
        self.assertEqual(res.returncode, 1)
        with open(self.aud_path) as f:
            data = json.load(f)
        self.assertEqual(data["evaluations"][0]["decision"], "DENIED")
        self.assertEqual(data["evaluations"][0]["reason"], "key_revoked")

    def test_key_not_found(self):
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {"id": "c-unknown", "key": "non-existent-key", "client_ip": "127.0.0.1"}
                ]
            }, f)
        res = testkit.run([self.ks_path, self.req_path, self.aud_path, "--now-epoch", str(self.now)])
        self.assertEqual(res.returncode, 1)
        with open(self.aud_path) as f:
            data = json.load(f)
        self.assertEqual(data["evaluations"][0]["decision"], "DENIED")
        self.assertEqual(data["evaluations"][0]["reason"], "key_not_found")

    def test_missing_args(self):
        res = testkit.run([])
        self.assertEqual(res.returncode, 2)

    def test_malformed_json(self):
        bad_path = os.path.join(self.tmpdir.name, "bad.json")
        with open(bad_path, "w") as f:
            f.write("bad json")
        res = testkit.run([self.ks_path, bad_path, self.aud_path])
        self.assertEqual(res.returncode, 2)

    def test_missing_client_ip_request_field(self):
        with open(self.req_path, "w") as f:
            json.dump({"requests": [{"id": "c1", "key": "key-valid"}]}, f)
        res = testkit.run([self.ks_path, self.req_path, self.aud_path])
        self.assertEqual(res.returncode, 2)
