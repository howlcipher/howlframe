import json
from itertools import accumulate
from pathlib import Path

temps = [int(x) for x in Path("in/temps.txt").read_text().strip().split(",")]
Path("out/result.json").write_text(json.dumps(list(accumulate(temps)), indent=2) + "\n")
