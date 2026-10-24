"""Integration: real agent CLIs in real jails, through the proxy, against the deterministic mock provider.

No paid model call is possible here: every jail has no network except the proxy socket, and the proxy's upstream
is the local mock. Skipped unless the pinned cache (CV1_CACHE, see cv1/toolchains.py), bubblewrap and the CLI
binaries (CV1_CLAUDE_BIN, CV1_CODEX_DIR) are present.
"""

import json
import os
import shutil
import subprocess
import sys
import tempfile
import threading
import unittest

from cv1 import agents, jail, pricing, runner, scorer
from cv1.budget import Budget

HERE = os.path.dirname(os.path.abspath(__file__))
HARNESS = os.path.dirname(HERE)
sys.path.insert(0, os.path.join(HARNESS, "mock"))
import mock_upstream  # noqa: E402

CACHE = os.environ.get("CV1_CACHE", "")
CLAUDE = os.environ.get("CV1_CLAUDE_BIN", "")
CODEX_DIR = os.environ.get("CV1_CODEX_DIR", "")
TEMPLATE = os.path.join(os.path.dirname(HARNESS), "prompts", "agent_task.md")
TOY = os.path.join(HERE, "fixtures", "toy_task")
PRICES = pricing.load_prices()
REAL_KEY = "REAL-KEY-MARKER"

HAVE_JAIL = bool(CACHE and os.path.isdir(os.path.join(CACHE, "root")) and shutil.which("bwrap"))
HAVE_CLAUDE = HAVE_JAIL and os.path.isfile(CLAUDE)
HAVE_CODEX = HAVE_JAIL and os.path.isfile(os.path.join(CODEX_DIR, "codex"))

PY_SOLUTION = r"""cat > /workspace/main.py <<'EOF'
import sys
try:
    nums = [int(a) for a in sys.argv[1:]]
except ValueError:
    print("not an integer", file=sys.stderr)
    sys.exit(2)
print(sum(nums))
EOF
printf '#!/bin/sh\nexec python3 /workspace/main.py "$@"\n' > /workspace/run
chmod +x /workspace/run && run-acceptance-tests"""

JS_SOLUTION = r"""cat > /workspace/main.js <<'EOF'
const args = process.argv.slice(2);
if (!args.every((a) => /^-?\d+$/.test(a))) { console.error("not an integer"); process.exit(2); }
console.log(args.reduce((s, a) => s + Number(a), 0));
EOF
printf '#!/bin/sh\nexec node /workspace/main.js "$@"\n' > /workspace/run
chmod +x /workspace/run && run-acceptance-tests"""

GO_SOLUTION = r"""cat > /workspace/main.go <<'EOF'
package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	sum := 0
	for _, a := range os.Args[1:] {
		n, err := strconv.Atoi(a)
		if err != nil {
			fmt.Fprintln(os.Stderr, "not an integer")
			os.Exit(2)
		}
		sum += n
	}
	fmt.Println(sum)
}
EOF
printf 'module toy\n\ngo 1.25\n' > /workspace/go.mod
printf '#!/bin/sh\nset -e\ncd /workspace\ngo build -o app .\n' > /workspace/build
printf '#!/bin/sh\nexec /workspace/app "$@"\n' > /workspace/run
chmod +x /workspace/build /workspace/run && /workspace/build && run-acceptance-tests"""

# A deliberately wrong HowlFrame-arm solution, to exercise the not-completed scoring path.
HF_WRONG = r"""printf '(cli_app\n  (do 0))\n' > /workspace/main.howl
printf '#!/bin/sh\nset -e\ncd /workspace\nhowlframe build main.howl -o app.hfbc\n' > /workspace/build
printf '#!/bin/sh\nexec howlframe run /workspace/app.hfbc "$@"\n' > /workspace/run
chmod +x /workspace/build /workspace/run && /workspace/build; run-acceptance-tests; true"""


def steps(solution):
    return [{"shell": solution}, {"shell": "task-complete"}, {"text": "done"}]


