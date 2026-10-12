import json
from pathlib import Path

temps = [int(x) for x in Path("in/temps.txt").read_text().strip().split(",")]
above = [t for t in temps if t > temps[0]]
Path("out/result.json").write_text(json.dumps(above, indent=2) + "\n")
