import json
import os
import tempfile
import unittest
import testkit

class TestRbacFilter(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()
        self.pol_path = os.path.join(self.tmpdir.name, "policy.json")
        self.req_path = os.path.join(self.tmpdir.name, "requests.json")
        self.dec_path = os.path.join(self.tmpdir.name, "decisions.json")

        self.default_policy = {
            "roles": {
                "admin": ["*"],
                "editor": ["doc:read", "doc:write", "doc:delete"],
                "viewer": ["doc:read"]
            },
            "owner_restricted_actions": ["doc:delete"]
        }
        with open(self.pol_path, "w") as f:
            json.dump(self.default_policy, f)

    def tearDown(self):
        self.tmpdir.cleanup()

    def test_basic_authorized_allow(self):
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "r1",
                        "user": "alice",
                        "tenant_id": "t1",
                        "roles": ["viewer"],
                        "action": "doc:read",
                        "resource": "doc-1",
                        "resource_tenant_id": "t1"
                    }
                ]
            }, f)
        res = testkit.run([self.pol_path, self.req_path, self.dec_path])
        self.assertEqual(res.returncode, 0)
        with open(self.dec_path) as f:
            data = json.load(f)
        self.assertEqual(data["allowed_count"], 1)
        self.assertEqual(data["decisions"][0]["decision"], "ALLOW")
        self.assertEqual(data["decisions"][0]["reason"], "authorized")
        self.assertEqual(data["decisions"][0]["effective_roles"], ["viewer"])

    def test_tenant_mismatch_denied(self):
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "r-mismatch",
                        "user": "alice",
                        "tenant_id": "t1",
                        "roles": ["admin"],
                        "action": "doc:read",
                        "resource": "doc-1",
                        "resource_tenant_id": "t2" # mismatch!
                    }
                ]
            }, f)
        res = testkit.run([self.pol_path, self.req_path, self.dec_path])
        self.assertEqual(res.returncode, 1)
        with open(self.dec_path) as f:
            data = json.load(f)
        self.assertEqual(data["denied_count"], 1)
        self.assertEqual(data["decisions"][0]["decision"], "DENY")
        self.assertEqual(data["decisions"][0]["reason"], "tenant_mismatch")

    def test_insufficient_permissions_denied(self):
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "r-noperm",
                        "user": "alice",
                        "tenant_id": "t1",
                        "roles": ["viewer"],
                        "action": "doc:write",
                        "resource": "doc-1",
                        "resource_tenant_id": "t1"
                    }
                ]
            }, f)
        res = testkit.run([self.pol_path, self.req_path, self.dec_path])
        self.assertEqual(res.returncode, 1)
        with open(self.dec_path) as f:
            data = json.load(f)
        self.assertEqual(data["decisions"][0]["decision"], "DENY")
        self.assertEqual(data["decisions"][0]["reason"], "insufficient_permissions")

    def test_owner_restricted_action_non_owner_denied(self):
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "r-not-owner",
                        "user": "alice",
                        "tenant_id": "t1",
                        "roles": ["editor"],
                        "action": "doc:delete",
                        "resource": "doc-1",
                        "resource_tenant_id": "t1",
                        "resource_owner": "bob"
                    }
                ]
            }, f)
        res = testkit.run([self.pol_path, self.req_path, self.dec_path])
        self.assertEqual(res.returncode, 1)
        with open(self.dec_path) as f:
            data = json.load(f)
        self.assertEqual(data["decisions"][0]["decision"], "DENY")
        self.assertEqual(data["decisions"][0]["reason"], "not_resource_owner")

    def test_owner_restricted_action_owner_allowed(self):
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "r-owner",
                        "user": "alice",
                        "tenant_id": "t1",
                        "roles": ["editor"],
                        "action": "doc:delete",
                        "resource": "doc-1",
                        "resource_tenant_id": "t1",
                        "resource_owner": "alice"
                    }
                ]
            }, f)
        res = testkit.run([self.pol_path, self.req_path, self.dec_path])
        self.assertEqual(res.returncode, 0)
        with open(self.dec_path) as f:
            data = json.load(f)
        self.assertEqual(data["decisions"][0]["decision"], "ALLOW")
        self.assertEqual(data["decisions"][0]["reason"], "authorized")

    def test_admin_wildcard_bypasses_ownership(self):
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "r-admin",
                        "user": "charlie",
                        "tenant_id": "t1",
                        "roles": ["admin"],
                        "action": "doc:delete",
                        "resource": "doc-1",
                        "resource_tenant_id": "t1",
                        "resource_owner": "bob"
                    }
                ]
            }, f)
        res = testkit.run([self.pol_path, self.req_path, self.dec_path])
        self.assertEqual(res.returncode, 0)
        with open(self.dec_path) as f:
            data = json.load(f)
        self.assertEqual(data["decisions"][0]["decision"], "ALLOW")

    def test_active_delegation_grants_access(self):
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "r-deleg-ok",
                        "user": "alice",
                        "tenant_id": "t1",
                        "roles": ["viewer"],
                        "action": "doc:write",
                        "resource": "doc-1",
                        "resource_tenant_id": "t1",
                        "delegation": {
                            "delegator": "bob",
                            "roles": ["editor"],
                            "valid_until_epoch": 1700050000
                        }
                    }
                ]
            }, f)
        res = testkit.run([self.pol_path, self.req_path, self.dec_path, "--timestamp", "1700000000"])
        self.assertEqual(res.returncode, 0)
        with open(self.dec_path) as f:
            data = json.load(f)
        self.assertEqual(data["decisions"][0]["decision"], "ALLOW")
        self.assertTrue(data["decisions"][0]["delegation_active"])
        self.assertEqual(data["decisions"][0]["effective_roles"], ["editor", "viewer"])

    def test_expired_delegation_ignored(self):
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "r-deleg-exp",
                        "user": "alice",
                        "tenant_id": "t1",
                        "roles": ["viewer"],
                        "action": "doc:write",
                        "resource": "doc-1",
                        "resource_tenant_id": "t1",
                        "delegation": {
                            "delegator": "bob",
                            "roles": ["editor"],
                            "valid_until_epoch": 1600000000 # expired!
                        }
                    }
                ]
            }, f)
        res = testkit.run([self.pol_path, self.req_path, self.dec_path, "--timestamp", "1700000000"])
        self.assertEqual(res.returncode, 1)
        with open(self.dec_path) as f:
            data = json.load(f)
        self.assertEqual(data["decisions"][0]["decision"], "DENY")
        self.assertFalse(data["decisions"][0]["delegation_active"])
        self.assertEqual(data["decisions"][0]["effective_roles"], ["viewer"])

    def test_missing_tenant_id_validation_error(self):
        with open(self.req_path, "w") as f:
            json.dump({
                "requests": [
                    {"id": "r-no-tenant", "user": "alice", "roles": ["viewer"], "action": "doc:read"}
                ]
            }, f)
        res = testkit.run([self.pol_path, self.req_path, self.dec_path])
        self.assertEqual(res.returncode, 2)

    def test_missing_args(self):
        res = testkit.run([])
        self.assertEqual(res.returncode, 2)

    def test_malformed_json(self):
        bad_path = os.path.join(self.tmpdir.name, "bad.json")
        with open(bad_path, "w") as f:
            f.write("bad json")
        res = testkit.run([self.pol_path, bad_path, self.dec_path])
        self.assertEqual(res.returncode, 2)
