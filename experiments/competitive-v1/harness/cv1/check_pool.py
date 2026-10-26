"""Check a candidate pool before its manifest is frozen (protocol §3.2 steps 1 to 3).

    python3 -m cv1.check_pool POOL_DIR --out OUT [--cache CACHE] [--corpus DIR ...]

For every candidate `POOL_DIR/<id>/`: validates task.json, prompt.md (and baseline_spec.md for D4) and the acceptance
tests, then scores the author's screen implementations (`screen/python`, `screen/javascript`) exactly as an agent's
workspace is scored: fresh jail, acceptance tests, canaries, delegation scan. Held-out tests, references in every
language and workload sizing come after the draw, for drawn tasks only.

`--corpus` directories (the HowlFrame-repository task corpora named in §3.2 step 1) are compared with each prompt by
word 5-gram overlap; the highest matches are reported for the reviewer, who decides.

Writes `pool_report.json`, including `manifest_sha256`: a digest over every candidate's tree manifest. Exits 0 only if
every candidate passes, each domain keeps at least six, and authority labels allow the §3.2 step 3 cap.
"""

import argparse
import glob
import hashlib
import json
import os
import re
import shutil
import sys

from . import runner, scorer

DOMAINS = ("D1", "D2", "D3", "D4")
SCREEN_ARMS = ("python", "javascript")
REQUIRED_META = ("id", "domain", "authority_centred", "title", "author", "attestation")
MIN_PER_DOMAIN = 6
CORPUS_SUFFIXES = (".md", ".json", ".txt", ".howl")


def shingles(text, n=5):
    words = re.findall(r"[a-z0-9]+", text.lower())
    return {" ".join(words[i:i + n]) for i in range(max(0, len(words) - n + 1))}


def corpus_index(dirs):
    index = []
    for d in dirs:
        for path in glob.glob(os.path.join(d, "**", "*"), recursive=True):
            if path.endswith(CORPUS_SUFFIXES) and os.path.isfile(path):
                try:
                    with open(path, errors="replace") as f:
                        sh = shingles(f.read())
                except OSError:
                    continue
                if sh:
                    index.append((path, sh))
    return index


def closest(prompt_shingles, index, top=3):
    scored = []
    for path, sh in index:
        if prompt_shingles:
            overlap = len(prompt_shingles & sh) / len(prompt_shingles)
            if overlap > 0:
                scored.append({"file": path, "shared_5gram_fraction": round(overlap, 4)})
    return sorted(scored, key=lambda x: -x["shared_5gram_fraction"])[:top]


