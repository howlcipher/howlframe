import json
import os
import sys
import tempfile
import time
import unittest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../kit")))
import testkit


class TestRateLimiter(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.store_path = os.path.join(self.temp_dir.name, "store.json")

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_register_key_success_201(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, headers, body = s.request("POST", "/keys", {
                "key": "key-std",
                "tier": "standard",
                "scopes": ["read", "write"]
            })
            self.assertEqual(status, 201)
            resp = json.loads(body.decode())
            self.assertEqual(resp["key"], "key-std")
            self.assertEqual(resp["tier"], "standard")
            self.assertEqual(resp["tokens"], 20.0)

    def test_register_invalid_tier_400(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, body = s.request("POST", "/keys", {
                "key": "key-bad",
                "tier": "mega_tier",
                "scopes": ["read"]
            })
            self.assertEqual(status, 400)
            resp = json.loads(body.decode())
            self.assertEqual(resp["error"], "invalid_tier")

    def test_check_invalid_key_401(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, body = s.request("POST", "/check", {
                "key": "key-unknown",
                "scope": "read"
            })
            self.assertEqual(status, 401)
            resp = json.loads(body.decode())
            self.assertFalse(resp["allowed"])
            self.assertEqual(resp["reason"], "invalid_key")

    def test_check_insufficient_scope_403(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/keys", {"key": "key-ro", "tier": "standard", "scopes": ["read"]})
            status, _, body = s.request("POST", "/check", {"key": "key-ro", "scope": "write"})
            self.assertEqual(status, 403)
            resp = json.loads(body.decode())
            self.assertFalse(resp["allowed"])
            self.assertEqual(resp["reason"], "insufficient_scope")

    def test_check_allowed_consumes_tokens_200(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/keys", {"key": "key-std", "tier": "standard", "scopes": ["read"]})
            status, _, body = s.request("POST", "/check", {"key": "key-std", "scope": "read", "cost": 5})
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertTrue(resp["allowed"])
            self.assertAlmostEqual(resp["remaining_tokens"], 15.0, delta=0.5)

    def test_check_rate_limit_exceeded_429(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            # Starter has 5 tokens
            s.request("POST", "/keys", {"key": "key-starter", "tier": "starter", "scopes": ["read"]})
            s.request("POST", "/check", {"key": "key-starter", "scope": "read", "cost": 5})
            status, _, body = s.request("POST", "/check", {"key": "key-starter", "scope": "read", "cost": 1})
            self.assertEqual(status, 429)
            resp = json.loads(body.decode())
            self.assertFalse(resp["allowed"])
            self.assertEqual(resp["reason"], "rate_limited")

    def test_get_key_status_200(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/keys", {"key": "key-ent", "tier": "enterprise", "scopes": ["admin"]})
            status, _, body = s.request("GET", "/keys/key-ent")
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertEqual(resp["key"], "key-ent")
            self.assertEqual(resp["tokens"], 100.0)

    def test_get_nonexistent_key_404(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, body = s.request("GET", "/keys/missing")
            self.assertEqual(status, 404)

    def test_reset_single_key(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/keys", {"key": "key-1", "tier": "starter", "scopes": ["read"]})
            s.request("POST", "/check", {"key": "key-1", "scope": "read", "cost": 4})
            status, _, _ = s.request("POST", "/reset", {"key": "key-1"})
            self.assertEqual(status, 200)
            _, _, body = s.request("GET", "/keys/key-1")
            resp = json.loads(body.decode())
            self.assertAlmostEqual(resp["tokens"], 5.0, delta=0.2)

    def test_reset_all_keys(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/keys", {"key": "k1", "tier": "starter", "scopes": ["read"]})
            s.request("POST", "/keys", {"key": "k2", "tier": "starter", "scopes": ["read"]})
            s.request("POST", "/check", {"key": "k1", "scope": "read", "cost": 4})
            s.request("POST", "/check", {"key": "k2", "scope": "read", "cost": 4})
            status, _, _ = s.request("POST", "/reset", {})
            self.assertEqual(status, 200)
            _, _, b1 = s.request("GET", "/keys/k1")
            _, _, b2 = s.request("GET", "/keys/k2")
            self.assertAlmostEqual(json.loads(b1.decode())["tokens"], 5.0, delta=0.2)
            self.assertAlmostEqual(json.loads(b2.decode())["tokens"], 5.0, delta=0.2)

    def test_persistence_across_restarts(self):
        # Write key in server 1
        with testkit.Server(args=["--store", self.store_path]) as s1:
            s1.request("POST", "/keys", {"key": "key-persist", "tier": "standard", "scopes": ["read"]})

        # Check key in server 2
        with testkit.Server(args=["--store", self.store_path]) as s2:
            status, _, body = s2.request("GET", "/keys/key-persist")
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertEqual(resp["key"], "key-persist")
            self.assertEqual(resp["tier"], "standard")

    def test_batch_replay_mode(self):
        reqs_path = os.path.join(self.temp_dir.name, "reqs.jsonl")
        with open(reqs_path, "w") as f:
            f.write(json.dumps({"method": "POST", "path": "/keys", "body": {"key": "kr", "tier": "starter", "scopes": ["read"]}}) + "\n")
            f.write(json.dumps({"method": "POST", "path": "/check", "body": {"key": "kr", "scope": "read", "cost": 1}}) + "\n")
            f.write(json.dumps({"method": "GET", "path": "/keys/kr"}) + "\n")

        res = testkit.run(["replay", "--store", self.store_path, reqs_path])
        self.assertEqual(res.returncode, 0)
        lines = [json.loads(l) for l in res.stdout.strip().split("\n") if l.strip()]
        self.assertEqual(len(lines), 3)
        self.assertEqual(lines[0]["status"], 201)
        self.assertEqual(lines[1]["status"], 200)
        self.assertTrue(lines[1]["body"]["allowed"])
        self.assertEqual(lines[2]["status"], 200)


if __name__ == "__main__":
    unittest.main()
