"""Proxy against the deterministic mock upstream: billing, caps, policy refusals, key handling, restart.

Every request goes to a local mock; no provider is contacted.
"""

import http.client
import json
import os
import shutil
import socket
import sys
import tempfile
import threading
import unittest

from cv1 import pricing
from cv1.budget import Budget
from cv1.proxy import RunProxy
from cv1.runner import short_socket_root

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(os.path.dirname(HERE), "mock"))
import mock_upstream  # noqa: E402

PRICES = pricing.load_prices()
REAL_KEY = "REAL-KEY-MARKER"


class UnixHTTPConnection(http.client.HTTPConnection):
    def __init__(self, path):
        super().__init__("localhost", timeout=30)
        self.path = path

    def connect(self):
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.connect(self.path)


def short_tmp():
    return tempfile.mkdtemp(prefix="cv1t-", dir=short_socket_root())


class ProxyHarness:
    def __init__(self, provider, scenario, cap=150, ceiling=30, tools=150, served=None):
        self.dir = short_tmp()
        self.record = os.path.join(self.dir, "mock.jsonl")
        self.server = mock_upstream.make_server(scenario, self.record)
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        self.provider = provider
        self.model = "claude-opus-5-5" if provider == "anthropic" else "gpt-6.1-sol"
        self.ledger = os.path.join(self.dir, "ledger.jsonl")
        self.budget = Budget(self.ledger, cap, ceiling, tools)
        self.events = []
        self.served = served
        self.proxies = []

    def proxy(self, run="r1", admit=True, marker=None):
        if admit:
            self.budget.admit(run)
        upstream = f"http://127.0.0.1:{self.server.server_address[1]}"
        p = RunProxy(run, self.provider, self.model, self.budget, PRICES, upstream, REAL_KEY,
                     os.path.join(self.dir, f"{run}.sock"), os.path.join(self.dir, run),
                     allowed_served_models=self.served, on_event=lambda k, i: self.events.append((k, i)),
                     complete_marker=marker).start()
        self.proxies.append(p)
        return p

    def call(self, p, body=None, path=None, stream=True, headers=None):
        if body is None:
            body = self.default_body(stream)
        path = path or ("/v1/messages?beta=true" if self.provider == "anthropic" else "/v1/responses")
        c = UnixHTTPConnection(p.socket_path)
        h = {"Content-Type": "application/json", "x-api-key": "dummy", "Authorization": "Bearer dummy"}
        h.update(headers or {})
        c.request("POST", path, body=json.dumps(body).encode(), headers=h)
        r = c.getresponse()
        data = r.read()
        c.close()
        return r.status, data

    def default_body(self, stream=True):
        if self.provider == "anthropic":
            return {"model": self.model, "max_tokens": 1000, "stream": stream,
                    "tools": [{"name": "Bash", "input_schema": {"type": "object"}}],
                    "messages": [{"role": "user", "content": "hi"}]}
        return {"model": self.model, "stream": stream, "input": [
            {"type": "additional_tools", "role": "developer",
             "tools": [{"type": "namespace", "name": "functions", "tools": [{"type": "custom", "name": "exec"}]}]},
            {"type": "message", "role": "user", "content": [{"type": "input_text", "text": "hi"}]}]}

    def mock_requests(self):
        if not os.path.exists(self.record):
            return []
        with open(self.record) as f:
            return [json.loads(line) for line in f]

    def close(self):
        for p in self.proxies:
            p.close()
        self.server.shutdown()
        self.server.server_close()
        shutil.rmtree(self.dir, ignore_errors=True)


SHELL_STEPS = [{"shell": "echo 1"}, {"shell": "echo 2"}, {"text": "done"}]


class Billing(unittest.TestCase):
    def run_billing(self, provider, stream):
        usage = {"input_tokens": 10_000, "cache_creation_input_tokens": 2_000, "cache_read_input_tokens": 50_000,
                 "output_tokens": 1_500, "cached_tokens": 5_000}
        h = ProxyHarness(provider, {"steps": SHELL_STEPS, "usage": usage})
        try:
            p = h.proxy()
            status, data = h.call(p, stream=stream)
            self.assertEqual(status, 200)
            if provider == "anthropic":
                expected = pricing.call_cost(PRICES, h.model, {k: usage[k] for k in (
                    "input_tokens", "cache_creation_input_tokens", "cache_read_input_tokens", "output_tokens")})
            else:
                expected = pricing.call_cost(PRICES, h.model, {
                    "input_tokens": 10_000, "input_tokens_details": {"cached_tokens": 5_000}, "output_tokens": 1_500})
            snap = h.budget.snapshot("r1")
            self.assertAlmostEqual(snap["run_spent_usd"], expected, places=6)
            self.assertEqual(snap["run_tool_calls"], 1)
            req = h.mock_requests()[0]
            self.assertTrue(req["auth_is_real_marker"], "proxy must replace the dummy credential with the real key")
            with open(os.path.join(p.record_dir, "proxy_calls.jsonl")) as f:
                rec = json.loads(f.readline())
            self.assertEqual(rec["served_model"], h.model)
            self.assertEqual(rec["cost_basis"], "usage")
            self.assertTrue(os.path.exists(os.path.join(p.record_dir, "first_request_surface.sha256")))
        finally:
            h.close()

    def test_anthropic_stream(self):
        self.run_billing("anthropic", True)

    def test_anthropic_json(self):
        self.run_billing("anthropic", False)

    def test_openai_stream(self):
        self.run_billing("openai", True)

    def test_openai_json(self):
        self.run_billing("openai", False)


