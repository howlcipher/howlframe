import json
from pathlib import Path

a = [int(x) for x in Path("in/load.txt").read_text().strip().split(",")]
Path("out/result.json").write_text(json.dumps([sum(a[i:i + 3]) // 3 for i in range(len(a) - 2)], indent=2) + "\n")
