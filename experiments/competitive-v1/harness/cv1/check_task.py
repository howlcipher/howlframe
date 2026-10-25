"""Check a task directory before it is frozen (protocol §3.2 step 5 reference feasibility proof, §3.3 tests).

    python3 -m cv1.check_task TASK_DIR --out OUT [--cache CACHE] [--cpus LIST]

Validates task.json and the layout, then scores each reference implementation in references/<arm>/ exactly as an
agent's final workspace would be scored: a fresh scoring jail, hash-verified acceptance and held-out tests, and the
runtime workload. No model or network is involved. Prints a JSON report with the hashes to seal, and exits 0 only
if the task is complete and every baseline reference (Python, Go, JavaScript) passes every test.
"""

import argparse
import hashlib
import json
import os
import shutil
import subprocess
import sys

from . import runner, scorer

REQUIRED_META = ("id", "domain", "authority_centred", "title", "author", "attestation", "workload")
BASELINES = ("python", "go", "javascript")
# §3.3: the workload is sized so the reference runs 0.5–5 s, excluding process startup.
WORKLOAD_RANGE_S = (0.5, 5.0)


def static_problems(task):
    """Problems that need no jail: metadata, prompt, test files, workload layout."""
    problems = []
    missing = [k for k in REQUIRED_META if k not in task.meta]
    if missing:
        problems.append(f"task.json lacks {missing}")
    if task.meta.get("authority_centred") not in (True, False):
        problems.append("task.json authority_centred must be true or false")
    prompt = os.path.join(task.path, "prompt.md")
    if not os.path.isfile(prompt) or not task.prompt_body().strip():
        problems.append("prompt.md is missing or empty")
    for name, d in (("acceptance", task.acceptance), ("heldout", task.heldout)):
        if not d or not any(f.startswith("test_") and f.endswith(".py") for f in os.listdir(d)):
            problems.append(f"{name}/ has no test_*.py files")
    try:
        w = task.workload()
    except ValueError as e:
        problems.append(str(e))
    else:
        if w and w["stdin"] and not os.path.isfile(os.path.join(w["dir"], w["stdin"])):
            problems.append(f"workload stdin {w['stdin']!r} is not a file in workload/")
    for arm in BASELINES:
        ref = task.reference(arm)
        if not ref:
            problems.append(f"references/{arm}/ is missing")
        elif not os.access(os.path.join(ref, "run"), os.X_OK):
            problems.append(f"references/{arm}/run is missing or not executable")
    return problems


def digest(manifest):
    """One SHA-256 over a tree manifest, for publishing a seal."""
    return hashlib.sha256(json.dumps(manifest, sort_keys=True).encode()).hexdigest()


def score_reference(task, arm, layout, out, frozen, frozen_heldout, limits):
    run_dir = os.path.join(out, f"reference-{arm}")
    shutil.rmtree(run_dir, ignore_errors=True)
    ws = os.path.join(run_dir, "workspace")
    shutil.copytree(task.reference(arm), ws, symlinks=True)
    # A minimal run record: score_run reads only these fields. It is not a run result and is not schema-validated.
    record = {"run_id": f"reference-{arm}", "task_id": task.id, "arm": arm, "status": "ended_by_agent",
              "final_workspace_sha256": runner.tree_manifest(ws), "acceptance_tests_sha256": frozen,
              "limits": limits}
    with open(os.path.join(run_dir, "result.json"), "w") as f:
        json.dump(record, f, indent=2, sort_keys=True)
    s = scorer.score_run(run_dir, task, layout, frozen, task.heldout, frozen_heldout, workload=task.workload(),
                         limits=limits, workload10=task.workload10())
    runtime = s["runtime"] or {}
    acc, held = s["acceptance"] or {}, s["heldout"] or {}
    return {"arm": arm, "outcome": s["outcome"],
            "acceptance": f"{acc.get('passed', 0)}/{acc.get('total', 0)}",
            "heldout": f"{held.get('passed', 0)}/{held.get('total', 0)}" if task.heldout else None,
            "heldout_all_passed": bool(held.get("all_passed")) if task.heldout else None,
            "execution_s": runtime.get("execution_s"), "startup_s": runtime.get("startup_s"),
            "resource_limit": (s["resource_limit"] or {}).get("behavior"), "canary_triggered": s["canary_triggered"],
            "delegation_flags": s["delegation_flags"], "notes": s["notes"]}


def check(task_dir, layout, out, cpus=None):
    task = runner.Task(task_dir)
    os.makedirs(out, exist_ok=True)
    try:
        task.prepare_workload(layout.python())
        problems = static_problems(task)
    except (ValueError, OSError, subprocess.CalledProcessError) as e:
        problems = [f"workload: {e}"]
        report = {"task": task.id, "problems": problems, "ok": False}
        with open(os.path.join(out, "check_task_report.json"), "w") as f:
            json.dump(report, f, indent=2)
        return report
    frozen = runner.tree_manifest(task.acceptance)
    frozen_heldout = runner.tree_manifest(task.heldout) if task.heldout else None
    limits = {"memory_max": "4G", "tasks_max": 512, "cpus": cpus, "use_systemd": True}
    refs = []
    for arm in runner.ARMS:
        if task.reference(arm) and os.access(os.path.join(task.reference(arm), "run"), os.X_OK):
            refs.append(score_reference(task, arm, layout, out, frozen, frozen_heldout, limits))
    for r in refs:
        if r["arm"] not in BASELINES:
            continue
        if r["outcome"] != "completed":
            problems.append(f"{r['arm']} reference: {r['outcome']} (acceptance {r['acceptance']})")
        if r["canary_triggered"]:
            problems.append(f"{r['arm']} reference triggered a security canary")
        if task.heldout and not r["heldout_all_passed"]:
            problems.append(f"{r['arm']} reference fails held-out tests ({r['heldout']})")
        lo, hi = WORKLOAD_RANGE_S
        if task.workload() is not None and r["outcome"] == "completed":
            t = r["execution_s"]
            if t is None or not lo <= t <= hi:
                problems.append(f"{r['arm']} reference workload time {t} s is outside {lo}-{hi} s")
    report = {"task": task.id, "domain": task.meta.get("domain"),
              "authority_centred": task.meta.get("authority_centred"),
              "seal": {"acceptance_sha256": digest(frozen),
                       "heldout_sha256": digest(frozen_heldout) if frozen_heldout is not None else None,
                       "workload_sha256": digest(runner.tree_manifest(os.path.join(task.path, "workload")))
                       if os.path.isdir(os.path.join(task.path, "workload")) else None},
              "references": refs, "missing_references": [a for a in runner.ARMS if not task.reference(a)],
              "problems": problems, "ok": not problems}
    with open(os.path.join(out, "check_task_report.json"), "w") as f:
        json.dump(report, f, indent=2)
    return report


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("task")
    p.add_argument("--out", required=True)
    p.add_argument("--cache", default=os.environ.get("CV1_CACHE"))
    p.add_argument("--cpus", help="taskset CPU list for the scoring jails")
    args = p.parse_args(argv)
    if not args.cache:
        p.error("--cache (or CV1_CACHE) is required")
    report = check(args.task, runner.Layout(args.cache), args.out, args.cpus)
    print(json.dumps(report, indent=2))
    return 0 if report["ok"] else 1


if __name__ == "__main__":
    sys.exit(main())
