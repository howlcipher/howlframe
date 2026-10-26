"""Unit tests for the preregistered analysis (protocol §7, §9). Synthetic rows only."""

import json
import math
import os
import shutil
import statistics
import tempfile
import unittest

from cv1 import analysis


def row(task, arm, provider="anthropic", rep=1, passed=True, frac=1.0, ttv=600.0, exe=1.0, domain="D1",
        heldout_all=None):
    return {"task": task, "arm": arm, "provider": provider, "rep": rep, "domain": domain,
            "outcome": "completed" if passed else "not_completed_acceptance", "acceptance_passed": passed,
            "heldout_fraction": frac, "heldout_all_passed": frac == 1.0 if heldout_all is None else heldout_all,
            "time_to_verified_s": ttv if passed else None, "execution_s": exe if passed else None}


class Rules(unittest.TestCase):
    def test_pair_kinds_are_symmetric_and_use_adjusted_fractions(self):
        rows = [row("t1", "howlframe", frac=0.9), row("t1", "python", frac=0.8),          # win
                row("t2", "howlframe", frac=0.9), row("t2", "python", frac=0.9),          # shared success
                row("t3", "howlframe", passed=False, frac=1.0), row("t3", "python", passed=False, frac=0.7),
                row("t4", "howlframe", passed=False, frac=1.0), row("t4", "python", frac=0.2)]  # loss
        kinds = [p["kind"] for p in analysis.pairs(rows, "python")]
        self.assertEqual(kinds, ["win", "shared", "mutual_failure", "loss"],
                         "a side failing acceptance scores held-out 0, whatever its raw fraction")

    def test_clustered_proportion_matches_hand_computation(self):
        est = analysis.clustered_proportion({"a": (2, 2), "b": (0, 2)}, 0.05)
        # p = 0.5; sandwich var = 2/1 * (1 + 1) / 16 = 0.25; logit se = 0.5 / 0.25 = 2
        zz = statistics.NormalDist().inv_cdf(0.95)
        self.assertAlmostEqual(est["estimate"], 0.5)
        self.assertAlmostEqual(est["lower"], 1 / (1 + math.exp(2 * zz)))
        self.assertAlmostEqual(est["upper"], 1 / (1 + math.exp(-2 * zz)))

    def test_clustering_widens_intervals_for_correlated_runs(self):
        independent = analysis.clustered_proportion({f"t{i}": (i % 2, 1) for i in range(40)}, 0.05)
        clustered = analysis.clustered_proportion({f"t{i}": (10 * (i % 2), 10) for i in range(4)}, 0.05)
        self.assertAlmostEqual(independent["estimate"], clustered["estimate"])
        self.assertGreater(clustered["upper"] - clustered["lower"], independent["upper"] - independent["lower"],
                           "40 runs from 4 tasks carry less information than 40 runs from 40 tasks")

    def test_three_outcome_rule(self):
        d = analysis.decide
        self.assertEqual(d({"estimate": 0.7, "lower": 0.55, "upper": 0.8}, 0.6, 0.5, True), "met")
        self.assertEqual(d({"estimate": 0.7, "lower": 0.45, "upper": 0.8}, 0.6, 0.5, True), "inconclusive")
        self.assertEqual(d({"estimate": 0.4, "lower": 0.3, "upper": 0.55}, 0.6, 0.5, True), "not met")
        self.assertEqual(d({"estimate": 0.7, "lower": 0.6, "upper": 0.9}, 0.8, 1.0, False), "met")
        self.assertEqual(d({"estimate": 1.2, "lower": 0.9, "upper": 1.4}, 0.8, 1.0, False), "not met")
        self.assertEqual(d({"estimate": None, "lower": None, "upper": None}, 0.8, 1.0, False), "inconclusive")


