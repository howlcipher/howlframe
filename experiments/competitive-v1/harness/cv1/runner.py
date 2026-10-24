"""Launch, supervise and record agent runs (protocol §5; Revision 3 G1, G2, G7, G8).

A run is one agent CLI session on one task in one language. The runner admits it against the budget, starts
its proxy, launches the CLI inside a fresh jail and ends the run on the first of:

* the agent running `task-complete` (outcome decided later by scoring),
* the wall-clock limit (not completed: time),
* the proxy refusing a call on the run ceiling, the experiment cap or the tool-call limit (not completed:
  budget or tool calls),
* the CLI exiting by itself (outcome decided by scoring; no completion handshake),
* an infrastructure fault: the jail or forwarder failing to start, a served-model mismatch, a refused
  server-side tool (a configuration error), or a cost-bound breach.

The four language arms of one (task, provider, repetition) are launched together by `run_group`.
Every run produces `result.json` (validated against `schema/run_result.schema.json`) and its raw records.
"""

import hashlib
import json
import os
import shutil
import signal
import subprocess
import tempfile
import threading
import time

from . import agents, jail
from .budget import BudgetRefused
from .proxy import RunProxy, wait_for_socket
from .results import validate_result

ARMS = ("howlframe", "python", "go", "javascript")
BUDGET_REASONS = {"run_ceiling": "budget", "experiment_cap": "budget", "experiment_stopped": "budget",
                  "tool_calls": "tool_calls"}
INFRA_REASONS = {"server_tool", "wrong_model", "speed_tier", "inference_geo", "background_mode", "route", "body",
                 "run_blocked", "run_not_admitted"}
HARNESS = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for block in iter(lambda: f.read(1 << 20), b""):
            h.update(block)
    return h.hexdigest()


def tree_manifest(root):
    """{relative path: sha256} for regular files, symlinks recorded by target."""
    out = {}
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames.sort()
        for name in sorted(filenames):
            full = os.path.join(dirpath, name)
            rel = os.path.relpath(full, root)
            if os.path.islink(full):
                out[rel] = "link:" + os.readlink(full)
            elif os.path.isfile(full):
                out[rel] = sha256_file(full)
    return out


class Layout:
    """Where the pinned artifacts live (see toolchains.py)."""

    def __init__(self, cache, claude_binary=None, codex_dir=None):
        self.cache = os.path.abspath(cache)
        self.root = os.path.join(self.cache, "root")
        self.testpy = os.path.join(self.cache, "toolchains", "python")
        self.claude_binary = claude_binary
        self.codex_dir = codex_dir

    def toolchain(self, arm):
        return os.path.join(self.cache, "toolchains", arm)

    def docs(self, arm):
        return os.path.join(self.cache, "docs", arm)


class Task:
    """A task directory: prompt.md, acceptance/ (visible tests), optional baseline/<arm>/ (D4) and task.json."""

    def __init__(self, path):
        self.path = os.path.abspath(path)
        with open(os.path.join(self.path, "task.json")) as f:
            self.meta = json.load(f)
        self.id = self.meta["id"]
        self.acceptance = os.path.join(self.path, "acceptance")

    def prompt_body(self):
        with open(os.path.join(self.path, "prompt.md")) as f:
            return f.read()

    def baseline(self, arm):
        p = os.path.join(self.path, "baseline", arm)
        return p if os.path.isdir(p) else None


LANGUAGE_FACTS = {
    "howlframe": {"name": "HowlFrame", "toolchain": "the HowlFrame compiler (`howlframe`; run `howlframe --help`)",
                  "docs": "/opt/docs (README.md, docs/cli.md, docs/reference/, SKILL.md)"},
    "python": {"name": "Python", "toolchain": "CPython 3.12 (`python3`), standard library only",
               "docs": "/opt/docs (the Python 3.12 library reference as text) and `pydoc`"},
    "go": {"name": "Go", "toolchain": "Go 1.25 (`go`), standard library only",
           "docs": "/opt/docs and `go doc`"},
    "javascript": {"name": "JavaScript", "toolchain": "Node.js 22 (`node`), built-in modules only",
                   "docs": "/opt/docs (the Node.js 22 API documentation as Markdown)"},
}


def render_prompt(template_path, task, arm):
    with open(template_path) as f:
        template = f.read()
    facts = LANGUAGE_FACTS[arm]
    return (template.replace("{{LANGUAGE}}", facts["name"]).replace("{{TOOLCHAIN}}", facts["toolchain"])
            .replace("{{DOCS}}", facts["docs"]).replace("{{TASK}}", task.prompt_body().strip()))


