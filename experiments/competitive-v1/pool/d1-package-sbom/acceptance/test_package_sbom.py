import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../kit")))
import testkit


class TestPackageSbom(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.policy_path = os.path.join(self.temp_dir.name, "policy.json")
        self.sbom_path = os.path.join(self.temp_dir.name, "sbom.json")
        self.report_path = os.path.join(self.temp_dir.name, "report.json")

        self.policy = {
            "allowed_licenses": ["MIT", "Apache-2.0"],
            "tier_capabilities": {
                "tier_1_core": ["network", "filesystem_read", "process_spawn"],
                "tier_2_plugin": ["filesystem_read"],
                "tier_3_untrusted": []
            },
            "package_tiers": {
                "pkg-core": "tier_1_core",
                "pkg-plugin": "tier_2_plugin"
            },
            "default_tier": "tier_3_untrusted"
        }
        with open(self.policy_path, "w") as f:
            json.dump(self.policy, f)

    def tearDown(self):
        self.temp_dir.cleanup()

    def _write_sbom(self, packages):
        with open(self.sbom_path, "w") as f:
            json.dump({"application": "test-app", "packages": packages}, f)

    def test_compliant_sbom_exit_0(self):
        self._write_sbom([
            {
                "name": "pkg-core",
                "version": "1.0.0",
                "license": "Apache-2.0",
                "requested_capabilities": ["filesystem_read", "network"]
            }
        ])
        res = testkit.run(["verify-sbom", "--sbom", self.sbom_path, "--policy", self.policy_path, "--report", self.report_path])
        self.assertEqual(res.returncode, 0)
        with open(self.report_path) as f:
            rep = json.load(f)
        self.assertTrue(rep["compliant"])
        self.assertEqual(rep["violations_count"], 0)
        self.assertEqual(rep["violations"], [])
        self.assertEqual(rep["effective_capabilities"]["pkg-core"], ["filesystem_read", "network"])

    def test_unauthorized_capability_exit_2(self):
        self._write_sbom([
            {
                "name": "pkg-plugin",
                "version": "1.0.0",
                "license": "MIT",
                "requested_capabilities": ["filesystem_read", "process_spawn"] # process_spawn not allowed in tier_2
            }
        ])
        res = testkit.run(["verify-sbom", "--sbom", self.sbom_path, "--policy", self.policy_path, "--report", self.report_path])
        self.assertEqual(res.returncode, 2)
        with open(self.report_path) as f:
            rep = json.load(f)
        self.assertFalse(rep["compliant"])
        self.assertEqual(rep["violations_count"], 1)
        v = rep["violations"][0]
        self.assertEqual(v["package"], "pkg-plugin")
        self.assertEqual(v["violation_type"], "unauthorized_capability")
        self.assertIn("process_spawn", v["detail"])
        self.assertEqual(rep["effective_capabilities"]["pkg-plugin"], ["filesystem_read"])

    def test_disallowed_license_exit_2(self):
        self._write_sbom([
            {
                "name": "pkg-core",
                "version": "1.0.0",
                "license": "GPL-3.0", # Not allowed
                "requested_capabilities": ["network"]
            }
        ])
        res = testkit.run(["verify-sbom", "--sbom", self.sbom_path, "--policy", self.policy_path, "--report", self.report_path])
        self.assertEqual(res.returncode, 2)
        with open(self.report_path) as f:
            rep = json.load(f)
        self.assertFalse(rep["compliant"])
        self.assertEqual(rep["violations_count"], 1)
        v = rep["violations"][0]
        self.assertEqual(v["violation_type"], "disallowed_license")
        self.assertIn("GPL-3.0", v["detail"])

    def test_default_tier_assignment(self):
        self._write_sbom([
            {
                "name": "pkg-unknown",
                "version": "0.1.0",
                "license": "MIT",
                "requested_capabilities": ["filesystem_read"]
            }
        ])
        res = testkit.run(["verify-sbom", "--sbom", self.sbom_path, "--policy", self.policy_path, "--report", self.report_path])
        self.assertEqual(res.returncode, 2)
        with open(self.report_path) as f:
            rep = json.load(f)
        self.assertFalse(rep["compliant"])
        self.assertIn("tier_3_untrusted", rep["violations"][0]["detail"])
        self.assertEqual(rep["effective_capabilities"]["pkg-unknown"], [])

    def test_effective_capabilities_computation(self):
        self._write_sbom([
            {
                "name": "pkg-core",
                "version": "1.0.0",
                "license": "MIT",
                "requested_capabilities": ["process_spawn", "network"]
            }
        ])
        res = testkit.run(["verify-sbom", "--sbom", self.sbom_path, "--policy", self.policy_path, "--report", self.report_path])
        self.assertEqual(res.returncode, 0)
        with open(self.report_path) as f:
            rep = json.load(f)
        # alphabetical sort
        self.assertEqual(rep["effective_capabilities"]["pkg-core"], ["network", "process_spawn"])

    def test_multiple_violations_same_package(self):
        self._write_sbom([
            {
                "name": "pkg-plugin",
                "version": "1.0.0",
                "license": "AGPL-3.0",
                "requested_capabilities": ["network"] # network not allowed in tier_2
            }
        ])
        res = testkit.run(["verify-sbom", "--sbom", self.sbom_path, "--policy", self.policy_path, "--report", self.report_path])
        self.assertEqual(res.returncode, 2)
        with open(self.report_path) as f:
            rep = json.load(f)
        self.assertEqual(rep["violations_count"], 2)
        types = [v["violation_type"] for v in rep["violations"]]
        self.assertIn("disallowed_license", types)
        self.assertIn("unauthorized_capability", types)

    def test_violations_sorting(self):
        self._write_sbom([
            {
                "name": "pkg-z",
                "version": "1.0.0",
                "license": "AGPL-3.0",
                "requested_capabilities": []
            },
            {
                "name": "pkg-a",
                "version": "1.0.0",
                "license": "AGPL-3.0",
                "requested_capabilities": []
            }
        ])
        res = testkit.run(["verify-sbom", "--sbom", self.sbom_path, "--policy", self.policy_path, "--report", self.report_path])
        self.assertEqual(res.returncode, 2)
        with open(self.report_path) as f:
            rep = json.load(f)
        self.assertEqual(rep["violations"][0]["package"], "pkg-a")
        self.assertEqual(rep["violations"][1]["package"], "pkg-z")

    def test_effective_capabilities_keys_sorted(self):
        self._write_sbom([
            {"name": "zebra", "version": "1.0.0", "license": "MIT", "requested_capabilities": []},
            {"name": "alpha", "version": "1.0.0", "license": "MIT", "requested_capabilities": []}
        ])
        res = testkit.run(["verify-sbom", "--sbom", self.sbom_path, "--policy", self.policy_path, "--report", self.report_path])
        self.assertEqual(res.returncode, 0)
        with open(self.report_path) as f:
            rep = json.load(f)
        keys = list(rep["effective_capabilities"].keys())
        self.assertEqual(keys, ["alpha", "zebra"])

    def test_empty_requested_capabilities(self):
        self._write_sbom([
            {"name": "pkg-core", "version": "1.0.0", "license": "MIT", "requested_capabilities": []}
        ])
        res = testkit.run(["verify-sbom", "--sbom", self.sbom_path, "--policy", self.policy_path, "--report", self.report_path])
        self.assertEqual(res.returncode, 0)
        with open(self.report_path) as f:
            rep = json.load(f)
        self.assertTrue(rep["compliant"])
        self.assertEqual(rep["effective_capabilities"]["pkg-core"], [])

    def test_missing_flags_exit_1(self):
        res = testkit.run(["verify-sbom", "--sbom", self.sbom_path])
        self.assertEqual(res.returncode, 1)

    def test_invalid_subcommand_exit_1(self):
        res = testkit.run(["check", "--sbom", self.sbom_path, "--policy", self.policy_path, "--report", self.report_path])
        self.assertEqual(res.returncode, 1)

    def test_nonexistent_files_exit_1(self):
        res = testkit.run(["verify-sbom", "--sbom", "/missing/sbom.json", "--policy", self.policy_path, "--report", self.report_path])
        self.assertEqual(res.returncode, 1)

    def test_malformed_json_input_exit_1(self):
        bad_json = os.path.join(self.temp_dir.name, "bad.json")
        with open(bad_json, "w") as f:
            f.write("{invalid: json}")
        res = testkit.run(["verify-sbom", "--sbom", bad_json, "--policy", self.policy_path, "--report", self.report_path])
        self.assertEqual(res.returncode, 1)


if __name__ == "__main__":
    unittest.main()
