"""Preregistered analysis of scored runs (protocol §7 decision rules, §9 task clustering).

    python3 -m cv1.analysis RUNS_ROOT --tasks TASKS_ROOT [--references REFS.json] [--h2-metric default|A|B]
                            [--h4-gate default|C]

Inputs are run directories holding `result.json` (runner) and `score.json` (scorer). Every run counts; nothing is
excluded after the fact (§7.9). Tasks are the independent units (§9):

* Proportions (H1, H2 in all three forms, H5) use the marginal mean with a task-clustered sandwich variance (GEE with
  an independence working correlation), and a Wald interval on the logit scale. If the estimate is 0 or 1, or the
  sandwich variance is zero, a Wilson interval counting each task as one observation is used instead.
* Ratios (H3 restricted mean time-to-verified, H4 median runtime ratio) use a task-cluster bootstrap (percentile,
  fixed seed).
* Per-baseline hypotheses (H2, H3, H4) use one-sided α = 0.05/3 (Bonferroni across the three baselines); H1 and H5
  use α = 0.05.

Interpretations the protocol leaves open are listed in `OPEN_INTERPRETATIONS` and printed with every analysis; they
need a ruling before the freeze.
"""

import argparse
import glob
import json
import math
import os
import random
import statistics
import sys

BASELINES = ("python", "go", "javascript")
WINDOW_S = 45 * 60
ALPHA = 0.05
BOOTSTRAP_RESAMPLES = 10000
BOOTSTRAP_SEED = 20261025
OPEN_INTERPRETATIONS = [
    "H4 decision basis: §7.4 says ratios use 'both the reference and the agent-produced implementations' but not "
    "which decides. This code decides on agent-produced matched pairs and reports the reference ratios beside them.",
    "H1/H5 alpha: §7.7's Bonferroni adjustment is 'across the three baselines'; H1 and H5 are not per baseline, so "
    "they use one-sided alpha = 0.05.",
    "H6 'pooled estimate meets its target' is read as the pooled point estimate; a provider breaks H6 when its own "
    "outcome is 'not met' (its interval lies entirely on the wrong side of the target).",
    "Interval method: GEE (independence, logit link, robust variance) for proportions; task-cluster percentile "
    "bootstrap for ratios. §9 names GEE or a mixed model for H2 and 'the same clustering' for the rest.",
]


# Loading -----------------------------------------------------------------------------------------------------
def task_domains(tasks_root):
    """{task id: domain} from every task.json under tasks_root."""
    out = {}
    for path in glob.glob(os.path.join(tasks_root, "**", "task.json"), recursive=True):
        with open(path) as f:
            meta = json.load(f)
        out[meta["id"]] = meta.get("domain")
    return out


def load_runs(root, domains=None):
    rows = []
    for result_path in sorted(glob.glob(os.path.join(root, "**", "result.json"), recursive=True)):
        run_dir = os.path.dirname(result_path)
        with open(result_path) as f:
            result = json.load(f)
        score_path = os.path.join(run_dir, "score.json")
        score = {}
        if os.path.exists(score_path):
            with open(score_path) as f:
                score = json.load(f)
        row = row_from(result, score)
        row["domain"] = (domains or {}).get(row["task"])
        rows.append(row)
    return rows


def row_from(result, score):
    acc = score.get("acceptance") or {}
    held = score.get("heldout") or {}
    runtime = score.get("runtime") or {}
    outcome = score.get("outcome") or result.get("status")
    return {"task": result["task_id"], "arm": result["arm"], "provider": result["agent"]["provider"],
            "rep": result.get("repetition", 1), "domain": None, "outcome": outcome,
            "acceptance_passed": bool(acc.get("all_passed")) and outcome == "completed",
            "heldout_fraction": score.get("heldout_fraction") or 0.0,
            "heldout_all_passed": bool(held.get("all_passed")),
            "time_to_verified_s": result.get("time_to_complete_s") if outcome == "completed" else None,
            "execution_s": runtime.get("execution_s")}


# Interval machinery --------------------------------------------------------------------------------------------
def z(alpha):
    return statistics.NormalDist().inv_cdf(1 - alpha)