class Env:
    def __init__(self, scenario, cap=150.0, ceiling=30.0, tools=150):
        self.dir = tempfile.mkdtemp(prefix="cv1-int-")
        self.record = os.path.join(self.dir, "mock.jsonl")
        self.mock = mock_upstream.make_server(scenario, self.record)
        threading.Thread(target=self.mock.serve_forever, daemon=True).start()
        self.budget = Budget(os.path.join(self.dir, "ledger.jsonl"), cap, ceiling, tools)
        self.layout = runner.Layout(CACHE, claude_binary=CLAUDE, codex_dir=CODEX_DIR)
        self.out = os.path.join(self.dir, "runs")
        os.makedirs(self.out)
        self.task = runner.Task(TOY)

    def spec(self, run_id, arm, provider, wall=120, **kw):
        model = "claude-opus-5-5" if provider == "anthropic" else "gpt-6.1-sol"
        cfg = agents.AgentConfig(provider, model, max_budget_usd=30 if provider == "anthropic" else None)
        return runner.RunSpec(run_id, self.task, arm, cfg, wall_clock_s=wall,
                              limits={"memory_max": "4G", "tasks_max": 512, "cpus": None, "use_systemd": True}, **kw)

    def kwargs(self, served=None):
        return dict(layout=self.layout, budget=self.budget, prices=PRICES, out_dir=self.out,
                    upstream_url=f"http://127.0.0.1:{self.mock.server_address[1]}", api_key=REAL_KEY,
                    template_path=TEMPLATE, allowed_served=served)

    def mock_requests(self):
        if not os.path.exists(self.record):
            return []
        with open(self.record) as f:
            return [json.loads(line) for line in f]

    def frozen(self, sub):
        return runner.tree_manifest(os.path.join(TOY, sub))

    def score(self, result):
        return scorer.score_run(os.path.join(self.out, result["run_id"]), self.task, self.layout,
                                self.frozen("acceptance"), os.path.join(TOY, "heldout"), self.frozen("heldout"),
                                workload={"args": ["1", "2", "3"]})

    def close(self):
        self.mock.shutdown()
        self.mock.server_close()
        if os.environ.get("CV1_KEEP"):
            print("kept", self.dir)
        else:
            shutil.rmtree(self.dir, ignore_errors=True)


def jail_run(arm, command, **spec_kw):
    """Run a command in an agent-style jail for `arm` without any agent; returns (exit, stdout, stderr)."""
    d = tempfile.mkdtemp(prefix="cv1-jail-")
    try:
        layout = runner.Layout(CACHE)
        ws, home, control = (os.path.join(d, x) for x in ("ws", "home", "control"))
        for x in (ws, home, control):
            os.makedirs(x)
        spec = jail.JailSpec(layout.root, jail.write_identity_files(os.path.join(d, "id")), ws,
                             toolchain=layout.toolchain(arm), docs=layout.docs(arm),
                             tests=os.path.join(TOY, "acceptance"),
                             cv1_bin=os.path.join(HARNESS, "bin"), testpy=layout.testpy, home=home, control=control,
                             extra_ro=[(os.path.join(HARNESS, "cv1"), "/opt/cv1/lib")], **spec_kw)
        env = {"PATH": jail.jail_path(), "HOME": "/home/agent", "LANG": "C.UTF-8"}
        p = subprocess.run(jail.bwrap_argv(spec, ["/bin/sh", "-c", command], env), capture_output=True, text=True,
                           timeout=120)
        return p.returncode, p.stdout, p.stderr
    finally:
        shutil.rmtree(d, ignore_errors=True)


