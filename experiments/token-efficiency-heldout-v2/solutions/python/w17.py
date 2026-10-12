from itertools import groupby
from pathlib import Path

raw = Path("in/raw.txt").read_text().strip()
Path("out/result.txt").write_text("".join(f"{c}{len(list(g))}" for c, g in groupby(raw)) + "\n")
