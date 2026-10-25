#!/usr/bin/env python3
import json
import os
import subprocess
import sys
import time

def fail(msg, code=2):
    sys.stderr.write(f"Error: {msg}\n")
    sys.exit(code)

def main():
    if len(sys.argv) != 4:
        fail("Usage: ./run <policy.json> <requests.json> <audit.json>", 2)

    policy_path = sys.argv[1]
    requests_path = sys.argv[2]
    audit_path = sys.argv[3]

    try:
        with open(policy_path, "r", encoding="utf-8") as f:
            policy = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse policy JSON: {e}", 2)

    try:
        with open(requests_path, "r", encoding="utf-8") as f:
            requests_data = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse requests JSON: {e}", 2)

    if not isinstance(policy, dict) or "roles" not in policy or not isinstance(policy["roles"], dict):
        fail("Policy JSON must contain 'roles' object", 2)

    if not isinstance(requests_data, dict) or "requests" not in requests_data or not isinstance(requests_data["requests"], list):
        fail("Requests JSON must contain 'requests' array", 2)

    jail_root = policy.get("jail_root")
    if jail_root is not None:
        jail_root = os.path.realpath(os.path.abspath(jail_root))

    executions = []
    allowed_and_executed = 0
    denied = 0
    had_execution_failure = False

    for req in requests_data["requests"]:
        if not isinstance(req, dict):
            fail("Request must be an object", 2)

        req_id = req.get("id")
        role_name = req.get("role")
        command = req.get("command")

        if not isinstance(req_id, str) or not req_id.strip():
            fail("Request missing valid 'id'", 2)
        if not isinstance(role_name, str) or not role_name.strip():
            fail(f"Request '{req_id}' missing valid 'role'", 2)
        if not isinstance(command, list) or len(command) == 0 or not all(isinstance(c, str) for c in command):
            fail(f"Request '{req_id}' missing valid 'command' array", 2)

        # 1. Role existence
        if role_name not in policy["roles"]:
            denied += 1
            executions.append({
                "id": req_id,
                "decision": "deny",
                "reason": "unknown_role"
            })
            continue

        role = policy["roles"][role_name]
        allowed_bins = role.get("allowed_binaries", [])
        disallowed_args = role.get("disallowed_args", [])
        allow_env = set(role.get("allow_env", []))
        max_timeout_ms = role.get("max_timeout_ms", 5000)
        max_output_bytes = role.get("max_output_bytes", 1048576)

        # 2. Binary whitelist check
        binary = command[0]
        if binary not in allowed_bins:
            denied += 1
            executions.append({
                "id": req_id,
                "decision": "deny",
                "reason": "binary_not_permitted"
            })
            continue

        # 3. Disallowed args check
        has_forbidden_arg = False
        for arg in command:
            for forbidden in disallowed_args:
                if forbidden in arg:
                    has_forbidden_arg = True
                    break
            if has_forbidden_arg:
                break

        if has_forbidden_arg:
            denied += 1
            executions.append({
                "id": req_id,
                "decision": "deny",
                "reason": "forbidden_argument"
            })
            continue

        # 4. Jail check
        working_dir = req.get("working_dir")
        if working_dir:
            real_work_dir = os.path.realpath(os.path.abspath(working_dir))
        else:
            real_work_dir = os.path.realpath(os.getcwd())

        if jail_root is not None:
            # Check if real_work_dir is within jail_root
            if not (real_work_dir == jail_root or real_work_dir.startswith(jail_root + os.sep)):
                denied += 1
                executions.append({
                    "id": req_id,
                    "decision": "deny",
                    "reason": "jail_violation"
                })
                continue

        # 5. Environment sanitization
        req_env = req.get("env", {})
        clean_env = {"PATH": os.environ.get("PATH", "/usr/bin:/bin")}
        if isinstance(req_env, dict):
            for k, v in req_env.items():
                if k in allow_env and isinstance(v, str):
                    clean_env[k] = v

        # 6. Execute subprocess with limits
        req_timeout = req.get("timeout_ms", 5000)
        effective_timeout = min(req_timeout, max_timeout_ms) / 1000.0

        allowed_and_executed += 1
        timed_out = False
        exit_code = None
        stdout_str = ""
        stderr_str = ""
        truncated = False
        status = "failed"

        try:
            proc = subprocess.run(
                command,
                cwd=real_work_dir,
                env=clean_env,
                capture_output=True,
                text=True,
                timeout=effective_timeout
            )
            exit_code = proc.returncode
            stdout_str = proc.stdout
            stderr_str = proc.stderr
            if exit_code == 0:
                status = "success"
            else:
                had_execution_failure = True
        except subprocess.TimeoutExpired as te:
            timed_out = True
            had_execution_failure = True
            status = "timed_out"
            stdout_str = te.stdout.decode() if isinstance(te.stdout, bytes) else (te.stdout or "")
            stderr_str = te.stderr.decode() if isinstance(te.stderr, bytes) else (te.stderr or "")

        # Truncate stdout if exceeded
        if len(stdout_str.encode("utf-8")) > max_output_bytes:
            truncated = True
            # truncate byte-wise or char-wise safely
            encoded = stdout_str.encode("utf-8")[:max_output_bytes]
            stdout_str = encoded.decode("utf-8", errors="ignore")

        executions.append({
            "id": req_id,
            "decision": "allow",
            "status": status,
            "exit_code": exit_code,
            "stdout": stdout_str,
            "stderr": stderr_str,
            "timed_out": timed_out,
            "truncated": truncated
        })

    report = {
        "total_requests": len(executions),
        "allowed_and_executed": allowed_and_executed,
        "denied": denied,
        "executions": executions
    }

    try:
        with open(audit_path, "w", encoding="utf-8") as f:
            json.dump(report, f, indent=2)
    except Exception as e:
        fail(f"Failed to write audit report: {e}", 2)

    sys.exit(1 if had_execution_failure else 0)

if __name__ == "__main__":
    main()
