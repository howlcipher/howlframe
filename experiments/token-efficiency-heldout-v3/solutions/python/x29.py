import json
from pathlib import Path

v = json.loads(Path("in/visits.json").read_text())
out = [x for i, x in enumerate(v) if x not in v[i + 1:]]
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
