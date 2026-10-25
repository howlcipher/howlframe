import json
import os
import shutil
import tempfile
import unittest
import testkit

class TestLogPurger(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()
        self.rules_path = os.path.join(self.tmpdir.name, "rules.json")
        self.log_dir = os.path.join(self.tmpdir.name, "logs")
        self.rep_path = os.path.join(self.tmpdir.name, "report.json")
        os.makedirs(self.log_dir, exist_ok=True)
        self.now = 1700000000

        self.default_rules = {
            "default_retention_days": 30,
            "tag_retention_days": {
                "compliance": 365,
                "debug": 3
            }
        }
        with open(self.rules_path, "w") as f:
            json.dump(self.default_rules, f)

    def tearDown(self):
        self.tmpdir.cleanup()

    def create_log(self, name, days_ago, tags=None, content="test message"):
        data = {
            "timestamp_epoch": self.now - (days_ago * 86400),
            "tags": tags or [],
            "message": content
        }
        path = os.path.join(self.log_dir, name)
        with open(path, "w") as f:
            json.dump(data, f)
        return path

    def test_default_age_pruning(self):
        # 10 days old (kept under 30d default), 40 days old (expired)
        f_kept = self.create_log("keep.json", 10)
        f_exp = self.create_log("exp.json", 40)

        res = testkit.run([self.rules_path, self.log_dir, self.rep_path, "--now-epoch", str(self.now)])
        self.assertEqual(res.returncode, 0)
        with open(self.rep_path) as f:
            rep = json.load(f)

        self.assertEqual(rep["total_scanned"], 2)
        self.assertEqual(rep["age_purged_count"], 1)
        self.assertEqual(rep["retained_count"], 1)
        self.assertEqual(rep["purged_files"][0]["filename"], "exp.json")
        self.assertEqual(rep["purged_files"][0]["reason"], "age_expired")

        self.assertTrue(os.path.exists(f_kept))
        self.assertFalse(os.path.exists(f_exp))

    def test_tag_retention_override_debug(self):
        # Debug log is 5 days old: default 30d would keep, but debug rule is 3d -> expired!
        f_dbg = self.create_log("debug.json", 5, tags=["debug"])
        res = testkit.run([self.rules_path, self.log_dir, self.rep_path, "--now-epoch", str(self.now)])
        self.assertEqual(res.returncode, 0)
        self.assertFalse(os.path.exists(f_dbg))

    def test_compliance_tag_extended_retention(self):
        # Compliance log is 100 days old: default 30d would expire, but compliance 365d keeps it
        f_comp = self.create_log("audit.json", 100, tags=["compliance"])
        res = testkit.run([self.rules_path, self.log_dir, self.rep_path, "--now-epoch", str(self.now)])
        self.assertEqual(res.returncode, 0)
        self.assertTrue(os.path.exists(f_comp))
        with open(self.rep_path) as f:
            rep = json.load(f)
        self.assertEqual(rep["retained_count"], 1)

    def test_multiple_tags_maximum_retention(self):
        # Matches both "debug" (3d) and "compliance" (365d) -> should use max (365d)
        f_multi = self.create_log("multi.json", 50, tags=["debug", "compliance"])
        res = testkit.run([self.rules_path, self.log_dir, self.rep_path, "--now-epoch", str(self.now)])
        self.assertEqual(res.returncode, 0)
        self.assertTrue(os.path.exists(f_multi))

    def test_dry_run_does_not_delete(self):
        f_exp = self.create_log("exp.json", 40)
        res = testkit.run([self.rules_path, self.log_dir, self.rep_path, "--dry-run", "--now-epoch", str(self.now)])
        self.assertEqual(res.returncode, 0)
        with open(self.rep_path) as f:
            rep = json.load(f)
        self.assertTrue(rep["dry_run"])
        self.assertEqual(rep["age_purged_count"], 1)
        # File must still exist on disk because of --dry-run
        self.assertTrue(os.path.exists(f_exp))

    def test_disk_quota_enforcement(self):
        # 3 surviving files of ~100 bytes each. Quota set to 150 bytes.
        # Oldest non-compliance file must be pruned for quota.
        f1 = self.create_log("a_old.json", 20, content="x" * 100)
        f2 = self.create_log("b_mid.json", 15, content="x" * 100)
        f3 = self.create_log("c_new.json", 5, content="x" * 100)

        # Total bytes around ~400. Set quota to 250 bytes so at least one is pruned.
        rules = {
            "default_retention_days": 30,
            "max_storage_bytes": 260
        }
        with open(self.rules_path, "w") as f:
            json.dump(rules, f)

        res = testkit.run([self.rules_path, self.log_dir, self.rep_path, "--now-epoch", str(self.now)])
        self.assertEqual(res.returncode, 0)
        with open(self.rep_path) as f:
            rep = json.load(f)
        self.assertGreaterEqual(rep["quota_purged_count"], 1)
        # a_old.json is the oldest non-compliance file and should have been pruned
        self.assertFalse(os.path.exists(f1))
        self.assertTrue(os.path.exists(f3))

    def test_compliance_files_exempt_from_quota(self):
        # Both files exceed quota, but one has compliance tag
        f_comp = self.create_log("comp.json", 20, tags=["compliance"], content="x" * 200)
        f_norm = self.create_log("norm.json", 5, tags=["app"], content="x" * 200)

        rules = {
            "default_retention_days": 30,
            "tag_retention_days": {"compliance": 365},
            "max_storage_bytes": 350
        }
        with open(self.rules_path, "w") as f:
            json.dump(rules, f)

        res = testkit.run([self.rules_path, self.log_dir, self.rep_path, "--now-epoch", str(self.now)])
        self.assertEqual(res.returncode, 0)
        # norm.json pruned, comp.json protected
        self.assertFalse(os.path.exists(f_norm))
        self.assertTrue(os.path.exists(f_comp))

    def test_quota_unmet_exit_1(self):
        # Only compliance file exists and it alone exceeds max_storage_bytes
        self.create_log("big_comp.json", 10, tags=["compliance"], content="x" * 500)
        rules = {
            "default_retention_days": 30,
            "tag_retention_days": {"compliance": 365},
            "max_storage_bytes": 50 # smaller than the compliance file!
        }
        with open(self.rules_path, "w") as f:
            json.dump(rules, f)

        res = testkit.run([self.rules_path, self.log_dir, self.rep_path, "--now-epoch", str(self.now)])
        self.assertEqual(res.returncode, 1)

    def test_legacy_retention_days_alias(self):
        # Accepts baseline "retention_days" key
        rules = {"retention_days": 10}
        with open(self.rules_path, "w") as f:
            json.dump(rules, f)
        self.create_log("old.json", 15)
        res = testkit.run([self.rules_path, self.log_dir, self.rep_path, "--now-epoch", str(self.now)])
        self.assertEqual(res.returncode, 0)
        with open(self.rep_path) as f:
            rep = json.load(f)
        self.assertEqual(rep["age_purged_count"], 1)

    def test_missing_args(self):
        res = testkit.run([])
        self.assertEqual(res.returncode, 2)

    def test_malformed_json_rules(self):
        with open(self.rules_path, "w") as f:
            f.write("{invalid")
        res = testkit.run([self.rules_path, self.log_dir, self.rep_path])
        self.assertEqual(res.returncode, 2)

    def test_invalid_directory(self):
        res = testkit.run([self.rules_path, "/non/existent/dir/path", self.rep_path])
        self.assertEqual(res.returncode, 2)
