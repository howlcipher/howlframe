"""Score a finished run in a fresh jail (protocol §3.3 scoring isolation, §6 measures, §7.9 outcomes).

The final workspace is copied out of the run directory and checked against the hashes recorded when the run
ended. Acceptance and held-out tests come from the harness's own copies, verified against their frozen hashes,
and run in a scoring jail that has the arm's toolchain, no network, no agent and no proxy. Each test suite gets its
own fresh copy of the workspace. Runtime is measured on the task's frozen workload, minus the arm's empty-program
startup. Signs that a solution delegates its work to another language are flagged for a blind grader.
"""

import json
import os
import re
import shutil
import subprocess
import tempfile
import time

from . import jail
from .results import validate_score
from .runner import HARNESS, tree_manifest

ARM_SOURCE_EXT = {"howlframe": {".howl"}, "python": {".py"}, "go": {".go"}, "javascript": {".js", ".mjs", ".cjs"}}
ALL_SOURCE_EXT = {".howl", ".py", ".go", ".js", ".mjs", ".cjs", ".ts", ".pl", ".rb", ".php", ".lua", ".awk",
                  ".c", ".cc", ".cpp", ".rs", ".java"}
DELEGATION_PATTERNS = [
    (re.compile(r"/opt/cv1/testpy"), "references the test runner's Python"),
    (re.compile(r"\b(python3?|node|perl|ruby|php|lua|awk|gawk)\b"), "names another interpreter"),
    (re.compile(r"\b(bash|sh)\s+-c\b"), "runs an inline shell script"),
]
SCORING_TIMEOUT_S = 15 * 60


class TestsNotVerified(Exception):
    pass


def _read_json(path):
    with open(path) as f:
        return json.load(f)


def verify_tree(path, expected):
    got = tree_manifest(path)
    if got != expected:
        missing = sorted(set(expected) - set(got))
        changed = sorted(k for k in expected if k in got and got[k] != expected[k])
        extra = sorted(set(got) - set(expected))
        raise TestsNotVerified(f"{path}: missing {missing}, changed {changed}, extra {extra}")


def _fresh_copy(src, parent):
    dst = tempfile.mkdtemp(prefix="ws-", dir=parent)
    shutil.copytree(src, dst, dirs_exist_ok=True, symlinks=True)
    return dst


def _scoring_jail(layout, arm, identity, ws, control, tests=None, extra_ro=None):
    return jail.JailSpec(layout.root, identity, ws, toolchain=layout.toolchain(arm), docs=None, tests=tests,
                         cv1_bin=os.path.join(HARNESS, "bin"), testpy=layout.testpy, home=None, control=control,
                         extra_ro=[(os.path.join(HARNESS, "cv1"), "/opt/cv1/lib")] + (extra_ro or []))


def _jail_env(arm):
    return {"PATH": jail.jail_path(), "LANG": "C.UTF-8", "HOME": "/home/agent", "CV1_ARM": arm,
            "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOCACHE": "/home/agent/.cache/go", "GOPATH": "/home/agent/go"}


def _run_in_jail(spec, command, arm, limits, timeout=SCORING_TIMEOUT_S, log=None):
    argv = jail.limits_prefix(limits.get("memory_max", "8G"), limits.get("tasks_max", 1024), limits.get("cpus"),
                              limits.get("use_systemd", True))
    argv += jail.bwrap_argv(spec, command, _jail_env(arm))
    try:
        p = subprocess.run(argv, capture_output=True, text=True, timeout=timeout)
        rc, out, err = p.returncode, p.stdout, p.stderr
    except subprocess.TimeoutExpired as e:
        rc, out, err = "timeout", e.stdout or "", e.stderr or ""
        out = out.decode() if isinstance(out, bytes) else out
        err = err.decode() if isinstance(err, bytes) else err
    if log:
        with open(log, "w") as f:
            f.write(f"$ {' '.join(command)}\nexit: {rc}\n--- stdout\n{out}\n--- stderr\n{err}\n")
    return rc, out, err