class RunSpec:
    def __init__(self, run_id, task, arm, agent_cfg, repetition=1, wall_clock_s=45 * 60, limits=None,
                 infrastructure_rerun_of=None):
        if arm not in ARMS:
            raise ValueError(arm)
        self.run_id = run_id
        self.task = task
        self.arm = arm
        self.agent = agent_cfg
        self.repetition = repetition
        self.wall_clock_s = wall_clock_s
        self.limits = limits or {"memory_max": "8G", "tasks_max": 1024, "cpus": None, "use_systemd": True}
        self.infrastructure_rerun_of = infrastructure_rerun_of


class _Stop:
    def __init__(self):
        self.reason = None
        self.detail = None
        self.event = threading.Event()

    def set(self, reason, detail=None):
        if not self.event.is_set():
            self.reason, self.detail = reason, detail
            self.event.set()


def _kill_group(proc, grace=5.0):
    if proc.poll() is not None:
        return
    try:
        os.killpg(proc.pid, signal.SIGTERM)
    except ProcessLookupError:
        return
    try:
        proc.wait(grace)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(proc.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        proc.wait()


def _binary_version(cfg, layout):
    path = layout.claude_binary if cfg.provider == "anthropic" else os.path.join(layout.codex_dir, "codex")
    return {"path": os.path.realpath(path), "sha256": sha256_file(path)}


def run_one(spec, layout, budget, prices, out_dir, upstream_url, api_key, template_path, allowed_served=None,
            socket_root=None):
    run_dir = os.path.join(out_dir, spec.run_id)
    if os.path.exists(run_dir):
        raise FileExistsError(run_dir)
    ws, home, control, records = (os.path.join(run_dir, d) for d in ("workspace", "home", "control", "records"))
    for d in (ws, home, control, records):
        os.makedirs(d)
    baseline = spec.task.baseline(spec.arm)
    if baseline:
        shutil.copytree(baseline, ws, dirs_exist_ok=True)
    agents.prepare_home(spec.agent, home)
    identity = jail.write_identity_files(os.path.join(run_dir, "identity"))
    prompt = render_prompt(template_path, spec.task, spec.arm)
    with open(os.path.join(records, "prompt.txt"), "w") as f:
        f.write(prompt)

    result = {
        "schema": "cv1-run-result/1", "run_id": spec.run_id, "task_id": spec.task.id, "arm": spec.arm,
        "repetition": spec.repetition, "agent": spec.agent.describe(),
        "cli_binary": _binary_version(spec.agent, layout), "wall_clock_limit_s": spec.wall_clock_s,
        "limits": spec.limits, "infrastructure_rerun_of": spec.infrastructure_rerun_of,
        "acceptance_tests_sha256": tree_manifest(spec.task.acceptance), "baseline_used": bool(baseline),
    }

    try:
        budget.admit(spec.run_id)
    except BudgetRefused as e:
        result.update(status="not_started", end_reason="admission_refused", end_detail=str(e), started_utc=None,
                      ended_utc=None, elapsed_s=0.0, completion_handshake=False, spend_usd=0.0, model_calls=0,
                      tool_calls=0, served_models=[], first_request_surface_sha256=None,
                      final_workspace_sha256={})
        _write_result(run_dir, result)
        return result

    stop = _Stop()

    def on_event(kind, info):
        if kind == "refused":
            reason = info["reason"]
            if reason in BUDGET_REASONS:
                stop.set("not_completed_" + BUDGET_REASONS[reason], info.get("detail"))
            elif reason in INFRA_REASONS:
                stop.set("infrastructure_failure", f"proxy refused: {reason} {info.get('detail', '')}".strip())
        elif kind in ("served_model_mismatch", "bound_exceeded"):
            stop.set("infrastructure_failure", kind)

    sock_dir = tempfile.mkdtemp(prefix="cv1-", dir=socket_root or os.environ.get("XDG_RUNTIME_DIR") or "/tmp")
    os.chmod(sock_dir, 0o755)
    proxy = RunProxy(spec.run_id, spec.agent.provider, spec.agent.model, budget, prices, upstream_url, api_key,
                     os.path.join(sock_dir, "proxy.sock"), records, allowed_served_models=allowed_served,
                     on_event=on_event, complete_marker=os.path.join(control, "complete")).start()
    wait_for_socket(proxy.socket_path)

    extra = [(os.path.join(HARNESS, "cv1"), "/opt/cv1/lib")]
    if spec.agent.provider == "anthropic":
        extra.append((layout.claude_binary, "/opt/agent/claude"))
        agent_dir = None
    else:
        agent_dir = layout.codex_dir
    jspec = jail.JailSpec(layout.root, identity, ws, toolchain=layout.toolchain(spec.arm), docs=layout.docs(spec.arm),
                          tests=spec.task.acceptance, agent_dir=agent_dir, cv1_bin=os.path.join(HARNESS, "bin"),
                          testpy=layout.testpy, home=home, control=control, proxy_socket_dir=sock_dir,
                          extra_ro=extra)
    base_env = {"PATH": jail.jail_path(), "LANG": "C.UTF-8", "TERM": "dumb", "CV1_ARM": spec.arm,
                "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOFLAGS": "-mod=mod", "GOCACHE": "/home/agent/.cache/go",
                "GOPATH": "/home/agent/go", "PIP_NO_INDEX": "1", "npm_config_offline": "true"}
    env = agents.agent_env(spec.agent, base_env)
    command = ["/opt/cv1/bin/jail-entry"] + agents.agent_command(spec.agent, prompt)
    argv = jail.limits_prefix(spec.limits.get("memory_max", "8G"), spec.limits.get("tasks_max", 1024),
                              spec.limits.get("cpus"), spec.limits.get("use_systemd", True))
    argv += jail.bwrap_argv(jspec, command, env)
    with open(os.path.join(records, "jail_argv.json"), "w") as f:
        json.dump([a if a != prompt else "<prompt.txt>" for a in argv], f, indent=1)

    started = time.time()
    result["started_utc"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(started))
    marker = os.path.join(control, "complete")
    with open(os.path.join(records, "transcript.jsonl"), "wb") as out, \
            open(os.path.join(records, "stderr.txt"), "wb") as err:
        proc = subprocess.Popen(argv, stdin=subprocess.DEVNULL, stdout=out, stderr=err, start_new_session=True)
        completed_at = None
        while True:
            if os.path.exists(marker):
                completed_at = time.time()
                stop.set("task_complete")
            if stop.event.is_set():
                break
            if proc.poll() is not None:
                stop.set("agent_exited", f"exit code {proc.returncode}")
                break
            if time.time() - started >= spec.wall_clock_s:
                stop.set("not_completed_time", f"{spec.wall_clock_s} s wall clock")
                break
            stop.event.wait(0.25)
        _kill_group(proc)
    ended = time.time()
    proxy.close()
    shutil.rmtree(sock_dir, ignore_errors=True)
    if completed_at is None and os.path.exists(marker):
        completed_at = os.path.getmtime(marker)
        if stop.reason == "agent_exited":
            stop.reason, stop.detail = "task_complete", "marker written before the CLI exited"
    if stop.reason == "agent_exited" and proc.returncode == 97:
        stop.reason, stop.detail = "infrastructure_failure", "jail entry failed (forwarder)"
    budget.release(spec.run_id, stop.reason)

    snap = budget.snapshot(spec.run_id)
    served = sorted({c.get("served_model") for c in _read_jsonl(os.path.join(records, "proxy_calls.jsonl"))
                     if c.get("served_model")})
    surface = os.path.join(records, "first_request_surface.sha256")
    status = {"task_complete": "ended_by_agent", "agent_exited": "ended_by_agent"}.get(stop.reason, stop.reason)
    result.update(
        status=status, end_reason=stop.reason, end_detail=stop.detail,
        ended_utc=time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(ended)), elapsed_s=round(ended - started, 3),
        completion_handshake=completed_at is not None,
        time_to_complete_s=round(completed_at - started, 3) if completed_at else None,
        spend_usd=snap["run_spent_usd"], model_calls=proxy.calls, tool_calls=snap["run_tool_calls"],
        served_models=served,
        first_request_surface_sha256=_read_text(surface).strip() if os.path.exists(surface) else None,
        final_workspace_sha256=tree_manifest(ws),
    )
    _write_result(run_dir, result)
    return result


