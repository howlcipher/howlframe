"""Unit tests: pricing, budget ledger, beacon draw, record schemas and task layout. No network, no jail, no model."""

import json
import os
import random
import shutil
import socket
import tempfile
import threading
import unittest

import datetime
import hashlib

from cv1 import beacon, check_task, feasibility, pricing, runner, scorer, testrunner, toolchains
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

    def test_call_in_flight_at_a_crash_is_charged_its_worst_case(self):
        b = Budget(self.ledger, 150, 30, 150)
        b.admit("r1")
        b.settle(b.reserve("r1", 5), 1.0, 0, {})
        b.reserve("r1", 7.5)  # forwarded, then the process dies before settling
        b2 = Budget(self.ledger, 150, 30, 150)
        self.assertAlmostEqual(b2.spent, 8.5)
        self.assertAlmostEqual(b2.run_spent["r1"], 8.5)
        b3 = Budget(self.ledger, 150, 30, 150)  # replaying again does not double-charge
        self.assertAlmostEqual(b3.spent, 8.5)

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


class Delegation(unittest.TestCase):
    def scan(self, arm, files):
        from cv1 import scorer
        d = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, d)
        for name, text in files.items():
            with open(os.path.join(d, name), "w") as f:
                f.write(text)
        return scorer.delegation_flags(arm, d)

    def test_own_interpreter_is_never_flagged_in_any_arm(self):
        self.assertEqual(self.scan("javascript", {"main.js": 'import fs from "node:fs";\nrequire("node:path");\n',
                                                  "run": "#!/bin/sh\nexec node main.js\n"}), [])
        self.assertEqual(self.scan("python", {"main.py": "#!/usr/bin/env python3\n# python\n",
                                              "run": "#!/bin/sh\nexec python3 main.py\n"}), [])

    def test_other_interpreters_and_languages_are_flagged(self):
        flags = self.scan("howlframe", {"main.howl": '(cli_app (do (exec "python3" "x.py")))\n', "x.py": "print(1)\n"})
        whys = sorted(f["why"] for f in flags)
        self.assertEqual(whys, ["names another interpreter", "source file in another language (.py)"])
        self.assertEqual(self.scan("javascript", {"main.js": 'spawn("python3", ["x"])\n'})[0]["match"], "python3")


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
                 ({"run_ceiling_usd": 200}, {}), ({"cap_usd": 500, "run_ceiling_usd": 30}, {}),
                 ({"cap_usd": 150, "run_ceiling_usd": 31}, {})]
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


class TaskLayout(unittest.TestCase):
    def make(self, meta, files):
        d = tempfile.mkdtemp(prefix="cv1-task-")
        self.addCleanup(shutil.rmtree, d)
        with open(os.path.join(d, "task.json"), "w") as f:
            json.dump(meta, f)
        for rel, text in files.items():
            path = os.path.join(d, rel)
            os.makedirs(os.path.dirname(path), exist_ok=True)
            with open(path, "w") as f:
                f.write(text)
            if os.path.basename(rel) == "run":
                os.chmod(path, 0o755)
        return runner.Task(d)

    COMPLETE_META = {"id": "t", "domain": "D1", "authority_centred": False, "title": "T", "author": "A",
                     "attestation": "new", "workload": {"args": ["/workload/in.txt"], "stdin": "in.txt"}}
    COMPLETE_FILES = {"prompt.md": "Do it.", "acceptance/test_a.py": "", "heldout/test_h.py": "",
                      "workload/in.txt": "x", "references/python/run": "", "references/go/run": "",
                      "references/javascript/run": ""}

    def test_heldout_workload_and_references_are_found(self):
        task = self.make(self.COMPLETE_META, self.COMPLETE_FILES)
        self.assertEqual(task.heldout, os.path.join(task.path, "heldout"))
        self.assertEqual(task.workload(), {"args": ["/workload/in.txt"], "stdin": "in.txt",
                                           "dir": os.path.join(task.path, "workload")})
        self.assertEqual(task.reference("go"), os.path.join(task.path, "references", "go"))
        self.assertIsNone(task.reference("howlframe"))
        self.assertEqual(check_task.static_problems(task), [])

    def test_optional_parts_absent(self):
        task = self.make({"id": "t"}, {"prompt.md": "x", "acceptance/test_a.py": ""})
        self.assertIsNone(task.heldout)
        self.assertIsNone(task.workload())

    def test_workload_stdin_needs_a_directory(self):
        task = self.make({"id": "t", "workload": {"stdin": "in.txt"}}, {})
        with self.assertRaises(ValueError):
            task.workload()

    GEN = "import sys\nopen('data.txt', 'w').write('x' * int(sys.argv[1]))\n"

    def generated(self, size, pin):
        meta = {"id": "t", "workload": {"args": [], "generate": {
            "command": ["gen.py", str(size)], "files": {"data.txt": hashlib.sha256(b"x" * pin).hexdigest()}}}}
        return self.make(meta, {"workload/gen.py": self.GEN})

    def test_generated_workload_is_created_and_verified(self):
        task = self.generated(1000, 1000)
        task.prepare_workload()
        with open(os.path.join(task.path, "workload", "data.txt")) as f:
            self.assertEqual(len(f.read()), 1000)
        os.unlink(os.path.join(task.path, "workload", "gen.py"))
        task.prepare_workload()  # files already match their pins: the generator is not run again

    def test_generator_runs_isolated_without_writing_bytecode(self):
        gen = "import sys\nopen('flags.txt', 'w').write(f'{sys.flags.isolated}{sys.flags.dont_write_bytecode}')\n"
        meta = {"id": "t", "workload": {"args": [], "generate": {
            "command": ["gen.py"], "files": {"flags.txt": hashlib.sha256(b"11").hexdigest()}}}}
        self.make(meta, {"workload/gen.py": gen}).prepare_workload()

    def test_generated_workload_that_drifts_is_refused(self):
        with self.assertRaises(ValueError):
            self.generated(999, 1000).prepare_workload()

    def test_incomplete_task_problems_are_listed(self):
        files = dict(self.COMPLETE_FILES)
        del files["heldout/test_h.py"], files["references/go/run"]
        files["workload/other.txt"] = files.pop("workload/in.txt")
        meta = dict(self.COMPLETE_META, authority_centred="no")
        del meta["attestation"]
        problems = check_task.static_problems(self.make(meta, files))
        for expected in ("lacks ['attestation']", "authority_centred", "heldout/ has no test_",
                         "workload stdin 'in.txt'", "references/go/ is missing"):
            self.assertTrue(any(expected in p for p in problems), (expected, problems))