def _build(layout, arm, identity, ws, control, limits, log):
    """Run ./build if the solution has one (same rule for every arm)."""
    if not os.path.exists(os.path.join(ws, "build")):
        return {"ran": False}
    spec = _scoring_jail(layout, arm, identity, ws, control)
    rc, _, _ = _run_in_jail(spec, ["/workspace/build"], arm, limits, log=log)
    return {"ran": True, "exit": rc}


def run_suite(layout, arm, identity, ws_src, tests_dir, work, name, limits):
    ws = _fresh_copy(ws_src, work)
    control = tempfile.mkdtemp(prefix=f"{name}-", dir=work)
    build = _build(layout, arm, identity, ws, control, limits, os.path.join(work, f"{name}_build.log"))
    spec = _scoring_jail(layout, arm, identity, ws, control, tests=tests_dir)
    cmd = ["/opt/cv1/testpy/bin/python3", "-I", "/opt/cv1/lib/testrunner.py", "--tests", "/tests",
           "--app", "/workspace/run", "--report", "/run/cv1/report.json"]
    rc, _, _ = _run_in_jail(spec, cmd, arm, limits, log=os.path.join(work, f"{name}_tests.log"))
    report_path = os.path.join(control, "report.json")
    report = _read_json(report_path) if os.path.exists(report_path) else None
    return {"build": build, "exit": rc, "report": report,
            "all_passed": bool(report and report.get("all_passed")),
            "passed": report["passed"] if report else 0, "total": report["total"] if report else 0}


def measure_runtime(layout, arm, identity, ws_src, workload, work, limits):
    """workload: {"args": [...], "stdin": "relative/path" or null, "files_dir": "task-relative dir" or null}"""
    out = {"loadavg_before": os.getloadavg(), "cpus": limits.get("cpus")}
    for label, src in (("program", ws_src), ("empty", os.path.join(HARNESS, "empty", arm))):
        ws = _fresh_copy(src, work)
        control = tempfile.mkdtemp(prefix=f"rt-{label}-", dir=work)
        _build(layout, arm, identity, ws, control, limits, os.path.join(work, f"runtime_{label}_build.log"))
        extra = []
        stdin = None
        if label == "program" and workload.get("dir"):
            extra.append((workload["dir"], "/workload"))
            if workload.get("stdin"):
                stdin = "/workload/" + workload["stdin"]
        spec = _scoring_jail(layout, arm, identity, ws, control, extra_ro=extra)
        args = json.dumps(workload.get("args", []) if label == "program" else [])
        cmd = ["/opt/cv1/testpy/bin/python3", "-I", "/opt/cv1/lib/runtime_probe.py", "--app", "/workspace/run",
               "--args", args, "--cwd", "/workspace"] + (["--stdin", stdin] if stdin else [])
        rc, stdout, _ = _run_in_jail(spec, cmd, arm, limits, log=os.path.join(work, f"runtime_{label}.log"))
        try:
            out[label] = json.loads(stdout.strip().splitlines()[-1])
        except (ValueError, IndexError):
            out[label] = {"error": f"probe exit {rc}"}
    out["loadavg_after"] = os.getloadavg()
    prog, empty = out.get("program", {}), out.get("empty", {})
    if prog.get("median_s") is not None and empty.get("median_s") is not None:
        out["execution_s"] = max(0.0, prog["median_s"] - empty["median_s"])
        out["startup_s"] = empty["median_s"]
    else:
        out["execution_s"] = None
    return out