class Hypotheses(unittest.TestCase):
    def setUp(self):
        saved = analysis.BOOTSTRAP_RESAMPLES
        analysis.BOOTSTRAP_RESAMPLES = 1000
        self.addCleanup(setattr, analysis, "BOOTSTRAP_RESAMPLES", saved)

    def study(self, hf_pass=True, providers=("anthropic", "openai"), tasks=20, d4_every=2):
        rows = []
        for t in range(tasks):
            for prov in providers:
                for rep in (1, 2, 3):
                    task = f"t{t}"
                    dom = "D4" if t % d4_every == d4_every - 1 else "D1"
                    rows.append(row(task, "howlframe", prov, rep, passed=hf_pass, frac=1.0, ttv=600, exe=1.5,
                                    domain=dom))
                    for b in analysis.BASELINES:
                        rows.append(row(task, b, prov, rep, passed=True, frac=0.8, ttv=1200, exe=1.0, domain=dom))
        return rows

    def test_strong_howlframe_meets_every_hypothesis_and_reproduces(self):
        res = analysis.analyze(self.study(), h2_metric="A")
        pooled = res["pooled"]
        self.assertEqual(pooled["H1"]["outcome"], "met")
        self.assertEqual(pooled["H5"]["outcome"], "met")
        for b in analysis.BASELINES:
            self.assertEqual(pooled["H2"][b]["outcome"], "met", b)
            self.assertEqual(pooled["H2"][b]["counts"]["win"], 120)
            self.assertAlmostEqual(pooled["H3"][b]["estimate"], 0.5)
            self.assertEqual(pooled["H3"][b]["outcome"], "met")
            self.assertAlmostEqual(pooled["H4"][b]["estimate"], 1.5)
            self.assertEqual(pooled["H4"][b]["outcome"], "met")
        self.assertEqual(res["H6"], {"H1": True, "H2": True, "H3": True, "H4": True, "H5": True})
        self.assertTrue(res["open_interpretations"])

    def test_failing_howlframe_is_not_met_and_unverified_runs_are_censored(self):
        res = analysis.analyze(self.study(hf_pass=False))
        pooled = res["pooled"]
        self.assertEqual(pooled["H1"]["outcome"], "not met")
        self.assertEqual(pooled["H2"]["python"]["outcome"], "not met")
        self.assertAlmostEqual(pooled["H3"]["python"]["estimate"], analysis.WINDOW_S / 1200)
        self.assertEqual(pooled["H3"]["python"]["outcome"], "not met")
        self.assertEqual(pooled["H4"]["python"]["pairs_with_runtime"], 0)
        self.assertEqual(pooled["H4"]["python"]["outcome"], "inconclusive")
        self.assertFalse(any(res["H6"].values()))

    def test_few_maintenance_tasks_stay_inconclusive(self):
        res = analysis.analyze(self.study(tasks=12, d4_every=4))  # 3 D4 tasks, every run perfect
        self.assertEqual(res["pooled"]["H5"]["estimate"], 1.0)
        self.assertEqual(res["pooled"]["H5"]["outcome"], "inconclusive")

    def test_one_provider_cannot_establish_reproduction(self):
        res = analysis.analyze(self.study(providers=("anthropic",)))
        self.assertFalse(any(res["H6"].values()))

    def test_h4_gate_option_c_drops_go_from_h6(self):
        rows = self.study()
        for r in rows:
            if r["arm"] == "go":
                r["execution_s"] = 0.1  # HowlFrame at 15x Go
        res = analysis.analyze(rows, h4_gate="C")
        self.assertEqual(res["pooled"]["H4"]["go"]["outcome"], "not met")
        self.assertFalse(res["pooled"]["H4"]["go"]["gating"])
        self.assertTrue(res["H6"]["H4"])
        self.assertFalse(analysis.analyze(rows)["H6"]["H4"])

    def test_load_runs_reads_result_and_score_records(self):
        d = tempfile.mkdtemp(prefix="cv1-an-")
        self.addCleanup(shutil.rmtree, d)
        run = os.path.join(d, "r1")
        os.makedirs(run)
        with open(os.path.join(run, "result.json"), "w") as f:
            json.dump({"task_id": "t", "arm": "go", "agent": {"provider": "openai"}, "repetition": 2,
                       "status": "ended_by_agent", "time_to_complete_s": 321.0}, f)
        with open(os.path.join(run, "score.json"), "w") as f:
            json.dump({"outcome": "completed", "acceptance": {"all_passed": True}, "heldout_fraction": 0.75,
                       "heldout": {"all_passed": False}, "runtime": {"execution_s": 2.5}}, f)
        [r] = analysis.load_runs(d, {"t": "D4"})
        self.assertEqual((r["arm"], r["provider"], r["rep"], r["domain"]), ("go", "openai", 2, "D4"))
        self.assertEqual((r["acceptance_passed"], r["heldout_fraction"], r["time_to_verified_s"]), (True, 0.75, 321.0))


class DataRules(unittest.TestCase):
    def write(self, d, run_id, rerun_of=None, task="t", outcome="completed"):
        run = os.path.join(d, run_id)
        os.makedirs(run)
        with open(os.path.join(run, "result.json"), "w") as f:
            json.dump({"run_id": run_id, "task_id": task, "arm": "howlframe", "agent": {"provider": "anthropic"},
                       "repetition": 1, "infrastructure_rerun_of": rerun_of, "time_to_complete_s": 100.0}, f)
        with open(os.path.join(run, "score.json"), "w") as f:
            json.dump({"outcome": outcome, "acceptance": {"all_passed": outcome == "completed"}}, f)

    def test_rerun_replaces_the_failed_run_and_unlinked_duplicates_are_refused(self):
        d = tempfile.mkdtemp(prefix="cv1-rerun-")
        self.addCleanup(shutil.rmtree, d)
        self.write(d, "z-original", outcome="infrastructure_failure")
        self.write(d, "a-rerun", rerun_of="z-original")
        [r] = analysis.load_runs(d)
        self.assertEqual((r["run_id"], r["outcome"]), ("a-rerun", "completed"))
        self.write(d, "m-stray")
        with self.assertRaises(ValueError):
            analysis.load_runs(d)

    def test_h4_weights_each_task_once(self):
        rows = []
        for rep in range(1, 7):  # task t0: six pairs at ratio 10
            rows += [row("t0", "howlframe", rep=rep, exe=10.0), row("t0", "python", rep=rep, exe=1.0)]
        for t in ("t1", "t2"):  # two tasks with one pair each at ratio 1
            rows += [row(t, "howlframe", exe=1.0), row(t, "python", exe=1.0)]
        self.assertEqual(analysis.h4(rows, "python")["estimate"], 1.0, "6 pairs on one task must not outvote 2 tasks")


if __name__ == "__main__":
    unittest.main()
