import json
from pathlib import Path

people = json.loads(Path("in/people.json").read_text())
out = {f"{p['last']}, {p['first']}": p["age"] for p in people}
Path("out/result.json").write_text(json.dumps(out, indent=2) + "\n")