def clustered_proportion(clusters, alpha):
    """clusters: {task: (sum of scores, count)}. Marginal mean, one-sided lower and upper bounds (logit Wald)."""
    clusters = {k: v for k, v in clusters.items() if v[1] > 0}
    n = sum(c for _, c in clusters.values())
    if not n:
        return {"estimate": None, "lower": None, "upper": None, "tasks": 0, "n": 0}
    y = sum(s for s, _ in clusters.values())
    p = y / n
    k = len(clusters)
    var = (k / (k - 1) if k > 1 else 1.0) * sum((s - p * c) ** 2 for s, c in clusters.values()) / n ** 2
    zz = z(alpha)
    if var <= 0 or p in (0.0, 1.0):
        # No between-task variation to estimate from (e.g. every run passed): a Wilson interval that counts each
        # task as a single observation, which is the most conservative clustering.
        centre = (p + zz * zz / (2 * k)) / (1 + zz * zz / k)
        half = zz * math.sqrt(p * (1 - p) / k + zz * zz / (4 * k * k)) / (1 + zz * zz / k)
        return {"estimate": p, "lower": max(0.0, centre - half), "upper": min(1.0, centre + half), "tasks": k,
                "n": n, "method": "wilson, one observation per task"}
    se = math.sqrt(var) / (p * (1 - p))
    logit = math.log(p / (1 - p))
    expit = (lambda x: 1 / (1 + math.exp(-x)))
    return {"estimate": p, "lower": expit(logit - zz * se), "upper": expit(logit + zz * se), "tasks": k, "n": n,
            "method": "gee independence, robust variance, logit wald"}


def bootstrap(by_task, statistic, alpha, resamples=None, seed=BOOTSTRAP_SEED):
    """Task-cluster percentile bootstrap. by_task: {task: [rows]}; statistic(list of rows) -> float or None."""
    resamples = resamples or BOOTSTRAP_RESAMPLES
    tasks = sorted(by_task)
    point = statistic([r for t in tasks for r in by_task[t]])
    if point is None or not tasks:
        return {"estimate": point, "lower": None, "upper": None, "tasks": len(tasks)}
    rng = random.Random(seed)
    stats = []
    for _ in range(resamples):
        sample = [r for t in (rng.choice(tasks) for _ in tasks) for r in by_task[t]]
        v = statistic(sample)
        if v is not None:
            stats.append(v)
    stats.sort()
    q = (lambda a: stats[min(len(stats) - 1, max(0, int(a * len(stats))))])
    return {"estimate": point, "lower": q(alpha), "upper": q(1 - alpha), "tasks": len(tasks),
            "resamples_used": len(stats)}


def decide(est, target, no_advantage, higher_is_better):
    """§7.7 three-outcome rule."""
    e, lo, hi = est["estimate"], est["lower"], est["upper"]
    if e is None or lo is None:
        return "inconclusive"
    if higher_is_better:
        if e >= target and lo > no_advantage:
            return "met"
        if hi < target:
            return "not met"
    else:
        if e <= target and hi < no_advantage:
            return "met"
        if lo > target:
            return "not met"
    return "inconclusive"


# Hypotheses ------------------------------------------------------------------------------------------------------
def _clusters(rows, score, count=lambda r: 1):
    out = {}
    for r in rows:
        s, c = out.get(r["task"], (0.0, 0))
        out[r["task"]] = (s + score(r), c + count(r))
    return out


def h1(rows):
    hf = [r for r in rows if r["arm"] == "howlframe"]
    est = clustered_proportion(_clusters(hf, lambda r: 1.0 if r["acceptance_passed"] else 0.0), ALPHA)
    return dict(est, outcome=decide(est, 0.80, 0.70, True))


def pairs(rows, baseline):
    """§7.2 matched pairs: same task, provider and repetition. A side failing acceptance scores held-out 0."""
    index = {(r["task"], r["provider"], r["rep"], r["arm"]): r for r in rows}
    out = []
    for (task, prov, rep, arm), r in sorted(index.items()):
        if arm != "howlframe":
            continue
        b = index.get((task, prov, rep, baseline))
        if b is None:
            continue
        fh = r["heldout_fraction"] if r["acceptance_passed"] else 0.0
        fb = b["heldout_fraction"] if b["acceptance_passed"] else 0.0
        kind = "win" if fh > fb else "shared" if fh == fb > 0 else "mutual_failure" if fh == fb == 0 else "loss"
        out.append({"task": task, "provider": prov, "rep": rep, "kind": kind, "hf": r, "base": b})
    return out


H2_METRICS = {
    "default": (lambda p: 1.0 if p["kind"] == "win" else 0.0, lambda p: 1),
    "A": (lambda p: {"win": 1.0, "shared": 0.5}.get(p["kind"], 0.0), lambda p: 1),
    "B": (lambda p: 1.0 if p["kind"] == "win" else 0.0, lambda p: 1 if p["kind"] in ("win", "loss") else 0),
}


def h2(rows, baseline, metric):
    ps = pairs(rows, baseline)
    counts = {k: sum(1 for p in ps if p["kind"] == k) for k in ("win", "shared", "mutual_failure", "loss")}
    alpha = ALPHA / len(BASELINES)
    forms = {}
    for name, (score, count) in H2_METRICS.items():
        forms[name] = clustered_proportion(_clusters(ps, score, count), alpha)
    est = forms[metric]
    return dict(est, outcome=decide(est, 0.60, 0.50, True), metric=metric, all_forms=forms, counts=counts)


