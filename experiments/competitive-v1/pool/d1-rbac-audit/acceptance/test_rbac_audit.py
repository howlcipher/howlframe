import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../kit")))
import testkit


class TestRbacAudit(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.iam_path = os.path.join(self.temp_dir.name, "iam.json")
        self.boundaries_path = os.path.join(self.temp_dir.name, "boundaries.json")
        self.out_path = os.path.join(self.temp_dir.name, "audit.json")

        self.boundaries = {
            "max_session_duration_hours": 10,
            "forbidden_actions": ["iam:DeleteAccount", "billing:*"],
            "privilege_escalation_pairs": [
                ["iam:PassRole", "lambda:CreateFunction"]
            ]
        }
        with open(self.boundaries_path, "w") as f:
            json.dump(self.boundaries, f)

    def tearDown(self):
        self.temp_dir.cleanup()

    def _write_iam(self, roles):
        with open(self.iam_path, "w") as f:
            json.dump({"roles": roles}, f)

    def test_clean_iam_exit_0(self):
        self._write_iam([
            {
                "role_name": "DevRole",
                "assigned_users": ["alice"],
                "session_duration_hours": 8,
                "permissions": ["s3:GetObject", "s3:PutObject"]
            }
        ])
        res = testkit.run(["audit-iam", "--iam", self.iam_path, "--boundaries", self.boundaries_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertTrue(out["clean"])
        self.assertEqual(out["findings_count"], 0)
        self.assertEqual(out["findings"], [])
        self.assertEqual(out["user_effective_roles"]["alice"], ["DevRole"])

    def test_forbidden_action_exact_match_exit_2(self):
        self._write_iam([
            {
                "role_name": "AdminRole",
                "assigned_users": ["bob"],
                "session_duration_hours": 8,
                "permissions": ["s3:GetObject", "iam:DeleteAccount"]
            }
        ])
        res = testkit.run(["audit-iam", "--iam", self.iam_path, "--boundaries", self.boundaries_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 2)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertFalse(out["clean"])
        self.assertEqual(out["findings_count"], 1)
        f = out["findings"][0]
        self.assertEqual(f["finding_type"], "forbidden_action")
        self.assertEqual(f["severity"], "CRITICAL")
        self.assertIn("iam:DeleteAccount", f["detail"])

    def test_forbidden_action_wildcard_prefix(self):
        self._write_iam([
            {
                "role_name": "BillingAuditor",
                "assigned_users": ["carol"],
                "session_duration_hours": 4,
                "permissions": ["billing:ViewInvoice"]
            }
        ])
        res = testkit.run(["audit-iam", "--iam", self.iam_path, "--boundaries", self.boundaries_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 2)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertFalse(out["clean"])
        f = out["findings"][0]
        self.assertEqual(f["finding_type"], "forbidden_action")
        self.assertEqual(f["severity"], "CRITICAL")
        self.assertIn("billing:ViewInvoice", f["detail"])

    def test_privilege_escalation_pair(self):
        self._write_iam([
            {
                "role_name": "Deployer",
                "assigned_users": ["dave"],
                "session_duration_hours": 6,
                "permissions": ["iam:PassRole", "lambda:CreateFunction"]
            }
        ])
        res = testkit.run(["audit-iam", "--iam", self.iam_path, "--boundaries", self.boundaries_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 2)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertFalse(out["clean"])
        f = out["findings"][0]
        self.assertEqual(f["finding_type"], "privilege_escalation")
        self.assertEqual(f["severity"], "HIGH")

    def test_partial_privilege_escalation_safe(self):
        self._write_iam([
            {
                "role_name": "DeployerSafe",
                "assigned_users": ["dave"],
                "session_duration_hours": 6,
                "permissions": ["iam:PassRole"] # Only 1 action of pair
            }
        ])
        res = testkit.run(["audit-iam", "--iam", self.iam_path, "--boundaries", self.boundaries_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertTrue(out["clean"])

    def test_excessive_session_duration(self):
        self._write_iam([
            {
                "role_name": "LongSessionRole",
                "assigned_users": ["eve"],
                "session_duration_hours": 16, # Exceeds 10h limit
                "permissions": ["s3:GetObject"]
            }
        ])
        res = testkit.run(["audit-iam", "--iam", self.iam_path, "--boundaries", self.boundaries_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 2)
        with open(self.out_path) as f:
            out = json.load(f)
        f = out["findings"][0]
        self.assertEqual(f["finding_type"], "excessive_session")
        self.assertEqual(f["severity"], "MEDIUM")

    def test_user_effective_roles_aggregation(self):
        self._write_iam([
            {"role_name": "Role-B", "assigned_users": ["alice"], "session_duration_hours": 4, "permissions": []},
            {"role_name": "Role-A", "assigned_users": ["alice", "bob"], "session_duration_hours": 4, "permissions": []}
        ])
        res = testkit.run(["audit-iam", "--iam", self.iam_path, "--boundaries", self.boundaries_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["user_effective_roles"]["alice"], ["Role-A", "Role-B"])
        self.assertEqual(out["user_effective_roles"]["bob"], ["Role-A"])

    def test_findings_severity_ranking(self):
        self._write_iam([
            {
                "role_name": "MultiIssueRole",
                "assigned_users": ["frank"],
                "session_duration_hours": 20, # MEDIUM
                "permissions": ["iam:DeleteAccount", "iam:PassRole", "lambda:CreateFunction"] # CRITICAL, HIGH
            }
        ])
        res = testkit.run(["audit-iam", "--iam", self.iam_path, "--boundaries", self.boundaries_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 2)
        with open(self.out_path) as f:
            out = json.load(f)
        severities = [f["severity"] for f in out["findings"]]
        self.assertEqual(severities, ["CRITICAL", "HIGH", "MEDIUM"])

    def test_empty_roles(self):
        self._write_iam([])
        res = testkit.run(["audit-iam", "--iam", self.iam_path, "--boundaries", self.boundaries_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertTrue(out["clean"])
        self.assertEqual(out["roles_audited"], 0)

    def test_missing_cli_flags_exit_1(self):
        res = testkit.run(["audit-iam", "--iam", self.iam_path])
        self.assertEqual(res.returncode, 1)

    def test_wrong_subcommand_exit_1(self):
        res = testkit.run(["verify", "--iam", self.iam_path, "--boundaries", self.boundaries_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 1)

    def test_missing_files_exit_1(self):
        res = testkit.run(["audit-iam", "--iam", "/missing/iam.json", "--boundaries", self.boundaries_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 1)

    def test_multiple_roles_findings_sorted(self):
        self._write_iam([
            {"role_name": "Role-Z", "assigned_users": [], "session_duration_hours": 15, "permissions": []},
            {"role_name": "Role-A", "assigned_users": [], "session_duration_hours": 15, "permissions": []}
        ])
        res = testkit.run(["audit-iam", "--iam", self.iam_path, "--boundaries", self.boundaries_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 2)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["findings"][0]["role_name"], "Role-A")
        self.assertEqual(out["findings"][1]["role_name"], "Role-Z")


if __name__ == "__main__":
    unittest.main()