class Enforcement(unittest.TestCase):
    def test_run_ceiling_stops_a_run_before_overshoot(self):
        # Each call really costs ~$2.4 (120K output at $20); the worst case per call is ~$2.6.
        h = ProxyHarness("anthropic", {"steps": [{"shell": "x"}] * 50,
                                       "usage": {"input_tokens": 100, "output_tokens": 120_000}}, ceiling=10)
        try:
            p = h.proxy()
            body = dict(h.default_body(), max_tokens=128_000)
            worst = pricing.worst_call_cost(PRICES, h.model, len(json.dumps(body).encode()), 128_000)
            per_call = pricing.call_cost(PRICES, h.model, {"input_tokens": 100, "output_tokens": 120_000})
            allowed = 0
            while allowed * per_call + worst <= 10:
                allowed += 1
            statuses = [h.call(p, body=body)[0] for _ in range(allowed + 2)]
            self.assertEqual(statuses, [200] * allowed + [400, 400])
            self.assertLessEqual(h.budget.snapshot("r1")["run_spent_usd"], 10)
            self.assertIn("run_ceiling", [i.get("reason") for k, i in h.events if k == "refused"])
            self.assertEqual(len(h.mock_requests()), allowed, "refused calls must never reach the provider")
        finally:
            h.close()

    def test_experiment_cap_stops_the_queue(self):
        h = ProxyHarness("anthropic", {"steps": SHELL_STEPS, "usage": {"output_tokens": 10}}, cap=60, ceiling=30)
        try:
            h.budget.admit("a")
            h.budget.admit("b")
            from cv1.budget import BudgetRefused
            with self.assertRaises(BudgetRefused):
                h.budget.admit("c")
        finally:
            h.close()

    def test_tool_call_limit_refuses_next_call(self):
        h = ProxyHarness("openai", {"steps": [{"shell": "x"}] * 9, "usage": {"output_tokens": 10}}, tools=2)
        try:
            p = h.proxy()
            self.assertEqual([h.call(p)[0] for _ in range(3)], [200, 200, 400])
            self.assertIn("tool_calls", [i.get("reason") for k, i in h.events if k == "refused"])
        finally:
            h.close()

    def test_policy_refusals_never_reach_provider(self):
        h = ProxyHarness("anthropic", {"steps": SHELL_STEPS})
        try:
            p = h.proxy()
            web = dict(h.default_body(), tools=[{"type": "web_search_20260209", "name": "web_search"}])
            self.assertEqual(h.call(p, body=web)[0], 400)
            wrong = dict(h.default_body(), model="claude-fable-5-1")
            self.assertEqual(h.call(p, body=wrong)[0], 400)
            mcp = dict(h.default_body(), mcp_servers=[{"type": "url", "url": "https://example.invalid"}])
            self.assertEqual(h.call(p, body=mcp)[0], 400)
            fast = dict(h.default_body(), speed="fast")
            self.assertEqual(h.call(p, body=fast)[0], 400)
            self.assertEqual(h.call(p, path="/v1/files")[0], 400)
            self.assertEqual(h.call(p, headers={"Content-Encoding": "zstd"})[0], 400)
            self.assertEqual(h.mock_requests(), [])
            reasons = [i["reason"] for k, i in h.events if k == "refused"]
            self.assertEqual(reasons, ["server_tool", "wrong_model", "server_tool", "speed_tier", "route", "body"])
        finally:
            h.close()

    def test_openai_server_tools_and_background_refused(self):
        h = ProxyHarness("openai", {"steps": SHELL_STEPS})
        try:
            p = h.proxy()
            body = h.default_body()
            body["input"][0]["tools"].append({"type": "web_search"})
            self.assertEqual(h.call(p, body=body)[0], 400)
            self.assertEqual(h.call(p, body=dict(h.default_body(), background=True))[0], 400)
            self.assertEqual(h.call(p, body=dict(h.default_body(), service_tier="priority"))[0], 400)
            self.assertEqual(h.mock_requests(), [])
        finally:
            h.close()

    def test_served_model_mismatch_blocks_the_run(self):
        h = ProxyHarness("openai", {"steps": SHELL_STEPS, "served_model": "gpt-6-luna"})
        try:
            p = h.proxy()
            self.assertEqual(h.call(p)[0], 200)
            self.assertIn("served_model_mismatch", [k for k, _ in h.events])
            self.assertEqual(h.call(p)[0], 400)
            self.assertEqual(len(h.mock_requests()), 1)
        finally:
            h.close()

    def test_dated_snapshot_can_be_allowed(self):
        h = ProxyHarness("openai", {"steps": SHELL_STEPS, "served_model": "gpt-6.1-sol-2026-09-30"},
                         served=["gpt-6.1-sol", "gpt-6.1-sol-2026-09-30"])
        try:
            p = h.proxy()
            self.assertEqual([h.call(p)[0] for _ in range(2)], [200, 200])
            self.assertNotIn("served_model_mismatch", [k for k, _ in h.events])
        finally:
            h.close()

    def test_calls_after_task_complete_are_refused_unpaid(self):
        h = ProxyHarness("anthropic", {"steps": SHELL_STEPS})
        try:
            marker = os.path.join(h.dir, "complete")
            p = h.proxy(marker=marker)
            self.assertEqual(h.call(p)[0], 200)
            spent = h.budget.snapshot("r1")["run_spent_usd"]
            open(marker, "w").close()
            self.assertEqual(h.call(p)[0], 400)
            self.assertEqual(h.budget.snapshot("r1")["run_spent_usd"], spent)
            self.assertEqual(len(h.mock_requests()), 1)
        finally:
            h.close()

    def test_unadmitted_run_is_refused(self):
        h = ProxyHarness("anthropic", {"steps": SHELL_STEPS})
        try:
            p = h.proxy(admit=False)
            self.assertEqual(h.call(p)[0], 400)
            self.assertEqual(h.mock_requests(), [])
        finally:
            h.close()

    def test_ledger_survives_proxy_restart(self):
        h = ProxyHarness("anthropic", {"steps": SHELL_STEPS, "usage": {"output_tokens": 900}})
        try:
            p = h.proxy()
            h.call(p)
            h.call(p)
            spent = h.budget.snapshot()["spent_usd"]
            p.close()
            h.proxies.remove(p)
            h.budget = Budget(h.ledger, 150, 30, 150)
            self.assertAlmostEqual(h.budget.snapshot()["spent_usd"], spent, places=6)
            p2 = h.proxy("r2")
            h.call(p2)
            self.assertGreater(h.budget.snapshot()["spent_usd"], spent)
        finally:
            h.close()

    def test_usage_above_the_worst_case_stops_the_experiment(self):
        # A real provider cannot return more output than max_tokens; if usage ever exceeds the bound, the bound's
        # assumptions are wrong, so the proxy stops every run rather than keep spending on a broken estimate.
        h = ProxyHarness("anthropic", {"steps": SHELL_STEPS, "usage": {"output_tokens": 50_000}})
        try:
            p = h.proxy()
            self.assertEqual(h.call(p)[0], 200)
            self.assertIn("bound_exceeded", [k for k, _ in h.events])
            self.assertEqual(h.call(p)[0], 400)
            self.assertIsNotNone(h.budget.snapshot()["stopped"])
        finally:
            h.close()

    def test_unpriceable_usage_still_settles_at_worst_case(self):
        h = ProxyHarness("anthropic", {"steps": SHELL_STEPS,
                                       "usage": {"input_tokens": "lots", "output_tokens": 10}})
        try:
            p = h.proxy()
            self.assertEqual(h.call(p)[0], 200)
            with open(os.path.join(p.record_dir, "proxy_calls.jsonl")) as f:
                rec = json.loads(f.readline())
            self.assertTrue(rec["cost_basis"].startswith("unpriceable usage"))
            self.assertAlmostEqual(rec["cost_usd"], rec["worst_usd"], places=6)
            self.assertEqual(h.budget.run_pending.get("r1", 0.0), 0.0, "the reservation must be released")
        finally:
            h.close()

    def test_incomplete_stream_is_charged_its_worst_case(self):
        class Cut(mock_upstream.Handler):
            def _sse(self, event, data):
                if event == "message_delta":
                    raise ConnectionResetError("cut")
                super()._sse(event, data)
        h = ProxyHarness("anthropic", {"steps": SHELL_STEPS, "usage": {"output_tokens": 10}})
        try:
            h.server.RequestHandlerClass = type("CutBound", (Cut,), {"state": h.server.RequestHandlerClass.state})
            p = h.proxy()
            h.call(p)
            with open(os.path.join(p.record_dir, "proxy_calls.jsonl")) as f:
                rec = json.loads(f.readline())
            self.assertTrue(rec["cost_basis"].startswith("usage missing or incomplete"))
            self.assertAlmostEqual(rec["cost_usd"], rec["worst_usd"], places=6)
        finally:
            h.close()


if __name__ == "__main__":
    unittest.main()