def rmst(rows):
    vals = [min(r["time_to_verified_s"], WINDOW_S) if r["acceptance_passed"] and r["time_to_verified_s"] is not None
            else WINDOW_S for r in rows]
    return statistics.fmean(vals) if vals else None


def h3(rows, baseline):
    keep = [r for r in rows if r["arm"] in ("howlframe", baseline)]
    by_task = {}
    for r in keep:
        by_task.setdefault(r["task"], []).append(r)

    def ratio(sample):
        a = rmst([r for r in sample if r["arm"] == "howlframe"])
        b = rmst([r for r in sample if r["arm"] == baseline])
        return a / b if a and b else None

    est = bootstrap(by_task, ratio, ALPHA / len(BASELINES))
    return dict(est, outcome=decide(est, 0.80, 1.0, False))


def h4(rows, baseline, reference_runtimes=None):
    ps = [p for p in pairs(rows, baseline)
          if p["hf"]["execution_s"] and p["base"]["execution_s"]]
    by_task = {}
    for p in ps:
        by_task.setdefault(p["task"], []).append(p["hf"]["execution_s"] / p["base"]["execution_s"])

    def median_ratio(sample):
        return statistics.median(sample) if sample else None

    est = bootstrap(by_task, median_ratio, ALPHA / len(BASELINES))
    refs = {}
    for task, times in sorted((reference_runtimes or {}).items()):
        if times.get("howlframe") and times.get(baseline):
            refs[task] = times["howlframe"] / times[baseline]
    return dict(est, outcome=decide(est, 2.0, 3.0, False), pairs_with_runtime=len(ps), reference_ratios=refs)


def h5(rows):
    d4 = [r for r in rows if r["arm"] == "howlframe" and r.get("domain") == "D4"]
    score = (lambda r: 1.0 if r["acceptance_passed"] and r["heldout_all_passed"] else 0.0)
    est = clustered_proportion(_clusters(d4, score), ALPHA)
    return dict(est, outcome=decide(est, 0.80, 0.70, True))


def analyze(rows, h2_metric="default", h4_gate="default", reference_runtimes=None):
    gated = BASELINES if h4_gate == "default" else ("python", "javascript")

    def run_all(subset):
        res = {"H1": h1(subset), "H5": h5(subset), "H2": {}, "H3": {}, "H4": {}}
        for b in BASELINES:
            res["H2"][b] = h2(subset, b, h2_metric)
            res["H3"][b] = h3(subset, b)
            res["H4"][b] = dict(h4(subset, b, reference_runtimes), gating=b in gated)
        return res

    pooled = run_all(rows)
    providers = sorted({r["provider"] for r in rows})
    per_provider = {p: run_all([r for r in rows if r["provider"] == p]) for p in providers}
    return {"pooled": pooled, "per_provider": per_provider, "H6": h6(pooled, per_provider, gated),
            "runs": len(rows), "tasks": len({r["task"] for r in rows}), "h2_metric": h2_metric, "h4_gate": h4_gate,
            "open_interpretations": OPEN_INTERPRETATIONS}


def _meets(est, target, higher):
    e = est.get("estimate")
    return e is not None and (e >= target if higher else e <= target)


def h6(pooled, per_provider, gated):
    """§7.6: holds per hypothesis if the pooled estimate meets its target and no provider's outcome is 'not met'."""
    spec = {"H1": (0.80, True), "H2": (0.60, True), "H3": (0.80, False), "H4": (2.0, False), "H5": (0.80, True)}
    out = {}
    for h, (target, higher) in spec.items():
        if h in ("H1", "H5"):
            cells = [(pooled[h], [pp[h] for pp in per_provider.values()])]
        else:
            bases = gated if h == "H4" else BASELINES
            cells = [(pooled[h][b], [pp[h][b] for pp in per_provider.values()]) for b in bases]
        out[h] = (len(per_provider) >= 2 and all(_meets(p, target, higher) and
                                                 all(x["outcome"] != "not met" for x in provs)
                                                 for p, provs in cells))
    return out


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("root")
    p.add_argument("--tasks", required=True, help="directory holding the tasks' task.json files (for domains)")
    p.add_argument("--references", help="JSON {task: {arm: execution_s}} from check_task reports")
    p.add_argument("--h2-metric", choices=sorted(H2_METRICS), default="A",
                   help="Owner option recorded 2026-10-22: A")
    p.add_argument("--h4-gate", choices=("default", "C"), default="default")
    args = p.parse_args(argv)
    refs = None
    if args.references:
        with open(args.references) as f:
            refs = json.load(f)
    rows = load_runs(args.root, task_domains(args.tasks))
    print(json.dumps(analyze(rows, args.h2_metric, args.h4_gate, refs), indent=2, default=str))
    return 0


if __name__ == "__main__":
    sys.exit(main())
