import json
from pathlib import Path

m = json.loads(Path("in/meter.json").read_text())
Path("out/result.json").write_text(json.dumps([b - a for a, b in zip(m, m[1:])], indent=2) + "\n")
