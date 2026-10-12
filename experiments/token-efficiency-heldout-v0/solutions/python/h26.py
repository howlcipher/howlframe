import json
from pathlib import Path

items = json.loads(Path("in/items.json").read_text())
by_id = {item["id"]: item["name"] for item in items}
Path("out/result.json").write_text(json.dumps(by_id, indent=2) + "\n")
