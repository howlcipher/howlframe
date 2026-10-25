import json
import os
import shutil
import tempfile
import unittest
import testkit

class TestSandboxExecPolicy(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()
        self.jail = os.path.realpath(os.path.join(self.tmpdir.name, "jail"))
        os.makedirs(self.jail, exist_ok=True)

        self.policy_path = os.path.join(self.tmpdir.name, "policy.json")
        self.requests_path = os.path.join(self.tmpdir.name, "requests.json")
        self.audit_path = os.path.join(self.tmpdir.name, "audit.json")

        self.default_policy = {
            "jail_root": self.jail,
            "roles": {
                "runner": {
                    "allowed_binaries": ["/usr/bin/coreutils", "/usr/bin/dash"],
                    "disallowed_args": [";", "&", "|", ".."],
                    "allow_env": ["SAFE_VAR"],
                    "max_timeout_ms": 3000,
                    "max_output_bytes": 1024
                }
            }
        }
        with open(self.policy_path, "w") as f:
            json.dump(self.default_policy, f)

    def tearDown(self):
        self.tmpdir.cleanup()

    def test_allowed_execution_success(self):
        with open(self.requests_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "r1",
                        "role": "runner",
                        "command": ["/usr/bin/coreutils", "echo", "hello"],
                        "working_dir": self.jail,
                        "timeout_ms": 1000
                    }
                ]
            }, f)
        res = testkit.run([self.policy_path, self.requests_path, self.audit_path])
        self.assertEqual(res.returncode, 0)
        with open(self.audit_path) as f:
            audit = json.load(f)
        self.assertEqual(audit["total_requests"], 1)
        self.assertEqual(audit["allowed_and_executed"], 1)
        self.assertEqual(audit["denied"], 0)
        self.assertEqual(audit["executions"][0]["decision"], "allow")
        self.assertEqual(audit["executions"][0]["status"], "success")
        self.assertEqual(audit["executions"][0]["exit_code"], 0)
        self.assertIn("hello", audit["executions"][0]["stdout"])

    def test_deny_unknown_role(self):
        with open(self.requests_path, "w") as f:
            json.dump({
                "requests": [
                    {"id": "r-bad-role", "role": "admin", "command": ["/usr/bin/coreutils", "echo", "hi"]}
                ]
            }, f)
        res = testkit.run([self.policy_path, self.requests_path, self.audit_path])
        self.assertEqual(res.returncode, 0)
        with open(self.audit_path) as f:
            audit = json.load(f)
        self.assertEqual(audit["denied"], 1)
        self.assertEqual(audit["executions"][0]["decision"], "deny")
        self.assertEqual(audit["executions"][0]["reason"], "unknown_role")

    def test_deny_unauthorized_binary(self):
        with open(self.requests_path, "w") as f:
            json.dump({
                "requests": [
                    {"id": "r-bad-bin", "role": "runner", "command": ["/usr/bin/bash"], "working_dir": self.jail}
                ]
            }, f)
        res = testkit.run([self.policy_path, self.requests_path, self.audit_path])
        self.assertEqual(res.returncode, 0)
        with open(self.audit_path) as f:
            audit = json.load(f)
        self.assertEqual(audit["executions"][0]["decision"], "deny")
        self.assertEqual(audit["executions"][0]["reason"], "binary_not_permitted")

    def test_deny_forbidden_argument(self):
        with open(self.requests_path, "w") as f:
            json.dump({
                "requests": [
                    {"id": "r-bad-arg", "role": "runner", "command": ["/usr/bin/coreutils", "echo", "foo;bar"], "working_dir": self.jail}
                ]
            }, f)
        res = testkit.run([self.policy_path, self.requests_path, self.audit_path])
        self.assertEqual(res.returncode, 0)
        with open(self.audit_path) as f:
            audit = json.load(f)
        self.assertEqual(audit["executions"][0]["decision"], "deny")
        self.assertEqual(audit["executions"][0]["reason"], "forbidden_argument")

    def test_deny_jail_escape(self):
        outside_dir = os.path.dirname(self.jail)
        with open(self.requests_path, "w") as f:
            json.dump({
                "requests": [
                    {"id": "r-escape", "role": "runner", "command": ["/usr/bin/coreutils", "echo", "escaped"], "working_dir": outside_dir}
                ]
            }, f)
        res = testkit.run([self.policy_path, self.requests_path, self.audit_path])
        self.assertEqual(res.returncode, 0)
        with open(self.audit_path) as f:
            audit = json.load(f)
        self.assertEqual(audit["executions"][0]["decision"], "deny")
        self.assertEqual(audit["executions"][0]["reason"], "jail_violation")

    def test_env_sanitization(self):
        script_file = os.path.join(self.jail, "check_env.sh")
        with open(script_file, "w") as f:
            f.write('echo "${SAFE_VAR:-NONE}:${BAD_VAR:-NONE}"\n')

        with open(self.requests_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "r-env",
                        "role": "runner",
                        "command": ["/usr/bin/dash", script_file],
                        "working_dir": self.jail,
                        "env": {"SAFE_VAR": "allowed_value", "BAD_VAR": "secret_leak"}
                    }
                ]
            }, f)
        res = testkit.run([self.policy_path, self.requests_path, self.audit_path])
        self.assertEqual(res.returncode, 0)
        with open(self.audit_path) as f:
            audit = json.load(f)
        self.assertEqual(audit["executions"][0]["stdout"].strip(), "allowed_value:NONE")

    def test_timeout_enforcement(self):
        script_file = os.path.join(self.jail, "sleep_script.sh")
        with open(script_file, "w") as f:
            f.write("/usr/bin/coreutils sleep 5\n")

        with open(self.requests_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "r-timeout",
                        "role": "runner",
                        "command": ["/usr/bin/dash", script_file],
                        "working_dir": self.jail,
                        "timeout_ms": 300
                    }
                ]
            }, f)
        res = testkit.run([self.policy_path, self.requests_path, self.audit_path], timeout=10)
        self.assertEqual(res.returncode, 1)
        with open(self.audit_path) as f:
            audit = json.load(f)
        self.assertEqual(audit["executions"][0]["decision"], "allow")
        self.assertEqual(audit["executions"][0]["status"], "timed_out")
        self.assertTrue(audit["executions"][0]["timed_out"])

    def test_executed_command_failure_exit_code(self):
        script_file = os.path.join(self.jail, "fail_script.sh")
        with open(script_file, "w") as f:
            f.write("exit 42\n")

        with open(self.requests_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "r-fail",
                        "role": "runner",
                        "command": ["/usr/bin/dash", script_file],
                        "working_dir": self.jail
                    }
                ]
            }, f)
        res = testkit.run([self.policy_path, self.requests_path, self.audit_path])
        self.assertEqual(res.returncode, 1)
        with open(self.audit_path) as f:
            audit = json.load(f)
        self.assertEqual(audit["executions"][0]["exit_code"], 42)
        self.assertEqual(audit["executions"][0]["status"], "failed")

    def test_output_truncation(self):
        script_file = os.path.join(self.jail, "gen_output.sh")
        with open(script_file, "w") as f:
            f.write('printf "%s" "' + ('A' * 2000) + '"\n')

        with open(self.requests_path, "w") as f:
            json.dump({
                "requests": [
                    {
                        "id": "r-trunc",
                        "role": "runner",
                        "command": ["/usr/bin/dash", script_file],
                        "working_dir": self.jail
                    }
                ]
            }, f)
        res = testkit.run([self.policy_path, self.requests_path, self.audit_path])
        self.assertEqual(res.returncode, 0)
        with open(self.audit_path) as f:
            audit = json.load(f)
        self.assertTrue(audit["executions"][0]["truncated"])
        self.assertEqual(len(audit["executions"][0]["stdout"]), 1024)

    def test_missing_args(self):
        res = testkit.run([self.policy_path])
        self.assertEqual(res.returncode, 2)

    def test_malformed_json(self):
        bad_path = os.path.join(self.tmpdir.name, "bad.json")
        with open(bad_path, "w") as f:
            f.write("invalid")
        res = testkit.run([self.policy_path, bad_path, self.audit_path])
        self.assertEqual(res.returncode, 2)
