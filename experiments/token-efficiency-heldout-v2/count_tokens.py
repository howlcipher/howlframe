"""Count verified heldout-v2 sources with the v0 tokenizers and stripping rules."""
import csv, hashlib, importlib.util, json, statistics
from pathlib import Path

import tiktoken
import tiktoken.load

ROOT = Path(__file__).resolve().parent
_spec = importlib.util.spec_from_file_location("v0count", ROOT.parent / "token-efficiency-v0/count_tokens.py")
v0 = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(v0)  # reuse strip_comments and offline guard unchanged
LANGUAGES = v0.LANGUAGES


def main():
    if tiktoken.__version__ != "0.8.0":
        raise RuntimeError("Install requirements.txt: tiktoken==0.8.0 required")
    tiktoken.load.read_file = v0.offline_read
    encodings = {n: tiktoken.get_encoding(n) for n in ("cl100k_base", "o200k_base")}
    tasks = json.loads((ROOT / "tasks/manifest.json").read_text())["tasks"]
    cats = {t["id"]: t["category"] for t in tasks}
    gaps = {(g["task"], g["lang"]) for g in json.loads((ROOT / "gaps.json").read_text())}
    checks = {(r["task"], r["lang"]): r for r in json.loads((ROOT / "results/verification.json").read_text())}
    expected = {(t["id"], l) for t in tasks for l in LANGUAGES} - gaps
    if set(checks) != expected:
        raise RuntimeError("verification coverage must be every task x language minus gaps.json")
    rows = []
    for t in tasks:
        for lang, ext in LANGUAGES.items():
            if (t["id"], lang) in gaps:
                continue
            raw = (ROOT / f"solutions/{lang}/{t['id']}.{ext}").read_bytes()
            c = checks[t["id"], lang]
            if hashlib.sha256(raw).hexdigest() != c["sha256"] or not c["pass"]:
                raise RuntimeError(f"unverified or changed source {t['id']}/{lang}")
            src = raw.decode()
            stripped = v0.strip_comments(src, lang)
            for name, enc in encodings.items():
                rows.append({"task": t["id"], "category": t["category"], "lang": lang, "encoding": name,
                             "tokens": len(enc.encode(src, disallowed_special=())),
                             "tokens_nocomments": len(enc.encode(stripped, disallowed_special=())),
                             "bytes": len(raw), "pass": c["pass"]})
    (ROOT / "results/tokens.json").write_text(json.dumps(rows, indent=2) + "\n")
    with (ROOT / "results/tokens.csv").open("w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(rows[0]))
        w.writeheader()
        w.writerows(rows)
    summary = {}
    for enc in encodings:
        summary[enc] = {}
        for metric in ("tokens", "tokens_nocomments"):
            by = {l: {r["task"]: r[metric] for r in rows if r["encoding"] == enc and r["lang"] == l} for l in LANGUAGES}
            solved = sorted(by["howl"])
            s = {"languages": {l: {"median_all": statistics.median(v.values()),
                                   "median_howl_solved": statistics.median(v[t] for t in solved),
                                   "mean_howl_solved": statistics.mean(v[t] for t in solved)} for l, v in by.items()},
                 "howl_vs": {}, "categories": {}}
            for l in ("python", "js", "go"):
                ratios = [by["howl"][t] / by[l][t] for t in solved]
                s["howl_vs"][l] = {
                    "median_per_task_ratio": statistics.median(ratios),
                    "ratio_of_medians": statistics.median(by["howl"].values()) / statistics.median(by[l][t] for t in solved),
                    "wins": sum(by["howl"][t] < by[l][t] for t in solved),
                    "ties": sum(by["howl"][t] == by[l][t] for t in solved),
                    "losses": sum(by["howl"][t] > by[l][t] for t in solved),
                    "gaps": len(by[l]) - len(solved),
                }
            for cat in sorted(set(cats.values())):
                ids = [t for t in by["python"] if cats[t] == cat]
                hs = [t for t in ids if t in by["howl"]]
                s["categories"][cat] = {
                    "n": len(ids), "howl_solved": len(hs),
                    **{f"mean_{l}": (statistics.mean(by[l][t] for t in hs) if hs else None) for l in LANGUAGES},
                    "median_ratio_vs_python": statistics.median(by["howl"][t] / by["python"][t] for t in hs) if hs else None,
                }
            summary[enc][metric] = s
    (ROOT / "results/summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    print(json.dumps({e: summary[e]["tokens"]["howl_vs"] for e in summary}, indent=1))


if __name__ == "__main__":
    main()