def check_candidate(cand_dir, layout, out, limits, index):
    task = runner.Task(cand_dir)
    problems = []
    meta = task.meta
    missing = [k for k in REQUIRED_META if k not in meta]
    if missing:
        problems.append(f"task.json lacks {missing}")
    if meta.get("domain") not in DOMAINS:
        problems.append(f"domain {meta.get('domain')!r}")
    if meta.get("authority_centred") not in (True, False):
        problems.append("authority_centred must be true or false")
    if os.path.basename(os.path.normpath(cand_dir)) != meta.get("id"):
        problems.append("directory name differs from task id")
    if not os.path.isfile(os.path.join(cand_dir, "prompt.md")):
        problems.append("prompt.md missing")
    if meta.get("domain") == "D4" and not os.path.isfile(os.path.join(cand_dir, "baseline_spec.md")):
        problems.append("D4 candidate lacks baseline_spec.md")
    tests = [f for f in os.listdir(task.acceptance)] if os.path.isdir(task.acceptance) else []
    if not any(f.startswith("test_") and f.endswith(".py") for f in tests):
        problems.append("acceptance/ has no test_*.py")
    screens = {}
    frozen = runner.tree_manifest(task.acceptance) if tests else {}
    for arm in SCREEN_ARMS:
        src = os.path.join(cand_dir, "screen", arm)
        if not os.access(os.path.join(src, "run"), os.X_OK):
            problems.append(f"screen/{arm}/run missing or not executable")
            continue
        run_dir = os.path.join(out, meta.get("id", "unknown"), f"screen-{arm}")
        shutil.rmtree(run_dir, ignore_errors=True)
        ws = os.path.join(run_dir, "workspace")
        shutil.copytree(src, ws, symlinks=True)
        with open(os.path.join(run_dir, "result.json"), "w") as f:
            json.dump({"run_id": f"screen-{arm}", "task_id": task.id, "arm": arm, "status": "ended_by_agent",
                       "final_workspace_sha256": runner.tree_manifest(ws), "acceptance_tests_sha256": frozen,
                       "limits": limits}, f)
        s = scorer.score_run(run_dir, task, layout, frozen, limits=limits)
        acc = s["acceptance"] or {}
        screens[arm] = {"outcome": s["outcome"], "acceptance": f"{acc.get('passed', 0)}/{acc.get('total', 0)}",
                        "canary_triggered": s["canary_triggered"], "delegation_flags": s["delegation_flags"]}
        if s["outcome"] != "completed":
            problems.append(f"screen/{arm}: {s['outcome']} ({screens[arm]['acceptance']})")
        if s["canary_triggered"]:
            problems.append(f"screen/{arm} triggered a canary")
    with open(os.path.join(cand_dir, "prompt.md"), errors="replace") as f:
        prompt = f.read()
    return {"id": meta.get("id"), "domain": meta.get("domain"), "authority_centred": meta.get("authority_centred"),
            "title": meta.get("title"), "acceptance_tests": len(tests), "screens": screens,
            "closest_corpus_files": closest(shingles(prompt), index) if index else None,
            "tree_sha256": hashlib.sha256(json.dumps(runner.tree_manifest(cand_dir), sort_keys=True).encode())
            .hexdigest(), "problems": problems}


def check(pool_dir, layout, out, corpus_dirs=(), cpus=None):
    os.makedirs(out, exist_ok=True)
    limits = {"memory_max": "4G", "tasks_max": 512, "cpus": cpus, "use_systemd": True}
    index = corpus_index(corpus_dirs)
    cands = sorted(d for d in glob.glob(os.path.join(pool_dir, "*")) if os.path.isfile(os.path.join(d, "task.json")))
    results = [check_candidate(d, layout, out, limits, index) for d in cands]
    problems = []
    for dom in DOMAINS:
        ok = [r for r in results if r["domain"] == dom and not r["problems"]]
        if len(ok) < MIN_PER_DOMAIN:
            problems.append(f"{dom}: {len(ok)} passing candidates, need {MIN_PER_DOMAIN}")
        if ok and all(r["authority_centred"] for r in ok):
            problems.append(f"{dom}: every candidate is authority-centred, so the pilot cap cannot be met")
    ids = [r["id"] for r in results]
    if len(ids) != len(set(ids)):
        problems.append("duplicate candidate ids")
    manifest = {r["id"]: r["tree_sha256"] for r in results}
    report = {"pool": os.path.abspath(pool_dir), "candidates": results,
              "manifest_sha256": hashlib.sha256(json.dumps(manifest, sort_keys=True).encode()).hexdigest(),
              "per_domain": {d: sum(1 for r in results if r["domain"] == d) for d in DOMAINS},
              "authority_centred": sorted(r["id"] for r in results if r["authority_centred"]),
              "corpus_dirs": list(corpus_dirs),
              "problems": problems + [f"{r['id']}: {p}" for r in results for p in r["problems"]]}
    report["ok"] = not report["problems"]
    with open(os.path.join(out, "pool_report.json"), "w") as f:
        json.dump(report, f, indent=2)
    return report


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("pool")
    p.add_argument("--out", required=True)
    p.add_argument("--cache", default=os.environ.get("CV1_CACHE"))
    p.add_argument("--corpus", action="append", default=[])
    p.add_argument("--cpus")
    args = p.parse_args(argv)
    if not args.cache:
        p.error("--cache (or CV1_CACHE) is required")
    report = check(args.pool, runner.Layout(args.cache), args.out, args.corpus, args.cpus)
    summary = {k: report[k] for k in ("manifest_sha256", "per_domain", "authority_centred", "problems", "ok")}
    print(json.dumps(summary, indent=2))
    return 0 if report["ok"] else 1


if __name__ == "__main__":
    sys.exit(main())
