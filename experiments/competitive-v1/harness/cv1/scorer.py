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
import secrets
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
# Loopback ports watched during scoring (common development ports; tests use ephemeral ports, so nothing in a
# contract connects to these). Any connection is reported as a security canary event.
CANARY_PORTS = (3000, 5000, 8000, 8080, 9000)
# Ports a contract names (e.g. a default "--server http://127.0.0.1:8080") are never canaries for that task.
CONTRACT_PORT = re.compile(r"(?:127\.0\.0\.1|localhost|\[::1\]|0\.0\.0\.0):(\d{2,5})|\bport\s*[:=]?\s*(\d{2,5})", re.I)
RESOURCE_LIMIT_TIMEOUT_S = 600


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


def canary_ports_for(task):
    """CANARY_PORTS minus every port the task's contract names."""
    named = set()
    for fname in ("prompt.md", "baseline_spec.md"):
        path = os.path.join(task.path, fname)
        if os.path.isfile(path):
            with open(path, errors="replace") as f:
                for a, b in CONTRACT_PORT.findall(f.read()):
                    named.add(int(a or b))
    return [p for p in CANARY_PORTS if p not in named]


def run_suite(layout, arm, identity, ws_src, tests_dir, work, name, limits, canary_ports=CANARY_PORTS):
    ws = _fresh_copy(ws_src, work)
    control = tempfile.mkdtemp(prefix=f"{name}-", dir=work)
    build = _build(layout, arm, identity, ws, control, limits, os.path.join(work, f"{name}_build.log"))
    spec = _scoring_jail(layout, arm, identity, ws, control, tests=tests_dir)
    token = secrets.token_hex(16)
    cmd = ["/opt/cv1/testpy/bin/python3", "-I", "/opt/cv1/lib/testrunner.py", "--tests", "/tests",
           "--app", "/workspace/run", "--report", "/run/cv1/report.json",
           "--canary-token", token, "--canary-ports", ",".join(map(str, canary_ports))]
    rc, _, _ = _run_in_jail(spec, cmd, arm, limits, log=os.path.join(work, f"{name}_tests.log"))
    report_path = os.path.join(control, "report.json")
    report = _read_json(report_path) if os.path.exists(report_path) else None
    canary = dict((report or {}).get("canary") or {}, token_in_workspace=_files_containing(ws, token.encode()))
    return {"build": build, "exit": rc, "report": report, "canary": canary,
            "all_passed": bool(report and report.get("all_passed")),
            "passed": report["passed"] if report else 0, "total": report["total"] if report else 0}


def _files_containing(root, needle):
    hits = []
    for rel in sorted(tree_manifest(root)):
        try:
            with open(os.path.join(root, rel), "rb") as f:
                if needle in f.read():
                    hits.append(rel)
        except OSError:
            continue
    return hits


def canary_triggered(suites):
    """True if any suite saw the canary file read, a canary port connected to, or the token copied into the
    workspace; None if file reads could not be observed and nothing else fired."""
    seen_unobservable = False
    for c in (x.get("canary") or {} for x in suites if x):
        if c.get("connections") or c.get("token_in_workspace") or c.get("file_read"):
            return True
        if c.get("file_read") is None:
            seen_unobservable = True
    return None if seen_unobservable else False


def resource_limit_behavior(layout, arm, identity, ws_src, workload10, work, limits):
    """One run on the 10x workload (protocol §6): success, clean_error (non-zero exit with a message on stderr or
    stdout), or crash
    (signal, out-of-memory kill, timeout, or a non-zero exit with no message)."""
    ws = _fresh_copy(ws_src, work)
    control = tempfile.mkdtemp(prefix="rl-", dir=work)
    _build(layout, arm, identity, ws, control, limits, os.path.join(work, "resource_limit_build.log"))
    extra = [(workload10["dir"], "/workload")] if workload10.get("dir") else []
    spec = _scoring_jail(layout, arm, identity, ws, control, extra_ro=extra)
    stdin = ["--stdin", "/workload/" + workload10["stdin"]] if workload10.get("stdin") else []
    cmd = ["/opt/cv1/testpy/bin/python3", "-I", "/opt/cv1/lib/runtime_probe.py", "--app", "/workspace/run",
           "--args", json.dumps(workload10.get("args", [])), "--cwd", "/workspace", "--once",
           "--timeout", str(RESOURCE_LIMIT_TIMEOUT_S)] + stdin
    rc, stdout, _ = _run_in_jail(spec, cmd, arm, limits, timeout=RESOURCE_LIMIT_TIMEOUT_S + 120,
                                 log=os.path.join(work, "resource_limit.log"))
    try:
        run = json.loads(stdout.strip().splitlines()[-1])
    except (ValueError, IndexError):
        return {"behavior": "crash", "detail": f"probe exit {rc}"}
    return dict(run, behavior=classify_exit(run))


