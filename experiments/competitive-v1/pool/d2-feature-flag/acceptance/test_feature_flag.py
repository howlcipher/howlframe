import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../kit")))
import testkit


class TestFeatureFlag(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.store_path = os.path.join(self.temp_dir.name, "store.json")

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_create_flag_201(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, body = s.request("POST", "/flags", {
                "key": "dark_mode",
                "enabled": True,
                "min_tier": "basic",
                "allowed_roles": ["user", "admin"],
                "user_overrides": []
            })
            self.assertEqual(status, 201)
            resp = json.loads(body.decode())
            self.assertEqual(resp["key"], "dark_mode")
            self.assertEqual(resp["min_tier"], "basic")

    def test_create_flag_invalid_tier_400(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, body = s.request("POST", "/flags", {
                "key": "bad_flag",
                "enabled": True,
                "min_tier": "gold",
                "allowed_roles": ["admin"]
            })
            self.assertEqual(status, 400)

    def test_evaluate_nonexistent_flag_404(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, body = s.request("POST", "/evaluate", {
                "key": "missing_feature",
                "user_id": "u-1",
                "tier": "enterprise",
                "role": "admin"
            })
            self.assertEqual(status, 404)

    def test_evaluate_disabled_flag(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/flags", {
                "key": "off_feature",
                "enabled": False,
                "min_tier": "basic",
                "allowed_roles": ["admin"],
                "user_overrides": ["u-1"]
            })
            status, _, body = s.request("POST", "/evaluate", {
                "key": "off_feature",
                "user_id": "u-1",
                "tier": "enterprise",
                "role": "admin"
            })
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertFalse(resp["granted"])
            self.assertEqual(resp["reason"], "flag_disabled")

    def test_evaluate_user_override(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/flags", {
                "key": "secret_ai",
                "enabled": True,
                "min_tier": "enterprise",
                "allowed_roles": ["admin"],
                "user_overrides": ["vip-tester"]
            })
            # vip-tester has basic tier and viewer role, but is overridden
            status, _, body = s.request("POST", "/evaluate", {
                "key": "secret_ai",
                "user_id": "vip-tester",
                "tier": "basic",
                "role": "viewer"
            })
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertTrue(resp["granted"])
            self.assertEqual(resp["reason"], "user_override")

    def test_evaluate_insufficient_tier(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/flags", {
                "key": "pdf_export",
                "enabled": True,
                "min_tier": "pro",
                "allowed_roles": ["admin"],
                "user_overrides": []
            })
            status, _, body = s.request("POST", "/evaluate", {
                "key": "pdf_export",
                "user_id": "u-2",
                "tier": "basic", # basic < pro
                "role": "admin"
            })
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertFalse(resp["granted"])
            self.assertEqual(resp["reason"], "insufficient_tier")

    def test_evaluate_role_unauthorized(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/flags", {
                "key": "manage_billing",
                "enabled": True,
                "min_tier": "basic",
                "allowed_roles": ["admin"],
                "user_overrides": []
            })
            status, _, body = s.request("POST", "/evaluate", {
                "key": "manage_billing",
                "user_id": "u-3",
                "tier": "enterprise",
                "role": "viewer" # viewer not in ["admin"]
            })
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertFalse(resp["granted"])
            self.assertEqual(resp["reason"], "role_unauthorized")

    def test_evaluate_entitled_success(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/flags", {
                "key": "cloud_sync",
                "enabled": True,
                "min_tier": "pro",
                "allowed_roles": ["admin", "editor"],
                "user_overrides": []
            })
            status, _, body = s.request("POST", "/evaluate", {
                "key": "cloud_sync",
                "user_id": "u-4",
                "tier": "enterprise", # enterprise >= pro
                "role": "editor"
            })
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertTrue(resp["granted"])
            self.assertEqual(resp["reason"], "entitled")

    def test_get_flag_by_key(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/flags", {"key": "feat1", "enabled": True, "min_tier": "basic", "allowed_roles": []})
            status, _, body = s.request("GET", "/flags/feat1")
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertEqual(resp["key"], "feat1")

            status404, _, _ = s.request("GET", "/flags/nonexistent")
            self.assertEqual(status404, 404)

    def test_delete_flag(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/flags", {"key": "to_del", "enabled": True, "min_tier": "basic", "allowed_roles": []})
            status, _, _ = s.request("DELETE", "/flags/to_del")
            self.assertEqual(status, 204)
            status404, _, _ = s.request("GET", "/flags/to_del")
            self.assertEqual(status404, 404)

    def test_list_flags_sorted(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/flags", {"key": "zeta", "enabled": True, "min_tier": "basic", "allowed_roles": []})
            s.request("POST", "/flags", {"key": "alpha", "enabled": True, "min_tier": "basic", "allowed_roles": []})
            status, _, body = s.request("GET", "/flags")
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            keys = [f["key"] for f in resp["flags"]]
            self.assertEqual(keys, ["alpha", "zeta"])

    def test_persistence_across_restart(self):
        with testkit.Server(args=["--store", self.store_path]) as s1:
            s1.request("POST", "/flags", {"key": "persisted_flag", "enabled": True, "min_tier": "pro", "allowed_roles": ["admin"]})

        with testkit.Server(args=["--store", self.store_path]) as s2:
            status, _, body = s2.request("GET", "/flags/persisted_flag")
            self.assertEqual(status, 200)
            self.assertEqual(json.loads(body.decode())["min_tier"], "pro")

    def test_batch_replay_mode(self):
        reqs_path = os.path.join(self.temp_dir.name, "reqs.jsonl")
        with open(reqs_path, "w") as f:
            f.write(json.dumps({"method": "POST", "path": "/flags", "body": {"key": "f1", "enabled": True, "min_tier": "basic", "allowed_roles": ["admin"]}}) + "\n")
            f.write(json.dumps({"method": "POST", "path": "/evaluate", "body": {"key": "f1", "user_id": "u", "tier": "basic", "role": "admin"}}) + "\n")

        res = testkit.run(["replay", "--store", self.store_path, reqs_path])
        self.assertEqual(res.returncode, 0)
        lines = [json.loads(l) for l in res.stdout.strip().split("\n") if l.strip()]
        self.assertEqual(len(lines), 2)
        self.assertEqual(lines[0]["status"], 201)
        self.assertEqual(lines[1]["status"], 200)
        self.assertTrue(lines[1]["body"]["granted"])


if __name__ == "__main__":
    unittest.main()
