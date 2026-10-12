import json
from pathlib import Path

values = {"I": 1, "V": 5, "X": 10, "L": 50, "C": 100, "D": 500, "M": 1000}
out = []
for line in Path("in/romans.txt").read_text().split("\n"):
    ds = [values[c] for c in line]
    out.append(sum(-d if i + 1 < len(ds) and d < ds[i + 1] else d for i, d in enumerate(ds)))
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
