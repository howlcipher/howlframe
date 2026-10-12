import json
from pathlib import Path

records = json.loads(Path("in/records.json").read_text())
slim = [{"id": r["id"], "status": r["status"]} for r in records]
Path("out/result.json").write_text(json.dumps(slim, indent=2) + "\n")
