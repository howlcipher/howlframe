"""Unit tests: pricing, budget ledger, beacon draw and record schemas. No network, no jail, no model."""

import json
import os
import random
import shutil
import tempfile
import threading
import unittest

import datetime

from cv1 import beacon, feasibility, pricing
from cv1.budget import Budget, BudgetRefused
from cv1.results import SchemaError, validate_result

HERE = os.path.dirname(os.path.abspath(__file__))
PRICES = pricing.load_prices()


class Pricing(unittest.TestCase):
    def test_anthropic_cost_uses_every_token_class(self):
        usage = {"input_tokens": 1_000_000, "cache_creation_input_tokens": 1_000_000,
                 "cache_read_input_tokens": 1_000_000, "output_tokens": 1_000_000}
        # 4 input + 5 five-minute write + 0.20 read + 20 output
        self.assertAlmostEqual(pricing.call_cost(PRICES, "claude-opus-5-5", usage), 29.2)

    def test_anthropic_one_hour_cache_write(self):
        usage = {"input_tokens": 0, "cache_creation_input_tokens": 1_000_000, "cache_read_input_tokens": 0,
                 "output_tokens": 0, "cache_creation": {"ephemeral_1h_input_tokens": 1_000_000,
                                                        "ephemeral_5m_input_tokens": 0}}
        self.assertAlmostEqual(pricing.call_cost(PRICES, "claude-opus-5-5", usage), 8.0)

    def test_openai_cached_and_unknown_writes(self):
        usage = {"input_tokens": 200_000, "input_tokens_details": {"cached_tokens": 100_000},
                 "output_tokens": 100_000}
        # 100K uncached charged at the cache-write rate 2.50 (writes unknown), 100K cached at 0.10, 100K out at 10
        self.assertAlmostEqual(pricing.call_cost(PRICES, "gpt-6.1-sol", usage), 0.25 + 0.01 + 1.0)

    def test_openai_long_context_rate_applies_to_whole_call(self):
        usage = {"input_tokens": 300_000, "input_tokens_details": {"cached_tokens": 300_000, "cache_write_tokens": 0},
                 "output_tokens": 0}
        self.assertAlmostEqual(pricing.call_cost(PRICES, "gpt-6.1-sol", usage), 300_000 * 0.2 / 1e6)

    def test_worst_case_covers_any_usage_within_the_bound(self):
        rng = random.Random(7)
        for model in ("claude-opus-5-5", "gpt-6.1-sol"):
            entry = pricing.model_entry(PRICES, model)
            for _ in range(500):
                body = rng.randint(1_000, 1_200_000)
                max_out = rng.choice([None, 32_000, 128_000])
                bound = pricing.input_token_bound(entry, body)
                total_in = rng.randint(0, bound)
                out = rng.randint(0, max_out or entry["max_output_tokens"])
                if entry["provider"] == "anthropic":
                    a = rng.randint(0, total_in)
                    b = rng.randint(0, total_in - a)
                    usage = {"input_tokens": a, "cache_creation_input_tokens": b,
                             "cache_read_input_tokens": total_in - a - b, "output_tokens": out}
                else:
                    cached = rng.randint(0, total_in)
                    usage = {"input_tokens": total_in, "input_tokens_details": {"cached_tokens": cached},
                             "output_tokens": out}
                self.assertLessEqual(pricing.call_cost(PRICES, model, usage),
                                     pricing.worst_call_cost(PRICES, model, body, max_out) + 1e-9)

    def test_unknown_model_refused(self):
        with self.assertRaises(pricing.PricingError):
            pricing.worst_call_cost(PRICES, "claude-unknown", 10)


