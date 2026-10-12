"""Before/after tables: frozen baseline (results/) vs post-records (postrec/results/)."""
import json, statistics
from pathlib import Path

ROOT = Path(__file__).resolve().parent
LANGS = ("python", "js", "go")


def load(d):
    rows = json.loads((ROOT / d / "tokens.json").read_text())
    out = {}
    for r in rows:
        out.setdefault(r["encoding"], {}).setdefault(r["lang"], {})[r["task"]] = r["tokens"]
    return out


def main():
    before, after = load("results"), load("postrec/results")
    cats = {t["id"]: t["category"] for t in json.loads((ROOT / "tasks/manifest.json").read_text())["tasks"]}
    ids = sorted(cats)
    md = []
    for enc in ("cl100k_base", "o200k_base"):
        b, a = before[enc], after[enc]
        md.append(f"### Headline ({enc})\n")
        md.append("| Comparison | Median per-task ratio before → after | Total-token ratio before → after | W/T/L before → after |")
        md.append("|---|---|---|---|")
        for l in LANGS:
            def stats(h):
                rs = [h[t] / b[l][t] for t in ids]
                w = sum(h[t] < b[l][t] for t in ids); ti = sum(h[t] == b[l][t] for t in ids)
                return statistics.median(rs), sum(h.values()) / sum(b[l].values()), f"{w}/{ti}/{len(ids) - w - ti}"
            m0, t0, w0 = stats(b["howl"]); m1, t1, w1 = stats(a["howl"])
            md.append(f"| HowlFrame vs {l} | {m0:.3f} → **{m1:.3f}** | {t0:.3f} → **{t1:.3f}** ({sum(b['howl'].values())} → {sum(a['howl'].values())} / {sum(b[l].values())}) | {w0} → **{w1}** |")
        md.append("")
        md.append(f"### Per category ({enc}, HowlFrame / Python)\n")
        md.append("| Category | n | Median per-task before → after | Total-token before → after |")
        md.append("|---|---|---|---|")
        for cat in sorted(set(cats.values()), key=lambda c: ["records", "text", "table", "numbers", "lists", "strings", "logic"].index(c)):
            cs = [t for t in ids if cats[t] == cat]
            f = lambda h: (statistics.median(h[t] / b["python"][t] for t in cs), sum(h[t] for t in cs) / sum(b["python"][t] for t in cs))
            (m0, t0), (m1, t1) = f(b["howl"]), f(a["howl"])
            md.append(f"| {cat} | {len(cs)} | {m0:.3f} → {m1:.3f} | {t0:.3f} → {t1:.3f} |")
        md.append("")
    md.append("### Per task (HowlFrame tokens before → after; cl100k_base and o200k_base)\n")
    md.append("| Task | Cat | cl100k howl | py | js | go | howl/py | o200k howl | py | js | go | howl/py |")
    md.append("|---|---|---|---|---|---|---|---|---|---|---|---|")
    for t in ids:
        cells = [t, cats[t]]
        for enc in ("cl100k_base", "o200k_base"):
            b, a = before[enc], after[enc]
            h0, h1 = b["howl"][t], a["howl"][t]
            r0, r1 = h0 / b["python"][t], h1 / b["python"][t]
            hv = f"{h0} → {h1}" if h0 != h1 else f"{h0} (=)"
            rv = f"{r0:.2f} → **{r1:.2f}**" if r1 < 1 else f"{r0:.2f} → {r1:.2f}"
            cells += [hv, str(b["python"][t]), str(b["js"][t]), str(b["go"][t]), rv]
        md.append("| " + " | ".join(cells) + " |")
    (ROOT / "postrec/results/compare.md").write_text("\n".join(md) + "\n")
    print("\n".join(md))


if __name__ == "__main__":
    main()