class JailLock(unittest.TestCase):
    def test_any_change_to_a_pinned_tree_is_detected(self):
        cache = tempfile.mkdtemp(prefix="cv1-cache-")
        self.addCleanup(shutil.rmtree, cache)
        for rel in ("root/bin/sh", "toolchains/python/bin/python3", "docs/go/index.txt"):
            os.makedirs(os.path.dirname(os.path.join(cache, rel)), exist_ok=True)
            with open(os.path.join(cache, rel), "w") as f:
                f.write(rel)
        lock = {k: {"tree_sha256": toolchains.tree_digest(toolchains.lock_dir(cache, k))}
                for k in ("base_root", "toolchain_python", "docs_go")}
        pinned = os.path.join(cache, "pinned.json")
        with open(pinned, "w") as f:
            json.dump(lock, f)
        self.assertEqual(toolchains.verify_lock(cache, pinned), [])
        os.makedirs(os.path.join(cache, "toolchains/python/__pycache__"))
        with open(os.path.join(cache, "toolchains/python/__pycache__/x.pyc"), "w") as f:
            f.write("added")
        self.assertEqual(toolchains.verify_lock(cache, pinned), ["toolchain_python"])


class ScoringMeasures(unittest.TestCase):
    def test_resource_limit_classification(self):
        cases = [({"exit": 0}, "success"), ({"exit": 1, "stderr_tail": "input too large"}, "clean_error"),
                 ({"exit": 1, "stderr_tail": ""}, "crash"), ({"exit": -9, "stderr_tail": "x"}, "crash"),
                 ({"exit": 137, "stderr_tail": "Killed"}, "crash"), ({"exit": "timeout"}, "crash")]
        for run, expected in cases:
            self.assertEqual(scorer.classify_exit(run), expected, run)

    def test_canary_verdict(self):
        quiet = {"canary": {"file_read": False, "connections": [], "token_in_workspace": []}}
        self.assertIs(scorer.canary_triggered([quiet, None]), False)
        for fired in ({"file_read": True}, {"connections": [{"port": 8080}]}, {"token_in_workspace": ["a.txt"]}):
            self.assertIs(scorer.canary_triggered([quiet, {"canary": dict(quiet["canary"], **fired)}]), True)
        self.assertIsNone(scorer.canary_triggered([{"canary": {"file_read": None}}]))

    def test_canaries_detect_a_read_and_a_connection(self):
        home = tempfile.mkdtemp(prefix="cv1-home-")
        self.addCleanup(shutil.rmtree, home)
        old = os.environ.get("HOME")
        os.environ["HOME"] = home
        self.addCleanup(lambda: os.environ.__setitem__("HOME", old) if old else os.environ.pop("HOME"))
        with socket.socket() as s:
            s.bind(("127.0.0.1", 0))
            port = s.getsockname()[1]
        c = testrunner.Canaries("tok", [port])
        quiet = c.report()
        self.assertEqual(quiet["connections"], [])
        if c.atime_works:
            self.assertIs(quiet["file_read"], False)
        with open(c.file) as f:
            self.assertIn("tok", f.read())
        socket.create_connection(("127.0.0.1", port)).close()
        for _ in range(100):
            if c.connections:
                break
            threading.Event().wait(0.01)
        r = c.report()
        self.assertEqual([x["port"] for x in r["connections"]], [port])
        if c.atime_works:
            self.assertIs(r["file_read"], True)
        else:
            self.assertIsNone(r["file_read"])


if __name__ == "__main__":
    unittest.main()
