#!/usr/bin/env python3
import argparse
import json
import os
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.request

def fail(msg, code=2):
    sys.stderr.write(f"Error: {msg}\n")
    sys.exit(code)

def http_post(url, data_dict, timeout=5):
    body = json.dumps(data_dict).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=body,
        method="POST",
        headers={"Content-Type": "application/json"}
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            resp_body = resp.read()
            parsed = json.loads(resp_body) if resp_body else {}
            return resp.status, parsed
    except urllib.error.HTTPError as he:
        resp_body = he.read()
        try:
            parsed = json.loads(resp_body)
        except Exception:
            parsed = {}
        return he.code, parsed
    except Exception:
        return None, {}

def main():
    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("job_path", nargs="?")
    parser.add_argument("result_path", nargs="?")
    parser.add_argument("--server", default="http://127.0.0.1:8080")
    parser.add_argument("--offline-mock", default=None)

    try:
        args, extra = parser.parse_known_args()
        if extra or not args.job_path or not args.result_path:
            fail("Missing required arguments", 2)
    except Exception as e:
        fail(f"Invalid CLI args: {e}", 2)

    try:
        with open(args.job_path, "r", encoding="utf-8") as f:
            job = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse job JSON: {e}", 2)

    if not isinstance(job, dict):
        fail("Job must be an object", 2)

    lock_name = job.get("lock_name")
    owner_id = job.get("owner_id")
    command = job.get("command")

    if not isinstance(lock_name, str) or not lock_name.strip():
        fail("Missing valid 'lock_name'", 2)
    if not isinstance(owner_id, str) or not owner_id.strip():
        fail("Missing valid 'owner_id'", 2)
    if not isinstance(command, list) or len(command) == 0 or not all(isinstance(c, str) for c in command):
        fail("Missing valid 'command' array of strings", 2)

    lease_ttl_ms = job.get("lease_ttl_ms", 2000)
    renew_interval_ms = job.get("renew_interval_ms", 500)
    max_acquire_wait_ms = job.get("max_acquire_wait_ms", 3000)
    timeout_ms = job.get("timeout_ms", 10000)

    server_base = args.server.rstrip("/")
    acquired = False
    token = None

    if args.offline_mock:
        try:
            with open(args.offline_mock, "r", encoding="utf-8") as f:
                mock_data = json.load(f)
            if mock_data.get("acquire_status") == 200:
                acquired = True
                token = mock_data.get("token", "mock-token-offline")
            else:
                acquired = False
        except Exception as e:
            fail(f"Failed reading mock: {e}", 2)
    else:
        deadline = time.time() + (max_acquire_wait_ms / 1000.0)
        while time.time() <= deadline:
            st, data = http_post(f"{server_base}/lock/acquire", {
                "lock_name": lock_name,
                "owner_id": owner_id,
                "ttl_ms": lease_ttl_ms
            })
            if st == 200 and data.get("acquired", True):
                acquired = True
                token = data.get("token", f"tok-{lock_name}")
                break
            time.sleep(0.05)

    if not acquired:
        report = {
            "lock_acquired": False,
            "token": None,
            "renew_count": 0,
            "command_executed": False,
            "exit_code": None,
            "stdout": "",
            "stderr": "",
            "lock_released": False
        }
        with open(args.result_path, "w", encoding="utf-8") as f:
            json.dump(report, f, indent=2)
        sys.exit(1)

    # Start renewal heartbeat
    renew_count = 0
    stop_event = threading.Event()

    def renew_loop():
        nonlocal renew_count
        interval_sec = renew_interval_ms / 1000.0
        while not stop_event.wait(interval_sec):
            if not args.offline_mock:
                st, _ = http_post(f"{server_base}/lock/renew", {
                    "lock_name": lock_name,
                    "owner_id": owner_id,
                    "token": token,
                    "ttl_ms": lease_ttl_ms
                })
                if st == 200:
                    renew_count += 1
            else:
                renew_count += 1

    thread = threading.Thread(target=renew_loop)
    thread.daemon = True
    thread.start()

    # Execute command
    exit_code = None
    stdout_str = ""
    stderr_str = ""
    timed_out = False

    try:
        proc = subprocess.run(
            command,
            capture_output=True,
            text=True,
            timeout=timeout_ms / 1000.0
        )
        exit_code = proc.returncode
        stdout_str = proc.stdout
        stderr_str = proc.stderr
    except subprocess.TimeoutExpired as te:
        timed_out = True
        exit_code = None
        stdout_str = te.stdout.decode() if isinstance(te.stdout, bytes) else (te.stdout or "")
        stderr_str = te.stderr.decode() if isinstance(te.stderr, bytes) else (te.stderr or "")
    finally:
        stop_event.set()
        thread.join(timeout=1.0)

    # Release lock
    released = False
    if not args.offline_mock:
        st, _ = http_post(f"{server_base}/lock/release", {
            "lock_name": lock_name,
            "owner_id": owner_id,
            "token": token
        })
        released = (st == 200)
    else:
        released = True

    report = {
        "lock_acquired": True,
        "token": token,
        "renew_count": renew_count,
        "command_executed": True,
        "exit_code": exit_code,
        "stdout": stdout_str,
        "stderr": stderr_str,
        "lock_released": released
    }

    try:
        with open(args.result_path, "w", encoding="utf-8") as f:
            json.dump(report, f, indent=2)
    except Exception as e:
        fail(f"Failed to write result: {e}", 2)

    sys.exit(0 if exit_code == 0 and not timed_out else 1)

if __name__ == "__main__":
    main()