def delegation_flags(arm, ws):
    flags = []
    own = ARM_SOURCE_EXT[arm]
    for rel in sorted(tree_manifest(ws)):
        ext = os.path.splitext(rel)[1]
        if ext in ALL_SOURCE_EXT and ext not in own:
            flags.append({"file": rel, "why": f"source file in another language ({ext})"})
        if ext in own or os.path.basename(rel) in ("run", "build"):
            try:
                with open(os.path.join(ws, rel), errors="replace") as f:
                    text = f.read()
            except OSError:
                continue
            for pattern, why in DELEGATION_PATTERNS:
                if arm == "python" and why == "names another interpreter" and ext == ".py":
                    continue
                m = pattern.search(text)
                if m and not (os.path.basename(rel) in ("run", "build") and why == "names another interpreter"
                              and m.group(0) in _arm_interpreters(arm)):
                    flags.append({"file": rel, "why": why, "match": m.group(0)})
    return flags


def _arm_interpreters(arm):
    return {"python": {"python", "python3"}, "javascript": {"node"}}.get(arm, set())


def score_run(run_dir, task, layout, frozen_acceptance, heldout_dir=None, frozen_heldout=None, workload=None,
              expressibility_gap=False, limits=None, human_interventions=None):
    result = _read_json(os.path.join(run_dir, "result.json"))
    arm = result["arm"]
    limits = limits or result.get("limits") or {}
    work = tempfile.mkdtemp(prefix="score-", dir=run_dir)
    identity = jail.write_identity_files(os.path.join(work, "identity"))
    notes = []
    score = {"schema": "cv1-score-result/1", "run_id": result["run_id"], "task_id": result["task_id"], "arm": arm,
             "acceptance": None, "heldout": None, "heldout_fraction": None, "tampering": [], "delegation_flags": [],
             "runtime": None, "notes": notes, "tests_verified": False,
             "scored_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}
    try:
        verify_tree(task.acceptance, frozen_acceptance)
        if heldout_dir:
            verify_tree(heldout_dir, frozen_heldout)
        score["tests_verified"] = True
    except TestsNotVerified as e:
        notes.append(f"test hashes do not match the freeze: {e}")
    ws_src = os.path.join(run_dir, "workspace")
    if result["status"] != "not_started":
        current = tree_manifest(ws_src)
        if current != result["final_workspace_sha256"]:
            notes.append("workspace changed after the run ended; scoring the recorded state is impossible")
            score["tampering"].append({"what": "workspace modified after the run"})
        if result.get("acceptance_tests_sha256") != frozen_acceptance:
            score["tampering"].append({"what": "acceptance tests at run time differ from the freeze"})
        if score["tests_verified"]:
            score["acceptance"] = run_suite(layout, arm, identity, ws_src, task.acceptance, work, "acceptance", limits)
            if heldout_dir:
                h = run_suite(layout, arm, identity, ws_src, heldout_dir, work, "heldout", limits)
                score["heldout"] = h
                score["heldout_fraction"] = (h["passed"] / h["total"]) if h["total"] else 0.0
            if workload and score["acceptance"]["all_passed"]:
                score["runtime"] = measure_runtime(layout, arm, identity, ws_src, workload, work, limits)
        score["delegation_flags"] = delegation_flags(arm, ws_src)

    status = result["status"]
    if status == "not_started":
        outcome = "not_started"
    elif status == "infrastructure_failure":
        outcome = "infrastructure_failure"
    elif human_interventions or result.get("human_interventions"):
        outcome = "not_autonomous"
    elif status in ("not_completed_time", "not_completed_budget", "not_completed_tool_calls"):
        outcome = status
    elif arm == "howlframe" and expressibility_gap:
        outcome = "expressibility_failure"
    elif not score["tests_verified"]:
        outcome = "infrastructure_failure"
    elif score["acceptance"] and score["acceptance"]["all_passed"]:
        outcome = "completed"
    else:
        outcome = "not_completed_acceptance"
    score["outcome"] = outcome
    validate_score(score)
    with open(os.path.join(run_dir, "score.json"), "w") as f:
        json.dump(score, f, indent=2, sort_keys=True)
        f.write("\n")
    return score
