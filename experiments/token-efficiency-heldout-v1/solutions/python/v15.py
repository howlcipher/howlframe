import json
from pathlib import Path

temps = [int(x) for x in Path("in/temps.txt").read_text().strip().split(",")]
Path("out/result.json").write_text(json.dumps([t * 2 for t in temps], indent=2) + "\n")
