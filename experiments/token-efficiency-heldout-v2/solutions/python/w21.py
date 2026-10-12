import json
from pathlib import Path

out = {}
for s in json.loads(Path("in/staff.json").read_text()):
    if s["manager"]:
        out.setdefault(s["manager"], []).append(s["name"])
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
