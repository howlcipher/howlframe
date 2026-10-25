#!/usr/bin/env python3
import argparse
import concurrent.futures
import json
import os
import subprocess
import sys
import threading
import time

def fail(msg, code=2):
    sys.stderr.write(f"Error: {msg}\n")
    sys.exit(code)

def main():
    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("matrix_path", nargs="?")
    parser.add_argument("results_path", nargs="?")
    parser.add_argument("--concurrency", type=int, default=None)

    try:
        args, extra = parser.parse_known_args()
        if extra or not args.matrix_path or not args.results_path:
            fail("Missing required arguments", 2)
    except Exception as e:
        fail(f"Invalid CLI args: {e}", 2)

    try:
        with open(args.matrix_path, "r", encoding="utf-8") as f:
            matrix = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse matrix JSON: {e}", 2)

    if not isinstance(matrix, dict) or "cases" not in matrix or not isinstance(matrix["cases"], list):
        fail("Matrix must contain 'cases' array", 2)

    if len(matrix["cases"]) == 0:
        fail("'cases' array must not be empty", 2)

    seen_ids = set()
    validated_cases = []
    for c in matrix["cases"]:
        if not isinstance(c, dict):
            fail("Case must be an object", 2)
        cid = c.get("id")
        cmd = c.get("command")
        if not isinstance(cid, str) or not cid.strip():
            fail("Case missing valid 'id'", 2)
        if cid in seen_ids:
            fail(f"Duplicate case id: {cid}", 2)
        seen_ids.add(cid)

        if not isinstance(cmd, list) or len(cmd) == 0 or not all(isinstance(x, str) for x in cmd):
            fail(f"Case '{cid}' missing valid 'command' array", 2)

        validated_cases.append(c)

    default_timeout = matrix.get("default_timeout_ms", 5000)
    fail_fast = bool(matrix.get("fail_fast", False))

    concurrency = args.concurrency
    if concurrency is None:
        concurrency = matrix.get("concurrency", 1)
    if not isinstance(concurrency, int) or concurrency < 1:
        fail("Concurrency must be >= 1", 2)

    results_map = {}
    lock = threading.Lock()
    aborted = threading.Event()

    def run_case(c):
        cid = c["id"]
        with lock:
            if fail_fast and aborted.is_set():
                results_map[cid] = {
                    "id": cid,
                    "status": "skipped",
                    "reason": "fail_fast_triggered"
                }
                return

        # Prepare env
        env = dict(os.environ)
        if isinstance(c.get("env"), dict):
            for k, v in c["env"].items():
                if isinstance(v, str):
                    env[k] = v

        timeout_ms = c.get("timeout_ms", default_timeout)
        t0 = time.monotonic()
        timed_out = False
        exit_code = None
        stdout_str = ""
        stderr_str = ""

        try:
            proc = subprocess.run(
                c["command"],
                env=env,
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

        duration_ms = max(0, int((time.monotonic() - t0) * 1000))

        status = "passed"
        if timed_out:
            status = "timed_out"
            if fail_fast:
                aborted.set()
        elif exit_code != 0:
            status = "failed"
            if fail_fast:
                aborted.set()

        res = {
            "id": cid,
            "status": status,
            "exit_code": exit_code,
            "duration_ms": duration_ms,
            "stdout": stdout_str,
            "stderr": stderr_str
        }

        with lock:
            results_map[cid] = res

    # Run using ThreadPoolExecutor
    with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as executor:
        futures = []
        for c in validated_cases:
            futures.append(executor.submit(run_case, c))
        concurrent.futures.wait(futures)

    # Order results as in input cases
    cases_output = []
    total_passed = 0
    total_failed = 0
    total_skipped = 0

    for c in validated_cases:
        cid = c["id"]
        res = results_map.get(cid)
        if not res:
            res = {
                "id": cid,
                "status": "skipped",
                "reason": "fail_fast_triggered"
            }
        cases_output.append(res)
        if res["status"] == "passed":
            total_passed += 1
        elif res["status"] in ("failed", "timed_out"):
            total_failed += 1
        elif res["status"] == "skipped":
            total_skipped += 1

    report = {
        "total": len(cases_output),
        "passed": total_passed,
        "failed": total_failed,
        "skipped": total_skipped,
        "cases": cases_output
    }

    try:
        with open(args.results_path, "w", encoding="utf-8") as f:
            json.dump(report, f, indent=2)
    except Exception as e:
        fail(f"Failed to write results: {e}", 2)

    sys.exit(0 if total_passed == len(cases_output) else 1)

if __name__ == "__main__":
    main()