class BudgetTests(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.mkdtemp()
        self.ledger = os.path.join(self.dir, "ledger.jsonl")

    def tearDown(self):
        shutil.rmtree(self.dir)

    def test_admission_reserves_whole_ceilings(self):
        b = Budget(self.ledger, 150, 30, 150)
        for i in range(5):
            b.admit(f"r{i}")
        with self.assertRaises(BudgetRefused) as e:
            b.admit("r5")
        self.assertEqual(e.exception.reason, "experiment_cap")

    def test_run_ceiling_counts_in_flight_reservations(self):
        b = Budget(self.ledger, 150, 30, 150)
        b.admit("r")
        t1 = b.reserve("r", 12)
        b.reserve("r", 12)
        with self.assertRaises(BudgetRefused) as e:
            b.reserve("r", 12)
        self.assertEqual(e.exception.reason, "run_ceiling")
        b.settle(t1, 1.0, 1, {})
        b.reserve("r", 12)  # 1 spent + 12 in flight + 12 = 25 <= 30

    def test_spend_never_exceeds_ceiling_when_calls_cost_their_worst(self):
        b = Budget(self.ledger, 150, 30, 10_000)
        b.admit("r")
        spent = 0.0
        while True:
            try:
                t = b.reserve("r", 7.3)
            except BudgetRefused:
                break
            b.settle(t, 7.3, 0, {})
            spent += 7.3
        self.assertLessEqual(spent, 30)
        self.assertAlmostEqual(spent, 4 * 7.3)

    def test_tool_call_limit(self):
        b = Budget(self.ledger, 150, 30, 3)
        b.admit("r")
        b.settle(b.reserve("r", 0.1), 0.01, 3, {})
        with self.assertRaises(BudgetRefused) as e:
            b.reserve("r", 0.1)
        self.assertEqual(e.exception.reason, "tool_calls")

    def test_cost_above_reserved_worst_stops_experiment(self):
        b = Budget(self.ledger, 150, 30, 150)
        b.admit("r")
        self.assertTrue(b.settle(b.reserve("r", 1.0), 2.0, 0, {}))
        with self.assertRaises(BudgetRefused) as e:
            b.reserve("r", 0.1)
        self.assertEqual(e.exception.reason, "experiment_stopped")
        with self.assertRaises(BudgetRefused):
            b.admit("other")

    def test_ledger_replay_after_restart(self):
        b = Budget(self.ledger, 150, 30, 150)
        b.admit("r1")
        b.settle(b.reserve("r1", 5), 4.25, 2, {})
        b.release("r1", "done")
        b.admit("r2")
        b.settle(b.reserve("r2", 5), 1.5, 1, {})
        # r2 never released: the process "died"
        b2 = Budget(self.ledger, 150, 30, 150)
        self.assertAlmostEqual(b2.spent, 5.75)
        self.assertEqual(b2.run_tools["r1"], 2)
        self.assertEqual(b2.active, set())
        with self.assertRaises(BudgetRefused) as e:
            b2.admit("r2")
        self.assertEqual(e.exception.reason, "duplicate_run")
        with open(self.ledger) as f:
            kinds = [json.loads(line)["kind"] for line in f]
        self.assertEqual(kinds[-1], "release")

    def test_changed_limits_need_a_new_ledger(self):
        Budget(self.ledger, 150, 30, 150)
        with self.assertRaises(ValueError):
            Budget(self.ledger, 300, 30, 150)

    def test_concurrent_reservations_are_atomic(self):
        b = Budget(self.ledger, 1000, 30, 10_000)
        b.admit("r")
        ok = []

        def grab():
            try:
                ok.append(b.reserve("r", 1.0))
            except BudgetRefused:
                pass

        threads = [threading.Thread(target=grab) for _ in range(100)]
        for t in threads:
            t.start()
        for t in threads:
            t.join()
        self.assertEqual(len(ok), 30)


class Beacon(unittest.TestCase):
    def setUp(self):
        d = os.path.join(HERE, "fixtures", "beacon")
        with open(os.path.join(d, "beacon_time_1792454400000.json")) as f:
            self.at = json.load(f)
        with open(os.path.join(d, "beacon_time_next_1792454400000.json")) as f:
            self.nxt = json.load(f)

    def fake_fetch(self, url):
        return self.nxt if "/time/next/" in url else self.at

    def test_pulse_at_the_minute_is_used(self):
        pulse, url, archive = beacon.select_pulse("2026-10-20T00:00:00Z", self.fake_fetch)
        self.assertEqual(pulse["pulseIndex"], 1990929)
        self.assertTrue(url.endswith("/time/1792454400000"))
        self.assertEqual(len(archive), 1)

    def test_falls_back_to_next_pulse_when_minute_has_none(self):
        def fetch(url):
            if "/time/next/" in url:
                return self.nxt
            doc = json.loads(json.dumps(self.at))
            doc["pulse"]["timeStamp"] = "2026-10-19T23:59:00.000Z"
            return doc
        pulse, url, archive = beacon.select_pulse("2026-10-20T00:00:00Z", fetch)
        self.assertEqual(pulse["pulseIndex"], 1990930)
        self.assertEqual(len(archive), 2)

    def test_not_before_must_be_whole_minute(self):
        with self.assertRaises(ValueError):
            beacon.minute_ms("2026-10-20T00:00:30Z")

    def test_seed_is_raw_bytes_and_case_insensitive(self):
        ov = self.at["pulse"]["outputValue"]
        m = "ab" * 32
        self.assertEqual(beacon.seed_from(ov, m), beacon.seed_from(ov.lower(), m.upper()))
        import hashlib
        self.assertEqual(beacon.seed_from(ov, m), hashlib.sha256(bytes.fromhex(ov) + bytes.fromhex(m)).digest())

    def test_draw_is_deterministic_and_respects_authority_cap(self):
        manifest = {"domains": [
            {"id": d, "candidates": [{"id": f"{d}-{i}", "authority_centred": i < 3} for i in range(6)]}
            for d in ("D1", "D2", "D3", "D4")]}
        seed = beacon.seed_from(self.at["pulse"]["outputValue"], "00" * 32)
        a = beacon.draw(manifest, seed)
        b = beacon.draw(json.loads(json.dumps(manifest)), seed)
        self.assertEqual(a, b)
        chosen = {s["task"] for s in a["selected"]}
        self.assertEqual(len(a["selected"]), 4)
        self.assertLessEqual(sum(1 for c in chosen if int(c.split("-")[1]) < 3), 1)
        other = beacon.draw(manifest, beacon.seed_from(self.nxt["pulse"]["outputValue"], "00" * 32))
        self.assertNotEqual(a["shuffled_order"], other["shuffled_order"])

    def test_shuffle_is_a_permutation_independent_of_input_order(self):
        seed = b"\x01" * 32
        ids = [f"t{i}" for i in range(20)]
        s1 = beacon.shuffle(ids, seed, "D1")
        s2 = beacon.shuffle(list(reversed(ids)), seed, "D1")
        self.assertEqual(s1, s2)
        self.assertEqual(sorted(s1), sorted(ids))


class Schemas(unittest.TestCase):
    def good(self):
        return {"schema": "cv1-run-result/1", "run_id": "r", "task_id": "t", "arm": "go", "repetition": 1,
                "agent": {"provider": "anthropic", "model": "m", "effort": "high", "cli": "claude"},
                "cli_binary": {"path": "/x", "sha256": "0"}, "status": "ended_by_agent", "end_reason": "task_complete",
                "end_detail": None, "started_utc": "x", "ended_utc": "y", "elapsed_s": 1.0,
                "completion_handshake": True, "spend_usd": 0.0, "model_calls": 1, "tool_calls": 0,
                "served_models": ["m"], "first_request_surface_sha256": None, "acceptance_tests_sha256": {},
                "final_workspace_sha256": {}, "wall_clock_limit_s": 2700, "limits": {},
                "infrastructure_rerun_of": None}

    def test_valid_record(self):
        validate_result(self.good())

    def test_missing_field_and_bad_enum_rejected(self):
        r = self.good()
        del r["spend_usd"]
        with self.assertRaises(SchemaError):
            validate_result(r)
        r = self.good()
        r["status"] = "success"
        with self.assertRaises(SchemaError):
            validate_result(r)


class Authorization(unittest.TestCase):
    def write(self, **over):
        auth = {"authorized_by": "Owner", "experiment": "competitive-v1 feasibility", "date": "2026-11-01",
                "valid_through": "2026-11-07", "provider": "anthropic", "model": "claude-opus-5-5", "cap_usd": 150,
                "run_ceiling_usd": 30, "task_id": "cal-1", "statement": "I authorize..."}
        auth.update(over)
        fd, path = tempfile.mkstemp(suffix=".json")
        with os.fdopen(fd, "w") as f:
            json.dump(auth, f)
        self.addCleanup(os.unlink, path)
        return path

    def check(self, path, provider="anthropic", task="cal-1", day="2026-11-03"):
        return feasibility.check_authorization(path, provider, task, datetime.date.fromisoformat(day))

    def test_valid_authorization(self):
        self.assertEqual(self.check(self.write())["cap_usd"], 150)

    def test_refusals(self):
        cases = [({"authorized_by": "agent"}, {}), ({}, {"day": "2026-11-08"}), ({}, {"day": "2026-10-31"}),
                 ({"model": "claude-fable-5-1"}, {}), ({}, {"provider": "openai"}), ({}, {"task": "other"}),
                 ({"run_ceiling_usd": 200}, {})]
        for over, kw in cases:
            with self.assertRaises(feasibility.NotAuthorized, msg=str((over, kw))):
                self.check(self.write(**over), **kw)
        path = self.write()
        with open(path) as f:
            d = json.load(f)
        del d["statement"]
        with open(path, "w") as f:
            json.dump(d, f)
        with self.assertRaises(feasibility.NotAuthorized):
            self.check(path)


if __name__ == "__main__":
    unittest.main()
