import json
from pathlib import Path

tickets = json.loads(Path("in/tickets.json").read_text())
ids = [t["id"] for t in tickets if "urgent" in t["meta"].get("tags", [])]
Path("out/result.txt").write_text("\n".join(ids) + "\n")
