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
    if len(sys.argv) != 3:
        fail("Usage: ./run <pipeline.json> <report.json>", 2)

    pipeline_path = sys.argv[1]
    report_path = sys.argv[2]

    try:
        with open(pipeline_path, "r", encoding="utf-8") as f:
            data = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse pipeline json: {e}", 2)

    if not isinstance(data, dict) or "tasks" not in data or not isinstance(data["tasks"], list):
        fail("Input JSON must have top-level object with 'tasks' array", 2)

    task_map = {}
    task_order = []
    for item in data["tasks"]:
        if not isinstance(item, dict):
            fail("Task must be an object", 2)
        tid = item.get("id")
        if not isinstance(tid, str) or not tid.strip():
            fail("Task 'id' must be a non-empty string", 2)
        if tid in task_map:
            fail(f"Duplicate task id: {tid}", 2)

        cmd = item.get("command")
        if not isinstance(cmd, list) or len(cmd) == 0 or not all(isinstance(c, str) for c in cmd):
            fail(f"Task '{tid}' must have a non-empty 'command' array of strings", 2)

        timeout_ms = item.get("timeout_ms", 5000)
        if not isinstance(timeout_ms, int) or timeout_ms <= 0:
            fail(f"Task '{tid}' invalid timeout_ms", 2)

        max_retries = item.get("max_retries", 0)
        if not isinstance(max_retries, int) or max_retries < 0:
            fail(f"Task '{tid}' invalid max_retries", 2)

        deps = item.get("depends_on", [])
        if not isinstance(deps, list) or not all(isinstance(d, str) for d in deps):
            fail(f"Task '{tid}' invalid depends_on", 2)

        task_entry = {
            "id": tid,
            "command": cmd,
            "timeout_ms": timeout_ms,
            "max_retries": max_retries,
            "depends_on": deps
        }
        task_map[tid] = task_entry
        task_order.append(tid)

    # Validate dependencies
    for tid, t in task_map.items():
        for dep in t["depends_on"]:
            if dep not in task_map:
                fail(f"Task '{tid}' depends on unknown task '{dep}'", 2)

    # Cycle detection
    visited = {} # tid -> 0 (visiting), 1 (visited)
    def dfs(node):
        visited[node] = 0
        for dep in task_map[node]["depends_on"]:
            if visited.get(dep) == 0:
                fail(f"Cycle detected involving '{node}' and '{dep}'", 2)
            if dep not in visited:
                dfs(dep)
        visited[node] = 1

    for tid in task_map:
        if tid not in visited:
            dfs(tid)

    results = {}
    pending = set(task_map.keys())

    while pending:
        # Find all tasks whose dependencies are satisfied
        ready = []
        for tid in sorted(pending):
            deps = task_map[tid]["depends_on"]
            if all(d in results for d in deps):
                ready.append(tid)

        if not ready:
            fail("Deadlock or unresolved dependencies in pipeline", 2)

        # Process ready tasks in ascending lexicographical order
        for tid in ready:
            t = task_map[tid]
            # Check if any dependency failed or was skipped
            dep_failed = any(results[d]["status"] in ("failed", "skipped") for d in t["depends_on"])
            if dep_failed:
                results[tid] = {
                    "status": "skipped",
                    "reason": "dependency_failed"
                }
                pending.remove(tid)
                continue

            # Execute task
            attempts = 0
            max_attempts = 1 + t["max_retries"]
            final_status = "failed"
            final_code = None
            final_timeout = False
            final_out = ""
            final_err = ""

            for attempt in range(max_attempts):
                attempts += 1
                try:
                    proc = subprocess.run(
                        t["command"],
                        capture_output=True,
                        text=True,
                        timeout=t["timeout_ms"] / 1000.0
                    )
                    final_code = proc.returncode
                    final_out = proc.stdout
                    final_err = proc.stderr
                    final_timeout = False
                    if proc.returncode == 0:
                        final_status = "success"
                        break
                except subprocess.TimeoutExpired as e:
                    final_timeout = True
                    final_code = None
                    final_out = e.stdout.decode() if isinstance(e.stdout, bytes) else (e.stdout or "")
                    final_err = e.stderr.decode() if isinstance(e.stderr, bytes) else (e.stderr or "")

            results[tid] = {
                "status": final_status,
                "attempts": attempts,
                "exit_code": final_code,
                "timed_out": final_timeout,
                "stdout": final_out,
                "stderr": final_err
            }
            pending.remove(tid)

    total_tasks = len(task_map)
    completed_tasks = sum(1 for r in results.values() if r["status"] == "success")
    failed_tasks = sum(1 for r in results.values() if r["status"] == "failed")
    skipped_tasks = sum(1 for r in results.values() if r["status"] == "skipped")
    all_success = (completed_tasks == total_tasks)

    # Order tasks alphabetically in output
    sorted_tasks = {k: results[k] for k in sorted(results.keys())}

    report = {
        "success": all_success,
        "total_tasks": total_tasks,
        "completed_tasks": completed_tasks,
        "failed_tasks": failed_tasks,
        "skipped_tasks": skipped_tasks,
        "tasks": sorted_tasks
    }

    try:
        with open(report_path, "w", encoding="utf-8") as f:
            json.dump(report, f, indent=2)
    except Exception as e:
        fail(f"Failed to write report: {e}", 2)

    sys.exit(0 if all_success else 1)

if __name__ == "__main__":
    main()
