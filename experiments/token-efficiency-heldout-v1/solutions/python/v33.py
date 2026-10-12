import json
from pathlib import Path

rows = Path("in/grid.txt").read_text().split("\n")
Path("out/result.json").write_text(json.dumps([row.count("#") for row in rows], indent=2) + "\n")