@unittest.skipUnless(HAVE_JAIL, "needs CV1_CACHE with the pinned root and bwrap")
class JailIsolation(unittest.TestCase):
    def test_each_arm_sees_only_its_own_toolchain(self):
        expect = {"python": "python3", "go": "go", "javascript": "node", "howlframe": "howlframe"}
        for arm, own in expect.items():
            rc, out, _ = jail_run(arm, "for t in python3 python go node howlframe perl; do "
                                       "command -v $t >/dev/null && echo $t; done; true")
            found = set(out.split())
            others = set(expect.values()) - {own} - {"python"}
            self.assertIn(own, found, arm)
            self.assertFalse(found & others, f"{arm} jail exposes {found & others}")
            self.assertIn("perl", found, "perl-base is part of the pinned root in every arm (disclosed)")

    def test_python_arm_runs_the_pinned_version(self):
        rc, out, _ = jail_run("python", "python3 -c 'import sys; print(sys.version.split()[0])'")
        self.assertEqual(out.strip(), "3.12.15")

    def test_no_network_and_no_host_files(self):
        probe = ("/opt/cv1/testpy/bin/python3 -I -c \"import socket; s=socket.socket(); s.settimeout(3);\n"
                 "try:\n s.connect(('1.1.1.1', 443)); print('NET-OPEN')\n"
                 "except OSError as e:\n print('NET-BLOCKED')\"; "
                 "test -e /run/media && echo HOST-VISIBLE; test -e /home/agent/.claude && echo CLAUDE-HOME; "
                 "ls /home; true")
        rc, out, _ = jail_run("python", probe)
        self.assertIn("NET-BLOCKED", out)
        self.assertNotIn("HOST-VISIBLE", out)
        self.assertNotIn("CLAUDE-HOME", out)

    def test_tests_and_toolchain_are_read_only(self):
        rc, out, _ = jail_run("python", "echo x >> /tests/test_sum.py && echo WROTE-TESTS; "
                                        "rm -f /tests/test_sum.py && echo RM-TESTS; "
                                        "touch /opt/toolchain/x && echo WROTE-TOOLCHAIN; "
                                        "touch /usr/bin/x && echo WROTE-ROOT; true")
        for leak in ("WROTE-TESTS", "RM-TESTS", "WROTE-TOOLCHAIN", "WROTE-ROOT"):
            self.assertNotIn(leak, out)


