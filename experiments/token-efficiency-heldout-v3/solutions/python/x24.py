import json, math
from functools import reduce
from pathlib import Path

vals = [int(x) for x in Path("in/vals.txt").read_text().strip().split(",")]
out = {"gcd": reduce(math.gcd, vals), "lcm": reduce(lambda a, b: a * b // math.gcd(a, b), vals)}
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