def _read_text(path):
    with open(path) as f:
        return f.read()


def _read_jsonl(path):
    if not os.path.exists(path):
        return []
    with open(path) as f:
        return [json.loads(line) for line in f if line.strip()]


def _write_result(run_dir, result):
    validate_result(result)
    with open(os.path.join(run_dir, "result.json"), "w") as f:
        json.dump(result, f, indent=2, sort_keys=True)
        f.write("\n")


def run_group(specs, **kwargs):
    """Launch the given runs together (the four arms of one task, provider and repetition) and wait for all."""
    results = [None] * len(specs)
    errors = [None] * len(specs)

    def work(i, s):
        try:
            results[i] = run_one(s, **kwargs)
        except Exception as e:  # recorded, never dropped (protocol §7.9)
            errors[i] = f"{type(e).__name__}: {e}"

    threads = [threading.Thread(target=work, args=(i, s)) for i, s in enumerate(specs)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    return results, errors


def audit_surfaces(results):
    """Revision 3 G7: within one provider, the first request's system prompt and tool list must be identical
    across arms apart from the task prompt. Returns the arms whose surface differs from the majority."""
    by_hash = {}
    for r in results:
        if r and r.get("first_request_surface_sha256"):
            by_hash.setdefault(r["first_request_surface_sha256"], []).append(r["arm"])
    if len(by_hash) <= 1:
        return []
    majority = max(by_hash.values(), key=len)
    return sorted(a for arms in by_hash.values() if arms is not majority for a in arms)