@unittest.skipUnless(HAVE_CLAUDE, "needs CV1_CLAUDE_BIN and the pinned cache")
class ClaudeGroup(unittest.TestCase):
    def test_four_arms_co_scheduled_and_scored(self):
        env = Env({"step_sets": [
            {"match": "task in Python", "steps": steps(PY_SOLUTION)},
            {"match": "task in JavaScript", "steps": steps(JS_SOLUTION)},
            {"match": "task in Go", "steps": steps(GO_SOLUTION)},
            {"match": "task in HowlFrame", "steps": steps(HF_WRONG)}],
            "usage": {"input_tokens": 3000, "cache_read_input_tokens": 20000, "output_tokens": 400}})
        try:
            specs = [env.spec(f"g-{arm}", arm, "anthropic") for arm in runner.ARMS]
            results, errors = runner.run_group(specs, **env.kwargs())
            self.assertEqual(errors, [None] * 4)
            by_arm = {r["arm"]: r for r in results}
            for arm, r in by_arm.items():
                self.assertEqual(r["end_reason"], "task_complete", (arm, r.get("end_detail")))
                self.assertTrue(r["completion_handshake"])
                self.assertGreater(r["spend_usd"], 0)
                self.assertEqual(r["served_models"], ["claude-opus-5-5"])
            self.assertEqual(runner.audit_surfaces(results), [], "arms of one provider must share one agent surface")
            reqs = env.mock_requests()
            self.assertTrue(reqs and all(q["auth_is_real_marker"] for q in reqs))
            for arm in runner.ARMS:
                run_dir = os.path.join(env.out, f"g-{arm}")
                for root, _, files in os.walk(os.path.join(run_dir, "home")):
                    for f in files:
                        with open(os.path.join(root, f), "rb") as fh:
                            self.assertNotIn(REAL_KEY.encode(), fh.read(), "the real key must never enter a jail")
            total = sum(r["spend_usd"] for r in results)
            self.assertAlmostEqual(env.budget.snapshot()["spent_usd"], total, places=6)
            scores = {arm: env.score(by_arm[arm]) for arm in runner.ARMS}
            for arm in ("python", "javascript", "go"):
                self.assertEqual(scores[arm]["outcome"], "completed", (arm, scores[arm]["acceptance"]))
                self.assertEqual(scores[arm]["heldout_fraction"], 1.0, arm)
                self.assertIsNotNone(scores[arm]["runtime"]["execution_s"], arm)
                self.assertEqual(scores[arm]["delegation_flags"], [], arm)
            self.assertEqual(scores["howlframe"]["outcome"], "not_completed_acceptance")
        finally:
            env.close()

    def test_run_ceiling_ends_the_run_as_budget_exhausted(self):
        env = Env({"steps": [{"shell": "echo working"}] * 40,
                   "usage": {"input_tokens": 2000, "output_tokens": 100_000}}, cap=60, ceiling=8)
        try:
            r = runner.run_one(env.spec("budget", "python", "anthropic"), **env.kwargs())
            self.assertEqual(r["status"], "not_completed_budget", r.get("end_detail"))
            self.assertLessEqual(r["spend_usd"], 8)
            self.assertLessEqual(env.budget.snapshot()["spent_usd"], 8)
            s = env.score(r)
            self.assertEqual(s["outcome"], "not_completed_budget")
        finally:
            env.close()

    def test_wall_clock_limit(self):
        env = Env({"steps": steps(PY_SOLUTION), "delay_seconds": 30})
        try:
            r = runner.run_one(env.spec("slow", "python", "anthropic", wall=5), **env.kwargs())
            self.assertEqual(r["status"], "not_completed_time")
            self.assertLess(r["elapsed_s"], 20)
        finally:
            env.close()

    def test_served_model_mismatch_is_an_infrastructure_failure(self):
        env = Env({"steps": steps(PY_SOLUTION), "served_model": "claude-sonnet-5-5"})
        try:
            r = runner.run_one(env.spec("mismatch", "python", "anthropic"), **env.kwargs())
            self.assertEqual(r["status"], "infrastructure_failure")
            self.assertEqual(env.score(r)["outcome"], "infrastructure_failure")
        finally:
            env.close()

    def test_admission_refusal_is_recorded_not_dropped(self):
        env = Env({"steps": steps(PY_SOLUTION), "delay_seconds": 2}, cap=30, ceiling=30)
        try:
            results, errors = runner.run_group([env.spec("a1", "python", "anthropic"),
                                                env.spec("a2", "go", "anthropic")], **env.kwargs())
            statuses = sorted(r["status"] for r in results)
            self.assertEqual(statuses, ["ended_by_agent", "not_started"])
            self.assertTrue(all(os.path.exists(os.path.join(env.out, r["run_id"], "result.json")) for r in results))
        finally:
            env.close()


@unittest.skipUnless(HAVE_CODEX, "needs CV1_CODEX_DIR and the pinned cache")
class CodexRun(unittest.TestCase):
    def test_codex_completes_and_is_scored(self):
        env = Env({"steps": steps(JS_SOLUTION),
                   "usage": {"input_tokens": 30000, "cached_tokens": 20000, "output_tokens": 500}})
        try:
            r = runner.run_one(env.spec("cx-js", "javascript", "openai"), **env.kwargs())
            self.assertEqual(r["end_reason"], "task_complete", r.get("end_detail"))
            self.assertEqual(r["served_models"], ["gpt-6.1-sol"])
            self.assertGreater(r["spend_usd"], 0)
            self.assertTrue(all(q["auth_is_real_marker"] for q in env.mock_requests()))
            self.assertEqual(env.score(r)["outcome"], "completed")
        finally:
            env.close()

    def test_codex_surface_is_identical_across_arms(self):
        env = Env({"step_sets": [{"match": "task in Python", "steps": steps(PY_SOLUTION)},
                                 {"match": "task in JavaScript", "steps": steps(JS_SOLUTION)}]})
        try:
            results, errors = runner.run_group([env.spec("cs-py", "python", "openai"),
                                                env.spec("cs-js", "javascript", "openai")], **env.kwargs())
            self.assertEqual(errors, [None, None])
            self.assertEqual(runner.audit_surfaces(results), [])
        finally:
            env.close()


if __name__ == "__main__":
    unittest.main()
