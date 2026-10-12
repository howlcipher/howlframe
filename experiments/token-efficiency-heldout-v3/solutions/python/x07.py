import json
from pathlib import Path

rows = json.loads(Path("in/rows.json").read_text())
ren = json.loads(Path("in/rename.json").read_text())
out = [{ren.get(k, k): v for k, v in r.items()} for r in rows]
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