def classify_exit(run):
    code = run.get("exit")
    if code == 0:
        return "success"
    message = (run.get("stderr_tail") or "") + (run.get("stdout_tail") or "")
    if isinstance(code, int) and 0 < code < 128 and message.strip():
        return "clean_error"
    return "crash"


def measure_runtime(layout, arm, identity, ws_src, workload, work, limits):
    """workload: {"args": [...], "stdin": file name in dir or null, "dir": host directory mounted at /workload}"""
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
                # An arm naming its own interpreter (`import "node:fs"`, `#!/usr/bin/env python3`, `exec node`) is
                # not delegation, in source files and in ./run or ./build alike; the rule is the same for every arm.
                hits = [m.group(0) for m in pattern.finditer(text)
                        if not (why == "names another interpreter" and m.group(0) in _arm_interpreters(arm))]
                if hits:
                    flags.append({"file": rel, "why": why, "match": hits[0]})
    return flags


def _arm_interpreters(arm):
    return {"python": {"python", "python3"}, "javascript": {"node"}}.get(arm, set())


def score_run(run_dir, task, layout, frozen_acceptance, heldout_dir=None, frozen_heldout=None, workload=None,
              expressibility_gap=False, limits=None, human_interventions=None, workload10=None):
    result = _read_json(os.path.join(run_dir, "result.json"))
    arm = result["arm"]
    limits = limits or result.get("limits") or {}
    work = tempfile.mkdtemp(prefix="score-", dir=run_dir)
    identity = jail.write_identity_files(os.path.join(work, "identity"))
    notes = []
    score = {"schema": "cv1-score-result/1", "run_id": result["run_id"], "task_id": result["task_id"], "arm": arm,
             "acceptance": None, "heldout": None, "heldout_fraction": None, "tampering": [], "delegation_flags": [],
             "runtime": None, "resource_limit": None, "canary_triggered": None, "notes": notes,
             "tests_verified": False,
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
        workspace_changed = current != result["final_workspace_sha256"]
        if workspace_changed:
            notes.append("workspace changed after the run ended; scoring the recorded state is impossible")
            score["tampering"].append({"what": "workspace modified after the run"})
        if result.get("acceptance_tests_sha256") != frozen_acceptance:
            score["tampering"].append({"what": "acceptance tests at run time differ from the freeze"})
        ports = canary_ports_for(task)
        if score["tests_verified"] and not workspace_changed:
            score["acceptance"] = run_suite(layout, arm, identity, ws_src, task.acceptance, work, "acceptance", limits,
                                            ports)
            if heldout_dir:
                h = run_suite(layout, arm, identity, ws_src, heldout_dir, work, "heldout", limits, ports)
                score["heldout"] = h
                score["heldout_fraction"] = (h["passed"] / h["total"]) if h["total"] else 0.0
            if workload and score["acceptance"]["all_passed"]:
                score["runtime"] = measure_runtime(layout, arm, identity, ws_src, workload, work, limits)
            if workload10 and score["acceptance"]["all_passed"]:
                score["resource_limit"] = resource_limit_behavior(layout, arm, identity, ws_src, workload10, work,
                                                                  limits)
            score["canary_triggered"] = canary_triggered([score["acceptance"], score["heldout"]])
        score["delegation_flags"] = delegation_flags(arm, ws_src)

    status = result["status"]
    workspace_changed = status != "not_started" and any(t["what"] == "workspace modified after the run"
                                                        for t in score["tampering"])
    if status == "not_started":
        outcome = "not_started"
    elif status == "infrastructure_failure" or workspace_changed:
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
