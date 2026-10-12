from pathlib import Path

spans = sorted(tuple(map(int, l.split("-"))) for l in Path("in/spans.txt").read_text().split())
out = []
for a, b in spans:
    if out and a <= out[-1][1]:
        out[-1][1] = max(out[-1][1], b)
    else:
        out.append([a, b])
Path("out/result.txt").write_text("\n".join(f"{a}-{b}" for a, b in out) + "\n")
