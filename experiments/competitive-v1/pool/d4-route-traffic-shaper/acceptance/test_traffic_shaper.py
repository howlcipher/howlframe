import hashlib
import json
import os
import tempfile
import unittest
import testkit

class TestRouteTrafficShaper(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()
        self.tbl_path = os.path.join(self.tmpdir.name, "routing_table.json")
        self.req_path = os.path.join(self.tmpdir.name, "requests.json")
        self.log_path = os.path.join(self.tmpdir.name, "dispatch_log.json")

        self.default_table = {
            "backends_health": {
                "core-v1": True,
                "core-v2": True,
                "core-beta": True,
                "pay-broken": False,
                "pay-fallback": True
            },
            "routes": [
                {
                    "path_prefix": "/users",
                    "headers_match": {"X-Beta": "1"},
                    "target": "core-beta"
                },
                {
                    "path_prefix": "/users",
                    "target": "core-v1",
                    "canary": {
                        "target": "core-v2",
                        "weight_pct": 50
                    }
                },
                {
                    "path_prefix": "/pay",
                    "target": "pay-broken",
                    "fallback_target": "pay-fallback"
                }
            ]
        }
        with open(self.tbl_path, "w") as f:
            json.dump(self.default_table, f)

    def tearDown(self):
        self.tmpdir.cleanup()

    def test_header_match_precedence_case_insensitive(self):
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "r-hdr",
                        "path": "/users/list",
                        "headers": {"x-beta": "1"} # lowercase header key
                    }
                ]
            }, f)
        res = testkit.run([self.tbl_path, self.req_path, self.log_path])
        self.assertEqual(res.returncode, 0)
        with open(self.log_path) as f:
            data = json.load(f)
        self.assertEqual(data["routed_count"], 1)
        self.assertEqual(data["dispatches"][0]["selected_target"], "core-beta")
        self.assertFalse(data["dispatches"][0]["canary_routed"])
        self.assertFalse(data["dispatches"][0]["fallback_triggered"])

    def test_canary_deterministic_allocation(self):
        # We need two user IDs: one whose hash % 100 < 50, and one whose hash % 100 >= 50
        # Let's find them:
        uid_canary = None
        uid_primary = None
        for i in range(100):
            uid = f"user-{i}"
            h = int(hashlib.sha256(uid.encode("utf-8")).hexdigest()[:8], 16) % 100
            if h < 50 and uid_canary is None:
                uid_canary = uid
            elif h >= 50 and uid_primary is None:
                uid_primary = uid
            if uid_canary and uid_primary:
                break

        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {"id": "r-canary", "path": "/users/profile", "user_id": uid_canary},
                    {"id": "r-primary", "path": "/users/profile", "user_id": uid_primary}
                ]
            }, f)

        res = testkit.run([self.tbl_path, self.req_path, self.log_path])
        self.assertEqual(res.returncode, 0)
        with open(self.log_path) as f:
            data = json.load(f)

        self.assertEqual(data["dispatches"][0]["selected_target"], "core-v2")
        self.assertTrue(data["dispatches"][0]["canary_routed"])

        self.assertEqual(data["dispatches"][1]["selected_target"], "core-v1")
        self.assertFalse(data["dispatches"][1]["canary_routed"])

    def test_canary_client_ip_fallback_when_no_user_id(self):
        # Test client_ip used for hashing when user_id is absent
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {"id": "r-ip", "path": "/users/info", "client_ip": "10.0.0.1"}
                ]
            }, f)
        res = testkit.run([self.tbl_path, self.req_path, self.log_path])
        self.assertEqual(res.returncode, 0)
        with open(self.log_path) as f:
            data = json.load(f)
        self.assertIn(data["dispatches"][0]["selected_target"], ["core-v1", "core-v2"])

    def test_fallback_triggered_when_target_unhealthy(self):
        # /pay matches pay-broken which has health false -> should fallback to pay-fallback
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {"id": "r-pay", "path": "/pay/checkout"}
                ]
            }, f)
        res = testkit.run([self.tbl_path, self.req_path, self.log_path])
        self.assertEqual(res.returncode, 0)
        with open(self.log_path) as f:
            data = json.load(f)
        self.assertEqual(data["dispatches"][0]["selected_target"], "pay-fallback")
        self.assertTrue(data["dispatches"][0]["fallback_triggered"])
        self.assertEqual(data["dispatches"][0]["status"], "routed")

    def test_service_unavailable_503_when_no_healthy_backend(self):
        # Fallback target is also unhealthy
        table = {
            "backends_health": {"down1": False, "down2": False},
            "routes": [
                {"path_prefix": "/fail", "target": "down1", "fallback_target": "down2"}
            ]
        }
        with open(self.tbl_path, "w") as f:
            json.dump(table, f)

        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {"id": "r-503", "path": "/fail/test"}
                ]
            }, f)
        res = testkit.run([self.tbl_path, self.req_path, self.log_path])
        self.assertEqual(res.returncode, 1)
        with open(self.log_path) as f:
            data = json.load(f)
        self.assertEqual(data["failed_count"], 1)
        self.assertEqual(data["dispatches"][0]["status"], "503_service_unavailable")
        self.assertIsNone(data["dispatches"][0]["selected_target"])

    def test_unmatched_route_404_not_found(self):
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {"id": "r-404", "path": "/unmatched/path"}
                ]
            }, f)
        res = testkit.run([self.tbl_path, self.req_path, self.log_path])
        self.assertEqual(res.returncode, 1)
        with open(self.log_path) as f:
            data = json.load(f)
        self.assertEqual(data["failed_count"], 1)
        self.assertEqual(data["dispatches"][0]["status"], "404_not_found")
        self.assertIsNone(data["dispatches"][0]["selected_target"])

    def test_missing_args(self):
        res = testkit.run([])
        self.assertEqual(res.returncode, 2)

    def test_malformed_json(self):
        bad_path = os.path.join(self.tmpdir.name, "bad.json")
        with open(bad_path, "w") as f:
            f.write("invalid")
        res = testkit.run([self.tbl_path, bad_path, self.log_path])
        self.assertEqual(res.returncode, 2)

    def test_missing_request_path_field(self):
        with open(self.req_path, "w") as f:
            json.dump({"requests": [{"id": "r1"}]}, f)
        res = testkit.run([self.tbl_path, self.req_path, self.log_path])
        self.assertEqual(res.returncode, 2)

    def test_canary_weight_zero_never_routes_canary(self):
        table = {
            "routes": [
                {
                    "path_prefix": "/zero",
                    "target": "target-stable",
                    "canary": {"target": "target-canary", "weight_pct": 0}
                }
            ]
        }
        with open(self.tbl_path, "w") as f:
            json.dump(table, f)
        with open(self.req_path, "w") as f:
            json.dump({"requests": [{"id": "r0", "path": "/zero/test", "user_id": "u1"}]}, f)
        res = testkit.run([self.tbl_path, self.req_path, self.log_path])
        self.assertEqual(res.returncode, 0)
        with open(self.log_path) as f:
            data = json.load(f)
        self.assertEqual(data["dispatches"][0]["selected_target"], "target-stable")
        self.assertFalse(data["dispatches"][0]["canary_routed"])

    def test_multiple_requests_mixed_outcomes(self):
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {"id": "r1", "path": "/users/abc"},
                    {"id": "r2", "path": "/pay/checkout"},
                    {"id": "r3", "path": "/not/found"}
                ]
            }, f)
        res = testkit.run([self.tbl_path, self.req_path, self.log_path])
        self.assertEqual(res.returncode, 1)
        with open(self.log_path) as f:
            data = json.load(f)
        self.assertEqual(data["total_requests"], 3)
        self.assertEqual(data["routed_count"], 2)
        self.assertEqual(data["failed_count"], 1)
